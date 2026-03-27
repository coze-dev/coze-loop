package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	toolDTO "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/tool"
	toolmanage "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/tool_manage"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity/toolmgmt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func ctxWithUser(userID string) context.Context {
	return session.WithCtxUser(context.Background(), &session.User{ID: userID})
}

// --- CreateTool unit tests ---

func TestCreateTool_UserNotFound(t *testing.T) {
	t.Parallel()

	app := &ToolManageApplicationImpl{}
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{name: "no user in context", ctx: context.Background()},
		{name: "empty user id", ctx: ctxWithUser("")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := &toolmanage.CreateToolRequest{
				WorkspaceID: ptr.Of(int64(100)),
				ToolName:    ptr.Of("example-tool"),
			}
			_, err := app.CreateTool(tt.ctx, req)
			unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.CommonInvalidParamCode), err)
		})
	}
}

func TestCreateTool_InvalidWorkspaceID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		workspaceID int64
	}{
		{name: "workspace_id is zero", workspaceID: 0},
		{name: "workspace_id is negative", workspaceID: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := &ToolManageApplicationImpl{}
			req := &toolmanage.CreateToolRequest{
				WorkspaceID: ptr.Of(tt.workspaceID),
				ToolName:    ptr.Of("example-tool"),
			}
			_, err := app.CreateTool(ctxWithUser("user-1"), req)
			unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.CommonInvalidParamCode), err)
		})
	}
}

func TestCreateTool_PermissionCheckError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(errorx.New("permission denied"))

	app := &ToolManageApplicationImpl{authRPCProvider: mockAuth}
	req := &toolmanage.CreateToolRequest{
		WorkspaceID: ptr.Of(int64(100)),
		ToolName:    ptr.Of("example-tool"),
	}
	_, err := app.CreateTool(ctxWithUser("user-1"), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestCreateTool_DraftDetailNil(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().CreateTool(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, tool *toolmgmt.Tool) (int64, error) {
			assert.Empty(t, tool.ToolCommit.ToolDetail.Content, "content should be empty when DraftDetail is nil")
			return 12345, nil
		},
	)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.CreateToolRequest{
		WorkspaceID:     ptr.Of(int64(100)),
		ToolName:        ptr.Of("example-tool"),
		ToolDescription: ptr.Of("desc"),
		DraftDetail:     nil,
	}
	resp, err := app.CreateTool(ctxWithUser("user-1"), req)
	assert.NoError(t, err)
	assert.Equal(t, int64(12345), resp.GetToolID())
}

func TestCreateTool_DraftDetailWithContent(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().CreateTool(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, tool *toolmgmt.Tool) (int64, error) {
			assert.Equal(t, "some content", tool.ToolCommit.ToolDetail.Content)
			assert.Equal(t, "user-1", tool.ToolBasic.CreatedBy)
			assert.Equal(t, "user-1", tool.ToolBasic.UpdatedBy)
			assert.Equal(t, toolmgmt.PublicDraftVersion, tool.ToolCommit.CommitInfo.Version)
			return 99, nil
		},
	)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.CreateToolRequest{
		WorkspaceID:     ptr.Of(int64(100)),
		ToolName:        ptr.Of("example-tool"),
		ToolDescription: ptr.Of("desc"),
		DraftDetail:     &toolDTO.ToolDetail{Content: ptr.Of("some content")},
	}
	resp, err := app.CreateTool(ctxWithUser("user-1"), req)
	assert.NoError(t, err)
	assert.Equal(t, int64(99), resp.GetToolID())
}

func TestCreateTool_RepoError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().CreateTool(gomock.Any(), gomock.Any()).Return(int64(0), errorx.New("db error"))

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.CreateToolRequest{
		WorkspaceID: ptr.Of(int64(100)),
		ToolName:    ptr.Of("example-tool"),
	}
	_, err := app.CreateTool(ctxWithUser("user-1"), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// --- GetToolDetail unit tests ---

func TestGetToolDetail_UserNotFound(t *testing.T) {
	t.Parallel()
	app := &ToolManageApplicationImpl{}
	req := &toolmanage.GetToolDetailRequest{
		ToolID:      ptr.Of(int64(1)),
		WorkspaceID: ptr.Of(int64(100)),
	}
	_, err := app.GetToolDetail(context.Background(), req)
	unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.CommonInvalidParamCode), err)
}

