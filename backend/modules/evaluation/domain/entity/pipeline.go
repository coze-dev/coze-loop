// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// Pipeline Pipeline 结构，参考 pipelinet.Pipeline 定义
type Pipeline struct {
	ID                 *int64      `json:"id,omitempty"`
	Name               *string     `json:"name,omitempty"`
	Description        *string     `json:"description,omitempty"`
	Flow               *FlowSchema `json:"flow,omitempty"` // Flow 使用 FlowSchema 表达，结构参考 workflow_graph.json
	Scheduler          *Scheduler  `json:"scheduler,omitempty"`
	PipelineType       *string     `json:"pipelineType,omitempty"`
	SameAsLatestCommit *bool       `json:"sameAsLatestCommit,omitempty"`
	SpaceID            *int64      `json:"spaceID,omitempty"`
	CreatedBy          *string     `json:"createdBy,omitempty"`
	CreatedAt          *int64      `json:"createdAt,omitempty"`
	UpdatedBy          *string     `json:"updatedBy,omitempty"`
	UpdatedAt          *int64      `json:"updatedAt,omitempty"`
}

// Scheduler 定时触发器配置，参考 pipelinet.Scheduler 定义
type Scheduler struct {
	Enabled   *bool   `json:"enabled,omitempty"`
	Frequency *string `json:"frequency,omitempty"`
	TriggerAt *int64  `json:"trigger_at,omitempty"`
	StartTime *int64  `json:"startTime,omitempty"`
	EndTime   *int64  `json:"endTime,omitempty"`
}

// RefType 引用类型
type RefType string

// NodeTemplateCategory 节点模板分类
type NodeTemplateCategory string

// NodeTemplateType 节点模板类型
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

// PipelineRun Pipeline 运行记录，参考 pipelinet.PipelineRunBrief 定义
type PipelineRun struct {
	ID         *int64  `json:"id,omitempty"`
	PipelineID *int64  `json:"pipelineID,omitempty"`
	VersionID  *int64  `json:"versionID,omitempty"`
	Status     *string `json:"status,omitempty"`
	SpaceID    *int64  `json:"spaceID,omitempty"`
	CreatedBy  *string `json:"createdBy,omitempty"`
	CreatedAt  *int64  `json:"createdAt,omitempty"`
	UpdatedBy  *string `json:"updatedBy,omitempty"`
	UpdatedAt  *int64  `json:"updatedAt,omitempty"`
	StartedAt  *int64  `json:"startedAt,omitempty"`
	EndedAt    *int64  `json:"endedAt,omitempty"`
}
