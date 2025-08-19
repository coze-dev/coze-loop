// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/common"
	runtimedto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/runtime"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/runtime"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestLLMCallParamConvert(t *testing.T) {
	tests := []struct {
		name  string
		param rpc.LLMCallParam
		want  *runtime.ChatRequest
	}{
		{
			name:  "empty param",
			param: rpc.LLMCallParam{},
			want: &runtime.ChatRequest{
				ModelConfig: nil,
				Messages:    nil,
				Tools:       nil,
				BizParam: &runtimedto.BizParam{
					WorkspaceID:           ptr.Of(int64(0)),
					UserID:                nil,
					Scenario:              ptr.Of(common.ScenarioDefault),
					ScenarioEntityID:      ptr.Of("0"),
					ScenarioEntityVersion: ptr.Of(""),
					ScenarioEntityKey:     ptr.Of(""),
				},
			},
		},
		{
			name: "full param",
			param: rpc.LLMCallParam{
				SpaceID:       123,
				PromptID:      456,
				PromptKey:     "test_key",
				PromptVersion: "1.0.0",
				Scenario:      entity.ScenarioDefault,
				UserID:        ptr.Of("test_user"),
				Messages: []*entity.Message{
					{
						Role: "user",
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeText,
								Text: ptr.Of("test message"),
							},
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URL: "test_url",
								},
							},
						},
					},
				},
				Tools: []*entity.Tool{
					{
						Type: entity.ToolTypeFunction,
						Function: &entity.Function{
							Name:        "get_weather",
							Description: "tool for get weather",
							Parameters:  "test_arguments_schema",
						},
					},
				},
				ToolCallConfig: &entity.ToolCallConfig{
					ToolChoice: entity.ToolChoiceTypeAuto,
				},
				ModelConfig: &entity.ModelConfig{
					ModelID:          1111,
					MaxTokens:        ptr.Of(int32(1000)),
					Temperature:      ptr.Of(0.5),
					TopK:             nil,
					TopP:             ptr.Of(0.1),
					PresencePenalty:  nil,
					FrequencyPenalty: nil,
					JSONMode:         nil,
				},
			},
			want: &runtime.ChatRequest{
				ModelConfig: &runtimedto.ModelConfig{
					ModelID:     1111,
					Temperature: ptr.Of(0.5),
					MaxTokens:   ptr.Of(int64(1000)),
					TopP:        ptr.Of(0.1),
					Stop:        nil,
					ToolChoice:  ptr.Of(runtimedto.ToolChoiceAuto),
				},
				Messages: []*runtimedto.Message{
					{
						Role: runtimedto.RoleUser,
						MultimodalContents: []*runtimedto.ChatMessagePart{
							{
								Type: ptr.Of(runtimedto.ChatMessagePartTypeText),
								Text: ptr.Of("test message"),
							},
							{
								Type: ptr.Of(runtimedto.ChatMessagePartTypeImageURL),
								ImageURL: &runtimedto.ChatMessageImageURL{
									URL: ptr.Of("test_url"),
								},
							},
						},
					},
				},
				Tools: []*runtimedto.Tool{
					{
						Name:    ptr.Of("get_weather"),
						Desc:    ptr.Of("tool for get weather"),
						DefType: ptr.Of(runtimedto.ToolDefTypeOpenAPIV3),
						Def:     ptr.Of("test_arguments_schema"),
					},
				},
				BizParam: &runtimedto.BizParam{
					WorkspaceID:           ptr.Of(int64(123)),
					UserID:                ptr.Of("test_user"),
					Scenario:              ptr.Of(common.ScenarioDefault),
					ScenarioEntityID:      ptr.Of("456"),
					ScenarioEntityVersion: ptr.Of("1.0.0"),
					ScenarioEntityKey:     ptr.Of("test_key"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := LLMCallParamConvert(tt.param)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMessageDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		do   *entity.Message
		want *runtimedto.Message
	}{
		{
			name: "nil message",
			do:   nil,
			want: nil,
		},
		{
			name: "user text message",
			do: &entity.Message{
				Role:    "user",
				Content: ptr.Of("Hello, how are you?"),
				Parts: []*entity.ContentPart{
					{
						Type: entity.ContentTypeText,
						Text: ptr.Of("Hello, how are you?"),
					},
				},
			},
			want: &runtimedto.Message{
				Role:    runtimedto.RoleUser,
				Content: ptr.Of("Hello, how are you?"),
				MultimodalContents: []*runtimedto.ChatMessagePart{
					{
						Type: ptr.Of(runtimedto.ChatMessagePartTypeText),
						Text: ptr.Of("Hello, how are you?"),
					},
				},
			},
		},
		{
			name: "user multimodal message",
			do: &entity.Message{
				Role: "user",
				Parts: []*entity.ContentPart{
					{
						Type: entity.ContentTypeText,
						Text: ptr.Of("What's in this image?"),
					},
					{
						Type: entity.ContentTypeImageURL,
						ImageURL: &entity.ImageURL{
							URL: "https://example.com/image.jpg",
							URI: "image.jpg",
						},
					},
				},
			},
			want: &runtimedto.Message{
				Role: runtimedto.RoleUser,
				MultimodalContents: []*runtimedto.ChatMessagePart{
					{
						Type: ptr.Of(runtimedto.ChatMessagePartTypeText),
						Text: ptr.Of("What's in this image?"),
					},
					{
						Type: ptr.Of(runtimedto.ChatMessagePartTypeImageURL),
						ImageURL: &runtimedto.ChatMessageImageURL{
							URL: ptr.Of("https://example.com/image.jpg"),
						},
					},
				},
			},
		},
		{
			name: "ai tool call message",
			do: &entity.Message{
				Role: "assistant",
				ToolCalls: []*entity.ToolCall{
					{
						Index: 0,
						ID:    "call_123",
						Type:  entity.ToolTypeFunction,
						FunctionCall: &entity.FunctionCall{
							Name:      "get_weather",
							Arguments: ptr.Of("{\"location\":\"Beijing\"}"),
						},
					},
				},
			},
			want: &runtimedto.Message{
				Role: runtimedto.RoleAssistant,
				ToolCalls: []*runtimedto.ToolCall{
					{
						Index: ptr.Of(int64(0)),
						ID:    ptr.Of("call_123"),
						Type:  ptr.Of(runtimedto.ToolTypeFunction),
						FunctionCall: &runtimedto.FunctionCall{
							Name:      ptr.Of("get_weather"),
							Arguments: ptr.Of("{\"location\":\"Beijing\"}"),
						},
					},
				},
			},
		},
		{
			name: "tool result message",
			do: &entity.Message{
				Role:       "tool",
				ToolCallID: ptr.Of("call_123"),
				Content:    ptr.Of("{\"temperature\":25,\"condition\":\"sunny\"}"),
			},
			want: &runtimedto.Message{
				Role:       runtimedto.RoleTool,
				ToolCallID: ptr.Of("call_123"),
				Content:    ptr.Of("{\"temperature\":25,\"condition\":\"sunny\"}"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MessageDO2DTO(tt.do)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplyItemDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		dto  *runtimedto.Message
		want *entity.ReplyItem
	}{
		{
			name: "nil message",
			dto:  nil,
			want: nil,
		},
		{
			name: "basic message without meta",
			dto: &runtimedto.Message{
				Role:    runtimedto.RoleAssistant,
				Content: ptr.Of("Here's the weather information."),
			},
			want: &entity.ReplyItem{
				Message: &entity.Message{
					Role:    entity.RoleAssistant,
					Content: ptr.Of("Here's the weather information."),
				},
				FinishReason: "",
				TokenUsage:   nil,
			},
		},
		{
			name: "message with finish reason",
			dto: &runtimedto.Message{
				Role:    runtimedto.RoleAssistant,
				Content: ptr.Of("Here's the weather information."),
				ResponseMeta: &runtimedto.ResponseMeta{
					FinishReason: ptr.Of("stop"),
				},
			},
			want: &entity.ReplyItem{
				Message: &entity.Message{
					Role:    entity.RoleAssistant,
					Content: ptr.Of("Here's the weather information."),
				},
				FinishReason: "stop",
				TokenUsage:   nil,
			},
		},
		{
			name: "message with token usage",
			dto: &runtimedto.Message{
				Role:    runtimedto.RoleAssistant,
				Content: ptr.Of("Here's the weather information."),
				ResponseMeta: &runtimedto.ResponseMeta{
					Usage: &runtimedto.TokenUsage{
						PromptTokens:     ptr.Of(int64(100)),
						CompletionTokens: ptr.Of(int64(50)),
						TotalTokens:      ptr.Of(int64(150)),
					},
				},
			},
			want: &entity.ReplyItem{
				Message: &entity.Message{
					Role:    entity.RoleAssistant,
					Content: ptr.Of("Here's the weather information."),
				},
				FinishReason: "",
				TokenUsage: &entity.TokenUsage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
		},
		{
			name: "message with all meta fields",
			dto: &runtimedto.Message{
				Role:    runtimedto.RoleAssistant,
				Content: ptr.Of("Here's the weather information."),
				ResponseMeta: &runtimedto.ResponseMeta{
					FinishReason: ptr.Of("tool_calls"),
					Usage: &runtimedto.TokenUsage{
						PromptTokens:     ptr.Of(int64(200)),
						CompletionTokens: ptr.Of(int64(100)),
						TotalTokens:      ptr.Of(int64(300)),
					},
				},
			},
			want: &entity.ReplyItem{
				Message: &entity.Message{
					Role:    entity.RoleAssistant,
					Content: ptr.Of("Here's the weather information."),
				},
				FinishReason: "tool_calls",
				TokenUsage: &entity.TokenUsage{
					InputTokens:  200,
					OutputTokens: 100,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ReplyItemDTO2DO(tt.dto)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMessageDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		dto  *runtimedto.Message
		want *entity.Message
	}{
		{
			name: "nil message",
			dto:  nil,
			want: nil,
		},
		{
			name: "basic text message",
			dto: &runtimedto.Message{
				Role:    runtimedto.RoleUser,
				Content: ptr.Of("Hello, how are you?"),
			},
			want: &entity.Message{
				Role:    entity.RoleUser,
				Content: ptr.Of("Hello, how are you?"),
			},
		},
		{
			name: "message with reasoning content",
			dto: &runtimedto.Message{
				Role:             runtimedto.RoleAssistant,
				Content:          ptr.Of("The weather is sunny."),
				ReasoningContent: ptr.Of("I checked the weather API and found that..."),
			},
			want: &entity.Message{
				Role:             entity.RoleAssistant,
				Content:          ptr.Of("The weather is sunny."),
				ReasoningContent: ptr.Of("I checked the weather API and found that..."),
			},
		},
		{
			name: "multimodal message",
			dto: &runtimedto.Message{
				Role: runtimedto.RoleUser,
				MultimodalContents: []*runtimedto.ChatMessagePart{
					{
						Type: ptr.Of(runtimedto.ChatMessagePartTypeText),
						Text: ptr.Of("What's in this image?"),
					},
					{
						Type: ptr.Of(runtimedto.ChatMessagePartTypeImageURL),
						ImageURL: &runtimedto.ChatMessageImageURL{
							URL: ptr.Of("https://example.com/image.jpg"),
						},
					},
				},
			},
			want: &entity.Message{
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
		{
			name: "message with tool call",
			dto: &runtimedto.Message{
				Role: runtimedto.RoleAssistant,
				ToolCalls: []*runtimedto.ToolCall{
					{
						Index: ptr.Of(int64(0)),
						ID:    ptr.Of("call_123"),
						Type:  ptr.Of(runtimedto.ToolTypeFunction),
						FunctionCall: &runtimedto.FunctionCall{
							Name:      ptr.Of("get_weather"),
							Arguments: ptr.Of("{\"location\":\"Beijing\"}"),
						},
					},
				},
			},
			want: &entity.Message{
				Role: entity.RoleAssistant,
				ToolCalls: []*entity.ToolCall{
					{
						Index: 0,
						ID:    "call_123",
						Type:  entity.ToolTypeFunction,
						FunctionCall: &entity.FunctionCall{
							Name:      "get_weather",
							Arguments: ptr.Of("{\"location\":\"Beijing\"}"),
						},
					},
				},
			},
		},
		{
			name: "tool result message",
			dto: &runtimedto.Message{
				Role:       runtimedto.RoleTool,
				ToolCallID: ptr.Of("call_123"),
				Content:    ptr.Of("{\"temperature\":25,\"condition\":\"sunny\"}"),
			},
			want: &entity.Message{
				Role:       entity.RoleTool,
				ToolCallID: ptr.Of("call_123"),
				Content:    ptr.Of("{\"temperature\":25,\"condition\":\"sunny\"}"),
			},
		},
		{
			name: "complete message with all fields",
			dto: &runtimedto.Message{
				Role:             runtimedto.RoleAssistant,
				Content:          ptr.Of("Here's the analysis."),
				ReasoningContent: ptr.Of("Let me analyze this step by step..."),
				MultimodalContents: []*runtimedto.ChatMessagePart{
					{
						Type: ptr.Of(runtimedto.ChatMessagePartTypeText),
						Text: ptr.Of("Here's the analysis."),
					},
				},
				ToolCallID: ptr.Of("call_123"),
				ToolCalls: []*runtimedto.ToolCall{
					{
						Index: ptr.Of(int64(0)),
						ID:    ptr.Of("call_123"),
						Type:  ptr.Of(runtimedto.ToolTypeFunction),
						FunctionCall: &runtimedto.FunctionCall{
							Name:      ptr.Of("analyze_data"),
							Arguments: ptr.Of("{\"data\":\"sample\"}"),
						},
					},
				},
			},
			want: &entity.Message{
				Role:             entity.RoleAssistant,
				Content:          ptr.Of("Here's the analysis."),
				ReasoningContent: ptr.Of("Let me analyze this step by step..."),
				Parts: []*entity.ContentPart{
					{
						Type: entity.ContentTypeText,
						Text: ptr.Of("Here's the analysis."),
					},
				},
				ToolCallID: ptr.Of("call_123"),
				ToolCalls: []*entity.ToolCall{
					{
						Index: 0,
						ID:    "call_123",
						Type:  entity.ToolTypeFunction,
						FunctionCall: &entity.FunctionCall{
							Name:      "analyze_data",
							Arguments: ptr.Of("{\"data\":\"sample\"}"),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MessageDTO2DO(tt.dto)
			assert.Equal(t, tt.want, got)
		})
	}
}
