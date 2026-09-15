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
//   - Single:          沿用原有单沙箱执行链路
//   - Dual:            先起一个从属沙箱拿到 session id，再起一个主沙箱运行 sandbox-pipeline
//   - MacVMPlusSandbox: 一次评测同时租借 1 台 Mac VM（跑被测桌面 agent）+ 1 台 Sandbox（跑 orchestrator），
//     两者靠 runtime 侧 WebSocket 互联。下游用同一租户 + ResourceType 区分两个 task。
//   - Shared:          远程 FaaS 对象专用：**一个沙箱里跑两个进程**（Runner + Orchestrator），
//     走 loopback 互联。之所以能省掉第二个沙箱，是因为远程对象跑在 ByteFaaS 上，
//     沙箱里根本没有 agent 进程 —— 双沙箱要隔离的那个不可信域不在这个沙箱里。
//     ⚠️ 只对远程 FaaS 对象成立；配给本地 CLI/GUI 对象会让防 Reward Hacking 的边界静默消失，
//     runtime 侧有 fail-closed 门拦截（cmd/orchestrator: validateSandboxTopologyScope）。
//   - MacVMPlusSSH:     一次评测租借 3 台资源：1 台主控 orchestrator Sandbox + 1 台 SSH Sandbox
//     （Runner 就地，跑 workspace/命令/评估，被 IDE 经 Remote-SSH 挂上来）+ 1 台 Mac VM（只跑 IDE 桌面，被 CUA 驱动）。
//     控制面走 HTTP（复用双沙箱链路），触达 Mac VM 只经 AIC gexec→CUA，无新增 WebSocket。
type SandboxCountMode string

const (
	SandboxCountModeSingle           SandboxCountMode = "single"
	SandboxCountModeDual             SandboxCountMode = "dual"
	SandboxCountModeMacVMPlusSandbox SandboxCountMode = "mac_vm_plus_sandbox"
	SandboxCountModeMacVMPlusSSH     SandboxCountMode = "mac_vm_plus_ssh"
	SandboxCountModeShared           SandboxCountMode = "shared"
)

// ResolveSandboxCountMode 空/未识别值一律回退到 Single，保持默认行为。
//
// ⚠️ **这个回退是 fail-OPEN 的，新增取值时必须同时改这里。** 少加一个 case 的现象是：
// 控制面明明配了新拓扑，实验却静默按 Single 的旧扁平链路跑完并报成功 —— 没有任何一层报错。
// 这也是为什么 runtime 侧的 fail-closed 兜不住控制面：一旦在这里回落成 Single，
// 那条链路压根不启动 runtime，runtime 的校验没有执行机会。
func ResolveSandboxCountMode(mode SandboxCountMode) SandboxCountMode {
	switch mode {
	case SandboxCountModeDual:
		return SandboxCountModeDual
	case SandboxCountModeMacVMPlusSandbox:
		return SandboxCountModeMacVMPlusSandbox
	case SandboxCountModeMacVMPlusSSH:
		return SandboxCountModeMacVMPlusSSH
	case SandboxCountModeShared:
		return SandboxCountModeShared
	default:
		return SandboxCountModeSingle
	}
}

// IsDualSandbox 判断 SandboxAgent 是否处于双沙箱模式；nil / 未填字段一律按 Single 处理。
func (a *SandboxAgent) IsDualSandbox() bool {
	if a == nil {
		return false
	}
	return ResolveSandboxCountMode(a.SandboxCountMode) == SandboxCountModeDual
}

// IsMacVMPlusSandbox 判断 SandboxAgent 是否处于 Mac VM + Sandbox 双资源模式；nil / 未填字段一律按 Single 处理。
func (a *SandboxAgent) IsMacVMPlusSandbox() bool {
	if a == nil {
		return false
	}
	return ResolveSandboxCountMode(a.SandboxCountMode) == SandboxCountModeMacVMPlusSandbox
}

// IsSharedSandbox 判断 SandboxAgent 是否处于共享沙箱模式（Runner 与 Orchestrator 同处一个沙箱）；
// nil / 未填字段一律按 Single 处理。
func (a *SandboxAgent) IsSharedSandbox() bool {
	if a == nil {
		return false
	}
	return ResolveSandboxCountMode(a.SandboxCountMode) == SandboxCountModeShared
}

// IsMacVMPlusSSH 判断 SandboxAgent 是否处于 Mac VM + SSH Sandbox 三资源模式（Remote-SSH IDE 评测）；
// nil / 未填字段一律按 Single 处理。
func (a *SandboxAgent) IsMacVMPlusSSH() bool {
	if a == nil {
		return false
	}
	return ResolveSandboxCountMode(a.SandboxCountMode) == SandboxCountModeMacVMPlusSSH
}

type SandboxEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SandboxAgent struct {
	Name      string           `json:"name"`
	Type      SandboxAgentType `json:"type"`
	ModelName string           `json:"model_name"`
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
