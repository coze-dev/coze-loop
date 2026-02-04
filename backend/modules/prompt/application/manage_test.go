// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
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

func TestPromptManageApplicationImpl_DeletePrompt(t *testing.T) {
	type fields struct {
		manageRepo      repo.IManageRepo
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
			name:         "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields { return fields{} },
			args: args{
				ctx: context.Background(),
				request: &manage.DeletePromptRequest{
					PromptID: ptr.Of(int64(1)),
				},
			},
			want:    manage.NewDeletePromptResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "get prompt error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 1}).Return(nil, errorx.New("get error"))
				return fields{manageRepo: repoMock}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.DeletePromptRequest{
					PromptID: ptr.Of(int64(1)),
				},
			},
			want:    manage.NewDeletePromptResponse(),
			wantErr: errorx.New("get error"),
		},
		{
			name: "snippet prompt not allowed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 2}).Return(&entity.Prompt{
					ID:          2,
					SpaceID:     10,
					PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeSnippet},
				}, nil)
				return fields{manageRepo: repoMock}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.DeletePromptRequest{
					PromptID: ptr.Of(int64(2)),
				},
			},
			want:    manage.NewDeletePromptResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Snippet prompt can not be deleted")),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 3}).Return(&entity.Prompt{
					ID:          3,
					SpaceID:     20,
					PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeNormal},
				}, nil)
				repoMock.EXPECT().DeletePrompt(gomock.Any(), int64(3)).Return(nil)
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(20), []int64{int64(3)}, consts.ActionLoopPromptEdit).Return(nil)
				return fields{manageRepo: repoMock, authRPCProvider: auth}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.DeletePromptRequest{
					PromptID: ptr.Of(int64(3)),
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

			tFields := caseData.fieldsGetter(ctrl)
			app := &PromptManageApplicationImpl{
				manageRepo:      tFields.manageRepo,
				authRPCProvider: tFields.authRPCProvider,
			}

			resp, err := app.DeletePrompt(caseData.args.ctx, caseData.args.request)
			unittest.AssertErrorEqual(t, caseData.wantErr, err)
			if err == nil {
				assert.Equal(t, caseData.want, resp)
			}
		})
	}
}

func TestPromptManageApplicationImpl_BatchGetPrompt(t *testing.T) {
	type fields struct {
		manageRepo repo.IManageRepo
	}
	type args struct {
		ctx     context.Context
		request *manage.BatchGetPromptRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantLen      int
		wantErr      error
	}{
		{
			name: "repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().MGetPrompt(gomock.Any(), gomock.Any()).Return(nil, errorx.New("mget error"))
				return fields{manageRepo: repoMock}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.BatchGetPromptRequest{
					Queries: []*manage.PromptQuery{
						{
							PromptID:      ptr.Of(int64(1)),
							WithCommit:    ptr.Of(true),
							CommitVersion: ptr.Of("v1"),
						},
					},
				},
			},
			wantLen: 0,
			wantErr: errorx.New("mget error"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().MGetPrompt(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, params []repo.GetPromptParam, _ ...repo.GetPromptOptionFunc) (map[repo.GetPromptParam]*entity.Prompt, error) {
					assert.Len(t, params, 1)
					return map[repo.GetPromptParam]*entity.Prompt{
						params[0]: {
							ID:        params[0].PromptID,
							SpaceID:   100,
							PromptKey: "key",
							PromptBasic: &entity.PromptBasic{
								DisplayName: "name",
							},
						},
					}, nil
				})
				return fields{manageRepo: repoMock}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.BatchGetPromptRequest{
					Queries: []*manage.PromptQuery{
						{
							PromptID:   ptr.Of(int64(5)),
							WithCommit: ptr.Of(false),
						},
					},
				},
			},
			wantLen: 1,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		caseData := tt
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tFields := caseData.fieldsGetter(ctrl)
			app := &PromptManageApplicationImpl{manageRepo: tFields.manageRepo}

			resp, err := app.BatchGetPrompt(caseData.args.ctx, caseData.args.request)
			unittest.AssertErrorEqual(t, caseData.wantErr, err)
			if err == nil {
				assert.Len(t, resp.Results, caseData.wantLen)
			}
		})
	}
}

func TestNewPromptManageApplication(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	manageRepo := repomocks.NewMockIManageRepo(ctrl)
	labelRepo := repomocks.NewMockILabelRepo(ctrl)
	promptService := servicemocks.NewMockIPromptService(ctrl)
	auth := mocks.NewMockIAuthProvider(ctrl)
	user := mocks.NewMockIUserProvider(ctrl)
	audit := mocks.NewMockIAuditProvider(ctrl)
	config := confmocks.NewMockIConfigProvider(ctrl)

	app := NewPromptManageApplication(manageRepo, labelRepo, promptService, auth, user, audit, config)
	impl, ok := app.(*PromptManageApplicationImpl)
	assert.True(t, ok)
	assert.Equal(t, manageRepo, impl.manageRepo)
	assert.Equal(t, labelRepo, impl.labelRepo)
	assert.Equal(t, promptService, impl.promptService)
	assert.Equal(t, auth, impl.authRPCProvider)
	assert.Equal(t, user, impl.userRPCProvider)
	assert.Equal(t, audit, impl.auditRPCProvider)
	assert.Equal(t, config, impl.configProvider)
}

