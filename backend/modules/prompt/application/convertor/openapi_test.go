// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type openAPIPromptTestCase struct {
	name string
	do   *entity.Prompt
	dto  *openapi.Prompt
}

func mockOpenAPIPromptCases() []openAPIPromptTestCase {
	return []openAPIPromptTestCase{
		{
			name: "nil input",
			do:   nil,
			dto:  nil,
		},
		{
			name: "empty prompt",
			do: &entity.Prompt{
				ID:        0,
				SpaceID:   0,
				PromptKey: "",
			},
			dto: &openapi.Prompt{
				WorkspaceID: ptr.Of(int64(0)),
				PromptKey:   ptr.Of(""),
				Version:     ptr.Of(""),
			},
		},
		{
			name: "basic prompt with only ID and workspace",
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version: "1.0.0",
					},
				},
			},
			dto: &openapi.Prompt{
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
				Version:     ptr.Of("1.0.0"),
			},
		},
		{
			name: "prompt with template only",
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptBasic: &entity.PromptBasic{
					LatestVersion: "1.0.0",
				},
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version: "1.0.0",
					},
					PromptDetail: &entity.PromptDetail{
						PromptTemplate: &entity.PromptTemplate{
							TemplateType: entity.TemplateTypeNormal,
							Messages: []*entity.Message{
								{
									Role:    entity.RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
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
					},
				},
			},
			dto: &openapi.Prompt{
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
				Version:     ptr.Of("1.0.0"),
				PromptTemplate: &openapi.PromptTemplate{
					TemplateType: ptr.Of(prompt.TemplateTypeNormal),
					Messages: []*openapi.Message{
						{
							Role:    ptr.Of(prompt.RoleSystem),
							Content: ptr.Of("You are a helpful assistant."),
						},
					},
					VariableDefs: []*openapi.VariableDef{
						{
							Key:  ptr.Of("var1"),
							Desc: ptr.Of("Variable 1"),
							Type: ptr.Of(prompt.VariableTypeString),
						},
					},
				},
			},
		},
		{
			name: "prompt with tools only",
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptBasic: &entity.PromptBasic{
					LatestVersion: "1.0.0",
				},
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version: "1.0.0",
					},
					PromptDetail: &entity.PromptDetail{
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
					},
				},
			},
			dto: &openapi.Prompt{
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
				Version:     ptr.Of("1.0.0"),
				Tools: []*openapi.Tool{
					{
						Type: ptr.Of(prompt.ToolTypeFunction),
						Function: &openapi.Function{
							Name:        ptr.Of("test_function"),
							Description: ptr.Of("Test Function"),
							Parameters:  ptr.Of(`{"type":"object","properties":{}}`),
						},
					},
				},
			},
		},
		{
			name: "prompt with tool call config only",
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptBasic: &entity.PromptBasic{
					LatestVersion: "1.0.0",
				},
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version: "1.0.0",
					},
					PromptDetail: &entity.PromptDetail{
						ToolCallConfig: &entity.ToolCallConfig{
							ToolChoice: entity.ToolChoiceTypeAuto,
						},
					},
				},
			},
			dto: &openapi.Prompt{
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
				Version:     ptr.Of("1.0.0"),
				ToolCallConfig: &openapi.ToolCallConfig{
					ToolChoice: ptr.Of(prompt.ToolChoiceTypeAuto),
				},
			},
		},
		{
			name: "prompt with model config only",
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptBasic: &entity.PromptBasic{
					LatestVersion: "1.0.0",
				},
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version: "1.0.0",
					},
					PromptDetail: &entity.PromptDetail{
						ModelConfig: &entity.ModelConfig{
							ModelID:          789,
							Temperature:      ptr.Of(0.7),
							MaxTokens:        ptr.Of(int32(1000)),
							TopK:             ptr.Of(int32(50)),
							TopP:             ptr.Of(0.9),
							PresencePenalty:  ptr.Of(0.5),
							FrequencyPenalty: ptr.Of(0.5),
							JSONMode:         ptr.Of(true),
						},
					},
				},
			},
			dto: &openapi.Prompt{
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
				Version:     ptr.Of("1.0.0"),
				LlmConfig: &openapi.LLMConfig{
					Temperature:      ptr.Of(0.7),
					MaxTokens:        ptr.Of(int32(1000)),
					TopK:             ptr.Of(int32(50)),
					TopP:             ptr.Of(0.9),
					PresencePenalty:  ptr.Of(0.5),
					FrequencyPenalty: ptr.Of(0.5),
					JSONMode:         ptr.Of(true),
				},
			},
		},
		{
			name: "complete prompt with all fields",
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptBasic: &entity.PromptBasic{
					LatestVersion: "1.0.0",
				},
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version: "1.0.0",
					},
					PromptDetail: &entity.PromptDetail{
						PromptTemplate: &entity.PromptTemplate{
							TemplateType: entity.TemplateTypeNormal,
							Messages: []*entity.Message{
								{
									Role:    entity.RoleSystem,
									Content: ptr.Of("You are a helpful assistant."),
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
						ModelConfig: &entity.ModelConfig{
							ModelID:          789,
							Temperature:      ptr.Of(0.7),
							MaxTokens:        ptr.Of(int32(1000)),
							TopK:             ptr.Of(int32(50)),
							TopP:             ptr.Of(0.9),
							PresencePenalty:  ptr.Of(0.5),
							FrequencyPenalty: ptr.Of(0.5),
							JSONMode:         ptr.Of(true),
						},
					},
				},
			},
			dto: &openapi.Prompt{
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
				Version:     ptr.Of("1.0.0"),
				PromptTemplate: &openapi.PromptTemplate{
					TemplateType: ptr.Of(prompt.TemplateTypeNormal),
					Messages: []*openapi.Message{
						{
							Role:    ptr.Of(prompt.RoleSystem),
							Content: ptr.Of("You are a helpful assistant."),
						},
					},
					VariableDefs: []*openapi.VariableDef{
						{
							Key:  ptr.Of("var1"),
							Desc: ptr.Of("Variable 1"),
							Type: ptr.Of(prompt.VariableTypeString),
						},
					},
				},
				Tools: []*openapi.Tool{
					{
						Type: ptr.Of(prompt.ToolTypeFunction),
						Function: &openapi.Function{
							Name:        ptr.Of("test_function"),
							Description: ptr.Of("Test Function"),
							Parameters:  ptr.Of(`{"type":"object","properties":{}}`),
						},
					},
				},
				ToolCallConfig: &openapi.ToolCallConfig{
					ToolChoice: ptr.Of(prompt.ToolChoiceTypeAuto),
				},
				LlmConfig: &openapi.LLMConfig{
					Temperature:      ptr.Of(0.7),
					MaxTokens:        ptr.Of(int32(1000)),
					TopK:             ptr.Of(int32(50)),
					TopP:             ptr.Of(0.9),
					PresencePenalty:  ptr.Of(0.5),
					FrequencyPenalty: ptr.Of(0.5),
					JSONMode:         ptr.Of(true),
				},
			},
		},
		{
			name: "prompt with nil prompt detail",
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptBasic: &entity.PromptBasic{
					LatestVersion: "1.0.0",
				},
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version: "1.0.0",
					},
					PromptDetail: nil,
				},
			},
			dto: &openapi.Prompt{
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
				Version:     ptr.Of("1.0.0"),
			},
		},
		{
			name: "prompt template metadata",
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{Version: "1.0.0"},
					PromptDetail: &entity.PromptDetail{
						PromptTemplate: &entity.PromptTemplate{
							Metadata: map[string]string{"commit": "meta"},
						},
					},
				},
			},
			dto: &openapi.Prompt{
				WorkspaceID: ptr.Of(int64(456)),
				PromptKey:   ptr.Of("test_prompt"),
				Version:     ptr.Of("1.0.0"),
				PromptTemplate: &openapi.PromptTemplate{
					TemplateType: ptr.Of(""),
					VariableDefs: []*openapi.VariableDef{},
					Metadata:     map[string]string{"commit": "meta"},
				},
			},
		},
	}
}

