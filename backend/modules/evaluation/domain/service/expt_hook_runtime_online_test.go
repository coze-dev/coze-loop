// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	evaluatorconvert "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/convertor"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type onlineStarterTestPort interface {
	StartOnlineWithHookSchedule(context.Context, int64, int64, int64, int, *entity.Session, map[string]string) (bool, error)
}

type onlineChainEvaluators struct {
	*executionChainEvaluators
	t     *testing.T
	calls int
}

func (s *onlineChainEvaluators) RunEvaluator(ctx context.Context, in *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
	require.Equal(s.t, s.f.key.RunID, in.ExperimentRunID)
	require.Equal(s.t, int64(111), in.EvaluatorVersionID)
	require.Empty(s.t, in.Alias, "Online uses its original global evaluator configuration")
	require.Equal(s.t, "original-env", gptr.Indirect(in.EvaluatorRunConf.Env))
	s.calls++
	r := &entity.EvaluatorRecord{ID: finalizationTestIDs.Add(1), SpaceID: in.SpaceID, ExperimentID: in.ExperimentID, ExperimentRunID: in.ExperimentRunID, ItemID: in.ItemID, TurnID: in.TurnID, EvaluatorVersionID: in.EvaluatorVersionID, SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess, EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.75)}}}
	return r, s.f.sql.WithContext(ctx).Create(evaluatorconvert.ConvertEvaluatorRecordDO2PO(r)).Error
}

func onlineRuntimeFixture(t *testing.T, before, after bool) (*finalizationManagerFixture, *HookRuntimeExecution, *executionChainPublisher, *executionChainTarget, *onlineChainEvaluators, *entity.ExptScheduleEvent) {
	t.Helper()
	f := foundationManagerFixture(t, before, after)
	ctx := context.Background()
	conf := &entity.EvaluationConfiguration{ItemConcurNum: gptr.Of(2), ConnectorConf: entity.Connector{TargetConf: &entity.TargetConf{TargetVersionID: 92, IngressConf: &entity.TargetIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}}, EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{EvaluatorVersionID: 111, IngressConf: &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}, RunConf: &entity.EvaluatorRunConfig{Env: gptr.Of("original-env")}}}}}}
	raw, err := json.Marshal(conf)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"expt_type": int32(entity.ExptType_Online), "eval_set_source_type": 1, "eval_set_id": 0, "eval_set_version_id": 0, "eval_conf": raw, "target_type": int32(entity.EvalTargetTypeLoopPrompt), "start_at": time.Now()}).Error)
	f.manager.configer = &scheduleConfiger{}
	f.manager.lwt = platestwrite.NewLatestWriteTracker(f.redis)
	f.manager.exptRepo = retryItemsAppReader{afterOnlyExperimentReader{sql: f.sql}}
	f.manager.mtr.(*metricmocks.MockExptMetric).EXPECT().EmitExptExecRun(gomock.Any(), gomock.Any()).AnyTimes()
	startPub := &scheduleEvents{ExptEventPublisher: f.manager.publisher}
	f.manager.publisher.(*eventmocks.MockExptEventPublisher).EXPECT().PublishExptOnlineEvalResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	f.manager.publisher = startPub
	starter, ok := any(f.manager).(onlineStarterTestPort)
	require.True(t, ok, "Online needs a durable original schedule entry")
	_, err = f.manager.mutex.UnlockForce(ctx, f.manager.makeExptMutexLockKey(f.expt))
	require.NoError(t, err)
	handled, err := starter.StartOnlineWithHookSchedule(ctx, f.expt, f.key.RunID, f.space, 0, &entity.Session{UserID: "original-user", AppID: 8}, map[string]string{"route": "original"})
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, startPub.sent, 1)
	event := startPub.sent[0]
	binding, err := LoadHookExecutionInitializationBinding(ctx, f.repo, f.manager.hooks.Codec, f.key, "local")
	require.NoError(t, err)
	stores, err := exptinfra.NewBoundHookRuntimeRepositories(f.p, binding)
	require.NoError(t, err)
	dataset := new(boundInitDataset)
	loader, err := NewHookFrozenPlanLoader(HookFrozenPlanLoaderDependencies{Plans: exptinfra.NewHookPlanRepo(f.p), Items: dataset, Versions: &loaderVersions{}, Sets: &loaderSets{value: &entity.EvaluationSet{ID: 71, SpaceID: f.space, EvaluationSetVersion: &entity.EvaluationSetVersion{EvaluationSetSchema: &entity.EvaluationSetSchema{ID: 73, SpaceID: f.space, EvaluationSetID: 71}}}}})
	require.NoError(t, err)
	loader = boundConsumerLoaderMutation{loader, func(p *entity.HookLoadedPlanPage) {
		for _, item := range p.Items {
			item.Item.BaseInfo = &entity.BaseInfo{}
		}
	}}
	deps, pub, target := executionChainDependencies(t, f, binding, stores, loader, entity.EvalTargetTypeLoopPrompt)
	deps.Consumer.(*ExptItemEventEvalServiceImpl).evaluationSetItemService = dataset
	evaluators := &onlineChainEvaluators{executionChainEvaluators: &executionChainEvaluators{f: f}, t: t}
	deps.Consumer.(*ExptItemEventEvalServiceImpl).evaluatorService = evaluators
	pub.ExptEventPublisher = f.manager.publisher
	runtime, err := NewBoundHookRuntimeExecution(deps)
	require.NoError(t, err)
	runtime.Manager.configer = &scheduleConfiger{}
	runtime.Manager.publisher = pub
	runtime.Scheduler.ResultSvc = hookPersistenceFilters{runtime.Scheduler.ResultSvc}
	runtime.Scheduler.schedulerModeFactory.(*DefaultSchedulerModeFactory).resultSvc = runtime.Scheduler.ResultSvc
	return f, runtime, pub, target, evaluators, event
}

