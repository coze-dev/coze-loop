// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
)

func newExptItemResultLogIDTestDAO(t *testing.T, ctrl *gomock.Controller) (*exptItemResultDAOImpl, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)

	provider := dbmock.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB).AnyTimes()
	return &exptItemResultDAOImpl{provider: provider}, mock
}

func TestExptItemResultDAO_FillItemRunLogLogIDIfEmpty_EmptyMapDoesNotAccessDB(t *testing.T) {
	ctrl := gomock.NewController(t)
	provider := dbmock.NewMockProvider(ctrl)
	dao := &exptItemResultDAOImpl{provider: provider}

	err := dao.FillItemRunLogLogIDIfEmpty(context.Background(), 101, 202, 303, map[int64]string{})
	require.NoError(t, err)
}

func TestExptItemResultDAO_FillItemRunLogLogIDIfEmpty_UpdatesMultipleItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	dao, mock := newExptItemResultLogIDTestDAO(t, ctrl)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `expt_item_result_run_log` SET `log_id`=CASE item_id WHEN \\? THEN \\? WHEN \\? THEN \\? ELSE log_id END WHERE \\(space_id = \\? AND expt_id = \\? AND expt_run_id = \\? AND item_id IN \\(\\?,\\?\\) AND log_id = ''\\) AND `expt_item_result_run_log`\\.`deleted_at` IS NULL").
		WithArgs(
			int64(11), "log-11",
			int64(22), "log-22",
			int64(303), int64(101), int64(202),
			int64(11), int64(22),
		).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	err := dao.FillItemRunLogLogIDIfEmpty(context.Background(), 101, 202, 303, map[int64]string{
		22: "log-22",
		11: "log-11",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExptItemResultDAO_FillItemRunLogLogIDIfEmpty_PropagatesUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	dao, mock := newExptItemResultLogIDTestDAO(t, ctrl)
	writeErr := errors.New("write failed")

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `expt_item_result_run_log`").
		WillReturnError(writeErr)
	mock.ExpectRollback()

	err := dao.FillItemRunLogLogIDIfEmpty(context.Background(), 101, 202, 303, map[int64]string{11: "log-11"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "FillItemRunLogLogIDIfEmpty fail")
	assert.ErrorContains(t, err, writeErr.Error())
	require.NoError(t, mock.ExpectationsWereMet())
}
