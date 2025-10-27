// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type promptTestCase struct {
	name string
	dto  *prompt.Prompt
	do   *entity.Prompt
}

func mockPromptCases() []promptTestCase {
	now := time.Now()
	nowMilli := now.UnixMilli()
	// 定义共享的测试用例
	return []promptTestCase{
		{
			name: "nil input",
			dto:  nil,
			do:   nil,
		},
		{
			name: "empty prompt",
			dto: &prompt.Prompt{
				ID:          ptr.Of(int64(0)),
				WorkspaceID: ptr.Of(int64(0)),
				PromptKey:   ptr.Of(""),
			},
			do: &entity.Prompt{
				ID:        0,
				SpaceID:   0,
				PromptKey: "",
			},
		},
		{
			name: "basic prompt with only ID and workspace",
			dto: &prompt.Prompt{
				ID:          ptr.Of(int64(123)),
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
			},
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
			},
		},
		{
			name: "complete prompt with all fields",
			dto: &prompt.Prompt{
				ID:          ptr.Of(int64(123)),
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
				PromptBasic: &prompt.PromptBasic{
					DisplayName:   ptr.Of("Test Prompt"),
					Description:   ptr.Of("Test PromptDescription"),
					LatestVersion: ptr.Of("1.0.0"),
					CreatedBy:     ptr.Of("test_user"),
					UpdatedBy:     ptr.Of("test_user"),
					CreatedAt:     ptr.Of(nowMilli),
					UpdatedAt:     ptr.Of(nowMilli),
				},
				PromptCommit: &prompt.PromptCommit{
					CommitInfo: &prompt.CommitInfo{
						Version:     ptr.Of("1.0.0"),
						BaseVersion: ptr.Of(""),
						Description: ptr.Of("Initial version"),
						CommittedBy: ptr.Of("test_user"),
						CommittedAt: ptr.Of(nowMilli),
					},
					Detail: &prompt.PromptDetail{
						PromptTemplate: &prompt.PromptTemplate{
							TemplateType: ptr.Of(prompt.TemplateTypeNormal),
							Messages: []*prompt.Message{
								{
									Role:    ptr.Of(prompt.RoleSystem),
									Content: ptr.Of("You are a helpful assistant."),
								},
								{
									Role: ptr.Of(prompt.RoleUser),
									Parts: []*prompt.ContentPart{
										{
											Type: ptr.Of(prompt.ContentTypeImageURL),
											ImageURL: &prompt.ImageURL{
												URI: ptr.Of("test_uri"),
												URL: ptr.Of("test_url"),
											},
										},
										{
											Type: ptr.Of(prompt.ContentTypeText),
											Text: ptr.Of("describe the content of the image"),
										},
									},
								},
							},
							VariableDefs: []*prompt.VariableDef{
								{
									Key:  ptr.Of("var1"),
									Desc: ptr.Of("Variable 1"),
									Type: ptr.Of(prompt.VariableTypeString),
								},
							},
						},
						ModelConfig: &prompt.ModelConfig{
							ModelID:     ptr.Of(int64(789)),
							Temperature: ptr.Of(0.7),
							MaxTokens:   ptr.Of(int32(1000)),
						},
						Tools: []*prompt.Tool{
							{
								Type: ptr.Of(prompt.ToolTypeFunction),
								Function: &prompt.Function{
									Name:        ptr.Of("test_function"),
									Description: ptr.Of("Test Function"),
									Parameters:  ptr.Of(`{"type":"object","properties":{}}`),
								},
							},
						},
						ToolCallConfig: &prompt.ToolCallConfig{
							ToolChoice: ptr.Of(prompt.ToolChoiceTypeAuto),
						},
					},
				},
				PromptDraft: &prompt.PromptDraft{
					DraftInfo: &prompt.DraftInfo{
						UserID:      ptr.Of("test_user"),
						BaseVersion: ptr.Of("1.0.0"),
						IsModified:  ptr.Of(true),
						CreatedAt:   ptr.Of(nowMilli),
						UpdatedAt:   ptr.Of(nowMilli),
					},
					Detail: &prompt.PromptDetail{
						PromptTemplate: &prompt.PromptTemplate{
							TemplateType: ptr.Of(prompt.TemplateTypeNormal),
							Messages: []*prompt.Message{
								{
									Role:    ptr.Of(prompt.RoleSystem),
									Content: ptr.Of("You are a helpful assistant. Draft version."),
								},
							},
						},
					},
				},
			},
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptBasic: &entity.PromptBasic{
					DisplayName:   "Test Prompt",
					Description:   "Test PromptDescription",
					LatestVersion: "1.0.0",
					CreatedBy:     "test_user",
					UpdatedBy:     "test_user",
					CreatedAt:     time.UnixMilli(nowMilli),
					UpdatedAt:     time.UnixMilli(nowMilli),
				},
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "",
						Description: "Initial version",
						CommittedBy: "test_user",
						CommittedAt: time.UnixMilli(nowMilli),
					},
					PromptDetail: &entity.PromptDetail{
						PromptTemplate: &entity.PromptTemplate{
							TemplateType: entity.TemplateTypeNormal,
							Messages: []*entity.Message{
								{
									Role:    entity.RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
								},
								{
									Role: entity.RoleUser,
									Parts: []*entity.ContentPart{
										{
											Type: entity.ContentTypeImageURL,
											ImageURL: &entity.ImageURL{
												URI: "test_uri",
												URL: "test_url",
											},
										},
										{
											Type: entity.ContentTypeText,
											Text: ptr.Of("describe the content of the image"),
										},
									},
								},
							},
							VariableDefs: []*entity.VariableDef{
								{
									Key:  "var1",
									Desc: "Variable 1",
									Type: entity.VariableTypeString,
								},
							},
						},
						ModelConfig: &entity.ModelConfig{
							ModelID:     789,
							Temperature: ptr.Of(0.7),
							MaxTokens:   ptr.Of(int32(1000)),
						},
						Tools: []*entity.Tool{
							{
								Type: entity.ToolTypeFunction,
								Function: &entity.Function{
									Name:        "test_function",
									Description: "Test Function",
									Parameters:  `{"type":"object","properties":{}}`,
								},
							},
						},
						ToolCallConfig: &entity.ToolCallConfig{
							ToolChoice: entity.ToolChoiceTypeAuto,
						},
					},
				},
				PromptDraft: &entity.PromptDraft{
					DraftInfo: &entity.DraftInfo{
						UserID:      "test_user",
						BaseVersion: "1.0.0",
						IsModified:  true,
						CreatedAt:   time.UnixMilli(nowMilli),
						UpdatedAt:   time.UnixMilli(nowMilli),
					},
					PromptDetail: &entity.PromptDetail{
						PromptTemplate: &entity.PromptTemplate{
							TemplateType: entity.TemplateTypeNormal,
							Messages: []*entity.Message{
								{
									Role:    entity.RoleSystem,
									Content: ptr.Of("You are a helpful assistant. Draft version."),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "prompt with only basic info",
			dto: &prompt.Prompt{
				ID:          ptr.Of(int64(123)),
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
				PromptBasic: &prompt.PromptBasic{
					DisplayName:   ptr.Of("Test Prompt"),
					Description:   ptr.Of("Test PromptDescription"),
					LatestVersion: ptr.Of("1.0.0"),
					CreatedBy:     ptr.Of("test_user"),
					UpdatedBy:     ptr.Of("test_user"),
					CreatedAt:     ptr.Of(nowMilli),
					UpdatedAt:     ptr.Of(nowMilli),
				},
			},
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptBasic: &entity.PromptBasic{
					DisplayName:   "Test Prompt",
					Description:   "Test PromptDescription",
					LatestVersion: "1.0.0",
					CreatedBy:     "test_user",
					UpdatedBy:     "test_user",
					CreatedAt:     time.UnixMilli(nowMilli),
					UpdatedAt:     time.UnixMilli(nowMilli),
				},
			},
		},
		{
			name: "prompt with only commit info",
			dto: &prompt.Prompt{
				ID:          ptr.Of(int64(123)),
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
				PromptCommit: &prompt.PromptCommit{
					CommitInfo: &prompt.CommitInfo{
						Version:     ptr.Of("1.0.0"),
						BaseVersion: ptr.Of(""),
						Description: ptr.Of("Initial version"),
						CommittedBy: ptr.Of("test_user"),
						CommittedAt: ptr.Of(nowMilli),
					},
				},
			},
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "",
						Description: "Initial version",
						CommittedBy: "test_user",
						CommittedAt: time.UnixMilli(nowMilli),
					},
				},
			},
		},
		{
			name: "prompt with only draft info",
			dto: &prompt.Prompt{
				ID:          ptr.Of(int64(123)),
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
				PromptDraft: &prompt.PromptDraft{
					DraftInfo: &prompt.DraftInfo{
						UserID:      ptr.Of("test_user"),
						BaseVersion: ptr.Of("1.0.0"),
						IsModified:  ptr.Of(true),
						CreatedAt:   ptr.Of(nowMilli),
						UpdatedAt:   ptr.Of(nowMilli),
					},
				},
			},
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptDraft: &entity.PromptDraft{
					DraftInfo: &entity.DraftInfo{
						UserID:      "test_user",
						BaseVersion: "1.0.0",
						IsModified:  true,
						CreatedAt:   time.UnixMilli(nowMilli),
						UpdatedAt:   time.UnixMilli(nowMilli),
					},
				},
			},
		},
		{
			name: "prompt template metadata",
			dto: &prompt.Prompt{
				ID:          ptr.Of(int64(0)),
				WorkspaceID: ptr.Of(int64(0)),
				PromptKey:   ptr.Of(""),
				PromptCommit: &prompt.PromptCommit{
					Detail: &prompt.PromptDetail{
						PromptTemplate: &prompt.PromptTemplate{
							TemplateType: ptr.Of(prompt.TemplateTypeNormal),
							Metadata:     map[string]string{"commit-meta": "value"},
						},
					},
				},
				PromptDraft: &prompt.PromptDraft{
					Detail: &prompt.PromptDetail{
						PromptTemplate: &prompt.PromptTemplate{
							TemplateType: ptr.Of(prompt.TemplateTypeNormal),
							Metadata:     map[string]string{"draft-meta": "value"},
						},
					},
				},
			},
			do: &entity.Prompt{
				PromptCommit: &entity.PromptCommit{
					PromptDetail: &entity.PromptDetail{
						PromptTemplate: &entity.PromptTemplate{
							TemplateType: entity.TemplateTypeNormal,
							Metadata:     map[string]string{"commit-meta": "value"},
						},
					},
				},
				PromptDraft: &entity.PromptDraft{
					PromptDetail: &entity.PromptDetail{
						PromptTemplate: &entity.PromptTemplate{
							TemplateType: entity.TemplateTypeNormal,
							Metadata:     map[string]string{"draft-meta": "value"},
						},
					},
				},
			},
		},
	}
}

