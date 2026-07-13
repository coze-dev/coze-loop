// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// UT-EVAL-01(app 层): GetEvaluationSetsCount 鉴权后委托 service.CountEvaluationSets，
// 组装 GetEvaluationSetsCountResponse{Total}；service err 透传，不吞错。
func TestEvaluationSetApplicationImpl_GetEvaluationSetsCount(t *testing.T) {
	workspaceID := int64(1001)

	t.Run("req 为 nil 返回参数错误", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		app := &EvaluationSetApplicationImpl{
			auth:                 rpcmocks.NewMockIAuthProvider(ctrl),
			evaluationSetService: servicemocks.NewMockIEvaluationSetService(ctrl),
		}
		resp, err := app.GetEvaluationSetsCount(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("鉴权失败直接返回，不调 service", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
		mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
		app := &EvaluationSetApplicationImpl{auth: mockAuth, evaluationSetService: mockSvc}

		mockAuth.EXPECT().
			Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).
			Return(errorx.NewByCode(errno.CommonNoPermissionCode))
		mockSvc.EXPECT().CountEvaluationSets(gomock.Any(), gomock.Any()).Times(0)

		resp, err := app.GetEvaluationSetsCount(context.Background(), &eval_set.GetEvaluationSetsCountRequest{WorkspaceID: workspaceID})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("SpaceID 透传 service 且 Total 正确回传", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
		mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
		app := &EvaluationSetApplicationImpl{auth: mockAuth, evaluationSetService: mockSvc}

		mockAuth.EXPECT().
			Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).
			Return(nil)
		mockSvc.EXPECT().
			CountEvaluationSets(gomock.Any(), workspaceID).
			Return(int64(5), nil)

		resp, err := app.GetEvaluationSetsCount(context.Background(), &eval_set.GetEvaluationSetsCountRequest{WorkspaceID: workspaceID})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, int64(5), resp.GetTotal())
	})

	t.Run("service 返回 error 透传，不吞错返回 0", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
		mockSvc := servicemocks.NewMockIEvaluationSetService(ctrl)
		app := &EvaluationSetApplicationImpl{auth: mockAuth, evaluationSetService: mockSvc}

		svcErr := errorx.NewByCode(errno.CommonInternalErrorCode)
		mockAuth.EXPECT().
			Authorization(gomock.Any(), gomock.AssignableToTypeOf(&rpc.AuthorizationParam{})).
			Return(nil)
		mockSvc.EXPECT().
			CountEvaluationSets(gomock.Any(), workspaceID).
			Return(int64(0), svcErr)

		resp, err := app.GetEvaluationSetsCount(context.Background(), &eval_set.GetEvaluationSetsCountRequest{WorkspaceID: workspaceID})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}
