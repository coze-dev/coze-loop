// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestAuthorizeEvaluatorAccess(t *testing.T) {
	const callerSpaceID, sourceSpaceID, otherSpaceID = int64(200), int64(100), int64(300)
	sharedOption := func() *entity.SharedResourceOption {
		return &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(sourceSpaceID)}
	}

	t.Run("builtin 直接放行且不查白名单", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authorizer := svcmocks.NewMockResourceAccessAuthorizer(ctrl)

		got, err := authorizeEvaluatorAccess(context.Background(), authorizer,
			&entity.Evaluator{ID: 1, SpaceID: sourceSpaceID, Builtin: true}, callerSpaceID, nil, consts.Read, false)
		assert.NoError(t, err)
		assert.False(t, got.IsShared())
		assert.Equal(t, callerSpaceID, got.ResourceSpaceID)
	})

	t.Run("未声明共享且跨空间返回未找到", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authorizer := svcmocks.NewMockResourceAccessAuthorizer(ctrl)

		_, err := authorizeEvaluatorAccess(context.Background(), authorizer,
			&entity.Evaluator{ID: 1, SpaceID: sourceSpaceID}, callerSpaceID, nil, consts.Read, false)
		assert.Error(t, err)
	})

	t.Run("未声明共享且同空间走本空间基础鉴权", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authorizer := svcmocks.NewMockResourceAccessAuthorizer(ctrl)
		authorizer.EXPECT().AuthorizeRead(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, req *entity.AuthorizeResourceRequest) (*entity.ResourceAccessContext, error) {
				assert.Equal(t, entity.SharedResourceTypeEvaluator, req.ResourceType)
				assert.Nil(t, req.SharedOption)
				return &entity.ResourceAccessContext{
					CallerSpaceID:   callerSpaceID,
					ResourceSpaceID: callerSpaceID,
					AccessMode:      entity.AccessModeDirect,
				}, nil
			})

		got, err := authorizeEvaluatorAccess(context.Background(), authorizer,
			&entity.Evaluator{ID: 1, SpaceID: callerSpaceID}, callerSpaceID, nil, consts.Read, false)
		assert.NoError(t, err)
		assert.False(t, got.IsShared())
	})

	t.Run("声明共享且白名单命中", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authorizer := svcmocks.NewMockResourceAccessAuthorizer(ctrl)
		authorizer.EXPECT().AuthorizeRead(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, req *entity.AuthorizeResourceRequest) (*entity.ResourceAccessContext, error) {
				assert.True(t, req.SharedOption.Enabled())
				assert.True(t, req.RequireContentRead)
				return &entity.ResourceAccessContext{
					CallerSpaceID:   callerSpaceID,
					ResourceSpaceID: sourceSpaceID,
					AccessMode:      entity.AccessModeShared,
					AccessLevel:     entity.SharedAccessLevelReadable,
				}, nil
			})

		got, err := authorizeEvaluatorAccess(context.Background(), authorizer,
			&entity.Evaluator{ID: 1, SpaceID: sourceSpaceID}, callerSpaceID, sharedOption(), consts.Read, true)
		assert.NoError(t, err)
		assert.True(t, got.IsShared())
		assert.Equal(t, sourceSpaceID, got.ResourceSpaceID)
	})

	t.Run("白名单命中的来源空间与评估器真实空间不一致时拒绝", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authorizer := svcmocks.NewMockResourceAccessAuthorizer(ctrl)
		authorizer.EXPECT().AuthorizeRead(gomock.Any(), gomock.Any()).Return(&entity.ResourceAccessContext{
			CallerSpaceID:   callerSpaceID,
			ResourceSpaceID: sourceSpaceID,
			AccessMode:      entity.AccessModeShared,
		}, nil)

		_, err := authorizeEvaluatorAccess(context.Background(), authorizer,
			&entity.Evaluator{ID: 1, SpaceID: otherSpaceID}, callerSpaceID, sharedOption(), consts.Read, false)
		assert.Error(t, err)
	})

	t.Run("授权失败原样透出", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authorizer := svcmocks.NewMockResourceAccessAuthorizer(ctrl)
		authorizer.EXPECT().AuthorizeRead(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

		_, err := authorizeEvaluatorAccess(context.Background(), authorizer,
			&entity.Evaluator{ID: 1, SpaceID: sourceSpaceID}, callerSpaceID, sharedOption(), consts.Read, false)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("evaluator 为 nil 时报错", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		authorizer := svcmocks.NewMockResourceAccessAuthorizer(ctrl)

		_, err := authorizeEvaluatorAccess(context.Background(), authorizer, nil, callerSpaceID, nil, consts.Read, false)
		assert.Error(t, err)
	})
}