func TestPromptDTO2DO(t *testing.T) {
	for _, tt := range mockPromptCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.do, PromptDTO2DO(tt.dto))
		})
	}
}

func TestPromptDO2DTO(t *testing.T) {
	for _, tt := range mockPromptCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.dto, PromptDO2DTO(tt.do))
		})
	}
}

type messageTestCase struct {
	name string
	dto  *prompt.Message
	do   *entity.Message
}

func mockMessageCases() []messageTestCase {
	return []messageTestCase{
		{
			name: "nil input",
			dto:  nil,
			do:   nil,
		},
		{
			name: "empty message",
			dto: &prompt.Message{
				Role: ptr.Of(prompt.RoleUser),
			},
			do: &entity.Message{
				Role: entity.RoleUser, // 默认值
			},
		},
		{
			name: "system role message with content",
			dto: &prompt.Message{
				Role:    ptr.Of(prompt.RoleSystem),
				Content: ptr.Of("You are a helpful assistant."),
			},
			do: &entity.Message{
				Role:    entity.RoleSystem,
				Content: ptr.Of("You are a helpful assistant."),
			},
		},
		{
			name: "user role message with content",
			dto: &prompt.Message{
				Role:    ptr.Of(prompt.RoleUser),
				Content: ptr.Of("Help me with this task."),
			},
			do: &entity.Message{
				Role:    entity.RoleUser,
				Content: ptr.Of("Help me with this task."),
			},
		},
		{
			name: "assistant role message with content",
			dto: &prompt.Message{
				Role:    ptr.Of(prompt.RoleAssistant),
				Content: ptr.Of("I'll help you with your task."),
			},
			do: &entity.Message{
				Role:    entity.RoleAssistant,
				Content: ptr.Of("I'll help you with your task."),
			},
		},
		{
			name: "tool role message with content",
			dto: &prompt.Message{
				Role:       ptr.Of(prompt.RoleTool),
				Content:    ptr.Of("Tool execution result"),
				ToolCallID: ptr.Of("tool-call-123"),
			},
			do: &entity.Message{
				Role:       entity.RoleTool,
				Content:    ptr.Of("Tool execution result"),
				ToolCallID: ptr.Of("tool-call-123"),
			},
		},
		{
			name: "placeholder role message",
			dto: &prompt.Message{
				Role:    ptr.Of(prompt.RolePlaceholder),
				Content: ptr.Of("placeholder-var"),
			},
			do: &entity.Message{
				Role:    entity.RolePlaceholder,
				Content: ptr.Of("placeholder-var"),
			},
		},
		{
			name: "user message with multimodal content",
			dto: &prompt.Message{
				Role: ptr.Of(prompt.RoleUser),
				Parts: []*prompt.ContentPart{
					{
						Type: ptr.Of(prompt.ContentTypeImageURL),
						ImageURL: &prompt.ImageURL{
							URI: ptr.Of("image-uri"),
							URL: ptr.Of("image-url"),
						},
					},
					{
						Type: ptr.Of(prompt.ContentTypeText),
						Text: ptr.Of("Describe this image"),
					},
				},
			},
			do: &entity.Message{
				Role: entity.RoleUser,
				Parts: []*entity.ContentPart{
					{
						Type: entity.ContentTypeImageURL,
						ImageURL: &entity.ImageURL{
							URI: "image-uri",
							URL: "image-url",
						},
					},
					{
						Type: entity.ContentTypeText,
						Text: ptr.Of("Describe this image"),
					},
				},
			},
		},
		{
			name: "user message with video content",
			dto: &prompt.Message{
				Role: ptr.Of(prompt.RoleUser),
				Parts: []*prompt.ContentPart{
					{
						Type: ptr.Of(prompt.ContentTypeVideoURL),
						VideoURL: &prompt.VideoURL{
							URL: ptr.Of("https://example.com/video.mp4"),
							URI: ptr.Of("video-uri"),
						},
						MediaConfig: &prompt.MediaConfig{
							Fps: ptr.Of(2.5),
						},
					},
				},
			},
			do: &entity.Message{
				Role: entity.RoleUser,
				Parts: []*entity.ContentPart{
					{
						Type: entity.ContentTypeVideoURL,
						VideoURL: &entity.VideoURL{
							URL: "https://example.com/video.mp4",
							URI: "video-uri",
						},
						MediaConfig: &entity.MediaConfig{
							Fps: ptr.Of(2.5),
						},
					},
				},
			},
		},
		{
			name: "assistant message with tool calls",
			dto: &prompt.Message{
				Role: ptr.Of(prompt.RoleAssistant),
				ToolCalls: []*prompt.ToolCall{
					{
						Index: ptr.Of(int64(0)),
						ID:    ptr.Of("tool-call-123"),
						Type:  ptr.Of(prompt.ToolTypeFunction),
						FunctionCall: &prompt.FunctionCall{
							Name:      ptr.Of("get_weather"),
							Arguments: ptr.Of(`{"location": "New York"}`),
						},
					},
				},
			},
			do: &entity.Message{
				Role: entity.RoleAssistant,
				ToolCalls: []*entity.ToolCall{
					{
						Index: 0,
						ID:    "tool-call-123",
						Type:  entity.ToolTypeFunction,
						FunctionCall: &entity.FunctionCall{
							Name:      "get_weather",
							Arguments: ptr.Of(`{"location": "New York"}`),
						},
					},
				},
			},
		},
		{
			name: "message with reasoning content",
			dto: &prompt.Message{
				Role:             ptr.Of(prompt.RoleAssistant),
				Content:          ptr.Of("Final answer"),
				ReasoningContent: ptr.Of("This is my reasoning process..."),
			},
			do: &entity.Message{
				Role:             entity.RoleAssistant,
				Content:          ptr.Of("Final answer"),
				ReasoningContent: ptr.Of("This is my reasoning process..."),
			},
		},
		{
			name: "message with metadata",
			dto: &prompt.Message{
				Role:     ptr.Of(prompt.RoleAssistant),
				Metadata: map[string]string{"key": "value"},
			},
			do: &entity.Message{
				Role:     entity.RoleAssistant,
				Metadata: map[string]string{"key": "value"},
			},
		},
	}
}

