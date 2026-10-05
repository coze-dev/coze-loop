// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	ec "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/convertor"
	em "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	tm "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

type retryExecutionMetric struct{ metrics.ExptMetric }

func (retryExecutionMetric) EmitExptExecRun(int64, int64) {}

func TestHookRetrySingleSetCapture(t *testing.T) {
	expt := &entity.Experiment{ID: 2, SpaceID: 1, ExptType: entity.ExptType_Offline, EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet, EvalSetID: 71, EvalSetVersionID: 72, EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConf: []*entity.EvaluatorConf{{EvaluatorVersionID: 111}}}}}}
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll} {
		log := &entity.ExptRunLog{SpaceID: 1, ExptID: 2, ExptRunID: 3, Mode: int32(mode)}
		snapshot, err := newHookExecutionSnapshot(expt, log, "local")
		require.NoError(t, err)
		require.NotNil(t, snapshot)
		require.Len(t, snapshot.Sets, 1)
		require.Equal(t, int64(71), snapshot.Sets[0].EvalSetID)
		require.Equal(t, int64(111), snapshot.EvaluatorFallback.Confs[0].EvaluatorVersionID)
	}
}

type retryExecutionEvaluators struct {
	*executionChainEvaluators
	calls    map[int64]int
	fail     bool
	mu       sync.Mutex
	failItem int64
}

func (s *retryExecutionEvaluators) BatchGetEvaluatorVersion(_ context.Context, _ *int64, ids []int64, _ bool) ([]*entity.Evaluator, error) {
	var out []*entity.Evaluator
	for _, id := range ids {
		out = append(out, &entity.Evaluator{ID: id + 1, SpaceID: s.f.space, EvaluatorType: entity.EvaluatorTypeCode, CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{ID: id, EvaluatorID: id + 1, SpaceID: s.f.space, CodeContent: "code"}})
	}
	return out, nil
}
func (s *retryExecutionEvaluators) RunEvaluator(ctx context.Context, in *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
	s.mu.Lock()
	s.calls[in.EvaluatorVersionID]++
	s.mu.Unlock()
	if s.fail && (in.EvaluatorVersionID == 333 || s.failItem > 0 && in.ItemID == s.failItem) {
		return nil, fmt.Errorf("retry-stage failure")
	}
	r := &entity.EvaluatorRecord{ID: finalizationTestIDs.Add(1), SpaceID: in.SpaceID, ExperimentID: in.ExperimentID, ExperimentRunID: in.ExperimentRunID, ItemID: in.ItemID, ItemVersionID: chainItemVersion(in.ItemID), TurnID: in.TurnID, EvaluatorVersionID: in.EvaluatorVersionID, Alias: in.Alias, SourceType: entity.EvaluatorRecordSourceTypeBuiltin, Status: entity.EvaluatorRunStatusSuccess, EvaluatorOutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.75)}}}
	if err := s.f.sql.WithContext(ctx).Create(ec.ConvertEvaluatorRecordDO2PO(r)).Error; err != nil {
		return nil, err
	}
	return r, nil
}

func retryExecutionAssemble(t *testing.T, d HookRuntimeExecutionDependencies) *HookRuntimeExecution {
	t.Helper()
	r, err := NewBoundHookRuntimeExecution(d)
	require.NoError(t, err)
	r.Scheduler.ResultSvc = retryExecutionResult{ExptResultService: hookPersistenceFilters{r.Scheduler.ResultSvc}, t: t}
	r.Scheduler.schedulerModeFactory.(*DefaultSchedulerModeFactory).resultSvc = r.Scheduler.ResultSvc
	return r
}

type retryExecutionResult struct {
	ExptResultService
	t *testing.T
}

