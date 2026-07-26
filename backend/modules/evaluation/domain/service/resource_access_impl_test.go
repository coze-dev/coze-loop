// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

const (
	testConsumerSpace = int64(100)
	testSourceSpace   = int64(200)
	testEvalSetID     = int64(10)
)

// sharedCfg 构造一份共享配置：来源空间 source 把 resourceID(evalSet) 以 accessLevel 共享给 caller。
func sharedCfg(sourceSpaceID, resourceID, callerSpaceID int64, accessLevel, versionPolicy string, specifiedIDs []int64) *entity.SharedResourceConfig {
	return &entity.SharedResourceConfig{
		SpaceRules: map[int64]*entity.SpaceSharedRules{
			sourceSpaceID: {
				Resources: []*entity.SharedResourceRule{
					{
						ResourceID:    resourceID,
						ResourceType:  entity.SharedResourceTypeEvalSet,
						VersionPolicy: versionPolicy,
						SpecifiedIDs:  specifiedIDs,
						AccessRules: []*entity.SharedAccessRule{
							{AccessLevel: accessLevel, Targets: []string{strconv.FormatInt(callerSpaceID, 10)}},
						},
					},
				},
			},
		},
	}
}

func evalSetAuthReq(callerSpaceID, resourceID int64, sharedOption *entity.SharedResourceOption, requireContentRead bool) *entity.AuthorizeResourceRequest {
	return &entity.AuthorizeResourceRequest{
		CallerSpaceID:      callerSpaceID,
		ResourceType:       entity.SharedResourceTypeEvalSet,
		ResourceID:         resourceID,
		SharedOption:       sharedOption,
		RequireContentRead: requireContentRead,
	}
}

// 未共享(sharedOption 为 nil / 未开启)→ 走本空间基础鉴权,返回 direct context。
func TestAuthorizeRead_NotShared_Direct(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	provider := componentmocks.NewMockSharedResourceConfigProvider(ctrl)

	// 本空间基础鉴权应以 consumer 作为 ResourceSpaceID
	mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *rpc.AuthorizationWithoutSPIParam) error {
			assert.Equal(t, testConsumerSpace, p.SpaceID)
			assert.Equal(t, testConsumerSpace, p.ResourceSpaceID)
			return nil
		},
	)
	// provider 不应被调用
	provider.EXPECT().GetSharedResourceConfig(gomock.Any()).Times(0)

	authorizer := NewResourceAccessAuthorizer(mockAuth, provider)
	got, err := authorizer.AuthorizeRead(context.Background(), evalSetAuthReq(testConsumerSpace, testEvalSetID, nil, false))
	assert.NoError(t, err)
	assert.False(t, got.IsShared())
	assert.Equal(t, testConsumerSpace, got.QuerySpaceID())
}

// 共享 + readable → 放行,且查询空间重定向为来源空间。
func TestAuthorizeRead_Shared_Readable_Allow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	provider := componentmocks.NewMockSharedResourceConfigProvider(ctrl)

	provider.EXPECT().GetSharedResourceConfig(gomock.Any()).Return(
		sharedCfg(testSourceSpace, testEvalSetID, testConsumerSpace, entity.SharedAccessLevelReadable, entity.SharedVersionPolicyAll, nil), nil,
	)
	// 共享场景:基础鉴权 ResourceSpaceID 应为来源空间
	mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *rpc.AuthorizationWithoutSPIParam) error {
			assert.Equal(t, testConsumerSpace, p.SpaceID)
			assert.Equal(t, testSourceSpace, p.ResourceSpaceID)
			return nil
		},
	)

	sharedOption := &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(testSourceSpace)}
	authorizer := NewResourceAccessAuthorizer(mockAuth, provider)
	got, err := authorizer.AuthorizeRead(context.Background(), evalSetAuthReq(testConsumerSpace, testEvalSetID, sharedOption, true))
	assert.NoError(t, err)
	assert.True(t, got.IsShared())
	assert.Equal(t, testSourceSpace, got.QuerySpaceID())
	assert.Equal(t, entity.SharedAccessLevelReadable, got.AccessLevel)
}

// 共享但资源未在白名单 → PermissionDenied(fail-closed)。
func TestAuthorizeRead_Shared_NotWhitelisted_Deny(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	provider := componentmocks.NewMockSharedResourceConfigProvider(ctrl)

	// 空配置 → Lookup 恒 nil
	provider.EXPECT().GetSharedResourceConfig(gomock.Any()).Return(&entity.SharedResourceConfig{}, nil)
	// 未命中白名单,不应走到基础鉴权
	mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)

	sharedOption := &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(testSourceSpace)}
	authorizer := NewResourceAccessAuthorizer(mockAuth, provider)
	_, err := authorizer.AuthorizeRead(context.Background(), evalSetAuthReq(testConsumerSpace, testEvalSetID, sharedOption, false))
	assert.Error(t, err)
}

// 共享 + execute + requireContentRead → PermissionDenied(黑盒不允许看内容)。
func TestAuthorizeRead_Execute_RequireContent_Deny(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	provider := componentmocks.NewMockSharedResourceConfigProvider(ctrl)

	provider.EXPECT().GetSharedResourceConfig(gomock.Any()).Return(
		sharedCfg(testSourceSpace, testEvalSetID, testConsumerSpace, entity.SharedAccessLevelExecute, entity.SharedVersionPolicyAll, nil), nil,
	)
	mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)

	sharedOption := &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(testSourceSpace)}
	authorizer := NewResourceAccessAuthorizer(mockAuth, provider)
	_, err := authorizer.AuthorizeRead(context.Background(), evalSetAuthReq(testConsumerSpace, testEvalSetID, sharedOption, true))
	assert.Error(t, err)
}