func TestHookOnlineRunInvokeFinishMySQL(t *testing.T) {
	f, runtime, pub, target, evaluators, event := onlineRuntimeFixture(t, true, true)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	invoke := func(id int64) error {
		return runtime.Manager.Invoke(ctx, &entity.InvokeExptReq{ExptID: f.expt, RunID: f.key.RunID, SpaceID: f.space, Session: &entity.Session{UserID: "different-caller"}, Items: []*entity.EvaluationSetItem{{ItemID: id, SpaceID: f.space, EvaluationSetID: 71, Turns: []*entity.Turn{{ID: 0}}}}})
	}
	require.NoError(t, invoke(801))
	require.NoError(t, invoke(801), "repeated Invoke must not reset the accepted item")
	require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
	require.Empty(t, pub.items)
	require.Zero(t, target.calls)
	completeExecutionChainBefore(t, f)
	require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
	require.Len(t, pub.items, 1)
	require.NoError(t, runtime.Consumer.Eval(ctx, pub.items[0]))
	require.NoError(t, invoke(803))
	expt, err := runtime.Manager.Get(ctx, f.expt, f.space, event.Session)
	require.NoError(t, err)
	require.NoError(t, runtime.Manager.Finish(ctx, expt, f.key.RunID, &entity.Session{UserID: "finish-caller"}))
	run := finalizationRead(t, f)
	require.Equal(t, entity.ExptStatus_Draining, run.State.Status)
	require.False(t, run.State.After.Activated)
	require.Error(t, invoke(804))
	require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
	require.Len(t, pub.items, 2)
	require.NoError(t, runtime.Consumer.Eval(ctx, pub.items[1]))
	require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
	run = finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize)
	require.True(t, run.State.After.Activated)
	require.Zero(t, target.calls, "the existing Online path does not invoke a target node")
	require.Equal(t, 2, evaluators.calls)
	require.Equal(t, "original-user", run.CreatedBy)
	for _, sent := range pub.published {
		require.Equal(t, event.CreatedAt, sent.CreatedAt)
		require.Equal(t, event.Session, sent.Session)
	}
}

func TestHookOnlineDuplicateWithoutSourceMySQL(t *testing.T) {
	f, runtime, _, _, _, _ := onlineRuntimeFixture(t, true, true)
	ctx := context.Background()
	in := &entity.InvokeExptReq{SpaceID: f.space, ExptID: f.expt, RunID: f.key.RunID, Items: []*entity.EvaluationSetItem{{ItemID: 801, SpaceID: f.space, EvaluationSetID: 71, Turns: []*entity.Turn{{ID: 0}}}}}
	require.NoError(t, runtime.Manager.Invoke(ctx, in))
	runtime.Manager.onlineItems.(*boundInitDataset).fail = true
	require.NoError(t, runtime.Manager.Invoke(ctx, in), "duplicate acceptance must not depend on the mutable source or reset records")
}

