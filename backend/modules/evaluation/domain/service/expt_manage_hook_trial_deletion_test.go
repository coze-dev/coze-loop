// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

// Existing fixtures use opaque snapshots; these tests characterize persisted Trial recovery, not SPI decoding.
func persistTrialDeletionMode(t *testing.T, f *finalizationManagerFixture) {
	t.Helper()
	updated := f.sql.Model(&model.ExptRunLog{}).Where("id=? AND space_id=? AND expt_id=?", f.key.RunID, f.space, f.expt).
		UpdateColumn("mode", int32(entity.EvaluationModeTrialRun))
	require.NoError(t, updated.Error)
	require.EqualValues(t, 1, updated.RowsAffected)
	run := finalizationRead(t, f)
	require.Equal(t, entity.EvaluationModeTrialRun, run.Mode)
	source, err := f.deps.Repository.ReadFinalizationSource(context.Background(), f.key)
	require.NoError(t, err)
	require.Equal(t, int32(entity.EvaluationModeTrialRun), source.RunLog.Mode)
	t.Logf("persisted Trial: space=%d expt=%d run=%d mode=%d", f.space, f.expt, f.key.RunID, run.Mode)
}

func assertTrialDeletionIdentity(t *testing.T, before, after *entity.HookStoredRun) {
	t.Helper()
	require.Equal(t, entity.EvaluationModeTrialRun, after.Mode)
	require.Equal(t, before.Snapshot, after.Snapshot)
	require.Equal(t, before.CreatedBy, after.CreatedBy)
	require.Equal(t, before.SourceRunID, after.SourceRunID)
	require.Equal(t, before.PlanHash, after.PlanHash)
	require.Equal(t, before.PlanCount, after.PlanCount)
	require.Equal(t, before.PlanReady, after.PlanReady)
	require.Len(t, after.Operations, len(before.Operations))
	for i, op := range before.Operations {
		require.Equal(t, op.HookOperationSeed, after.Operations[i].HookOperationSeed)
		require.Equal(t, op.Phase, after.Operations[i].Phase)
	}
}

func assertTrialNoExecutionRows(t *testing.T, f *finalizationManagerFixture) {
	t.Helper()
	for _, table := range []any{&model.ExptItemResult{}, &model.ExptTurnResult{}, &model.ExptItemResultRunLog{}, &model.ExptTurnResultRunLog{}} {
		var count int64
		require.NoError(t, f.sql.Unscoped().Model(table).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.key.RunID).Count(&count).Error)
		require.Zero(t, count, "%T must not be fabricated by deletion/recovery", table)
	}
	var life model.ExptLifecycleRun
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&life).Error)
	require.False(t, life.ExecutionInitialized)
	require.False(t, life.ExecutionStarted)
}

