// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type reviewProtocolGuard struct {
	component.ICentralReservationGuard
	prepareErr      error
	prepareCalls    int
	terminalErr     error
	terminalItems   []*entity.ExptItemResultRunLog
	historicalItems []*entity.ExptItemResultRunLog
	historicalErr   error
	legacyReleases  int
	generations     map[string][]string
}

func (g *reviewProtocolGuard) OnHistoricalItemTerminal(_ context.Context, item *entity.ExptItemResultRunLog) error {
	copy := *item
	g.historicalItems = append(g.historicalItems, &copy)
	return g.historicalErr
}

func (g *reviewProtocolGuard) recordGeneration(ctx context.Context, hook string) {
	if g.generations == nil {
		g.generations = make(map[string][]string)
	}
	value, known := component.EvaluationResourceRequestID(ctx)
	g.generations[hook] = append(g.generations[hook], fmt.Sprintf("%t:%s", known, value))
}
func (g *reviewProtocolGuard) ConfirmItemRunning(ctx context.Context, _ *entity.Experiment, _ *entity.ExptItemResultRunLog) (bool, error) {
	g.recordGeneration(ctx, "confirm")
	return true, nil
}
func (g *reviewProtocolGuard) PrepareItemResources(ctx context.Context, _ *entity.ExptItemEvalCtx) (context.Context, error) {
	g.recordGeneration(ctx, "prepare")
	g.prepareCalls++
	return ctx, g.prepareErr
}
func (g *reviewProtocolGuard) OnItemTerminal(ctx context.Context, item *entity.ExptItemResultRunLog) error {
	g.recordGeneration(ctx, "terminal")
	copy := *item
	g.terminalItems = append(g.terminalItems, &copy)
	return g.terminalErr
}
func (g *reviewProtocolGuard) Release(context.Context, string, int64, int64, string, string) error {
	g.legacyReleases++
	return nil
}
func (g *reviewProtocolGuard) ReleaseTerminalReservation(context.Context, string, int64, int64, string) (component.TerminalReservationReleaseResult, error) {
	g.legacyReleases++
	return component.TerminalReservationReleased, nil
}

type reviewProtocolTargets struct {
	IEvalTargetService
	record      *entity.EvalTargetRecord
	starts      int
	stops       int
	stopOptions []bool
	stopErr     error
}

func (s *reviewProtocolTargets) GetRecordByID(context.Context, int64, int64) (*entity.EvalTargetRecord, error) {
	return s.record, nil
}
func (s *reviewProtocolTargets) AsyncExecuteTarget(_ context.Context, space, target, version int64, param *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, string, error) {
	s.starts++
	return &entity.EvalTargetRecord{ID: int64(900 + s.starts), SpaceID: space, TargetID: target, TargetVersionID: version,
		ExperimentRunID: gptr.Indirect(param.ExperimentRunID), ItemID: param.ItemID, TurnID: param.TurnID,
		Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking), Ext: input.GetExt()}, "sandbox", nil
}
func (s *reviewProtocolTargets) CheckSandboxTerminated(_ context.Context, _ int64, ids []int64) ([]int64, map[int64]string) {
	states := make(map[int64]string, len(ids))
	for _, id := range ids {
		states[id] = "Terminated"
	}
	return ids, states
}
func (s *reviewProtocolTargets) TerminateAsyncRecordsAndDestroySandbox(_ context.Context, _ int64, _ []int64, _ int32, _ string, _ bool, options ...bool) error {
	s.stops++
	s.stopOptions = options
	return s.stopErr
}
func (s *reviewProtocolTargets) TerminateSandboxAttemptsForRunItems(_ context.Context, _, _ int64, _ []int64, options ...bool) error {
	s.stops++
	s.stopOptions = options
	return s.stopErr
}

