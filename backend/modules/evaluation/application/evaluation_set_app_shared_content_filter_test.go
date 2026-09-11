// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domain_common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	userinfomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func sharedAccessCtx(sourceSpaceID, resourceID int64) *entity.ResourceAccessContext {
	return &entity.ResourceAccessContext{
		ResourceSpaceID: sourceSpaceID,
		ResourceID:      resourceID,
		AccessMode:      entity.AccessModeShared,
		AccessLevel:     entity.SharedAccessLevelReadable,
	}
}

// name / tagFilter 皆空 → 原样返回且零 RPC。这是放开内容过滤后最关键的回归断言：
// 不带过滤的共享列表必须与改动前完全一致。
func TestFilterSharedAccessContextsByContent_NoFilterNoRPC(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
	mockSvc.EXPECT().ListEvaluationSets(gomock.Any(), gomock.Any()).Times(0)

	accessCtxs := []*entity.ResourceAccessContext{sharedAccessCtx(10, 1), sharedAccessCtx(10, 2)}

	for _, tt := range []struct {
		name      string
		filterArg *string
	}{
		{"name 为 nil", nil},
		{"name 为空串", gptr.Of("")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterSharedAccessContextsByContent(context.Background(), mockSvc, accessCtxs, tt.filterArg, nil)
			require.NoError(t, err)
			assert.Equal(t, accessCtxs, got)
		})
	}

	t.Run("空集合", func(t *testing.T) {
		got, err := filterSharedAccessContextsByContent(context.Background(), mockSvc, nil, gptr.Of("x"), nil)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

// 跨两个来源空间、每空间各命中一部分 → 白名单求交后只留命中的。
func TestFilterSharedAccessContextsByContent_MultiSourceSpaces(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)

	mockSvc.EXPECT().ListEvaluationSets(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(
		func(_ context.Context, param *entity.ListEvaluationSetsParam) ([]*entity.EvaluationSet, *int64, *string, error) {
			require.NotNil(t, param.SharedOption)
			assert.True(t, param.SharedOption.IsShared)
			assert.Equal(t, param.SpaceID, gptr.Indirect(param.SharedOption.SourceSpaceID))
			assert.Equal(t, "hit", gptr.Indirect(param.Name))
			switch param.SpaceID {
			case 10:
				return []*entity.EvaluationSet{{ID: 2, SpaceID: 10}}, nil, nil, nil
			case 20:
				return []*entity.EvaluationSet{{ID: 4, SpaceID: 20}}, nil, nil, nil
			}
			return nil, nil, nil, nil
		})

	got, err := filterSharedAccessContextsByContent(context.Background(), mockSvc, []*entity.ResourceAccessContext{
		sharedAccessCtx(10, 1), sharedAccessCtx(10, 2), sharedAccessCtx(20, 3), sharedAccessCtx(20, 4),
	}, gptr.Of("hit"), nil)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, int64(2), got[0].ResourceID)
	assert.Equal(t, int64(4), got[1].ResourceID)
}

// ID 数超过一页上限 → 按 maxSharedPageSize 切块，块数正确。
func TestFilterSharedAccessContextsByContent_ChunksByPageSize(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)

	accessCtxs := make([]*entity.ResourceAccessContext, 0, maxSharedPageSize*2+1)
	for i := 1; i <= maxSharedPageSize*2+1; i++ {
		accessCtxs = append(accessCtxs, sharedAccessCtx(10, int64(i)))
	}

	var chunkSizes []int
	mockSvc.EXPECT().ListEvaluationSets(gomock.Any(), gomock.Any()).Times(3).DoAndReturn(
		func(_ context.Context, param *entity.ListEvaluationSetsParam) ([]*entity.EvaluationSet, *int64, *string, error) {
			chunkSizes = append(chunkSizes, len(param.EvaluationSetIDs))
			return nil, nil, nil, nil
		})

	got, err := filterSharedAccessContextsByContent(context.Background(), mockSvc, accessCtxs,
		nil, &entity.TagFilter{TagNames: []string{"t"}, Relation: entity.TagFilterRelationOr})
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Equal(t, []int{maxSharedPageSize, maxSharedPageSize, 1}, chunkSizes)
}

func TestFilterSharedAccessContextsByContent_DownstreamError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
	mockSvc.EXPECT().ListEvaluationSets(gomock.Any(), gomock.Any()).Return(nil, nil, nil, errors.New("rpc down"))

	_, err := filterSharedAccessContextsByContent(context.Background(), mockSvc,
		[]*entity.ResourceAccessContext{sharedAccessCtx(10, 1)}, gptr.Of("x"), nil)
	assert.Error(t, err)
}

func newSharedEvalSetApp(ctrl *gomock.Controller) (*EvaluationSetApplicationImpl, *servicemocks.MockIEvaluationSetService, *servicemocks.MockResourceAccessAuthorizer) {
	mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
	mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
	mockAuthorizer := servicemocks.NewMockResourceAccessAuthorizer(ctrl)
	mockUserInfo := userinfomocks.NewMockUserInfoService(ctrl)
	mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockUserInfo.EXPECT().PackUserInfo(gomock.Any(), gomock.Any()).AnyTimes()
	return &EvaluationSetApplicationImpl{
		auth:                     mockAuth,
		evaluationSetService:     mockSvc,
		resourceAccessAuthorizer: mockAuthorizer,
		userInfoService:          mockUserInfo,
	}, mockSvc, mockAuthorizer
}

