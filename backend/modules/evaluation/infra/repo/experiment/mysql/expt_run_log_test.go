// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestExptRunLogDAO_Update_SpaceScopedSQL(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	provider, err := db.NewDB(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)

	mock.ExpectExec("UPDATE `expt_run_log` SET `status`=? WHERE `expt_run_log`.`space_id` = ? AND `expt_run_log`.`expt_id` = ? AND `expt_run_log`.`expt_run_id` = ? AND `expt_run_log`.`deleted_at` IS NULL").
		WithArgs(int64(15), int64(789), int64(123), int64(456)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = NewExptRunLogDAO(provider).Update(context.Background(), 123, 456, 789, map[string]any{"status": int64(15)})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExptRunLogDAO_Update_OnlyMatchingLiveRun(t *testing.T) {
	ctx := context.Background()
	provider := db.NewTestDB(t, &model.ExptRunLog{})
	require.NotNil(t, provider)
	session := provider.NewSession(ctx)
	rows := []*model.ExptRunLog{
		{ID: 1, SpaceID: 789, ExptID: 123, ExptRunID: 456, Status: gptr.Of(int64(1))},
		{ID: 2, SpaceID: 790, ExptID: 123, ExptRunID: 456, Status: gptr.Of(int64(2))},
		{ID: 3, SpaceID: 789, ExptID: 124, ExptRunID: 456, Status: gptr.Of(int64(3))},
		{ID: 4, SpaceID: 789, ExptID: 123, ExptRunID: 457, Status: gptr.Of(int64(4))},
		{
			ID: 5, SpaceID: 789, ExptID: 123, ExptRunID: 458, Status: gptr.Of(int64(5)),
			DeletedAt: gorm.DeletedAt{Time: time.Now(), Valid: true},
		},
	}
	require.NoError(t, session.Create(rows).Error)

	dao := NewExptRunLogDAO(provider)
	require.NoError(t, dao.Update(ctx, 123, 456, 789, map[string]any{"status": int64(15)}))
	require.NoError(t, dao.Update(ctx, 123, 458, 789, map[string]any{"status": int64(15)}))
	require.NoError(t, dao.Update(ctx, 123, 456, 999, map[string]any{"status": int64(16)}))

	var got []*model.ExptRunLog
	require.NoError(t, session.Unscoped().Order("id").Find(&got).Error)
	require.Len(t, got, 5)
	for i, want := range []int64{15, 2, 3, 4, 5} {
		require.NotNil(t, got[i].Status)
		require.Equal(t, want, *got[i].Status, "run log id=%d", got[i].ID)
	}
}

// Get 在 exptID 未知（<=0）时必须只按 run id 查：destroy 链路用 Get(0, runID) 反查 expt 作为 sandbox taskID，
// 若这里要求 expt_id 相等，查询恒空会让 taskID 变空串、双沙箱的 -orch 销毁请求被丢弃（沙箱泄漏）。
func TestExptRunLogDAO_Get_ByRunIDOnlyWhenExptUnknown(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	provider, err := db.NewDB(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)

	mock.ExpectQuery("SELECT \\* FROM `expt_run_log` WHERE id = \\? AND `expt_run_log`.`deleted_at` IS NULL").
		WithArgs(int64(456), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "expt_id", "expt_run_id"}).AddRow(456, 123, 456))

	got, err := NewExptRunLogDAO(provider).Get(context.Background(), 0, 456)
	require.NoError(t, err)
	require.Equal(t, int64(123), got.ExptID)
	require.NoError(t, mock.ExpectationsWereMet())
}
