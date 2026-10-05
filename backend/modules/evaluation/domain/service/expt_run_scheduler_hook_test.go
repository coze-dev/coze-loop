// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

type schedulerHookGate struct {
	decision entity.HookAdmissionDecision
	err      error
	calls    []entity.HookRunKey
	onCall   func(context.Context)
}

func (g *schedulerHookGate) CanDispatch(ctx context.Context, key entity.HookRunKey) (entity.HookAdmissionDecision, error) {
	g.calls = append(g.calls, key)
	if g.onCall != nil {
		g.onCall(ctx)
	}
	return g.decision, g.err
}

type schedulerHookManager struct {
	IExptManager
	expt      *entity.Experiment
	run       *entity.ExptRunLog
	completed int
	details   int
}

func (m *schedulerHookManager) GetRunLog(context.Context, int64, int64, int64, *entity.Session) (*entity.ExptRunLog, error) {
	return m.run, nil
}
func (m *schedulerHookManager) GetDetail(context.Context, int64, int64, *entity.Session, ...entity.GetExptTupleOptionFn) (*entity.Experiment, error) {
	m.details++
	return m.expt, nil
}
func (m *schedulerHookManager) CompleteRun(context.Context, int64, int64, int64, *entity.Session, ...entity.CompleteExptOptionFn) error {
	m.completed++
	return nil
}
func (m *schedulerHookManager) CompleteExpt(context.Context, int64, *int64, int64, *entity.Session, ...entity.CompleteExptOptionFn) error {
	m.completed++
	return nil
}

type schedulerHookConfig struct{ component.IConfiger }

func (*schedulerHookConfig) GetSchedulerAbortCtrl(context.Context) *entity.SchedulerAbortCtrl {
	return &entity.SchedulerAbortCtrl{}
}
func (*schedulerHookConfig) GetExptExecConf(context.Context, int64) *entity.ExptExecConf {
	return &entity.ExptExecConf{ZombieIntervalSecond: 60, ExptItemEvalConf: &entity.ExptItemEvalConf{}}
}
func (*schedulerHookConfig) GetConsumerConf(context.Context) *entity.ExptConsumerConf {
	return &entity.ExptConsumerConf{ExptExecConf: &entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{}}}
}

type schedulerHookLock struct{ lock.ILocker }

func (*schedulerHookLock) LockBackoffWithRenew(ctx context.Context, _ string, _ time.Duration, _ time.Duration) (bool, context.Context, func(), error) {
	return true, ctx, func() {}, nil
}
func (*schedulerHookLock) Unlock(string) (bool, error) { return true, nil }

type schedulerHookPublisher struct {
	events.ExptEventPublisher
	published      []*entity.ExptScheduleEvent
	delays         []time.Duration
	err            error
	onPublish      func(context.Context, *entity.ExptScheduleEvent)
	itemDispatches int
}

func (p *schedulerHookPublisher) PublishExptScheduleEvent(ctx context.Context, event *entity.ExptScheduleEvent, delay *time.Duration) error {
	p.published = append(p.published, event)
	if delay != nil {
		p.delays = append(p.delays, *delay)
	}
	if p.onPublish != nil {
		p.onPublish(ctx, event)
	}
	return p.err
}
func (p *schedulerHookPublisher) BatchPublishExptRecordEvalEvent(context.Context, []*entity.ExptItemEvalEvent, *time.Duration) error {
	p.itemDispatches++
	return nil
}
func (p *schedulerHookPublisher) PublishExptTurnResultFilterEvent(context.Context, *entity.ExptTurnResultFilterEvent, *time.Duration) error {
	return nil
}

type schedulerHookMode struct {
	entity.ExptSchedulerMode
	calls              []string
	toSubmit, complete []*entity.ExptEvalItem
	next               bool
	endToSubmit        int
}