func TestPromptManageApplicationImpl_CreatePrompt(t *testing.T) {
	type fields struct {
		promptService    service.IPromptService
		authRPCProvider  rpc.IAuthProvider
		auditRPCProvider rpc.IAuditProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.CreatePromptRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *manage.CreatePromptResponse
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				request: &manage.CreatePromptRequest{
					WorkspaceID: ptr.Of(int64(100)),
					PromptKey:   ptr.Of("prompt_key"),
					PromptName:  ptr.Of("prompt_name"),
				},
			},
			want:    manage.NewCreatePromptResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "permission denied",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceCreateLoopPrompt).Return(errorx.New("permission denied"))
				return fields{authRPCProvider: auth}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.CreatePromptRequest{
					WorkspaceID: ptr.Of(int64(100)),
					PromptKey:   ptr.Of("prompt_key"),
					PromptName:  ptr.Of("prompt_name"),
				},
			},
			want:    manage.NewCreatePromptResponse(),
			wantErr: errorx.New("permission denied"),
		},
		{
			name: "audit failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceCreateLoopPrompt).Return(nil)
				audit := mocks.NewMockIAuditProvider(ctrl)
				audit.EXPECT().AuditPrompt(gomock.Any(), gomock.Any()).Return(errorx.New("audit failed"))
				return fields{authRPCProvider: auth, auditRPCProvider: audit}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.CreatePromptRequest{
					WorkspaceID:       ptr.Of(int64(100)),
					PromptKey:         ptr.Of("prompt_key"),
					PromptName:        ptr.Of("prompt_name"),
					PromptDescription: ptr.Of("desc"),
					DraftDetail:       &prompt.PromptDetail{},
				},
			},
			want:    manage.NewCreatePromptResponse(),
			wantErr: errorx.New("audit failed"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceCreateLoopPrompt).Return(nil)
				audit := mocks.NewMockIAuditProvider(ctrl)
				audit.EXPECT().AuditPrompt(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, promptDO *entity.Prompt) error {
					assert.Equal(t, int64(100), promptDO.SpaceID)
					assert.Equal(t, "user", promptDO.PromptBasic.CreatedBy)
					return nil
				})
				promptSvc := servicemocks.NewMockIPromptService(ctrl)
				promptSvc.EXPECT().CreatePrompt(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, promptDO *entity.Prompt) (int64, error) {
					assert.Equal(t, entity.PromptTypeNormal, promptDO.PromptBasic.PromptType)
					assert.NotNil(t, promptDO.PromptDraft)
					return 999, nil
				})
				return fields{
					promptService:    promptSvc,
					authRPCProvider:  auth,
					auditRPCProvider: audit,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.CreatePromptRequest{
					WorkspaceID:       ptr.Of(int64(100)),
					PromptKey:         ptr.Of("prompt_key"),
					PromptName:        ptr.Of("prompt_name"),
					PromptDescription: ptr.Of("desc"),
					DraftDetail: &prompt.PromptDetail{
						PromptTemplate: &prompt.PromptTemplate{},
					},
				},
			},
			want:    &manage.CreatePromptResponse{PromptID: ptr.Of(int64(999))},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		caseData := tt
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tFields := caseData.fieldsGetter(ctrl)
			app := &PromptManageApplicationImpl{
				promptService:    tFields.promptService,
				authRPCProvider:  tFields.authRPCProvider,
				auditRPCProvider: tFields.auditRPCProvider,
			}

			resp, err := app.CreatePrompt(caseData.args.ctx, caseData.args.request)
			unittest.AssertErrorEqual(t, caseData.wantErr, err)
			if err == nil {
				assert.Equal(t, caseData.want, resp)
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

	baseTime := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	draftTime := baseTime.Add(time.Minute)
	snippetTime := baseTime.Add(2 * time.Minute)

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
			name: "prompt service error when commit version provided",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Times(0)
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().GetPrompt(gomock.Any(), service.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
					WithDraft:     false,
					UserID:        "123",
					ExpandSnippet: false,
				}).Return(nil, errorx.New("prompt service error"))
				return fields{
					manageRepo:    mockRepo,
					promptService: mockPromptService,
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
			wantErr: errorx.New("prompt service error"),
		},
		{
			name: "get prompt with commit success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Times(0)
				promptDO := &entity.Prompt{
					ID:        1,
					SpaceID:   100,
					PromptKey: "commit_key",
					PromptBasic: &entity.PromptBasic{
						PromptType:    entity.PromptTypeNormal,
						DisplayName:   "commit name",
						Description:   "commit description",
						LatestVersion: "1.0.0",
						CreatedBy:     "creator",
						UpdatedBy:     "updater",
						CreatedAt:     baseTime,
						UpdatedAt:     baseTime,
					},
					PromptCommit: &entity.PromptCommit{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{{
									Role:    entity.RoleUser,
									Content: ptr.Of("commit content"),
								}},
								HasSnippets: false,
							},
						},
						CommitInfo: &entity.CommitInfo{
							Version:     "1.0.0",
							BaseVersion: "0.9.0",
							Description: "commit description",
							CommittedBy: "committer",
							CommittedAt: baseTime,
						},
					},
				}
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().GetPrompt(gomock.Any(), service.GetPromptParam{
					PromptID:      1,
					WithCommit:    true,
					CommitVersion: "1.0.0",
					WithDraft:     false,
					UserID:        "123",
					ExpandSnippet: false,
				}).Return(promptDO, nil)
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
					PromptID:      ptr.Of(int64(1)),
					WithCommit:    ptr.Of(true),
					CommitVersion: ptr.Of("1.0.0"),
				},
			},
			want: func() *manage.GetPromptResponse {
				resp := manage.NewGetPromptResponse()
				resp.Prompt = &prompt.Prompt{
					ID:          ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					PromptKey:   ptr.Of("commit_key"),
					PromptBasic: &prompt.PromptBasic{
						DisplayName:   ptr.Of("commit name"),
						Description:   ptr.Of("commit description"),
						LatestVersion: ptr.Of("1.0.0"),
						CreatedBy:     ptr.Of("creator"),
						UpdatedBy:     ptr.Of("updater"),
						CreatedAt:     ptr.Of(baseTime.UnixMilli()),
						UpdatedAt:     ptr.Of(baseTime.UnixMilli()),
						PromptType:    ptr.Of(prompt.PromptTypeNormal),
					},
					PromptCommit: &prompt.PromptCommit{
						Detail: &prompt.PromptDetail{
							PromptTemplate: &prompt.PromptTemplate{
								TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								HasSnippet:   ptr.Of(false),
								Messages: []*prompt.Message{{
									Role:    ptr.Of(prompt.RoleUser),
									Content: ptr.Of("commit content"),
								}},
							},
						},
						CommitInfo: &prompt.CommitInfo{
							Version:     ptr.Of("1.0.0"),
							BaseVersion: ptr.Of("0.9.0"),
							Description: ptr.Of("commit description"),
							CommittedBy: ptr.Of("committer"),
							CommittedAt: ptr.Of(baseTime.UnixMilli()),
						},
					},
				}
				return resp
			}(),
			wantErr: nil,
		},
		{
			name: "get prompt with draft and default config success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Times(0)
				promptDO := &entity.Prompt{
					ID:        2,
					SpaceID:   200,
					PromptKey: "draft_key",
					PromptBasic: &entity.PromptBasic{
						PromptType:    entity.PromptTypeNormal,
						DisplayName:   "draft name",
						Description:   "draft description",
						LatestVersion: "2.0.0",
						CreatedBy:     "creator",
						UpdatedBy:     "updater",
						CreatedAt:     draftTime,
						UpdatedAt:     draftTime,
					},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{{
									Role:    entity.RoleSystem,
									Content: ptr.Of("draft content"),
								}},
								HasSnippets: false,
							},
						},
						DraftInfo: &entity.DraftInfo{
							UserID:      "123",
							BaseVersion: "2.0.0",
							IsModified:  true,
							CreatedAt:   draftTime,
							UpdatedAt:   draftTime,
						},
					},
				}
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().GetPrompt(gomock.Any(), service.GetPromptParam{
					PromptID:      2,
					WithCommit:    false,
					CommitVersion: "",
					WithDraft:     true,
					UserID:        "123",
					ExpandSnippet: false,
				}).Return(promptDO, nil)
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(200), []int64{int64(2)}, consts.ActionLoopPromptRead).Return(nil)
				mockConfig := confmocks.NewMockIConfigProvider(ctrl)
				mockConfig.EXPECT().GetPromptDefaultConfig(gomock.Any()).Return(&prompt.PromptDetail{
					PromptTemplate: &prompt.PromptTemplate{
						TemplateType: ptr.Of(prompt.TemplateTypeNormal),
						HasSnippet:   ptr.Of(false),
						Messages: []*prompt.Message{{
							Role:    ptr.Of(prompt.RoleSystem),
							Content: ptr.Of("default config"),
						}},
					},
				}, nil)

				return fields{
					manageRepo:      mockRepo,
					promptService:   mockPromptService,
					authRPCProvider: mockAuth,
					configProvider:  mockConfig,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					PromptID:          ptr.Of(int64(2)),
					WithDraft:         ptr.Of(true),
					WithDefaultConfig: ptr.Of(true),
				},
			},
			want: func() *manage.GetPromptResponse {
				resp := manage.NewGetPromptResponse()
				resp.Prompt = &prompt.Prompt{
					ID:          ptr.Of(int64(2)),
					WorkspaceID: ptr.Of(int64(200)),
					PromptKey:   ptr.Of("draft_key"),
					PromptBasic: &prompt.PromptBasic{
						DisplayName:   ptr.Of("draft name"),
						Description:   ptr.Of("draft description"),
						LatestVersion: ptr.Of("2.0.0"),
						CreatedBy:     ptr.Of("creator"),
						UpdatedBy:     ptr.Of("updater"),
						CreatedAt:     ptr.Of(draftTime.UnixMilli()),
						UpdatedAt:     ptr.Of(draftTime.UnixMilli()),
						PromptType:    ptr.Of(prompt.PromptTypeNormal),
					},
					PromptDraft: &prompt.PromptDraft{
						Detail: &prompt.PromptDetail{
							PromptTemplate: &prompt.PromptTemplate{
								TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								HasSnippet:   ptr.Of(false),
								Messages: []*prompt.Message{{
									Role:    ptr.Of(prompt.RoleSystem),
									Content: ptr.Of("draft content"),
								}},
							},
						},
						DraftInfo: &prompt.DraftInfo{
							UserID:      ptr.Of("123"),
							BaseVersion: ptr.Of("2.0.0"),
							IsModified:  ptr.Of(true),
							CreatedAt:   ptr.Of(draftTime.UnixMilli()),
							UpdatedAt:   ptr.Of(draftTime.UnixMilli()),
						},
					},
				}
				resp.DefaultConfig = &prompt.PromptDetail{
					PromptTemplate: &prompt.PromptTemplate{
						TemplateType: ptr.Of(prompt.TemplateTypeNormal),
						HasSnippet:   ptr.Of(false),
						Messages: []*prompt.Message{{
							Role:    ptr.Of(prompt.RoleSystem),
							Content: ptr.Of("default config"),
						}},
					},
				}
				return resp
			}(),
			wantErr: nil,
		},
		{
			name: "workspace mismatch returns resource not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().GetPrompt(gomock.Any(), gomock.Any()).Times(0)
				promptDO := &entity.Prompt{
					ID:        3,
					SpaceID:   300,
					PromptKey: "workspace_key",
					PromptBasic: &entity.PromptBasic{
						PromptType:    entity.PromptTypeNormal,
						DisplayName:   "workspace name",
						Description:   "workspace description",
						LatestVersion: "3.0.0",
						CreatedBy:     "creator",
						UpdatedBy:     "updater",
						CreatedAt:     baseTime,
						UpdatedAt:     baseTime,
					},
				}
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().GetPrompt(gomock.Any(), service.GetPromptParam{
					PromptID:      3,
					WithCommit:    true,
					CommitVersion: "3.0.0",
					WithDraft:     false,
					UserID:        "123",
					ExpandSnippet: false,
				}).Return(promptDO, nil)
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(300), []int64{int64(3)}, consts.ActionLoopPromptRead).Return(nil)

				return fields{
					manageRepo:      mockRepo,
					promptService:   mockPromptService,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					PromptID:      ptr.Of(int64(3)),
					WorkspaceID:   ptr.Of(int64(999)),
					WithCommit:    ptr.Of(true),
					CommitVersion: ptr.Of("3.0.0"),
				},
			},
			want:    manage.NewGetPromptResponse(),
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("WorkspaceID not match")),
		},
		{
			name: "snippet prompt parent references success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				promptDO := &entity.Prompt{
					ID:        4,
					SpaceID:   400,
					PromptKey: "snippet_key",
					PromptBasic: &entity.PromptBasic{
						PromptType:    entity.PromptTypeSnippet,
						DisplayName:   "snippet name",
						Description:   "snippet description",
						LatestVersion: "4.0.0",
						CreatedBy:     "creator",
						UpdatedBy:     "updater",
						CreatedAt:     snippetTime,
						UpdatedAt:     snippetTime,
					},
				}
				mockPromptService := servicemocks.NewMockIPromptService(ctrl)
				mockPromptService.EXPECT().GetPrompt(gomock.Any(), service.GetPromptParam{
					PromptID:      4,
					WithCommit:    false,
					CommitVersion: "",
					WithDraft:     false,
					UserID:        "123",
					ExpandSnippet: false,
				}).Return(promptDO, nil)
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(400), []int64{int64(4)}, consts.ActionLoopPromptRead).Return(nil)

				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				gomock.InOrder(
					mockRepo.EXPECT().MGetVersionsByPromptID(gomock.Any(), int64(4)).Return([]string{"4.0.0", "4.1.0"}, nil),
					mockRepo.EXPECT().ListParentPrompt(gomock.Any(), repo.ListParentPromptParam{
						SubPromptID:       4,
						SubPromptVersions: []string{"4.0.0", "4.1.0"},
					}).Return(map[string][]*repo.PromptCommitVersions{
						"4.0.0": {{CommitVersions: []string{"10.0.0", "10.1.0"}}},
						"4.1.0": {
							{CommitVersions: []string{"11.0.0"}},
							{CommitVersions: []string{"12.0.0", "12.1.0"}},
						},
					}, nil),
				)

				return fields{
					manageRepo:      mockRepo,
					promptService:   mockPromptService,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				request: &manage.GetPromptRequest{
					PromptID: ptr.Of(int64(4)),
				},
			},
			want: func() *manage.GetPromptResponse {
				resp := manage.NewGetPromptResponse()
				resp.Prompt = &prompt.Prompt{
					ID:          ptr.Of(int64(4)),
					WorkspaceID: ptr.Of(int64(400)),
					PromptKey:   ptr.Of("snippet_key"),
					PromptBasic: &prompt.PromptBasic{
						DisplayName:   ptr.Of("snippet name"),
						Description:   ptr.Of("snippet description"),
						LatestVersion: ptr.Of("4.0.0"),
						CreatedBy:     ptr.Of("creator"),
						UpdatedBy:     ptr.Of("updater"),
						CreatedAt:     ptr.Of(snippetTime.UnixMilli()),
						UpdatedAt:     ptr.Of(snippetTime.UnixMilli()),
						PromptType:    ptr.Of(prompt.PromptTypeSnippet),
					},
				}
				resp.TotalParentReferences = ptr.Of(int32(5))
				return resp
			}(),
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
				userRPCProvider: ff.userRPCProvider,
				configProvider:  ff.configProvider,
			}

			got, err := app.GetPrompt(caseData.args.ctx, caseData.args.request)
			unittest.AssertErrorEqual(t, caseData.wantErr, err)
			if err == nil {
				assert.Equal(t, caseData.want, got)
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
					OrderBy:           mysql.ListPromptBasicOrderByCreatedAt,
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
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, ids []string) ([]*rpc.UserInfo, error) {
					assert.ElementsMatch(t, []string{"test_creator", "test_updater"}, ids)
					return []*rpc.UserInfo{
						{UserID: "test_creator", UserName: "Test Creator"},
						{UserID: "test_updater", UserName: "Test Updater"},
					}, nil
				})

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
					{
						UserID:    ptr.Of("test_updater"),
						Name:      ptr.Of("Test Updater"),
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
					OrderBy:           mysql.ListPromptBasicOrderByCreatedAt,
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
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, ids []string) ([]*rpc.UserInfo, error) {
					assert.ElementsMatch(t, []string{"test_creator", "test_updater"}, ids)
					return []*rpc.UserInfo{
						{UserID: "test_creator", UserName: "Test Creator"},
						{UserID: "test_updater", UserName: "Test Updater"},
					}, nil
				})

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
					{
						UserID:    ptr.Of("test_updater"),
						Name:      ptr.Of("Test Updater"),
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
					OrderBy:           mysql.ListPromptBasicOrderByCreatedAt,
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
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, ids []string) ([]*rpc.UserInfo, error) {
					assert.ElementsMatch(t, []string{"test_creator", "test_updater"}, ids)
					return []*rpc.UserInfo{
						{UserID: "test_creator", UserName: "Test Creator"},
						{UserID: "test_updater", UserName: "Test Updater"},
					}, nil
				})

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
					{
						UserID:    ptr.Of("test_updater"),
						Name:      ptr.Of("Test Updater"),
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
					OrderBy:           mysql.ListPromptBasicOrderByCreatedAt,
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
					OrderBy:           mysql.ListPromptBasicOrderByCreatedAt,
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
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, ids []string) ([]*rpc.UserInfo, error) {
					assert.ElementsMatch(t, []string{"test_creator", "test_updater"}, ids)
					return []*rpc.UserInfo{
						{UserID: "test_creator", UserName: "Test Creator"},
						{UserID: "test_updater", UserName: "Test Updater"},
					}, nil
				})

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
					{
						UserID:    ptr.Of("test_updater"),
						Name:      ptr.Of("Test Updater"),
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
				}).Return(map[string][]*repo.PromptCommitVersions{
					"v1.0.0": {
						{
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
				ParentPrompts: map[string][]*prompt.PromptCommitVersions{
					"v1.0.0": {
						{
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

func TestPromptManageApplicationImpl_UpdatePrompt(t *testing.T) {
	type fields struct {
		manageRepo    repo.IManageRepo
		authProvider  rpc.IAuthProvider
		auditProvider rpc.IAuditProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.UpdatePromptRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name:         "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields { return fields{} },
			args: args{
				ctx:     context.Background(),
				request: &manage.UpdatePromptRequest{PromptID: ptr.Of(int64(1))},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 2}).Return(&entity.Prompt{
					ID:          2,
					SpaceID:     20,
					PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeNormal},
				}, nil)
				repoMock.EXPECT().UpdatePrompt(gomock.Any(), repo.UpdatePromptParam{
					PromptID:          2,
					UpdatedBy:         "user",
					PromptName:        "name",
					PromptDescription: "desc",
				}).Return(nil)
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(20), []int64{int64(2)}, consts.ActionLoopPromptEdit).Return(nil)
				audit := mocks.NewMockIAuditProvider(ctrl)
				audit.EXPECT().AuditPrompt(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, promptDO *entity.Prompt) error {
					assert.Equal(t, int64(2), promptDO.ID)
					assert.Equal(t, "name", promptDO.PromptBasic.DisplayName)
					return nil
				})
				return fields{manageRepo: repoMock, authProvider: auth, auditProvider: audit}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.UpdatePromptRequest{
					PromptID:          ptr.Of(int64(2)),
					PromptName:        ptr.Of("name"),
					PromptDescription: ptr.Of("desc"),
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		caseData := tt
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tFields := caseData.fieldsGetter(ctrl)
			app := &PromptManageApplicationImpl{
				manageRepo:       tFields.manageRepo,
				authRPCProvider:  tFields.authProvider,
				auditRPCProvider: tFields.auditProvider,
			}

			resp, err := app.UpdatePrompt(caseData.args.ctx, caseData.args.request)
			unittest.AssertErrorEqual(t, caseData.wantErr, err)
			if err == nil {
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestPromptManageApplicationImpl_GetPrompt_AutoCommitVersion(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockIManageRepo(ctrl)
	repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 10}).Return(&entity.Prompt{
		ID:          10,
		PromptBasic: &entity.PromptBasic{LatestVersion: "v2"},
	}, nil)
	promptSvc := servicemocks.NewMockIPromptService(ctrl)
	promptSvc.EXPECT().GetPrompt(gomock.Any(), service.GetPromptParam{
		PromptID:      10,
		WithCommit:    true,
		CommitVersion: "v2",
		WithDraft:     false,
		UserID:        "user",
		ExpandSnippet: false,
	}).Return(&entity.Prompt{ID: 10, SpaceID: 200}, nil)
	auth := mocks.NewMockIAuthProvider(ctrl)
	auth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(200), []int64{int64(10)}, consts.ActionLoopPromptRead).Return(nil)

	app := &PromptManageApplicationImpl{
		manageRepo:      repoMock,
		promptService:   promptSvc,
		authRPCProvider: auth,
	}

	resp, err := app.GetPrompt(session.WithCtxUser(context.Background(), &session.User{ID: "user"}), &manage.GetPromptRequest{
		PromptID:   ptr.Of(int64(10)),
		WithCommit: ptr.Of(true),
	})
	unittest.AssertErrorEqual(t, nil, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(10), resp.GetPrompt().GetID())
}

func TestPromptManageApplicationImpl_SaveDraft(t *testing.T) {
	type fields struct {
		manageRepo    repo.IManageRepo
		authProvider  rpc.IAuthProvider
		promptService service.IPromptService
	}
	type args struct {
		ctx     context.Context
		request *manage.SaveDraftRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name:         "invalid draft",
			fieldsGetter: func(ctrl *gomock.Controller) fields { return fields{} },
			args: args{
				ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.SaveDraftRequest{PromptDraft: &prompt.PromptDraft{}},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Draft is not specified")),
		},
		{
			name: "get prompt error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 3}).Return(nil, errorx.New("repo error"))
				return fields{manageRepo: repoMock}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.SaveDraftRequest{
					PromptID: ptr.Of(int64(3)),
					PromptDraft: &prompt.PromptDraft{
						DraftInfo: &prompt.DraftInfo{},
						Detail:    &prompt.PromptDetail{},
					},
				},
			},
			wantErr: errorx.New("repo error"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 3}).Return(&entity.Prompt{ID: 3, SpaceID: 30}, nil)
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(30), []int64{int64(3)}, consts.ActionLoopPromptEdit).Return(nil)
				promptSvc := servicemocks.NewMockIPromptService(ctrl)
				promptSvc.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, promptDO *entity.Prompt) (*entity.DraftInfo, error) {
					assert.Equal(t, "user", promptDO.PromptDraft.DraftInfo.UserID)
					return &entity.DraftInfo{UserID: "user", IsModified: true}, nil
				})
				return fields{manageRepo: repoMock, authProvider: auth, promptService: promptSvc}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.SaveDraftRequest{
					PromptID: ptr.Of(int64(3)),
					PromptDraft: &prompt.PromptDraft{
						DraftInfo: &prompt.DraftInfo{},
						Detail:    &prompt.PromptDetail{},
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		caseData := tt
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tFields := caseData.fieldsGetter(ctrl)
			app := &PromptManageApplicationImpl{
				manageRepo:       tFields.manageRepo,
				authRPCProvider:  tFields.authProvider,
				promptService:    tFields.promptService,
			}

			resp, err := app.SaveDraft(caseData.args.ctx, caseData.args.request)
			unittest.AssertErrorEqual(t, caseData.wantErr, err)
			if err == nil {
				assert.NotNil(t, resp.DraftInfo)
			}
		})
	}
}

