// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

// SandboxAgent metric error_type 枚举（与稳定性看板口径对齐）。
const (
	SandboxAgentErrorTypeEngineering    = "engineering"     // 工程问题：稳定性归属 Fornax 的错误
	SandboxAgentErrorTypeNonEngineering = "non_engineering" // 非工程问题：模型/数据/题目侧错误
	SandboxAgentErrorTypeUnknown        = "unknown"         // 无法分类的失败
)

// ClassifySandboxAgentError 依据 error 生成 error_type tag 值：
//   - nil                       -> ""（调用侧应据此判定 success=true）
//   - IsAffectStability=true    -> engineering
//   - IsAffectStability=false   -> non_engineering
//   - 其他                       -> unknown
func ClassifySandboxAgentError(err error) (success bool, errorType string) {
	if err == nil {
		return true, ""
	}
	code, isError := GetCode(err)
	_ = code
	if isError == 1 {
		return false, SandboxAgentErrorTypeEngineering
	}
	return false, SandboxAgentErrorTypeNonEngineering
}
