// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_eval_target "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestEvalTargetApplicationImpl_ListSourceEvalTargetVersions_SharedSpecified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)
	mockResourceAccess := mocks.NewMockResourceAccessAuthorizer(ctrl)
	targetType := domain_eval_target.EvalTargetType(1)
	app := &EvalTargetApplicationImpl{
		auth:                     mockAuth,
		resourceAccessAuthorizer: mockResourceAccess,
		typedOperators: map[entity.EvalTargetType]service.ISourceEvalTargetOperateService{
			1: mockOperator,
		},
	}
	accessCtx := &entity.ResourceAccessContext{
		CallerSpaceID:     123,
		ResourceSpaceID:   456,
		ResourceType:      entity.SharedResourceTypeEvalTarget,
		ResourceID:        101,
		AccessMode:        entity.AccessModeShared,
		AccessLevel:       entity.SharedAccessLevelReadable,
		VersionPolicy:     entity.SharedVersionPolicySpecified,
		SpecifiedVersions: []string{"v2", "v1"},
	}
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	mockResourceAccess.EXPECT().AuthorizeRead(gomock.Any(), gomock.Any()).Return(accessCtx, nil).Times(2)
	for _, version := range accessCtx.SpecifiedVersions {
		mockOperator.EXPECT().BuildBySource(gomock.Any(), int64(456), "101", version).Return(&entity.EvalTarget{
			SpaceID:        456,
			SourceTargetID: "101",
			EvalTargetType: 1,
			EvalTargetVersion: &entity.EvalTargetVersion{
				SourceTargetVersion: version,
				EvalTargetType:      1,
			},
		}, nil)
	}

	pageSize := int32(1)
	req := &eval_target.ListSourceEvalTargetVersionsRequest{
		WorkspaceID:    123,
		TargetType:     &targetType,
		SourceTargetID: "101",
		PageSize:       &pageSize,
		SharedOption: &common.SharedResourceOption{
			IsShared:      gptr.Of(true),
			SourceSpaceID: gptr.Of(int64(456)),
		},
	}
	firstPage, err := app.ListSourceEvalTargetVersions(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, firstPage.Versions, 1)
	assert.Equal(t, "v2", firstPage.Versions[0].GetSourceTargetVersion())
	assert.True(t, firstPage.GetHasMore())
	assert.NotEmpty(t, firstPage.GetNextPageToken())

	req.PageToken = firstPage.NextPageToken
	secondPage, err := app.ListSourceEvalTargetVersions(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, secondPage.Versions, 1)
	assert.Equal(t, "v1", secondPage.Versions[0].GetSourceTargetVersion())
	assert.False(t, secondPage.GetHasMore())
}

func TestEvalTargetApplicationImpl_ListSourceEvalTargets_SharedPassesTargetType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockTypedOperator := mocks.NewMockISourceEvalTargetOperateService(ctrl)
	mockResourceAccess := mocks.NewMockResourceAccessAuthorizer(ctrl)
	targetType := domain_eval_target.EvalTargetType(entity.EvalTargetTypeLoopPrompt)
	app := &EvalTargetApplicationImpl{
		auth:                     mockAuth,
		resourceAccessAuthorizer: mockResourceAccess,
		typedOperators: map[entity.EvalTargetType]service.ISourceEvalTargetOperateService{
			entity.EvalTargetTypeLoopPrompt: mockTypedOperator,
		},
	}
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	mockResourceAccess.EXPECT().ListSharedResources(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *entity.ListSharedResourcesRequest) ([]*entity.ResourceAccessContext, error) {
			assert.Equal(t, entity.SharedResourceTypeEvalTarget, req.ResourceType)
			assert.Equal(t, entity.EvalTargetTypeLoopPrompt, req.TargetType)
			return []*entity.ResourceAccessContext{{
				CallerSpaceID:   123,
				ResourceSpaceID: 456,
				ResourceType:    entity.SharedResourceTypeEvalTarget,
				ResourceID:      101,
				TargetType:      entity.EvalTargetTypeLoopPrompt,
				AccessMode:      entity.AccessModeShared,
				AccessLevel:     entity.SharedAccessLevelReadable,
				VersionPolicy:   entity.SharedVersionPolicyAll,
			}}, nil
		},
	)
	mockTypedOperator.EXPECT().BatchGetSource(gomock.Any(), int64(456), []string{"101"}).Return([]*entity.EvalTarget{{
		SpaceID:        456,
		SourceTargetID: "101",
		EvalTargetType: entity.EvalTargetTypeLoopPrompt,
	}}, nil)

	resp, err := app.ListSourceEvalTargets(context.Background(), &eval_target.ListSourceEvalTargetsRequest{
		WorkspaceID: 123,
		TargetType:  &targetType,
		SharedOption: &common.SharedResourceOption{
			IsShared:      gptr.Of(true),
			SourceSpaceID: gptr.Of(int64(456)),
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.EvalTargets, 1)
	assert.Equal(t, "101", resp.EvalTargets[0].GetSourceTargetID())
}

func TestEvalTargetApplicationImpl_GetEvalTargetVersion_SharedAuthorizesSourceTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalTargetService := mocks.NewMockIEvalTargetService(ctrl)
	mockResourceAccess := mocks.NewMockResourceAccessAuthorizer(ctrl)
	app := &EvalTargetApplicationImpl{
		evalTargetService:        mockEvalTargetService,
		resourceAccessAuthorizer: mockResourceAccess,
	}
	versionID := int64(202)
	mockEvalTargetService.EXPECT().
		GetEvalTargetVersion(gomock.Any(), int64(456), versionID, false).
		Return(&entity.EvalTarget{
			ID:             999,
			SpaceID:        456,
			SourceTargetID: "101",
			EvalTargetType: entity.EvalTargetTypeLoopPrompt,
			EvalTargetVersion: &entity.EvalTargetVersion{
				ID:                  versionID,
				SourceTargetVersion: "v2",
				EvalTargetType:      entity.EvalTargetTypeLoopPrompt,
			},
		}, nil)
	mockResourceAccess.EXPECT().AuthorizeRead(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, req *entity.AuthorizeResourceRequest) (*entity.ResourceAccessContext, error) {
			require.NotNil(t, req.VersionID)
			require.NotNil(t, req.VersionName)
			assert.Equal(t, int64(101), req.ResourceID)
			assert.Equal(t, entity.EvalTargetTypeLoopPrompt, req.TargetType)
			assert.Equal(t, versionID, *req.VersionID)
			assert.Equal(t, "v2", *req.VersionName)
			return &entity.ResourceAccessContext{
				CallerSpaceID:   123,
				ResourceSpaceID: 456,
				ResourceType:    entity.SharedResourceTypeEvalTarget,
				ResourceID:      101,
				TargetType:      entity.EvalTargetTypeLoopPrompt,
				AccessMode:      entity.AccessModeShared,
				AccessLevel:     entity.SharedAccessLevelReadable,
				VersionPolicy:   entity.SharedVersionPolicyAll,
			}, nil
		},
	)

	resp, err := app.GetEvalTargetVersion(context.Background(), &eval_target.GetEvalTargetVersionRequest{
		WorkspaceID:         123,
		EvalTargetVersionID: gptr.Of(versionID),
		SharedOption: &common.SharedResourceOption{
			IsShared:      gptr.Of(true),
			SourceSpaceID: gptr.Of(int64(456)),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetEvalTarget())
	assert.Equal(t, int64(999), resp.GetEvalTarget().GetID())
}
