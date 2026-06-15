// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type SandboxAgentType string

const (
	SandboxAgentTypeSingleRunCLI SandboxAgentType = "single_run_cli"
)

type SandboxEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SandboxAgent struct {
	Name          string           `json:"name"`
	Type          SandboxAgentType `json:"type"`
	ModelName     string           `json:"model_name"`
	AgentSetupCmd string           `json:"agent_setup_cmd"`
	AgentRunCmd   string           `json:"agent_run_cmd"`
	Envs          []*SandboxEnvVar `json:"envs"`
}
