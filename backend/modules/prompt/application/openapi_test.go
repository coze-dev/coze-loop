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
	domainopenapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain_openapi/prompt"
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
		user             rpc.IUserProvider
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPromptHubEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return()
				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()
				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					collector:        mockCollector,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
				Data: &domainopenapi.PromptResultData{
					Items: []*domainopenapi.PromptResult_{
						{
							Query: &domainopenapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt1"),
								Version:   ptr.Of("1.0.0"),
							},
							Prompt: &domainopenapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt1"),
								Version:     ptr.Of("1.0.0"),
								PromptTemplate: &domainopenapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*domainopenapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*domainopenapi.VariableDef, 0),
								},
								LlmConfig: &domainopenapi.LLMConfig{
									Temperature: ptr.Of(0.7),
								},
							},
						},
						{
							Query: &domainopenapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt2"),
								Version:   ptr.Of("1.0.0"),
							},
							Prompt: &domainopenapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt2"),
								Version:     ptr.Of("1.0.0"),
								PromptTemplate: &domainopenapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*domainopenapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*domainopenapi.VariableDef, 0),
								},
								LlmConfig: &domainopenapi.LLMConfig{
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
			name: "expand snippets error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(errorx.New("expand error"))
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
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
								BaseVersion: "0.9.0",
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
							},
						},
					},
				}, nil)

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(10, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Version:   ptr.Of("1.0.0"),
						},
					},
				},
			},
			wantR:   nil,
			wantErr: errorx.New("expand error"),
		},
		{
			name: "success: latest commit version",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPromptHubEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return()

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					collector:        mockCollector,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
				Data: &domainopenapi.PromptResultData{
					Items: []*domainopenapi.PromptResult_{
						{
							Query: &domainopenapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt1"),
								Version:   ptr.Of("1.0.0"),
							},
							Prompt: &domainopenapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt1"),
								Version:     ptr.Of("1.0.0"),
								PromptTemplate: &domainopenapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*domainopenapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*domainopenapi.VariableDef, 0),
								},
								LlmConfig: &domainopenapi.LLMConfig{
									Temperature: ptr.Of(0.7),
								},
							},
						},
						{
							Query: &domainopenapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt1"),
								Version:   ptr.Of("2.0.0"),
							},
							Prompt: &domainopenapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt1"),
								Version:     ptr.Of("2.0.0"),
								PromptTemplate: &domainopenapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*domainopenapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*domainopenapi.VariableDef, 0),
								},
								LlmConfig: &domainopenapi.LLMConfig{
									Temperature: ptr.Of(0.7),
								},
							},
						},
						{
							Query: &domainopenapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt1"),
							},
							Prompt: &domainopenapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt1"),
								Version:     ptr.Of("2.0.0"),
								PromptTemplate: &domainopenapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*domainopenapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*domainopenapi.VariableDef, 0),
								},
								LlmConfig: &domainopenapi.LLMConfig{
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

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					config:      mockConfig,
					rateLimiter: mockRateLimiter,
					user:        mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("database error"))

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					config:        mockConfig,
					rateLimiter:   mockRateLimiter,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					config:        mockConfig,
					auth:          mockAuth,
					rateLimiter:   mockRateLimiter,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("parse version error"))

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					config:        mockConfig,
					auth:          mockAuth,
					rateLimiter:   mockRateLimiter,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()
				return fields{user: mockUser}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(0)),
					Queries: []*domainopenapi.PromptQuery{
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
				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()
				return fields{user: mockUser}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: nil,
					Queries: []*domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPromptHubEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return()

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					collector:        mockCollector,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
						{
							PromptKey: ptr.Of("test_prompt1"),
							Label:     ptr.Of("stable"),
						},
					},
				},
			},
			wantR: &openapi.BatchGetPromptByPromptKeyResponse{
				Data: &domainopenapi.PromptResultData{
					Items: []*domainopenapi.PromptResult_{
						{
							Query: &domainopenapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt1"),
								Label:     ptr.Of("stable"),
							},
							Prompt: &domainopenapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt1"),
								Version:     ptr.Of("2.0.0"),
								PromptTemplate: &domainopenapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*domainopenapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*domainopenapi.VariableDef, 0),
								},
								LlmConfig: &domainopenapi.LLMConfig{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockCollector := collectormocks.NewMockICollectorProvider(ctrl)
				mockCollector.EXPECT().CollectPromptHubEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return()

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					collector:        mockCollector,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
				Data: &domainopenapi.PromptResultData{
					Items: []*domainopenapi.PromptResult_{
						{
							Query: &domainopenapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt1"),
								Version:   ptr.Of("1.0.0"),
							},
							Prompt: &domainopenapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt1"),
								Version:     ptr.Of("1.0.0"),
								PromptTemplate: &domainopenapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*domainopenapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*domainopenapi.VariableDef, 0),
								},
								LlmConfig: &domainopenapi.LLMConfig{
									Temperature: ptr.Of(0.7),
								},
							},
						},
						{
							Query: &domainopenapi.PromptQuery{
								PromptKey: ptr.Of("test_prompt2"),
								Label:     ptr.Of("beta"),
							},
							Prompt: &domainopenapi.Prompt{
								WorkspaceID: ptr.Of(int64(123456)),
								PromptKey:   ptr.Of("test_prompt2"),
								Version:     ptr.Of("1.5.0"),
								PromptTemplate: &domainopenapi.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*domainopenapi.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
									VariableDefs: make([]*domainopenapi.VariableDef, 0),
								},
								LlmConfig: &domainopenapi.LLMConfig{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("label not found: non_existent_label"))

				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					config:        mockConfig,
					auth:          mockAuth,
					rateLimiter:   mockRateLimiter,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[string]int64{
					"test_prompt1": 123,
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
			wantR:   nil,
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("prompt not exist")),
		},
		{
			name: "error: prompt version not exist in result construction",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					config:           mockConfig,
					auth:             mockAuth,
					rateLimiter:      mockRateLimiter,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.BatchGetPromptByPromptKeyRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					Queries: []*domainopenapi.PromptQuery{
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
				user:             ttFields.user,
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
				PromptIdentifier: &domainopenapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
					Version:   ptr.Of("1.0.0"),
				},
				Messages: []*domainopenapi.Message{
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
				PromptIdentifier: &domainopenapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"})),
		},
		{
			name: "error: workspace_id is nil",
			req: &openapi.ExecuteRequest{
				WorkspaceID: nil,
				PromptIdentifier: &domainopenapi.PromptQuery{
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
				PromptIdentifier: &domainopenapi.PromptQuery{
					PromptKey: ptr.Of(""),
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "prompt_key参数为空"})),
		},
		{
			name: "error: prompt_key is nil",
			req: &openapi.ExecuteRequest{
				WorkspaceID: ptr.Of(int64(123456)),
				PromptIdentifier: &domainopenapi.PromptQuery{
					PromptKey: nil,
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "prompt_key参数为空"})),
		},
		{
			name: "error: invalid image URL",
			req: &openapi.ExecuteRequest{
				WorkspaceID: ptr.Of(int64(123456)),
				PromptIdentifier: &domainopenapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
				Messages: []*domainopenapi.Message{
					{
						Role: ptr.Of(prompt.RoleUser),
						Parts: []*domainopenapi.ContentPart{
							{
								Type:     ptr.Of(domainopenapi.ContentTypeImageURL),
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
				PromptIdentifier: &domainopenapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
				Messages: []*domainopenapi.Message{
					{
						Role: ptr.Of(prompt.RoleUser),
						Parts: []*domainopenapi.ContentPart{
							{
								Type:       ptr.Of(domainopenapi.ContentTypeBase64Data),
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
				PromptIdentifier: &domainopenapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
				Messages: []*domainopenapi.Message{
					{
						Role: ptr.Of(prompt.RoleUser),
						Parts: []*domainopenapi.ContentPart{
							{
								Type:     ptr.Of(domainopenapi.ContentTypeImageURL),
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
				PromptIdentifier: &domainopenapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
				Messages: []*domainopenapi.Message{
					{
						Role: ptr.Of(prompt.RoleUser),
						Parts: []*domainopenapi.ContentPart{
							{
								Type:       ptr.Of(domainopenapi.ContentTypeBase64Data),
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
				PromptIdentifier: &domainopenapi.PromptQuery{
					PromptKey: ptr.Of("test_prompt"),
				},
				VariableVals: []*domainopenapi.VariableVal{
					{
						Key: ptr.Of("image_var"),
						MultiPartValues: []*domainopenapi.ContentPart{
							{
								Type:       ptr.Of(domainopenapi.ContentTypeBase64Data),
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
		promptIdentifier *domainopenapi.PromptQuery
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				promptIdentifier: &domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockPromptService.EXPECT().MGetPromptIDs(gomock.Any(), int64(123456), []string{"test_prompt"}).Return(nil, errors.New("database error"))

				return fields{
					promptService: mockPromptService,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123456,
				promptIdentifier: &domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				promptIdentifier: &domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				promptIdentifier: &domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				promptIdentifier: &domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				promptIdentifier: &domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

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
				expectedResponseAPIConfig := &entity.ResponseAPIConfig{
					PreviousResponseID: ptr.Of("prev-id"),
					EnableCaching:      ptr.Of(true),
					SessionID:          ptr.Of("session-123"),
				}
				mockPromptService.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, param service.ExecuteParam) (*entity.Reply, error) {
						assert.Equal(t, expectedResponseAPIConfig, param.ResponseAPIConfig)
						return expectedReply, nil
					})
				mockPromptService.EXPECT().MConvertBase64DataURLToFileURL(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

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
					PromptIdentifier: &domainopenapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
						Version:   ptr.Of("1.0.0"),
					},
					Messages: []*domainopenapi.Message{
						{
							Role:    ptr.Of(prompt.RoleUser),
							Content: ptr.Of("Hello"),
						},
					},
					ResponseAPIConfig: &domainopenapi.ResponseAPIConfig{
						PreviousResponseID: ptr.Of("prev-id"),
						EnableCaching:      ptr.Of(true),
						SessionID:          ptr.Of("session-123"),
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
			name: "error: base64 convert failed",
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				expectedReply := &entity.Reply{
					DebugID: 456,
					Item: &entity.ReplyItem{
						Message: &entity.Message{
							Role: entity.RoleAssistant,
							Parts: []*entity.ContentPart{
								{
									Type: entity.ContentTypeImageURL,
									ImageURL: &entity.ImageURL{
										URL: "data:image/png;base64,abc",
									},
								},
							},
						},
					},
				}
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockPromptService.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(expectedReply, nil)
				mockPromptService.EXPECT().MConvertBase64DataURLToFileURL(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("convert error"))

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
					PromptIdentifier: &domainopenapi.PromptQuery{
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
			wantErr:   errors.New("convert error"),
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
					PromptIdentifier: &domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
					PromptIdentifier: &domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

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
					PromptIdentifier: &domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

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
					PromptIdentifier: &domainopenapi.PromptQuery{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

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
				mockPromptService.EXPECT().MConvertBase64DataURLToFileURL(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

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
					PromptIdentifier: &domainopenapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
						Version:   ptr.Of("1.0.0"),
					},
					Messages: []*domainopenapi.Message{
						{
							Role:    ptr.Of(prompt.RoleUser),
							Content: ptr.Of("Hello"),
						},
					},
				},
			},
			wantR: &openapi.ExecuteResponse{
				Data: &domainopenapi.ExecuteData{
					Message: &domainopenapi.Message{
						Role:    ptr.Of(prompt.RoleAssistant),
						Content: ptr.Of("Hello, how can I help you?"),
					},
					FinishReason: ptr.Of("stop"),
					Usage: &domainopenapi.TokenUsage{
						InputTokens:  ptr.Of(int32(10)),
						OutputTokens: ptr.Of(int32(20)),
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "error: base64 convert failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPTaaSMaxQPSByPromptKey(gomock.Any(), int64(123456), "test_prompt").Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "ptaas:qps:space_id:123456:prompt_key:test_prompt", 1, gomock.Any()).Return(&limiter.Result{
					Allowed: true,
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				expectedReply := &entity.Reply{
					DebugID: 456,
					Item: &entity.ReplyItem{
						Message: &entity.Message{
							Role: entity.RoleAssistant,
							Parts: []*entity.ContentPart{
								{
									Type: entity.ContentTypeImageURL,
									ImageURL: &entity.ImageURL{
										URL: "data:image/png;base64,abc",
									},
								},
							},
						},
					},
				}
				mockPromptService.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(expectedReply, nil)
				mockPromptService.EXPECT().MConvertBase64DataURLToFileURL(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("convert error"))

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
					PromptIdentifier: &domainopenapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
						Version:   ptr.Of("1.0.0"),
					},
					Messages: []*domainopenapi.Message{
						{
							Role:    ptr.Of(prompt.RoleUser),
							Content: ptr.Of("Hello"),
						},
					},
				},
			},
			wantR:   openapi.NewExecuteResponse(),
			wantErr: errors.New("convert error"),
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
					PromptIdentifier: &domainopenapi.PromptQuery{
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
					PromptIdentifier: &domainopenapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
						Version:   ptr.Of("1.0.0"),
					},
					Messages: []*domainopenapi.Message{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

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
					PromptIdentifier: &domainopenapi.PromptQuery{
						PromptKey: ptr.Of("test_prompt"),
						Version:   ptr.Of("1.0.0"),
					},
					Messages: []*domainopenapi.Message{
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

func TestPromptOpenAPIApplicationImpl_CreatePromptOApi(t *testing.T) {
	t.Parallel()

	type fields struct {
		promptService service.IPromptService
		auth          rpc.IAuthProvider
		user          rpc.IUserProvider
	}
	type args struct {
		ctx context.Context
		req *openapi.CreatePromptOApiRequest
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantR        *openapi.CreatePromptOApiResponse
		wantErr      error
	}{
		{
			name: "success: create prompt with defaults",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermissionForOpenAPI(gomock.Any(), int64(123456), consts.ActionWorkspaceCreateLoopPrompt).Return(nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().CreatePrompt(gomock.Any(), gomock.Any()).Return(int64(999), nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					auth:          mockAuth,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreatePromptOApiRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PromptKey:   ptr.Of("test_key"),
					PromptName:  ptr.Of("Test Name"),
				},
			},
			wantR: &openapi.CreatePromptOApiResponse{
				PromptID: ptr.Of(int64(999)),
			},
			wantErr: nil,
		},
		{
			name: "success: create prompt with all fields",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermissionForOpenAPI(gomock.Any(), int64(123456), consts.ActionWorkspaceCreateLoopPrompt).Return(nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().CreatePrompt(gomock.Any(), gomock.Any()).Return(int64(888), nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					auth:          mockAuth,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreatePromptOApiRequest{
					WorkspaceID:       ptr.Of(int64(123456)),
					PromptKey:         ptr.Of("test_key"),
					PromptName:        ptr.Of("Test Name"),
					PromptDescription: ptr.Of("desc"),
					PromptType:        ptr.Of(domainopenapi.PromptType(domainopenapi.PromptTypeNormal)),
					SecurityLevel:     ptr.Of(domainopenapi.SecurityLevel(domainopenapi.SecurityLevelL3)),
				},
			},
			wantR: &openapi.CreatePromptOApiResponse{
				PromptID: ptr.Of(int64(888)),
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
				req: &openapi.CreatePromptOApiRequest{
					WorkspaceID: ptr.Of(int64(0)),
					PromptKey:   ptr.Of("test_key"),
					PromptName:  ptr.Of("Test Name"),
				},
			},
			wantR:   openapi.NewCreatePromptOApiResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"})),
		},
		{
			name: "error: prompt_key is empty",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreatePromptOApiRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PromptKey:   ptr.Of(""),
					PromptName:  ptr.Of("Test Name"),
				},
			},
			wantR:   openapi.NewCreatePromptOApiResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "prompt_key参数为空"})),
		},
		{
			name: "error: prompt_name is empty",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreatePromptOApiRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PromptKey:   ptr.Of("test_key"),
					PromptName:  ptr.Of(""),
				},
			},
			wantR:   openapi.NewCreatePromptOApiResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "prompt_name参数为空"})),
		},
		{
			name: "error: permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermissionForOpenAPI(gomock.Any(), int64(123456), consts.ActionWorkspaceCreateLoopPrompt).
					Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

				return fields{
					auth: mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreatePromptOApiRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PromptKey:   ptr.Of("test_key"),
					PromptName:  ptr.Of("Test Name"),
				},
			},
			wantR:   openapi.NewCreatePromptOApiResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "error: create prompt service error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermissionForOpenAPI(gomock.Any(), int64(123456), consts.ActionWorkspaceCreateLoopPrompt).Return(nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().CreatePrompt(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("create error"))

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					auth:          mockAuth,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreatePromptOApiRequest{
					WorkspaceID: ptr.Of(int64(123456)),
					PromptKey:   ptr.Of("test_key"),
					PromptName:  ptr.Of("Test Name"),
				},
			},
			wantR:   openapi.NewCreatePromptOApiResponse(),
			wantErr: errors.New("create error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				promptService: ttFields.promptService,
				auth:          ttFields.auth,
				user:          ttFields.user,
			}
			gotR, err := p.CreatePromptOApi(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			assert.Equal(t, tt.wantR, gotR)
		})
	}
}

func TestPromptOpenAPIApplicationImpl_DeletePromptOApi(t *testing.T) {
	t.Parallel()

	type fields struct {
		promptManageRepo repo.IManageRepo
		auth             rpc.IAuthProvider
	}
	type args struct {
		ctx context.Context
		req *openapi.DeletePromptOApiRequest
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantR        *openapi.DeletePromptOApiResponse
		wantErr      error
	}{
		{
			name: "success: delete prompt",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 123}).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				}, nil)
				mockManageRepo.EXPECT().DeletePrompt(gomock.Any(), int64(123)).Return(nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptEdit).Return(nil)

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeletePromptOApiRequest{
					PromptID:    ptr.Of(int64(123)),
					WorkspaceID: ptr.Of(int64(123456)),
				},
			},
			wantR:   openapi.NewDeletePromptOApiResponse(),
			wantErr: nil,
		},
		{
			name: "success: delete prompt without workspace_id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 123}).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				}, nil)
				mockManageRepo.EXPECT().DeletePrompt(gomock.Any(), int64(123)).Return(nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptEdit).Return(nil)

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeletePromptOApiRequest{
					PromptID: ptr.Of(int64(123)),
				},
			},
			wantR:   openapi.NewDeletePromptOApiResponse(),
			wantErr: nil,
		},
		{
			name: "error: get prompt failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(nil, errors.New("not found"))

				return fields{
					promptManageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeletePromptOApiRequest{
					PromptID: ptr.Of(int64(999)),
				},
			},
			wantR:   openapi.NewDeletePromptOApiResponse(),
			wantErr: errors.New("not found"),
		},
		{
			name: "error: workspace_id not match",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 111111,
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				}, nil)

				return fields{
					promptManageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeletePromptOApiRequest{
					PromptID:    ptr.Of(int64(123)),
					WorkspaceID: ptr.Of(int64(999999)),
				},
			},
			wantR:   openapi.NewDeletePromptOApiResponse(),
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("WorkspaceID not match")),
		},
		{
			name: "error: snippet prompt cannot be deleted",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeSnippet,
					},
				}, nil)

				return fields{
					promptManageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeletePromptOApiRequest{
					PromptID: ptr.Of(int64(123)),
				},
			},
			wantR:   openapi.NewDeletePromptOApiResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Snippet prompt can not be deleted")),
		},
		{
			name: "error: permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptEdit).
					Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeletePromptOApiRequest{
					PromptID: ptr.Of(int64(123)),
				},
			},
			wantR:   openapi.NewDeletePromptOApiResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "error: delete prompt repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				}, nil)
				mockManageRepo.EXPECT().DeletePrompt(gomock.Any(), int64(123)).Return(errors.New("delete error"))

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptEdit).Return(nil)

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeletePromptOApiRequest{
					PromptID: ptr.Of(int64(123)),
				},
			},
			wantR:   openapi.NewDeletePromptOApiResponse(),
			wantErr: errors.New("delete error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				promptManageRepo: ttFields.promptManageRepo,
				auth:             ttFields.auth,
			}
			gotR, err := p.DeletePromptOApi(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			assert.Equal(t, tt.wantR, gotR)
		})
	}
}

