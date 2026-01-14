// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type DebugContext struct {
	PromptID      int64          `json:"prompt_id"`
	UserID        string         `json:"user_id"`
	DebugCore     *DebugCore     `json:"debug_core,omitempty"`
	DebugConfig   *DebugConfig   `json:"debug_config,omitempty"`
	CompareConfig *CompareConfig `json:"compare_config,omitempty"`
}

type DebugCore struct {
	MockContexts  []*DebugMessage `json:"mock_contexts,omitempty"`
	MockVariables []*VariableVal  `json:"mock_variables,omitempty"`
	MockTools     []*MockTool     `json:"mock_tools,omitempty"`
}

type DebugMessage struct {
	Role             Role             `json:"role"`
	ReasoningContent *string          `json:"reasoning_content,omitempty"`
	Content          *string          `json:"content,omitempty"`
	Parts            []*ContentPart   `json:"parts,omitempty"`
	ToolCallID       *string          `json:"tool_call_id,omitempty"`
	ToolCalls        []*DebugToolCall `json:"tool_calls,omitempty"`
	Signature        *string          `json:"signature,omitempty"` // gemini3 thought_signature
	DebugID          *string          `json:"debug_id,omitempty"`
	InputTokens      *int64           `json:"input_tokens,omitempty"`
	OutputTokens     *int64           `json:"output_tokens,omitempty"`
	CostMS           *int64           `json:"cost_ms,omitempty"`
}

type DebugToolCall struct {
	ToolCall      ToolCall `json:"tool_call"`
	MockResponse  string   `json:"mock_response"`
	DebugTraceKey string   `json:"debug_trace_key"`
}

type MockTool struct {
	Name         string `json:"name"`
	MockResponse string `json:"mock_response"`
}

type DebugConfig struct {
	SingleStepDebug *bool `json:"single_step_debug,omitempty"`
}

type CompareConfig struct {
	Groups []*CompareGroup `json:"groups,omitempty"`
}

type CompareGroup struct {
	PromptDetail *PromptDetail `json:"prompt_detail,omitempty"`
	DebugCore    *DebugCore    `json:"debug_core,omitempty"`
}
