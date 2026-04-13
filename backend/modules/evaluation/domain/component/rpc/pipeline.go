// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IPipelineListAdapter 查询 ml_flow PipelineService ListPipeline 的适配器接口
//
//go:generate mockgen -destination=./mocks/pipeline_list_adapter.go -package=mocks . IPipelineListAdapter
type IPipelineListAdapter interface {
	ListPipelineFlow(ctx context.Context, req *ListPipelineFlowRequest) (*ListPipelineFlowResponse, error)
	ListPipelineRun(ctx context.Context, req *ListPipelineRunRequest) (*ListPipelineRunResponse, error)
	// PipelineNodeFinishCallback Pipeline 节点完成时回调 ml_flow（例如同步节点状态）；首参为评测实验 ID（ExperimentID）
	PipelineNodeFinishCallback(ctx context.Context, experimentID, spaceID int64) error
}

// ListPipelineFlowRequest ListPipeline 查询请求参数
type ListPipelineFlowRequest struct {
	SpaceID    *int64
	Name       *string
	Page       *int32
	PageSize   *int32
	WithDetail bool    // 为 true 时返回 Flow 详情（含 nodes、edges）
	IDList     []int64 // 按 ID 筛选，非空时只返回指定 ID 的 Pipeline
}

// ListPipelineFlowResponse ListPipeline 查询响应，返回完整 Pipeline 列表
type ListPipelineFlowResponse struct {
	Total int64
	Items []*entity.Pipeline
}

// ListPipelineRunRequest ListPipelineRun 查询请求参数
type ListPipelineRunRequest struct {
	PipelineID *int64
	SpaceID    *int64
	Page       *int32
	PageSize   *int32
}

// ListPipelineRunResponse ListPipelineRun 查询响应
type ListPipelineRunResponse struct {
	Items []*entity.PipelineRun
}
