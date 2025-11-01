// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
)

// MockEvaluatorTagDAO 模拟标签DAO
type MockEvaluatorTagDAO struct {
	mock.Mock
}

func (m *MockEvaluatorTagDAO) GetSourceIDsByFilterConditions(ctx context.Context, tagType int32, filterOption *entity.EvaluatorFilterOption, pageSize, pageNum int32, langType string, opts ...db.Option) ([]int64, int64, error) {
	args := m.Called(ctx, tagType, filterOption, pageSize, pageNum, langType, opts)
	return args.Get(0).([]int64), args.Get(1).(int64), args.Error(2)
}

func (m *MockEvaluatorTagDAO) BatchCreateEvaluatorTags(ctx context.Context, evaluatorTags []*model.EvaluatorTag, opts ...db.Option) error {
	args := m.Called(ctx, evaluatorTags, opts)
	return args.Error(0)
}

func (m *MockEvaluatorTagDAO) DeleteEvaluatorTagsByConditions(ctx context.Context, sourceID int64, tagType int32, langType string, tags map[string][]string, opts ...db.Option) error {
	args := m.Called(ctx, sourceID, tagType, langType, tags, opts)
	return args.Error(0)
}

func (m *MockEvaluatorTagDAO) BatchGetTagsBySourceIDsAndType(ctx context.Context, sourceIDs []int64, tagType int32, langType string, opts ...db.Option) ([]*model.EvaluatorTag, error) {
	args := m.Called(ctx, sourceIDs, tagType, langType, opts)
	return args.Get(0).([]*model.EvaluatorTag), args.Error(1)
}

// stubIDGen 为测试提供简单的自增ID生成器
type stubIDGen struct{ cur int64 }

func (s *stubIDGen) GenID(ctx context.Context) (int64, error) {
	s.cur++
	return s.cur, nil
}

func (s *stubIDGen) GenMultiIDs(ctx context.Context, counts int) ([]int64, error) {
	ids := make([]int64, counts)
	for i := 0; i < counts; i++ {
		s.cur++
		ids[i] = s.cur
	}
	return ids, nil
}

// MockEvaluatorTemplateDAO 模拟模板DAO
type MockEvaluatorTemplateDAO struct {
	mock.Mock
}

func (m *MockEvaluatorTemplateDAO) CreateEvaluatorTemplate(ctx context.Context, template *model.EvaluatorTemplate, opts ...db.Option) (*model.EvaluatorTemplate, error) {
	args := m.Called(ctx, template, opts)
	return args.Get(0).(*model.EvaluatorTemplate), args.Error(1)
}

func (m *MockEvaluatorTemplateDAO) UpdateEvaluatorTemplate(ctx context.Context, template *model.EvaluatorTemplate, opts ...db.Option) (*model.EvaluatorTemplate, error) {
	args := m.Called(ctx, template, opts)
	return args.Get(0).(*model.EvaluatorTemplate), args.Error(1)
}

func (m *MockEvaluatorTemplateDAO) DeleteEvaluatorTemplate(ctx context.Context, id int64, userID string, opts ...db.Option) error {
	args := m.Called(ctx, id, userID, opts)
	return args.Error(0)
}

func (m *MockEvaluatorTemplateDAO) GetEvaluatorTemplate(ctx context.Context, id int64, includeDeleted bool, opts ...db.Option) (*model.EvaluatorTemplate, error) {
	args := m.Called(ctx, id, includeDeleted, opts)
	return args.Get(0).(*model.EvaluatorTemplate), args.Error(1)
}

func (m *MockEvaluatorTemplateDAO) ListEvaluatorTemplate(ctx context.Context, req *mysql.ListEvaluatorTemplateRequest, opts ...db.Option) (*mysql.ListEvaluatorTemplateResponse, error) {
	args := m.Called(ctx, req, opts)
	return args.Get(0).(*mysql.ListEvaluatorTemplateResponse), args.Error(1)
}

func (m *MockEvaluatorTemplateDAO) IncrPopularityByID(ctx context.Context, id int64, opts ...db.Option) error {
	args := m.Called(ctx, id, opts)
	return args.Error(0)
}

