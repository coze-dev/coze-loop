// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHookTxCreateDifferentExperimentsShareMissingGap(t *testing.T) {
	fixtures := []*hookTxFixture{newHookTxFixture(t), newHookTxFixture(t)}
	inputs := []entity.HookCreateRunInput{fixtures[0].input(true, 0), fixtures[1].input(true, 0)}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	arrived, release := make(chan struct{}, 2), make(chan struct{})
	var missing, inserts [2]atomic.Int32
	var results [2]entity.HookStoreResult
	var errs [2]error
	var wg sync.WaitGroup
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer func() { cancel(); unblock(); wg.Wait() }()
	for i, f := range fixtures {
		var isolation string
		require.NoError(t, f.sql.Raw("SELECT @@transaction_isolation").Scan(&isolation).Error)
		require.Equal(t, "REPEATABLE-READ", isolation)
		probeName, insertName := fmt.Sprintf("hook_gap_probe_%d", f.expt), fmt.Sprintf("hook_gap_insert_%d", f.expt)
		require.NoError(t, f.sql.Callback().Query().After("gorm:query").Register(probeName, func(tx *gorm.DB) {
			if tx.Statement.Table == model.TableNameExptLifecycleRun && errors.Is(tx.Error, gorm.ErrRecordNotFound) {
				missing[i].Add(1)
			}
		}))
		require.NoError(t, f.sql.Callback().Create().Before("gorm:create").Register(insertName, func(tx *gorm.DB) {
			if tx.Statement.Table != model.TableNameExptLifecycleRun {
				return
			}
			inserts[i].Add(1)
			select {
			case arrived <- struct{}{}:
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
		t.Cleanup(func() {
			require.NoError(t, f.sql.Callback().Query().Remove(probeName))
			require.NoError(t, f.sql.Callback().Create().Remove(insertName))
		})
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = f.repo.CreateRunWithHooks(ctx, inputs[i])
		}()
	}
	for range fixtures {
		select {
		case <-arrived:
		case <-ctx.Done():
			t.Fatal("both transactions must reach lifecycle INSERT before release")
		}
	}
	unblock()
	wg.Wait()
	for i, f := range fixtures {
		require.Equal(t, int32(1), missing[i].Load())
		require.Equal(t, int32(1), inserts[i].Load(), "creation must not hide a deadlock by retrying")
		require.NoError(t, errs[i], "different experiments must both create successfully")
		require.True(t, results[i].Changed)
		require.True(t, results[i].LatestProjected)
		require.Equal(t, inputs[i].Key.RunID, f.latest(t))
		require.Len(t, results[i].Run.Operations, 2)
		var log model.ExptRunLog
		require.NoError(t, hookRunScope(f.sql, inputs[i].Key).First(&log).Error)
		require.Equal(t, int32(1), gptr.Indirect(log.LifecycleHookVersion))
		stored, err := f.repo.GetRun(ctx, inputs[i].Key)
		require.NoError(t, err)
		require.Equal(t, results[i].Run, stored)
	}
}

func TestHookTxCreateWaiterReadsCommittedRunAfterParentLock(t *testing.T) {
	for _, sameRun := range []bool{true, false} {
		t.Run(fmt.Sprint(sameRun), func(t *testing.T) {
			f := newHookTxFixture(t)
			a, b := f.input(true, 0), f.input(true, 0)
			if sameRun {
				b = a
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			type waiterKey struct{}
			writerReady, waiterReady, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var releaseOnce, writerOnce, waiterOnce sync.Once
			var wg sync.WaitGroup
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer func() { cancel(); unblock(); wg.Wait() }()
			queryName, updateName := fmt.Sprintf("hook_waiter_query_%d", f.expt), fmt.Sprintf("hook_waiter_update_%d", f.expt)
			require.NoError(t, f.sql.Callback().Query().Before("gorm:query").Register(queryName, func(tx *gorm.DB) {
				if _, ok := tx.Statement.Dest.(*model.Experiment); ok && tx.Statement.Context.Value(waiterKey{}) == true {
					waiterOnce.Do(func() { close(waiterReady) })
				}
			}))
			require.NoError(t, f.sql.Callback().Update().After("gorm:update").Register(updateName, func(tx *gorm.DB) {
				if tx.Statement.Table != model.TableNameExperiment || tx.Error != nil || tx.Statement.Context.Value(waiterKey{}) == true {
					return
				}
				writerOnce.Do(func() { close(writerReady) })
				select {
				case <-release:
				case <-ctx.Done():
					tx.AddError(ctx.Err())
				}
			}))
			t.Cleanup(func() {
				require.NoError(t, f.sql.Callback().Query().Remove(queryName))
				require.NoError(t, f.sql.Callback().Update().Remove(updateName))
			})
			var results [2]entity.HookStoreResult
			var errs [2]error
			wg.Add(1)
			go func() { defer wg.Done(); results[0], errs[0] = f.repo.CreateRunWithHooks(ctx, a) }()
			select {
			case <-writerReady:
			case <-ctx.Done():
				t.Fatal("first create did not reach uncommitted latest-run update")
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[1], errs[1] = f.repo.CreateRunWithHooks(context.WithValue(ctx, waiterKey{}, true), b)
			}()
			select {
			case <-waiterReady:
			case <-ctx.Done():
				t.Fatal("second create did not reach the parent lock")
			}
			unblock()
			wg.Wait()
			require.NoError(t, errs[0])
			require.True(t, results[0].Changed)
			require.False(t, results[1].Changed)
			if sameRun {
				require.NoError(t, errs[1])
				require.Equal(t, results[0].Run, results[1].Run)
			} else {
				require.ErrorIs(t, errs[1], entity.ErrHookStoreConflict)
				require.Nil(t, results[1].Run)
			}
			require.Equal(t, a.Key.RunID, f.latest(t))
			var count int64
			require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&count).Error)
			require.Equal(t, int64(1), count)
		})
	}
}

func TestHookTxCreateCallbackFailureRollsBackAllRows(t *testing.T) {
	for _, table := range []string{model.TableNameExptRunLog, model.TableNameExptLifecycleRun, model.TableNameExptLifecycleHookRun, model.TableNameExperiment} {
		t.Run(table, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := f.input(true, 0)
			injected := errors.New("hook create write failure")
			name := fmt.Sprintf("hook_create_failure_%d", f.expt)
			var calls int
			fail := func(tx *gorm.DB) {
				if tx.Statement.Table == table && tx.Error == nil {
					calls++
					tx.AddError(injected)
				}
			}
			if table == model.TableNameExperiment {
				require.NoError(t, f.sql.Callback().Update().After("gorm:update").Register(name, fail))
				t.Cleanup(func() { require.NoError(t, f.sql.Callback().Update().Remove(name)) })
			} else {
				require.NoError(t, f.sql.Callback().Create().After("gorm:create").Register(name, fail))
				t.Cleanup(func() { require.NoError(t, f.sql.Callback().Create().Remove(name)) })
			}
			out, err := f.repo.CreateRunWithHooks(context.Background(), in)
			require.ErrorIs(t, err, injected)
			require.Equal(t, 1, calls)
			require.Nil(t, out.Run)
			require.False(t, out.Changed)
			require.Zero(t, f.latest(t))
			for _, name := range []string{model.TableNameExptRunLog, model.TableNameExptLifecycleRun, model.TableNameExptLifecycleHookRun} {
				var count int64
				require.NoError(t, f.sql.Table(name).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&count).Error)
				require.Zero(t, count)
			}
		})
	}
}

