// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// TestEvaluatorTemplateServiceImpl_CreateEvaluatorTemplate 测试创建评估器模板
func TestEvaluatorTemplateServiceImpl_CreateEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		req            *entity.CreateEvaluatorTemplateRequest
		mockSetup      func(mockRepo *repomocks.MockEvaluatorTemplateRepo)
		expectedError  bool
		expectedErrCode int32
		description    string
	}{
		{
			name: "成功 - 创建Prompt类型模板",
			req: &entity.CreateEvaluatorTemplateRequest{
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
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					CreateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, template *entity.EvaluatorTemplate) (*entity.EvaluatorTemplate, error) {
						template.ID = 1
						return template, nil
					})
			},
			expectedError: false,
			description:  "成功创建Prompt类型评估器模板",
		},
		{
			name: "成功 - 创建Code类型模板",
			req: &entity.CreateEvaluatorTemplateRequest{
				SpaceID:       100,
				Name:          "Test Template",
				Description:   "Test Description",
				EvaluatorType: entity.EvaluatorTypeCode,
				CodeEvaluatorContent: &entity.CodeEvaluatorContent{
					CodeContent: "def evaluate(): pass",
				},
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					CreateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, template *entity.EvaluatorTemplate) (*entity.EvaluatorTemplate, error) {
						template.ID = 1
						return template, nil
					})
			},
			expectedError: false,
			description:  "成功创建Code类型评估器模板",
		},
		{
			name: "失败 - 无效的SpaceID",
			req: &entity.CreateEvaluatorTemplateRequest{
				SpaceID:       0,
				Name:          "Test Template",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "无效的空间ID应该返回错误",
		},
		{
			name: "失败 - 空的模板名称",
			req: &entity.CreateEvaluatorTemplateRequest{
				SpaceID:       100,
				Name:          "",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "空的模板名称应该返回错误",
		},
		{
			name: "失败 - 模板名称过长",
			req: &entity.CreateEvaluatorTemplateRequest{
				SpaceID:       100,
				Name:          string(make([]byte, 101)), // 101个字符
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorContent: &entity.PromptEvaluatorContent{
					MessageList: []*entity.Message{},
				},
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "模板名称长度超过100应该返回错误",
		},
		{
			name: "失败 - Prompt类型缺少PromptEvaluatorContent",
			req: &entity.CreateEvaluatorTemplateRequest{
				SpaceID:       100,
				Name:          "Test Template",
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "Prompt类型评估器缺少PromptEvaluatorContent应该返回错误",
		},
		{
			name: "失败 - Code类型缺少CodeEvaluatorContent",
			req: &entity.CreateEvaluatorTemplateRequest{
				SpaceID:       100,
				Name:          "Test Template",
				EvaluatorType: entity.EvaluatorTypeCode,
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "Code类型评估器缺少CodeEvaluatorContent应该返回错误",
		},
		{
			name: "失败 - 用户ID缺失",
			req: &entity.CreateEvaluatorTemplateRequest{
				SpaceID:       100,
				Name:          "Test Template",
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorContent: &entity.PromptEvaluatorContent{
					MessageList: []*entity.Message{},
				},
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "用户ID缺失应该返回错误",
		},
		{
			name: "失败 - repo创建错误",
			req: &entity.CreateEvaluatorTemplateRequest{
				SpaceID:       100,
				Name:          "Test Template",
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorContent: &entity.PromptEvaluatorContent{
					MessageList: []*entity.Message{},
				},
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					CreateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("repo error"))
			},
			expectedError: true,
			expectedErrCode: errno.CommonInternalErrorCode,
			description:   "repo创建错误应该返回内部错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := repomocks.NewMockEvaluatorTemplateRepo(ctrl)
			service := NewEvaluatorTemplateService(mockRepo)

			ctx := context.Background()
			if !tt.expectedError || (tt.name != "失败 - 无效的SpaceID" && tt.name != "失败 - 空的模板名称" && tt.name != "失败 - 模板名称过长" && tt.name != "失败 - Prompt类型缺少PromptEvaluatorContent" && tt.name != "失败 - Code类型缺少CodeEvaluatorContent" && tt.name != "失败 - 用户ID缺失") {
				ctx = session.WithCtxUser(ctx, &session.User{ID: "user123"})
			}

			tt.mockSetup(mockRepo)

			result, err := service.CreateEvaluatorTemplate(ctx, tt.req)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.expectedErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.expectedErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotNil(t, result.Template)
				assert.Equal(t, tt.req.SpaceID, result.Template.SpaceID)
				assert.Equal(t, tt.req.Name, result.Template.Name)
				assert.Equal(t, tt.req.Description, result.Template.Description)
				assert.Equal(t, tt.req.EvaluatorType, result.Template.EvaluatorType)
			}
		})
	}
}

// TestEvaluatorTemplateServiceImpl_UpdateEvaluatorTemplate 测试更新评估器模板
func TestEvaluatorTemplateServiceImpl_UpdateEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		req            *entity.UpdateEvaluatorTemplateRequest
		mockSetup      func(mockRepo *repomocks.MockEvaluatorTemplateRepo)
		expectedError  bool
		expectedErrCode int32
		description    string
	}{
		{
			name: "成功 - 更新模板名称",
			req: &entity.UpdateEvaluatorTemplateRequest{
				ID:   1,
				Name: gptr.Of("Updated Template"),
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				existingTemplate := &entity.EvaluatorTemplate{
					ID:      1,
					SpaceID: 100,
					Name:    "Original Template",
					BaseInfo: &entity.BaseInfo{},
				}
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(existingTemplate, nil)

				mockRepo.EXPECT().
					UpdateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, template *entity.EvaluatorTemplate) (*entity.EvaluatorTemplate, error) {
						return template, nil
					})
			},
			expectedError: false,
			description:  "成功更新评估器模板名称",
		},
		{
			name: "成功 - 更新多个字段",
			req: &entity.UpdateEvaluatorTemplateRequest{
				ID:          1,
				Name:        gptr.Of("Updated Template"),
				Description: gptr.Of("Updated Description"),
				Benchmark:   gptr.Of("benchmark1"),
				Vendor:      gptr.Of("vendor1"),
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				existingTemplate := &entity.EvaluatorTemplate{
					ID:      1,
					SpaceID: 100,
					Name:    "Original Template",
					BaseInfo: &entity.BaseInfo{},
				}
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(existingTemplate, nil)

				mockRepo.EXPECT().
					UpdateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, template *entity.EvaluatorTemplate) (*entity.EvaluatorTemplate, error) {
						return template, nil
					})
			},
			expectedError: false,
			description:  "成功更新多个字段",
		},
		{
			name: "失败 - 无效的模板ID",
			req: &entity.UpdateEvaluatorTemplateRequest{
				ID: 0,
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "无效的模板ID应该返回错误",
		},
		{
			name: "失败 - 空的模板名称",
			req: &entity.UpdateEvaluatorTemplateRequest{
				ID:   1,
				Name: gptr.Of(""),
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "空的模板名称应该返回错误",
		},
		{
			name: "失败 - 模板名称过长",
			req: &entity.UpdateEvaluatorTemplateRequest{
				ID:   1,
				Name: gptr.Of(string(make([]byte, 101))),
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "模板名称长度超过100应该返回错误",
		},
		{
			name: "失败 - 用户ID缺失",
			req: &entity.UpdateEvaluatorTemplateRequest{
				ID:   1,
				Name: gptr.Of("Updated Template"),
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "用户ID缺失应该返回错误",
		},
		{
			name: "失败 - 模板不存在",
			req: &entity.UpdateEvaluatorTemplateRequest{
				ID:   1,
				Name: gptr.Of("Updated Template"),
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(nil, nil)
			},
			expectedError: true,
			expectedErrCode: errno.ResourceNotFoundCode,
			description:   "模板不存在应该返回错误",
		},
		{
			name: "失败 - 获取模板错误",
			req: &entity.UpdateEvaluatorTemplateRequest{
				ID:   1,
				Name: gptr.Of("Updated Template"),
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(nil, errors.New("repo error"))
			},
			expectedError: true,
			expectedErrCode: errno.CommonInternalErrorCode,
			description:   "获取模板错误应该返回内部错误",
		},
		{
			name: "失败 - 更新模板错误",
			req: &entity.UpdateEvaluatorTemplateRequest{
				ID:   1,
				Name: gptr.Of("Updated Template"),
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				existingTemplate := &entity.EvaluatorTemplate{
					ID:      1,
					SpaceID: 100,
					Name:    "Original Template",
					BaseInfo: &entity.BaseInfo{},
				}
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(existingTemplate, nil)

				mockRepo.EXPECT().
					UpdateEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("repo error"))
			},
			expectedError: true,
			expectedErrCode: errno.CommonInternalErrorCode,
			description:   "更新模板错误应该返回内部错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := repomocks.NewMockEvaluatorTemplateRepo(ctrl)
			service := NewEvaluatorTemplateService(mockRepo)

			ctx := context.Background()
			if !tt.expectedError || (tt.name != "失败 - 无效的模板ID" && tt.name != "失败 - 空的模板名称" && tt.name != "失败 - 模板名称过长" && tt.name != "失败 - 用户ID缺失") {
				ctx = session.WithCtxUser(ctx, &session.User{ID: "user123"})
			}

			tt.mockSetup(mockRepo)

			result, err := service.UpdateEvaluatorTemplate(ctx, tt.req)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.expectedErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.expectedErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotNil(t, result.Template)
				if tt.req.Name != nil {
					assert.Equal(t, *tt.req.Name, result.Template.Name)
				}
			}
		})
	}
}