func TestEvaluatorTemplateRepoImpl_ListEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		request           *repo.ListEvaluatorTemplateRequest
		mockTagIDs        []int64
		mockTagError      error
		mockTemplates     *mysql.ListEvaluatorTemplateResponse
		mockTemplateError error
		expectedResult    *repo.ListEvaluatorTemplateResponse
		expectedError     bool
		description       string
	}{
		{
			name: "no filter conditions",
			request: &repo.ListEvaluatorTemplateRequest{
				SpaceID:        123,
				FilterOption:   nil,
				PageSize:       10,
				PageNum:        1,
				IncludeDeleted: false,
			},
			mockTemplates: &mysql.ListEvaluatorTemplateResponse{
				TotalCount: 2,
				Templates: []*model.EvaluatorTemplate{
					{
						ID:            1,
						SpaceID:       gptr.Of(int64(123)),
						Name:          gptr.Of("Template A"),
						Description:   gptr.Of("Description A"),
						EvaluatorType: gptr.Of(int32(1)),
						Benchmark:     gptr.Of("benchmark1"),
						Vendor:        gptr.Of("vendor1"),
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
						Benchmark:     gptr.Of("benchmark2"),
						Vendor:        gptr.Of("vendor2"),
						Popularity:    200,
						CreatedBy:     "user2",
						UpdatedBy:     "user2",
					},
				},
			},
			expectedResult: &repo.ListEvaluatorTemplateResponse{
				TotalCount: 2,
				Templates: []*entity.EvaluatorTemplate{
					{
						ID:            1,
						SpaceID:       123,
						Name:          "Template A",
						Description:   "Description A",
						EvaluatorType: entity.EvaluatorType(1),
						Benchmark:     "benchmark1",
						Vendor:        "vendor1",
						Popularity:    100,
						BaseInfo: &entity.BaseInfo{
							CreatedBy: &entity.UserInfo{
								UserID: gptr.Of("user1"),
							},
							UpdatedBy: &entity.UserInfo{
								UserID: gptr.Of("user1"),
							},
						},
					},
					{
						ID:            2,
						SpaceID:       123,
						Name:          "Template B",
						Description:   "Description B",
						EvaluatorType: entity.EvaluatorType(2),
						Benchmark:     "benchmark2",
						Vendor:        "vendor2",
						Popularity:    200,
						BaseInfo: &entity.BaseInfo{
							CreatedBy: &entity.UserInfo{
								UserID: gptr.Of("user2"),
							},
							UpdatedBy: &entity.UserInfo{
								UserID: gptr.Of("user2"),
							},
						},
					},
				},
			},
			expectedError: false,
			description:   "无筛选条件时，应该直接查询所有模板",
		},
		{
			name: "with filter conditions",
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
			mockTagIDs: []int64{1, 3},
			mockTemplates: &mysql.ListEvaluatorTemplateResponse{
				TotalCount: 1,
				Templates: []*model.EvaluatorTemplate{
					{
						ID:            1,
						SpaceID:       gptr.Of(int64(123)),
						Name:          gptr.Of("Template A"),
						Description:   gptr.Of("Description A"),
						EvaluatorType: gptr.Of(int32(1)),
						Benchmark:     gptr.Of("benchmark1"),
						Vendor:        gptr.Of("vendor1"),
						Popularity:    100,
						CreatedBy:     "user1",
						UpdatedBy:     "user1",
					},
				},
			},
			expectedResult: &repo.ListEvaluatorTemplateResponse{
				TotalCount: 1,
				Templates: []*entity.EvaluatorTemplate{
					{
						ID:            1,
						SpaceID:       123,
						Name:          "Template A",
						Description:   "Description A",
						EvaluatorType: entity.EvaluatorType(1),
						Benchmark:     "benchmark1",
						Vendor:        "vendor1",
						Popularity:    100,
						BaseInfo: &entity.BaseInfo{
							CreatedBy: &entity.UserInfo{
								UserID: gptr.Of("user1"),
							},
							UpdatedBy: &entity.UserInfo{
								UserID: gptr.Of("user1"),
							},
						},
					},
				},
			},
			expectedError: false,
			description:   "有筛选条件时，应该先通过标签查询获取ID，再查询模板详情",
		},
		{
			name: "tag query error",
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
			mockTagError:  assert.AnError,
			expectedError: true,
			description:   "标签查询出错时，应该返回错误",
		},
		{
			name: "template query error",
			request: &repo.ListEvaluatorTemplateRequest{
				SpaceID:        123,
				FilterOption:   nil,
				PageSize:       10,
				PageNum:        1,
				IncludeDeleted: false,
			},
			mockTemplateError: assert.AnError,
			expectedError:     true,
			description:       "模板查询出错时，应该返回错误",
		},
		{
			name: "empty search keyword",
			request: &repo.ListEvaluatorTemplateRequest{
				SpaceID: 123,
				FilterOption: entity.NewEvaluatorFilterOption().
					WithSearchKeyword(""),
				PageSize:       10,
				PageNum:        1,
				IncludeDeleted: false,
			},
			mockTemplates: &mysql.ListEvaluatorTemplateResponse{
				TotalCount: 2,
				Templates: []*model.EvaluatorTemplate{
					{
						ID:            1,
						SpaceID:       gptr.Of(int64(123)),
						Name:          gptr.Of("Template A"),
						Description:   gptr.Of("Description A"),
						EvaluatorType: gptr.Of(int32(1)),
						Benchmark:     gptr.Of("benchmark1"),
						Vendor:        gptr.Of("vendor1"),
						Popularity:    100,
						CreatedBy:     "user1",
						UpdatedBy:     "user1",
					},
				},
			},
			expectedResult: &repo.ListEvaluatorTemplateResponse{
				TotalCount: 2,
				Templates: []*entity.EvaluatorTemplate{
					{
						ID:            1,
						SpaceID:       123,
						Name:          "Template A",
						Description:   "Description A",
						EvaluatorType: entity.EvaluatorType(1),
						Benchmark:     "benchmark1",
						Vendor:        "vendor1",
						Popularity:    100,
						BaseInfo: &entity.BaseInfo{
							CreatedBy: &entity.UserInfo{
								UserID: gptr.Of("user1"),
							},
							UpdatedBy: &entity.UserInfo{
								UserID: gptr.Of("user1"),
							},
						},
					},
				},
			},
			expectedError: false,
			description:   "空搜索关键词时，应该忽略筛选条件，查询所有模板",
		},
		{
			name: "filter conditions hit zero results",
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
			mockTagIDs: []int64{}, // 空结果
			expectedResult: &repo.ListEvaluatorTemplateResponse{
				TotalCount: 0,
				Templates:  []*entity.EvaluatorTemplate{},
			},
			expectedError: false,
			description:   "筛选条件命中数为0时，应该直接返回空结果",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// 创建mock对象
			mockTagDAO := &MockEvaluatorTagDAO{}
			mockTemplateDAO := &MockEvaluatorTemplateDAO{}

			// 设置mock期望
			hasValidFilters := false
			if tt.request.FilterOption != nil {
				// 检查SearchKeyword是否有效
				if tt.request.FilterOption.SearchKeyword != nil && *tt.request.FilterOption.SearchKeyword != "" {
					hasValidFilters = true
				}
				// 检查FilterConditions是否有效
				if tt.request.FilterOption.Filters != nil && len(tt.request.FilterOption.Filters.FilterConditions) > 0 {
					hasValidFilters = true
				}
			}

			if hasValidFilters {
				mockTagDAO.On("GetSourceIDsByFilterConditions", mock.Anything, int32(2), tt.request.FilterOption, int32(0), int32(0), mock.Anything, mock.Anything).Return(tt.mockTagIDs, int64(len(tt.mockTagIDs)), tt.mockTagError)
			}

			// Set up mock for BatchGetTagsBySourceIDsAndType - this is always called when there are template IDs
			if tt.mockTemplates != nil && len(tt.mockTemplates.Templates) > 0 {
				templateIDs := make([]int64, len(tt.mockTemplates.Templates))
				for i, template := range tt.mockTemplates.Templates {
					templateIDs[i] = template.ID
				}
				mockTagDAO.On("BatchGetTagsBySourceIDsAndType", mock.Anything, templateIDs, int32(2), "en-US", mock.Anything).Return([]*model.EvaluatorTag{}, nil)
			}

			// 设置templateDAO的期望
			// 只有在tag查询有错误或筛选条件命中数为0时才不设置templateDAO的期望
			if tt.mockTagError == nil && !(hasValidFilters && len(tt.mockTagIDs) == 0) {
				// 如果有筛选条件，templateDAO应该被调用时传入筛选后的IDs
				// 如果没有筛选条件，templateDAO应该被调用时传入空的IDs
				expectedIDs := []int64{}
				if hasValidFilters {
					expectedIDs = tt.mockTagIDs
				}
				// 确保不是nil
				if expectedIDs == nil {
					expectedIDs = []int64{}
				}

				expectedDAOReq := &mysql.ListEvaluatorTemplateRequest{
					IDs:            expectedIDs,
					PageSize:       tt.request.PageSize,
					PageNum:        tt.request.PageNum,
					IncludeDeleted: tt.request.IncludeDeleted,
				}
				mockTemplateDAO.On("ListEvaluatorTemplate", mock.Anything, expectedDAOReq, mock.Anything).Return(tt.mockTemplates, tt.mockTemplateError)
			}

			// 创建repo实例
			repo := NewEvaluatorTemplateRepo(mockTagDAO, mockTemplateDAO, &stubIDGen{})

			// 执行测试
			ctx := context.Background()
			result, err := repo.ListEvaluatorTemplate(ctx, tt.request)

			// 验证结果
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult.TotalCount, result.TotalCount)
				assert.Len(t, result.Templates, len(tt.expectedResult.Templates))

				// 验证模板内容
				for i, template := range result.Templates {
					expected := tt.expectedResult.Templates[i]
					assert.Equal(t, expected.ID, template.ID)
					assert.Equal(t, expected.SpaceID, template.SpaceID)
					assert.Equal(t, expected.Name, template.Name)
					assert.Equal(t, expected.Description, template.Description)
					assert.Equal(t, expected.EvaluatorType, template.EvaluatorType)
					assert.Equal(t, expected.Benchmark, template.Benchmark)
					assert.Equal(t, expected.Vendor, template.Vendor)
					assert.Equal(t, expected.Popularity, template.Popularity)
					assert.NotNil(t, template.BaseInfo)
					assert.NotNil(t, template.BaseInfo.CreatedBy)
					assert.NotNil(t, template.BaseInfo.UpdatedBy)
				}
			}

			// 验证mock调用
			mockTagDAO.AssertExpectations(t)
			mockTemplateDAO.AssertExpectations(t)
		})
	}
}

