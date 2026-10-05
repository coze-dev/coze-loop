// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func newHookItemSourceFixture(t *testing.T, marker *int32) (*hookTxFixture, entity.HookRunKey) {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
	if dsn == "" {
		t.Skip("requires isolated HOOK_MYSQL_TX_DSN")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "/Users/bytedance/fornax_workspace/_local/scratch/hook-mysql-20260922/m.sock", cfg.Addr)
	f := newHookTxFixture(t)
	key := entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: hookTxSequence.Add(1)}
	require.NoError(t, f.sql.Create(&model.ExptRunLog{
		ID: key.RunID, ExptRunID: key.RunID, SpaceID: key.WorkspaceID, ExptID: key.ExperimentID,
		Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit)),
		CreatedBy: "item-source-fixture", LifecycleHookVersion: marker,
	}).Error)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=? AND space_id=?", f.expt, f.space).
		UpdateColumn("latest_run_id", key.RunID).Error)
	t.Cleanup(func() {
		// Tests can corrupt scope fields; clean only the two IDs allocated here.
		require.NoError(t, f.sql.Unscoped().Delete(&model.ExptRunLog{}, "id=?", key.RunID).Error)
		require.NoError(t, f.sql.Unscoped().Delete(&model.Experiment{}, "id=?", key.ExperimentID).Error)
	})
	return f, key
}

func readHookItemSourceForTest(ctx context.Context, f *hookTxFixture, key entity.HookRunKey) (*entity.HookRunInitialization, error) {
	return NewHookItemSourceRepo(f.p).ReadItemSource(ctx, key)
}

func TestHookItemSourceMySQLLegacyParentLock(t *testing.T) {
	f, key := newHookItemSourceFixture(t, nil)
	holder := f.sql.Begin()
	require.NoError(t, holder.Error)
	defer func() { require.NoError(t, holder.Rollback().Error) }()
	var parent model.Experiment
	require.NoError(t, holder.Select("id").Where("id=? AND space_id=?", f.expt, f.space).
		Clauses(clause.Locking{Strength: "UPDATE"}).Take(&parent).Error)
	initializationMySQLRequireParentLocked(t, f, context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	source, err := readHookItemSourceForTest(ctx, f, key)
	t.Logf("source read while parent is locked: elapsed=%v err=%v", time.Since(start), err)
	require.NoError(t, err, "legacy item classification must not wait for the experiment parent lock")
	require.NoError(t, ctx.Err())
	require.NotNil(t, source)
	require.False(t, source.Managed)
	require.Equal(t, key.RunID, source.RunLog.ExptRunID)
	initializationMySQLRequireParentLocked(t, f, context.Background())
}

func TestHookItemSourceMySQLLegacyMalformedConfig(t *testing.T) {
	f, key := newHookItemSourceFixture(t, gptr.Of(int32(0)))
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=? AND space_id=?", f.expt, f.space).
		UpdateColumn("lifecycle_hook_conf", []byte("{malformed-current-config")).Error)
	source, err := readHookItemSourceForTest(context.Background(), f, key)
	require.NoError(t, err, "legacy classification must depend on the Run marker, not current Hook configuration")
	require.NotNil(t, source)
	require.False(t, source.Managed)
}

func TestHookItemSourceMySQLClassificationAndReadIsolation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		marker  *int32
		managed bool
	}{
		{"legacy null", nil, false}, {"legacy zero", gptr.Of(int32(0)), false}, {"managed", gptr.Of(int32(1)), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, key := newHookItemSourceFixture(t, tc.marker)
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", key.ExperimentID).UpdateColumn("lifecycle_hook_conf", []byte("{unrelated-corrupt-config")).Error)
			require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", key.RunID).UpdateColumn("item_ids", []byte("{unrelated-corrupt-items")).Error)
			var queries []string
			name := fmt.Sprintf("item_source_query_%d", key.RunID)
			require.NoError(t, f.sql.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) { queries = append(queries, strings.ToLower(tx.Statement.SQL.String())) }))
			t.Cleanup(func() { require.NoError(t, f.sql.Callback().Query().Remove(name)) })
			got, err := readHookItemSourceForTest(context.Background(), f, key)
			require.NoError(t, err)
			require.Equal(t, tc.managed, got.Managed)
			require.Equal(t, key.RunID, got.LatestRunID)
			require.Equal(t, &entity.ExptRunLog{ID: key.RunID, SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, Status: int64(entity.ExptStatus_Processing)}, got.RunLog)
			require.Empty(t, got.ConfigRevision)
			require.False(t, got.HooksEnabled)
			require.Len(t, queries, 1, "classification must be a single consistent primary SELECT")
			for _, forbidden := range []string{"for update", "lock in share", "lifecycle_hook_conf", "expt_lifecycle", "snapshot", "item_ids", "created_by", "select *"} {
				require.NotContains(t, queries[0], forbidden)
			}
		})
	}
}

