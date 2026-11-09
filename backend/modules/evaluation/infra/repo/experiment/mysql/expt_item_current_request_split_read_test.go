// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type capabilityTestProvider struct {
	db.Provider
	supportsCrossTableJoin bool
}

func (p *capabilityTestProvider) SupportsCrossTableJoin() bool { return p.supportsCrossTableJoin }

func newCurrentRequestTestDAO(t *testing.T, ctrl *gomock.Controller, supportsCrossTableJoin bool) (*exptItemResultDAOImpl, sqlmock.Sqlmock, *gorm.DB) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	provider := dbmock.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB).AnyTimes()
	dao := &exptItemResultDAOImpl{provider: &capabilityTestProvider{Provider: provider, supportsCrossTableJoin: supportsCrossTableJoin}}
	return dao, mock, gormDB
}

// 模拟不支持跨表 JOIN 的存储：读取路径必须不产生 JOIN。
func rejectCrossTableJoin(t *testing.T, conn *gorm.DB) {
	t.Helper()
	require.NoError(t, conn.Callback().Query().Before("gorm:query").Register("reject_join", func(tx *gorm.DB) {
		callbacks.BuildQuerySQL(tx)
		if strings.Contains(strings.ToUpper(tx.Statement.SQL.String()), " JOIN ") {
			_ = tx.AddError(errors.New("Error 1105 (HY000): statement not supported"))
		}
	}))
}

func TestCurrentRequestReadWithoutCrossTableJoin(t *testing.T) {
	ctrl := gomock.NewController(t)
	dao, mock, conn := newCurrentRequestTestDAO(t, ctrl, false)
	rejectCrossTableJoin(t, conn)

	now := time.Now().Truncate(time.Second)
	mock.ExpectQuery("FROM `expt_item_result_run_log`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "item_id", "item_version_id", "status", "result_state", "retry_times", "quota_reservation_state", "updated_at"}).
			AddRow(int64(101), int64(11), int64(31), int32(entity.ItemRunState_Queueing), int32(0), int32(0), int32(0), now).
			AddRow(int64(102), int64(12), int64(31), int32(entity.ItemRunState_Queueing), int32(0), int32(0), int32(0), now))
	mock.ExpectQuery("FROM `expt_item_result`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "item_id", "expt_run_id", "status", "item_version_id", "ext"}).
			AddRow(int64(21), int64(11), int64(8), int32(entity.ItemRunState_Queueing), int64(31), []byte(`{}`)).
			AddRow(int64(22), int64(12), int64(8), int32(entity.ItemRunState_Queueing), int64(31), []byte(`{}`)))
	mock.ExpectQuery("FROM `expt_run_log`").
		WillReturnRows(sqlmock.NewRows([]string{"mode", "status"}).
			AddRow(int32(entity.EvaluationModeSubmit), int64(entity.ExptStatus_Processing)))

	rows, err := dao.readCurrentRequests(context.Background(), 9, 7, 8, nil, false)
	require.NoError(t, err, "store without join support must be read as single-table queries")
	require.Len(t, rows, 2)
	for i, itemID := range []int64{11, 12} {
		require.Equal(t, itemID, rows[i].ItemID)
		require.Equal(t, int64(31), rows[i].ItemVersionID)
		require.Equal(t, entity.ItemRunState_Queueing, rows[i].Status)
		require.Equal(t, entity.EvaluationModeSubmit, rows[i].Mode)
		require.Empty(t, rows[i].RequestID)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCurrentRequestReadKeepsJoinByDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	dao, mock, _ := newCurrentRequestTestDAO(t, ctrl, true)

	mock.ExpectQuery("SELECT .*l.retry_times.*LEFT JOIN expt_item_result AS c.*LEFT JOIN expt_run_log AS r.*l.deleted_at IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"item_id", "item_version_id", "canonical_item_version_id", "canonical_id", "canonical_run_id", "status", "canonical_status", "retry_times", "quota_reservation_state", "mode", "run_status", "ext"}).
			AddRow(int64(11), int64(0), int64(0), int64(21), int64(8), int32(0), int32(0), int32(0), int32(0), int32(entity.EvaluationModeSubmit), int64(entity.ExptStatus_Processing), []byte(`{}`)))

	rows, err := dao.readCurrentRequests(context.Background(), 9, 7, 8, nil, false)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(11), rows[0].ItemID)
	require.NoError(t, mock.ExpectationsWereMet())
}