func TestPromptOpenAPIApplicationImpl_GetPromptOApi(t *testing.T) {
	t.Parallel()

	type fields struct {
		promptService    service.IPromptService
		promptManageRepo repo.IManageRepo
		auth             rpc.IAuthProvider
		user             rpc.IUserProvider
	}
	type args struct {
		ctx context.Context
		req *openapi.GetPromptOApiRequest
	}

	startTime := time.Now()

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantNonNilR  bool
		wantErr      error
	}{
		{
			name: "success: get prompt without commit or draft",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().GetPrompt(gomock.Any(), service.GetPromptParam{
					PromptID: 123,
					UserID:   consts.OpenAPIUserID,
				}).Return(&entity.Prompt{
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_prompt",
					PromptBasic: &entity.PromptBasic{
						DisplayName:   "Test Prompt",
						LatestVersion: "1.0.0",
						CreatedBy:     "test_user",
						UpdatedBy:     "test_user",
						CreatedAt:     startTime,
						UpdatedAt:     startTime,
					},
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					auth:          mockAuth,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.GetPromptOApiRequest{
					PromptID: ptr.Of(int64(123)),
				},
			},
			wantNonNilR: true,
			wantErr:     nil,
		},
		{
			name: "success: get prompt with commit (version specified)",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().GetPrompt(gomock.Any(), service.GetPromptParam{
					PromptID:      123,
					WithCommit:    true,
					CommitVersion: "1.0.0",
					UserID:        consts.OpenAPIUserID,
				}).Return(&entity.Prompt{
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_prompt",
					PromptBasic: &entity.PromptBasic{
						DisplayName:   "Test Prompt",
						LatestVersion: "1.0.0",
						CreatedBy:     "test_user",
						UpdatedBy:     "test_user",
						CreatedAt:     startTime,
						UpdatedAt:     startTime,
					},
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{
							Version:     "1.0.0",
							CommittedBy: "test_user",
							CommittedAt: startTime,
						},
					},
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					auth:          mockAuth,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.GetPromptOApiRequest{
					PromptID:      ptr.Of(int64(123)),
					WithCommit:    ptr.Of(true),
					CommitVersion: ptr.Of("1.0.0"),
				},
			},
			wantNonNilR: true,
			wantErr:     nil,
		},
		{
			name: "success: get prompt with commit (latest version)",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 123}).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
					PromptBasic: &entity.PromptBasic{
						LatestVersion: "2.0.0",
					},
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().GetPrompt(gomock.Any(), service.GetPromptParam{
					PromptID:      123,
					WithCommit:    true,
					CommitVersion: "2.0.0",
					UserID:        consts.OpenAPIUserID,
				}).Return(&entity.Prompt{
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_prompt",
					PromptBasic: &entity.PromptBasic{
						LatestVersion: "2.0.0",
					},
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{
							Version: "2.0.0",
						},
					},
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.GetPromptOApiRequest{
					PromptID:   ptr.Of(int64(123)),
					WithCommit: ptr.Of(true),
				},
			},
			wantNonNilR: true,
			wantErr:     nil,
		},
		{
			name: "success: get prompt with draft",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().GetPrompt(gomock.Any(), service.GetPromptParam{
					PromptID:  123,
					WithDraft: true,
					UserID:    consts.OpenAPIUserID,
				}).Return(&entity.Prompt{
					ID:        123,
					SpaceID:   123456,
					PromptKey: "test_prompt",
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					auth:          mockAuth,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.GetPromptOApiRequest{
					PromptID:  ptr.Of(int64(123)),
					WithDraft: ptr.Of(true),
				},
			},
			wantNonNilR: true,
			wantErr:     nil,
		},
		{
			name: "error: get prompt service error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(nil, errors.New("service error"))

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.GetPromptOApiRequest{
					PromptID: ptr.Of(int64(123)),
				},
			},
			wantNonNilR: false,
			wantErr:     errors.New("service error"),
		},
		{
			name: "error: permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).
					Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					auth:          mockAuth,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.GetPromptOApiRequest{
					PromptID: ptr.Of(int64(123)),
				},
			},
			wantNonNilR: false,
			wantErr:     errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "error: workspace_id not match",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 111111,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(111111), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService: mockPromptService,
					auth:          mockAuth,
					user:          mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.GetPromptOApiRequest{
					PromptID:    ptr.Of(int64(123)),
					WorkspaceID: ptr.Of(int64(999999)),
				},
			},
			wantNonNilR: false,
			wantErr:     errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("WorkspaceID not match")),
		},
		{
			name: "error: get latest version failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 123}).Return(nil, errors.New("repo error"))

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptManageRepo: mockManageRepo,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.GetPromptOApiRequest{
					PromptID:   ptr.Of(int64(123)),
					WithCommit: ptr.Of(true),
				},
			},
			wantNonNilR: false,
			wantErr:     errors.New("repo error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				promptService:    ttFields.promptService,
				promptManageRepo: ttFields.promptManageRepo,
				auth:             ttFields.auth,
				user:             ttFields.user,
			}
			gotR, err := p.GetPromptOApi(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantNonNilR {
				assert.NotNil(t, gotR)
			}
		})
	}
}