// provider 拉配置报错 → fail-closed。
func TestAuthorizeRead_ProviderError_FailClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	provider := componentmocks.NewMockSharedResourceConfigProvider(ctrl)

	provider.EXPECT().GetSharedResourceConfig(gomock.Any()).Return(nil, errors.New("config unavailable"))
	mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)

	sharedOption := &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(testSourceSpace)}
	authorizer := NewResourceAccessAuthorizer(mockAuth, provider)
	_, err := authorizer.AuthorizeRead(context.Background(), evalSetAuthReq(testConsumerSpace, testEvalSetID, sharedOption, false))
	assert.Error(t, err)
}

// source == consumer 视为非法共享参数 → InvalidParam。
func TestAuthorizeRead_SourceEqualsConsumer_Invalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	provider := componentmocks.NewMockSharedResourceConfigProvider(ctrl)
	provider.EXPECT().GetSharedResourceConfig(gomock.Any()).Times(0)
	mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)

	sharedOption := &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(testConsumerSpace)}
	authorizer := NewResourceAccessAuthorizer(mockAuth, provider)
	_, err := authorizer.AuthorizeRead(context.Background(), evalSetAuthReq(testConsumerSpace, testEvalSetID, sharedOption, false))
	assert.Error(t, err)
}

// nil provider → 共享读 fail-closed。
func TestAuthorizeRead_NilProvider_FailClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)

	sharedOption := &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(testSourceSpace)}
	authorizer := NewResourceAccessAuthorizer(mockAuth, nil)
	_, err := authorizer.AuthorizeRead(context.Background(), evalSetAuthReq(testConsumerSpace, testEvalSetID, sharedOption, false))
	assert.Error(t, err)
}

// ListSharedResources：命中枚举（单源过滤）。
func TestListSharedResources_SingleSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	provider := componentmocks.NewMockSharedResourceConfigProvider(ctrl)
	provider.EXPECT().GetSharedResourceConfig(gomock.Any()).Return(
		sharedCfg(testSourceSpace, testEvalSetID, testConsumerSpace, entity.SharedAccessLevelReadable, entity.SharedVersionPolicyAll, nil), nil,
	)

	authorizer := NewResourceAccessAuthorizer(mockAuth, provider)
	got, err := authorizer.ListSharedResources(context.Background(), &entity.ListSharedResourcesRequest{
		CallerSpaceID:     testConsumerSpace,
		ResourceType:      entity.SharedResourceTypeEvalSet,
		SourceSpaceFilter: gptr.Of(testSourceSpace),
	})
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, testSourceSpace, got[0].ResourceSpaceID)
	assert.Equal(t, testEvalSetID, got[0].ResourceID)
	assert.True(t, got[0].IsShared())
}

// ListSharedResources：跨全部来源空间枚举（SourceSpaceFilter=nil）。
func TestListSharedResources_CrossSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	provider := componentmocks.NewMockSharedResourceConfigProvider(ctrl)

	cfg := &entity.SharedResourceConfig{
		SpaceRules: map[int64]*entity.SpaceSharedRules{
			testSourceSpace: {Resources: []*entity.SharedResourceRule{{
				ResourceID: 10, ResourceType: entity.SharedResourceTypeEvalSet, VersionPolicy: entity.SharedVersionPolicyAll,
				AccessRules: []*entity.SharedAccessRule{{AccessLevel: entity.SharedAccessLevelReadable, Targets: []string{strconv.FormatInt(testConsumerSpace, 10)}}},
			}}},
			testSourceSpace + 1: {Resources: []*entity.SharedResourceRule{{
				ResourceID: 20, ResourceType: entity.SharedResourceTypeEvalSet, VersionPolicy: entity.SharedVersionPolicyAll,
				AccessRules: []*entity.SharedAccessRule{{AccessLevel: entity.SharedAccessLevelReadable, Targets: []string{strconv.FormatInt(testConsumerSpace, 10)}}},
			}}},
		},
	}
	provider.EXPECT().GetSharedResourceConfig(gomock.Any()).Return(cfg, nil)

	authorizer := NewResourceAccessAuthorizer(mockAuth, provider)
	got, err := authorizer.ListSharedResources(context.Background(), &entity.ListSharedResourcesRequest{
		CallerSpaceID: testConsumerSpace,
		ResourceType:  entity.SharedResourceTypeEvalSet,
	})
	assert.NoError(t, err)
	assert.Len(t, got, 2)
}

// ListSharedResources：空配置 → 空列表(fail-closed，非错误)。
func TestListSharedResources_EmptyConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	provider := componentmocks.NewMockSharedResourceConfigProvider(ctrl)
	provider.EXPECT().GetSharedResourceConfig(gomock.Any()).Return(nil, nil)

	authorizer := NewResourceAccessAuthorizer(mockAuth, provider)
	got, err := authorizer.ListSharedResources(context.Background(), &entity.ListSharedResourcesRequest{
		CallerSpaceID: testConsumerSpace,
		ResourceType:  entity.SharedResourceTypeEvalSet,
	})
	assert.NoError(t, err)
	assert.Empty(t, got)
}

// ListSharedResources：provider 报错 → fail-closed(错误)。
func TestListSharedResources_ProviderError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	provider := componentmocks.NewMockSharedResourceConfigProvider(ctrl)
	provider.EXPECT().GetSharedResourceConfig(gomock.Any()).Return(nil, errors.New("config unavailable"))

	authorizer := NewResourceAccessAuthorizer(mockAuth, provider)
	_, err := authorizer.ListSharedResources(context.Background(), &entity.ListSharedResourcesRequest{
		CallerSpaceID: testConsumerSpace,
		ResourceType:  entity.SharedResourceTypeEvalSet,
	})
	assert.Error(t, err)
}