func TestOpenAPIPromptDO2DTO(t *testing.T) {
	for _, tt := range mockOpenAPIPromptCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := OpenAPIPromptDO2DTO(tt.do)
			assert.Equal(t, tt.dto, result)
		})
	}
}

// 测试单个组件的转换函数
func TestOpenAPIPromptTemplateDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		do   *entity.PromptTemplate
		want *openapi.PromptTemplate
	}{
		{
			name: "nil input",
			do:   nil,
			want: nil,
		},
		{
			name: "valid prompt template",
			do: &entity.PromptTemplate{
				TemplateType: entity.TemplateTypeNormal,
				Messages: []*entity.Message{
					{
						Role:    entity.RoleSystem,
						Content: ptr.Of("You are a helpful assistant."),
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
			want: &openapi.PromptTemplate{
				TemplateType: ptr.Of(prompt.TemplateTypeNormal),
				Messages: []*openapi.Message{
					{
						Role:    ptr.Of(prompt.RoleSystem),
						Content: ptr.Of("You are a helpful assistant."),
					},
				},
				VariableDefs: []*openapi.VariableDef{
					{
						Key:  ptr.Of("var1"),
						Desc: ptr.Of("Variable 1"),
						Type: ptr.Of(prompt.VariableTypeString),
					},
				},
			},
		},
		{
			name: "template with metadata",
			do: &entity.PromptTemplate{
				Metadata: map[string]string{"k": "v"},
			},
			want: &openapi.PromptTemplate{
				TemplateType: ptr.Of(""),
				Messages:     nil,
				VariableDefs: []*openapi.VariableDef{},
				Metadata:     map[string]string{"k": "v"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIPromptTemplateDO2DTO(tt.do))
		})
	}
}

func TestOpenAPIModelConfigDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		do   *entity.ModelConfig
		want *openapi.LLMConfig
	}{
		{
			name: "nil input",
			do:   nil,
			want: nil,
		},
		{
			name: "valid model config",
			do: &entity.ModelConfig{
				ModelID:          789,
				Temperature:      ptr.Of(0.7),
				MaxTokens:        ptr.Of(int32(1000)),
				TopK:             ptr.Of(int32(50)),
				TopP:             ptr.Of(0.9),
				PresencePenalty:  ptr.Of(0.5),
				FrequencyPenalty: ptr.Of(0.5),
				JSONMode:         ptr.Of(true),
			},
			want: &openapi.LLMConfig{
				Temperature:      ptr.Of(0.7),
				MaxTokens:        ptr.Of(int32(1000)),
				TopK:             ptr.Of(int32(50)),
				TopP:             ptr.Of(0.9),
				PresencePenalty:  ptr.Of(0.5),
				FrequencyPenalty: ptr.Of(0.5),
				JSONMode:         ptr.Of(true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIModelConfigDO2DTO(tt.do))
		})
	}
}

func TestOpenAPIToolCallConfigDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		do   *entity.ToolCallConfig
		want *openapi.ToolCallConfig
	}{
		{
			name: "nil input",
			do:   nil,
			want: nil,
		},
		{
			name: "valid tool call config",
			do: &entity.ToolCallConfig{
				ToolChoice: entity.ToolChoiceTypeAuto,
			},
			want: &openapi.ToolCallConfig{
				ToolChoice: ptr.Of(prompt.ToolChoiceTypeAuto),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIToolCallConfigDO2DTO(tt.do))
		})
	}
}

func TestOpenAPIContentTypeDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		do   entity.ContentType
		want openapi.ContentType
	}{
		{
			name: "text content type",
			do:   entity.ContentTypeText,
			want: openapi.ContentTypeText,
		},
		{
			name: "multi part variable content type",
			do:   entity.ContentTypeMultiPartVariable,
			want: openapi.ContentTypeMultiPartVariable,
		},
		{
			name: "image url content type",
			do:   entity.ContentTypeImageURL,
			want: openapi.ContentTypeImageURL,
		},
		{
			name: "video url content type",
			do:   entity.ContentTypeVideoURL,
			want: openapi.ContentTypeVideoURL,
		},
		{
			name: "unknown content type - should default to text",
			do:   entity.ContentType("unknown"),
			want: openapi.ContentTypeText,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIContentTypeDO2DTO(tt.do))
		})
	}
}

func TestOpenAPIContentPartDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		do   *entity.ContentPart
		want *openapi.ContentPart
	}{
		{
			name: "nil input",
			do:   nil,
			want: nil,
		},
		{
			name: "text content part with text",
			do: &entity.ContentPart{
				Type: entity.ContentTypeText,
				Text: ptr.Of("Hello world"),
			},
			want: &openapi.ContentPart{
				Type: ptr.Of(openapi.ContentTypeText),
				Text: ptr.Of("Hello world"),
			},
		},
		{
			name: "multi part variable content part",
			do: &entity.ContentPart{
				Type: entity.ContentTypeMultiPartVariable,
				Text: ptr.Of("{{variable}}"),
			},
			want: &openapi.ContentPart{
				Type: ptr.Of(openapi.ContentTypeMultiPartVariable),
				Text: ptr.Of("{{variable}}"),
			},
		},
		{
			name: "content part with nil text",
			do: &entity.ContentPart{
				Type: entity.ContentTypeText,
				Text: nil,
			},
			want: &openapi.ContentPart{
				Type: ptr.Of(openapi.ContentTypeText),
				Text: nil,
			},
		},
		{
			name: "image url content part",
			do: &entity.ContentPart{
				Type: entity.ContentTypeImageURL,
				Text: ptr.Of("image description"),
				ImageURL: &entity.ImageURL{
					URI: "https://example.com/image.jpg",
					URL: "https://example.com/image.jpg",
				},
			},
			want: &openapi.ContentPart{
				Type:     ptr.Of(openapi.ContentTypeImageURL),
				Text:     ptr.Of("image description"),
				ImageURL: ptr.Of("https://example.com/image.jpg"),
			},
		},
		{
			name: "empty text content part",
			do: &entity.ContentPart{
				Type: entity.ContentTypeText,
				Text: ptr.Of(""),
			},
			want: &openapi.ContentPart{
				Type: ptr.Of(openapi.ContentTypeText),
				Text: ptr.Of(""),
			},
		},
		{
			name: "video url content part with fps",
			do: &entity.ContentPart{
				Type: entity.ContentTypeVideoURL,
				VideoURL: &entity.VideoURL{
					URL: "https://example.com/video.mp4",
				},
				MediaConfig: &entity.MediaConfig{
					Fps: ptr.Of(2.0),
				},
			},
			want: &openapi.ContentPart{
				Type:     ptr.Of(openapi.ContentTypeVideoURL),
				VideoURL: ptr.Of("https://example.com/video.mp4"),
				Config: &openapi.MediaConfig{
					Fps: ptr.Of(2.0),
				},
			},
		},
		{
			name: "video url content part without fps",
			do: &entity.ContentPart{
				Type: entity.ContentTypeVideoURL,
				VideoURL: &entity.VideoURL{
					URL: "https://example.com/video.mp4",
				},
			},
			want: &openapi.ContentPart{
				Type:     ptr.Of(openapi.ContentTypeVideoURL),
				VideoURL: ptr.Of("https://example.com/video.mp4"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIContentPartDO2DTO(tt.do))
		})
	}
}

func TestOpenAPIBatchContentPartDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		do   []*entity.ContentPart
		want []*openapi.ContentPart
	}{
		{
			name: "nil input",
			do:   nil,
			want: nil,
		},
		{
			name: "empty array",
			do:   []*entity.ContentPart{},
			want: []*openapi.ContentPart{},
		},
		{
			name: "array with nil elements",
			do: []*entity.ContentPart{
				nil,
				{
					Type: entity.ContentTypeText,
					Text: ptr.Of("Hello"),
				},
				nil,
			},
			want: []*openapi.ContentPart{
				{
					Type: ptr.Of(openapi.ContentTypeText),
					Text: ptr.Of("Hello"),
				},
			},
		},
		{
			name: "normal array conversion",
			do: []*entity.ContentPart{
				{
					Type: entity.ContentTypeText,
					Text: ptr.Of("Hello"),
				},
				{
					Type: entity.ContentTypeMultiPartVariable,
					Text: ptr.Of("{{variable}}"),
				},
			},
			want: []*openapi.ContentPart{
				{
					Type: ptr.Of(openapi.ContentTypeText),
					Text: ptr.Of("Hello"),
				},
				{
					Type: ptr.Of(openapi.ContentTypeMultiPartVariable),
					Text: ptr.Of("{{variable}}"),
				},
			},
		},
		{
			name: "mixed types array",
			do: []*entity.ContentPart{
				{
					Type: entity.ContentTypeText,
					Text: ptr.Of("Text content"),
				},
				{
					Type: entity.ContentTypeImageURL,
					Text: ptr.Of("Image description"),
					ImageURL: &entity.ImageURL{
						URI: "https://example.com/image.jpg",
						URL: "https://example.com/image.jpg",
					},
				},
				{
					Type: entity.ContentTypeMultiPartVariable,
					Text: ptr.Of("{{user_input}}"),
				},
			},
			want: []*openapi.ContentPart{
				{
					Type: ptr.Of(openapi.ContentTypeText),
					Text: ptr.Of("Text content"),
				},
				{
					Type:     ptr.Of(openapi.ContentTypeImageURL),
					Text:     ptr.Of("Image description"),
					ImageURL: ptr.Of("https://example.com/image.jpg"),
				},
				{
					Type: ptr.Of(openapi.ContentTypeMultiPartVariable),
					Text: ptr.Of("{{user_input}}"),
				},
			},
		},
		{
			name: "array with all nil elements",
			do: []*entity.ContentPart{
				nil,
				nil,
				nil,
			},
			want: []*openapi.ContentPart{},
		},
		{
			name: "array with video url part",
			do: []*entity.ContentPart{
				{
					Type: entity.ContentTypeVideoURL,
					VideoURL: &entity.VideoURL{
						URL: "https://example.com/video.mp4",
					},
					MediaConfig: &entity.MediaConfig{
						Fps: ptr.Of(1.5),
					},
				},
			},
			want: []*openapi.ContentPart{
				{
					Type:     ptr.Of(openapi.ContentTypeVideoURL),
					VideoURL: ptr.Of("https://example.com/video.mp4"),
					Config: &openapi.MediaConfig{
						Fps: ptr.Of(1.5),
					},
				},
			},
		},
		{
			name: "base64 content part carries fps",
			do: []*entity.ContentPart{
				{
					Type:       entity.ContentTypeBase64Data,
					Base64Data: ptr.Of("data:video/mp4;base64,QUJDRA=="),
					MediaConfig: &entity.MediaConfig{
						Fps: ptr.Of(2.4),
					},
				},
			},
			want: []*openapi.ContentPart{
				{
					Type:       ptr.Of(openapi.ContentTypeBase64Data),
					Base64Data: ptr.Of("data:video/mp4;base64,QUJDRA=="),
					Config: &openapi.MediaConfig{
						Fps: ptr.Of(2.4),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIBatchContentPartDO2DTO(tt.do))
		})
	}
}

// ============ 新增字段的增量测试 ============

