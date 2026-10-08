// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type hookGateFixture struct {
	sql   *gorm.DB
	repo  repo.IHookGateRepo
	key   entity.HookRunKey
	scope string
}

func newHookGateFixture(t *testing.T) *hookGateFixture {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_SCAN_DSN")
	if dsn == "" {
		t.Skip("requires HOOK_MYSQL_SCAN_DSN with isolated scan database")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, hookScanSocket, cfg.Addr)
	require.Equal(t, "hook_7378265404_scan", cfg.DBName)
	require.True(t, cfg.ParseTime)
	require.Equal(t, time.UTC, cfg.Loc)
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	s := p.NewSession(context.Background())
	c, err := s.DB()
	require.NoError(t, err)
	c.SetMaxOpenConns(4)
	t.Cleanup(func() { require.NoError(t, c.Close()) })
	// Only add missing, already-defined old tables; never alter the shared schema.
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	for _, table := range []string{"experiment", "expt_run_log"} {
		if !s.Migrator().HasTable(table) {
			ddl, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../../../release/deployment/docker-compose/bootstrap/mysql-init/init-sql", table+".sql"))
			require.NoError(t, err)
			require.NoError(t, s.Exec(string(ddl)).Error)
		}
	}
	base := hookScanSequence.Add(100000)
	f := &hookGateFixture{sql: s, key: entity.HookRunKey{WorkspaceID: base, ExperimentID: base + 1, RunID: base + 2}, scope: fmt.Sprint("gate_", base)}
	f.repo = NewHookGateRepo(p, func(context.Context) (string, error) { return f.scope, nil })
	t.Cleanup(func() {
		for _, table := range []string{"expt_lifecycle_hook_run", "expt_lifecycle_run", "expt_run_log", "experiment"} {
			require.NoError(t, s.Unscoped().Table(table).Where("space_id=?", f.key.WorkspaceID).Delete(nil).Error)
			var count int64
			require.NoError(t, s.Table(table).Where("space_id=?", f.key.WorkspaceID).Count(&count).Error)
			require.Zero(t, count, "fixture rows must be removed")
		}
	})
	require.NoError(t, s.Create(&model.Experiment{ID: f.key.ExperimentID, SpaceID: f.key.WorkspaceID, LatestRunID: f.key.RunID, Status: 3, Name: f.scope, LifecycleHookConf: gptr.Of([]byte("private configuration"))}).Error)
	require.NoError(t, s.Create(&model.ExptRunLog{ID: f.key.RunID, SpaceID: f.key.WorkspaceID, ExptID: f.key.ExperimentID, ExptRunID: f.key.RunID, Status: gptr.Of(int64(3)), LifecycleHookVersion: gptr.Of(int32(1)), CreatedBy: "private user"}).Error)
	return f
}

func (f *hookGateFixture) seed(t *testing.T, before, after bool) {
	t.Helper()
	gate := int32(0)
	if !before {
		gate = 1
	}
	require.NoError(t, f.sql.Create(&model.ExptLifecycleRun{SpaceID: f.key.WorkspaceID, ExptID: f.key.ExperimentID, ExptRunID: f.key.RunID,
		BeforeEnabled: before, AfterEnabled: after, ExecutionScope: f.scope, Gate: gate, SnapshotCipher: []byte("private snapshot"), SnapshotKeyID: "private key", SnapshotHash: strings.Repeat("a", 64)}).Error)
	for i, enabled := range []bool{before, after} {
		if !enabled {
			continue
		}
		phase := []string{"before", "after"}[i]
		id := f.key.RunID + int64(i) + 1
		require.NoError(t, f.sql.Create(&model.ExptLifecycleHookRun{ID: id, SpaceID: f.key.WorkspaceID, ExptID: f.key.ExperimentID, ExptRunID: f.key.RunID,
			Phase: phase, OperationID: fmt.Sprint("hook_", id), IdempotencyKey: fmt.Sprint("key_", id), Status: "pending", ExecutionScope: f.scope, UpdatedAt: time.Now(), ResultRedacted: gptr.Of([]byte("private result"))}).Error)
	}
}

func (f *hookGateFixture) read(t *testing.T, want entity.HookGateState, bad bool) {
	t.Helper()
	got, err := f.repo.CanDispatch(context.Background(), f.key)
	if bad {
		require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
	} else {
		require.NoError(t, err)
	}
	require.Equal(t, want, got.Gate)
}

