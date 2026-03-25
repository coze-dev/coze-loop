// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"

	"github.com/bytedance/sonic"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ChatProcessor struct{}

func (c *ChatProcessor) Transform(ctx context.Context, spans loop_span.SpanList) (loop_span.SpanList, error) {
	for _, span := range spans {
		if span == nil || !span.IsChatSpan() {
			continue
		}
		if span.Input == "" {
			continue
		}
		processedInput := c.extractLastUserMessage(ctx, span.Input)
		if processedInput != "" {
			span.Input = processedInput
		}
	}
	return spans, nil
}

func (c *ChatProcessor) extractLastUserMessage(ctx context.Context, input string) string {
	var inputMap map[string]interface{}
	if err := sonic.UnmarshalString(input, &inputMap); err != nil {
		logs.CtxDebug(ctx, "chat processor: input is not a valid JSON object")
		return ""
	}

	if messages := c.tryExtractFromStandardChat(inputMap); messages != nil {
		return c.buildUserInput(ctx, messages)
	}

	if messages := c.tryExtractFromResponsesAPI(inputMap); messages != nil {
		return c.buildUserInput(ctx, messages)
	}

	return ""
}

func (c *ChatProcessor) tryExtractFromStandardChat(inputMap map[string]interface{}) []interface{} {
	messages, ok := inputMap["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return nil
	}
	return c.filterLastUserMessage(messages)
}

func (c *ChatProcessor) tryExtractFromResponsesAPI(inputMap map[string]interface{}) []interface{} {
	inputField, ok := inputMap["input"]
	if !ok {
		return nil
	}

	switch v := inputField.(type) {
	case string:
		if v != "" {
			return []interface{}{
				map[string]interface{}{
					"role":    "user",
					"content": v,
				},
			}
		}
	case []interface{}:
		messages := c.convertResponsesAPIToStandard(v)
		return c.filterLastUserMessage(messages)
	}
	return nil
}

func (c *ChatProcessor) convertResponsesAPIToStandard(input []interface{}) []interface{} {
	var messages []interface{}
	for _, item := range input {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		itemType, _ := itemMap["type"].(string)
		switch itemType {
		case "message", "":
			role, _ := itemMap["role"].(string)
			if role == "" {
				continue
			}
			content := itemMap["content"]
			messages = append(messages, map[string]interface{}{
				"role":    role,
				"content": content,
			})
		}
	}
	return messages
}

func (c *ChatProcessor) filterLastUserMessage(messages []interface{}) []interface{} {
	lastUserIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role == "user" {
			lastUserIndex = i
			break
		}
	}

	if lastUserIndex == -1 {
		return nil
	}

	return []interface{}{messages[lastUserIndex]}
}

func (c *ChatProcessor) buildUserInput(ctx context.Context, messages []interface{}) string {
	if len(messages) == 0 {
		return ""
	}

	result := map[string]interface{}{
		"messages": messages,
	}

	output, err := sonic.MarshalString(result)
	if err != nil {
		logs.CtxWarn(ctx, "chat processor: failed to marshal user input: %v", err)
		return ""
	}
	return output
}

type ChatProcessorFactory struct{}

func (c *ChatProcessorFactory) CreateProcessor(ctx context.Context, set Settings) (Processor, error) {
	return &ChatProcessor{}, nil
}

func NewChatProcessorFactory() Factory {
	return new(ChatProcessorFactory)
}