func TestHookRetryR07AndRetryAllExecuteMySQL(t *testing.T) {
	for _, kind := range []string{"r07", "retry_all"} {
		t.Run(kind, func(t *testing.T) {
			f, d, pub, target := executionChainFixture(t, entity.EvaluationModeSubmit, "before")
			ctx, cancel := context.WithTimeout(context.Background(), 110*time.Second)
			defer cancel()
			original := f.key
			sourceRuntime := retryExecutionAssemble(t, d)
			t.Cleanup(func() {
				require.NoError(t, f.sql.Unscoped().Where("experiment_run_id=?", original.RunID).Delete(&tm.TargetRecord{}).Error)
				require.NoError(t, f.sql.Unscoped().Where("experiment_run_id=?", original.RunID).Delete(&em.EvaluatorRecord{}).Error)
			})
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit, ExptType: entity.ExptType_Offline, CreatedAt: time.Now().Unix(), Session: &entity.Session{UserID: "original-user"}}
			if kind == "r07" {
				completeExecutionChainBeforeOutcome(t, f, entity.HookOutcome{Code: entity.HookFailed})
				require.NoError(t, sourceRuntime.Manager.FinalizeRun(ctx, original, entity.HookTerminalIntent{}))
				require.Equal(t, "HOOK_BEFORE_FAILED", finalizationRead(t, f).State.Intent.Reason)
				require.Zero(t, target.calls)
			} else {
				completeExecutionChainBefore(t, f)
				require.NoError(t, sourceRuntime.Scheduler.Schedule(ctx, event))
				for _, item := range pub.items {
					require.NoError(t, sourceRuntime.Consumer.Eval(ctx, item))
				}
				require.NoError(t, sourceRuntime.Scheduler.Schedule(ctx, event))
				require.Equal(t, 2, target.calls)
			}
			sourceState := finalizationRead(t, f)
			mode := entity.EvaluationModeFailRetry
			if kind == "retry_all" {
				mode = entity.EvaluationModeRetryAll
			}
			f.key.RunID = finalizationTestIDs.Add(1)
			foundationCreate(t, f, mode)
			consumer := d.Consumer.(*ExptItemEventEvalServiceImpl)
			refs := exptinfra.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(f.p))
			var selector hook.PlanSelector = NewHookPlanSelector(new(boundConsumerDataset), refs, consumer.exptTurnResultRepo, consumer.exptItemResultRepo)
			if kind == "r07" {
				selector = &preparerSelector{selectFn: func(context.Context, entity.HookSelectionInput) (entity.HookSelectionPage, error) {
					t.Fatal("R07 must copy the source plan, not reselect current data")
					return entity.HookSelectionPage{}, nil
				}}
			}
			preparer, err := NewHookPlanPreparer(HookPlanPreparerDependencies{Runs: f.repo, Plans: exptinfra.NewHookPlanRepo(f.p), Selector: selector, Codec: f.manager.hooks.Codec, Experiments: f.manager.exptRepo, Initialization: f.manager.hooks.Initialization, ResultReader: exptinfra.NewHookPlanResultReader(f.p), IDs: activeReferenceIDs{}})
			require.NoError(t, err)
			for i := 0; i < 2; i++ {
				require.NoError(t, preparer.PreparePlan(ctx, hook.WorkerRunInput{ExecutionScope: "local", Candidate: entity.HookRunCandidate{Key: f.key}}))
			}
			created := finalizationRead(t, f)
			require.True(t, created.PlanReady)
			require.Equal(t, int64(2), created.PlanCount)
			require.Equal(t, original.RunID, gptr.Indirect(created.SourceRunID))
			require.NotEqual(t, sourceState.State.Before.ID, created.State.Before.ID)
			d.Binding, err = LoadHookExecutionInitializationBinding(ctx, f.repo, f.manager.hooks.Codec, f.key, "local")
			require.NoError(t, err)
			d.Repositories, err = exptinfra.NewBoundHookRuntimeRepositories(f.p, d.Binding)
			require.NoError(t, err)
			runtime := retryExecutionAssemble(t, d)
			pub.items = nil
			event.ExptRunID, event.ExptRunMode, event.CreatedAt = f.key.RunID, mode, time.Now().Unix()
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			require.Empty(t, pub.items)
			completeExecutionChainBefore(t, f)
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			require.Len(t, pub.items, 2)
			for _, item := range pub.items {
				require.NoError(t, runtime.Consumer.Eval(ctx, item))
			}
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			finished := finalizationRead(t, f)
			require.Equal(t, entity.ExptStatus_Success, finished.State.Intent.Status)
			require.True(t, finished.State.After.Activated)
			wantCalls := 2
			if kind == "retry_all" {
				wantCalls = 4
			}
			require.Equal(t, wantCalls, target.calls)
			var stats model.ExptStats
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
			require.Equal(t, int32(2), stats.SuccessCnt)
			beforeLate, err := f.repo.GetRun(ctx, original)
			require.NoError(t, err)
			require.Equal(t, sourceState, beforeLate)
			late := *pub.items[0]
			late.ExptRunID = original.RunID
			late.ExptRunMode = entity.EvaluationModeSubmit
			require.NoError(t, sourceRuntime.Consumer.Eval(ctx, &late))
			require.Equal(t, wantCalls, target.calls)
		})
	}
}

