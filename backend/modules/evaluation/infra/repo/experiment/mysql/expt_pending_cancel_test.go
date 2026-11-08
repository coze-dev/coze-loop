// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// SQLite exercises the real transaction writes; MySQL row-lock tests remain separate.
func pendingCancelDB(t *testing.T, run int64) (repo.IExptRunTerminationRepo, *gorm.DB) {
	t.Helper()
	provider, err := db.NewDB(sqlite.Open(":memory:"))
	require.NoError(t, err)
	conn := provider.NewSession(context.Background())
	sqlDB, err := conn.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, ddl := range []string{
		"CREATE TABLE experiment (id INTEGER PRIMARY KEY, space_id INTEGER, latest_run_id INTEGER, status INTEGER, expt_type INTEGER, end_at DATETIME, status_message BLOB, deleted_at DATETIME)",
		"CREATE TABLE expt_run_log (id INTEGER PRIMARY KEY, space_id INTEGER, expt_id INTEGER, expt_run_id INTEGER, status INTEGER, status_message BLOB, pending_cnt INTEGER, processing_cnt INTEGER, success_cnt INTEGER, fail_cnt INTEGER, terminated_cnt INTEGER, deleted_at DATETIME)",
		"CREATE TABLE expt_item_result (id INTEGER PRIMARY KEY, space_id INTEGER, expt_id INTEGER, expt_run_id INTEGER, item_id INTEGER, status INTEGER, deleted_at DATETIME)",
		"CREATE TABLE expt_turn_result (id INTEGER PRIMARY KEY, space_id INTEGER, expt_id INTEGER, status INTEGER, deleted_at DATETIME)",
		"CREATE TABLE expt_stats (id INTEGER PRIMARY KEY, space_id INTEGER, expt_id INTEGER, pending_cnt INTEGER, processing_cnt INTEGER, success_cnt INTEGER, fail_cnt INTEGER, terminated_cnt INTEGER, deleted_at DATETIME)",
	} {
		require.NoError(t, conn.Exec(ddl).Error)
	}
	require.NoError(t, conn.Exec("INSERT INTO experiment(id,space_id,latest_run_id,status,expt_type) VALUES(2,1,?,?,?)", run, entity.ExptStatus_Pending, entity.ExptType_Offline).Error)
	require.NoError(t, conn.Exec("INSERT INTO expt_stats(id,space_id,expt_id) VALUES(4,1,2)").Error)
	if run > 0 {
		require.NoError(t, conn.Exec("INSERT INTO expt_run_log(id,space_id,expt_id,expt_run_id,status) VALUES(?,1,2,?,?)", run, run, entity.ExptStatus_Pending).Error)
	}
	return NewExptRunFinishDAO(provider).(repo.IExptRunTerminationRepo), conn
}

func TestPendingCancelTransactionAndReplay(t *testing.T) {
	for _, run := range []int64{0, 3} {
		t.Run(fmt.Sprint(run), func(t *testing.T) {
			port, conn := pendingCancelDB(t, run)
			if run > 0 {
				require.NoError(t, conn.Exec("INSERT INTO expt_item_result(id,space_id,expt_id,expt_run_id,item_id,status) VALUES(7,1,2,3,7,?)", entity.ItemRunState_Success).Error)
			}
			begin := &entity.ExptRunTerminationRequest{SpaceID: 1, ExptID: 2, RunID: run, Begin: true}
			for i := 0; i < 2; i++ {
				result, err := port.TerminateCurrentRun(context.Background(), begin)
				require.NoError(t, err)
				require.True(t, result.Applied)
			}
			finish := &entity.ExptRunTerminationRequest{SpaceID: 1, ExptID: 2, RunID: run, Finalize: true, Status: entity.ExptStatus_Terminated}
			first, err := port.TerminateCurrentRun(context.Background(), finish)
			require.NoError(t, err)
			require.Equal(t, entity.ExptStatus_Terminated, first.Experiment.Status)
			again, err := port.TerminateCurrentRun(context.Background(), finish)
			require.NoError(t, err)
			require.True(t, first.Experiment.EndAt.Equal(*again.Experiment.EndAt))
			if run > 0 {
				var status int32
				require.NoError(t, conn.Table("expt_item_result").Select("status").Scan(&status).Error)
				require.Equal(t, int32(entity.ItemRunState_Success), status)
			}
			var logs []struct{ Status int64 }
			require.NoError(t, conn.Table("expt_run_log").Select("status").Scan(&logs).Error)
			if run == 0 {
				require.Empty(t, logs, "cancel must not invent an execution attempt")
			} else {
				require.Len(t, logs, 1)
				require.Equal(t, int64(entity.ExptStatus_Terminated), logs[0].Status)
			}
		})
	}
}