// TestEvaluatorTemplateServiceImpl_DeleteEvaluatorTemplate 测试删除评估器模板
func TestEvaluatorTemplateServiceImpl_DeleteEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		req            *entity.DeleteEvaluatorTemplateRequest
		mockSetup      func(mockRepo *repomocks.MockEvaluatorTemplateRepo)
		expectedError  bool
		expectedErrCode int32
		description    string
	}{
		{
			name: "成功 - 删除模板",
			req: &entity.DeleteEvaluatorTemplateRequest{
				ID: 1,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				existingTemplate := &entity.EvaluatorTemplate{
					ID:     1,
					SpaceID: 100,
				}
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(existingTemplate, nil)

				mockRepo.EXPECT().
					DeleteEvaluatorTemplate(gomock.Any(), int64(1), "user123").
					Return(nil)
			},
			expectedError: false,
			description:  "成功删除评估器模板",
		},
		{
			name: "失败 - 无效的模板ID",
			req: &entity.DeleteEvaluatorTemplateRequest{
				ID: 0,
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "无效的模板ID应该返回错误",
		},
		{
			name: "失败 - 用户ID缺失",
			req: &entity.DeleteEvaluatorTemplateRequest{
				ID: 1,
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "用户ID缺失应该返回错误",
		},
		{
			name: "失败 - 模板不存在",
			req: &entity.DeleteEvaluatorTemplateRequest{
				ID: 1,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(nil, nil)
			},
			expectedError: true,
			expectedErrCode: errno.ResourceNotFoundCode,
			description:   "模板不存在应该返回错误",
		},
		{
			name: "失败 - 获取模板错误",
			req: &entity.DeleteEvaluatorTemplateRequest{
				ID: 1,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(nil, errors.New("repo error"))
			},
			expectedError: true,
			expectedErrCode: errno.CommonInternalErrorCode,
			description:   "获取模板错误应该返回内部错误",
		},
		{
			name: "失败 - 删除模板错误",
			req: &entity.DeleteEvaluatorTemplateRequest{
				ID: 1,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				existingTemplate := &entity.EvaluatorTemplate{
					ID:     1,
					SpaceID: 100,
				}
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(existingTemplate, nil)

				mockRepo.EXPECT().
					DeleteEvaluatorTemplate(gomock.Any(), int64(1), "user123").
					Return(errors.New("repo error"))
			},
			expectedError: true,
			expectedErrCode: errno.CommonInternalErrorCode,
			description:   "删除模板错误应该返回内部错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := repomocks.NewMockEvaluatorTemplateRepo(ctrl)
			service := NewEvaluatorTemplateService(mockRepo)

			ctx := context.Background()
			if !tt.expectedError || (tt.name != "失败 - 无效的模板ID" && tt.name != "失败 - 用户ID缺失") {
				ctx = session.WithCtxUser(ctx, &session.User{ID: "user123"})
			}

			tt.mockSetup(mockRepo)

			result, err := service.DeleteEvaluatorTemplate(ctx, tt.req)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.expectedErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.expectedErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.Success)
			}
		})
	}
}

