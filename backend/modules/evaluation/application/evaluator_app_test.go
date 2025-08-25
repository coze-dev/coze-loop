// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	auditmocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatorservice "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmock "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	userinfomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestEvaluatorHandlerImpl_ListEvaluators(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		userInfoService:  mockUserInfoService,
	}

	// Test data
	validSpaceID := int64(123)
	validEvaluators := []*entity.Evaluator{
		{
			ID:             1,
			SpaceID:        validSpaceID,
			Name:           "test-evaluator-1",
			EvaluatorType:  entity.EvaluatorTypePrompt,
			Description:    "test description 1",
			DraftSubmitted: true,
		},
		{
			ID:             2,
			SpaceID:        validSpaceID,
			Name:           "test-evaluator-2",
			EvaluatorType:  entity.EvaluatorTypeCode,
			Description:    "test description 2",
			DraftSubmitted: false,
		},
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.ListEvaluatorsRequest
		mockSetup   func()
		wantResp    *evaluatorservice.ListEvaluatorsResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID: validSpaceID,
			},
			mockSetup: func() {
				// Mock auth
				mockAuth.EXPECT().Authorization(gomock.Any(), &rpc.AuthorizationParam{
					ObjectID:      strconv.FormatInt(validSpaceID, 10),
					SpaceID:       validSpaceID,
					ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
				}).Return(nil)

				// Mock service call
				mockEvaluatorService.EXPECT().ListEvaluator(gomock.Any(), gomock.Any()).
					Return(validEvaluators, int64(2), nil)

				// Mock user info service
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return()
			},
			wantResp: &evaluatorservice.ListEvaluatorsResponse{
				Total: gptr.Of(int64(2)),
				Evaluators: []*evaluatordto.Evaluator{
					evaluator.ConvertEvaluatorDO2DTO(validEvaluators[0]),
					evaluator.ConvertEvaluatorDO2DTO(validEvaluators[1]),
				},
			},
			wantErr: false,
		},
		{
			name: "error - auth failed",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID: validSpaceID,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
		{
			name: "error - service failure",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID: validSpaceID,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorService.EXPECT().ListEvaluator(gomock.Any(), gomock.Any()).
					Return(nil, int64(0), errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonInternalErrorCode,
		},
		{
			name: "success - with pagination",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID: validSpaceID,
				PageSize:    gptr.Of(int32(1)),
				PageNumber:  gptr.Of(int32(1)),
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorService.EXPECT().ListEvaluator(gomock.Any(), gomock.Any()).
					Return(validEvaluators[:1], int64(2), nil)
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return()
			},
			wantResp: &evaluatorservice.ListEvaluatorsResponse{
				Total: gptr.Of(int64(2)),
				Evaluators: []*evaluatordto.Evaluator{
					evaluator.ConvertEvaluatorDO2DTO(validEvaluators[0]),
				},
			},
			wantErr: false,
		},
		{
			name: "success - with search name",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID: validSpaceID,
				SearchName:  gptr.Of("test-evaluator-1"),
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorService.EXPECT().ListEvaluator(gomock.Any(), gomock.Any()).
					Return(validEvaluators[:1], int64(1), nil)
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return()
			},
			wantResp: &evaluatorservice.ListEvaluatorsResponse{
				Total: gptr.Of(int64(1)),
				Evaluators: []*evaluatordto.Evaluator{
					evaluator.ConvertEvaluatorDO2DTO(validEvaluators[0]),
				},
			},
			wantErr: false,
		},
		{
			name: "success - with evaluator type filter",
			req: &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID:   validSpaceID,
				EvaluatorType: []evaluatordto.EvaluatorType{evaluatordto.EvaluatorType_Prompt},
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorService.EXPECT().ListEvaluator(gomock.Any(), gomock.Any()).
					Return(validEvaluators[:1], int64(1), nil)
				mockUserInfoService.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).Return()
			},
			wantResp: &evaluatorservice.ListEvaluatorsResponse{
				Total: gptr.Of(int64(1)),
				Evaluators: []*evaluatordto.Evaluator{
					evaluator.ConvertEvaluatorDO2DTO(validEvaluators[0]),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.ListEvaluators(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp.Total, resp.Total)
				assert.Equal(t, len(tt.wantResp.Evaluators), len(resp.Evaluators))
				for i, evaluator := range tt.wantResp.Evaluators {
					assert.Equal(t, evaluator.GetEvaluatorID(), resp.Evaluators[i].GetEvaluatorID())
					assert.Equal(t, evaluator.GetWorkspaceID(), resp.Evaluators[i].GetWorkspaceID())
					assert.Equal(t, evaluator.GetName(), resp.Evaluators[i].GetName())
					assert.Equal(t, evaluator.GetEvaluatorType(), resp.Evaluators[i].GetEvaluatorType())
				}
			}
		})
	}
}

