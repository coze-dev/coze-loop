// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type reviewTransactionalItemDAO interface {
	AuthorizeItemRetry(context.Context, int64, int64, int64, int64, int32, []byte, ...db.Option) (bool, error)
	UpdateItemRunLogIfNonterminal(context.Context, int64, int64, []int64, map[string]any, int64, ...db.Option) error
	ScanItemRunLogsForCancellation(context.Context, int64, int64, *int64, []int64, int64, int64, ...db.Option) ([]*model.ExptItemResultRunLog, int64, error)
}

type reviewTransactionDAO struct {
	mysql.IExptItemResultDAO
	delegate reviewTransactionalItemDAO
	option   db.Option
}

func (d reviewTransactionDAO) AuthorizeItemRetry(ctx context.Context, space, expt, run, item int64, retry int32, msg []byte, opts ...db.Option) (bool, error) {
	return d.delegate.AuthorizeItemRetry(ctx, space, expt, run, item, retry, msg, append(opts, d.option)...)
}
func (d reviewTransactionDAO) UpdateItemRunLogIfNonterminal(ctx context.Context, expt, run int64, items []int64, fields map[string]any, space int64, opts ...db.Option) error {
	return d.delegate.UpdateItemRunLogIfNonterminal(ctx, expt, run, items, fields, space, append(opts, d.option)...)
}
func (d reviewTransactionDAO) ScanItemRunLogsForCancellation(ctx context.Context, space, expt int64, run *int64, items []int64, cursor, limit int64, opts ...db.Option) ([]*model.ExptItemResultRunLog, int64, error) {
	return d.delegate.ScanItemRunLogsForCancellation(ctx, space, expt, run, items, cursor, limit, append(opts, d.option)...)
}

func TestReviewItemDAOOptionsPreserveCallerTransactionRollback(t *testing.T) {
	ctx := context.Background()
	provider := db.NewTestDB(t, &model.ExptItemResultRunLog{})
	require.NotNil(t, provider)
	dao := mysql.NewExptItemResultDAO(provider)
	delegate, ok := dao.(reviewTransactionalItemDAO)
	require.True(t, ok, "the three new DAO methods must accept caller transaction options")
	items := NewExptItemResultRepo(dao)
	require.NoError(t, items.BatchCreateNXRunLogs(ctx, []*entity.ExptItemResultRunLog{{ID: 1, SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 4, Status: int32(entity.ItemRunState_Processing)}}))
	rollback := errors.New("rollback caller transaction")
	err := provider.Transaction(ctx, func(tx *gorm.DB) error {
		scoped := NewExptItemResultRepo(reviewTransactionDAO{IExptItemResultDAO: dao, delegate: delegate, option: db.WithTransaction(tx)})
		claimed, err := scoped.(repo.IExptItemRetryRepo).AuthorizeItemRetry(ctx, 3, 1, 2, 4, 0, []byte("retry"))
		require.NoError(t, err)
		require.True(t, claimed)
		require.NoError(t, scoped.(repo.IExptItemCancellationRepo).UpdateItemRunLogIfNonterminal(ctx, 1, 2, []int64{4}, map[string]any{"status": int32(entity.ItemRunState_Terminal)}, 3))
		rows, _, err := scoped.(repo.IExptItemCancellationRepo).ScanItemRunLogsForCancellation(ctx, 3, 1, nil, nil, 0, 10)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, int32(entity.ItemRunState_Terminal), rows[0].Status)
		require.Equal(t, int32(1), rows[0].RetryTimes)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	row, err := items.GetItemRunLog(ctx, 1, 2, 4, 3)
	require.NoError(t, err)
	require.Equal(t, int32(entity.ItemRunState_Processing), row.Status)
	require.Equal(t, int32(0), row.RetryTimes)
}
