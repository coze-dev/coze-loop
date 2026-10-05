// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func hookAttemptInput(t *testing.T, f *hookTxFixture, before bool) entity.HookClaimAttemptInput {
	t.Helper()
	ctx := context.Background()
	seed := f.input(before, 0)
	created, err := f.repo.CreateRunWithHooks(ctx, seed)
	require.NoError(t, err)
	phase, op := entity.HookPhaseBefore, seed.Before
	var ready entity.HookStoreResult
	if before {
		ready, err = f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: seed.Key, ExpectedVersion: created.Run.Version}, Count: 0, Hash: seed.Snapshot.Hash})
	} else {
		// The Append fixture must have stopped accepting items before normal finalization.
		require.NoError(t, hookRunScope(f.sql.Model(&model.ExptRunLog{}), seed.Key).UpdateColumn("status", int64(entity.ExptStatus_Draining)).Error)
		intent := entity.HookTerminalIntent{Status: entity.ExptStatus_Success, Reason: "complete"}
		ready, err = f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: seed.Key, ExpectedVersion: created.Run.Version}, Intent: intent})
		require.NoError(t, err)
		ready, err = f.repo.CommitFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: seed.Key, ExpectedVersion: ready.Run.Version}, Intent: intent})
		phase, op = entity.HookPhaseAfter, seed.After
	}
	require.NoError(t, err)
	version := int64(0)
	for _, item := range ready.Run.Operations {
		if item.Phase == phase {
			version = item.Version
		}
	}
	return entity.HookClaimAttemptInput{HookAttemptScope: entity.HookAttemptScope{Key: seed.Key, OperationID: op.OperationID, Phase: phase, ExpectedVersion: version, ExecutionScope: seed.Snapshot.ExecutionScope, SnapshotHash: seed.Snapshot.Hash}, Owner: "worker-a", AttemptID: hookTxSequence.Add(1), Config: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}, TimeoutSeconds: gptr.Of(int32(10))}}
}

func hookAttemptRow(t *testing.T, f *hookTxFixture, op string) model.ExptLifecycleHookRun {
	t.Helper()
	var row model.ExptLifecycleHookRun
	require.NoError(t, f.sql.Where("operation_id=?", op).First(&row).Error)
	return row
}

func hookAttemptAudit(t *testing.T, f *hookTxFixture, id int64) model.ExptLifecycleHookAttempt {
	t.Helper()
	var row model.ExptLifecycleHookAttempt
	require.NoError(t, f.sql.Where("id=?", id).First(&row).Error)
	return row
}

func TestHookAttemptClaimConcurrentSingleWinner(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var results [2]entity.HookAttemptStoreResult
	var errs [2]error
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			c := in
			c.AttemptID += int64(i)
			c.Owner = fmt.Sprint("worker-", i)
			results[i], errs[i] = f.repo.ClaimAttempt(context.Background(), c)
		}(i)
	}
	close(start)
	wg.Wait()
	winners := 0
	for i, out := range results {
		if errs[i] != nil {
			require.ErrorIs(t, errs[i], entity.ErrHookStoreConflict)
			continue
		}
		winners++
		require.True(t, out.Changed)
		require.NotNil(t, out.Claim)
		require.Equal(t, int32(1), out.Claim.Token.Attempt)
		require.Equal(t, int64(1), out.Claim.Token.Generation)
		require.Equal(t, 80*time.Second, out.Claim.Deadline.Sub(out.Claim.StartedAt))
		require.Equal(t, 15*time.Second, out.Claim.LeaseUntil.Sub(out.Claim.StartedAt))
		require.Equal(t, 10*time.Second, out.Claim.AttemptDeadline.Sub(out.Claim.StartedAt))
	}
	require.Equal(t, 1, winners)
	var count int64
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookAttempt{}).Where("operation_id=?", in.OperationID).Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.Equal(t, int32(1), hookAttemptRow(t, f, in.OperationID).Attempt)
}

func TestHookAttemptClaimAuditFailureRollsBack(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	before := hookAttemptRow(t, f, in.OperationID)
	// Duplicate primary key is a real audit insert failure without changing the schema.
	require.NoError(t, f.sql.Create(&model.ExptLifecycleHookAttempt{ID: in.AttemptID, OperationID: in.OperationID, Attempt: 11, DeliveryID: fmt.Sprint("occupied-", in.AttemptID), StartedAt: time.Now().UTC()}).Error)
	_, err := f.repo.ClaimAttempt(context.Background(), in)
	require.ErrorContains(t, err, "Duplicate entry")
	require.Equal(t, before, hookAttemptRow(t, f, in.OperationID))
}

func TestHookAttemptClaimAdmissionGuards(t *testing.T) {
	for _, name := range []string{"version", "scope", "snapshot", "operation", "phase", "future", "plan", "cancel", "old before"} {
		t.Run(name, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := hookAttemptInput(t, f, true)
			switch name {
			case "version":
				in.ExpectedVersion++
			case "scope":
				in.ExecutionScope = "other"
			case "snapshot":
				in.SnapshotHash = string(make([]byte, 64))
			case "operation":
				in.OperationID = "other"
			case "phase":
				in.Phase = entity.HookPhaseAfter
			case "future":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumn("next_attempt_at", time.Now().UTC().Add(time.Hour)).Error)
			case "plan":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("expt_run_id=?", in.Key.RunID).UpdateColumn("plan_state", 0).Error)
			case "cancel":
				run, err := f.repo.GetRun(context.Background(), in.Key)
				require.NoError(t, err)
				_, err = f.repo.BeginFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}})
				require.NoError(t, err)
			case "old before":
				_, err := f.repo.CreateRunWithHooks(context.Background(), f.input(false, in.Key.RunID))
				require.NoError(t, err)
			}
			_, err := f.repo.ClaimAttempt(context.Background(), in)
			require.Error(t, err)
			require.Zero(t, hookAttemptRow(t, f, func() string {
				if name == "operation" {
					return fmt.Sprintf("hook_before_%d", in.Key.RunID)
				}
				return in.OperationID
			}()).Attempt)
		})
	}
}

func TestHookAttemptAfterOldRunAndSoftDelete(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, false)
	newer := f.input(false, in.Key.RunID)
	_, err := f.repo.CreateRunWithHooks(context.Background(), newer)
	require.NoError(t, err)
	require.NoError(t, f.sql.Delete(&model.Experiment{}, "id=?", f.expt).Error)
	got, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, got.Claim)
	require.Equal(t, in.Key, got.Claim.Token.Run)
	require.Equal(t, newer.Key.RunID, f.latest(t))
	complete := hookCompletion(t, f, in, got.Claim)
	before, err := f.repo.GetRun(context.Background(), newer.Key)
	require.NoError(t, err)
	finished, err := f.repo.CompleteAttempt(context.Background(), complete)
	require.NoError(t, err)
	require.False(t, finished.LatestProjected)
	after, err := f.repo.GetRun(context.Background(), newer.Key)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestHookAttemptClaimReadsClockAfterOperationLock(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	now, err := hookDBNow(f.sql)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.OperationID).UpdateColumn("next_attempt_at", now.Add(200*time.Millisecond)).Error)
	tx := f.sql.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, tx.Exec("SELECT id FROM expt_lifecycle_hook_run WHERE operation_id=? FOR UPDATE", in.OperationID).Error)
	done := make(chan error, 1)
	go func() { _, err := f.repo.ClaimAttempt(context.Background(), in); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("claim finished before lock release: %v", err)
	case <-time.After(350 * time.Millisecond):
	}
	require.NoError(t, tx.Commit().Error)
	require.NoError(t, <-done)
}
