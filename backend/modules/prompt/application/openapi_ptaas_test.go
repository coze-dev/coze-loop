// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/cozeloop-go"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	limitermocks "github.com/coze-dev/coze-loop/backend/infra/limiter/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/collector"
	collectormocks "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/collector/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func TestPromptOpenAPIApplicationImpl_PTaaSExecute(t *testing.T) {
	t.Parallel()

	type fields struct {
		promptService    service.IPromptService
		promptManageRepo repo.IManageRepo
		config           conf.IConfigProvider
		auth             rpc.IAuthProvider
		rateLimiter      limiter.IRateLimiter
		collector        collector.ICollectorProvider
	}
	type args struct {
		ctx context.Context
		req *openapi.ExecuteRequest
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantR        *openapi.ExecuteResponse
		wantErr      error
	}{
		{
			name: "success: execute prompt",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)
				mockPromptService.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(&entity.Reply{
					Item: &entity.ReplyItem{
						Message: &entity.Message{
							Role:    entity.RoleAssistant,
							Content: ptr.Of("Hello! How can I help you?"),
						},
						FinishReason: "stop",
						TokenUsage: &entity.TokenUsage{
							InputTokens:  10,
							OutputTokens: 8,
						},
					},
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: {
						ID:        123,
						SpaceID:   123456,
						PromptKey: "test_prompt",
						PromptBasic: &entity.PromptBasic{
							DisplayName:   "Test Prompt",
							Description:   "Test Description",
							LatestVersion: "1.0.0",
							CreatedBy:     "test_user",
							UpdatedBy:     "test_user",
							CreatedAt:     startTime,
							UpdatedAt:     startTime,
						},
						PromptCommit: &entity.PromptCommit{
							CommitInfo: &entity.CommitInfo{
								Version:     "1.0.0",
								BaseVersion: "",
								Description: "Initial version",
								CommittedBy: "test_user",
								CommittedAt: startTime,
							},
							PromptDetail: &entity.PromptDetail{
								PromptTemplate: &entity.PromptTemplate{
									TemplateType: entity.TemplateTypeNormal,
									Messages: []*entity.Message{
										{
											Role:    entity.RoleSystem,
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
								},
								ModelConfig: &entity.ModelConfig{
									ModelID:     123,
									Temperature: ptr.Of(0.7),
								},
							},
						},
					},
				}, nil)

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPTaaSEvent(gomock.Any(), gomock.Any()).Return()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					collector:        mockCollector,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ExecuteRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PromptIdentifier: &openapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
						Version:   ptr.Of("1.0.0"),
					},
					Messages: []*openapi.Message{
						{
							Role:    ptr.Of(prompt.RoleUser),
							Content: ptr.Of("Hello"),
						},
					},
				},
			},
			wantR: &openapi.ExecuteResponse{
				Data: &openapi.ExecuteData{
					Message: &openapi.Message{
						Role:    ptr.Of(prompt.RoleAssistant),
						Content: ptr.Of("Hello! How can I help you?"),
					},
					FinishReason: ptr.Of("stop"),
					Usage: &openapi.TokenUsage{
						InputTokens:  ptr.Of(int32(10)),
						OutputTokens: ptr.Of(int32(8)),
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "error: invalid workspace_id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPTaaSEvent(gomock.Any(), gomock.Any()).Return()
				return fields{
					collector: mockCollector,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ExecuteRequest{
					WorkspaceID: ptr.Of(int64(0)),
					PromptIdentifier: &openapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
					},
				},
			},
			wantR:   openapi.NewExecuteResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"})),
		},
		{
			name: "error: prompt_key is empty",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPTaaSEvent(gomock.Any(), gomock.Any()).Return()
				return fields{
					collector: mockCollector,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ExecuteRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PromptIdentifier: &openapi.PromptQuery{
						PromptKey: ptr.Of(""),
					},
				},
			},
			wantR:   openapi.NewExecuteResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "prompt_key参数为空"})),
		},
		{
			name: "error: rate limit exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(1, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: false,
				}, nil)

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPTaaSEvent(gomock.Any(), gomock.Any()).Return()

				return fields{
					config:      mockConfig,
					rateLimiter: mockRateLimiter,
					collector:   mockCollector,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ExecuteRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PromptIdentifier: &openapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
						Version:   ptr.Of("1.0.0"),
					},
					Messages: []*openapi.Message{
						{
							Role:    ptr.Of(prompt.RoleUser),
							Content: ptr.Of("Hello"),
						},
					},
				},
			},
			wantR:   openapi.NewExecuteResponse(),
			wantErr: errorx.NewByCode(prompterr.PTaaSQPSLimitCode, errorx.WithExtraMsg("qps limit exceeded")),
		},
		{
			name: "error: permission denied",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: {
						ID:        123,
						SpaceID:   123456,
						PromptKey: "test_prompt",
						PromptBasic: &entity.PromptBasic{
							DisplayName:   "Test Prompt",
							LatestVersion: "1.0.0",
							CreatedAt:     startTime,
							UpdatedAt:     startTime,
						},
						PromptCommit: &entity.PromptCommit{
							CommitInfo: &entity.CommitInfo{
								Version: "1.0.0",
							},
						},
					},
				}, nil)

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(
					errorx.NewByCode(prompterr.CommonNoPermissionCode))

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPTaaSEvent(gomock.Any(), gomock.Any()).Return()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					collector:        mockCollector,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ExecuteRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PromptIdentifier: &openapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
						Version:   ptr.Of("1.0.0"),
					},
					Messages: []*openapi.Message{
						{
							Role:    ptr.Of(prompt.RoleUser),
							Content: ptr.Of("Hello"),
						},
					},
				},
			},
			wantR:   openapi.NewExecuteResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "error: execute failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)
				mockPromptService.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, errors.New("execution failed"))

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: {
						ID:        123,
						SpaceID:   123456,
						PromptKey: "test_prompt",
						PromptBasic: &entity.PromptBasic{
							DisplayName:   "Test Prompt",
							LatestVersion: "1.0.0",
							CreatedAt:     startTime,
							UpdatedAt:     startTime,
						},
						PromptCommit: &entity.PromptCommit{
							CommitInfo: &entity.CommitInfo{
								Version: "1.0.0",
							},
						},
					},
				}, nil)

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPTaaSEvent(gomock.Any(), gomock.Any()).Return()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					collector:        mockCollector,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ExecuteRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PromptIdentifier: &openapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
						Version:   ptr.Of("1.0.0"),
					},
					Messages: []*openapi.Message{
						{
							Role:    ptr.Of(prompt.RoleUser),
							Content: ptr.Of("Hello"),
						},
					},
				},
			},
			wantR:   openapi.NewExecuteResponse(),
			wantErr: errors.New("execution failed"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				promptService:    ttFields.promptService,
				promptManageRepo: ttFields.promptManageRepo,
				config:           ttFields.config,
				auth:             ttFields.auth,
				rateLimiter:      ttFields.rateLimiter,
				collector:        ttFields.collector,
			}
			gotR, err := p.Execute(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil && tt.wantR != nil && gotR != nil {
				if tt.wantR.Data != nil && gotR.Data != nil {
					if tt.wantR.Data.Message != nil && gotR.Data.Message != nil {
						assert.Equal(t, tt.wantR.Data.Message.GetRole(), gotR.Data.Message.GetRole())
						assert.Equal(t, tt.wantR.Data.Message.GetContent(), gotR.Data.Message.GetContent())
					}
					if tt.wantR.Data.FinishReason != nil && gotR.Data.FinishReason != nil {
						assert.Equal(t, tt.wantR.Data.GetFinishReason(), gotR.Data.GetFinishReason())
					}
					if tt.wantR.Data.Usage != nil && gotR.Data.Usage != nil {
						assert.Equal(t, tt.wantR.Data.Usage.GetInputTokens(), gotR.Data.Usage.GetInputTokens())
						assert.Equal(t, tt.wantR.Data.Usage.GetOutputTokens(), gotR.Data.Usage.GetOutputTokens())
					}
				}
			} else {
				assert.Equal(t, tt.wantR, gotR)
			}
		})
	}
}

