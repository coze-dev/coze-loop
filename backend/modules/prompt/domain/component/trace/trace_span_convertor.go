// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"github.com/coze-dev/cozeloop-go/spec/tracespec"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func VariableValsToSpanPromptVariables(variables []*entity.VariableVal) []*tracespec.PromptArgument {
	if variables == nil {
		return nil
	}
	spanVariables := make([]*tracespec.PromptArgument, 0, len(variables))
	for _, variable := range variables {
		if variable == nil {
			continue
		}
		spanVariables = append(spanVariables, VariableValToSpanPromptVariable(variable))
	}
	return spanVariables
}

func VariableValToSpanPromptVariable(variable *entity.VariableVal) *tracespec.PromptArgument {
	if variable == nil {
		return nil
	}
	var val any
	var valueType = tracespec.PromptArgumentValueTypeText
	val = ptr.From(variable.Value)
	if val == "" && len(variable.PlaceholderMessages) > 0 {
		val = MessagesToSpanMessages(variable.PlaceholderMessages)
		valueType = tracespec.PromptArgumentValueTypeModelMessage
	}
	if val == "" && len(variable.MultiPartValues) > 0 {
		val = ContentPartsToSpanParts(variable.MultiPartValues)
		valueType = tracespec.PromptArgumentValueTypeMessagePart
	}
	return &tracespec.PromptArgument{
		Key:       variable.Key,
		Value:     val,
		Source:    "input",
		ValueType: valueType,
	}
}

func MessagesToSpanMessages(messages []*entity.Message) []*tracespec.ModelMessage {
	if messages == nil {
		return nil
	}
	spanMessages := make([]*tracespec.ModelMessage, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		spanMessages = append(spanMessages, MessageToSpanMessage(message))
	}
	return spanMessages
}

func MessageToSpanMessage(message *entity.Message) *tracespec.ModelMessage {
	if message == nil {
		return nil
	}
	return &tracespec.ModelMessage{
		Role:       RoleToSpanRole(message.Role),
		Content:    ptr.From(message.Content),
		Parts:      ContentPartsToSpanParts(message.Parts),
		ToolCalls:  ToolCallsToSpanToolCalls(message.ToolCalls),
		ToolCallID: ptr.From(message.ToolCallID),
	}
}

func RoleToSpanRole(role entity.Role) string {
	switch role {
	case entity.RoleSystem:
		return tracespec.VRoleSystem
	case entity.RoleUser:
		return tracespec.VRoleUser
	case entity.RoleAssistant:
		return tracespec.VRoleAssistant
	case entity.RoleTool:
		return tracespec.VRoleTool
	default:
		return string(role)
	}
}

func ContentPartsToSpanParts(parts []*entity.ContentPart) []*tracespec.ModelMessagePart {
	if parts == nil {
		return nil
	}
	spanParts := make([]*tracespec.ModelMessagePart, 0, len(parts))
	for _, part := range parts {
		if part == nil {
			continue
		}
		spanParts = append(spanParts, ContentPartToSpanPart(part))
	}
	return spanParts
}

func ContentPartToSpanPart(part *entity.ContentPart) *tracespec.ModelMessagePart {
	if part == nil {
		return nil
	}
	var imageURL *tracespec.ModelImageURL
	if part.ImageURL != nil {
		imageURL = &tracespec.ModelImageURL{
			URL: part.ImageURL.URL,
		}
	}
	return &tracespec.ModelMessagePart{
		Type:     ContentTypeToSpanPartType(part.Type),
		Text:     ptr.From(part.Text),
		ImageURL: imageURL,
	}
}

func ContentTypeToSpanPartType(partType entity.ContentType) tracespec.ModelMessagePartType {
	switch partType {
	case entity.ContentTypeText:
		return tracespec.ModelMessagePartTypeText
	case entity.ContentTypeImageURL:
		return tracespec.ModelMessagePartTypeImage
	case entity.ContentTypeMultiPartVariable:
		return "multi_part_variable"
	default:
		return tracespec.ModelMessagePartType(partType)
	}
}

func ToolCallsToSpanToolCalls(toolCalls []*entity.ToolCall) []*tracespec.ModelToolCall {
	if toolCalls == nil {
		return nil
	}
	spanToolCalls := make([]*tracespec.ModelToolCall, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		if toolCall == nil {
			continue
		}
		spanToolCalls = append(spanToolCalls, ToolCallToSpanToolCall(toolCall))
	}
	return spanToolCalls
}

func ToolCallToSpanToolCall(toolCall *entity.ToolCall) *tracespec.ModelToolCall {
	if toolCall == nil {
		return nil
	}
	var function *tracespec.ModelToolCallFunction
	if toolCall.FunctionCall != nil {
		function = &tracespec.ModelToolCallFunction{
			Name:      toolCall.FunctionCall.Name,
			Arguments: ptr.From(toolCall.FunctionCall.Arguments),
		}
	}
	return &tracespec.ModelToolCall{
		ID:       toolCall.ID,
		Type:     string(toolCall.Type),
		Function: function,
	}
}

type SpanPrompt struct {
	WorkspaceID    int64                  `json:"workspace_id"`
	PromptKey      string                 `json:"prompt_key"`
	Version        string                 `json:"version"`
	PromptTemplate *entity.PromptTemplate `json:"prompt_template,omitempty"`
	Tools          []*entity.Tool         `json:"tools,omitempty"`
	ToolCallConfig *entity.ToolCallConfig `json:"tool_call_config,omitempty"`
	LlmConfig      *entity.ModelConfig    `json:"llm_config,omitempty"`
}

func PromptToSpanPrompt(prompt *entity.Prompt) *SpanPrompt {
	if prompt == nil {
		return nil
	}
	promptDetail := prompt.GetPromptDetail()
	var promptTemplate *entity.PromptTemplate
	var tools []*entity.Tool
	var toolCallConfig *entity.ToolCallConfig
	var llmConfig *entity.ModelConfig
	if promptDetail != nil {
		promptTemplate = promptDetail.PromptTemplate
		tools = promptDetail.Tools
		toolCallConfig = promptDetail.ToolCallConfig
		llmConfig = promptDetail.ModelConfig
	}
	return &SpanPrompt{
		WorkspaceID:    prompt.SpaceID,
		PromptKey:      prompt.PromptKey,
		Version:        prompt.GetVersion(),
		PromptTemplate: promptTemplate,
		Tools:          tools,
		ToolCallConfig: toolCallConfig,
		LlmConfig:      llmConfig,
	}
}

func ReplyItemToSpanOutput(replyItem *entity.ReplyItem) *tracespec.ModelOutput {
	if replyItem == nil {
		return nil
	}
	return &tracespec.ModelOutput{
		Choices: []*tracespec.ModelChoice{
			{
				Index:        0,
				FinishReason: replyItem.FinishReason,
				Message:      MessageToSpanMessage(replyItem.Message),
			},
		},
	}
}
