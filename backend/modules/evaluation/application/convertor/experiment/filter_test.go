// Copyright 2026
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package experiment

import (
	"context"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	eval_target "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestExptFilterConvertor_Convert_NilOption(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	conv := NewExptFilterConvertor(mockEvalTargetSvc)

	got, err := conv.Convert(context.Background(), nil, 100)
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestExptFilterConvertor_ConvertFilters_BasicFieldsAndDefaultType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	conv := NewExptFilterConvertor(mockEvalTargetSvc)

	filters := &domain_expt.Filters{}
	filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
	filters.SetFilterConditions([]*domain_expt.FilterCondition{
		{
			Field: &domain_expt.FilterField{
				FieldType: domain_expt.FieldType_CreatorBy,
			},
			Operator: domain_expt.FilterOperatorType_Equal,
			Value:    "user1",
		},
		{
			Field: &domain_expt.FilterField{
				FieldType: domain_expt.FieldType_ExptStatus,
			},
			Operator: domain_expt.FilterOperatorType_In,
			Value:    "1,2",
		},
		{
			Field: &domain_expt.FilterField{
				FieldType: domain_expt.FieldType_SourceID,
			},
			Operator: domain_expt.FilterOperatorType_In,
			Value:    "s1,s2",
		},
	})

	got, err := conv.ConvertFilters(context.Background(), filters, 100)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, []string{"user1"}, got.Includes.CreatedBy)
	assert.ElementsMatch(t, []int64{1, 2}, got.Includes.Status)
	assert.ElementsMatch(t, []string{"s1", "s2"}, got.Includes.SourceID)
	// 未显式传 ExptType，应该默认补上 Offline
	assert.ElementsMatch(t, []int64{int64(domain_expt.ExptType_Offline)}, got.Includes.ExptType)
}

func TestExptFilterConvertor_ConvertFilters_InvalidLogicOp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	conv := NewExptFilterConvertor(mockEvalTargetSvc)

	filters := &domain_expt.Filters{}
	filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_Or))

	got, err := conv.ConvertFilters(context.Background(), filters, 100)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestExptFilterConvertor_ConvertFilters_SourceTarget_SingleNoTargets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	conv := NewExptFilterConvertor(mockEvalTargetSvc)

	// 当 SourceTargetIds 只有一个且查不到目标时，应写入 -1 作为兜底
	filters := &domain_expt.Filters{}
	filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
	filters.SetFilterConditions([]*domain_expt.FilterCondition{
		{
			Field: &domain_expt.FilterField{
				FieldType: domain_expt.FieldType_SourceTarget,
			},
			Operator: domain_expt.FilterOperatorType_In,
			SourceTarget: &domain_expt.SourceTarget{
				EvalTargetType:  eval_target.EvalTargetTypePtr(eval_target.EvalTargetType_CozeBot),
				SourceTargetIds: []string{"123"},
			},
		},
	})

	mockEvalTargetSvc.EXPECT().
		BatchGetEvalTargetBySource(gomock.Any(), &entity.BatchGetEvalTargetBySourceParam{
			SpaceID:        100,
			SourceTargetID: []string{"123"},
			TargetType:     entity.EvalTargetType(eval_target.EvalTargetType_CozeBot),
		}).
		Return([]*entity.EvalTarget{}, nil)

	got, err := conv.ConvertFilters(context.Background(), filters, 100)
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Contains(t, got.Includes.TargetIDs, int64(-1))
}

func TestParseIntListAndStringList(t *testing.T) {
	ints, err := parseIntList("1,2,3")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int64{1, 2, 3}, ints)

	_, err = parseIntList("a,b")
	assert.Error(t, err)

	strs := parseStringList("a,b,c")
	assert.ElementsMatch(t, []string{"a", "b", "c"}, strs)
}

