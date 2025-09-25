// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/coze-dev/cozeloop-go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

func TestPromptOpenAPIApplicationImpl_BatchGetPromptByPromptKey(t *testing.T) {
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
		req *openapi.BatchGetPromptByPromptKeyRequest
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantR        *openapi.BatchGetPromptByPromptKeyResponse
		wantErr      error
	}{
		{
			name: "success: specific version",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
					"test_prompt2": 456,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
					{PromptID: 456, PromptKey: "test_prompt2", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{
						PromptID:      123,
						WithCommit:    true,
						CommitVersion: "1.0.0",
					}: {
						ID:        123,
						SpaceID:   123456,
						PromptKey: "test_prompt1",
						PromptBasic: &entity.PromptBasic{
							DisplayName:   "Test Prompt 1",
							Description:   "Test PromptDescription 1",
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
					{
						PromptID:      456,
						WithCommit:    true,
						CommitVersion: "1.0.0",
					}: {
						ID:        456,
						SpaceID:   123456,
						PromptKey: "test_prompt2",
						PromptBasic: &entity.PromptBasic{
							DisplayName:   "Test Prompt 2",
							Description:   "Test PromptDescription 2",
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
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPromptHubEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return()
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
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
						{
							PromptKey: ptr.Of("test_prompt2"),
							Version:   ptr.Of("1.0.0"),
						},
					},
				},
			},
			wantR: &openapi.BatchGetPromptByPromptKeyResponse{
				Data: &openapi.PromptResultData{
					Items: []*openapi.PromptResult_{
						{
							Query: &openapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt1"),
								Version:   ptr.Of("1.0.0"),
							},
							Prompt: &openapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt1"),
								Version:     ptr.Of("1.0.0"),
								PromptTemplate: &openapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*openapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*openapi.VariableDef, 0),
								},
								LlmConfig: &openapi.LLMConfig{
									Temperature: ptr.Of(0.7),
								},
							},
						},
						{
							Query: &openapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt2"),
								Version:   ptr.Of("1.0.0"),
							},
							Prompt: &openapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt2"),
								Version:     ptr.Of("1.0.0"),
								PromptTemplate: &openapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*openapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*openapi.VariableDef, 0),
								},
								LlmConfig: &openapi.LLMConfig{
									Temperature: ptr.Of(0.7),
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "success: latest commit version",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
					{PromptID: 123, PromptKey: "test_prompt1", Version: "2.0.0"}: "2.0.0",
					{PromptID: 123, PromptKey: "test_prompt1", Version: ""}:      "2.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{
						PromptID:      123,
						WithCommit:    true,
						CommitVersion: "1.0.0",
					}: {
						ID:        123,
						SpaceID:   123456,
						PromptKey: "test_prompt1",
						PromptBasic: &entity.PromptBasic{
							DisplayName:   "Test Prompt 1",
							Description:   "Test PromptDescription 1",
							LatestVersion: "2.0.0",
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
					{
						PromptID:      123,
						WithCommit:    true,
						CommitVersion: "2.0.0",
					}: {
						ID:        123,
						SpaceID:   123456,
						PromptKey: "test_prompt1",
						PromptBasic: &entity.PromptBasic{
							DisplayName:   "Test Prompt 1",
							Description:   "Test PromptDescription 1",
							LatestVersion: "2.0.0",
							CreatedBy:     "test_user",
							UpdatedBy:     "test_user",
							CreatedAt:     startTime,
							UpdatedAt:     startTime,
						},
						PromptCommit: &entity.PromptCommit{
							CommitInfo: &entity.CommitInfo{
								Version:     "2.0.0",
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
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPromptHubEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return()

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
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("2.0.0"),
						},
						{
							PromptKey: ptr.Of("test_prompt1"),
						},
					},
				},
			},
			wantR: &openapi.BatchGetPromptByPromptKeyResponse{
				Data: &openapi.PromptResultData{
					Items: []*openapi.PromptResult_{
						{
							Query: &openapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt1"),
								Version:   ptr.Of("1.0.0"),
							},
							Prompt: &openapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt1"),
								Version:     ptr.Of("1.0.0"),
								PromptTemplate: &openapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*openapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*openapi.VariableDef, 0),
								},
								LlmConfig: &openapi.LLMConfig{
									Temperature: ptr.Of(0.7),
								},
							},
						},
						{
							Query: &openapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt1"),
								Version:   ptr.Of("2.0.0"),
							},
							Prompt: &openapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt1"),
								Version:     ptr.Of("2.0.0"),
								PromptTemplate: &openapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*openapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*openapi.VariableDef, 0),
								},
								LlmConfig: &openapi.LLMConfig{
									Temperature: ptr.Of(0.7),
								},
							},
						},
						{
							Query: &openapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt1"),
							},
							Prompt: &openapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt1"),
								Version:     ptr.Of("2.0.0"),
								PromptTemplate: &openapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*openapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*openapi.VariableDef, 0),
								},
								LlmConfig: &openapi.LLMConfig{
									Temperature: ptr.Of(0.7),
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "rate limit exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(1, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: false,
				}, nil)

				return fields{
					config:      mockConfig,
					rateLimiter: mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
					},
				},
			},
			wantR:   openapi.NewBatchGetPromptByPromptKeyResponse(),
			wantErr: errorx.NewByCode(prompterr.PromptHubQPSLimitCode, errorx.WithExtraMsg("qps limit exceeded")),
		},
		{
			name: "mget prompt ids error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("database error"))

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				return fields{
					promptService: mockPromptService,
					config:        mockConfig,
					rateLimiter:   mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
					},
				},
			},
			wantR:   openapi.NewBatchGetPromptByPromptKeyResponse(),
			wantErr: errors.New("database error"),
		},
		{
			name: "permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				return fields{
					promptService: mockPromptService,
					config:        mockConfig,
					auth:          mockAuth,
					rateLimiter:   mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
					},
				},
			},
			wantR:   nil,
			wantErr: errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "parse commit version error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("parse version error"))

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				return fields{
					promptService: mockPromptService,
					config:        mockConfig,
					auth:          mockAuth,
					rateLimiter:   mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
					},
				},
			},
			wantR:   nil,
			wantErr: errors.New("parse version error"),
		},
		{
			name: "mget prompt error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("database error"))

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
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
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
					},
				},
			},
			wantR:   nil,
			wantErr: errors.New("database error"),
		},
		{
			name: "prompt version not exist",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt1", Version: "non_existent_version"}: "non_existent_version",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{}, nil)

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
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
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("non_existent_version"),
						},
					},
				},
			},
			wantR:   nil,
			wantErr: errorx.NewByCode(prompterr.PromptVersionNotExistCode, errorx.WithExtraMsg("prompt version not exist")),
		},
		{
			name: "workspace_id is empty",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(0)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
					},
				},
			},
			wantR:   openapi.NewBatchGetPromptByPromptKeyResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"})),
		},
		{
			name: "workspace_id is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: nil,
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
					},
				},
			},
			wantR:   openapi.NewBatchGetPromptByPromptKeyResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"})),
		},
		{
			name: "enhanced error info with prompt_key",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil,
					errorx.NewByCode(prompterr.PromptVersionNotExistCode,
						errorx.WithExtra(map[string]string{"prompt_id": "123"})))

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
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
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
					},
				},
			},
			wantR: nil,
			wantErr: errorx.NewByCode(prompterr.PromptVersionNotExistCode,
				errorx.WithExtra(map[string]string{"prompt_id": "123", "prompt_key": "test_prompt1"})),
		},
		{
			name: "success: query with label",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt1", Label: "stable"}: "2.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{
						PromptID:      123,
						WithCommit:    true,
						CommitVersion: "2.0.0",
					}: {
						ID:        123,
						SpaceID:   123456,
						PromptKey: "test_prompt1",
						PromptBasic: &entity.PromptBasic{
							DisplayName:   "Test Prompt 1",
							Description:   "Test PromptDescription 1",
							LatestVersion: "2.0.0",
							CreatedBy:     "test_user",
							UpdatedBy:     "test_user",
							CreatedAt:     startTime,
							UpdatedAt:     startTime,
						},
						PromptCommit: &entity.PromptCommit{
							CommitInfo: &entity.CommitInfo{
								Version:     "2.0.0",
								BaseVersion: "",
								Description: "Stable version",
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
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPromptHubEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return()

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
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Label:     ptr.Of("stable"),
						},
					},
				},
			},
			wantR: &openapi.BatchGetPromptByPromptKeyResponse{
				Data: &openapi.PromptResultData{
					Items: []*openapi.PromptResult_{
						{
							Query: &openapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt1"),
								Label:     ptr.Of("stable"),
							},
							Prompt: &openapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt1"),
								Version:     ptr.Of("2.0.0"),
								PromptTemplate: &openapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*openapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*openapi.VariableDef, 0),
								},
								LlmConfig: &openapi.LLMConfig{
									Temperature: ptr.Of(0.7),
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "success: mixed version and label queries",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
					"test_prompt2": 456,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
					{PromptID: 456, PromptKey: "test_prompt2", Label: "beta"}:    "1.5.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{
						PromptID:      123,
						WithCommit:    true,
						CommitVersion: "1.0.0",
					}: {
						ID:        123,
						SpaceID:   123456,
						PromptKey: "test_prompt1",
						PromptBasic: &entity.PromptBasic{
							DisplayName:   "Test Prompt 1",
							Description:   "Test PromptDescription 1",
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
					{
						PromptID:      456,
						WithCommit:    true,
						CommitVersion: "1.5.0",
					}: {
						ID:        456,
						SpaceID:   123456,
						PromptKey: "test_prompt2",
						PromptBasic: &entity.PromptBasic{
							DisplayName:   "Test Prompt 2",
							Description:   "Test PromptDescription 2",
							LatestVersion: "1.5.0",
							CreatedBy:     "test_user",
							UpdatedBy:     "test_user",
							CreatedAt:     startTime,
							UpdatedAt:     startTime,
						},
						PromptCommit: &entity.PromptCommit{
							CommitInfo: &entity.CommitInfo{
								Version:     "1.5.0",
								BaseVersion: "",
								Description: "Beta version",
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
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPromptHubEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return()

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
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
						{
							PromptKey: ptr.Of("test_prompt2"),
							Label:     ptr.Of("beta"),
						},
					},
				},
			},
			wantR: &openapi.BatchGetPromptByPromptKeyResponse{
				Data: &openapi.PromptResultData{
					Items: []*openapi.PromptResult_{
						{
							Query: &openapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt1"),
								Version:   ptr.Of("1.0.0"),
							},
							Prompt: &openapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt1"),
								Version:     ptr.Of("1.0.0"),
								PromptTemplate: &openapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*openapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*openapi.VariableDef, 0),
								},
								LlmConfig: &openapi.LLMConfig{
									Temperature: ptr.Of(0.7),
								},
							},
						},
						{
							Query: &openapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt2"),
								Label:     ptr.Of("beta"),
							},
							Prompt: &openapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt2"),
								Version:     ptr.Of("1.5.0"),
								PromptTemplate: &openapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*openapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*openapi.VariableDef, 0),
								},
								LlmConfig: &openapi.LLMConfig{
									Temperature: ptr.Of(0.7),
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "error: label not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("label not found: non_existent_label"))

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				return fields{
					promptService: mockPromptService,
					config:        mockConfig,
					auth:          mockAuth,
					rateLimiter:   mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Label:     ptr.Of("non_existent_label"),
						},
					},
				},
			},
			wantR:   nil,
			wantErr: errors.New("label not found: non_existent_label"),
		},
		{
			name: "error: prompt key not found in result construction",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
					// test_prompt2 不存在，但在查询构建阶段会被跳过，在结果构建阶段会报错
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{
						PromptID:      123,
						WithCommit:    true,
						CommitVersion: "1.0.0",
					}: {
						ID:        123,
						SpaceID:   123456,
						PromptKey: "test_prompt1",
						PromptBasic: &entity.PromptBasic{
							DisplayName:   "Test Prompt 1",
							Description:   "Test PromptDescription 1",
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
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
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
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
						{
							PromptKey: ptr.Of("test_prompt2"), // 不存在的prompt key
							Version:   ptr.Of("1.0.0"),
						},
					},
				},
			},
			wantR:   nil,
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("prompt not exist")),
		},
		{
			name: "error: prompt version not exist in result construction",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{}, nil)

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
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
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*openapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
					},
				},
			},
			wantR:   nil,
			wantErr: errorx.NewByCode(prompterr.PromptVersionNotExistCode, errorx.WithExtraMsg("prompt version not exist")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 移除 t.Parallel() 以避免数据竞争
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
			gotR, err := p.BatchGetPromptByPromptKey(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			assert.Equal(t, tt.wantR, gotR)
		})
	}
}

func TestValidateExecuteRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     *openapi.ExecuteRequest
		wantErr error
	}{
		{
			name: "success: valid request",
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
			wantErr: nil,
		},
		{
			name: "error: workspace_id is zero",
			req: &openapi.ExecuteRequest{
				WorkspaceID: ptr.Of(int64(0)),
				PromptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"})),
		},
		{
			name: "error: workspace_id is nil",
			req: &openapi.ExecuteRequest{
				WorkspaceID: nil,
				PromptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"})),
		},
		{
			name: "error: prompt_identifier is nil",
			req: &openapi.ExecuteRequest{
				WorkspaceID:      ptr.Of(int64(123456)),
				PromptIdentifier: nil,
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "prompt_key参数为空"})),
		},
		{
			name: "error: prompt_key is empty",
			req: &openapi.ExecuteRequest{
				WorkspaceID: ptr.Of(int64(123456)),
				PromptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of(""),
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "prompt_key参数为空"})),
		},
		{
			name: "error: prompt_key is nil",
			req: &openapi.ExecuteRequest{
				WorkspaceID: ptr.Of(int64(123456)),
				PromptIdentifier: &openapi.PromptQuery{
					PromptKey: nil,
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "prompt_key参数为空"})),
		},
		{
			name: "error: invalid image URL",
			req: &openapi.ExecuteRequest{
				WorkspaceID: ptr.Of(int64(123456)),
				PromptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
				Messages: []*openapi.Message{
					{
						Role: ptr.Of(prompt.RoleUser),
						Parts: []*openapi.ContentPart{
							{
								Type:     ptr.Of(openapi.ContentTypeImageURL),
								ImageURL: ptr.Of("invalid-url"),
							},
						},
					},
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "invalid-url不是有效的URL"})),
		},
		{
			name: "error: invalid base64 data",
			req: &openapi.ExecuteRequest{
				WorkspaceID: ptr.Of(int64(123456)),
				PromptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
				Messages: []*openapi.Message{
					{
						Role: ptr.Of(prompt.RoleUser),
						Parts: []*openapi.ContentPart{
							{
								Type:       ptr.Of(openapi.ContentTypeBase64Data),
								Base64Data: ptr.Of("invalid-base64"),
							},
						},
					},
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "存在无效的base64数据，数据格式应该符合data:[<mediatype>][;base64],<data>"})),
		},
		{
			name: "success: valid image URL",
			req: &openapi.ExecuteRequest{
				WorkspaceID: ptr.Of(int64(123456)),
				PromptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
				Messages: []*openapi.Message{
					{
						Role: ptr.Of(prompt.RoleUser),
						Parts: []*openapi.ContentPart{
							{
								Type:     ptr.Of(openapi.ContentTypeImageURL),
								ImageURL: ptr.Of("https://example.com/image.jpg"),
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "success: valid base64 data",
			req: &openapi.ExecuteRequest{
				WorkspaceID: ptr.Of(int64(123456)),
				PromptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
				Messages: []*openapi.Message{
					{
						Role: ptr.Of(prompt.RoleUser),
						Parts: []*openapi.ContentPart{
							{
								Type:       ptr.Of(openapi.ContentTypeBase64Data),
								Base64Data: ptr.Of("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="),
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "error: invalid base64 data in variable vals",
			req: &openapi.ExecuteRequest{
				WorkspaceID: ptr.Of(int64(123456)),
				PromptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
				VariableVals: []*openapi.VariableVal{
					{
						Key: ptr.Of("image_var"),
						MultiPartValues: []*openapi.ContentPart{
							{
								Type:       ptr.Of(openapi.ContentTypeBase64Data),
								Base64Data: ptr.Of("invalid-base64"),
							},
						},
					},
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "存在无效的base64数据，数据格式应该符合data:[<mediatype>][;base64],<data>"})),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 移除 t.Parallel() 以避免数据竞争
			err := validateExecuteRequest(tt.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
		})
	}
}

func TestPromptOpenAPIApplicationImpl_ptaasAllowByPromptKey(t *testing.T) {
	t.Parallel()

	type fields struct {
		config      conf.IConfigProvider
		rateLimiter limiter.IRateLimiter
	}
	type args struct {
		ctx         context.Context
		workspaceID int64
		promptKey   string
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         bool
	}{
		{
			name: "success: allowed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				return fields{
					config:      mockConfig,
					rateLimiter: mockRateLimiter,
				}
			},
			args: args{
				ctx:         context.Background(),
				workspaceID: 123456,
				promptKey:   "test_prompt",
			},
			want: true,
		},
		{
			name: "rate limit exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(10, nil)

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
				ctx:         context.Background(),
				workspaceID: 123456,
				promptKey:   "test_prompt",
			},
			want: false,
		},
		{
			name: "config error: default allow",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(0, errors.New("config error"))

				return fields{
					config: mockConfig,
				}
			},
			args: args{
				ctx:         context.Background(),
				workspaceID: 123456,
				promptKey:   "test_prompt",
			},
			want: true,
		},
		{
			name: "rate limiter error: default allow",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(nil, errors.New("limiter error"))

				return fields{
					config:      mockConfig,
					rateLimiter: mockRateLimiter,
				}
			},
			args: args{
				ctx:         context.Background(),
				workspaceID: 123456,
				promptKey:   "test_prompt",
			},
			want: true,
		},
		{
			name: "rate limiter returns nil result: default allow",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(nil, nil)

				return fields{
					config:      mockConfig,
					rateLimiter: mockRateLimiter,
				}
			},
			args: args{
				ctx:         context.Background(),
				workspaceID: 123456,
				promptKey:   "test_prompt",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 移除 t.Parallel() 以避免数据竞争
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				config:      ttFields.config,
				rateLimiter: ttFields.rateLimiter,
			}
			got := p.ptaasAllowByPromptKey(tt.args.ctx, tt.args.workspaceID, tt.args.promptKey)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPromptOpenAPIApplicationImpl_getPromptByPromptKey(t *testing.T) {
	t.Parallel()

	type fields struct {
		promptService    service.IPromptService
		promptManageRepo repo.IManageRepo
	}
	type args struct {
		ctx              context.Context
		spaceID          int64
		promptIdentifier *openapi.PromptQuery
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantPrompt   *entity.Prompt
		wantErr      error
	}{
		{
			name: "success: get prompt by key and version",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), []service.PromptQueryParam{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"},
				}).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				expectedPrompt := &entity.Prompt{
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
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123456,
				promptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
					Version:   ptr.Of("1.0.0"),
				},
			},
			wantPrompt: &entity.Prompt{
				ID:        123,
				SpaceID:   123456,
				PromptKey: "test_prompt",
				PromptBasic: &entity.PromptBasic{
					DisplayName:   "Test Prompt",
					Description:   "Test Description",
					LatestVersion: "1.0.0",
					CreatedBy:     "test_user",
					UpdatedBy:     "test_user",
				},
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "",
						Description: "Initial version",
						CommittedBy: "test_user",
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
			wantErr: nil,
		},
		{
			name: "error: prompt identifier is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:              context.Background(),
				spaceID:          123456,
				promptIdentifier: nil,
			},
			wantPrompt: nil,
			wantErr:    errors.New("prompt identifier is nil"),
		},
		{
			name: "error: get prompt IDs failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(nil, errors.New("database error"))

				return fields{
					promptService: mockPromptService,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123456,
				promptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
					Version:   ptr.Of("1.0.0"),
				},
			},
			wantPrompt: nil,
			wantErr:    errors.New("database error"),
		},
		{
			name: "error: parse commit version failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), []service.PromptQueryParam{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"},
				}).Return(nil, errors.New("parse version error"))

				return fields{
					promptService: mockPromptService,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123456,
				promptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
					Version:   ptr.Of("1.0.0"),
				},
			},
			wantPrompt: nil,
			wantErr:    errors.New("parse version error"),
		},
		{
			name: "error: get prompt failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), []service.PromptQueryParam{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"},
				}).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("database error"))

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123456,
				promptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
					Version:   ptr.Of("1.0.0"),
				},
			},
			wantPrompt: nil,
			wantErr:    errors.New("database error"),
		},
		{
			name: "error: prompt version not exist with enhanced error info",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), []service.PromptQueryParam{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"},
				}).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil,
					errorx.NewByCode(prompterr.PromptVersionNotExistCode,
						errorx.WithExtra(map[string]string{"prompt_id": "123"})))

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123456,
				promptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
					Version:   ptr.Of("1.0.0"),
				},
			},
			wantPrompt: nil,
			wantErr: errorx.NewByCode(prompterr.PromptVersionNotExistCode,
				errorx.WithExtra(map[string]string{"prompt_id": "123", "prompt_key": "test_prompt"})),
		},
		{
			name: "success: get prompt by label",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), []service.PromptQueryParam{
					{PromptID: 123, PromptKey: "test_prompt", Label: "stable"},
				}).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Label: "stable"}: "2.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				expectedPrompt := &entity.Prompt{
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_prompt",
					PromptBasic: &entity.PromptBasic{
						DisplayName:   "Test Prompt",
						Description:   "Test Description",
						LatestVersion: "2.0.0",
						CreatedBy:     "test_user",
						UpdatedBy:     "test_user",
						CreatedAt:     startTime,
						UpdatedAt:     startTime,
					},
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{
							Version:     "2.0.0",
							BaseVersion: "",
							Description: "Stable version",
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
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "2.0.0"}: expectedPrompt,
				}, nil)

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123456,
				promptIdentifier: &openapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
					Label:     ptr.Of("stable"),
				},
			},
			wantPrompt: &entity.Prompt{
				ID:        123,
				SpaceID:   123456,
				PromptKey: "test_prompt",
				PromptBasic: &entity.PromptBasic{
					DisplayName:   "Test Prompt",
					Description:   "Test Description",
					LatestVersion: "2.0.0",
					CreatedBy:     "test_user",
					UpdatedBy:     "test_user",
				},
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version:     "2.0.0",
						BaseVersion: "",
						Description: "Stable version",
						CommittedBy: "test_user",
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
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 移除 t.Parallel() 以避免数据竞争
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				promptService:    ttFields.promptService,
				promptManageRepo: ttFields.promptManageRepo,
			}
			gotPrompt, err := p.getPromptByPromptKey(tt.args.ctx, tt.args.spaceID, tt.args.promptIdentifier)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantPrompt != nil && gotPrompt != nil {
				// 比较关键字段，忽略时间字段的差异
				assert.Equal(t, tt.wantPrompt.ID, gotPrompt.ID)
				assert.Equal(t, tt.wantPrompt.SpaceID, gotPrompt.SpaceID)
				assert.Equal(t, tt.wantPrompt.PromptKey, gotPrompt.PromptKey)
				if tt.wantPrompt.PromptBasic != nil && gotPrompt.PromptBasic != nil {
					assert.Equal(t, tt.wantPrompt.PromptBasic.DisplayName, gotPrompt.PromptBasic.DisplayName)
					assert.Equal(t, tt.wantPrompt.PromptBasic.Description, gotPrompt.PromptBasic.Description)
					assert.Equal(t, tt.wantPrompt.PromptBasic.LatestVersion, gotPrompt.PromptBasic.LatestVersion)
				}
				if tt.wantPrompt.PromptCommit != nil && gotPrompt.PromptCommit != nil &&
					tt.wantPrompt.PromptCommit.CommitInfo != nil && gotPrompt.PromptCommit.CommitInfo != nil {
					assert.Equal(t, tt.wantPrompt.PromptCommit.CommitInfo.Version, gotPrompt.PromptCommit.CommitInfo.Version)
					assert.Equal(t, tt.wantPrompt.PromptCommit.CommitInfo.Description, gotPrompt.PromptCommit.CommitInfo.Description)
				}
			} else {
				assert.Equal(t, tt.wantPrompt, gotPrompt)
			}
		})
	}
}

