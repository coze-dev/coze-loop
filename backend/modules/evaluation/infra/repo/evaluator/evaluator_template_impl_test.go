// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	mysqlmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
)

// TestEvaluatorTemplateRepoImpl_ListEvaluatorTemplate 测试查询评估器模板列表
func TestEvaluatorTemplateRepoImpl_ListEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		request        *repo.ListEvaluatorTemplateRequest
		mockSetup      func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO)
		expectedResult *repo.ListEvaluatorTemplateResponse
		expectedError  bool
		description    string
	}{
		{
			name: "成功 - 无筛选条件",
			request: &repo.ListEvaluatorTemplateRequest{
				SpaceID:        123,
				FilterOption:   nil,
				PageSize:       10,
				PageNum:        1,
				IncludeDeleted: false,
			},
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO) {
				expectedDAOReq := &mysql.ListEvaluatorTemplateRequest{
					IDs:            []int64{},
					PageSize:       10,
					PageNum:        1,
					IncludeDeleted: false,
				}
				mockTemplateDAO.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), expectedDAOReq).
					Return(&mysql.ListEvaluatorTemplateResponse{
						TotalCount: 2,
						Templates: []*model.EvaluatorTemplate{
                            {
								ID:            1,
								SpaceID:       gptr.Of(int64(123)),
								Name:          gptr.Of("Template A"),
								Description:   gptr.Of("Description A"),
								EvaluatorType: gptr.Of(int32(1)),
                                EvaluatorInfo: gptr.Of([]byte(`{"benchmark":"benchmark1","vendor":"vendor1"}`)),
								Popularity:    100,
								CreatedBy:     "user1",
								UpdatedBy:     "user1",
							},
							{
								ID:            2,
								SpaceID:       gptr.Of(int64(123)),
								Name:          gptr.Of("Template B"),
								Description:   gptr.Of("Description B"),
								EvaluatorType: gptr.Of(int32(2)),
                                EvaluatorInfo: gptr.Of([]byte(`{"benchmark":"benchmark2","vendor":"vendor2"}`)),
								Popularity:    200,
								CreatedBy:     "user2",
								UpdatedBy:     "user2",
							},
						},
					}, nil)

				mockTagDAO.EXPECT().
					BatchGetTagsBySourceIDsAndType(gomock.Any(), []int64{1, 2}, int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), gomock.Any()).
					Return([]*model.EvaluatorTag{}, nil)
			},
			expectedResult: &repo.ListEvaluatorTemplateResponse{
				TotalCount: 2,
				Templates: []*entity.EvaluatorTemplate{
					{ID: 1, SpaceID: 123, Name: "Template A"},
					{ID: 2, SpaceID: 123, Name: "Template B"},
				},
			},
			expectedError: false,
			description:   "无筛选条件时，应该直接查询所有模板",
		},
		{
			name: "成功 - 有筛选条件",
			request: &repo.ListEvaluatorTemplateRequest{
				SpaceID: 123,
				FilterOption: entity.NewEvaluatorFilterOption().
					WithFilters(
						entity.NewEvaluatorFilters().
							WithLogicOp(entity.FilterLogicOp_And).
							AddCondition(entity.NewEvaluatorFilterCondition(
								entity.EvaluatorTagKey_Category,
								entity.EvaluatorFilterOperatorType_Equal,
								"LLM",
							)),
					),
				PageSize:       10,
				PageNum:        1,
				IncludeDeleted: false,
			},
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO) {
				filterOption := entity.NewEvaluatorFilterOption().
					WithFilters(
						entity.NewEvaluatorFilters().
							WithLogicOp(entity.FilterLogicOp_And).
							AddCondition(entity.NewEvaluatorFilterCondition(
								entity.EvaluatorTagKey_Category,
								entity.EvaluatorFilterOperatorType_Equal,
								"LLM",
							)),
					)
				mockTagDAO.EXPECT().
					GetSourceIDsByFilterConditions(gomock.Any(), int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), filterOption, int32(0), int32(0), gomock.Any()).
					Return([]int64{1, 3}, int64(2), nil)

				expectedDAOReq := &mysql.ListEvaluatorTemplateRequest{
					IDs:            []int64{1, 3},
					PageSize:       10,
					PageNum:        1,
					IncludeDeleted: false,
				}
				mockTemplateDAO.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), expectedDAOReq).
					Return(&mysql.ListEvaluatorTemplateResponse{
						TotalCount: 1,
						Templates: []*model.EvaluatorTemplate{
                            {
								ID:            1,
								SpaceID:       gptr.Of(int64(123)),
								Name:          gptr.Of("Template A"),
								Description:   gptr.Of("Description A"),
								EvaluatorType: gptr.Of(int32(1)),
                                EvaluatorInfo: gptr.Of([]byte(`{"benchmark":"benchmark1","vendor":"vendor1"}`)),
								Popularity:    100,
								CreatedBy:     "user1",
								UpdatedBy:     "user1",
							},
						},
					}, nil)

				mockTagDAO.EXPECT().
					BatchGetTagsBySourceIDsAndType(gomock.Any(), []int64{1}, int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), gomock.Any()).
					Return([]*model.EvaluatorTag{}, nil)
			},
			expectedResult: &repo.ListEvaluatorTemplateResponse{
				TotalCount: 1,
				Templates: []*entity.EvaluatorTemplate{
					{ID: 1, SpaceID: 123, Name: "Template A"},
				},
			},
			expectedError: false,
			description:   "有筛选条件时，应该先通过标签查询获取ID，再查询模板详情",
		},
		{
			name: "失败 - 标签查询错误",
			request: &repo.ListEvaluatorTemplateRequest{
				SpaceID: 123,
				FilterOption: entity.NewEvaluatorFilterOption().
					WithFilters(
						entity.NewEvaluatorFilters().
							WithLogicOp(entity.FilterLogicOp_And).
							AddCondition(entity.NewEvaluatorFilterCondition(
								entity.EvaluatorTagKey_Category,
								entity.EvaluatorFilterOperatorType_Equal,
								"LLM",
							)),
					),
				PageSize:       10,
				PageNum:        1,
				IncludeDeleted: false,
			},
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO) {
				filterOption := entity.NewEvaluatorFilterOption().
					WithFilters(
						entity.NewEvaluatorFilters().
							WithLogicOp(entity.FilterLogicOp_And).
							AddCondition(entity.NewEvaluatorFilterCondition(
								entity.EvaluatorTagKey_Category,
								entity.EvaluatorFilterOperatorType_Equal,
								"LLM",
							)),
					)
				mockTagDAO.EXPECT().
					GetSourceIDsByFilterConditions(gomock.Any(), int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), filterOption, int32(0), int32(0), gomock.Any()).
					Return(nil, int64(0), errors.New("tag query error"))
			},
			expectedResult: nil,
			expectedError:  true,
			description:    "标签查询出错时，应该返回错误",
		},
		{
			name: "成功 - 筛选条件命中数为0",
			request: &repo.ListEvaluatorTemplateRequest{
				SpaceID: 123,
				FilterOption: entity.NewEvaluatorFilterOption().
					WithFilters(
						entity.NewEvaluatorFilters().
							WithLogicOp(entity.FilterLogicOp_And).
							AddCondition(entity.NewEvaluatorFilterCondition(
								entity.EvaluatorTagKey_Category,
								entity.EvaluatorFilterOperatorType_Equal,
								"NonExistentCategory",
							)),
					),
				PageSize:       10,
				PageNum:        1,
				IncludeDeleted: false,
			},
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO) {
				filterOption := entity.NewEvaluatorFilterOption().
					WithFilters(
						entity.NewEvaluatorFilters().
							WithLogicOp(entity.FilterLogicOp_And).
							AddCondition(entity.NewEvaluatorFilterCondition(
								entity.EvaluatorTagKey_Category,
								entity.EvaluatorFilterOperatorType_Equal,
								"NonExistentCategory",
							)),
					)
				mockTagDAO.EXPECT().
					GetSourceIDsByFilterConditions(gomock.Any(), int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), filterOption, int32(0), int32(0), gomock.Any()).
					Return([]int64{}, int64(0), nil)
			},
			expectedResult: &repo.ListEvaluatorTemplateResponse{
				TotalCount: 0,
				Templates:  []*entity.EvaluatorTemplate{},
			},
			expectedError: false,
			description:  "筛选条件命中数为0时，应该直接返回空结果",
		},
		{
			name: "失败 - 模板查询错误",
			request: &repo.ListEvaluatorTemplateRequest{
				SpaceID:        123,
				FilterOption:   nil,
				PageSize:       10,
				PageNum:        1,
				IncludeDeleted: false,
			},
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO) {
				expectedDAOReq := &mysql.ListEvaluatorTemplateRequest{
					IDs:            []int64{},
					PageSize:       10,
					PageNum:        1,
					IncludeDeleted: false,
				}
				mockTemplateDAO.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), expectedDAOReq).
					Return(nil, errors.New("template query error"))
			},
			expectedResult: nil,
			expectedError:  true,
			description:    "模板查询出错时，应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTagDAO := mysqlmocks.NewMockEvaluatorTagDAO(ctrl)
			mockTemplateDAO := mysqlmocks.NewMockEvaluatorTemplateDAO(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)

			tt.mockSetup(mockTagDAO, mockTemplateDAO)

			repo := NewEvaluatorTemplateRepo(mockTagDAO, mockTemplateDAO, mockIDGen)

			ctx := context.Background()
			result, err := repo.ListEvaluatorTemplate(ctx, tt.request)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedResult.TotalCount, result.TotalCount)
				if len(tt.expectedResult.Templates) > 0 {
					assert.Len(t, result.Templates, len(tt.expectedResult.Templates))
					// 验证模板基本属性
					for i, template := range result.Templates {
						expected := tt.expectedResult.Templates[i]
						assert.Equal(t, expected.ID, template.ID)
						assert.Equal(t, expected.SpaceID, template.SpaceID)
						assert.Equal(t, expected.Name, template.Name)
					}
				} else {
					assert.Len(t, result.Templates, 0)
				}
			}
		})
	}
}

