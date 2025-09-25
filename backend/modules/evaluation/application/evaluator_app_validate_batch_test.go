// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatorservice "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// TestEvaluatorHandlerImpl_ValidateEvaluator 测试 ValidateEvaluator 方法
func TestEvaluatorHandlerImpl_ValidateEvaluator(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:                    mockAuth,
		evaluatorSourceServices: map[entity.EvaluatorType]service.EvaluatorSourceService{
			entity.EvaluatorTypePrompt: mockEvaluatorSourceService,
			entity.EvaluatorTypeCode:   mockEvaluatorSourceService,
		},
	}

	ctx := context.Background()
	validWorkspaceID := int64(123)

	tests := []struct {
		name        string
		req         *evaluatorservice.ValidateEvaluatorRequest
		mockSetup   func()
		wantResp    *evaluatorservice.ValidateEvaluatorResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - valid prompt evaluator",
			req: &evaluatorservice.ValidateEvaluatorRequest{
				WorkspaceID:    validWorkspaceID,
				EvaluatorType:  evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validWorkspaceID, 10),
					SpaceID:       validWorkspaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("debugLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				// Mock evaluator source service validate
				mockEvaluatorSourceService.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantResp: &evaluatorservice.ValidateEvaluatorResponse{
				Valid: gptr.Of(true),
			},
			wantErr: false,
		},
		{
			name: "success - valid code evaluator",
			req: &evaluatorservice.ValidateEvaluatorRequest{
				WorkspaceID:    validWorkspaceID,
				EvaluatorType:  evaluatordto.EvaluatorType_Code,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeContent:  gptr.Of("def evaluate(input): return 1.0"),
						LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
					},
				},
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validWorkspaceID, 10),
					SpaceID:       validWorkspaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("debugLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				// Mock evaluator source service validate
				mockEvaluatorSourceService.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantResp: &evaluatorservice.ValidateEvaluatorResponse{
				Valid: gptr.Of(true),
			},
			wantErr: false,
		},
		{
			name: "failure - auth error",
			req: &evaluatorservice.ValidateEvaluatorRequest{
				WorkspaceID:    validWorkspaceID,
				EvaluatorType:  evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
			},
			mockSetup: func() {
				// Mock auth failure
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errors.New("auth failed"))
			},
			wantErr:     true,
			wantErrCode: 0, // Generic error
		},
		{
			name: "failure - convert evaluator content error",
			req: &evaluatorservice.ValidateEvaluatorRequest{
				WorkspaceID:      validWorkspaceID,
				EvaluatorType:    evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: nil, // Invalid content
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantResp: &evaluatorservice.ValidateEvaluatorResponse{
				Valid:        gptr.Of(false),
				ErrorMessage: gptr.Of("evaluator content is nil"),
			},
			wantErr: false,
		},
		{
			name: "failure - unsupported evaluator type",
			req: &evaluatorservice.ValidateEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType(999), // Unsupported type
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantResp: &evaluatorservice.ValidateEvaluatorResponse{
				Valid:        gptr.Of(false),
				ErrorMessage: gptr.Of("unsupported evaluator type: 999"),
			},
			wantErr: false,
		},
		{
			name: "failure - validation error from source service",
			req: &evaluatorservice.ValidateEvaluatorRequest{
				WorkspaceID:    validWorkspaceID,
				EvaluatorType:  evaluatordto.EvaluatorType_Code,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeContent:  gptr.Of("invalid code"),
						LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
					},
				},
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)

				// Mock evaluator source service validate with error
				mockEvaluatorSourceService.EXPECT().Validate(gomock.Any(), gomock.Any()).Return(errors.New("syntax error"))
			},
			wantResp: &evaluatorservice.ValidateEvaluatorResponse{
				Valid:        gptr.Of(false),
				ErrorMessage: gptr.Of("syntax error"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mockSetup()

			resp, err := app.ValidateEvaluator(ctx, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
									var codeErr *errorx.CodeError
				if errors.As(err, &codeErr) {
					assert.Equal(t, tt.wantErrCode, codeErr.Code())
				}
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp.GetValid(), resp.GetValid())
				if tt.wantResp.ErrorMessage != nil {
					assert.Contains(t, resp.GetErrorMessage(), tt.wantResp.GetErrorMessage())
				}
			}
		})
	}
}