func reviewProtocolFixture(t *testing.T) (*lateWriteFixture, *ExptItemEventEvalServiceImpl, *reviewProtocolGuard, *reviewProtocolTargets, *resourceBroker, *entity.ExptItemEvalEvent, context.Context) {
	t.Helper()
	f := newLateWriteFixture(t, "rq1")
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).
		Updates(map[string]any{"scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("retry_times", 0).Error)
	guard, broker := &reviewProtocolGuard{}, &resourceBroker{}
	target := &reviewProtocolTargets{record: &entity.EvalTargetRecord{ID: f.base + 41, SpaceID: f.space,
		ExperimentRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: 0,
		Status: gptr.Of(entity.EvalTargetRunStatusFail), Ext: map[string]string{centralItemRetryRecordKey: "0"}}}
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard, exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo(),
		experimentRepo: f.experimentRepo(), dispatchRepo: exptrepo.NewExptItemDispatchRepo(exptmysql.NewExptItemDispatchDAO(f.provider)),
		itemLocker: lateWriteLocker(t, gomock.NewController(t)), configer: resourceConfig{retry: 1}, publisher: broker, evaTargetService: target}
	event := f.event()
	event.RetryTimes, event.MaxRetryTimes = 0, 1
	event.Ext = map[string]string{entity.RetryYieldExtKey: "true"}
	ctx := ctxcache.Init(context.Background())
	expt, err := svc.experimentRepo.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	event.WithCtxCentralAdmittedExpt(ctx, expt)
	return f, svc, guard, target, broker, event, ctx
}

func TestResourceReviewR1CallbackRedeliveryReplaysAuthorizedRetry(t *testing.T) {
	f, svc, guard, _, broker, event, ctx := reviewProtocolFixture(t)
	event = event.CloneWithResumeSource("target", f.base+41, 0)
	event.AsyncReportTrigger = true
	executionFailure := errors.New("failed original target callback")
	broker.err = errors.New("broker unavailable after authorization")
	calls := 0
	svc.endpoints = svc.HandleEventErr(svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error {
		calls++
		return executionFailure
	}))
	require.ErrorIs(t, svc.Eval(ctx, event), broker.err)
	require.Equal(t, int32(1), f.source().Current.RetryTimes)
	require.Empty(t, guard.terminalItems)
	broker.err, broker.event = nil, nil
	require.NoError(t, svc.Eval(ctx, event))
	require.Equal(t, 1, calls, "redelivery must replay authorization, not process the same failed callback again")
	require.Equal(t, int32(1), f.source().Current.RetryTimes)
	require.Equal(t, entity.ItemRunState_Processing, f.source().Current.Status)
	require.NotNil(t, broker.event)
	require.Equal(t, 1, broker.event.RetryTimes)
	require.Equal(t, event.RequestID, broker.event.RequestID)
	require.Nil(t, broker.event.ResumeSource, "an authorized retry is a new invocation, not the old callback")
	require.Equal(t, event.ExptRunMode, broker.event.ExptRunMode)
	require.Empty(t, guard.terminalItems)
}

func TestResourceReviewC1EvaluatorReplayCanAuthorizeNextFailure(t *testing.T) {
	f, svc, _, _, broker, event, ctx := reviewProtocolFixture(t)
	records := svcmocks.NewMockEvaluatorRecordService(gomock.NewController(t))
	records.EXPECT().GetEvaluatorRecord(gomock.Any(), f.base+61, false).Return(&entity.EvaluatorRecord{
		ID: f.base + 61, SpaceID: f.space, ExperimentID: f.expt, ExperimentRunID: f.run,
		ItemID: f.item, ItemVersionID: f.version, TurnID: 0, EvaluatorVersionID: f.base + 51,
		Alias: "alias", Status: entity.EvaluatorRunStatusFail}, nil).AnyTimes()
	svc.evaluatorRecordService = records
	svc.configer, event.MaxRetryTimes = resourceConfig{retry: 3}, 3
	event = event.CloneWithResumeSource("evaluator", f.base+61, 0)
	event.AsyncEvaluatorReportTrigger = true
	calls := 0
	svc.endpoints = svc.HandleEventErr(svc.HandleCentralReservation(func(ctx context.Context, current *entity.ExptItemEvalEvent) error {
		calls++
		if current.RetryTimes == 1 {
			source := executionSource(ctx).Source
			next := *source.Turns[0]
			next.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{
				{VersionID: f.base + 51, Alias: "alias", RecordID: f.base + 71}}}
			applied, err := f.port.StoreCurrentTurnRunResult(ctx, source, &next)
			require.NoError(t, err)
			require.True(t, applied)
			source.Turns[0] = &next
		}
		return errors.New("async evaluator kickoff failed")
	}))
	broker.err = errors.New("retry publication unavailable")
	require.ErrorIs(t, svc.Eval(ctx, event), broker.err)
	broker.err = nil
	require.NoError(t, svc.Eval(ctx, event))
	replay := broker.event
	require.NotNil(t, replay)
	require.Nil(t, replay.ResumeSource)
	require.NoError(t, svc.Eval(ctx, replay))
	require.Equal(t, 2, calls)
	require.Equal(t, int32(2), f.source().Current.RetryTimes)
	require.Equal(t, 2, broker.event.RetryTimes, "new failed evaluator reference must not suppress the next authorization")
	require.Equal(t, event.RequestID, broker.event.RequestID)
	require.Equal(t, event.ExptRunMode, broker.event.ExptRunMode)
	require.NoError(t, svc.Eval(ctx, event), "the old callback remains fenced after a new evaluator reference")
	require.Equal(t, 2, calls)
}

