// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestHookTxFinalizeFourStatuses(t *testing.T) {
	for _, status := range []entity.ExptStatus{11, 12, 13, 14} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := newHookTxFixture(t)
			ctx := context.Background()
			in := f.input(true, 0)
			in.RunLog.Mode = int32(entity.EvaluationModeSubmit)
			_, err := f.repo.CreateRunWithHooks(ctx, in)
			require.NoError(t, err)
			_, err = f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, Hash: strings.Repeat("b", 64)})
			require.NoError(t, err)
			require.NoError(t, f.sql.Exec("UPDATE expt_lifecycle_hook_run SET status='running',attempt=1,lease_generation=7,version=5,lease_owner='worker',lease_until=DATE_ADD(NOW(3),INTERVAL 30 SECOND),attempt_deadline=DATE_ADD(NOW(3),INTERVAL 180 SECOND),operation_deadline=DATE_ADD(NOW(3),INTERVAL 420 SECOND) WHERE space_id=? AND expt_id=? AND expt_run_id=? AND phase='before'", f.space, f.expt, in.Key.RunID).Error)
			request := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: 1}, Intent: entity.HookTerminalIntent{Status: status, Reason: "test-terminal"}}
			begun, err := f.repo.BeginFinalize(ctx, request)
			require.NoError(t, err)
			require.True(t, begun.Changed)
			require.True(t, begun.Effects.FenceBefore)
			require.Equal(t, entity.HookGateClosed, begun.Run.State.Gate)
			require.Equal(t, entity.HookFinalizePending, begun.Run.State.Finalize)
			require.Equal(t, entity.ExptStatus_Processing, begun.Run.State.Status)
			require.Equal(t, int64(8), begun.Run.State.Before.Generation)
			require.Equal(t, int32(1), begun.Run.State.Before.Attempt)
			require.True(t, begun.Run.State.Before.LeaseUntil.IsZero())
			require.False(t, begun.Run.State.After.Activated)
			require.NotNil(t, begun.Run.TerminalAt)
			require.NotNil(t, begun.Run.NextReconcileAt)
			repeat, err := f.repo.BeginFinalize(ctx, request)
			require.NoError(t, err)
			require.False(t, repeat.Changed)
			require.Equal(t, begun.Run, repeat.Run)
			conflict := request
			conflict.Intent.Reason = "different"
			_, err = f.repo.BeginFinalize(ctx, conflict)
			require.Error(t, err)
			request.ExpectedVersion = begun.Run.Version
			committed, err := f.repo.CommitFinalize(ctx, request)
			require.NoError(t, err)
			require.True(t, committed.Changed)
			require.True(t, committed.LatestProjected)
			require.True(t, committed.Effects.ActivateAfter)
			require.Equal(t, status, committed.Run.State.Status)
			require.Equal(t, entity.HookFinalizeCommitted, committed.Run.State.Finalize)
			require.True(t, committed.Run.State.After.Activated)
			require.Nil(t, committed.Run.NextReconcileAt)
			for _, op := range committed.Run.Operations {
				if op.Phase == entity.HookPhaseAfter {
					require.NotNil(t, op.ActivatedAt)
					require.Equal(t, op.ActivatedAt, op.OccurredAt)
					require.Equal(t, op.ActivatedAt, op.NextAttemptAt)
				}
			}
			repeat, err = f.repo.CommitFinalize(ctx, request)
			require.NoError(t, err)
			require.False(t, repeat.Changed)
			require.False(t, repeat.Effects.ActivateAfter)
			require.Equal(t, committed.Run, repeat.Run)
			var projection int32
			require.NoError(t, f.sql.Raw("SELECT status FROM experiment WHERE id=? AND space_id=?", f.expt, f.space).Scan(&projection).Error)
			require.Equal(t, int32(status), projection)
		})
	}
}

