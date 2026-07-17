// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// TestExptFilterConvertor_ConvertFilters_EvalSetVersionID 验证 §4 新补的 EvalSetVersionID case
// (之前落 default 被静默丢弃)。
func TestExptFilterConvertor_ConvertFilters_EvalSetVersionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	conv := NewExptFilterConvertor(svcmocks.NewMockIEvalTargetService(ctrl))

	t.Run("Include (In)", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field:    &domain_expt.FilterField{FieldType: domain_expt.FieldType_EvalSetVersionID},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "110,220",
			},
		})
		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{110, 220}, got.Includes.EvalSetVersionIDs)
		assert.Nil(t, got.Excludes.EvalSetVersionIDs)
	})

	t.Run("Exclude (NotIn)", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field:    &domain_expt.FilterField{FieldType: domain_expt.FieldType_EvalSetVersionID},
				Operator: domain_expt.FilterOperatorType_NotIn,
				Value:    "330",
			},
		})
		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{330}, got.Excludes.EvalSetVersionIDs)
	})

	t.Run("空值跳过", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field:    &domain_expt.FilterField{FieldType: domain_expt.FieldType_EvalSetVersionID},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "",
			},
		})
		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Nil(t, got.Includes.EvalSetVersionIDs)
	})
}