// TestEvaluatorTemplateRepoImpl_CreateEvaluatorTemplate 测试创建评估器模板
func TestEvaluatorTemplateRepoImpl_CreateEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		template       *entity.EvaluatorTemplate
		mockSetup      func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO, mockIDGen *idgenmocks.MockIIDGenerator)
		expectedError  bool
		description    string
	}{
		{
			name: "成功 - 创建模板无标签",
			template: &entity.EvaluatorTemplate{
				SpaceID:       100,
				Name:          "Test Template",
				Description:   "Test Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO, mockIDGen *idgenmocks.MockIIDGenerator) {
				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 1).
					Return([]int64{1}, nil)

				mockTemplateDAO.EXPECT().
					CreateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, template *model.EvaluatorTemplate, opts ...interface{}) (*model.EvaluatorTemplate, error) {
						template.ID = 1
						return template, nil
					})
			},
			expectedError: false,
			description:  "成功创建评估器模板（无标签）",
		},
		{
			name: "成功 - 创建模板带标签",
			template: &entity.EvaluatorTemplate{
				SpaceID:       100,
				Name:          "Test Template",
				Description:   "Test Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
				Tags: map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string{
					entity.EvaluatorTagLangType_Zh: {
						entity.EvaluatorTagKey_Category: {"category1", "category2"},
					},
				},
			},
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO, mockIDGen *idgenmocks.MockIIDGenerator) {
				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 1).
					Return([]int64{1}, nil)

				mockTemplateDAO.EXPECT().
					CreateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, template *model.EvaluatorTemplate, opts ...interface{}) (*model.EvaluatorTemplate, error) {
						template.ID = 1
						return template, nil
					})

				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 2).
					Return([]int64{10, 11}, nil)

				mockTagDAO.EXPECT().
					BatchCreateEvaluatorTags(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedError: false,
			description:  "成功创建评估器模板（带标签）",
		},
		{
			name: "失败 - nil模板",
			template: nil,
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO, mockIDGen *idgenmocks.MockIIDGenerator) {
				// 不设置任何mock期望
			},
			expectedError: true,
			description:  "传入nil模板应该返回错误",
		},
		{
			name: "失败 - ID生成错误",
			template: &entity.EvaluatorTemplate{
				SpaceID:       100,
				Name:          "Test Template",
				Description:   "Test Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO, mockIDGen *idgenmocks.MockIIDGenerator) {
				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 1).
					Return(nil, errors.New("id generation error"))
			},
			expectedError: true,
			description:  "ID生成失败应该返回错误",
		},
		{
			name: "失败 - 模板创建错误",
			template: &entity.EvaluatorTemplate{
				SpaceID:       100,
				Name:          "Test Template",
				Description:   "Test Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO, mockIDGen *idgenmocks.MockIIDGenerator) {
				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 1).
					Return([]int64{1}, nil)

				mockTemplateDAO.EXPECT().
					CreateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("template creation error"))
			},
			expectedError: true,
			description:  "模板创建失败应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTagDAO := mysqlmocks.NewMockEvaluatorTagDAO(ctrl)
			mockTemplateDAO := mysqlmocks.NewMockEvaluatorTemplateDAO(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)

			tt.mockSetup(mockTagDAO, mockTemplateDAO, mockIDGen)

			repo := NewEvaluatorTemplateRepo(mockTagDAO, mockTemplateDAO, mockIDGen)

			ctx := session.WithCtxUser(context.Background(), &session.User{ID: "user123"})
			result, err := repo.CreateEvaluatorTemplate(ctx, tt.template)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.template.SpaceID, result.SpaceID)
				assert.Equal(t, tt.template.Name, result.Name)
				assert.Equal(t, tt.template.Description, result.Description)
				assert.Equal(t, tt.template.EvaluatorType, result.EvaluatorType)
			}
		})
	}
}

