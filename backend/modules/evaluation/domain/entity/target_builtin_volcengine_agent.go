// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package entity

type VolcengineAgent struct {
	ID int64

	Name                     string
	Description              string
	VolcengineAgentEndpoints []*VolcengineAgentEndpoint
}

type VolcengineAgentEndpoint struct {
	EndpointID string
	APIKey     string
}
