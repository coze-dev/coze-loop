// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	store "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type itemHookPorts struct {
	repo.IHookRepo
	repo.IHookItemSourceRepo
	repo.IHookTurnProgressRepo
	source                               *entity.HookRunInitialization
	run                                  *entity.HookStoredRun
	gates                                []entity.HookGateState
	sourceErr, gateErr, runErr, admitErr error
	admitResult                          entity.HookAdmitItemResult
	trace                                []string
	held                                 bool
	admitted                             []entity.HookAdmitItemInput
	beforeAdmit                          func()
}

func (p *itemHookPorts) ReadItemSource(context.Context, entity.HookRunKey) (*entity.HookRunInitialization, error) {
	p.trace = append(p.trace, "source")
	return p.source, p.sourceErr
}
func (p *itemHookPorts) CanDispatch(context.Context, entity.HookRunKey) (entity.HookAdmissionDecision, error) {
	where := "gate-outside"
	if p.held {
		where = "gate-inside"
	}
	p.trace = append(p.trace, where)
	gate := p.gates[0]
	if len(p.gates) > 1 {
		p.gates = p.gates[1:]
	}
	return entity.HookAdmissionDecision{Gate: gate}, p.gateErr
}
func (p *itemHookPorts) GetRun(context.Context, entity.HookRunKey) (*entity.HookStoredRun, error) {
	p.trace = append(p.trace, "run")
	return p.run, p.runErr
}
func (p *itemHookPorts) AdmitItem(_ context.Context, in entity.HookAdmitItemInput) (entity.HookAdmitItemResult, error) {
	p.trace = append(p.trace, "admit")
	if !p.held {
		return entity.HookAdmitItemResult{}, errors.New("admission outside item lock")
	}
	p.admitted = append(p.admitted, in)
	if p.beforeAdmit != nil {
		p.beforeAdmit()
	}
	return p.admitResult, p.admitErr
}

type itemHookLock struct {
	lock.ILocker
	ports *itemHookPorts
	err   error
	deny  bool
}

func (l *itemHookLock) LockWithRenew(ctx context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
	l.ports.trace = append(l.ports.trace, "lock")
	if l.err != nil || l.deny {
		return false, ctx, func() {}, l.err
	}
	l.ports.held = true
	return true, ctx, func() {}, nil
}
func (l *itemHookLock) Unlock(string) (bool, error) {
	l.ports.trace = append(l.ports.trace, "unlock")
	l.ports.held = false
	return true, nil
}

type itemHookPublisher struct {
	events.ExptEventPublisher
	events []*entity.ExptItemEvalEvent
	delays []time.Duration
	err    error
	mutate bool
}

func (p *itemHookPublisher) PublishExptRecordEvalEvent(_ context.Context, event *entity.ExptItemEvalEvent, delay *time.Duration, modify func(*entity.ExptItemEvalEvent)) error {
	if modify != nil {
		modify(event)
	}
	p.events = append(p.events, event)
	p.delays = append(p.delays, gptr.Indirect(delay))
	if p.mutate {
		event.Ext["scope"] = "changed"
		event.Session.UserID = "changed"
	}
	return p.err
}

type itemHookGuard struct {
	component.ICentralReservationGuard
	ports    *itemHookPorts
	releases int
	err      error
	deny     bool
}

func (g *itemHookGuard) ConfirmRunning(context.Context, string, int64, int64) (bool, error) {
	g.ports.trace = append(g.ports.trace, "reservation")
	return !g.deny, g.err
}
func (g *itemHookGuard) Release(context.Context, string, int64, int64, string) error {
	g.releases++
	return nil
}

type itemHookFixture struct {
	base, svc                                      ExptItemEvalEvent
	ports                                          *itemHookPorts
	publisher                                      *itemHookPublisher
	lock                                           *itemHookLock
	guard                                          *itemHookGuard
	event                                          *entity.ExptItemEvalEvent
	item                                           *entity.ExptItemResultRunLog
	expt                                           *entity.Experiment
	turnLogs                                       []*entity.ExptTurnResultRunLog
	configCalls, metrics, writes, preEval, details int
	itemErr                                        error
}

