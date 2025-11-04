// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

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
			name:         "nil filter option",
			tagType:      1,
			filterOption: nil,
			expectedErr:  false,
			description:  "当筛选选项为nil时，应该返回空列表",
		},
		{
			name:         "empty filter option",
			tagType:      1,
			filterOption: &entity.EvaluatorFilterOption{},
			expectedErr:  false,
			description:  "当筛选选项为空时，应该返回空列表",
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
		{
			name:    "nested sub filters (AND with OR and AND groups)",
			tagType: 1,
			filterOption: func() *entity.EvaluatorFilterOption {
				// 顶层：AND + Category=LLM
				top := entity.NewEvaluatorFilters().
					WithLogicOp(entity.FilterLogicOp_And).
					AddCondition(entity.NewEvaluatorFilterCondition(
						entity.EvaluatorTagKey_Category,
						entity.EvaluatorFilterOperatorType_Equal,
						"LLM",
					))

				// 子组1：OR => TargetType IN(Text,Image) OR Name LIKE Qual
				or := entity.FilterLogicOp_Or
				sub1 := (&entity.EvaluatorFilters{LogicOp: &or}).
					AddCondition(entity.NewEvaluatorFilterCondition(
						entity.EvaluatorTagKey_TargetType,
						entity.EvaluatorFilterOperatorType_In,
						"Text,Image",
					)).
					AddCondition(entity.NewEvaluatorFilterCondition(
						entity.EvaluatorTagKey_Name,
						entity.EvaluatorFilterOperatorType_Like,
						"Qual",
					))

				// 子组2：AND => Objective = Quality
				and := entity.FilterLogicOp_And
				sub2 := (&entity.EvaluatorFilters{LogicOp: &and}).
					AddCondition(entity.NewEvaluatorFilterCondition(
						entity.EvaluatorTagKey_Objective,
						entity.EvaluatorFilterOperatorType_Equal,
						"Quality",
					))

				// 绑定子组
				top.SubFilters = []*entity.EvaluatorFilters{sub1, sub2}

				return (&entity.EvaluatorFilterOption{}).WithFilters(top)
			}(),
			expectedErr: false,
			description: "嵌套子过滤组应正确展开并构造 SQL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// 创建sqlmock连接
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer sqlDB.Close()

			// 创建真实的GORM数据库连接
			gormDB, err := gorm.Open(mysql.New(mysql.Config{
				Conn:                      sqlDB,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{})
			if err != nil {
				t.Fatalf("failed to open gorm db: %v", err)
			}

			// 创建mock provider
			mockProvider := dbmock.NewMockProvider(ctrl)

			// 对于nil的filterOption，方法会直接返回，不需要数据库调用
			if tt.filterOption == nil {
				// 这种情况下方法直接返回，不需要设置mock期望
			} else {
				// 对于非nil的filterOption，方法会调用NewSession并执行查询
				mockProvider.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB).Times(1)

                // Mock COUNT 查询（放宽匹配，兼容 JOIN、别名与列限定）
                countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
                mock.ExpectQuery("SELECT COUNT\\(DISTINCT\\(.*source_id.*\\)\\) FROM `evaluator_tag`.*").WillReturnRows(countRows)

                // Mock SELECT 查询（放宽匹配，兼容 DISTINCT、JOIN、ORDER BY 等）
                selectRows := sqlmock.NewRows([]string{"source_id"})
                mock.ExpectQuery("SELECT DISTINCT .*source_id.* FROM `evaluator_tag`.*").WillReturnRows(selectRows)
			}

			// 创建DAO实例
			dao := &EvaluatorTagDAOImpl{
				provider: mockProvider,
			}

			// 执行测试
			ctx := context.Background()
			result, total, err := dao.GetSourceIDsByFilterConditions(ctx, tt.tagType, tt.filterOption, 0, 0, "")
			_ = total

			// 验证结果
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				// 对于nil或空的filterOption，应该返回空列表
				if tt.filterOption == nil || (tt.filterOption.SearchKeyword == nil && (tt.filterOption.Filters == nil || (len(tt.filterOption.Filters.FilterConditions) == 0 && len(tt.filterOption.Filters.SubFilters) == 0))) {
					assert.Empty(t, result)
				}
			}

			// 验证所有期望的SQL查询都被执行
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestBuildSingleCondition(t *testing.T) {
	t.Parallel()

	dao := &EvaluatorTagDAOImpl{}

	tests := []struct {
		name         string
		condition    *entity.EvaluatorFilterCondition
		expectedSQL  string
		expectedArgs []interface{}
		expectedErr  bool
	}{
		{
			name: "equal condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_Category,
				entity.EvaluatorFilterOperatorType_Equal,
				"LLM",
			),
			expectedSQL:  "evaluator_tag.tag_key = ? AND evaluator_tag.tag_value = ?",
			expectedArgs: []interface{}{"Category", "LLM"},
			expectedErr:  false,
		},
		{
			name: "not equal condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_Category,
				entity.EvaluatorFilterOperatorType_NotEqual,
				"Code",
			),
			expectedSQL:  "evaluator_tag.tag_key = ? AND evaluator_tag.tag_value != ?",
			expectedArgs: []interface{}{"Category", "Code"},
			expectedErr:  false,
		},
		{
			name: "in condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_TargetType,
				entity.EvaluatorFilterOperatorType_In,
				"Text,Image,Video",
			),
			expectedSQL:  "evaluator_tag.tag_key = ? AND evaluator_tag.tag_value IN (?,?,?)",
			expectedArgs: []interface{}{"TargetType", "Text", "Image", "Video"},
			expectedErr:  false,
		},
		{
			name: "like condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_Name,
				entity.EvaluatorFilterOperatorType_Like,
				"Quality",
			),
			expectedSQL:  "evaluator_tag.tag_key = ? AND evaluator_tag.tag_value LIKE ?",
			expectedArgs: []interface{}{"Name", "%Quality%"},
			expectedErr:  false,
		},
		{
			name: "is null condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_Objective,
				entity.EvaluatorFilterOperatorType_IsNull,
				"",
			),
			expectedSQL:  "evaluator_tag.tag_key = ? AND evaluator_tag.tag_value IS NULL",
			expectedArgs: []interface{}{"Objective"},
			expectedErr:  false,
		},
		{
			name: "is not null condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_Objective,
				entity.EvaluatorFilterOperatorType_IsNotNull,
				"",
			),
			expectedSQL:  "evaluator_tag.tag_key = ? AND evaluator_tag.tag_value IS NOT NULL",
			expectedArgs: []interface{}{"Objective"},
			expectedErr:  false,
		},
		{
			name: "empty in condition",
			condition: entity.NewEvaluatorFilterCondition(
				entity.EvaluatorTagKey_TargetType,
				entity.EvaluatorFilterOperatorType_In,
				"",
			),
			expectedSQL:  "evaluator_tag.tag_key = ? AND evaluator_tag.tag_value IN (?)",
			expectedArgs: []interface{}{"TargetType", ""},
			expectedErr:  false,
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

func TestGetSourceIDsByFilterConditions_SelfJoinAndLike(t *testing.T) {
    t.Parallel()

    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    // sqlmock
    sqlDB, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("failed to create sqlmock: %v", err)
    }
    defer sqlDB.Close()

    gormDB, err := gorm.Open(mysql.New(mysql.Config{
        Conn:                      sqlDB,
        SkipInitializeWithVersion: true,
    }), &gorm.Config{})
    if err != nil {
        t.Fatalf("failed to open gorm db: %v", err)
    }

    mockProvider := dbmock.NewMockProvider(ctrl)
    mockProvider.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB).Times(1)

    // 构造筛选：AND(Category=LLM, BusinessScenario=安全风控) + SearchKeyword("AI")
    filters := entity.NewEvaluatorFilters().
        WithLogicOp(entity.FilterLogicOp_And).
        AddCondition(entity.NewEvaluatorFilterCondition(
            entity.EvaluatorTagKey_Category,
            entity.EvaluatorFilterOperatorType_In,
            "LLM",
        )).
        AddCondition(entity.NewEvaluatorFilterCondition(
            entity.EvaluatorTagKey_BusinessScenario,
            entity.EvaluatorFilterOperatorType_In,
            "安全风控",
        ))
    option := entity.NewEvaluatorFilterOption().WithSearchKeyword("AI").WithFilters(filters)

    // 断言 COUNT：包含 LEFT JOIN t_name、JOIN t_1 / t_2，且基表为 evaluator_tag
    countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
    mock.ExpectQuery(
        "SELECT COUNT\\(DISTINCT\\(.*source_id.*\\)\\) FROM `evaluator_tag`.*LEFT JOIN evaluator_tag AS t_name.*JOIN evaluator_tag AS t_1.*JOIN evaluator_tag AS t_2.*",
    ).WillReturnRows(countRows)

    // 断言 SELECT：包含 DISTINCT、LEFT JOIN t_name、JOIN t_1 / t_2、LIKE 与 非 Category 限定
    selectRows := sqlmock.NewRows([]string{"source_id"})
    mock.ExpectQuery(
        "SELECT DISTINCT .*source_id.* FROM `evaluator_tag`.*LEFT JOIN evaluator_tag AS t_name.*JOIN evaluator_tag AS t_1.*JOIN evaluator_tag AS t_2.*WHERE .*evaluator_tag.tag_key <> .* AND evaluator_tag.tag_value LIKE .*",
    ).WillReturnRows(selectRows)

    dao := &EvaluatorTagDAOImpl{provider: mockProvider}
    _, _, err = dao.GetSourceIDsByFilterConditions(context.Background(), 1, option, 12, 1, "zh-CN")
    assert.NoError(t, err)

    if err := mock.ExpectationsWereMet(); err != nil {
        t.Errorf("there were unfulfilled expectations: %s", err)
    }
}