func TestPromptOpenAPIApplicationImpl_PTaaSdoExecute(t *testing.T) {
	t.Parallel()

	type fields struct {
		promptService    service.IPromptService
		promptManageRepo repo.IManageRepo
		config           conf.IConfigProvider
		auth             rpc.IAuthProvider
		rateLimiter      limiter.IRateLimiter
	}
	type args struct {
		ctx context.Context
		req *openapi.ExecuteRequest
	}

	tests := []struct {
		name           string
		fieldsGetter   func(ctrl *gomock.Controller) fields
		args           args
		wantPromptDO   *entity.Prompt
		wantReply      *entity.Reply
		wantErr        error
	}{
		{
			name: "success: execute prompt",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)
				expectedReply := &entity.Reply{
					Item: &entity.ReplyItem{
						Message: &entity.Message{
							Role:    entity.RoleAssistant,
							Content: ptr.Of("Hello! How can I help you?"),
						},
						FinishReason: "stop",
						TokenUsage: &entity.TokenUsage{
							InputTokens:  10,
							OutputTokens: 8,
						},
					},
				}
				mockPromptService.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(expectedReply, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				expectedPrompt := &entity.Prompt{
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_prompt",
					PromptBasic: &entity.PromptBasic{
						DisplayName:   "Test Prompt",
						LatestVersion: "1.0.0",
						CreatedAt:     startTime,
						UpdatedAt:     startTime,
					},
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{
							Version: "1.0.0",
						},
					},
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ExecuteRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PromptIdentifier: &openapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
						Version:   ptr.Of("1.0.0"),
					},
					Messages: []*openapi.Message{
						{
							Role:    ptr.Of(prompt.RoleUser),
							Content: ptr.Of("Hello"),
						},
					},
				},
			},
			wantPromptDO: &entity.Prompt{
				ID:        123,
				SpaceID:   123456,
				PromptKey: "test_prompt",
			},
			wantReply: &entity.Reply{
				Item: &entity.ReplyItem{
					Message: &entity.Message{
						Role:    entity.RoleAssistant,
						Content: ptr.Of("Hello! How can I help you?"),
					},
					FinishReason: "stop",
					TokenUsage: &entity.TokenUsage{
						InputTokens:  10,
						OutputTokens: 8,
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "error: rate limit exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(1, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: false,
				}, nil)

				return fields{
					config:      mockConfig,
					rateLimiter: mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ExecuteRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PromptIdentifier: &openapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
						Version:   ptr.Of("1.0.0"),
					},
				},
			},
			wantPromptDO: &entity.Prompt{},
			wantReply:    nil,
			wantErr:      errorx.NewByCode(prompterr.PTaaSQPSLimitCode, errorx.WithExtraMsg("qps limit exceeded")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				promptService:    ttFields.promptService,
				promptManageRepo: ttFields.promptManageRepo,
				config:           ttFields.config,
				auth:             ttFields.auth,
				rateLimiter:      ttFields.rateLimiter,
			}
			gotPromptDO, gotReply, err := p.doExecute(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.wantPromptDO.ID, gotPromptDO.ID)
				assert.Equal(t, tt.wantPromptDO.SpaceID, gotPromptDO.SpaceID)
				assert.Equal(t, tt.wantPromptDO.PromptKey, gotPromptDO.PromptKey)
				if tt.wantReply != nil && gotReply != nil && tt.wantReply.Item != nil && gotReply.Item != nil {
					assert.Equal(t, tt.wantReply.Item.FinishReason, gotReply.Item.FinishReason)
					assert.Equal(t, tt.wantReply.Item.TokenUsage.InputTokens, gotReply.Item.TokenUsage.InputTokens)
					assert.Equal(t, tt.wantReply.Item.TokenUsage.OutputTokens, gotReply.Item.TokenUsage.OutputTokens)
				}
			}
		})
	}
}

