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

func TestHookAttemptRecoveryEarlyLateAndBudget(t *testing.T) {
	for _, name := range []string{"early", "late", "exhausted block", "exhausted continue"} {
		t.Run(name, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := hookAttemptInput(t, f, true)
			if name == "exhausted continue" {
				in.Config.OnFailure = gptr.Of(entity.HookFailurePolicyContinue)
			}
			claimed, err := f.repo.ClaimAttempt(context.Background(), in)
			require.NoError(t, err)
			oldResponse := hookCompletion(t, f, in, claimed.Claim)
			now, err := hookDBNow(f.sql)
			require.NoError(t, err)
			fields := map[string]any{"lease_until": now.Add(-time.Second)}
			if name != "early" {
				fields["attempt_deadline"] = now.Add(-10 * time.Second)
			}
			if name == "exhausted block" || name == "exhausted continue" {
				fields["operation_deadline"] = now.Add(-time.Second)
			}
			require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumns(fields).Error)
			before := hookAttemptRow(t, f, in.OperationID)
			recovery := entity.HookRecoverExpiredAttemptInput{HookAttemptScope: in.HookAttemptScope, Config: in.Config}
			recovery.ExpectedVersion = claimed.Claim.Version
			out, err := f.repo.RecoverExpiredAttempt(context.Background(), recovery)
			require.NoError(t, err)
			require.Nil(t, out.Claim)
			if name == "early" {
				require.False(t, out.Changed)
				require.Equal(t, before, hookAttemptRow(t, f, in.OperationID))
				require.Nil(t, hookAttemptAudit(t, f, in.AttemptID).ResultCategory)
				return
			}
			require.True(t, out.Changed)
			after := hookAttemptRow(t, f, in.OperationID)
			audit := hookAttemptAudit(t, f, in.AttemptID)
			require.Equal(t, "TIMEOUT_UNCERTAIN", *audit.ResultCategory)
			require.Nil(t, audit.FinishedAt)
			require.Equal(t, before.Attempt, after.Attempt)
			require.Greater(t, after.LeaseGeneration, before.LeaseGeneration)
			require.Equal(t, before.OperationDeadline, after.OperationDeadline)
			require.Equal(t, claimed.Claim.DeliveryID, audit.DeliveryID)
			require.Equal(t, claimed.Claim.Token.Generation, audit.LeaseGeneration)
			if name == "late" {
				require.Equal(t, "retry_wait", after.Status)
			} else {
				require.Equal(t, "failed", after.Status)
				require.Nil(t, after.NextAttemptAt)
			}
			if name == "exhausted block" {
				require.Equal(t, entity.HookGateClosed, out.Run.State.Gate)
				require.Equal(t, entity.HookFinalizePending, out.Run.State.Finalize)
			}
			if name == "exhausted continue" {
				require.Equal(t, entity.HookGateReady, out.Run.State.Gate)
				require.Equal(t, entity.HookFinalizeNone, out.Run.State.Finalize)
			}
			late, err := f.repo.CompleteAttempt(context.Background(), oldResponse)
			require.NoError(t, err)
			require.True(t, late.Effects.LateIgnored)
			require.Equal(t, after, hookAttemptRow(t, f, in.OperationID))
			lateAudit := hookAttemptAudit(t, f, in.AttemptID)
			audit.LateIgnored = true
			require.Equal(t, audit, lateAudit)
			if name == "late" {
				require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumn("next_attempt_at", now).Error)
				originalID := in.AttemptID
				in.ExpectedVersion = after.Version
				in.AttemptID = hookTxSequence.Add(1)
				next, err := f.repo.ClaimAttempt(context.Background(), in)
				require.NoError(t, err)
				require.NotNil(t, next.Claim)
				require.Equal(t, int32(2), next.Claim.Token.Attempt)
				require.NotEqual(t, claimed.Claim.DeliveryID, next.Claim.DeliveryID)
				require.Equal(t, claimed.Claim.IdempotencyKey, next.Claim.IdempotencyKey)
				require.Equal(t, claimed.Claim.Deadline, next.Claim.Deadline)
				require.Equal(t, lateAudit, hookAttemptAudit(t, f, originalID))
			}
		})
	}
}

func TestHookAttemptRetryClaimExhaustedByRestart(t *testing.T) {
	for _, policy := range []entity.HookFailurePolicy{entity.HookFailurePolicyBlock, entity.HookFailurePolicyContinue} {
		t.Run(string(policy), func(t *testing.T) {
			f := newHookTxFixture(t)
			in := hookAttemptInput(t, f, true)
			in.Config.OnFailure = gptr.Of(policy)
			got, err := f.repo.ClaimAttempt(context.Background(), in)
			require.NoError(t, err)
			complete := hookCompletion(t, f, in, got.Claim)
			complete.Outcome = entity.HookOutcome{Code: entity.HookTransportError}
			_, err = f.repo.CompleteAttempt(context.Background(), complete)
			require.NoError(t, err)
			audit := hookAttemptAudit(t, f, in.AttemptID)
			op := hookAttemptRow(t, f, in.OperationID)
			now, err := hookDBNow(f.sql)
			require.NoError(t, err)
			require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumns(map[string]any{"next_attempt_at": now.Add(-time.Second), "attempt_deadline": now.Add(-time.Second), "operation_deadline": now.Add(time.Second)}).Error)
			originalID := in.AttemptID
			in.AttemptID = hookTxSequence.Add(1)
			in.ExpectedVersion = op.Version
			out, err := f.repo.ClaimAttempt(context.Background(), in)
			require.NoError(t, err)
			require.True(t, out.Changed)
			require.Nil(t, out.Claim)
			require.Equal(t, "failed", hookAttemptRow(t, f, in.OperationID).Status)
			require.Equal(t, int32(1), hookAttemptRow(t, f, in.OperationID).Attempt)
			require.Equal(t, audit, hookAttemptAudit(t, f, originalID))
			if policy == entity.HookFailurePolicyBlock {
				require.Equal(t, entity.HookFinalizePending, out.Run.State.Finalize)
			} else {
				require.Equal(t, entity.HookGateReady, out.Run.State.Gate)
			}
		})
	}
}