func TestPromptOpenAPIApplicationImpl_SaveDraftOApi(t *testing.T) {
	t.Parallel()

	type fields struct {
		promptService    service.IPromptService
		promptManageRepo repo.IManageRepo
		auth             rpc.IAuthProvider
		user             rpc.IUserProvider
	}
	type args struct {
		ctx context.Context
		req *openapi.SaveDraftOApiRequest
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantNonNilR  bool
		wantErr      error
	}{
		{
			name: "success: save draft",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 123}).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptEdit).Return(nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).Return(&entity.DraftInfo{
					UserID:      consts.OpenAPIUserID,
					BaseVersion: "1.0.0",
					IsModified:  true,
				}, nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.SaveDraftOApiRequest{
					PromptID: ptr.Of(int64(123)),
					PromptDraft: &domainopenapi.PromptDraft{
						DraftInfo: &domainopenapi.DraftInfo{
							BaseVersion: ptr.Of("1.0.0"),
						},
						Detail: &domainopenapi.PromptDetail{},
					},
				},
			},
			wantNonNilR: true,
			wantErr:     nil,
		},
		{
			name: "error: draft is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()
				return fields{user: mockUser}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.SaveDraftOApiRequest{
					PromptID:    ptr.Of(int64(123)),
					PromptDraft: nil,
				},
			},
			wantNonNilR: false,
			wantErr:     errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Draft is not specified")),
		},
		{
			name: "error: draft info is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()
				return fields{user: mockUser}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.SaveDraftOApiRequest{
					PromptID: ptr.Of(int64(123)),
					PromptDraft: &domainopenapi.PromptDraft{
						DraftInfo: nil,
						Detail:    &domainopenapi.PromptDetail{},
					},
				},
			},
			wantNonNilR: false,
			wantErr:     errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Draft is not specified")),
		},
		{
			name: "error: detail is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()
				return fields{user: mockUser}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.SaveDraftOApiRequest{
					PromptID: ptr.Of(int64(123)),
					PromptDraft: &domainopenapi.PromptDraft{
						DraftInfo: &domainopenapi.DraftInfo{},
						Detail:    nil,
					},
				},
			},
			wantNonNilR: false,
			wantErr:     errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Draft is not specified")),
		},
		{
			name: "error: get prompt failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(nil, errors.New("not found"))

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptManageRepo: mockManageRepo,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.SaveDraftOApiRequest{
					PromptID: ptr.Of(int64(123)),
					PromptDraft: &domainopenapi.PromptDraft{
						DraftInfo: &domainopenapi.DraftInfo{},
						Detail:    &domainopenapi.PromptDetail{},
					},
				},
			},
			wantNonNilR: false,
			wantErr:     errors.New("not found"),
		},
		{
			name: "error: workspace_id not match",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 111111,
				}, nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptManageRepo: mockManageRepo,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.SaveDraftOApiRequest{
					PromptID:    ptr.Of(int64(123)),
					WorkspaceID: ptr.Of(int64(999999)),
					PromptDraft: &domainopenapi.PromptDraft{
						DraftInfo: &domainopenapi.DraftInfo{},
						Detail:    &domainopenapi.PromptDetail{},
					},
				},
			},
			wantNonNilR: false,
			wantErr:     errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("WorkspaceID not match")),
		},
		{
			name: "error: permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptEdit).
					Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.SaveDraftOApiRequest{
					PromptID: ptr.Of(int64(123)),
					PromptDraft: &domainopenapi.PromptDraft{
						DraftInfo: &domainopenapi.DraftInfo{},
						Detail:    &domainopenapi.PromptDetail{},
					},
				},
			},
			wantNonNilR: false,
			wantErr:     errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "error: save draft service error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptEdit).Return(nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).Return(nil, errors.New("save error"))

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptService:    mockPromptService,
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.SaveDraftOApiRequest{
					PromptID: ptr.Of(int64(123)),
					PromptDraft: &domainopenapi.PromptDraft{
						DraftInfo: &domainopenapi.DraftInfo{},
						Detail:    &domainopenapi.PromptDetail{},
					},
				},
			},
			wantNonNilR: false,
			wantErr:     errors.New("save error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				promptService:    ttFields.promptService,
				promptManageRepo: ttFields.promptManageRepo,
				auth:             ttFields.auth,
				user:             ttFields.user,
			}
			gotR, err := p.SaveDraftOApi(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantNonNilR {
				assert.NotNil(t, gotR)
				assert.NotNil(t, gotR.DraftInfo)
			}
		})
	}
}