func TestResourceReviewR2ValidatedCallbackStartsNeverStartedLaterTurn(t *testing.T) {
	f, svc, _, target, _, event, ctx := reviewProtocolFixture(t)
	later := f.base + 22
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log"} {
		require.NoError(t, f.conn.Table(table).Where("space_id=? AND item_id=?", f.space, f.item).
			Update("status", entity.ItemRunState_Queueing).Error)
	}
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=?", f.space).
		Update("quota_reservation_state", entity.QuotaReservationStateReserved).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=?", f.space).
		Update("status", entity.TurnRunState_Queueing).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).
		Updates(map[string]any{"status": entity.TurnRunState_Queueing, "target_result_id": 0, "evaluator_result_ids": nil}).Error)
	target.record.Status = gptr.Of(entity.EvalTargetRunStatusSuccess)
	expt := &entity.Experiment{ID: f.expt, SpaceID: f.space, TargetID: 10, TargetVersionID: 20,
		TargetType: entity.EvalTargetTypeSandboxAgent, ExptDispatchMode: entity.ExptDispatchModeEnforce,
		Target: &entity.EvalTarget{ID: 10, EvalTargetType: entity.EvalTargetTypeSandboxAgent,
			EvalTargetVersion: &entity.EvalTargetVersion{ID: 20, EvalTargetType: entity.EvalTargetTypeSandboxAgent}},
		EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{TargetConf: &entity.TargetConf{
			TargetVersionID: 20, IngressConf: &entity.TargetIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}}}}}
	turnService := &DefaultExptTurnEvaluationImpl{metric: supplementMetric{}, evalTargetService: target,
		benefitService: supplementBenefit{}, evalAsyncRepo: supplementAsyncStore{}}
	svc.endpoints = svc.HandleEventErr(svc.HandleCentralReservation(func(ctx context.Context, admitted *entity.ExptItemEvalEvent) error {
		require.True(t, executionSource(ctx).SourceValidated)
		source := executionSource(ctx).Source
		if admitted.ResumeSource == nil {
			first := *source.Turns[0]
			first.Status, first.TargetResultID = entity.TurnRunState_Processing, f.base+41
			applied, err := f.port.StoreCurrentTurnRunResult(ctx, source, &first)
			require.NoError(t, err)
			require.True(t, applied)
			source.Turns[0] = &first
			return nil
		}
		if source.Turns[0].Status != entity.TurnRunState_Success {
			completed := *source.Turns[0]
			completed.Status = entity.TurnRunState_Success
			applied, err := f.port.StoreCurrentTurnRunResult(ctx, source, &completed)
			require.NoError(t, err)
			require.True(t, applied)
			source.Turns[0] = &completed
		}
		etec := &entity.ExptTurnEvalCtx{ExptItemEvalCtx: &entity.ExptItemEvalCtx{Event: admitted.CloneForTurn(later), Expt: expt,
			EvalSetItem: &entity.EvaluationSetItem{ID: f.item}}, Turn: &entity.Turn{ID: later}, ExptTurnRunResult: &entity.ExptTurnRunResult{}}
		executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo()}
		ctx, err := executor.prepareResourceTurnStart(ctx, etec)
		if err != nil {
			return err
		}
		_, err = turnService.CallTarget(ctx, etec)
		return err
	}))
	require.NoError(t, svc.Eval(ctx, event))
	require.Equal(t, entity.TurnRunState_Processing, f.source().Turns[later].Status, "real dispatch advances every pending turn")
	event = event.CloneWithResumeSource("target", f.base+41, 0)
	event.AsyncReportTrigger = true
	require.NoError(t, svc.Eval(ctx, event))
	require.Equal(t, 1, target.starts)
	require.ErrorIs(t, svc.Eval(ctx, event), errCentralItemAdmission, "an unrecorded later attempt must not be blindly restarted")
	require.Equal(t, 1, target.starts)
}

