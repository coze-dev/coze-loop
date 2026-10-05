// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHookAttemptCompletionAuditFailureRollsBackGateAndIntent(t *testing.T) {
	for _, outcome := range []entity.HookOutcomeCode{entity.HookSucceeded, entity.HookFailed} {
		t.Run(string(outcome), func(t *testing.T) {
			f := newHookTxFixture(t)
			in := hookAttemptInput(t, f, true)
			got, err := f.repo.ClaimAttempt(context.Background(), in)
			require.NoError(t, err)
			complete := hookCompletion(t, f, in, got.Claim)
			complete.Outcome.Code = outcome
			before, err := f.repo.GetRun(context.Background(), in.Key)
			require.NoError(t, err)
			op := hookAttemptRow(t, f, in.OperationID)
			audit := hookAttemptAudit(t, f, in.AttemptID)
			// Fail only the final audit write; preceding SQL writes execute against real MySQL.
			require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register("hook_attempt_audit_fail", func(tx *gorm.DB) {
				if tx.Statement.Table == model.TableNameExptLifecycleHookAttempt {
					tx.AddError(errors.New("audit write unavailable"))
				}
			}))
			t.Cleanup(func() { require.NoError(t, f.sql.Callback().Update().Remove("hook_attempt_audit_fail")) })
			_, err = f.repo.CompleteAttempt(context.Background(), complete)
			require.ErrorContains(t, err, "audit write unavailable")
			after, err := f.repo.GetRun(context.Background(), in.Key)
			require.NoError(t, err)
			require.Equal(t, before, after)
			require.Equal(t, op, hookAttemptRow(t, f, in.OperationID))
			require.Equal(t, audit, hookAttemptAudit(t, f, in.AttemptID))
		})
	}
}

func TestHookAttemptRenewRacesCancellation(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	got, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	run, err := f.repo.GetRun(context.Background(), in.Key)
	require.NoError(t, err)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var renewed, cancelled error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, renewed = f.repo.RenewAttempt(context.Background(), hookLeaseInput(in, got.Claim))
	}()
	go func() {
		defer wg.Done()
		<-start
		_, cancelled = f.repo.BeginFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}})
	}()
	close(start)
	wg.Wait()
	require.NoError(t, cancelled)
	if renewed != nil {
		require.ErrorIs(t, renewed, entity.ErrHookStoreConflict)
	}
	final, err := f.repo.GetRun(context.Background(), in.Key)
	require.NoError(t, err)
	require.Equal(t, entity.HookGateClosed, final.State.Gate)
	require.Equal(t, entity.HookFinalizePending, final.State.Finalize)
	op := hookAttemptRow(t, f, in.OperationID)
	require.Nil(t, op.LeaseOwner)
	require.Nil(t, op.LeaseUntil)
	require.Greater(t, op.LeaseGeneration, got.Claim.Token.Generation)
}

func TestHookAttemptOldResultAfterRetryOnlyMarksOriginalAudit(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	first, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	complete := hookCompletion(t, f, in, first.Claim)
	complete.Outcome.Code = entity.HookTransportError
	_, err = f.repo.CompleteAttempt(context.Background(), complete)
	require.NoError(t, err)
	firstAudit := hookAttemptAudit(t, f, in.AttemptID)
	oldID := in.AttemptID
	op := hookAttemptRow(t, f, in.OperationID)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumn("next_attempt_at", time.Now().UTC()).Error)
	in.ExpectedVersion = op.Version
	in.AttemptID = hookTxSequence.Add(1)
	in.Owner = "worker-b"
	second, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, second.Claim)
	op = hookAttemptRow(t, f, in.OperationID)
	secondAudit := hookAttemptAudit(t, f, in.AttemptID)
	complete.Outcome.Code = entity.HookSucceeded
	late, err := f.repo.CompleteAttempt(context.Background(), complete)
	require.NoError(t, err)
	require.True(t, late.Effects.LateIgnored)
	firstAudit.LateIgnored = true
	require.Equal(t, firstAudit, hookAttemptAudit(t, f, oldID))
	require.Equal(t, secondAudit, hookAttemptAudit(t, f, in.AttemptID))
	require.Equal(t, op, hookAttemptRow(t, f, in.OperationID))
}

func TestHookAttemptCompleteClockAfterAuditLock(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	got, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	complete := hookCompletion(t, f, in, got.Claim)
	now, err := hookDBNow(f.sql)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumn("lease_until", now.Add(200*time.Millisecond)).Error)
	op := hookAttemptRow(t, f, in.OperationID)
	tx := f.sql.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, tx.Exec("SELECT id FROM expt_lifecycle_hook_attempt WHERE id=? FOR UPDATE", in.AttemptID).Error)
	done := make(chan error, 1)
	go func() { _, err := f.repo.CompleteAttempt(context.Background(), complete); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("complete finished before lock release: %v", err)
	case <-time.After(350 * time.Millisecond):
	}
	require.NoError(t, tx.Commit().Error)
	require.NoError(t, <-done)
	require.Equal(t, op, hookAttemptRow(t, f, in.OperationID))
	audit := hookAttemptAudit(t, f, in.AttemptID)
	require.True(t, audit.LateIgnored)
	require.Nil(t, audit.ResultCategory)
}

func TestHookAttemptLegacyRunDoesNotInitialize(t *testing.T) {
	f := newHookTxFixture(t)
	seed := f.input(true, 0)
	require.NoError(t, f.sql.Create(&model.ExptRunLog{ID: seed.Key.RunID, SpaceID: f.space, ExptID: f.expt, ExptRunID: seed.Key.RunID, CreatedBy: "user"}).Error)
	in := entity.HookClaimAttemptInput{HookAttemptScope: entity.HookAttemptScope{Key: seed.Key, OperationID: seed.Before.OperationID, Phase: entity.HookPhaseBefore, ExecutionScope: seed.Snapshot.ExecutionScope, SnapshotHash: seed.Snapshot.Hash}, Owner: "worker", AttemptID: hookTxSequence.Add(1), Config: hookTestEnabledConfig()}
	result, err := f.repo.ClaimAttempt(context.Background(), in)
	require.ErrorIs(t, err, entity.ErrHookStoreMissing)
	require.Nil(t, result.Claim)
	for _, table := range []string{"expt_lifecycle_run", "expt_lifecycle_hook_run"} {
		var count int64
		require.NoError(t, f.sql.Table(table).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&count).Error)
		require.Zero(t, count)
	}
	var count int64
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookAttempt{}).Where("id=?", in.AttemptID).Count(&count).Error)
	require.Zero(t, count)
}

func hookTestEnabledConfig() *entity.HookConfig {
	enabled, url := true, "https://example.com/hook"
	return &entity.HookConfig{Enabled: &enabled, InvokeHTTPInfo: &entity.HookHTTPInfo{URL: &url}}
}