func TestPromptManageApplicationImpl_CommitDraft(t *testing.T) {
	invalidVersionErr := func() error {
		_, err := semver.StrictNewVersion("invalid")
		return err
	}()
	type fields struct {
		manageRepo    repo.IManageRepo
		authProvider  rpc.IAuthProvider
		auditProvider rpc.IAuditProvider
		promptService service.IPromptService
	}
	type args struct {
		ctx     context.Context
		request *manage.CommitDraftRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name:         "invalid semver",
			fieldsGetter: func(ctrl *gomock.Controller) fields { return fields{} },
			args: args{
				ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.CommitDraftRequest{PromptID: ptr.Of(int64(4)), CommitVersion: ptr.Of("invalid")},
			},
			wantErr: invalidVersionErr,
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 4, WithDraft: true, UserID: "user"}).Return(&entity.Prompt{ID: 4, SpaceID: 40}, nil)
				repoMock.EXPECT().CommitDraft(gomock.Any(), repo.CommitDraftParam{
					PromptID:          4,
					UserID:            "user",
					CommitVersion:     "1.0.0",
					CommitDescription: "desc",
					LabelKeys:         []string{"label"},
				}).Return(nil)
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(40), []int64{int64(4)}, consts.ActionLoopPromptEdit).Return(nil)
				audit := mocks.NewMockIAuditProvider(ctrl)
				audit.EXPECT().AuditPrompt(gomock.Any(), gomock.Any()).Return(nil)
				promptSvc := servicemocks.NewMockIPromptService(ctrl)
				promptSvc.EXPECT().ValidateLabelsExist(gomock.Any(), int64(40), []string{"label"}).Return(nil)
				return fields{manageRepo: repoMock, authProvider: auth, auditProvider: audit, promptService: promptSvc}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.CommitDraftRequest{
					PromptID:          ptr.Of(int64(4)),
					CommitVersion:     ptr.Of("1.0.0"),
					CommitDescription: ptr.Of("desc"),
					LabelKeys:         []string{"label"},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		caseData := tt
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tFields := caseData.fieldsGetter(ctrl)
			app := &PromptManageApplicationImpl{
				manageRepo:       tFields.manageRepo,
				authRPCProvider:  tFields.authProvider,
				auditRPCProvider: tFields.auditProvider,
				promptService:    tFields.promptService,
			}

			resp, err := app.CommitDraft(caseData.args.ctx, caseData.args.request)
			unittest.AssertErrorEqual(t, caseData.wantErr, err)
			if err == nil {
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestPromptManageApplicationImpl_ListCommit(t *testing.T) {
	type fields struct {
		manageRepo    repo.IManageRepo
		authProvider  rpc.IAuthProvider
		promptService service.IPromptService
		userProvider  rpc.IUserProvider
	}
	type args struct {
		ctx     context.Context
		request *manage.ListCommitRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name: "invalid page token",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 5}).Return(&entity.Prompt{ID: 5, SpaceID: 50}, nil)
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(50), []int64{int64(5)}, consts.ActionLoopPromptRead).Return(nil)
				return fields{manageRepo: repoMock, authProvider: auth}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.ListCommitRequest{
					PromptID:  ptr.Of(int64(5)),
					PageToken: ptr.Of("bad"),
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Page token is invalid, page token = bad")),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 5}).Return(&entity.Prompt{ID: 5, SpaceID: 50, PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeNormal}}, nil)
				repoMock.EXPECT().ListCommitInfo(gomock.Any(), repo.ListCommitInfoParam{PromptID: 5, PageSize: 10, Asc: true}).Return(&repo.ListCommitResult{
					CommitInfoDOs: []*entity.CommitInfo{{Version: "1.0.0", CommittedBy: "userA"}},
					CommitDOs:     []*entity.PromptCommit{{CommitInfo: &entity.CommitInfo{Version: "1.0.0"}}},
					NextPageToken: 77,
				}, nil)
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(50), []int64{int64(5)}, consts.ActionLoopPromptRead).Return(nil)
				promptSvc := servicemocks.NewMockIPromptService(ctrl)
				promptSvc.EXPECT().BatchGetCommitLabels(gomock.Any(), int64(5), []string{"1.0.0"}).Return(map[string][]string{"1.0.0": {"label"}}, nil)
				userProvider := mocks.NewMockIUserProvider(ctrl)
				userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"userA"}).Return([]*rpc.UserInfo{{UserID: "userA", UserName: "User A"}}, nil)
				return fields{manageRepo: repoMock, authProvider: auth, promptService: promptSvc, userProvider: userProvider}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.ListCommitRequest{
					PromptID:         ptr.Of(int64(5)),
					PageSize:         ptr.Of(int32(10)),
					Asc:              ptr.Of(true),
					WithCommitDetail: ptr.Of(true),
				},
			},
			wantErr: nil,
		},
		{
			name: "snippet success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 6}).Return(&entity.Prompt{ID: 6, SpaceID: 60, PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeSnippet}}, nil)
				repoMock.EXPECT().ListCommitInfo(gomock.Any(), repo.ListCommitInfoParam{PromptID: 6, PageSize: 5, Asc: false}).Return(&repo.ListCommitResult{
					CommitInfoDOs: []*entity.CommitInfo{{Version: "2.0.0", CommittedBy: "userB"}},
					CommitDOs:     []*entity.PromptCommit{{CommitInfo: &entity.CommitInfo{Version: "2.0.0"}}},
				}, nil)
				repoMock.EXPECT().ListParentPrompt(gomock.Any(), repo.ListParentPromptParam{SubPromptID: 6, SubPromptVersions: []string{"2.0.0"}}).Return(map[string][]*repo.PromptCommitVersions{
					"2.0.0": {{CommitVersions: []string{"3.0.0", "3.1.0"}}},
				}, nil)
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(60), []int64{int64(6)}, consts.ActionLoopPromptRead).Return(nil)
				promptSvc := servicemocks.NewMockIPromptService(ctrl)
				promptSvc.EXPECT().BatchGetCommitLabels(gomock.Any(), int64(6), []string{"2.0.0"}).Return(map[string][]string{"2.0.0": {"labelB"}}, nil)
				userProvider := mocks.NewMockIUserProvider(ctrl)
				userProvider.EXPECT().MGetUserInfo(gomock.Any(), []string{"userB"}).Return([]*rpc.UserInfo{{UserID: "userB"}}, nil)
				return fields{manageRepo: repoMock, authProvider: auth, promptService: promptSvc, userProvider: userProvider}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.ListCommitRequest{
					PromptID: ptr.Of(int64(6)),
					PageSize: ptr.Of(int32(5)),
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		caseData := tt
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tFields := caseData.fieldsGetter(ctrl)
			app := &PromptManageApplicationImpl{
				manageRepo:      tFields.manageRepo,
				authRPCProvider: tFields.authProvider,
				promptService:   tFields.promptService,
				userRPCProvider: tFields.userProvider,
			}

			resp, err := app.ListCommit(caseData.args.ctx, caseData.args.request)
			unittest.AssertErrorEqual(t, caseData.wantErr, err)
			if err == nil {
				assert.NotNil(t, resp)
				assert.Len(t, resp.PromptCommitInfos, 1)
				switch caseData.name {
				case "success":
					assert.Equal(t, ptr.Of("77"), resp.NextPageToken)
					assert.Equal(t, ptr.Of(true), resp.HasMore)
				case "snippet success":
					assert.Nil(t, resp.NextPageToken)
				}
			}
		})
	}
}

