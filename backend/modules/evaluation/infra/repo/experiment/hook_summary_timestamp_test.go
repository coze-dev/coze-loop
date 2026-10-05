// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func hookSummaryAssertUpdated(t *testing.T, f *hookTxFixture, operation string, action func()) {
	t.Helper()
	old := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", operation).UpdateColumn("updated_at", old).Error)
	start, err := hookDBNow(f.sql)
	require.NoError(t, err)
	action()
	end, err := hookDBNow(f.sql)
	require.NoError(t, err)
	row := hookAttemptRow(t, f, operation)
	require.False(t, row.UpdatedAt.Before(start), "mutation must advance the operation's persisted time")
	require.False(t, row.UpdatedAt.After(end), "timestamp must not use a future retry or lease time")
	key := entity.HookRunKey{WorkspaceID: row.SpaceID, ExperimentID: row.ExptID, RunID: row.ExptRunID}
	for i := 0; i < 2; i++ {
		got := hookSummaryRead(t, f, key)[key]
		phase := got.Before
		if row.Phase == "after" {
			phase = got.After
		}
		require.NotNil(t, phase.UpdatedAt)
		require.Equal(t, row.UpdatedAt, *phase.UpdatedAt)
	}
	require.Equal(t, row, hookAttemptRow(t, f, operation), "reads do not mutate operations")
}

func TestHookSummaryCreationTimestampUsesTransactionClock(t *testing.T) {
	f := newHookTxFixture(t)
	// Separate the sampled transaction time from each insert's default clock.
	const callback = "summary_create_latency"
	require.NoError(t, f.sql.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_lifecycle_hook_run" {
			time.Sleep(5 * time.Millisecond)
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Create().Remove(callback)) })
	in := f.input(true, 0)
	_, err := f.repo.CreateRunWithHooks(context.Background(), in)
	require.NoError(t, err)
	var life model.ExptLifecycleRun
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, in.Key.RunID).First(&life).Error)
	require.Equal(t, life.CreatedAt, hookAttemptRow(t, f, in.Before.OperationID).UpdatedAt)
	require.Equal(t, life.CreatedAt, hookAttemptRow(t, f, in.After.OperationID).UpdatedAt)
}

func TestHookSummaryTimestampClaimRenewComplete(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	var claimed entity.HookAttemptStoreResult
	hookSummaryAssertUpdated(t, f, in.OperationID, func() {
		var err error
		claimed, err = f.repo.ClaimAttempt(context.Background(), in)
		require.NoError(t, err)
	})
	hookSummaryAssertUpdated(t, f, in.OperationID, func() {
		var err error
		claimed, err = f.repo.RenewAttempt(context.Background(), hookLeaseInput(in, claimed.Claim))
		require.NoError(t, err)
	})
	completion := hookCompletion(t, f, in, claimed.Claim)
	hookSummaryAssertUpdated(t, f, in.OperationID, func() {
		_, err := f.repo.CompleteAttempt(context.Background(), completion)
		require.NoError(t, err)
	})
	before := hookAttemptRow(t, f, in.OperationID)
	_, err := f.repo.CompleteAttempt(context.Background(), completion)
	require.NoError(t, err)
	require.Equal(t, before, hookAttemptRow(t, f, in.OperationID))
}

func TestHookSummaryTimestampRecoveryAndLateAudit(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	claimed, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	completion := hookCompletion(t, f, in, claimed.Claim)
	now, err := hookDBNow(f.sql)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumns(map[string]any{"lease_until": now.Add(-time.Second), "attempt_deadline": now.Add(-time.Second)}).Error)
	recovery := entity.HookRecoverExpiredAttemptInput{HookAttemptScope: in.HookAttemptScope, Config: in.Config}
	recovery.ExpectedVersion = claimed.Claim.Version
	hookSummaryAssertUpdated(t, f, in.OperationID, func() {
		out, err := f.repo.RecoverExpiredAttempt(context.Background(), recovery)
		require.NoError(t, err)
		require.True(t, out.Changed)
	})
	before := hookAttemptRow(t, f, in.OperationID)
	require.Equal(t, "retry_wait", before.Status)
	require.True(t, before.NextAttemptAt.After(before.UpdatedAt))
	out, err := f.repo.CompleteAttempt(context.Background(), completion)
	require.NoError(t, err)
	require.True(t, out.Effects.LateIgnored)
	require.Equal(t, before, hookAttemptRow(t, f, in.OperationID))
	require.True(t, hookAttemptAudit(t, f, in.AttemptID).LateIgnored)
}

func TestHookSummaryTimestampFinishPlanAndFinalize(t *testing.T) {
	f := newHookTxFixture(t)
	in := f.input(true, 0)
	run, err := f.repo.CreateRunWithHooks(context.Background(), in)
	require.NoError(t, err)
	hookSummaryAssertUpdated(t, f, in.Before.OperationID, func() {
		run, err = f.repo.FinishPlan(context.Background(), entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: run.Run.Version}, Count: 0, Hash: in.Snapshot.Hash})
		require.NoError(t, err)
	})
	intent := entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}
	hookSummaryAssertUpdated(t, f, in.Before.OperationID, func() {
		run, err = f.repo.BeginFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: run.Run.Version}, Intent: intent})
		require.NoError(t, err)
	})
	hookSummaryAssertUpdated(t, f, in.After.OperationID, func() {
		run, err = f.repo.CommitFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: run.Run.Version}, Intent: intent})
		require.NoError(t, err)
	})
	require.NotEmpty(t, hookSummaryRead(t, f, in.Key)[in.Key].Before.Error.GetMessage())
}