func TestParseOperator(t *testing.T) {
	cases := []struct {
		op   domain_expt.FilterOperatorType
		want string
		err  bool
	}{
		{domain_expt.FilterOperatorType_Equal, "=", false},
		{domain_expt.FilterOperatorType_NotEqual, "!=", false},
		{domain_expt.FilterOperatorType_Greater, ">", false},
		{domain_expt.FilterOperatorType_GreaterOrEqual, ">=", false},
		{domain_expt.FilterOperatorType_Less, "<", false},
		{domain_expt.FilterOperatorType_LessOrEqual, "<=", false},
		{domain_expt.FilterOperatorType_In, "IN", false},
		{domain_expt.FilterOperatorType_NotIn, "NOT IN", false},
		{domain_expt.FilterOperatorType(999), "", true},
	}

	for _, c := range cases {
		got, err := parseOperator(c.op)
		if c.err {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, c.want, got)
		}
	}
}

func TestIntersectIgnoreNull(t *testing.T) {
	// s1 为空，返回 s2
	assert.ElementsMatch(t, []int{1, 2}, intersectIgnoreNull([]int{}, []int{1, 2}))
	// s2 为空，返回 s1
	assert.ElementsMatch(t, []int{1, 2}, intersectIgnoreNull([]int{1, 2}, []int{}))
	// 交集
	assert.ElementsMatch(t, []int{2, 3}, intersectIgnoreNull([]int{1, 2, 3}, []int{2, 3, 4}))
}

// TestExptFilterConvertor_ConvertFilters_FieldTypes_75_103 测试 ConvertFilters 中各种字段类型的筛选条件 (75-103行)
func TestExptFilterConvertor_ConvertFilters_FieldTypes_75_103(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	conv := NewExptFilterConvertor(mockEvalTargetSvc)

	t.Run("CreatorBy字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_CreatorBy,
				},
				Operator: domain_expt.FilterOperatorType_Equal,
				Value:    "user1",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Equal(t, []string{"user1"}, got.Includes.CreatedBy)
	})

	t.Run("CreatorBy字段值为空，跳过", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_CreatorBy,
				},
				Operator: domain_expt.FilterOperatorType_Equal,
				Value:    "",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Nil(t, got.Includes.CreatedBy)
	})

	t.Run("UpdatedBy字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_UpdatedBy,
				},
				Operator: domain_expt.FilterOperatorType_Equal,
				Value:    "user2",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Equal(t, []string{"user2"}, got.Includes.UpdatedBy)
	})

	t.Run("ExptStatus字段，包含Processing时添加Draining", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_ExptStatus,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "3", // Processing = 3
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		// 应该包含 Processing (3) 和 Draining (21)
		assert.Contains(t, got.Includes.Status, int64(domain_expt.ExptStatus_Processing))
		assert.Contains(t, got.Includes.Status, int64(domain_expt.ExptStatus_Draining))
	})

	t.Run("ExptStatus字段，不包含Processing时不添加Draining", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_ExptStatus,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "3", // 其他状态
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Contains(t, got.Includes.Status, int64(3))
		assert.NotContains(t, got.Includes.Status, int64(2)) // 不应该包含Draining
	})

	t.Run("ExptStatus字段，解析错误返回错误", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_ExptStatus,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "invalid",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("EvalSetID字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_EvalSetID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "10,20",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{10, 20}, got.Includes.EvalSetIDs)
	})

	t.Run("EvalSetID字段值为空，跳过", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_EvalSetID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Nil(t, got.Includes.EvalSetIDs)
	})
}

