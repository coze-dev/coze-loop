// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func settledFinalizationFixture(t *testing.T) *executionFixture {
	return settledFinalizationFixtureCount(t, 2)
}

func settledFinalizationFixtureCount(t *testing.T, count int) *executionFixture {
	t.Helper()
	f := newExecutionFixture(t, count)
	ctx := context.Background()
	page, err := f.init.ReadExecutionInitializationPage(ctx, f.readInput())
	require.NoError(t, err)
	for start := 0; start < count; start += 100 {
		in := f.writeInput(page.RunVersion)
		in.StartOrdinal = int64(start)
		in.Items = f.manifests[start:min(start+100, count)]
		page, err = f.init.WriteExecutionInitializationPage(ctx, in)
		require.NoError(t, err)
	}
	_, err = f.init.CompleteExecutionInitialization(ctx, f.completeInput(page.RunVersion))
	require.NoError(t, err)
	initializer := NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil }).(repo.IHookTurnLogInitializer)
	for i, m := range f.manifests {
		state, err := f.repo.GetRun(ctx, f.key)
		require.NoError(t, err)
		_, err = f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: state.Version}, ItemID: m.Frozen.ItemID})
		require.NoError(t, err)
		var candidates []*entity.ExptTurnResultRunLog
		for _, tr := range m.Turns {
			candidates = append(candidates, &entity.ExptTurnResultRunLog{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, TurnID: tr.TurnID, Status: entity.TurnRunState_Processing})
		}
		handled, _, err := initializer.InitializeHookTurnRunLogs(ctx, f.key, m.Frozen.ItemID, m.Frozen.ItemVersionID, candidates)
		require.NoError(t, err)
		require.True(t, handled)
		var ledger model.ExptLifecycleRunItem
		require.NoError(t, f.sql.First(&ledger, m.Frozen.ID).Error)
		require.NotNil(t, ledger.ExecutionManifest)
		require.NoError(t, json.Unmarshal(*ledger.ExecutionManifest, &f.manifests[i]))
	}
	require.NoError(t, hookRunScope(f.sql.Model(&model.ExptItemResultRunLog{}), f.key).UpdateColumn("status", int32(entity.ItemRunState_Success)).Error)
	require.NoError(t, hookRunScope(f.sql.Model(&model.ExptTurnResultRunLog{}), f.key).UpdateColumn("status", int32(entity.TurnRunState_Success)).Error)
	require.NoError(t, hookRunScope(f.sql.Model(&model.ExptTurnResult{}), f.key).UpdateColumn("status", int32(entity.TurnRunState_Success)).Error)
	require.NoError(t, hookRunScope(f.sql.Model(&model.ExptItemResult{}), f.key).UpdateColumn("status", int32(entity.ItemRunState_Success)).Error)
	require.NoError(t, hookRunScope(f.sql.Model(&model.ExptItemResultRunLog{}), f.key).UpdateColumn("result_state", int32(entity.ExptItemResultStateResulted)).Error)
	return f
}

func TestHookFinalizationStatsAtomicOriginalRun(t *testing.T) {
	for _, superseded := range []bool{false, true} {
		name := "latest"
		if superseded {
			name = "superseded"
		}
		t.Run(name, func(t *testing.T) {
			f := settledFinalizationFixture(t)
			ctx := context.Background()
			state, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			in := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: state.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Success}}
			begun, err := f.repo.BeginFinalize(ctx, in)
			require.NoError(t, err)
			in.ExpectedVersion = begun.Run.Version
			// Literal wire-shaped input lets this test exercise CommitFinalize before the optional field exists.
			raw, err := json.Marshal(map[string]any{"Stats": map[string]any{"Key": f.key, "ExecutionScope": "local", "Items": map[string]int{"Success": 2}, "Turns": map[string]int{"Success": 4}}})
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, &in))
			if superseded {
				next := f.input(false, f.key.RunID)
				_, err = f.repo.CreateRunWithHooks(ctx, next)
				require.NoError(t, err)
				require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumns(map[string]any{"pending_cnt": 99, "success_cnt": 0}).Error)
			}
			result, err := f.repo.CommitFinalize(ctx, in)
			require.NoError(t, err)
			require.True(t, result.Effects.ActivateAfter)
			var log model.ExptRunLog
			require.NoError(t, f.sql.First(&log, f.key.RunID).Error)
			require.Equal(t, int32(4), log.SuccessCnt, "original Run terminal counts must commit with after")
			var stats model.ExptStats
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
			if superseded {
				require.Equal(t, int32(99), stats.PendingCnt)
				require.Zero(t, stats.SuccessCnt)
			} else {
				require.Equal(t, int32(2), stats.SuccessCnt)
				require.Zero(t, stats.PendingCnt)
			}
			replay, err := f.repo.CommitFinalize(ctx, in)
			require.NoError(t, err)
			require.False(t, replay.Changed)
			require.False(t, replay.Effects.ActivateAfter)
		})
	}
}
