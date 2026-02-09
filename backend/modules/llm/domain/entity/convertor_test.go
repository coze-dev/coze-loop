// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/manage"
	druntime "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/runtime"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/stretchr/testify/assert"
)

func TestMergeStreamMsgs(t *testing.T) {
	type args struct {
		msgs []*Message
	}
	tests := []struct {
		name string
		args args
		want *Message
	}{
		{
			name: "MergeStreamMsgs",
			args: args{
				msgs: []*Message{
					{
						ToolCalls: []*ToolCall{
							{
								ID: "id1",
								Function: &FunctionCall{
									Name:      "",
									Arguments: "arg1",
								},
							},
						},
					},
					{
						Role:             RoleAssistant,
						Content:          "你",
						ReasoningContent: "你",
						MultiModalContent: []*ChatMessagePart{
							{
								Type: ChatMessagePartTypeText,
								Text: "你好",
							},
						},
						Name: "",
						ToolCalls: []*ToolCall{
							{
								Function: &FunctionCall{
									Name:      "",
									Arguments: "arg2",
								},
							},
						},
						ToolCallID: "",
						ResponseMeta: &ResponseMeta{
							FinishReason: "stop",
							Usage:        nil,
						},
					},
					{
						Role:             RoleAssistant,
						Content:          "好",
						ReasoningContent: "好",
						MultiModalContent: []*ChatMessagePart{
							{
								Type: ChatMessagePartTypeText,
								Text: "你好",
							},
						},
						Name: "",
						ToolCalls: []*ToolCall{
							{
								ID: "id2",
								Function: &FunctionCall{
									Name:      "",
									Arguments: "arg1",
								},
							},
						},
						ToolCallID: "",
						ResponseMeta: &ResponseMeta{
							FinishReason: "",
							Usage: &TokenUsage{
								PromptTokens:     100,
								CompletionTokens: 10,
								TotalTokens:      110,
							},
						},
					},
				},
			},
			want: &Message{
				Role:             RoleAssistant,
				Content:          "你好",
				ReasoningContent: "你好",
				MultiModalContent: []*ChatMessagePart{
					{
						Type: ChatMessagePartTypeText,
						Text: "你好",
					},
					{
						Type: ChatMessagePartTypeText,
						Text: "你好",
					},
				},
				Name: "",
				ToolCalls: []*ToolCall{
					{
						ID: "id1",
						Function: &FunctionCall{
							Name:      "",
							Arguments: "arg1arg2",
						},
					},
					{
						ID: "id2",
						Function: &FunctionCall{
							Name:      "",
							Arguments: "arg1",
						},
					},
				},
				ToolCallID: "",
				ResponseMeta: &ResponseMeta{
					FinishReason: "stop",
					Usage: &TokenUsage{
						PromptTokens:     100,
						CompletionTokens: 10,
						TotalTokens:      110,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, MergeStreamMsgs(tt.args.msgs), "MergeStreamMsgs(%v)", tt.args.msgs)
		})
	}
}

func TestOptionsToTrace(t *testing.T) {
	type args struct {
		os []Option
	}
	tests := []struct {
		name string
		args args
		want *tracespec.ModelCallOption
	}{
		{
			name: "OptionsToTrace success",
			args: args{
				os: []Option{
					WithTemperature(0.5),
					WithMaxTokens(100),
					WithTopP(0.5),
				},
			},
			want: &tracespec.ModelCallOption{
				Temperature: 0.5,
				MaxTokens:   100,
				TopP:        0.5,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, OptionsToTrace(tt.args.os), "OptionsToTrace(%v)", tt.args.os)
		})
	}
}

