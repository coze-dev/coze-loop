// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/manage"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	serviceMocks "github.com/coze-dev/coze-loop/backend/modules/llm/domain/service/mocks"
)

func TestManageApp_ListModels(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSrv := serviceMocks.NewMockIManage(ctrl)
	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	app := NewManageApplication(mockSrv, mockAuth)

	ctx := context.Background()

	t.Run("auth_error", func(t *testing.T) {
		mockAuth.EXPECT().CheckSpacePermission(ctx, int64(1), "listModels").Return(assert.AnError)
		req := &manage.ListModelsRequest{WorkspaceID: gptr.Of(int64(1))}
		res, err := app.ListModels(ctx, req)
		assert.Error(t, err)
		assert.NotNil(t, res)
	})

	t.Run("success", func(t *testing.T) {
		mockAuth.EXPECT().CheckSpacePermission(ctx, int64(1), "listModels").Return(nil)
		mockSrv.EXPECT().ListModels(ctx, gomock.Any()).Return([]*entity.Model{{ID: 1}}, int64(1), false, int64(0), nil)

		req := &manage.ListModelsRequest{
			WorkspaceID: gptr.Of(int64(1)),
			Scenario:    gptr.Of(common.ScenarioEvaluator),
			PageToken:   gptr.Of("0"),
			PageSize:    gptr.Of(int32(10)),
		}
		res, err := app.ListModels(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, res.Models, 1)
		assert.Equal(t, int32(1), *res.Total)
	})

	t.Run("success_no_scenario", func(t *testing.T) {
		mockAuth.EXPECT().CheckSpacePermission(ctx, int64(1), "listModels").Return(nil)
		mockSrv.EXPECT().ListModels(ctx, gomock.Any()).Return(nil, int64(0), false, int64(0), nil)

		req := &manage.ListModelsRequest{WorkspaceID: gptr.Of(int64(1))}
		_, err := app.ListModels(ctx, req)
		assert.NoError(t, err)
	})

	t.Run("srv_error", func(t *testing.T) {
		mockAuth.EXPECT().CheckSpacePermission(ctx, int64(1), "listModels").Return(nil)
		mockSrv.EXPECT().ListModels(ctx, gomock.Any()).Return(nil, int64(0), false, int64(0), assert.AnError)

		req := &manage.ListModelsRequest{WorkspaceID: gptr.Of(int64(1))}
		_, err := app.ListModels(ctx, req)
		assert.Error(t, err)
	})
}

func TestManageApp_GetModel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSrv := serviceMocks.NewMockIManage(ctrl)
	mockAuth := mocks.NewMockIAuthProvider(ctrl)
	app := NewManageApplication(mockSrv, mockAuth)

	ctx := context.Background()

	t.Run("auth_error", func(t *testing.T) {
		mockAuth.EXPECT().CheckSpacePermission(ctx, int64(1), "getModel").Return(assert.AnError)
		req := &manage.GetModelRequest{WorkspaceID: gptr.Of(int64(1)), ModelID: gptr.Of(int64(1))}
		_, err := app.GetModel(ctx, req)
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		mockAuth.EXPECT().CheckSpacePermission(ctx, int64(1), "getModel").Return(nil)
		mockSrv.EXPECT().GetModelByID(ctx, int64(100)).Return(&entity.Model{ID: 100}, nil)

		req := &manage.GetModelRequest{WorkspaceID: gptr.Of(int64(1)), ModelID: gptr.Of(int64(100))}
		res, err := app.GetModel(ctx, req)
		assert.NoError(t, err)
		assert.Equal(t, int64(100), *res.Model.ModelID)
	})

	t.Run("srv_error", func(t *testing.T) {
		mockAuth.EXPECT().CheckSpacePermission(ctx, int64(1), "getModel").Return(nil)
		mockSrv.EXPECT().GetModelByID(ctx, int64(100)).Return(nil, assert.AnError)

		req := &manage.GetModelRequest{WorkspaceID: gptr.Of(int64(1)), ModelID: gptr.Of(int64(100))}
		_, err := app.GetModel(ctx, req)
		assert.Error(t, err)
	})
}
