// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// newAggrTestDAO 用 sqlmock(正则匹配器)起一个真实 GORM 连接, 返回 DAO + mock。
// sqlmock 用 ExpectQuery 的正则匹配实际执行的 SQL, 匹配不上会让查询失败, 从而断言 SQL 形状。
func newAggrTestDAO(t *testing.T, ctrl *gomock.Controller) (*EvaluatorRecordDAOImpl, sqlmock.Sqlmock, func()) {
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
	dao := &EvaluatorRecordDAOImpl{provider: mockProvider}
	return dao, mock, func() { _ = sqlDB.Close() }
}

// TestBatchGetEvaluatorRecordForAggr_SQL 锁定聚合窄查询的核心契约:
// 只投影 id/score/status(不取 input_data/output_data/ext), 且带 status + score IS NOT NULL + 软删除过滤。
// ExpectQuery 的正则同时充当"SQL 必须长这样"的断言——匹配不上则查询返回错误, 用例失败。
func TestBatchGetEvaluatorRecordForAggr_SQL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, cleanup := newAggrTestDAO(t, ctrl)
	defer cleanup()

	score3 := 3.0
	score5 := 5.0
	rows := sqlmock.NewRows([]string{"id", "score", "status"}).
		AddRow(int64(1), score3, int32(entity.EvaluatorRunStatusSuccess)).
		AddRow(int64(2), score5, int32(entity.EvaluatorRunStatusSuccess))

	// 正则逐段断言: 只选 id/score/status, WHERE 带 id IN / status / score IS NOT NULL / deleted_at(软删除)。
	// 负向: 不出现三个 mediumblob 大字段。
	mock.ExpectQuery("SELECT `id`,`score`,`status` FROM `evaluator_record` WHERE id IN .+ AND status = .+ AND score IS NOT NULL AND `evaluator_record`.`deleted_at` IS NULL").
		WillReturnRows(rows)

	got, err := dao.BatchGetEvaluatorRecordForAggr(context.Background(), []int64{1, 2, 3})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Len(t, got, 2)
	assert.Equal(t, int64(1), got[0].ID)
	assert.Equal(t, score3, *got[0].Score)
	assert.Equal(t, int32(entity.EvaluatorRunStatusSuccess), got[1].Status)
}

func TestBatchGetEvaluatorRecordForAggr_DAOError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, cleanup := newAggrTestDAO(t, ctrl)
	defer cleanup()

	mock.ExpectQuery("SELECT .+ FROM `evaluator_record`").
		WillReturnError(errors.New("db boom"))

	got, err := dao.BatchGetEvaluatorRecordForAggr(context.Background(), []int64{1})
	assert.Error(t, err)
	assert.Nil(t, got)
}