func TestHookOnlineFinishBeforeReadyAndSingleStagesMySQL(t *testing.T) {
	for _, before := range []bool{true, false} {
		t.Run(map[bool]string{true: "before_only", false: "after_only"}[before], func(t *testing.T) {
			f, runtime, pub, _, evaluators, event := onlineRuntimeFixture(t, before, !before)
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			require.NoError(t, runtime.Manager.Invoke(ctx, &entity.InvokeExptReq{SpaceID: f.space, ExptID: f.expt, RunID: f.key.RunID, Items: []*entity.EvaluationSetItem{{ItemID: 801, SpaceID: f.space, EvaluationSetID: 71, Turns: []*entity.Turn{{ID: 0}}}}}))
			expt, err := runtime.Manager.Get(ctx, f.expt, f.space, event.Session)
			require.NoError(t, err)
			require.NoError(t, runtime.Manager.Finish(ctx, expt, f.key.RunID, event.Session))
			require.NoError(t, runtime.Manager.Finish(ctx, expt, f.key.RunID, event.Session))
			if before {
				require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
				require.Empty(t, pub.items)
				completeExecutionChainBefore(t, f)
			}
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			require.Len(t, pub.items, 1)
			require.NoError(t, runtime.Consumer.Eval(ctx, pub.items[0]))
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			run := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize)
			require.Equal(t, !before, run.State.After.Activated)
			require.Equal(t, 1, evaluators.calls)
		})
	}
}

func TestHookOnlineCancelWaitingActivatesAfterMySQL(t *testing.T) {
	f, runtime, _, _, evaluators, event := onlineRuntimeFixture(t, true, true)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	require.NoError(t, runtime.Manager.Invoke(ctx, &entity.InvokeExptReq{SpaceID: f.space, ExptID: f.expt, RunID: f.key.RunID, Items: []*entity.EvaluationSetItem{{ItemID: 801, SpaceID: f.space, EvaluationSetID: 71, Turns: []*entity.Turn{{ID: 0}}}}}))
	require.NoError(t, runtime.Manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "", event.Session))
	run := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize)
	require.True(t, run.State.After.Activated)
	require.Equal(t, entity.ExptStatus_Terminated, run.State.Status)
	require.Zero(t, evaluators.calls)
}

func TestHookOnlineCancelWaitingPreservesBatchMetadataMySQL(t *testing.T) {
	f, runtime, _, _, _, event := onlineRuntimeFixture(t, true, true)
	ctx := context.Background()
	metadata := map[string]string{"batch": "accepted"}
	require.NoError(t, runtime.Manager.Invoke(ctx, &entity.InvokeExptReq{SpaceID: f.space, ExptID: f.expt, RunID: f.key.RunID, Ext: metadata, Items: []*entity.EvaluationSetItem{{ItemID: 801, SpaceID: f.space, EvaluationSetID: 71}}}))
	require.NoError(t, runtime.Manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "", event.Session))
	run := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize)
	require.True(t, run.State.After.Activated)
	var item model.ExptItemResult
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 801).First(&item).Error)
	var got map[string]string
	require.NoError(t, json.Unmarshal(gptr.Indirect(item.Ext), &got))
	require.Equal(t, metadata, got)
	require.Error(t, runtime.Manager.Invoke(ctx, &entity.InvokeExptReq{SpaceID: f.space, ExptID: f.expt, RunID: f.key.RunID, Items: []*entity.EvaluationSetItem{{ItemID: 803, SpaceID: f.space, EvaluationSetID: 71}}}), "terminal Run must reject new acceptance")
}

func TestHookOnlineTTLStopsAcceptanceWhileWaitingMySQL(t *testing.T) {
	f, runtime, _, _, _, event := onlineRuntimeFixture(t, true, true)
	ctx := context.Background()
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"start_at": time.Now().Add(-time.Minute), "max_alive_time": int64(1000)}).Error)
	require.NoError(t, runtime.Manager.PrepareOnlinePlan(ctx, f.key))
	run := finalizationRead(t, f)
	require.Equal(t, entity.ExptStatus_Draining, run.State.Status)
	require.Equal(t, entity.HookGateWaiting, run.State.Gate)
	require.False(t, run.State.After.Activated)
	require.NoError(t, runtime.Manager.PublishOnlineContinuation(ctx, f.key))
	_, original, err := runtime.Manager.readOnlineRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, event.CreatedAt, original.CreatedAt)
}