func TestResourceReviewC3HistoricalTerminalCleanupAfterMigration(t *testing.T) {
	f, _, guard, target, _, _, _ := reviewProtocolFixture(t)
	manager := &ExptMangerImpl{itemResultRepo: f.itemRepo(), turnResultRepo: f.turnRepo(),
		centralGuard: guard, evalTargetService: target, mutex: lateWriteLocker(t, gomock.NewController(t))()}
	expt := &entity.Experiment{ID: f.expt, SpaceID: f.space, ExptDispatchMode: entity.ExptDispatchModeEnforce}
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=?", f.space).
		Update("status", entity.ItemRunState_Terminal).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=?", f.space).
		Update("expt_run_id", f.run+1).Error)
	require.NoError(t, manager.cancelCentralItems(context.Background(), expt, nil, nil))
	require.Positive(t, target.stops)
	require.Empty(t, guard.generations["terminal"], "never obtain historical RequestID from replacement")
	require.Len(t, guard.historicalItems, 3, "fixture retains three immutable Terminal run logs")
	for _, item := range guard.historicalItems {
		require.Less(t, item.ExptRunID, f.run+1)
	}
	guard.historicalErr = errors.New("historical binding completion unavailable")
	require.ErrorIs(t, manager.cancelCentralItems(context.Background(), expt, nil, nil), guard.historicalErr)
	target.stopErr = errors.New("historical physical stop unavailable")
	require.ErrorIs(t, manager.cancelCentralItems(context.Background(), expt, nil, nil), target.stopErr)
	require.Len(t, guard.historicalItems, 4, "unknown stop must not reach binding finalization")
}

func TestResourceReviewC2CompetingClaimsAndFreshGeneration(t *testing.T) {
	f, svc, _, _, _, event, _ := reviewProtocolFixture(t)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).
		Updates(map[string]any{"status": entity.TurnRunState_Queueing, "target_result_id": 0, "evaluator_result_ids": nil}).Error)
	item, err := f.itemRepo().GetItemRunLog(context.Background(), f.expt, f.run, f.item, f.space)
	require.NoError(t, err)
	newContext := func(source *entity.ExptCurrentItemRunSource) context.Context {
		ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{
			Source: source, ItemRunLogID: item.ID, SourceValidated: true})
		ctx = context.WithValue(ctx, centralResourceExecutionKey{}, true)
		return context.WithValue(ctx, centralResourceFirstStartKey{}, true)
	}
	require.NoError(t, svc.prepareResourceTurnStarts(newContext(f.source())))
	// Both contenders capture the same persisted pending claim, not one another's update.
	sources := []*entity.ExptCurrentItemRunSource{f.source(), f.source()}
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, source := range sources {
		workers.Add(1)
		go func(source *entity.ExptCurrentItemRunSource) {
			defer workers.Done()
			<-start
			executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo()}
			_, err := executor.prepareResourceTurnStart(newContext(source), &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{Event: event}, Turn: &entity.Turn{ID: 0},
				ExptTurnRunResult: &entity.ExptTurnRunResult{}})
			results <- err
		}(source)
	}
	close(start)
	workers.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else {
			require.ErrorIs(t, err, errExptNonCurrentSource)
		}
	}
	require.Equal(t, 1, winners, "canonical-first transaction must compare the persisted claim")
	old := f.source().Turns[0].Ext[entity.ResourceTurnStartExtKey]
	f.token("rq2")
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).
		Update("status", entity.TurnRunState_Queueing).Error)
	ctx := newContext(f.source())
	require.NoError(t, svc.prepareResourceTurnStarts(ctx))
	require.NotEqual(t, old, f.source().Turns[0].Ext[entity.ResourceTurnStartExtKey])
	event = event.Clone()
	event.RequestID = "rq2"
	executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo()}
	_, err = executor.prepareResourceTurnStart(ctx, &entity.ExptTurnEvalCtx{
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{Event: event}, Turn: &entity.Turn{ID: 0},
		ExptTurnRunResult: &entity.ExptTurnRunResult{}})
	require.NoError(t, err, "copied old Ext cannot block a new persisted generation")
}

