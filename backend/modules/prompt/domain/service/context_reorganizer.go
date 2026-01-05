// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"slices"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	loopslices "github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
)

// ReorganizeContextParam defines the parameters for reorganizing contexts
type ReorganizeContextParam struct {
	Prompt       *entity.Prompt
	Messages     []*entity.Message
	MockTools    []*entity.MockTool
	Reply        *entity.Reply
	ResultStream chan<- *entity.Reply // only used in streaming mode
}

// IContextReorganizer defines the interface for reorganizing contexts after each iteration
type IContextReorganizer interface {
	ReorganizeContexts(ctx context.Context, param ReorganizeContextParam) ([]*entity.Message, error)
}

// ContextReorganizer provides the default implementation of IContextReorganizer
type ContextReorganizer struct{}

// NewContextReorganizer creates a new instance of ContextReorganizer
func NewContextReorganizer() IContextReorganizer {
	return &ContextReorganizer{}
}

// ReorganizeContexts implements the IContextReorganizer interface
func (c *ContextReorganizer) ReorganizeContexts(ctx context.Context, param ReorganizeContextParam) ([]*entity.Message, error) {
	newContexts := slices.Clone(param.Messages)
	if param.Reply == nil || param.Reply.Item == nil || param.Reply.Item.Message == nil {
		return newContexts, nil
	}
	newContexts = append(newContexts, param.Reply.Item.Message)
	if len(param.Reply.Item.Message.ToolCalls) > 0 {
		// 如果有工具调用，则需要mock response
		mockToolResponseMap := loopslices.ToMap(param.MockTools, func(m *entity.MockTool) (string, string) {
			if m == nil {
				return "", ""
			}
			return m.Name, m.MockResponse
		})
		for _, toolCall := range param.Reply.Item.Message.ToolCalls {
			if toolCall.FunctionCall != nil {
				newContexts = append(newContexts, &entity.Message{
					Role:       entity.RoleTool,
					ToolCallID: ptr.Of(toolCall.ID),
					Content:    ptr.Of(mockToolResponseMap[toolCall.FunctionCall.Name]),
				})
			}
		}
	}
	return newContexts, nil
}
