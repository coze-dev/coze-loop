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
