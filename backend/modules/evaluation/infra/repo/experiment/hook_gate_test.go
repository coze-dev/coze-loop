// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
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

var gateTestKey = entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}

func gateMock(t *testing.T) (db.Provider, sqlmock.Sqlmock) {
	t.Helper()
	c, m, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		for _, forbidden := range []string{"snapshot", "parameters", "created_by", "result", "error_message", "error_code", "lifecycle_hook_conf", "select *", "for update", "lock in share"} {
			if strings.Contains(strings.ToLower(actual), forbidden) {
				return fmt.Errorf("forbidden gate query column or lock: %s", forbidden)
			}
		}
		return sqlmock.QueryMatcherRegexp.Match(expected, actual)
	})))
	require.NoError(t, err)
	p, err := db.NewDB(mysql.New(mysql.Config{Conn: c, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.ExpectationsWereMet()); _ = c.Close() })
	return &gateMasterProvider{Provider: p, t: t}, m
}

type gateMasterProvider struct {
	db.Provider
	t *testing.T
}

func (p *gateMasterProvider) NewSession(ctx context.Context, opts ...db.Option) *gorm.DB {
	p.t.Helper()
	require.True(p.t, db.ContainWithMasterOpt(opts), "gate reads must select the primary")
	return p.Provider.NewSession(ctx, opts...)
}

type gateLoggedProvider struct {
	db.Provider
	output logger.Interface
}

func (p *gateLoggedProvider) NewSession(ctx context.Context, opts ...db.Option) *gorm.DB {
	return p.Provider.NewSession(ctx, opts...).Session(&gorm.Session{Logger: p.output})
}

func TestHookGateDoesNotLogSQLOnFailure(t *testing.T) {
	p, m := gateMock(t)
	var output bytes.Buffer
	p = &gateLoggedProvider{Provider: p, output: logger.New(log.New(&output, "", 0), logger.Config{LogLevel: logger.Info})}
	m.ExpectQuery("SELECT").WillReturnError(errors.New("SQL with private-parameter"))
	got, err := NewHookGateRepo(p, nil).CanDispatch(context.Background(), gateTestKey)
	require.Equal(t, entity.HookGateWaiting, got.Gate)
	require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
	require.Empty(t, output.String())
}

func TestHookGateInvalidKeyDoesNotRead(t *testing.T) {
	for _, key := range []entity.HookRunKey{{}, {WorkspaceID: 1, ExperimentID: 2}, {WorkspaceID: -1, ExperimentID: 2, RunID: 3}, {WorkspaceID: 1, ExperimentID: -2, RunID: 3}} {
		p, _ := gateMock(t)
		got, err := NewHookGateRepo(p, nil).CanDispatch(context.Background(), key)
		require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
		require.Equal(t, entity.HookGateWaiting, got.Gate)
	}
	got, err := NewHookGateRepo(nil, nil).CanDispatch(context.Background(), gateTestKey)
	require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
	require.Equal(t, entity.HookGateWaiting, got.Gate)
}

func TestHookGateMissingProviderDoesNotPanic(t *testing.T) {
	p, _ := gateMock(t)
	actual := p.(*gateMasterProvider).Provider
	typedNil := reflect.Zero(reflect.TypeOf(actual)).Interface().(db.Provider)
	for _, tc := range []struct {
		name     string
		provider db.Provider
	}{
		{"nil interface", nil},
		{"typed nil real provider", typedNil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got entity.HookAdmissionDecision
			var err error
			require.NotPanics(t, func() {
				got, err = NewHookGateRepo(tc.provider, nil).CanDispatch(context.Background(), gateTestKey)
			})
			require.Equal(t, entity.HookAdmissionDecision{Gate: entity.HookGateWaiting, Reason: "HOOK_STATE_UNAVAILABLE"}, got)
			require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
			require.Equal(t, entity.ErrHookGateUnavailable.Error(), err.Error())
		})
	}
}

func TestHookGateNilReceiverDoesNotPanic(t *testing.T) {
	var gate *hookGateRepo
	var got entity.HookAdmissionDecision
	var err error
	require.NotPanics(t, func() {
		got, err = gate.CanDispatch(context.Background(), gateTestKey)
	})
	require.Equal(t, entity.HookAdmissionDecision{Gate: entity.HookGateWaiting, Reason: "HOOK_STATE_UNAVAILABLE"}, got)
	require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
}

func TestHookGateLegacyValueProvider(t *testing.T) {
	p, m := gateMock(t)
	gateExpectLog(m, gateLogValues(nil))
	got, err := NewHookGateRepo(struct{ db.Provider }{p}, nil).CanDispatch(context.Background(), gateTestKey)
	require.NoError(t, err)
	require.Equal(t, entity.HookAdmissionDecision{Gate: entity.HookGateReady}, got)
}

