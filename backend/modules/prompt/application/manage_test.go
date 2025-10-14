// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/user"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/manage"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func TestPromptManageApplicationImpl_ClonePrompt(t *testing.T) {
	type fields struct {
		manageRepo      repo.IManageRepo
		promptService   service.IPromptService
		authRPCProvider rpc.IAuthProvider
		userRPCProvider rpc.IUserProvider
		configProvider  conf.IConfigProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.ClonePromptRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.ClonePromptResponse
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.ClonePromptRequest{
					PromptID:                ptr.Of(int64(1)),
					CommitVersion:           ptr.Of("1.0.0"),
					ClonedPromptKey:         ptr.Of("test_key"),
					ClonedPromptDescription: ptr.Of("test description"),
				},
			},
			want:    manage.NewClonePromptResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "get prompt error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				}).Return(nil, errorx.New("get prompt error"))

				return fields{
					manageRepo: mockRepo,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ClonePromptRequest{
					PromptID:                ptr.Of(int64(1)),
					CommitVersion:           ptr.Of("1.0.0"),
					ClonedPromptKey:         ptr.Of("test_key"),
					ClonedPromptDescription: ptr.Of("test description"),
				},
			},
			want:    manage.NewClonePromptResponse(),
			wantErr: errorx.New("get prompt error"),
		},
		{
			name: "create prompt error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "source_key",
					PromptCommit: &entity.PromptCommit{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("test content"),
									},
								},
							},
						},
					},
				}, nil)

				// 注意：在promptService.CreatePrompt内部会调用manageRepo.CreatePrompt
				// 当manageRepo.CreatePrompt返回错误时，promptService.CreatePrompt也会返回错误
				mockRepo.EXPECT().CreatePrompt(gomock.Any(), gomock.Any()).Return(int64(0), errorx.New("create prompt error")).MinTimes(0).MaxTimes(1)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().CreatePrompt(gomock.Any(), gomock.Any()).Return(int64(0), errorx.New("create prompt error"))

				return fields{
					manageRepo:      mockRepo,
					promptService:   mockPromptService,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ClonePromptRequest{
					PromptID:                ptr.Of(int64(1)),
					CommitVersion:           ptr.Of("1.0.0"),
					ClonedPromptKey:         ptr.Of("test_key"),
					ClonedPromptDescription: ptr.Of("test description"),
				},
			},
			want:    manage.NewClonePromptResponse(),
			wantErr: errorx.New("create prompt error"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "source_key",
					PromptCommit: &entity.PromptCommit{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("test content"),
									},
								},
							},
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().CreatePrompt(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, prompt *entity.Prompt) (int64, error) {
					assert.Equal(t, "test_key", prompt.PromptKey)
					assert.Equal(t, "test_key", prompt.PromptBasic.DisplayName)
					assert.Equal(t, "test description", prompt.PromptBasic.Description)
					assert.Equal(t, "123", prompt.PromptBasic.CreatedBy)
					assert.Equal(t, "123", prompt.PromptDraft.DraftInfo.UserID)
					assert.True(t, prompt.PromptDraft.DraftInfo.IsModified)
					assert.Equal(t, entity.TemplateTypeNormal, prompt.PromptDraft.PromptDetail.PromptTemplate.TemplateType)
					assert.Equal(t, "test content", *prompt.PromptDraft.PromptDetail.PromptTemplate.Messages[0].Content)
					return 1001, nil
				})

				return fields{
					manageRepo:      mockRepo,
					promptService:   mockPromptService,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ClonePromptRequest{
					PromptID:                ptr.Of(int64(1)),
					CommitVersion:           ptr.Of("1.0.0"),
					ClonedPromptName:        ptr.Of("test_key"),
					ClonedPromptKey:         ptr.Of("test_key"),
					ClonedPromptDescription: ptr.Of("test description"),
				},
			},
			want: &manage.ClonePromptResponse{
				ClonedPromptID: ptr.Of(int64(1001)),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			d := &PromptManageApplicationImpl{
				manageRepo:      ttFields.manageRepo,
				promptService:   ttFields.promptService,
				authRPCProvider: ttFields.authRPCProvider,
				userRPCProvider: ttFields.userRPCProvider,
				configProvider:  ttFields.configProvider,
			}

			got, err := d.ClonePrompt(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPromptManageApplicationImpl_GetPrompt(t *testing.T) {
	type fields struct {
		manageRepo      repo.IManageRepo
		promptService   service.IPromptService
		authRPCProvider rpc.IAuthProvider
		userRPCProvider rpc.IUserProvider
		configProvider  conf.IConfigProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.GetPromptRequest
	}
	now := time.Now()
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.GetPromptResponse
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.GetPromptRequest{
					PromptID: ptr.Of(int64(1)),
				},
			},
			want:    manage.NewGetPromptResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "get latest version error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID: 1,
				}).Return(nil, errorx.New("get prompt error"))

				return fields{
					manageRepo: mockRepo,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					PromptID:      ptr.Of(int64(1)),
					WithCommit:    ptr.Of(true),
					CommitVersion: nil,
				},
			},
			want:    manage.NewGetPromptResponse(),
			wantErr: errorx.New("get prompt error"),
		},
		{
			name: "get prompt error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
					WithDraft:     false,
					UserID:        "123",
				}).Return(nil, errorx.New("get prompt error"))

				return fields{
					manageRepo: mockRepo,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					PromptID:      ptr.Of(int64(1)),
					WithCommit:    ptr.Of(true),
					CommitVersion: ptr.Of("1.0.0"),
				},
			},
			want:    manage.NewGetPromptResponse(),
			wantErr: errorx.New("get prompt error"),
		},
		{
			name: "get prompt with commit success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
					WithDraft:     false,
					UserID:        "123",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						DisplayName:       "test_name",
						Description:       "test_description",
						LatestVersion:     "1.0.0",
						CreatedBy:         "test_creator",
						UpdatedBy:         "test_updater",
						CreatedAt:         now,
						UpdatedAt:         now,
						LatestCommittedAt: nil,
					},
					PromptCommit: &entity.PromptCommit{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("test content"),
									},
								},
							},
						},
						CommitInfo: &entity.CommitInfo{
							Version:     "1.0.0",
							BaseVersion: "0.9.0",
							Description: "test commit",
							CommittedBy: "test_user",
							CommittedAt: now,
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					PromptID:      ptr.Of(int64(1)),
					WithCommit:    ptr.Of(true),
					CommitVersion: ptr.Of("1.0.0"),
				},
			},
			want: &manage.GetPromptResponse{
				Prompt: &prompt.Prompt{
					ID:          ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					PromptKey:   ptr.Of("test_key"),
					PromptBasic: &prompt.PromptBasic{
						DisplayName:   ptr.Of("test_name"),
						Description:   ptr.Of("test_description"),
						LatestVersion: ptr.Of("1.0.0"),
						CreatedBy:     ptr.Of("test_creator"),
						UpdatedBy:     ptr.Of("test_updater"),
						CreatedAt:     ptr.Of(now.UnixMilli()),
						UpdatedAt:     ptr.Of(now.UnixMilli()),
						PromptType:    ptr.Of(prompt.PromptTypeNormal),
					},
					PromptCommit: &prompt.PromptCommit{
						Detail: &prompt.PromptDetail{
							PromptTemplate: &prompt.PromptTemplate{
								TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								HasSnippet:   ptr.Of(false),
								Messages: []*prompt.Message{
									{
										Role:    ptr.Of(prompt.RoleUser),
										Content: ptr.Of("test content"),
									},
								},
							},
						},
						CommitInfo: &prompt.CommitInfo{
							Version:     ptr.Of("1.0.0"),
							BaseVersion: ptr.Of("0.9.0"),
							Description: ptr.Of("test commit"),
							CommittedBy: ptr.Of("test_user"),
							CommittedAt: ptr.Of(now.UnixMilli()),
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "get prompt with draft success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:  1,
					WithDraft: true,
					UserID:    "123",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						DisplayName:       "test_name",
						Description:       "test_description",
						LatestVersion:     "1.0.0",
						CreatedBy:         "test_creator",
						UpdatedBy:         "test_updater",
						CreatedAt:         now,
						UpdatedAt:         now,
						LatestCommittedAt: nil,
						PromptType:        entity.PromptTypeNormal,
					},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("test content"),
									},
								},
								HasSnippets: false,
							},
						},
						DraftInfo: &entity.DraftInfo{
							UserID:      "123",
							BaseVersion: "1.0.0",
							IsModified:  true,
							CreatedAt:   now,
							UpdatedAt:   now,
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					PromptID:  ptr.Of(int64(1)),
					WithDraft: ptr.Of(true),
				},
			},
			want: &manage.GetPromptResponse{
				Prompt: &prompt.Prompt{
					ID:          ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					PromptKey:   ptr.Of("test_key"),
					PromptBasic: &prompt.PromptBasic{
						DisplayName:       ptr.Of("test_name"),
						Description:       ptr.Of("test_description"),
						LatestVersion:     ptr.Of("1.0.0"),
						CreatedBy:         ptr.Of("test_creator"),
						UpdatedBy:         ptr.Of("test_updater"),
						CreatedAt:         ptr.Of(now.UnixMilli()),
						UpdatedAt:         ptr.Of(now.UnixMilli()),
						LatestCommittedAt: nil,
						PromptType:        ptr.Of(prompt.PromptTypeNormal),
					},
					PromptDraft: &prompt.PromptDraft{
						Detail: &prompt.PromptDetail{
							PromptTemplate: &prompt.PromptTemplate{
								TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								HasSnippet:   ptr.Of(false),
								Messages: []*prompt.Message{
									{
										Role:    ptr.Of(prompt.RoleUser),
										Content: ptr.Of("test content"),
									},
								},
							},
						},
						DraftInfo: &prompt.DraftInfo{
							UserID:      ptr.Of("123"),
							BaseVersion: ptr.Of("1.0.0"),
							IsModified:  ptr.Of(true),
							CreatedAt:   ptr.Of(now.UnixMilli()),
							UpdatedAt:   ptr.Of(now.UnixMilli()),
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "get prompt with latest version success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID: 1,
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						DisplayName:       "test_name",
						Description:       "test_description",
						LatestVersion:     "1.0.0",
						CreatedBy:         "test_creator",
						UpdatedBy:         "test_updater",
						CreatedAt:         now,
						UpdatedAt:         now,
						LatestCommittedAt: nil,
					},
				}, nil)

				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
					WithDraft:     false,
					UserID:        "123",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						DisplayName:   "test_name",
						Description:   "test_description",
						CreatedBy:     "test_creator",
						UpdatedBy:     "test_updater",
						CreatedAt:     now,
						UpdatedAt:     now,
						LatestVersion: "1.0.0",
					},
					PromptCommit: &entity.PromptCommit{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("test content"),
									},
								},
							},
						},
						CommitInfo: &entity.CommitInfo{
							Version:     "1.0.0",
							BaseVersion: "0.9.0",
							Description: "test commit",
							CommittedBy: "test_user",
							CommittedAt: now,
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					PromptID:      ptr.Of(int64(1)),
					WithCommit:    ptr.Of(true),
					CommitVersion: nil,
				},
			},
			want: &manage.GetPromptResponse{
				Prompt: &prompt.Prompt{
					ID:          ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					PromptKey:   ptr.Of("test_key"),
					PromptBasic: &prompt.PromptBasic{
						DisplayName:       ptr.Of("test_name"),
						Description:       ptr.Of("test_description"),
						LatestVersion:     ptr.Of("1.0.0"),
						CreatedBy:         ptr.Of("test_creator"),
						UpdatedBy:         ptr.Of("test_updater"),
						CreatedAt:         ptr.Of(now.UnixMilli()),
						UpdatedAt:         ptr.Of(now.UnixMilli()),
						LatestCommittedAt: nil,
						PromptType:        ptr.Of(prompt.PromptTypeNormal),
					},
					PromptCommit: &prompt.PromptCommit{
						Detail: &prompt.PromptDetail{
							PromptTemplate: &prompt.PromptTemplate{
								TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								HasSnippet:   ptr.Of(false),
								Messages: []*prompt.Message{
									{
										Role:    ptr.Of(prompt.RoleUser),
										Content: ptr.Of("test content"),
									},
								},
							},
						},
						CommitInfo: &prompt.CommitInfo{
							Version:     ptr.Of("1.0.0"),
							BaseVersion: ptr.Of("0.9.0"),
							Description: ptr.Of("test commit"),
							CommittedBy: ptr.Of("test_user"),
							CommittedAt: ptr.Of(now.UnixMilli()),
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "get prompt with default config success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:  1,
					WithDraft: true,
					UserID:    "123",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						DisplayName:       "test_name",
						Description:       "test_description",
						LatestVersion:     "1.0.0",
						CreatedBy:         "test_creator",
						UpdatedBy:         "test_updater",
						CreatedAt:         now,
						UpdatedAt:         now,
						LatestCommittedAt: nil,
					},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("test content"),
									},
								},
							},
						},
						DraftInfo: &entity.DraftInfo{
							UserID:      "123",
							BaseVersion: "1.0.0",
							IsModified:  true,
							CreatedAt:   now,
							UpdatedAt:   now,
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockConfig := &prompt.PromptDetail{
					PromptTemplate: &prompt.PromptTemplate{
						TemplateType: ptr.Of(prompt.TemplateTypeNormal),
						HasSnippet:   ptr.Of(false),
						Messages: []*prompt.Message{
							{
								Role:    ptr.Of(prompt.RoleSystem),
								Content: ptr.Of("Default system message"),
							},
						},
					},
				}
				mockConfigProvider := confmocks.NewMockIConfigProvider(ctrl)
				mockConfigProvider.EXPECT().GetPromptDefaultConfig(gomock.Any()).Return(mockConfig, nil)

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
					configProvider:  mockConfigProvider,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					PromptID:          ptr.Of(int64(1)),
					WithDraft:         ptr.Of(true),
					WithDefaultConfig: ptr.Of(true),
				},
			},
			want: &manage.GetPromptResponse{
				Prompt: &prompt.Prompt{
					ID:          ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					PromptKey:   ptr.Of("test_key"),
					PromptBasic: &prompt.PromptBasic{
						DisplayName:       ptr.Of("test_name"),
						Description:       ptr.Of("test_description"),
						LatestVersion:     ptr.Of("1.0.0"),
						CreatedBy:         ptr.Of("test_creator"),
						UpdatedBy:         ptr.Of("test_updater"),
						CreatedAt:         ptr.Of(now.UnixMilli()),
						UpdatedAt:         ptr.Of(now.UnixMilli()),
						LatestCommittedAt: nil,
						PromptType:        ptr.Of(prompt.PromptTypeNormal),
					},
					PromptDraft: &prompt.PromptDraft{
						Detail: &prompt.PromptDetail{
							PromptTemplate: &prompt.PromptTemplate{
								TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								HasSnippet:   ptr.Of(false),
								Messages: []*prompt.Message{
									{
										Role:    ptr.Of(prompt.RoleUser),
										Content: ptr.Of("test content"),
									},
								},
							},
						},
						DraftInfo: &prompt.DraftInfo{
							UserID:      ptr.Of("123"),
							BaseVersion: ptr.Of("1.0.0"),
							IsModified:  ptr.Of(true),
							CreatedAt:   ptr.Of(now.UnixMilli()),
							UpdatedAt:   ptr.Of(now.UnixMilli()),
						},
					},
				},
				DefaultConfig: &prompt.PromptDetail{
					PromptTemplate: &prompt.PromptTemplate{
						TemplateType: ptr.Of(prompt.TemplateTypeNormal),
						HasSnippet:   ptr.Of(false),
						Messages: []*prompt.Message{
							{
								Role:    ptr.Of(prompt.RoleSystem),
								Content: ptr.Of("Default system message"),
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "get prompt with default config false",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:  1,
					WithDraft: true,
					UserID:    "123",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						DisplayName:       "test_name",
						Description:       "test_description",
						LatestVersion:     "1.0.0",
						CreatedBy:         "test_creator",
						UpdatedBy:         "test_updater",
						CreatedAt:         now,
						UpdatedAt:         now,
						LatestCommittedAt: nil,
					},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("test content"),
									},
								},
							},
						},
						DraftInfo: &entity.DraftInfo{
							UserID:      "123",
							BaseVersion: "1.0.0",
							IsModified:  true,
							CreatedAt:   now,
							UpdatedAt:   now,
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					PromptID:          ptr.Of(int64(1)),
					WithDraft:         ptr.Of(true),
					WithDefaultConfig: ptr.Of(false),
				},
			},
			want: &manage.GetPromptResponse{
				Prompt: &prompt.Prompt{
					ID:          ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					PromptKey:   ptr.Of("test_key"),
					PromptBasic: &prompt.PromptBasic{
						DisplayName:       ptr.Of("test_name"),
						Description:       ptr.Of("test_description"),
						LatestVersion:     ptr.Of("1.0.0"),
						CreatedBy:         ptr.Of("test_creator"),
						UpdatedBy:         ptr.Of("test_updater"),
						CreatedAt:         ptr.Of(now.UnixMilli()),
						UpdatedAt:         ptr.Of(now.UnixMilli()),
						LatestCommittedAt: nil,
						PromptType:        ptr.Of(prompt.PromptTypeNormal),
					},
					PromptDraft: &prompt.PromptDraft{
						Detail: &prompt.PromptDetail{
							PromptTemplate: &prompt.PromptTemplate{
								TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								HasSnippet:   ptr.Of(false),
								Messages: []*prompt.Message{
									{
										Role:    ptr.Of(prompt.RoleUser),
										Content: ptr.Of("test content"),
									},
								},
							},
						},
						DraftInfo: &prompt.DraftInfo{
							UserID:      ptr.Of("123"),
							BaseVersion: ptr.Of("1.0.0"),
							IsModified:  ptr.Of(true),
							CreatedAt:   ptr.Of(now.UnixMilli()),
							UpdatedAt:   ptr.Of(now.UnixMilli()),
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "config provider error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:  1,
					WithDraft: true,
					UserID:    "123",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						DisplayName:       "test_name",
						Description:       "test_description",
						LatestVersion:     "1.0.0",
						CreatedBy:         "test_creator",
						UpdatedBy:         "test_updater",
						CreatedAt:         now,
						UpdatedAt:         now,
						LatestCommittedAt: nil,
					},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("test content"),
									},
								},
							},
						},
						DraftInfo: &entity.DraftInfo{
							UserID:      "123",
							BaseVersion: "1.0.0",
							IsModified:  true,
							CreatedAt:   now,
							UpdatedAt:   now,
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockConfigProvider := confmocks.NewMockIConfigProvider(ctrl)
				mockConfigProvider.EXPECT().GetPromptDefaultConfig(gomock.Any()).Return(nil, errorx.New("config provider error"))

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
					configProvider:  mockConfigProvider,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					PromptID:          ptr.Of(int64(1)),
					WithDraft:         ptr.Of(true),
					WithDefaultConfig: ptr.Of(true),
				},
			},
			want:    manage.NewGetPromptResponse(),
			wantErr: errorx.New("config provider error"),
		},
		{
			name: "expand snippets success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
					WithDraft:     false,
					UserID:        "123",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						PromptType:    entity.PromptTypeNormal,
						DisplayName:   "test_name",
						Description:   "test_description",
						LatestVersion: "1.0.0",
						CreatedBy:     "test_creator",
						UpdatedBy:     "test_updater",
						CreatedAt:     now,
						UpdatedAt:     now,
					},
					PromptCommit: &entity.PromptCommit{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								HasSnippets:  true,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("snippet content"),
									},
								},
							},
						},
						CommitInfo: &entity.CommitInfo{
							Version:     "1.0.0",
							BaseVersion: "0.9.0",
							Description: "test commit",
							CommittedBy: "test_user",
							CommittedAt: now,
						},
					},
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(100), []int64{int64(1)}, consts.ActionLoopPromptRead).Return(nil)

				return fields{
					manageRepo:      mockRepo,
					promptService:   mockPromptService,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					WorkspaceID:   ptr.Of(int64(100)),
					PromptID:      ptr.Of(int64(1)),
					WithCommit:    ptr.Of(true),
					CommitVersion: ptr.Of("1.0.0"),
					ExpandSnippet: ptr.Of(true),
				},
			},
			want: &manage.GetPromptResponse{
				Prompt: &prompt.Prompt{
					ID:          ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					PromptKey:   ptr.Of("test_key"),
					PromptBasic: &prompt.PromptBasic{
						PromptType:    ptr.Of(prompt.PromptTypeNormal),
						DisplayName:   ptr.Of("test_name"),
						Description:   ptr.Of("test_description"),
						LatestVersion: ptr.Of("1.0.0"),
						CreatedBy:     ptr.Of("test_creator"),
						UpdatedBy:     ptr.Of("test_updater"),
						CreatedAt:     ptr.Of(now.UnixMilli()),
						UpdatedAt:     ptr.Of(now.UnixMilli()),
					},
					PromptCommit: &prompt.PromptCommit{
						Detail: &prompt.PromptDetail{
							PromptTemplate: &prompt.PromptTemplate{
								TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								HasSnippet:   ptr.Of(true),
								Messages: []*prompt.Message{
									{
										Role:    ptr.Of(prompt.RoleUser),
										Content: ptr.Of("snippet content"),
									},
								},
							},
						},
						CommitInfo: &prompt.CommitInfo{
							Version:     ptr.Of("1.0.0"),
							BaseVersion: ptr.Of("0.9.0"),
							Description: ptr.Of("test commit"),
							CommittedBy: ptr.Of("test_user"),
							CommittedAt: ptr.Of(now.UnixMilli()),
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "expand snippets error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
					WithDraft:     false,
					UserID:        "123",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().ExpandSnippets(gomock.Any(), gomock.Any()).Return(errorx.New("expand snippets error"))

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					manageRepo:      mockRepo,
					promptService:   mockPromptService,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					WorkspaceID:   ptr.Of(int64(100)),
					PromptID:      ptr.Of(int64(1)),
					WithCommit:    ptr.Of(true),
					CommitVersion: ptr.Of("1.0.0"),
					ExpandSnippet: ptr.Of(true),
				},
			},
			want:    manage.NewGetPromptResponse(),
			wantErr: errorx.New("expand snippets error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			d := &PromptManageApplicationImpl{
				manageRepo:      ttFields.manageRepo,
				promptService:   ttFields.promptService,
				authRPCProvider: ttFields.authRPCProvider,
				userRPCProvider: ttFields.userRPCProvider,
				configProvider:  ttFields.configProvider,
			}

			got, err := d.GetPrompt(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPromptManageApplicationImpl_DeletePrompt(t *testing.T) {
	type fields struct {
		manageRepo      repo.IManageRepo
		promptService   service.IPromptService
		authRPCProvider rpc.IAuthProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.DeletePromptRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.DeletePromptResponse
		wantErr      error
	}{
		{
			name: "snippet prompt can not be deleted",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID: 1,
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "snippet_key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeSnippet,
					},
				}, nil)

				return fields{
					manageRepo: mockRepo,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.DeletePromptRequest{
					PromptID: ptr.Of(int64(1)),
				},
			},
			want:    manage.NewDeletePromptResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Snippet prompt can not be deleted")),
		},
		{
			name: "delete prompt success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID: 1,
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "normal_key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				}, nil)
				mockRepo.EXPECT().DeletePrompt(gomock.Any(), int64(1)).Return(nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(100), []int64{int64(1)}, consts.ActionLoopPromptEdit).Return(nil)

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.DeletePromptRequest{
					PromptID: ptr.Of(int64(1)),
				},
			},
			want:    manage.NewDeletePromptResponse(),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		caseData := tt
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ff := caseData.fieldsGetter(ctrl)
			app := &PromptManageApplicationImpl{
				manageRepo:      ff.manageRepo,
				promptService:   ff.promptService,
				authRPCProvider: ff.authRPCProvider,
			}

			got, err := app.DeletePrompt(caseData.args.ctx, caseData.args.request)
			unittest.AssertErrorEqual(t, caseData.wantErr, err)
			if err == nil {
				assert.Equal(t, caseData.want, got)
			}
		})
	}
}