func (m *schedulerHookMode) ExptStart(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error {
	m.calls = append(m.calls, "ExptStart")
	return nil
}
func (m *schedulerHookMode) ScheduleStart(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error {
	m.calls = append(m.calls, "ScheduleStart")
	return nil
}
func (m *schedulerHookMode) ScanEvalItems(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) ([]*entity.ExptEvalItem, []*entity.ExptEvalItem, []*entity.ExptEvalItem, error) {
	m.calls = append(m.calls, "ScanEvalItems")
	return m.toSubmit, nil, m.complete, nil
}
func (m *schedulerHookMode) ExptEnd(_ context.Context, _ *entity.ExptScheduleEvent, _ *entity.Experiment, toSubmit, incomplete int) (bool, error) {
	m.calls = append(m.calls, "ExptEnd")
	m.endToSubmit = toSubmit
	return m.next, nil
}
func (m *schedulerHookMode) NextTick(context.Context, *entity.ExptScheduleEvent, bool) error {
	m.calls = append(m.calls, "NextTick")
	return nil
}
func (m *schedulerHookMode) PublishResult(context.Context, []*entity.ExptTurnEvaluatorResultRef, *entity.ExptScheduleEvent) error {
	m.calls = append(m.calls, "PublishResult")
	return nil
}

type schedulerHookModeFactory struct{ mode *schedulerHookMode }

func (f schedulerHookModeFactory) NewSchedulerMode(entity.ExptRunMode) (entity.ExptSchedulerMode, error) {
	return f.mode, nil
}

type schedulerHookResults struct {
	ExptResultService
	archived []int64
}

func (r *schedulerHookResults) RecordItemRunLogs(_ context.Context, _, _ int64, item int64, _ int64, _ *entity.Experiment) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	r.archived = append(r.archived, item)
	return nil, nil
}
func (*schedulerHookResults) UpsertExptTurnResultFilter(context.Context, int64, int64, []int64) error {
	return nil
}

type schedulerHookFixture struct {
	scheduler ExptSchedulerEvent
	base      ExptSchedulerEvent
	manager   *schedulerHookManager
	mode      *schedulerHookMode
	publisher *schedulerHookPublisher
	results   *schedulerHookResults
	event     *entity.ExptScheduleEvent
}

func newSchedulerHookFixture(t *testing.T, gate repo.IHookGateRepo) *schedulerHookFixture {
	t.Helper()
	f := &schedulerHookFixture{manager: &schedulerHookManager{expt: &entity.Experiment{ID: 2, SpaceID: 1, LatestRunID: 3, Status: entity.ExptStatus_Processing}, run: &entity.ExptRunLog{ID: 3, SpaceID: 1, ExptID: 2, ExptRunID: 3, Status: int64(entity.ExptStatus_Processing)}}, mode: &schedulerHookMode{}, publisher: &schedulerHookPublisher{}, results: &schedulerHookResults{}, event: &entity.ExptScheduleEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, ExptRunMode: entity.EvaluationModeSubmit, CreatedAt: time.Now().Unix(), Session: &entity.Session{UserID: "scheduler-user", AppID: 7}, Ext: map[string]string{"scope": "ppe-hook"}, ExecEvalSetItemIDs: []int64{11, 12}}}
	f.base = NewExptSchedulerSvc(f.manager, nil, nil, nil, nil, nil, nil, nil, &schedulerHookConfig{}, nil, &schedulerHookLock{}, f.publisher, nil, nil, f.results, nil, nil, schedulerHookModeFactory{f.mode}, nil, nil, nil, nil, nil)
	f.scheduler = f.base
	if gate != nil {
		// These are Gate middleware tests over an explicitly persisted noHook Run.
		// Managed construction and persistence use the real frozen fixtures in separate tests.
		p := newFinalizationManagerFixture(t, "tx")
		require.NoError(t, p.sql.Model(&model.ExptRunLog{}).Where("id=?", p.key.RunID).UpdateColumn("lifecycle_hook_version", 0).Error)
		for _, table := range []any{&model.ExptLifecycleHookRun{}, &model.ExptLifecycleRunItem{}, &model.ExptLifecycleRun{}} {
			require.NoError(t, p.sql.Where("space_id=? AND expt_id=?", p.space, p.expt).Delete(table).Error)
		}
		f.event.SpaceID, f.event.ExptID, f.event.ExptRunID = p.space, p.expt, p.key.RunID
		f.manager.expt.ID, f.manager.expt.SpaceID, f.manager.expt.LatestRunID = p.expt, p.space, p.key.RunID
		f.manager.run.ID, f.manager.run.ExptRunID, f.manager.run.ExptID, f.manager.run.SpaceID = p.key.RunID, p.key.RunID, p.expt, p.space
		base := *f.base.(*ExptSchedulerImpl)
		base.Manager, base.ResultSvc = p.manager, &ExptResultServiceImpl{}
		aware, err := NewHookAwareExptSchedulerSvc(&base, gate)
		require.NoError(t, err)
		scheduler := aware.(*ExptSchedulerImpl)
		source, err := scheduler.hookScheduler.ReadFinalizationSource(context.Background(), p.key)
		require.NoError(t, err)
		require.False(t, source.Managed)
		// Keep read-only metadata/call counting and unrelated archive effects isolated.
		scheduler.Manager, scheduler.ResultSvc = f.manager, f.results
		f.scheduler = scheduler
	}
	return f
}

