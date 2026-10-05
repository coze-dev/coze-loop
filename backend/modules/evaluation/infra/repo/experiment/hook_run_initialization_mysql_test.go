// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type initializationMySQLActor struct{}

// Pause the first writer while its real transaction holds the fixture's locks.
func initializationMySQLPauseUpdate(t *testing.T, f *hookTxFixture, table string) (<-chan struct{}, <-chan struct{}, func()) {
	t.Helper()
	writer, waiter, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var writerOnce, waiterOnce, releaseOnce sync.Once
	updateName, queryName := fmt.Sprintf("init_mysql_write_%d", f.expt), fmt.Sprintf("init_mysql_wait_%d", f.expt)
	require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register(updateName, func(tx *gorm.DB) {
		if tx.Error != nil || tx.Statement.Table != table || tx.Statement.Context.Value(initializationMySQLActor{}) != "first" {
			return
		}
		writerOnce.Do(func() { close(writer) })
		select {
		case <-release:
		case <-tx.Statement.Context.Done():
			tx.AddError(tx.Statement.Context.Err())
		}
	}))
	require.NoError(t, f.sql.Callback().Query().Before("gorm:query").Register(queryName, func(tx *gorm.DB) {
		if tx.Statement.Table == model.TableNameExperiment && tx.Statement.Context.Value(initializationMySQLActor{}) == "second" {
			if locking, ok := tx.Statement.Clauses["FOR"].Expression.(clause.Locking); ok && locking.Strength == "UPDATE" {
				waiterOnce.Do(func() { close(waiter) })
			}
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, f.sql.Callback().Update().Remove(updateName))
		require.NoError(t, f.sql.Callback().Query().Remove(queryName))
	})
	return writer, waiter, func() { releaseOnce.Do(func() { close(release) }) }
}

func initializationMySQLRequireParentLocked(t *testing.T, f *hookTxFixture, ctx context.Context) {
	t.Helper()
	err := f.p.Transaction(ctx, func(tx *gorm.DB) error {
		var row model.Experiment
		return tx.Select("id").Where("id=? AND space_id=?", f.expt, f.space).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}).Take(&row).Error
	}, db.WithMaster())
	var lockError *driver.MySQLError
	require.ErrorAs(t, err, &lockError)
	require.Equal(t, uint16(3572), lockError.Number, "the first writer must hold the actual experiment parent lock")
}

func initializationMySQLRequireRows(t *testing.T, f *hookTxFixture, logs int64) {
	t.Helper()
	for _, table := range []string{model.TableNameExptRunLog, model.TableNameExptLifecycleRun, model.TableNameExptLifecycleHookRun, model.TableNameExptLifecycleRunItem} {
		var count int64
		require.NoError(t, f.sql.Table(table).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&count).Error)
		want := int64(0)
		if table == model.TableNameExptRunLog {
			want = logs
		}
		require.Equal(t, want, count, table)
	}
}

func initializationMySQLLog(t *testing.T, f *hookTxFixture, key entity.HookRunKey) model.ExptRunLog {
	t.Helper()
	var log model.ExptRunLog
	require.NoError(t, hookRunScope(f.sql.Unscoped(), key).Where("id=?", key.RunID).First(&log).Error)
	return log
}