func TestHookTxAppendNormalFinalizeRequiresDraining(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Success, entity.ExptStatus_Failed} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := newHookTxFixture(t)
			ctx := context.Background()
			in := f.input(false, 0)
			in.RunLog.Mode = int32(entity.EvaluationModeAppend)
			initial, err := f.repo.CreateRunWithHooks(ctx, in)
			require.NoError(t, err)
			request := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, Intent: entity.HookTerminalIntent{Status: status}}
			_, err = f.repo.BeginFinalize(ctx, request)
			require.ErrorIs(t, err, entity.ErrHookFinalizationUnsettled)
			unchanged, err := f.repo.GetRun(ctx, in.Key)
			require.NoError(t, err)
			require.Equal(t, initial.Run, unchanged)
			require.NoError(t, hookRunScope(f.sql.Model(&model.ExptRunLog{}), in.Key).Where("id=?", in.Key.RunID).UpdateColumn("status", int64(entity.ExptStatus_Draining)).Error)
			begun, err := f.repo.BeginFinalize(ctx, request)
			require.NoError(t, err)
			require.True(t, begun.Changed)
			require.Equal(t, entity.HookFinalizePending, begun.Run.State.Finalize)
			require.False(t, begun.Run.State.After.Activated)
		})
	}
}

func TestHookTxFinalizeOldAndDeletedRun(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	old := f.input(false, 0)
	_, err := f.repo.CreateRunWithHooks(ctx, old)
	require.NoError(t, err)
	next := f.input(false, old.Key.RunID)
	fresh, err := f.repo.CreateRunWithHooks(ctx, next)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=? AND space_id=?", f.expt, f.space).UpdateColumn("deleted_at", time.Now().UTC()).Error)
	request := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: old.Key}, Intent: entity.HookTerminalIntent{Status: 13, Reason: "delete"}}
	_, err = f.repo.CommitFinalize(ctx, request)
	require.Error(t, err)
	begun, err := f.repo.BeginFinalize(ctx, request)
	require.NoError(t, err)
	request.ExpectedVersion = begun.Run.Version
	done, err := f.repo.CommitFinalize(ctx, request)
	require.NoError(t, err)
	require.False(t, done.LatestProjected)
	require.True(t, done.Effects.ActivateAfter)
	current, err := f.repo.GetRun(ctx, next.Key)
	require.NoError(t, err)
	require.Equal(t, fresh.Run, current)
	require.Equal(t, next.Key.RunID, f.latest(t))
	var expt model.Experiment
	require.NoError(t, f.sql.Unscoped().First(&expt, "id=? AND space_id=?", f.expt, f.space).Error)
	require.True(t, expt.DeletedAt.Valid)
	require.Equal(t, int32(3), expt.Status)
}

func TestHookTxFinalizeRollback(t *testing.T) {
	for _, stage := range []string{"begin", "commit"} {
		t.Run(stage, func(t *testing.T) {
			f := newHookTxFixture(t)
			ctx := context.Background()
			in := f.input(true, 0)
			initial, err := f.repo.CreateRunWithHooks(ctx, in)
			require.NoError(t, err)
			request := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, Intent: entity.HookTerminalIntent{Status: 13}}
			if stage == "commit" {
				initial, err = f.repo.BeginFinalize(ctx, request)
				require.NoError(t, err)
				request.ExpectedVersion = initial.Run.Version
			}
			trigger := fmt.Sprintf("hook_finalize_fail_%d", f.expt)
			require.NoError(t, f.sql.Exec(fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE ON expt_lifecycle_run FOR EACH ROW BEGIN IF NEW.expt_id=%d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='hook finalization failure'; END IF; END", trigger, f.expt)).Error)
			t.Cleanup(func() { require.NoError(t, f.sql.Exec("DROP TRIGGER "+trigger).Error) })
			var got entity.HookStoreResult
			if stage == "begin" {
				got, err = f.repo.BeginFinalize(ctx, request)
			} else {
				got, err = f.repo.CommitFinalize(ctx, request)
			}
			require.ErrorContains(t, err, "hook finalization failure")
			require.Nil(t, got.Run)
			read, err := f.repo.GetRun(ctx, in.Key)
			require.NoError(t, err)
			require.Equal(t, initial.Run, read)
			var status int32
			require.NoError(t, f.sql.Raw("SELECT status FROM experiment WHERE id=? AND space_id=?", f.expt, f.space).Scan(&status).Error)
			require.Equal(t, int32(3), status)
		})
	}
}