func TestEvaluatorHandlerImpl_GetEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
	mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

	app := &EvaluatorHandlerImpl{
		auth:             mockAuth,
		evaluatorService: mockEvaluatorService,
		userInfoService:  mockUserInfoService,
	}

	// Test data
	validSpaceID := int64(123)
	validEvaluatorID := int64(456)
	validEvaluator := &entity.Evaluator{
		ID:             validEvaluatorID,
		SpaceID:        validSpaceID,
		Name:           "Test Evaluator",
		EvaluatorType:  entity.EvaluatorTypePrompt,
		Description:    "Test Description",
		DraftSubmitted: true,
	}

	tests := []struct {
		name        string
		req         *evaluatorservice.GetEvaluatorRequest
		mockSetup   func()
		wantResp    *evaluatorservice.GetEvaluatorResponse
		wantErr     bool
		wantErrCode int32
	}{
		{
			name: "success - normal request",
			req: &evaluatorservice.GetEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: &validEvaluatorID,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).
					Return(validEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(validEvaluator.ID, 10),
						SpaceID:       validSpaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil)

				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Return()
			},
			wantResp: &evaluatorservice.GetEvaluatorResponse{
				Evaluator: evaluator.ConvertEvaluatorDO2DTO(validEvaluator),
			},
			wantErr: false,
		},
		{
			name: "error - evaluator not found",
			req: &evaluatorservice.GetEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: &validEvaluatorID,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).
					Return(nil, nil)
			},
			wantResp: &evaluatorservice.GetEvaluatorResponse{},
			wantErr:  false,
		},
		{
			name: "error - auth failed",
			req: &evaluatorservice.GetEvaluatorRequest{
				WorkspaceID: validSpaceID,
				EvaluatorID: &validEvaluatorID,
			},
			mockSetup: func() {
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), validSpaceID, validEvaluatorID, false).
					Return(validEvaluator, nil)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantResp:    nil,
			wantErr:     true,
			wantErrCode: errno.CommonNoPermissionCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			resp, err := app.GetEvaluator(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.wantErrCode, statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantResp, resp)
			}
		})
	}
}

// 新增的复杂业务逻辑测试