// TestEvaluatorTemplateServiceImpl_GetEvaluatorTemplate 测试获取评估器模板
func TestEvaluatorTemplateServiceImpl_GetEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		req            *entity.GetEvaluatorTemplateRequest
		mockSetup      func(mockRepo *repomocks.MockEvaluatorTemplateRepo)
		expectedError  bool
		expectedErrCode int32
		description    string
	}{
		{
			name: "成功 - 获取模板",
			req: &entity.GetEvaluatorTemplateRequest{
				ID: 1,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockTemplate := &entity.EvaluatorTemplate{
					ID:      1,
					SpaceID: 100,
					Name:    "Test Template",
				}
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(mockTemplate, nil)
			},
			expectedError: false,
			description:  "成功获取评估器模板",
		},
		{
			name: "成功 - 包含已删除记录",
			req: &entity.GetEvaluatorTemplateRequest{
				ID:            1,
				IncludeDeleted: true,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockTemplate := &entity.EvaluatorTemplate{
					ID:      1,
					SpaceID: 100,
					Name:    "Test Template",
				}
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), true).
					Return(mockTemplate, nil)
			},
			expectedError: false,
			description:  "成功获取已删除的模板",
		},
		{
			name: "失败 - 无效的模板ID",
			req: &entity.GetEvaluatorTemplateRequest{
				ID: 0,
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "无效的模板ID应该返回错误",
		},
		{
			name: "失败 - 模板不存在",
			req: &entity.GetEvaluatorTemplateRequest{
				ID: 1,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(nil, nil)
			},
			expectedError: true,
			expectedErrCode: errno.ResourceNotFoundCode,
			description:   "模板不存在应该返回错误",
		},
		{
			name: "失败 - 获取模板错误",
			req: &entity.GetEvaluatorTemplateRequest{
				ID: 1,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					GetEvaluatorTemplate(gomock.Any(), int64(1), false).
					Return(nil, errors.New("repo error"))
			},
			expectedError: true,
			expectedErrCode: errno.CommonInternalErrorCode,
			description:   "获取模板错误应该返回内部错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := repomocks.NewMockEvaluatorTemplateRepo(ctrl)
			service := NewEvaluatorTemplateService(mockRepo)

			tt.mockSetup(mockRepo)

			result, err := service.GetEvaluatorTemplate(context.Background(), tt.req)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.expectedErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.expectedErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotNil(t, result.Template)
				assert.Equal(t, tt.req.ID, result.Template.ID)
			}
		})
	}
}

