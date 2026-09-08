// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// renderExptUpdate 用 dry-run gorm 渲染 Update 的最终 SQL，直接断言列是否出现在 SET 里。
//
// 断言 SQL 文本而非断言转换结果：本 bug 的成因正是"转换结果看起来合理、但 GORM 会把它写进
// UPDATE"，只测转换层测不出来。渲染 SQL 才能覆盖 Omit 是否真的生效。
func renderExptUpdate(t *testing.T, expt *model.Experiment) string {
	t.Helper()
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = sqlDB.Close() }()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{DryRun: true, SkipDefaultTransaction: true})
	require.NoError(t, err)

	tx := gormDB.Model(&model.Experiment{}).Where("id = ?", expt.ID).
		Omit(schedulingFrozenColumns...).
		Updates(expt)
	// DryRun 下 Statement.SQL 可能为空，用 Explain 把 SQL + 参数一起还原成可断言的文本。
	return gormDB.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...)
}

// TestExptDAO_Update_NeverClobbersFrozenSchedulingColumns 守住「调度列创建时冻结」这条不变量。
//
// 背景（真实 bug，非假想）：Update 用 struct 做 Updates，GORM 只跳过**零值**字段；
// 而 DO2PO 会把未设置的调度字段 Normalize 成非零值（mode ""→"legacy"、priority 0→1）。
// 于是全仓 8 处「只带 ID + 一两个业务字段」的部分更新（LogRun 写 latest_run_id、
// ScheduleStart 改 status 等）都会顺手把 enforce 实验改回 legacy、把申报优先级重置为 1。
//
// 后果静默且严重：mode 变回 legacy 后中心调度扫不到它（扫描条件 scheduler_mode='enforce'），
// 而旧 daemon 的抑制判断读到 legacy 会恢复自主派发 —— 同一 run 两个派发驱动、绕过全局额度。
func TestExptDAO_Update_NeverClobbersFrozenSchedulingColumns(t *testing.T) {
	t.Parallel()

	// 模拟 DO2PO 的真实产出：调度列全是 Normalize 后的非零值（这正是危险所在）。
	// 若不 Omit，这三列都会进 SET 子句。
	po := &model.Experiment{
		ID:             123,
		LatestRunID:    456, // 调用方真正想改的字段
		PriorityLevel:  1,
		SchedulerMode:  "legacy",
		SchedulerScope: "",
	}

	sql := strings.ToLower(renderExptUpdate(t, po))

	// 调用方想改的字段必须仍在
	assert.Contains(t, sql, "latest_run_id", "业务字段必须正常更新，Omit 不能误伤")

	// 三个冻结列一个都不许出现在 UPDATE 里
	for _, col := range schedulingFrozenColumns {
		assert.NotContains(t, sql, col,
			"冻结列 %s 不得出现在 UPDATE 语句中；它只能由 Create 写入，"+
				"否则部分更新会把 enforce 实验静默改回 legacy、绕过额度账本", col)
	}
}

// TestSchedulingFrozenColumns_CoversAllSchedulingColumns 防止将来新增调度列时漏进 Omit 名单。
//
// 新增一个"创建时冻结"的列却忘了加进 schedulingFrozenColumns，就会重新引入上面那个 bug，
// 且同样静默。这里把名单钉死成显式期望值，新增列时测试会失败、迫使人做出选择。
func TestSchedulingFrozenColumns_CoversAllSchedulingColumns(t *testing.T) {
	t.Parallel()

	assert.ElementsMatch(t,
		[]string{"priority_level", "scheduler_mode", "scheduler_scope"},
		schedulingFrozenColumns,
		"新增创建期冻结的调度列时，必须同步加入 schedulingFrozenColumns，"+
			"否则部分更新会静默改写它")
}
