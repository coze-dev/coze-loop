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
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func TestPromptOpenAPIApplicationImpl_BatchGetPromptByPromptKey(t *testing.T) {
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
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptKeyVersionPair]string{
					{PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
					{PromptKey: "test_prompt2", Version: "1.0.0"}: "1.0.0",
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
					"test_prompt2": 456,
				}, nil)
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptKeyVersionPair]string{
					{PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
					{PromptKey: "test_prompt1", Version: "2.0.0"}: "2.0.0",
					{PromptKey: "test_prompt1", Version: ""}:      "2.0.0",
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
									TemplateType: ptr.Of(openapi.TemplateTypeNormal),
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
									TemplateType: ptr.Of(openapi.TemplateTypeNormal),
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
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptKeyVersionPair]string{
					{PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
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
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptKeyVersionPair]string{
					{PromptKey: "test_prompt1", Version: "non_existent_version"}: "non_existent_version",
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
				mockPromptService.EXPECT().MParseCommitVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[service.PromptKeyVersionPair]string{
					{PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
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
			gotR, err := p.BatchGetPromptByPromptKey(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			assert.Equal(t, tt.wantR, gotR)
		})
	}
}
