// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// MockEvaluatorTemplateRepo 模拟模板仓库
type MockEvaluatorTemplateRepo struct {
	mock.Mock
}

func (m *MockEvaluatorTemplateRepo) CreateEvaluatorTemplate(ctx context.Context, template *entity.EvaluatorTemplate) (*entity.EvaluatorTemplate, error) {
	args := m.Called(ctx, template)
	return args.Get(0).(*entity.EvaluatorTemplate), args.Error(1)
}

func (m *MockEvaluatorTemplateRepo) UpdateEvaluatorTemplate(ctx context.Context, template *entity.EvaluatorTemplate) (*entity.EvaluatorTemplate, error) {
	args := m.Called(ctx, template)
	return args.Get(0).(*entity.EvaluatorTemplate), args.Error(1)
}

func (m *MockEvaluatorTemplateRepo) DeleteEvaluatorTemplate(ctx context.Context, id int64, userID string) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *MockEvaluatorTemplateRepo) GetEvaluatorTemplate(ctx context.Context, id int64, includeDeleted bool) (*entity.EvaluatorTemplate, error) {
	args := m.Called(ctx, id, includeDeleted)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.EvaluatorTemplate), args.Error(1)
}

func (m *MockEvaluatorTemplateRepo) ListEvaluatorTemplate(ctx context.Context, req *repo.ListEvaluatorTemplateRequest) (*repo.ListEvaluatorTemplateResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*repo.ListEvaluatorTemplateResponse), args.Error(1)
}