func TestGetToolDetail_InvalidToolID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		toolID int64
	}{
		{name: "tool_id is zero", toolID: 0},
		{name: "tool_id is negative", toolID: -5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := &ToolManageApplicationImpl{}
			req := &toolmanage.GetToolDetailRequest{
				ToolID:      ptr.Of(tt.toolID),
				WorkspaceID: ptr.Of(int64(100)),
			}
			_, err := app.GetToolDetail(ctxWithUser("user-1"), req)
			unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.CommonInvalidParamCode), err)
		})
	}
}

func TestGetToolDetail_InvalidWorkspaceID(t *testing.T) {
	t.Parallel()
	app := &ToolManageApplicationImpl{}
	req := &toolmanage.GetToolDetailRequest{
		ToolID:      ptr.Of(int64(1)),
		WorkspaceID: ptr.Of(int64(0)),
	}
	_, err := app.GetToolDetail(ctxWithUser("user-1"), req)
	unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.CommonInvalidParamCode), err)
}

func TestGetToolDetail_ToolNotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(nil, nil)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.GetToolDetailRequest{
		ToolID:      ptr.Of(int64(999)),
		WorkspaceID: ptr.Of(int64(100)),
	}
	_, err := app.GetToolDetail(ctxWithUser("user-1"), req)
	unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.ResourceNotFoundCode), err)
}

func TestGetToolDetail_SpaceIDMismatch(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{
		ID:      1,
		SpaceID: 200, // different from requested workspace_id=100
	}, nil)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.GetToolDetailRequest{
		ToolID:      ptr.Of(int64(1)),
		WorkspaceID: ptr.Of(int64(100)),
	}
	_, err := app.GetToolDetail(ctxWithUser("user-1"), req)
	unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.ResourceNotFoundCode), err)
}

// --- ListTool unit tests ---

func TestListTool_InvalidPagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pageNum  int32
		pageSize int32
	}{
		{name: "page_num is zero", pageNum: 0, pageSize: 10},
		{name: "page_size is zero", pageNum: 1, pageSize: 0},
		{name: "page_num is negative", pageNum: -1, pageSize: 10},
		{name: "page_size is negative", pageNum: 1, pageSize: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := &ToolManageApplicationImpl{}
			req := &toolmanage.ListToolRequest{
				WorkspaceID: ptr.Of(int64(100)),
				PageNum:     ptr.Of(tt.pageNum),
				PageSize:    ptr.Of(tt.pageSize),
			}
			_, err := app.ListTool(ctxWithUser("user-1"), req)
			unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.CommonInvalidParamCode), err)
		})
	}
}

func TestListTool_NilListResult(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().ListTool(gomock.Any(), gomock.Any()).Return(nil, nil)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.ListToolRequest{
		WorkspaceID: ptr.Of(int64(100)),
		PageNum:     ptr.Of(int32(1)),
		PageSize:    ptr.Of(int32(10)),
	}
	resp, err := app.ListTool(ctxWithUser("user-1"), req)
	assert.NoError(t, err)
	assert.Nil(t, resp.Tools)
}

func TestListTool_UserInfoFetchError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().ListTool(gomock.Any(), gomock.Any()).Return(&repo.ListToolResult{
		Total: 1,
		Tools: []*toolmgmt.Tool{
			{
				ID:      1,
				SpaceID: 100,
				ToolBasic: &toolmgmt.ToolBasic{
					Name:      "example-tool",
					CreatedBy: "user-1",
					UpdatedBy: "user-2",
				},
			},
		},
	}, nil)

	mockUser := mocks.NewMockIUserProvider(ctrl)
	mockUser.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return(nil, errorx.New("user rpc error"))

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
		userRPCProvider: mockUser,
	}
	req := &toolmanage.ListToolRequest{
		WorkspaceID: ptr.Of(int64(100)),
		PageNum:     ptr.Of(int32(1)),
		PageSize:    ptr.Of(int32(10)),
	}
	_, err := app.ListTool(ctxWithUser("user-1"), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user rpc error")
}

// --- listToolOrderBy unit tests ---