func newItemHookFixture(t *testing.T, aware, central bool, keys ...entity.HookRunKey) *itemHookFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	if len(keys) > 0 {
		key = keys[0]
	}
	f := &itemHookFixture{
		event:     &entity.ExptItemEvalEvent{SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, EvalSetItemID: 4, ExptRunMode: entity.EvaluationModeSubmit, RetryTimes: 2, MaxRetryTimes: 3, CreateAt: 123, Session: &entity.Session{UserID: "user"}, Ext: map[string]string{"scope": "original"}},
		item:      &entity.ExptItemResultRunLog{SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, ItemID: 4, Status: int32(entity.ItemRunState_Processing)},
		publisher: &itemHookPublisher{},
		ports: &itemHookPorts{
			source:      &entity.HookRunInitialization{LatestRunID: key.RunID, Managed: true, RunLog: &entity.ExptRunLog{ID: key.RunID, SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, Mode: 1, Status: int64(entity.ExptStatus_Processing)}},
			run:         &entity.HookStoredRun{Version: 7, PlanReady: true, State: entity.HookRunState{Key: key, Status: entity.ExptStatus_Processing, Gate: entity.HookGateReady, Finalize: entity.HookFinalizeNone, Before: entity.HookOperation{Status: entity.HookOperationDisabled}, After: entity.HookOperation{ID: "after", Status: entity.HookOperationPending}}},
			gates:       []entity.HookGateState{entity.HookGateReady},
			admitResult: entity.HookAdmitItemResult{Admitted: true, NewlyAdmitted: true, AdmittedAt: time.Unix(100, 0), Version: 8},
		},
	}
	f.lock = &itemHookLock{ports: f.ports}
	f.guard = &itemHookGuard{ports: f.ports}
	manager := svcmocks.NewMockIExptManager(ctrl)
	config := configmocks.NewMockIConfiger(ctrl)
	metric := metricmocks.NewMockExptMetric(ctrl)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	dispatch := repomocks.NewMockIExptItemDispatchRepo(ctrl)
	sets := svcmocks.NewMockEvaluationSetItemService(ctrl)
	ids := idmocks.NewMockIIDGenerator(ctrl)
	expt := &entity.Experiment{ID: key.ExperimentID, SpaceID: key.WorkspaceID, LatestRunID: key.RunID, Status: entity.ExptStatus_Processing, EvalSet: &entity.EvaluationSet{EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 12, EvaluationSetID: 11}}}
	f.expt = expt
	if central {
		expt.ExptDispatchMode = entity.ExptDispatchModeEnforce
		expt.SchedulerScope = "trusted-scope"
	}
	manager.EXPECT().GetRunLog(gomock.Any(), key.ExperimentID, key.RunID, key.WorkspaceID, gomock.Any()).Return(f.ports.source.RunLog, nil).AnyTimes()
	manager.EXPECT().GetDetail(gomock.Any(), key.ExperimentID, key.WorkspaceID, gomock.Any()).DoAndReturn(func(context.Context, int64, int64, *entity.Session, ...entity.GetExptTupleOptionFn) (*entity.Experiment, error) {
		f.details++
		return expt, nil
	}).AnyTimes()
	expts.EXPECT().GetByID(gomock.Any(), key.ExperimentID, key.WorkspaceID).Return(expt, nil).AnyTimes()
	config.EXPECT().GetErrRetryConf(gomock.Any(), key.WorkspaceID, gomock.Any()).DoAndReturn(func(context.Context, int64, error) *entity.RetryConf {
		f.configCalls++
		return &entity.RetryConf{RetryTimes: 3, RetryIntervalSecond: 1}
	}).AnyTimes()
	metric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(func(int64, int64, bool, bool, bool, int64, int64) { f.metrics++ }).AnyTimes()
	items.EXPECT().GetItemRunLog(gomock.Any(), key.ExperimentID, key.RunID, int64(4), key.WorkspaceID).DoAndReturn(func(context.Context, int64, int64, int64, int64) (*entity.ExptItemResultRunLog, error) {
		return f.item, f.itemErr
	}).AnyTimes()
	items.EXPECT().BatchGet(gomock.Any(), key.WorkspaceID, key.ExperimentID, []int64{4}).Return([]*entity.ExptItemResult{{Status: entity.ItemRunState_Processing}}, nil).AnyTimes()
	turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), key.ExperimentID, key.RunID, int64(4), key.WorkspaceID).DoAndReturn(func(context.Context, int64, int64, int64, int64) ([]*entity.ExptTurnResultRunLog, error) {
		return f.turnLogs, nil
	}).AnyTimes()
	sets.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: 4, BaseInfo: &entity.BaseInfo{}, Turns: []*entity.Turn{{ID: 9}}}}, nil).AnyTimes()
	ids.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{101}, nil).AnyTimes()
	turns.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, []*entity.ExptTurnResultRunLog) error {
		f.ports.trace = append(f.ports.trace, "PreEval")
		f.preEval++
		return errors.New("injected PreEval write failure")
	}).AnyTimes()
	dispatch.EXPECT().StartReservedItem(gomock.Any(), key.WorkspaceID, key.ExperimentID, key.RunID, int64(4)).DoAndReturn(func(context.Context, int64, int64, int64, int64) (bool, error) {
		f.ports.trace = append(f.ports.trace, "projection")
		f.writes++
		return true, nil
	}).AnyTimes()
	dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), key.WorkspaceID, key.ExperimentID, key.RunID, []int64{4}).Return(nil, nil).AnyTimes()
	f.base = NewExptRecordEvalService(manager, config, f.publisher, items, turns, nil, expts, nil, nil, f.lock, nil, nil, metric, nil, nil, sets, nil, nil, ids, nil, nil, nil, nil, f.guard, dispatch, component.NewNoopCentralSchedulerScopeOwner())
	f.svc = f.base
	if aware {
		var err error
		f.svc, err = NewHookAwareExptRecordEvalService(f.base, f.ports, f.ports, f.ports, f.ports)
		require.NoError(t, err)
	}
	return f
}