// TestExptFilterConvertor_ConvertFilters_FieldTypes_173_261 测试 ConvertFilters 中更多字段类型和辅助函数 (173-261行)
func TestExptFilterConvertor_ConvertFilters_FieldTypes_173_261(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	conv := NewExptFilterConvertor(mockEvalTargetSvc)

	t.Run("SourceType字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_SourceType,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "1,2",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 2}, got.Includes.SourceType)
	})

	t.Run("SourceID字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_SourceID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "s1,s2",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"s1", "s2"}, got.Includes.SourceID)
	})

	t.Run("SourceID字段值为空，跳过", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_SourceID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Nil(t, got.Includes.SourceID)
	})

	t.Run("ExperimentTemplateID字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_ExperimentTemplateID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "100,200",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{100, 200}, got.Includes.ExptTemplateIDs)
	})

	t.Run("ExperimentTemplateID字段值为空，跳过", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_ExperimentTemplateID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Nil(t, got.Includes.ExptTemplateIDs)
	})

	t.Run("ExperimentTemplateID字段解析错误返回错误", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_ExperimentTemplateID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "invalid",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("intersectIgnoreNull函数测试", func(t *testing.T) {
		// 测试字符串类型
		assert.ElementsMatch(t, []string{"a", "b"}, intersectIgnoreNull([]string{}, []string{"a", "b"}))
		assert.ElementsMatch(t, []string{"a", "b"}, intersectIgnoreNull([]string{"a", "b"}, []string{}))
		assert.ElementsMatch(t, []string{"b"}, intersectIgnoreNull([]string{"a", "b"}, []string{"b", "c"}))

		// 测试int64类型
		assert.ElementsMatch(t, []int64{1, 2}, intersectIgnoreNull([]int64{}, []int64{1, 2}))
		assert.ElementsMatch(t, []int64{1, 2}, intersectIgnoreNull([]int64{1, 2}, []int64{}))
		assert.ElementsMatch(t, []int64{2}, intersectIgnoreNull([]int64{1, 2}, []int64{2, 3}))
	})

	t.Run("parseIntList函数测试", func(t *testing.T) {
		result, err := parseIntList("1,2,3")
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 2, 3}, result)

		result, err = parseIntList("10")
		assert.NoError(t, err)
		assert.Equal(t, []int64{10}, result)

		_, err = parseIntList("invalid")
		assert.Error(t, err)

		_, err = parseIntList("1,invalid,3")
		assert.Error(t, err)
	})

	t.Run("parseStringList函数测试", func(t *testing.T) {
		result := parseStringList("a,b,c")
		assert.ElementsMatch(t, []string{"a", "b", "c"}, result)

		result = parseStringList("single")
		assert.Equal(t, []string{"single"}, result)

		result = parseStringList("")
		assert.Equal(t, []string{""}, result)
	})

	t.Run("parseOperator函数测试", func(t *testing.T) {
		// 这些测试已经在TestParseOperator中覆盖，这里补充一些边界情况
		operator, err := parseOperator(domain_expt.FilterOperatorType_Equal)
		assert.NoError(t, err)
		assert.Equal(t, "=", operator)

		operator, err = parseOperator(domain_expt.FilterOperatorType_NotEqual)
		assert.NoError(t, err)
		assert.Equal(t, "!=", operator)

		_, err = parseOperator(domain_expt.FilterOperatorType(999))
		assert.Error(t, err)
	})
}