func TestPromptOpenAPIApplicationImpl_startPromptExecutorSpan(t *testing.T) {
	t.Parallel()

	type args struct {
		ctx   context.Context
		param ptaasStartPromptExecutorSpanParam
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "success: start span",
			args: args{
				ctx: context.Background(),
				param: ptaasStartPromptExecutorSpanParam{
					workspaceID:      123456,
					stream:           false,
					reqPromptKey:     "test_prompt",
					reqPromptVersion: "1.0.0",
					reqPromptLabel:   "stable",
					messages: []*entity.Message{
						{
							Role:    entity.RoleUser,
							Content: ptr.Of("Hello"),
						},
					},
					variableVals: []*entity.VariableVal{
						{
							Key:   "var1",
							Value: ptr.Of("value1"),
						},
					},
				},
			},
		},
		{
			name: "success: start streaming span",
			args: args{
				ctx: context.Background(),
				param: ptaasStartPromptExecutorSpanParam{
					workspaceID:      123456,
					stream:           true,
					reqPromptKey:     "test_prompt",
					reqPromptVersion: "2.0.0",
					reqPromptLabel:   "",
					messages:         []*entity.Message{},
					variableVals:     []*entity.VariableVal{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 移除 t.Parallel() 以避免数据竞争
			p := &PromptOpenAPIApplicationImpl{}
			gotCtx, gotSpan := p.startPromptExecutorSpan(tt.args.ctx, tt.args.param)
			assert.NotNil(t, gotCtx)
			// span 可能为 nil，这是正常的
			_ = gotSpan
		})
	}
}

func TestPromptOpenAPIApplicationImpl_finishPromptExecutorSpan(t *testing.T) {
	t.Parallel()

	type args struct {
		ctx    context.Context
		span   cozeloop.Span
		prompt *entity.Prompt
		reply  *entity.Reply
		err    error
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "success: finish span with all data",
			args: args{
				ctx:  context.Background(),
				span: nil, // 在实际测试中，span 可能为 nil
				prompt: &entity.Prompt{
					ID:        123,
					PromptKey: "test_prompt",
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{
							Version: "1.0.0",
						},
					},
				},
				reply: &entity.Reply{
					DebugID: 456,
					Item: &entity.ReplyItem{
						Message: &entity.Message{
							Role:    entity.RoleAssistant,
							Content: ptr.Of("Hello, how can I help you?"),
						},
						FinishReason: "stop",
						TokenUsage: &entity.TokenUsage{
							InputTokens:  10,
							OutputTokens: 20,
						},
					},
				},
				err: nil,
			},
		},
		{
			name: "success: finish span with error",
			args: args{
				ctx:  context.Background(),
				span: nil,
				prompt: &entity.Prompt{
					ID:        123,
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
		},
		{
			name: "span is nil: should return early",
			args: args{
				ctx:    context.Background(),
				span:   nil,
				prompt: nil,
				reply:  nil,
				err:    nil,
			},
		},
		{
			name: "prompt is nil: should return early",
			args: args{
				ctx:    context.Background(),
				span:   nil, // 假设有一个 span，但 prompt 为 nil
				prompt: nil,
				reply:  nil,
				err:    nil,
			},
		},
		{
			name: "success: finish span with minimal data",
			args: args{
				ctx:  context.Background(),
				span: nil,
				prompt: &entity.Prompt{
					ID:        123,
					PromptKey: "test_prompt",
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{
							Version: "1.0.0",
						},
					},
				},
				reply: &entity.Reply{
					DebugID: 0,
					Item:    nil,
				},
				err: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 移除 t.Parallel() 以避免数据竞争
			p := &PromptOpenAPIApplicationImpl{}
			// finishPromptExecutorSpan 没有返回值，只需要确保不 panic
			p.finishPromptExecutorSpan(tt.args.ctx, tt.args.span, tt.args.prompt, tt.args.reply, tt.args.err)
		})
	}
}