func (f *itemHookFixture) noExecution(t *testing.T) {
	t.Helper()
	require.Zero(t, f.preEval)
	require.Zero(t, f.details)
	require.Zero(t, f.writes)
	require.Zero(t, f.guard.releases)
	require.Zero(t, f.configCalls, "control flow must not enter generic retry/error handling")
	require.Zero(t, f.metrics)
	require.Equal(t, 2, f.event.RetryTimes)
}

func TestItemHookWaitingPreservesCallbackAndRetryBudget(t *testing.T) {
	for _, flags := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
		f := newItemHookFixture(t, true, true)
		f.ports.gates = []entity.HookGateState{entity.HookGateWaiting}
		f.event.AsyncReportTrigger, f.event.AsyncEvaluatorReportTrigger = flags[0], flags[1]
		require.NoError(t, f.svc.Eval(context.Background(), f.event))
		f.noExecution(t)
		require.Equal(t, []string{"source", "gate-outside"}, f.ports.trace)
		require.Len(t, f.publisher.events, 1)
		expected := *f.event
		expected.HookControlContinuation = true
		require.Equal(t, &expected, f.publisher.events[0])
		require.NotSame(t, f.event, f.publisher.events[0])
		require.Equal(t, 5*time.Second, f.publisher.delays[0])
	}
}

func TestItemHookAtomicAdmissionPrecedesReservationAndPreEval(t *testing.T) {
	f := newItemHookFixture(t, true, true)
	require.NoError(t, f.svc.Eval(context.Background(), f.event))
	require.Equal(t, []string{"source", "gate-outside", "lock", "gate-inside", "run", "admit", "reservation", "projection", "PreEval", "unlock"}, f.ports.trace)
	require.Equal(t, []entity.HookAdmitItemInput{{HookStoreGuard: entity.HookStoreGuard{Key: entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, ExpectedVersion: 7}, ItemID: 4}}, f.ports.admitted)
	require.Equal(t, 1, f.preEval)
	require.Equal(t, 1, f.writes)
	require.False(t, f.ports.held)
}

