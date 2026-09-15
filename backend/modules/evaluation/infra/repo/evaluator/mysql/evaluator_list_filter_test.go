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

// TestEvaluatorDAOImpl_ListEvaluator_UpdaterIDsFilter 验证按更新人（updater_ids）筛选时，
// 查询会带上 updated_by IN 过滤条件（与 creator_ids -> created_by IN 平行）。
func TestEvaluatorDAOImpl_ListEvaluator_UpdaterIDsFilter(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	mockProvider := dbmock.NewMockProvider(ctrl)
	mockProvider.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB).AnyTimes()

	// 期望 count 与 find 两次查询都带上 updated_by IN 过滤条件
	countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
	mock.ExpectQuery("SELECT count\\(\\*\\).+updated_by IN").
		WithArgs(int64(100), int64(1001), int64(1002)).
		WillReturnRows(countRows)

	findRows := sqlmock.NewRows([]string{"id"})
	mock.ExpectQuery("SELECT \\*.+updated_by IN").
		WithArgs(int64(100), int64(1001), int64(1002)).
		WillReturnRows(findRows)

	dao := &EvaluatorDAOImpl{provider: mockProvider}

	resp, err := dao.ListEvaluator(context.Background(), &ListEvaluatorRequest{
		SpaceID:    100,
		UpdaterIDs: []int64{1001, 1002},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
