// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"

	"github.com/bytedance/sonic"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const noQueryParsed = "no_query_parsed"

type ChatProcessor struct{}

func (c *ChatProcessor) Transform(ctx context.Context, spans loop_span.SpanList) (loop_span.SpanList, error) {
	for _, span := range spans {
		if span == nil {
			continue
		}
		if span.IsModelSpan() {
			span.Input = c.processModelInput(ctx, span.Input)
		}
	}
	return spans, nil
}

func (c *ChatProcessor) processModelInput(ctx context.Context, input string) string {
	if input == "" {
		return ""
	}

	var inputMap map[string]interface{}
	if err := sonic.UnmarshalString(input, &inputMap); err != nil {
		logs.CtxDebug(ctx, "chat processor: input is not a valid JSON object")
		return noQueryParsed
	}

	if result, ok := c.tryProcessStandardChat(ctx, inputMap); ok {
		return result
	}

	if result, ok := c.tryProcessResponsesAPI(ctx, input, inputMap); ok {
		return result
	}

	return noQueryParsed
}

func (c *ChatProcessor) tryProcessStandardChat(ctx context.Context, inputMap map[string]interface{}) (string, bool) {
	messages, ok := inputMap["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return "", false
	}

	lastMsg, ok := messages[len(messages)-1].(map[string]interface{})
	if !ok {
		return "", false
	}
	role, _ := lastMsg["role"].(string)
	if role == "human" {
		role = "user"
		lastMsg["role"] = "user"
	}
	if role != "user" {
		return "", true
	}
	// TODO: more compaction
	inputMap["messages"] = []interface{}{lastMsg}
	result, err := sonic.MarshalString(inputMap)
	if err != nil {
		logs.CtxWarn(ctx, "chat processor: failed to marshal input: %v", err)
		return "", true
	}
	return result, true
}

func (c *ChatProcessor) tryProcessResponsesAPI(ctx context.Context, input string, inputMap map[string]interface{}) (string, bool) {
	inputField, ok := inputMap["input"]
	if !ok {
		return "", false
	}

	switch v := inputField.(type) {
	case string:
		return input, true
	case []interface{}:
		return c.processResponsesAPIMessages(ctx, inputMap, v)
	}
	return "", false
}

func (c *ChatProcessor) processResponsesAPIMessages(ctx context.Context, inputMap map[string]interface{}, items []interface{}) (string, bool) {
	if len(items) == 0 {
		return "", true
	}

	lastItem, ok := items[len(items)-1].(map[string]interface{})
	if !ok {
		return "", true
	}

	role, _ := lastItem["role"].(string)
	if role == "human" {
		role = "user"
		lastItem["role"] = "user"
	}
	if role != "user" {
		return "", true
	}

	inputMap["input"] = []interface{}{lastItem}
	result, err := sonic.MarshalString(inputMap)
	if err != nil {
		logs.CtxWarn(ctx, "chat processor: failed to marshal input: %v", err)
		return "", true
	}
	return result, true
}

type ChatProcessorFactory struct{}

func (c *ChatProcessorFactory) CreateProcessor(ctx context.Context, set Settings) (Processor, error) {
	return &ChatProcessor{}, nil
}

func NewChatProcessorFactory() Factory {
	return new(ChatProcessorFactory)
}