func TestItemHookFinalReadAndAtomicRacesDoNotExecute(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*itemHookFixture)
		wantErr bool
	}{
		{"final wait", func(f *itemHookFixture) {
			f.ports.gates = []entity.HookGateState{entity.HookGateReady, entity.HookGateWaiting}
		}, false},
		{"final closed", func(f *itemHookFixture) {
			f.ports.gates = []entity.HookGateState{entity.HookGateReady, entity.HookGateClosed}
		}, true},
		{"cancel at admit", func(f *itemHookFixture) {
			f.ports.beforeAdmit = func() { f.ports.admitErr = entity.ErrHookAdmissionDenied }
		}, false},
		{"version conflict", func(f *itemHookFixture) { f.ports.admitErr = entity.ErrHookStoreConflict }, false},
		{"not admitted", func(f *itemHookFixture) { f.ports.admitResult = entity.HookAdmitItemResult{} }, false},
		{"missing admission receipt", func(f *itemHookFixture) { f.ports.admitResult.AdmittedAt = time.Time{} }, false},
		{"stale admission receipt", func(f *itemHookFixture) { f.ports.admitResult.Version = 6 }, false},
		{"item read unavailable", func(f *itemHookFixture) { f.itemErr = errors.New("private SQL") }, false},
		{"item terminal", func(f *itemHookFixture) { f.item.Status = int32(entity.ItemRunState_Terminal) }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newItemHookFixture(t, true, true)
			tc.change(f)
			err := f.svc.Eval(context.Background(), f.event)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			f.noExecution(t)
			require.False(t, f.ports.held)
		})
	}
}

func TestItemHookUnknownSourceNeverFallsThrough(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*itemHookFixture)
	}{
		{"source error", func(f *itemHookFixture) { f.ports.sourceErr = errors.New("private database error") }},
		{"source nil", func(f *itemHookFixture) { f.ports.source = nil }},
		{"runlog nil", func(f *itemHookFixture) { f.ports.source.RunLog = nil }},
		{"wrong owner", func(f *itemHookFixture) { f.ports.source.RunLog.SpaceID = 8 }},
		{"unknown status", func(f *itemHookFixture) { f.ports.source.RunLog.Status = 0 }},
		{"gate error", func(f *itemHookFixture) { f.ports.gateErr = errors.New("secret") }},
		{"unknown gate", func(f *itemHookFixture) { f.ports.gates = []entity.HookGateState{"future"} }},
		{"managed run missing", func(f *itemHookFixture) { f.ports.run = nil }},
		{"managed read error", func(f *itemHookFixture) { f.ports.runErr = errors.New("private SQL") }},
		{"managed version invalid", func(f *itemHookFixture) { f.ports.run.Version = -1 }},
		{"managed state malformed", func(f *itemHookFixture) { f.ports.run.State.After.Status = "future" }},
		{"managed run wrong key", func(f *itemHookFixture) { f.ports.run.State.Key.RunID = 99 }},
		{"managed plan not ready", func(f *itemHookFixture) { f.ports.run.PlanReady = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newItemHookFixture(t, true, false)
			tc.change(f)
			require.NoError(t, f.svc.Eval(context.Background(), f.event))
			f.noExecution(t)
			require.Len(t, f.publisher.events, 1)
		})
	}
}

func TestItemHookTransientPublishFailureIsVisibleAndDoesNotMutateSource(t *testing.T) {
	f := newItemHookFixture(t, true, true)
	f.ports.gates = []entity.HookGateState{entity.HookGateWaiting}
	f.event.AsyncReportTrigger = true
	f.event.AsyncEvaluatorReportTrigger = true
	f.publisher.err = errors.New("sensitive publish error")
	f.publisher.mutate = true
	err := f.svc.Eval(context.Background(), f.event)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "sensitive")
	f.noExecution(t)
	require.Equal(t, "original", f.event.Ext["scope"])
	require.Equal(t, "user", f.event.Session.UserID)
	f.publisher.err = nil
	f.publisher.mutate = false
	require.NoError(t, f.svc.Eval(context.Background(), f.event))
	require.Len(t, f.publisher.events, 2)
	require.True(t, f.publisher.events[1].AsyncReportTrigger)
	require.True(t, f.publisher.events[1].AsyncEvaluatorReportTrigger)
	require.Equal(t, 2, f.publisher.events[1].RetryTimes)
}