type reviewCurrentOnlyGuard struct {
	component.ICentralReservationGuard
	component.ICentralItemExecution
	component.ICentralItemLifecycle
}

func TestResourceReviewC3HistoricalDeleteAndMDelete(t *testing.T) {
	for _, entrance := range []string{"Delete", "MDelete"} {
		t.Run(entrance, func(t *testing.T) {
			f, _, guard, target, _, _, _ := reviewProtocolFixture(t)
			require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=?", f.space).
				Update("status", entity.ItemRunState_Terminal).Error)
			require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=?", f.space).
				Update("expt_run_id", f.run+1).Error)
			manager := &ExptMangerImpl{exptRepo: f.experimentRepo(), itemResultRepo: f.itemRepo(), turnResultRepo: f.turnRepo(),
				evalTargetService: target, mutex: lateWriteLocker(t, gomock.NewController(t))()}
			remove := func() error {
				if entrance == "Delete" {
					return manager.Delete(context.Background(), f.expt, f.space, &entity.Session{})
				}
				return manager.MDelete(context.Background(), []int64{f.expt}, f.space, &entity.Session{})
			}
			manager.centralGuard = reviewCurrentOnlyGuard{guard, guard, guard}
			require.ErrorContains(t, remove(), "historical item lifecycle not wired")
			require.Zero(t, target.stops, "missing immutable binding capability must fail closed")
			manager.centralGuard = guard
			guard.historicalErr = errors.New("persistent historical binding not finalized")
			require.ErrorIs(t, remove(), guard.historicalErr)
			var deleted int64
			require.NoError(t, f.conn.Table("experiment").Where("space_id=? AND deleted_at IS NOT NULL", f.space).Count(&deleted).Error)
			require.Zero(t, deleted)
			guard.historicalErr = nil
			require.NoError(t, remove())
			require.NoError(t, f.conn.Table("experiment").Where("space_id=? AND deleted_at IS NOT NULL", f.space).Count(&deleted).Error)
			require.Equal(t, int64(1), deleted)
			require.Zero(t, guard.legacyReleases)
			require.Empty(t, guard.generations["terminal"], "historical finalization never adopts replacement generation")
		})
	}
}

func TestResourceReviewR3CurrentSweepRetainsBudgetUntilSiblingStops(t *testing.T) {
	f, _, guard, target, _, _, _ := reviewProtocolFixture(t)
	target.stopErr = errors.New("sibling sandbox still running")
	source := f.source()
	item := &entity.ExptEvalItem{ItemID: f.item, ExptID: f.expt, State: entity.ItemRunState_Processing,
		Source: source, RetryTimes: source.Current.RetryTimes, UpdatedAt: source.Current.UpdatedAt}
	svc := &ExptSchedulerImpl{ExptItemResultRepo: f.itemRepo(), centralGuard: guard, evalTargetService: target,
		itemLocker: lateWriteLocker(t, gomock.NewController(t))}
	before := f.snapshot()
	_, _, err := svc.sweepCurrentSandboxItems(context.Background(),
		&entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}, []*entity.ExptEvalItem{item},
		&entity.Experiment{ID: f.expt, SpaceID: f.space, ExptDispatchMode: entity.ExptDispatchModeEnforce})
	require.ErrorIs(t, err, target.stopErr)
	require.Equal(t, []bool{true, true}, target.stopOptions)
	require.Equal(t, before, f.snapshot())
	require.Empty(t, guard.terminalItems)
}

