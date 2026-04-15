// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type WebAgent struct {
	ID          int64
	Name        string
	Description string

	AgentConfig  *AgentConfig
	PromptConfig *WebAgentTargetPromptConfig
}

type WebAgentTargetPromptConfig struct {
	MessageList []*Message
	OutputRule  *WebAgentTargetPromptConfigOutputRule
}

type WebAgentTargetPromptConfigOutputRule struct {
	Message *Message
}
