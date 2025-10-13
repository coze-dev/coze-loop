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
}

type VolcengineAgentEndpoint struct {
	EndpointID string
	APIKey     string
}

type VolcengineAgentProtocol int64

const (
	VolcengineAgentProtocol_MCP   VolcengineAgentProtocol = 1
	VolcengineAgentProtocol_A2A   VolcengineAgentProtocol = 2
	VolcengineAgentProtocol_Other VolcengineAgentProtocol = 3
)