// TestEvaluatorHandlerImpl_BatchDebugEvaluator 测试 BatchDebugEvaluator 方法
func TestEvaluatorHandlerImpl_BatchDebugEvaluator(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockFileProvider := rpcmocks.NewMockIFileProvider(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		benefitService:   mockBenefitService,
		evaluatorService: mockEvaluatorService,
		fileProvider:     mockFileProvider,
	}

	ctx := context.Background()
	validWorkspaceID := int64(123)

	tests := []struct {
		name        string
		req         *evaluatorservice.BatchDebugEvaluatorRequest
		mockSetup   func()
		wantResp    *evaluatorservice.BatchDebugEvaluatorResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - single input data",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input"),
							},
						},
					},
				},
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validWorkspaceID, 10),
					SpaceID:       validWorkspaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("debugLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				// Mock benefit check
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckEvaluatorBenefitResult{}, nil)

				// Mock evaluator service debug
				mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					&entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{
							Score:     gptr.Of(0.8),
							Reasoning: "good result",
						},
					}, nil)
			},
			wantResp: &evaluatorservice.BatchDebugEvaluatorResponse{
				EvaluatorOutputData: []*evaluatordto.EvaluatorOutputData{
					{
						EvaluatorResult_: &common.EvaluatorResult{
							Score:  gptr.Of(0.8),
							Reason: gptr.Of("good result"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "success - multiple input data",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Code,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeContent:  gptr.Of("def evaluate(input): return 1.0"),
						LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input 1"),
							},
						},
					},
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input 2"),
							},
						},
					},
				},
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)

				// Mock benefit check
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckEvaluatorBenefitResult{}, nil)

				// Mock evaluator service debug - called twice for each input
				mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					&entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{
							Score:     gptr.Of(0.9),
							Reasoning: "result 1",
						},
					}, nil).Times(1)

				mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					&entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{
							Score:     gptr.Of(0.7),
							Reasoning: "result 2",
						},
					}, nil).Times(1)
			},
			wantResp: &evaluatorservice.BatchDebugEvaluatorResponse{
				EvaluatorOutputData: []*evaluatordto.EvaluatorOutputData{
					{
						EvaluatorResult_: &common.EvaluatorResult{
							Score:  gptr.Of(0.9),
							Reason: gptr.Of("result 1"),
						},
					},
					{
						EvaluatorResult_: &common.EvaluatorResult{
							Score:  gptr.Of(0.7),
							Reason: gptr.Of("result 2"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "failure - auth error",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input"),
							},
						},
					},
				},
			},
			mockSetup: func() {
				// Mock auth failure
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errors.New("auth failed"))
			},
			wantErr:     true,
			wantErrCode: 0, // Generic error
		},
		{
			name: "failure - benefit check denied",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input"),
							},
						},
					},
				},
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)

				// Mock benefit check denied
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(
					&benefit.CheckEvaluatorBenefitResult{
						DenyReason: gptr.Of("quota exceeded"),
					}, nil)
			},
			wantErr:     true,
			wantErrCode: errno.EvaluatorBenefitDenyCode,
		},
		{
			name: "failure - benefit check service error",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input"),
							},
						},
					},
				},
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)

				// Mock benefit check service error
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(nil, errors.New("benefit service error"))
			},
			wantErr:     true,
			wantErrCode: 0, // Generic error
		},
		{
			name: "success - partial failures in batch",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Code,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeContent:  gptr.Of("def evaluate(input): return 1.0"),
						LanguageType: gptr.Of(evaluatordto.LanguageTypePython),
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input 1"),
							},
						},
					},
					{
						InputFields: map[string]*common.Content{
							"input": {
								ContentType: gptr.Of(common.ContentTypeText),
								Text:        gptr.Of("test input 2"),
							},
						},
					},
				},
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)

				// Mock benefit check
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckEvaluatorBenefitResult{}, nil)

				// Mock evaluator service debug - first succeeds, second fails
				mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					&entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{
							Score:     gptr.Of(0.8),
							Reasoning: "success result",
						},
					}, nil).Times(1)

				mockEvaluatorService.EXPECT().DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					nil, errors.New("evaluation failed")).Times(1)
			},
			wantResp: &evaluatorservice.BatchDebugEvaluatorResponse{
				EvaluatorOutputData: []*evaluatordto.EvaluatorOutputData{
					{
						EvaluatorResult_: &common.EvaluatorResult{
							Score:  gptr.Of(0.8),
							Reason: gptr.Of("success result"),
						},
					},
					{
						EvaluatorRunError: &evaluatordto.EvaluatorRunError{
							Code:    gptr.Of(int32(500)),
							Message: gptr.Of("evaluation failed"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "success - empty input data",
			req: &evaluatorservice.BatchDebugEvaluatorRequest{
				WorkspaceID:   validWorkspaceID,
				EvaluatorType: evaluatordto.EvaluatorType_Prompt,
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						MessageList: []*common.Message{
							{
								Role: common.RolePtr(common.Role_User),
								Content: &common.Content{
									ContentType: gptr.Of(common.ContentTypeText),
									Text:        gptr.Of("test prompt"),
								},
							},
						},
						ModelConfig: &common.ModelConfig{
							ModelID: gptr.Of(int64(1)),
						},
					},
				},
				InputData: []*evaluatordto.EvaluatorInputData{},
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)

				// Mock benefit check
				mockBenefitService.EXPECT().CheckEvaluatorBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckEvaluatorBenefitResult{}, nil)

				// No evaluator service debug calls expected for empty input
			},
			wantResp: &evaluatorservice.BatchDebugEvaluatorResponse{
				EvaluatorOutputData: []*evaluatordto.EvaluatorOutputData{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.mockSetup()

			resp, err := app.BatchDebugEvaluator(ctx, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
									var codeErr *errorx.CodeError
				if errors.As(err, &codeErr) {
					assert.Equal(t, tt.wantErrCode, codeErr.Code())
				}
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, len(tt.wantResp.EvaluatorOutputData), len(resp.EvaluatorOutputData))

				for i, expectedOutput := range tt.wantResp.EvaluatorOutputData {
					actualOutput := resp.EvaluatorOutputData[i]

					if expectedOutput.EvaluatorResult_ != nil {
						assert.NotNil(t, actualOutput.EvaluatorResult_)
						assert.Equal(t, expectedOutput.EvaluatorResult_.GetScore(), actualOutput.EvaluatorResult_.GetScore())
						assert.Equal(t, expectedOutput.EvaluatorResult_.GetReason(), actualOutput.EvaluatorResult_.GetReason())
					}

					if expectedOutput.EvaluatorRunError != nil {
						assert.NotNil(t, actualOutput.EvaluatorRunError)
						assert.Equal(t, expectedOutput.EvaluatorRunError.GetCode(), actualOutput.EvaluatorRunError.GetCode())
						assert.Contains(t, actualOutput.EvaluatorRunError.GetMessage(), expectedOutput.EvaluatorRunError.GetMessage())
					}
				}
			}
		})
	}
}