func TestConvertEvaluatorTemplatePO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		po       *model.EvaluatorTemplate
		expected *entity.EvaluatorTemplate
	}{
		{
			name:     "nil po",
			po:       nil,
			expected: nil,
		},
		{
			name: "valid po",
			po: &model.EvaluatorTemplate{
				ID:            1,
				SpaceID:       gptr.Of(int64(123)),
				Name:          gptr.Of("Test Template"),
				Description:   gptr.Of("Test Description"),
				EvaluatorType: gptr.Of(int32(1)),
				Benchmark:     gptr.Of("test_benchmark"),
				Vendor:        gptr.Of("test_vendor"),
				Popularity:    100,
				CreatedBy:     "user1",
				UpdatedBy:     "user1",
			},
			expected: &entity.EvaluatorTemplate{
				ID:            1,
				SpaceID:       123,
				Name:          "Test Template",
				Description:   "Test Description",
				EvaluatorType: entity.EvaluatorType(1),
				Benchmark:     "test_benchmark",
				Vendor:        "test_vendor",
				Popularity:    100,
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{
						UserID: gptr.Of("user1"),
					},
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("user1"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := convertor.ConvertEvaluatorTemplatePO2DOWithBaseInfo(tt.po)
			assert.NoError(t, err)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.ID, result.ID)
				assert.Equal(t, tt.expected.SpaceID, result.SpaceID)
				assert.Equal(t, tt.expected.Name, result.Name)
				assert.Equal(t, tt.expected.Description, result.Description)
				assert.Equal(t, tt.expected.EvaluatorType, result.EvaluatorType)
				assert.Equal(t, tt.expected.Benchmark, result.Benchmark)
				assert.Equal(t, tt.expected.Vendor, result.Vendor)
				assert.Equal(t, tt.expected.Popularity, result.Popularity)
				assert.NotNil(t, result.BaseInfo)
				assert.NotNil(t, result.BaseInfo.CreatedBy)
				assert.NotNil(t, result.BaseInfo.UpdatedBy)
				assert.Equal(t, tt.expected.BaseInfo.CreatedBy.UserID, result.BaseInfo.CreatedBy.UserID)
				assert.Equal(t, tt.expected.BaseInfo.UpdatedBy.UserID, result.BaseInfo.UpdatedBy.UserID)
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
		mockTemplate   *model.EvaluatorTemplate
		mockError      error
		expectedResult *entity.EvaluatorTemplate
		expectedError  bool
		description    string
	}{
		{
			name: "successful creation",
			template: &entity.EvaluatorTemplate{
				ID:            1,
				SpaceID:       100,
				Name:          "Test Template",
				Description:   "Test Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			mockTemplate: &model.EvaluatorTemplate{
				ID:            1,
				SpaceID:       gptr.Of(int64(100)),
				Name:          gptr.Of("Test Template"),
				Description:   gptr.Of("Test Description"),
				EvaluatorType: gptr.Of(int32(1)),
			},
			expectedResult: &entity.EvaluatorTemplate{
				ID:            1,
				SpaceID:       100,
				Name:          "Test Template",
				Description:   "Test Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			expectedError: false,
			description:   "成功创建评估器模板",
		},
		{
			name:          "nil template",
			template:      nil,
			expectedError: true,
			description:   "传入nil模板应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTemplateDAO := &MockEvaluatorTemplateDAO{}
			mockTagDAO := &MockEvaluatorTagDAO{}

			repo := NewEvaluatorTemplateRepo(mockTagDAO, mockTemplateDAO, &stubIDGen{})

			if tt.template != nil {
				mockTemplateDAO.On("CreateEvaluatorTemplate", mock.Anything, mock.Anything, mock.Anything).Return(tt.mockTemplate, tt.mockError)
			}

			result, err := repo.CreateEvaluatorTemplate(context.Background(), tt.template)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedResult.ID, result.ID)
				assert.Equal(t, tt.expectedResult.SpaceID, result.SpaceID)
				assert.Equal(t, tt.expectedResult.Name, result.Name)
				assert.Equal(t, tt.expectedResult.Description, result.Description)
				assert.Equal(t, tt.expectedResult.EvaluatorType, result.EvaluatorType)
			}

			mockTemplateDAO.AssertExpectations(t)
		})
	}
}