func TestItemHookClosedCallbacksAreNotAcknowledgedOrGrantedByFlags(t *testing.T) {
	for _, flags := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
		f := newItemHookFixture(t, true, true)
		f.event.AsyncReportTrigger, f.event.AsyncEvaluatorReportTrigger = flags[0], flags[1]
		f.ports.gates = []entity.HookGateState{entity.HookGateClosed}
		require.Error(t, f.svc.Eval(context.Background(), f.event))
		f.noExecution(t)
		require.Empty(t, f.ports.admitted)
		require.Empty(t, f.publisher.events)
	}
}

func TestItemHookLegacyAndOriginalConstructorStayOnOldChain(t *testing.T) {
	for _, aware := range []bool{false, true} {
		f := newItemHookFixture(t, aware, false)
		f.ports.source.Managed = false
		f.ports.gateErr = errors.New("legacy must not depend on gate")
		require.NoError(t, f.svc.Eval(context.Background(), f.event))
		require.Equal(t, 1, f.preEval)
		require.NotContains(t, f.ports.trace, "admit")
		require.NotContains(t, f.ports.trace, "gate-outside")
		require.NotContains(t, f.ports.trace, "gate-inside")
		require.Equal(t, 1, f.configCalls)
	}
	f := newItemHookFixture(t, true, false)
	f.ports.gates = []entity.HookGateState{entity.HookGateWaiting}
	require.NoError(t, f.base.Eval(context.Background(), f.event))
	require.Equal(t, 1, f.preEval, "opt-in must not mutate base or retain base-bound middleware closures")
}

