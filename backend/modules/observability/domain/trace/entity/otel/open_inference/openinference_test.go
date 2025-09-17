// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package open_inference

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertToModelInput(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
		wantErr  bool
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "non-slice input",
			input:    "not a slice",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "empty slice",
			input:    []interface{}{},
			expected: map[string]interface{}{"messages": []interface{}{}},
			wantErr:  false,
		},
		{
			name: "single message with content",
			input: []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "user",
						"content": "Hello, world!",
					},
				},
			},
			expected: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello, world!",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple messages",
			input: []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hi there!",
					},
				},
			},
			expected: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
					map[string]interface{}{
						"role":    "assistant",
						"content": "Hi there!",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "message with contents (multipart)",
			input: []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role": "user",
						"contents": []interface{}{
							map[string]interface{}{
								"message_content": map[string]interface{}{
									"type": "text",
									"text": "Hello",
								},
							},
							map[string]interface{}{
								"message_content": map[string]interface{}{
									"type": "image",
									"image": map[string]interface{}{
										"url": "https://example.com/image.jpg",
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role": "user",
						"parts": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": "Hello",
							},
							map[string]interface{}{
								"type":      "image_url",
								"image_url": map[string]interface{}{"url": "https://example.com/image.jpg"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "message with tool calls",
			input: []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role": "assistant",
						"tool_calls": []interface{}{
							map[string]interface{}{
								"tool_call": map[string]interface{}{
									"function": map[string]interface{}{
										"name":      "get_weather",
										"arguments": `{"location": "New York"}`,
									},
								},
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role": "assistant",
						"tool_calls": []interface{}{
							map[string]interface{}{
								"type": "function",
								"function": map[string]interface{}{
									"name":      "get_weather",
									"arguments": `{"location": "New York"}`,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid message structure",
			input: []interface{}{
				"not a map",
			},
			expected: map[string]interface{}{"messages": []interface{}{}},
			wantErr:  false,
		},
		{
			name: "message without message field",
			input: []interface{}{
				map[string]interface{}{
					"other_field": "value",
				},
			},
			expected: map[string]interface{}{"messages": []interface{}{}},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertToModelInput(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestConvertToModelOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
		wantErr  bool
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "non-slice input",
			input:    "not a slice",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "empty slice",
			input:    []interface{}{},
			expected: map[string]interface{}{"choices": []interface{}{}},
			wantErr:  false,
		},
		{
			name: "single choice",
			input: []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello, how can I help you?",
					},
				},
			},
			expected: map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Hello, how can I help you?",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple choices",
			input: []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Option 1",
					},
				},
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Option 2",
					},
				},
			},
			expected: map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Option 1",
						},
					},
					map[string]interface{}{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Option 2",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid surface structure",
			input: []interface{}{
				"not a map",
			},
			expected: map[string]interface{}{"choices": []interface{}{}},
			wantErr:  false,
		},
		{
			name: "surface without message field",
			input: []interface{}{
				map[string]interface{}{
					"other_field": "value",
				},
			},
			expected: map[string]interface{}{"choices": []interface{}{}},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertToModelOutput(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestAddTools2ModelInput(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		tools    interface{}
		expected interface{}
		wantErr  bool
	}{
		{
			name:     "non-map input",
			input:    "not a map",
			tools:    nil,
			expected: nil,
			wantErr:  true,
		},
		{
			name: "nil tools",
			input: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
			},
			tools: nil,
			expected: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "non-slice tools",
			input: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
			},
			tools: "not a slice",
			expected: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty tools slice",
			input: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
			},
			tools: []interface{}{},
			expected: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid tools",
			input: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "What's the weather?",
					},
				},
			},
			tools: []interface{}{
				map[string]interface{}{
					"tool": map[string]interface{}{
						"json_schema": `{"name": "get_weather", "description": "Get weather info", "parameters": {"type": "object", "properties": {"location": {"type": "string"}}}}`,
					},
				},
			},
			expected: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "What's the weather?",
					},
				},
				"tools": []interface{}{
					map[string]interface{}{
						"type": "function",
						"function": map[string]interface{}{
							"name":        "get_weather",
							"description": "Get weather info",
							// parameters is json.RawMessage, not parsed map
							"parameters": json.RawMessage(`{"type": "object", "properties": {"location": {"type": "string"}}}`),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid tool structure",
			input: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
			},
			tools: []interface{}{
				"not a map",
			},
			expected: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "tool without tool field",
			input: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
			},
			tools: []interface{}{
				map[string]interface{}{
					"other_field": "value",
				},
			},
			expected: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "tool with invalid json schema",
			input: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
			},
			tools: []interface{}{
				map[string]interface{}{
					"tool": map[string]interface{}{
						"json_schema": "invalid json",
					},
				},
			},
			expected: map[string]interface{}{
				"messages": []interface{}{
					map[string]interface{}{
						"role":    "user",
						"content": "Hello",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AddTools2ModelInput(tt.input, tt.tools)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestConvertModelMsg(t *testing.T) {
	tests := []struct {
		name     string
		msg      map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "basic message with role and content",
			msg: map[string]interface{}{
				"role":    "user",
				"content": "Hello, world!",
			},
			expected: map[string]interface{}{
				"role":    "user",
				"content": "Hello, world!",
			},
		},
		{
			name: "message with role only",
			msg: map[string]interface{}{
				"role": "assistant",
			},
			expected: map[string]interface{}{
				"role": "assistant",
			},
		},
		{
			name: "message with contents (multipart)",
			msg: map[string]interface{}{
				"role": "user",
				"contents": []interface{}{
					map[string]interface{}{
						"message_content": map[string]interface{}{
							"type": "text",
							"text": "Hello",
						},
					},
					map[string]interface{}{
						"message_content": map[string]interface{}{
							"type": "image",
							"image": map[string]interface{}{
								"url": "https://example.com/image.jpg",
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"role": "user",
				"parts": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": "Hello",
					},
					map[string]interface{}{
						"type":      "image_url",
						"image_url": map[string]interface{}{"url": "https://example.com/image.jpg"},
					},
				},
			},
		},
		{
			name: "message with empty contents",
			msg: map[string]interface{}{
				"role":     "user",
				"contents": []interface{}{},
			},
			expected: map[string]interface{}{
				"role": "user",
			},
		},
		{
			name: "message with tool calls",
			msg: map[string]interface{}{
				"role": "assistant",
				"tool_calls": []interface{}{
					map[string]interface{}{
						"tool_call": map[string]interface{}{
							"function": map[string]interface{}{
								"name":      "get_weather",
								"arguments": `{"location": "New York"}`,
							},
						},
					},
				},
			},
			expected: map[string]interface{}{
				"role": "assistant",
				"tool_calls": []interface{}{
					map[string]interface{}{
						"type": "function",
						"function": map[string]interface{}{
							"name":      "get_weather",
							"arguments": `{"location": "New York"}`,
						},
					},
				},
			},
		},
		{
			name: "message with empty tool calls",
			msg: map[string]interface{}{
				"role":       "assistant",
				"tool_calls": []interface{}{},
			},
			expected: map[string]interface{}{
				"role": "assistant",
			},
		},
		{
			name: "message with invalid content type",
			msg: map[string]interface{}{
				"role":    "user",
				"content": 123, // not a string
			},
			expected: map[string]interface{}{
				"role": "user",
			},
		},
		{
			name: "message with invalid contents type",
			msg: map[string]interface{}{
				"role":     "user",
				"contents": "not a slice",
			},
			expected: map[string]interface{}{
				"role": "user",
			},
		},
		{
			name: "message with invalid tool calls type",
			msg: map[string]interface{}{
				"role":       "assistant",
				"tool_calls": "not a slice",
			},
			expected: map[string]interface{}{
				"role": "assistant",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertModelMsg(tt.msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLiteralConstants(t *testing.T) {
	assert.Equal(t, Literal("text"), TextLiteral)
	assert.Equal(t, Literal("image"), ImageLiteral)
}

func TestModelMessagePartTypeConstants(t *testing.T) {
	assert.Equal(t, ModelMessagePartType("text"), ModelMessagePartTypeText)
	assert.Equal(t, ModelMessagePartType("image_url"), ModelMessagePartTypeImage)
}
