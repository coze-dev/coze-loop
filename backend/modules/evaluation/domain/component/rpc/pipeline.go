// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import "context"

// IPipelineListAdapter 查询 ml_flow PipelineService ListPipeline 的适配器接口
//go:generate mockgen -destination=./mocks/pipeline_list_adapter.go -package=mocks . IPipelineListAdapter
type IPipelineListAdapter interface {
	ListPipelineFlow(ctx context.Context, req *ListPipelineFlowRequest) (*ListPipelineFlowResponse, error)
}

// ListPipelineFlowRequest ListPipeline 查询请求参数
type ListPipelineFlowRequest struct {
	SpaceID    *int64
	Name       *string
	Page       *int32
	PageSize   *int32
	WithDetail bool // 为 true 时返回 Flow 详情（含 nodes、edges）
}

// PipelineFlowItem 单个 Pipeline 的 Flow 信息
type PipelineFlowItem struct {
	PipelineID int64
	Name       string
	Flow       *FlowSchema
}

// ListPipelineFlowResponse ListPipeline 查询响应
type ListPipelineFlowResponse struct {
	Total int64
	Items []*PipelineFlowItem
}

// RefType 引用类型
type RefType string
type NodeTemplateCategory string
type NodeTemplateType string

// FlowSchema Pipeline 画布结构，承载 nodes 和 edges
type FlowSchema struct {
	Nodes []*Node `json:"nodes,omitempty"`
	Edges []*Edge `json:"edges,omitempty"`
}

// Node 画布节点
type Node struct {
	ID                   string               `json:"id,omitempty"`
	NodeTemplateCategory NodeTemplateCategory `json:"node_template_category,omitempty"`
	NodeTemplateType     NodeTemplateType     `json:"node_template_type,omitempty"`
	Refs                 map[string]*NodeRef  `json:"refs,omitempty"`
}

// NodeRef 节点引用
type NodeRef struct {
	Type    RefType `json:"type,omitempty"`
	Content string  `json:"content,omitempty"`
}

// Edge 画布边，连接两个节点
type Edge struct {
	Source string `json:"source,omitempty"`
	Target string `json:"target,omitempty"`
}
