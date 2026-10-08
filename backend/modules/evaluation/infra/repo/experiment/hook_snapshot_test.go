// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type snapshotExecutorFunc func(context.Context, func(*gorm.DB) error) error

func (f snapshotExecutorFunc) ReadSnapshot(ctx context.Context, fn func(*gorm.DB) error) error {
	return f(ctx, fn)
}

func TestHookSnapshotWrapperRejectsNilDependencies(t *testing.T) {
	base, _ := initializationDB(t)
	var nilBase *snapshotCapabilityProvider
	var nilExecutor snapshotExecutorFunc
	for _, tc := range []struct {
		name     string
		base     db.Provider
		executor HookSnapshotExecutor
	}{
		{"nil_base", nil, snapshotExecutorFunc(func(context.Context, func(*gorm.DB) error) error { return nil })},
		{"typed_nil_base", nilBase, snapshotExecutorFunc(func(context.Context, func(*gorm.DB) error) error { return nil })},
		{"nil_executor", base, nil},
		{"typed_nil_executor", base, nilExecutor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := WithHookSnapshotExecutor(tc.base, tc.executor)
			require.Error(t, err)
			require.Nil(t, p)
		})
	}
}

func TestHookSnapshotWrapperPassesOrdinaryWorkThrough(t *testing.T) {
	base, m := initializationDB(t)
	failure := errors.New("snapshot executor failure")
	calls := 0
	p, err := WithHookSnapshotExecutor(base, snapshotExecutorFunc(func(context.Context, func(*gorm.DB) error) error { calls++; return failure }))
	require.NoError(t, err)
	ctx := context.Background()
	session := base.NewSession(ctx)
	forwarded := p.NewSession(ctx, db.WithMaster(), db.WithTransaction(session))
	require.Same(t, session.Statement.ConnPool, forwarded.Statement.ConnPool)
	m.ExpectQuery("SELECT 41").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(41))
	var value int
	require.NoError(t, forwarded.Raw("SELECT 41").Scan(&value).Error)
	require.Equal(t, 41, value)
	m.ExpectBegin()
	m.ExpectQuery("SELECT 42").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(42))
	m.ExpectCommit()
	require.NoError(t, p.Transaction(ctx, func(tx *gorm.DB) error { return tx.Raw("SELECT 42").Scan(&value).Error }, db.WithMaster()))
	require.Equal(t, 42, value)
	require.Zero(t, calls, "ordinary provider operations must never enter the snapshot executor")
	executor, ok := p.(HookSnapshotExecutor)
	require.True(t, ok, "wrapper must retain the injected capability")
	require.ErrorIs(t, executor.ReadSnapshot(ctx, func(*gorm.DB) error { t.Fatal("callback must not run on executor error"); return nil }), failure)
	require.Equal(t, 1, calls)
}

func TestHookSnapshotHelperFailureAndContextDoNotFallback(t *testing.T) {
	base, _ := initializationDB(t)
	failure := errors.New("executor unavailable")
	calls := 0
	p, err := WithHookSnapshotExecutor(base, snapshotExecutorFunc(func(context.Context, func(*gorm.DB) error) error { calls++; return failure }))
	require.NoError(t, err)
	fn := func(*gorm.DB) error { t.Fatal("failed/cancelled snapshot must not execute callback"); return nil }
	require.ErrorIs(t, readHookSnapshot(context.Background(), p, fn), failure)
	require.Equal(t, 1, calls)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, readHookSnapshot(ctx, p, fn), context.Canceled)
	require.Equal(t, 1, calls)
	var nilProvider *snapshotCapabilityProvider
	for _, invalid := range []db.Provider{nil, nilProvider} {
		require.Error(t, readHookSnapshot(context.Background(), invalid, fn))
	}
	require.Error(t, readHookSnapshot(nil, p, fn))
	require.Error(t, readHookSnapshot(context.Background(), p, nil))
}

func TestHookSnapshotFallbackRetainsOptionsAndSuppressesSQL(t *testing.T) {
	base, m, pool := hookSnapshotDB(t)
	var output bytes.Buffer
	p := &gateLoggedProvider{Provider: base, output: logger.New(log.New(&output, "", 0), logger.Config{LogLevel: logger.Info})}
	m.ExpectBegin()
	m.ExpectQuery("SELECT private_parameter").WillReturnError(errors.New("sensitive SQL error"))
	m.ExpectRollback()
	err := readHookSnapshot(context.Background(), p, func(tx *gorm.DB) error { return tx.Raw("SELECT private_parameter").Scan(new(int)).Error })
	require.Error(t, err)
	require.Equal(t, []sql.TxOptions{{Isolation: sql.LevelRepeatableRead, ReadOnly: true}}, pool.options)
	require.Empty(t, output.String())
}