func TestListToolOrderBy(t *testing.T) {
	t.Parallel()

	app := &ToolManageApplicationImpl{}

	tests := []struct {
		name     string
		orderBy  *toolmanage.ListToolOrderBy
		expected int
	}{
		{name: "nil orderBy defaults to ID", orderBy: nil, expected: 1},
		{name: "committed_at", orderBy: ptr.Of(toolmanage.ListToolOrderByCommittedAt), expected: 4},
		{name: "created_at", orderBy: ptr.Of(toolmanage.ListToolOrderByCreatedAt), expected: 2},
		{name: "unknown value defaults to ID", orderBy: ptr.Of(toolmanage.ListToolOrderBy("unknown_order")), expected: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := app.listToolOrderBy(tt.orderBy)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- SaveToolDetail unit tests ---

func TestSaveToolDetail_InvalidParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *toolmanage.SaveToolDetailRequest
	}{
		{
			name: "tool_id is zero",
			req: &toolmanage.SaveToolDetailRequest{
				ToolID:      ptr.Of(int64(0)),
				WorkspaceID: ptr.Of(int64(100)),
				ToolDetail:  &toolDTO.ToolDetail{Content: ptr.Of("content")},
			},
		},
		{
			name: "tool_detail is nil",
			req: &toolmanage.SaveToolDetailRequest{
				ToolID:      ptr.Of(int64(1)),
				WorkspaceID: ptr.Of(int64(100)),
				ToolDetail:  nil,
			},
		},
		{
			name: "workspace_id is zero",
			req: &toolmanage.SaveToolDetailRequest{
				ToolID:      ptr.Of(int64(1)),
				WorkspaceID: ptr.Of(int64(0)),
				ToolDetail:  &toolDTO.ToolDetail{Content: ptr.Of("content")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := &ToolManageApplicationImpl{}
			_, err := app.SaveToolDetail(ctxWithUser("user-1"), tt.req)
			unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.CommonInvalidParamCode), err)
		})
	}
}

func TestSaveToolDetail_ToolNotFound(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(nil, nil)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.SaveToolDetailRequest{
		ToolID:      ptr.Of(int64(999)),
		WorkspaceID: ptr.Of(int64(100)),
		ToolDetail:  &toolDTO.ToolDetail{Content: ptr.Of("content")},
	}
	_, err := app.SaveToolDetail(ctxWithUser("user-1"), req)
	unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.ResourceNotFoundCode), err)
}

func TestSaveToolDetail_SpaceIDMismatch(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{
		ID:      999,
		SpaceID: 200,
	}, nil)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.SaveToolDetailRequest{
		ToolID:      ptr.Of(int64(999)),
		WorkspaceID: ptr.Of(int64(100)),
		ToolDetail:  &toolDTO.ToolDetail{Content: ptr.Of("content")},
	}
	_, err := app.SaveToolDetail(ctxWithUser("user-1"), req)
	unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.ResourceNotFoundCode), err)
}

// --- CommitToolDraft unit tests ---

func TestCommitToolDraft_InvalidSemver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
	}{
		{name: "empty version", version: ""},
		{name: "invalid format", version: "not-a-version"},
		{name: "missing patch", version: "1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := &ToolManageApplicationImpl{}
			req := &toolmanage.CommitToolDraftRequest{
				ToolID:        ptr.Of(int64(1)),
				WorkspaceID:   ptr.Of(int64(100)),
				CommitVersion: ptr.Of(tt.version),
			}
			_, err := app.CommitToolDraft(ctxWithUser("user-1"), req)
			assert.Error(t, err)
		})
	}
}

func TestCommitToolDraft_InvalidToolID(t *testing.T) {
	t.Parallel()
	app := &ToolManageApplicationImpl{}
	req := &toolmanage.CommitToolDraftRequest{
		ToolID:        ptr.Of(int64(0)),
		WorkspaceID:   ptr.Of(int64(100)),
		CommitVersion: ptr.Of("1.0.0"),
	}
	_, err := app.CommitToolDraft(ctxWithUser("user-1"), req)
	unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.CommonInvalidParamCode), err)
}

func TestCommitToolDraft_InvalidWorkspaceID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	// No expectation needed since validation should fail before permission check

	app := &ToolManageApplicationImpl{authRPCProvider: mockAuth}
	req := &toolmanage.CommitToolDraftRequest{
		ToolID:        ptr.Of(int64(1)),
		WorkspaceID:   ptr.Of(int64(0)),
		CommitVersion: ptr.Of("1.0.0"),
	}
	_, err := app.CommitToolDraft(ctxWithUser("user-1"), req)
	unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.CommonInvalidParamCode), err)
}