func TestHookTxCreateRejectsOrphansWithoutRepair(t *testing.T) {
	for _, table := range []string{model.TableNameExptRunLog, model.TableNameExptLifecycleHookRun, model.TableNameExptLifecycleRunItem} {
		t.Run(table, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := f.input(true, 0)
			var orphan any
			switch table {
			case model.TableNameExptRunLog:
				orphan = &model.ExptRunLog{ID: in.Key.RunID, SpaceID: f.space, ExptID: f.expt, ExptRunID: in.Key.RunID}
			case model.TableNameExptLifecycleHookRun:
				orphan = &model.ExptLifecycleHookRun{ID: in.Before.ID, SpaceID: f.space, ExptID: f.expt, ExptRunID: in.Key.RunID, Phase: "before", OperationID: in.Before.OperationID, IdempotencyKey: in.Before.IdempotencyKey, Status: "pending", ExecutionScope: "local"}
			case model.TableNameExptLifecycleRunItem:
				orphan = &model.ExptLifecycleRunItem{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: in.Key.RunID, ItemID: 1}
			}
			require.NoError(t, f.sql.Create(orphan).Error)
			out, err := f.repo.CreateRunWithHooks(context.Background(), in)
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			require.Nil(t, out.Run)
			require.Zero(t, f.latest(t))
			for _, name := range []string{model.TableNameExptRunLog, model.TableNameExptLifecycleRun, model.TableNameExptLifecycleHookRun, model.TableNameExptLifecycleRunItem} {
				var count int64
				require.NoError(t, hookRunScope(f.sql.Table(name), in.Key).Count(&count).Error)
				if name == table {
					require.Equal(t, int64(1), count)
				} else {
					require.Zero(t, count)
				}
			}
		})
	}
}