// 共享模式下 name 与 tag_filter 已被接受，且过滤发生在分页之前 —— total 与翻页都按过滤后的集合算。
func TestEvaluationSetApplicationImpl_ListSharedEvaluationSets_NameAndTagFilterAccepted(t *testing.T) {
	const workspaceID = int64(1001)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app, mockSvc, mockAuthorizer := newSharedEvalSetApp(ctrl)

	mockAuthorizer.EXPECT().ListSharedResources(gomock.Any(), gomock.Any()).Return([]*entity.ResourceAccessContext{
		sharedAccessCtx(10, 1), sharedAccessCtx(10, 2), sharedAccessCtx(10, 3),
	}, nil)
	mockSvc.EXPECT().ListEvaluationSets(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, param *entity.ListEvaluationSetsParam) ([]*entity.EvaluationSet, *int64, *string, error) {
			assert.Equal(t, int64(10), param.SpaceID)
			assert.Equal(t, "hit", gptr.Indirect(param.Name))
			assert.Equal(t, &entity.TagFilter{TagNames: []string{"tag-a"}, Relation: entity.TagFilterRelationOr}, param.TagFilter)
			return []*entity.EvaluationSet{{ID: 2, SpaceID: 10}, {ID: 3, SpaceID: 10}}, nil, nil, nil
		})
	mockSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gptr.Of(workspaceID), []int64{2}, gomock.Any(), gomock.Any()).Return(
		[]*entity.EvaluationSet{{ID: 2, SpaceID: 10}}, nil)

	resp, err := app.ListEvaluationSets(context.Background(), &eval_set.ListEvaluationSetsRequest{
		WorkspaceID:  workspaceID,
		Name:         gptr.Of("hit"),
		TagFilter:    &domain_eval_set.TagFilter{TagNames: []string{"tag-a"}},
		PageSize:     gptr.Of(int32(1)),
		SharedOption: &domain_common.SharedResourceOption{IsShared: gptr.Of(true)},
	})
	require.NoError(t, err)
	require.Len(t, resp.EvaluationSets, 1)
	assert.Equal(t, int64(2), resp.EvaluationSets[0].GetID())
	// total 是过滤后的数量（3 个共享资源里只有 2 个命中），不是共享配置的全量。
	assert.Equal(t, int64(2), resp.GetTotal())
	require.NotNil(t, resp.NextPageToken)
}

// 下游一条都没命中 → 空列表 + total=0，不报错。
func TestEvaluationSetApplicationImpl_ListSharedEvaluationSets_NoMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app, mockSvc, mockAuthorizer := newSharedEvalSetApp(ctrl)

	mockAuthorizer.EXPECT().ListSharedResources(gomock.Any(), gomock.Any()).Return([]*entity.ResourceAccessContext{
		sharedAccessCtx(10, 1),
	}, nil)
	mockSvc.EXPECT().ListEvaluationSets(gomock.Any(), gomock.Any()).Return(nil, nil, nil, nil)
	mockSvc.EXPECT().BatchGetEvaluationSets(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	resp, err := app.ListEvaluationSets(context.Background(), &eval_set.ListEvaluationSetsRequest{
		WorkspaceID:  1001,
		Name:         gptr.Of("nope"),
		SharedOption: &domain_common.SharedResourceOption{IsShared: gptr.Of(true)},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.EvaluationSets)
	assert.Equal(t, int64(0), resp.GetTotal())
	assert.Nil(t, resp.NextPageToken)
}

// 守卫仍拒的四项：creators / type / dataset_keys / order_bys。
func TestEvaluationSetApplicationImpl_ListSharedEvaluationSets_StillRejectedFilters(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mutuate func(*eval_set.ListEvaluationSetsRequest)
	}{
		{"creators", func(r *eval_set.ListEvaluationSetsRequest) { r.Creators = []string{"u1"} }},
		{"type", func(r *eval_set.ListEvaluationSetsRequest) {
			r.Type = gptr.Of(domain_eval_set.EvaluationSetTypeDefault)
		}},
		{"dataset_keys", func(r *eval_set.ListEvaluationSetsRequest) { r.DatasetKeys = []string{"k"} }},
		{"order_bys", func(r *eval_set.ListEvaluationSetsRequest) {
			r.OrderBys = []*domain_common.OrderBy{{Field: gptr.Of("name")}}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			app, mockSvc, mockAuthorizer := newSharedEvalSetApp(ctrl)
			mockAuthorizer.EXPECT().ListSharedResources(gomock.Any(), gomock.Any()).Times(0)
			mockSvc.EXPECT().ListEvaluationSets(gomock.Any(), gomock.Any()).Times(0)

			req := &eval_set.ListEvaluationSetsRequest{
				WorkspaceID:  1001,
				SharedOption: &domain_common.SharedResourceOption{IsShared: gptr.Of(true)},
			}
			tt.mutuate(req)
			_, err := app.ListEvaluationSets(context.Background(), req)
			assert.Error(t, err)
		})
	}
}
