// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// renderExptConds 用 dry-run gorm 把 toConditions 产出的闭包渲染成最终 SQL，断言倒排子查询形态。
func renderExptConds(t *testing.T, conds []func(tx *gorm.DB) *gorm.DB) string {
	t.Helper()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()
	gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	tx := gormDB.Model(&model.Experiment{}).Where("space_id = ?", int64(100))
	for _, cond := range conds {
		tx = cond(tx)
	}
	tx = tx.Find(&[]model.Experiment{})
	return tx.Statement.SQL.String()
}

// TestExptDAO_toConditions_EvalSetID_Inverted 验证 §4: EvalSetID Include 走「列 OR 倒排子查询」。
func TestExptDAO_toConditions_EvalSetID_Inverted(t *testing.T) {
	dao := &exptDAOImpl{}

	conds, ok := dao.toConditions(&entity.ExptListFilter{
		Includes: &entity.ExptFilterFields{EvalSetIDs: []int64{10, 20}},
		Excludes: &entity.ExptFilterFields{},
	}, nil, 100)
	assert.True(t, ok)
	sql := renderExptConds(t, conds)

	// Include: eval_set_id IN OR id IN (子查询 expt_item_ref)
	assert.Contains(t, sql, "eval_set_id IN")
	assert.Contains(t, sql, "FROM `expt_item_ref`")
	assert.Contains(t, sql, "OR id IN", "include 应是列匹配 OR 倒排子查询, got: %s", sql)
}

// TestExptDAO_toConditions_EvalSetVersionID_InvertedExclude 验证 EvalSetVersionID Exclude 走「列 NOT IN AND 倒排 NOT IN」。
func TestExptDAO_toConditions_EvalSetVersionID_InvertedExclude(t *testing.T) {
	dao := &exptDAOImpl{}

	conds, ok := dao.toConditions(&entity.ExptListFilter{
		Includes: &entity.ExptFilterFields{},
		Excludes: &entity.ExptFilterFields{EvalSetVersionIDs: []int64{330}},
	}, nil, 100)
	assert.True(t, ok)
	sql := renderExptConds(t, conds)

	assert.Contains(t, sql, "eval_set_version_id NOT IN")
	assert.Contains(t, sql, "FROM `expt_item_ref`")
	assert.Contains(t, sql, "AND id NOT IN", "exclude 应是列不匹配 AND 倒排不匹配, got: %s", sql)
}

// TestExptDAO_toConditions_EvalSetSourceType 验证顶层 EvalSetSourceTypes (与 FuzzyName 同级) → eval_set_source_type IN。
func TestExptDAO_toConditions_EvalSetSourceType(t *testing.T) {
	dao := &exptDAOImpl{}

	conds, ok := dao.toConditions(&entity.ExptListFilter{
		EvalSetSourceTypes: []int64{1},
		Includes:           &entity.ExptFilterFields{},
		Excludes:           &entity.ExptFilterFields{},
	}, nil, 100)
	assert.True(t, ok)
	sql := renderExptConds(t, conds)
	assert.Contains(t, sql, "eval_set_source_type IN")
}
