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
	"gorm.io/gorm/clause"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// newItemResultTestDAO 用 sqlmock(正则匹配器)起真实 GORM 连接：ExpectQuery 的正则同时是
// 「SQL 必须长这样」的断言，匹配不上即查询报错、用例失败——因此 ORDER BY / FORCE INDEX / id>cursor
// 被逐字钉死。用于回归 ScanItemRunLogs 的游标分页协议(§4.0)与让位降权排序(§4.4.3)。
func newItemResultTestDAO(t *testing.T, ctrl *gomock.Controller) (*exptItemResultDAOImpl, sqlmock.Sqlmock, func()) {
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
	return &exptItemResultDAOImpl{provider: provider}, mock, func() { _ = sqlDB.Close() }
}

// TestExptItemResultDAO_ScanItemRunLogs_GenBranch_FlagOff_IdAsc 防回归(§6.2a):
// flag=false 时 gen 分支 ORDER BY 与改造前完全一致(id asc)、ForceIndex 仍是 uk_expt_run_item_turn。
func TestExptItemResultDAO_ScanItemRunLogs_GenBranch_FlagOff_IdAsc(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, closeFn := newItemResultTestDAO(t, ctrl)
	defer closeFn()

	mock.ExpectQuery("SELECT \\* FROM `expt_item_result_run_log` FORCE INDEX \\(`uk_expt_run_item_turn`\\).*ORDER BY `expt_item_result_run_log`\\.`id` ASC LIMIT \\?$").
		WillReturnRows(sqlmock.NewRows([]string{"id", "item_id", "status", "retry_times"}).AddRow(101, 10, 0, 0))

	res, ncursor, err := dao.ScanItemRunLogs(context.Background(), 1, 2, &entity.ExptItemRunLogFilter{
		Status: []entity.ItemRunState{entity.ItemRunState_Queueing},
	}, 0, 5, 3)
	require.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, int64(101), ncursor) // 末行 ID 作下一页游标
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestExptItemResultDAO_ScanItemRunLogs_GenBranch_FlagOff_Cursor 防回归(§6.2a 游标翻页):
// flag=false + cursor>0 时必须带 `id > ?` 且仍 id asc → 保证多次翻页不漏不重。
func TestExptItemResultDAO_ScanItemRunLogs_GenBranch_FlagOff_Cursor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, closeFn := newItemResultTestDAO(t, ctrl)
	defer closeFn()

	// 游标条件 id > ? 必须出现, 且 ORDER BY 仍是 id asc(游标分页协议正确性依赖此)
	mock.ExpectQuery("SELECT \\* FROM `expt_item_result_run_log` FORCE INDEX \\(`uk_expt_run_item_turn`\\).*`id` > \\?.*ORDER BY `expt_item_result_run_log`\\.`id` ASC LIMIT \\?$").
		WillReturnRows(sqlmock.NewRows([]string{"id", "item_id", "status", "retry_times"}).AddRow(205, 20, 0, 0))

	res, ncursor, err := dao.ScanItemRunLogs(context.Background(), 1, 2, &entity.ExptItemRunLogFilter{
		Status: []entity.ItemRunState{entity.ItemRunState_Queueing},
	}, 100, 5, 3)
	require.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, int64(205), ncursor)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestExptItemResultDAO_ScanItemRunLogs_GenBranch_FlagOn_RetryOrder 覆盖让位降权排序(§6.2a/§4.4.3):
// flag=true + cursor==0 + 索引已建成(RetryPickIndexReady=true) 时,
// gen 分支 FORCE INDEX 到 idx_expt_run_retry_pick、ORDER BY retry_times,id。
func TestExptItemResultDAO_ScanItemRunLogs_GenBranch_FlagOn_RetryOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, closeFn := newItemResultTestDAO(t, ctrl)
	defer closeFn()

	mock.ExpectQuery("SELECT \\* FROM `expt_item_result_run_log` FORCE INDEX \\(`idx_expt_run_retry_pick`\\).*ORDER BY `expt_item_result_run_log`\\.`retry_times`,`expt_item_result_run_log`\\.`id` LIMIT \\?$").
		WillReturnRows(sqlmock.NewRows([]string{"id", "item_id", "status", "retry_times"}).
			AddRow(101, 10, 0, 0).
			AddRow(102, 11, 0, 2))

	res, ncursor, err := dao.ScanItemRunLogs(context.Background(), 1, 2, &entity.ExptItemRunLogFilter{
		Status:                 []entity.ItemRunState{entity.ItemRunState_Queueing},
		OrderByRetryTimesFirst: true,
		RetryPickIndexReady:    true,
	}, 0, 5, 3)
	require.NoError(t, err)
	assert.Len(t, res, 2)
	assert.Equal(t, int64(102), ncursor)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestExptItemResultDAO_ScanItemRunLogs_GenBranch_FlagOn_IndexNotReady 覆盖「只加列不加索引」形态:
