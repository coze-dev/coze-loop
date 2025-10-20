// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth"
	authentity "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/domain/auth"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/auth/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestAuthProviderImpl_CheckWorkspacePermission(t *testing.T) {
	type fields struct {
		cli *mocks.MockClient
	}
	type args struct {
		ctx         context.Context
		action      string
		workspaceId string
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "check workspace permission successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
						return &auth.MCheckPermissionResponse{
							BaseResp: &base.BaseResp{StatusCode: 0},
							AuthRes: []*authentity.SubjectActionObjectAuthRes{
								{IsAllowed: ptr.Of(true)},
							},
						}, nil
					})
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
			},
			wantErr: false,
		},
		{
			name: "workspace id conversion failed - non-numeric string",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "invalid_id",
			},
			wantErr: true,
		},
		{
			name: "rpc call failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("rpc error"))
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
			},
			wantErr: true,
		},
		{
			name: "response is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, nil)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
			},
			wantErr: true,
		},
		{
			name: "response status code non-zero",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&auth.MCheckPermissionResponse{
						BaseResp: &base.BaseResp{StatusCode: 500},
					}, nil)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
			},
			wantErr: true,
		},
		{
			name: "permission denied - IsAllowed false",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&auth.MCheckPermissionResponse{
						BaseResp: &base.BaseResp{StatusCode: 0},
						AuthRes: []*authentity.SubjectActionObjectAuthRes{
							{IsAllowed: ptr.Of(false)},
						},
					}, nil)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
			},
			wantErr: true,
		},
		{
			name: "response with nil base resp",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&auth.MCheckPermissionResponse{
						BaseResp: nil,
						AuthRes: []*authentity.SubjectActionObjectAuthRes{
							{IsAllowed: ptr.Of(true)},
						},
					}, nil)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
			},
			wantErr: false,
		},
		{
			name: "auth result is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&auth.MCheckPermissionResponse{
						BaseResp: &base.BaseResp{StatusCode: 0},
						AuthRes: []*authentity.SubjectActionObjectAuthRes{
							nil,
						},
					}, nil)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
			},
			wantErr: false,
		},
		{
			name: "empty auth results",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&auth.MCheckPermissionResponse{
						BaseResp: &base.BaseResp{StatusCode: 0},
						AuthRes:  []*authentity.SubjectActionObjectAuthRes{},
					}, nil)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			a := &AuthProviderImpl{
				cli: f.cli,
			}
			err := a.CheckWorkspacePermission(tt.args.ctx, tt.args.action, tt.args.workspaceId)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestAuthProviderImpl_CheckViewPermission(t *testing.T) {
	type fields struct {
		cli *mocks.MockClient
	}
	type args struct {
		ctx         context.Context
		action      string
		workspaceId string
		viewId      string
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "check view permission successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
						return &auth.MCheckPermissionResponse{
							BaseResp: &base.BaseResp{StatusCode: 0},
							AuthRes: []*authentity.SubjectActionObjectAuthRes{
								{IsAllowed: ptr.Of(true)},
							},
						}, nil
					})
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
				viewId:      "view123",
			},
			wantErr: false,
		},
		{
			name: "workspace id conversion failed - non-numeric string",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "invalid_id",
				viewId:      "view123",
			},
			wantErr: true,
		},
		{
			name: "rpc call failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("rpc error"))
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
				viewId:      "view123",
			},
			wantErr: true,
		},
		{
			name: "response is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, nil)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
				viewId:      "view123",
			},
			wantErr: true,
		},
		{
			name: "response status code non-zero",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&auth.MCheckPermissionResponse{
						BaseResp: &base.BaseResp{StatusCode: 500},
					}, nil)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
				viewId:      "view123",
			},
			wantErr: true,
		},
		{
			name: "permission denied - IsAllowed false",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&auth.MCheckPermissionResponse{
						BaseResp: &base.BaseResp{StatusCode: 0},
						AuthRes: []*authentity.SubjectActionObjectAuthRes{
							{IsAllowed: ptr.Of(false)},
						},
					}, nil)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				action:      "read",
				workspaceId: "12345",
				viewId:      "view123",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			a := &AuthProviderImpl{
				cli: f.cli,
			}
			err := a.CheckViewPermission(tt.args.ctx, tt.args.action, tt.args.workspaceId, tt.args.viewId)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestAuthProviderImpl_CheckIngestPermission(t *testing.T) {
	type fields struct {
		cli *mocks.MockClient
	}
	type args struct {
		ctx         context.Context
		workspaceId string
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "check ingest permission successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
						// Verify the action is correct
						assert.Equal(t, rpc.AuthActionTraceIngest, *req.Auths[0].Action)
						return &auth.MCheckPermissionResponse{
							BaseResp: &base.BaseResp{StatusCode: 0},
							AuthRes: []*authentity.SubjectActionObjectAuthRes{
								{IsAllowed: ptr.Of(true)},
							},
						}, nil
					})
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				workspaceId: "12345",
			},
			wantErr: false,
		},
		{
			name: "check ingest permission failed - workspace permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("rpc error"))
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				workspaceId: "12345",
			},
			wantErr: true,
		},
		{
			name: "check ingest permission failed - invalid workspace id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:         context.Background(),
				workspaceId: "invalid_id",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			a := &AuthProviderImpl{
				cli: f.cli,
			}
			err := a.CheckIngestPermission(tt.args.ctx, tt.args.workspaceId)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestAuthProviderImpl_CheckQueryPermission(t *testing.T) {
	type fields struct {
		cli *mocks.MockClient
	}
	type args struct {
		ctx          context.Context
		workspaceId  string
		platformType string
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "check query permission successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
						// Verify the action is correct
						assert.Equal(t, rpc.AuthActionTraceList, *req.Auths[0].Action)
						return &auth.MCheckPermissionResponse{
							BaseResp: &base.BaseResp{StatusCode: 0},
							AuthRes: []*authentity.SubjectActionObjectAuthRes{
								{IsAllowed: ptr.Of(true)},
							},
						}, nil
					})
				return fields{cli: mockClient}
			},
			args: args{
				ctx:          context.Background(),
				workspaceId:  "12345",
				platformType: "coze",
			},
			wantErr: false,
		},
		{
			name: "check query permission failed - workspace permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("rpc error"))
				return fields{cli: mockClient}
			},
			args: args{
				ctx:          context.Background(),
				workspaceId:  "12345",
				platformType: "coze",
			},
			wantErr: true,
		},
		{
			name: "check query permission failed - invalid workspace id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockClient := mocks.NewMockClient(ctrl)
				return fields{cli: mockClient}
			},
			args: args{
				ctx:          context.Background(),
				workspaceId:  "invalid_id",
				platformType: "coze",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			a := &AuthProviderImpl{
				cli: f.cli,
			}
			err := a.CheckQueryPermission(tt.args.ctx, tt.args.workspaceId, tt.args.platformType)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestNewAuthProvider(t *testing.T) {
	type args struct {
		cli *mocks.MockClient
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) args
		want         rpc.IAuthProvider
	}{
		{
			name: "create new auth provider successfully",
			fieldsGetter: func(ctrl *gomock.Controller) args {
				mockClient := mocks.NewMockClient(ctrl)
				return args{cli: mockClient}
			},
			want: &AuthProviderImpl{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			got := NewAuthProvider(f.cli)
			assert.NotNil(t, got)
			assert.IsType(t, &AuthProviderImpl{}, got)
		})
	}
}

