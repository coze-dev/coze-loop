// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"testing"

	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/assert"
)

func TestConvertEvaluatorFilterOptionDTO2DO(t *testing.T) {
	t.Parallel()

	// 测试空值
	result := ConvertEvaluatorFilterOptionDTO2DO(nil)
	assert.Nil(t, result)

	// 测试只有搜索关键词的情况
	searchKeyword := "test keyword"
	dto := &evaluatordto.EvaluatorFilterOption{
		SearchKeyword: &searchKeyword,
	}
	result = ConvertEvaluatorFilterOptionDTO2DO(dto)
	assert.NotNil(t, result)
	assert.NotNil(t, result.SearchKeyword)
	assert.Equal(t, searchKeyword, *result.SearchKeyword)
	assert.Nil(t, result.Filters)

	// 测试只有筛选条件的情况
	filterCondition := &evaluatordto.EvaluatorFilterCondition{
		TagKey:   evaluatordto.EvaluatorTagKeyCategory,
		Operator: evaluatordto.EvaluatorFilterOperatorTypeEqual,
		Value:    "LLM",
	}
	logicOp := evaluatordto.EvaluatorFilterLogicOpAnd
	filters := &evaluatordto.EvaluatorFilters{
		FilterConditions: []*evaluatordto.EvaluatorFilterCondition{filterCondition},
		LogicOp:          &logicOp,
	}
	dto = &evaluatordto.EvaluatorFilterOption{
		Filters: filters,
	}
	result = ConvertEvaluatorFilterOptionDTO2DO(dto)
	assert.NotNil(t, result)
	assert.Nil(t, result.SearchKeyword)
	assert.NotNil(t, result.Filters)
	assert.Len(t, result.Filters.FilterConditions, 1)
	assert.Equal(t, evaluatordo.EvaluatorTagKey_Category, result.Filters.FilterConditions[0].TagKey)
	assert.Equal(t, evaluatordo.EvaluatorFilterOperatorType_Equal, result.Filters.FilterConditions[0].Operator)
	assert.Equal(t, "LLM", result.Filters.FilterConditions[0].Value)
	assert.NotNil(t, result.Filters.LogicOp)
	assert.Equal(t, evaluatordo.FilterLogicOp_And, *result.Filters.LogicOp)

	// 测试完整的情况
	dto = &evaluatordto.EvaluatorFilterOption{
		SearchKeyword: &searchKeyword,
		Filters:       filters,
	}
	result = ConvertEvaluatorFilterOptionDTO2DO(dto)
	assert.NotNil(t, result)
	assert.NotNil(t, result.SearchKeyword)
	assert.Equal(t, searchKeyword, *result.SearchKeyword)
	assert.NotNil(t, result.Filters)
	assert.Len(t, result.Filters.FilterConditions, 1)
}

func TestConvertEvaluatorTagKeyDTO2DO(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		dtoKey   evaluatordto.EvaluatorTagKey
		expected evaluatordo.EvaluatorTagKey
	}{
		{evaluatordto.EvaluatorTagKeyCategory, evaluatordo.EvaluatorTagKey_Category},
		{evaluatordto.EvaluatorTagKeyTargetType, evaluatordo.EvaluatorTagKey_TargetType},
		{evaluatordto.EvaluatorTagKeyObjective, evaluatordo.EvaluatorTagKey_Objective},
		{evaluatordto.EvaluatorTagKeyBusinessScenario, evaluatordo.EvaluatorTagKey_BusinessScenario},
		{evaluatordto.EvaluatorTagKeyBoxType, evaluatordo.EvaluatorTagKey_BoxType},
		{evaluatordto.EvaluatorTagKeyName, evaluatordo.EvaluatorTagKey_Name},
	}

	for _, tc := range testCases {
		result := ConvertEvaluatorTagKeyDTO2DO(tc.dtoKey)
		assert.Equal(t, tc.expected, result)
	}
}

func TestConvertFilterLogicOpDTO2DO(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		dtoOp    evaluatordto.EvaluatorFilterLogicOp
		expected evaluatordo.FilterLogicOp
	}{
		{evaluatordto.EvaluatorFilterLogicOpAnd, evaluatordo.FilterLogicOp_And},
		{evaluatordto.EvaluatorFilterLogicOpOr, evaluatordo.FilterLogicOp_Or},
		{evaluatordto.EvaluatorFilterLogicOpUnknown, evaluatordo.FilterLogicOp_Unknown},
		{"", evaluatordo.FilterLogicOp_Unknown},
	}

	for _, tc := range testCases {
		result := ConvertFilterLogicOpDTO2DO(tc.dtoOp)
		assert.Equal(t, tc.expected, result)
	}
}

