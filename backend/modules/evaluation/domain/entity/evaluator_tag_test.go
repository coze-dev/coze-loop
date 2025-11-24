// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluatorFilterOption(t *testing.T) {
	t.Parallel()

	// 测试创建空的筛选选项
	option := NewEvaluatorFilterOption()
	assert.NotNil(t, option)
	assert.Nil(t, option.SearchKeyword)
	assert.Nil(t, option.Filters)

	// 测试设置搜索关键词
	keyword := "test keyword"
	option = NewEvaluatorFilterOption().WithSearchKeyword(keyword)
	assert.NotNil(t, option.SearchKeyword)
	assert.Equal(t, keyword, *option.SearchKeyword)

	// 测试设置筛选条件
	filters := NewEvaluatorFilters()
	option = NewEvaluatorFilterOption().WithFilters(filters)
	assert.NotNil(t, option.Filters)
	assert.Equal(t, filters, option.Filters)
}

func TestEvaluatorFilters(t *testing.T) {
	t.Parallel()

	// 测试创建筛选器
	filters := NewEvaluatorFilters()
	assert.NotNil(t, filters)
	assert.NotNil(t, filters.LogicOp)
	assert.Equal(t, FilterLogicOp_And, *filters.LogicOp)
	assert.Empty(t, filters.FilterConditions)

	// 测试设置逻辑操作符
	filters = NewEvaluatorFilters().WithLogicOp(FilterLogicOp_Or)
	assert.Equal(t, FilterLogicOp_Or, *filters.LogicOp)

	// 测试添加筛选条件
	condition1 := NewEvaluatorFilterCondition(
		EvaluatorTagKey_Category,
		EvaluatorFilterOperatorType_Equal,
		"LLM",
	)
	condition2 := NewEvaluatorFilterCondition(
		EvaluatorTagKey_TargetType,
		EvaluatorFilterOperatorType_In,
		"Text,Image",
	)

	filters = NewEvaluatorFilters().
		AddCondition(condition1).
		AddCondition(condition2)

	assert.Len(t, filters.FilterConditions, 2)
	assert.Equal(t, condition1, filters.FilterConditions[0])
	assert.Equal(t, condition2, filters.FilterConditions[1])
}

func TestEvaluatorFilterCondition(t *testing.T) {
	t.Parallel()

	// 测试创建筛选条件
	condition := NewEvaluatorFilterCondition(
		EvaluatorTagKey_Category,
		EvaluatorFilterOperatorType_Equal,
		"LLM",
	)

	assert.Equal(t, EvaluatorTagKey_Category, condition.TagKey)
	assert.Equal(t, EvaluatorFilterOperatorType_Equal, condition.Operator)
	assert.Equal(t, "LLM", condition.Value)
}

func TestFilterLogicOp(t *testing.T) {
	t.Parallel()

	// 测试字符串表示
	assert.Equal(t, "AND", FilterLogicOp_And.String())
	assert.Equal(t, "OR", FilterLogicOp_Or.String())
	assert.Equal(t, "UNKNOWN", FilterLogicOp_Unknown.String())

	// 测试有效性检查
	assert.True(t, FilterLogicOp_And.IsValid())
	assert.True(t, FilterLogicOp_Or.IsValid())
	assert.False(t, FilterLogicOp_Unknown.IsValid())
}

func TestEvaluatorFilterOperatorType(t *testing.T) {
	t.Parallel()

	// 测试字符串表示
	assert.Equal(t, "EQUAL", EvaluatorFilterOperatorType_Equal.String())
	assert.Equal(t, "NOT_EQUAL", EvaluatorFilterOperatorType_NotEqual.String())
	assert.Equal(t, "IN", EvaluatorFilterOperatorType_In.String())
	assert.Equal(t, "NOT_IN", EvaluatorFilterOperatorType_NotIn.String())
	assert.Equal(t, "LIKE", EvaluatorFilterOperatorType_Like.String())
	assert.Equal(t, "IS_NULL", EvaluatorFilterOperatorType_IsNull.String())
	assert.Equal(t, "IS_NOT_NULL", EvaluatorFilterOperatorType_IsNotNull.String())
	assert.Equal(t, "UNKNOWN", EvaluatorFilterOperatorType_Unknown.String())

	// 测试有效性检查
	assert.True(t, EvaluatorFilterOperatorType_Equal.IsValid())
	assert.True(t, EvaluatorFilterOperatorType_NotEqual.IsValid())
	assert.True(t, EvaluatorFilterOperatorType_In.IsValid())
	assert.True(t, EvaluatorFilterOperatorType_NotIn.IsValid())
	assert.True(t, EvaluatorFilterOperatorType_Like.IsValid())
	assert.True(t, EvaluatorFilterOperatorType_IsNull.IsValid())
	assert.True(t, EvaluatorFilterOperatorType_IsNotNull.IsValid())
	assert.False(t, EvaluatorFilterOperatorType_Unknown.IsValid())
}

func TestComplexFilterScenario(t *testing.T) {
	t.Parallel()

	// 测试复杂筛选场景：搜索关键词 + 多个筛选条件
	keyword := "AI evaluation"
	option := NewEvaluatorFilterOption().
		WithSearchKeyword(keyword).
		WithFilters(
			NewEvaluatorFilters().
				WithLogicOp(FilterLogicOp_And).
				AddCondition(NewEvaluatorFilterCondition(
					EvaluatorTagKey_Category,
					EvaluatorFilterOperatorType_Equal,
					"LLM",
				)).
				AddCondition(NewEvaluatorFilterCondition(
					EvaluatorTagKey_TargetType,
					EvaluatorFilterOperatorType_In,
					"Text,Image",
				)).
				AddCondition(NewEvaluatorFilterCondition(
					EvaluatorTagKey_Objective,
					EvaluatorFilterOperatorType_Like,
					"Quality",
				)),
		)

	// 验证结果
	assert.NotNil(t, option)
	assert.NotNil(t, option.SearchKeyword)
	assert.Equal(t, keyword, *option.SearchKeyword)
	assert.NotNil(t, option.Filters)
	assert.Equal(t, FilterLogicOp_And, *option.Filters.LogicOp)
	assert.Len(t, option.Filters.FilterConditions, 3)

	// 验证各个条件
	conditions := option.Filters.FilterConditions
	assert.Equal(t, EvaluatorTagKey_Category, conditions[0].TagKey)
	assert.Equal(t, EvaluatorFilterOperatorType_Equal, conditions[0].Operator)
	assert.Equal(t, "LLM", conditions[0].Value)

	assert.Equal(t, EvaluatorTagKey_TargetType, conditions[1].TagKey)
	assert.Equal(t, EvaluatorFilterOperatorType_In, conditions[1].Operator)
	assert.Equal(t, "Text,Image", conditions[1].Value)

	assert.Equal(t, EvaluatorTagKey_Objective, conditions[2].TagKey)
	assert.Equal(t, EvaluatorFilterOperatorType_Like, conditions[2].Operator)
	assert.Equal(t, "Quality", conditions[2].Value)
}