// TestExptTemplateFilterConvertor_Convert_527_676 测试 ExptTemplateFilterConvertor 的 Convert 和 ConvertFilters 方法 (527-676行)
func TestExptTemplateFilterConvertor_Convert_527_676(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	conv := NewExptTemplateFilterConvertor(mockEvalTargetSvc)

	t.Run("Convert方法，nil选项返回nil", func(t *testing.T) {
		got, err := conv.Convert(context.Background(), nil, 100)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("Convert方法，带关键词搜索", func(t *testing.T) {
		etf := &domain_expt.ExperimentTemplateFilter{}
		keywordSearch := &domain_expt.KeywordSearch{}
		keywordSearch.SetKeyword(gptr.Of("test"))
		etf.SetKeywordSearch(keywordSearch)
		etf.SetFilters(&domain_expt.Filters{
			LogicOp: domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And),
		})

		got, err := conv.Convert(context.Background(), etf, 100)
		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, "test", got.FuzzyName)
	})

	t.Run("Convert方法，关键词为空，不设置FuzzyName", func(t *testing.T) {
		etf := &domain_expt.ExperimentTemplateFilter{}
		keywordSearch := &domain_expt.KeywordSearch{}
		keywordSearch.SetKeyword(gptr.Of(""))
		etf.SetKeywordSearch(keywordSearch)
		etf.SetFilters(&domain_expt.Filters{
			LogicOp: domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And),
		})

		got, err := conv.Convert(context.Background(), etf, 100)
		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got.FuzzyName)
	})

	t.Run("ConvertFilters方法，nil filters返回空过滤器", func(t *testing.T) {
		got, err := conv.ConvertFilters(context.Background(), nil, 100)
		assert.NoError(t, err)
		assert.NotNil(t, got)
		assert.NotNil(t, got.Includes)
		assert.NotNil(t, got.Excludes)
	})

	t.Run("ConvertFilters方法，无效的逻辑操作符返回错误", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_Or))

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("ConvertFilters方法，CreatorBy字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_CreatorBy,
				},
				Operator: domain_expt.FilterOperatorType_Equal,
				Value:    "user1",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Equal(t, []string{"user1"}, got.Includes.CreatedBy)
	})

	t.Run("ConvertFilters方法，UpdatedBy字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_UpdatedBy,
				},
				Operator: domain_expt.FilterOperatorType_Equal,
				Value:    "user2",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Equal(t, []string{"user2"}, got.Includes.UpdatedBy)
	})

	t.Run("ConvertFilters方法，EvalSetID字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_EvalSetID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "10,20",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{10, 20}, got.Includes.EvalSetIDs)
	})

	t.Run("ConvertFilters方法，TargetID字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TargetID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "30,40",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{30, 40}, got.Includes.TargetIDs)
	})

	t.Run("ConvertFilters方法，EvaluatorID字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_EvaluatorID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "50,60",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{50, 60}, got.Includes.EvaluatorIDs)
	})

	t.Run("ConvertFilters方法，TargetType字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TargetType,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "1,2",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 2}, got.Includes.TargetType)
	})

	t.Run("ConvertFilters方法，SourceTarget字段，单个ID查不到目标时返回-1", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_SourceTarget,
				},
				Operator: domain_expt.FilterOperatorType_In,
				SourceTarget: &domain_expt.SourceTarget{
					EvalTargetType:  eval_target.EvalTargetTypePtr(eval_target.EvalTargetType_CozeBot),
					SourceTargetIds: []string{"source1"},
				},
			},
		})

		mockEvalTargetSvc.EXPECT().
			BatchGetEvalTargetBySource(gomock.Any(), &entity.BatchGetEvalTargetBySourceParam{
				SpaceID:        100,
				SourceTargetID: []string{"source1"},
				TargetType:     entity.EvalTargetTypeCozeBot,
			}).
			Return([]*entity.EvalTarget{}, nil)

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Contains(t, got.Includes.TargetIDs, int64(-1))
	})

	t.Run("ConvertFilters方法，SourceTarget字段，多个ID查不到目标时不返回-1", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_SourceTarget,
				},
				Operator: domain_expt.FilterOperatorType_In,
				SourceTarget: &domain_expt.SourceTarget{
					EvalTargetType:  eval_target.EvalTargetTypePtr(eval_target.EvalTargetType_CozeBot),
					SourceTargetIds: []string{"source1", "source2"},
				},
			},
		})

		mockEvalTargetSvc.EXPECT().
			BatchGetEvalTargetBySource(gomock.Any(), &entity.BatchGetEvalTargetBySourceParam{
				SpaceID:        100,
				SourceTargetID: []string{"source1", "source2"},
				TargetType:     entity.EvalTargetTypeCozeBot,
			}).
			Return([]*entity.EvalTarget{}, nil)

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.NotContains(t, got.Includes.TargetIDs, int64(-1))
	})

	t.Run("ConvertFilters方法，SourceTarget字段，查询成功", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_SourceTarget,
				},
				Operator: domain_expt.FilterOperatorType_In,
				SourceTarget: &domain_expt.SourceTarget{
					EvalTargetType:  eval_target.EvalTargetTypePtr(eval_target.EvalTargetType_CozeBot),
					SourceTargetIds: []string{"source1"},
				},
			},
		})

		mockEvalTargetSvc.EXPECT().
			BatchGetEvalTargetBySource(gomock.Any(), &entity.BatchGetEvalTargetBySourceParam{
				SpaceID:        100,
				SourceTargetID: []string{"source1"},
				TargetType:     entity.EvalTargetTypeCozeBot,
			}).
			Return([]*entity.EvalTarget{
				{ID: 100},
				{ID: 200},
			}, nil)

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{100, 200}, got.Includes.TargetIDs)
	})

	t.Run("ConvertFilters方法，SourceTarget字段查询失败返回错误", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_SourceTarget,
				},
				Operator: domain_expt.FilterOperatorType_In,
				SourceTarget: &domain_expt.SourceTarget{
					EvalTargetType:  eval_target.EvalTargetTypePtr(eval_target.EvalTargetType_CozeBot),
					SourceTargetIds: []string{"source1"},
				},
			},
		})

		mockEvalTargetSvc.EXPECT().
			BatchGetEvalTargetBySource(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("query error"))

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("ConvertFilters方法，ExptType字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_ExptType,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "1,2",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 2}, got.Includes.ExptType)
	})

	t.Run("ConvertFilters方法，不支持的字段类型记录警告", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType(999), // 不支持的字段类型
				},
				Operator: domain_expt.FilterOperatorType_Equal,
				Value:    "test",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.NotNil(t, got)
	})

	t.Run("ConvertFilters方法，NotIn操作符设置到Excludes", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_EvalSetID,
				},
				Operator: domain_expt.FilterOperatorType_NotIn,
				Value:    "10,20",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{10, 20}, got.Excludes.EvalSetIDs)
	})

	t.Run("ConvertFilters方法，NotEqual操作符设置到Excludes", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_CreatorBy,
				},
				Operator: domain_expt.FilterOperatorType_NotEqual,
				Value:    "user1",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Equal(t, []string{"user1"}, got.Excludes.CreatedBy)
	})
}