func TestHookInitializationMySQLConcurrentLegacyCreate(t *testing.T) {
	f := newHookTxFixture(t)
	r := NewHookRunInitializationRepo(f.p)
	a, b := f.input(false, 0), f.input(false, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	writer, waiter, release := initializationMySQLPauseUpdate(t, f, model.TableNameExperiment)
	var wg sync.WaitGroup
	defer func() { cancel(); release(); wg.Wait() }()
	var changed [2]bool
	var errs [2]error
	wg.Add(1)
	go func() {
		defer wg.Done()
		changed[0], errs[0] = r.CreateRunWithoutHooks(context.WithValue(ctx, initializationMySQLActor{}, "first"), a.RunLog, 0, "")
	}()
	configWait(t, writer)
	initializationMySQLRequireParentLocked(t, f, ctx)
	// The inserted RunLog and unpublished Latest must remain invisible together.
	require.Zero(t, f.latest(t))
	initializationMySQLRequireRows(t, f, 0)
	wg.Add(1)
	go func() {
		defer wg.Done()
		changed[1], errs[1] = r.CreateRunWithoutHooks(context.WithValue(ctx, initializationMySQLActor{}, "second"), b.RunLog, 0, "")
	}()
	configWait(t, waiter)
	release()
	wg.Wait()
	require.NoError(t, errs[0])
	require.True(t, changed[0])
	require.ErrorIs(t, errs[1], entity.ErrHookStoreConflict)
	require.False(t, changed[1])
	require.Equal(t, a.Key.RunID, f.latest(t))
	initializationMySQLRequireRows(t, f, 1)
	log := initializationMySQLLog(t, f, a.Key)
	require.Equal(t, a.RunLog.CreatedBy, log.CreatedBy)
	require.Nil(t, log.LifecycleHookVersion)
	var loserCount int64
	require.NoError(t, hookRunScope(f.sql.Model(&model.ExptRunLog{}), b.Key).Count(&loserCount).Error)
	require.Zero(t, loserCount)
}

func TestHookInitializationMySQLRollbackAndDisabledReplay(t *testing.T) {
	t.Run("rollback after latest write", func(t *testing.T) {
		f := newHookTxFixture(t)
		in := f.input(false, 0)
		name := fmt.Sprintf("init_mysql_rollback_%d", f.expt)
		writes := 0
		require.NoError(t, f.sql.Callback().Update().After("gorm:update").Register(name, func(tx *gorm.DB) {
			if tx.Error == nil && tx.Statement.Table == model.TableNameExperiment && tx.Statement.Context.Value(initializationMySQLActor{}) == "rollback" {
				writes++
				tx.AddError(errors.New("fixture latest write failure"))
			}
		}))
		t.Cleanup(func() { require.NoError(t, f.sql.Callback().Update().Remove(name)) })
		changed, err := NewHookRunInitializationRepo(f.p).CreateRunWithoutHooks(context.WithValue(context.Background(), initializationMySQLActor{}, "rollback"), in.RunLog, 0, "")
		require.Error(t, err)
		require.False(t, changed)
		require.Equal(t, 1, writes, "failure must occur after the actual Latest SQL, not before RunLog insertion")
		require.Zero(t, f.latest(t))
		initializationMySQLRequireRows(t, f, 0)
	})
	t.Run("disabled config and same run replay", func(t *testing.T) {
		f := newHookTxFixture(t)
		ctx := context.Background()
		configs := NewHookConfigRepo(f.p, configTestCodec(t))
		changed, err := configs.UpdateConfig(ctx, configOwner(f), entity.HookConfigUpdateInput{KeyID: "test-key", Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false)}, After: &entity.HookConfig{Enabled: gptr.Of(false)}}})
		require.NoError(t, err)
		require.True(t, changed)
		in := f.input(false, 0)
		in.RunLog.ItemIds = []entity.ExptRunLogItems{{ItemIDs: []int64{71}, CreateAt: gptr.Of(int64(123))}}
		r := NewHookRunInitializationRepo(f.p)
		initial, err := r.ReadRunInitialization(ctx, in.Key)
		require.NoError(t, err)
		require.False(t, initial.HooksEnabled)
		require.NotEmpty(t, initial.ConfigRevision)
		raw := configRaw(t, f, configOwner(f))
		changed, err = r.CreateRunWithoutHooks(ctx, in.RunLog, 0, initial.ConfigRevision)
		require.NoError(t, err)
		require.True(t, changed)
		original := initializationMySQLLog(t, f, in.Key)
		changed, err = r.CreateRunWithoutHooks(ctx, in.RunLog, 0, initial.ConfigRevision)
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, original, initializationMySQLLog(t, f, in.Key))
		require.Equal(t, raw, configRaw(t, f, configOwner(f)))
		require.Equal(t, in.Key.RunID, f.latest(t))
		initializationMySQLRequireRows(t, f, 1)
	})
}

