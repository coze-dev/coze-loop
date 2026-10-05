// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func resultReadKey() entity.HookRunKey {
	return entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
}

func resultReadSQL(table string) string {
	return "^" + regexp.QuoteMeta("SELECT `id` FROM `"+table+"` WHERE (space_id=? AND expt_id=?) AND `"+table+"`.`deleted_at` IS NULL LIMIT ?") + "$"
}

func resultReadLoad(t *testing.T, m sqlmock.Sqlmock, f planReadFixture) {
	t.Helper()
	m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WithArgs(int64(20), int64(10), 1).WillReturnRows(budgetProjectionRow(t, f.expt))
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.*FOR UPDATE").WithArgs(int64(10), int64(20), int64(30), 1).WillReturnRows(budgetProjectionRow(t, f.life))
	m.ExpectQuery("SELECT .*FROM .expt_run_log.*FOR UPDATE").WithArgs(int64(10), int64(20), int64(30), 1).WillReturnRows(budgetProjectionRow(t, f.log))
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_hook_run.*FOR UPDATE").WithArgs(int64(10), int64(20), int64(30)).WillReturnRows(budgetProjectionRow(t, f.op))
}

func resultReadRows(exists bool) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id"})
	if exists {
		rows.AddRow(91)
	}
	return rows
}

func TestHookPlanResultReaderSQLPresence(t *testing.T) {
	for _, tc := range []struct {
		name       string
		item, turn bool
	}{
		{"no_base_results", false, false}, {"item_short_circuits", true, false}, {"only_turn_result", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, m := initializationDB(t)
			reader := NewHookPlanResultReader(planMasterProvider{p, t})
			f := planFixture()
			f.life.PlanCount = 9000
			m.ExpectBegin()
			resultReadLoad(t, m, f)
			m.ExpectQuery(resultReadSQL("expt_item_result")).WithArgs(int64(10), int64(20), 1).WillReturnRows(resultReadRows(tc.item))
			if !tc.item {
				m.ExpectQuery(resultReadSQL("expt_turn_result")).WithArgs(int64(10), int64(20), 1).WillReturnRows(resultReadRows(tc.turn))
			}
			m.ExpectCommit()
			got, err := reader.HasExperimentResults(context.Background(), resultReadKey(), "local")
			require.NoError(t, err)
			require.Equal(t, tc.item || tc.turn, got)
		})
	}
}

func TestHookPlanResultReaderSQLHistoricalRun(t *testing.T) {
	p, m := initializationDB(t)
	f := planFixture()
	f.expt.LatestRunID = 99
	f.life.Gate = 2
	f.life.FinalizeState = 2
	f.life.TerminalStatus = gptr.Of(int32(13))
	f.life.TerminalAt = gptr.Of(time.Unix(100, 0))
	f.log.Status = gptr.Of(int64(13))
	f.op.Status = "failed"
	m.ExpectBegin()
	resultReadLoad(t, m, f)
	m.ExpectQuery(resultReadSQL("expt_item_result")).WithArgs(int64(10), int64(20), 1).WillReturnRows(resultReadRows(true))
	m.ExpectCommit()
	got, err := NewHookPlanResultReader(p).HasExperimentResults(context.Background(), resultReadKey(), "local")
	require.NoError(t, err)
	require.True(t, got)
}