func TestHookRetrySourceOwnershipGuardsMySQL(t *testing.T) {
	for _, corruption := range []string{"source-active", "ref-config", "projection-id", "new-latest", "foreign-reuse"} {
		t.Run(corruption, func(t *testing.T) {
			f := newBoundInitFixture(t, 2, entity.EvaluationModeSubmit, false, true)
			init := boundInitializationRepo(t, f)
			_, err := f.initialize(t, init)
			require.NoError(t, err)
			bindRuntimeFinalization(t, f)
			ctx := context.Background()
			original := f.key
			require.NoError(t, f.manager.FinalizeRun(ctx, original, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}))
			f.key.RunID = finalizationTestIDs.Add(1)
			foundationCreate(t, f.finalizationManagerFixture, entity.EvaluationModeFailRetry)
			item := f.items[1]
			item.ID = finalizationTestIDs.Add(1)
			run := finalizationRead(t, f.finalizationManagerFixture)
			page, err := f.repo.AppendPlanPage(ctx, entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Items: []entity.HookPlanItem{item}})
			require.NoError(t, err)
			digest, err := entity.AppendHookPlanDigest(entity.NewHookPlanDigest(), []entity.HookPlanItem{item})
			require.NoError(t, err)
			_, err = f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: page.Run.Version}, Count: 1, Hash: digest.Hash})
			require.NoError(t, err)
			init = boundInitializationRepo(t, f)
			switch corruption {
			case "source-active":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", original.RunID).UpdateColumn("status", int64(entity.ExptStatus_Processing)).Error)
			case "ref-config":
				require.NoError(t, f.sql.Model(&model.ExptItemRef{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, item.ItemID).UpdateColumn("item_config", []byte(`{}`)).Error)
			case "projection-id":
				require.NoError(t, f.sql.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, item.ItemID).UpdateColumn("id", finalizationTestIDs.Add(1)).Error)
			case "new-latest":
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("latest_run_id", finalizationTestIDs.Add(1)).Error)
			}
			_, err = f.initialize(t, init)
			if corruption != "foreign-reuse" {
				require.Error(t, err)
				var count int64
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Count(&count).Error)
				require.Zero(t, count)
				return
			}
			require.NoError(t, err)
			run = finalizationRead(t, f.finalizationManagerFixture)
			_, err = init.(repo.IHookRepo).AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ItemID: item.ItemID})
			require.NoError(t, err)
			stores, err := exptinfra.NewBoundHookRuntimeRepositories(f.p, f.binding)
			require.NoError(t, err)
			_, _, err = stores.Progress.(repo.IHookRetryTurnLogInitializer).InitializeHookRetryTurnRunLogs(ctx, f.key, item.ItemID, item.ItemVersionID, []*entity.ExptTurnResultRunLog{{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID, TurnID: 0, Status: entity.TurnRunState_Processing, TargetResultID: 999999}})
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			var count int64
			require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Count(&count).Error)
			require.Zero(t, count)
		})
	}
}

