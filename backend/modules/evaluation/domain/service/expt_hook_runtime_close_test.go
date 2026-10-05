// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

// Catch a multi-set original Run being left Pending instead of cleaning its committed manifests.
func TestHookBoundRuntimeCancelActivatesAfterMySQL(t *testing.T) {
	f, _ := boundConsumerFixture(t)
	bindRuntimeFinalization(t, f)
	err := f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated})
	require.NoError(t, err)
	run := finalizationRead(t, f.finalizationManagerFixture)
	require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize)
	var after model.ExptLifecycleHookRun
	require.NoError(t, f.sql.Where("operation_id=?", run.State.After.ID).First(&after).Error)
	require.NotNil(t, after.ActivatedAt)
}

func TestHookBoundRuntimePartialDeleteAndSuccessorMySQL(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		for _, state := range []string{"partial", "initialized", "successor", "deleted"} {
			t.Run(fmt.Sprintf("%d/%s", mode, state), func(t *testing.T) {
				f := newBoundInitFixture(t, 2, mode, false, true)
				r := boundInitializationRepo(t, f)
				ctx := context.Background()
				if state == "partial" {
					p, err := r.ReadExecutionInitializationPage(ctx, entity.HookPlanReadInput{Key: f.key, ExecutionScope: "local", Limit: 1})
					require.NoError(t, err)
					m := entity.HookExecutionManifest{Version: 1, Key: f.key, Frozen: p.Items[0].Frozen, ItemResultID: finalizationTestIDs.Add(1), ItemRunLogID: finalizationTestIDs.Add(1), Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, ResultID: finalizationTestIDs.Add(1)}}, TurnLogsInitialized: gptr.Of(false), ItemRef: &entity.HookExecutionItemRef{ID: finalizationTestIDs.Add(1), ConfigHash: p.Items[0].ItemRefConfigHash}}
					_, err = r.WriteExecutionInitializationPage(ctx, entity.HookExecutionInitializationWriteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: p.RunVersion}, ExecutionScope: "local", PlanHash: p.Hash, Items: []entity.HookExecutionManifest{m}})
					require.NoError(t, err)
				} else {
					_, err := f.initialize(t, r)
					require.NoError(t, err)
				}
				bindRuntimeFinalization(t, f)
				var successor *entity.HookStoredRun
				if state == "successor" {
					key := f.key
					key.RunID = finalizationTestIDs.Add(1)
					initial, err := f.manager.hooks.Initialization.ReadRunInitialization(ctx, key)
					require.NoError(t, err)
					created, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{Key: key, ExpectedLatestRunID: f.key.RunID, ExpectedConfigRevision: initial.ConfigRevision, RunLog: &entity.ExptRunLog{ID: key.RunID, SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, Mode: int32(mode), Status: int64(entity.ExptStatus_Processing), CreatedBy: "successor"}, Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{2}, KeyID: "key", Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ExecutionScope: "local"}, After: &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(key.RunID), IdempotencyKey: fmt.Sprint(key.RunID)}})
					require.NoError(t, err)
					successor = created.Run
					require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("target_id", 777).Error)
				}
				if state == "deleted" {
					_, err := r.(repo.IHookDeletionRepo).DeleteExperiments(ctx, []int64{f.expt}, f.space, "local")
					require.NoError(t, err)
				}
				require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}))
				run := finalizationRead(t, f.finalizationManagerFixture)
				require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize)
				require.True(t, run.State.After.Activated)
				if state == "successor" {
					var row model.Experiment
					require.NoError(t, f.sql.First(&row, f.expt).Error)
					require.Equal(t, successor.State.Key.RunID, row.LatestRunID)
					require.Equal(t, int32(entity.ExptStatus_Processing), row.Status)
					require.Equal(t, int64(777), row.TargetID)
					unchanged, err := f.repo.GetRun(ctx, successor.State.Key)
					require.NoError(t, err)
					require.Equal(t, successor, unchanged)
				}
			})
		}
	}
}

func bindRuntimeFinalization(t *testing.T, f *boundInitFixture) {
	t.Helper()
	r, err := exptinfra.NewBoundHookExecutionInitializationRepo(f.p, f.binding)
	require.NoError(t, err)
	storage, err := exptinfra.NewBoundHookFinalizationRepo(f.p, f.binding)
	require.NoError(t, err)
	f.manager.finalization.Runs = r.(repo.IHookRepo)
	f.manager.finalization.Repository = storage
	f.manager.finalization.NewItemLocker = func() lock.ILocker { return lock.NewRedisLocker(f.redis) }
}

func TestHookBoundRuntimeSourceIgnoresCurrentConfigMySQL(t *testing.T) {
	f, _ := boundConsumerFixture(t)
	bindRuntimeFinalization(t, f)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"target_id": 777, "target_version_id": 778, "eval_conf": []byte(`{}`)}).Error)
	source, err := f.manager.finalization.Repository.ReadFinalizationSource(context.Background(), f.key)
	require.NoError(t, err)
	require.Equal(t, int64(91), source.Experiment.TargetID)
	require.Equal(t, int64(92), source.Experiment.TargetVersionID)
	require.NotNil(t, source.Experiment.EvalConf.ConnectorConf.EvaluatorsConf)
	require.Equal(t, "original-env", *source.Experiment.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf[0].RunConf.Env)
}

func TestHookBoundRuntimeRefTamperBlocksAfterMySQL(t *testing.T) {
	for _, field := range []string{"id", "item_config", "order_idx", "item_version_id"} {
		t.Run(field, func(t *testing.T) {
			f, _ := boundConsumerFixture(t)
			bindRuntimeFinalization(t, f)
			values := map[string]any{"id": finalizationTestIDs.Add(1), "item_config": []byte(`{}`), "order_idx": 99, "item_version_id": 123}
			require.NoError(t, f.sql.Model(&model.ExptItemRef{}).Where("expt_id=? AND space_id=?", f.expt, f.space).UpdateColumn(field, values[field]).Error)
			require.Error(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}))
			run := finalizationRead(t, f.finalizationManagerFixture)
			require.False(t, run.State.After.Activated)
			var row model.ExptItemResultRunLog
			require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&row).Error)
			require.Equal(t, int32(entity.ItemRunState_Queueing), row.Status)
		})
	}
}

func TestHookBoundRuntimeSourcePreservesLiveConcurrencyMySQL(t *testing.T) {
	f, _ := boundConsumerFixture(t)
	bindRuntimeFinalization(t, f)
	source, err := f.manager.finalization.Repository.ReadFinalizationSource(context.Background(), f.key)
	require.NoError(t, err)
	require.Equal(t, 2, gptr.Indirect(source.Experiment.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConcurNum))
	require.Equal(t, 77, gptr.Indirect(source.Experiment.EvalConf.ItemConcurNum))
}
