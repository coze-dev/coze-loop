// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth"
	authentity "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/domain/auth"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/rpc/mocks"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestNewAuthRPCProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	provider := NewAuthRPCProvider(mockClient)

	assert.NotNil(t, provider)
	adapter, ok := provider.(*AuthRPCAdapter)
	assert.True(t, ok)
	assert.Equal(t, mockClient, adapter.client)
}

func createAuthRes(isAllowed bool, promptID string) *authentity.SubjectActionObjectAuthRes {
	if promptID == "" {
		return &authentity.SubjectActionObjectAuthRes{
			IsAllowed: ptr.Of(isAllowed),
		}
	}
	return &authentity.SubjectActionObjectAuthRes{
		IsAllowed: ptr.Of(isAllowed),
		SubjectActionObjects: &authentity.SubjectActionObjects{
			Objects: []*authentity.AuthEntity{
				{ID: ptr.Of(promptID)},
			},
		},
	}
}

func TestAuthRPCAdapter_MCheckPromptPermission(t *testing.T) {
	tests := []struct {
		name        string
		spaceID     int64
		promptIDs   []int64
		action      string
		setupMock   func(*mocks.MockClient)
		expectError bool
		errorCode   int
	}{
		{
			name:      "success - all permissions allowed",
			spaceID:   123,
			promptIDs: []int64{1, 2, 3},
			action:    "read",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
						assert.Equal(t, int64(123), ptr.From(req.SpaceID))
						assert.Equal(t, 3, len(req.Auths))

						return &auth.MCheckPermissionResponse{
							AuthRes: []*authentity.SubjectActionObjectAuthRes{
								createAuthRes(true, ""),
								createAuthRes(true, ""),
								createAuthRes(true, ""),
							},
						}, nil
					},
				)
			},
			expectError: false,
		},
		{
			name:      "failure - permission denied",
			spaceID:   123,
			promptIDs: []int64{1, 2},
			action:    "write",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
						return &auth.MCheckPermissionResponse{
							AuthRes: []*authentity.SubjectActionObjectAuthRes{
								createAuthRes(false, "1"),
								createAuthRes(true, ""),
							},
						}, nil
					},
				)
			},
			expectError: true,
			errorCode:   prompterr.CommonNoPermissionCode,
		},
		{
			name:      "failure - RPC error",
			spaceID:   123,
			promptIDs: []int64{1},
			action:    "read",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(
					nil, errors.New("RPC connection failed"),
				)
			},
			expectError: true,
		},
		{
			name:      "success - empty prompt IDs",
			spaceID:   123,
			promptIDs: []int64{},
			action:    "read",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
						assert.Equal(t, 0, len(req.Auths))
						return &auth.MCheckPermissionResponse{
							AuthRes: []*authentity.SubjectActionObjectAuthRes{},
						}, nil
					},
				)
			},
			expectError: false,
		},
		{
			name:      "failure - multiple permissions denied",
			spaceID:   123,
			promptIDs: []int64{1, 2, 3},
			action:    "delete",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
						return &auth.MCheckPermissionResponse{
							AuthRes: []*authentity.SubjectActionObjectAuthRes{
								createAuthRes(false, "1"),
								createAuthRes(false, "2"),
								createAuthRes(true, ""),
							},
						}, nil
					},
				)
			},
			expectError: true,
			errorCode:   prompterr.CommonNoPermissionCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockClient(ctrl)
			tt.setupMock(mockClient)

			adapter := &AuthRPCAdapter{
				client: mockClient,
			}

			err := adapter.MCheckPromptPermission(context.Background(), tt.spaceID, tt.promptIDs, tt.action)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, int32(tt.errorCode), statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthRPCAdapter_MCheckPromptPermissionForOpenAPI(t *testing.T) {
	tests := []struct {
		name        string
		spaceID     int64
		promptIDs   []int64
		action      string
		setupMock   func(*mocks.MockClient)
		expectError bool
	}{
		{
			name:      "success - OpenAPI permission allowed",
			spaceID:   456,
			promptIDs: []int64{10, 20},
			action:    "execute",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(
					&auth.MCheckPermissionResponse{
						AuthRes: []*authentity.SubjectActionObjectAuthRes{
							createAuthRes(true, ""),
							createAuthRes(true, ""),
						},
					}, nil,
				)
			},
			expectError: false,
		},
		{
			name:      "failure - OpenAPI permission denied",
			spaceID:   456,
			promptIDs: []int64{10},
			action:    "execute",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(
					&auth.MCheckPermissionResponse{
						AuthRes: []*authentity.SubjectActionObjectAuthRes{
							createAuthRes(false, "10"),
						},
					}, nil,
				)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockClient(ctrl)
			tt.setupMock(mockClient)

			adapter := &AuthRPCAdapter{
				client: mockClient,
			}

			err := adapter.MCheckPromptPermissionForOpenAPI(context.Background(), tt.spaceID, tt.promptIDs, tt.action)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthRPCAdapter_CheckSpacePermission(t *testing.T) {
	tests := []struct {
		name        string
		spaceID     int64
		action      string
		setupMock   func(*mocks.MockClient)
		expectError bool
		errorCode   int
	}{
		{
			name:    "success - space permission allowed",
			spaceID: 789,
			action:  "manage",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
						assert.Equal(t, int64(789), ptr.From(req.SpaceID))
						assert.Equal(t, 1, len(req.Auths))
						assert.Equal(t, "manage", ptr.From(req.Auths[0].Action))

						// Verify entity type is Space
						assert.Equal(t, 1, len(req.Auths[0].Objects))
						assert.Equal(t, authentity.AuthEntityTypeSpace, ptr.From(req.Auths[0].Objects[0].EntityType))
						assert.Equal(t, "789", ptr.From(req.Auths[0].Objects[0].ID))

						return &auth.MCheckPermissionResponse{
							AuthRes: []*authentity.SubjectActionObjectAuthRes{
								createAuthRes(true, ""),
							},
						}, nil
					},
				)
			},
			expectError: false,
		},
		{
			name:    "failure - space permission denied",
			spaceID: 789,
			action:  "admin",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(
					&auth.MCheckPermissionResponse{
						AuthRes: []*authentity.SubjectActionObjectAuthRes{
							createAuthRes(false, ""),
						},
					}, nil,
				)
			},
			expectError: true,
			errorCode:   prompterr.CommonNoPermissionCode,
		},
		{
			name:    "failure - RPC error",
			spaceID: 789,
			action:  "view",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(
					nil, errors.New("network error"),
				)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockClient(ctrl)
			tt.setupMock(mockClient)

			adapter := &AuthRPCAdapter{
				client: mockClient,
			}

			err := adapter.CheckSpacePermission(context.Background(), tt.spaceID, tt.action)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, int32(tt.errorCode), statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthRPCAdapter_CheckSpacePermissionForOpenAPI(t *testing.T) {
	tests := []struct {
		name        string
		spaceID     int64
		action      string
		setupMock   func(*mocks.MockClient)
		expectError bool
	}{
		{
			name:    "success - OpenAPI space permission allowed",
			spaceID: 999,
			action:  "create_prompt",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(
					&auth.MCheckPermissionResponse{
						AuthRes: []*authentity.SubjectActionObjectAuthRes{
							createAuthRes(true, ""),
						},
					}, nil,
				)
			},
			expectError: false,
		},
		{
			name:    "failure - OpenAPI space permission denied",
			spaceID: 999,
			action:  "delete_space",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).Return(
					&auth.MCheckPermissionResponse{
						AuthRes: []*authentity.SubjectActionObjectAuthRes{
							createAuthRes(false, ""),
						},
					}, nil,
				)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockClient(ctrl)
			tt.setupMock(mockClient)

			adapter := &AuthRPCAdapter{
				client: mockClient,
			}

			err := adapter.CheckSpacePermissionForOpenAPI(context.Background(), tt.spaceID, tt.action)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthRPCAdapter_mCheckPromptPermissionBase(t *testing.T) {
	tests := []struct {
		name        string
		spaceID     int64
		promptIDs   []int64
		action      string
		setupMock   func(*mocks.MockClient)
		expectError bool
		validateReq bool
	}{
		{
			name:      "validate request structure",
			spaceID:   123,
			promptIDs: []int64{1, 2},
			action:    "read",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
						// Validate request structure
						assert.NotNil(t, req)
						assert.Equal(t, int64(123), ptr.From(req.SpaceID))
						assert.Equal(t, 2, len(req.Auths))

						for i, authPair := range req.Auths {
							assert.NotNil(t, authPair.Subject)
							assert.Equal(t, authentity.AuthPrincipalType_CozeIdentifier, ptr.From(authPair.Subject.AuthPrincipalType))
							assert.NotNil(t, authPair.Subject.AuthCozeIdentifier)
							assert.Nil(t, authPair.Subject.AuthCozeIdentifier.IdentityTicket)

							assert.Equal(t, "read", ptr.From(authPair.Action))
							assert.Equal(t, 1, len(authPair.Objects))

							expectedID := int64(i + 1)
							assert.Equal(t, string(rune(expectedID+'0')), ptr.From(authPair.Objects[0].ID))
							assert.Equal(t, authentity.AuthEntityTypePrompt, ptr.From(authPair.Objects[0].EntityType))
						}

						return &auth.MCheckPermissionResponse{
							AuthRes: []*authentity.SubjectActionObjectAuthRes{
								createAuthRes(true, ""),
								createAuthRes(true, ""),
							},
						}, nil
					},
				)
			},
			expectError: false,
			validateReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockClient(ctrl)
			tt.setupMock(mockClient)

			adapter := &AuthRPCAdapter{
				client: mockClient,
			}

			err := adapter.mCheckPromptPermissionBase(context.Background(), tt.spaceID, tt.promptIDs, tt.action)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuthRPCAdapter_checkSpacePermissionBase(t *testing.T) {
	tests := []struct {
		name        string
		spaceID     int64
		action      string
		setupMock   func(*mocks.MockClient)
		expectError bool
		validateReq bool
	}{
		{
			name:    "validate space request structure",
			spaceID: 456,
			action:  "manage",
			setupMock: func(mc *mocks.MockClient) {
				mc.EXPECT().MCheckPermission(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *auth.MCheckPermissionRequest, opts ...interface{}) (*auth.MCheckPermissionResponse, error) {
						// Validate request structure
						assert.NotNil(t, req)
						assert.Equal(t, int64(456), ptr.From(req.SpaceID))
						assert.Equal(t, 1, len(req.Auths))

						authPair := req.Auths[0]
						assert.NotNil(t, authPair.Subject)
						assert.Equal(t, authentity.AuthPrincipalType_CozeIdentifier, ptr.From(authPair.Subject.AuthPrincipalType))
						assert.NotNil(t, authPair.Subject.AuthCozeIdentifier)
						assert.Nil(t, authPair.Subject.AuthCozeIdentifier.IdentityTicket)

						assert.Equal(t, "manage", ptr.From(authPair.Action))
						assert.Equal(t, 1, len(authPair.Objects))
						assert.Equal(t, "456", ptr.From(authPair.Objects[0].ID))
						assert.Equal(t, authentity.AuthEntityTypeSpace, ptr.From(authPair.Objects[0].EntityType))

						return &auth.MCheckPermissionResponse{
							AuthRes: []*authentity.SubjectActionObjectAuthRes{
								createAuthRes(true, ""),
							},
						}, nil
					},
				)
			},
			expectError: false,
			validateReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockClient(ctrl)
			tt.setupMock(mockClient)

			adapter := &AuthRPCAdapter{
				client: mockClient,
			}

			err := adapter.checkSpacePermissionBase(context.Background(), tt.spaceID, tt.action)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