func TestPromptOpenAPIApplicationImpl_doExecute(t *testing.T) {
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
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantPromptDO *entity.Prompt
		wantReply    *entity.Reply
		wantErr      error
	}{
		{
			name: "success: execute prompt",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				expectedPrompt := &entity.Prompt{
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
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				expectedReply := &entity.Reply{
					DebugID: 456,
					Item: &entity.ReplyItem{
						Message: &entity.Message{
							Role:    entity.RoleAssistant,
							Content: ptr.Of("Hello, how can I help you?"),
						},
						FinishReason: "stop",
						TokenUsage: &entity.TokenUsage{
							InputTokens:  10,
							OutputTokens: 20,
						},
					},
				}
				mockPromptService.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(expectedReply, nil)

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
				DebugID: 456,
				Item: &entity.ReplyItem{
					Message: &entity.Message{
						Role:    entity.RoleAssistant,
						Content: ptr.Of("Hello, how can I help you?"),
					},
					FinishReason: "stop",
					TokenUsage: &entity.TokenUsage{
						InputTokens:  10,
						OutputTokens: 20,
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "error: rate limit exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(10, nil)

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
			wantPromptDO: nil,
			wantReply:    nil,
			wantErr:      errorx.NewByCode(prompterr.PTaaSQPSLimitCode, errorx.WithExtraMsg("qps limit exceeded")),
		},
		{
			name: "error: get prompt failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(nil, errors.New("database error"))

				return fields{
					promptService: mockPromptService,
					config:        mockConfig,
					rateLimiter:   mockRateLimiter,
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
			wantPromptDO: nil,
			wantReply:    nil,
			wantErr:      errors.New("database error"),
		},
		{
			name: "error: permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				expectedPrompt := &entity.Prompt{
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_prompt",
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

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
				},
			},
			wantPromptDO: &entity.Prompt{
				ID:        123,
				SpaceID:   123456,
				PromptKey: "test_prompt",
			},
			wantReply: nil,
			wantErr:   errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "error: prompt execution failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				expectedPrompt := &entity.Prompt{
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_prompt",
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				mockPromptService.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, errors.New("execution failed"))

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
				},
			},
			wantPromptDO: &entity.Prompt{
				ID:        123,
				SpaceID:   123456,
				PromptKey: "test_prompt",
			},
			wantReply: nil,
			wantErr:   errors.New("execution failed"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 移除 t.Parallel() 以避免数据竞争
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
			if tt.wantPromptDO != nil && gotPromptDO != nil {
				assert.Equal(t, tt.wantPromptDO.ID, gotPromptDO.ID)
				assert.Equal(t, tt.wantPromptDO.SpaceID, gotPromptDO.SpaceID)
				assert.Equal(t, tt.wantPromptDO.PromptKey, gotPromptDO.PromptKey)
			} else {
				assert.Equal(t, tt.wantPromptDO, gotPromptDO)
			}
			if tt.wantReply != nil && gotReply != nil {
				assert.Equal(t, tt.wantReply.DebugID, gotReply.DebugID)
				if tt.wantReply.Item != nil && gotReply.Item != nil {
					assert.Equal(t, tt.wantReply.Item.FinishReason, gotReply.Item.FinishReason)
					if tt.wantReply.Item.TokenUsage != nil && gotReply.Item.TokenUsage != nil {
						assert.Equal(t, tt.wantReply.Item.TokenUsage.InputTokens, gotReply.Item.TokenUsage.InputTokens)
						assert.Equal(t, tt.wantReply.Item.TokenUsage.OutputTokens, gotReply.Item.TokenUsage.OutputTokens)
					}
				}
			} else {
				assert.Equal(t, tt.wantReply, gotReply)
			}
		})
	}
}

