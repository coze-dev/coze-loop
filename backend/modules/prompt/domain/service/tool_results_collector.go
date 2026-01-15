// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// CollectToolResultsParam defines the parameters for processing tool results
type CollectToolResultsParam struct {
	Prompt           *entity.Prompt
	MockTools        []*entity.MockTool
	Reply            *entity.Reply
	ResultStream     chan<- *entity.Reply                    // only used in streaming mode, can be nil
	ReplyItemWrapper func(v *entity.ReplyItem) *entity.Reply // only used in streaming mode, can be nil
}

// IToolResultsCollector defines the interface for processing tool results
//
//go:generate mockgen -destination=mocks/tool_results_collector.go -package=mocks . IToolResultsCollector
type IToolResultsCollector interface {
	CollectToolResults(ctx context.Context, param CollectToolResultsParam) (map[string]string, error)
}

// ToolResultsCollector provides the default implementation of IToolResultsCollector
type ToolResultsCollector struct{}

// NewToolResultsCollector creates a new instance of ToolResultsCollector
func NewToolResultsCollector() IToolResultsCollector {
	return &ToolResultsCollector{}
}

// CollectToolResults ProcessToolResults implements the IToolResultsCollector interface
func (t *ToolResultsCollector) CollectToolResults(ctx context.Context, param CollectToolResultsParam) (map[string]string, error) {
	toolResultMap := make(map[string]string)

	// 如果没有 mock tools，直接返回空 map
	if len(param.MockTools) == 0 {
		return toolResultMap, nil
	}

	// 构建 mock tool name 到 mock response 的映射
	mockToolResponseMap := make(map[string]string, len(param.MockTools))
	for _, mockTool := range param.MockTools {
		if mockTool == nil || mockTool.Name == "" {
			continue
		}
		mockToolResponseMap[mockTool.Name] = mockTool.MockResponse
	}

	// 从 reply 中获取 toolCalls，构建正确的 key（与 reorganizeContexts 中的 key 生成逻辑保持一致）
	if param.Reply != nil && param.Reply.Item != nil && param.Reply.Item.Message != nil {
		for _, toolCall := range param.Reply.Item.Message.ToolCalls {
			if toolCall == nil || toolCall.FunctionCall == nil {
				continue
			}

			toolName := toolCall.FunctionCall.Name
			// 检查是否是 mock tool
			if mockResponse, ok := mockToolResponseMap[toolName]; ok {
				// 使用与 reorganizeContexts 相同的 key 生成方式: toolCallId + name + signature
				toolCallId := toolCall.ID
				signature := toolCall.Signature
				toolResultKey := toolCallId + toolName + ptr.From(signature)
				toolResultMap[toolResultKey] = mockResponse
			}
		}
	}

	return toolResultMap, nil
}
