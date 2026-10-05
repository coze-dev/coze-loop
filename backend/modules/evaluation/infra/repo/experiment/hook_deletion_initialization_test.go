// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

// Deletion retains enough original-Run evidence to finish without completing initialization.
func TestHookDeletionPartialInitializationRecovery(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			f := newExecutionFixture(t, count)
			ctx := context.Background()
			run, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			in := f.writeInput(run.Version)
			in.Items = f.manifests[:1]
			_, err = f.init.WriteExecutionInitializationPage(ctx, in)
			require.NoError(t, err)
			before, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			var parent model.Experiment
			require.NoError(t, f.sql.First(&parent, f.expt).Error)
			deleted, err := deletionRepo(t, f.hookTxFixture).DeleteExperiments(ctx, []int64{f.expt}, f.space, "local")
			require.NoError(t, err)
			require.Len(t, deleted, 1)
			after, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, before.State.After.ID, after.State.After.ID)
			require.Equal(t, entity.HookGateClosed, after.State.Gate)
			var visible model.Experiment
			require.Error(t, f.sql.First(&visible, f.expt).Error)
			deleted, err = deletionRepo(t, f.hookTxFixture).DeleteExperiments(ctx, []int64{f.expt}, f.space, "local")
			require.NoError(t, err)
			require.Empty(t, deleted)
			storage := NewHookFinalizationRepo(f.p)
			handled, err := storage.(repo.IHookPartialInitializationFinalizer).PreparePartialInitializationTermination(ctx, f.key, "local")
			require.NoError(t, err)
			require.True(t, handled)
			stats, err := storage.ReadFinalizationStats(ctx, f.key, "local")
			require.NoError(t, err)
			require.Equal(t, int32(1), stats.Items.Terminated)
			require.Equal(t, int32(2), stats.Turns.Terminated)
			require.Len(t, stats.ItemIDs, count)
			run, err = f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			result, err := f.repo.CommitFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Intent: run.State.Intent, Stats: stats})
			require.NoError(t, err)
			require.Equal(t, entity.HookFinalizeCommitted, result.Run.State.Finalize)
			require.True(t, result.Run.State.After.Activated)
			var life model.ExptLifecycleRun
			require.NoError(t, hookRunScope(f.sql, f.key).First(&life).Error)
			require.False(t, life.ExecutionInitialized)
			require.False(t, life.ExecutionStarted)
		})
	}
}

func TestHookDeletionPartialProofRejectsUnrecoverablePreInitDecision(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	in := deletionRun(t, f, 0)
	pending, err := f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Success}})
	require.NoError(t, err)
	_, err = deletionRepo(t, f).DeleteExperiments(ctx, []int64{f.expt}, f.space, "local")
	require.ErrorIs(t, err, entity.ErrHookFinalizationUnsettled)
	current, err := f.repo.GetRun(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, pending.Run, current)
	var visible model.Experiment
	require.NoError(t, f.sql.First(&visible, f.expt).Error)
}

func TestHookDeletionCorruptPartialInitializationPendingAndBatchRollback(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(fmt.Sprint(pending), func(t *testing.T) {
			f := newExecutionFixture(t, 1)
			ctx := context.Background()
			run, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			page, err := f.init.WriteExecutionInitializationPage(ctx, f.writeInput(run.Version))
			require.NoError(t, err)
			require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", f.manifests[0].Turns[0].ResultID).UpdateColumn("weighted_score", 0).Error)
			if pending {
				_, err = f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: page.RunVersion}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancelled"}})
				require.NoError(t, err)
			}
			before, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			legacy := f.space
			require.NoError(t, f.sql.Create(&model.Experiment{ID: legacy, SpaceID: f.space, Name: fmt.Sprint(legacy)}).Error)
			t.Cleanup(func() { require.NoError(t, f.sql.Unscoped().Delete(&model.Experiment{}, legacy).Error) })
			deleted, err := deletionRepo(t, f.hookTxFixture).DeleteExperiments(ctx, []int64{legacy, f.expt}, f.space, "local")
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			require.Empty(t, deleted)
			after, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, before, after, "previously closed gates must not be reopened")
			var count int64
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id IN ?", []int64{legacy, f.expt}).Count(&count).Error)
			require.Equal(t, int64(2), count)
		})
	}
}