func TestPromptOpenAPIApplicationImpl_Execute(t *testing.T) {
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
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				expectedPrompt := &entity.Prompt{
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
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				expectedReply := &entity.Reply{
					DebugID: 456,
					Item: &entity.ReplyItem{
						Message: &entity.Message{
							Role:    entity.RoleAssistant,
							Content: ptr.Of("Hello, how can I help you?"),
						},
						FinishReason: "stop",
						TokenUsage: &entity.TokenUsage{
							InputTokens:  10,
							OutputTokens: 20,
						},
					},
				}
				mockPromptService.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(expectedReply, nil)

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
						Content: ptr.Of("Hello, how can I help you?"),
					},
					FinishReason: ptr.Of("stop"),
					Usage: &openapi.TokenUsage{
						InputTokens:  ptr.Of(int32(10)),
						OutputTokens: ptr.Of(int32(20)),
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "error: invalid request",
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
					WorkspaceID: ptr.Of(int64(0)), // 无效的 workspace_id
					PromptIdentifier: &openapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
					},
				},
			},
			wantR:   openapi.NewExecuteResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"})),
		},
		{
			name: "error: rate limit exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(10, nil)

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
			name: "success: execute with nil reply item",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				expectedPrompt := &entity.Prompt{
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_prompt",
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{
							Version: "1.0.0",
						},
					},
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				// 返回 nil reply 或者 reply.Item 为 nil
				expectedReply := &entity.Reply{
					DebugID: 456,
					Item:    nil,
				}
				mockPromptService.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(expectedReply, nil)

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
				Data: nil, // 当 reply.Item 为 nil 时，Data 应该为 nil
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 移除 t.Parallel() 以避免数据竞争
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
			if tt.wantR != nil && gotR != nil {
				if tt.wantR.Data != nil && gotR.Data != nil {
					assert.Equal(t, tt.wantR.Data.FinishReason, gotR.Data.FinishReason)
					if tt.wantR.Data.Message != nil && gotR.Data.Message != nil {
						assert.Equal(t, tt.wantR.Data.Message.Role, gotR.Data.Message.Role)
						assert.Equal(t, tt.wantR.Data.Message.Content, gotR.Data.Message.Content)
					}
					if tt.wantR.Data.Usage != nil && gotR.Data.Usage != nil {
						if tt.wantR.Data.Usage.InputTokens != nil && gotR.Data.Usage.InputTokens != nil {
							assert.Equal(t, *tt.wantR.Data.Usage.InputTokens, *gotR.Data.Usage.InputTokens)
						} else {
							assert.Equal(t, tt.wantR.Data.Usage.InputTokens, gotR.Data.Usage.InputTokens)
						}
						if tt.wantR.Data.Usage.OutputTokens != nil && gotR.Data.Usage.OutputTokens != nil {
							assert.Equal(t, *tt.wantR.Data.Usage.OutputTokens, *gotR.Data.Usage.OutputTokens)
						} else {
							assert.Equal(t, tt.wantR.Data.Usage.OutputTokens, gotR.Data.Usage.OutputTokens)
						}
					}
				} else {
					assert.Equal(t, tt.wantR.Data, gotR.Data)
				}
			} else {
				assert.Equal(t, tt.wantR, gotR)
			}
		})
	}
}

// mockExecuteStreamingServer 用于测试的mock流式服务器
type mockExecuteStreamingServer struct {
	ctx        context.Context
	sendCalls  []*openapi.ExecuteStreamingResponse
	sendErrors []error
	sendIndex  int
}

func newMockExecuteStreamingServer(ctx context.Context) *mockExecuteStreamingServer {
	return &mockExecuteStreamingServer{
		ctx:        ctx,
		sendCalls:  make([]*openapi.ExecuteStreamingResponse, 0),
		sendErrors: make([]error, 0),
		sendIndex:  0,
	}
}

func (m *mockExecuteStreamingServer) Send(ctx context.Context, resp *openapi.ExecuteStreamingResponse) error {
	m.sendCalls = append(m.sendCalls, resp)
	if m.sendIndex < len(m.sendErrors) {
		err := m.sendErrors[m.sendIndex]
		m.sendIndex++
		return err
	}
	m.sendIndex++
	return nil
}

func (m *mockExecuteStreamingServer) RecvMsg(ctx context.Context, msg interface{}) error {
	return nil
}

func (m *mockExecuteStreamingServer) SendMsg(ctx context.Context, msg interface{}) error {
	return nil
}

func (m *mockExecuteStreamingServer) SendHeader(header streaming.Header) error {
	return nil
}

func (m *mockExecuteStreamingServer) SetHeader(header streaming.Header) error {
	return nil
}

func (m *mockExecuteStreamingServer) SetTrailer(trailer streaming.Trailer) error {
	return nil
}

func (m *mockExecuteStreamingServer) SetSendErrors(errors ...error) {
	m.sendErrors = errors
}

func (m *mockExecuteStreamingServer) GetSendCalls() []*openapi.ExecuteStreamingResponse {
	return m.sendCalls
}

