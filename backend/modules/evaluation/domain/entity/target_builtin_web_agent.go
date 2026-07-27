// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type WebAgent struct {
	ID          int64
	Name        string
	Description string

	AgentConfig  *AgentConfig
	PromptConfig *WebAgentTargetPromptConfig

	// EnableAnalysis 是否开启分析：创建评测对象时从 application.usages（含 "analysis"）反查固化，
	// 控制 item-complete MQ 是否发送（与 TCC 空间白名单 AND）。
	EnableAnalysis bool `json:"enable_analysis,omitempty"`
}

type WebAgentTargetPromptConfig struct {
	MessageList []*Message
	OutputRule  *WebAgentTargetPromptConfigOutputRule
}

type WebAgentTargetPromptConfigOutputRule struct {
	Message *Message
}