// TestEvaluatorTemplateRepoImpl_UpdateEvaluatorTemplate 测试更新评估器模板
func TestEvaluatorTemplateRepoImpl_UpdateEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		template       *entity.EvaluatorTemplate
		mockSetup      func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO, mockIDGen *idgenmocks.MockIIDGenerator)
		expectedError  bool
		description    string
	}{
		{
			name: "成功 - 更新模板无标签",
			template: &entity.EvaluatorTemplate{
				ID:            1,
				SpaceID:       100,
				Name:          "Updated Template",
				Description:   "Updated Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
				Tags:          nil,
			},
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO, mockIDGen *idgenmocks.MockIIDGenerator) {
				mockTemplateDAO.EXPECT().
					UpdateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, template *model.EvaluatorTemplate, opts ...interface{}) (*model.EvaluatorTemplate, error) {
						return template, nil
					})
			},
			expectedError: false,
			description:  "成功更新评估器模板（无标签）",
		},
		{
			name: "成功 - 更新模板标签对齐",
			template: &entity.EvaluatorTemplate{
				ID:            1,
				SpaceID:       100,
				Name:          "Updated Template",
				Description:   "Updated Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
				Tags: map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string{
					entity.EvaluatorTagLangType_Zh: {
						entity.EvaluatorTagKey_Category: {"category1"},
					},
				},
			},
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO, mockIDGen *idgenmocks.MockIIDGenerator) {
				mockTemplateDAO.EXPECT().
					UpdateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, template *model.EvaluatorTemplate, opts ...interface{}) (*model.EvaluatorTemplate, error) {
						return template, nil
					})

				// 获取现有标签
				mockTagDAO.EXPECT().
					BatchGetTagsBySourceIDsAndType(gomock.Any(), []int64{1}, int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), string(entity.EvaluatorTagLangType_Zh)).
					Return([]*model.EvaluatorTag{
						{
							ID:       10,
							SourceID: 1,
							TagKey:   string(entity.EvaluatorTagKey_Category),
							TagValue: "category2",
							LangType: string(entity.EvaluatorTagLangType_Zh),
						},
					}, nil)

				// 删除不需要的标签
				mockTagDAO.EXPECT().
					DeleteEvaluatorTagsByConditions(gomock.Any(), int64(1), int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), string(entity.EvaluatorTagLangType_Zh), gomock.Any()).
					Return(nil)

				// 添加新标签
				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 1).
					Return([]int64{20}, nil)

				mockTagDAO.EXPECT().
					BatchCreateEvaluatorTags(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedError: false,
			description:  "成功更新评估器模板（标签对齐）",
		},
		{
			name: "失败 - nil模板",
			template: nil,
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO, mockIDGen *idgenmocks.MockIIDGenerator) {
				// 不设置任何mock期望
			},
			expectedError: true,
			description:  "传入nil模板应该返回错误",
		},
		{
			name: "失败 - 模板更新错误",
			template: &entity.EvaluatorTemplate{
				ID:            1,
				SpaceID:       100,
				Name:          "Updated Template",
				Description:   "Updated Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			mockSetup: func(mockTagDAO *mysqlmocks.MockEvaluatorTagDAO, mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO, mockIDGen *idgenmocks.MockIIDGenerator) {
				mockTemplateDAO.EXPECT().
					UpdateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("template update error"))
			},
			expectedError: true,
			description:  "模板更新失败应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTagDAO := mysqlmocks.NewMockEvaluatorTagDAO(ctrl)
			mockTemplateDAO := mysqlmocks.NewMockEvaluatorTemplateDAO(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)

			tt.mockSetup(mockTagDAO, mockTemplateDAO, mockIDGen)

			repo := NewEvaluatorTemplateRepo(mockTagDAO, mockTemplateDAO, mockIDGen)

			ctx := session.WithCtxUser(context.Background(), &session.User{ID: "user123"})
			result, err := repo.UpdateEvaluatorTemplate(ctx, tt.template)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.template.ID, result.ID)
				assert.Equal(t, tt.template.Name, result.Name)
			}
		})
	}
}

