// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// NoopPipelineListAdapter 当无 ml_flow PipelineService 客户端时的空实现，返回空列表
// 可替换为调用 ml_flow ListPipeline 的真实实现
type NoopPipelineListAdapter struct{}

func NewNoopPipelineListAdapter() rpc.IPipelineListAdapter {
	return &NoopPipelineListAdapter{}
}

func (n *NoopPipelineListAdapter) ListPipelineFlow(ctx context.Context, req *rpc.ListPipelineFlowRequest) (*rpc.ListPipelineFlowResponse, error) {
	return &rpc.ListPipelineFlowResponse{
		Total: 0,
		Items: []*entity.Pipeline{},
	}, nil
}
