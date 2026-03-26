// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	tool_manage "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/tool_manage"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service/mocks"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

// toolTestFields 用于测试 ToolManageApplicationImpl 的依赖字段
type toolTestFields struct {
	toolRepo        repo.IToolRepo
	toolService     service.IToolService
	authRPCProvider rpc.IAuthProvider
	userRPCProvider rpc.IUserProvider
}

func TestToolManageApplicationImpl_CreateTool(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *tool_manage.CreateToolRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolTestFields
		args         args
		want         *tool_manage.CreateToolResponse
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				return toolTestFields{}
			},
			args: args{
				ctx: context.Background(),
				request: &tool_manage.CreateToolRequest{
					WorkspaceID:     ptr.Of(int64(100)),
					ToolName:        ptr.Of("test_tool"),
					ToolDescription: ptr.Of("test description"),
				},
			},
			want:    tool_manage.NewCreateToolResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

				mockToolService := servicemocks.NewMockIToolService(ctrl)
				mockToolService.EXPECT().CreateTool(gomock.Any(), gomock.Any()).Return(int64(1001), nil)

				return toolTestFields{
					toolService:     mockToolService,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "test_user"}),
				request: &tool_manage.CreateToolRequest{
					WorkspaceID:     ptr.Of(int64(100)),
					ToolName:        ptr.Of("test_tool"),
					ToolDescription: ptr.Of("test description"),
				},
			},
			want: &tool_manage.CreateToolResponse{
				ToolID: ptr.Of(int64(1001)),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)

			app := &ToolManageApplicationImpl{
				toolRepo:        f.toolRepo,
				toolService:     f.toolService,
				authRPCProvider: f.authRPCProvider,
				userRPCProvider: f.userRPCProvider,
			}

			got, err := app.CreateTool(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestToolManageApplicationImpl_GetToolDetail(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *tool_manage.GetToolDetailRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolTestFields
		args         args
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				return toolTestFields{}
			},
			args: args{
				ctx: context.Background(),
				request: &tool_manage.GetToolDetailRequest{
					ToolID:      ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					WithCommit:  ptr.Of(true),
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), repo.GetToolParam{
					ToolID:     int64(1),
					SpaceID:    int64(100),
					WithCommit: true,
				}).Return(&entity.CommonTool{
					ID:      1,
					SpaceID: 100,
					ToolBasic: &entity.CommonToolBasic{
						Name:        "test_tool",
						Description: "test description",
					},
				}, nil)

				return toolTestFields{
					toolRepo:        mockRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "test_user"}),
				request: &tool_manage.GetToolDetailRequest{
					ToolID:      ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					WithCommit:  ptr.Of(true),
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
			f := tt.fieldsGetter(ctrl)

			app := &ToolManageApplicationImpl{
				toolRepo:        f.toolRepo,
				toolService:     f.toolService,
				authRPCProvider: f.authRPCProvider,
				userRPCProvider: f.userRPCProvider,
			}

			got, err := app.GetToolDetail(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
				assert.NotNil(t, got.Tool)
			}
		})
	}
}

func TestToolManageApplicationImpl_ListTool(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *tool_manage.ListToolRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolTestFields
		args         args
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				return toolTestFields{}
			},
			args: args{
				ctx: context.Background(),
				request: &tool_manage.ListToolRequest{
					WorkspaceID: ptr.Of(int64(100)),
					PageNum:     ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().ListTool(gomock.Any(), gomock.Any()).Return(&repo.ListToolResult{
					Total: 1,
					ToolDOs: []*entity.CommonTool{
						{
							ID:      1,
							SpaceID: 100,
							ToolBasic: &entity.CommonToolBasic{
								Name:      "test_tool",
								CreatedBy: "test_user",
								UpdatedBy: "test_user",
							},
						},
					},
				}, nil)

				mockUserRPC := mocks.NewMockIUserProvider(ctrl)
				mockUserRPC.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return([]*rpc.UserInfo{
					{
						UserID:   "test_user",
						UserName: "Test User",
					},
				}, nil)

				return toolTestFields{
					toolRepo:        mockRepo,
					authRPCProvider: mockAuth,
					userRPCProvider: mockUserRPC,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "test_user"}),
				request: &tool_manage.ListToolRequest{
					WorkspaceID: ptr.Of(int64(100)),
					PageNum:     ptr.Of(int32(1)),
					PageSize:    ptr.Of(int32(10)),
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
			f := tt.fieldsGetter(ctrl)

			app := &ToolManageApplicationImpl{
				toolRepo:        f.toolRepo,
				toolService:     f.toolService,
				authRPCProvider: f.authRPCProvider,
				userRPCProvider: f.userRPCProvider,
			}

			got, err := app.ListTool(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
				assert.Equal(t, int32(1), got.GetTotal())
				assert.Len(t, got.GetTools(), 1)
				assert.NotNil(t, got.GetUsers())
			}
		})
	}
}