func gateLogValues(marker driver.Value) []driver.Value {
	return []driver.Value{int64(30), int64(10), int64(20), int64(30), marker, nil, int64(3), int64(20), int64(10), int64(30), int64(3), nil}
}

func gateExpectLog(m sqlmock.Sqlmock, values []driver.Value) {
	query := func() {
		rows := sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "lifecycle_hook_version", "deleted_at", "status", "experiment_id", "experiment_space_id", "latest_run_id", "experiment_status", "experiment_deleted_at"})
		if values != nil {
			rows.AddRow(values...)
		}
		m.ExpectQuery("SELECT .* FROM expt_run_log AS l LEFT JOIN experiment AS e .* WHERE l.id=\\?").WithArgs(int64(30)).WillReturnRows(rows)
	}
	query()
	if values != nil && values[4] == int64(1) {
		m.ExpectBegin()
		query()
	}
}

func TestHookGateLegacySkipsHookAndScope(t *testing.T) {
	for _, marker := range []driver.Value{nil, int64(0)} {
		p, m := gateMock(t)
		gateExpectLog(m, gateLogValues(marker))
		got, err := NewHookGateRepo(p, func(context.Context) (string, error) {
			t.Error("legacy must not resolve scope")
			return "", errors.New("unavailable")
		}).CanDispatch(context.Background(), gateTestKey)
		require.NoError(t, err)
		require.Equal(t, entity.HookGateReady, got.Gate)
	}
}

func TestHookGateRunBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		index int
		value driver.Value
		gate  entity.HookGateState
		bad   bool
	}{
		{"wrong workspace", 1, int64(11), entity.HookGateClosed, false},
		{"wrong experiment", 2, int64(21), entity.HookGateClosed, false},
		{"wrong run", 3, int64(31), entity.HookGateClosed, false},
		{"old run", 9, int64(31), entity.HookGateClosed, false},
		{"deleted run", 5, time.Now(), entity.HookGateClosed, false},
		{"deleted experiment", 11, time.Now(), entity.HookGateClosed, false},
		{"experiment workspace", 8, int64(11), entity.HookGateClosed, false},
		{"run success", 6, int64(11), entity.HookGateClosed, false},
		{"run failed", 6, int64(12), entity.HookGateClosed, false},
		{"run terminated", 6, int64(13), entity.HookGateClosed, false},
		{"run system terminated", 6, int64(14), entity.HookGateClosed, false},
		{"run cancelling", 6, int64(15), entity.HookGateClosed, false},
		{"experiment cancelling", 10, int64(15), entity.HookGateClosed, false},
		{"experiment terminal", 10, int64(11), entity.HookGateClosed, false},
		{"run draining", 6, int64(21), entity.HookGateReady, false},
		{"experiment draining", 10, int64(21), entity.HookGateReady, false},
		{"unknown marker", 4, int64(2), entity.HookGateWaiting, true},
		{"null status", 6, nil, entity.HookGateWaiting, true},
		{"unknown status", 10, int64(99), entity.HookGateWaiting, true},
		{"missing experiment", 7, nil, entity.HookGateWaiting, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, m := gateMock(t)
			values := gateLogValues(nil)
			values[tc.index] = tc.value
			gateExpectLog(m, values)
			got, err := NewHookGateRepo(p, nil).CanDispatch(context.Background(), gateTestKey)
			if tc.bad {
				require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.gate, got.Gate)
		})
	}
}

func TestHookGateMissingAndDBFailure(t *testing.T) {
	for _, where := range []string{"begin", "log", "missing", "commit"} {
		t.Run(where, func(t *testing.T) {
			p, m := gateMock(t)
			secret := errors.New("SELECT private SQL with parameter secret-user")
			switch where {
			case "begin":
				m.ExpectQuery("SELECT").WillReturnRows(legacyGateRows(int64(1)))
				m.ExpectBegin().WillReturnError(secret)
			case "log":
				m.ExpectQuery("SELECT").WillReturnError(secret)
			case "missing":
				gateExpectLog(m, nil)
			case "commit":
				values := gateLogValues(int64(1))
				values[6] = int64(entity.ExptStatus_Success)
				gateExpectLog(m, values)
				m.ExpectCommit().WillReturnError(secret)
			}
			got, err := NewHookGateRepo(p, nil).CanDispatch(context.Background(), gateTestKey)
			require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
			require.NotContains(t, err.Error(), "secret")
			require.Equal(t, entity.HookGateWaiting, got.Gate)
		})
	}
}
