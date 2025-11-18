// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
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
            defer func() { _ = sqlDB.Close() }()

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

			// GetSourceIDsByFilterConditions 即使 filterOption 为 nil，也会创建一个空的 EvaluatorFilterOption 并继续执行查询
			// 所以所有测试用例都需要 mock NewSession 和数据库查询
			mockProvider.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB).Times(1)

			// 判断是否有 SearchKeyword 和 Filters
			hasSearchKeyword := tt.filterOption != nil && tt.filterOption.SearchKeyword != nil && *tt.filterOption.SearchKeyword != ""
			hasFilters := tt.filterOption != nil && tt.filterOption.Filters != nil

			// Mock COUNT 查询（放宽匹配，兼容 JOIN、别名与列限定）
			// 所有查询都会包含 LEFT JOIN t_name（用于排序）
			// 如果有 SearchKeyword，COUNT 查询也会包含 t_name.tag_value LIKE
			countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
			if hasSearchKeyword {
				mock.ExpectQuery("SELECT COUNT\\(DISTINCT\\(.*source_id.*\\)\\) FROM `evaluator_tag`.*LEFT JOIN evaluator_tag AS t_name.*WHERE .*t_name\\.tag_value.*LIKE.*").WillReturnRows(countRows)
			} else {
				mock.ExpectQuery("SELECT COUNT\\(DISTINCT\\(.*source_id.*\\)\\) FROM `evaluator_tag`.*LEFT JOIN evaluator_tag AS t_name.*").WillReturnRows(countRows)
			}

			// Mock SELECT 查询（放宽匹配，兼容 DISTINCT、JOIN、ORDER BY 等）
			// 所有查询都会包含 LEFT JOIN t_name（用于排序）和 ORDER BY t_name.tag_value
			selectRows := sqlmock.NewRows([]string{"source_id"})

			if hasSearchKeyword && hasFilters {
				// 既有 SearchKeyword 又有 Filters 时，SQL 包含：
				// - LEFT JOIN t_name
				// - JOIN t_1, JOIN t_2 等（自连接，来自 Filters 的 AND 条件）
				// - WHERE t_name.tag_value LIKE ? (来自 SearchKeyword，使用 t_name 别名)
				// - 可能还有其他 WHERE 条件（来自 Filters 的 OR 条件）
				// - ORDER BY
				// 注意：WHERE 条件可能包含括号，且使用参数化查询（?），需要更宽松的匹配
				mock.ExpectQuery("SELECT DISTINCT .*source_id.* FROM `evaluator_tag`.*LEFT JOIN evaluator_tag AS t_name.*JOIN evaluator_tag AS.*WHERE .*t_name\\.tag_value.*LIKE.*ORDER BY.*").WillReturnRows(selectRows)
			} else if hasSearchKeyword {
				// 只有 SearchKeyword 时，SQL 包含 WHERE t_name.tag_value LIKE ? (使用 t_name 别名)
				// 注意：WHERE 条件可能包含括号，且还有其他基础 WHERE 条件（tag_type, deleted_at），使用参数化查询
				mock.ExpectQuery("SELECT DISTINCT .*source_id.* FROM `evaluator_tag`.*LEFT JOIN evaluator_tag AS t_name.*WHERE .*t_name\\.tag_value.*LIKE.*ORDER BY.*").WillReturnRows(selectRows)
			} else if hasFilters {
				// 只有 Filters 时，SQL 可能包含 JOIN 其他表（自连接）和 WHERE 条件
				// 需要匹配可能包含的 JOIN 自连接
				mock.ExpectQuery("SELECT DISTINCT .*source_id.* FROM `evaluator_tag`.*LEFT JOIN evaluator_tag AS t_name.*ORDER BY.*").WillReturnRows(selectRows)
			} else {
				// 无 SearchKeyword 和 Filters 时，SQL 只包含基础查询和 LEFT JOIN t_name
				mock.ExpectQuery("SELECT DISTINCT .*source_id.* FROM `evaluator_tag`.*LEFT JOIN evaluator_tag AS t_name.*ORDER BY.*").WillReturnRows(selectRows)
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
    defer func() { _ = sqlDB.Close() }()

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
    // COUNT 查询的 WHERE 子句也使用 t_name.tag_value LIKE
    countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
    mock.ExpectQuery(
        "SELECT COUNT\\(DISTINCT\\(.*source_id.*\\)\\) FROM `evaluator_tag`.*LEFT JOIN evaluator_tag AS t_name.*JOIN evaluator_tag AS t_1.*JOIN evaluator_tag AS t_2.*WHERE .*t_name\\.tag_value.*LIKE.*",
    ).WillReturnRows(countRows)

    // 断言 SELECT：包含 DISTINCT、LEFT JOIN t_name、JOIN t_1 / t_2、LIKE 与 Name 标签限定
    // SearchKeyword 现在使用 t_name 别名进行 LIKE 查询，避免与主表条件冲突
    selectRows := sqlmock.NewRows([]string{"source_id"})
    mock.ExpectQuery(
        "SELECT DISTINCT .*source_id.* FROM `evaluator_tag`.*LEFT JOIN evaluator_tag AS t_name.*JOIN evaluator_tag AS t_1.*JOIN evaluator_tag AS t_2.*WHERE .*t_name\\.tag_value.*LIKE.*ORDER BY.*",
    ).WillReturnRows(selectRows)

    dao := &EvaluatorTagDAOImpl{provider: mockProvider}
    _, _, err = dao.GetSourceIDsByFilterConditions(context.Background(), 1, option, 12, 1, "zh-CN")
    assert.NoError(t, err)

    if err := mock.ExpectationsWereMet(); err != nil {
        t.Errorf("there were unfulfilled expectations: %s", err)
    }
}

