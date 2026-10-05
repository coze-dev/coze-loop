// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func hookAttemptConcurrencyContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
	if dsn == "" {
		t.Skip("requires HOOK_MYSQL_TX_DSN real MySQL lock tests")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "/Users/bytedance/fornax_workspace/_local/scratch/hook-mysql-20260922/m.sock", cfg.Addr)
	require.Equal(t, "hook_7378265404_tx", cfg.DBName)
	return context.WithTimeout(context.Background(), 20*time.Second)
}

func TestHookAttemptDistinctRunClaimsDoNotDeadlock(t *testing.T) {
	ctx, cancel := hookAttemptConcurrencyContext(t)
	defer cancel()
	fixtures := []*hookTxFixture{newHookTxFixture(t), newHookTxFixture(t)}
	inputs := []entity.HookClaimAttemptInput{hookAttemptInput(t, fixtures[0], true), hookAttemptInput(t, fixtures[1], true)}
	require.NotEqual(t, inputs[0].Key.ExperimentID, inputs[1].Key.ExperimentID)
	require.NotEqual(t, inputs[0].Key.RunID, inputs[1].Key.RunID)
	require.NotEqual(t, inputs[0].OperationID, inputs[1].OperationID)

	// Place both test IDs in the same missing upper gap; production never uses MAX(id).
	var highest int64
	require.NoError(t, fixtures[0].sql.WithContext(ctx).Raw("SELECT COALESCE(MAX(id),0) FROM expt_lifecycle_hook_attempt").Scan(&highest).Error)
	if next := hookTxSequence.Add(2); next > highest {
		highest = next
	}
	for i := range inputs {
		inputs[i].AttemptID = highest + int64(i) + 1
		inputs[i].Owner = fmt.Sprintf("claim-worker-%d", i)
	}

	type arrival struct {
		connection int64
		isolation  string
	}
	ready := make(chan arrival, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	for i, f := range fixtures {
		name := fmt.Sprintf("hook_attempt_insert_barrier_%d", i)
		require.NoError(t, f.sql.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
			if tx.Statement.Table != model.TableNameExptLifecycleHookAttempt {
				return
			}
			var a arrival
			if err := tx.Session(&gorm.Session{NewDB: true}).Raw("SELECT CONNECTION_ID(), @@transaction_isolation").Row().Scan(&a.connection, &a.isolation); err != nil {
				tx.AddError(err)
				return
			}
			select {
			case ready <- a:
			case <-ctx.Done():
				tx.AddError(ctx.Err())
				return
			}
			select {
			case <-release:
			case <-ctx.Done():
				tx.AddError(ctx.Err())
			}
		}))
		t.Cleanup(func() { require.NoError(t, f.sql.Callback().Create().Remove(name)) })
	}

	type result struct {
		index int
		out   entity.HookAttemptStoreResult
		err   error
	}
	done := make(chan result, 2)
	received := 0
	// Cancel and join before fixture cleanup, including assertion failures at the barrier.
	defer func() {
		cancel()
		releaseOnce.Do(func() { close(release) })
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for received < 2 {
			select {
			case <-done:
				received++
			case <-timer.C:
				t.Error("claim goroutines did not stop after context cancellation")
				return
			}
		}
	}()
	for i, f := range fixtures {
		go func(i int, f *hookTxFixture) {
			out, err := f.repo.ClaimAttempt(ctx, inputs[i])
			done <- result{index: i, out: out, err: err}
		}(i, f)
	}
	var arrivals [2]arrival
	for i := range arrivals {
		select {
		case arrivals[i] = <-ready:
			require.Equal(t, "REPEATABLE-READ", arrivals[i].isolation)
			t.Logf("insert barrier: connection=%d isolation=%s", arrivals[i].connection, arrivals[i].isolation)
		case <-ctx.Done():
			t.Fatalf("insert barrier timed out: %v", ctx.Err())
		}
	}
	require.NotEqual(t, arrivals[0].connection, arrivals[1].connection)
	releaseOnce.Do(func() { close(release) })
	var results [2]result
	for received < 2 {
		select {
		case r := <-done:
			results[r.index] = r
			received++
		case <-ctx.Done():
			t.Fatalf("claim completion timed out: %v", ctx.Err())
		}
	}
	for i, r := range results {
		f, in := fixtures[i], inputs[i]
		op := hookAttemptRow(t, f, in.OperationID)
		var count int64
		require.NoError(t, f.sql.WithContext(ctx).Model(&model.ExptLifecycleHookAttempt{}).Where("operation_id=?", in.OperationID).Count(&count).Error)
		if r.err != nil {
			t.Errorf("independent claim %d must commit without retry: %v", i, r.err)
			require.Zero(t, op.Attempt)
			require.Zero(t, count)
			require.Nil(t, r.out.Claim)
			continue
		}
		require.True(t, r.out.Changed)
		require.NotNil(t, r.out.Claim)
		require.Equal(t, "running", op.Status)
		require.Equal(t, int32(1), op.Attempt)
		require.Equal(t, int64(1), count)
		audit := hookAttemptAudit(t, f, in.AttemptID)
		require.Equal(t, in.OperationID, audit.OperationID)
		require.Equal(t, r.out.Claim.DeliveryID, audit.DeliveryID)
		require.Equal(t, r.out.Claim.StartedAt, audit.StartedAt)
	}
}

func TestHookAttemptRenewThenCompleteVersionRetry(t *testing.T) {
	ctx, cancel := hookAttemptConcurrencyContext(t)
	defer cancel()
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, true)
	claimed, err := f.repo.ClaimAttempt(ctx, in)
	require.NoError(t, err)
	complete := hookCompletion(t, f, in, claimed.Claim)
	renewed, err := f.repo.RenewAttempt(ctx, hookLeaseInput(in, claimed.Claim))
	require.NoError(t, err)
	require.Equal(t, claimed.Claim.Version+1, renewed.Claim.Version)
	require.Equal(t, claimed.Claim.HookAttemptIdentity, renewed.Claim.HookAttemptIdentity)
	before := hookAttemptAudit(t, f, in.AttemptID)
	opBefore := hookAttemptRow(t, f, in.OperationID)
	_, err = f.repo.CompleteAttempt(ctx, complete)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Equal(t, before, hookAttemptAudit(t, f, in.AttemptID))
	require.Equal(t, opBefore, hookAttemptRow(t, f, in.OperationID))
	require.False(t, hookAttemptAudit(t, f, in.AttemptID).LateIgnored)
	run, err := f.repo.GetRun(ctx, in.Key)
	require.NoError(t, err)
	for _, op := range run.Operations {
		if op.OperationID == in.OperationID {
			complete.ExpectedVersion = op.Version
		}
	}
	require.Equal(t, renewed.Claim.Version, complete.ExpectedVersion)
	out, err := f.repo.CompleteAttempt(ctx, complete)
	require.NoError(t, err)
	require.True(t, out.Changed)
	require.False(t, out.Effects.LateIgnored)
	require.Equal(t, "succeeded", hookAttemptRow(t, f, in.OperationID).Status)
	require.Equal(t, complete.CompletedAt, *hookAttemptAudit(t, f, in.AttemptID).FinishedAt)
}