// Catches Submit-only eligibility and recovery that invents execution or reactivates before.
func TestHookTrialDeletionManagerNoExecution(t *testing.T) {
	for _, tc := range []struct {
		name             string
		ready, batch     bool
		claim, succeeded bool
	}{
		{name: "preparing_Delete"},
		{name: "before_waiting_MDelete", ready: true, batch: true, claim: true},
		{name: "before_succeeded_Delete", ready: true, claim: true, succeeded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := neverAdmittedFinalizationFixture(t, tc.ready, tc.ready, "tx")
			persistTrialDeletionMode(t, f)
			ctx := context.Background()
			var completion entity.HookCompleteAttemptInput
			if tc.claim {
				completion = claimTerminationBefore(t, f)
				if tc.succeeded {
					_, err := f.repo.CompleteAttempt(ctx, completion)
					require.NoError(t, err)
				}
			}
			before := finalizationRead(t, f)
			if tc.succeeded {
				require.Equal(t, entity.HookOperationSucceeded, before.State.Before.Status)
				require.Equal(t, entity.HookGateReady, before.State.Gate)
			} else {
				require.Equal(t, entity.HookGateWaiting, before.State.Gate)
			}
			assertTrialNoExecutionRows(t, f)
			m, spy := deletionManager(t, f)
			if tc.batch {
				require.NoError(t, m.MDelete(ctx, []int64{f.expt, f.expt}, f.space, nil))
			} else {
				require.NoError(t, m.Delete(ctx, f.expt, f.space, nil))
			}
			pending := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
			require.Equal(t, entity.HookGateClosed, pending.State.Gate)
			require.Equal(t, entity.ExptStatus_Terminated, pending.State.Intent.Status)
			require.False(t, pending.State.After.Activated)
			assertTrialDeletionIdentity(t, before, pending)
			if tc.claim && !tc.succeeded {
				late, err := f.repo.CompleteAttempt(ctx, completion)
				require.NoError(t, err)
				require.True(t, late.Effects.LateIgnored)
				require.False(t, late.Changed)
			}
			require.NoError(t, m.MDelete(ctx, []int64{f.expt}, f.space, nil))
			require.Equal(t, pending, finalizationRead(t, f))
			require.Zero(t, spy.deletes)
			var parent model.Experiment
			require.NoError(t, f.sql.Unscoped().First(&parent, f.expt).Error)
			require.True(t, parent.DeletedAt.Valid)
			m.exptRepo = nil
			require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			done := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizeCommitted, done.State.Finalize)
			require.Equal(t, pending.State.Intent, done.State.Intent)
			require.Equal(t, entity.ExptStatus_Terminated, done.State.Status)
			require.True(t, done.State.After.Activated)
			assertTrialDeletionIdentity(t, before, done)
			assertTrialNoExecutionRows(t, f)
			var log model.ExptRunLog
			require.NoError(t, f.sql.First(&log, f.key.RunID).Error)
			require.Equal(t, int32(entity.EvaluationModeTrialRun), gptr.Indirect(log.Mode))
			require.Equal(t, int64(entity.ExptStatus_Terminated), gptr.Indirect(log.Status))
			require.Zero(t, log.PendingCnt+log.ProcessingCnt+log.SuccessCnt+log.FailCnt+log.TerminatedCnt)
			for i := 0; i < 2; i++ {
				require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
				require.Equal(t, done, finalizationRead(t, f))
			}
			var unchanged model.Experiment
			require.NoError(t, f.sql.Unscoped().First(&unchanged, f.expt).Error)
			require.Equal(t, parent, unchanged)
			q, err := dao.NewQuotaDAO(f.redis).GetQuotaSpaceExpt(ctx, f.space)
			require.NoError(t, err)
			require.NotContains(t, q.ExptID2RunTime, f.expt)
			require.Zero(t, f.redis.Exists(ctx, fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)).Val())
			require.Zero(t, f.notifications)
			t.Logf("Trial recovered: run=%d mode=%d finalize=%v after=%t execution_rows=0", f.key.RunID, done.Mode, done.State.Finalize, done.State.After.Activated)
		})
	}
}

