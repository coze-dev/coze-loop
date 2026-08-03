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

	domain_common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestEvaluationSetApplicationImpl_ListEvaluationSetItems_NonSharedMatchesMain(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		workspaceID = int64(1001)
		setID       = int64(2001)
	)
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
	mockItemSvc := servicemocks.NewMockEvaluationSetItemService(ctrl)
	app := &EvaluationSetApplicationImpl{
		auth:                     mockAuth,
		evaluationSetService:     mockSetSvc,
		evaluationSetItemService: mockItemSvc,
		resourceAccessAuthorizer: servicemocks.NewMockResourceAccessAuthorizer(ctrl),
	}

	set := &entity.EvaluationSet{ID: setID, SpaceID: workspaceID}
	mockSetSvc.EXPECT().
		GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), setID, gptr.Of(true), nil).
		Return(set, nil)
	mockAuth.EXPECT().
		AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, param *rpc.AuthorizationWithoutSPIParam) error {
			assert.Equal(t, workspaceID, param.SpaceID)
			assert.Equal(t, workspaceID, param.ResourceSpaceID)
			require.Len(t, param.ActionObjects, 1)
			assert.Equal(t, consts.Read, gptr.Indirect(param.ActionObjects[0].Action))
			return nil
		})
	total := int64(0)
	mockItemSvc.EXPECT().
		ListEvaluationSetItems(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, param *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
			assert.Equal(t, workspaceID, param.SpaceID)
			assert.Equal(t, setID, param.EvaluationSetID)
			assert.Nil(t, param.VersionID, "non-shared requests must keep main's draft/current behavior")
			return nil, &total, nil, nil, nil
		})

	resp, err := app.ListEvaluationSetItems(context.Background(), &eval_set.ListEvaluationSetItemsRequest{
		WorkspaceID:     workspaceID,
		EvaluationSetID: setID,
		SharedOption: &domain_common.SharedResourceOption{
			IsShared:      gptr.Of(false),
			SourceSpaceID: gptr.Of(int64(3001)),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, total, resp.GetTotal())
}

func TestEvaluationSetApplicationImpl_ListEvaluationSetItems_SharedRequiresVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	app := &EvaluationSetApplicationImpl{}
	resp, err := app.ListEvaluationSetItems(context.Background(), &eval_set.ListEvaluationSetItemsRequest{
		WorkspaceID:     1001,
		EvaluationSetID: 2001,
		SharedOption: &domain_common.SharedResourceOption{
			IsShared:      gptr.Of(true),
			SourceSpaceID: gptr.Of(int64(3001)),
		},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	statusErr, ok := errorx.FromStatusError(err)
	require.True(t, ok)
	assert.EqualValues(t, errno.CommonInvalidParamCode, statusErr.Code())
}

func TestEvaluationSetApplicationImpl_ListEvaluationSetItems_SharedSpecifiedAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		workspaceID = int64(1001)
		sourceID    = int64(3001)
		setID       = int64(2001)
		versionID   = int64(4001)
	)
	mockSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
	mockVersionSvc := servicemocks.NewMockEvaluationSetVersionService(ctrl)
	mockItemSvc := servicemocks.NewMockEvaluationSetItemService(ctrl)
	mockAuthorizer := servicemocks.NewMockResourceAccessAuthorizer(ctrl)
	app := &EvaluationSetApplicationImpl{
		evaluationSetService:        mockSetSvc,
		evaluationSetVersionService: mockVersionSvc,
		evaluationSetItemService:    mockItemSvc,
		resourceAccessAuthorizer:    mockAuthorizer,
	}

	set := &entity.EvaluationSet{ID: setID, SpaceID: sourceID, LatestVersion: "v2"}
	version := &entity.EvaluationSetVersion{ID: versionID, EvaluationSetID: setID, SpaceID: sourceID, Version: "v1"}
	sharedOption := &domain_common.SharedResourceOption{
		IsShared:      gptr.Of(true),
		SourceSpaceID: gptr.Of(sourceID),
	}
	mockSetSvc.EXPECT().
		GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), setID, gptr.Of(true), gomock.Any()).
		Return(set, nil)
	mockAuthorizer.EXPECT().
		AuthorizeRead(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *entity.AuthorizeResourceRequest) (*entity.ResourceAccessContext, error) {
			require.NotNil(t, req.VersionID)
			assert.Equal(t, versionID, *req.VersionID)
			assert.True(t, req.RequireContentRead)
			return &entity.ResourceAccessContext{
				CallerSpaceID:   workspaceID,
				ResourceSpaceID: sourceID,
				AccessMode:      entity.AccessModeShared,
				AccessLevel:     entity.SharedAccessLevelReadable,
				VersionPolicy:   entity.SharedVersionPolicySpecified,
				SpecifiedIDs:    []int64{versionID},
			}, nil
		})
	mockVersionSvc.EXPECT().
		GetEvaluationSetVersion(gomock.Any(), workspaceID, versionID, gptr.Of(true), gomock.Any()).
		Return(version, set, nil)
	total := int64(0)
	mockItemSvc.EXPECT().
		ListEvaluationSetItems(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, param *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
			assert.Equal(t, sourceID, param.SpaceID)
			require.NotNil(t, param.VersionID)
			assert.Equal(t, versionID, *param.VersionID)
			return nil, &total, nil, nil, nil
		})

	resp, err := app.ListEvaluationSetItems(context.Background(), &eval_set.ListEvaluationSetItemsRequest{
		WorkspaceID:     workspaceID,
		EvaluationSetID: setID,
		VersionID:       gptr.Of(versionID),
		SharedOption:    sharedOption,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestEvaluationSetApplicationImpl_ListEvaluationSetItems_SharedLatestRejectsHistoricalVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		workspaceID = int64(1001)
		sourceID    = int64(3001)
		setID       = int64(2001)
		versionID   = int64(4001)
	)
	mockSetSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
	mockVersionSvc := servicemocks.NewMockEvaluationSetVersionService(ctrl)
	mockAuthorizer := servicemocks.NewMockResourceAccessAuthorizer(ctrl)
	app := &EvaluationSetApplicationImpl{
		evaluationSetService:        mockSetSvc,
		evaluationSetVersionService: mockVersionSvc,
		resourceAccessAuthorizer:    mockAuthorizer,
	}

	set := &entity.EvaluationSet{ID: setID, SpaceID: sourceID, LatestVersion: "v2"}
	historicalVersion := &entity.EvaluationSetVersion{
		ID:              versionID,
		EvaluationSetID: setID,
		SpaceID:         sourceID,
		Version:         "v1",
	}
	mockSetSvc.EXPECT().
		GetEvaluationSet(gomock.Any(), gptr.Of(workspaceID), setID, gptr.Of(true), gomock.Any()).
		Return(set, nil)
	mockAuthorizer.EXPECT().
		AuthorizeRead(gomock.Any(), gomock.Any()).
		Return(&entity.ResourceAccessContext{
			CallerSpaceID:   workspaceID,
			ResourceSpaceID: sourceID,
			AccessMode:      entity.AccessModeShared,
			AccessLevel:     entity.SharedAccessLevelReadable,
			VersionPolicy:   entity.SharedVersionPolicyLatest,
		}, nil)
	mockVersionSvc.EXPECT().
		GetEvaluationSetVersion(gomock.Any(), workspaceID, versionID, gptr.Of(true), gomock.Any()).
		Return(historicalVersion, set, nil)

	resp, err := app.ListEvaluationSetItems(context.Background(), &eval_set.ListEvaluationSetItemsRequest{
		WorkspaceID:     workspaceID,
		EvaluationSetID: setID,
		VersionID:       gptr.Of(versionID),
		SharedOption: &domain_common.SharedResourceOption{
			IsShared:      gptr.Of(true),
			SourceSpaceID: gptr.Of(sourceID),
		},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	statusErr, ok := errorx.FromStatusError(err)
	require.True(t, ok)
	assert.EqualValues(t, errno.ResourceNotFoundCode, statusErr.Code())
}