func TestOpenAPIMessageDO2DTO_NewFields(t *testing.T) {
	tests := []struct {
		name string
		do   *entity.Message
		want *openapi.Message
	}{
		{
			name: "nil input",
			do:   nil,
			want: nil,
		},
		{
			name: "message with reasoning content",
			do: &entity.Message{
				Role:             entity.RoleAssistant,
				ReasoningContent: ptr.Of("thinking..."),
				Content:          ptr.Of("response"),
			},
			want: &openapi.Message{
				Role:             ptr.Of(prompt.RoleAssistant),
				ReasoningContent: ptr.Of("thinking..."),
				Content:          ptr.Of("response"),
			},
		},
		{
			name: "message with tool call id",
			do: &entity.Message{
				Role:       entity.RoleTool,
				Content:    ptr.Of("tool response"),
				ToolCallID: ptr.Of("call_123"),
			},
			want: &openapi.Message{
				Role:       ptr.Of(prompt.RoleTool),
				Content:    ptr.Of("tool response"),
				ToolCallID: ptr.Of("call_123"),
			},
		},
		{
			name: "message with tool calls",
			do: &entity.Message{
				Role:    entity.RoleAssistant,
				Content: ptr.Of("I'll use a tool"),
				ToolCalls: []*entity.ToolCall{
					{
						Index: 0,
						ID:    "call_123",
						Type:  entity.ToolTypeFunction,
						FunctionCall: &entity.FunctionCall{
							Name:      "test_function",
							Arguments: ptr.Of(`{"arg1": "value1"}`),
						},
					},
				},
			},
			want: &openapi.Message{
				Role:    ptr.Of(prompt.RoleAssistant),
				Content: ptr.Of("I'll use a tool"),
				ToolCalls: []*openapi.ToolCall{
					{
						Index: ptr.Of(int32(0)),
						ID:    ptr.Of("call_123"),
						Type:  ptr.Of(openapi.ToolTypeFunction),
						FunctionCall: &openapi.FunctionCall{
							Name:      ptr.Of("test_function"),
							Arguments: ptr.Of(`{"arg1": "value1"}`),
						},
					},
				},
			},
		},
		{
			name: "message with all new fields",
			do: &entity.Message{
				Role:             entity.RoleAssistant,
				ReasoningContent: ptr.Of("analyzing the request"),
				Content:          ptr.Of("I need to call a function"),
				ToolCallID:       ptr.Of("call_456"),
				ToolCalls: []*entity.ToolCall{
					{
						Index: 1,
						ID:    "call_789",
						Type:  entity.ToolTypeFunction,
						FunctionCall: &entity.FunctionCall{
							Name:      "another_function",
							Arguments: ptr.Of(`{"param": "test"}`),
						},
					},
				},
			},
			want: &openapi.Message{
				Role:             ptr.Of(prompt.RoleAssistant),
				ReasoningContent: ptr.Of("analyzing the request"),
				Content:          ptr.Of("I need to call a function"),
				ToolCallID:       ptr.Of("call_456"),
				ToolCalls: []*openapi.ToolCall{
					{
						Index: ptr.Of(int32(1)),
						ID:    ptr.Of("call_789"),
						Type:  ptr.Of(openapi.ToolTypeFunction),
						FunctionCall: &openapi.FunctionCall{
							Name:      ptr.Of("another_function"),
							Arguments: ptr.Of(`{"param": "test"}`),
						},
					},
				},
			},
		},
		{
			name: "message with metadata",
			do: &entity.Message{
				Role:     entity.RoleAssistant,
				Metadata: map[string]string{"meta": "value"},
			},
			want: &openapi.Message{
				Role:     ptr.Of(prompt.RoleAssistant),
				Metadata: map[string]string{"meta": "value"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIMessageDO2DTO(tt.do))
		})
	}
}

func TestOpenAPIContentPartDO2DTO_NewFields(t *testing.T) {
	tests := []struct {
		name string
		do   *entity.ContentPart
		want *openapi.ContentPart
	}{
		{
			name: "nil input",
			do:   nil,
			want: nil,
		},
		{
			name: "content part with image url field",
			do: &entity.ContentPart{
				Type: entity.ContentTypeImageURL,
				Text: ptr.Of("image description"),
				ImageURL: &entity.ImageURL{
					URI: "https://example.com/image.jpg",
					URL: "https://example.com/image.jpg",
				},
			},
			want: &openapi.ContentPart{
				Type:     ptr.Of(openapi.ContentTypeImageURL),
				Text:     ptr.Of("image description"),
				ImageURL: ptr.Of("https://example.com/image.jpg"),
			},
		},
		{
			name: "content part with base64 data field",
			do: &entity.ContentPart{
				Type:       entity.ContentTypeBase64Data,
				Text:       ptr.Of("base64 image"),
				Base64Data: ptr.Of("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="),
			},
			want: &openapi.ContentPart{
				Type:       ptr.Of(openapi.ContentTypeBase64Data),
				Text:       ptr.Of("base64 image"),
				Base64Data: ptr.Of("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="),
			},
		},
		{
			name: "content part with both image url and base64 data",
			do: &entity.ContentPart{
				Type: entity.ContentTypeImageURL,
				Text: ptr.Of("image with multiple formats"),
				ImageURL: &entity.ImageURL{
					URI: "https://example.com/image.png",
					URL: "https://example.com/image.png",
				},
				Base64Data: ptr.Of("base64data"),
			},
			want: &openapi.ContentPart{
				Type:       ptr.Of(openapi.ContentTypeImageURL),
				Text:       ptr.Of("image with multiple formats"),
				ImageURL:   ptr.Of("https://example.com/image.png"),
				Base64Data: ptr.Of("base64data"),
			},
		},
		{
			name: "content part with nil image url",
			do: &entity.ContentPart{
				Type:       entity.ContentTypeText,
				Text:       ptr.Of("just text"),
				ImageURL:   nil,
				Base64Data: nil,
			},
			want: &openapi.ContentPart{
				Type:       ptr.Of(openapi.ContentTypeText),
				Text:       ptr.Of("just text"),
				ImageURL:   nil,
				Base64Data: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIContentPartDO2DTO(tt.do))
		})
	}
}

func TestOpenAPIContentTypeDO2DTO_NewTypes(t *testing.T) {
	tests := []struct {
		name string
		do   entity.ContentType
		want openapi.ContentType
	}{
		{
			name: "text content type",
			do:   entity.ContentTypeText,
			want: openapi.ContentTypeText,
		},
		{
			name: "image url content type",
			do:   entity.ContentTypeImageURL,
			want: openapi.ContentTypeImageURL,
		},
		{
			name: "video url content type",
			do:   entity.ContentTypeVideoURL,
			want: openapi.ContentTypeVideoURL,
		},
		{
			name: "base64 data content type",
			do:   entity.ContentTypeBase64Data,
			want: openapi.ContentTypeBase64Data,
		},
		{
			name: "multi part variable content type",
			do:   entity.ContentTypeMultiPartVariable,
			want: openapi.ContentTypeMultiPartVariable,
		},
		{
			name: "unknown content type - should default to text",
			do:   entity.ContentType("unknown"),
			want: openapi.ContentTypeText,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIContentTypeDO2DTO(tt.do))
		})
	}
}

// ============ 新增顶层函数的完整测试 ============

func TestOpenAPIBatchMessageDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		dtos []*openapi.Message
		want []*entity.Message
	}{
		{
			name: "nil input",
			dtos: nil,
			want: nil,
		},
		{
			name: "empty array",
			dtos: []*openapi.Message{},
			want: nil,
		},
		{
			name: "array with nil elements",
			dtos: []*openapi.Message{
				nil,
				{
					Role:    ptr.Of(prompt.RoleUser),
					Content: ptr.Of("Hello"),
				},
				nil,
			},
			want: []*entity.Message{
				{
					Role:    entity.RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
		},
		{
			name: "normal array conversion",
			dtos: []*openapi.Message{
				{
					Role:    ptr.Of(prompt.RoleSystem),
					Content: ptr.Of("You are a helpful assistant."),
				},
				{
					Role:             ptr.Of(prompt.RoleAssistant),
					ReasoningContent: ptr.Of("thinking..."),
					Content:          ptr.Of("I can help you."),
				},
			},
			want: []*entity.Message{
				{
					Role:    entity.RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
				},
				{
					Role:             entity.RoleAssistant,
					ReasoningContent: ptr.Of("thinking..."),
					Content:          ptr.Of("I can help you."),
				},
			},
		},
		{
			name: "complex messages with tool calls",
			dtos: []*openapi.Message{
				{
					Role:    ptr.Of(prompt.RoleUser),
					Content: ptr.Of("Calculate 2+2"),
				},
				{
					Role:    ptr.Of(prompt.RoleAssistant),
					Content: ptr.Of("I'll calculate that for you."),
					ToolCalls: []*openapi.ToolCall{
						{
							Index: ptr.Of(int32(0)),
							ID:    ptr.Of("call_123"),
							Type:  ptr.Of(openapi.ToolTypeFunction),
							FunctionCall: &openapi.FunctionCall{
								Name:      ptr.Of("calculator"),
								Arguments: ptr.Of(`{"expression": "2+2"}`),
							},
						},
					},
				},
				{
					Role:       ptr.Of(prompt.RoleTool),
					Content:    ptr.Of("4"),
					ToolCallID: ptr.Of("call_123"),
				},
			},
			want: []*entity.Message{
				{
					Role:    entity.RoleUser,
					Content: ptr.Of("Calculate 2+2"),
				},
				{
					Role:    entity.RoleAssistant,
					Content: ptr.Of("I'll calculate that for you."),
					ToolCalls: []*entity.ToolCall{
						{
							Index: 0,
							ID:    "call_123",
							Type:  entity.ToolTypeFunction,
							FunctionCall: &entity.FunctionCall{
								Name:      "calculator",
								Arguments: ptr.Of(`{"expression": "2+2"}`),
							},
						},
					},
				},
				{
					Role:       entity.RoleTool,
					Content:    ptr.Of("4"),
					ToolCallID: ptr.Of("call_123"),
				},
			},
		},
		{
			name: "messages with content parts",
			dtos: []*openapi.Message{
				{
					Role: ptr.Of(prompt.RoleUser),
					Parts: []*openapi.ContentPart{
						{
							Type: ptr.Of(openapi.ContentTypeText),
							Text: ptr.Of("What's in this image?"),
						},
						{
							Type:     ptr.Of(openapi.ContentTypeImageURL),
							ImageURL: ptr.Of("https://example.com/image.jpg"),
						},
					},
				},
			},
			want: []*entity.Message{
				{
					Role: entity.RoleUser,
					Parts: []*entity.ContentPart{
						{
							Type: entity.ContentTypeText,
							Text: ptr.Of("What's in this image?"),
						},
						{
							Type: entity.ContentTypeImageURL,
							ImageURL: &entity.ImageURL{
								URL: "https://example.com/image.jpg",
							},
						},
					},
				},
			},
		},
		{
			name: "messages with metadata",
			dtos: []*openapi.Message{
				{
					Role:     ptr.Of(prompt.RoleAssistant),
					Metadata: map[string]string{"meta": "value"},
				},
			},
			want: []*entity.Message{
				{
					Role:     entity.RoleAssistant,
					Metadata: map[string]string{"meta": "value"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIBatchMessageDTO2DO(tt.dtos))
		})
	}
}

func TestOpenAPIBatchContentPartDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		dtos []*openapi.ContentPart
		want []*entity.ContentPart
	}{
		{
			name: "nil input",
			dtos: nil,
			want: nil,
		},
		{
			name: "empty array",
			dtos: []*openapi.ContentPart{},
			want: []*entity.ContentPart{},
		},
		{
			name: "array with nil elements",
			dtos: []*openapi.ContentPart{
				nil,
				{
					Type: ptr.Of(openapi.ContentTypeText),
					Text: ptr.Of("Hello"),
				},
				nil,
			},
			want: []*entity.ContentPart{
				{
					Type: entity.ContentTypeText,
					Text: ptr.Of("Hello"),
				},
			},
		},
		{
			name: "normal array conversion",
			dtos: []*openapi.ContentPart{
				{
					Type: ptr.Of(openapi.ContentTypeText),
					Text: ptr.Of("Hello world"),
				},
				{
					Type: ptr.Of(openapi.ContentTypeMultiPartVariable),
					Text: ptr.Of("{{variable}}"),
				},
			},
			want: []*entity.ContentPart{
				{
					Type: entity.ContentTypeText,
					Text: ptr.Of("Hello world"),
				},
				{
					Type: entity.ContentTypeMultiPartVariable,
					Text: ptr.Of("{{variable}}"),
				},
			},
		},
		{
			name: "mixed types with image url and base64",
			dtos: []*openapi.ContentPart{
				{
					Type: ptr.Of(openapi.ContentTypeText),
					Text: ptr.Of("Text content"),
				},
				{
					Type:     ptr.Of(openapi.ContentTypeImageURL),
					Text:     ptr.Of("Image description"),
					ImageURL: ptr.Of("https://example.com/image.jpg"),
				},
				{
					Type:       ptr.Of(openapi.ContentTypeBase64Data),
					Text:       ptr.Of("Base64 image"),
					Base64Data: ptr.Of("base64data"),
				},
			},
			want: []*entity.ContentPart{
				{
					Type: entity.ContentTypeText,
					Text: ptr.Of("Text content"),
				},
				{
					Type: entity.ContentTypeImageURL,
					Text: ptr.Of("Image description"),
					ImageURL: &entity.ImageURL{
						URL: "https://example.com/image.jpg",
					},
				},
				{
					Type:       entity.ContentTypeBase64Data,
					Text:       ptr.Of("Base64 image"),
					Base64Data: ptr.Of("base64data"),
				},
			},
		},
		{
			name: "empty image url handling",
			dtos: []*openapi.ContentPart{
				{
					Type:     ptr.Of(openapi.ContentTypeImageURL),
					ImageURL: ptr.Of(""),
				},
				{
					Type:     ptr.Of(openapi.ContentTypeImageURL),
					ImageURL: nil,
				},
			},
			want: []*entity.ContentPart{
				{
					Type:     entity.ContentTypeImageURL,
					ImageURL: nil,
				},
				{
					Type:     entity.ContentTypeImageURL,
					ImageURL: nil,
				},
			},
		},
		{
			name: "video url handling with fps config",
			dtos: []*openapi.ContentPart{
				{
					Type:     ptr.Of(openapi.ContentTypeVideoURL),
					VideoURL: ptr.Of("https://example.com/video.mp4"),
					Config: &openapi.MediaConfig{
						Fps: ptr.Of(1.8),
					},
				},
			},
			want: []*entity.ContentPart{
				{
					Type: entity.ContentTypeVideoURL,
					VideoURL: &entity.VideoURL{
						URL: "https://example.com/video.mp4",
					},
					MediaConfig: &entity.MediaConfig{
						Fps: ptr.Of(1.8),
					},
				},
			},
		},
		{
			name: "base64 video carries fps without video url",
			dtos: []*openapi.ContentPart{
				{
					Type:       ptr.Of(openapi.ContentTypeBase64Data),
					Base64Data: ptr.Of("data:video/mp4;base64,QUJDRA=="),
					Config: &openapi.MediaConfig{
						Fps: ptr.Of(2.2),
					},
				},
			},
			want: []*entity.ContentPart{
				{
					Type:       entity.ContentTypeBase64Data,
					Base64Data: ptr.Of("data:video/mp4;base64,QUJDRA=="),
					MediaConfig: &entity.MediaConfig{
						Fps: ptr.Of(2.2),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIBatchContentPartDTO2DO(tt.dtos))
		})
	}
}

func TestOpenAPIBatchVariableValDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		dtos []*openapi.VariableVal
		want []*entity.VariableVal
	}{
		{
			name: "nil input",
			dtos: nil,
			want: nil,
		},
		{
			name: "empty array",
			dtos: []*openapi.VariableVal{},
			want: nil,
		},
		{
			name: "array with nil elements",
			dtos: []*openapi.VariableVal{
				nil,
				{
					Key:   ptr.Of("var1"),
					Value: ptr.Of("value1"),
				},
				nil,
			},
			want: []*entity.VariableVal{
				{
					Key:   "var1",
					Value: ptr.Of("value1"),
				},
			},
		},
		{
			name: "normal array conversion",
			dtos: []*openapi.VariableVal{
				{
					Key:   ptr.Of("var1"),
					Value: ptr.Of("simple value"),
				},
				{
					Key:   ptr.Of("var2"),
					Value: ptr.Of("another value"),
				},
			},
			want: []*entity.VariableVal{
				{
					Key:   "var1",
					Value: ptr.Of("simple value"),
				},
				{
					Key:   "var2",
					Value: ptr.Of("another value"),
				},
			},
		},
		{
			name: "complex variable values with placeholder messages",
			dtos: []*openapi.VariableVal{
				{
					Key:   ptr.Of("placeholder_var"),
					Value: ptr.Of("placeholder value"),
					PlaceholderMessages: []*openapi.Message{
						{
							Role:    ptr.Of(prompt.RoleUser),
							Content: ptr.Of("Placeholder content"),
						},
					},
				},
			},
			want: []*entity.VariableVal{
				{
					Key:   "placeholder_var",
					Value: ptr.Of("placeholder value"),
					PlaceholderMessages: []*entity.Message{
						{
							Role:    entity.RoleUser,
							Content: ptr.Of("Placeholder content"),
						},
					},
				},
			},
		},
		{
			name: "variable values with multi part values",
			dtos: []*openapi.VariableVal{
				{
					Key:   ptr.Of("multipart_var"),
					Value: ptr.Of("multipart value"),
					MultiPartValues: []*openapi.ContentPart{
						{
							Type: ptr.Of(openapi.ContentTypeText),
							Text: ptr.Of("Part 1"),
						},
						{
							Type:     ptr.Of(openapi.ContentTypeImageURL),
							ImageURL: ptr.Of("https://example.com/image.jpg"),
						},
					},
				},
			},
			want: []*entity.VariableVal{
				{
					Key:   "multipart_var",
					Value: ptr.Of("multipart value"),
					MultiPartValues: []*entity.ContentPart{
						{
							Type: entity.ContentTypeText,
							Text: ptr.Of("Part 1"),
						},
						{
							Type: entity.ContentTypeImageURL,
							ImageURL: &entity.ImageURL{
								URL: "https://example.com/image.jpg",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIBatchVariableValDTO2DO(tt.dtos))
		})
	}
}

func TestOpenAPITokenUsageDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		do   *entity.TokenUsage
		want *openapi.TokenUsage
	}{
		{
			name: "nil input",
			do:   nil,
			want: nil,
		},
		{
			name: "zero token usage",
			do: &entity.TokenUsage{
				InputTokens:  0,
				OutputTokens: 0,
			},
			want: &openapi.TokenUsage{
				InputTokens:  ptr.Of(int32(0)),
				OutputTokens: ptr.Of(int32(0)),
			},
		},
		{
			name: "normal token usage",
			do: &entity.TokenUsage{
				InputTokens:  100,
				OutputTokens: 50,
			},
			want: &openapi.TokenUsage{
				InputTokens:  ptr.Of(int32(100)),
				OutputTokens: ptr.Of(int32(50)),
			},
		},
		{
			name: "large token usage",
			do: &entity.TokenUsage{
				InputTokens:  999999,
				OutputTokens: 888888,
			},
			want: &openapi.TokenUsage{
				InputTokens:  ptr.Of(int32(999999)),
				OutputTokens: ptr.Of(int32(888888)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPITokenUsageDO2DTO(tt.do))
		})
	}
}

func TestOpenAPIBatchToolCallDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		dos  []*entity.ToolCall
		want []*openapi.ToolCall
	}{
		{
			name: "nil input",
			dos:  nil,
			want: nil,
		},
		{
			name: "empty array",
			dos:  []*entity.ToolCall{},
			want: []*openapi.ToolCall{},
		},
		{
			name: "array with nil elements",
			dos: []*entity.ToolCall{
				nil,
				{
					Index: 0,
					ID:    "call_123",
					Type:  entity.ToolTypeFunction,
					FunctionCall: &entity.FunctionCall{
						Name:      "test_function",
						Arguments: ptr.Of(`{"arg": "value"}`),
					},
				},
				nil,
			},
			want: []*openapi.ToolCall{
				{
					Index: ptr.Of(int32(0)),
					ID:    ptr.Of("call_123"),
					Type:  ptr.Of(openapi.ToolTypeFunction),
					FunctionCall: &openapi.FunctionCall{
						Name:      ptr.Of("test_function"),
						Arguments: ptr.Of(`{"arg": "value"}`),
					},
				},
			},
		},
		{
			name: "normal array conversion",
			dos: []*entity.ToolCall{
				{
					Index: 0,
					ID:    "call_123",
					Type:  entity.ToolTypeFunction,
					FunctionCall: &entity.FunctionCall{
						Name:      "function1",
						Arguments: ptr.Of(`{"param1": "value1"}`),
					},
				},
				{
					Index: 1,
					ID:    "call_456",
					Type:  entity.ToolTypeFunction,
					FunctionCall: &entity.FunctionCall{
						Name:      "function2",
						Arguments: ptr.Of(`{"param2": "value2"}`),
					},
				},
			},
			want: []*openapi.ToolCall{
				{
					Index: ptr.Of(int32(0)),
					ID:    ptr.Of("call_123"),
					Type:  ptr.Of(openapi.ToolTypeFunction),
					FunctionCall: &openapi.FunctionCall{
						Name:      ptr.Of("function1"),
						Arguments: ptr.Of(`{"param1": "value1"}`),
					},
				},
				{
					Index: ptr.Of(int32(1)),
					ID:    ptr.Of("call_456"),
					Type:  ptr.Of(openapi.ToolTypeFunction),
					FunctionCall: &openapi.FunctionCall{
						Name:      ptr.Of("function2"),
						Arguments: ptr.Of(`{"param2": "value2"}`),
					},
				},
			},
		},
		{
			name: "tool call with nil function call",
			dos: []*entity.ToolCall{
				{
					Index:        0,
					ID:           "call_789",
					Type:         entity.ToolTypeFunction,
					FunctionCall: nil,
				},
			},
			want: []*openapi.ToolCall{
				{
					Index:        ptr.Of(int32(0)),
					ID:           ptr.Of("call_789"),
					Type:         ptr.Of(openapi.ToolTypeFunction),
					FunctionCall: nil,
				},
			},
		},
		{
			name: "tool call with function call having nil arguments",
			dos: []*entity.ToolCall{
				{
					Index: 0,
					ID:    "call_999",
					Type:  entity.ToolTypeFunction,
					FunctionCall: &entity.FunctionCall{
						Name:      "function_no_args",
						Arguments: nil,
					},
				},
			},
			want: []*openapi.ToolCall{
				{
					Index: ptr.Of(int32(0)),
					ID:    ptr.Of("call_999"),
					Type:  ptr.Of(openapi.ToolTypeFunction),
					FunctionCall: &openapi.FunctionCall{
						Name:      ptr.Of("function_no_args"),
						Arguments: nil,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIBatchToolCallDO2DTO(tt.dos))
		})
	}
}

func TestOpenAPIBatchToolCallDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		dtos []*openapi.ToolCall
		want []*entity.ToolCall
	}{
		{
			name: "nil input",
			dtos: nil,
			want: nil,
		},
		{
			name: "empty array",
			dtos: []*openapi.ToolCall{},
			want: []*entity.ToolCall{},
		},
		{
			name: "array with nil elements",
			dtos: []*openapi.ToolCall{
				nil,
				{
					Index: ptr.Of(int32(0)),
					ID:    ptr.Of("call_123"),
					Type:  ptr.Of(openapi.ToolTypeFunction),
					FunctionCall: &openapi.FunctionCall{
						Name:      ptr.Of("test_function"),
						Arguments: ptr.Of(`{"arg": "value"}`),
					},
				},
				nil,
			},
			want: []*entity.ToolCall{
				{
					Index: 0,
					ID:    "call_123",
					Type:  entity.ToolTypeFunction,
					FunctionCall: &entity.FunctionCall{
						Name:      "test_function",
						Arguments: ptr.Of(`{"arg": "value"}`),
					},
				},
			},
		},
		{
			name: "normal array conversion",
			dtos: []*openapi.ToolCall{
				{
					Index: ptr.Of(int32(0)),
					ID:    ptr.Of("call_123"),
					Type:  ptr.Of(openapi.ToolTypeFunction),
					FunctionCall: &openapi.FunctionCall{
						Name:      ptr.Of("function1"),
						Arguments: ptr.Of(`{"param1": "value1"}`),
					},
				},
				{
					Index: ptr.Of(int32(1)),
					ID:    ptr.Of("call_456"),
					Type:  ptr.Of(openapi.ToolTypeFunction),
					FunctionCall: &openapi.FunctionCall{
						Name:      ptr.Of("function2"),
						Arguments: ptr.Of(`{"param2": "value2"}`),
					},
				},
			},
			want: []*entity.ToolCall{
				{
					Index: 0,
					ID:    "call_123",
					Type:  entity.ToolTypeFunction,
					FunctionCall: &entity.FunctionCall{
						Name:      "function1",
						Arguments: ptr.Of(`{"param1": "value1"}`),
					},
				},
				{
					Index: 1,
					ID:    "call_456",
					Type:  entity.ToolTypeFunction,
					FunctionCall: &entity.FunctionCall{
						Name:      "function2",
						Arguments: ptr.Of(`{"param2": "value2"}`),
					},
				},
			},
		},
		{
			name: "tool call with nil function call",
			dtos: []*openapi.ToolCall{
				{
					Index:        ptr.Of(int32(0)),
					ID:           ptr.Of("call_789"),
					Type:         ptr.Of(openapi.ToolTypeFunction),
					FunctionCall: nil,
				},
			},
			want: []*entity.ToolCall{
				{
					Index:        0,
					ID:           "call_789",
					Type:         entity.ToolTypeFunction,
					FunctionCall: nil,
				},
			},
		},
		{
			name: "tool call with function call having nil arguments",
			dtos: []*openapi.ToolCall{
				{
					Index: ptr.Of(int32(0)),
					ID:    ptr.Of("call_999"),
					Type:  ptr.Of(openapi.ToolTypeFunction),
					FunctionCall: &openapi.FunctionCall{
						Name:      ptr.Of("function_no_args"),
						Arguments: nil,
					},
				},
			},
			want: []*entity.ToolCall{
				{
					Index: 0,
					ID:    "call_999",
					Type:  entity.ToolTypeFunction,
					FunctionCall: &entity.FunctionCall{
						Name:      "function_no_args",
						Arguments: nil,
					},
				},
			},
		},
		{
			name: "tool call with default values from getters",
			dtos: []*openapi.ToolCall{
				{
					// 测试GetIndex()、GetID()、GetType()的默认值处理
				},
			},
			want: []*entity.ToolCall{
				{
					Index: 0,                       // int32默认值转int64
					ID:    "",                      // string默认值
					Type:  entity.ToolTypeFunction, // 默认映射到Function
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIBatchToolCallDTO2DO(tt.dtos))
		})
	}
}

