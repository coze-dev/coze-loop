// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	em "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	tm "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestHookRetrySchedulerFailureProjectionMySQL(t *testing.T) {
	for _, kind := range []string{"zombie", "sandbox"} {
		t.Run(kind, func(t *testing.T) {
			f, d, pub, target := executionChainFixture(t, entity.EvaluationModeSubmit, "before", "retry-stage")
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
			defer cancel()
			consumer := d.Consumer.(*ExptItemEventEvalServiceImpl)
			consumer.evaluatorService = &retryExecutionEvaluators{executionChainEvaluators: consumer.evaluatorService.(*executionChainEvaluators), calls: map[int64]int{}, fail: true}
			original := f.key
			t.Cleanup(func() {
				require.NoError(t, f.sql.Unscoped().Where("experiment_run_id=?", original.RunID).Delete(&tm.TargetRecord{}).Error)
				require.NoError(t, f.sql.Unscoped().Where("experiment_run_id=?", original.RunID).Delete(&em.EvaluatorRecord{}).Error)
			})
			sourceRuntime := retryExecutionAssemble(t, d)
			completeExecutionChainBefore(t, f)
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: original.RunID, ExptRunMode: entity.EvaluationModeSubmit, CreatedAt: time.Now().Unix(), Session: &entity.Session{UserID: "original-user"}}
			require.NoError(t, sourceRuntime.Scheduler.Schedule(ctx, event))
			require.Len(t, pub.items, 2)
			for _, item := range pub.items {
				target.fail = kind == "sandbox" && item.EvalSetItemID == 802
				require.NoError(t, sourceRuntime.Consumer.Eval(ctx, item))
			}
			target.fail = false
			require.NoError(t, sourceRuntime.Scheduler.Schedule(ctx, event))
			sourceState := finalizationRead(t, f)
			require.Equal(t, entity.ExptStatus_Failed, sourceState.State.Intent.Status)
			var success model.ExptItemResult
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND item_id=801", f.space, f.expt).First(&success).Error)
			var sourceTargets []tm.TargetRecord
			var sourceEvaluators []em.EvaluatorRecord
			require.NoError(t, f.sql.Where("experiment_run_id=?", original.RunID).Order("id").Find(&sourceTargets).Error)
			require.NoError(t, f.sql.Where("experiment_run_id=?", original.RunID).Order("id").Find(&sourceEvaluators).Error)
			f.key.RunID = finalizationTestIDs.Add(1)
			foundationCreate(t, f, entity.EvaluationModeFailRetry)
			refs := exptinfra.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(f.p))
			selector := NewHookPlanSelector(new(boundConsumerDataset), refs, consumer.exptTurnResultRepo, consumer.exptItemResultRepo)
			preparer, err := NewHookPlanPreparer(HookPlanPreparerDependencies{Runs: f.repo, Plans: exptinfra.NewHookPlanRepo(f.p), Selector: selector, Codec: f.manager.hooks.Codec, Experiments: f.manager.exptRepo, Initialization: f.manager.hooks.Initialization, ResultReader: exptinfra.NewHookPlanResultReader(f.p), IDs: activeReferenceIDs{}})
			require.NoError(t, err)
			for i := 0; i < 4; i++ {
				require.NoError(t, preparer.PreparePlan(ctx, hook.WorkerRunInput{ExecutionScope: "local", Candidate: entity.HookRunCandidate{Key: f.key}}))
			}
			require.Equal(t, int64(1), finalizationRead(t, f).PlanCount)
			d.Binding, err = LoadHookExecutionInitializationBinding(ctx, f.repo, f.manager.hooks.Codec, f.key, "local")
			require.NoError(t, err)
			d.Repositories, err = exptinfra.NewBoundHookRuntimeRepositories(f.p, d.Binding)
			require.NoError(t, err)
			runtime := retryExecutionAssemble(t, d)
			pub.items = nil
			event.ExptRunID, event.ExptRunMode, event.CreatedAt = f.key.RunID, entity.EvaluationModeFailRetry, time.Now().Unix()
			completeExecutionChainBefore(t, f)
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			require.Len(t, pub.items, 1)
			require.Equal(t, int64(802), pub.items[0].EvalSetItemID)
			run := finalizationRead(t, f)
			_, err = d.Repositories.Runs.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ItemID: 802})
			require.NoError(t, err)
			itemCtx, err := runtime.Consumer.buildBoundHookConsumerContext(ctx, pub.items[0])
			require.NoError(t, err)
			progressCtx := context.WithValue(ctx, itemHookProgressContextKey{}, itemHookProgressBinding{key: f.key, itemID: 802, itemVersion: 7, repo: d.Repositories.Progress, targets: new(sync.Map)})
			pre := &ExptRecordEvalModeFailRetry{idgen: activeReferenceIDs{}, evalTargetService: target, evaluatorRecordSvc: consumer.evaluatorRecordService, exptTurnResultRepo: consumer.exptTurnResultRepo}
			require.NoError(t, pre.PreEval(progressCtx, itemCtx))
			require.Equal(t, int64(0), itemCtx.HookManifest.Ordinal)
			require.Equal(t, int64(1), itemCtx.HookManifest.ProjectionOrdinal())
			require.Len(t, itemCtx.ExistItemEvalResult.TurnResultRunLogs, 1)
			var observed []int64
			if kind == "sandbox" {
				for _, turn := range itemCtx.ExistItemEvalResult.TurnResultRunLogs {
					require.Zero(t, turn.TargetResultID)
					record, _, err := target.AsyncExecuteTarget(ctx, f.space+90, 91, 92, &entity.ExecuteTargetCtx{ExperimentRunID: gptr.Of(f.key.RunID), ItemID: 802, TurnID: turn.TurnID}, &entity.EvalTargetInputData{})
					require.NoError(t, err)
					progress := *turn
					progress.TargetResultID = record.ID
					_, err = d.Repositories.Progress.WriteTurnProgress(ctx, entity.HookTurnProgressInput{Base: turn, Progress: &progress})
					require.NoError(t, err)
					observed = append(observed, record.ID)
				}
			}
			old := time.Now().Add(-time.Hour)
			require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_run_id=? AND item_id=802", f.space, f.key.RunID).UpdateColumn("updated_at", old).Error)
			storage := d.Repositories.Finalization.(repo.IHookSchedulerFailureRepo)
			prepared, err := storage.ReadHookSchedulerFailureItem(ctx, f.key, "local", 802)
			require.NoError(t, err)
			require.NotNil(t, prepared)
			source, err := d.Repositories.Finalization.ReadFinalizationSource(ctx, f.key)
			require.NoError(t, err)
			archive := runtime.Scheduler.ResultSvc
			runtime.Scheduler.ResultSvc = runtime.Manager.exptResultService
			runtime.Scheduler.evalTargetService = target
			require.NoError(t, runtime.Scheduler.loadHookSchedulerFailureRecords(ctx, source, prepared))
			runtime.Scheduler.ResultSvc = archive
			in := entity.HookSchedulerFailureInput{ExecutionScope: "local", Item: prepared, Zombie: kind == "zombie", ZombieSeconds: 60, ExpiredBefore: time.Now().Add(-time.Minute), ObservedTargetIDs: observed, SandboxStatus: "Failed"}
			changed, err := storage.ApplyHookSchedulerFailure(ctx, in)
			require.NoError(t, err, "old projection ordinal 1 is lawful in a one-item retry plan")
			require.True(t, changed)
			var failed model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&failed, prepared.Item.ID).Error)
			require.Equal(t, int32(entity.ItemRunState_Fail), failed.Status)
			require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(failed.ResultState))
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			finished := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizeCommitted, finished.State.Finalize)
			require.Equal(t, entity.ExptStatus_Failed, finished.State.Intent.Status)
			require.True(t, finished.State.After.Activated)
			var unchanged, projection model.ExptItemResult
			require.NoError(t, f.sql.First(&unchanged, success.ID).Error)
			require.Equal(t, success, unchanged)
			require.NoError(t, f.sql.First(&projection, prepared.Manifest.ItemResultID).Error)
			require.Equal(t, int32(1), gptr.Indirect(projection.ItemIdx))
			require.Equal(t, int32(entity.ItemRunState_Fail), projection.Status)
			var afterTargets []tm.TargetRecord
			var afterEvaluators []em.EvaluatorRecord
			require.NoError(t, f.sql.Where("experiment_run_id=?", original.RunID).Order("id").Find(&afterTargets).Error)
			require.NoError(t, f.sql.Where("experiment_run_id=?", original.RunID).Order("id").Find(&afterEvaluators).Error)
			require.Equal(t, sourceTargets, afterTargets)
			require.Equal(t, sourceEvaluators, afterEvaluators)
			var stats model.ExptStats
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
			require.Equal(t, int32(1), stats.SuccessCnt)
			require.Equal(t, int32(1), stats.FailCnt)
			require.Zero(t, stats.PendingCnt+stats.ProcessingCnt)
			changed, err = storage.ApplyHookSchedulerFailure(ctx, in)
			require.NoError(t, err)
			require.False(t, changed)
			unchangedSource, err := f.repo.GetRun(ctx, original)
			require.NoError(t, err)
			require.Equal(t, sourceState, unchangedSource)
		})
	}
}
