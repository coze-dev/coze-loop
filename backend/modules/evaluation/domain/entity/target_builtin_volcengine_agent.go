// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package entity

type VolcengineAgent struct {
	ID int64

	Name                     string `json:"-"`
	Description              string `json:"-"`
	VolcengineAgentEndpoints []*VolcengineAgentEndpoint
	BaseInfo                 *BaseInfo `json:"-"` // 基础信息
}

type VolcengineAgentEndpoint struct {
	EndpointID string
	APIKey     string
}