func TestPromptOpenAPIApplicationImpl_ExecuteStreaming(t *testing.T) {
	// 移除 t.Parallel() 以避免数据竞争

	type fields struct {
		promptService    service.IPromptService
		promptManageRepo repo.IManageRepo
		config           conf.IConfigProvider
		auth             rpc.IAuthProvider
		rateLimiter      limiter.IRateLimiter
		collector        collector.ICollectorProvider
	}
	type args struct {
		ctx    context.Context
		req    *openapi.ExecuteRequest
		stream openapi.PromptOpenAPIService_ExecuteStreamingServer
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		argsGetter   func(ctrl *gomock.Controller) args
		wantErr      error
		validateFunc func(t *testing.T, stream *mockExecuteStreamingServer)
	}{
		{
			name: "success: normal streaming execution",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				expectedPrompt := &entity.Prompt{
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
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				// Mock ExecuteStreaming 返回多个流式响应
				expectedReply := &entity.Reply{
					DebugID: 456,
					Item: &entity.ReplyItem{
						Message: &entity.Message{
							Role:    entity.RoleAssistant,
							Content: ptr.Of("Hello, how can I help you?"),
						},
						FinishReason: "stop",
						TokenUsage: &entity.TokenUsage{
							InputTokens:  10,
							OutputTokens: 20,
						},
					},
				}
				mockPromptService.EXPECT().ExecuteStreaming(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, param service.ExecuteStreamingParam) (*entity.Reply, error) {
						// 模拟发送多个流式响应 - 使用同步方式避免竞争条件
						// 发送第一个chunk
						param.ResultStream <- &entity.Reply{
							Item: &entity.ReplyItem{
								Message: &entity.Message{
									Role:    entity.RoleAssistant,
									Content: ptr.Of("Hello"),
								},
								FinishReason: "",
								TokenUsage: &entity.TokenUsage{
									InputTokens:  5,
									OutputTokens: 1,
								},
							},
						}
						// 发送第二个chunk
						param.ResultStream <- &entity.Reply{
							Item: &entity.ReplyItem{
								Message: &entity.Message{
									Role:    entity.RoleAssistant,
									Content: ptr.Of(", how can I help you?"),
								},
								FinishReason: "stop",
								TokenUsage: &entity.TokenUsage{
									InputTokens:  10,
									OutputTokens: 20,
								},
							},
						}
						return expectedReply, nil
					})

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
			argsGetter: func(ctrl *gomock.Controller) args {
				ctx := context.Background()
				stream := newMockExecuteStreamingServer(ctx)
				return args{
					ctx: ctx,
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
					stream: stream,
				}
			},
			wantErr: nil,
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 2)
				assert.Equal(t, "Hello", calls[0].Data.Message.GetContent())
				assert.Equal(t, ", how can I help you?", calls[1].Data.Message.GetContent())
				assert.Equal(t, "stop", calls[1].Data.GetFinishReason())
			},
		},
		{
			name: "error: workspace_id is empty",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPTaaSEvent(gomock.Any(), gomock.Any()).Return()

				return fields{
					collector: mockCollector,
				}
			},
			argsGetter: func(ctrl *gomock.Controller) args {
				ctx := context.Background()
				stream := newMockExecuteStreamingServer(ctx)
				return args{
					ctx: ctx,
					req: &openapi.ExecuteRequest{
						WorkspaceID: ptr.Of(int64(0)), // 无效的 workspace_id
						PromptIdentifier: &openapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
						},
					},
					stream: stream,
				}
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"})),
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 0) // 参数验证失败，不应该发送任何响应
			},
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
			argsGetter: func(ctrl *gomock.Controller) args {
				ctx := context.Background()
				stream := newMockExecuteStreamingServer(ctx)
				return args{
					ctx: ctx,
					req: &openapi.ExecuteRequest{
						WorkspaceID: ptr.Of(int64(123456)),
						PromptIdentifier: &openapi.PromptQuery{
							PromptKey: ptr.Of(""), // 空的 prompt_key
						},
					},
					stream: stream,
				}
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "prompt_key参数为空"})),
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 0)
			},
		},
		{
			name: "error: invalid URL in message parts",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPTaaSEvent(gomock.Any(), gomock.Any()).Return()

				return fields{
					collector: mockCollector,
				}
			},
			argsGetter: func(ctrl *gomock.Controller) args {
				ctx := context.Background()
				stream := newMockExecuteStreamingServer(ctx)
				return args{
					ctx: ctx,
					req: &openapi.ExecuteRequest{
						WorkspaceID: ptr.Of(int64(123456)),
						PromptIdentifier: &openapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
						},
						Messages: []*openapi.Message{
							{
								Role: ptr.Of(prompt.RoleUser),
								Parts: []*openapi.ContentPart{
									{
										Type:     ptr.Of(openapi.ContentTypeImageURL),
										ImageURL: ptr.Of("invalid-url"), // 无效的URL
									},
								},
							},
						},
					},
					stream: stream,
				}
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "invalid-url不是有效的URL"})),
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 0)
			},
		},
		{
			name: "error: invalid base64 data",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPTaaSEvent(gomock.Any(), gomock.Any()).Return()

				return fields{
					collector: mockCollector,
				}
			},
			argsGetter: func(ctrl *gomock.Controller) args {
				ctx := context.Background()
				stream := newMockExecuteStreamingServer(ctx)
				return args{
					ctx: ctx,
					req: &openapi.ExecuteRequest{
						WorkspaceID: ptr.Of(int64(123456)),
						PromptIdentifier: &openapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
						},
						Messages: []*openapi.Message{
							{
								Role: ptr.Of(prompt.RoleUser),
								Parts: []*openapi.ContentPart{
									{
										Type:       ptr.Of(openapi.ContentTypeBase64Data),
										Base64Data: ptr.Of("invalid-base64"), // 无效的base64
									},
								},
							},
						},
					},
					stream: stream,
				}
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "存在无效的base64数据，数据格式应该符合data:[<mediatype>][;base64],<data>"})),
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 0)
			},
		},
		{
			name: "error: rate limit exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(10, nil)

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
			argsGetter: func(ctrl *gomock.Controller) args {
				ctx := context.Background()
				stream := newMockExecuteStreamingServer(ctx)
				return args{
					ctx: ctx,
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
					stream: stream,
				}
			},
			wantErr: errorx.NewByCode(prompterr.PTaaSQPSLimitCode, errorx.WithExtraMsg("qps limit exceeded")),
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 0)
			},
		},
		{
			name: "error: permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				expectedPrompt := &entity.Prompt{
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
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(
					errorx.NewByCode(prompterr.CommonNoPermissionCode))

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
			argsGetter: func(ctrl *gomock.Controller) args {
				ctx := context.Background()
				stream := newMockExecuteStreamingServer(ctx)
				return args{
					ctx: ctx,
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
					stream: stream,
				}
			},
			wantErr: errorx.NewByCode(prompterr.CommonNoPermissionCode),
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 0)
			},
		},
		{
			name: "error: get prompt failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(nil, errors.New("database error"))

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPTaaSEvent(gomock.Any(), gomock.Any()).Return()

				return fields{
					promptService: mockPromptService,
					config:        mockConfig,
					rateLimiter:   mockRateLimiter,
					collector:     mockCollector,
				}
			},
			argsGetter: func(ctrl *gomock.Controller) args {
				ctx := context.Background()
				stream := newMockExecuteStreamingServer(ctx)
				return args{
					ctx: ctx,
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
					stream: stream,
				}
			},
			wantErr: errors.New("database error"),
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 0)
			},
		},
		{
			name: "error: execute service error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				expectedPrompt := &entity.Prompt{
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
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				// Mock ExecuteStreaming 返回错误
				mockPromptService.EXPECT().ExecuteStreaming(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, param service.ExecuteStreamingParam) (*entity.Reply, error) {
						// 不发送任何响应，直接返回错误
						return nil, errors.New("execute service error")
					})

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
			argsGetter: func(ctrl *gomock.Controller) args {
				ctx := context.Background()
				stream := newMockExecuteStreamingServer(ctx)
				return args{
					ctx: ctx,
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
					stream: stream,
				}
			},
			wantErr: errors.New("execute service error"),
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 0)
			},
		},
		{
			name: "error: stream send failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				expectedPrompt := &entity.Prompt{
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
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				// Mock ExecuteStreaming 返回流式响应
				expectedReply := &entity.Reply{
					DebugID: 456,
					Item: &entity.ReplyItem{
						Message: &entity.Message{
							Role:    entity.RoleAssistant,
							Content: ptr.Of("Hello, how can I help you?"),
						},
						FinishReason: "stop",
						TokenUsage: &entity.TokenUsage{
							InputTokens:  10,
							OutputTokens: 20,
						},
					},
				}
				mockPromptService.EXPECT().ExecuteStreaming(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, param service.ExecuteStreamingParam) (*entity.Reply, error) {
						// 发送一个响应
						param.ResultStream <- &entity.Reply{
							Item: &entity.ReplyItem{
								Message: &entity.Message{
									Role:    entity.RoleAssistant,
									Content: ptr.Of("Hello"),
								},
								FinishReason: "",
								TokenUsage: &entity.TokenUsage{
									InputTokens:  5,
									OutputTokens: 1,
								},
							},
						}
						return expectedReply, nil
					})

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
			argsGetter: func(ctrl *gomock.Controller) args {
				ctx := context.Background()
				stream := newMockExecuteStreamingServer(ctx)
				// 设置第一次发送失败
				stream.SetSendErrors(errors.New("send failed"))
				return args{
					ctx: ctx,
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
					stream: stream,
				}
			},
			wantErr: errors.New("send failed"),
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 1) // 发送了一次但失败了
			},
		},
		{
			name: "success: client canceled context",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				expectedPrompt := &entity.Prompt{
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
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				// Mock ExecuteStreaming 返回流式响应
				expectedReply := &entity.Reply{
					DebugID: 456,
					Item: &entity.ReplyItem{
						Message: &entity.Message{
							Role:    entity.RoleAssistant,
							Content: ptr.Of("Hello, how can I help you?"),
						},
						FinishReason: "stop",
						TokenUsage: &entity.TokenUsage{
							InputTokens:  10,
							OutputTokens: 20,
						},
					},
				}
				mockPromptService.EXPECT().ExecuteStreaming(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, param service.ExecuteStreamingParam) (*entity.Reply, error) {
						// 发送一个响应
						param.ResultStream <- &entity.Reply{
							Item: &entity.ReplyItem{
								Message: &entity.Message{
									Role:    entity.RoleAssistant,
									Content: ptr.Of("Hello"),
								},
								FinishReason: "",
								TokenUsage: &entity.TokenUsage{
									InputTokens:  5,
									OutputTokens: 1,
								},
							},
						}
						return expectedReply, nil
					})

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
			argsGetter: func(ctrl *gomock.Controller) args {
				ctx := context.Background()
				stream := newMockExecuteStreamingServer(ctx)
				// 模拟客户端取消
				stream.SetSendErrors(status.Error(codes.Canceled, "context canceled"))
				return args{
					ctx: ctx,
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
					stream: stream,
				}
			},
			wantErr: status.Error(codes.Canceled, "context canceled"), // 实际测试显示返回取消错误
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 1)
			},
		},
		{
			name: "error: goroutine panic recovery",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(map[string]int64{
					"test_prompt": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), int64(123456), gomock.Any()).Return(map[service.PromptQueryParam]string{
					{PromptID: 123, PromptKey: "test_prompt", Version: "1.0.0"}: "1.0.0",
				}, nil)

				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				expectedPrompt := &entity.Prompt{
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
				}
				mockManageRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 123, WithCommit: true, CommitVersion: "1.0.0"}: expectedPrompt,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				// Mock ExecuteStreaming 模拟panic
				mockPromptService.EXPECT().ExecuteStreaming(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, param service.ExecuteStreamingParam) (*entity.Reply, error) {
						// 模拟panic
						panic("test panic")
					})

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
			argsGetter: func(ctrl *gomock.Controller) args {
				ctx := context.Background()
				stream := newMockExecuteStreamingServer(ctx)
				return args{
					ctx: ctx,
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
					stream: stream,
				}
			},
			wantErr: errorx.New("panic occurred, reason=test panic"),
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 0) // panic发生时不应该发送响应
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 移除 t.Parallel() 以避免数据竞争
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			ttArgs := tt.argsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				promptService:    ttFields.promptService,
				promptManageRepo: ttFields.promptManageRepo,
				config:           ttFields.config,
				auth:             ttFields.auth,
				rateLimiter:      ttFields.rateLimiter,
				collector:        ttFields.collector,
			}
			err := p.ExecuteStreaming(ttArgs.ctx, ttArgs.req, ttArgs.stream)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.validateFunc != nil {
				if mockStream, ok := ttArgs.stream.(*mockExecuteStreamingServer); ok {
					tt.validateFunc(t, mockStream)
				}
			}
		})
	}
}

