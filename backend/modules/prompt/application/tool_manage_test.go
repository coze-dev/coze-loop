// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/tool"
	toolmanage "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/tool_manage"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity/toolmgmt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

type toolManageFields struct {
	toolRepo        repo.IToolRepo
	authRPCProvider rpc.IAuthProvider
	userRPCProvider rpc.IUserProvider
}

func newToolManageApp(f toolManageFields) *ToolManageApplicationImpl {
	return &ToolManageApplicationImpl{
		toolRepo:        f.toolRepo,
		authRPCProvider: f.authRPCProvider,
		userRPCProvider: f.userRPCProvider,
	}
}

func TestToolManageApplicationImpl_CreateTool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolManageFields
		ctx          context.Context
		request      *toolmanage.CreateToolRequest
		want         *toolmanage.CreateToolResponse
		wantErr      error
	}{
		{
			name:         "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          context.Background(),
			request: &toolmanage.CreateToolRequest{
				WorkspaceID: ptr.Of(int64(1)),
				ToolName:    ptr.Of("test"),
			},
			want:    toolmanage.NewCreateToolResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name:         "workspace id required",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.CreateToolRequest{
				WorkspaceID: ptr.Of(int64(0)),
			},
			want:    toolmanage.NewCreateToolResponse(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required")),
		},
		{
			name: "auth error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceCreateLoopPrompt).Return(errorx.New("auth error"))
				return toolManageFields{authRPCProvider: mockAuth}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.CreateToolRequest{
				WorkspaceID: ptr.Of(int64(100)),
				ToolName:    ptr.Of("test"),
			},
			want:    toolmanage.NewCreateToolResponse(),
			wantErr: errorx.New("auth error"),
		},
		{
			name: "create tool repo error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceCreateLoopPrompt).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().CreateTool(gomock.Any(), gomock.Any()).Return(int64(0), errorx.New("create error"))
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.CreateToolRequest{
				WorkspaceID: ptr.Of(int64(100)),
				ToolName:    ptr.Of("test_tool"),
			},
			want:    toolmanage.NewCreateToolResponse(),
			wantErr: errorx.New("create error"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceCreateLoopPrompt).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().CreateTool(gomock.Any(), gomock.Any()).Return(int64(999), nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.CreateToolRequest{
				WorkspaceID:     ptr.Of(int64(100)),
				ToolName:        ptr.Of("test_tool"),
				ToolDescription: ptr.Of("test desc"),
				DraftDetail:     &tool.ToolDetail{Content: ptr.Of("content")},
			},
			want:    &toolmanage.CreateToolResponse{ToolID: ptr.Of(int64(999))},
			wantErr: nil,
		},
		{
			name: "success with nil draft detail",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceCreateLoopPrompt).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().CreateTool(gomock.Any(), gomock.Any()).Return(int64(1000), nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.CreateToolRequest{
				WorkspaceID: ptr.Of(int64(100)),
				ToolName:    ptr.Of("test_tool"),
			},
			want:    &toolmanage.CreateToolResponse{ToolID: ptr.Of(int64(1000))},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			app := newToolManageApp(tt.fieldsGetter(ctrl))
			got, err := app.CreateTool(tt.ctx, tt.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestToolManageApplicationImpl_GetToolDetail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolManageFields
		ctx          context.Context
		request      *toolmanage.GetToolDetailRequest
		want         *toolmanage.GetToolDetailResponse
		wantErr      error
	}{
		{
			name:         "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          context.Background(),
			request:      &toolmanage.GetToolDetailRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(1))},
			want:         toolmanage.NewGetToolDetailResponse(),
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name:         "tool id required",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request:      &toolmanage.GetToolDetailRequest{ToolID: ptr.Of(int64(0)), WorkspaceID: ptr.Of(int64(1))},
			want:         toolmanage.NewGetToolDetailResponse(),
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Tool ID is required")),
		},
		{
			name:         "workspace id required",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request:      &toolmanage.GetToolDetailRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(0))},
			want:         toolmanage.NewGetToolDetailResponse(),
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required")),
		},
		{
			name: "auth error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(errorx.New("auth error"))
				return toolManageFields{authRPCProvider: mockAuth}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.GetToolDetailRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100))},
			want:    toolmanage.NewGetToolDetailResponse(),
			wantErr: errorx.New("auth error"),
		},
		{
			name: "get tool repo error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(nil, errorx.New("get error"))
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.GetToolDetailRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100))},
			want:    toolmanage.NewGetToolDetailResponse(),
			wantErr: errorx.New("get error"),
		},
		{
			name: "tool not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(nil, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.GetToolDetailRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100))},
			want:    toolmanage.NewGetToolDetailResponse(),
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode),
		},
		{
			name: "workspace mismatch",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 200}, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.GetToolDetailRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100))},
			want:    toolmanage.NewGetToolDetailResponse(),
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), repo.GetToolParam{
					ToolID:     1,
					WithCommit: true,
					WithDraft:  false,
				}).Return(&toolmgmt.Tool{
					ID:      1,
					SpaceID: 100,
					ToolBasic: &toolmgmt.ToolBasic{
						Name: "test_tool",
					},
				}, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.GetToolDetailRequest{
				ToolID:      ptr.Of(int64(1)),
				WorkspaceID: ptr.Of(int64(100)),
				WithCommit:  ptr.Of(true),
			},
			want: &toolmanage.GetToolDetailResponse{
				Tool: &tool.Tool{
					ID:          ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(100)),
					ToolBasic: &tool.ToolBasic{
						Name:                   ptr.Of("test_tool"),
						Description:            ptr.Of(""),
						LatestCommittedVersion: ptr.Of(""),
						CreatedBy:              ptr.Of(""),
						UpdatedBy:              ptr.Of(""),
						CreatedAt:              ptr.Of(int64(-62135596800000)),
						UpdatedAt:              ptr.Of(int64(-62135596800000)),
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			app := newToolManageApp(tt.fieldsGetter(ctrl))
			got, err := app.GetToolDetail(tt.ctx, tt.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestToolManageApplicationImpl_ListTool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolManageFields
		ctx          context.Context
		request      *toolmanage.ListToolRequest
		wantErr      error
	}{
		{
			name:         "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          context.Background(),
			request:      &toolmanage.ListToolRequest{WorkspaceID: ptr.Of(int64(1)), PageNum: ptr.Of(int32(1)), PageSize: ptr.Of(int32(10))},
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name:         "workspace id required",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request:      &toolmanage.ListToolRequest{WorkspaceID: ptr.Of(int64(0)), PageNum: ptr.Of(int32(1)), PageSize: ptr.Of(int32(10))},
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required")),
		},
		{
			name:         "page num invalid",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request:      &toolmanage.ListToolRequest{WorkspaceID: ptr.Of(int64(1)), PageNum: ptr.Of(int32(0)), PageSize: ptr.Of(int32(10))},
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("PageNum or PageSize is invalid")),
		},
		{
			name: "auth error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(errorx.New("auth error"))
				return toolManageFields{authRPCProvider: mockAuth}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.ListToolRequest{WorkspaceID: ptr.Of(int64(100)), PageNum: ptr.Of(int32(1)), PageSize: ptr.Of(int32(10))},
			wantErr: errorx.New("auth error"),
		},
		{
			name: "list tool repo error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().ListTool(gomock.Any(), gomock.Any()).Return(nil, errorx.New("list error"))
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.ListToolRequest{WorkspaceID: ptr.Of(int64(100)), PageNum: ptr.Of(int32(1)), PageSize: ptr.Of(int32(10))},
			wantErr: errorx.New("list error"),
		},
		{
			name: "nil list result",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().ListTool(gomock.Any(), gomock.Any()).Return(nil, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.ListToolRequest{WorkspaceID: ptr.Of(int64(100)), PageNum: ptr.Of(int32(1)), PageSize: ptr.Of(int32(10))},
			wantErr: nil,
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionWorkspaceListLoopPrompt).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().ListTool(gomock.Any(), gomock.Any()).Return(&repo.ListToolResult{
					Total: 1,
					Tools: []*toolmgmt.Tool{
						{
							ID:      1,
							SpaceID: 100,
							ToolBasic: &toolmgmt.ToolBasic{
								Name:      "test_tool",
								CreatedBy: "user1",
								UpdatedBy: "user2",
							},
						},
					},
				}, nil)
				mockUser := mocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return([]*rpc.UserInfo{
					{UserID: "user1", UserName: "User One"},
				}, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo, userRPCProvider: mockUser}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.ListToolRequest{WorkspaceID: ptr.Of(int64(100)), PageNum: ptr.Of(int32(1)), PageSize: ptr.Of(int32(10))},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			app := newToolManageApp(tt.fieldsGetter(ctrl))
			got, err := app.ListTool(tt.ctx, tt.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestToolManageApplicationImpl_SaveToolDetail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolManageFields
		ctx          context.Context
		request      *toolmanage.SaveToolDetailRequest
		wantErr      error
	}{
		{
			name:         "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          context.Background(),
			request:      &toolmanage.SaveToolDetailRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(1)), ToolDetail: &tool.ToolDetail{Content: ptr.Of("c")}},
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name:         "tool id or detail required",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request:      &toolmanage.SaveToolDetailRequest{ToolID: ptr.Of(int64(0)), WorkspaceID: ptr.Of(int64(1))},
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Tool ID or ToolDetail is required")),
		},
		{
			name:         "workspace id required",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request:      &toolmanage.SaveToolDetailRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(0)), ToolDetail: &tool.ToolDetail{Content: ptr.Of("c")}},
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required")),
		},
		{
			name: "auth error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptEdit).Return(errorx.New("auth error"))
				return toolManageFields{authRPCProvider: mockAuth}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.SaveToolDetailRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100)), ToolDetail: &tool.ToolDetail{Content: ptr.Of("c")}},
			wantErr: errorx.New("auth error"),
		},
		{
			name: "tool not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptEdit).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(nil, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.SaveToolDetailRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100)), ToolDetail: &tool.ToolDetail{Content: ptr.Of("c")}},
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode),
		},
		{
			name: "workspace mismatch",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptEdit).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 200}, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.SaveToolDetailRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100)), ToolDetail: &tool.ToolDetail{Content: ptr.Of("c")}},
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode),
		},
		{
			name: "save error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptEdit).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 100}, nil)
				mockRepo.EXPECT().SaveToolDetail(gomock.Any(), gomock.Any()).Return(errorx.New("save error"))
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.SaveToolDetailRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100)), ToolDetail: &tool.ToolDetail{Content: ptr.Of("c")}},
			wantErr: errorx.New("save error"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptEdit).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 100}, nil)
				mockRepo.EXPECT().SaveToolDetail(gomock.Any(), repo.SaveToolDetailParam{
					ToolID:      1,
					BaseVersion: "1.0.0",
					Content:     "new content",
					UpdatedBy:   "user1",
				}).Return(nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.SaveToolDetailRequest{
				ToolID:      ptr.Of(int64(1)),
				WorkspaceID: ptr.Of(int64(100)),
				BaseVersion: ptr.Of("1.0.0"),
				ToolDetail:  &tool.ToolDetail{Content: ptr.Of("new content")},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			app := newToolManageApp(tt.fieldsGetter(ctrl))
			got, err := app.SaveToolDetail(tt.ctx, tt.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestToolManageApplicationImpl_CommitToolDraft(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolManageFields
		ctx          context.Context
		request      *toolmanage.CommitToolDraftRequest
		wantErr      error
	}{
		{
			name:         "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          context.Background(),
			request:      &toolmanage.CommitToolDraftRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(1)), CommitVersion: ptr.Of("1.0.0")},
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name:         "invalid semver",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request:      &toolmanage.CommitToolDraftRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(1)), CommitVersion: ptr.Of("invalid")},
			wantErr:      fmt.Errorf("Invalid Semantic Version"),
		},
		{
			name:         "tool id required",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request:      &toolmanage.CommitToolDraftRequest{ToolID: ptr.Of(int64(0)), WorkspaceID: ptr.Of(int64(1)), CommitVersion: ptr.Of("1.0.0")},
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Tool ID is required")),
		},
		{
			name:         "workspace id required",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request:      &toolmanage.CommitToolDraftRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(0)), CommitVersion: ptr.Of("1.0.0")},
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required")),
		},
		{
			name: "auth error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptEdit).Return(errorx.New("auth error"))
				return toolManageFields{authRPCProvider: mockAuth}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.CommitToolDraftRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100)), CommitVersion: ptr.Of("1.0.0")},
			wantErr: errorx.New("auth error"),
		},
		{
			name: "tool not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptEdit).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(nil, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.CommitToolDraftRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100)), CommitVersion: ptr.Of("1.0.0")},
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode),
		},
		{
			name: "workspace mismatch",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptEdit).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 200}, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.CommitToolDraftRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100)), CommitVersion: ptr.Of("1.0.0")},
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode),
		},
		{
			name: "commit error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptEdit).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 100}, nil)
				mockRepo.EXPECT().CommitToolDraft(gomock.Any(), gomock.Any()).Return(errorx.New("commit error"))
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.CommitToolDraftRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100)), CommitVersion: ptr.Of("1.0.0")},
			wantErr: errorx.New("commit error"),
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptEdit).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 100}, nil)
				mockRepo.EXPECT().CommitToolDraft(gomock.Any(), repo.CommitToolDraftParam{
					ToolID:            1,
					CommitVersion:     "1.0.0",
					CommitDescription: "initial",
					BaseVersion:       "0.9.0",
					CommittedBy:       "user1",
				}).Return(nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.CommitToolDraftRequest{
				ToolID:            ptr.Of(int64(1)),
				WorkspaceID:       ptr.Of(int64(100)),
				CommitVersion:     ptr.Of("1.0.0"),
				CommitDescription: ptr.Of("initial"),
				BaseVersion:       ptr.Of("0.9.0"),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			app := newToolManageApp(tt.fieldsGetter(ctrl))
			got, err := app.CommitToolDraft(tt.ctx, tt.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestToolManageApplicationImpl_ListToolCommit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolManageFields
		ctx          context.Context
		request      *toolmanage.ListToolCommitRequest
		wantErr      error
	}{
		{
			name:         "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          context.Background(),
			request:      &toolmanage.ListToolCommitRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(1))},
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name:         "tool id required",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request:      &toolmanage.ListToolCommitRequest{ToolID: ptr.Of(int64(0)), WorkspaceID: ptr.Of(int64(1))},
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Tool ID is required")),
		},
		{
			name:         "workspace id required",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request:      &toolmanage.ListToolCommitRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(0))},
			wantErr:      errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required")),
		},
		{
			name: "auth error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(errorx.New("auth error"))
				return toolManageFields{authRPCProvider: mockAuth}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.ListToolCommitRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100))},
			wantErr: errorx.New("auth error"),
		},
		{
			name: "tool not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(nil, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.ListToolCommitRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100))},
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode),
		},
		{
			name: "workspace mismatch",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 200}, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx:     session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.ListToolCommitRequest{ToolID: ptr.Of(int64(1)), WorkspaceID: ptr.Of(int64(100))},
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode),
		},
		{
			name: "invalid page token",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 100}, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.ListToolCommitRequest{
				ToolID:      ptr.Of(int64(1)),
				WorkspaceID: ptr.Of(int64(100)),
				PageToken:   ptr.Of("not_a_number"),
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode),
		},
		{
			name: "nil list result",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 100}, nil)
				mockRepo.EXPECT().ListToolCommit(gomock.Any(), gomock.Any()).Return(nil, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.ListToolCommitRequest{
				ToolID:      ptr.Of(int64(1)),
				WorkspaceID: ptr.Of(int64(100)),
				PageSize:    ptr.Of(int32(10)),
			},
			wantErr: nil,
		},
		{
			name: "success with page token and has more",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 100}, nil)
				mockRepo.EXPECT().ListToolCommit(gomock.Any(), gomock.Any()).Return(&repo.ListToolCommitResult{
					CommitInfos: []*toolmgmt.CommitInfo{
						{Version: "1.0.0", CommittedBy: "user1"},
					},
					NextPageToken: 12345,
				}, nil)
				mockUser := mocks.NewMockIUserProvider(ctrl)
				mockUser.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return([]*rpc.UserInfo{
					{UserID: "user1", UserName: "User One"},
				}, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo, userRPCProvider: mockUser}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.ListToolCommitRequest{
				ToolID:      ptr.Of(int64(1)),
				WorkspaceID: ptr.Of(int64(100)),
				PageSize:    ptr.Of(int32(10)),
				PageToken:   ptr.Of("100"),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			app := newToolManageApp(tt.fieldsGetter(ctrl))
			got, err := app.ListToolCommit(tt.ctx, tt.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestToolManageApplicationImpl_BatchGetTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) toolManageFields
		ctx          context.Context
		request      *toolmanage.BatchGetToolsRequest
		wantErr      error
	}{
		{
			name:         "user not found",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          context.Background(),
			request: &toolmanage.BatchGetToolsRequest{
				WorkspaceID: ptr.Of(int64(1)),
				Queries:     []*toolmanage.ToolQuery{{ToolID: ptr.Of(int64(1))}},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("User not found")),
		},
		{
			name:         "workspace id required",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.BatchGetToolsRequest{
				WorkspaceID: ptr.Of(int64(0)),
				Queries:     []*toolmanage.ToolQuery{{ToolID: ptr.Of(int64(1))}},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Workspace ID is required")),
		},
		{
			name:         "queries required",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields { return toolManageFields{} },
			ctx:          session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.BatchGetToolsRequest{
				WorkspaceID: ptr.Of(int64(1)),
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("Queries is required")),
		},
		{
			name: "auth error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(errorx.New("auth error"))
				return toolManageFields{authRPCProvider: mockAuth}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.BatchGetToolsRequest{
				WorkspaceID: ptr.Of(int64(100)),
				Queries:     []*toolmanage.ToolQuery{{ToolID: ptr.Of(int64(1))}},
			},
			wantErr: errorx.New("auth error"),
		},
		{
			name: "batch get error",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().BatchGetTools(gomock.Any(), gomock.Any()).Return(nil, errorx.New("batch error"))
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.BatchGetToolsRequest{
				WorkspaceID: ptr.Of(int64(100)),
				Queries:     []*toolmanage.ToolQuery{{ToolID: ptr.Of(int64(1)), Version: ptr.Of("1.0.0")}},
			},
			wantErr: errorx.New("batch error"),
		},
		{
			name: "success with nil query filtered",
			fieldsGetter: func(ctrl *gomock.Controller) toolManageFields {
				mockAuth := mocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), consts.ActionLoopPromptRead).Return(nil)
				mockRepo := repomocks.NewMockIToolRepo(ctrl)
				mockRepo.EXPECT().BatchGetTools(gomock.Any(), gomock.Any()).Return([]*repo.BatchGetToolsResult{
					{
						Query: repo.BatchGetToolsQuery{ToolID: 1, Version: "1.0.0"},
						Tool: &toolmgmt.Tool{
							ID:        1,
							SpaceID:   100,
							ToolBasic: &toolmgmt.ToolBasic{Name: "tool1"},
						},
					},
				}, nil)
				return toolManageFields{authRPCProvider: mockAuth, toolRepo: mockRepo}
			},
			ctx: session.WithCtxUser(context.Background(), &session.User{ID: "user1"}),
			request: &toolmanage.BatchGetToolsRequest{
				WorkspaceID: ptr.Of(int64(100)),
				Queries: []*toolmanage.ToolQuery{
					{ToolID: ptr.Of(int64(1)), Version: ptr.Of("1.0.0")},
					nil,
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			app := newToolManageApp(tt.fieldsGetter(ctrl))
			got, err := app.BatchGetTools(tt.ctx, tt.request)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestToolManageApplicationImpl_listToolOrderBy(t *testing.T) {
	t.Parallel()

	app := &ToolManageApplicationImpl{}
	committedAt := toolmanage.ListToolOrderByCommittedAt
	createdAt := toolmanage.ListToolOrderByCreatedAt
	unknown := toolmanage.ListToolOrderBy("unknown")

	tests := []struct {
		name    string
		orderBy *toolmanage.ListToolOrderBy
		want    int
	}{
		{
			name:    "nil order by",
			orderBy: nil,
			want:    mysql.ListToolBasicOrderByID,
		},
		{
			name:    "committed_at",
			orderBy: &committedAt,
			want:    mysql.ListToolBasicOrderByCommittedAt,
		},
		{
			name:    "created_at",
			orderBy: &createdAt,
			want:    mysql.ListToolBasicOrderByCreatedAt,
		},
		{
			name:    "unknown defaults to id",
			orderBy: &unknown,
			want:    mysql.ListToolBasicOrderByID,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, app.listToolOrderBy(tt.orderBy))
		})
	}
}
