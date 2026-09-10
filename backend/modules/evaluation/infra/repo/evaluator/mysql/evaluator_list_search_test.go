// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
)

// newListSearchTestDAO 用 sqlmock(正则匹配器)起真实 GORM 连接, 返回 DAO + mock。
// ExpectQuery 的正则同时充当 SQL 形状断言, 匹配不上查询即失败。
func newListSearchTestDAO(t *testing.T, ctrl *gomock.Controller) (*EvaluatorDAOImpl, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}
	mockProvider := dbmock.NewMockProvider(ctrl)
	mockProvider.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB).AnyTimes()
	dao := &EvaluatorDAOImpl{provider: mockProvider}
	return dao, mock, func() { _ = sqlDB.Close() }
}

// TestListEvaluator_SearchDescription_SQL 断言: 当 SearchDescription 非空时,
// count 查询与 find 查询的 SQL 都带上 description LIKE ? 且参数为 %desc%。
func TestListEvaluator_SearchDescription_SQL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, cleanup := newListSearchTestDAO(t, ctrl)
	defer cleanup()

	// count 查询: 带 description LIKE ? 且参数 %desc%
	mock.ExpectQuery("SELECT count.+FROM `evaluator`.+description LIKE \\?").
		WithArgs(int64(1), "%desc%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	// find 查询: 同样带 description LIKE ? 且参数 %desc%
	mock.ExpectQuery("SELECT \\* FROM `evaluator`.+description LIKE \\?").
		WithArgs(int64(1), "%desc%").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	resp, err := dao.ListEvaluator(context.Background(), &ListEvaluatorRequest{
		SpaceID:           1,
		SearchDescription: "desc",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NoError(t, mock.ExpectationsWereMet())
}
