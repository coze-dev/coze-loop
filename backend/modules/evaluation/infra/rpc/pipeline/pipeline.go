// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// PipelineListAdapter 查询 ml_flow PipelineService 的适配器；当前无下游客户端时为空实现
// （返回空列表、回调 no-op），可替换为真实 RPC 实现
type PipelineListAdapter struct{}

func NewPipelineListAdapter() *PipelineListAdapter {
	return &PipelineListAdapter{}
}

// NewNoopPipelineListAdapter 与 NewPipelineListAdapter 等价，保留供 wire 与历史调用方使用
func NewNoopPipelineListAdapter() rpc.IPipelineListAdapter {
	return NewPipelineListAdapter()
}

func (a *PipelineListAdapter) ListPipelineFlow(ctx context.Context, req *rpc.ListPipelineFlowRequest) (*rpc.ListPipelineFlowResponse, error) {
	return &rpc.ListPipelineFlowResponse{
		Total: 0,
		Items: []*entity.Pipeline{},
	}, nil
}

func (a *PipelineListAdapter) ListPipelineRun(ctx context.Context, req *rpc.ListPipelineRunRequest) (*rpc.ListPipelineRunResponse, error) {
	return &rpc.ListPipelineRunResponse{
		Items: []*entity.PipelineRun{},
	}, nil
}

func (a *PipelineListAdapter) PipelineNodeFinishCallback(ctx context.Context, experimentID, spaceID int64) error {
	return nil
}
