// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type initializationItems struct{ want []int64 }

func (a initializationItems) Match(value driver.Value) bool {
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return false
	}
	var chunks []entity.ExptRunLogItems
	if json.Unmarshal(raw, &chunks) != nil {
		return false
	}
	log := &entity.ExptRunLog{ItemIds: chunks}
	got := log.GetItemIDs()
	if len(got) != len(a.want) {
		return false
	}
	for i := range got {
		if got[i] != a.want[i] {
			return false
		}
	}
	return len(chunks) == 2 && chunks[0].CreateAt != nil && *chunks[0].CreateAt == 10
}

func initializationDB(t *testing.T) (db.Provider, sqlmock.Sqlmock) {
	t.Helper()
	conn, mock, err := sqlmock.New()
	require.NoError(t, err)
	provider, err := db.NewDB(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mock.ExpectationsWereMet()); _ = conn.Close() })
	return provider, mock
}

func TestHookInitializationAppendFences(t *testing.T) {
	for _, tc := range []struct {
		name         string
		version      int64
		status, gate int
		scope        string
		want         error
	}{
		{"append waiting", 4, 2, 0, "local", nil},
		{"cancel won", 4, 13, 2, "local", entity.ErrHookAdmissionDenied},
		{"draining", 4, 21, 0, "local", entity.ErrHookAdmissionDenied},
		{"terminating", 4, 15, 0, "local", entity.ErrHookAdmissionDenied},
		{"stale version", 3, 2, 0, "local", entity.ErrHookStoreConflict},
		{"wrong scope", 4, 2, 0, "other", entity.ErrHookStoreConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, m := initializationDB(t)
			writer, ok := NewHookRunRepo(p).(interface {
				AppendHookRunItems(context.Context, entity.HookAppendRunItemsInput) error
			})
			require.True(t, ok, "managed append must not use the legacy full-row Save")
			m.ExpectBegin()
			m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status"}).AddRow(20, 10, 30, 2))
			m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "expt_run_id", "version", "before_enabled", "after_enabled", "gate", "snapshot_cipher", "snapshot_key_id", "snapshot_hash", "execution_scope"}).AddRow(10, 20, 30, 4, true, false, tc.gate, []byte("frozen"), "key", strings.Repeat("a", 64), "local"))
			m.ExpectQuery("SELECT .*FROM .expt_run_log.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "lifecycle_hook_version", "status", "mode", "created_by", "item_ids"}).AddRow(30, 10, 20, 30, 1, tc.status, 5, "original-user", []byte(`[{"ItemIDs":[1],"CreateAt":10}]`)))
			m.ExpectQuery("SELECT .*FROM .expt_lifecycle_hook_run.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "operation_id", "idempotency_key", "phase", "status", "execution_scope"}).AddRow(40, "hook_40", "key_40", "before", "pending", "local"))
			if tc.want != nil {
				m.ExpectRollback()
			} else {
				m.ExpectQuery("SELECT CURRENT_TIMESTAMP").WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(time.Unix(100, 0)))
				m.ExpectExec("UPDATE .expt_run_log.*SET .item_ids.=.*updated_at.*lifecycle_hook_version=1 AND status=").WithArgs(initializationItems{[]int64{1, 2}}, time.Unix(100, 0), int64(10), int64(20), int64(30), int64(30), int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("UPDATE .expt_lifecycle_run.*SET .updated_at.=.*version.=").WithArgs(time.Unix(100, 0), int64(5), int64(10), int64(20), int64(30), int64(4)).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			}
			err := writer.AppendHookRunItems(context.Background(), entity.HookAppendRunItemsInput{HookStoreGuard: entity.HookStoreGuard{Key: entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, ExpectedVersion: tc.version}, ExecutionScope: tc.scope, ItemIDs: []int64{2}})
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHookInitializationRejectsMalformedProjection(t *testing.T) {
	for _, raw := range []string{`[]`, `true`, `{"version":2,"purpose":"configuration","projection":{"before":{"enabled":false}}}`, `{"version":1,"purpose":"configuration","projection":{}}`, `{"version":1,"purpose":"configuration","projection":{"before":{}}}`} {
		p, m := initializationDB(t)
		m.ExpectBegin()
		m.ExpectQuery("SELECT .*lifecycle_hook_conf.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "latest_run_id"}).AddRow([]byte(raw), 0))
		m.ExpectRollback()
		got, err := NewHookRunInitializationRepo(p).ReadRunInitialization(context.Background(), entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30})
		require.Error(t, err)
		require.Nil(t, got)
	}
}

func TestHookInitializationUnknownMarkerFailsClosed(t *testing.T) {
	p, m := initializationDB(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*lifecycle_hook_conf.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "latest_run_id"}).AddRow(nil, 30))
	m.ExpectQuery("SELECT .*FROM .expt_run_log.").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "lifecycle_hook_version"}).AddRow(30, 10, 20, 30, 2))
	m.ExpectRollback()
	got, err := NewHookRunInitializationRepo(p).ReadRunInitialization(context.Background(), entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30})
	require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
	require.Nil(t, got)
}

