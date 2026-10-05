// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestHookPartialInitializationRejectsCorruptionBeforeDeletionAndCleanup(t *testing.T) {
	for _, kind := range []string{"admitted", "started", "manifest_missing", "manifest_corrupt", "turn_pin", "no_execution_failure", "missing_log", "replaced_log", "foreign_turn", "foreign_projection", "softdeleted_turn", "turn_log", "target", "score", "trace", "ref", "retry", "quota", "processing", "extra_unmaterialized", "plan_hash"} {
		t.Run(kind, func(t *testing.T) {
			f := newExecutionFixture(t, 2)
			ctx := context.Background()
			run, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			in := f.writeInput(run.Version)
			in.Items = in.Items[:1]
			page, err := f.init.WriteExecutionInitializationPage(ctx, in)
			require.NoError(t, err)
			m := f.manifests[0]
			switch kind {
			case "admitted":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", m.Frozen.ID).UpdateColumn("admitted_at", time.Now()).Error)
			case "started":
				require.NoError(t, hookRunScope(f.sql.Model(&model.ExptLifecycleRun{}), f.key).UpdateColumn("execution_started", true).Error)
			case "manifest_missing":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", m.Frozen.ID).UpdateColumn("execution_manifest", nil).Error)
			case "manifest_corrupt":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", m.Frozen.ID).UpdateColumn("execution_manifest", []byte("{} trailing")).Error)
			case "turn_pin", "no_execution_failure":
				if kind == "turn_pin" {
					m.TurnLogsInitialized = nil
				} else {
					m.NoExecutionFailure = true
				}
				raw, err := json.Marshal(m)
				require.NoError(t, err)
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", m.Frozen.ID).UpdateColumn("execution_manifest", raw).Error)
			case "missing_log":
				require.NoError(t, f.sql.Unscoped().Delete(&model.ExptItemResultRunLog{}, m.ItemRunLogID).Error)
			case "replaced_log":
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("id", hookTxSequence.Add(1)).Error)
			case "foreign_turn":
				require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", m.Turns[0].ResultID).UpdateColumn("space_id", f.space+100000).Error)
				t.Cleanup(func() {
					require.NoError(t, f.sql.Unscoped().Delete(&model.ExptTurnResult{}, m.Turns[0].ResultID).Error)
				})
			case "foreign_projection":
				require.NoError(t, f.sql.Model(&model.ExptItemResult{}).Where("id=?", m.ItemResultID).UpdateColumn("expt_run_id", f.key.RunID+100000).Error)
			case "softdeleted_turn":
				require.NoError(t, f.sql.Delete(&model.ExptTurnResult{}, m.Turns[0].ResultID).Error)
			case "turn_log":
				require.NoError(t, f.sql.Create(&model.ExptTurnResultRunLog{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: m.Frozen.ItemID, TurnID: m.Turns[0].TurnID}).Error)
			case "target":
				require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", m.Turns[0].ResultID).UpdateColumn("target_result_id", 99).Error)
			case "score":
				require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", m.Turns[0].ResultID).UpdateColumn("weighted_score", 0).Error)
			case "trace":
				require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", m.Turns[0].ResultID).UpdateColumn("log_id", "actual-execution").Error)
			case "ref":
				id := hookTxSequence.Add(1)
				require.NoError(t, f.sql.Create(&model.ExptTurnEvaluatorResultRef{ID: id, SpaceID: f.space, ExptID: f.expt, ExptTurnResultID: m.Turns[0].ResultID, EvaluatorVersionID: 55, EvaluatorResultID: 66}).Error)
				t.Cleanup(func() { require.NoError(t, f.sql.Unscoped().Delete(&model.ExptTurnEvaluatorResultRef{}, id).Error) })
			case "retry":
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("retry_times", 1).Error)
			case "quota":
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("quota_reservation_state", 1).Error)
			case "processing":
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("status", int32(entity.ItemRunState_Processing)).Error)
			case "extra_unmaterialized":
				require.NoError(t, f.sql.Create(&model.ExptItemResultRunLog{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: f.manifests[1].Frozen.ItemID}).Error)
			case "plan_hash":
				require.NoError(t, hookRunScope(f.sql.Model(&model.ExptLifecycleRun{}), f.key).UpdateColumn("plan_hash", entity.NewHookPlanDigest().Hash).Error)
			}
			_, err = deletionRepo(t, f.hookTxFixture).DeleteExperiments(ctx, []int64{f.expt}, f.space, "local")
			require.Error(t, err)
			var visible model.Experiment
			require.NoError(t, f.sql.First(&visible, f.expt).Error)
			pending, err := f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: page.RunVersion}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}})
			require.NoError(t, err)
			_, err = NewHookFinalizationRepo(f.p).(repo.IHookPartialInitializationFinalizer).PreparePartialInitializationTermination(ctx, f.key, "local")
			require.Error(t, err)
			after, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, pending.Run, after)
			require.False(t, after.State.After.Activated)
		})
	}
}

func TestHookPartialInitializationPendingBatchDeletionRecovery(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := newExecutionFixture(t, 2)
			ctx := context.Background()
			run, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			in := f.writeInput(run.Version)
			in.StartOrdinal, in.Items = 1, in.Items[1:]
			page, err := f.init.WriteExecutionInitializationPage(ctx, in)
			require.NoError(t, err)
			pending, err := f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: page.RunVersion}, Intent: entity.HookTerminalIntent{Status: status, Reason: "original"}})
			require.NoError(t, err)
			legacy := f.space
			require.NoError(t, f.sql.Create(&model.Experiment{ID: legacy, SpaceID: f.space, Name: fmt.Sprint(legacy)}).Error)
			t.Cleanup(func() { require.NoError(t, f.sql.Unscoped().Delete(&model.Experiment{}, legacy).Error) })
			deleted, err := deletionRepo(t, f.hookTxFixture).DeleteExperiments(ctx, []int64{legacy, f.expt}, f.space, "local")
			require.NoError(t, err)
			require.Len(t, deleted, 2)
			after, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, pending.Run, after)
			storage := NewHookFinalizationRepo(f.p)
			for i := 0; i < 2; i++ {
				handled, err := storage.(repo.IHookPartialInitializationFinalizer).PreparePartialInitializationTermination(ctx, f.key, "local")
				require.NoError(t, err)
				require.True(t, handled)
			}
			stats, err := storage.ReadFinalizationStats(ctx, f.key, "local")
			require.NoError(t, err)
			out, err := f.repo.CommitFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: pending.Run.Version}, Intent: pending.Run.State.Intent, Stats: stats})
			require.NoError(t, err)
			require.Equal(t, status, out.Run.State.Status)
			require.True(t, out.Run.State.After.Activated)
			var log model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&log, f.manifests[1].ItemRunLogID).Error)
			require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(log.ResultState))
		})
	}
}