func TestHookRetryCreationPendingCASAndReplayMySQL(t *testing.T) {
	f, d, _, _ := executionChainFixture(t, entity.EvaluationModeSubmit, "before")
	ctx := context.Background()
	source := f.key
	runtime := retryExecutionAssemble(t, d)
	completeExecutionChainBeforeOutcome(t, f, entity.HookOutcome{Code: entity.HookFailed})
	require.NoError(t, runtime.Manager.FinalizeRun(ctx, source, entity.HookTerminalIntent{}))
	sourceState := finalizationRead(t, f)
	f.key.RunID = finalizationTestIDs.Add(1)
	foundationCreate(t, f, entity.EvaluationModeFailRetry)
	created := finalizationRead(t, f)
	var parent model.Experiment
	require.NoError(t, f.sql.First(&parent, f.expt).Error)
	require.Equal(t, int32(entity.ExptStatus_Pending), parent.Status)
	require.Equal(t, f.key.RunID, parent.LatestRunID)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("status", int32(entity.ExptStatus_Processing)).Error)
	input := entity.HookCreateRunInput{Key: f.key, ExpectedLatestRunID: source.RunID, SourceRunID: gptr.Of(source.RunID), RunLog: &entity.ExptRunLog{ID: f.key.RunID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, CreatedBy: created.CreatedBy, Mode: int32(created.Mode), Status: int64(entity.ExptStatus_Pending)}, Snapshot: created.Snapshot, Before: &entity.HookOperationSeed{ID: 1, OperationID: "unused-before", IdempotencyKey: "unused-before"}, After: &entity.HookOperationSeed{ID: 2, OperationID: "unused-after", IdempotencyKey: "unused-after"}}
	replayed, err := f.repo.CreateRunWithHooks(ctx, input)
	require.NoError(t, err)
	require.False(t, replayed.Changed)
	require.Equal(t, created, replayed.Run)
	require.NoError(t, f.sql.First(&parent, f.expt).Error)
	require.Equal(t, int32(entity.ExptStatus_Processing), parent.Status)
	input.Key.RunID = finalizationTestIDs.Add(1)
	input.RunLog.ID = input.Key.RunID
	input.RunLog.ExptRunID = input.Key.RunID
	_, err = f.repo.CreateRunWithHooks(ctx, input)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.NoError(t, f.sql.First(&parent, f.expt).Error)
	require.Equal(t, f.key.RunID, parent.LatestRunID)
	require.Equal(t, int32(entity.ExptStatus_Processing), parent.Status)
	unchanged, err := f.repo.GetRun(ctx, source)
	require.NoError(t, err)
	require.Equal(t, sourceState, unchanged)
}

func (r retryExecutionResult) RecordItemRunLogs(ctx context.Context, expt, run, item, space int64, view *entity.Experiment) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	refs, err := r.ExptResultService.RecordItemRunLogs(ctx, expt, run, item, space, view)
	require.NoError(r.t, err, "first archival attempt must pass without a retry hiding a deterministic contract error")
	return refs, err
}