// TestEvaluatorTemplateServiceImpl_ListEvaluatorTemplate 测试查询评估器模板列表
func TestEvaluatorTemplateServiceImpl_ListEvaluatorTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		req            *entity.ListEvaluatorTemplateRequest
		mockSetup      func(mockRepo *repomocks.MockEvaluatorTemplateRepo)
		expectedError  bool
		expectedErrCode int32
		expectedTotalPages int32
		description    string
	}{
		{
			name: "成功 - 正常查询",
			req: &entity.ListEvaluatorTemplateRequest{
				SpaceID:  100,
				PageSize: 10,
				PageNum:  1,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), &repo.ListEvaluatorTemplateRequest{
						SpaceID:        100,
						PageSize:       10,
						PageNum:        1,
						IncludeDeleted: false,
					}).
					Return(&repo.ListEvaluatorTemplateResponse{
						TotalCount: 25,
						Templates: []*entity.EvaluatorTemplate{
							{ID: 1, Name: "Template 1"},
							{ID: 2, Name: "Template 2"},
						},
					}, nil)
			},
			expectedError: false,
			expectedTotalPages: 3, // (25 + 10 - 1) / 10 = 3
			description:   "成功查询评估器模板列表",
		},
		{
			name: "成功 - 包含筛选条件",
			req: &entity.ListEvaluatorTemplateRequest{
				SpaceID:      100,
				FilterOption: &entity.EvaluatorFilterOption{},
				PageSize:     10,
				PageNum:      1,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), &repo.ListEvaluatorTemplateRequest{
						SpaceID:        100,
						FilterOption:   &entity.EvaluatorFilterOption{},
						PageSize:       10,
						PageNum:        1,
						IncludeDeleted: false,
					}).
					Return(&repo.ListEvaluatorTemplateResponse{
						TotalCount: 5,
						Templates: []*entity.EvaluatorTemplate{
							{ID: 1, Name: "Template 1"},
						},
					}, nil)
			},
			expectedError: false,
			expectedTotalPages: 1, // (5 + 10 - 1) / 10 = 1
			description:   "成功查询带筛选条件的模板列表",
		},
		{
			name: "成功 - 包含已删除记录",
			req: &entity.ListEvaluatorTemplateRequest{
				SpaceID:       100,
				PageSize:      10,
				PageNum:       1,
				IncludeDeleted: true,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), &repo.ListEvaluatorTemplateRequest{
						SpaceID:        100,
						PageSize:       10,
						PageNum:        1,
						IncludeDeleted: true,
					}).
					Return(&repo.ListEvaluatorTemplateResponse{
						TotalCount: 10,
						Templates: []*entity.EvaluatorTemplate{
							{ID: 1, Name: "Template 1"},
						},
					}, nil)
			},
			expectedError: false,
			expectedTotalPages: 1,
			description:   "成功查询包含已删除记录的模板列表",
		},
		{
			name: "失败 - 无效的分页大小（为0）",
			req: &entity.ListEvaluatorTemplateRequest{
				SpaceID:  100,
				PageSize: 0,
				PageNum:  1,
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "无效的分页大小应该返回错误",
		},
		{
			name: "失败 - 分页大小超过100",
			req: &entity.ListEvaluatorTemplateRequest{
				SpaceID:  100,
				PageSize: 101,
				PageNum:  1,
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "分页大小超过100应该返回错误",
		},
		{
			name: "失败 - 无效的页码",
			req: &entity.ListEvaluatorTemplateRequest{
				SpaceID:  100,
				PageSize: 10,
				PageNum:  0,
			},
			mockSetup:     func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {},
			expectedError: true,
			expectedErrCode: errno.CommonInvalidParamCode,
			description:   "无效的页码应该返回错误",
		},
		{
			name: "失败 - repo查询错误",
			req: &entity.ListEvaluatorTemplateRequest{
				SpaceID:  100,
				PageSize: 10,
				PageNum:  1,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("repo error"))
			},
			expectedError: true,
			expectedErrCode: errno.CommonInternalErrorCode,
			description:   "repo查询错误应该返回内部错误",
		},
		{
			name: "成功 - 计算总页数（整除）",
			req: &entity.ListEvaluatorTemplateRequest{
				SpaceID:  100,
				PageSize: 10,
				PageNum:  1,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(&repo.ListEvaluatorTemplateResponse{
						TotalCount: 20,
						Templates: []*entity.EvaluatorTemplate{
							{ID: 1, Name: "Template 1"},
						},
					}, nil)
			},
			expectedError: false,
			expectedTotalPages: 2, // 20 / 10 = 2
			description:   "正确计算总页数（整除）",
		},
		{
			name: "成功 - 计算总页数（不整除）",
			req: &entity.ListEvaluatorTemplateRequest{
				SpaceID:  100,
				PageSize: 10,
				PageNum:  1,
			},
			mockSetup: func(mockRepo *repomocks.MockEvaluatorTemplateRepo) {
				mockRepo.EXPECT().
					ListEvaluatorTemplate(gomock.Any(), gomock.Any()).
					Return(&repo.ListEvaluatorTemplateResponse{
						TotalCount: 21,
						Templates: []*entity.EvaluatorTemplate{
							{ID: 1, Name: "Template 1"},
						},
					}, nil)
			},
			expectedError: false,
			expectedTotalPages: 3, // (21 + 10 - 1) / 10 = 3
			description:   "正确计算总页数（不整除）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := repomocks.NewMockEvaluatorTemplateRepo(ctrl)
			service := NewEvaluatorTemplateService(mockRepo)

			tt.mockSetup(mockRepo)

			result, err := service.ListEvaluatorTemplate(context.Background(), tt.req)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.expectedErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.expectedErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.req.PageSize, result.PageSize)
				assert.Equal(t, tt.req.PageNum, result.PageNum)
				if tt.expectedTotalPages > 0 {
					assert.Equal(t, tt.expectedTotalPages, result.TotalPages)
				}
			}
		})
	}
}