// TestExptFilterConvertor_ConvertFilters_FieldTypes_110_140 测试 ConvertFilters 中 TargetID, EvaluatorID, TargetType, SourceTarget 字段 (110-140行)
func TestExptFilterConvertor_ConvertFilters_FieldTypes_110_140(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	conv := NewExptFilterConvertor(mockEvalTargetSvc)

	t.Run("TargetID字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TargetID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "10,20",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{10, 20}, got.Includes.TargetIDs)
	})

	t.Run("TargetID字段值为空，跳过", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TargetID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Nil(t, got.Includes.TargetIDs)
	})

	t.Run("EvaluatorID字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_EvaluatorID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "50,60",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{50, 60}, got.Includes.EvaluatorIDs)
	})

	t.Run("EvaluatorID字段值为空，跳过", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_EvaluatorID,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Nil(t, got.Includes.EvaluatorIDs)
	})

	t.Run("TargetType字段", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TargetType,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "1,2",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{1, 2}, got.Includes.TargetType)
	})

	t.Run("TargetType字段值为空，跳过", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TargetType,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Nil(t, got.Includes.TargetType)
	})

	t.Run("SourceTarget字段，SourceTarget为nil，跳过", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_SourceTarget,
				},
				Operator:     domain_expt.FilterOperatorType_In,
				SourceTarget: nil,
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Nil(t, got.Includes.TargetIDs)
	})

	t.Run("SourceTarget字段，SourceTargetIds为空，跳过", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_SourceTarget,
				},
				Operator: domain_expt.FilterOperatorType_In,
				SourceTarget: &domain_expt.SourceTarget{
					EvalTargetType:  eval_target.EvalTargetTypePtr(eval_target.EvalTargetType_CozeBot),
					SourceTargetIds: []string{},
				},
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.Nil(t, got.Includes.TargetIDs)
	})
}