// Catches Latest-only deletion, original-Run identity loss, and cleanup that overwrites a newer Run.
func TestHookTrialDeletionManagerInitializedOriginalAndLatest(t *testing.T) {
	for _, newer := range []bool{false, true} {
		t.Run(fmt.Sprintf("newer=%t", newer), func(t *testing.T) {
			f, manifests := activeTerminationFixture(t, entity.ItemRunState_Processing, entity.ItemRunState_Success, entity.ItemRunState_Queueing)
			persistTrialDeletionMode(t, f)
			ctx := context.Background()
			before := finalizationRead(t, f)
			var initialized model.ExptLifecycleRun
			require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&initialized).Error)
			require.True(t, initialized.ExecutionInitialized)
			require.True(t, initialized.ExecutionStarted)
			var nextKey entity.HookRunKey
			lockKey := fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)
			var nextOwner string
			if newer {
				nextKey = entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: finalizationTestIDs.Add(1)}
				_, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{Key: nextKey, ExpectedLatestRunID: f.key.RunID,
					RunLog:   &entity.ExptRunLog{ID: nextKey.RunID, ExptRunID: nextKey.RunID, SpaceID: f.space, ExptID: f.expt, Mode: int32(entity.EvaluationModeTrialRun), Status: int64(entity.ExptStatus_Processing), CreatedBy: "trial-successor"},
					Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{2}, KeyID: "key", Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ExecutionScope: "local"},
					Before:   &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprintf("trial-next-before-%d", nextKey.RunID), IdempotencyKey: fmt.Sprintf("trial-next-before-%d", nextKey.RunID)},
					After:    &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprintf("trial-next-after-%d", nextKey.RunID), IdempotencyKey: fmt.Sprintf("trial-next-after-%d", nextKey.RunID)}})
				require.NoError(t, err)
				nextOwner = fmt.Sprintf("hook_run:%d:abcdef0123456789abcdef0123456789", nextKey.RunID)
				require.NoError(t, f.redis.Set(ctx, lockKey, nextOwner, 0).Err())
			}
			m, _ := deletionManager(t, f)
			if newer {
				require.NoError(t, m.MDelete(ctx, []int64{f.expt, f.expt}, f.space, nil))
			} else {
				require.NoError(t, m.Delete(ctx, f.expt, f.space, nil))
			}
			var nextPending *entity.HookStoredRun
			var nextLog model.ExptRunLog
			if newer {
				var err error
				nextPending, err = f.repo.GetRun(ctx, nextKey)
				require.NoError(t, err)
				require.Equal(t, entity.EvaluationModeTrialRun, nextPending.Mode)
				require.Equal(t, entity.HookFinalizePending, nextPending.State.Finalize)
				require.NoError(t, f.sql.First(&nextLog, nextKey.RunID).Error)
			}
			var parent model.Experiment
			require.NoError(t, f.sql.Unscoped().First(&parent, f.expt).Error)
			require.True(t, parent.DeletedAt.Valid)
			if newer {
				require.Equal(t, nextKey.RunID, parent.LatestRunID)
			} else {
				require.Equal(t, f.key.RunID, parent.LatestRunID)
			}
			var stats model.ExptStats
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
			var projectedTurns []model.ExptTurnResult
			require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Order("id").Find(&projectedTurns).Error)
			var projectedItems []model.ExptItemResult
			require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Order("id").Find(&projectedItems).Error)
			m.exptRepo = nil
			require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			done := finalizationRead(t, f)
			assertTrialDeletionIdentity(t, before, done)
			require.Equal(t, entity.HookFinalizeCommitted, done.State.Finalize)
			require.True(t, done.State.After.Activated)
			require.Equal(t, entity.ExptStatus_Terminated, done.State.Status)
			for i, manifest := range manifests {
				wantItem, wantTurn := entity.ItemRunState_Terminal, entity.TurnRunState_Terminal
				if i == 1 {
					wantItem, wantTurn = entity.ItemRunState_Success, entity.TurnRunState_Success
				}
				var item model.ExptItemResultRunLog
				require.NoError(t, f.sql.First(&item, manifest.ItemRunLogID).Error)
				require.Equal(t, f.key.RunID, item.ExptRunID)
				require.Equal(t, int32(wantItem), item.Status)
				require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
				var turn model.ExptTurnResult
				require.NoError(t, f.sql.First(&turn, manifest.Turns[0].ResultID).Error)
				require.Equal(t, f.key.RunID, turn.ExptRunID)
				if !newer {
					require.Equal(t, int32(wantTurn), turn.Status)
					var projected model.ExptItemResult
					require.NoError(t, f.sql.First(&projected, manifest.ItemResultID).Error)
					require.Equal(t, int32(wantItem), projected.Status)
				}
				require.Equal(t, "original-log", turn.LogID)
				if manifest.Turns[0].RunLogID != 0 {
					var log model.ExptTurnResultRunLog
					require.NoError(t, f.sql.First(&log, manifest.Turns[0].RunLogID).Error)
					require.Equal(t, int32(wantTurn), log.Status)
					require.Equal(t, "original-log", log.LogID)
				}
			}
			var runLog model.ExptRunLog
			require.NoError(t, f.sql.First(&runLog, f.key.RunID).Error)
			require.Equal(t, int32(entity.EvaluationModeTrialRun), gptr.Indirect(runLog.Mode))
			require.EqualValues(t, 1, runLog.SuccessCnt)
			require.EqualValues(t, 2, runLog.TerminatedCnt)
			require.Zero(t, runLog.PendingCnt+runLog.ProcessingCnt+runLog.FailCnt)
			var turnLogCount int64
			require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Count(&turnLogCount).Error)
			require.EqualValues(t, 2, turnLogCount, "queued item must not acquire an execution log")
			if newer {
				var unchangedTurns []model.ExptTurnResult
				require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Order("id").Find(&unchangedTurns).Error)
				require.Equal(t, projectedTurns, unchangedTurns, "non-Latest recovery must not rewrite read-side turns")
				var unchangedItems []model.ExptItemResult
				require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Order("id").Find(&unchangedItems).Error)
				require.Equal(t, projectedItems, unchangedItems, "non-Latest recovery must not rewrite read-side items")
				nextUnchanged, err := f.repo.GetRun(ctx, nextKey)
				require.NoError(t, err)
				require.Equal(t, nextPending, nextUnchanged)
				var unchangedLog model.ExptRunLog
				require.NoError(t, f.sql.First(&unchangedLog, nextKey.RunID).Error)
				require.Equal(t, nextLog, unchangedLog)
				q, err := dao.NewQuotaDAO(f.redis).GetQuotaSpaceExpt(ctx, f.space)
				require.NoError(t, err)
				require.Contains(t, q.ExptID2RunTime, f.expt)
				require.Equal(t, nextOwner, f.redis.Get(ctx, lockKey).Val())
				require.NoError(t, m.FinalizeRun(ctx, nextKey, entity.HookTerminalIntent{}))
				nextDone, err := f.repo.GetRun(ctx, nextKey)
				require.NoError(t, err)
				require.Equal(t, entity.HookFinalizeCommitted, nextDone.State.Finalize)
				require.True(t, nextDone.State.After.Activated)
				require.NoError(t, m.FinalizeRun(ctx, nextKey, entity.HookTerminalIntent{}))
			}
			require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			require.Equal(t, done, finalizationRead(t, f))
			var unchangedParent model.Experiment
			require.NoError(t, f.sql.Unscoped().First(&unchangedParent, f.expt).Error)
			require.Equal(t, parent, unchangedParent)
			var unchangedStats model.ExptStats
			require.NoError(t, f.sql.First(&unchangedStats, stats.ID).Error)
			require.Equal(t, stats, unchangedStats)
			q, err := dao.NewQuotaDAO(f.redis).GetQuotaSpaceExpt(ctx, f.space)
			require.NoError(t, err)
			require.NotContains(t, q.ExptID2RunTime, f.expt)
			require.Zero(t, f.redis.Exists(ctx, lockKey).Val())
			require.Zero(t, f.notifications)
			t.Logf("Trial initialized cleanup: run=%d mode=%d success=%d terminated=%d turn_logs=%d newer=%t after=%t", f.key.RunID, done.Mode, runLog.SuccessCnt, runLog.TerminatedCnt, turnLogCount, newer, done.State.After.Activated)
		})
	}
}