func TestCommitToolDraft_SpaceIDMismatch(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{
		ID:      1,
		SpaceID: 200,
	}, nil)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.CommitToolDraftRequest{
		ToolID:        ptr.Of(int64(1)),
		WorkspaceID:   ptr.Of(int64(100)),
		CommitVersion: ptr.Of("1.0.0"),
	}
	_, err := app.CommitToolDraft(ctxWithUser("user-1"), req)
	unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.ResourceNotFoundCode), err)
}

// --- ListToolCommit unit tests ---

func TestListToolCommit_InvalidPageToken(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{
		ID:      1,
		SpaceID: 100,
	}, nil)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.ListToolCommitRequest{
		ToolID:      ptr.Of(int64(1)),
		WorkspaceID: ptr.Of(int64(100)),
		PageSize:    ptr.Of(int32(10)),
		PageToken:   ptr.Of("not-a-number"),
	}
	_, err := app.ListToolCommit(ctxWithUser("user-1"), req)
	unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.CommonInvalidParamCode), err)
}

func TestListToolCommit_NilListResult(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 100}, nil)
	mockToolRepo.EXPECT().ListToolCommit(gomock.Any(), gomock.Any()).Return(nil, nil)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.ListToolCommitRequest{
		ToolID:      ptr.Of(int64(1)),
		WorkspaceID: ptr.Of(int64(100)),
		PageSize:    ptr.Of(int32(10)),
	}
	resp, err := app.ListToolCommit(ctxWithUser("user-1"), req)
	assert.NoError(t, err)
	assert.Nil(t, resp.ToolCommitInfos)
}

func TestListToolCommit_WithCommitDetailAndNextPage(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 100}, nil)
	mockToolRepo.EXPECT().ListToolCommit(gomock.Any(), gomock.Any()).Return(&repo.ListToolCommitResult{
		CommitInfos: []*toolmgmt.CommitInfo{
			{Version: "1.0.0", CommittedBy: "user-1"},
		},
		CommitDetails: map[string]*toolmgmt.ToolDetail{
			"1.0.0": {Content: "detail-content"},
		},
		NextPageToken: 12345,
	}, nil)

	mockUser := mocks.NewMockIUserProvider(ctrl)
	mockUser.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return([]*rpc.UserInfo{
		{UserID: "user-1", UserName: "Demo User"},
	}, nil)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
		userRPCProvider: mockUser,
	}
	req := &toolmanage.ListToolCommitRequest{
		ToolID:           ptr.Of(int64(1)),
		WorkspaceID:      ptr.Of(int64(100)),
		PageSize:         ptr.Of(int32(10)),
		WithCommitDetail: ptr.Of(true),
	}
	resp, err := app.ListToolCommit(ctxWithUser("user-1"), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp.NextPageToken)
	assert.Equal(t, "12345", resp.GetNextPageToken())
	assert.True(t, resp.GetHasMore())
	assert.Len(t, resp.ToolCommitInfos, 1)
	assert.NotNil(t, resp.ToolCommitDetailMapping)
	assert.NotNil(t, resp.Users)
}

func TestListToolCommit_NilCommitInfoSkipped(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 100}, nil)
	mockToolRepo.EXPECT().ListToolCommit(gomock.Any(), gomock.Any()).Return(&repo.ListToolCommitResult{
		CommitInfos: []*toolmgmt.CommitInfo{
			nil,
			{Version: "1.0.0", CommittedBy: "user-1"},
			nil,
		},
		NextPageToken: 0,
	}, nil)

	mockUser := mocks.NewMockIUserProvider(ctrl)
	mockUser.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return(nil, nil)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
		userRPCProvider: mockUser,
	}
	req := &toolmanage.ListToolCommitRequest{
		ToolID:      ptr.Of(int64(1)),
		WorkspaceID: ptr.Of(int64(100)),
		PageSize:    ptr.Of(int32(10)),
	}
	resp, err := app.ListToolCommit(ctxWithUser("user-1"), req)
	assert.NoError(t, err)
	assert.Len(t, resp.ToolCommitInfos, 1)
	assert.Nil(t, resp.HasMore)
}

