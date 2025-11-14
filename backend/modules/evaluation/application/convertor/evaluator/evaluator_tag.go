// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ConvertEvaluatorTagKeyDTO2DO 将DTO的EvaluatorTagKey转换为DO的EvaluatorTagKey
func ConvertEvaluatorTagKeyDTO2DO(dtoKey evaluatordto.EvaluatorTagKey) evaluatordo.EvaluatorTagKey {
	switch dtoKey {
	case evaluatordto.EvaluatorTagKeyCategory:
		return evaluatordo.EvaluatorTagKey_Category
	case evaluatordto.EvaluatorTagKeyTargetType:
		return evaluatordo.EvaluatorTagKey_TargetType
	case evaluatordto.EvaluatorTagKeyObjective:
		return evaluatordo.EvaluatorTagKey_Objective
	case evaluatordto.EvaluatorTagKeyBusinessScenario:
		return evaluatordo.EvaluatorTagKey_BusinessScenario
	case evaluatordto.EvaluatorTagKeyName:
		return evaluatordo.EvaluatorTagKey_Name
	default:
		return evaluatordo.EvaluatorTagKey(dtoKey)
	}
}

// ConvertFilterLogicOpDTO2DO 将DTO的EvaluatorFilterLogicOp转换为DO的FilterLogicOp
func ConvertFilterLogicOpDTO2DO(dtoOp evaluatordto.EvaluatorFilterLogicOp) evaluatordo.FilterLogicOp {
	switch dtoOp {
	case evaluatordto.EvaluatorFilterLogicOpAnd:
		return evaluatordo.FilterLogicOp_And
	case evaluatordto.EvaluatorFilterLogicOpOr:
		return evaluatordo.FilterLogicOp_Or
	default:
		return evaluatordo.FilterLogicOp_Unknown
	}
}

// ConvertEvaluatorFilterOperatorTypeDTO2DO 将DTO的EvaluatorFilterOperatorType转换为DO的EvaluatorFilterOperatorType
func ConvertEvaluatorFilterOperatorTypeDTO2DO(dtoOp evaluatordto.EvaluatorFilterOperatorType) evaluatordo.EvaluatorFilterOperatorType {
	switch dtoOp {
	case evaluatordto.EvaluatorFilterOperatorTypeEqual:
		return evaluatordo.EvaluatorFilterOperatorType_Equal
	case evaluatordto.EvaluatorFilterOperatorTypeNotEqual:
		return evaluatordo.EvaluatorFilterOperatorType_NotEqual
	case evaluatordto.EvaluatorFilterOperatorTypeIn:
		return evaluatordo.EvaluatorFilterOperatorType_In
	case evaluatordto.EvaluatorFilterOperatorTypeNotIn:
		return evaluatordo.EvaluatorFilterOperatorType_NotIn
	case evaluatordto.EvaluatorFilterOperatorTypeLike:
		return evaluatordo.EvaluatorFilterOperatorType_Like
	case evaluatordto.EvaluatorFilterOperatorTypeIsNull:
		return evaluatordo.EvaluatorFilterOperatorType_IsNull
	case evaluatordto.EvaluatorFilterOperatorTypeIsNotNull:
		return evaluatordo.EvaluatorFilterOperatorType_IsNotNull
	default:
		return evaluatordo.EvaluatorFilterOperatorType_Unknown
	}
}

// ConvertEvaluatorFilterConditionDTO2DO 将DTO的EvaluatorFilterCondition转换为DO的EvaluatorFilterCondition
func ConvertEvaluatorFilterConditionDTO2DO(dto *evaluatordto.EvaluatorFilterCondition) *evaluatordo.EvaluatorFilterCondition {
	if dto == nil {
		return nil
	}

	return &evaluatordo.EvaluatorFilterCondition{
		TagKey:   ConvertEvaluatorTagKeyDTO2DO(dto.GetTagKey()),
		Operator: ConvertEvaluatorFilterOperatorTypeDTO2DO(dto.GetOperator()),
		Value:    dto.GetValue(),
	}
}

// ConvertEvaluatorFiltersDTO2DO 将DTO的EvaluatorFilters转换为DO的EvaluatorFilters
func ConvertEvaluatorFiltersDTO2DO(dto *evaluatordto.EvaluatorFilters) *evaluatordo.EvaluatorFilters {
	if dto == nil {
		return nil
	}

	// 转换筛选条件列表
	var filterConditions []*evaluatordo.EvaluatorFilterCondition
	if dto.GetFilterConditions() != nil {
		filterConditions = make([]*evaluatordo.EvaluatorFilterCondition, 0, len(dto.GetFilterConditions()))
		for _, condition := range dto.GetFilterConditions() {
			if convertedCondition := ConvertEvaluatorFilterConditionDTO2DO(condition); convertedCondition != nil {
				filterConditions = append(filterConditions, convertedCondition)
			}
		}
	}

	// 转换逻辑操作符
	var logicOp *evaluatordo.FilterLogicOp
	if dto.GetLogicOp() != "" {
		convertedLogicOp := ConvertFilterLogicOpDTO2DO(dto.GetLogicOp())
		logicOp = &convertedLogicOp
	}

	// 递归转换子筛选组
	var subFilters []*evaluatordo.EvaluatorFilters
	// 通过接口断言兼容存在 GetSubFilters 方法的生成版本
	type hasSubFilters interface{ GetSubFilters() []*evaluatordto.EvaluatorFilters }
	if sfProvider, ok := any(dto).(hasSubFilters); ok {
		if sf := sfProvider.GetSubFilters(); sf != nil {
			subFilters = make([]*evaluatordo.EvaluatorFilters, 0, len(sf))
			for _, sub := range sf {
				if converted := ConvertEvaluatorFiltersDTO2DO(sub); converted != nil {
					subFilters = append(subFilters, converted)
				}
			}
		}
	}

	return &evaluatordo.EvaluatorFilters{
		FilterConditions: filterConditions,
		LogicOp:          logicOp,
		SubFilters:       subFilters,
	}
}

// ConvertEvaluatorFilterOptionDTO2DO 将DTO的EvaluatorFilterOption转换为DO的EvaluatorFilterOption
func ConvertEvaluatorFilterOptionDTO2DO(dto *evaluatordto.EvaluatorFilterOption) *evaluatordo.EvaluatorFilterOption {
	if dto == nil {
		return nil
	}

	// 转换搜索关键词
	var searchKeyword *string
	if dto.GetSearchKeyword() != "" {
		keyword := dto.GetSearchKeyword()
		searchKeyword = &keyword
	}

	// 转换筛选条件
	filters := ConvertEvaluatorFiltersDTO2DO(dto.GetFilters())

	return &evaluatordo.EvaluatorFilterOption{
		SearchKeyword: searchKeyword,
		Filters:       filters,
	}
}