// TestExptFilterConvertor_ConvertFilters_SourceTarget_155_166 测试 SourceTarget 处理后的 targetIDs 构建和 ExptType 字段 (155-166行)
func TestExptFilterConvertor_ConvertFilters_SourceTarget_155_166(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalTargetSvc := svcmocks.NewMockIEvalTargetService(ctrl)
	conv := NewExptFilterConvertor(mockEvalTargetSvc)

	t.Run("SourceTarget查询成功，构建targetIDs", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_SourceTarget,
				},
				Operator: domain_expt.FilterOperatorType_In,
				SourceTarget: &domain_expt.SourceTarget{
					EvalTargetType:  eval_target.EvalTargetTypePtr(eval_target.EvalTargetType_CozeBot),
					SourceTargetIds: []string{"source1", "source2"},
				},
			},
		})

		mockEvalTargetSvc.EXPECT().
			BatchGetEvalTargetBySource(gomock.Any(), &entity.BatchGetEvalTargetBySourceParam{
				SpaceID:        100,
				SourceTargetID: []string{"source1", "source2"},
				TargetType:     entity.EvalTargetTypeCozeBot,
			}).
			Return([]*entity.EvalTarget{
				{ID: 100},
				{ID: 200},
			}, nil)

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{100, 200}, got.Includes.TargetIDs)
	})

	t.Run("ExptType字段，设置setDefaultExptTypeFlag为false", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_ExptType,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "2", // Online，不包含Offline (1)
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []int64{2}, got.Includes.ExptType)
		// 当设置了ExptType时，不应该有默认的Offline类型（除非用户显式指定）
		// 这里用户只指定了Online (2)，所以不应该包含Offline (1)
		assert.NotContains(t, got.Includes.ExptType, int64(domain_expt.ExptType_Offline))
	})

	t.Run("ExptType字段解析错误返回错误", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_ExptType,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "invalid",
			},
		})

		got, err := conv.ConvertFilters(context.Background(), filters, 100)
		assert.Error(t, err)
		assert.Nil(t, got)
	})
}

