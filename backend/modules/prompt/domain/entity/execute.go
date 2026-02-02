// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type Reply struct {
	Item          *ReplyItem `json:"item,omitempty"`
	DebugID       int64      `json:"debug_id"`
	DebugStep     int32      `json:"debug_step"`
	DebugTraceKey string     `json:"debug_trace_key"`
}

type ReplyItem struct {
	Message      *Message    `json:"message,omitempty"`
	FinishReason string      `json:"finish_reason"`
	TokenUsage   *TokenUsage `json:"token_usage,omitempty"`
}

type TokenUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type Scenario string

const (
	ScenarioDefault     Scenario = "default"
	ScenarioPromptDebug Scenario = "prompt_debug"
	ScenarioPTaaS       Scenario = "ptaas"
	ScenarioEvalTarget  Scenario = "eval_target"
)

type ResponseAPIConfig struct {
	PreviousResponseID *string `json:"previous_response_id,omitempty"` // 上一次响应的ID
	EnableCaching      *bool   `json:"enable_caching,omitempty"`       // 是否开启缓存
	SessionID          *string `json:"session_id,omitempty"`           // 一轮会话的唯一标识
}
