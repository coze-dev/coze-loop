// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHookAttemptRenewIgnoresUnrelatedRunVersion(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	got, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	old := hookAttemptRow(t, f, in.OperationID)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("expt_run_id=?", in.Key.RunID).UpdateColumn("version", gorm.Expr("version+1")).Error)
	result, err := f.repo.RenewAttempt(context.Background(), hookLeaseInput(in, got.Claim))
	require.NoError(t, err)
	require.NotNil(t, result.Claim)
	current := hookAttemptRow(t, f, in.OperationID)
	require.Equal(t, old.OperationDeadline, current.OperationDeadline)
	require.Equal(t, old.AttemptDeadline, current.AttemptDeadline)
	require.False(t, current.LeaseUntil.After(current.AttemptDeadline.Add(5*time.Second)))
	require.Equal(t, old.Attempt, current.Attempt)
	require.Equal(t, old.LeaseGeneration, current.LeaseGeneration)
}

func TestHookAttemptRenewRejectsLostLeaseOrIdentity(t *testing.T) {
	for _, name := range []string{"expired", "owner", "delivery", "token", "version", "cancel"} {
		t.Run(name, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := hookAttemptInput(t, f, true)
			got, err := f.repo.ClaimAttempt(context.Background(), in)
			require.NoError(t, err)
			lease := hookLeaseInput(in, got.Claim)
			switch name {
			case "expired":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumn("lease_until", time.Now().UTC().Add(-time.Second)).Error)
			case "owner":
				lease.Owner = "other"
			case "delivery":
				lease.DeliveryID = "other"
			case "token":
				lease.Token.Generation++
			case "version":
				lease.ExpectedVersion++
			case "cancel":
				run, err := f.repo.GetRun(context.Background(), in.Key)
				require.NoError(t, err)
				_, err = f.repo.BeginFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}})
				require.NoError(t, err)
			}
			before := hookAttemptRow(t, f, in.OperationID)
			_, err = f.repo.RenewAttempt(context.Background(), lease)
			require.Error(t, err)
			require.Equal(t, before, hookAttemptRow(t, f, in.OperationID))
		})
	}
}

func TestHookAttemptRenewUsesClockAfterAuditLock(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	got, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	tx := f.sql.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, tx.Exec("SELECT id FROM expt_lifecycle_hook_attempt WHERE id=? FOR UPDATE", in.AttemptID).Error)
	now, err := hookDBNow(f.sql)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumn("lease_until", now.Add(200*time.Millisecond)).Error)
	done := make(chan error, 1)
	go func() {
		_, err := f.repo.RenewAttempt(context.Background(), hookLeaseInput(in, got.Claim))
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("renew finished before audit lock released: %v", err)
	case <-time.After(350 * time.Millisecond):
	}
	require.NoError(t, tx.Commit().Error)
	require.ErrorIs(t, <-done, entity.ErrHookStoreConflict)
}
