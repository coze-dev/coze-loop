// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// newTurnResultTestDAO 用 sqlmock(正则匹配器)起真实 GORM 连接：
// ExpectQuery 的正则同时是「SQL 必须长这样」的断言，匹配不上即查询报错、用例失败，
// 因此 LIMIT / OFFSET 的有无被逐字钉死。
func newTurnResultTestDAO(t *testing.T, ctrl *gomock.Controller) (*ExptTurnResultDAOImpl, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)

	provider := dbmock.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB).AnyTimes()
	return &ExptTurnResultDAOImpl{provider: provider}, mock, func() { _ = sqlDB.Close() }
}

// TestExptTurnResultDAO_ListTurnResultByItemIDs_FirstPageHasLimit 回归：第一页必须带 LIMIT。
//
// 原实现是 `if page.Offset() > 0 && page.Limit() > 0`，而 Page.Offset() = (offset-1)*limit，
// 第一页(offset=1)恒为 0 —— 两个条件相 AND 导致第一页**完全不加 LIMIT**，
// 该实验所有 turn 全表返回，再叠加下游 BatchGet + PayloadBuilder 组装，页面直接卡死。
// 本用例钉死：第一页也必须出现 LIMIT。
func TestExptTurnResultDAO_ListTurnResultByItemIDs_FirstPageHasLimit(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, closeFn := newTurnResultTestDAO(t, ctrl)
	defer closeFn()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `expt_turn_result`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(940))
	// 关键断言：LIMIT 必须在，且不带 OFFSET（第一页 offset 为 0）；ORDER BY 保证翻页确定性
	mock.ExpectQuery("SELECT \\* FROM `expt_turn_result` WHERE space_id = \\? AND expt_id = \\? AND `expt_turn_result`\\.`deleted_at` IS NULL ORDER BY item_id,turn_id LIMIT \\?$").
		WithArgs(int64(1), int64(2), 20).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, total, err := dao.ListTurnResultByItemIDs(context.Background(), 1, 2, nil, entity.NewPage(1, 20), false)
	require.NoError(t, err)
	assert.Equal(t, int64(940), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestExptTurnResultDAO_ListTurnResultByItemIDs_SecondPageHasLimitOffset 第二页仍须 LIMIT + OFFSET。
func TestExptTurnResultDAO_ListTurnResultByItemIDs_SecondPageHasLimitOffset(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, closeFn := newTurnResultTestDAO(t, ctrl)
	defer closeFn()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `expt_turn_result`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(940))
	mock.ExpectQuery("SELECT \\* FROM `expt_turn_result` WHERE space_id = \\? AND expt_id = \\? AND `expt_turn_result`\\.`deleted_at` IS NULL ORDER BY item_id,turn_id LIMIT \\? OFFSET \\?$").
		WithArgs(int64(1), int64(2), 20, 20).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, _, err := dao.ListTurnResultByItemIDs(context.Background(), 1, 2, nil, entity.NewPage(2, 20), false)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestExptTurnResultDAO_ListTurnResultByItemIDs_ZeroPageNoLimit 空 Page 仍是全量语义。
//
// 这是加速器路径「itemIDs 已是精确集合、无需再分页」时的刻意用法（上游 page = entity.Page{}），
// 此处钉死它不会被本次修复误伤成带 LIMIT。
func TestExptTurnResultDAO_ListTurnResultByItemIDs_ZeroPageNoLimit(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, closeFn := newTurnResultTestDAO(t, ctrl)
	defer closeFn()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `expt_turn_result`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	// 无 LIMIT / 无 OFFSET / 也不加 ORDER BY（不分页就没有翻页一致性问题，
	// 保持原 SQL 形状，避免给大结果集平白引入排序开销）
	mock.ExpectQuery("SELECT \\* FROM `expt_turn_result` WHERE space_id = \\? AND expt_id = \\? AND item_id IN \\(\\?,\\?\\) AND `expt_turn_result`\\.`deleted_at` IS NULL$").
		WithArgs(int64(1), int64(2), int64(10), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, _, err := dao.ListTurnResultByItemIDs(context.Background(), 1, 2, []int64{10, 11}, entity.Page{}, false)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestExptTurnResultDAO_ListTurnResultByItemIDs_PagedQueryIsOrdered
// 钉死「分页必须带确定顺序」这个不变量。
//
// 回归背景：把第一页的 LIMIT 修回来之后，UpsertExptTurnResultFilter 里那个
// `offset++` 循环才第一次真正开始逐页翻取（此前第一页无 LIMIT 拿到全量后就 break 了，
// 循环形同虚设）。而本查询原先没有 ORDER BY —— 无序结果做 LIMIT/OFFSET 翻页，
// 行序由执行计划决定、并发写入还会让行位移，会漏行/重复行。
// 漏的 turn 落不进 CK 筛选表，后续筛选就永久查不到这些行 —— 又一个静默错数据。
//
// 排序键取 (item_id, turn_id)：与唯一键 uk_expt_item_turn 的后两段一致，
// 前两段 space_id/expt_id 已在 WHERE 固定，因此走索引、不产生额外 filesort。
func TestExptTurnResultDAO_ListTurnResultByItemIDs_PagedQueryIsOrdered(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, closeFn := newTurnResultTestDAO(t, ctrl)
	defer closeFn()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `expt_turn_result`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(940))
	// ORDER BY 必须出现在 LIMIT 之前；缺了它本用例转红
	mock.ExpectQuery("ORDER BY item_id,turn_id LIMIT \\? OFFSET \\?$").
		WithArgs(int64(1), int64(2), 200, 400).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	// offset=3, limit=200 → 对应 UpsertExptTurnResultFilter 循环的第三页
	_, _, err := dao.ListTurnResultByItemIDs(context.Background(), 1, 2, nil, entity.NewPage(3, 200), false)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
