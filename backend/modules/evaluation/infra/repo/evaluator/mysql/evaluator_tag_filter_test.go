// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// MockProvider 模拟数据库提供者
type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) NewSession(ctx context.Context, opts ...db.Option) *gorm.DB {
	args := m.Called(ctx, opts)
	return args.Get(0).(*gorm.DB)
}

func (m *MockProvider) Transaction(ctx context.Context, fc func(tx *gorm.DB) error, opts ...db.Option) error {
	args := m.Called(ctx, fc, opts)
	return args.Error(0)
}

func TestEvaluatorTagDAOImpl_GetSourceIDsByFilterConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		tagType      int32
		filterOption *entity.EvaluatorFilterOption
		expectedErr  bool
		description  string
	}{
		{
			name:        "nil filter option",
			tagType:     1,
			filterOption: nil,
			expectedErr: false,
			description: "当筛选选项为nil时，应该返回空列表",
		},
		{
			name:    "empty filter option",
			tagType: 1,
			filterOption: &entity.EvaluatorFilterOption{},
			expectedErr: false,
			description: "当筛选选项为空时，应该返回空列表",
		},
		{
			name:    "search keyword only",
			tagType: 1,
			filterOption: entity.NewEvaluatorFilterOption().
				WithSearchKeyword("AI"),
			expectedErr: false,
			description: "只有搜索关键词时，应该正确构建查询",
		},
		{
			name:    "single equal condition",
			tagType: 1,
			filterOption: entity.NewEvaluatorFilterOption().
				WithFilters(
					entity.NewEvaluatorFilters().
						WithLogicOp(entity.FilterLogicOp_And).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_Category,
							entity.EvaluatorFilterOperatorType_Equal,
							"LLM",
						)),
				),
			expectedErr: false,
			description: "单个等于条件，应该正确构建查询",
		},
		{
			name:    "multiple AND conditions",
			tagType: 1,
			filterOption: entity.NewEvaluatorFilterOption().
				WithFilters(
					entity.NewEvaluatorFilters().
						WithLogicOp(entity.FilterLogicOp_And).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_Category,
							entity.EvaluatorFilterOperatorType_Equal,
							"LLM",
						)).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_TargetType,
							entity.EvaluatorFilterOperatorType_In,
							"Text,Image",
						)),
				),
			expectedErr: false,
			description: "多个AND条件，应该正确构建查询",
		},
		{
			name:    "multiple OR conditions",
			tagType: 1,
			filterOption: entity.NewEvaluatorFilterOption().
				WithFilters(
					entity.NewEvaluatorFilters().
						WithLogicOp(entity.FilterLogicOp_Or).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_Category,
							entity.EvaluatorFilterOperatorType_Equal,
							"LLM",
						)).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_Category,
							entity.EvaluatorFilterOperatorType_Equal,
							"Code",
						)),
				),
			expectedErr: false,
			description: "多个OR条件，应该正确构建查询",
		},
		{
			name:    "like condition",
			tagType: 1,
			filterOption: entity.NewEvaluatorFilterOption().
				WithFilters(
					entity.NewEvaluatorFilters().
						WithLogicOp(entity.FilterLogicOp_And).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_Name,
							entity.EvaluatorFilterOperatorType_Like,
							"Quality",
						)),
				),
			expectedErr: false,
			description: "LIKE条件，应该正确构建查询",
		},
		{
			name:    "in condition",
			tagType: 1,
			filterOption: entity.NewEvaluatorFilterOption().
				WithFilters(
					entity.NewEvaluatorFilters().
						WithLogicOp(entity.FilterLogicOp_And).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_TargetType,
							entity.EvaluatorFilterOperatorType_In,
							"Text,Image,Video",
						)),
				),
			expectedErr: false,
			description: "IN条件，应该正确构建查询",
		},
		{
			name:    "not in condition",
			tagType: 1,
			filterOption: entity.NewEvaluatorFilterOption().
				WithFilters(
					entity.NewEvaluatorFilters().
						WithLogicOp(entity.FilterLogicOp_And).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_TargetType,
							entity.EvaluatorFilterOperatorType_NotIn,
							"Audio,Video",
						)),
				),
			expectedErr: false,
			description: "NOT_IN条件，应该正确构建查询",
		},
		{
			name:    "is null condition",
			tagType: 1,
			filterOption: entity.NewEvaluatorFilterOption().
				WithFilters(
					entity.NewEvaluatorFilters().
						WithLogicOp(entity.FilterLogicOp_And).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_Objective,
							entity.EvaluatorFilterOperatorType_IsNull,
							"",
						)),
				),
			expectedErr: false,
			description: "IS_NULL条件，应该正确构建查询",
		},
		{
			name:    "is not null condition",
			tagType: 1,
			filterOption: entity.NewEvaluatorFilterOption().
				WithFilters(
					entity.NewEvaluatorFilters().
						WithLogicOp(entity.FilterLogicOp_And).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_Objective,
							entity.EvaluatorFilterOperatorType_IsNotNull,
							"",
						)),
				),
			expectedErr: false,
			description: "IS_NOT_NULL条件，应该正确构建查询",
		},
		{
			name:    "complex combination",
			tagType: 1,
			filterOption: entity.NewEvaluatorFilterOption().
				WithSearchKeyword("AI").
				WithFilters(
					entity.NewEvaluatorFilters().
						WithLogicOp(entity.FilterLogicOp_And).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_Category,
							entity.EvaluatorFilterOperatorType_Equal,
							"LLM",
						)).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_TargetType,
							entity.EvaluatorFilterOperatorType_In,
							"Text,Image",
						)).
						AddCondition(entity.NewEvaluatorFilterCondition(
							entity.EvaluatorTagKey_Objective,
							entity.EvaluatorFilterOperatorType_Like,
							"Quality",
						)),
				),
			expectedErr: false,
			description: "复杂组合条件（搜索关键词+多个AND条件），应该正确构建查询",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// 创建mock provider
			mockProvider := &MockProvider{}

			// 创建DAO实例
			dao := &EvaluatorTagDAOImpl{
				provider: mockProvider,
			}

			// 模拟数据库会话
			mockSession := &gorm.DB{}
			mockProvider.On("NewSession", mock.Anything, mock.Anything).Return(mockSession)

			// 执行测试
			ctx := context.Background()
			result, err := dao.GetSourceIDsByFilterConditions(ctx, tt.tagType, tt.filterOption)

			// 验证结果
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				// 对于nil或空的filterOption，应该返回空列表
				if tt.filterOption == nil || (tt.filterOption.SearchKeyword == nil && (tt.filterOption.Filters == nil || len(tt.filterOption.Filters.FilterConditions) == 0)) {
					assert.Empty(t, result)
				}
			}

			// 验证mock调用
			mockProvider.AssertExpectations(t)
		})
	}
}

