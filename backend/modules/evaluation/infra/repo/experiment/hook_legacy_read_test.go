// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func legacyGateRows(marker driver.Value) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "lifecycle_hook_version", "deleted_at", "status", "experiment_id", "experiment_space_id", "latest_run_id", "experiment_status", "experiment_deleted_at"}).AddRow(gateLogValues(marker)...)
}

func TestHookLegacyGateWithoutTransaction(t *testing.T) {
	for _, marker := range []driver.Value{nil, int64(0)} {
		t.Run(fmt.Sprint(marker), func(t *testing.T) {
			p, m := gateMock(t)
			m.ExpectQuery("SELECT .* FROM expt_run_log AS l LEFT JOIN experiment AS e .* WHERE l.id=\\?").WithArgs(int64(30)).WillReturnRows(legacyGateRows(marker))
			got, err := NewHookGateRepo(p, nil).CanDispatch(context.Background(), gateTestKey)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateReady, got.Gate)
		})
	}
}

func legacySummaryRows(marker driver.Value) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "lifecycle_hook_version", "deleted_at", "status"}).AddRow(30, 10, 20, 30, marker, nil, 3)
}

func TestHookLegacySummaryWithoutTransaction(t *testing.T) {
	for _, marker := range []driver.Value{nil, int64(0)} {
		t.Run(fmt.Sprint(marker), func(t *testing.T) {
			p, m := initializationDB(t)
			m.ExpectQuery("SELECT .* FROM expt_run_log AS l JOIN experiment AS e .* WHERE l.id IN").WithArgs(int64(30)).WillReturnRows(legacySummaryRows(marker))
			got, err := NewHookSummaryRepo(p).MGetSummaries(context.Background(), []entity.HookRunKey{gateTestKey})
			require.NoError(t, err)
			require.Equal(t, map[entity.HookRunKey]*entity.LifecycleHookRunSummary{gateTestKey: nil}, got)
		})
	}
}

func legacySourceRows(marker driver.Value, changes ...map[string]driver.Value) *sqlmock.Rows {
	columns := []string{"id", "space_id", "latest_run_id", "name", "description", "status", "eval_set_id", "eval_set_version_id", "target_id", "created_by", "eval_conf", "run_id", "run_space_id", "run_expt_id", "run_expt_run_id", "run_lifecycle_hook_version", "run_status", "run_mode", "run_created_by", "run_item_ids", "run_pending_cnt", "run_success_cnt", "run_fail_cnt", "run_processing_cnt", "run_terminated_cnt", "run_credit_cost", "run_token_cost", "run_status_message", "run_created_at", "run_updated_at", "deleted_at", "run_deleted_at"}
	values := []driver.Value{20, 10, 30, "complete legacy", "retained description", 3, 41, 42, 43, "experiment-owner", []byte(`{"ItemConcurNum":2}`), 30, 10, 20, 30, marker, 3, 1, "run-owner", []byte(`[{"ItemIDs":[91,92]}]`), 2, 3, 4, 5, 6, 1.25, 78, []byte("status detail"), time.Unix(100, 0), time.Unix(200, 0), nil, nil}
	for _, change := range changes {
		for i, column := range columns {
			if v, ok := change[column]; ok {
				values[i] = v
			}
		}
	}
	return sqlmock.NewRows(columns).AddRow(values...)
}