func TestHookInitializationMySQLConfigCreateExclusion(t *testing.T) {
	for _, configFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("config_first=%v", configFirst), func(t *testing.T) {
			f := newHookTxFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			r := NewHookRunInitializationRepo(f.p)
			configs := NewHookConfigRepo(f.p, configTestCodec(t))
			in := f.input(false, 0)
			initial, err := r.ReadRunInitialization(ctx, in.Key)
			require.NoError(t, err)
			require.False(t, initial.HooksEnabled)
			writer, waiter, release := initializationMySQLPauseUpdate(t, f, model.TableNameExperiment)
			var wg sync.WaitGroup
			defer func() { cancel(); release(); wg.Wait() }()
			var configChanged, runChanged bool
			var configErr, runErr error
			update := func(actor string) {
				configChanged, configErr = configs.UpdateConfig(context.WithValue(ctx, initializationMySQLActor{}, actor), configOwner(f), entity.HookConfigUpdateInput{Config: configBefore(), KeyID: "test-key", ExpectedRevision: initial.ConfigRevision})
			}
			create := func(actor string) {
				runChanged, runErr = r.CreateRunWithoutHooks(context.WithValue(ctx, initializationMySQLActor{}, actor), in.RunLog, initial.LatestRunID, initial.ConfigRevision)
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				if configFirst {
					update("first")
				} else {
					create("first")
				}
			}()
			configWait(t, writer)
			initializationMySQLRequireParentLocked(t, f, ctx)
			wg.Add(1)
			go func() {
				defer wg.Done()
				if configFirst {
					create("second")
				} else {
					update("second")
				}
			}()
			configWait(t, waiter)
			release()
			wg.Wait()
			require.NotEqual(t, configChanged, runChanged)
			if configFirst {
				require.True(t, configChanged)
				require.NoError(t, configErr)
				require.ErrorIs(t, runErr, entity.ErrHookStoreConflict)
				require.Zero(t, f.latest(t))
				current, err := r.ReadRunInitialization(ctx, in.Key)
				require.NoError(t, err)
				require.True(t, current.HooksEnabled)
				require.NotEqual(t, initial.ConfigRevision, current.ConfigRevision)
				for _, revision := range []string{initial.ConfigRevision, current.ConfigRevision} {
					changed, err := r.CreateRunWithoutHooks(ctx, in.RunLog, 0, revision)
					require.ErrorIs(t, err, entity.ErrHookStoreConflict)
					require.False(t, changed)
				}
				initializationMySQLRequireRows(t, f, 0)
			} else {
				require.True(t, runChanged)
				require.NoError(t, runErr)
				require.ErrorIs(t, configErr, entity.ErrHookConfigImmutable)
				require.Equal(t, in.Key.RunID, f.latest(t))
				require.Empty(t, configRaw(t, f, configOwner(f)))
				initializationMySQLRequireRows(t, f, 1)
			}
		})
	}
}