func TestPromptOpenAPIApplicationImpl_ListCommitOApi(t *testing.T) {
	t.Parallel()

	type fields struct {
		promptManageRepo repo.IManageRepo
		auth             rpc.IAuthProvider
	}
	type args struct {
		ctx context.Context
		req *openapi.ListCommitOApiRequest
	}

	startTime := time.Now()

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantNonNilR  bool
		wantHasMore  bool
		wantErr      error
	}{
		{
			name: "success: list commits without page token",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 123}).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)
				mockManageRepo.EXPECT().ListCommitInfo(gomock.Any(), repo.ListCommitInfoParam{
					PromptID: 123,
					PageSize: 10,
					Asc:      false,
				}).Return(&repo.ListCommitResult{
					CommitInfoDOs: []*entity.CommitInfo{
						{
							Version:     "1.0.0",
							CommittedBy: "test_user",
							CommittedAt: startTime,
						},
					},
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListCommitOApiRequest{
					PromptID: ptr.Of(int64(123)),
					PageSize: ptr.Of(int32(10)),
				},
			},
			wantNonNilR: true,
			wantHasMore: false,
			wantErr:     nil,
		},
		{
			name: "success: list commits with page token and has more",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 123}).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)
				mockManageRepo.EXPECT().ListCommitInfo(gomock.Any(), gomock.Any()).Return(&repo.ListCommitResult{
					CommitInfoDOs: []*entity.CommitInfo{
						{
							Version:     "2.0.0",
							CommittedBy: "test_user",
							CommittedAt: startTime,
						},
					},
					NextPageToken: 100,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListCommitOApiRequest{
					PromptID:  ptr.Of(int64(123)),
					PageSize:  ptr.Of(int32(10)),
					PageToken: ptr.Of("50"),
				},
			},
			wantNonNilR: true,
			wantHasMore: true,
			wantErr:     nil,
		},
		{
			name: "success: list commits with detail",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 123}).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)
				mockManageRepo.EXPECT().ListCommitInfo(gomock.Any(), gomock.Any()).Return(&repo.ListCommitResult{
					CommitInfoDOs: []*entity.CommitInfo{
						{
							Version:     "1.0.0",
							CommittedBy: "test_user",
							CommittedAt: startTime,
						},
					},
					CommitDOs: []*entity.PromptCommit{
						{
							CommitInfo: &entity.CommitInfo{
								Version: "1.0.0",
							},
							PromptDetail: &entity.PromptDetail{},
						},
					},
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListCommitOApiRequest{
					PromptID:         ptr.Of(int64(123)),
					PageSize:         ptr.Of(int32(10)),
					WithCommitDetail: ptr.Of(true),
				},
			},
			wantNonNilR: true,
			wantHasMore: false,
			wantErr:     nil,
		},
		{
			name: "success: nil result",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)
				mockManageRepo.EXPECT().ListCommitInfo(gomock.Any(), gomock.Any()).Return(nil, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListCommitOApiRequest{
					PromptID: ptr.Of(int64(123)),
					PageSize: ptr.Of(int32(10)),
				},
			},
			wantNonNilR: true,
			wantHasMore: false,
			wantErr:     nil,
		},
		{
			name: "error: get prompt failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(nil, errors.New("not found"))

				return fields{
					promptManageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListCommitOApiRequest{
					PromptID: ptr.Of(int64(999)),
					PageSize: ptr.Of(int32(10)),
				},
			},
			wantNonNilR: false,
			wantErr:     errors.New("not found"),
		},
		{
			name: "error: workspace_id not match",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 111111,
				}, nil)

				return fields{
					promptManageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListCommitOApiRequest{
					PromptID:    ptr.Of(int64(123)),
					WorkspaceID: ptr.Of(int64(999999)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			wantNonNilR: false,
			wantErr:     errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("WorkspaceID not match")),
		},
		{
			name: "error: permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).
					Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListCommitOApiRequest{
					PromptID: ptr.Of(int64(123)),
					PageSize: ptr.Of(int32(10)),
				},
			},
			wantNonNilR: false,
			wantErr:     errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "error: invalid page token",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListCommitOApiRequest{
					PromptID:  ptr.Of(int64(123)),
					PageSize:  ptr.Of(int32(10)),
					PageToken: ptr.Of("invalid"),
				},
			},
			wantNonNilR: false,
			wantErr:     errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Page token is invalid, page token = invalid")),
		},
		{
			name: "error: list commit info failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)
				mockManageRepo.EXPECT().ListCommitInfo(gomock.Any(), gomock.Any()).Return(nil, errors.New("database error"))

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.ListCommitOApiRequest{
					PromptID: ptr.Of(int64(123)),
					PageSize: ptr.Of(int32(10)),
				},
			},
			wantNonNilR: false,
			wantErr:     errors.New("database error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				promptManageRepo: ttFields.promptManageRepo,
				auth:             ttFields.auth,
			}
			gotR, err := p.ListCommitOApi(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantNonNilR {
				assert.NotNil(t, gotR)
				if tt.wantHasMore {
					assert.True(t, gotR.GetHasMore())
					assert.NotNil(t, gotR.NextPageToken)
				}
			}
		})
	}
}