// flag=true 但 RetryPickIndexReady=false(线上未建 idx_expt_run_retry_pick) 时,
// gen 分支【不下 FORCE INDEX hint】、由优化器自选(退化 filesort), 但 ORDER BY 语义必须一字不变。
//
// 为何需要这条: 线上 expt_item_result_run_log 是高频写入热表(实测某库 99.6M 行 / 36.1GB),
// 建 6 列复合索引的构建期风险与写放大高于 filesort 代价, 故先只加列。若此处误下 hint,
// 挑选会直接报 Key doesn't exist 使整个实验调度瘫痪。
func TestExptItemResultDAO_ScanItemRunLogs_GenBranch_FlagOn_IndexNotReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, closeFn := newItemResultTestDAO(t, ctrl)
	defer closeFn()

	// 断言 SELECT 之后紧跟 FROM 表名、中间【没有】FORCE INDEX 片段; ORDER BY 仍是 retry_times,id。
	mock.ExpectQuery("SELECT \\* FROM `expt_item_result_run_log` WHERE.*ORDER BY `expt_item_result_run_log`\\.`retry_times`,`expt_item_result_run_log`\\.`id` LIMIT \\?$").
		WillReturnRows(sqlmock.NewRows([]string{"id", "item_id", "status", "retry_times"}).
			AddRow(201, 20, 0, 0).
			AddRow(202, 21, 0, 3))

	res, ncursor, err := dao.ScanItemRunLogs(context.Background(), 1, 2, &entity.ExptItemRunLogFilter{
		Status:                 []entity.ItemRunState{entity.ItemRunState_Queueing},
		OrderByRetryTimesFirst: true,
		RetryPickIndexReady:    false,
	}, 0, 5, 3)
	require.NoError(t, err)
	assert.Len(t, res, 2)
	assert.Equal(t, int64(202), ncursor)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestExptItemResultDAO_ScanItemRunLogs_RawBranch_FlagOn_IndexNotReady 同上, 覆盖 RawFilter 分支。
// 该分支原先是无条件 Clauses(hints.ForceIndex(...)), 改造后需能整段省略 hint。
func TestExptItemResultDAO_ScanItemRunLogs_RawBranch_FlagOn_IndexNotReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, closeFn := newItemResultTestDAO(t, ctrl)
	defer closeFn()

	mock.ExpectQuery("SELECT \\* FROM `expt_item_result_run_log` WHERE.*ORDER BY retry_times asc, id asc LIMIT \\?$").
		WillReturnRows(sqlmock.NewRows([]string{"id", "item_id", "status", "retry_times"}).
			AddRow(301, 30, 0, 1))

	res, _, err := dao.ScanItemRunLogs(context.Background(), 1, 2, &entity.ExptItemRunLogFilter{
		RawFilter:              true,
		RawCond:                clause.Expr{SQL: "status IN (?)", Vars: []interface{}{[]int32{0}}},
		OrderByRetryTimesFirst: true,
		RetryPickIndexReady:    false,
	}, 0, 5, 3)
	require.NoError(t, err)
	assert.Len(t, res, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestExptItemResultDAO_ScanItemRunLogs_RawBranch_FlagOn_IndexReady RawFilter 分支索引已建成时仍下 hint。
func TestExptItemResultDAO_ScanItemRunLogs_RawBranch_FlagOn_IndexReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, closeFn := newItemResultTestDAO(t, ctrl)
	defer closeFn()

	mock.ExpectQuery("SELECT \\* FROM `expt_item_result_run_log` FORCE INDEX \\(`idx_expt_run_retry_pick`\\).*ORDER BY retry_times asc, id asc LIMIT \\?$").
		WillReturnRows(sqlmock.NewRows([]string{"id", "item_id", "status", "retry_times"}).
			AddRow(401, 40, 0, 0))

	res, _, err := dao.ScanItemRunLogs(context.Background(), 1, 2, &entity.ExptItemRunLogFilter{
		RawFilter:              true,
		RawCond:                clause.Expr{SQL: "status IN (?)", Vars: []interface{}{[]int32{0}}},
		OrderByRetryTimesFirst: true,
		RetryPickIndexReady:    true,
	}, 0, 5, 3)
	require.NoError(t, err)
	assert.Len(t, res, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestExptItemResultDAO_ScanItemRunLogs_FlagOn_CursorRejected 覆盖 §6.2a / §4.0 互斥约束:
// flag=true 且 cursor>0 时立即报错(排序模式与游标翻页互斥), 不发出任何查询。
func TestExptItemResultDAO_ScanItemRunLogs_FlagOn_CursorRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	dao, mock, closeFn := newItemResultTestDAO(t, ctrl)
	defer closeFn()
	// 不设置 ExpectQuery: 若代码发了 SQL, ExpectationsWereMet 之外 sqlmock 会因意外查询报错

	// gen 分支
	_, _, err := dao.ScanItemRunLogs(context.Background(), 1, 2, &entity.ExptItemRunLogFilter{
		Status:                 []entity.ItemRunState{entity.ItemRunState_Queueing},
		OrderByRetryTimesFirst: true,
	}, 50, 5, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incompatible with cursor paging")

	// RawFilter 分支同样拒绝
	_, _, err = dao.ScanItemRunLogs(context.Background(), 1, 2, &entity.ExptItemRunLogFilter{
		RawFilter:              true,
		RawCond:                clause.Expr{SQL: "status IN (?)", Vars: []interface{}{[]int32{0}}},
		OrderByRetryTimesFirst: true,
	}, 50, 5, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incompatible with cursor paging")

	assert.NoError(t, mock.ExpectationsWereMet())
}