func TestHookSnapshotInjectedCallbackKeepsErrorAndSuppressesSQL(t *testing.T) {
	base, m := initializationDB(t)
	var output bytes.Buffer
	loud := &gateLoggedProvider{Provider: base, output: logger.New(log.New(&output, "", 0), logger.Config{LogLevel: logger.Info})}
	p, err := WithHookSnapshotExecutor(base, snapshotExecutorFunc(func(ctx context.Context, fn func(*gorm.DB) error) error { return fn(loud.NewSession(ctx)) }))
	require.NoError(t, err)
	failure := errors.New("sensitive callback SQL")
	m.ExpectQuery("SELECT private_parameter").WillReturnError(failure)
	err = readHookSnapshot(context.Background(), p, func(tx *gorm.DB) error { return tx.Raw("SELECT private_parameter").Scan(new(int)).Error })
	require.ErrorIs(t, err, failure)
	require.Empty(t, output.String())
}

func TestHookSnapshotRejectsNilSessionAndLateCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		base, _ := initializationDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		p, err := WithHookSnapshotExecutor(base, snapshotExecutorFunc(func(ctx context.Context, fn func(*gorm.DB) error) error {
			if cancelled {
				cancel()
				return nil
			}
			return fn(nil)
		}))
		require.NoError(t, err)
		err = readHookSnapshot(ctx, p, func(*gorm.DB) error { t.Fatal("invalid snapshot must not reach consumer"); return nil })
		if cancelled {
			require.ErrorIs(t, err, context.Canceled)
		} else {
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
		}
		cancel()
	}
}

func TestHookSnapshotLegacyReadersDoNotUseExecutor(t *testing.T) {
	base, m := initializationDB(t)
	calls := 0
	p, err := WithHookSnapshotExecutor(base, snapshotExecutorFunc(func(context.Context, func(*gorm.DB) error) error { calls++; return errors.New("must not be used") }))
	require.NoError(t, err)
	m.ExpectQuery("SELECT .* FROM expt_run_log AS l").WithArgs(int64(30)).WillReturnRows(legacyGateRows(nil))
	gate, err := NewHookGateRepo(p, nil).CanDispatch(context.Background(), gateTestKey)
	require.NoError(t, err)
	require.Equal(t, entity.HookGateReady, gate.Gate)
	m.ExpectQuery("SELECT .* FROM expt_run_log AS l").WithArgs(int64(30)).WillReturnRows(legacySummaryRows(nil))
	summaries, err := NewHookSummaryRepo(p).MGetSummaries(context.Background(), []entity.HookRunKey{gateTestKey})
	require.NoError(t, err)
	require.Contains(t, summaries, gateTestKey)
	require.Nil(t, summaries[gateTestKey])
	m.ExpectQuery("SELECT .* FROM experiment AS e LEFT JOIN expt_run_log AS l").WithArgs(int64(30), int64(20), int64(10)).WillReturnRows(legacySourceRows(nil))
	source, err := NewHookFinalizationRepo(p).ReadFinalizationSource(context.Background(), gateTestKey)
	require.NoError(t, err)
	require.False(t, source.Managed)
	require.Equal(t, gateTestKey, source.Key)
	require.Zero(t, calls)
}

func TestHookSnapshotBoundRepositoriesKeepCapability(t *testing.T) {
	f := newBatchDeletionFixture(t)
	run := f.run(20, 30, entity.EvaluationModeFailRetry, true, false)
	snapshot, err := f.codec.DecodeSnapshot(context.Background(), run.State.Key, "test-scope", run.Snapshot)
	require.NoError(t, err)
	binding, err := entity.NewHookExecutionInitializationBinding(run, snapshot)
	require.NoError(t, err)
	calls := 0
	failure := errors.New("bound snapshot unavailable")
	p, err := WithHookSnapshotExecutor(f.p, snapshotExecutorFunc(func(context.Context, func(*gorm.DB) error) error { calls++; return failure }))
	require.NoError(t, err)
	bound, err := NewBoundHookRuntimeRepositories(p, binding)
	require.NoError(t, err)
	gate, err := bound.Gate.CanDispatch(context.Background(), run.State.Key)
	require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
	require.Equal(t, entity.HookGateWaiting, gate.Gate)
	source, err := bound.Finalization.ReadFinalizationSource(context.Background(), run.State.Key)
	require.ErrorIs(t, err, failure)
	require.Nil(t, source)
	stats, err := bound.Finalization.ReadFinalizationStats(context.Background(), run.State.Key, "test-scope")
	require.ErrorIs(t, err, failure)
	require.Nil(t, stats)
	require.Equal(t, 3, calls)
	require.NoError(t, f.mock.ExpectationsWereMet())
}