func TestEvaluatorTagDAOImpl_AggregateTagValuesByType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		tagType      int32
		langType     string
		mockSetup    func(*sqlmock.Sqlmock)
		expectedTags []*entity.AggregatedEvaluatorTag
		expectedErr  bool
		description  string
	}{
		{
			name:     "success - with lang type",
			tagType:  1,
			langType: "zh-CN",
			mockSetup: func(mock *sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"tag_key", "tag_value"}).
					AddRow("Category", "LLM").
					AddRow("Category", "Code").
					AddRow("TargetType", "Text").
					AddRow("TargetType", "Image")
				(*mock).ExpectQuery("SELECT tag_key, tag_value FROM `evaluator_tag` WHERE tag_type = \\? AND deleted_at IS NULL AND lang_type = \\? GROUP BY tag_key, tag_value").
					WithArgs(1, "zh-CN").
					WillReturnRows(rows)
			},
			expectedTags: []*entity.AggregatedEvaluatorTag{
				{TagKey: "Category", TagValue: "LLM"},
				{TagKey: "Category", TagValue: "Code"},
				{TagKey: "TargetType", TagValue: "Text"},
				{TagKey: "TargetType", TagValue: "Image"},
			},
			expectedErr: false,
			description: "有语言类型时，应该正确聚合标签",
		},
		{
			name:     "success - without lang type",
			tagType:  1,
			langType: "",
			mockSetup: func(mock *sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"tag_key", "tag_value"}).
					AddRow("Category", "LLM").
					AddRow("TargetType", "Text")
				(*mock).ExpectQuery("SELECT tag_key, tag_value FROM `evaluator_tag` WHERE tag_type = \\? AND deleted_at IS NULL GROUP BY tag_key, tag_value").
					WithArgs(1).
					WillReturnRows(rows)
			},
			expectedTags: []*entity.AggregatedEvaluatorTag{
				{TagKey: "Category", TagValue: "LLM"},
				{TagKey: "TargetType", TagValue: "Text"},
			},
			expectedErr: false,
			description: "无语言类型时，应该正确聚合标签",
		},
		{
			name:     "success - empty result",
			tagType:  1,
			langType: "zh-CN",
			mockSetup: func(mock *sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"tag_key", "tag_value"})
				(*mock).ExpectQuery("SELECT tag_key, tag_value FROM `evaluator_tag` WHERE tag_type = \\? AND deleted_at IS NULL AND lang_type = \\? GROUP BY tag_key, tag_value").
					WithArgs(1, "zh-CN").
					WillReturnRows(rows)
			},
			expectedTags: []*entity.AggregatedEvaluatorTag{},
			expectedErr:  false,
			description: "无结果时，应该返回空切片",
		},
		{
			name:     "success - record not found",
			tagType:  1,
			langType: "zh-CN",
			mockSetup: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectQuery("SELECT tag_key, tag_value FROM `evaluator_tag` WHERE tag_type = \\? AND deleted_at IS NULL AND lang_type = \\? GROUP BY tag_key, tag_value").
					WithArgs(1, "zh-CN").
					WillReturnError(gorm.ErrRecordNotFound)
			},
			expectedTags: []*entity.AggregatedEvaluatorTag{},
			expectedErr:  false,
			description: "记录不存在时，应该返回空切片",
		},
		{
			name:     "error - database error",
			tagType:  1,
			langType: "zh-CN",
			mockSetup: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectQuery("SELECT tag_key, tag_value FROM `evaluator_tag` WHERE tag_type = \\? AND deleted_at IS NULL AND lang_type = \\? GROUP BY tag_key, tag_value").
					WithArgs(1, "zh-CN").
					WillReturnError(errors.New("database error"))
			},
			expectedTags: nil,
			expectedErr:  true,
			description: "数据库错误时，应该返回错误",
		},
		{
			name:     "success - template tag type",
			tagType:  2,
			langType: "en-US",
			mockSetup: func(mock *sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"tag_key", "tag_value"}).
					AddRow("Category", "Prompt").
					AddRow("Category", "Code")
				(*mock).ExpectQuery("SELECT tag_key, tag_value FROM `evaluator_tag` WHERE tag_type = \\? AND deleted_at IS NULL AND lang_type = \\? GROUP BY tag_key, tag_value").
					WithArgs(2, "en-US").
					WillReturnRows(rows)
			},
			expectedTags: []*entity.AggregatedEvaluatorTag{
				{TagKey: "Category", TagValue: "Prompt"},
				{TagKey: "Category", TagValue: "Code"},
			},
			expectedErr: false,
			description: "模板标签类型时，应该正确聚合标签",
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
			defer func() { _ = sqlDB.Close() }()

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
			mockProvider.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB).Times(1)

			// 设置mock期望
			tt.mockSetup(&mock)

			// 创建DAO实例
			dao := &EvaluatorTagDAOImpl{
				provider: mockProvider,
			}

			// 执行测试
			ctx := context.Background()
			result, err := dao.AggregateTagValuesByType(ctx, tt.tagType, tt.langType)

			// 验证结果
			if tt.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, len(tt.expectedTags), len(result))
				for i, expected := range tt.expectedTags {
					if i < len(result) {
						assert.Equal(t, expected.TagKey, result[i].TagKey)
						assert.Equal(t, expected.TagValue, result[i].TagValue)
					}
				}
			}

			// 验证所有期望的SQL查询都被执行
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
		})
	}
}
