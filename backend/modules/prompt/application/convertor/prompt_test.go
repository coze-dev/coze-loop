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
							ParamConfigValues: []*prompt.ParamConfigValue{
								{
									Name:  ptr.Of("temperature"),
									Label: ptr.Of("Temperature"),
									Value: &prompt.ParamOption{
										Value: ptr.Of("0.7"),
										Label: ptr.Of("0.7"),
									},
								},
								{
									Name:  ptr.Of("top_p"),
									Label: ptr.Of("Top P"),
									Value: &prompt.ParamOption{
										Value: ptr.Of("0.9"),
										Label: ptr.Of("0.9"),
									},
								},
							},
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
							ParamConfigValues: []*entity.ParamConfigValue{
								{
									Name:  "temperature",
									Label: "Temperature",
									Value: &entity.ParamOption{
										Value: "0.7",
										Label: "0.7",
									},
								},
								{
									Name:  "top_p",
									Label: "Top P",
									Value: &entity.ParamOption{
										Value: "0.9",
										Label: "0.9",
									},
								},
							},
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

type paramOptionTestCase struct {
	name string
	dto  *prompt.ParamOption
	do   *entity.ParamOption
}

func mockParamOptionCases() []paramOptionTestCase {
	return []paramOptionTestCase{
		{
			name: "nil input",
			dto:  nil,
			do:   nil,
		},
		{
			name: "empty param option",
			dto: &prompt.ParamOption{
				Value: ptr.Of(""),
				Label: ptr.Of(""),
			},
			do: &entity.ParamOption{
				Value: "",
				Label: "",
			},
		},
		{
			name: "basic param option",
			dto: &prompt.ParamOption{
				Value: ptr.Of("value1"),
				Label: ptr.Of("Label 1"),
			},
			do: &entity.ParamOption{
				Value: "value1",
				Label: "Label 1",
			},
		},
		{
			name: "param option with special characters",
			dto: &prompt.ParamOption{
				Value: ptr.Of("option_value_123"),
				Label: ptr.Of("Option Label (Special: 测试)"),
			},
			do: &entity.ParamOption{
				Value: "option_value_123",
				Label: "Option Label (Special: 测试)",
			},
		},
	}
}

func TestParamOptionDTO2DO(t *testing.T) {
	for _, tt := range mockParamOptionCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.do, ParamOptionDTO2DO(tt.dto))
		})
	}
}

func TestParamOptionDO2DTO(t *testing.T) {
	for _, tt := range mockParamOptionCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.dto, ParamOptionDO2DTO(tt.do))
		})
	}
}

type paramConfigValueTestCase struct {
	name string
	dto  *prompt.ParamConfigValue
	do   *entity.ParamConfigValue
}

func mockParamConfigValueCases() []paramConfigValueTestCase {
	return []paramConfigValueTestCase{
		{
			name: "nil input",
			dto:  nil,
			do:   nil,
		},
		{
			name: "empty param config value",
			dto: &prompt.ParamConfigValue{
				Name:  ptr.Of(""),
				Label: ptr.Of(""),
				Value: nil,
			},
			do: &entity.ParamConfigValue{
				Name:  "",
				Label: "",
				Value: nil,
			},
		},
		{
			name: "basic param config value",
			dto: &prompt.ParamConfigValue{
				Name:  ptr.Of("temperature"),
				Label: ptr.Of("Temperature"),
				Value: &prompt.ParamOption{
					Value: ptr.Of("0.7"),
					Label: ptr.Of("0.7"),
				},
			},
			do: &entity.ParamConfigValue{
				Name:  "temperature",
				Label: "Temperature",
				Value: &entity.ParamOption{
					Value: "0.7",
					Label: "0.7",
				},
			},
		},
		{
			name: "param config value with complex option",
			dto: &prompt.ParamConfigValue{
				Name:  ptr.Of("top_p"),
				Label: ptr.Of("Top P"),
				Value: &prompt.ParamOption{
					Value: ptr.Of("0.9"),
					Label: ptr.Of("Top P: 0.9 (Recommended)"),
				},
			},
			do: &entity.ParamConfigValue{
				Name:  "top_p",
				Label: "Top P",
				Value: &entity.ParamOption{
					Value: "0.9",
					Label: "Top P: 0.9 (Recommended)",
				},
			},
		},
		{
			name: "param config value without value",
			dto: &prompt.ParamConfigValue{
				Name:  ptr.Of("max_tokens"),
				Label: ptr.Of("Max Tokens"),
				Value: nil,
			},
			do: &entity.ParamConfigValue{
				Name:  "max_tokens",
				Label: "Max Tokens",
				Value: nil,
			},
		},
	}
}

func TestParamConfigValueDTO2DO(t *testing.T) {
	for _, tt := range mockParamConfigValueCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.do, ParamConfigValueDTO2DO(tt.dto))
		})
	}
}

func TestParamConfigValueDO2DTO(t *testing.T) {
	for _, tt := range mockParamConfigValueCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.dto, ParamConfigValueDO2DTO(tt.do))
		})
	}
}

func TestBatchParamConfigValueDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		dtos []*prompt.ParamConfigValue
		dos  []*entity.ParamConfigValue
	}{
		{
			name: "nil input",
			dtos: nil,
			dos:  nil,
		},
		{
			name: "empty slice",
			dtos: []*prompt.ParamConfigValue{},
			dos:  []*entity.ParamConfigValue{},
		},
		{
			name: "single param config value",
			dtos: []*prompt.ParamConfigValue{
				{
					Name:  ptr.Of("temperature"),
					Label: ptr.Of("Temperature"),
					Value: &prompt.ParamOption{
						Value: ptr.Of("0.7"),
						Label: ptr.Of("0.7"),
					},
				},
			},
			dos: []*entity.ParamConfigValue{
				{
					Name:  "temperature",
					Label: "Temperature",
					Value: &entity.ParamOption{
						Value: "0.7",
						Label: "0.7",
					},
				},
			},
		},
		{
			name: "multiple param config values",
			dtos: []*prompt.ParamConfigValue{
				{
					Name:  ptr.Of("temperature"),
					Label: ptr.Of("Temperature"),
					Value: &prompt.ParamOption{
						Value: ptr.Of("0.7"),
						Label: ptr.Of("0.7"),
					},
				},
				{
					Name:  ptr.Of("top_p"),
					Label: ptr.Of("Top P"),
					Value: &prompt.ParamOption{
						Value: ptr.Of("0.9"),
						Label: ptr.Of("0.9"),
					},
				},
			},
			dos: []*entity.ParamConfigValue{
				{
					Name:  "temperature",
					Label: "Temperature",
					Value: &entity.ParamOption{
						Value: "0.7",
						Label: "0.7",
					},
				},
				{
					Name:  "top_p",
					Label: "Top P",
					Value: &entity.ParamOption{
						Value: "0.9",
						Label: "0.9",
					},
				},
			},
		},
		{
			name: "with nil elements (should be skipped)",
			dtos: []*prompt.ParamConfigValue{
				{
					Name:  ptr.Of("temperature"),
					Label: ptr.Of("Temperature"),
					Value: &prompt.ParamOption{
						Value: ptr.Of("0.7"),
						Label: ptr.Of("0.7"),
					},
				},
				nil,
				{
					Name:  ptr.Of("top_p"),
					Label: ptr.Of("Top P"),
					Value: &prompt.ParamOption{
						Value: ptr.Of("0.9"),
						Label: ptr.Of("0.9"),
					},
				},
			},
			dos: []*entity.ParamConfigValue{
				{
					Name:  "temperature",
					Label: "Temperature",
					Value: &entity.ParamOption{
						Value: "0.7",
						Label: "0.7",
					},
				},
				{
					Name:  "top_p",
					Label: "Top P",
					Value: &entity.ParamOption{
						Value: "0.9",
						Label: "0.9",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.dos, BatchParamConfigValueDTO2DO(tt.dtos))
		})
	}
}

func TestBatchParamConfigValueDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		dos  []*entity.ParamConfigValue
		dtos []*prompt.ParamConfigValue
	}{
		{
			name: "nil input",
			dos:  nil,
			dtos: nil,
		},
		{
			name: "empty slice",
			dos:  []*entity.ParamConfigValue{},
			dtos: []*prompt.ParamConfigValue{},
		},
		{
			name: "single param config value",
			dos: []*entity.ParamConfigValue{
				{
					Name:  "temperature",
					Label: "Temperature",
					Value: &entity.ParamOption{
						Value: "0.7",
						Label: "0.7",
					},
				},
			},
			dtos: []*prompt.ParamConfigValue{
				{
					Name:  ptr.Of("temperature"),
					Label: ptr.Of("Temperature"),
					Value: &prompt.ParamOption{
						Value: ptr.Of("0.7"),
						Label: ptr.Of("0.7"),
					},
				},
			},
		},
		{
			name: "multiple param config values",
			dos: []*entity.ParamConfigValue{
				{
					Name:  "temperature",
					Label: "Temperature",
					Value: &entity.ParamOption{
						Value: "0.7",
						Label: "0.7",
					},
				},
				{
					Name:  "top_p",
					Label: "Top P",
					Value: &entity.ParamOption{
						Value: "0.9",
						Label: "0.9",
					},
				},
			},
			dtos: []*prompt.ParamConfigValue{
				{
					Name:  ptr.Of("temperature"),
					Label: ptr.Of("Temperature"),
					Value: &prompt.ParamOption{
						Value: ptr.Of("0.7"),
						Label: ptr.Of("0.7"),
					},
				},
				{
					Name:  ptr.Of("top_p"),
					Label: ptr.Of("Top P"),
					Value: &prompt.ParamOption{
						Value: ptr.Of("0.9"),
						Label: ptr.Of("0.9"),
					},
				},
			},
		},
		{
			name: "with nil elements (should be skipped)",
			dos: []*entity.ParamConfigValue{
				{
					Name:  "temperature",
					Label: "Temperature",
					Value: &entity.ParamOption{
						Value: "0.7",
						Label: "0.7",
					},
				},
				nil,
				{
					Name:  "top_p",
					Label: "Top P",
					Value: &entity.ParamOption{
						Value: "0.9",
						Label: "0.9",
					},
				},
			},
			dtos: []*prompt.ParamConfigValue{
				{
					Name:  ptr.Of("temperature"),
					Label: ptr.Of("Temperature"),
					Value: &prompt.ParamOption{
						Value: ptr.Of("0.7"),
						Label: ptr.Of("0.7"),
					},
				},
				{
					Name:  ptr.Of("top_p"),
					Label: ptr.Of("Top P"),
					Value: &prompt.ParamOption{
						Value: ptr.Of("0.9"),
						Label: ptr.Of("0.9"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.dtos, BatchParamConfigValueDO2DTO(tt.dos))
		})
	}
}
