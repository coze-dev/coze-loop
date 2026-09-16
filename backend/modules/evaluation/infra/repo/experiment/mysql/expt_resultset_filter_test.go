// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// TestExptDAO_toConditions_OnlyResultSetEval 验证结果集评测筛选 (仅返回无评测对象的实验, target_id = 0):
// true → 落 target_id = 0, 且豁免「未传 source_types 默认排除 MultiSet」(结果集评测实验本身即 MultiSetConfig,
// 只传该开关不传 source_types 时若仍套默认排除会滤成空); 缺省 → 维持默认排除、不加 target_id 条件。
func TestExptDAO_toConditions_OnlyResultSetEval(t *testing.T) {
	dao := &exptDAOImpl{}

	// true: target_id = 0, 豁免默认排除
	conds, ok := dao.toConditions(&entity.ExptListFilter{
		OnlyResultSetEval: true,
		Includes:          &entity.ExptFilterFields{},
		Excludes:          &entity.ExptFilterFields{},
	}, nil, 100)
	assert.True(t, ok)
	sql := renderExptConds(t, conds)
	assert.Contains(t, sql, "target_id = ?", "应落 target_id = 0 条件, got: %s", sql)
	assert.NotContains(t, sql, "eval_set_source_type <> ?", "应豁免默认排除 MultiSet, got: %s", sql)
	assert.NotContains(t, sql, "eval_set_source_type IS NULL", "应豁免默认排除 MultiSet, got: %s", sql)

	// 缺省: 维持默认排除, 无 target_id 条件
	conds2, ok := dao.toConditions(&entity.ExptListFilter{
		Includes: &entity.ExptFilterFields{},
		Excludes: &entity.ExptFilterFields{},
	}, nil, 100)
	assert.True(t, ok)
	sql2 := renderExptConds(t, conds2)
	assert.Contains(t, sql2, "eval_set_source_type <> ?", "缺省应维持默认排除 MultiSet, got: %s", sql2)
	assert.NotContains(t, sql2, "target_id = ?", "缺省不应有 target_id 条件, got: %s", sql2)

	// true + 显式 source_types: 两者 AND (调用方显式意图优先)
	conds3, ok := dao.toConditions(&entity.ExptListFilter{
		OnlyResultSetEval: true,
		EvalSetSourceTypes: []int64{int64(entity.ExptEvalSetSourceType_MultiSetConfig)},
		Includes:           &entity.ExptFilterFields{},
		Excludes:           &entity.ExptFilterFields{},
	}, nil, 100)
	assert.True(t, ok)
	sql3 := renderExptConds(t, conds3)
	assert.Contains(t, sql3, "eval_set_source_type IN", "显式白名单应走 IN, got: %s", sql3)
	assert.Contains(t, sql3, "target_id = ?", "应同时落 target_id = 0, got: %s", sql3)
}