func TestStreamMsgsToTraceModelChoices(t *testing.T) {
	type args struct {
		msgs []*Message
	}
	tests := []struct {
		name string
		args args
		want *tracespec.ModelOutput
	}{
		{
			name: "StreamMsgsToTraceModelChoices success",
			args: args{
				msgs: []*Message{
					{
						ToolCalls: []*ToolCall{
							{
								ID: "id1",
								Function: &FunctionCall{
									Name:      "",
									Arguments: "arg1",
								},
							},
						},
					},
					{
						Role:             RoleAssistant,
						Content:          "你",
						ReasoningContent: "你",
						MultiModalContent: []*ChatMessagePart{
							{
								Type: ChatMessagePartTypeText,
								Text: "你好",
							},
						},
						Name: "",
						ToolCalls: []*ToolCall{
							{
								Function: &FunctionCall{
									Name:      "",
									Arguments: "arg2",
								},
							},
						},
						ToolCallID: "",
						ResponseMeta: &ResponseMeta{
							FinishReason: "stop",
							Usage:        nil,
						},
					},
					{
						Role:             RoleAssistant,
						Content:          "好",
						ReasoningContent: "好",
						MultiModalContent: []*ChatMessagePart{
							{
								Type: ChatMessagePartTypeText,
								Text: "你好",
							},
						},
						Name: "",
						ToolCalls: []*ToolCall{
							{
								ID: "id2",
								Function: &FunctionCall{
									Name:      "",
									Arguments: "arg1",
								},
							},
						},
						ToolCallID: "",
						ResponseMeta: &ResponseMeta{
							FinishReason: "",
							Usage: &TokenUsage{
								PromptTokens:     100,
								CompletionTokens: 10,
								TotalTokens:      110,
							},
						},
					},
				},
			},
			want: &tracespec.ModelOutput{
				Choices: []*tracespec.ModelChoice{
					{
						FinishReason: "stop",
						Index:        0,
						Message: &tracespec.ModelMessage{
							Role:             tracespec.VRoleAssistant,
							Content:          "你好",
							ReasoningContent: "你好",
							Parts: []*tracespec.ModelMessagePart{
								{
									Type: tracespec.ModelMessagePartTypeText,
									Text: "你好",
								},
								{
									Type: tracespec.ModelMessagePartTypeText,
									Text: "你好",
								},
							},
							ToolCalls: []*tracespec.ModelToolCall{
								{
									ID: "id1",
									Function: &tracespec.ModelToolCallFunction{
										Name:      "",
										Arguments: "arg1arg2",
									},
								},
								{
									ID: "id2",
									Function: &tracespec.ModelToolCallFunction{
										Name:      "",
										Arguments: "arg1",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, StreamMsgsToTraceModelChoices(tt.args.msgs), "StreamMsgsToTraceModelChoices(%v)", tt.args.msgs)
		})
	}
}

func TestToTraceModelInput(t *testing.T) {
	type args struct {
		msgs []*Message
		ts   []*ToolInfo
		tc   *ToolChoice
	}
	tests := []struct {
		name string
		args args
		want *tracespec.ModelInput
	}{
		{
			name: "ToTraceModelInput success",
			args: args{
				msgs: []*Message{
					{
						Role:             RoleAssistant,
						Content:          "你好",
						ReasoningContent: "你好",
						MultiModalContent: []*ChatMessagePart{
							{
								Type: ChatMessagePartTypeText,
								Text: "你好",
							},
							{
								Type: ChatMessagePartTypeImageURL,
								ImageURL: &ChatMessageImageURL{
									URL: "your url",
								},
							},
						},
						ToolCalls: []*ToolCall{
							{
								ID: "id1",
								Function: &FunctionCall{
									Name:      "",
									Arguments: "arg1",
								},
							},
						},
					},
				},
				ts: []*ToolInfo{
					{
						Name:        "test",
						Desc:        "test",
						ToolDefType: ToolDefTypeOpenAPIV3,
						Def:         "test",
					},
				},
				tc: ptr.Of(ToolChoiceAuto),
			},
			want: &tracespec.ModelInput{
				Messages: []*tracespec.ModelMessage{
					{
						Role:             tracespec.VRoleAssistant,
						Content:          "你好",
						ReasoningContent: "你好",
						Parts: []*tracespec.ModelMessagePart{
							{
								Type: tracespec.ModelMessagePartTypeText,
								Text: "你好",
							},
							{
								Type: tracespec.ModelMessagePartTypeImage,
								ImageURL: &tracespec.ModelImageURL{
									URL: "your url",
								},
							},
						},
						ToolCalls: []*tracespec.ModelToolCall{
							{
								ID: "id1",
								Function: &tracespec.ModelToolCallFunction{
									Name:      "",
									Arguments: "arg1",
								},
							},
						},
					},
				},
				Tools: []*tracespec.ModelTool{
					{
						Type: tracespec.VToolChoiceFunction,
						Function: &tracespec.ModelToolFunction{
							Name:        "test",
							Description: "test",
							Parameters:  []byte("test"),
						},
					},
				},
				ModelToolChoice: &tracespec.ModelToolChoice{
					Type: string(ToolChoiceAuto),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, ToTraceModelInput(tt.args.msgs, tt.args.ts, tt.args.tc), "ToTraceModelInput(%v, %v, %v)", tt.args.msgs, tt.args.ts, tt.args.tc)
		})
	}
}

func TestConvertToParamValues(t *testing.T) {
	type args struct {
		model       *Model
		paramValues []*druntime.ParamConfigValue
	}
	tests := []struct {
		name string
		args args
		want map[string]*ParamValue
	}{
		{
			name: "model is nil",
			args: args{
				model:       nil,
				paramValues: []*druntime.ParamConfigValue{},
			},
			want: nil,
		},
		{
			name: "model param config is nil",
			args: args{
				model:       &Model{},
				paramValues: []*druntime.ParamConfigValue{},
			},
			want: nil,
		},
		{
			name: "param values is empty",
			args: args{
				model: &Model{
					ParamConfig: &ParamConfig{
						ParamSchemas: []*ParamSchema{
							{
								Name:     "temperature",
								Type:     ParamTypeFloat,
								JsonPath: "temperature",
							},
						},
					},
				},
				paramValues: []*druntime.ParamConfigValue{},
			},
			want: map[string]*ParamValue{},
		},
		{
			name: "param values with non-existent param",
			args: args{
				model: &Model{
					ParamConfig: &ParamConfig{
						ParamSchemas: []*ParamSchema{
							{
								Name:     "temperature",
								Type:     ParamTypeFloat,
								JsonPath: "temperature",
							},
						},
					},
				},
				paramValues: []*druntime.ParamConfigValue{
					{
						Name: ptr.Of("non_existent"),
						Value: &manage.ParamOption{
							Value: ptr.Of("0.5"),
						},
					},
				},
			},
			want: map[string]*ParamValue{},
		},
		{
			name: "param values with existing param",
			args: args{
				model: &Model{
					ParamConfig: &ParamConfig{
						ParamSchemas: []*ParamSchema{
							{
								Name:     "temperature",
								Type:     ParamTypeFloat,
								JsonPath: "temperature",
							},
							{
								Name:     "max_tokens",
								Type:     ParamTypeInt,
								JsonPath: "max_tokens",
							},
						},
					},
				},
				paramValues: []*druntime.ParamConfigValue{
					{
						Name: ptr.Of("temperature"),
						Value: &manage.ParamOption{
							Value: ptr.Of("0.5"),
						},
					},
					{
						Name: ptr.Of("max_tokens"),
						Value: &manage.ParamOption{
							Value: ptr.Of("1000"),
						},
					},
					{
						Name: ptr.Of("non_existent"),
						Value: &manage.ParamOption{
							Value: ptr.Of("test"),
						},
					},
				},
			},
			want: map[string]*ParamValue{
				"temperature": {
					Name:      "temperature",
					ParamType: ParamTypeFloat,
					Value:     "0.5",
					JsonPath:  "temperature",
				},
				"max_tokens": {
					Name:      "max_tokens",
					ParamType: ParamTypeInt,
					Value:     "1000",
					JsonPath:  "max_tokens",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, ConvertToParamValues(tt.args.model, tt.args.paramValues), "ConvertToParamValues(%v, %v)", tt.args.model, tt.args.paramValues)
		})
	}
}