func TestMessageDTO2DO(t *testing.T) {
	for _, tt := range mockMessageCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.do, MessageDTO2DO(tt.dto))
		})
	}
	extraTests := []struct {
		name string
		dto  *prompt.Message
		want *entity.Message
	}{
		{
			name: "message with invalid role",
			dto: &prompt.Message{
				Role:    ptr.Of("invalid"), // 无效值
				Content: ptr.Of("Some content"),
			},
			want: &entity.Message{
				Role:    entity.RoleUser, // 默认为user
				Content: ptr.Of("Some content"),
			},
		},
	}
	for _, tt := range extraTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, MessageDTO2DO(tt.dto))
		})
	}
}

func TestMessageDO2DTO(t *testing.T) {
	for _, tt := range mockMessageCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.dto, MessageDO2DTO(tt.do))
		})
	}
}

func TestModelConfigExtraConversion(t *testing.T) {
	extra := ptr.Of(`{"foo":"bar"}`)
	dto := &prompt.ModelConfig{
		Extra: extra,
	}

	do := ModelConfigDTO2DO(dto)
	assert.NotNil(t, do)
	assert.Equal(t, extra, do.Extra)

	dtoBack := ModelConfigDO2DTO(do)
	assert.NotNil(t, dtoBack)
	assert.Equal(t, extra, dtoBack.Extra)
}
