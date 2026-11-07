// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

var _ IExptRunLogLatestRunDAO = &ExptRunLogDAOImpl{}

// TestExptRunLogDAOUpdateIfLatestRunMySQL proves the CAS write accepts the
// current run (including a zero-row idempotent rewrite) and rejects a run that
// has been replaced, without touching the replacement run's state.
func TestExptRunLogDAOUpdateIfLatestRunMySQL(t *testing.T) {
	dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("EXPT_RETRY_MYSQL_DSN missing; REAL MYSQL NOT EXECUTED")
	}
	ctx := context.Background()
	provider, err := db.NewDB(mysqldriver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn := provider.NewSession(ctx, db.WithMaster())
	var identity struct{ Version, Schema string }
	require.NoError(t, conn.Raw("SELECT VERSION() AS version,DATABASE() AS `schema`").Scan(&identity).Error)
	require.Equal(t, "expt_running_retry_test", identity.Schema)
	t.Logf("REAL_MYSQL_EXECUTED version=%s schema=%s", identity.Version, identity.Schema)

	base := int64(890000000000000000) + time.Now().UnixNano()%1000000000000
	space, expt, r1, r2 := base+1, base+2, base+3, base+4
	t.Cleanup(func() {
		require.NoError(t, conn.Exec("DELETE FROM expt_run_log WHERE space_id = ? AND expt_id = ?", space, expt).Error)
		require.NoError(t, conn.Exec("DELETE FROM experiment WHERE space_id = ? AND id = ?", space, expt).Error)
		sqlDB, err := conn.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, conn.Exec("INSERT INTO experiment(id,space_id,latest_run_id,status,expt_type) VALUES(?,?,?,?,?)",
		expt, space, r1, entity.ExptStatus_Processing, entity.ExptType_Offline).Error)
	require.NoError(t, conn.Create(&model.ExptRunLog{
		ID: r1, SpaceID: space, ExptID: expt, ExptRunID: r1,
		Status: gptr.Of(int64(entity.ExptStatus_Processing)), PendingCnt: 1,
	}).Error)
	require.NoError(t, conn.Create(&model.ExptRunLog{
		ID: r2, SpaceID: space, ExptID: expt, ExptRunID: r2,
		Status: gptr.Of(int64(entity.ExptStatus_Pending)), PendingCnt: 7,
	}).Error)

	dao := &ExptRunLogDAOImpl{provider: provider}
	loadRun := func(runID int64) *model.ExptRunLog {
		var row model.ExptRunLog
		require.NoError(t, conn.Where("space_id = ? AND expt_id = ? AND expt_run_id = ?", space, expt, runID).First(&row).Error)
		return &row
	}

	// R1 is current: the write is applied.
	updatedAt := time.Unix(1700000000, 0)
	applied, err := dao.UpdateIfLatestRun(ctx, expt, r1, space, map[string]any{
		"pending_cnt": int32(3), "status": int64(entity.ExptStatus_Failed), "updated_at": updatedAt,
	})
	require.NoError(t, err)
	require.True(t, applied)
	stored := loadRun(r1)
	require.Equal(t, int32(3), stored.PendingCnt)
	require.Equal(t, int64(entity.ExptStatus_Failed), gptr.Indirect(stored.Status))

	// A same-values rewrite affects zero rows but still owns the experiment.
	// Prove the fixture really is an idempotent rewrite, so the DAO must have
	// taken its verified-ownership branch rather than the RowsAffected branch.
	probe := conn.Exec("UPDATE expt_run_log SET pending_cnt = ?, status = ?, updated_at = ? "+
		"WHERE space_id = ? AND expt_id = ? AND expt_run_id = ? AND deleted_at IS NULL",
		int32(3), int64(entity.ExptStatus_Failed), updatedAt, space, expt, r1)
	require.NoError(t, probe.Error)
	require.Zero(t, probe.RowsAffected, "fixture must be a genuine zero-row rewrite")
	applied, err = dao.UpdateIfLatestRun(ctx, expt, r1, space, map[string]any{
		"pending_cnt": int32(3), "status": int64(entity.ExptStatus_Failed), "updated_at": updatedAt,
	})
	require.NoError(t, err)
	require.True(t, applied, "zero-row idempotent rewrite must not be mistaken for ownership loss")

	// R2 replaces R1: the stale write is rejected and nothing is persisted.
	require.NoError(t, conn.Exec("UPDATE experiment SET latest_run_id = ? WHERE id = ? AND space_id = ?", r2, expt, space).Error)
	applied, err = dao.UpdateIfLatestRun(ctx, expt, r1, space, map[string]any{
		"pending_cnt": int32(99), "status": int64(entity.ExptStatus_Success), "updated_at": time.Unix(1700000100, 0),
	})
	require.NoError(t, err)
	require.False(t, applied)
	stored = loadRun(r1)
	require.Equal(t, int32(3), stored.PendingCnt, "superseded run log must stay untouched")
	require.Equal(t, int64(entity.ExptStatus_Failed), gptr.Indirect(stored.Status))

	r2Row := loadRun(r2)
	require.Equal(t, int32(7), r2Row.PendingCnt)
	require.Equal(t, int64(entity.ExptStatus_Pending), gptr.Indirect(r2Row.Status))
	var latestRunID, exptStatus int64
	require.NoError(t, conn.Raw("SELECT latest_run_id, status FROM experiment WHERE id = ? AND space_id = ?", expt, space).
		Row().Scan(&latestRunID, &exptStatus))
	require.Equal(t, r2, latestRunID)
	require.Equal(t, int64(entity.ExptStatus_Processing), exptStatus)

	t.Log("REAL_MYSQL_RUN_LOG_LATEST_CAS current-accepted idempotent-accepted replacement-rejected r2-state-preserved")
}