func TestAuthProviderImpl_Interface(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mocks.NewMockClient(ctrl)

	// Verify that AuthProviderImpl implements IAuthProvider interface
	var _ rpc.IAuthProvider = &AuthProviderImpl{}

	provider := NewAuthProvider(mockClient)
	assert.NotNil(t, provider)
	assert.IsType(t, &AuthProviderImpl{}, provider)
}

func TestAuthProviderImpl_ErrorCodes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	provider := &AuthProviderImpl{cli: mockClient}

	// Test invalid workspace ID error code
	err := provider.CheckWorkspacePermission(context.Background(), "read", "invalid")
	assert.NotNil(t, err)

	// Check if error is wrapped with correct error code
	assert.Contains(t, err.Error(), "internal error")
	assert.Contains(t, err.Error(), "internal error")
	// Test RPC error code
	mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("rpc error"))

	err = provider.CheckWorkspacePermission(context.Background(), "read", "12345")
	assert.NotNil(t, err)

	// Test permission denied error code
	mockClient.EXPECT().MCheckPermission(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&auth.MCheckPermissionResponse{
			BaseResp: &base.BaseResp{StatusCode: 0},
			AuthRes: []*authentity.SubjectActionObjectAuthRes{
				{IsAllowed: ptr.Of(false)},
			},
		}, nil)

	err = provider.CheckWorkspacePermission(context.Background(), "read", "12345")
	assert.NotNil(t, err)
}