func TestSchedulerHookWaitingDoesNotStartOrScan(t *testing.T) {
	gate := &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateWaiting}}
	f := newSchedulerHookFixture(t, gate)
	require.NoError(t, f.scheduler.Schedule(context.Background(), f.event))
	require.Empty(t, f.mode.calls, "waiting must not start, scan or end the experiment")
	require.Zero(t, f.manager.details)
	require.Zero(t, f.manager.completed)
	require.Len(t, f.publisher.published, 1)
	require.Equal(t, []entity.HookRunKey{{WorkspaceID: f.event.SpaceID, ExperimentID: f.event.ExptID, RunID: f.event.ExptRunID}}, gate.calls)
}

func TestSchedulerHookWaitingBypassesZombieTermination(t *testing.T) {
	f := newSchedulerHookFixture(t, &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateWaiting}})
	f.event.CreatedAt = time.Now().Add(-time.Hour).Unix()
	require.NoError(t, f.scheduler.Schedule(context.Background(), f.event))
	require.Zero(t, f.manager.completed, "before waiting must not enter legacy zombie termination")
	require.Empty(t, f.mode.calls)
	require.Len(t, f.publisher.published, 1)
}

func TestSchedulerHookAdmissionMatrix(t *testing.T) {
	for _, tc := range []struct {
		name         string
		gate         entity.HookGateState
		err          error
		ready        bool
		publications int
	}{
		{"waiting", entity.HookGateWaiting, nil, false, 1},
		{"closed", entity.HookGateClosed, nil, false, 0},
		{"unknown", "", nil, false, 1},
		{"unknown enum", "unexpected", nil, false, 1},
		{"read failure", entity.HookGateReady, errors.New("private database failure"), false, 1},
		{"ready", entity.HookGateReady, nil, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate := &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: tc.gate}, err: tc.err}
			f := newSchedulerHookFixture(t, gate)
			require.NoError(t, f.scheduler.Schedule(context.Background(), f.event))
			if tc.ready {
				require.Equal(t, []string{"ExptStart", "ScheduleStart", "ScanEvalItems", "ExptEnd"}, f.mode.calls)
				require.Len(t, gate.calls, 2)
			} else {
				require.Empty(t, f.mode.calls)
			}
			require.Zero(t, f.manager.completed)
			require.Len(t, f.publisher.published, tc.publications)
		})
	}
}

func TestSchedulerHookRechecksAfterDetailBeforeStart(t *testing.T) {
	for _, gateState := range []entity.HookGateState{entity.HookGateWaiting, entity.HookGateClosed} {
		t.Run(string(gateState), func(t *testing.T) {
			gate := &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateReady}}
			f := newSchedulerHookFixture(t, gate)
			gate.onCall = func(context.Context) {
				if len(gate.calls) == 2 {
					require.Equal(t, 1, f.manager.details)
					gate.decision.Gate = gateState
				}
			}
			require.NoError(t, f.scheduler.Schedule(context.Background(), f.event))
			require.Empty(t, f.mode.calls)
			require.Zero(t, f.manager.completed)
			require.Len(t, gate.calls, 2)
		})
	}
}