func TestPromptOpenAPIApplicationImpl_PTaaSStartPromptExecutorSpan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		param ptaasStartPromptExecutorSpanParam
	}{
		{
			name: "success: start span with valid parameters",
			param: ptaasStartPromptExecutorSpanParam{
				workspaceID:      123456,
				stream:           false,
				reqPromptKey:     "test_prompt",
				reqPromptVersion: "1.0.0",
				reqPromptLabel:   "",
				messages: []*entity.Message{
					{
						Role:    entity.RoleUser,
						Content: ptr.Of("Hello"),
					},
				},
				variableVals: []*entity.VariableVal{
					{
						Key:   "test_var",
						Value: ptr.Of("test_value"),
					},
				},
			},
		},
		{
			name: "success: start span with stream enabled",
			param: ptaasStartPromptExecutorSpanParam{
				workspaceID:      123456,
				stream:           true,
				reqPromptKey:     "test_prompt",
				reqPromptVersion: "1.0.0",
				reqPromptLabel:   "stable",
				messages:         []*entity.Message{},
				variableVals:     []*entity.VariableVal{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &PromptOpenAPIApplicationImpl{}
			ctx, span := p.startPromptExecutorSpan(context.Background(), tt.param)
			assert.NotNil(t, ctx)
			// span可能为nil，这取决于tracer的实现
			_ = span
		})
	}
}

func TestPromptOpenAPIApplicationImpl_PTaaSFinishPromptExecutorSpan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		span   cozeloop.Span
		prompt *entity.Prompt
		reply  *entity.Reply
		err    error
	}{
		{
			name:   "success: finish span with valid data",
			span:   nil, // 在实际测试中，span可能为nil
			prompt: &entity.Prompt{
				PromptKey: "test_prompt",
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version: "1.0.0",
					},
				},
			},
			reply: &entity.Reply{
				DebugID: 123,
				Item: &entity.ReplyItem{
					TokenUsage: &entity.TokenUsage{
						InputTokens:  10,
						OutputTokens: 8,
					},
				},
			},
			err: nil,
		},
		{
			name:   "handle nil span",
			span:   nil,
			prompt: nil,
			reply:  nil,
			err:    nil,
		},
		{
			name:   "handle nil prompt",
			span:   nil,
			prompt: nil,
			reply: &entity.Reply{
				Item: &entity.ReplyItem{},
			},
			err: nil,
		},
		{
			name:   "handle error case",
			span:   nil,
			prompt: &entity.Prompt{
				PromptKey: "test_prompt",
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version: "1.0.0",
					},
				},
			},
			reply: nil,
			err:   errors.New("execution error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &PromptOpenAPIApplicationImpl{}
			// finishPromptExecutorSpan不返回值，我们只需要确保它不会panic
			p.finishPromptExecutorSpan(context.Background(), tt.span, tt.prompt, tt.reply, tt.err)
		})
	}
}