func TestOpenAPIPromptBasicDO2DTO(t *testing.T) {
	createdAt := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
	latestCommittedAt := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		do   *entity.Prompt
		want *openapi.PromptBasic
	}{
		{
			name: "nil input",
			do:   nil,
			want: nil,
		},
		{
			name: "nil prompt basic",
			do: &entity.Prompt{
				ID:          123,
				SpaceID:     456,
				PromptKey:   "test_prompt",
				PromptBasic: nil,
			},
			want: nil,
		},
		{
			name: "empty prompt basic",
			do: &entity.Prompt{
				ID:          123,
				SpaceID:     456,
				PromptKey:   "test_prompt",
				PromptBasic: &entity.PromptBasic{},
			},
			want: &openapi.PromptBasic{
				ID:                ptr.Of(int64(123)),
				WorkspaceID:       ptr.Of(int64(456)),
				PromptKey:         ptr.Of("test_prompt"),
				DisplayName:       ptr.Of(""),
				Description:       ptr.Of(""),
				LatestVersion:     ptr.Of(""),
				CreatedBy:         ptr.Of(""),
				UpdatedBy:         ptr.Of(""),
				CreatedAt:         ptr.Of(time.Time{}.UnixMilli()), // zero value time
				UpdatedAt:         ptr.Of(time.Time{}.UnixMilli()), // zero value time
				LatestCommittedAt: nil,
			},
		},
		{
			name: "complete prompt basic without latest committed at",
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptBasic: &entity.PromptBasic{
					DisplayName:       "Test Prompt",
					Description:       "A test prompt for testing",
					LatestVersion:     "1.0.0",
					CreatedBy:         "user123",
					UpdatedBy:         "user456",
					CreatedAt:         createdAt,
					UpdatedAt:         updatedAt,
					LatestCommittedAt: nil,
				},
			},
			want: &openapi.PromptBasic{
				ID:                ptr.Of(int64(123)),
				WorkspaceID:       ptr.Of(int64(456)),
				PromptKey:         ptr.Of("test_prompt"),
				DisplayName:       ptr.Of("Test Prompt"),
				Description:       ptr.Of("A test prompt for testing"),
				LatestVersion:     ptr.Of("1.0.0"),
				CreatedBy:         ptr.Of("user123"),
				UpdatedBy:         ptr.Of("user456"),
				CreatedAt:         ptr.Of(createdAt.UnixMilli()),
				UpdatedAt:         ptr.Of(updatedAt.UnixMilli()),
				LatestCommittedAt: nil,
			},
		},
		{
			name: "complete prompt basic with latest committed at",
			do: &entity.Prompt{
				ID:        123,
				SpaceID:   456,
				PromptKey: "test_prompt",
				PromptBasic: &entity.PromptBasic{
					DisplayName:       "Test Prompt",
					Description:       "A test prompt for testing",
					LatestVersion:     "1.0.0",
					CreatedBy:         "user123",
					UpdatedBy:         "user456",
					CreatedAt:         createdAt,
					UpdatedAt:         updatedAt,
					LatestCommittedAt: &latestCommittedAt,
				},
			},
			want: &openapi.PromptBasic{
				ID:                ptr.Of(int64(123)),
				WorkspaceID:       ptr.Of(int64(456)),
				PromptKey:         ptr.Of("test_prompt"),
				DisplayName:       ptr.Of("Test Prompt"),
				Description:       ptr.Of("A test prompt for testing"),
				LatestVersion:     ptr.Of("1.0.0"),
				CreatedBy:         ptr.Of("user123"),
				UpdatedBy:         ptr.Of("user456"),
				CreatedAt:         ptr.Of(createdAt.UnixMilli()),
				UpdatedAt:         ptr.Of(updatedAt.UnixMilli()),
				LatestCommittedAt: ptr.Of(latestCommittedAt.UnixMilli()),
			},
		},
		{
			name: "prompt basic with zero IDs",
			do: &entity.Prompt{
				ID:        0,
				SpaceID:   0,
				PromptKey: "",
				PromptBasic: &entity.PromptBasic{
					DisplayName:   "New Prompt",
					Description:   "A newly created prompt",
					LatestVersion: "",
					CreatedBy:     "user789",
					UpdatedBy:     "user789",
					CreatedAt:     createdAt,
					UpdatedAt:     createdAt,
				},
			},
			want: &openapi.PromptBasic{
				ID:                ptr.Of(int64(0)),
				WorkspaceID:       ptr.Of(int64(0)),
				PromptKey:         ptr.Of(""),
				DisplayName:       ptr.Of("New Prompt"),
				Description:       ptr.Of("A newly created prompt"),
				LatestVersion:     ptr.Of(""),
				CreatedBy:         ptr.Of("user789"),
				UpdatedBy:         ptr.Of("user789"),
				CreatedAt:         ptr.Of(createdAt.UnixMilli()),
				UpdatedAt:         ptr.Of(createdAt.UnixMilli()),
				LatestCommittedAt: nil,
			},
		},
		{
			name: "prompt basic with special characters in text fields",
			do: &entity.Prompt{
				ID:        999,
				SpaceID:   888,
				PromptKey: "prompt_with_special_chars_@#$",
				PromptBasic: &entity.PromptBasic{
					DisplayName:   "Prompt with 中文 and émojis 🎉",
					Description:   "Description with\nnewlines\tand\ttabs",
					LatestVersion: "2.3.1-beta",
					CreatedBy:     "user@example.com",
					UpdatedBy:     "another.user@example.com",
					CreatedAt:     createdAt,
					UpdatedAt:     updatedAt,
				},
			},
			want: &openapi.PromptBasic{
				ID:                ptr.Of(int64(999)),
				WorkspaceID:       ptr.Of(int64(888)),
				PromptKey:         ptr.Of("prompt_with_special_chars_@#$"),
				DisplayName:       ptr.Of("Prompt with 中文 and émojis 🎉"),
				Description:       ptr.Of("Description with\nnewlines\tand\ttabs"),
				LatestVersion:     ptr.Of("2.3.1-beta"),
				CreatedBy:         ptr.Of("user@example.com"),
				UpdatedBy:         ptr.Of("another.user@example.com"),
				CreatedAt:         ptr.Of(createdAt.UnixMilli()),
				UpdatedAt:         ptr.Of(updatedAt.UnixMilli()),
				LatestCommittedAt: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIPromptBasicDO2DTO(tt.do))
		})
	}
}