func TestListToolCommit_UserInfoFetchError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().GetTool(gomock.Any(), gomock.Any()).Return(&toolmgmt.Tool{ID: 1, SpaceID: 100}, nil)
	mockToolRepo.EXPECT().ListToolCommit(gomock.Any(), gomock.Any()).Return(&repo.ListToolCommitResult{
		CommitInfos: []*toolmgmt.CommitInfo{
			{Version: "1.0.0", CommittedBy: "user-1"},
		},
	}, nil)

	mockUser := mocks.NewMockIUserProvider(ctrl)
	mockUser.EXPECT().MGetUserInfo(gomock.Any(), gomock.Any()).Return(nil, errorx.New("user rpc error"))

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
		userRPCProvider: mockUser,
	}
	req := &toolmanage.ListToolCommitRequest{
		ToolID:      ptr.Of(int64(1)),
		WorkspaceID: ptr.Of(int64(100)),
		PageSize:    ptr.Of(int32(10)),
	}
	resp, err := app.ListToolCommit(ctxWithUser("user-1"), req)
	assert.Error(t, err)
	// When user fetch fails, the method returns a fresh response
	assert.NotNil(t, resp)
}

// --- BatchGetTools unit tests ---

func TestBatchGetTools_EmptyQueries(t *testing.T) {
	t.Parallel()
	app := &ToolManageApplicationImpl{}
	req := &toolmanage.BatchGetToolsRequest{
		WorkspaceID: ptr.Of(int64(100)),
		Queries:     []*toolmanage.ToolQuery{},
	}
	_, err := app.BatchGetTools(ctxWithUser("user-1"), req)
	unittest.AssertErrorEqual(t, errorx.NewByCode(prompterr.CommonInvalidParamCode), err)
}

func TestBatchGetTools_NilQueriesSkipped(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	// After filtering nil queries, should still call BatchGetTools with non-nil entries
	mockToolRepo.EXPECT().BatchGetTools(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, param repo.BatchGetToolsParam) ([]*repo.BatchGetToolsResult, error) {
			assert.Len(t, param.Queries, 1)
			assert.Equal(t, int64(1), param.Queries[0].ToolID)
			return []*repo.BatchGetToolsResult{
				{
					Query: repo.BatchGetToolsQuery{ToolID: 1, Version: "1.0.0"},
					Tool: &toolmgmt.Tool{
						ID:      1,
						SpaceID: 100,
					},
				},
			}, nil
		},
	)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.BatchGetToolsRequest{
		WorkspaceID: ptr.Of(int64(100)),
		Queries: []*toolmanage.ToolQuery{
			nil,
			{ToolID: ptr.Of(int64(1)), Version: ptr.Of("1.0.0")},
			nil,
		},
	}
	resp, err := app.BatchGetTools(ctxWithUser("user-1"), req)
	assert.NoError(t, err)
	assert.Len(t, resp.Items, 1)
}

func TestBatchGetTools_NilResultsSkipped(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), int64(100), gomock.Any()).Return(nil)

	mockToolRepo := repomocks.NewMockIToolRepo(ctrl)
	mockToolRepo.EXPECT().BatchGetTools(gomock.Any(), gomock.Any()).Return([]*repo.BatchGetToolsResult{
		nil,
		{Query: repo.BatchGetToolsQuery{ToolID: 1, Version: "1.0.0"}, Tool: nil},
		{Query: repo.BatchGetToolsQuery{ToolID: 2, Version: "1.0.0"}, Tool: &toolmgmt.Tool{ID: 2, SpaceID: 100}},
	}, nil)

	app := &ToolManageApplicationImpl{
		toolRepo:        mockToolRepo,
		authRPCProvider: mockAuth,
	}
	req := &toolmanage.BatchGetToolsRequest{
		WorkspaceID: ptr.Of(int64(100)),
		Queries: []*toolmanage.ToolQuery{
			{ToolID: ptr.Of(int64(1)), Version: ptr.Of("1.0.0")},
			{ToolID: ptr.Of(int64(2)), Version: ptr.Of("1.0.0")},
		},
	}
	resp, err := app.BatchGetTools(ctxWithUser("user-1"), req)
	assert.NoError(t, err)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, int64(2), resp.Items[0].Tool.GetID())
}

