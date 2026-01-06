// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

func TestToolConfigProvider_GetToolConfig(t *testing.T) {
	provider := NewToolConfigProvider()
	tools := []*entity.Tool{
		{
			Type: entity.ToolTypeFunction,
			Function: &entity.Function{
				Name: "tool_a",
			},
		},
	}

	tests := []struct {
		name       string
		prompt     *entity.Prompt
		singleStep bool
		wantTools  []*entity.Tool
		wantConfig *entity.ToolCallConfig
		wantErr    string
	}{
		{
			name:       "nil prompt",
			prompt:     nil,
			singleStep: true,
			wantTools:  nil,
			wantConfig: nil,
		},
		{
			name: "tool choice none ignored",
			prompt: &entity.Prompt{
				PromptCommit: &entity.PromptCommit{
					PromptDetail: &entity.PromptDetail{
						Tools: tools,
						ToolCallConfig: &entity.ToolCallConfig{
							ToolChoice: entity.ToolChoiceTypeNone,
						},
					},
				},
			},
			singleStep: true,
			wantTools:  nil,
			wantConfig: nil,
		},
		{
			name: "specific tool choice without single step returns error",
			prompt: &entity.Prompt{
				PromptCommit: &entity.PromptCommit{
					PromptDetail: &entity.PromptDetail{
						Tools: tools,
						ToolCallConfig: &entity.ToolCallConfig{
							ToolChoice: entity.ToolChoiceTypeSpecific,
							ToolChoiceSpecification: &entity.ToolChoiceSpecification{
								Type: entity.ToolTypeFunction,
								Name: "tool_a",
							},
						},
					},
				},
			},
			singleStep: false,
			wantErr:    "tool choice specific must be used with single step mode to avoid infinite loops",
		},
		{
			name: "specific tool choice without specification returns error",
			prompt: &entity.Prompt{
				PromptCommit: &entity.PromptCommit{
					PromptDetail: &entity.PromptDetail{
						Tools: tools,
						ToolCallConfig: &entity.ToolCallConfig{
							ToolChoice: entity.ToolChoiceTypeSpecific,
						},
					},
				},
			},
			singleStep: true,
			wantErr:    "tool_choice_specification must not be empty when tool choice is specific",
		},
		{
			name: "specific tool choice with specification succeeds",
			prompt: &entity.Prompt{
				PromptCommit: &entity.PromptCommit{
					PromptDetail: &entity.PromptDetail{
						Tools: tools,
						ToolCallConfig: &entity.ToolCallConfig{
							ToolChoice: entity.ToolChoiceTypeSpecific,
							ToolChoiceSpecification: &entity.ToolChoiceSpecification{
								Type: entity.ToolTypeFunction,
								Name: "tool_a",
							},
						},
					},
				},
			},
			singleStep: true,
			wantTools:  tools,
			wantConfig: &entity.ToolCallConfig{
				ToolChoice: entity.ToolChoiceTypeSpecific,
				ToolChoiceSpecification: &entity.ToolChoiceSpecification{
					Type: entity.ToolTypeFunction,
					Name: "tool_a",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTools, gotConfig, err := provider.GetToolConfig(context.Background(), tt.prompt, tt.singleStep)
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantTools, gotTools)
			assert.Equal(t, tt.wantConfig, gotConfig)
		})
	}
}