func TestHookOnlineWrongRunCannotFallBackMySQL(t *testing.T) {
	f, runtime, _, _, _, _ := onlineRuntimeFixture(t, true, true)
	ctx := context.Background()
	wrong := f.key
	wrong.RunID += 100000
	_, err := runtime.Manager.CheckOnlineRun(ctx, wrong)
	require.Error(t, err, "a missing requested Run cannot become a legacy Invoke")
	require.Error(t, runtime.Manager.Invoke(ctx, &entity.InvokeExptReq{SpaceID: f.space, ExptID: f.expt, RunID: wrong.RunID, Items: []*entity.EvaluationSetItem{{ItemID: 801, SpaceID: f.space, EvaluationSetID: 71}}}))
	expt, err := runtime.Manager.Get(ctx, f.expt, f.space, nil)
	require.NoError(t, err)
	require.Error(t, runtime.Manager.Finish(ctx, expt, wrong.RunID, nil))
}

func TestHookOnlineDeleteBindingPreparationMySQL(t *testing.T) {
	f, runtime, _, _, _, _ := onlineRuntimeFixture(t, true, true)
	ctx := context.Background()
	deletion, err := exptinfra.PrepareHookDeletion(ctx, f.p, f.repo, f.manager.hooks.Codec, []int64{f.expt}, f.space, "local")
	require.NoError(t, err, "Online cancellation must remain available through the existing deletion binding path")
	_, err = deletion.DeleteExperiments(ctx, []int64{f.expt}, f.space, "local")
	require.NoError(t, err)
	require.NoError(t, runtime.Manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}))
	run := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize)
	require.True(t, run.State.After.Activated)
}

func TestHookOnlineWaitingDrainingScanMySQL(t *testing.T) {
	f, runtime, _, _, _, event := onlineRuntimeFixture(t, true, true)
	ctx := context.Background()
	expt, err := runtime.Manager.Get(ctx, f.expt, f.space, event.Session)
	require.NoError(t, err)
	require.NoError(t, runtime.Manager.Finish(ctx, expt, f.key.RunID, event.Session))
	now := time.Now().UTC().Truncate(time.Millisecond)
	input := entity.HookScanInput{ExecutionScope: "local", Now: now, Limit: 100, Cursor: &entity.HookScanCursor{Kind: entity.HookScanPreparing, ExecutionScope: "local", Status: "preparing", Now: now, WorkspaceID: f.space, RunID: f.key.RunID - 1}}
	page, err := exptinfra.NewHookScheduleScanRepo(f.p).ScanPreparingPlans(ctx, input)
	require.NoError(t, err)
	found := false
	for _, candidate := range page.Candidates {
		if candidate.Key == f.key {
			found = true
		}
	}
	require.True(t, found, "waiting Draining Run must remain recoverable before before completes")
}

type onlineGrowingInitialization struct {
	repo.IHookExecutionInitializationRepo
	grow func()
}

func (r *onlineGrowingInitialization) ReadExecutionInitializationPage(ctx context.Context, in entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error) {
	page, err := r.IHookExecutionInitializationRepo.ReadExecutionInitializationPage(ctx, in)
	if err == nil && r.grow != nil {
		grow := r.grow
		r.grow = nil
		grow()
	}
	return page, err
}

func TestHookOnlineConcurrentInitialGrowthRecoversMySQL(t *testing.T) {
	f, runtime, _, _, _, _ := onlineRuntimeFixture(t, true, true)
	ctx := context.Background()
	var items []*entity.EvaluationSetItem
	for i := int64(0); i < 101; i++ {
		items = append(items, &entity.EvaluationSetItem{ItemID: 20000 + i, SpaceID: f.space, EvaluationSetID: 71})
	}
	require.NoError(t, runtime.Manager.Invoke(ctx, &entity.InvokeExptReq{SpaceID: f.space, ExptID: f.expt, RunID: f.key.RunID, Items: items}))
	completeExecutionChainBefore(t, f)
	init := *runtime.Scheduler.hookBoundInitializer
	init.deps.Repository = &onlineGrowingInitialization{IHookExecutionInitializationRepo: init.deps.Repository, grow: func() {
		require.NoError(t, runtime.Manager.Invoke(ctx, &entity.InvokeExptReq{SpaceID: f.space, ExptID: f.expt, RunID: f.key.RunID, Items: []*entity.EvaluationSetItem{{ItemID: 30000, SpaceID: f.space, EvaluationSetID: 71}}}))
	}}
	_, err := init.InitializeExecution(ctx, f.key, "local")
	require.ErrorIs(t, err, entity.ErrHookStoreConflict, "a newly accepted tail during initial paging is retryable, not corruption")
	require.False(t, permanentHookInitializationError(err))
	done, err := init.InitializeExecution(ctx, f.key, "local")
	require.NoError(t, err)
	require.True(t, done.Initialized)
	require.Equal(t, int64(102), finalizationRead(t, f).PlanCount)
}