func TestSchedulerHookContinuationPreservesIdentityAndIsBounded(t *testing.T) {
	gate := &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateWaiting}}
	f := newSchedulerHookFixture(t, gate)
	f.event.CreatedAt = time.Now().Add(-time.Hour).Unix()
	originalTime := f.event.CreatedAt
	type scopeKey struct{}
	ctx := context.WithValue(context.Background(), scopeKey{}, "trusted-scope")
	checkContext := func(c context.Context) {
		deadline, ok := c.Deadline()
		require.True(t, ok)
		require.Positive(t, time.Until(deadline))
		require.LessOrEqual(t, time.Until(deadline), 5*time.Second)
		require.Equal(t, "trusted-scope", c.Value(scopeKey{}))
	}
	gate.onCall = checkContext
	f.publisher.onPublish = func(c context.Context, _ *entity.ExptScheduleEvent) { checkContext(c) }
	require.NoError(t, f.scheduler.Schedule(ctx, f.event))
	require.Len(t, f.publisher.published, 1)
	next := f.publisher.published[0]
	require.Equal(t, []time.Duration{5 * time.Second}, f.publisher.delays)
	require.Equal(t, f.event.ExptID, next.ExptID)
	require.Equal(t, f.event.ExptRunID, next.ExptRunID)
	require.Equal(t, f.event.SpaceID, next.SpaceID)
	require.Equal(t, f.event.Ext, next.Ext)
	require.Equal(t, f.event.Session, next.Session)
	require.Equal(t, f.event.ExecEvalSetItemIDs, next.ExecEvalSetItemIDs)
	require.Equal(t, originalTime, next.CreatedAt)
	require.Equal(t, originalTime, f.event.CreatedAt)
	next.Ext["scope"] = "mutated"
	next.Session.UserID = "mutated"
	next.ExecEvalSetItemIDs[0] = 99
	require.Equal(t, "ppe-hook", f.event.Ext["scope"])
	require.Equal(t, "scheduler-user", f.event.Session.UserID)
	require.Equal(t, int64(11), f.event.ExecEvalSetItemIDs[0])
}

func TestSchedulerHookPublishFailureNeverUsesLegacyTerminationBudget(t *testing.T) {
	f := newSchedulerHookFixture(t, &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateWaiting}})
	f.publisher.err = errors.New("private publish error")
	f.event.InfraErrorRetryTimes = 99
	for i := 0; i < 12; i++ {
		err := f.scheduler.Schedule(context.Background(), f.event)
		require.ErrorIs(t, err, schedulerHookRetryError{})
		require.NotContains(t, err.Error(), "private")
	}
	require.Len(t, f.publisher.published, 12)
	require.Equal(t, 99, f.event.InfraErrorRetryTimes)
	require.Zero(t, f.manager.completed)
	require.Empty(t, f.mode.calls)
}

func TestSchedulerHookWaitThenReadyResumesOriginalPath(t *testing.T) {
	gate := &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateWaiting}}
	f := newSchedulerHookFixture(t, gate)
	f.event.CreatedAt = time.Now().Add(-30 * time.Second).Unix()
	require.NoError(t, f.scheduler.Schedule(context.Background(), f.event))
	gate.decision.Gate = entity.HookGateReady
	require.NoError(t, f.scheduler.Schedule(context.Background(), f.publisher.published[0]))
	require.Equal(t, []string{"ExptStart", "ScheduleStart", "ScanEvalItems", "ExptEnd"}, f.mode.calls)
	require.Zero(t, f.manager.completed)
}

func TestSchedulerHookConstructorRejectsMissingDependencies(t *testing.T) {
	f := newSchedulerHookFixture(t, nil)
	gate := &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateReady}}
	var nilGate *schedulerHookGate
	for _, g := range []repo.IHookGateRepo{nil, nilGate} {
		_, err := NewHookAwareExptSchedulerSvc(f.base, g)
		require.Error(t, err)
	}
	var nilScheduler *ExptSchedulerImpl
	for _, base := range []ExptSchedulerEvent{nil, nilScheduler} {
		_, err := NewHookAwareExptSchedulerSvc(base, gate)
		require.Error(t, err)
	}
	base := *(f.base.(*ExptSchedulerImpl))
	base.Publisher = nil
	_, err := NewHookAwareExptSchedulerSvc(&base, gate)
	require.Error(t, err)
	var nilPublisher *schedulerHookPublisher
	base.Publisher = nilPublisher
	_, err = NewHookAwareExptSchedulerSvc(&base, gate)
	require.Error(t, err)
}

func TestSchedulerHookOldConstructorAndOriginalInstanceRemainLegacy(t *testing.T) {
	gate := &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateWaiting}}
	f := newSchedulerHookFixture(t, gate)
	require.NoError(t, f.base.Schedule(context.Background(), f.event))
	require.Empty(t, gate.calls)
	require.Equal(t, []string{"ExptStart", "ScheduleStart", "ScanEvalItems", "ExptEnd"}, f.mode.calls)
	require.Empty(t, f.publisher.published)
}