// TestEvaluatorTemplateRepoImpl_DeleteEvaluatorTemplate 测试删除评估器模板
func TestEvaluatorTemplateRepoImpl_DeleteEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		id            int64
		userID        string
		mockSetup     func(mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO)
		expectedError bool
		description   string
	}{
		{
			name:   "成功 - 删除模板",
			id:     1,
			userID: "user123",
			mockSetup: func(mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO) {
				mockTemplateDAO.EXPECT().
					DeleteEvaluatorTemplate(gomock.Any(), int64(1), "user123").
					Return(nil)
			},
			expectedError: false,
			description:  "成功删除评估器模板",
		},
		{
			name:   "失败 - 删除错误",
			id:     1,
			userID: "user123",
			mockSetup: func(mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO) {
				mockTemplateDAO.EXPECT().
					DeleteEvaluatorTemplate(gomock.Any(), int64(1), "user123").
					Return(errors.New("database error"))
			},
			expectedError: true,
			description:  "删除时发生数据库错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTagDAO := mysqlmocks.NewMockEvaluatorTagDAO(ctrl)
			mockTemplateDAO := mysqlmocks.NewMockEvaluatorTemplateDAO(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)

			tt.mockSetup(mockTemplateDAO)

			repo := NewEvaluatorTemplateRepo(mockTagDAO, mockTemplateDAO, mockIDGen)

			err := repo.DeleteEvaluatorTemplate(context.Background(), tt.id, tt.userID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEvaluatorTemplateRepoImpl_GetEvaluatorTemplate 测试获取评估器模板
func TestEvaluatorTemplateRepoImpl_GetEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		id             int64
		includeDeleted bool
		mockSetup      func(mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO)
		expectedError  bool
		description    string
	}{
		{
			name:           "成功 - 获取模板",
			id:             1,
			includeDeleted: false,
			mockSetup: func(mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO) {
				mockTemplateDAO.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(&model.EvaluatorTemplate{
						ID:            1,
						SpaceID:       gptr.Of(int64(100)),
						Name:          gptr.Of("Test Template"),
						Description:   gptr.Of("Test Description"),
						EvaluatorType: gptr.Of(int32(1)),
					}, nil)
			},
			expectedError: false,
			description:  "成功获取评估器模板",
		},
		{
			name:           "成功 - 模板不存在",
			id:             1,
			includeDeleted: false,
			mockSetup: func(mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO) {
				mockTemplateDAO.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(nil, nil)
			},
			expectedError: false,
			description:  "模板不存在时返回nil",
		},
		{
			name:           "失败 - 数据库错误",
			id:             1,
			includeDeleted: false,
			mockSetup: func(mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO) {
				mockTemplateDAO.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(nil, errors.New("database error"))
			},
			expectedError: true,
			description:  "数据库查询错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTagDAO := mysqlmocks.NewMockEvaluatorTagDAO(ctrl)
			mockTemplateDAO := mysqlmocks.NewMockEvaluatorTemplateDAO(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)

			tt.mockSetup(mockTemplateDAO)

			repo := NewEvaluatorTemplateRepo(mockTagDAO, mockTemplateDAO, mockIDGen)

			result, err := repo.GetEvaluatorTemplate(context.Background(), tt.id, tt.includeDeleted)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				if result != nil {
					assert.Equal(t, tt.id, result.ID)
				}
			}
		})
	}
}

