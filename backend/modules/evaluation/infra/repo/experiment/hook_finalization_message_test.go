// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestHookFinalizationMessageRepoPendingAndCollision(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	created := f.input(false, 0)
	created.RunLog.Mode = int32(entity.EvaluationModeSubmit)
	_, err := f.repo.CreateRunWithHooks(ctx, created)
	require.NoError(t, err)
	prefix := strings.Repeat("q", 128)
	a, b := prefix+"A", prefix+"B"
	in := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: created.Key}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Failed}}
	raw, err := json.Marshal(map[string]any{"DisplayMessage": a})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &in))
	begun, err := f.repo.BeginFinalize(ctx, in)
	require.NoError(t, err)
	var log model.ExptRunLog
	require.NoError(t, f.sql.First(&log, created.Key.RunID).Error)
	require.Equal(t, a, string(gptr.Indirect(log.StatusMessage)))
	collision := in
	collision.DisplayMessage = nil
	raw, err = json.Marshal(map[string]any{"DisplayMessage": b})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &collision))
	_, err = f.repo.BeginFinalize(ctx, collision)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	in.ExpectedVersion = begun.Run.Version
	committed, err := f.repo.CommitFinalize(ctx, in)
	require.NoError(t, err)
	require.True(t, committed.Effects.ActivateAfter)
	var expt model.Experiment
	require.NoError(t, f.sql.First(&expt, f.expt).Error)
	require.Equal(t, a, string(gptr.Indirect(expt.StatusMessage)))
	_, err = f.repo.CommitFinalize(ctx, collision)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	replay, err := f.repo.CommitFinalize(ctx, in)
	require.NoError(t, err)
	require.False(t, replay.Changed)
}

func TestHookFinalizationMessageBeginRollback(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	created := f.input(false, 0)
	created.RunLog.Mode = int32(entity.EvaluationModeSubmit)
	_, err := f.repo.CreateRunWithHooks(ctx, created)
	require.NoError(t, err)
	display := strings.Repeat("m", 200)
	in := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: created.Key}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Failed}, DisplayMessage: &display}
	injected := errors.New("lifecycle write failed")
	require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register("finalization_message_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_lifecycle_run" {
			tx.AddError(injected)
		}
	}))
	_, err = f.repo.BeginFinalize(ctx, in)
	require.ErrorIs(t, err, injected)
	var log model.ExptRunLog
	require.NoError(t, f.sql.First(&log, created.Key.RunID).Error)
	require.Nil(t, log.StatusMessage)
	state, err := f.repo.GetRun(ctx, created.Key)
	require.NoError(t, err)
	require.Equal(t, entity.HookFinalizeNone, state.State.Finalize)
	require.False(t, state.State.After.Activated)
	require.NoError(t, f.sql.Callback().Update().Remove("finalization_message_failure"))
	_, err = f.repo.BeginFinalize(ctx, in)
	require.NoError(t, err)
	require.NoError(t, f.sql.First(&log, created.Key.RunID).Error)
	require.Equal(t, display, string(gptr.Indirect(log.StatusMessage)))
}

func TestHookFinalizationMessageAbsentInputCompatibility(t *testing.T) {
	t.Run("nil_stats", func(t *testing.T) {
		f := newHookTxFixture(t)
		ctx := context.Background()
		create := f.input(false, 0)
		create.RunLog.Mode = int32(entity.EvaluationModeSubmit)
		create.RunLog.StatusMessage = []byte("old-run")
		_, err := f.repo.CreateRunWithHooks(ctx, create)
		require.NoError(t, err)
		require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("status_message", []byte("old-experiment")).Error)
		in := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: create.Key}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Failed, Reason: "reason"}}
		begun, err := f.repo.BeginFinalize(ctx, in)
		require.NoError(t, err)
		in.ExpectedVersion = begun.Run.Version
		_, err = f.repo.CommitFinalize(ctx, in)
		require.NoError(t, err)
		var log model.ExptRunLog
		require.NoError(t, f.sql.First(&log, create.Key.RunID).Error)
		require.Equal(t, "old-run", string(gptr.Indirect(log.StatusMessage)))
		var expt model.Experiment
		require.NoError(t, f.sql.First(&expt, f.expt).Error)
		require.Equal(t, "old-experiment", string(gptr.Indirect(expt.StatusMessage)))
	})
	t.Run("stats_without_display_input", func(t *testing.T) {
		f := settledFinalizationFixture(t)
		ctx := context.Background()
		require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("status_message", []byte("old-run")).Error)
		stats, err := NewHookFinalizationRepo(f.p).ReadFinalizationStats(ctx, f.key, "local")
		require.NoError(t, err)
		run, err := f.repo.GetRun(ctx, f.key)
		require.NoError(t, err)
		in := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Failed, Reason: "reason"}, Stats: stats}
		begun, err := f.repo.BeginFinalize(ctx, in)
		require.NoError(t, err)
		in.ExpectedVersion = begun.Run.Version
		_, err = f.repo.CommitFinalize(ctx, in)
		require.NoError(t, err)
		var log model.ExptRunLog
		require.NoError(t, f.sql.First(&log, f.key.RunID).Error)
		require.Equal(t, "reason", string(gptr.Indirect(log.StatusMessage)))
	})
}