func TestHookLegacySourceRejectsInvalidRows(t *testing.T) {
	for _, tc := range []struct {
		name    string
		changes map[string]driver.Value
	}{
		{"missing_run", map[string]driver.Value{"run_id": nil}},
		{"wrong_run_id", map[string]driver.Value{"run_id": 31}},
		{"wrong_run_tuple", map[string]driver.Value{"run_expt_run_id": 31}},
		{"wrong_run_space", map[string]driver.Value{"run_space_id": 11}},
		{"wrong_run_experiment", map[string]driver.Value{"run_expt_id": 21}},
		{"wrong_parent_space", map[string]driver.Value{"space_id": 11}},
		{"wrong_parent", map[string]driver.Value{"id": 21}},
		{"unknown_marker", map[string]driver.Value{"run_lifecycle_hook_version": 9}},
		{"missing_status", map[string]driver.Value{"run_status": nil}},
		{"missing_mode", map[string]driver.Value{"run_mode": nil}},
		{"deleted_run", map[string]driver.Value{"run_deleted_at": time.Now()}},
		{"deleted_parent", map[string]driver.Value{"deleted_at": time.Now()}},
		{"malformed_items", map[string]driver.Value{"run_item_ids": []byte(`bad`)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, m := initializationDB(t)
			m.ExpectQuery("SELECT .* FROM experiment AS e LEFT JOIN expt_run_log AS l").WithArgs(int64(30), int64(20), int64(10)).WillReturnRows(legacySourceRows(nil, tc.changes))
			out, err := NewHookFinalizationRepo(p).ReadFinalizationSource(context.Background(), gateTestKey)
			require.Error(t, err)
			require.Nil(t, out)
		})
	}
}

func TestHookLegacySourceKeepsHistoricalAndCancelledRun(t *testing.T) {
	p, m := initializationDB(t)
	m.ExpectQuery("SELECT .* FROM experiment AS e LEFT JOIN expt_run_log AS l ON l.id=\\?").WithArgs(int64(30), int64(20), int64(10)).WillReturnRows(legacySourceRows(nil, map[string]driver.Value{"latest_run_id": 31, "run_status": 15, "status": 15}))
	out, err := NewHookFinalizationRepo(p).ReadFinalizationSource(context.Background(), gateTestKey)
	require.NoError(t, err)
	require.Equal(t, gateTestKey, out.Key)
	require.Equal(t, int64(31), out.Experiment.LatestRunID)
	require.Equal(t, int64(15), out.RunLog.Status)
}

type hookSnapshotPool struct {
	gorm.ConnPool
	options []sql.TxOptions
}

func (p *hookSnapshotPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	if opts != nil {
		p.options = append(p.options, *opts)
	}
	return p.ConnPool.(gorm.TxBeginner).BeginTx(ctx, opts)
}

func hookSnapshotDB(t *testing.T) (db.Provider, sqlmock.Sqlmock, *hookSnapshotPool) {
	t.Helper()
	c, m, err := sqlmock.New()
	require.NoError(t, err)
	pool := &hookSnapshotPool{ConnPool: c}
	p, err := db.NewDB(mysql.New(mysql.Config{Conn: pool, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.ExpectationsWereMet()); _ = c.Close() })
	return p, m, pool
}

func TestHookLegacyProbeManagedGateRechecksCancellation(t *testing.T) {
	p, m, pool := hookSnapshotDB(t)
	m.ExpectQuery("SELECT .* FROM expt_run_log AS l").WillReturnRows(legacyGateRows(int64(1)))
	m.ExpectBegin()
	m.ExpectQuery("SELECT .* FROM expt_run_log AS l").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "lifecycle_hook_version", "status", "experiment_id", "experiment_space_id", "latest_run_id", "experiment_status"}).AddRow(30, 10, 20, 30, 1, 15, 20, 10, 30, 15))
	m.ExpectCommit()
	out, err := NewHookGateRepo(p, nil).CanDispatch(context.Background(), gateTestKey)
	require.NoError(t, err)
	require.Equal(t, entity.HookGateClosed, out.Gate)
	require.Equal(t, []sql.TxOptions{{Isolation: sql.LevelRepeatableRead, ReadOnly: true}}, pool.options)
}

func TestHookLegacyProbeMixedSummaryRereadsWholeBatch(t *testing.T) {
	p, m, pool := hookSnapshotDB(t)
	columns := []string{"id", "space_id", "expt_id", "expt_run_id", "lifecycle_hook_version", "deleted_at", "status"}
	m.ExpectQuery("SELECT .* FROM expt_run_log AS l").WithArgs(int64(30), int64(31)).WillReturnRows(sqlmock.NewRows(columns).AddRow(30, 10, 20, 30, nil, nil, 3).AddRow(31, 10, 20, 31, 1, nil, 3))
	m.ExpectBegin()
	// The legacy member disappeared after the probe; a partial managed-only reread would miss it.
	m.ExpectQuery("SELECT .* FROM expt_run_log AS l").WithArgs(int64(30), int64(31)).WillReturnRows(sqlmock.NewRows(columns).AddRow(31, 10, 20, 31, 1, nil, 3))
	m.ExpectRollback()
	out, err := NewHookSummaryRepo(p).MGetSummaries(context.Background(), []entity.HookRunKey{gateTestKey, {WorkspaceID: 10, ExperimentID: 20, RunID: 31}})
	require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
	require.Nil(t, out)
	require.Equal(t, []sql.TxOptions{{Isolation: sql.LevelRepeatableRead, ReadOnly: true}}, pool.options)
}