func TestConvertEvaluatorFilterOperatorTypeDTO2DO(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		dtoOp    evaluatordto.EvaluatorFilterOperatorType
		expected evaluatordo.EvaluatorFilterOperatorType
	}{
		{evaluatordto.EvaluatorFilterOperatorTypeEqual, evaluatordo.EvaluatorFilterOperatorType_Equal},
		{evaluatordto.EvaluatorFilterOperatorTypeNotEqual, evaluatordo.EvaluatorFilterOperatorType_NotEqual},
		{evaluatordto.EvaluatorFilterOperatorTypeIn, evaluatordo.EvaluatorFilterOperatorType_In},
		{evaluatordto.EvaluatorFilterOperatorTypeNotIn, evaluatordo.EvaluatorFilterOperatorType_NotIn},
		{evaluatordto.EvaluatorFilterOperatorTypeLike, evaluatordo.EvaluatorFilterOperatorType_Like},
		{evaluatordto.EvaluatorFilterOperatorTypeIsNull, evaluatordo.EvaluatorFilterOperatorType_IsNull},
		{evaluatordto.EvaluatorFilterOperatorTypeIsNotNull, evaluatordo.EvaluatorFilterOperatorType_IsNotNull},
		{evaluatordto.EvaluatorFilterOperatorTypeUnknown, evaluatordo.EvaluatorFilterOperatorType_Unknown},
		{"", evaluatordo.EvaluatorFilterOperatorType_Unknown},
	}

	for _, tc := range testCases {
		result := ConvertEvaluatorFilterOperatorTypeDTO2DO(tc.dtoOp)
		assert.Equal(t, tc.expected, result)
	}
}

func TestConvertEvaluatorFilterConditionDTO2DO(t *testing.T) {
	t.Parallel()

	// 测试空值
	result := ConvertEvaluatorFilterConditionDTO2DO(nil)
	assert.Nil(t, result)

	// 测试正常情况
	dto := &evaluatordto.EvaluatorFilterCondition{
		TagKey:   evaluatordto.EvaluatorTagKeyCategory,
		Operator: evaluatordto.EvaluatorFilterOperatorTypeEqual,
		Value:    "LLM",
	}
	result = ConvertEvaluatorFilterConditionDTO2DO(dto)
	assert.NotNil(t, result)
	assert.Equal(t, evaluatordo.EvaluatorTagKey_Category, result.TagKey)
	assert.Equal(t, evaluatordo.EvaluatorFilterOperatorType_Equal, result.Operator)
	assert.Equal(t, "LLM", result.Value)
}

func TestConvertEvaluatorFiltersDTO2DO(t *testing.T) {
	t.Parallel()

	// 测试空值
	result := ConvertEvaluatorFiltersDTO2DO(nil)
	assert.Nil(t, result)

	// 测试只有逻辑操作符的情况
	logicOp := evaluatordto.EvaluatorFilterLogicOpOr
	dto := &evaluatordto.EvaluatorFilters{
		LogicOp: &logicOp,
	}
	result = ConvertEvaluatorFiltersDTO2DO(dto)
	assert.NotNil(t, result)
	assert.Empty(t, result.FilterConditions)
	assert.NotNil(t, result.LogicOp)
	assert.Equal(t, evaluatordo.FilterLogicOp_Or, *result.LogicOp)

	// 测试只有筛选条件的情况
	filterCondition := &evaluatordto.EvaluatorFilterCondition{
		TagKey:   evaluatordto.EvaluatorTagKeyCategory,
		Operator: evaluatordto.EvaluatorFilterOperatorTypeEqual,
		Value:    "LLM",
	}
	dto = &evaluatordto.EvaluatorFilters{
		FilterConditions: []*evaluatordto.EvaluatorFilterCondition{filterCondition},
	}
	result = ConvertEvaluatorFiltersDTO2DO(dto)
	assert.NotNil(t, result)
	assert.Len(t, result.FilterConditions, 1)
	assert.Nil(t, result.LogicOp)

	// 测试完整的情况
	dto = &evaluatordto.EvaluatorFilters{
		FilterConditions: []*evaluatordto.EvaluatorFilterCondition{filterCondition},
		LogicOp:          &logicOp,
	}
	result = ConvertEvaluatorFiltersDTO2DO(dto)
	assert.NotNil(t, result)
	assert.Len(t, result.FilterConditions, 1)
	assert.NotNil(t, result.LogicOp)
	assert.Equal(t, evaluatordo.FilterLogicOp_Or, *result.LogicOp)
}