type reviewProtocolSetItems struct {
	EvaluationSetItemService
	item *entity.EvaluationSetItem
}

func (s reviewProtocolSetItems) BatchGetEvaluationSetItems(context.Context, *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	return []*entity.EvaluationSetItem{s.item}, nil
}

func TestResourceReviewR4PreparationDependencyDoesNotConsumeExecutionBudget(t *testing.T) {
	f, svc, guard, _, _, event, ctx := reviewProtocolFixture(t)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).
		Updates(map[string]any{"target_result_id": 0, "evaluator_result_ids": nil}).Error)
	event.MaxRetryTimes = 0
	svc.configer = resourceConfig{}
	svc.manager = resourceManager{expt: &entity.Experiment{ID: f.expt, SpaceID: f.space,
		EvalSet: &entity.EvaluationSet{ID: 7, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 8, EvaluationSetID: 7}}}}
	svc.evaluationSetItemService = reviewProtocolSetItems{item: &entity.EvaluationSetItem{ID: f.item,
		Turns: []*entity.Turn{{ID: 0}, {ID: f.base + 22}}}}
	guard.prepareErr = errors.New("grant dependency unavailable")
	svc.endpoints = svc.HandleEventErr(svc.HandleCurrentSource(svc.HandleEventExec(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })))
	before := f.snapshot()
	err := svc.Eval(ctx, event)
	require.Equal(t, 1, guard.prepareCalls)
	require.ErrorIs(t, err, guard.prepareErr)
	require.True(t, errno.NeedMQRetry(err))
	require.Equal(t, before, f.snapshot())
	require.Empty(t, guard.terminalItems)
}

func TestResourceReviewR5StaleDeliveryCannotFinalizeReplacement(t *testing.T) {
	f, svc, guard, _, _, event, ctx := reviewProtocolFixture(t)
	f.token("replacement")
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log"} {
		require.NoError(t, f.conn.Table(table).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", entity.ItemRunState_Success).Error)
	}
	svc.endpoints = svc.HandleCurrentSource(func(context.Context, *entity.ExptItemEvalEvent) error {
		return errors.New("stale delivery must not execute")
	})
	before := f.snapshot()
	require.NoError(t, svc.Eval(ctx, event))
	require.Empty(t, guard.terminalItems, "request A cannot emit request B's terminal lifecycle")
	require.Equal(t, before, f.snapshot())
}

func TestResourceReviewR6TakeoverFinalizesOldSourceBeforeMigration(t *testing.T) {
	for _, entrance := range []string{"before_latest_run", "canonical_migration"} {
		t.Run(entrance, func(t *testing.T) {
			p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
			f, manager := p.f, p.manager
			guard, target := &reviewProtocolGuard{}, &reviewProtocolTargets{}
			manager.centralGuard, manager.evalTargetService = guard, target
			require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).
				Updates(map[string]any{"status": entity.ExptStatus_Success, "scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)
			for _, table := range []string{"expt_item_result", "expt_item_result_run_log"} {
				require.NoError(t, f.conn.Table(table).Where("space_id=?", f.space).Update("status", entity.ItemRunState_Success).Error)
			}
			next := f.run + 1000
			if entrance == "before_latest_run" {
				require.NoError(t, manager.releaseCurrentSupersededRunQuota(context.Background(), f.expt, f.space, next, entity.EvaluationModeRetryAll))
			} else {
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("latest_run_id", next).Error)
				writes := 0
				require.NoError(t, manager.withNewRunItemLocks(context.Background(), f.space, f.expt, next, []int64{f.item}, true,
					func(context.Context, int64) error {
						require.NotEmpty(t, guard.terminalItems, "old Task association must remain readable until source-owned finalization")
						writes++
						return nil
					}))
				require.Equal(t, 1, writes)
			}
			require.Zero(t, guard.legacyReleases)
			require.NotEmpty(t, guard.terminalItems)
			require.Positive(t, target.stops)
			require.Equal(t, []bool{true, true}, target.stopOptions)
			for _, item := range guard.terminalItems {
				require.Equal(t, f.run, item.ExptRunID)
				require.Equal(t, f.item, item.ItemID)
			}
			t.Log(fmt.Sprintf("REAL_MYSQL_RESOURCE_TAKEOVER %s source-owned stop/finalization before migration", entrance))
		})
	}
}

