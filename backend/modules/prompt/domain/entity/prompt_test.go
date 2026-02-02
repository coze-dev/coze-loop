// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestPrompt_GetVersion(t *testing.T) {
	tests := []struct {
		name     string
		prompt   *Prompt
		expected string
	}{
		{
			name:     "nil prompt",
			prompt:   nil,
			expected: "",
		},
		{
			name: "nil PromptCommit",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
			},
			expected: "",
		},
		{
			name: "nil CommitInfo",
			prompt: &Prompt{
				ID:           1,
				SpaceID:      123,
				PromptKey:    "test_prompt",
				PromptCommit: &PromptCommit{},
			},
			expected: "",
		},
		{
			name: "normal case with version",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptCommit: &PromptCommit{
					CommitInfo: &CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "",
						Description: "Initial version",
						CommittedBy: "test_user",
						CommittedAt: time.Now(),
					},
				},
			},
			expected: "1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := tt.prompt.GetVersion()
			assert.Equal(t, tt.expected, version)
		})
	}
}

func TestPrompt_GetPromptDetail(t *testing.T) {
	tests := []struct {
		name     string
		prompt   *Prompt
		expected *PromptDetail
	}{
		{
			name:     "nil prompt",
			prompt:   nil,
			expected: nil,
		},
		{
			name: "nil PromptDraft and PromptCommit",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
			},
			expected: nil,
		},
		{
			name: "nil PromptDraft.PromptDetail",
			prompt: &Prompt{
				ID:           1,
				SpaceID:      123,
				PromptKey:    "test_prompt",
				PromptDraft:  &PromptDraft{},
				PromptCommit: &PromptCommit{},
			},
			expected: nil,
		},
		{
			name: "nil PromptCommit.PromptDetail",
			prompt: &Prompt{
				ID:           1,
				SpaceID:      123,
				PromptKey:    "test_prompt",
				PromptDraft:  &PromptDraft{},
				PromptCommit: &PromptCommit{},
			},
			expected: nil,
		},
		{
			name: "get PromptDetail from PromptDraft",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptDraft: &PromptDraft{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
								},
							},
						},
						ModelConfig: &ModelConfig{
							ModelID:     123,
							Temperature: ptr.Of(0.7),
						},
					},
				},
			},
			expected: &PromptDetail{
				PromptTemplate: &PromptTemplate{
					TemplateType: TemplateTypeNormal,
					Messages: []*Message{
						{
							Role:    RoleSystem,
							Content: ptr.Of("You are a helpful assistant."),
						},
					},
				},
				ModelConfig: &ModelConfig{
					ModelID:     123,
					Temperature: ptr.Of(0.7),
				},
			},
		},
		{
			name: "get PromptDetail from PromptCommit",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptCommit: &PromptCommit{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
								},
							},
						},
						ModelConfig: &ModelConfig{
							ModelID:     123,
							Temperature: ptr.Of(0.7),
						},
					},
				},
			},
			expected: &PromptDetail{
				PromptTemplate: &PromptTemplate{
					TemplateType: TemplateTypeNormal,
					Messages: []*Message{
						{
							Role:    RoleSystem,
							Content: ptr.Of("You are a helpful assistant."),
						},
					},
				},
				ModelConfig: &ModelConfig{
					ModelID:     123,
					Temperature: ptr.Of(0.7),
				},
			},
		},
		{
			name: "PromptDraft takes precedence over PromptCommit",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptDraft: &PromptDraft{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("Draft version"),
								},
							},
						},
					},
				},
				PromptCommit: &PromptCommit{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("Commit version"),
								},
							},
						},
					},
				},
			},
			expected: &PromptDetail{
				PromptTemplate: &PromptTemplate{
					TemplateType: TemplateTypeNormal,
					Messages: []*Message{
						{
							Role:    RoleSystem,
							Content: ptr.Of("Draft version"),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail := tt.prompt.GetPromptDetail()
			assert.Equal(t, tt.expected, detail)
		})
	}
}

func TestPrompt_FormatMessages(t *testing.T) {
	tests := []struct {
		name          string
		prompt        *Prompt
		messages      []*Message
		variableVals  []*VariableVal
		expectedMsgs  []*Message
		expectedError error
	}{
		{
			name:          "nil prompt",
			prompt:        nil,
			messages:      []*Message{},
			variableVals:  []*VariableVal{},
			expectedMsgs:  nil,
			expectedError: nil,
		},
		{
			name: "nil PromptDetail",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
			},
			messages:      []*Message{},
			variableVals:  []*VariableVal{},
			expectedMsgs:  nil,
			expectedError: nil,
		},
		{
			name: "nil PromptTemplate",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptDraft: &PromptDraft{
					PromptDetail: &PromptDetail{},
				},
			},
			messages:      []*Message{},
			variableVals:  []*VariableVal{},
			expectedMsgs:  nil,
			expectedError: nil,
		},
		{
			name: "format messages without variable",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptDraft: &PromptDraft{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
								},
							},
						},
					},
				},
			},
			messages: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			variableVals: []*VariableVal{},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
				},
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			expectedError: nil,
		},
		{
			name: "format messages with variables",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptDraft: &PromptDraft{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("You are a {{role}}."),
								},
							},
							VariableDefs: []*VariableDef{
								{
									Key:  "role",
									Desc: "role",
									Type: VariableTypeString,
								},
							},
						},
					},
				},
			},
			messages: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			variableVals: []*VariableVal{
				{
					Key:   "role",
					Value: ptr.Of("helpful assistant"),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
				},
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			expectedError: nil,
		},
		{
			name: "format messages from PromptCommit",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptCommit: &PromptCommit{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
								},
							},
						},
					},
				},
			},
			messages: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			variableVals: []*VariableVal{},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
				},
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formattedMsgs, err := tt.prompt.FormatMessages(tt.messages, tt.variableVals)
			assert.Equal(t, tt.expectedError, err)
			assert.Equal(t, normalizeSkipRenderPromptMessages(tt.expectedMsgs), normalizeSkipRenderPromptMessages(formattedMsgs))
		})
	}
}