func TestHookItemSourceMySQLRejectsBadTuplesAndMissingData(t *testing.T) {
	for _, name := range []string{"requested workspace", "requested experiment", "requested run", "stored run id", "stored run space", "stored run experiment", "experiment space", "deleted run", "deleted experiment", "missing run", "missing experiment", "unknown marker", "large marker", "missing status", "unknown status", "missing latest"} {
		t.Run(name, func(t *testing.T) {
			f, key := newHookItemSourceFixture(t, gptr.Of(int32(1)))
			request := key
			run := f.sql.Unscoped().Model(&model.ExptRunLog{}).Where("id=?", key.RunID)
			expt := f.sql.Unscoped().Model(&model.Experiment{}).Where("id=?", key.ExperimentID)
			var err error
			switch name {
			case "requested workspace":
				request.WorkspaceID++
			case "requested experiment":
				request.ExperimentID++
			case "requested run":
				request.RunID++
			case "stored run id":
				err = run.UpdateColumn("expt_run_id", key.RunID+100).Error
			case "stored run space":
				err = run.UpdateColumn("space_id", key.WorkspaceID+100).Error
			case "stored run experiment":
				err = run.UpdateColumn("expt_id", key.ExperimentID+100).Error
			case "experiment space":
				err = expt.UpdateColumn("space_id", key.WorkspaceID+100).Error
			case "deleted run":
				err = run.UpdateColumn("deleted_at", time.Now()).Error
			case "deleted experiment":
				err = expt.UpdateColumn("deleted_at", time.Now()).Error
			case "missing run":
				err = run.Delete(nil).Error
			case "missing experiment":
				err = expt.Delete(nil).Error
			case "unknown marker":
				err = run.UpdateColumn("lifecycle_hook_version", 2).Error
			case "large marker":
				err = run.UpdateColumn("lifecycle_hook_version", 255).Error
			case "missing status":
				err = run.UpdateColumn("status", nil).Error
			case "unknown status":
				err = run.UpdateColumn("status", 999).Error
			case "missing latest":
				err = expt.UpdateColumn("latest_run_id", 0).Error
			}
			require.NoError(t, err)
			got, err := readHookItemSourceForTest(context.Background(), f, request)
			require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
			require.Nil(t, got, "unknown data must not become legacy")
		})
	}
}

func TestHookItemSourceMySQLOldAndClosedRunsRemainClassifiable(t *testing.T) {
	f, key := newHookItemSourceFixture(t, gptr.Of(int32(1)))
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", key.ExperimentID).UpdateColumn("latest_run_id", key.RunID+1).Error)
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", key.RunID).UpdateColumn("status", int64(entity.ExptStatus_Terminated)).Error)
	got, err := readHookItemSourceForTest(context.Background(), f, key)
	require.NoError(t, err)
	require.True(t, got.Managed)
	require.Equal(t, key.RunID+1, got.LatestRunID)
	require.Equal(t, int64(entity.ExptStatus_Terminated), got.RunLog.Status)
}

func TestHookItemSourcePrimaryReadAndSafeErrors(t *testing.T) {
	for _, fail := range []bool{false, true} {
		p, m := gateMock(t)
		var logged bytes.Buffer
		p = &gateLoggedProvider{Provider: p, output: logger.New(log.New(&logged, "", 0), logger.Config{LogLevel: logger.Info})}
		query := m.ExpectQuery("SELECT .* FROM expt_run_log AS l LEFT JOIN experiment AS e .* WHERE l.id=\\?").WithArgs(int64(30))
		if fail {
			query.WillReturnError(errors.New("sensitive-database-error"))
		} else {
			query.WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "status", "lifecycle_hook_version", "experiment_id", "experiment_space_id", "latest_run_id"}).AddRow(30, 10, 20, 30, 3, nil, 20, 10, 30))
		}
		got, err := NewHookItemSourceRepo(p).ReadItemSource(context.Background(), gateTestKey)
		if fail {
			require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
			require.Nil(t, got)
		} else {
			require.NoError(t, err)
			require.False(t, got.Managed)
		}
		require.Empty(t, logged.String())
	}
}

func TestHookItemSourceInvalidDependenciesAndInputs(t *testing.T) {
	p, _ := gateMock(t)
	typedNil := reflect.Zero(reflect.TypeOf(p.(*gateMasterProvider).Provider)).Interface().(db.Provider)
	for _, provider := range []db.Provider{nil, typedNil} {
		got, err := NewHookItemSourceRepo(provider).ReadItemSource(context.Background(), gateTestKey)
		require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
		require.Nil(t, got)
	}
	for _, key := range []entity.HookRunKey{{}, {WorkspaceID: 10, ExperimentID: 20}, {WorkspaceID: -1, ExperimentID: 20, RunID: 30}} {
		got, err := NewHookItemSourceRepo(p).ReadItemSource(context.Background(), key)
		require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
		require.Nil(t, got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, ctx} {
		got, err := NewHookItemSourceRepo(p).ReadItemSource(ctx, gateTestKey)
		require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
		require.Nil(t, got)
	}
	var reader *hookItemSourceRepo
	got, err := reader.ReadItemSource(context.Background(), gateTestKey)
	require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
	require.Nil(t, got)
}
