// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	cm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

func neverAdmittedFinalizationFixture(t *testing.T, ready, populated bool, database ...string) *finalizationManagerFixture {
	t.Helper()
	f := newFinalizationManagerFixture(t, database...)
	for _, table := range []string{"expt_item_result_run_log", "expt_turn_result_run_log", "expt_item_result", "expt_turn_result"} {
		require.NoError(t, f.sql.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_run_id=?", f.space, f.key.RunID).Error)
	}
	fields := map[string]any{"execution_initialized": false, "execution_started": false, "before_enabled": true, "gate": 0}
	opID := fmt.Sprintf("before-%d", finalizationTestIDs.Add(1))
	before := &model.ExptLifecycleHookRun{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID,
		Phase: "before", Status: "pending", OperationID: opID, IdempotencyKey: opID, ExecutionScope: "local"}
	if ready {
		now := time.Now().UTC().Add(-time.Second).Truncate(time.Millisecond)
		before.ActivatedAt, before.OccurredAt, before.NextAttemptAt = &now, &now, &now
	}
	require.NoError(t, f.sql.Create(before).Error)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Exec("DELETE FROM expt_lifecycle_hook_attempt WHERE operation_id=?", opID).Error)
	})
	if populated {
		require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("execution_manifest", nil).Error)
	} else {
		require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Delete(&model.ExptLifecycleRunItem{}).Error)
		fields["plan_count"] = 0
		fields["plan_hash"] = entity.NewHookPlanDigest().Hash
	}
	if !ready {
		fields["plan_state"], fields["plan_hash"] = 0, nil
	}
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumns(fields).Error)
	return f
}

// Missing termination support must fail, not silently infer Success from zero records.
func TestHookTerminationNeverAdmitted(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated} {
		for _, ready := range []bool{false, true} {
			for _, populated := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/ready=%t/populated=%t", status, ready, populated), func(t *testing.T) {
					f := neverAdmittedFinalizationFixture(t, ready, populated)
					ctx := context.Background()
					before := finalizationRead(t, f)
					intent := entity.HookTerminalIntent{Status: status}
					require.NoError(t, f.manager.FinalizeRun(ctx, f.key, intent))
					done := finalizationRead(t, f)
					require.Equal(t, intent, done.State.Intent)
					require.Equal(t, entity.HookFinalizeCommitted, done.State.Finalize)
					require.True(t, done.State.After.Activated)
					require.Equal(t, before.PlanReady, done.PlanReady)
					require.Equal(t, before.PlanCount, done.PlanCount)
					var life model.ExptLifecycleRun
					require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&life).Error)
					require.False(t, life.ExecutionInitialized)
					require.False(t, life.ExecutionStarted)
					var log model.ExptRunLog
					require.NoError(t, f.sql.First(&log, f.key.RunID).Error)
					require.Equal(t, int64(status), *log.Status)
					require.Zero(t, log.PendingCnt+log.ProcessingCnt+log.SuccessCnt+log.FailCnt+log.TerminatedCnt)
					var stats model.ExptStats
					require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
					require.Zero(t, stats.PendingCnt+stats.SuccessCnt+stats.FailCnt+stats.ProcessingCnt+stats.TerminatedCnt)
					q, err := dao.NewQuotaDAO(f.redis).GetQuotaSpaceExpt(ctx, f.space)
					require.NoError(t, err)
					require.NotContains(t, q.ExptID2RunTime, f.expt)
					require.Zero(t, f.redis.Exists(ctx, fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)).Val())
					require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
					require.Equal(t, done, finalizationRead(t, f))
					require.Equal(t, 1, f.notifications)
					require.ErrorIs(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Success}), entity.ErrHookStoreConflict)
				})
			}
		}
	}
}

func TestHookTerminationExistingExecutionRetainsIntent(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := newFinalizationManagerFixture(t)
			ctx := context.Background()
			intent := entity.HookTerminalIntent{Status: status}
			require.ErrorIs(t, f.manager.FinalizeRun(ctx, f.key, intent), entity.ErrHookFinalizationUnsettled)
			pending := finalizationRead(t, f)
			require.Equal(t, intent, pending.State.Intent)
			require.Equal(t, entity.HookGateClosed, pending.State.Gate)
			require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
			require.False(t, pending.State.After.Activated)
			require.NotNil(t, pending.NextReconcileAt)
			require.ErrorIs(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}), entity.ErrHookFinalizationUnsettled)
			require.Equal(t, pending, finalizationRead(t, f))
			require.Zero(t, f.notifications)
		})
	}
}

func TestHookTerminationCleanupFailureRecovery(t *testing.T) {
	f := neverAdmittedFinalizationFixture(t, false, false)
	q := &finalizationQuotaFault{QuotaRepo: f.quota, fail: true}
	f.base.(*ExptMangerImpl).quotaRepo = q
	finalizationRecreate(t, f)
	ctx := context.Background()
	require.ErrorContains(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}), "quota unavailable")
	pending := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
	require.False(t, pending.State.After.Activated)
	require.Equal(t, entity.ExptStatus_Processing, pending.State.Status)
	q.fail = false
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	require.Equal(t, entity.HookFinalizeCommitted, finalizationRead(t, f).State.Finalize)
}