// TestEvaluatorHandlerImpl_ComplexBusinessScenarios 测试复杂业务场景
func TestEvaluatorHandlerImpl_ComplexBusinessScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "多层依赖服务交互测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				// 创建所有依赖的 mock
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockConfiger := confmocks.NewMockIConfiger(ctrl)
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockEvaluatorRecordService := mocks.NewMockEvaluatorRecordService(ctrl)
				mockMetrics := metricsmock.NewMockEvaluatorExecMetrics(ctrl)
				mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)
				mockAuditClient := auditmocks.NewMockIAuditService(ctrl)
				mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
				mockFileProvider := rpcmocks.NewMockIFileProvider(ctrl)

				handler := NewEvaluatorHandlerImpl(
					mockIDGen,
					mockConfiger,
					mockAuth,
					mockEvaluatorService,
					mockEvaluatorRecordService,
					mockMetrics,
					mockUserInfoService,
					mockAuditClient,
					mockBenefitService,
					mockFileProvider,
				)

				// 测试复杂的调试场景，涉及多个服务交互
				request := &evaluatorservice.DebugEvaluatorRequest{
					WorkspaceID:   123,
					EvaluatorType: evaluatordto.EvaluatorType_Prompt,
					EvaluatorContent: &evaluatordto.EvaluatorContent{
						PromptEvaluator: &evaluatordto.PromptEvaluator{
							MessageList: []*common.Message{
								{
									Role: common.RolePtr(common.Role_User),
									Content: &common.Content{
										ContentType: gptr.Of(common.ContentTypeMultiPart),
										MultiPart: []*common.Content{
											{
												ContentType: gptr.Of(common.ContentTypeText),
												Text:        gptr.Of("请分析这张图片："),
											},
											{
												ContentType: gptr.Of(common.ContentTypeImage),
												Image: &common.Image{
													URI: gptr.Of("test-image-uri"),
												},
											},
										},
									},
								},
							},
						},
					},
					InputData: &evaluatordto.EvaluatorInputData{
						InputFields: map[string]*common.Content{
							"image": {
								ContentType: gptr.Of(common.ContentTypeImage),
								Image: &common.Image{
									URI: gptr.Of("input-image-uri"),
								},
							},
						},
					},
				}

				// 设置复杂的 mock 期望
				// 1. 鉴权
				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      "123",
						SpaceID:       int64(123),
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("debugLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil).
					Times(1)

				// 2. 权益检查
				mockBenefitService.EXPECT().
					CheckEvaluatorBenefit(gomock.Any(), &benefit.CheckEvaluatorBenefitParams{
						ConnectorUID: "",
						SpaceID:      123,
					}).
					Return(&benefit.CheckEvaluatorBenefitResult{DenyReason: nil}, nil).
					Times(1)

				// 3. 文件 URI 转 URL
				mockFileProvider.EXPECT().
					MGetFileURL(gomock.Any(), []string{"input-image-uri"}).
					Return(map[string]string{"input-image-uri": "https://example.com/image.jpg"}, nil).
					Times(1)

				// 4. 评估器调试
				mockEvaluatorService.EXPECT().
					DebugEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData) (*entity.EvaluatorOutputData, error) {
						// 验证输入数据已被正确处理
						assert.Equal(t, int64(123), evaluator.SpaceID)
						assert.Equal(t, entity.EvaluatorTypePrompt, evaluator.EvaluatorType)

						// 验证 URI 已转换为 URL
						imageContent := input.InputFields["image"]
						assert.NotNil(t, imageContent)
						assert.NotNil(t, imageContent.Image)
						assert.Equal(t, "https://example.com/image.jpg", gptr.Indirect(imageContent.Image.URL))

						return &entity.EvaluatorOutputData{
							EvaluatorResult: &entity.EvaluatorResult{
								Score:     gptr.Of(0.85),
								Reasoning: "多模态内容分析完成",
							},
						}, nil
					}).
					Times(1)

				ctx := context.Background()
				resp, err := handler.DebugEvaluator(ctx, request)

				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.EvaluatorOutputData)
				assert.Equal(t, 0.85, gptr.Indirect(resp.EvaluatorOutputData.EvaluatorResult_.Score))
			},
		},
		{
			name: "权限验证和审核流程测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockAuditClient := auditmocks.NewMockIAuditService(ctrl)
				mockMetrics := metricsmock.NewMockEvaluatorExecMetrics(ctrl)

				handler := &EvaluatorHandlerImpl{
					auth:             mockAuth,
					evaluatorService: mockEvaluatorService,
					auditClient:      mockAuditClient,
					metrics:          mockMetrics,
				}

				// 测试包含敏感内容的创建请求
				request := &evaluatorservice.CreateEvaluatorRequest{
					Evaluator: &evaluatordto.Evaluator{
						WorkspaceID:   gptr.Of(int64(123)),
						Name:          gptr.Of("敏感内容评估器"),
						Description:   gptr.Of("包含敏感词汇的描述"),
						EvaluatorType: gptr.Of(evaluatordto.EvaluatorType_Prompt),
						CurrentVersion: &evaluatordto.EvaluatorVersion{
							Version:     gptr.Of("1.0.0"),
							Description: gptr.Of("版本描述包含敏感内容"),
							EvaluatorContent: &evaluatordto.EvaluatorContent{
								PromptEvaluator: &evaluatordto.PromptEvaluator{},
							},
						},
					},
				}

				// 设置审核被拒绝的场景
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, param audit.AuditParam) (audit.AuditRecord, error) {
						// 验证审核参数
						assert.Equal(t, audit.AuditType_CozeLoopEvaluatorModify, param.AuditType)
						assert.Contains(t, param.AuditData["texts"], "敏感内容评估器")

						return audit.AuditRecord{
							AuditStatus:  audit.AuditStatus_Rejected,
							FailedReason: gptr.Of("内容包含敏感词汇"),
						}, nil
					}).
					Times(1)

				ctx := context.Background()
				resp, err := handler.CreateEvaluator(ctx, request)

				assert.Error(t, err)
				assert.Nil(t, resp)

				// 验证错误类型
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, int32(errno.RiskContentDetectedCode), statusErr.Code())
			},
		},
		{
			name: "并发安全和数据一致性测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

				handler := &EvaluatorHandlerImpl{
					auth:             mockAuth,
					evaluatorService: mockEvaluatorService,
					userInfoService:  mockUserInfoService,
				}

				// 模拟并发访问同一个评估器
				evaluatorID := int64(123)
				spaceID := int64(456)

				evaluator := &entity.Evaluator{
					ID:      evaluatorID,
					SpaceID: spaceID,
					Name:    "并发测试评估器",
				}

				// 设置并发调用的期望
				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), spaceID, evaluatorID, false).
					Return(evaluator, nil).
					Times(10) // 10个并发请求

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(10)

				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Times(10)

				// 并发调用
				const numGoroutines = 10
				results := make(chan error, numGoroutines)

				for i := 0; i < numGoroutines; i++ {
					go func() {
						ctx := context.Background()
						request := &evaluatorservice.GetEvaluatorRequest{
							WorkspaceID: spaceID,
							EvaluatorID: &evaluatorID,
						}

						resp, err := handler.GetEvaluator(ctx, request)
						if err != nil {
							results <- err
							return
						}

						// 验证响应数据一致性
						if resp.Evaluator.GetEvaluatorID() != evaluatorID {
							results <- fmt.Errorf("inconsistent evaluator ID: expected %d, got %d",
								evaluatorID, resp.Evaluator.GetEvaluatorID())
							return
						}

						results <- nil
					}()
				}

				// 收集结果
				for i := 0; i < numGoroutines; i++ {
					select {
					case err := <-results:
						assert.NoError(t, err)
					case <-time.After(5 * time.Second):
						t.Fatal("Timeout waiting for concurrent calls")
					}
				}
			},
		},
		{
			name: "错误处理和恢复机制测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockEvaluatorRecordService := mocks.NewMockEvaluatorRecordService(ctrl)

				handler := &EvaluatorHandlerImpl{
					auth:                   mockAuth,
					evaluatorService:       mockEvaluatorService,
					evaluatorRecordService: mockEvaluatorRecordService,
				}

				// 测试运行评估器时的错误恢复
				request := &evaluatorservice.RunEvaluatorRequest{
					EvaluatorVersionID: 123,
					WorkspaceID:        456,
					InputData: &evaluatordto.EvaluatorInputData{
						InputFields: map[string]*common.Content{},
					},
				}

				// 第一次调用失败，第二次成功（模拟重试机制）
				callCount := 0
				mockEvaluatorService.EXPECT().
					GetEvaluatorVersion(gomock.Any(), int64(123), false).
					DoAndReturn(func(ctx context.Context, id int64, includeDeleted bool) (*entity.Evaluator, error) {
						callCount++
						if callCount == 1 {
							return nil, errors.New("temporary database error")
						}
						return &entity.Evaluator{
							ID:      1,
							SpaceID: 456,
							Name:    "test-evaluator",
						}, nil
					}).
					Times(2)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					RunEvaluator(gomock.Any(), gomock.Any()).
					Return(&entity.EvaluatorRecord{
						ID:                 789,
						EvaluatorVersionID: 123,
						SpaceID:            456,
					}, nil).
					Times(1)

				ctx := context.Background()

				// 第一次调用应该失败
				resp1, err1 := handler.RunEvaluator(ctx, request)
				assert.Error(t, err1)
				assert.Nil(t, resp1)

				// 第二次调用应该成功
				resp2, err2 := handler.RunEvaluator(ctx, request)
				assert.NoError(t, err2)
				assert.NotNil(t, resp2)
				assert.Equal(t, int64(789), resp2.Record.GetID())
			},
		},
		{
			name: "大数据量处理性能测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)

				handler := &EvaluatorHandlerImpl{
					auth:             mockAuth,
					evaluatorService: mockEvaluatorService,
					userInfoService:  mockUserInfoService,
				}

				// 创建大量评估器数据
				const numEvaluators = 1000
				evaluators := make([]*entity.Evaluator, numEvaluators)
				for i := 0; i < numEvaluators; i++ {
					evaluators[i] = &entity.Evaluator{
						ID:      int64(i + 1),
						SpaceID: 123,
						Name:    fmt.Sprintf("evaluator-%d", i+1),
					}
				}

				request := &evaluatorservice.ListEvaluatorsRequest{
					WorkspaceID: 123,
					PageSize:    gptr.Of(int32(numEvaluators)),
					PageNumber:  gptr.Of(int32(1)),
				}

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					ListEvaluator(gomock.Any(), gomock.Any()).
					Return(evaluators, int64(numEvaluators), nil).
					Times(1)

				mockUserInfoService.EXPECT().
					PackUserInfo(gomock.Any(), gomock.Any()).
					Times(1)

				ctx := context.Background()
				start := time.Now()
				resp, err := handler.ListEvaluators(ctx, request)
				duration := time.Since(start)

				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, int64(numEvaluators), gptr.Indirect(resp.Total))
				assert.Len(t, resp.Evaluators, numEvaluators)
				assert.Less(t, duration, 2*time.Second) // 确保性能合理

				// 验证数据完整性
				for i, evaluatorDTO := range resp.Evaluators {
					assert.Equal(t, int64(i+1), evaluatorDTO.GetEvaluatorID())
					assert.Equal(t, fmt.Sprintf("evaluator-%d", i+1), evaluatorDTO.GetName())
				}
			},
		},
		{
			name: "复杂业务流程端到端测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				// 创建完整的依赖链
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockConfiger := confmocks.NewMockIConfiger(ctrl)
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockEvaluatorRecordService := mocks.NewMockEvaluatorRecordService(ctrl)
				mockMetrics := metricsmock.NewMockEvaluatorExecMetrics(ctrl)
				mockUserInfoService := userinfomocks.NewMockUserInfoService(ctrl)
				mockAuditClient := auditmocks.NewMockIAuditService(ctrl)
				mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
				mockFileProvider := rpcmocks.NewMockIFileProvider(ctrl)

				handler := NewEvaluatorHandlerImpl(
					mockIDGen,
					mockConfiger,
					mockAuth,
					mockEvaluatorService,
					mockEvaluatorRecordService,
					mockMetrics,
					mockUserInfoService,
					mockAuditClient,
					mockBenefitService,
					mockFileProvider,
				)

				// 模拟完整的评估器生命周期：创建 -> 更新 -> 提交版本 -> 运行 -> 删除
				ctx := context.Background()
				spaceID := int64(123)
				evaluatorID := int64(456)

				// 1. 创建评估器
				createRequest := &evaluatorservice.CreateEvaluatorRequest{
					Evaluator: &evaluatordto.Evaluator{
						WorkspaceID:   gptr.Of(spaceID),
						Name:          gptr.Of("端到端测试评估器"),
						Description:   gptr.Of("用于端到端测试的评估器"),
						EvaluatorType: gptr.Of(evaluatordto.EvaluatorType_Prompt),
						CurrentVersion: &evaluatordto.EvaluatorVersion{
							Version: gptr.Of("1.0.0"),
							EvaluatorContent: &evaluatordto.EvaluatorContent{
								PromptEvaluator: &evaluatordto.PromptEvaluator{},
							},
						},
					},
				}

				// Mock 创建流程
				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).
					Times(1)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(spaceID, 10),
						SpaceID:       spaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
					}).
					Return(nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					CreateEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(evaluatorID, nil).
					Times(1)

				mockMetrics.EXPECT().
					EmitCreate(spaceID, nil).
					Times(1)

				createResp, err := handler.CreateEvaluator(ctx, createRequest)
				assert.NoError(t, err)
				assert.Equal(t, evaluatorID, gptr.Indirect(createResp.EvaluatorID))

				// 2. 更新评估器
				updateRequest := &evaluatorservice.UpdateEvaluatorRequest{
					WorkspaceID: spaceID,
					EvaluatorID: evaluatorID,
					Name:        gptr.Of("更新后的评估器"),
					Description: gptr.Of("更新后的描述"),
				}

				evaluator := &entity.Evaluator{
					ID:      evaluatorID,
					SpaceID: spaceID,
					Name:    "端到端测试评估器",
				}

				mockEvaluatorService.EXPECT().
					GetEvaluator(gomock.Any(), spaceID, evaluatorID, false).
					Return(evaluator, nil).
					Times(1)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(evaluatorID, 10),
						SpaceID:       spaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil).
					Times(1)

				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					UpdateEvaluatorMeta(gomock.Any(), evaluatorID, spaceID, "更新后的评估器", "更新后的描述", gomock.Any()).
					Return(nil).
					Times(1)

				updateResp, err := handler.UpdateEvaluator(ctx, updateRequest)
				assert.NoError(t, err)
				assert.NotNil(t, updateResp)

				// 3. 删除评估器
				deleteRequest := &evaluatorservice.DeleteEvaluatorRequest{
					WorkspaceID: spaceID,
					EvaluatorID: &evaluatorID,
				}

				mockEvaluatorService.EXPECT().
					BatchGetEvaluator(gomock.Any(), spaceID, []int64{evaluatorID}, false).
					Return([]*entity.Evaluator{evaluator}, nil).
					Times(1)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), &rpc.AuthorizationParam{
						ObjectID:      strconv.FormatInt(evaluatorID, 10),
						SpaceID:       spaceID,
						ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
					}).
					Return(nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					DeleteEvaluator(gomock.Any(), []int64{evaluatorID}, gomock.Any()).
					Return(nil).
					Times(1)

				deleteResp, err := handler.DeleteEvaluator(ctx, deleteRequest)
				assert.NoError(t, err)
				assert.NotNil(t, deleteResp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

// TestEvaluatorHandlerImpl_EdgeCasesAndBoundaryConditions 测试边界条件
func TestEvaluatorHandlerImpl_EdgeCasesAndBoundaryConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "空请求和 nil 参数处理",
			testFunc: func(t *testing.T) {
				t.Parallel()

				handler := &EvaluatorHandlerImpl{}
				ctx := context.Background()

				// 测试各种 nil 请求
				_, err1 := handler.CreateEvaluator(ctx, nil)
				assert.Error(t, err1)

				_, err2 := handler.UpdateEvaluator(ctx, nil)
				assert.Error(t, err2)

				_, err3 := handler.CreateEvaluator(ctx, &evaluatorservice.CreateEvaluatorRequest{Evaluator: nil})
				assert.Error(t, err3)
			},
		},
		{
			name: "超长字符串处理",
			testFunc: func(t *testing.T) {
				t.Parallel()

				handler := &EvaluatorHandlerImpl{}
				ctx := context.Background()

				// 创建超长名称
				longName := string(make([]rune, consts.MaxEvaluatorNameLength+100))
				longDesc := string(make([]rune, consts.MaxEvaluatorDescLength+100))

				request := &evaluatorservice.CreateEvaluatorRequest{
					Evaluator: &evaluatordto.Evaluator{
						WorkspaceID:   gptr.Of(int64(123)),
						Name:          gptr.Of(longName),
						Description:   gptr.Of(longDesc),
						EvaluatorType: gptr.Of(evaluatordto.EvaluatorType_Prompt),
						CurrentVersion: &evaluatordto.EvaluatorVersion{
							Version: gptr.Of("1.0.0"),
							EvaluatorContent: &evaluatordto.EvaluatorContent{
								PromptEvaluator: &evaluatordto.PromptEvaluator{},
							},
						},
					},
				}

				_, err := handler.CreateEvaluator(ctx, request)
				assert.Error(t, err)

				// 验证错误类型
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, int32(errno.EvaluatorNameExceedMaxLengthCode), statusErr.Code())
			},
		},
		{
			name: "特殊字符和编码处理",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)
				mockAuditClient := auditmocks.NewMockIAuditService(ctrl)
				mockMetrics := metricsmock.NewMockEvaluatorExecMetrics(ctrl)

				handler := &EvaluatorHandlerImpl{
					auth:             mockAuth,
					evaluatorService: mockEvaluatorService,
					auditClient:      mockAuditClient,
					metrics:          mockMetrics,
				}

				// 包含各种特殊字符的请求
				request := &evaluatorservice.CreateEvaluatorRequest{
					Evaluator: &evaluatordto.Evaluator{
						WorkspaceID:   gptr.Of(int64(123)),
						Name:          gptr.Of("测试🚀评估器💡"),
						Description:   gptr.Of("包含emoji和特殊字符的描述：<>&\"'"),
						EvaluatorType: gptr.Of(evaluatordto.EvaluatorType_Prompt),
						CurrentVersion: &evaluatordto.EvaluatorVersion{
							Version: gptr.Of("1.0.0"),
							EvaluatorContent: &evaluatordto.EvaluatorContent{
								PromptEvaluator: &evaluatordto.PromptEvaluator{},
							},
						},
					},
				}

				mockAuditClient.EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, param audit.AuditParam) (audit.AuditRecord, error) {
						// 验证特殊字符被正确处理
						assert.Contains(t, param.AuditData["texts"], "测试🚀评估器💡")
						return audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil
					}).
					Times(1)

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					Return(nil).
					Times(1)

				mockEvaluatorService.EXPECT().
					CreateEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(123), nil).
					Times(1)

				mockMetrics.EXPECT().
					EmitCreate(gomock.Any(), gomock.Any()).
					Times(1)

				ctx := context.Background()
				resp, err := handler.CreateEvaluator(ctx, request)

				assert.NoError(t, err)
				assert.NotNil(t, resp)
			},
		},
		{
			name: "上下文取消和超时处理",
			testFunc: func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockEvaluatorService := mocks.NewMockEvaluatorService(ctrl)

				handler := &EvaluatorHandlerImpl{
					auth:             mockAuth,
					evaluatorService: mockEvaluatorService,
				}

				// 创建已取消的上下文
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				request := &evaluatorservice.ListEvaluatorsRequest{
					WorkspaceID: 123,
				}

				mockAuth.EXPECT().
					Authorization(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, param *rpc.AuthorizationParam) error {
						// 检查上下文是否已取消
						select {
						case <-ctx.Done():
							return ctx.Err()
						default:
							return nil
						}
					}).
					Times(1)

				_, err := handler.ListEvaluators(ctx, request)
				assert.Error(t, err)
				assert.Equal(t, context.Canceled, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

func TestEvaluatorHandlerImpl_GetTemplateInfoResponse(t *testing.T) {
	// This test was accidentally merged, removing content
	t.Skip("Duplicate test content removed")
}