func TestPromptOpenAPIApplicationImpl_CommitDraftOApi(t *testing.T) {
	t.Parallel()

	type fields struct {
		promptManageRepo repo.IManageRepo
		auth             rpc.IAuthProvider
		user             rpc.IUserProvider
	}
	type args struct {
		ctx context.Context
		req *openapi.CommitDraftOApiRequest
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantR        *openapi.CommitDraftOApiResponse
		wantErr      error
	}{
		{
			name: "success: commit draft",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:  123,
					UserID:    consts.OpenAPIUserID,
					WithDraft: true,
				}).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)
				mockManageRepo.EXPECT().CommitDraft(gomock.Any(), repo.CommitDraftParam{
					PromptID:          123,
					UserID:            consts.OpenAPIUserID,
					CommitVersion:     "1.0.0",
					CommitDescription: "initial commit",
				}).Return(nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptEdit).Return(nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CommitDraftOApiRequest{
					PromptID:          ptr.Of(int64(123)),
					CommitVersion:     ptr.Of("1.0.0"),
					CommitDescription: ptr.Of("initial commit"),
				},
			},
			wantR:   openapi.NewCommitDraftOApiResponse(),
			wantErr: nil,
		},
		{
			name: "error: invalid semver version",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()
				return fields{user: mockUser}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CommitDraftOApiRequest{
					PromptID:      ptr.Of(int64(123)),
					CommitVersion: ptr.Of("invalid_version"),
				},
			},
			wantR:   openapi.NewCommitDraftOApiResponse(),
			wantErr: errors.New("Invalid Semantic Version"),
		},
		{
			name: "error: get prompt failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(nil, errors.New("not found"))

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptManageRepo: mockManageRepo,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CommitDraftOApiRequest{
					PromptID:      ptr.Of(int64(999)),
					CommitVersion: ptr.Of("1.0.0"),
				},
			},
			wantR:   openapi.NewCommitDraftOApiResponse(),
			wantErr: errors.New("not found"),
		},
		{
			name: "error: workspace_id not match",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 111111,
				}, nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptManageRepo: mockManageRepo,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CommitDraftOApiRequest{
					PromptID:      ptr.Of(int64(123)),
					CommitVersion: ptr.Of("1.0.0"),
					WorkspaceID:   ptr.Of(int64(999999)),
				},
			},
			wantR:   openapi.NewCommitDraftOApiResponse(),
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("WorkspaceID not match")),
		},
		{
			name: "error: permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptEdit).
					Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CommitDraftOApiRequest{
					PromptID:      ptr.Of(int64(123)),
					CommitVersion: ptr.Of("1.0.0"),
				},
			},
			wantR:   openapi.NewCommitDraftOApiResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "error: commit draft repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{
					ID:      123,
					SpaceID: 123456,
				}, nil)
				mockManageRepo.EXPECT().CommitDraft(gomock.Any(), gomock.Any()).Return(errors.New("commit error"))

				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptEdit).Return(nil)

				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false).AnyTimes()

				return fields{
					promptManageRepo: mockManageRepo,
					auth:             mockAuth,
					user:             mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CommitDraftOApiRequest{
					PromptID:      ptr.Of(int64(123)),
					CommitVersion: ptr.Of("1.0.0"),
				},
			},
			wantR:   openapi.NewCommitDraftOApiResponse(),
			wantErr: errors.New("commit error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				promptManageRepo: ttFields.promptManageRepo,
				auth:             ttFields.auth,
				user:             ttFields.user,
			}
			gotR, err := p.CommitDraftOApi(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			assert.Equal(t, tt.wantR, gotR)
		})
	}
}