func TestResourceReviewR7ValidatedGenerationReachesAllPorts(t *testing.T) {
	for _, requestID := range []string{"", "rq1"} {
		t.Run(requestID, func(t *testing.T) {
			f, svc, guard, _, _, event, ctx := reviewProtocolFixture(t)
			if requestID == "" {
				f.token("")
				event.RequestID, event.ExptRunMode = "", entity.EvaluationModeSubmit
			}
			ctx = component.WithEvaluationResourceRequestID(ctx, "untrusted-parent")
			svc.endpoints = svc.HandleEventErr(svc.HandleCentralReservation(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
				preparedCtx, err := guard.PrepareItemResources(ctx, &entity.ExptItemEvalCtx{Event: event})
				require.NoError(t, err)
				execution := executionSource(ctx)
				applied, err := f.port.CompleteCurrentItemRun(preparedCtx, execution.Source,
					&entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Success, RetryTimes: execution.Source.Current.RetryTimes})
				require.NoError(t, err)
				require.True(t, applied)
				execution.Completed = true
				return nil
			}))
			require.NoError(t, svc.Eval(ctx, event))
			for _, hook := range []string{"confirm", "prepare", "terminal"} {
				require.Equal(t, []string{"true:" + requestID}, guard.generations[hook], hook)
			}
		})
	}
}

func TestResourceReviewR7ManualRetryFinalizesBeforeReplacingSource(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f, manager := p.f, p.manager
	ctx := context.Background()
	source := f.source()
	turn := *source.Turns[0]
	turn.Status = entity.TurnRunState_Success
	applied, err := f.port.StoreCurrentTurnRunResult(ctx, source, &turn)
	require.NoError(t, err)
	require.True(t, applied)
	source.Turns[0] = &turn
	applied, err = f.port.CompleteCurrentItemRun(ctx, source, &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Success})
	require.NoError(t, err)
	require.True(t, applied)
	publicationSource, err := p.result.GetItemResultPublishSource(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	prepared, err := p.result.PrepareItemResultPublication(ctx, publicationSource, p.expt)
	require.NoError(t, err)
	applied, err = p.result.PublishPreparedItemResult(ctx, prepared, nil)
	require.NoError(t, err)
	require.True(t, applied)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).
		Updates(map[string]any{"scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)
	guard, target := &reviewProtocolGuard{terminalErr: errors.New("old generation not finalized")}, &reviewProtocolTargets{}
	manager.centralGuard, manager.evalTargetService = guard, target
	before := f.snapshot()
	oldLog, err := f.itemRepo().GetItemRunLog(ctx, f.expt, f.run, f.item, f.space)
	require.NoError(t, err)
	oldID := oldLog.ID
	request := &entity.RetryInCurrentRunRequest{SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run,
		Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{f.item}, Session: &entity.Session{UserID: "test"}}
	_, err = manager.RetryInCurrentRun(ctx, request)
	require.ErrorIs(t, err, guard.terminalErr)
	require.Equal(t, before, f.snapshot(), "failed old-generation finalization must leave the original source intact")
	require.Zero(t, guard.legacyReleases)
	guard.terminalErr = nil
	receipt, err := manager.RetryInCurrentRun(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, receipt)
	current := f.source()
	require.Equal(t, f.run, current.Current.RunID)
	require.NotEmpty(t, current.Current.RequestID)
	log, err := f.itemRepo().GetItemRunLog(ctx, f.expt, f.run, f.item, f.space)
	require.NoError(t, err)
	require.Equal(t, oldID, log.ID, "generation is the persisted UUID, not a fabricated replacement log ID")
	require.Zero(t, guard.legacyReleases)
	require.Equal(t, []string{"true:", "true:"}, guard.generations["terminal"], "both attempts must finalize the exact old empty generation")
}

var _ component.ICentralItemLifecycle = (*reviewProtocolGuard)(nil)
