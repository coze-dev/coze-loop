// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type SandboxAgentType string

const (
	SandboxAgentTypeSingleRunCLI SandboxAgentType = "single_run_cli"
)

// SandboxAgentExtKeyExtraExecuteID 是 EvalTargetOutputData.Ext 里用于携带
// SandboxAgent 评测对象**额外沙箱 execute id** 的 key。双沙箱模式的从沙箱走此扩展点，
// 让完成回调时 backend 能在销毁主 execute 之外，再销毁一次此 id 关联的执行。
const SandboxAgentExtKeyExtraExecuteID = "sandbox_agent_extra_execute_id"

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
