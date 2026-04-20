// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
)

func TestPipelineListAdapter_ListPipelineFlow(t *testing.T) {
	adapter := NewPipelineListAdapter()
	ctx := context.Background()

	t.Run("返回空列表", func(t *testing.T) {
		spaceID := int64(123)
		pageSize := int32(10)
		req := &rpc.ListPipelineFlowRequest{
			SpaceID:  &spaceID,
			PageSize: &pageSize,
		}

		resp, err := adapter.ListPipelineFlow(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, int64(0), resp.Total)
		assert.Len(t, resp.Items, 0)
	})

	t.Run("请求参数为空也返回空列表", func(t *testing.T) {
		resp, err := adapter.ListPipelineFlow(ctx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, int64(0), resp.Total)
		assert.Len(t, resp.Items, 0)
	})
}

func TestPipelineListAdapter_PipelineNodeFinishCallback(t *testing.T) {
	adapter := NewPipelineListAdapter()
	err := adapter.PipelineNodeFinishCallback(context.Background(), 1, 2)
	assert.NoError(t, err)
}

func TestNewPipelineListAdapter(t *testing.T) {
	adapter := NewPipelineListAdapter()
	assert.NotNil(t, adapter)
	assert.IsType(t, &PipelineListAdapter{}, adapter)
}

func TestNewNoopPipelineListAdapter(t *testing.T) {
	adapter := NewNoopPipelineListAdapter()
	assert.NotNil(t, adapter)
	assert.IsType(t, &PipelineListAdapter{}, adapter)
}