func TestHookRetryFailurePreservesSuccessfulItemAndStagesMySQL(t *testing.T) {
	f, d, pub, target := executionChainFixture(t, entity.EvaluationModeSubmit, "before", "retry-stage")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	consumer := d.Consumer.(*ExptItemEventEvalServiceImpl)
	evals := &retryExecutionEvaluators{executionChainEvaluators: consumer.evaluatorService.(*executionChainEvaluators), calls: map[int64]int{}, fail: true}
	consumer.evaluatorService = evals
	original := f.key
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("experiment_run_id=?", original.RunID).Delete(&tm.TargetRecord{}).Error)
		require.NoError(t, f.sql.Unscoped().Where("experiment_run_id=?", original.RunID).Delete(&em.EvaluatorRecord{}).Error)
	})
	runtime := retryExecutionAssemble(t, d)
	completeExecutionChainBefore(t, f)
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit, ExptType: entity.ExptType_Offline, CreatedAt: time.Now().Unix(), Session: &entity.Session{UserID: "original-user"}}
	require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
	require.Len(t, pub.items, 2)
	for _, item := range pub.items {
		require.NoError(t, runtime.Consumer.Eval(ctx, item))
	}
	require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
	require.Equal(t, entity.ExptStatus_Failed, finalizationRead(t, f).State.Intent.Status)
	var success model.ExptItemResult
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND item_id=801", f.space, f.expt).First(&success).Error)
	var oldStage model.ExptTurnEvaluatorResultRef
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND evaluator_version_id=222", f.space, f.expt).First(&oldStage).Error)
	f.key.RunID = finalizationTestIDs.Add(1)
	f.manager.configer = consumer.configer
	f.manager.mtr = retryExecutionMetric{f.manager.mtr}
	foundationCreate(t, f, entity.EvaluationModeFailRetry)
	newRun := finalizationRead(t, f)
	require.Equal(t, original.RunID, gptr.Indirect(newRun.SourceRunID))
	refs := exptinfra.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(f.p))
	selector := NewHookPlanSelector(new(boundConsumerDataset), refs, consumer.exptTurnResultRepo, consumer.exptItemResultRepo)
	preparer, err := NewHookPlanPreparer(HookPlanPreparerDependencies{Runs: f.repo, Plans: exptinfra.NewHookPlanRepo(f.p), Selector: selector, Codec: f.manager.hooks.Codec, Experiments: f.manager.exptRepo, Initialization: f.manager.hooks.Initialization, ResultReader: exptinfra.NewHookPlanResultReader(f.p), IDs: activeReferenceIDs{}})
	require.NoError(t, err)
	for i := 0; i < 4; i++ {
		require.NoError(t, preparer.PreparePlan(ctx, hook.WorkerRunInput{ExecutionScope: "local", Candidate: entity.HookRunCandidate{Key: f.key}}))
	}
	newRun = finalizationRead(t, f)
	require.True(t, newRun.PlanReady)
	require.Equal(t, int64(1), newRun.PlanCount)
	d.Binding, err = LoadHookExecutionInitializationBinding(ctx, f.repo, f.manager.hooks.Codec, f.key, "local")
	require.NoError(t, err)
	d.Repositories, err = exptinfra.NewBoundHookRuntimeRepositories(f.p, d.Binding)
	require.NoError(t, err)
	retry := retryExecutionAssemble(t, d)
	pub.items = nil
	event.ExptRunID, event.ExptRunMode, event.CreatedAt = f.key.RunID, entity.EvaluationModeFailRetry, time.Now().Unix()
	require.NoError(t, retry.Scheduler.Schedule(ctx, event))
	require.Empty(t, pub.items)
	completeExecutionChainBefore(t, f)
	evals.fail = false
	require.NoError(t, retry.Scheduler.Schedule(ctx, event))
	require.Len(t, pub.items, 1)
	require.Equal(t, int64(802), pub.items[0].EvalSetItemID)
	require.NoError(t, retry.Consumer.Eval(ctx, pub.items[0]))
	require.NoError(t, retry.Scheduler.Schedule(ctx, event))
	finished := finalizationRead(t, f)
	require.Equal(t, entity.ExptStatus_Success, finished.State.Intent.Status)
	require.True(t, finished.State.After.Activated)
	var unchanged model.ExptItemResult
	require.NoError(t, f.sql.First(&unchanged, success.ID).Error)
	require.Equal(t, success, unchanged)
	var reused model.ExptTurnEvaluatorResultRef
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND evaluator_version_id=222", f.space, f.expt).First(&reused).Error)
	require.Equal(t, oldStage.EvaluatorResultID, reused.EvaluatorResultID)
	require.Equal(t, 2, target.calls)
	require.Equal(t, 1, evals.calls[111])
	require.Equal(t, 1, evals.calls[222])
	require.Equal(t, 2, evals.calls[333])
	var stats model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
	require.Equal(t, int32(2), stats.SuccessCnt)
	require.Zero(t, stats.FailCnt+stats.PendingCnt+stats.ProcessingCnt)
	require.NoError(t, retry.Consumer.Eval(ctx, pub.items[0]))
	require.Equal(t, 2, target.calls)
}
