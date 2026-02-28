// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// IToolConfigProvider defines the interface for extracting tool definitions and configuration
type IToolConfigProvider interface {
	GetToolConfig(ctx context.Context, prompt *entity.Prompt, singleStep bool) (newCtx context.Context, tools []*entity.Tool, toolCallConfig *entity.ToolCallConfig, err error)
}

// ToolConfigProvider provides the default implementation of IToolConfigProvider
type ToolConfigProvider struct{}

// NewToolConfigProvider creates a new instance of ToolConfigProvider
func NewToolConfigProvider() IToolConfigProvider {
	return &ToolConfigProvider{}
}

// GetToolConfig implements the IToolConfigProvider interface
func (t *ToolConfigProvider) GetToolConfig(ctx context.Context, prompt *entity.Prompt, singleStep bool) (newCtx context.Context, tools []*entity.Tool, toolCallConfig *entity.ToolCallConfig, err error) {
	newCtx = ctx

	promptDetail := prompt.GetPromptDetail()
	if promptDetail != nil {
		if promptDetail.ToolCallConfig != nil && promptDetail.ToolCallConfig.ToolChoice != entity.ToolChoiceTypeNone {
			tools = promptDetail.Tools
			toolCallConfig = promptDetail.ToolCallConfig
		}
	}

	// Validate tool choice specification
	if toolCallConfig != nil && toolCallConfig.ToolChoice == entity.ToolChoiceTypeSpecific {
		// When tool choice is specific, must be in single step mode
		if !singleStep {
			return newCtx, nil, nil, errorx.New("tool choice specific must be used with single step mode to avoid infinite loops")
		}
		// ToolChoiceSpecification must not be empty
		if toolCallConfig.ToolChoiceSpecification == nil {
			return newCtx, nil, nil, errorx.New("tool_choice_specification must not be empty when tool choice is specific")
		}
	}

	return newCtx, tools, toolCallConfig, nil
}