func TestHookInitializationMySQLAppendCancelExclusion(t *testing.T) {
	for _, appendFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("append_first=%v", appendFirst), func(t *testing.T) {
			f := newHookTxFixture(t)
			in := f.input(true, 0)
			in.RunLog.CreatedBy = "original-author"
			in.RunLog.Mode = int32(entity.EvaluationModeRetryItems)
			in.RunLog.ItemIds = []entity.ExptRunLogItems{{ItemIDs: []int64{71}, CreateAt: gptr.Of(int64(123))}}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			created, err := f.repo.CreateRunWithHooks(ctx, in)
			require.NoError(t, err)
			originalLog := initializationMySQLLog(t, f, in.Key)
			r := NewHookRunInitializationRepo(f.p)
			appendInput := entity.HookAppendRunItemsInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: created.Run.Version}, ExecutionScope: "local", ItemIDs: []int64{72}}
			finalizeInput := entity.HookFinalizeInput{HookStoreGuard: appendInput.HookStoreGuard, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}}
			table := model.TableNameExptLifecycleHookRun
			if appendFirst {
				table = model.TableNameExptRunLog
			}
			writer, waiter, release := initializationMySQLPauseUpdate(t, f, table)
			var wg sync.WaitGroup
			defer func() { cancel(); release(); wg.Wait() }()
			var appendErr, finalizeErr error
			appendItems := func(actor string) {
				appendErr = r.AppendHookRunItems(context.WithValue(ctx, initializationMySQLActor{}, actor), appendInput)
			}
			finalize := func(actor string) {
				_, finalizeErr = f.repo.BeginFinalize(context.WithValue(ctx, initializationMySQLActor{}, actor), finalizeInput)
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				if appendFirst {
					appendItems("first")
				} else {
					finalize("first")
				}
			}()
			configWait(t, writer)
			initializationMySQLRequireParentLocked(t, f, ctx)
			wg.Add(1)
			go func() {
				defer wg.Done()
				if appendFirst {
					finalize("second")
				} else {
					appendItems("second")
				}
			}()
			configWait(t, waiter)
			release()
			wg.Wait()
			afterRace, err := f.repo.GetRun(ctx, in.Key)
			require.NoError(t, err)
			require.Equal(t, created.Run.Version+1, afterRace.Version)
			if appendFirst {
				require.NoError(t, appendErr)
				require.ErrorIs(t, finalizeErr, entity.ErrHookStoreConflict)
				require.Equal(t, created.Run.State.Before, afterRace.State.Before)
				require.Equal(t, created.Run.State.After, afterRace.State.After)
				require.Equal(t, created.Run.Operations, afterRace.Operations)
				finalizeInput.ExpectedVersion = afterRace.Version
				_, err = f.repo.BeginFinalize(ctx, finalizeInput)
				require.NoError(t, err)
			} else {
				require.NoError(t, finalizeErr)
				require.ErrorIs(t, appendErr, entity.ErrHookStoreConflict)
			}
			cancelled, err := f.repo.GetRun(ctx, in.Key)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateClosed, cancelled.State.Gate)
			require.Equal(t, entity.HookFinalizePending, cancelled.State.Finalize)
			require.Equal(t, entity.HookOperationFailed, cancelled.State.Before.Status)
			require.Equal(t, created.Run.State.After, cancelled.State.After)
			require.Equal(t, created.Run.CreatedBy, cancelled.CreatedBy)
			require.Equal(t, created.Run.Snapshot, cancelled.Snapshot)
			require.Len(t, cancelled.Operations, 2)
			for i := range created.Run.Operations {
				require.Equal(t, created.Run.Operations[i].Phase, cancelled.Operations[i].Phase)
				require.Equal(t, created.Run.Operations[i].HookOperationSeed, cancelled.Operations[i].HookOperationSeed)
			}
			log := initializationMySQLLog(t, f, in.Key)
			require.Equal(t, "original-author", log.CreatedBy)
			require.Equal(t, gptr.Of(int32(1)), log.LifecycleHookVersion)
			require.Equal(t, originalLog.CreatedAt, log.CreatedAt)
			do, err := convert.NewExptRunLogConvertor().PO2DO(&log)
			require.NoError(t, err)
			if appendFirst {
				require.Equal(t, []int64{71, 72}, do.GetItemIDs())
			} else {
				require.Equal(t, []int64{71}, do.GetItemIDs())
			}
			require.Equal(t, in.RunLog.ItemIds[0], do.ItemIds[0])
			appendInput.ExpectedVersion = cancelled.Version
			appendInput.ItemIDs = []int64{73}
			require.ErrorIs(t, r.AppendHookRunItems(ctx, appendInput), entity.ErrHookAdmissionDenied)
			require.Equal(t, log, initializationMySQLLog(t, f, in.Key))
			afterDenied, err := f.repo.GetRun(ctx, in.Key)
			require.NoError(t, err)
			require.Equal(t, cancelled, afterDenied)
			require.Equal(t, in.Key.RunID, f.latest(t))
		})
	}
}