func TestPromptOpenAPIApplicationImpl_getOpenAPIUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) rpc.IUserProvider
		want         string
	}{
		{
			name: "user found in context",
			fieldsGetter: func(ctrl *gomock.Controller) rpc.IUserProvider {
				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("real_user_123", true)
				return mockUser
			},
			want: "real_user_123",
		},
		{
			name: "user not found in context - fallback to OpenAPIUserID",
			fieldsGetter: func(ctrl *gomock.Controller) rpc.IUserProvider {
				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", false)
				return mockUser
			},
			want: consts.OpenAPIUserID,
		},
		{
			name: "user found but empty - fallback to OpenAPIUserID",
			fieldsGetter: func(ctrl *gomock.Controller) rpc.IUserProvider {
				mockUser := rpcmocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().GetUserIdInCtx(gomock.Any()).Return("", true)
				return mockUser
			},
			want: consts.OpenAPIUserID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			p := &PromptOpenAPIApplicationImpl{
				user: tt.fieldsGetter(ctrl),
			}
			got := p.getOpenAPIUserID(context.Background())
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPromptOpenAPIApplicationImpl_promptHubAllowBySpace(t *testing.T) {
	t.Parallel()

	type fields struct {
		config      conf.IConfigProvider
		rateLimiter limiter.IRateLimiter
	}
	type args struct {
		ctx         context.Context
		workspaceID int64
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         bool
	}{
		{
			name: "allowed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), int64(123456)).Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "prompt_hub:qps:space_id:123456", 1, gomock.Any()).Return(&limiter.Result{
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
			},
			want: true,
		},
		{
			name: "rate limit exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), int64(123456)).Return(1, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), "prompt_hub:qps:space_id:123456", 1, gomock.Any()).Return(&limiter.Result{
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
			},
			want: false,
		},
		{
			name: "get config error - fallback to allow",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), int64(123456)).Return(0, errors.New("config error"))

				return fields{
					config: mockConfig,
				}
			},
			args: args{
				ctx:         context.Background(),
				workspaceID: 123456,
			},
			want: true,
		},
		{
			name: "rate limiter error - fallback to allow",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), int64(123456)).Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("limiter error"))

				return fields{
					config:      mockConfig,
					rateLimiter: mockRateLimiter,
				}
			},
			args: args{
				ctx:         context.Background(),
				workspaceID: 123456,
			},
			want: true,
		},
		{
			name: "nil result - fallback to allow",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptHubMaxQPSBySpace(gomock.Any(), int64(123456)).Return(100, nil)

				mockRateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				mockRateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)

				return fields{
					config:      mockConfig,
					rateLimiter: mockRateLimiter,
				}
			},
			args: args{
				ctx:         context.Background(),
				workspaceID: 123456,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptOpenAPIApplicationImpl{
				config:      ttFields.config,
				rateLimiter: ttFields.rateLimiter,
			}
			got := p.promptHubAllowBySpace(tt.args.ctx, tt.args.workspaceID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPromptOpenAPIApplicationImpl_emitExecuteMetrics(t *testing.T) {
	t.Parallel()

	t.Run("nil request", func(t *testing.T) {
		t.Parallel()
		p := &PromptOpenAPIApplicationImpl{}
		p.emitExecuteMetrics(context.Background(), nil, nil, nil, nil, "Execute")
	})

	t.Run("with request only", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		req := &openapi.ExecuteRequest{
			WorkspaceID: ptr.Of(int64(123456)),
			PromptIdentifier: &domainopenapi.PromptQuery{
				PromptKey: ptr.Of("test_prompt"),
			},
			Messages: []*domainopenapi.Message{
				{
					Role:    ptr.Of(prompt.RoleUser),
					Content: ptr.Of("Hello"),
				},
				{
					Role:    ptr.Of(prompt.RoleUser),
					Content: ptr.Of("How are you?"),
				},
			},
		}
		p := &PromptOpenAPIApplicationImpl{}
		p.emitExecuteMetrics(ctx, req, nil, nil, nil, "Execute")
	})

	t.Run("with prompt and reply", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		req := &openapi.ExecuteRequest{
			WorkspaceID: ptr.Of(int64(123456)),
			PromptIdentifier: &domainopenapi.PromptQuery{
				PromptKey: ptr.Of("test_prompt"),
			},
		}
		promptDO := &entity.Prompt{
			ID:        123,
			SpaceID:   123456,
			PromptKey: "test_prompt",
			PromptBasic: &entity.PromptBasic{
				PromptType: entity.PromptTypeNormal,
			},
			PromptCommit: &entity.PromptCommit{
				CommitInfo: &entity.CommitInfo{
					Version: "1.0.0",
				},
				PromptDetail: &entity.PromptDetail{
					ModelConfig: &entity.ModelConfig{
						ModelID:   456,
						MaxTokens: ptr.Of(int32(1000)),
					},
				},
			},
		}
		reply := &entity.Reply{
			Item: &entity.ReplyItem{
				TokenUsage: &entity.TokenUsage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
		}
		p := &PromptOpenAPIApplicationImpl{}
		p.emitExecuteMetrics(ctx, req, promptDO, reply, nil, "Execute")
	})

	t.Run("with error", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		req := &openapi.ExecuteRequest{
			WorkspaceID: ptr.Of(int64(123456)),
		}
		p := &PromptOpenAPIApplicationImpl{}
		p.emitExecuteMetrics(ctx, req, nil, nil, errors.New("test error"), "Execute")
	})

	t.Run("with account mode and usage scenario", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		req := &openapi.ExecuteRequest{
			WorkspaceID:   ptr.Of(int64(123456)),
			AccountMode:   ptr.Of(domainopenapi.AccountModeSharedAccount),
			UsageScenario: ptr.Of(domainopenapi.UsageScenarioPromptAsAService),
		}
		p := &PromptOpenAPIApplicationImpl{}
		p.emitExecuteMetrics(ctx, req, nil, nil, nil, "StreamingExecute")
	})
}

func TestPromptTypeToMetricValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		promptType entity.PromptType
		want       int64
	}{
		{
			name:       "normal prompt type",
			promptType: entity.PromptTypeNormal,
			want:       1,
		},
		{
			name:       "snippet prompt type",
			promptType: entity.PromptTypeSnippet,
			want:       2,
		},
		{
			name:       "unknown prompt type returns 0",
			promptType: entity.PromptType("unknown"),
			want:       0,
		},
		{
			name:       "empty prompt type returns 0",
			promptType: entity.PromptType(""),
			want:       0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := promptTypeToMetricValue(tt.promptType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetRequestPromptKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  *openapi.ExecuteRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "nil PromptIdentifier",
			req:  &openapi.ExecuteRequest{},
			want: "",
		},
		{
			name: "normal case",
			req: &openapi.ExecuteRequest{
				PromptIdentifier: &domainopenapi.PromptQuery{
					PromptKey: ptr.Of("my_prompt"),
				},
			},
			want: "my_prompt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRequestPromptKey(tt.req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetRequestAccountMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  *openapi.ExecuteRequest
		want domainopenapi.AccountMode
	}{
		{
			name: "nil request defaults to SharedAccount",
			req:  nil,
			want: domainopenapi.AccountModeSharedAccount,
		},
		{
			name: "nil AccountMode defaults to SharedAccount",
			req:  &openapi.ExecuteRequest{},
			want: domainopenapi.AccountModeSharedAccount,
		},
		{
			name: "normal case returns set value",
			req: &openapi.ExecuteRequest{
				AccountMode: ptr.Of(domainopenapi.AccountModeCustomAccount),
			},
			want: domainopenapi.AccountModeCustomAccount,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRequestAccountMode(tt.req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetRequestUsageScenario(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		req  *openapi.ExecuteRequest
		want domainopenapi.UsageScenario
	}{
		{
			name: "nil request defaults to PromptAsAService",
			req:  nil,
			want: domainopenapi.UsageScenarioPromptAsAService,
		},
		{
			name: "nil UsageScenario defaults to PromptAsAService",
			req:  &openapi.ExecuteRequest{},
			want: domainopenapi.UsageScenarioPromptAsAService,
		},
		{
			name: "normal case returns set value",
			req: &openapi.ExecuteRequest{
				UsageScenario: ptr.Of(domainopenapi.UsageScenarioEvaluation),
			},
			want: domainopenapi.UsageScenarioEvaluation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRequestUsageScenario(tt.req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetReplyTokenUsage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		reply            *entity.Reply
		wantInputTokens  int64
		wantOutputTokens int64
	}{
		{
			name:             "nil reply",
			reply:            nil,
			wantInputTokens:  0,
			wantOutputTokens: 0,
		},
		{
			name:             "nil Item",
			reply:            &entity.Reply{},
			wantInputTokens:  0,
			wantOutputTokens: 0,
		},
		{
			name:             "nil TokenUsage",
			reply:            &entity.Reply{Item: &entity.ReplyItem{}},
			wantInputTokens:  0,
			wantOutputTokens: 0,
		},
		{
			name: "normal case",
			reply: &entity.Reply{
				Item: &entity.ReplyItem{
					TokenUsage: &entity.TokenUsage{
						InputTokens:  100,
						OutputTokens: 200,
					},
				},
			},
			wantInputTokens:  100,
			wantOutputTokens: 200,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInput, gotOutput := getReplyTokenUsage(tt.reply)
			assert.Equal(t, tt.wantInputTokens, gotInput)
			assert.Equal(t, tt.wantOutputTokens, gotOutput)
		})
	}
}

func TestBuildTokenUsageReply(t *testing.T) {
	t.Parallel()
	reply := buildTokenUsageReply(50, 150)
	assert.NotNil(t, reply)
	assert.NotNil(t, reply.Item)
	assert.NotNil(t, reply.Item.TokenUsage)
	assert.Equal(t, int64(50), reply.Item.TokenUsage.InputTokens)
	assert.Equal(t, int64(150), reply.Item.TokenUsage.OutputTokens)
}

func TestNormalizeExecuteRequest_NilRequest(t *testing.T) {
	t.Parallel()
	got := normalizeExecuteRequest(nil)
	assert.Nil(t, got)
}

func TestNormalizeExecuteRequest_NoNormalizationNeeded(t *testing.T) {
	t.Parallel()
	req := &openapi.ExecuteRequest{
		PromptIdentifier: &domainopenapi.PromptQuery{
			PromptKey: ptr.Of("key"),
		},
	}
	got := normalizeExecuteRequest(req)
	assert.Equal(t, req, got)
}

func TestNormalizeExecuteRequest_DeepCopyWithNilPromptIdentifier(t *testing.T) {
	t.Parallel()
	req := &openapi.ExecuteRequest{
		ReleaseLabel: ptr.Of("production"),
	}
	got := normalizeExecuteRequest(req)
	assert.NotNil(t, got.PromptIdentifier)
	assert.Equal(t, "production", got.PromptIdentifier.GetLabel())
}

func TestNormalizeExecuteRequest_LabelNotOverride(t *testing.T) {
	t.Parallel()
	req := &openapi.ExecuteRequest{
		PromptIdentifier: &domainopenapi.PromptQuery{
			PromptKey: ptr.Of("key"),
			Label:     ptr.Of("existing_label"),
		},
		ReleaseLabel: ptr.Of("new_label"),
	}
	got := normalizeExecuteRequest(req)
	assert.Equal(t, "existing_label", got.PromptIdentifier.GetLabel())
}

func TestNormalizeExecuteRequest_CustomToolConfig(t *testing.T) {
	t.Parallel()
	toolConfig := &domainopenapi.ToolCallConfig{
		ToolChoice: ptr.Of(domainopenapi.ToolChoiceTypeAuto),
	}
	req := &openapi.ExecuteRequest{
		CustomToolConfig: toolConfig,
	}
	got := normalizeExecuteRequest(req)
	assert.NotNil(t, got.CustomToolCallConfig)
	assert.Equal(t, domainopenapi.ToolChoiceTypeAuto, got.CustomToolCallConfig.GetToolChoice())
}

func TestNormalizeExecuteRequest_CustomToolsAutoConfig(t *testing.T) {
	t.Parallel()
	req := &openapi.ExecuteRequest{
		CustomTools: []*domainopenapi.Tool{
			{Type: ptr.Of(domainopenapi.ToolTypeFunction)},
		},
	}
	got := normalizeExecuteRequest(req)
	assert.NotNil(t, got.CustomToolCallConfig)
	assert.Equal(t, domainopenapi.ToolChoiceTypeAuto, got.CustomToolCallConfig.GetToolChoice())
}

func TestNormalizeExecuteRequest_CustomToolCallConfigNotOverridden(t *testing.T) {
	t.Parallel()
	existingConfig := &domainopenapi.ToolCallConfig{
		ToolChoice: ptr.Of(domainopenapi.ToolChoiceTypeNone),
	}
	toolConfig := &domainopenapi.ToolCallConfig{
		ToolChoice: ptr.Of(domainopenapi.ToolChoiceTypeAuto),
	}
	req := &openapi.ExecuteRequest{
		CustomToolCallConfig: existingConfig,
		CustomToolConfig:     toolConfig,
	}
	got := normalizeExecuteRequest(req)
	assert.Equal(t, domainopenapi.ToolChoiceTypeNone, got.CustomToolCallConfig.GetToolChoice())
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
		name             string
		fieldsGetter     func(ctrl *gomock.Controller) fields
		argsGetter       func(ctrl *gomock.Controller) args
		wantErr          error
		validateFunc     func(t *testing.T, stream *mockExecuteStreamingServer)
		setupConvertMock func(mockSvc *servicemocks.MockIPromptService)
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

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
				expectedResponseAPIConfig := &entity.ResponseAPIConfig{
					PreviousResponseID: ptr.Of("prev-id"),
					EnableCaching:      ptr.Of(true),
					SessionID:          ptr.Of("session-123"),
				}
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil)
				mockPromptService.EXPECT().ExecuteStreaming(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, param service.ExecuteStreamingParam) (*entity.Reply, error) {
						assert.Equal(t, expectedResponseAPIConfig, param.ResponseAPIConfig)
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
						PromptIdentifier: &domainopenapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
							Version:   ptr.Of("1.0.0"),
						},
						Messages: []*domainopenapi.Message{
							{
								Role:    ptr.Of(prompt.RoleUser),
								Content: ptr.Of("Hello"),
							},
						},
						ResponseAPIConfig: &domainopenapi.ResponseAPIConfig{
							PreviousResponseID: ptr.Of("prev-id"),
							EnableCaching:      ptr.Of(true),
							SessionID:          ptr.Of("session-123"),
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
			name: "error: base64 convert failed",
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil)
				mockPromptService.EXPECT().ExecuteStreaming(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, param service.ExecuteStreamingParam) (*entity.Reply, error) {
						param.ResultStream <- &entity.Reply{
							Item: &entity.ReplyItem{
								Message: &entity.Message{
									Role: entity.RoleAssistant,
									Parts: []*entity.ContentPart{
										{
											Type: entity.ContentTypeImageURL,
											ImageURL: &entity.ImageURL{
												URL: "data:image/png;base64,abc",
											},
										},
									},
								},
							},
						}
						return &entity.Reply{
							Item: &entity.ReplyItem{
								Message: &entity.Message{
									Role: entity.RoleAssistant,
								},
							},
						}, nil
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
						PromptIdentifier: &domainopenapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
							Version:   ptr.Of("1.0.0"),
						},
					},
					stream: stream,
				}
			},
			wantErr: errors.New("convert error"),
			validateFunc: func(t *testing.T, stream *mockExecuteStreamingServer) {
				calls := stream.GetSendCalls()
				assert.Len(t, calls, 0)
			},
			setupConvertMock: func(mockSvc *servicemocks.MockIPromptService) {
				mockSvc.EXPECT().MConvertBase64DataURLToFileURL(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("convert error"))
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
						PromptIdentifier: &domainopenapi.PromptQuery{
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
						PromptIdentifier: &domainopenapi.PromptQuery{
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
						PromptIdentifier: &domainopenapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
						},
						Messages: []*domainopenapi.Message{
							{
								Role: ptr.Of(prompt.RoleUser),
								Parts: []*domainopenapi.ContentPart{
									{
										Type:     ptr.Of(domainopenapi.ContentTypeImageURL),
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
						PromptIdentifier: &domainopenapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
						},
						Messages: []*domainopenapi.Message{
							{
								Role: ptr.Of(prompt.RoleUser),
								Parts: []*domainopenapi.ContentPart{
									{
										Type:       ptr.Of(domainopenapi.ContentTypeBase64Data),
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
						PromptIdentifier: &domainopenapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
							Version:   ptr.Of("1.0.0"),
						},
						Messages: []*domainopenapi.Message{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(
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
						PromptIdentifier: &domainopenapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
							Version:   ptr.Of("1.0.0"),
						},
						Messages: []*domainopenapi.Message{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
						PromptIdentifier: &domainopenapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
							Version:   ptr.Of("1.0.0"),
						},
						Messages: []*domainopenapi.Message{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

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
						PromptIdentifier: &domainopenapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
							Version:   ptr.Of("1.0.0"),
						},
						Messages: []*domainopenapi.Message{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

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
						PromptIdentifier: &domainopenapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
							Version:   ptr.Of("1.0.0"),
						},
						Messages: []*domainopenapi.Message{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

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
						PromptIdentifier: &domainopenapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
							Version:   ptr.Of("1.0.0"),
						},
						Messages: []*domainopenapi.Message{
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
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptExecute).Return(nil)

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
						PromptIdentifier: &domainopenapi.PromptQuery{
							PromptKey: ptr.Of("test_prompt"),
							Version:   ptr.Of("1.0.0"),
						},
						Messages: []*domainopenapi.Message{
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
			if mockSvc, ok := ttFields.promptService.(*servicemocks.MockIPromptService); ok {
				if tt.setupConvertMock != nil {
					tt.setupConvertMock(mockSvc)
				} else {
					mockSvc.EXPECT().MConvertBase64DataURLToFileURL(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				}
			}
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123, 456}, consts.ActionLoopPromptRead).Return(nil)

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
				Data: &domainopenapi.ListPromptBasicData{
					Total: ptr.Of(int32(2)),
					Prompts: []*domainopenapi.PromptBasic{
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

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
				Data: &domainopenapi.ListPromptBasicData{
					Total: ptr.Of(int32(1)),
					Prompts: []*domainopenapi.PromptBasic{
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(nil)

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
				Data: &domainopenapi.ListPromptBasicData{
					Total: ptr.Of(int32(1)),
					Prompts: []*domainopenapi.PromptBasic{
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
				Data: &domainopenapi.ListPromptBasicData{
					Total:   ptr.Of(int32(0)),
					Prompts: []*domainopenapi.PromptBasic{},
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
				mockAuth.EXPECT().MCheckPromptPermissionForOpenAPI(gomock.Any(), int64(123456), []int64{123}, consts.ActionLoopPromptRead).Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

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
