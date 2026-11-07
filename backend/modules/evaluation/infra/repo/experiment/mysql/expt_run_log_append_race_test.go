// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"encoding/json"
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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// TestAppendRunItemIDsMySQL proves the append the legacy retry join performs
// under the completing mutex still commits to item_ids and keeps rejecting
// duplicates, including items appended by a previous run.
func TestAppendRunItemIDsMySQL(t *testing.T) {
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

	base := int64(870000000000000000) + time.Now().UnixNano()%1000000000000
	space, expt, run := base+1, base+2, base+3
	t.Cleanup(func() {
		require.NoError(t, conn.Exec("DELETE FROM expt_run_log WHERE space_id = ? AND expt_id = ?", space, expt).Error)
		sqlDB, err := conn.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	initial, err := entity.EncodeExptRunLogItems([]entity.ExptRunLogItems{{ItemIDs: []int64{10}, CreateAt: gptr.Of(int64(1700000000))}})
	require.NoError(t, err)
	require.NoError(t, conn.Create(&model.ExptRunLog{
		ID: run, SpaceID: space, ExptID: expt, ExptRunID: run,
		Mode: gptr.Of(int32(entity.EvaluationModeRetryItems)), Status: gptr.Of(int64(entity.ExptStatus_Processing)),
		ItemIds: &initial,
	}).Error)

	dao := NewExptRunLogDAO(provider)
	loadIDs := func() []int64 {
		var row model.ExptRunLog
		require.NoError(t, conn.Where("space_id = ? AND expt_id = ? AND expt_run_id = ?", space, expt, run).First(&row).Error)
		var chunks []entity.ExptRunLogItems
		require.NoError(t, json.Unmarshal(gptr.Indirect(row.ItemIds), &chunks))
		var ids []int64
		for _, chunk := range chunks {
			ids = append(ids, chunk.ItemIDs...)
		}
		return ids
	}

	require.NoError(t, dao.AppendRunItemIDs(ctx, expt, run, space, []int64{11}))
	require.Equal(t, []int64{10, 11}, loadIDs(), "appended item must commit into item_ids")

	for _, duplicate := range []int64{11, 10} {
		err := dao.AppendRunItemIDs(ctx, expt, run, space, []int64{duplicate})
		require.Error(t, err)
		statusErr, ok := errorx.FromStatusError(err)
		require.True(t, ok)
		require.Equal(t, int32(errno.EvalItemAlreadyRetryingCode), statusErr.Code())
		require.Equal(t, []int64{10, 11}, loadIDs(), "a rejected duplicate must not mutate item_ids")
	}

	require.NoError(t, dao.AppendRunItemIDs(ctx, expt, run, space, []int64{12, 13}))
	require.Equal(t, []int64{10, 11, 12, 13}, loadIDs())
	t.Log("REAL_MYSQL_APPEND_ITEM_IDS committed duplicates-rejected")
}