func TestHookInitializationLatestConflictRollsBack(t *testing.T) {
	p, m := initializationDB(t)
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status"}).AddRow(20, 10, 29, 2))
	m.ExpectQuery("SELECT .*lifecycle_hook_conf.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "latest_run_id"}).AddRow(nil, 29))
	m.ExpectQuery("SELECT .*FROM .expt_run_log.").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectRollback()
	changed, err := NewHookRunInitializationRepo(p).CreateRunWithoutHooks(context.Background(), &entity.ExptRunLog{ID: 30, ExptRunID: 30, ExptID: 20, SpaceID: 10, CreatedBy: "user", Mode: 1, Status: 2}, 0, "")
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.False(t, changed)
}

// A concurrent enable must prevent a stale no-Hook decision from publishing Latest.
func TestHookInitializationLegacyRevisionConflict(t *testing.T) {
	p, mock := initializationDB(t)
	writer, ok := NewHookRunRepo(p).(interface {
		CreateRunWithoutHooks(context.Context, *entity.ExptRunLog, int64, string) (bool, error)
	})
	require.True(t, ok, "atomic legacy initialization must share the experiment parent lock with Hook configuration writes")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status"}).AddRow(20, 10, 0, 1))
	mock.ExpectQuery("SELECT .*lifecycle_hook_conf.*FROM .experiment.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "latest_run_id"}).AddRow([]byte(`{"changed":true}`), 0))
	mock.ExpectRollback()
	changed, err := writer.CreateRunWithoutHooks(context.Background(), &entity.ExptRunLog{ID: 30, ExptRunID: 30, ExptID: 20, SpaceID: 10, CreatedBy: "user", Mode: 1, Status: 2}, 0, "")
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.False(t, changed)
}

func TestHookInitializationNoHookAtomicCreate(t *testing.T) {
	for _, raw := range []string{"", "null", "{}", `{"version":1,"purpose":"configuration","projection":{"before":{"enabled":false,"environment":"Prod"},"after":{"enabled":false,"environment":"Prod"}}}`} {
		t.Run(raw, func(t *testing.T) {
			p, mock := initializationDB(t)
			writer, ok := NewHookRunRepo(p).(interface {
				CreateRunWithoutHooks(context.Context, *entity.ExptRunLog, int64, string) (bool, error)
			})
			require.True(t, ok)
			revision := ""
			if raw != "" {
				revision = fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
			}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status"}).AddRow(20, 10, 0, 2))
			mock.ExpectQuery("SELECT .*lifecycle_hook_conf.*FROM .experiment.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"lifecycle_hook_conf", "latest_run_id"}).AddRow([]byte(raw), 0))
			mock.ExpectQuery("SELECT .*FROM .expt_run_log.").WillReturnRows(sqlmock.NewRows([]string{"id"}))
			mock.ExpectQuery("SELECT CURRENT_TIMESTAMP").WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(time.Unix(100, 0)))
			mock.ExpectExec("INSERT INTO .expt_run_log.").WillReturnResult(sqlmock.NewResult(30, 1))
			mock.ExpectExec("UPDATE .experiment.*latest_run_id").WithArgs(int64(30), time.Unix(100, 0), int64(20), int64(10), int64(0)).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			changed, err := writer.CreateRunWithoutHooks(context.Background(), &entity.ExptRunLog{ID: 30, ExptRunID: 30, ExptID: 20, SpaceID: 10, CreatedBy: "legacy-user", Mode: 1, Status: 2}, 0, revision)
			require.NoError(t, err)
			require.True(t, changed)
		})
	}
}
