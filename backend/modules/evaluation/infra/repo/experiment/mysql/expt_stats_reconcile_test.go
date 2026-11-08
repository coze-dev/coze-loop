// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

var _ IExptStatsReconcileDAO = &exptStatsDAOImpl{}

func TestExptStatsUpdateIfMatchRejectsStaleReconciliation(t *testing.T) {
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

	base := int64(891000000000000000) + time.Now().UnixNano()%1000000000000
	space, expt := base+1, base+2
	t.Cleanup(func() {
		require.NoError(t, conn.Exec("DELETE FROM expt_stats WHERE space_id=? AND expt_id=?", space, expt).Error)
		sqlDB, err := conn.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, conn.Create(&model.ExptStats{ID: base + 3, SpaceID: space, ExptID: expt, PendingCnt: 1}).Error)

	dao := &exptStatsDAOImpl{db: provider}
	expected := &model.ExptStats{PendingCnt: 1}
	require.NoError(t, conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", space, expt).
		Updates(map[string]any{"pending_cnt": 0, "processing_cnt": 1}).Error)
	applied, err := dao.UpdateByExptIDIfMatch(ctx, expt, space, expected, &model.ExptStats{PendingCnt: 2})
	require.NoError(t, err)
	require.False(t, applied, "a stats version changed by an item commit must reject the stale reconciliation")
	var current model.ExptStats
	require.NoError(t, conn.Where("space_id=? AND expt_id=?", space, expt).Take(&current).Error)
	require.Zero(t, current.PendingCnt)
	require.Equal(t, int32(1), current.ProcessingCnt)

	applied, err = dao.UpdateByExptIDIfMatch(ctx, expt, space, &current, &model.ExptStats{ProcessingCnt: 1, SuccessCnt: 1})
	require.NoError(t, err)
	require.True(t, applied)
	require.NoError(t, conn.Where("space_id=? AND expt_id=?", space, expt).Take(&current).Error)
	require.Equal(t, int32(1), current.SuccessCnt)
	t.Log("REAL_MYSQL_STATS_RECONCILE_CAS stale-rejected current-accepted")
}