func TestItemHookReadyCallbacksPersistExistingResultsThroughRealConsumer(t *testing.T) {
	if os.Getenv("HOOK_MYSQL_TX_DSN") == "" {
		t.Skip("requires existing isolated HOOK_MYSQL_TX_DSN")
	}
	for _, evaluatorCallback := range []bool{false, true} {
		t.Run(map[bool]string{false: "target callback", true: "evaluator callback"}[evaluatorCallback], func(t *testing.T) {
			key := entity.HookRunKey{WorkspaceID: hookRecoveryIDs.Add(1), ExperimentID: hookRecoveryIDs.Add(1), RunID: hookRecoveryIDs.Add(1)}
			f := newItemHookFixture(t, false, true, key)
			base := f.base.(*ExptItemEventEvalServiceImpl)
			ctrl := gomock.NewController(t)
			target := svcmocks.NewMockIEvalTargetService(ctrl)
			records := svcmocks.NewMockEvaluatorRecordService(ctrl)
			base.evaTargetService, base.evaluatorRecordService = target, records
			f.event.AsyncReportTrigger = !evaluatorCallback
			f.event.AsyncEvaluatorReportTrigger = evaluatorCallback
			f.ports.admitResult.NewlyAdmitted = false
			f.ports.admitResult.Version = 7
			f.item.ID, f.item.ItemVersionID = hookRecoveryIDs.Add(1), 7
			f.expt.TargetID = 70
			f.expt.TargetVersionID = 72
			f.expt.Target = &entity.EvalTarget{ID: 70, SpaceID: key.WorkspaceID, EvalTargetVersion: &entity.EvalTargetVersion{ID: 72}}
			f.expt.EvalSet.SpaceID = key.WorkspaceID
			f.expt.EvalConf = &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{EvaluatorsConf: &entity.EvaluatorsConf{}}}
			f.expt.Evaluators = []*entity.Evaluator{{SpaceID: key.WorkspaceID, EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 81}}}
			f.turnLogs = []*entity.ExptTurnResultRunLog{{ID: hookRecoveryIDs.Add(1), SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, ItemID: 4, ItemVersionID: 7, TurnID: 9, Status: entity.TurnRunState_Processing, TargetResultID: 71, EvaluatorResultIds: &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 81, RecordID: 91}}}}}
			target.EXPECT().GetRecordByID(gomock.Any(), key.WorkspaceID, int64(71)).Return(&entity.EvalTargetRecord{ID: 71, SpaceID: key.WorkspaceID, TargetID: 70, TargetVersionID: 72, ExperimentRunID: key.RunID, ItemID: 4, ItemVersionID: 7, TurnID: 9, Status: gptr.Of(entity.EvalTargetRunStatusSuccess), EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}}}, nil)
			records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{91}, false, false).Return([]*entity.EvaluatorRecord{{ID: 91, SpaceID: key.WorkspaceID, ExperimentID: key.ExperimentID, ExperimentRunID: key.RunID, ItemID: 4, ItemVersionID: 7, TurnID: 9, TargetRecordID: 71, EvaluatorVersionID: 81, Status: entity.EvaluatorRunStatusSuccess}}, nil)
			base.configer.(*configmocks.MockIConfiger).EXPECT().BuildEvalExt(gomock.Any(), key.WorkspaceID, gomock.Any()).Return(nil)
			metric := base.metric.(*metricmocks.MockExptMetric)
			metric.EXPECT().EmitTurnExecEval(key.WorkspaceID, int64(entity.EvaluationModeSubmit))
			metric.EXPECT().EmitTurnExecResult(key.WorkspaceID, int64(entity.EvaluationModeSubmit), true, gomock.Any(), gomock.Any(), gomock.Any())
			expectedEvaluatorRecordID := int64(91)
			if !evaluatorCallback {
				benefits := benefitmocks.NewMockIBenefitService(ctrl)
				evaluators := svcmocks.NewMockEvaluatorService(ctrl)
				base.benefitService, base.evaluatorService = benefits, evaluators
				benefits.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
				f.expt.EvalConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{EvaluatorConcurNum: gptr.Of(1), EvaluatorConf: []*entity.EvaluatorConf{{EvaluatorVersionID: 81, IngressConf: &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{}, TargetAdapter: &entity.FieldAdapter{}}}}}
				evaluators.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil)
				evaluators.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
					if req.SpaceID != key.WorkspaceID || req.ExperimentID != key.ExperimentID || req.ExperimentRunID != key.RunID || req.ItemID != 4 || req.TurnID != 9 || req.EvaluatorVersionID != 81 {
						return nil, errors.New("unexpected callback evaluator scope")
					}
					return &entity.EvaluatorRecord{ID: 92, SpaceID: key.WorkspaceID, ExperimentID: key.ExperimentID, ExperimentRunID: key.RunID, ItemID: 4, ItemVersionID: 7, TurnID: 9, TargetRecordID: 71, EvaluatorVersionID: 81, Status: entity.EvaluatorRunStatusSuccess}, nil
				})
				metric.EXPECT().EmitTurnExecEvaluatorResult(key.WorkspaceID, false)
				expectedEvaluatorRecordID = 92
			}
			progress, sql := newItemHookCallbackStorage(t, f)
			var err error
			f.svc, err = NewHookAwareExptRecordEvalService(f.base, f.ports, f.ports, f.ports, progress)
			require.NoError(t, err)
			require.NoError(t, f.svc.Eval(context.Background(), f.event))
			require.Len(t, f.ports.admitted, 1, "callback flags must not bypass atomic admission")
			require.Equal(t, key, f.ports.admitted[0].Key)
			require.Equal(t, int64(4), f.ports.admitted[0].ItemID)
			var savedPOs []*model.ExptTurnResultRunLog
			require.NoError(t, sql.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", key.WorkspaceID, key.ExperimentID, key.RunID, 4).Find(&savedPOs).Error)
			var saved []*entity.ExptTurnResultRunLog
			for _, po := range savedPOs {
				row, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(po)
				require.NoError(t, err)
				saved = append(saved, row)
			}
			require.Len(t, saved, 1)
			require.Equal(t, entity.HookTurnProgressKey{HookRunKey: key, LogID: f.turnLogs[0].ID, ItemID: 4, ItemVersionID: 7, TurnID: 9}, entity.HookTurnProgressIdentity(saved[0]))
			require.Equal(t, int64(71), saved[0].TargetResultID)
			require.Len(t, saved[0].EvaluatorResultIds.Registered, 1)
			require.Equal(t, int64(81), saved[0].EvaluatorResultIds.Registered[0].VersionID)
			require.Equal(t, expectedEvaluatorRecordID, saved[0].EvaluatorResultIds.Registered[0].RecordID)
			require.Equal(t, entity.TurnRunState_Success, saved[0].Status)
			require.Empty(t, saved[0].ErrMsg)
			require.Equal(t, "original", saved[0].Ext["scope"])
			var item model.ExptItemResultRunLog
			require.NoError(t, sql.Where("id=? AND space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND item_version_id=?", f.item.ID, key.WorkspaceID, key.ExperimentID, key.RunID, 4, 7).First(&item).Error)
			require.Equal(t, int32(entity.ItemRunState_Success), item.Status)
			require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(item.ResultState))
			require.Empty(t, gptr.Indirect(item.ErrMsg))
			require.Empty(t, f.publisher.events)
			require.Equal(t, !evaluatorCallback, f.event.AsyncReportTrigger)
			require.Equal(t, evaluatorCallback, f.event.AsyncEvaluatorReportTrigger)
			require.Zero(t, f.preEval, "existing runlogs must not be replaced")
		})
	}
}