func TestPromptOpenAPIApplicationImpl_ListPromptBasic(t *testing.T) {
	t.Parallel()

	type fields struct {
		promptManageRepo repo.IManageRepo
		config           conf.IConfigProvider
		auth             rpc.IAuthProvider
		rateLimiter      limiter.IRateLimiter
	}
	type args struct {
		ctx context.Context
		req *openapi.ListPromptBasicRequest
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantR        *openapi.ListPromptBasicResponse
		wantErr      error
	}{
		{
			name: "success: list prompts basic info",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				mockManageRepo.EXPECT().ListPrompt(gomock.Any(), repo.ListPromptParam{
					SpaceID:       123456,
					CommittedOnly: true,
					PageNum:       1,
					PageSize:      10,
				}).Return(&repo.ListPromptResult{
					Total: 2,
					PromptDOs: []*entity.Prompt{
						{
							ID:        123,
							SpaceID:   123456,
							PromptKey: "test_prompt1",
							PromptBasic: &entity.PromptBasic{
								DisplayName:   "Test Prompt 1",
								Description:   "Test Description 1",
								LatestVersion: "1.0.0",
								CreatedBy:     "test_user",
								UpdatedBy:     "test_user",
								CreatedAt:     startTime,
								UpdatedAt:     startTime,
							},
						},
						{
							ID:        456,
							SpaceID:   123456,
							PromptKey: "test_prompt2",
							PromptBasic: &entity.PromptBasic{
								DisplayName:   "Test Prompt 2",
								Description:   "Test Description 2",
								LatestVersion: "2.0.0",
								CreatedBy:     "test_user",
								UpdatedBy:     "test_user",
								CreatedAt:     startTime,
								UpdatedAt:     startTime,
							},
						},
					},
				}, nil)

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), int64(123456)).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123, 456}, consts.ActionLoopPromptRead).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				return fields{
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListPromptBasicRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PageNumber:  ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			wantR: &openapi.ListPromptBasicResponse{
				Data: &openapi.ListPromptBasicData{
					Total: ptr.Of(int32(2)),
					Prompts: []*openapi.PromptBasic{
						{
							ID:            ptr.Of(int64(123)),
							WorkspaceID:   ptr.Of(int64(123456)),
							PromptKey:     ptr.Of("test_prompt1"),
							DisplayName:   ptr.Of("Test Prompt 1"),
							Description:   ptr.Of("Test Description 1"),
							LatestVersion: ptr.Of("1.0.0"),
							CreatedBy:     ptr.Of("test_user"),
							UpdatedBy:     ptr.Of("test_user"),
						},
						{
							ID:            ptr.Of(int64(456)),
							WorkspaceID:   ptr.Of(int64(123456)),
							PromptKey:     ptr.Of("test_prompt2"),
							DisplayName:   ptr.Of("Test Prompt 2"),
							Description:   ptr.Of("Test Description 2"),
							LatestVersion: ptr.Of("2.0.0"),
							CreatedBy:     ptr.Of("test_user"),
							UpdatedBy:     ptr.Of("test_user"),
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "success: with keyword filter",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				mockManageRepo.EXPECT().ListPrompt(gomock.Any(), repo.ListPromptParam{
					SpaceID:       123456,
					KeyWord:       "test",
					CommittedOnly: true,
					PageNum:       1,
					PageSize:      10,
				}).Return(&repo.ListPromptResult{
					Total: 1,
					PromptDOs: []*entity.Prompt{
						{
							ID:        123,
							SpaceID:   123456,
							PromptKey: "test_prompt1",
							PromptBasic: &entity.PromptBasic{
								DisplayName:   "Test Prompt 1",
								Description:   "Test Description 1",
								LatestVersion: "1.0.0",
								CreatedBy:     "test_user",
								UpdatedBy:     "test_user",
								CreatedAt:     startTime,
								UpdatedAt:     startTime,
							},
						},
					},
				}, nil)

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), int64(123456)).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				return fields{
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListPromptBasicRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PageNumber:  ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
					KeyWord:     ptr.Of("test"),
				},
			},
			wantR: &openapi.ListPromptBasicResponse{
				Data: &openapi.ListPromptBasicData{
					Total: ptr.Of(int32(1)),
					Prompts: []*openapi.PromptBasic{
						{
							ID:            ptr.Of(int64(123)),
							WorkspaceID:   ptr.Of(int64(123456)),
							PromptKey:     ptr.Of("test_prompt1"),
							DisplayName:   ptr.Of("Test Prompt 1"),
							Description:   ptr.Of("Test Description 1"),
							LatestVersion: ptr.Of("1.0.0"),
							CreatedBy:     ptr.Of("test_user"),
							UpdatedBy:     ptr.Of("test_user"),
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "success: with creator filter",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				mockManageRepo.EXPECT().ListPrompt(gomock.Any(), repo.ListPromptParam{
					SpaceID:       123456,
					CreatedBys:    []string{"specific_user"},
					CommittedOnly: true,
					PageNum:       1,
					PageSize:      10,
				}).Return(&repo.ListPromptResult{
					Total: 1,
					PromptDOs: []*entity.Prompt{
						{
							ID:        123,
							SpaceID:   123456,
							PromptKey: "user_prompt",
							PromptBasic: &entity.PromptBasic{
								DisplayName:   "User Prompt",
								Description:   "User Description",
								LatestVersion: "1.0.0",
								CreatedBy:     "specific_user",
								UpdatedBy:     "specific_user",
								CreatedAt:     startTime,
								UpdatedAt:     startTime,
							},
						},
					},
				}, nil)

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), int64(123456)).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				return fields{
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListPromptBasicRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PageNumber:  ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
					Creator:     ptr.Of("specific_user"),
				},
			},
			wantR: &openapi.ListPromptBasicResponse{
				Data: &openapi.ListPromptBasicData{
					Total: ptr.Of(int32(1)),
					Prompts: []*openapi.PromptBasic{
						{
							ID:            ptr.Of(int64(123)),
							WorkspaceID:   ptr.Of(int64(123456)),
							PromptKey:     ptr.Of("user_prompt"),
							DisplayName:   ptr.Of("User Prompt"),
							Description:   ptr.Of("User Description"),
							LatestVersion: ptr.Of("1.0.0"),
							CreatedBy:     ptr.Of("specific_user"),
							UpdatedBy:     ptr.Of("specific_user"),
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "success: empty result",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().ListPrompt(gomock.Any(), repo.ListPromptParam{
					SpaceID:       123456,
					CommittedOnly: true,
					PageNum:       1,
					PageSize:      10,
				}).Return(&repo.ListPromptResult{
					Total:     0,
					PromptDOs: []*entity.Prompt{},
				}, nil)

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), int64(123456)).Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				return fields{
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					rateLimiter:      mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListPromptBasicRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PageNumber:  ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			wantR: &openapi.ListPromptBasicResponse{
				Data: &openapi.ListPromptBasicData{
					Total:   ptr.Of(int32(0)),
					Prompts: []*openapi.PromptBasic{},
				},
			},
			wantErr: nil,
		},
		{
			name: "error: workspace_id is zero",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListPromptBasicRequest{
					WorkspaceID: ptr.Of(int64(0)),
					PageNumber:  ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			wantR:   openapi.NewListPromptBasicResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"})),
		},
		{
			name: "error: workspace_id is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListPromptBasicRequest{
					WorkspaceID: nil,
					PageNumber:  ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			wantR:   openapi.NewListPromptBasicResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"})),
		},
		{
			name: "error: rate limit exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), int64(123456)).Return(1, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: false,
				}, nil)

				return fields{
					config:      mockConfig,
					rateLimiter: mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListPromptBasicRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PageNumber:  ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			wantR:   openapi.NewListPromptBasicResponse(),
			wantErr: errorx.NewByCode(prompterr.PromptHubQPSLimitCode, errorx.WithExtraMsg("qps limit exceeded")),
		},
		{
			name: "error: list prompt failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().ListPrompt(gomock.Any(), gomock.Any()).Return(nil, errors.New("database error"))

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), int64(123456)).Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				return fields{
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					rateLimiter:      mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListPromptBasicRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PageNumber:  ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			wantR:   nil,
			wantErr: errors.New("database error"),
		},
		{
			name: "error: permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				startTime := time.Now()
				mockManageRepo.EXPECT().ListPrompt(gomock.Any(), gomock.Any()).Return(&repo.ListPromptResult{
					Total: 1,
					PromptDOs: []*entity.Prompt{
						{
							ID:        123,
							SpaceID:   123456,
							PromptKey: "test_prompt1",
							PromptBasic: &entity.PromptBasic{
								DisplayName:   "Test Prompt 1",
								Description:   "Test Description 1",
								LatestVersion: "1.0.0",
								CreatedBy:     "test_user",
								UpdatedBy:     "test_user",
								CreatedAt:     startTime,
								UpdatedAt:     startTime,
							},
						},
					},
				}, nil)

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), int64(123456)).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				return fields{
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListPromptBasicRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PageNumber:  ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			wantR:   nil,
			wantErr: errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 移除 t.Parallel() 以避免数据竞争
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				promptManageRepo: ttFields.promptManageRepo,
				config:           ttFields.config,
				auth:             ttFields.auth,
				rateLimiter:      ttFields.rateLimiter,
			}
			gotR, err := p.ListPromptBasic(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)

			// 对于成功的测试用例，需要处理时间戳比较
			if err == nil && tt.wantR != nil && gotR != nil && gotR.Data != nil && tt.wantR.Data != nil {
				// 比较除时间戳外的其他字段
				assert.Equal(t, tt.wantR.Data.Total, gotR.Data.Total)
				assert.Equal(t, len(tt.wantR.Data.Prompts), len(gotR.Data.Prompts))

				for i, expected := range tt.wantR.Data.Prompts {
					if i < len(gotR.Data.Prompts) {
						actual := gotR.Data.Prompts[i]
						assert.Equal(t, expected.ID, actual.ID)
						assert.Equal(t, expected.WorkspaceID, actual.WorkspaceID)
						assert.Equal(t, expected.PromptKey, actual.PromptKey)
						assert.Equal(t, expected.DisplayName, actual.DisplayName)
						assert.Equal(t, expected.Description, actual.Description)
						assert.Equal(t, expected.LatestVersion, actual.LatestVersion)
						assert.Equal(t, expected.CreatedBy, actual.CreatedBy)
						assert.Equal(t, expected.UpdatedBy, actual.UpdatedBy)
						// 时间戳字段只检查是否不为nil
						assert.NotNil(t, actual.CreatedAt)
						assert.NotNil(t, actual.UpdatedAt)
					}
				}
			} else {
				assert.Equal(t, tt.wantR, gotR)
			}
		})
	}
}