func TestSchedulerHookEnforceStillArchivesEndsAndTicksWithoutDispatch(t *testing.T) {
	f := newSchedulerHookFixture(t, &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateReady}})
	f.manager.expt.ExptDispatchMode = entity.ExptDispatchModeEnforce
	f.mode.toSubmit = []*entity.ExptEvalItem{{ItemID: 11, State: entity.ItemRunState_Queueing}}
	f.mode.complete = []*entity.ExptEvalItem{{ItemID: 12, State: entity.ItemRunState_Terminal}}
	f.mode.next = true
	require.NoError(t, f.scheduler.Schedule(context.Background(), f.event))
	require.Equal(t, []int64{12}, f.results.archived)
	require.Zero(t, f.publisher.itemDispatches)
	require.Zero(t, f.mode.endToSubmit)
	require.Equal(t, []string{"ExptStart", "ScheduleStart", "ScanEvalItems", "PublishResult", "ExptEnd", "NextTick"}, f.mode.calls)
}

func TestSchedulerHookFinishingAndClosedRunsNeverRestart(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Terminating, entity.ExptStatus_Draining, entity.ExptStatus_Success} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := newSchedulerHookFixture(t, &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateReady}})
			f.manager.run.Status = int64(status)
			require.NoError(t, f.scheduler.Schedule(context.Background(), f.event))
			require.Empty(t, f.mode.calls)
			require.Zero(t, f.manager.completed)
			require.Empty(t, f.publisher.published)
		})
	}
	f := newSchedulerHookFixture(t, &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateWaiting}})
	f.manager.run.Status = int64(entity.ExptStatus_Draining)
	require.NoError(t, f.scheduler.Schedule(context.Background(), f.event))
	require.Len(t, f.publisher.published, 1)
	require.Empty(t, f.mode.calls)
}

func TestSchedulerHookCanceledContextDoesNotPublishOrRun(t *testing.T) {
	gate := &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateReady}}
	f := newSchedulerHookFixture(t, gate)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, f.scheduler.Schedule(ctx, f.event), schedulerHookRetryError{})
	require.Empty(t, gate.calls)
	require.Empty(t, f.publisher.published)
	require.Empty(t, f.mode.calls)
	require.Zero(t, f.manager.completed)
}

func TestSchedulerHookReadyAfterFailedWaitPublishResumesSameRun(t *testing.T) {
	gate := &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateWaiting}}
	f := newSchedulerHookFixture(t, gate)
	f.event.CreatedAt = time.Now().Add(-30 * time.Second).Unix()
	f.publisher.err = errors.New("publish failed")
	require.ErrorIs(t, f.scheduler.Schedule(context.Background(), f.event), schedulerHookRetryError{})
	require.Zero(t, f.manager.completed)
	// MQ redelivers the original payload, potentially to a fresh process.
	gate.decision.Gate = entity.HookGateReady
	recovered := newSchedulerHookFixture(t, gate)
	recovered.event = f.event
	require.NoError(t, recovered.scheduler.Schedule(context.Background(), recovered.event))
	require.Zero(t, recovered.manager.completed, "a wait within the original deadline must remain recoverable")
	require.Equal(t, []string{"ExptStart", "ScheduleStart", "ScanEvalItems", "ExptEnd"}, recovered.mode.calls)
}

func TestSchedulerHookLegacyReadyKeepsOriginalZombieDeadline(t *testing.T) {
	f := newSchedulerHookFixture(t, &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateReady}})
	f.event.CreatedAt = time.Now().Add(-time.Hour).Unix()
	require.NoError(t, f.scheduler.Schedule(context.Background(), f.event))
	require.Equal(t, 2, f.manager.completed)
	require.Empty(t, f.mode.calls)
}

func TestSchedulerHookCancellationDuringGateOrPublishStaysRecoverable(t *testing.T) {
	for _, at := range []string{"gate", "publish"} {
		t.Run(at, func(t *testing.T) {
			gate := &schedulerHookGate{decision: entity.HookAdmissionDecision{Gate: entity.HookGateWaiting}}
			f := newSchedulerHookFixture(t, gate)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if at == "gate" {
				gate.onCall = func(context.Context) { cancel() }
			} else {
				f.publisher.onPublish = func(context.Context, *entity.ExptScheduleEvent) { cancel() }
			}
			require.ErrorIs(t, f.scheduler.Schedule(ctx, f.event), schedulerHookRetryError{})
			require.Zero(t, f.manager.completed)
			require.Empty(t, f.mode.calls)
			if at == "gate" {
				require.Empty(t, f.publisher.published)
			}
		})
	}
}
