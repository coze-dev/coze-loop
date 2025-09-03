// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracer

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/stretchr/testify/assert"

	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestConvertPrompt2Ob(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		originMessages []*commonentity.Message
		variables      []*tracespec.PromptArgument
		expected       *tracespec.PromptInput
	}{
		{
			name:           "nil messages",
			originMessages: nil,
			variables:      nil,
			expected: &tracespec.PromptInput{
				Templates: []*tracespec.ModelMessage{},
				Arguments: nil,
			},
		},
		{
			name: "single message",
			originMessages: []*commonentity.Message{
				{
					Role: commonentity.RoleUser,
					Content: &commonentity.Content{
						Text: gptr.Of("Hello"),
					},
				},
			},
			variables: []*tracespec.PromptArgument{
				{
					Key:   "user_name",
					Value: "John",
				},
			},
			expected: &tracespec.PromptInput{
				Templates: []*tracespec.ModelMessage{
					{
						Role:      "user",
						Content:   "Hello",
						Parts:     []*tracespec.ModelMessagePart{},
						ToolCalls: []*tracespec.ModelToolCall{},
					},
				},
				Arguments: []*tracespec.PromptArgument{
					{
						Key:   "user_name",
						Value: "John",
					},
				},
			},
		},
		{
			name: "multiple messages",
			originMessages: []*commonentity.Message{
				{
					Role: commonentity.RoleSystem,
					Content: &commonentity.Content{
						Text: gptr.Of("You are a helpful assistant"),
					},
				},
				{
					Role: commonentity.RoleUser,
					Content: &commonentity.Content{
						Text: gptr.Of("What is AI?"),
					},
				},
			},
			variables: nil,
			expected: &tracespec.PromptInput{
				Templates: []*tracespec.ModelMessage{
					{
						Role:      "system",
						Content:   "You are a helpful assistant",
						Parts:     []*tracespec.ModelMessagePart{},
						ToolCalls: []*tracespec.ModelToolCall{},
					},
					{
						Role:      "user",
						Content:   "What is AI?",
						Parts:     []*tracespec.ModelMessagePart{},
						ToolCalls: []*tracespec.ModelToolCall{},
					},
				},
				Arguments: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertPrompt2Ob(tt.originMessages, tt.variables)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertModel2Ob(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		originMessages []*commonentity.Message
		tools          []*commonentity.Tool
		expectedKeys   []string
	}{
		{
			name:           "nil inputs",
			originMessages: nil,
			tools:          nil,
			expectedKeys:   []string{tracespec.Input},
		},
		{
			name: "single message",
			originMessages: []*commonentity.Message{
				{
					Role: commonentity.RoleUser,
					Content: &commonentity.Content{
						Text: gptr.Of("Hello"),
					},
				},
			},
			tools:        nil,
			expectedKeys: []string{tracespec.Input},
		},
		{
			name: "message with tool",
			originMessages: []*commonentity.Message{
				{
					Role: commonentity.RoleUser,
					Content: &commonentity.Content{
						Text: gptr.Of("Call a function"),
					},
				},
			},
			tools: []*commonentity.Tool{
				{
					Function: &commonentity.Function{
						Name:        "get_weather",
						Parameters:  `{"type": "object"}`,
						Description: "Get weather information",
					},
				},
			},
			expectedKeys: []string{tracespec.Input},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertModel2Ob(tt.originMessages, tt.tools)

			// Check that all expected keys are present
			for _, key := range tt.expectedKeys {
				assert.Contains(t, result, key)
			}

			// Check that the input value is a string
			if inputValue, ok := result[tracespec.Input]; ok {
				assert.IsType(t, "", inputValue)
			}
		})
	}
}

func TestConvertTool2Ob(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		originTool *commonentity.Tool
		expected   *tracespec.ModelTool
	}{
		{
			name:       "nil tool",
			originTool: nil,
			expected:   nil,
		},
		{
			name: "complete tool",
			originTool: &commonentity.Tool{
				Function: &commonentity.Function{
					Name:        "get_weather",
					Parameters:  `{"type": "object", "properties": {"location": {"type": "string"}}}`,
					Description: "Get current weather for a location",
				},
			},
			expected: &tracespec.ModelTool{
				Type: "function",
				Function: &tracespec.ModelToolFunction{
					Name:        "get_weather",
					Parameters:  []byte(`{"type": "object", "properties": {"location": {"type": "string"}}}`),
					Description: "Get current weather for a location",
				},
			},
		},
		{
			name: "minimal tool",
			originTool: &commonentity.Tool{
				Function: &commonentity.Function{
					Name: "simple_func",
				},
			},
			expected: &tracespec.ModelTool{
				Type: "function",
				Function: &tracespec.ModelToolFunction{
					Name:       "simple_func",
					Parameters: []byte(""),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertTool2Ob(tt.originTool)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertMsg2Ob(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		msg      *commonentity.Message
		expected *tracespec.ModelMessage
	}{
		{
			name:     "nil message",
			msg:      nil,
			expected: nil,
		},
		{
			name: "simple text message",
			msg: &commonentity.Message{
				Role: commonentity.RoleUser,
				Content: &commonentity.Content{
					Text: gptr.Of("Hello World"),
				},
			},
			expected: &tracespec.ModelMessage{
				Role:      "user",
				Content:   "Hello World",
				Parts:     []*tracespec.ModelMessagePart{},
				Name:      "",
				ToolCalls: []*tracespec.ModelToolCall{},
			},
		},
		{
			name: "message with multipart content",
			msg: &commonentity.Message{
				Role: commonentity.RoleUser,
				Content: &commonentity.Content{
					Text: gptr.Of("Check this image"),
					MultiPart: []*commonentity.Content{
						{
							ContentType: gptr.Of(commonentity.ContentTypeText),
							Text:        gptr.Of("Part 1"),
						},
						{
							ContentType: gptr.Of(commonentity.ContentTypeImage),
							Image: &commonentity.Image{
								Name: gptr.Of("test.jpg"),
								URL:  gptr.Of("https://example.com/test.jpg"),
							},
						},
					},
				},
			},
			expected: &tracespec.ModelMessage{
				Role:    "user",
				Content: "Check this image",
				Parts: []*tracespec.ModelMessagePart{
					{
						Type: tracespec.ModelMessagePartType("text"),
						Text: "Part 1",
					},
					{
						Type: tracespec.ModelMessagePartType("image_url"),
						Text: "",
						ImageURL: &tracespec.ModelImageURL{
							Name:   "test.jpg",
							URL:    "https://example.com/test.jpg",
							Detail: "",
						},
					},
				},
				Name:      "",
				ToolCalls: []*tracespec.ModelToolCall{},
			},
		},
		{
			name: "assistant role message",
			msg: &commonentity.Message{
				Role: commonentity.RoleAssistant,
				Content: &commonentity.Content{
					Text: gptr.Of("I can help you"),
				},
			},
			expected: &tracespec.ModelMessage{
				Role:      "assistant",
				Content:   "I can help you",
				Parts:     []*tracespec.ModelMessagePart{},
				Name:      "",
				ToolCalls: []*tracespec.ModelToolCall{},
			},
		},
		{
			name: "system role message",
			msg: &commonentity.Message{
				Role: commonentity.RoleSystem,
				Content: &commonentity.Content{
					Text: gptr.Of("You are a helpful assistant"),
				},
			},
			expected: &tracespec.ModelMessage{
				Role:      "system",
				Content:   "You are a helpful assistant",
				Parts:     []*tracespec.ModelMessagePart{},
				Name:      "",
				ToolCalls: []*tracespec.ModelToolCall{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertMsg2Ob(tt.msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertContent2Ob(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  *commonentity.Content
		expected *tracespec.ModelMessagePart
	}{
		{
			name: "text content",
			content: &commonentity.Content{
				ContentType: gptr.Of(commonentity.ContentTypeText),
				Text:        gptr.Of("Hello World"),
			},
			expected: &tracespec.ModelMessagePart{
				Type: tracespec.ModelMessagePartType("text"),
				Text: "Hello World",
			},
		},
		{
			name: "image content",
			content: &commonentity.Content{
				ContentType: gptr.Of(commonentity.ContentTypeImage),
				Image: &commonentity.Image{
					Name: gptr.Of("test.jpg"),
					URL:  gptr.Of("https://example.com/test.jpg"),
				},
			},
			expected: &tracespec.ModelMessagePart{
				Type: tracespec.ModelMessagePartType("image_url"),
				Text: "",
				ImageURL: &tracespec.ModelImageURL{
					Name:   "test.jpg",
					URL:    "https://example.com/test.jpg",
					Detail: "",
				},
			},
		},
		{
			name: "multipart variable content",
			content: &commonentity.Content{
				ContentType: gptr.Of(commonentity.ContentTypeMultipartVariable),
				Text:        gptr.Of("Variable content"),
			},
			expected: &tracespec.ModelMessagePart{
				Type: tracespec.ModelMessagePartType("multi_part_variable"),
				Text: "Variable content",
			},
		},
		{
			name: "unknown content type defaults to text",
			content: &commonentity.Content{
				ContentType: gptr.Of(commonentity.ContentType("unknown")),
				Text:        gptr.Of("Unknown type"),
			},
			expected: &tracespec.ModelMessagePart{
				Type: tracespec.ModelMessagePartType("text"),
				Text: "Unknown type",
			},
		},
		{
			name: "nil content type defaults to text",
			content: &commonentity.Content{
				Text: gptr.Of("No type specified"),
			},
			expected: &tracespec.ModelMessagePart{
				Type: tracespec.ModelMessagePartType("text"),
				Text: "No type specified",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertContent2Ob(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertPromptMessageType2String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		messageType commonentity.Role
		expected    string
	}{
		{
			name:        "system role",
			messageType: commonentity.RoleSystem,
			expected:    tracespec.VRoleSystem,
		},
		{
			name:        "user role",
			messageType: commonentity.RoleUser,
			expected:    tracespec.VRoleUser,
		},
		{
			name:        "assistant role",
			messageType: commonentity.RoleAssistant,
			expected:    tracespec.VRoleAssistant,
		},
		{
			name:        "tool role",
			messageType: commonentity.RoleTool,
			expected:    tracespec.VRoleTool,
		},
		{
			name:        "undefined role defaults to system",
			messageType: commonentity.RoleUndefined,
			expected:    tracespec.VRoleSystem,
		},
		{
			name:        "unknown role defaults to system",
			messageType: commonentity.Role(999),
			expected:    tracespec.VRoleSystem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertPromptMessageType2String(tt.messageType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEvaluatorToolCall2Ob(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		evaluatorToolCall *commonentity.Tool
		expected          *tracespec.ModelToolCall
	}{
		{
			name: "complete tool call",
			evaluatorToolCall: &commonentity.Tool{
				Function: &commonentity.Function{
					Name:       "calculate",
					Parameters: `{"expression": "2+2"}`,
				},
			},
			expected: &tracespec.ModelToolCall{
				Type: "function",
				Function: &tracespec.ModelToolCallFunction{
					Name:      "calculate",
					Arguments: `{"expression": "2+2"}`,
				},
			},
		},
		{
			name: "minimal tool call",
			evaluatorToolCall: &commonentity.Tool{
				Function: &commonentity.Function{
					Name: "simple_func",
				},
			},
			expected: &tracespec.ModelToolCall{
				Type: "function",
				Function: &tracespec.ModelToolCallFunction{
					Name:      "simple_func",
					Arguments: "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorToolCall2Ob(tt.evaluatorToolCall)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvert2TraceString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: `"hello"`,
		},
		{
			name:     "simple number",
			input:    42,
			expected: "42",
		},
		{
			name:     "nil input",
			input:    nil,
			expected: "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := Convert2TraceString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Test object separately since JSON field order is not guaranteed
	t.Run("simple object", func(t *testing.T) {
		t.Parallel()
		input := map[string]interface{}{
			"name": "test",
			"age":  25,
		}
		result := Convert2TraceString(input)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, `"name":"test"`)
		assert.Contains(t, result, `"age":25`)
	})
}

func TestContentToSpanParts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		parts    []*commonentity.Content
		expected []*tracespec.ModelMessagePart
	}{
		{
			name:     "nil parts",
			parts:    nil,
			expected: nil,
		},
		{
			name:     "empty parts",
			parts:    []*commonentity.Content{},
			expected: []*tracespec.ModelMessagePart{},
		},
		{
			name: "text part",
			parts: []*commonentity.Content{
				{
					ContentType: gptr.Of(commonentity.ContentTypeText),
					Text:        gptr.Of("Hello World"),
				},
			},
			expected: []*tracespec.ModelMessagePart{
				{
					Type: tracespec.ModelMessagePartTypeText,
					Text: "Hello World",
				},
			},
		},
		{
			name: "image part",
			parts: []*commonentity.Content{
				{
					ContentType: gptr.Of(commonentity.ContentTypeImage),
					Image: &commonentity.Image{
						Name: gptr.Of("test.jpg"),
						URL:  gptr.Of("https://example.com/test.jpg"),
					},
				},
			},
			expected: []*tracespec.ModelMessagePart{
				{
					Type: tracespec.ModelMessagePartTypeImage,
					ImageURL: &tracespec.ModelImageURL{
						URL:  "https://example.com/test.jpg",
						Name: "test.jpg",
					},
				},
			},
		},
		{
			name: "mixed parts",
			parts: []*commonentity.Content{
				{
					ContentType: gptr.Of(commonentity.ContentTypeText),
					Text:        gptr.Of("Check this image:"),
				},
				{
					ContentType: gptr.Of(commonentity.ContentTypeImage),
					Image: &commonentity.Image{
						Name: gptr.Of("example.png"),
						URL:  gptr.Of("https://example.com/example.png"),
					},
				},
			},
			expected: []*tracespec.ModelMessagePart{
				{
					Type: tracespec.ModelMessagePartTypeText,
					Text: "Check this image:",
				},
				{
					Type: tracespec.ModelMessagePartTypeImage,
					ImageURL: &tracespec.ModelImageURL{
						URL:  "https://example.com/example.png",
						Name: "example.png",
					},
				},
			},
		},
		{
			name: "nil part in slice",
			parts: []*commonentity.Content{
				{
					ContentType: gptr.Of(commonentity.ContentTypeText),
					Text:        gptr.Of("Valid part"),
				},
				nil,
				{
					ContentType: gptr.Of(commonentity.ContentTypeText),
					Text:        gptr.Of("Another valid part"),
				},
			},
			expected: []*tracespec.ModelMessagePart{
				{
					Type: tracespec.ModelMessagePartTypeText,
					Text: "Valid part",
				},
				{
					Type: tracespec.ModelMessagePartTypeText,
					Text: "Another valid part",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ContentToSpanParts(tt.parts)
			assert.Equal(t, tt.expected, result)
		})
	}
}