func TestPrompt_GetTemplateMessages(t *testing.T) {
	tests := []struct {
		name         string
		prompt       *Prompt
		messages     []*Message
		expectedMsgs []*Message
	}{
		{
			name:         "nil prompt",
			prompt:       nil,
			messages:     []*Message{},
			expectedMsgs: nil,
		},
		{
			name: "nil PromptDetail",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
			},
			messages:     []*Message{},
			expectedMsgs: nil,
		},
		{
			name: "nil PromptTemplate",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptDraft: &PromptDraft{
					PromptDetail: &PromptDetail{},
				},
			},
			messages:     []*Message{},
			expectedMsgs: nil,
		},
		{
			name: "get template messages from PromptDraft",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptDraft: &PromptDraft{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
								},
							},
						},
					},
				},
			},
			messages: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
				},
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
		},
		{
			name: "get template messages from PromptCommit",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptCommit: &PromptCommit{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
								},
							},
						},
					},
				},
			},
			messages: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
				},
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
		},
		{
			name: "empty messages",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptDraft: &PromptDraft{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
								},
							},
						},
					},
				},
			},
			messages: []*Message{},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateMsgs := tt.prompt.GetTemplateMessages(tt.messages)
			assert.Equal(t, normalizeSkipRenderPromptMessages(tt.expectedMsgs), normalizeSkipRenderPromptMessages(templateMsgs))
		})
	}
}

func normalizeSkipRenderPromptMessages(messages []*Message) []*Message {
	for _, message := range messages {
		if message == nil {
			continue
		}
		message.SkipRender = nil
	}
	return messages
}

func TestPrompt_CloneDetail(t *testing.T) {
	tests := []struct {
		name     string
		prompt   *Prompt
		expected *Prompt
	}{
		{
			name:     "nil prompt",
			prompt:   nil,
			expected: nil,
		},
		{
			name: "empty prompt",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
			},
			expected: &Prompt{
				ID:        0,
				SpaceID:   123,
				PromptKey: "",
			},
		},
		{
			name: "prompt with PromptBasic",
			prompt: &Prompt{
				ID:          1,
				SpaceID:     123,
				PromptKey:   "test_prompt",
				PromptBasic: &PromptBasic{},
			},
			expected: &Prompt{
				ID:        0,
				SpaceID:   123,
				PromptKey: "",
			},
		},
		{
			name: "prompt with PromptDraft and DraftInfo",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptDraft: &PromptDraft{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
								},
							},
						},
					},
					DraftInfo: &DraftInfo{
						UserID:      "test_user",
						BaseVersion: "1.0.0",
						IsModified:  true,
					},
				},
			},
			expected: &Prompt{
				ID:        0,
				SpaceID:   123,
				PromptKey: "",
				PromptDraft: &PromptDraft{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
								},
							},
						},
					},
					DraftInfo: nil,
				},
			},
		},
		{
			name: "prompt with PromptCommit and CommitInfo",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptCommit: &PromptCommit{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
								},
							},
						},
					},
					CommitInfo: &CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "Initial version",
						CommittedBy: "test_user",
						CommittedAt: time.Now(),
					},
				},
			},
			expected: &Prompt{
				ID:        0,
				SpaceID:   123,
				PromptKey: "",
				PromptCommit: &PromptCommit{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
								},
							},
						},
					},
					CommitInfo: nil,
				},
			},
		},
		{
			name: "prompt with both PromptDraft and PromptCommit",
			prompt: &Prompt{
				ID:        1,
				SpaceID:   123,
				PromptKey: "test_prompt",
				PromptDraft: &PromptDraft{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("Draft version"),
								},
							},
						},
					},
					DraftInfo: &DraftInfo{
						UserID:      "test_user",
						BaseVersion: "1.0.0",
						IsModified:  true,
					},
				},
				PromptCommit: &PromptCommit{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("Commit version"),
								},
							},
						},
					},
					CommitInfo: &CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "Initial version",
						CommittedBy: "test_user",
						CommittedAt: time.Now(),
					},
				},
			},
			expected: &Prompt{
				ID:        0,
				SpaceID:   123,
				PromptKey: "",
				PromptDraft: &PromptDraft{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("Draft version"),
								},
							},
						},
					},
					DraftInfo: nil,
				},
				PromptCommit: &PromptCommit{
					PromptDetail: &PromptDetail{
						PromptTemplate: &PromptTemplate{
							TemplateType: TemplateTypeNormal,
							Messages: []*Message{
								{
									Role:    RoleSystem,
									Content: ptr.Of("Commit version"),
								},
							},
						},
					},
					CommitInfo: nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cloned := tt.prompt.CloneDetail()
			assert.Equal(t, tt.expected, cloned)
		})
	}
}
