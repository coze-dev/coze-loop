// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	datasetdto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset"
	domain_dataset "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
)

// UT-EVAL-01(adapter 层): CountDatasets 构造 CountDatasetsRequest{WorkspaceID, Category=Evaluation}，
// 回传 resp.Total；下游 err / 非 0 BaseResp 透传，不吞错返回 0。
func TestDatasetRPCAdapter_CountDatasets(t *testing.T) {
	ctx := context.Background()

	t.Run("SpaceID 透传 + Category 固定 Evaluation + Total 回传", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		adapter, mockClient := newTestAdapter(ctrl)

		var captured *datasetdto.CountDatasetsRequest
		mockClient.EXPECT().
			CountDatasets(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *datasetdto.CountDatasetsRequest, _ ...interface{}) (*datasetdto.CountDatasetsResponse, error) {
				captured = req
				return &datasetdto.CountDatasetsResponse{Total: gptr.Of(int64(7))}, nil
			})

		total, err := adapter.CountDatasets(ctx, 100)
		assert.NoError(t, err)
		assert.Equal(t, int64(7), total)

		assert.NotNil(t, captured)
		assert.Equal(t, int64(100), captured.WorkspaceID)
		assert.NotNil(t, captured.Category)
		assert.Equal(t, domain_dataset.DatasetCategory_Evaluation, *captured.Category)
	})

	t.Run("下游 RPC error 透传，不返回 0 兜底", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		adapter, mockClient := newTestAdapter(ctrl)

		rpcErr := errors.New("rpc down")
		mockClient.EXPECT().
			CountDatasets(gomock.Any(), gomock.Any()).
			Return(nil, rpcErr)

		total, err := adapter.CountDatasets(ctx, 100)
		assert.Error(t, err)
		assert.Equal(t, rpcErr, err)
		assert.Equal(t, int64(0), total)
	})

	t.Run("非 0 BaseResp 透传为 error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().
			CountDatasets(gomock.Any(), gomock.Any()).
			Return(&datasetdto.CountDatasetsResponse{
				BaseResp: &base.BaseResp{StatusCode: 601, StatusMessage: "biz err"},
			}, nil)

		total, err := adapter.CountDatasets(ctx, 100)
		assert.Error(t, err)
		assert.Equal(t, int64(0), total)
	})
}