func newItemHookCallbackStorage(t *testing.T, f *itemHookFixture) (repo.IHookTurnProgressRepo, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
	if dsn == "" {
		t.Skip("requires existing isolated HOOK_MYSQL_TX_DSN")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "/Users/bytedance/fornax_workspace/_local/scratch/hook-mysql-20260922/m.sock", cfg.Addr)
	require.Equal(t, "hook_7378265404_tx", cfg.DBName)
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	sql := p.NewSession(context.Background(), db.WithMaster())
	pool, err := sql.DB()
	require.NoError(t, err)
	key := f.ports.run.State.Key
	t.Cleanup(func() {
		for _, table := range []string{"expt_turn_result_run_log", "expt_item_result_run_log", "expt_lifecycle_run_item", "expt_lifecycle_run"} {
			require.NoError(t, sql.Table(table).Where("space_id=? AND expt_id=? AND expt_run_id=?", key.WorkspaceID, key.ExperimentID, key.RunID).Delete(nil).Error)
		}
		require.NoError(t, pool.Close())
	})
	scope := f.expt.SchedulerScope
	require.Equal(t, "trusted-scope", scope)
	require.NoError(t, sql.Create(&model.ExptLifecycleRun{SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, ExecutionScope: scope, SnapshotCipher: []byte{1}, SnapshotKeyID: "k", SnapshotHash: "hash", Gate: 1, PlanState: 1}).Error)
	require.NoError(t, sql.Create(&model.ExptLifecycleRunItem{ID: hookRecoveryIDs.Add(1), SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, ItemID: 4, ItemVersionID: 7, AdmittedAt: gptr.Of(time.Now())}).Error)
	require.NoError(t, sql.Create(convert.NewExptItemResultRunLogConverter().DO2PO(f.item)).Error)
	row, err := convert.NewExptTurnResultRunLogConvertor().DO2PO(f.turnLogs[0])
	require.NoError(t, err)
	require.NoError(t, sql.Create(row).Error)
	return store.NewHookTurnProgressRepo(p, func(context.Context) (string, error) { return scope, nil }), sql
}