func TestHookLegacyProbeManagedSourceResolvesLatestAgain(t *testing.T) {
	p, m, pool := hookSnapshotDB(t)
	m.ExpectQuery("SELECT .* FROM experiment AS e LEFT JOIN expt_run_log AS l ON l.id=e.latest_run_id").WithArgs(int64(20), int64(10)).WillReturnRows(legacySourceRows(int64(1)))
	m.ExpectBegin()
	m.ExpectQuery("SELECT .* FROM .experiment.").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "name"}).AddRow(20, 10, 31, "new snapshot"))
	m.ExpectQuery("SELECT .* FROM .expt_run_log.").WithArgs(int64(31), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "lifecycle_hook_version", "status", "mode"}).AddRow(31, 10, 20, 31, 1, 3, 1))
	m.ExpectCommit()
	out, err := NewHookFinalizationRepo(p).ReadFinalizationSource(context.Background(), entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20})
	require.NoError(t, err)
	require.True(t, out.Managed)
	require.Equal(t, int64(31), out.Key.RunID)
	require.Equal(t, int64(31), out.RunLog.ExptRunID)
	require.Equal(t, "new snapshot", out.Experiment.Name)
	require.Equal(t, []sql.TxOptions{{Isolation: sql.LevelRepeatableRead, ReadOnly: true}}, pool.options)
}

func TestHookLegacyProbeReadFailuresStayErrors(t *testing.T) {
	for _, kind := range []string{"source", "summary"} {
		t.Run(kind, func(t *testing.T) {
			p, m := initializationDB(t)
			failure := errors.New("read failed")
			m.ExpectQuery("SELECT").WillReturnError(failure)
			if kind == "source" {
				out, err := NewHookFinalizationRepo(p).ReadFinalizationSource(context.Background(), gateTestKey)
				require.ErrorIs(t, err, failure)
				require.Nil(t, out)
			} else {
				out, err := NewHookSummaryRepo(p).MGetSummaries(context.Background(), []entity.HookRunKey{gateTestKey})
				require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
				require.Nil(t, out)
			}
		})
	}
}

func TestHookLegacyProbeBoundReadersKeepFence(t *testing.T) {
	t.Run("gate_rejects_legacy", func(t *testing.T) {
		p, m, pool := hookSnapshotDB(t)
		m.ExpectBegin()
		m.ExpectQuery("SELECT .* FROM expt_run_log AS l").WillReturnRows(legacyGateRows(nil))
		m.ExpectRollback()
		r := &hookGateRepo{provider: p, binding: &boundHookExecution{source: entity.HookExecutionInitializationSource{Key: gateTestKey}}}
		out, err := r.CanDispatch(context.Background(), gateTestKey)
		require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
		require.Equal(t, entity.HookGateWaiting, out.Gate)
		require.Equal(t, []sql.TxOptions{{Isolation: sql.LevelRepeatableRead, ReadOnly: true}}, pool.options)
	})
	t.Run("source_keeps_bound_key_check", func(t *testing.T) {
		p, m, pool := hookSnapshotDB(t)
		m.ExpectBegin()
		m.ExpectQuery("SELECT .* FROM .experiment.").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id"}).AddRow(20, 10, 30))
		m.ExpectRollback()
		r := &hookFinalizationRepo{provider: p, binding: &boundHookExecution{source: entity.HookExecutionInitializationSource{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 31}}}}
		out, err := r.ReadFinalizationSource(context.Background(), gateTestKey)
		require.ErrorIs(t, err, entity.ErrHookStoreConflict)
		require.Nil(t, out)
		require.Equal(t, []sql.TxOptions{{Isolation: sql.LevelRepeatableRead, ReadOnly: true}}, pool.options)
	})
}

