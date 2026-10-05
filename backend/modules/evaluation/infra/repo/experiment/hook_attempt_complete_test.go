// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func hookLeaseInput(in entity.HookClaimAttemptInput, claim *entity.HookAttemptClaim) entity.HookRenewAttemptInput {
	scope := in.HookAttemptScope
	scope.ExpectedVersion = claim.Version
	return entity.HookRenewAttemptInput{HookAttemptScope: scope, HookAttemptIdentity: claim.HookAttemptIdentity}
}

func hookCompletion(t *testing.T, f *hookTxFixture, in entity.HookClaimAttemptInput, claim *entity.HookAttemptClaim) entity.HookCompleteAttemptInput {
	t.Helper()
	now, err := hookDBNow(f.sql)
	require.NoError(t, err)
	return entity.HookCompleteAttemptInput{HookRenewAttemptInput: hookLeaseInput(in, claim), Config: in.Config, Outcome: entity.HookOutcome{Code: entity.HookSucceeded}, CompletedAt: now, ResultRedacted: []byte(`{"ok":"redacted"}`)}
}

func TestHookAttemptCompletePoliciesAndDuplicate(t *testing.T) {
	for _, name := range []string{"success", "block", "continue", "retry", "after"} {
		t.Run(name, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := hookAttemptInput(t, f, name != "after")
			if name == "continue" {
				in.Config.OnFailure = gptr.Of(entity.HookFailurePolicyContinue)
			}
			got, err := f.repo.ClaimAttempt(context.Background(), in)
			require.NoError(t, err)
			complete := hookCompletion(t, f, in, got.Claim)
			if name == "block" || name == "continue" {
				complete.Outcome = entity.HookOutcome{Code: entity.HookFailed}
			}
			if name == "retry" {
				complete.Outcome = entity.HookOutcome{Code: entity.HookTransportError}
			}
			result, err := f.repo.CompleteAttempt(context.Background(), complete)
			require.NoError(t, err)
			require.True(t, result.Changed)
			op := hookAttemptRow(t, f, in.OperationID)
			audit := hookAttemptAudit(t, f, in.AttemptID)
			require.Equal(t, complete.CompletedAt, *audit.FinishedAt)
			require.Equal(t, string(complete.Outcome.Code), *audit.ResultCategory)
			require.Equal(t, int32(1), op.Attempt)
			switch name {
			case "success":
				require.Equal(t, "succeeded", op.Status)
				require.Nil(t, op.ErrorCode)
				require.Nil(t, op.ErrorMessage)
				require.Equal(t, entity.HookGateReady, result.Run.State.Gate)
			case "block":
				require.Equal(t, "failed", op.Status)
				require.Equal(t, entity.HookFinalizePending, result.Run.State.Finalize)
				require.Equal(t, "HOOK_BEFORE_FAILED", result.Run.State.Intent.Reason)
				require.NotNil(t, result.Run.NextReconcileAt)
			case "continue":
				require.Equal(t, "failed", op.Status)
				require.Equal(t, entity.HookGateReady, result.Run.State.Gate)
				require.Equal(t, entity.HookFinalizeNone, result.Run.State.Finalize)
			case "retry":
				require.Equal(t, "retry_wait", op.Status)
				require.NotNil(t, op.NextAttemptAt)
				require.True(t, result.Effects.Retry.Retry)
			case "after":
				require.Equal(t, entity.HookFinalizeCommitted, result.Run.State.Finalize)
				require.False(t, result.LatestProjected)
			}
			duplicate := complete
			duplicate.Outcome = entity.HookOutcome{Code: entity.HookFailed}
			duplicate.ResultRedacted = []byte("overwrite")
			replay, err := f.repo.CompleteAttempt(context.Background(), duplicate)
			require.NoError(t, err)
			require.False(t, replay.Changed)
			require.Equal(t, op, hookAttemptRow(t, f, in.OperationID))
			require.Equal(t, audit, hookAttemptAudit(t, f, in.AttemptID))
		})
	}
}

func TestHookAttemptCompleteWrongIdentityDoesNotPolluteAudit(t *testing.T) {
	for _, name := range []string{"owner", "delivery", "generation", "attempt", "run", "operation", "phase"} {
		t.Run(name, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := hookAttemptInput(t, f, true)
			got, err := f.repo.ClaimAttempt(context.Background(), in)
			require.NoError(t, err)
			complete := hookCompletion(t, f, in, got.Claim)
			audit := hookAttemptAudit(t, f, in.AttemptID)
			op := hookAttemptRow(t, f, in.OperationID)
			switch name {
			case "owner":
				complete.Owner = "other"
			case "delivery":
				complete.DeliveryID = "other"
			case "generation":
				complete.Token.Generation++
			case "attempt":
				complete.Token.Attempt++
			case "run":
				complete.Token.Run.RunID++
			case "operation":
				complete.Token.OperationID = "other"
			case "phase":
				complete.Token.Phase = entity.HookPhaseAfter
			}
			_, err = f.repo.CompleteAttempt(context.Background(), complete)
			require.Error(t, err)
			require.Equal(t, audit, hookAttemptAudit(t, f, in.AttemptID))
			require.Equal(t, op, hookAttemptRow(t, f, in.OperationID))
		})
	}
}

func TestHookAttemptCompleteBeforeDeadlineAfterDeadlineCommit(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	got, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	complete := hookCompletion(t, f, in, got.Claim)
	// DB fixtures shorten the deadline while keeping the original completion strictly earlier.
	complete.CompletedAt = complete.CompletedAt.Add(-time.Second)
	now, err := hookDBNow(f.sql)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumns(map[string]any{"attempt_deadline": now.Add(-500 * time.Millisecond), "lease_until": now.Add(4 * time.Second)}).Error)
	result, err := f.repo.CompleteAttempt(context.Background(), complete)
	require.NoError(t, err)
	require.True(t, result.Changed)
	require.Equal(t, "succeeded", hookAttemptRow(t, f, in.OperationID).Status)
	require.Equal(t, complete.CompletedAt, *hookAttemptAudit(t, f, in.AttemptID).FinishedAt)
}

func TestHookAttemptCompleteCancelledOnlyMarksOriginalAudit(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	got, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	complete := hookCompletion(t, f, in, got.Claim)
	run, err := f.repo.GetRun(context.Background(), in.Key)
	require.NoError(t, err)
	cancelled, err := f.repo.BeginFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}})
	require.NoError(t, err)
	result, err := f.repo.CompleteAttempt(context.Background(), complete)
	require.NoError(t, err)
	require.True(t, result.Effects.LateIgnored)
	require.Equal(t, cancelled.Run, result.Run)
	audit := hookAttemptAudit(t, f, in.AttemptID)
	require.True(t, audit.LateIgnored)
	require.Nil(t, audit.ResultCategory)
	require.Nil(t, audit.FinishedAt)
}