func TestHookTerminationBeforeFailedRecovery(t *testing.T) {
	for _, populated := range []bool{false, true} {
		t.Run(fmt.Sprint(populated), func(t *testing.T) {
			f := neverAdmittedFinalizationFixture(t, true, populated)
			ctx := context.Background()
			complete := claimTerminationBefore(t, f)
			complete.Outcome = entity.HookOutcome{Code: entity.HookFailed}
			failed, err := f.repo.CompleteAttempt(ctx, complete)
			require.NoError(t, err)
			intent := entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "HOOK_BEFORE_FAILED"}
			require.Equal(t, intent, failed.Run.State.Intent)
			require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			done := finalizationRead(t, f)
			require.Equal(t, intent, done.State.Intent)
			require.Equal(t, entity.HookFinalizeCommitted, done.State.Finalize)
			var expt model.Experiment
			require.NoError(t, f.sql.First(&expt, f.expt).Error)
			require.Empty(t, string(gptr.Indirect(expt.StatusMessage)), "platform reason is not display text")
		})
	}
}

func claimTerminationBefore(t *testing.T, f *finalizationManagerFixture) entity.HookCompleteAttemptInput {
	t.Helper()
	run := finalizationRead(t, f)
	scope := entity.HookAttemptScope{Key: f.key, OperationID: run.State.Before.ID, Phase: entity.HookPhaseBefore, ExecutionScope: "local", SnapshotHash: run.Snapshot.Hash}
	config := &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}, TimeoutSeconds: gptr.Of(int32(30))}
	claimed, err := f.repo.ClaimAttempt(context.Background(), entity.HookClaimAttemptInput{HookAttemptScope: scope, Owner: "test-worker", AttemptID: finalizationTestIDs.Add(1), Config: config})
	require.NoError(t, err)
	require.NotNil(t, claimed.Claim)
	scope.ExpectedVersion = claimed.Claim.Version
	return entity.HookCompleteAttemptInput{HookRenewAttemptInput: entity.HookRenewAttemptInput{HookAttemptScope: scope, HookAttemptIdentity: claimed.Claim.HookAttemptIdentity},
		Config: config, Outcome: entity.HookOutcome{Code: entity.HookSucceeded}, CompletedAt: claimed.Claim.StartedAt}
}

func TestHookTerminationFencesBeforeAndLateCompletion(t *testing.T) {
	f := neverAdmittedFinalizationFixture(t, true, true)
	ctx := context.Background()
	complete := claimTerminationBefore(t, f)
	q := &finalizationQuotaFault{QuotaRepo: f.quota, fail: true}
	f.base.(*ExptMangerImpl).quotaRepo = q
	finalizationRecreate(t, f)
	require.ErrorContains(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}), "quota unavailable")
	pending := finalizationRead(t, f)
	require.Equal(t, complete.Token.Generation+1, pending.State.Before.Generation)
	require.True(t, pending.State.Before.LeaseUntil.IsZero())
	require.Equal(t, entity.HookOperationFailed, pending.State.Before.Status)
	late, err := f.repo.CompleteAttempt(ctx, complete)
	require.NoError(t, err)
	require.True(t, late.Effects.LateIgnored)
	require.Equal(t, pending, late.Run)
	var item model.ExptLifecycleRunItem
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&item).Error)
	_, err = f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: pending.Version}, ItemID: item.ItemID})
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	q.fail = false
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	done := finalizationRead(t, f)
	late, err = f.repo.CompleteAttempt(ctx, complete)
	require.NoError(t, err)
	require.True(t, late.Effects.LateIgnored)
	require.Equal(t, done, late.Run)
}

func TestHookTerminationAfterOnlyNoExecutionCommits(t *testing.T) {
	f := neverAdmittedFinalizationFixture(t, false, false)
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=? AND phase='before'", f.space, f.key.RunID).Delete(&model.ExptLifecycleHookRun{}).Error)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumns(map[string]any{"before_enabled": false, "gate": 1}).Error)
	require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}))
	done := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, done.State.Finalize)
	require.True(t, done.State.After.Activated)
	require.Equal(t, entity.HookOperationDisabled, done.State.Before.Status)
}

func TestHookTerminationMultiSetRemainsPending(t *testing.T) {
	f := neverAdmittedFinalizationFixture(t, false, false)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("eval_set_source_type", 2).Error)
	require.ErrorIs(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}), entity.ErrHookFinalizationUnsettled)
	require.Equal(t, entity.HookFinalizePending, finalizationRead(t, f).State.Finalize)
}

