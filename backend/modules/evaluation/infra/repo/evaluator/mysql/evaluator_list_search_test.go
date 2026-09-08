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

// newEvaluatorTestDAO 用 sqlmock(正则匹配器)起一个真实 GORM 连接, 返回 EvaluatorDAOImpl + mock。
func newEvaluatorTestDAO(t *testing.T, ctrl *gomock.Controller) (*EvaluatorDAOImpl, sqlmock.Sqlmock, func()) {
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

// TestListEvaluator_SearchDescription_SQL 锁定描述模糊搜索契约:
// SearchDescription 非空时, count 与 find 两次查询都带 `description LIKE ?` 且参数为 `%<值>%`。
// ExpectQuery 的正则同时充当"SQL 必须带此条件"的断言, WithArgs 断言 LIKE 参数形状。
func TestListEvaluator_SearchDescription_SQL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, cleanup := newEvaluatorTestDAO(t, ctrl)
	defer cleanup()

	// count 查询: WHERE 带 description LIKE ?, 参数 %desc%
	mock.ExpectQuery("SELECT count.+ FROM `evaluator` WHERE space_id = .+ AND description LIKE .+").
		WithArgs(int64(1), "%desc%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	// find 查询: 同样带 description LIKE ?, 参数 %desc%
	mock.ExpectQuery("SELECT .+ FROM `evaluator` WHERE space_id = .+ AND description LIKE .+").
		WithArgs(int64(1), "%desc%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "test"))

	resp, err := dao.ListEvaluator(context.Background(), &ListEvaluatorRequest{
		SpaceID:           1,
		SearchDescription: "desc",
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, int64(1), resp.TotalCount)
}

// TestListEvaluator_SearchDescription_Empty_NoLike 反向锁定契约:
// SearchDescription 为空串时, count 与 find 两次查询的 WHERE 都不得带 `description LIKE ?`。
// 双重机器可判定证据:
//  1. 正则要求 `space_id = ?` 紧接 `AND deleted_at IS NULL`, 中间不允许出现任何 LIKE 子句;
//  2. WithArgs 只声明一个 int64(1) 参数 —— 若 DAO 误加 LIKE, 会多出一个 `%%` 实参导致参数数量不匹配。
//
// 任一被破坏都会让 ExpectationsWereMet 报错, 从而证明"空值不加过滤"。
func TestListEvaluator_SearchDescription_Empty_NoLike(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, cleanup := newEvaluatorTestDAO(t, ctrl)
	defer cleanup()

	// count 查询: space_id = ? 紧接 deleted_at IS NULL(中间无 LIKE), 仅一个参数。
	// GORM 软删除模型会自行追加 `evaluator`.`deleted_at` IS NULL 收尾。
	mock.ExpectQuery("SELECT count.+ FROM `evaluator` WHERE space_id = \\? AND deleted_at IS NULL AND .evaluator...deleted_at. IS NULL$").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	// find 查询: 同样 space_id = ? 紧接 deleted_at(无 LIKE), 仅一个参数。
	mock.ExpectQuery("SELECT .+ FROM `evaluator` WHERE space_id = \\? AND deleted_at IS NULL AND .evaluator...deleted_at. IS NULL.*$").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "test"))

	resp, err := dao.ListEvaluator(context.Background(), &ListEvaluatorRequest{
		SpaceID:           1,
		SearchDescription: "",
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, int64(1), resp.TotalCount)
}