func TestBuildSingleCondition(t *testing.T) {
	t.Parallel()

	dao := &EvaluatorTagDAOImpl{}

	tests := []struct {
		name        string
		condition   *entity.EvaluatorFilterCondition
		expectedSQL string
		expectedArgs []interface{}
		expectedErr bool
	}{
		{
			name: "equal condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_Category,
				entity.EvaluatorFilterOperatorType_Equal,
				"LLM",
			),
			expectedSQL: "tag_key = ? AND tag_value = ?",
			expectedArgs: []interface{}{"Category", "LLM"},
			expectedErr: false,
		},
		{
			name: "not equal condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_Category,
				entity.EvaluatorFilterOperatorType_NotEqual,
				"Code",
			),
			expectedSQL: "tag_key = ? AND tag_value != ?",
			expectedArgs: []interface{}{"Category", "Code"},
			expectedErr: false,
		},
		{
			name: "in condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_TargetType,
				entity.EvaluatorFilterOperatorType_In,
				"Text,Image,Video",
			),
			expectedSQL: "tag_key = ? AND tag_value IN (?,?,?)",
			expectedArgs: []interface{}{"TargetType", "Text", "Image", "Video"},
			expectedErr: false,
		},
		{
			name: "like condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_Name,
				entity.EvaluatorFilterOperatorType_Like,
				"Quality",
			),
			expectedSQL: "tag_key = ? AND tag_value LIKE ?",
			expectedArgs: []interface{}{"Name", "%Quality%"},
			expectedErr: false,
		},
		{
			name: "is null condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_Objective,
				entity.EvaluatorFilterOperatorType_IsNull,
				"",
			),
			expectedSQL: "tag_key = ? AND tag_value IS NULL",
			expectedArgs: []interface{}{"Objective"},
			expectedErr: false,
		},
		{
			name: "is not null condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_Objective,
				entity.EvaluatorFilterOperatorType_IsNotNull,
				"",
			),
			expectedSQL: "tag_key = ? AND tag_value IS NOT NULL",
			expectedArgs: []interface{}{"Objective"},
			expectedErr: false,
		},
		{
			name: "empty in condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_TargetType,
				entity.EvaluatorFilterOperatorType_In,
				"",
			),
			expectedSQL: "tag_key = ? AND tag_value IN (?)",
			expectedArgs: []interface{}{"TargetType", ""},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sql, args, err := dao.buildSingleCondition(tt.condition)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSQL, sql)
				assert.Equal(t, tt.expectedArgs, args)
			}
		})
	}
}

func TestConvertToInterfaceSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		expected []interface{}
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []interface{}{},
		},
		{
			name:     "single element",
			input:    []string{"test"},
			expected: []interface{}{"test"},
		},
		{
			name:     "multiple elements",
			input:    []string{"a", "b", "c"},
			expected: []interface{}{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := convertToInterfaceSlice(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
