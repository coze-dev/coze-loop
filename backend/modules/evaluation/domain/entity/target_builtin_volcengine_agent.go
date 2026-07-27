// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type VolcengineAgent struct {
	ID int64

	Name                     string `json:"-"`
	Description              string `json:"-"`
	VolcengineAgentEndpoints []*VolcengineAgentEndpoint
	BaseInfo                 *BaseInfo `json:"-"` // 基础信息
	Protocol                 *VolcengineAgentProtocol
	RuntimeID                *string

	// EnableAnalysis 是否开启分析：创建评测对象时从 application.usages（含 "analysis"）反查固化，
	// 控制 item-complete MQ 是否发送（与 TCC 空间白名单 AND）。
	EnableAnalysis bool `json:"enable_analysis,omitempty"`
}

type VolcengineAgentEndpoint struct {
	EndpointID string
	APIKey     string
}

type VolcengineAgentProtocol = string

const (
	VolcengineAgentProtocolMCP   = "mcp"
	VolcengineAgentProtocolA2A   = "a2a"
	VolcengineAgentProtocolOther = "other"
)
