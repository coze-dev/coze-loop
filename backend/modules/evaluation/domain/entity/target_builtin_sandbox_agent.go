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

// IsDualSandbox 判断 SandboxAgent 是否处于双沙箱模式；nil / 未填字段一律按 Single 处理。
func (a *SandboxAgent) IsDualSandbox() bool {
	if a == nil {
		return false
	}
	return ResolveSandboxCountMode(a.SandboxCountMode) == SandboxCountModeDual
}

type SandboxEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SandboxAgent struct {
	Name          string           `json:"name"`
	Type          SandboxAgentType `json:"type"`
	ModelName     string           `json:"model_name"`
	// ModelID 平台模型服务 model_id。填写后被测 Agent 模型密钥可经 GetModelAndAccount 解析，
	// 与 SUA 的 sua_model_id 对称；缺省 0 时仅用 ModelName + TCC 替换规则（行为不变）。
	ModelID       int64            `json:"model_id,omitempty"`
	AgentSetupCmd string           `json:"agent_setup_cmd"`
	AgentRunCmd   string           `json:"agent_run_cmd"`
	Envs          []*SandboxEnvVar `json:"envs"`
	Image         string           `json:"image"`
	// 自定义输出结果，与 CustomRPCServer.CustomFieldSchemas 对齐
	CustomFieldSchemas []*CustomFieldSchema `json:"custom_field_schemas,omitempty"`
	// EnableAnalysis 是否开启分析：创建评测对象时从 application.usages（含 "analysis"）反查固化，
	// 控制 item-complete MQ 是否发送（与 TCC 空间白名单 AND）。
	EnableAnalysis bool `json:"enable_analysis,omitempty"`
	// SandboxCountMode 单/双沙箱模式；未填 / 未识别一律按 Single 处理。
	SandboxCountMode SandboxCountMode `json:"sandbox_count_mode,omitempty"`
}