// Catches deletion resetting an already active after lease, identity, or committed terminal decision.
func TestHookTrialDeletionManagerDuringAfter(t *testing.T) {
	f := neverAdmittedFinalizationFixture(t, true, true, "tx")
	persistTrialDeletionMode(t, f)
	ctx := context.Background()
	m, _ := deletionManager(t, f)
	require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}))
	done := finalizationRead(t, f)
	require.True(t, done.State.After.Activated)
	config := &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}, TimeoutSeconds: gptr.Of(int32(30))}
	in := entity.HookClaimAttemptInput{HookAttemptScope: entity.HookAttemptScope{Key: f.key, OperationID: done.State.After.ID, Phase: entity.HookPhaseAfter, SnapshotHash: done.Snapshot.Hash, ExecutionScope: "local"}, Owner: "trial-after-worker", AttemptID: finalizationTestIDs.Add(1), Config: config}
	for _, op := range done.Operations {
		if op.Phase == entity.HookPhaseAfter {
			in.ExpectedVersion = op.Version
		}
	}
	claim, err := f.repo.ClaimAttempt(ctx, in)
	require.NoError(t, err)
	require.NotNil(t, claim.Claim)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Where("operation_id=?", in.OperationID).Delete(&model.ExptLifecycleHookAttempt{}).Error)
	})
	claimed := finalizationRead(t, f)
	require.NoError(t, m.MDelete(ctx, []int64{f.expt, f.expt}, f.space, nil))
	require.Equal(t, claimed, finalizationRead(t, f))
	in.ExpectedVersion = claim.Claim.Version
	completed, err := f.repo.CompleteAttempt(ctx, entity.HookCompleteAttemptInput{HookRenewAttemptInput: entity.HookRenewAttemptInput{HookAttemptScope: in.HookAttemptScope, HookAttemptIdentity: claim.Claim.HookAttemptIdentity}, Config: config, CompletedAt: claim.Claim.StartedAt, Outcome: entity.HookOutcome{Code: entity.HookSucceeded}, ResultRedacted: []byte(`{"trial":"original-after"}`)})
	require.NoError(t, err)
	require.True(t, completed.Changed)
	require.False(t, completed.LatestProjected)
	require.Equal(t, entity.HookOperationSucceeded, completed.Run.State.After.Status)
	require.Equal(t, done.State.Intent, completed.Run.State.Intent)
	assertTrialDeletionIdentity(t, done, completed.Run)
	require.NoError(t, m.Delete(ctx, f.expt, f.space, nil))
	m.exptRepo = nil
	require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	require.Equal(t, completed.Run, finalizationRead(t, f))
	var afterRow model.ExptLifecycleHookRun
	require.NoError(t, f.sql.Where("operation_id=?", in.OperationID).First(&afterRow).Error)
	require.JSONEq(t, `{"trial":"original-after"}`, string(gptr.Indirect(afterRow.ResultRedacted)))
	var attempts int64
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookAttempt{}).Where("operation_id=?", in.OperationID).Count(&attempts).Error)
	require.EqualValues(t, 1, attempts)
	t.Logf("Trial after survived deletion: run=%d mode=%d operation=%s attempts=%d status=%v", f.key.RunID, completed.Run.Mode, in.OperationID, attempts, completed.Run.State.After.Status)
}