// TestEvaluatorTemplateRepoImpl_UpdateEvaluatorTemplate 测试更新评估器模板
func TestEvaluatorTemplateRepoImpl_UpdateEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		template       *entity.EvaluatorTemplate
		mockTemplate   *model.EvaluatorTemplate
		mockError      error
		expectedResult *entity.EvaluatorTemplate
		expectedError  bool
		description    string
	}{
		{
			name: "successful update",
			template: &entity.EvaluatorTemplate{
				ID:            1,
				SpaceID:       100,
				Name:          "Updated Template",
				Description:   "Updated Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			mockTemplate: &model.EvaluatorTemplate{
				ID:            1,
				SpaceID:       gptr.Of(int64(100)),
				Name:          gptr.Of("Updated Template"),
				Description:   gptr.Of("Updated Description"),
				EvaluatorType: gptr.Of(int32(1)),
			},
			expectedResult: &entity.EvaluatorTemplate{
				ID:            1,
				SpaceID:       100,
				Name:          "Updated Template",
				Description:   "Updated Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			expectedError: false,
			description:   "成功更新评估器模板",
		},
		{
			name:          "nil template",
			template:      nil,
			expectedError: true,
			description:   "传入nil模板应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTemplateDAO := &MockEvaluatorTemplateDAO{}
			mockTagDAO := &MockEvaluatorTagDAO{}

			repo := NewEvaluatorTemplateRepo(mockTagDAO, mockTemplateDAO, &stubIDGen{})

			if tt.template != nil {
				mockTemplateDAO.On("UpdateEvaluatorTemplate", mock.Anything, mock.Anything, mock.Anything).Return(tt.mockTemplate, tt.mockError)
			}

			result, err := repo.UpdateEvaluatorTemplate(context.Background(), tt.template)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedResult.ID, result.ID)
				assert.Equal(t, tt.expectedResult.SpaceID, result.SpaceID)
				assert.Equal(t, tt.expectedResult.Name, result.Name)
				assert.Equal(t, tt.expectedResult.Description, result.Description)
				assert.Equal(t, tt.expectedResult.EvaluatorType, result.EvaluatorType)
			}

			mockTemplateDAO.AssertExpectations(t)
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
		mockError     error
		expectedError bool
		description   string
	}{
		{
			name:          "successful deletion",
			id:            1,
			userID:        "user123",
			expectedError: false,
			description:   "成功删除评估器模板",
		},
		{
			name:          "deletion error",
			id:            1,
			userID:        "user123",
			mockError:     errors.New("database error"),
			expectedError: true,
			description:   "删除时发生数据库错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTemplateDAO := &MockEvaluatorTemplateDAO{}
			mockTagDAO := &MockEvaluatorTagDAO{}

			repo := NewEvaluatorTemplateRepo(mockTagDAO, mockTemplateDAO, &stubIDGen{})

			mockTemplateDAO.On("DeleteEvaluatorTemplate", mock.Anything, tt.id, tt.userID, mock.Anything).Return(tt.mockError)

			err := repo.DeleteEvaluatorTemplate(context.Background(), tt.id, tt.userID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockTemplateDAO.AssertExpectations(t)
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
		mockTemplate   *model.EvaluatorTemplate
		mockError      error
		expectedResult *entity.EvaluatorTemplate
		expectedError  bool
		description    string
	}{
		{
			name:           "successful get",
			id:             1,
			includeDeleted: false,
			mockTemplate: &model.EvaluatorTemplate{
				ID:            1,
				SpaceID:       gptr.Of(int64(100)),
				Name:          gptr.Of("Test Template"),
				Description:   gptr.Of("Test Description"),
				EvaluatorType: gptr.Of(int32(1)),
			},
			expectedResult: &entity.EvaluatorTemplate{
				ID:            1,
				SpaceID:       100,
				Name:          "Test Template",
				Description:   "Test Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			expectedError: false,
			description:   "成功获取评估器模板",
		},
		{
			name:           "template not found",
			id:             1,
			includeDeleted: false,
			mockTemplate:   nil,
			expectedResult: nil,
			expectedError:  false,
			description:    "模板不存在",
		},
		{
			name:           "database error",
			id:             1,
			includeDeleted: false,
			mockError:      errors.New("database error"),
			expectedError:  true,
			description:    "数据库查询错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTemplateDAO := &MockEvaluatorTemplateDAO{}
			mockTagDAO := &MockEvaluatorTagDAO{}

			repo := NewEvaluatorTemplateRepo(mockTagDAO, mockTemplateDAO, &stubIDGen{})

			mockTemplateDAO.On("GetEvaluatorTemplate", mock.Anything, tt.id, tt.includeDeleted, mock.Anything).Return(tt.mockTemplate, tt.mockError)

			result, err := repo.GetEvaluatorTemplate(context.Background(), tt.id, tt.includeDeleted)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				if tt.expectedResult == nil {
					assert.Nil(t, result)
				} else {
					assert.NotNil(t, result)
					assert.Equal(t, tt.expectedResult.ID, result.ID)
					assert.Equal(t, tt.expectedResult.SpaceID, result.SpaceID)
					assert.Equal(t, tt.expectedResult.Name, result.Name)
					assert.Equal(t, tt.expectedResult.Description, result.Description)
					assert.Equal(t, tt.expectedResult.EvaluatorType, result.EvaluatorType)
				}
			}

			mockTemplateDAO.AssertExpectations(t)
		})
	}
}
