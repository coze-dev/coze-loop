// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestTaskDaoImpl_ListNonFinalTasks(t *testing.T) {
	ctx := context.Background()

	t.Run("only non-final statuses", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })
		gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		provider := mocks.NewMockProvider(ctrl)
		provider.EXPECT().NewSession(ctx).Return(gormDB)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `task` WHERE `task`.`task_status` IN (?,?,?)")).
			WithArgs(string(entity.TaskStatusUnstarted), string(entity.TaskStatusRunning), string(entity.TaskStatusPending)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "task_status"}).
				AddRow(1, string(entity.TaskStatusUnstarted)).
				AddRow(2, string(entity.TaskStatusRunning)).
				AddRow(3, string(entity.TaskStatusPending)))

		tasks, err := NewTaskDaoImpl(provider).ListNonFinalTasks(ctx)
		require.NoError(t, err)
		require.Len(t, tasks, 3)
		assert.Equal(t, []string{
			string(entity.TaskStatusUnstarted),
			string(entity.TaskStatusRunning),
			string(entity.TaskStatusPending),
		}, []string{tasks[0].TaskStatus, tasks[1].TaskStatus, tasks[2].TaskStatus})
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty result", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })
		gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		provider := mocks.NewMockProvider(ctrl)
		provider.EXPECT().NewSession(ctx).Return(gormDB)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `task` WHERE `task`.`task_status` IN (?,?,?)")).
			WithArgs("unstarted", "running", "pending").
			WillReturnRows(sqlmock.NewRows([]string{"id", "task_status"}))

		tasks, err := NewTaskDaoImpl(provider).ListNonFinalTasks(ctx)
		require.NoError(t, err)
		assert.Empty(t, tasks)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })
		gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		provider := mocks.NewMockProvider(ctrl)
		provider.EXPECT().NewSession(ctx).Return(gormDB)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `task` WHERE `task`.`task_status` IN (?,?,?)")).
			WithArgs("unstarted", "running", "pending").
			WillReturnError(errors.New("database unavailable"))

		tasks, err := NewTaskDaoImpl(provider).ListNonFinalTasks(ctx)
		require.Error(t, err)
		assert.Nil(t, tasks)
		statusErr, ok := errorx.FromStatusError(err)
		require.True(t, ok)
		assert.Equal(t, int32(obErrorx.CommonMySqlErrorCode), statusErr.Code())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