func TestToolManageApplicationImpl_SaveToolDetail(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *tool_manage.SaveToolDetailRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolTestFields
		args         args
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				return toolTestFields{}
			},
			args: args{
				ctx: context.Background(),
				request: &tool_manage.SaveToolDetailRequest{
					ToolID:      ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					BaseVersion: ptr.Of("1.0.0"),
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().SaveDraft(gomock.Any(), gomock.Any()).Return(nil)

				return toolTestFields{
					toolRepo:        mockRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "test_user"}),
				request: &tool_manage.SaveToolDetailRequest{
					ToolID:      ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					BaseVersion: ptr.Of("1.0.0"),
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
			f := tt.fieldsGetter(ctrl)

			app := &ToolManageApplicationImpl{
				toolRepo:        f.toolRepo,
				toolService:     f.toolService,
				authRPCProvider: f.authRPCProvider,
				userRPCProvider: f.userRPCProvider,
			}

			got, err := app.SaveToolDetail(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestToolManageApplicationImpl_CommitToolDraft(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *tool_manage.CommitToolDraftRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolTestFields
		args         args
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				return toolTestFields{}
			},
			args: args{
				ctx: context.Background(),
				request: &tool_manage.CommitToolDraftRequest{
					ToolID:            ptr.Of(int64(1)),
					WorkspaceID:       ptr.Of(int64(100)),
					CommitVersion:     ptr.Of("1.0.0"),
					CommitDescription: ptr.Of("first commit"),
					BaseVersion:       ptr.Of("$PublicDraft"),
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().CommitDraft(gomock.Any(), repo.CommitToolDraftParam{
					ToolID:            int64(1),
					SpaceID:           int64(100),
					CommitVersion:     "1.0.0",
					CommitDescription: "first commit",
					BaseVersion:       "$PublicDraft",
					CommittedBy:       "test_user",
				}).Return(nil)

				return toolTestFields{
					toolRepo:        mockRepo,
					authRPCProvider: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "test_user"}),
				request: &tool_manage.CommitToolDraftRequest{
					ToolID:            ptr.Of(int64(1)),
					WorkspaceID:       ptr.Of(int64(100)),
					CommitVersion:     ptr.Of("1.0.0"),
					CommitDescription: ptr.Of("first commit"),
					BaseVersion:       ptr.Of("$PublicDraft"),
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
			f := tt.fieldsGetter(ctrl)

			app := &ToolManageApplicationImpl{
				toolRepo:        f.toolRepo,
				toolService:     f.toolService,
				authRPCProvider: f.authRPCProvider,
				userRPCProvider: f.userRPCProvider,
			}

			got, err := app.CommitToolDraft(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestToolManageApplicationImpl_ListToolCommit(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *tool_manage.ListToolCommitRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolTestFields
		args         args
		wantErr      error
	}{
		{
			name: "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				return toolTestFields{}
			},
			args: args{
				ctx: context.Background(),
				request: &tool_manage.ListToolCommitRequest{
					ToolID:      ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().ListToolCommitInfo(gomock.Any(), gomock.Any()).Return(&repo.ListToolCommitResult{
					CommitInfoDOs: []*entity.CommonToolCommitInfo{
						{
							Version:     "1.0.0",
							CommittedBy: "test_user",
						},
					},
					CommitDetailMapping: map[string]*entity.CommonToolDetail{},
					HasMore:             false,
				}, nil)

				mockUserRPC := mocks.NewMockIUserProvider(ctrl)
				mockUserRPC.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return([]*rpc.UserInfo{
					{
						UserID:   "test_user",
						UserName: "Test User",
					},
				}, nil)

				return toolTestFields{
					toolRepo:        mockRepo,
					authRPCProvider: mockAuth,
					userRPCProvider: mockUserRPC,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "test_user"}),
				request: &tool_manage.ListToolCommitRequest{
					ToolID:      ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					PageSize:    ptr.Of(int32(10)),
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
			f := tt.fieldsGetter(ctrl)

			app := &ToolManageApplicationImpl{
				toolRepo:        f.toolRepo,
				toolService:     f.toolService,
				authRPCProvider: f.authRPCProvider,
				userRPCProvider: f.userRPCProvider,
			}

			got, err := app.ListToolCommit(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
				assert.NotNil(t, got.ToolCommitInfos)
				assert.False(t, got.GetHasMore())
			}
		})
	}
}

func TestToolManageApplicationImpl_BatchGetTools(t *testing.T) {
	type args struct {
		ctx     context.Context
		request *tool_manage.BatchGetToolsRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolTestFields
		args         args
		wantErr      error
	}{
		{
			name: "empty queries",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().MGetTool(gomock.Any(), gomock.Any()).Return(
					map[repo.MGetToolQuery]*entity.CommonTool{}, nil,
				)

				return toolTestFields{
					toolRepo: mockRepo,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "test_user"}),
				request: &tool_manage.BatchGetToolsRequest{
					WorkspaceID: ptr.Of(int64(100)),
					Queries:     []*tool_manage.ToolQuery{},
				},
			},
			wantErr: nil,
		},
		{
			name: "success with queries",
			fieldsGetter: func(ctrl *gomock.Controller) toolTestFields {
				query := repo.MGetToolQuery{
					ToolID:  int64(1),
					Version: "1.0.0",
				}
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().MGetTool(gomock.Any(), gomock.Any()).Return(
					map[repo.MGetToolQuery]*entity.CommonTool{
						query: {
							ID:      1,
							SpaceID: 100,
							ToolBasic: &entity.CommonToolBasic{
								Name: "test_tool",
							},
						},
					}, nil,
				)

				return toolTestFields{
					toolRepo: mockRepo,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "test_user"}),
				request: &tool_manage.BatchGetToolsRequest{
					WorkspaceID: ptr.Of(int64(100)),
					Queries: []*tool_manage.ToolQuery{
						{
							ToolID:  ptr.Of(int64(1)),
							Version: ptr.Of("1.0.0"),
						},
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
			f := tt.fieldsGetter(ctrl)

			app := &ToolManageApplicationImpl{
				toolRepo:        f.toolRepo,
				toolService:     f.toolService,
				authRPCProvider: f.authRPCProvider,
				userRPCProvider: f.userRPCProvider,
			}

			got, err := app.BatchGetTools(tt.args.ctx, tt.args.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
			}
		})
	}
}
