// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	gomysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func newManualRetrySQLProvider(t *testing.T) (db.Provider, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	provider, err := db.NewDB(gomysql.New(gomysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	return provider, mock
}

func TestManualRetryStatsAtomicUpdateError(t *testing.T) {
	provider, mock := newManualRetrySQLProvider(t)
	want := errors.New("stats write failed")
	mock.ExpectExec("UPDATE `expt_stats` SET .*`fail_cnt`=fail_cnt \\+ \\?.*`pending_cnt`=pending_cnt \\+ \\?.*WHERE \\(space_id = \\? AND expt_id = \\?\\)").
		WillReturnError(want)
	err := NewExptStatsDAO(provider).ArithOperateCount(context.Background(), 7, 9, &entity.StatsCntArithOp{OpStatusCnt: map[entity.ItemRunState]int{entity.ItemRunState_Fail: -1, entity.ItemRunState_Queueing: 1}})
	require.ErrorContains(t, err, "stats write failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestManualRetryDAOTransactionOptions(t *testing.T) {
	provider, rootMock := newManualRetrySQLProvider(t)
	txProvider, mock := newManualRetrySQLProvider(t)
	expt, ok := any(NewExptDAO(provider)).(interface {
		GetByID(context.Context, int64, ...db.Option) (*model.Experiment, error)
		UpdateFields(context.Context, int64, map[string]any, ...db.Option) error
	})
	require.True(t, ok, "experiment DAO must consume transaction options")
	stats, ok := any(NewExptStatsDAO(provider)).(interface {
		Get(context.Context, int64, int64, ...db.Option) (*model.ExptStats, error)
		UpdateByExptID(context.Context, int64, int64, *model.ExptStats, ...db.Option) error
		ArithOperateCount(context.Context, int64, int64, *entity.StatsCntArithOp, ...db.Option) error
	})
	require.True(t, ok, "stats DAO must consume transaction options")
	ctx := context.Background()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `experiment` WHERE .*`id` = \\?.*FOR UPDATE").WithArgs(int64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectExec("UPDATE `experiment` SET `latest_run_id`=\\? WHERE .*`id` = \\?").WithArgs(int64(8), int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM `expt_stats` WHERE .*`expt_id` = \\?.*`space_id` = \\?.*FOR UPDATE").WithArgs(int64(7), int64(9), 1).
		WillReturnRows(sqlmock.NewRows([]string{"expt_id", "space_id"}).AddRow(7, 9))
	mock.ExpectExec("UPDATE `expt_stats` SET .*`fail_cnt`=\\?.*`pending_cnt`=\\?.*`processing_cnt`=\\?.*`success_cnt`=\\?.*`terminated_cnt`=\\?.*WHERE .*`expt_id` = \\?.*`space_id` = \\?").
		WithArgs(int32(0), int32(1), int32(0), int32(0), int32(0), sqlmock.AnyArg(), int64(7), int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM `expt_run_log` WHERE \\(id = \\? AND expt_id = \\? AND expt_run_id = \\?\\).*FOR UPDATE").WithArgs(int64(8), int64(7), int64(8), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "expt_id", "expt_run_id", "space_id"}).AddRow(8, 7, 8, 9))
	mock.ExpectExec("UPDATE `expt_run_log` SET `status_message`=\\? WHERE .*`space_id` = \\?.*`expt_id` = \\?.*`expt_run_id` = \\?").WithArgs(nil, int64(9), int64(7), int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `expt_stats` SET `fail_cnt`=fail_cnt \\+ \\?,`pending_cnt`=pending_cnt \\+ \\?,`updated_at`=\\? WHERE \\(space_id = \\? AND expt_id = \\?\\)").
		WithArgs(-1, 1, sqlmock.AnyArg(), int64(9), int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err := txProvider.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []db.Option{db.WithTransaction(tx), db.WithMaster(), db.WithSelectForUpdate()}
		_, err := expt.GetByID(ctx, 7, opts...)
		if err != nil {
			return err
		}
		if err = expt.UpdateFields(ctx, 7, map[string]any{"latest_run_id": int64(8)}, opts...); err != nil {
			return err
		}
		if _, err = stats.Get(ctx, 7, 9, opts...); err != nil {
			return err
		}
		if err = stats.UpdateByExptID(ctx, 7, 9, &model.ExptStats{PendingCnt: 1}, opts...); err != nil {
			return err
		}
		runDAO := NewExptRunLogDAO(provider)
		if _, err = runDAO.Get(ctx, 7, 8, opts...); err != nil {
			return err
		}
		if err = runDAO.Update(ctx, 7, 8, 9, map[string]any{"status_message": nil}, opts...); err != nil {
			return err
		}
		return stats.ArithOperateCount(ctx, 7, 9, &entity.StatsCntArithOp{OpStatusCnt: map[entity.ItemRunState]int{entity.ItemRunState_Fail: -1, entity.ItemRunState_Queueing: 1}}, opts...)
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.NoError(t, rootMock.ExpectationsWereMet())
}

func TestManualRetryRefsDeleteScoped(t *testing.T) {
	provider, rootMock := newManualRetrySQLProvider(t)
	txProvider, mock := newManualRetrySQLProvider(t)
	dao, ok := any(NewExptTurnEvaluatorResultRefDAO(provider)).(interface {
		DeleteByTurnResultIDs(context.Context, int64, int64, []int64, []int64, ...db.Option) error
	})
	require.True(t, ok, "refs deletion must be scoped and preserve selected ref IDs")
	ctx := context.Background()
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `expt_turn_evaluator_result_ref` WHERE \\(space_id = \\? AND expt_id = \\? AND expt_turn_result_id IN \\(\\?,\\?\\)\\) AND id NOT IN \\(\\?\\)").
		WithArgs(int64(9), int64(7), int64(11), int64(12), int64(100)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM `expt_turn_evaluator_result_ref` WHERE space_id = \\? AND expt_id = \\? AND expt_turn_result_id IN \\(\\?\\)").
		WithArgs(int64(9), int64(7), int64(13)).WillReturnError(errors.New("delete failed"))
	mock.ExpectRollback()
	err := txProvider.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []db.Option{db.WithTransaction(tx), db.WithMaster()}
		if err := dao.DeleteByTurnResultIDs(ctx, 9, 7, nil, nil, opts...); err != nil {
			return err
		}
		if err := dao.DeleteByTurnResultIDs(ctx, 9, 7, []int64{11, 12}, []int64{100}, opts...); err != nil {
			return err
		}
		return dao.DeleteByTurnResultIDs(ctx, 9, 7, []int64{13}, nil, opts...)
	})
	require.ErrorContains(t, err, "delete failed")
	require.NoError(t, mock.ExpectationsWereMet())
	require.NoError(t, rootMock.ExpectationsWereMet())
}