// TestConvertExptTurnResultFilter 测试 ConvertExptTurnResultFilter 函数
func TestConvertExptTurnResultFilter(t *testing.T) {
	t.Run("nil filters返回空结果", func(t *testing.T) {
		result, err := ConvertExptTurnResultFilter(nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.TrunRunStateFilters)
		assert.Empty(t, result.ScoreFilters)
	})

	t.Run("空filters返回空结果", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{})

		result, err := ConvertExptTurnResultFilter(filters)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.TrunRunStateFilters)
		assert.Empty(t, result.ScoreFilters)
	})

	t.Run("无效的逻辑操作符返回错误", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_Or))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TurnRunState,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "1",
			},
		})

		result, err := ConvertExptTurnResultFilter(filters)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid logic op")
	})

	t.Run("nil filterCondition跳过", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			nil,
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TurnRunState,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "1",
			},
		})

		result, err := ConvertExptTurnResultFilter(filters)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.TrunRunStateFilters, 1)
	})

	t.Run("TurnRunState字段，成功转换", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TurnRunState,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "1,2",
			},
		})

		result, err := ConvertExptTurnResultFilter(filters)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.TrunRunStateFilters, 1)
		assert.Equal(t, "IN", result.TrunRunStateFilters[0].Operator)
		assert.Len(t, result.TrunRunStateFilters[0].Status, 2)
		assert.Contains(t, result.TrunRunStateFilters[0].Status, entity.TurnRunState(1))
		assert.Contains(t, result.TrunRunStateFilters[0].Status, entity.TurnRunState(2))
	})

	t.Run("TurnRunState字段，NotIn操作符", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TurnRunState,
				},
				Operator: domain_expt.FilterOperatorType_NotIn,
				Value:    "1",
			},
		})

		result, err := ConvertExptTurnResultFilter(filters)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.TrunRunStateFilters, 1)
		assert.Equal(t, "NOT IN", result.TrunRunStateFilters[0].Operator)
	})

	t.Run("TurnRunState字段，无效操作符返回错误", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TurnRunState,
				},
				Operator: domain_expt.FilterOperatorType_Equal, // 无效操作符
				Value:    "1",
			},
		})

		result, err := ConvertExptTurnResultFilter(filters)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid operator")
	})

	t.Run("TurnRunState字段，parseTurnRunState错误返回错误", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TurnRunState,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "invalid",
			},
		})

		result, err := ConvertExptTurnResultFilter(filters)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid turn run state")
	})

	t.Run("EvaluatorScore字段，成功转换", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		field := &domain_expt.FilterField{
			FieldType: domain_expt.FieldType_EvaluatorScore,
		}
		field.SetFieldKey(gptr.Of("101"))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field:    field,
				Operator: domain_expt.FilterOperatorType_Greater,
				Value:    "0.8",
			},
		})

		result, err := ConvertExptTurnResultFilter(filters)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.ScoreFilters, 1)
		assert.Equal(t, 0.8, result.ScoreFilters[0].Score)
		assert.Equal(t, ">", result.ScoreFilters[0].Operator)
		assert.Equal(t, int64(101), result.ScoreFilters[0].EvaluatorVersionID)
	})

	t.Run("EvaluatorScore字段，解析score错误返回错误", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		field := &domain_expt.FilterField{
			FieldType: domain_expt.FieldType_EvaluatorScore,
		}
		field.SetFieldKey(gptr.Of("101"))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field:    field,
				Operator: domain_expt.FilterOperatorType_Greater,
				Value:    "invalid",
			},
		})

		result, err := ConvertExptTurnResultFilter(filters)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("EvaluatorScore字段，解析evaluatorVersionID错误返回错误", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		field := &domain_expt.FilterField{
			FieldType: domain_expt.FieldType_EvaluatorScore,
		}
		field.SetFieldKey(gptr.Of("invalid"))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field:    field,
				Operator: domain_expt.FilterOperatorType_Greater,
				Value:    "0.8",
			},
		})

		result, err := ConvertExptTurnResultFilter(filters)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("不支持的字段类型返回错误", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_CreatorBy, // 不支持的字段类型
				},
				Operator: domain_expt.FilterOperatorType_Equal,
				Value:    "user1",
			},
		})

		result, err := ConvertExptTurnResultFilter(filters)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid field type")
	})

	t.Run("多个条件，混合TurnRunState和EvaluatorScore", func(t *testing.T) {
		filters := &domain_expt.Filters{}
		filters.SetLogicOp(domain_expt.FilterLogicOpPtr(domain_expt.FilterLogicOp_And))
		scoreField := &domain_expt.FilterField{
			FieldType: domain_expt.FieldType_EvaluatorScore,
		}
		scoreField.SetFieldKey(gptr.Of("101"))
		filters.SetFilterConditions([]*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_TurnRunState,
				},
				Operator: domain_expt.FilterOperatorType_In,
				Value:    "1",
			},
			{
				Field:    scoreField,
				Operator: domain_expt.FilterOperatorType_Greater,
				Value:    "0.8",
			},
		})

		result, err := ConvertExptTurnResultFilter(filters)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.TrunRunStateFilters, 1)
		assert.Len(t, result.ScoreFilters, 1)
	})
}