func TestItemHookReadyAfterWaitUsesOriginalEvent(t *testing.T) {
	f := newItemHookFixture(t, true, true)
	f.ports.gates = []entity.HookGateState{entity.HookGateWaiting, entity.HookGateReady}
	f.event.AsyncReportTrigger = true
	require.NoError(t, f.svc.Eval(context.Background(), f.event))
	require.Len(t, f.publisher.events, 1)
	require.Equal(t, 2, f.publisher.events[0].RetryTimes)
	require.NoError(t, f.svc.Eval(context.Background(), f.publisher.events[0]))
	require.Len(t, f.ports.admitted, 1)
	require.Equal(t, 1, f.preEval)
	require.True(t, f.publisher.events[0].AsyncReportTrigger)
}

func TestItemHookDependencyValidationAndCancelledContext(t *testing.T) {
	f := newItemHookFixture(t, false, false)
	var nilPorts *itemHookPorts
	for _, tc := range []struct {
		gate   repo.IHookGateRepo
		source repo.IHookItemSourceRepo
		runs   repo.IHookRepo
	}{
		{nil, f.ports, f.ports}, {nilPorts, f.ports, f.ports}, {f.ports, nilPorts, f.ports}, {f.ports, f.ports, nilPorts},
	} {
		_, err := NewHookAwareExptRecordEvalService(f.base, tc.gate, tc.source, tc.runs, f.ports)
		require.Error(t, err)
	}
	_, err := NewHookAwareExptRecordEvalService(nil, f.ports, f.ports, f.ports, f.ports)
	require.Error(t, err)
	f.svc, err = NewHookAwareExptRecordEvalService(f.base, f.ports, f.ports, f.ports, f.ports)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, f.svc.Eval(ctx, f.event))
	require.Error(t, f.svc.Eval(nil, f.event))
	require.Error(t, f.svc.Eval(context.Background(), nil))
	f.noExecution(t)
	require.Empty(t, f.publisher.events)
}

func TestItemHookPreExecutionFailuresNeverUseItemRetryPolicy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*itemHookFixture)
		wantErr bool
	}{
		{"lock unavailable", func(f *itemHookFixture) { f.lock.err = errors.New("lock unavailable") }, false},
		{"lock busy", func(f *itemHookFixture) { f.lock.deny = true }, true},
		{"reservation unavailable", func(f *itemHookFixture) { f.guard.err = errors.New("redis unavailable") }, false},
		{"scope missing", func(f *itemHookFixture) { f.expt.SchedulerScope = "" }, true},
		{"reservation absent", func(f *itemHookFixture) { f.guard.deny = true }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newItemHookFixture(t, true, true)
			tc.change(f)
			f.event.AsyncEvaluatorReportTrigger = true
			err := f.svc.Eval(context.Background(), f.event)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			f.noExecution(t)
			for _, published := range f.publisher.events {
				require.True(t, published.AsyncEvaluatorReportTrigger)
				require.Equal(t, 2, published.RetryTimes)
			}
		})
	}
}

func TestItemHookCancellationDuringAdmissionPreservesUncertainReceipt(t *testing.T) {
	f := newItemHookFixture(t, true, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.ports.beforeAdmit = cancel
	require.Error(t, f.svc.Eval(ctx, f.event))
	f.noExecution(t)
	require.Len(t, f.ports.admitted, 1)
	require.Empty(t, f.publisher.events, "cancelled continuation must not be acknowledged")
	require.False(t, f.ports.held)
}

func TestItemHookWaitingIgnoresExhaustedRetryAndYieldFlags(t *testing.T) {
	f := newItemHookFixture(t, true, true)
	f.ports.gates = []entity.HookGateState{entity.HookGateWaiting}
	f.event.RetryTimes, f.event.MaxRetryTimes = 10, 1
	f.event.Ext[entity.RetryYieldExtKey] = "true"
	require.NoError(t, f.svc.Eval(context.Background(), f.event))
	require.Len(t, f.publisher.events, 1)
	require.Equal(t, 10, f.publisher.events[0].RetryTimes)
	require.Equal(t, 1, f.publisher.events[0].MaxRetryTimes)
	require.Equal(t, "true", f.publisher.events[0].Ext[entity.RetryYieldExtKey])
	require.Zero(t, f.configCalls)
	require.Zero(t, f.details)
	require.Zero(t, f.writes)
	require.Zero(t, f.guard.releases)
}