func TestPromptManageApplicationImpl_RevertDraftFromCommit(t *testing.T) {
	type fields struct {
		manageRepo       repo.IManageRepo
		promptService    service.IPromptService
		authRPCProvider  rpc.IAuthProvider
		userRPCProvider  rpc.IUserProvider
		auditRPCProvider rpc.IAuditProvider
		configProvider   conf.IConfigProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.RevertDraftFromCommitRequest
	}
	now := time.Now()
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.RevertDraftFromCommitResponse
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.RevertDraftFromCommitRequest{
					PromptID:                   ptr.Of(int64(1)),
					CommitVersionRevertingFrom: ptr.Of("1.0.0"),
				},
			},
			want:    manage.NewRevertDraftFromCommitResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "get prompt error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				}).Return(nil, errorx.New("get prompt error"))

				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.RevertDraftFromCommitRequest{
					PromptID:                   ptr.Of(int64(1)),
					CommitVersionRevertingFrom: ptr.Of("1.0.0"),
				},
			},
			want:    manage.NewRevertDraftFromCommitResponse(),
			wantErr: errorx.New("get prompt error"),
		},
		{
			name: "prompt or commit not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						DisplayName:       "test_name",
						Description:       "test_description",
						LatestVersion:     "1.0.0",
						CreatedBy:         "test_creator",
						UpdatedBy:         "test_updater",
						CreatedAt:         now,
						UpdatedAt:         now,
						LatestCommittedAt: nil,
					},
				}, nil)

				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.RevertDraftFromCommitRequest{
					PromptID:                   ptr.Of(int64(1)),
					CommitVersionRevertingFrom: ptr.Of("1.0.0"),
				},
			},
			want:    manage.NewRevertDraftFromCommitResponse(),
			wantErr: errorx.New("Prompt or commit not found, prompt id = 1, commit version = 1.0.0"),
		},
		{
			name: "save draft error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						DisplayName:       "test_name",
						Description:       "test_description",
						LatestVersion:     "1.0.0",
						CreatedBy:         "test_creator",
						UpdatedBy:         "test_updater",
						CreatedAt:         now,
						UpdatedAt:         now,
						LatestCommittedAt: nil,
					},
					PromptCommit: &entity.PromptCommit{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("test content"),
									},
								},
							},
						},
						CommitInfo: &entity.CommitInfo{
							Version:     "1.0.0",
							BaseVersion: "0.9.0",
							Description: "test commit",
							CommittedBy: "test_user",
							CommittedAt: now,
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).Return(nil, errorx.New("save draft error"))

				return fields{
					manageRepo:      mockManageRepo,
					authRPCProvider: mockAuth,
					promptService:   mockPromptService,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.RevertDraftFromCommitRequest{
					PromptID:                   ptr.Of(int64(1)),
					CommitVersionRevertingFrom: ptr.Of("1.0.0"),
				},
			},
			want:    manage.NewRevertDraftFromCommitResponse(),
			wantErr: errorx.New("save draft error"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
				}).Return(&entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						DisplayName:       "test_name",
						Description:       "test_description",
						LatestVersion:     "1.0.0",
						CreatedBy:         "test_creator",
						UpdatedBy:         "test_updater",
						CreatedAt:         now,
						UpdatedAt:         now,
						LatestCommittedAt: nil,
					},
					PromptCommit: &entity.PromptCommit{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("test content"),
									},
								},
							},
						},
						CommitInfo: &entity.CommitInfo{
							Version:     "1.0.0",
							BaseVersion: "0.9.0",
							Description: "test commit",
							CommittedBy: "test_user",
							CommittedAt: now,
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, promptDO *entity.Prompt) (*entity.DraftInfo, error) {
					assert.Equal(t, int64(1), promptDO.ID)
					assert.Equal(t, "123", promptDO.PromptDraft.DraftInfo.UserID)
					assert.Equal(t, "1.0.0", promptDO.PromptDraft.DraftInfo.BaseVersion)
					assert.Equal(t, entity.TemplateTypeNormal, promptDO.PromptDraft.PromptDetail.PromptTemplate.TemplateType)
					assert.Equal(t, 1, len(promptDO.PromptDraft.PromptDetail.PromptTemplate.Messages))
					assert.Equal(t, entity.RoleUser, promptDO.PromptDraft.PromptDetail.PromptTemplate.Messages[0].Role)
					assert.Equal(t, "test content", *promptDO.PromptDraft.PromptDetail.PromptTemplate.Messages[0].Content)
					return &entity.DraftInfo{}, nil
				})

				return fields{
					manageRepo:      mockManageRepo,
					authRPCProvider: mockAuth,
					promptService:   mockPromptService,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.RevertDraftFromCommitRequest{
					PromptID:                   ptr.Of(int64(1)),
					CommitVersionRevertingFrom: ptr.Of("1.0.0"),
				},
			},
			want:    manage.NewRevertDraftFromCommitResponse(),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			app := &PromptManageApplicationImpl{
				manageRepo:       ttFields.manageRepo,
				promptService:    ttFields.promptService,
				authRPCProvider:  ttFields.authRPCProvider,
				userRPCProvider:  ttFields.userRPCProvider,
				auditRPCProvider: ttFields.auditRPCProvider,
				configProvider:   ttFields.configProvider,
			}

			got, err := app.RevertDraftFromCommit(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPromptManageApplicationImpl_ListCommit(t *testing.T) {
	type fields struct {
		manageRepo       repo.IManageRepo
		promptService    service.IPromptService
		authRPCProvider  rpc.IAuthProvider
		userRPCProvider  rpc.IUserProvider
		auditRPCProvider rpc.IAuditProvider
		configProvider   conf.IConfigProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.ListCommitRequest
	}
	now := time.Now()
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.ListCommitResponse
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.ListCommitRequest{
					PromptID:  ptr.Of(int64(1)),
					PageSize:  ptr.Of(int32(10)),
					PageToken: nil,
					Asc:       ptr.Of(false),
				},
			},
			want:    manage.NewListCommitResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "invalid page token",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{ID: 1}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					manageRepo:      mockManageRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListCommitRequest{
					PromptID:  ptr.Of(int64(1)),
					PageSize:  ptr.Of(int32(10)),
					PageToken: ptr.Of("invalid"),
					Asc:       ptr.Of(false),
				},
			},
			want:    manage.NewListCommitResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Page token is invalid, page token = invalid")),
		},
		{
			name: "list commit error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{ID: 1}, nil)
				mockManageRepo.EXPECT().ListCommitInfo(gomock.Any(), repo.ListCommitInfoParam{
					PromptID:  1,
					PageSize:  10,
					PageToken: nil,
					Asc:       false,
				}).Return(nil, errorx.New("list commit error"))

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					manageRepo:      mockManageRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListCommitRequest{
					PromptID:  ptr.Of(int64(1)),
					PageSize:  ptr.Of(int32(10)),
					PageToken: nil,
					Asc:       ptr.Of(false),
				},
			},
			want:    manage.NewListCommitResponse(),
			wantErr: errorx.New("list commit error"),
		},
		{
			name: "empty result",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{ID: 1}, nil)
				mockManageRepo.EXPECT().ListCommitInfo(gomock.Any(), repo.ListCommitInfoParam{
					PromptID:  1,
					PageSize:  10,
					PageToken: nil,
					Asc:       false,
				}).Return(nil, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					manageRepo:      mockManageRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListCommitRequest{
					PromptID:  ptr.Of(int64(1)),
					PageSize:  ptr.Of(int32(10)),
					PageToken: nil,
					Asc:       ptr.Of(false),
				},
			},
			want:    manage.NewListCommitResponse(),
			wantErr: nil,
		},
		{
			name: "single page result",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{ID: 1}, nil)
				mockManageRepo.EXPECT().ListCommitInfo(gomock.Any(), repo.ListCommitInfoParam{
					PromptID:  1,
					PageSize:  10,
					PageToken: nil,
					Asc:       false,
				}).Return(&repo.ListCommitResult{
					CommitInfoDOs: []*entity.CommitInfo{
						{
							Version:     "1.0.0",
							BaseVersion: "0.9.0",
							Description: "test commit 1",
							CommittedBy: "test_user",
							CommittedAt: now,
						},
						{
							Version:     "1.1.0",
							BaseVersion: "1.0.0",
							Description: "test commit 2",
							CommittedBy: "test_user",
							CommittedAt: now,
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockUser := mocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), []string{"test_user"}).Return([]*rpc.UserInfo{
					{
						UserID:   "test_user",
						UserName: "Test User",
					},
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().BatchGetCommitLabels(gomock.Any(), int64(1), []string{"1.0.0", "1.1.0"}).Return(map[string][]string{}, nil)

				return fields{
					manageRepo:      mockManageRepo,
					promptService:   mockPromptService,
					authRPCProvider: mockAuth,
					userRPCProvider: mockUser,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListCommitRequest{
					PromptID:  ptr.Of(int64(1)),
					PageSize:  ptr.Of(int32(10)),
					PageToken: nil,
					Asc:       ptr.Of(false),
				},
			},
			want: &manage.ListCommitResponse{
				PromptCommitInfos: []*prompt.CommitInfo{
					{
						Version:     ptr.Of("1.0.0"),
						BaseVersion: ptr.Of("0.9.0"),
						Description: ptr.Of("test commit 1"),
						CommittedBy: ptr.Of("test_user"),
						CommittedAt: ptr.Of(now.UnixMilli()),
					},
					{
						Version:     ptr.Of("1.1.0"),
						BaseVersion: ptr.Of("1.0.0"),
						Description: ptr.Of("test commit 2"),
						CommittedBy: ptr.Of("test_user"),
						CommittedAt: ptr.Of(now.UnixMilli()),
					},
				},
				CommitVersionLabelMapping: map[string][]*prompt.Label{},
				Users: []*user.UserInfoDetail{
					{
						UserID:    ptr.Of("test_user"),
						Name:      ptr.Of("Test User"),
						NickName:  ptr.Of(""),
						AvatarURL: ptr.Of(""),
						Email:     ptr.Of(""),
						Mobile:    ptr.Of(""),
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "multiple pages result",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{ID: 1}, nil)
				mockManageRepo.EXPECT().ListCommitInfo(gomock.Any(), repo.ListCommitInfoParam{
					PromptID:  1,
					PageSize:  2,
					PageToken: nil,
					Asc:       false,
				}).Return(&repo.ListCommitResult{
					CommitInfoDOs: []*entity.CommitInfo{
						{
							Version:     "1.0.0",
							BaseVersion: "0.9.0",
							Description: "test commit 1",
							CommittedBy: "test_user",
							CommittedAt: now,
						},
						{
							Version:     "1.1.0",
							BaseVersion: "1.0.0",
							Description: "test commit 2",
							CommittedBy: "test_user",
							CommittedAt: now,
						},
					},
					NextPageToken: 3,
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockUser := mocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), []string{"test_user"}).Return([]*rpc.UserInfo{
					{
						UserID:   "test_user",
						UserName: "Test User",
					},
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().BatchGetCommitLabels(gomock.Any(), int64(1), []string{"1.0.0", "1.1.0"}).Return(map[string][]string{}, nil)

				return fields{
					manageRepo:      mockManageRepo,
					promptService:   mockPromptService,
					authRPCProvider: mockAuth,
					userRPCProvider: mockUser,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListCommitRequest{
					PromptID:  ptr.Of(int64(1)),
					PageSize:  ptr.Of(int32(2)),
					PageToken: nil,
					Asc:       ptr.Of(false),
				},
			},
			want: &manage.ListCommitResponse{
				PromptCommitInfos: []*prompt.CommitInfo{
					{
						Version:     ptr.Of("1.0.0"),
						BaseVersion: ptr.Of("0.9.0"),
						Description: ptr.Of("test commit 1"),
						CommittedBy: ptr.Of("test_user"),
						CommittedAt: ptr.Of(now.UnixMilli()),
					},
					{
						Version:     ptr.Of("1.1.0"),
						BaseVersion: ptr.Of("1.0.0"),
						Description: ptr.Of("test commit 2"),
						CommittedBy: ptr.Of("test_user"),
						CommittedAt: ptr.Of(now.UnixMilli()),
					},
				},
				CommitVersionLabelMapping: map[string][]*prompt.Label{},
				HasMore:                   ptr.Of(true),
				NextPageToken:             ptr.Of("3"),
				Users: []*user.UserInfoDetail{
					{
						UserID:    ptr.Of("test_user"),
						Name:      ptr.Of("Test User"),
						NickName:  ptr.Of(""),
						AvatarURL: ptr.Of(""),
						Email:     ptr.Of(""),
						Mobile:    ptr.Of(""),
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "with page token and asc",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Return(&entity.Prompt{ID: 1}, nil)
				mockManageRepo.EXPECT().ListCommitInfo(gomock.Any(), repo.ListCommitInfoParam{
					PromptID:  1,
					PageSize:  10,
					PageToken: ptr.Of(int64(2)),
					Asc:       true,
				}).Return(&repo.ListCommitResult{
					CommitInfoDOs: []*entity.CommitInfo{
						{
							Version:     "1.2.0",
							BaseVersion: "1.1.0",
							Description: "test commit 3",
							CommittedBy: "test_user",
							CommittedAt: now,
						},
						{
							Version:     "1.3.0",
							BaseVersion: "1.2.0",
							Description: "test commit 4",
							CommittedBy: "test_user",
							CommittedAt: now,
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				mockUser := mocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), []string{"test_user"}).Return([]*rpc.UserInfo{
					{
						UserID:   "test_user",
						UserName: "Test User",
					},
				}, nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().BatchGetCommitLabels(gomock.Any(), int64(1), []string{"1.2.0", "1.3.0"}).Return(map[string][]string{}, nil)

				return fields{
					manageRepo:      mockManageRepo,
					promptService:   mockPromptService,
					authRPCProvider: mockAuth,
					userRPCProvider: mockUser,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListCommitRequest{
					PromptID:  ptr.Of(int64(1)),
					PageSize:  ptr.Of(int32(10)),
					PageToken: ptr.Of("2"),
					Asc:       ptr.Of(true),
				},
			},
			want: &manage.ListCommitResponse{
				PromptCommitInfos: []*prompt.CommitInfo{
					{
						Version:     ptr.Of("1.2.0"),
						BaseVersion: ptr.Of("1.1.0"),
						Description: ptr.Of("test commit 3"),
						CommittedBy: ptr.Of("test_user"),
						CommittedAt: ptr.Of(now.UnixMilli()),
					},
					{
						Version:     ptr.Of("1.3.0"),
						BaseVersion: ptr.Of("1.2.0"),
						Description: ptr.Of("test commit 4"),
						CommittedBy: ptr.Of("test_user"),
						CommittedAt: ptr.Of(now.UnixMilli()),
					},
				},
				CommitVersionLabelMapping: map[string][]*prompt.Label{},
				Users: []*user.UserInfoDetail{
					{
						UserID:    ptr.Of("test_user"),
						Name:      ptr.Of("Test User"),
						NickName:  ptr.Of(""),
						AvatarURL: ptr.Of(""),
						Email:     ptr.Of(""),
						Mobile:    ptr.Of(""),
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			app := &PromptManageApplicationImpl{
				manageRepo:       ttFields.manageRepo,
				promptService:    ttFields.promptService,
				authRPCProvider:  ttFields.authRPCProvider,
				userRPCProvider:  ttFields.userRPCProvider,
				auditRPCProvider: ttFields.auditRPCProvider,
				configProvider:   ttFields.configProvider,
			}

			got, err := app.ListCommit(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPromptManageApplicationImpl_CommitDraft(t *testing.T) {
	type fields struct {
		manageRepo       repo.IManageRepo
		promptService    service.IPromptService
		authRPCProvider  rpc.IAuthProvider
		userRPCProvider  rpc.IUserProvider
		auditRPCProvider rpc.IAuditProvider
		configProvider   conf.IConfigProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.CommitDraftRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.CommitDraftResponse
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.CommitDraftRequest{
					PromptID:      ptr.Of(int64(1)),
					CommitVersion: ptr.Of("1.0.0"),
				},
			},
			want:    manage.NewCommitDraftResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "invalid version format",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.CommitDraftRequest{
					PromptID:      ptr.Of(int64(1)),
					CommitVersion: ptr.Of("invalid-version"),
				},
			},
			want:    manage.NewCommitDraftResponse(),
			wantErr: errorx.New("Invalid Semantic Version"),
		},
		{
			name: "get prompt error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID: 1,
				}).Return(nil, errorx.New("get prompt error"))

				return fields{
					manageRepo: mockRepo,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.CommitDraftRequest{
					PromptID:      ptr.Of(int64(1)),
					CommitVersion: ptr.Of("1.0.0"),
				},
			},
			want:    manage.NewCommitDraftResponse(),
			wantErr: errorx.New("get prompt error"),
		},
		{
			name: "permission check error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID: 1,
				}).Return(&entity.Prompt{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(100), []int64{1}, consts.ActionLoopPromptEdit).Return(errorx.New("permission denied"))

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.CommitDraftRequest{
					PromptID:      ptr.Of(int64(1)),
					CommitVersion: ptr.Of("1.0.0"),
				},
			},
			want:    manage.NewCommitDraftResponse(),
			wantErr: errorx.New("permission denied"),
		},
		{
			name: "commit draft error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID: 1,
				}).Return(&entity.Prompt{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(100), []int64{1}, consts.ActionLoopPromptEdit).Return(nil)

				mockRepo.EXPECT().CommitDraft(gomock.Any(), repo.CommitDraftParam{
					PromptID:          1,
					UserID:            "123",
					CommitVersion:     "1.0.0",
					CommitDescription: "test commit",
				}).Return(errorx.New("commit draft error"))

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.CommitDraftRequest{
					PromptID:          ptr.Of(int64(1)),
					CommitVersion:     ptr.Of("1.0.0"),
					CommitDescription: ptr.Of("test commit"),
				},
			},
			want:    manage.NewCommitDraftResponse(),
			wantErr: errorx.New("commit draft error"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{
					PromptID: 1,
				}).Return(&entity.Prompt{
					ID:      1,
					SpaceID: 100,
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(100), []int64{1}, consts.ActionLoopPromptEdit).Return(nil)

				mockRepo.EXPECT().CommitDraft(gomock.Any(), repo.CommitDraftParam{
					PromptID:          1,
					UserID:            "123",
					CommitVersion:     "1.0.0",
					CommitDescription: "test commit",
				}).Return(nil)

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.CommitDraftRequest{
					PromptID:          ptr.Of(int64(1)),
					CommitVersion:     ptr.Of("1.0.0"),
					CommitDescription: ptr.Of("test commit"),
				},
			},
			want:    manage.NewCommitDraftResponse(),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			app := &PromptManageApplicationImpl{
				manageRepo:       ttFields.manageRepo,
				promptService:    ttFields.promptService,
				authRPCProvider:  ttFields.authRPCProvider,
				userRPCProvider:  ttFields.userRPCProvider,
				auditRPCProvider: ttFields.auditRPCProvider,
				configProvider:   ttFields.configProvider,
			}

			got, err := app.CommitDraft(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPromptManageApplicationImpl_ListPrompt(t *testing.T) {
	type fields struct {
		manageRepo       repo.IManageRepo
		promptService    service.IPromptService
		authRPCProvider  rpc.IAuthProvider
		userRPCProvider  rpc.IUserProvider
		auditRPCProvider rpc.IAuditProvider
		configProvider   conf.IConfigProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.ListPromptRequest
	}
	now := time.Now()
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.ListPromptResponse
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.ListPromptRequest{
					WorkspaceID: ptr.Of(int64(100)),
					PageNum:     ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			want:    manage.NewListPromptResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "permission check error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(errorx.New("permission denied"))

				return fields{
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListPromptRequest{
					WorkspaceID: ptr.Of(int64(100)),
					PageNum:     ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			want:    manage.NewListPromptResponse(),
			wantErr: errorx.New("permission denied"),
		},
		{
			name: "list prompt with committed only true",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().ListPrompt(gomock.Any(), repo.ListPromptParam{
					SpaceID:           100,
					UserID:            "123",
					CommittedOnly:     true,
					FilterPromptTypes: []entity.PromptType{prompt.PromptTypeNormal},
					PageNum:           1,
					PageSize:          10,
					OrderBy:           mysql.ListPromptBasicOrderByID,
					Asc:               false,
				}).Return(&repo.ListPromptResult{
					Total: 1,
					PromptDOs: []*entity.Prompt{
						{
							ID:        1,
							SpaceID:   100,
							PromptKey: "test_key",
							PromptBasic: &entity.PromptBasic{
								DisplayName:       "test_name",
								Description:       "test_description",
								LatestVersion:     "1.0.0",
								CreatedBy:         "test_creator",
								UpdatedBy:         "test_updater",
								CreatedAt:         now,
								UpdatedAt:         now,
								LatestCommittedAt: &now,
								PromptType:        entity.PromptTypeNormal,
							},
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(nil)

				mockUser := mocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), []string{"test_creator"}).Return([]*rpc.UserInfo{
					{
						UserID:   "test_creator",
						UserName: "Test Creator",
					},
				}, nil)

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
					userRPCProvider: mockUser,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListPromptRequest{
					WorkspaceID:   ptr.Of(int64(100)),
					CommittedOnly: ptr.Of(true),
					PageNum:       ptr.Of(int32(1)),
					PageSize:      ptr.Of(int32(10)),
				},
			},
			want: &manage.ListPromptResponse{
				Total: ptr.Of(int32(1)),
				Prompts: []*prompt.Prompt{
					{
						ID:          ptr.Of(int64(1)),
						WorkspaceID: ptr.Of(int64(100)),
						PromptKey:   ptr.Of("test_key"),
						PromptBasic: &prompt.PromptBasic{
							DisplayName:       ptr.Of("test_name"),
							Description:       ptr.Of("test_description"),
							LatestVersion:     ptr.Of("1.0.0"),
							CreatedBy:         ptr.Of("test_creator"),
							UpdatedBy:         ptr.Of("test_updater"),
							CreatedAt:         ptr.Of(now.UnixMilli()),
							UpdatedAt:         ptr.Of(now.UnixMilli()),
							LatestCommittedAt: ptr.Of(now.UnixMilli()),
							PromptType:        ptr.Of(prompt.PromptTypeNormal),
						},
					},
				},
				Users: []*user.UserInfoDetail{
					{
						UserID:    ptr.Of("test_creator"),
						Name:      ptr.Of("Test Creator"),
						NickName:  ptr.Of(""),
						AvatarURL: ptr.Of(""),
						Email:     ptr.Of(""),
						Mobile:    ptr.Of(""),
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "list prompt with committed only false",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().ListPrompt(gomock.Any(), repo.ListPromptParam{
					SpaceID:           100,
					UserID:            "123",
					CommittedOnly:     false,
					FilterPromptTypes: []entity.PromptType{entity.PromptTypeNormal},
					PageNum:           1,
					PageSize:          10,
					OrderBy:           mysql.ListPromptBasicOrderByID,
					Asc:               false,
				}).Return(&repo.ListPromptResult{
					Total: 2,
					PromptDOs: []*entity.Prompt{
						{
							ID:        1,
							SpaceID:   100,
							PromptKey: "test_key_1",
							PromptBasic: &entity.PromptBasic{
								DisplayName:       "test_name_1",
								Description:       "test_description_1",
								LatestVersion:     "1.0.0",
								CreatedBy:         "test_creator",
								UpdatedBy:         "test_updater",
								CreatedAt:         now,
								UpdatedAt:         now,
								LatestCommittedAt: &now,
								PromptType:        entity.PromptTypeNormal,
							},
						},
						{
							ID:        2,
							SpaceID:   100,
							PromptKey: "test_key_2",
							PromptBasic: &entity.PromptBasic{
								DisplayName:       "test_name_2",
								Description:       "test_description_2",
								LatestVersion:     "",
								CreatedBy:         "test_creator",
								UpdatedBy:         "test_updater",
								CreatedAt:         now,
								UpdatedAt:         now,
								LatestCommittedAt: nil,
							},
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(nil)

				mockUser := mocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), []string{"test_creator"}).Return([]*rpc.UserInfo{
					{
						UserID:   "test_creator",
						UserName: "Test Creator",
					},
				}, nil)

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
					userRPCProvider: mockUser,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListPromptRequest{
					WorkspaceID:   ptr.Of(int64(100)),
					CommittedOnly: ptr.Of(false),
					PageNum:       ptr.Of(int32(1)),
					PageSize:      ptr.Of(int32(10)),
				},
			},
			want: &manage.ListPromptResponse{
				Total: ptr.Of(int32(2)),
				Prompts: []*prompt.Prompt{
					{
						ID:          ptr.Of(int64(1)),
						WorkspaceID: ptr.Of(int64(100)),
						PromptKey:   ptr.Of("test_key_1"),
						PromptBasic: &prompt.PromptBasic{
							DisplayName:       ptr.Of("test_name_1"),
							Description:       ptr.Of("test_description_1"),
							LatestVersion:     ptr.Of("1.0.0"),
							CreatedBy:         ptr.Of("test_creator"),
							UpdatedBy:         ptr.Of("test_updater"),
							CreatedAt:         ptr.Of(now.UnixMilli()),
							UpdatedAt:         ptr.Of(now.UnixMilli()),
							LatestCommittedAt: ptr.Of(now.UnixMilli()),
							PromptType:        ptr.Of(prompt.PromptTypeNormal),
						},
					},
					{
						ID:          ptr.Of(int64(2)),
						WorkspaceID: ptr.Of(int64(100)),
						PromptKey:   ptr.Of("test_key_2"),
						PromptBasic: &prompt.PromptBasic{
							DisplayName:   ptr.Of("test_name_2"),
							Description:   ptr.Of("test_description_2"),
							LatestVersion: ptr.Of(""),
							CreatedBy:     ptr.Of("test_creator"),
							UpdatedBy:     ptr.Of("test_updater"),
							CreatedAt:     ptr.Of(now.UnixMilli()),
							UpdatedAt:     ptr.Of(now.UnixMilli()),
							PromptType:    ptr.Of(prompt.PromptTypeNormal),
						},
					},
				},
				Users: []*user.UserInfoDetail{
					{
						UserID:    ptr.Of("test_creator"),
						Name:      ptr.Of("Test Creator"),
						NickName:  ptr.Of(""),
						AvatarURL: ptr.Of(""),
						Email:     ptr.Of(""),
						Mobile:    ptr.Of(""),
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "list prompt with user draft association",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().ListPrompt(gomock.Any(), repo.ListPromptParam{
					SpaceID:           100,
					UserID:            "123",
					KeyWord:           "draft",
					FilterPromptTypes: []entity.PromptType{entity.PromptTypeNormal},
					PageNum:           1,
					PageSize:          10,
					OrderBy:           mysql.ListPromptBasicOrderByID,
					Asc:               false,
				}).Return(&repo.ListPromptResult{
					Total: 1,
					PromptDOs: []*entity.Prompt{
						{
							ID:        1,
							SpaceID:   100,
							PromptKey: "test_key",
							PromptBasic: &entity.PromptBasic{
								DisplayName:       "test_name",
								Description:       "test_description",
								LatestVersion:     "1.0.0",
								CreatedBy:         "test_creator",
								UpdatedBy:         "test_updater",
								CreatedAt:         now,
								UpdatedAt:         now,
								LatestCommittedAt: &now,
								PromptType:        entity.PromptTypeNormal,
							},
							PromptDraft: &entity.PromptDraft{
								PromptDetail: &entity.PromptDetail{
									PromptTemplate: &entity.PromptTemplate{
										TemplateType: entity.TemplateTypeNormal,
										Messages: []*entity.Message{
											{
												Role:    entity.RoleUser,
												Content: ptr.Of("draft content"),
											},
										},
										HasSnippets: false,
									},
								},
								DraftInfo: &entity.DraftInfo{
									UserID:      "123",
									BaseVersion: "1.0.0",
									IsModified:  true,
									CreatedAt:   now,
									UpdatedAt:   now,
								},
							},
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(nil)

				mockUser := mocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), []string{"test_creator"}).Return([]*rpc.UserInfo{
					{
						UserID:   "test_creator",
						UserName: "Test Creator",
					},
				}, nil)

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
					userRPCProvider: mockUser,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListPromptRequest{
					WorkspaceID: ptr.Of(int64(100)),
					KeyWord:     ptr.Of("draft"),
					PageNum:     ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			want: &manage.ListPromptResponse{
				Total: ptr.Of(int32(1)),
				Prompts: []*prompt.Prompt{
					{
						ID:          ptr.Of(int64(1)),
						WorkspaceID: ptr.Of(int64(100)),
						PromptKey:   ptr.Of("test_key"),
						PromptBasic: &prompt.PromptBasic{
							DisplayName:       ptr.Of("test_name"),
							Description:       ptr.Of("test_description"),
							LatestVersion:     ptr.Of("1.0.0"),
							CreatedBy:         ptr.Of("test_creator"),
							UpdatedBy:         ptr.Of("test_updater"),
							CreatedAt:         ptr.Of(now.UnixMilli()),
							UpdatedAt:         ptr.Of(now.UnixMilli()),
							LatestCommittedAt: ptr.Of(now.UnixMilli()),
							PromptType:        ptr.Of(prompt.PromptTypeNormal),
						},
						PromptDraft: &prompt.PromptDraft{
							Detail: &prompt.PromptDetail{
								PromptTemplate: &prompt.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									Messages: []*prompt.Message{
										{
											Role:    ptr.Of(prompt.RoleUser),
											Content: ptr.Of("draft content"),
										},
									},
									HasSnippet: ptr.Of(false),
								},
							},
							DraftInfo: &prompt.DraftInfo{
								UserID:      ptr.Of("123"),
								BaseVersion: ptr.Of("1.0.0"),
								IsModified:  ptr.Of(true),
								CreatedAt:   ptr.Of(now.UnixMilli()),
								UpdatedAt:   ptr.Of(now.UnixMilli()),
							},
						},
					},
				},
				Users: []*user.UserInfoDetail{
					{
						UserID:    ptr.Of("test_creator"),
						Name:      ptr.Of("Test Creator"),
						NickName:  ptr.Of(""),
						AvatarURL: ptr.Of(""),
						Email:     ptr.Of(""),
						Mobile:    ptr.Of(""),
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "list prompt repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().ListPrompt(gomock.Any(), repo.ListPromptParam{
					SpaceID:           100,
					UserID:            "123",
					FilterPromptTypes: []entity.PromptType{entity.PromptTypeNormal},
					PageNum:           1,
					PageSize:          10,
					OrderBy:           mysql.ListPromptBasicOrderByID,
					Asc:               false,
				}).Return(nil, errorx.New("list prompt error"))

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(nil)

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListPromptRequest{
					WorkspaceID: ptr.Of(int64(100)),
					PageNum:     ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			want:    manage.NewListPromptResponse(),
			wantErr: errorx.New("list prompt error"),
		},
		{
			name: "list prompt with snippet type filter",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().ListPrompt(gomock.Any(), repo.ListPromptParam{
					SpaceID:           100,
					UserID:            "123",
					FilterPromptTypes: []entity.PromptType{entity.PromptTypeSnippet},
					PageNum:           1,
					PageSize:          10,
					OrderBy:           mysql.ListPromptBasicOrderByID,
					Asc:               false,
				}).Return(&repo.ListPromptResult{
					Total: 1,
					PromptDOs: []*entity.Prompt{
						{
							ID:        1,
							SpaceID:   100,
							PromptKey: "snippet_key",
							PromptBasic: &entity.PromptBasic{
								DisplayName:       "snippet_name",
								Description:       "snippet_description",
								LatestVersion:     "1.0.0",
								CreatedBy:         "test_creator",
								UpdatedBy:         "test_updater",
								CreatedAt:         now,
								UpdatedAt:         now,
								LatestCommittedAt: &now,
								PromptType:        entity.PromptTypeSnippet,
							},
						},
					},
				}, nil)

				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(nil)

				mockUser := mocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), []string{"test_creator"}).Return([]*rpc.UserInfo{
					{
						UserID:   "test_creator",
						UserName: "Test Creator",
					},
				}, nil)

				return fields{
					manageRepo:      mockRepo,
					authRPCProvider: mockAuth,
					userRPCProvider: mockUser,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListPromptRequest{
					WorkspaceID:       ptr.Of(int64(100)),
					FilterPromptTypes: []prompt.PromptType{prompt.PromptTypeSnippet},
					PageNum:           ptr.Of(int32(1)),
					PageSize:          ptr.Of(int32(10)),
				},
			},
			want: &manage.ListPromptResponse{
				Total: ptr.Of(int32(1)),
				Prompts: []*prompt.Prompt{
					{
						ID:          ptr.Of(int64(1)),
						WorkspaceID: ptr.Of(int64(100)),
						PromptKey:   ptr.Of("snippet_key"),
						PromptBasic: &prompt.PromptBasic{
							DisplayName:       ptr.Of("snippet_name"),
							Description:       ptr.Of("snippet_description"),
							LatestVersion:     ptr.Of("1.0.0"),
							CreatedBy:         ptr.Of("test_creator"),
							UpdatedBy:         ptr.Of("test_updater"),
							CreatedAt:         ptr.Of(now.UnixMilli()),
							UpdatedAt:         ptr.Of(now.UnixMilli()),
							LatestCommittedAt: ptr.Of(now.UnixMilli()),
							PromptType:        ptr.Of(prompt.PromptTypeSnippet),
						},
					},
				},
				Users: []*user.UserInfoDetail{
					{
						UserID:    ptr.Of("test_creator"),
						Name:      ptr.Of("Test Creator"),
						NickName:  ptr.Of(""),
						AvatarURL: ptr.Of(""),
						Email:     ptr.Of(""),
						Mobile:    ptr.Of(""),
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			app := &PromptManageApplicationImpl{
				manageRepo:       ttFields.manageRepo,
				promptService:    ttFields.promptService,
				authRPCProvider:  ttFields.authRPCProvider,
				userRPCProvider:  ttFields.userRPCProvider,
				auditRPCProvider: ttFields.auditRPCProvider,
				configProvider:   ttFields.configProvider,
			}

			got, err := app.ListPrompt(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPromptManageApplicationImpl_CreateLabel(t *testing.T) {
	t.Parallel()

	type fields struct {
		manageRepo       repo.IManageRepo
		labelRepo        repo.ILabelRepo
		promptService    service.IPromptService
		authRPCProvider  rpc.IAuthProvider
		userRPCProvider  rpc.IUserProvider
		auditRPCProvider rpc.IAuditProvider
		configProvider   conf.IConfigProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.CreateLabelRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.CreateLabelResponse
		wantErr      error
	}{
		{
			name: "成功创建标签",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceCreateLoopPrompt).Return(nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().CreateLabel(gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					authRPCProvider: mockAuth,
					promptService:   mockPromptService,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.CreateLabelRequest{
					WorkspaceID: ptr.Of(int64(100)),
					Label: &prompt.Label{
						Key: ptr.Of("test-label"),
					},
				},
			},
			want:    manage.NewCreateLabelResponse(),
			wantErr: nil,
		},
		{
			name: "用户未找到",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.CreateLabelRequest{
					WorkspaceID: ptr.Of(int64(100)),
					Label: &prompt.Label{
						Key: ptr.Of("test-label"),
					},
				},
			},
			want:    manage.NewCreateLabelResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "权限检查失败",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceCreateLoopPrompt).Return(errorx.New("permission denied"))

				return fields{
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.CreateLabelRequest{
					WorkspaceID: ptr.Of(int64(100)),
					Label: &prompt.Label{
						Key: ptr.Of("test-label"),
					},
				},
			},
			want:    manage.NewCreateLabelResponse(),
			wantErr: errorx.New("permission denied"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			app := &PromptManageApplicationImpl{
				manageRepo:       ttFields.manageRepo,
				labelRepo:        ttFields.labelRepo,
				promptService:    ttFields.promptService,
				authRPCProvider:  ttFields.authRPCProvider,
				userRPCProvider:  ttFields.userRPCProvider,
				auditRPCProvider: ttFields.auditRPCProvider,
				configProvider:   ttFields.configProvider,
			}

			got, err := app.CreateLabel(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPromptManageApplicationImpl_ListLabel(t *testing.T) {
	t.Parallel()

	type fields struct {
		manageRepo       repo.IManageRepo
		labelRepo        repo.ILabelRepo
		promptService    service.IPromptService
		authRPCProvider  rpc.IAuthProvider
		userRPCProvider  rpc.IUserProvider
		auditRPCProvider rpc.IAuditProvider
		configProvider   conf.IConfigProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.ListLabelRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.ListLabelResponse
		wantErr      error
	}{
		{
			name: "成功列出标签",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().ListLabel(gomock.Any(), gomock.Any()).Return([]*entity.PromptLabel{
					{
						ID:       1,
						SpaceID:  100,
						LabelKey: "test-label",
					},
				}, ptr.Of(int64(2)), nil)

				return fields{
					authRPCProvider: mockAuth,
					promptService:   mockPromptService,
				}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.ListLabelRequest{
					WorkspaceID: ptr.Of(int64(100)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			want: &manage.ListLabelResponse{
				Labels: []*prompt.Label{
					{
						Key: ptr.Of("test-label"),
					},
				},
				NextPageToken: ptr.Of("2"),
				HasMore:       ptr.Of(true),
			},
			wantErr: nil,
		},
		{
			name: "权限检查失败",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(errorx.New("permission denied"))

				return fields{
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.ListLabelRequest{
					WorkspaceID: ptr.Of(int64(100)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			want:    manage.NewListLabelResponse(),
			wantErr: errorx.New("permission denied"),
		},
		{
			name: "需要版本映射但未提供PromptID",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(nil)

				return fields{
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.ListLabelRequest{
					WorkspaceID:              ptr.Of(int64(100)),
					PageSize:                 ptr.Of(int32(10)),
					WithPromptVersionMapping: ptr.Of(true),
				},
			},
			want:    manage.NewListLabelResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("PromptID must be provided when WithPromptVersionMapping is true")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			app := &PromptManageApplicationImpl{
				manageRepo:       ttFields.manageRepo,
				labelRepo:        ttFields.labelRepo,
				promptService:    ttFields.promptService,
				authRPCProvider:  ttFields.authRPCProvider,
				userRPCProvider:  ttFields.userRPCProvider,
				auditRPCProvider: ttFields.auditRPCProvider,
				configProvider:   ttFields.configProvider,
			}

			got, err := app.ListLabel(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPromptManageApplicationImpl_BatchGetLabel(t *testing.T) {
	t.Parallel()

	type fields struct {
		manageRepo       repo.IManageRepo
		labelRepo        repo.ILabelRepo
		promptService    service.IPromptService
		authRPCProvider  rpc.IAuthProvider
		userRPCProvider  rpc.IUserProvider
		auditRPCProvider rpc.IAuditProvider
		configProvider   conf.IConfigProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.BatchGetLabelRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.BatchGetLabelResponse
		wantErr      error
	}{
		{
			name: "成功批量获取标签",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(nil)

				mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
				mockLabelRepo.EXPECT().BatchGetLabel(gomock.Any(), int64(100), []string{"label1", "label2"}).Return([]*entity.PromptLabel{
					{
						ID:       1,
						SpaceID:  100,
						LabelKey: "label1",
					},
					{
						ID:       2,
						SpaceID:  100,
						LabelKey: "label2",
					},
				}, nil)

				return fields{
					authRPCProvider: mockAuth,
					labelRepo:       mockLabelRepo,
				}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.BatchGetLabelRequest{
					WorkspaceID: ptr.Of(int64(100)),
					LabelKeys:   []string{"label1", "label2"},
				},
			},
			want: &manage.BatchGetLabelResponse{
				Labels: []*prompt.Label{
					{
						Key: ptr.Of("label1"),
					},
					{
						Key: ptr.Of("label2"),
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "权限检查失败",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(errorx.New("permission denied"))

				return fields{
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.BatchGetLabelRequest{
					WorkspaceID: ptr.Of(int64(100)),
					LabelKeys:   []string{"label1", "label2"},
				},
			},
			want:    manage.NewBatchGetLabelResponse(),
			wantErr: errorx.New("permission denied"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			app := &PromptManageApplicationImpl{
				manageRepo:       ttFields.manageRepo,
				labelRepo:        ttFields.labelRepo,
				promptService:    ttFields.promptService,
				authRPCProvider:  ttFields.authRPCProvider,
				userRPCProvider:  ttFields.userRPCProvider,
				auditRPCProvider: ttFields.auditRPCProvider,
				configProvider:   ttFields.configProvider,
			}

			got, err := app.BatchGetLabel(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPromptManageApplicationImpl_UpdateCommitLabels(t *testing.T) {
	t.Parallel()

	type fields struct {
		manageRepo       repo.IManageRepo
		labelRepo        repo.ILabelRepo
		promptService    service.IPromptService
		authRPCProvider  rpc.IAuthProvider
		userRPCProvider  rpc.IUserProvider
		auditRPCProvider rpc.IAuditProvider
		configProvider   conf.IConfigProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.UpdateCommitLabelsRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.UpdateCommitLabelsResponse
		wantErr      error
	}{
		{
			name: "成功更新提交标签",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(100), []int64{1}, consts.ActionLoopPromptEdit).Return(nil)

				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().UpdateCommitLabels(gomock.Any(), gomock.Any()).Return(nil)

				return fields{
					authRPCProvider: mockAuth,
					promptService:   mockPromptService,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.UpdateCommitLabelsRequest{
					WorkspaceID:   ptr.Of(int64(100)),
					PromptID:      ptr.Of(int64(1)),
					CommitVersion: ptr.Of("1.0.0"),
					LabelKeys:     []string{"label1", "label2"},
				},
			},
			want:    manage.NewUpdateCommitLabelsResponse(),
			wantErr: nil,
		},
		{
			name: "用户未找到",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.UpdateCommitLabelsRequest{
					WorkspaceID:   ptr.Of(int64(100)),
					PromptID:      ptr.Of(int64(1)),
					CommitVersion: ptr.Of("1.0.0"),
					LabelKeys:     []string{"label1", "label2"},
				},
			},
			want:    manage.NewUpdateCommitLabelsResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "权限检查失败",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(100), []int64{1}, consts.ActionLoopPromptEdit).Return(errorx.New("permission denied"))

				return fields{
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.UpdateCommitLabelsRequest{
					WorkspaceID:   ptr.Of(int64(100)),
					PromptID:      ptr.Of(int64(1)),
					CommitVersion: ptr.Of("1.0.0"),
					LabelKeys:     []string{"label1", "label2"},
				},
			},
			want:    manage.NewUpdateCommitLabelsResponse(),
			wantErr: errorx.New("permission denied"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			app := &PromptManageApplicationImpl{
				manageRepo:       ttFields.manageRepo,
				labelRepo:        ttFields.labelRepo,
				promptService:    ttFields.promptService,
				authRPCProvider:  ttFields.authRPCProvider,
				userRPCProvider:  ttFields.userRPCProvider,
				auditRPCProvider: ttFields.auditRPCProvider,
				configProvider:   ttFields.configProvider,
			}

			got, err := app.UpdateCommitLabels(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPromptManageApplicationImpl_ListParentPrompt(t *testing.T) {
	type fields struct {
		manageRepo       repo.IManageRepo
		promptService    service.IPromptService
		authRPCProvider  rpc.IAuthProvider
		userRPCProvider  rpc.IUserProvider
		auditRPCProvider rpc.IAuditProvider
		configProvider   conf.IConfigProvider
		labelRepo        repo.ILabelRepo
	}
	type args struct {
		ctx     context.Context
		request *manage.ListParentPromptRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.ListParentPromptResponse
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.ListParentPromptRequest{
					WorkspaceID: ptr.Of(int64(1)),
					PromptID:    ptr.Of(int64(1)),
				},
			},
			want:    manage.NewListParentPromptResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "permission denied",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(1), consts.ActionLoopPromptRead).
					Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))

				return fields{
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListParentPromptRequest{
					WorkspaceID: ptr.Of(int64(1)),
					PromptID:    ptr.Of(int64(1)),
				},
			},
			want:    manage.NewListParentPromptResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "invalid prompt ID",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(1), consts.ActionLoopPromptRead).
					Return(nil)

				return fields{
					authRPCProvider:  mockAuth,
					manageRepo:       nil,
					promptService:    nil,
					userRPCProvider:  nil,
					auditRPCProvider: nil,
					configProvider:   nil,
					labelRepo:        nil,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListParentPromptRequest{
					WorkspaceID: ptr.Of(int64(1)),
					PromptID:    ptr.Of(int64(0)),
				},
			},
			want:    manage.NewListParentPromptResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Prompt ID is required")),
		},
		{
			name: "successful list parent prompts",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(1), consts.ActionLoopPromptRead).
					Return(nil)

				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().ListParentPrompt(gomock.Any(), repo.ListParentPromptParam{
					SubPromptID:       1,
					SubPromptVersions: []string{"v1.0.0"},
				}).Return(map[string]*repo.PromptCommitVersions{
					"v1.0.0": {
						PromptID:  2,
						PromptKey: "parent_prompt",
						SpaceID:   1,
						PromptBasic: &entity.PromptBasic{
							DisplayName:   "parent name",
							Description:   "parent description",
							LatestVersion: "2.0.0",
							PromptType:    entity.PromptTypeSnippet,
						},
						CommitVersions: []string{"v2.0.0"},
					},
				}, nil)

				return fields{
					manageRepo:       mockRepo,
					authRPCProvider:  mockAuth,
					promptService:    nil,
					userRPCProvider:  nil,
					auditRPCProvider: nil,
					configProvider:   nil,
					labelRepo:        nil,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListParentPromptRequest{
					WorkspaceID:    ptr.Of(int64(1)),
					PromptID:       ptr.Of(int64(1)),
					CommitVersions: []string{"v1.0.0"},
				},
			},
			want: &manage.ListParentPromptResponse{
				ParentPrompts: map[string]*prompt.PromptCommitVersions{
					"v1.0.0": {
						ID:          ptr.Of(int64(2)),
						WorkspaceID: ptr.Of(int64(1)),
						PromptKey:   ptr.Of("parent_prompt"),
						PromptBasic: &prompt.PromptBasic{
							DisplayName:   ptr.Of("parent name"),
							Description:   ptr.Of("parent description"),
							LatestVersion: ptr.Of("2.0.0"),
							PromptType:    ptr.Of(prompt.PromptTypeSnippet),
							CreatedBy:     ptr.Of(""),
							UpdatedBy:     ptr.Of(""),
							CreatedAt:     ptr.Of(time.Time{}.UnixMilli()),
							UpdatedAt:     ptr.Of(time.Time{}.UnixMilli()),
						},
						CommitVersions: []string{"v2.0.0"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "repository error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(1), consts.ActionLoopPromptRead).
					Return(nil)

				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().ListParentPrompt(gomock.Any(), repo.ListParentPromptParam{
					SubPromptID: 1,
				}).Return(nil, errorx.New("database error"))

				return fields{
					manageRepo:       mockRepo,
					authRPCProvider:  mockAuth,
					promptService:    nil,
					userRPCProvider:  nil,
					auditRPCProvider: nil,
					configProvider:   nil,
					labelRepo:        nil,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.ListParentPromptRequest{
					WorkspaceID: ptr.Of(int64(1)),
					PromptID:    ptr.Of(int64(1)),
				},
			},
			want:    manage.NewListParentPromptResponse(),
			wantErr: errorx.New("database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ttFields := tt.fieldsGetter(ctrl)

			app := &PromptManageApplicationImpl{
				manageRepo:       ttFields.manageRepo,
				labelRepo:        ttFields.labelRepo,
				promptService:    ttFields.promptService,
				authRPCProvider:  ttFields.authRPCProvider,
				userRPCProvider:  ttFields.userRPCProvider,
				auditRPCProvider: ttFields.auditRPCProvider,
				configProvider:   ttFields.configProvider,
			}

			got, err := app.ListParentPrompt(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want.ParentPrompts, got.ParentPrompts)
			}
		})
	}
}