// TestParseTurnRunState 测试 parseTurnRunState 函数
func TestParseTurnRunState(t *testing.T) {
	t.Run("成功解析单个状态", func(t *testing.T) {
		cond := &domain_expt.FilterCondition{}
		cond.SetValue("1")

		result, err := parseTurnRunState(cond)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, entity.TurnRunState(1), result[0])
	})

	t.Run("成功解析多个状态", func(t *testing.T) {
		cond := &domain_expt.FilterCondition{}
		cond.SetValue("1,2,3")

		result, err := parseTurnRunState(cond)
		assert.NoError(t, err)
		assert.Len(t, result, 3)
		assert.Contains(t, result, entity.TurnRunState(1))
		assert.Contains(t, result, entity.TurnRunState(2))
		assert.Contains(t, result, entity.TurnRunState(3))
	})

	t.Run("空字符串跳过", func(t *testing.T) {
		cond := &domain_expt.FilterCondition{}
		cond.SetValue("1,,3")

		result, err := parseTurnRunState(cond)
		assert.NoError(t, err)
		assert.Len(t, result, 2) // 空字符串被跳过
		assert.Contains(t, result, entity.TurnRunState(1))
		assert.Contains(t, result, entity.TurnRunState(3))
	})

	t.Run("值开头为空字符串", func(t *testing.T) {
		cond := &domain_expt.FilterCondition{}
		cond.SetValue(",1,2")

		result, err := parseTurnRunState(cond)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Contains(t, result, entity.TurnRunState(1))
		assert.Contains(t, result, entity.TurnRunState(2))
	})

	t.Run("值结尾为空字符串", func(t *testing.T) {
		cond := &domain_expt.FilterCondition{}
		cond.SetValue("1,2,")

		result, err := parseTurnRunState(cond)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Contains(t, result, entity.TurnRunState(1))
		assert.Contains(t, result, entity.TurnRunState(2))
	})

	t.Run("无效的状态值返回错误", func(t *testing.T) {
		cond := &domain_expt.FilterCondition{}
		cond.SetValue("invalid")

		result, err := parseTurnRunState(cond)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid turn run state")
	})

	t.Run("部分无效的状态值返回错误", func(t *testing.T) {
		cond := &domain_expt.FilterCondition{}
		cond.SetValue("1,invalid,3")

		result, err := parseTurnRunState(cond)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid turn run state")
	})
}

// TestCheckFilterCondition 测试 checkFilterCondition 函数
func TestCheckFilterCondition(t *testing.T) {
	t.Run("TurnRunState字段，In操作符，通过验证", func(t *testing.T) {
		cond := domain_expt.FilterCondition{}
		cond.SetField(&domain_expt.FilterField{
			FieldType: domain_expt.FieldType_TurnRunState,
		})
		cond.SetOperator(domain_expt.FilterOperatorType_In)

		err := checkFilterCondition(cond)
		assert.NoError(t, err)
	})

	t.Run("TurnRunState字段，NotIn操作符，通过验证", func(t *testing.T) {
		cond := domain_expt.FilterCondition{}
		cond.SetField(&domain_expt.FilterField{
			FieldType: domain_expt.FieldType_TurnRunState,
		})
		cond.SetOperator(domain_expt.FilterOperatorType_NotIn)

		err := checkFilterCondition(cond)
		assert.NoError(t, err)
	})

	t.Run("TurnRunState字段，无效操作符返回错误", func(t *testing.T) {
		cond := domain_expt.FilterCondition{}
		cond.SetField(&domain_expt.FilterField{
			FieldType: domain_expt.FieldType_TurnRunState,
		})
		cond.SetOperator(domain_expt.FilterOperatorType_Equal) // 无效操作符

		err := checkFilterCondition(cond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid operator")
	})

	t.Run("TurnRunState字段，Greater操作符返回错误", func(t *testing.T) {
		cond := domain_expt.FilterCondition{}
		cond.SetField(&domain_expt.FilterField{
			FieldType: domain_expt.FieldType_TurnRunState,
		})
		cond.SetOperator(domain_expt.FilterOperatorType_Greater)

		err := checkFilterCondition(cond)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid operator")
	})

	t.Run("其他字段类型，通过验证", func(t *testing.T) {
		cond := domain_expt.FilterCondition{}
		cond.SetField(&domain_expt.FilterField{
			FieldType: domain_expt.FieldType_CreatorBy,
		})
		cond.SetOperator(domain_expt.FilterOperatorType_Equal)

		err := checkFilterCondition(cond)
		assert.NoError(t, err)
	})
}
