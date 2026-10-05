// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func readyHookLedger(t *testing.T) (*hookTxFixture, entity.HookCreateRunInput, int64) {
	t.Helper()
	f := newHookTxFixture(t)
	in := f.input(false, 0)
	ctx := context.Background()
	_, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	_, err = f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, Hash: strings.Repeat("b", 64)})
	require.NoError(t, err)
	item := hookTxSequence.Add(1)
	// Simulate a future authorized ledger append, not the yet-unwired Invoke path.
	require.NoError(t, f.sql.Create(&model.ExptLifecycleRunItem{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: in.Key.RunID, Ordinal: 99, SourceSpaceID: f.space, EvalSetID: 8, ItemID: item}).Error)
	return f, in, item
}

func TestHookTxAdmitLaterLedgerAndRepeat(t *testing.T) {
	f, in, item := readyHookLedger(t)
	ctx := context.Background()
	request := entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: 1}, ItemID: item}
	got, err := f.repo.AdmitItem(ctx, request)
	require.NoError(t, err)
	require.True(t, got.Admitted)
	require.True(t, got.NewlyAdmitted)
	require.False(t, got.AdmittedAt.IsZero())
	require.Equal(t, int64(2), got.Version)
	request.ExpectedVersion = got.Version
	again, err := f.repo.AdmitItem(ctx, request)
	require.NoError(t, err)
	require.True(t, again.Admitted)
	require.False(t, again.NewlyAdmitted)
	require.Equal(t, got.AdmittedAt, again.AdmittedAt)
	read, err := f.repo.GetRun(ctx, in.Key)
	require.NoError(t, err)
	require.True(t, read.ExecutionStarted)
	require.Zero(t, read.PlanCount)
	bad := request
	bad.ItemID++
	_, err = f.repo.AdmitItem(ctx, bad)
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	bad = request
	bad.ExpectedVersion = 0
	_, err = f.repo.AdmitItem(ctx, bad)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
}

func TestHookTxAdmitRejectsNotReadyAndWrongOwner(t *testing.T) {
	f := newHookTxFixture(t)
	in := f.input(false, 0)
	ctx := context.Background()
	_, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	request := entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, ItemID: 1}
	_, err = f.repo.AdmitItem(ctx, request)
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	bad := request
	bad.Key.WorkspaceID++
	_, err = f.repo.AdmitItem(ctx, bad)
	require.Error(t, err)
	bad = request
	bad.Key.ExperimentID++
	_, err = f.repo.AdmitItem(ctx, bad)
	require.Error(t, err)
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).UpdateColumn("lifecycle_hook_version", nil).Error)
	_, err = f.repo.GetRun(ctx, in.Key)
	require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
}

type hookTxPauseKey struct{}

func TestHookTxCancelAdmissionLockOrdering(t *testing.T) {
	for _, first := range []string{"cancel", "admit"} {
		t.Run(first, func(t *testing.T) {
			f, in, item := readyHookLedger(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			locked, release := make(chan struct{}), make(chan struct{})
			var once, unpause sync.Once
			defer unpause.Do(func() { close(release) })
			name := fmt.Sprintf("hook_pause_%d", f.expt)
			require.NoError(t, f.sql.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
				if tx.Error == nil && tx.Statement.Table == "expt_lifecycle_run" && tx.Statement.Context.Value(hookTxPauseKey{}) == first {
					once.Do(func() { close(locked); <-release })
				}
			}))
			defer f.sql.Callback().Query().Remove(name)
			admitReq := entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: 1}, ItemID: item}
			cancelReq := entity.HookFinalizeInput{HookStoreGuard: admitReq.HookStoreGuard, Intent: entity.HookTerminalIntent{Status: 13, Reason: "cancel"}}
			doneAdmit, doneCancel := make(chan error, 1), make(chan error, 1)
			admit := func(c context.Context) { _, err := f.repo.AdmitItem(c, admitReq); doneAdmit <- err }
			stop := func(c context.Context) {
				_, err := f.repo.BeginFinalize(c, cancelReq)
				if err == entity.ErrHookStoreConflict {
					run, e := f.repo.GetRun(c, in.Key)
					if e != nil {
						doneCancel <- e
						return
					}
					cancelReq.ExpectedVersion = run.Version
					_, err = f.repo.BeginFinalize(c, cancelReq)
				}
				doneCancel <- err
			}
			paused := context.WithValue(ctx, hookTxPauseKey{}, first)
			if first == "cancel" {
				go stop(paused)
			} else {
				go admit(paused)
			}
			select {
			case <-locked:
			case <-ctx.Done():
				t.Fatal("first operation did not acquire row lock")
			}
			var second <-chan error
			if first == "cancel" {
				go admit(ctx)
				second = doneAdmit
			} else {
				go stop(ctx)
				second = doneCancel
			}
			select {
			case err := <-second:
				t.Fatalf("second operation escaped row lock: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			unpause.Do(func() { close(release) })
			cancelErr, admitErr := <-doneCancel, <-doneAdmit
			require.NoError(t, cancelErr)
			var row model.ExptLifecycleRunItem
			require.NoError(t, f.sql.First(&row, "space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, in.Key.RunID, item).Error)
			if first == "cancel" {
				require.Error(t, admitErr)
				require.Nil(t, row.AdmittedAt)
			} else {
				require.NoError(t, admitErr)
				require.NotNil(t, row.AdmittedAt)
			}
			run, err := f.repo.GetRun(ctx, in.Key)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateClosed, run.State.Gate)
			admitReq.ExpectedVersion = run.Version
			_, err = f.repo.AdmitItem(ctx, admitReq)
			require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
		})
	}
}

func TestHookTxAdmissionClockAfterLocks(t *testing.T) {
	for _, table := range []string{"run", "item"} {
		t.Run(table, func(t *testing.T) {
			f, in, item := readyHookLedger(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			tx := f.sql.Begin()
			require.NoError(t, tx.Error)
			defer tx.Rollback()
			if table == "run" {
				var row model.ExptLifecycleRun
				require.NoError(t, tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, in.Key.RunID).Error)
			} else {
				var row model.ExptLifecycleRunItem
				require.NoError(t, tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, in.Key.RunID, item).Error)
			}
			type result struct {
				value entity.HookAdmitItemResult
				err   error
			}
			done := make(chan result, 1)
			go func() {
				v, err := f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: 1}, ItemID: item})
				done <- result{v, err}
			}()
			select {
			case r := <-done:
				t.Fatalf("admission escaped held row lock: %v", r.err)
			case <-time.After(100 * time.Millisecond):
			}
			var barrier time.Time
			require.NoError(t, tx.Raw("SELECT CURRENT_TIMESTAMP(3)").Row().Scan(&barrier))
			require.NoError(t, tx.Commit().Error)
			r := <-done
			require.NoError(t, r.err)
			require.True(t, r.value.Admitted)
			require.False(t, r.value.AdmittedAt.Before(barrier), "admission time was captured before lock wait ended")
		})
	}
}