// TestEvaluatorTemplateServiceImpl_CreateEvaluatorTemplate 测试创建评估器模板
func TestEvaluatorTemplateServiceImpl_CreateEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		req            *CreateEvaluatorTemplateRequest
		mockTemplate   *entity.EvaluatorTemplate
		mockError      error
		expectedError  bool
		description    string
	}{
		{
			name: "successful creation",
			req: &CreateEvaluatorTemplateRequest{
				SpaceID:       100,
				Name:          "Test Template",
				Description:   "Test Description",
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorContent: &entity.PromptEvaluatorContent{
					MessageList: []*entity.Message{
						{
							Content: &entity.Content{
								Text: gptr.Of("Test prompt"),
							},
						},
					},
				},
			},
			mockTemplate: &entity.EvaluatorTemplate{
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
			name: "invalid space ID",
			req: &CreateEvaluatorTemplateRequest{
				SpaceID:       0,
				Name:          "Test Template",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			expectedError: true,
			description:   "无效的空间ID应该返回错误",
		},
		{
			name: "empty name",
			req: &CreateEvaluatorTemplateRequest{
				SpaceID:       100,
				Name:          "",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			expectedError: true,
			description:   "空的模板名称应该返回错误",
		},
		{
			name: "missing prompt content for prompt type",
			req: &CreateEvaluatorTemplateRequest{
				SpaceID:       100,
				Name:          "Test Template",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			expectedError: true,
			description:   "Prompt类型评估器缺少PromptEvaluatorContent应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &MockEvaluatorTemplateRepo{}
			service := NewEvaluatorTemplateService(mockRepo)

			// 设置context with userID
			ctx := context.Background()
			if !tt.expectedError {
				user := &session.User{ID: "user123"}
				ctx = session.WithCtxUser(ctx, user)
				mockRepo.On("CreateEvaluatorTemplate", mock.Anything, mock.Anything).Return(tt.mockTemplate, tt.mockError)
			} else {
				// 对于参数验证失败的情况，不设置userID
				if tt.name != "invalid space ID" && tt.name != "empty name" && tt.name != "missing prompt content for prompt type" {
					user := &session.User{ID: "user123"}
					ctx = session.WithCtxUser(ctx, user)
				}
			}

			result, err := service.CreateEvaluatorTemplate(ctx, tt.req)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.mockTemplate.ID, result.Template.ID)
				assert.Equal(t, tt.mockTemplate.Name, result.Template.Name)
			}

			if !tt.expectedError {
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

// TestEvaluatorTemplateServiceImpl_UpdateEvaluatorTemplate 测试更新评估器模板
func TestEvaluatorTemplateServiceImpl_UpdateEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		req            *UpdateEvaluatorTemplateRequest
		existingTemplate *entity.EvaluatorTemplate
		mockTemplate   *entity.EvaluatorTemplate
		mockError      error
		expectedError  bool
		description    string
	}{
		{
			name: "successful update",
			req: &UpdateEvaluatorTemplateRequest{
				ID:   1,
				Name: gptr.Of("Updated Template"),
			},
			existingTemplate: &entity.EvaluatorTemplate{
				ID:      1,
				SpaceID:  100,
				Name:    "Original Template",
			},
			mockTemplate: &entity.EvaluatorTemplate{
				ID:      1,
				SpaceID: 100,
				Name:    "Updated Template",
			},
			expectedError: false,
			description:   "成功更新评估器模板",
		},
		{
			name: "template not found",
			req: &UpdateEvaluatorTemplateRequest{
				ID:   1,
				Name: gptr.Of("Updated Template"),
			},
			existingTemplate: nil,
			expectedError:   true,
			description:    "模板不存在应该返回错误",
		},
		{
			name: "invalid ID",
			req: &UpdateEvaluatorTemplateRequest{
				ID: 0,
			},
			expectedError: true,
			description:   "无效的模板ID应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &MockEvaluatorTemplateRepo{}
			service := NewEvaluatorTemplateService(mockRepo)

			// 设置context with userID
			ctx := context.Background()
			if tt.req.ID > 0 {
				ctx = session.WithCtxUser(ctx, &session.User{ID: "user123"})
			}

			// 只为会调用repo层的情况设置mock期望
			if tt.req.ID > 0 {
				if tt.existingTemplate != nil {
					mockRepo.On("GetEvaluatorTemplate", mock.Anything, tt.req.ID, false).Return(tt.existingTemplate, nil)
					if !tt.expectedError {
						mockRepo.On("UpdateEvaluatorTemplate", mock.Anything, mock.Anything).Return(tt.mockTemplate, tt.mockError)
					}
				} else if tt.expectedError {
					mockRepo.On("GetEvaluatorTemplate", mock.Anything, tt.req.ID, false).Return(nil, nil)
				}
			}

			result, err := service.UpdateEvaluatorTemplate(ctx, tt.req)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.mockTemplate.ID, result.Template.ID)
				assert.Equal(t, tt.mockTemplate.Name, result.Template.Name)
			}

			if tt.req.ID > 0 {
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

// TestEvaluatorTemplateServiceImpl_DeleteEvaluatorTemplate 测试删除评估器模板
func TestEvaluatorTemplateServiceImpl_DeleteEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		req            *DeleteEvaluatorTemplateRequest
		existingTemplate *entity.EvaluatorTemplate
		mockError      error
		expectedError  bool
		description    string
	}{
		{
			name: "successful deletion",
			req: &DeleteEvaluatorTemplateRequest{
				ID: 1,
			},
			existingTemplate: &entity.EvaluatorTemplate{
				ID:     1,
				SpaceID: 100,
			},
			expectedError: false,
			description:   "成功删除评估器模板",
		},
		{
			name: "template not found",
			req: &DeleteEvaluatorTemplateRequest{
				ID: 1,
			},
			existingTemplate: nil,
			expectedError:   true,
			description:    "模板不存在应该返回错误",
		},
		{
			name: "invalid ID",
			req: &DeleteEvaluatorTemplateRequest{
				ID: 0,
			},
			expectedError: true,
			description:   "无效的模板ID应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &MockEvaluatorTemplateRepo{}
			service := NewEvaluatorTemplateService(mockRepo)

			// 设置context with userID
			ctx := context.Background()
			if tt.req.ID > 0 {
				ctx = session.WithCtxUser(ctx, &session.User{ID: "user123"})
			}

			// 只为会调用repo层的情况设置mock期望
			if tt.req.ID > 0 {
				if tt.existingTemplate != nil {
					mockRepo.On("GetEvaluatorTemplate", mock.Anything, tt.req.ID, false).Return(tt.existingTemplate, nil)
					if !tt.expectedError {
						mockRepo.On("DeleteEvaluatorTemplate", mock.Anything, tt.req.ID, "user123").Return(tt.mockError)
					}
				} else if tt.expectedError {
					mockRepo.On("GetEvaluatorTemplate", mock.Anything, tt.req.ID, false).Return(nil, nil)
				}
			}

			result, err := service.DeleteEvaluatorTemplate(ctx, tt.req)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.Success)
			}

			if tt.req.ID > 0 {
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

// TestEvaluatorTemplateServiceImpl_GetEvaluatorTemplate 测试获取评估器模板
func TestEvaluatorTemplateServiceImpl_GetEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		req            *GetEvaluatorTemplateRequest
		mockTemplate   *entity.EvaluatorTemplate
		mockError      error
		expectedError  bool
		description    string
	}{
		{
			name: "successful get",
			req: &GetEvaluatorTemplateRequest{
				ID: 1,
			},
			mockTemplate: &entity.EvaluatorTemplate{
				ID:      1,
				SpaceID: 100,
				Name:    "Test Template",
			},
			expectedError: false,
			description:   "成功获取评估器模板",
		},
		{
			name: "template not found",
			req: &GetEvaluatorTemplateRequest{
				ID: 1,
			},
			mockTemplate:  nil,
			expectedError: true,
			description:   "模板不存在应该返回错误",
		},
		{
			name: "invalid ID",
			req: &GetEvaluatorTemplateRequest{
				ID: 0,
			},
			expectedError: true,
			description:   "无效的模板ID应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &MockEvaluatorTemplateRepo{}
			service := NewEvaluatorTemplateService(mockRepo)

			// 只为会调用repo层的情况设置mock期望
			if tt.req.ID > 0 {
				mockRepo.On("GetEvaluatorTemplate", mock.Anything, tt.req.ID, tt.req.IncludeDeleted).Return(tt.mockTemplate, tt.mockError)
			}

			result, err := service.GetEvaluatorTemplate(context.Background(), tt.req)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.mockTemplate.ID, result.Template.ID)
				assert.Equal(t, tt.mockTemplate.Name, result.Template.Name)
			}

			if tt.req.ID > 0 {
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

// TestEvaluatorTemplateServiceImpl_ListEvaluatorTemplate 测试查询评估器模板列表
func TestEvaluatorTemplateServiceImpl_ListEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		req            *ListEvaluatorTemplateRequest
		mockResponse   *repo.ListEvaluatorTemplateResponse
		mockError      error
		expectedError  bool
		description    string
	}{
		{
			name: "successful list",
			req: &ListEvaluatorTemplateRequest{
				SpaceID:  100,
				PageSize: 10,
				PageNum:  1,
			},
			mockResponse: &repo.ListEvaluatorTemplateResponse{
				TotalCount: 2,
				Templates: []*entity.EvaluatorTemplate{
					{ID: 1, Name: "Template 1"},
					{ID: 2, Name: "Template 2"},
				},
			},
			expectedError: false,
			description:   "成功查询评估器模板列表",
		},
		{
			name: "invalid space ID",
			req: &ListEvaluatorTemplateRequest{
				SpaceID:  0,
				PageSize: 10,
				PageNum:  1,
			},
			expectedError: true,
			description:   "无效的空间ID应该返回错误",
		},
		{
			name: "invalid page size",
			req: &ListEvaluatorTemplateRequest{
				SpaceID:  100,
				PageSize: 0,
				PageNum:  1,
			},
			expectedError: true,
			description:   "无效的分页大小应该返回错误",
		},
		{
			name: "invalid page num",
			req: &ListEvaluatorTemplateRequest{
				SpaceID:  100,
				PageSize: 10,
				PageNum:  0,
			},
			expectedError: true,
			description:   "无效的页码应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &MockEvaluatorTemplateRepo{}
			service := NewEvaluatorTemplateService(mockRepo)

			if !tt.expectedError {
				mockRepo.On("ListEvaluatorTemplate", mock.Anything, mock.Anything).Return(tt.mockResponse, tt.mockError)
			}

			result, err := service.ListEvaluatorTemplate(context.Background(), tt.req)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.mockResponse.TotalCount, result.TotalCount)
				assert.Equal(t, len(tt.mockResponse.Templates), len(result.Templates))
				assert.Equal(t, tt.req.PageSize, result.PageSize)
				assert.Equal(t, tt.req.PageNum, result.PageNum)
			}

			if !tt.expectedError {
				mockRepo.AssertExpectations(t)
			}
		})
	}
}
