// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestHookFinalizationReadOnlyWhileInitializerLocked(t *testing.T) {
	f := settledFinalizationFixture(t)
	tx := f.sql.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, tx.Exec("SELECT id FROM experiment WHERE id=? FOR UPDATE", f.expt).Error)
	require.NoError(t, tx.Exec("SELECT expt_run_id FROM expt_lifecycle_run WHERE space_id=? AND expt_run_id=? FOR UPDATE", f.space, f.key.RunID).Error)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r := NewHookFinalizationRepo(f.p)
	source, err := r.ReadFinalizationSource(ctx, entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt})
	require.NoError(t, err)
	require.Equal(t, f.key, source.Key)
	require.True(t, source.Managed)
	stats, err := r.ReadFinalizationStats(ctx, f.key, "local")
	require.NoError(t, err)
	require.Equal(t, int32(2), stats.Items.Success)
	require.Equal(t, int32(4), stats.Turns.Success)
}

func TestHookFinalizationStatsRejectsCorruption(t *testing.T) {
	for _, kind := range []string{"missing-manifest", "missing-item", "extra-turn", "wrong-version", "wrong-scope", "bad-hash"} {
		t.Run(kind, func(t *testing.T) {
			f := settledFinalizationFixture(t)
			ctx := context.Background()
			scope := "local"
			switch kind {
			case "missing-manifest":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("expt_run_id=?", f.key.RunID).UpdateColumn("execution_manifest", nil).Error)
			case "missing-item":
				require.NoError(t, f.sql.Where("id=?", f.manifests[0].ItemRunLogID).Delete(&model.ExptItemResultRunLog{}).Error)
			case "extra-turn":
				require.NoError(t, f.sql.Create(&model.ExptTurnResultRunLog{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: f.manifests[0].Frozen.ItemID, TurnID: 99, Status: 1}).Error)
			case "wrong-version":
				require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("expt_run_id=?", f.key.RunID).UpdateColumn("item_version_id", 999).Error)
			case "wrong-scope":
				scope = "other"
			case "bad-hash":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("expt_run_id=?", f.key.RunID).UpdateColumn("plan_hash", "bad").Error)
			}
			_, err := NewHookFinalizationRepo(f.p).ReadFinalizationStats(ctx, f.key, scope)
			require.Error(t, err)
		})
	}
}

func TestHookFinalizationCommitRechecksOriginalStats(t *testing.T) {
	f := settledFinalizationFixture(t)
	ctx := context.Background()
	r := NewHookFinalizationRepo(f.p)
	stats, err := r.ReadFinalizationStats(ctx, f.key, "local")
	require.NoError(t, err)
	state, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	in := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: state.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Success}, Stats: stats}
	began, err := f.repo.BeginFinalize(ctx, in)
	require.NoError(t, err)
	in.ExpectedVersion = began.Run.Version
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", f.manifests[0].ItemRunLogID).UpdateColumn("status", 3).Error)
	_, err = f.repo.CommitFinalize(ctx, in)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	state, err = f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.False(t, state.State.After.Activated)
	require.Equal(t, entity.HookFinalizePending, state.State.Finalize)
}

func TestHookFinalizationCommitRechecksArchival(t *testing.T) {
	f := settledFinalizationFixture(t)
	ctx := context.Background()
	stats, err := NewHookFinalizationRepo(f.p).ReadFinalizationStats(ctx, f.key, "local")
	require.NoError(t, err)
	state, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	in := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: state.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Success}, Stats: stats}
	begun, err := f.repo.BeginFinalize(ctx, in)
	require.NoError(t, err)
	in.ExpectedVersion = begun.Run.Version
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", f.manifests[0].ItemRunLogID).UpdateColumn("result_state", int32(entity.ExptItemResultStateLogged)).Error)
	_, err = f.repo.CommitFinalize(ctx, in)
	require.ErrorIs(t, err, entity.ErrHookFinalizationUnsettled)
	state, err = f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, entity.HookFinalizePending, state.State.Finalize)
	require.False(t, state.State.After.Activated)
}