func TestHookGateMySQLStatesAndIntegrity(t *testing.T) {
	for _, name := range []string{"waiting", "ready", "continue", "after only", "draining waiting", "draining ready", "cancelled", "deleted run", "deleted experiment", "old run", "wrong workspace", "unknown marker", "missing lifecycle", "missing before", "missing after", "extra phase", "scope mismatch", "both disabled"} {
		t.Run(name, func(t *testing.T) {
			f := newHookGateFixture(t)
			f.seed(t, name != "after only", true)
			life := f.sql.Table("expt_lifecycle_run").Where("space_id=?", f.key.WorkspaceID)
			op := f.sql.Table("expt_lifecycle_hook_run").Where("space_id=? AND phase='before'", f.key.WorkspaceID)
			log := f.sql.Table("expt_run_log").Where("id=?", f.key.RunID)
			expt := f.sql.Table("experiment").Where("id=?", f.key.ExperimentID)
			want, bad := entity.HookGateWaiting, false
			switch name {
			case "ready", "continue", "draining ready":
				status := "succeeded"
				if name == "continue" {
					status = "failed"
				}
				require.NoError(t, op.UpdateColumns(map[string]any{"status": status, "attempt": 1}).Error)
				require.NoError(t, life.UpdateColumns(map[string]any{"gate": 1, "plan_state": 1, "plan_hash": strings.Repeat("b", 64)}).Error)
				want = entity.HookGateReady
				if name == "draining ready" {
					require.NoError(t, log.Update("status", 21).Error)
					require.NoError(t, expt.Update("status", 21).Error)
				}
			case "draining waiting":
				require.NoError(t, log.Update("status", 21).Error)
				require.NoError(t, expt.Update("status", 21).Error)
			case "after only":
				want = entity.HookGateReady
			case "cancelled":
				require.NoError(t, log.Update("status", 15).Error)
				want = entity.HookGateClosed
			case "deleted run":
				require.NoError(t, log.Update("deleted_at", time.Now()).Error)
				want = entity.HookGateClosed
			case "deleted experiment":
				require.NoError(t, expt.Update("deleted_at", time.Now()).Error)
				want = entity.HookGateClosed
			case "old run":
				require.NoError(t, expt.Update("latest_run_id", f.key.RunID+10).Error)
				want = entity.HookGateClosed
			case "wrong workspace":
				f.key.WorkspaceID++
				defer func() { f.key.WorkspaceID-- }()
				want = entity.HookGateClosed
			case "unknown marker":
				require.NoError(t, log.Update("lifecycle_hook_version", 2).Error)
				bad = true
			case "missing lifecycle":
				require.NoError(t, life.Delete(nil).Error)
				bad = true
			case "missing before":
				require.NoError(t, op.Delete(nil).Error)
				bad = true
			case "missing after":
				require.NoError(t, f.sql.Table("expt_lifecycle_hook_run").Where("space_id=? AND phase='after'", f.key.WorkspaceID).Delete(nil).Error)
				bad = true
			case "extra phase":
				require.NoError(t, op.Update("phase", "extra").Error)
				bad = true
			case "scope mismatch":
				require.NoError(t, life.Update("execution_scope", "wrong").Error)
				bad = true
			case "both disabled":
				require.NoError(t, life.UpdateColumns(map[string]any{"before_enabled": false, "after_enabled": false}).Error)
				bad = true
			}
			f.read(t, want, bad)
		})
	}
}

func TestHookGateMySQLSnapshotAndNoLocks(t *testing.T) {
	f := newHookGateFixture(t)
	f.seed(t, true, true)
	lock := f.sql.Begin()
	require.NoError(t, lock.Error)
	defer lock.Rollback()
	for _, table := range []string{"experiment", "expt_run_log", "expt_lifecycle_run", "expt_lifecycle_hook_run"} {
		require.NoError(t, lock.Exec("SELECT space_id FROM "+table+" WHERE space_id=? FOR UPDATE", f.key.WorkspaceID).Error)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := f.repo.CanDispatch(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, entity.HookGateWaiting, got.Gate)
	require.NoError(t, lock.Rollback().Error)
	changed := false
	const callback = "gate_concurrent_commit"
	require.NoError(t, f.sql.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if _, inSnapshot := tx.Statement.ConnPool.(gorm.TxCommitter); !inSnapshot {
			return
		}
		if changed || !strings.Contains(tx.Statement.SQL.String(), "expt_run_log AS l") {
			return
		}
		changed = true
		require.NoError(t, f.sql.Transaction(func(writer *gorm.DB) error {
			if err := writer.Table("expt_lifecycle_run").Where("space_id=?", f.key.WorkspaceID).UpdateColumns(map[string]any{"gate": 1, "plan_state": 1, "plan_hash": strings.Repeat("c", 64)}).Error; err != nil {
				return err
			}
			return writer.Table("expt_lifecycle_hook_run").Where("space_id=? AND phase='before'", f.key.WorkspaceID).UpdateColumns(map[string]any{"status": "succeeded", "attempt": 1}).Error
		}))
	}))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Query().Remove(callback)) })
	f.read(t, entity.HookGateWaiting, false)
	require.True(t, changed)
	f.read(t, entity.HookGateReady, false)
}

func TestHookGateMySQLQueryPrivacyAndLegacy(t *testing.T) {
	f := newHookGateFixture(t)
	f.seed(t, false, true)
	var queries []string
	const callback = "gate_query_audit"
	require.NoError(t, f.sql.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) { queries = append(queries, strings.ToLower(tx.Statement.SQL.String())) }))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Query().Remove(callback)) })
	f.read(t, entity.HookGateReady, false)
	require.Len(t, queries, 2)
	for _, query := range queries {
		for _, forbidden := range []string{"snapshot", "parameters", "created_by", "result", "error_message", "error_code", "lifecycle_hook_conf", "select *", "for update", "lock in share"} {
			require.NotContains(t, query, forbidden)
		}
	}
	for _, marker := range []any{nil, 0} {
		require.NoError(t, f.sql.Table("expt_run_log").Where("id=?", f.key.RunID).Update("lifecycle_hook_version", marker).Error)
		queries = nil
		f.repo.(*hookGateRepo).executionScope = func(context.Context) (string, error) { t.Error("legacy scope called"); return "", nil }
		f.read(t, entity.HookGateReady, false)
		require.Len(t, queries, 1)
		require.NotContains(t, queries[0], "expt_lifecycle")
	}
}