func TestHookLegacyProbeSummaryRejectsCorruptRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows *sqlmock.Rows
	}{
		{"missing", sqlmock.NewRows([]string{"id"})},
		{"unknown_marker", legacySummaryRows(int64(9))},
		{"wrong_owner", sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id"}).AddRow(30, 10, 21, 30)},
		{"deleted", sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "deleted_at"}).AddRow(30, 10, 20, 30, time.Now())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, m := initializationDB(t)
			m.ExpectQuery("SELECT .* FROM expt_run_log AS l").WithArgs(int64(30)).WillReturnRows(tc.rows)
			out, err := NewHookSummaryRepo(p).MGetSummaries(context.Background(), []entity.HookRunKey{gateTestKey})
			require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
			require.Nil(t, out)
		})
	}
}

func TestHookLegacyProbeMixedSummaryKeepsManagedResult(t *testing.T) {
	p, m, pool := hookSnapshotDB(t)
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
	require.NotNil(t, out[gateTestKey])
	require.Equal(t, entity.HookOperationPending, out[gateTestKey].Before.Status)
	require.Equal(t, entity.HookOperationDisabled, out[gateTestKey].After.Status)
	require.Equal(t, []sql.TxOptions{{Isolation: sql.LevelRepeatableRead, ReadOnly: true}}, pool.options)
}

func TestHookLegacyFinalizationSourceWithoutTransaction(t *testing.T) {
	for _, runID := range []int64{0, 30} {
		for _, marker := range []driver.Value{nil, int64(0)} {
			t.Run(fmt.Sprintf("run=%d/marker=%v", runID, marker), func(t *testing.T) {
				p, m := initializationDB(t)
				query := "SELECT .* FROM experiment AS e LEFT JOIN expt_run_log AS l ON l.id=e.latest_run_id WHERE e.id=\\? AND e.space_id=\\?"
				args := []driver.Value{int64(20), int64(10)}
				if runID != 0 {
					query = "SELECT .* FROM experiment AS e LEFT JOIN expt_run_log AS l ON l.id=\\? WHERE e.id=\\? AND e.space_id=\\?"
					args = []driver.Value{runID, int64(20), int64(10)}
				}
				m.ExpectQuery(query).WithArgs(args...).WillReturnRows(legacySourceRows(marker))
				got, err := NewHookFinalizationRepo(p).ReadFinalizationSource(context.Background(), entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: runID})
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, gateTestKey, got.Key)
				require.False(t, got.Managed)
				require.Equal(t, "complete legacy", got.Experiment.Name)
				require.Equal(t, "retained description", got.Experiment.Description)
				require.Equal(t, int64(41), got.Experiment.EvalSetID)
				require.Equal(t, int64(42), got.Experiment.EvalSetVersionID)
				require.Equal(t, int64(43), got.Experiment.TargetID)
				require.Equal(t, "experiment-owner", got.Experiment.CreatedBy)
				require.NotNil(t, got.Experiment.EvalConf.ItemConcurNum)
				require.Equal(t, 2, *got.Experiment.EvalConf.ItemConcurNum)
				require.Equal(t, "run-owner", got.RunLog.CreatedBy)
				require.Equal(t, int32(1), got.RunLog.Mode)
				require.Equal(t, int64(3), got.RunLog.Status)
				require.Equal(t, []int64{91, 92}, got.RunLog.GetItemIDs())
				require.Equal(t, []int32{2, 3, 4, 5, 6}, []int32{got.RunLog.PendingCnt, got.RunLog.SuccessCnt, got.RunLog.FailCnt, got.RunLog.ProcessingCnt, got.RunLog.TerminatedCnt})
				require.Equal(t, 1.25, got.RunLog.CreditCost)
				require.Equal(t, int64(78), got.RunLog.TokenCost)
				require.Equal(t, []byte("status detail"), got.RunLog.StatusMessage)
				require.Equal(t, time.Unix(100, 0), got.RunLog.CreatedAt)
				require.Equal(t, time.Unix(200, 0), got.RunLog.UpdatedAt)
			})
		}
	}
}