func TestPromptManageApplicationImpl_RevertDraftFromCommit(t *testing.T) {
	type fields struct {
		manageRepo    repo.IManageRepo
		authProvider  rpc.IAuthProvider
		promptService service.IPromptService
	}
	type args struct {
		ctx     context.Context
		request *manage.RevertDraftFromCommitRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name: "commit missing",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 6, WithCommit: true, CommitVersion: "1.0.0"}).Return(&entity.Prompt{ID: 6, PromptCommit: nil}, nil)
				return fields{manageRepo: repoMock}
			},
			args: args{
				ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.RevertDraftFromCommitRequest{PromptID: ptr.Of(int64(6)), CommitVersionRevertingFrom: ptr.Of("1.0.0")},
			},
			wantErr: errorx.New("Prompt or commit not found, prompt id = 6, commit version = 1.0.0"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 6, WithCommit: true, CommitVersion: "1.0.0"}).Return(&entity.Prompt{
					ID:      6,
					SpaceID: 60,
					PromptCommit: &entity.PromptCommit{
						CommitInfo:   &entity.CommitInfo{Version: "1.0.0"},
						PromptDetail: &entity.PromptDetail{PromptTemplate: &entity.PromptTemplate{}},
					},
				}, nil)
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().MCheckPromptPermission(gomock.Any(), int64(60), []int64{int64(6)}, consts.ActionLoopPromptEdit).Return(nil)
				promptSvc := servicemocks.NewMockIPromptService(ctrl)
				promptSvc.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).Return(&entity.DraftInfo{}, nil)
				return fields{manageRepo: repoMock, authProvider: auth, promptService: promptSvc}
			},
			args: args{
				ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.RevertDraftFromCommitRequest{PromptID: ptr.Of(int64(6)), CommitVersionRevertingFrom: ptr.Of("1.0.0")},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		caseData := tt
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tFields := caseData.fieldsGetter(ctrl)
			app := &PromptManageApplicationImpl{
				manageRepo:      tFields.manageRepo,
				authRPCProvider: tFields.authProvider,
				promptService:   tFields.promptService,
			}

			resp, err := app.RevertDraftFromCommit(caseData.args.ctx, caseData.args.request)
			unittest.AssertErrorEqual(t, caseData.wantErr, err)
			if err == nil {
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestPromptManageApplicationImpl_listPromptOrderBy(t *testing.T) {
	app := &PromptManageApplicationImpl{}
	tests := []struct {
		name string
		arg  *manage.ListPromptOrderBy
		exp  int
	}{
		{"nil", nil, mysql.ListPromptBasicOrderByCreatedAt},
		{"created", ptr.Of(manage.ListPromptOrderByCreatedAt), mysql.ListPromptBasicOrderByCreatedAt},
		{"committed", ptr.Of(manage.ListPromptOrderByCommitedAt), mysql.ListPromptBasicOrderByLatestCommittedAt},
		{"default", ptr.Of(manage.ListPromptOrderBy("unknown")), mysql.ListPromptBasicOrderByID},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.exp, app.listPromptOrderBy(tt.arg))
	}
}

func TestPromptManageApplicationImpl_ListLabelAdditional(t *testing.T) {
	type fields struct {
		authProvider  rpc.IAuthProvider
		promptService service.IPromptService
	}
	type args struct {
		ctx     context.Context
		request *manage.ListLabelRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name: "mapping without prompt id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckSpacePermission(gomock.Any(), int64(70), consts.ActionWorkspaceListLoopPrompt).Return(nil)
				return fields{authProvider: auth}
			},
			args: args{
				ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.ListLabelRequest{WorkspaceID: ptr.Of(int64(70)), WithPromptVersionMapping: ptr.Of(true)},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("PromptID must be provided when WithPromptVersionMapping is true")),
		},
		{
			name: "invalid page token",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckSpacePermission(gomock.Any(), int64(70), consts.ActionWorkspaceListLoopPrompt).Return(nil)
				return fields{authProvider: auth}
			},
			args: args{
				ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.ListLabelRequest{WorkspaceID: ptr.Of(int64(70)), PageToken: ptr.Of("bad")},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Invalid page token")),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				auth := mocks.NewMockIAuthProvider(ctrl)
				auth.EXPECT().CheckSpacePermission(gomock.Any(), int64(70), consts.ActionWorkspaceListLoopPrompt).Return(nil)
				promptSvc := servicemocks.NewMockIPromptService(ctrl)
				next := int64(88)
				promptSvc.EXPECT().ListLabel(gomock.Any(), service.ListLabelParam{SpaceID: 70, LabelKeyLike: "key", PageSize: 10}).Return([]*entity.PromptLabel{{LabelKey: "label"}}, &next, nil)
				promptSvc.EXPECT().BatchGetLabelMappingPromptVersion(gomock.Any(), []service.PromptLabelQuery{{PromptID: 99, LabelKey: "label"}}).Return(map[service.PromptLabelQuery]string{{PromptID: 99, LabelKey: "label"}: "1.0.0"}, nil)
				return fields{authProvider: auth, promptService: promptSvc}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user"}),
				request: &manage.ListLabelRequest{
					WorkspaceID:              ptr.Of(int64(70)),
					PromptID:                 ptr.Of(int64(99)),
					LabelKeyLike:             ptr.Of("key"),
					PageSize:                 ptr.Of(int32(10)),
					WithPromptVersionMapping: ptr.Of(true),
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		caseData := tt
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tFields := caseData.fieldsGetter(ctrl)
			app := &PromptManageApplicationImpl{
				authRPCProvider: tFields.authProvider,
				promptService:   tFields.promptService,
			}

			resp, err := app.ListLabel(caseData.args.ctx, caseData.args.request)
			unittest.AssertErrorEqual(t, caseData.wantErr, err)
			if err == nil {
				assert.Len(t, resp.Labels, 1)
				assert.Equal(t, ptr.Of("88"), resp.NextPageToken)
				assert.Equal(t, "1.0.0", resp.PromptVersionMapping["label"])
			}
		})
	}
}