func TestHookTerminationNewerRunUntouched(t *testing.T) {
	f := neverAdmittedFinalizationFixture(t, true, true)
	ctx := context.Background()
	next := finalizationTestIDs.Add(1)
	key := entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: next}
	fresh, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{Key: key, ExpectedLatestRunID: f.key.RunID,
		RunLog:   &entity.ExptRunLog{ID: next, ExptRunID: next, SpaceID: f.space, ExptID: f.expt, Mode: 1, Status: 3, CreatedBy: "user"},
		Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1}, Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", KeyID: "key", ExecutionScope: "local"},
		After:    &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(next), IdempotencyKey: fmt.Sprint(next)}})
	require.NoError(t, err)
	owner := fmt.Sprintf("hook_run:%d:abcdef0123456789abcdef0123456789", next)
	lockKey := fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)
	require.NoError(t, f.redis.Set(ctx, lockKey, owner, time.Hour).Err())
	require.NoError(t, f.quota.CreateOrUpdate(ctx, f.space, func(q *entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error) {
		q.ExptID2RunTime[f.expt] = 789
		return q, true, nil
	}, nil))
	var projection model.Experiment
	require.NoError(t, f.sql.First(&projection, f.expt).Error)
	var stats model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}))
	done := finalizationRead(t, f)
	require.True(t, done.State.After.Activated)
	got, err := f.repo.GetRun(ctx, key)
	require.NoError(t, err)
	require.Equal(t, fresh.Run, got)
	var after model.Experiment
	require.NoError(t, f.sql.First(&after, f.expt).Error)
	require.Equal(t, projection, after)
	var afterStats model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&afterStats).Error)
	require.Equal(t, stats, afterStats)
	q, err := dao.NewQuotaDAO(f.redis).GetQuotaSpaceExpt(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, int64(789), q.ExptID2RunTime[f.expt])
	require.Equal(t, owner, f.redis.Get(ctx, lockKey).Val())
	require.Zero(t, f.notifications)
}

type terminationCommitProbe struct {
	repo.IHookRepo
	beforeCommit func()
}

func (p terminationCommitProbe) CommitFinalize(ctx context.Context, in entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	p.beforeCommit()
	return p.IHookRepo.CommitFinalize(ctx, in)
}

func TestHookTerminationCommitRechecksAbsentExecution(t *testing.T) {
	f := neverAdmittedFinalizationFixture(t, true, true)
	f.deps.Runs = terminationCommitProbe{IHookRepo: f.repo, beforeCommit: func() {
		require.NoError(t, f.sql.Create(&model.ExptItemResultRunLog{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: 998, Status: 2}).Error)
	}}
	finalizationRecreate(t, f)
	require.ErrorIs(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}), entity.ErrHookFinalizationUnsettled)
	pending := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
	require.False(t, pending.State.After.Activated)
}

func TestHookTerminationCentralReservationCleanup(t *testing.T) {
	f := neverAdmittedFinalizationFixture(t, true, true)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"scheduler_mode": "enforce", "scheduler_scope": "local-scheduler"}).Error)
	var item model.ExptLifecycleRunItem
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&item).Error)
	guard := cm.NewMockICentralReservationGuard(gomock.NewController(t))
	f.base.(*ExptMangerImpl).centralGuard = guard
	finalizationRecreate(t, f)
	guard.EXPECT().Release(gomock.Any(), "local-scheduler", f.key.RunID, item.ItemID, gomock.Any()).Return(errors.New("reservation release unavailable"))
	require.ErrorContains(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}), "reservation release unavailable")
	pending := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
	require.False(t, pending.State.After.Activated)
	guard.EXPECT().Release(gomock.Any(), "local-scheduler", f.key.RunID, item.ItemID, gomock.Any()).Return(nil)
	require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
	require.True(t, finalizationRead(t, f).State.After.Activated)
}

func TestHookTerminationRejectsOrphansAndContradictions(t *testing.T) {
	for _, kind := range []string{"item-log", "turn-log", "item-result", "turn-result", "admitted", "manifest", "started", "count", "hash"} {
		t.Run(kind, func(t *testing.T) {
			f := neverAdmittedFinalizationFixture(t, true, true)
			scope := "space_id=? AND expt_run_id=?"
			switch kind {
			case "item-log":
				require.NoError(t, f.sql.Create(&model.ExptItemResultRunLog{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: 998, Status: 2}).Error)
			case "turn-log":
				require.NoError(t, f.sql.Create(&model.ExptTurnResultRunLog{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: 998, Status: 1}).Error)
			case "item-result":
				require.NoError(t, f.sql.Create(&model.ExptItemResult{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: 998, Status: 2}).Error)
			case "turn-result":
				require.NoError(t, f.sql.Create(&model.ExptTurnResult{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: 998, Status: 1}).Error)
			case "admitted":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where(scope, f.space, f.key.RunID).UpdateColumn("admitted_at", time.Now()).Error)
			case "manifest":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where(scope, f.space, f.key.RunID).UpdateColumn("execution_manifest", []byte("{} ")).Error)
			case "started":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where(scope, f.space, f.key.RunID).UpdateColumn("execution_started", true).Error)
			case "count":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where(scope, f.space, f.key.RunID).UpdateColumn("plan_count", 0).Error)
			case "hash":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where(scope, f.space, f.key.RunID).UpdateColumn("plan_hash", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb").Error)
			}
			err := f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated})
			require.Error(t, err)
			pending := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
			require.False(t, pending.State.After.Activated)
			require.Zero(t, f.notifications)
		})
	}
}