func TestHookPlanResultReaderSQLMissingParentIsNotAbsent(t *testing.T) {
	p, m := initializationDB(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WithArgs(int64(20), int64(10), 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectRollback()
	got, err := NewHookPlanResultReader(p).HasExperimentResults(context.Background(), resultReadKey(), "local")
	require.ErrorIs(t, err, entity.ErrHookStoreMissing)
	require.False(t, got)
}

func TestHookPlanResultReaderSQLInputAndCancellation(t *testing.T) {
	p, _ := initializationDB(t)
	reader := NewHookPlanResultReader(p)
	for _, key := range []entity.HookRunKey{{}, {WorkspaceID: 10, ExperimentID: 20}, {WorkspaceID: -1, ExperimentID: 20, RunID: 30}} {
		got, err := reader.HasExperimentResults(context.Background(), key, "local")
		require.Error(t, err)
		require.False(t, got)
	}
	for _, scope := range []string{"", "bad scope", "\n", "中文", strings.Repeat("a", 129)} {
		got, err := reader.HasExperimentResults(context.Background(), resultReadKey(), scope)
		require.Error(t, err)
		require.False(t, got)
	}
	got, err := reader.HasExperimentResults(nil, resultReadKey(), "local")
	require.Error(t, err)
	require.False(t, got)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err = reader.HasExperimentResults(ctx, resultReadKey(), "local")
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, got)
	for _, provider := range []db.Provider{nil, (*planMasterProvider)(nil)} {
		got, err := NewHookPlanResultReader(provider).HasExperimentResults(context.Background(), resultReadKey(), "local")
		require.ErrorIs(t, err, entity.ErrHookPlanStorage)
		require.False(t, got)
	}
}

func TestHookPlanResultReaderSQLScopeAndMarker(t *testing.T) {
	for _, tc := range []struct {
		name   string
		marker *int32
		scope  string
		want   error
	}{
		{"scope_mismatch", gptr.Of(int32(1)), "other", entity.ErrHookStoreConflict},
		{"legacy_marker_nil", nil, "local", entity.ErrHookStoreCorrupt},
		{"legacy_marker_zero", gptr.Of(int32(0)), "local", entity.ErrHookStoreCorrupt},
		{"unknown_marker", gptr.Of(int32(2)), "local", entity.ErrHookStoreCorrupt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, m := initializationDB(t)
			f := planFixture()
			f.log.LifecycleHookVersion = tc.marker
			m.ExpectBegin()
			m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnRows(budgetProjectionRow(t, f.expt))
			m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.*FOR UPDATE").WillReturnRows(budgetProjectionRow(t, f.life))
			m.ExpectQuery("SELECT .*FROM .expt_run_log.*FOR UPDATE").WillReturnRows(budgetProjectionRow(t, f.log))
			if tc.name == "scope_mismatch" {
				m.ExpectQuery("SELECT .*FROM .expt_lifecycle_hook_run.*FOR UPDATE").WillReturnRows(budgetProjectionRow(t, f.op))
			}
			m.ExpectRollback()
			got, err := NewHookPlanResultReader(p).HasExperimentResults(context.Background(), resultReadKey(), tc.scope)
			require.ErrorIs(t, err, tc.want)
			require.False(t, got)
		})
	}
}

func TestHookPlanResultReaderSQLErrorsNeverMeanAbsent(t *testing.T) {
	injected := errors.New("driver detail must not escape")
	for _, stage := range []string{"begin", "parent", "item", "turn", "commit", "cancelled_query"} {
		t.Run(stage, func(t *testing.T) {
			p, m := initializationDB(t)
			if stage == "begin" {
				m.ExpectBegin().WillReturnError(injected)
			} else {
				m.ExpectBegin()
				if stage == "parent" {
					m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnError(injected)
				} else {
					resultReadLoad(t, m, planFixture())
					q := m.ExpectQuery(resultReadSQL("expt_item_result")).WithArgs(int64(10), int64(20), 1)
					switch stage {
					case "item":
						q.WillReturnError(injected)
					case "cancelled_query":
						q.WillReturnError(context.Canceled)
					case "turn":
						q.WillReturnRows(resultReadRows(false))
						m.ExpectQuery(resultReadSQL("expt_turn_result")).WithArgs(int64(10), int64(20), 1).WillReturnError(injected)
					case "commit":
						q.WillReturnRows(resultReadRows(true))
					}
				}
				if stage == "commit" {
					m.ExpectCommit().WillReturnError(injected)
				} else {
					m.ExpectRollback()
				}
			}
			got, err := NewHookPlanResultReader(p).HasExperimentResults(context.Background(), resultReadKey(), "local")
			require.ErrorIs(t, err, entity.ErrHookPlanStorage)
			require.False(t, got)
			require.NotContains(t, err.Error(), "driver detail")
		})
	}
}

func TestHookPlanResultReaderSQLRealPrimaryTransaction(t *testing.T) {
	primary, pm, err := sqlmock.New()
	require.NoError(t, err)
	replica, rm, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pm.ExpectationsWereMet())
		require.NoError(t, rm.ExpectationsWereMet())
		_ = primary.Close()
		_ = replica.Close()
	})
	p, err := db.NewDB(mysql.New(mysql.Config{Conn: primary, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, p.NewSession(context.Background()).Use(dbresolver.Register(dbresolver.Config{Replicas: []gorm.Dialector{mysql.New(mysql.Config{Conn: replica, SkipInitializeWithVersion: true})}})))
	rm.ExpectBegin()
	rm.ExpectQuery("SELECT 7").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(7))
	rm.ExpectCommit()
	require.NoError(t, p.NewSession(context.Background()).Clauses(dbresolver.Read).Transaction(func(tx *gorm.DB) error {
		var n int
		err := tx.Raw("SELECT 7").Scan(&n).Error
		require.Equal(t, 7, n)
		return err
	}))
	pm.ExpectBegin()
	resultReadLoad(t, pm, planFixture())
	pm.ExpectQuery(resultReadSQL("expt_item_result")).WithArgs(int64(10), int64(20), 1).WillReturnRows(resultReadRows(false))
	pm.ExpectQuery(resultReadSQL("expt_turn_result")).WithArgs(int64(10), int64(20), 1).WillReturnRows(resultReadRows(true))
	pm.ExpectCommit()
	got, err := NewHookPlanResultReader(planMasterProvider{p, t}).HasExperimentResults(context.Background(), resultReadKey(), "local")
	require.NoError(t, err)
	require.True(t, got)
}