type snapshotCapabilityProvider struct {
	db.Provider
	calls int
}

func (p *snapshotCapabilityProvider) ReadSnapshot(ctx context.Context, fn func(*gorm.DB) error) error {
	p.calls++
	return p.Provider.NewSession(ctx, db.WithMaster()).Transaction(fn, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}

func TestHookSnapshotManagedReadersUseCapability(t *testing.T) {
	t.Run("gate", func(t *testing.T) {
		base, m := gateMock(t)
		p := &snapshotCapabilityProvider{Provider: base}
		gateExpectLog(m, gateLogValues(int64(1)))
		m.ExpectQuery("SELECT .* FROM expt_lifecycle_run AS l").WithArgs(int64(10), int64(30), 3).WillReturnRows(gateManagedRows(gateManagedRow()))
		m.ExpectCommit()
		out, err := NewHookGateRepo(p, func(context.Context) (string, error) { return "platform-local", nil }).CanDispatch(context.Background(), gateTestKey)
		require.NoError(t, err)
		require.Equal(t, entity.HookGateWaiting, out.Gate)
		require.Equal(t, 1, p.calls, "managed gate must execute all snapshot reads through the injected capability")
	})
	t.Run("mixed_summary", func(t *testing.T) {
		base, m := initializationDB(t)
		p := &snapshotCapabilityProvider{Provider: base}
		columns := []string{"id", "space_id", "expt_id", "expt_run_id", "lifecycle_hook_version", "deleted_at", "status"}
		rows := func() *sqlmock.Rows {
			return sqlmock.NewRows(columns).AddRow(30, 10, 20, 30, 1, nil, 3).AddRow(31, 10, 20, 31, nil, nil, 3)
		}
		m.ExpectQuery("SELECT .* FROM expt_run_log AS l").WithArgs(int64(30), int64(31)).WillReturnRows(rows())
		m.ExpectBegin()
		m.ExpectQuery("SELECT .* FROM expt_run_log AS l").WithArgs(int64(30), int64(31)).WillReturnRows(rows())
		m.ExpectQuery("SELECT .* FROM expt_lifecycle_run AS l").WithArgs(int64(10), int64(30), 3).WillReturnRows(gateManagedRows(gateManagedRow()))
		m.ExpectCommit()
		legacy := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 31}
		out, err := NewHookSummaryRepo(p).MGetSummaries(context.Background(), []entity.HookRunKey{gateTestKey, legacy})
		require.NoError(t, err)
		require.Len(t, out, 2)
		require.Nil(t, out[legacy])
		require.Equal(t, entity.HookOperationPending, out[gateTestKey].Before.Status)
		require.Equal(t, 1, p.calls, "mixed batch must share one complete snapshot callback")
	})
	t.Run("finalization_source", func(t *testing.T) {
		base, m := initializationDB(t)
		p := &snapshotCapabilityProvider{Provider: base}
		m.ExpectQuery("SELECT .* FROM experiment AS e LEFT JOIN expt_run_log AS l").WithArgs(int64(20), int64(10)).WillReturnRows(legacySourceRows(int64(1)))
		m.ExpectBegin()
		m.ExpectQuery("SELECT .* FROM .experiment.").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "name"}).AddRow(20, 10, 31, "snapshot latest"))
		m.ExpectQuery("SELECT .* FROM .expt_run_log.").WithArgs(int64(31), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "lifecycle_hook_version", "status", "mode"}).AddRow(31, 10, 20, 31, 1, 3, 1))
		m.ExpectCommit()
		out, err := NewHookFinalizationRepo(p).ReadFinalizationSource(context.Background(), entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20})
		require.NoError(t, err)
		require.True(t, out.Managed)
		require.Equal(t, int64(31), out.Key.RunID)
		require.Equal(t, "snapshot latest", out.Experiment.Name)
		require.Equal(t, 1, p.calls, "source and run must come from the executor snapshot")
	})
}
