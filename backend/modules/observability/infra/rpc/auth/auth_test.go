// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth"
	authentity "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/domain/auth"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/auth/mocks"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestNewAuthProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	provider := NewAuthProvider(mockClient)

	assert.NotNil(t, provider)
	assert.IsType(t, &AuthProviderImpl{}, provider)
}

func TestAuthProviderImpl_CheckWorkspacePermission(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	provider := &AuthProviderImpl{cli: mockClient}

	tests := []struct {
		name        string
		action      string
		workspaceId string
		isOpi       bool
		mockSetup   func()
		wantErr     bool
		expectedErr int
	}{
		{
			name:        "success - permission granted",
			action:      "testAction",
			workspaceId: "123",
			isOpi:       true,
			mockSetup: func() {
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(&auth.MCheckPermissionResponse{
					BaseResp: &base.BaseResp{StatusCode: 0},
					AuthRes: []*authentity.SubjectActionObjectAuthRes{
						{IsAllowed: ptr.Of(true)},
					},
				}, nil)
			},
			wantErr: false,
		},
		{
			name:        "permission denied",
			action:      "testAction",
			workspaceId: "123",
			isOpi:       true,
			mockSetup: func() {
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(&auth.MCheckPermissionResponse{
					BaseResp: &base.BaseResp{StatusCode: 0},
					AuthRes: []*authentity.SubjectActionObjectAuthRes{
						{IsAllowed: ptr.Of(false)},
					},
				}, nil)
			},
			wantErr:     true,
			expectedErr: obErrorx.CommonNoPermissionCode,
		},
		{
			name:        "invalid workspace ID",
			action:      "testAction",
			workspaceId: "invalid",
			isOpi:       true,
			mockSetup:   func() {},
			wantErr:     true,
			expectedErr: obErrorx.CommonInternalErrorCode,
		},
		{
			name:        "RPC error",
			action:      "testAction",
			workspaceId: "123",
			isOpi:       true,
			mockSetup: func() {
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(nil, errors.New("RPC error"))
			},
			wantErr:     true,
			expectedErr: obErrorx.CommercialCommonRPCErrorCodeCode,
		},
		{
			name:        "nil response",
			action:      "testAction",
			workspaceId: "123",
			isOpi:       true,
			mockSetup: func() {
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			wantErr:     true,
			expectedErr: obErrorx.CommercialCommonRPCErrorCodeCode,
		},
		{
			name:        "non-zero status code",
			action:      "testAction",
			workspaceId: "123",
			isOpi:       true,
			mockSetup: func() {
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(&auth.MCheckPermissionResponse{
					BaseResp: &base.BaseResp{StatusCode: 1},
					AuthRes:  []*authentity.SubjectActionObjectAuthRes{},
				}, nil)
			},
			wantErr:     true,
			expectedErr: obErrorx.CommercialCommonRPCErrorCodeCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := provider.CheckWorkspacePermission(context.Background(), tt.action, tt.workspaceId, tt.isOpi)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != 0 {
					assert.Contains(t, err.Error(), fmt.Sprintf("%d", tt.expectedErr))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthProviderImpl_CheckIngestPermission(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	provider := &AuthProviderImpl{cli: mockClient}

	tests := []struct {
		name        string
		workspaceId string
		mockSetup   func()
		wantErr     bool
		expectedErr int
	}{
		{
			name:        "success - ingest permission granted",
			workspaceId: "123",
			mockSetup: func() {
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(&auth.MCheckPermissionResponse{
					BaseResp: &base.BaseResp{StatusCode: 0},
					AuthRes: []*authentity.SubjectActionObjectAuthRes{
						{IsAllowed: ptr.Of(true)},
					},
				}, nil)
			},
			wantErr: false,
		},
		{
			name:        "ingest permission denied",
			workspaceId: "123",
			mockSetup: func() {
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(&auth.MCheckPermissionResponse{
					BaseResp: &base.BaseResp{StatusCode: 0},
					AuthRes: []*authentity.SubjectActionObjectAuthRes{
						{IsAllowed: ptr.Of(false)},
					},
				}, nil)
			},
			wantErr:     true,
			expectedErr: obErrorx.CommonNoPermissionCode,
		},
		{
			name:        "invalid workspace ID for ingest",
			workspaceId: "invalid",
			mockSetup:   func() {},
			wantErr:     true,
			expectedErr: obErrorx.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := provider.CheckIngestPermission(context.Background(), tt.workspaceId)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != 0 {
					assert.Contains(t, err.Error(), fmt.Sprintf("%d", tt.expectedErr))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthProviderImpl_CheckQueryPermission(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	provider := &AuthProviderImpl{cli: mockClient}

	tests := []struct {
		name         string
		workspaceId  string
		platformType string
		mockSetup    func()
		wantErr      bool
		expectedErr  int
	}{
		{
			name:         "success - query permission granted",
			workspaceId:  "123",
			platformType: "web",
			mockSetup: func() {
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(&auth.MCheckPermissionResponse{
					BaseResp: &base.BaseResp{StatusCode: 0},
					AuthRes: []*authentity.SubjectActionObjectAuthRes{
						{IsAllowed: ptr.Of(true)},
					},
				}, nil)
			},
			wantErr: false,
		},
		{
			name:         "query permission denied",
			workspaceId:  "123",
			platformType: "web",
			mockSetup: func() {
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(&auth.MCheckPermissionResponse{
					BaseResp: &base.BaseResp{StatusCode: 0},
					AuthRes: []*authentity.SubjectActionObjectAuthRes{
						{IsAllowed: ptr.Of(false)},
					},
				}, nil)
			},
			wantErr:     true,
			expectedErr: obErrorx.CommonNoPermissionCode,
		},
		{
			name:         "invalid workspace ID for query",
			workspaceId:  "invalid",
			platformType: "web",
			mockSetup:    func() {},
			wantErr:      true,
			expectedErr:  obErrorx.CommonInternalErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := provider.CheckQueryPermission(context.Background(), tt.workspaceId, tt.platformType)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != 0 {
					assert.Contains(t, err.Error(), fmt.Sprintf("%d", tt.expectedErr))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthProviderImpl_CheckIngestPermission_UsesCorrectAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	provider := &AuthProviderImpl{cli: mockClient}

	// Test that CheckIngestPermission uses the correct action
	mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
			assert.Equal(t, rpc.AuthActionTraceIngest, *req.Auths[0].Action)
			return &auth.MCheckPermissionResponse{
				BaseResp: &base.BaseResp{StatusCode: 0},
				AuthRes: []*authentity.SubjectActionObjectAuthRes{
					{IsAllowed: ptr.Of(true)},
				},
			}, nil
		})

	err := provider.CheckIngestPermission(context.Background(), "123")
	assert.NoError(t, err)
}

func TestAuthProviderImpl_CheckQueryPermission_UsesCorrectAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	provider := &AuthProviderImpl{cli: mockClient}

	// Test that CheckQueryPermission uses the correct action
	mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
			assert.Equal(t, rpc.AuthActionTraceList, *req.Auths[0].Action)
			return &auth.MCheckPermissionResponse{
				BaseResp: &base.BaseResp{StatusCode: 0},
				AuthRes: []*authentity.SubjectActionObjectAuthRes{
					{IsAllowed: ptr.Of(true)},
				},
			}, nil
		})

	err := provider.CheckQueryPermission(context.Background(), "123", "web")
	assert.NoError(t, err)
}

func TestAuthProviderImpl_CheckTaskPermission(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	provider := &AuthProviderImpl{cli: mockClient}

	t.Run("success", func(t *testing.T) {
		ctx := session.WithCtxUser(context.Background(), &session.User{ID: "user1"})
		mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(callCtx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
				authorization := req.Auths[0]
				assert.Equal(t, rpc.AuthActionTraceTaskCreate, authorization.GetAction())
				subject := authorization.GetSubject()
				if assert.NotNil(t, subject) {
					assert.Equal(t, authentity.AuthPrincipalType_User, subject.GetAuthPrincipalType())
					if assert.NotNil(t, subject.GetAuthUser()) {
						assert.Equal(t, "user1", subject.GetAuthUser().GetUserID())
					}
				}
				objects := authorization.GetObjects()
				if assert.Len(t, objects, 1) {
					assert.Equal(t, authentity.AuthEntityTypeTraceTask, objects[0].GetEntityType())
					assert.Equal(t, "task456", objects[0].GetID())
					assert.Equal(t, "123", objects[0].GetSpaceID())
					assert.Equal(t, "user1", objects[0].GetOwnerUserID())
				}
				assert.Equal(t, int64(123), req.GetSpaceID())
				return &auth.MCheckPermissionResponse{
					BaseResp: &base.BaseResp{StatusCode: 0},
					AuthRes: []*authentity.SubjectActionObjectAuthRes{
						{IsAllowed: ptr.Of(true)},
					},
				}, nil
			})
		err := provider.CheckTaskPermission(ctx, rpc.AuthActionTraceTaskCreate, "123", "task456")
		assert.NoError(t, err)
	})

	t.Run("invalid workspace id", func(t *testing.T) {
		err := provider.CheckTaskPermission(context.Background(), rpc.AuthActionTraceTaskList, "invalid", "task")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("%d", obErrorx.CommonInternalErrorCode))
	})

	t.Run("rpc error", func(t *testing.T) {
		ctx := session.WithCtxUser(context.Background(), &session.User{ID: "user1"})
		mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("rpc error"))
		err := provider.CheckTaskPermission(ctx, rpc.AuthActionTraceTaskEdit, "123", "task456")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("%d", obErrorx.CommercialCommonRPCErrorCodeCode))
	})

	t.Run("permission denied", func(t *testing.T) {
		ctx := session.WithCtxUser(context.Background(), &session.User{ID: "user1"})
		mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(&auth.MCheckPermissionResponse{
			BaseResp: &base.BaseResp{StatusCode: 0},
			AuthRes: []*authentity.SubjectActionObjectAuthRes{
				{IsAllowed: ptr.Of(false)},
			},
		}, nil)
		err := provider.CheckTaskPermission(ctx, rpc.AuthActionTraceTaskEdit, "123", "task456")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("%d", obErrorx.CommonNoPermissionCode))
	})
}
