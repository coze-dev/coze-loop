// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type SandboxAgentType string

const (
	SandboxAgentTypeSingleRunCLI SandboxAgentType = "single_run_cli"
)

// SandboxCountMode 指定单次评测使用一个还是一对沙箱。
// - Single: 沿用原有单沙箱执行链路
// - Dual:   先起一个从属沙箱拿到 session id，再起一个主沙箱运行 sandbox-pipeline
type SandboxCountMode string

const (
	SandboxCountModeSingle SandboxCountMode = "single"
	SandboxCountModeDual   SandboxCountMode = "dual"
)

// ResolveSandboxCountMode 空/未识别值一律回退到 Single，保持默认行为。
func ResolveSandboxCountMode(mode SandboxCountMode) SandboxCountMode {
	if mode == SandboxCountModeDual {
		return SandboxCountModeDual
	}
	return SandboxCountModeSingle
}

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
	Image         string           `json:"image"`
	// 自定义输出结果，与 CustomRPCServer.CustomFieldSchemas 对齐
	CustomFieldSchemas []*CustomFieldSchema `json:"custom_field_schemas,omitempty"`
	// SandboxCountMode 单/双沙箱模式；未填 / 未识别一律按 Single 处理。
	SandboxCountMode SandboxCountMode `json:"sandbox_count_mode,omitempty"`
}