func TestPendingCancelPreservesSetupAbortAndRejectsRows(t *testing.T) {
	port, conn := pendingCancelDB(t, 3)
	_, err := port.TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: 1, ExptID: 2, RunID: 3})
	require.Error(t, err, "Pending is not a row-level cancellation")
	result, err := port.TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: 1, ExptID: 2, RunID: 3, SetupAbort: true})
	require.NoError(t, err)
	require.True(t, result.Applied)
	var exptStatus, runStatus int32
	require.NoError(t, conn.Table("experiment").Select("status").Scan(&exptStatus).Error)
	require.NoError(t, conn.Table("expt_run_log").Select("status").Scan(&runStatus).Error)
	require.Equal(t, int32(entity.ExptStatus_Pending), exptStatus)
	require.Equal(t, int32(entity.ExptStatus_Failed), runStatus)
}

func TestPendingCancelBeginRollbackAndStartWins(t *testing.T) {
	for _, startWins := range []bool{false, true} {
		t.Run(fmt.Sprint(startWins), func(t *testing.T) {
			port, conn := pendingCancelDB(t, 3)
			if startWins {
				require.NoError(t, conn.Table("experiment").Where("id=2").Update("status", entity.ExptStatus_Processing).Error)
				require.NoError(t, conn.Table("expt_run_log").Where("id=3").Update("status", entity.ExptStatus_Processing).Error)
			} else {
				require.NoError(t, conn.Callback().Update().Before("gorm:update").Register("pending_cancel_fault", func(tx *gorm.DB) {
					if tx.Statement.Table == "experiment" {
						_ = tx.AddError(errors.New("write failed"))
					}
				}))
			}
			_, err := port.TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: 1, ExptID: 2, RunID: 3, Begin: true})
			var runStatus int32
			require.NoError(t, conn.Table("expt_run_log").Select("status").Scan(&runStatus).Error)
			if startWins {
				require.NoError(t, err)
				require.Equal(t, int32(entity.ExptStatus_Terminating), runStatus)
			} else {
				require.Error(t, err)
				require.Equal(t, int32(entity.ExptStatus_Pending), runStatus, "run update must roll back with experiment")
			}
		})
	}
}

func TestPendingCancelDoesNotAffectReplacementOrCompletedExperiment(t *testing.T) {
	for _, current := range []int64{0, 3} {
		port, conn := pendingCancelDB(t, current)
		require.NoError(t, conn.Table("experiment").Where("id=2").Update("latest_run_id", 4).Error)
		result, err := port.TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: 1, ExptID: 2, RunID: current, Begin: true})
		require.NoError(t, err)
		require.True(t, result.NonCurrent)
		require.NoError(t, conn.Table("experiment").Where("id=2").Updates(map[string]any{"latest_run_id": current, "status": entity.ExptStatus_Success}).Error)
		_, err = port.TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: 1, ExptID: 2, RunID: current, Begin: true})
		require.Error(t, err)
		var status int32
		require.NoError(t, conn.Table("experiment").Select("status").Scan(&status).Error)
		require.Equal(t, int32(entity.ExptStatus_Success), status)
	}
}

func TestPendingCancelRunLogCheckReadsPrimary(t *testing.T) {
	ctx := context.Background()
	provider, err := db.NewDB(sqlite.Open(filepath.Join(t.TempDir(), "primary.db")))
	require.NoError(t, err)
	replicaPath := filepath.Join(t.TempDir(), "replica.db")
	replica, err := gorm.Open(sqlite.Open(replicaPath))
	require.NoError(t, err)
	for conn, status := range map[*gorm.DB]entity.ExptStatus{provider.NewSession(ctx): entity.ExptStatus_Terminated, replica: entity.ExptStatus_Pending} {
		require.NoError(t, conn.Exec("CREATE TABLE expt_run_log(id INTEGER PRIMARY KEY, space_id INTEGER, expt_id INTEGER, expt_run_id INTEGER, status INTEGER, deleted_at DATETIME)").Error)
		require.NoError(t, conn.Exec("INSERT INTO expt_run_log(id,space_id,expt_id,expt_run_id,status) VALUES(3,1,2,3,?)", status).Error)
		raw, err := conn.DB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = raw.Close() })
	}
	require.NoError(t, provider.NewSession(ctx).Use(dbresolver.Register(dbresolver.Config{Replicas: []gorm.Dialector{sqlite.Open(replicaPath)}})))
	dao := NewExptRunLogDAO(provider)
	stale, err := dao.Get(ctx, 2, 3)
	require.NoError(t, err)
	require.Equal(t, int64(entity.ExptStatus_Pending), *stale.Status)
	current, err := dao.Get(contexts.WithCtxWriteDB(ctx), 2, 3)
	require.NoError(t, err)
	require.Equal(t, int64(entity.ExptStatus_Terminated), *current.Status)
}