// TestEvaluatorTemplateRepoImpl_IncrPopularityByID 测试增加模板热度
func TestEvaluatorTemplateRepoImpl_IncrPopularityByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		id            int64
		mockSetup     func(mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO)
		expectedError bool
		description   string
	}{
		{
			name: "成功 - 增加热度",
			id:   1,
			mockSetup: func(mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO) {
				mockTemplateDAO.EXPECT().
					IncrPopularityByID(gomock.Any(), int64(1)).
					Return(nil)
			},
			expectedError: false,
			description:  "成功增加模板热度",
		},
		{
			name: "失败 - 数据库错误",
			id:   1,
			mockSetup: func(mockTemplateDAO *mysqlmocks.MockEvaluatorTemplateDAO) {
				mockTemplateDAO.EXPECT().
					IncrPopularityByID(gomock.Any(), int64(1)).
					Return(errors.New("database error"))
			},
			expectedError: true,
			description:  "数据库更新错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTagDAO := mysqlmocks.NewMockEvaluatorTagDAO(ctrl)
			mockTemplateDAO := mysqlmocks.NewMockEvaluatorTemplateDAO(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)

			tt.mockSetup(mockTemplateDAO)

			repo := NewEvaluatorTemplateRepo(mockTagDAO, mockTemplateDAO, mockIDGen)

			err := repo.IncrPopularityByID(context.Background(), tt.id)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEvaluatorTemplateRepoImpl_setTemplateTags 测试 setTemplateTags 方法
func TestEvaluatorTemplateRepoImpl_setTemplateTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		template       *entity.EvaluatorTemplate
		templateID     int64
		tagsBySourceID map[int64][]*model.EvaluatorTag
		expectedTags   map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string
		description    string
	}{
		{
			name: "成功 - tagsBySourceID中不存在该templateID",
			template: &entity.EvaluatorTemplate{
				ID:   1,
				Name: "Test Template",
			},
			templateID:     1,
			tagsBySourceID: map[int64][]*model.EvaluatorTag{},
			expectedTags:   nil,
			description:    "当tagsBySourceID中不存在该templateID时，Tags应该保持为nil",
		},
		{
			name: "成功 - tagsBySourceID[templateID]存在但为空数组",
			template: &entity.EvaluatorTemplate{
				ID:   1,
				Name: "Test Template",
			},
			templateID: 1,
			tagsBySourceID: map[int64][]*model.EvaluatorTag{
				1: {},
			},
			expectedTags: nil,
			description:  "当tagsBySourceID[templateID]存在但为空数组时，Tags应该保持为nil",
		},
		{
			name: "成功 - 单个标签，模板Tags为nil",
			template: &entity.EvaluatorTemplate{
				ID:   1,
				Name: "Test Template",
			},
			templateID: 1,
			tagsBySourceID: map[int64][]*model.EvaluatorTag{
				1: {
					{
						ID:       10,
						SourceID: 1,
						TagKey:   string(entity.EvaluatorTagKey_Category),
						TagValue: "LLM",
						LangType: string(entity.EvaluatorTagLangType_Zh),
					},
				},
			},
			expectedTags: map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string{
				entity.EvaluatorTagLangType_Zh: {
					entity.EvaluatorTagKey_Category: {"LLM"},
				},
			},
			description: "当有单个标签且模板Tags为nil时，应该初始化Tags并设置标签",
		},
		{
			name: "成功 - 多个标签不同TagKey，模板Tags为nil",
			template: &entity.EvaluatorTemplate{
				ID:   1,
				Name: "Test Template",
			},
			templateID: 1,
			tagsBySourceID: map[int64][]*model.EvaluatorTag{
				1: {
					{
						ID:       10,
						SourceID: 1,
						TagKey:   string(entity.EvaluatorTagKey_Category),
						TagValue: "LLM",
						LangType: string(entity.EvaluatorTagLangType_Zh),
					},
					{
						ID:       11,
						SourceID: 1,
						TagKey:   string(entity.EvaluatorTagKey_BusinessScenario),
						TagValue: "安全风控",
						LangType: string(entity.EvaluatorTagLangType_Zh),
					},
				},
			},
			expectedTags: map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string{
				entity.EvaluatorTagLangType_Zh: {
					entity.EvaluatorTagKey_Category:         {"LLM"},
					entity.EvaluatorTagKey_BusinessScenario: {"安全风控"},
				},
			},
			description: "当有多个不同TagKey的标签且模板Tags为nil时，应该按TagKey分组设置",
		},
		{
			name: "成功 - 多个标签相同TagKey，模板Tags为nil",
			template: &entity.EvaluatorTemplate{
				ID:   1,
				Name: "Test Template",
			},
			templateID: 1,
			tagsBySourceID: map[int64][]*model.EvaluatorTag{
				1: {
					{
						ID:       10,
						SourceID: 1,
						TagKey:   string(entity.EvaluatorTagKey_Category),
						TagValue: "LLM",
						LangType: string(entity.EvaluatorTagLangType_Zh),
					},
					{
						ID:       11,
						SourceID: 1,
						TagKey:   string(entity.EvaluatorTagKey_Category),
						TagValue: "NLP",
						LangType: string(entity.EvaluatorTagLangType_Zh),
					},
				},
			},
			expectedTags: map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string{
				entity.EvaluatorTagLangType_Zh: {
					entity.EvaluatorTagKey_Category: {"LLM", "NLP"},
				},
			},
			description: "当有多个相同TagKey的标签时，应该合并到同一个slice中",
		},
		{
			name: "成功 - 标签设置覆盖模板已有的Tags",
			template: &entity.EvaluatorTemplate{
				ID:   1,
				Name: "Test Template",
				Tags: map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string{
					entity.EvaluatorTagLangType_En: {
						entity.EvaluatorTagKey_Category: {"OldCategory"},
					},
				},
			},
			templateID: 1,
			tagsBySourceID: map[int64][]*model.EvaluatorTag{
				1: {
					{
						ID:       10,
						SourceID: 1,
						TagKey:   string(entity.EvaluatorTagKey_Category),
						TagValue: "LLM",
						LangType: string(entity.EvaluatorTagLangType_Zh),
					},
				},
			},
			expectedTags: map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string{
				entity.EvaluatorTagLangType_En: {
					entity.EvaluatorTagKey_Category: {"OldCategory"},
				},
				entity.EvaluatorTagLangType_Zh: {
					entity.EvaluatorTagKey_Category: {"LLM"},
				},
			},
			description: "当模板已有Tags时，新标签应该添加到对应语言下，不会覆盖其他语言",
		},
		{
			name: "成功 - 混合场景：多个标签不同TagKey，部分相同TagKey",
			template: &entity.EvaluatorTemplate{
				ID:   1,
				Name: "Test Template",
			},
			templateID: 1,
			tagsBySourceID: map[int64][]*model.EvaluatorTag{
				1: {
					{
						ID:       10,
						SourceID: 1,
						TagKey:   string(entity.EvaluatorTagKey_Category),
						TagValue: "LLM",
						LangType: string(entity.EvaluatorTagLangType_Zh),
					},
					{
						ID:       11,
						SourceID: 1,
						TagKey:   string(entity.EvaluatorTagKey_Category),
						TagValue: "NLP",
						LangType: string(entity.EvaluatorTagLangType_Zh),
					},
					{
						ID:       12,
						SourceID: 1,
						TagKey:   string(entity.EvaluatorTagKey_BusinessScenario),
						TagValue: "安全风控",
						LangType: string(entity.EvaluatorTagLangType_Zh),
					},
					{
						ID:       13,
						SourceID: 1,
						TagKey:   string(entity.EvaluatorTagKey_BusinessScenario),
						TagValue: "内容审核",
						LangType: string(entity.EvaluatorTagLangType_Zh),
					},
				},
			},
			expectedTags: map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string{
				entity.EvaluatorTagLangType_Zh: {
					entity.EvaluatorTagKey_Category:         {"LLM", "NLP"},
					entity.EvaluatorTagKey_BusinessScenario: {"安全风控", "内容审核"},
				},
			},
			description: "混合场景：多个不同TagKey的标签，每个TagKey有多个值，应该正确分组和合并",
		},
		{
			name: "成功 - 不同templateID的标签不会影响",
			template: &entity.EvaluatorTemplate{
				ID:   1,
				Name: "Test Template",
			},
			templateID: 1,
			tagsBySourceID: map[int64][]*model.EvaluatorTag{
				1: {
					{
						ID:       10,
						SourceID: 1,
						TagKey:   string(entity.EvaluatorTagKey_Category),
						TagValue: "LLM",
						LangType: string(entity.EvaluatorTagLangType_Zh),
					},
				},
				2: {
					{
						ID:       20,
						SourceID: 2,
						TagKey:   string(entity.EvaluatorTagKey_Category),
						TagValue: "OtherCategory",
						LangType: string(entity.EvaluatorTagLangType_Zh),
					},
				},
			},
			expectedTags: map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string{
				entity.EvaluatorTagLangType_Zh: {
					entity.EvaluatorTagKey_Category: {"LLM"},
				},
			},
			description: "不同templateID的标签不应该影响当前模板",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockTagDAO := mysqlmocks.NewMockEvaluatorTagDAO(ctrl)
			mockTemplateDAO := mysqlmocks.NewMockEvaluatorTemplateDAO(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)

			repo := NewEvaluatorTemplateRepo(mockTagDAO, mockTemplateDAO, mockIDGen).(*EvaluatorTemplateRepoImpl)

			// 调用 setTemplateTags 方法
			repo.setTemplateTags(tt.template, tt.templateID, tt.tagsBySourceID)

			// 验证结果
			if tt.expectedTags == nil {
				assert.Nil(t, tt.template.Tags, tt.description)
			} else {
				assert.NotNil(t, tt.template.Tags, tt.description)
				assert.Equal(t, len(tt.expectedTags), len(tt.template.Tags), tt.description)
				for lang, expectedTagMap := range tt.expectedTags {
					actualTagMap, exists := tt.template.Tags[lang]
					assert.True(t, exists, tt.description+" - 语言 %s 应该存在", lang)
					assert.Equal(t, len(expectedTagMap), len(actualTagMap), tt.description+" - 语言 %s 的TagKey数量应该相等", lang)
					for tagKey, expectedValues := range expectedTagMap {
						actualValues, exists := actualTagMap[tagKey]
						assert.True(t, exists, tt.description+" - TagKey %s 应该存在", tagKey)
						assert.Equal(t, expectedValues, actualValues, tt.description+" - TagKey %s 的值应该相等", tagKey)
					}
				}
			}
		})
	}
}
