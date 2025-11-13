// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestExecutePromptParam_RuntimeParam(t *testing.T) {
	tests := []struct {
		name         string
		runtimeParam *string
		expected     *string
	}{
		{
			name:         "with_runtime_param",
			runtimeParam: stringPtr(`{"model_config":{"model_id":"123","temperature":0.7}}`),
			expected:     stringPtr(`{"model_config":{"model_id":"123","temperature":0.7}}`),
		},
		{
			name:         "without_runtime_param_nil",
			runtimeParam: nil,
			expected:     nil,
		},
		{
			name:         "empty_runtime_param_string",
			runtimeParam: stringPtr(""),
			expected:     stringPtr(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := &ExecutePromptParam{
				PromptID:      12345,
				PromptVersion: "v1.0",
				Variables:     []*entity.VariableVal{},
				History:       []*entity.Message{},
				RuntimeParam:  tt.runtimeParam,
			}

			// Test that RuntimeParam field is correctly set
			assert.Equal(t, tt.expected, param.RuntimeParam)

			// Test field integration in complete ExecutePromptParam structure
			assert.Equal(t, int64(12345), param.PromptID)
			assert.Equal(t, "v1.0", param.PromptVersion)
			assert.NotNil(t, param.Variables)
			assert.NotNil(t, param.History)
		})
	}
}

func TestExecutePromptParam_Structure_Integrity(t *testing.T) {
	tests := []struct {
		name         string
		promptID     int64
		version      string
		variables    []*entity.VariableVal
		history      []*entity.Message
		runtimeParam *string
	}{
		{
			name:     "complete_param_with_runtime_param",
			promptID: 67890,
			version:  "v2.1",
			variables: []*entity.VariableVal{
				{Key: stringPtr("var1"), Value: stringPtr("value1")},
			},
			history: []*entity.Message{
				{Role: entity.RoleUser, Content: &entity.Content{Text: stringPtr("test message")}},
			},
			runtimeParam: stringPtr(`{"model_config":{"model_id":"test_model","max_tokens":100}}`),
		},
		{
			name:         "minimal_param_without_runtime_param",
			promptID:     11111,
			version:      "v1.0",
			variables:    []*entity.VariableVal{},
			history:      []*entity.Message{},
			runtimeParam: nil,
		},
		{
			name:         "param_with_empty_runtime_param",
			promptID:     22222,
			version:      "v3.0",
			variables:    nil,
			history:      nil,
			runtimeParam: stringPtr(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := &ExecutePromptParam{
				PromptID:      tt.promptID,
				PromptVersion: tt.version,
				Variables:     tt.variables,
				History:       tt.history,
				RuntimeParam:  tt.runtimeParam,
			}

			// Verify all fields are correctly set
			assert.Equal(t, tt.promptID, param.PromptID)
			assert.Equal(t, tt.version, param.PromptVersion)
			assert.Equal(t, tt.variables, param.Variables)
			assert.Equal(t, tt.history, param.History)
			assert.Equal(t, tt.runtimeParam, param.RuntimeParam)

			// Verify struct can be used in interface contexts
			assert.NotNil(t, param)
			assert.IsType(t, &ExecutePromptParam{}, param)
		})
	}
}

func TestExecutePromptParam_UserQuery(t *testing.T) {
	tests := []struct {
		name      string
		userQuery *entity.Message
		wantNil   bool
	}{
		{
			name: "with_user_query_text_message",
			userQuery: &entity.Message{
				Role: entity.RoleUser,
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("test user query"),
				},
			},
			wantNil: false,
		},
		{
			name: "with_user_query_multipart_message",
			userQuery: &entity.Message{
				Role: entity.RoleUser,
				Content: &entity.Content{
					ContentType: gptr.Of(entity.ContentTypeMultipart),
					MultiPart: []*entity.Content{
						{
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("part 1"),
						},
						{
							ContentType: gptr.Of(entity.ContentTypeImage),
							Image: &entity.Image{
								URL: gptr.Of("http://example.com/image.jpg"),
							},
						},
					},
				},
			},
			wantNil: false,
		},
		{
			name:      "without_user_query_nil",
			userQuery: nil,
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := &ExecutePromptParam{
				PromptID:      12345,
				PromptVersion: "v1.0",
				Variables:     []*entity.VariableVal{},
				History:       []*entity.Message{},
				UserQuery:     tt.userQuery,
			}

			// Test that UserQuery field is correctly set
			if tt.wantNil {
				assert.Nil(t, param.UserQuery)
			} else {
				assert.NotNil(t, param.UserQuery)
				assert.Equal(t, tt.userQuery, param.UserQuery)
				assert.Equal(t, entity.RoleUser, param.UserQuery.Role)
			}
		})
	}
}

func TestExecutePromptResult_MultiContent(t *testing.T) {
	tests := []struct {
		name          string
		content       *string
		toolCalls     []*entity.ToolCall
		tokenUsage    *entity.TokenUsage
		multiContent  *entity.Content
		expectedType  entity.ContentType
		expectedText  string
		expectedMulti bool
	}{
		{
			name: "with_multi_content_text",
			multiContent: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of("multi content text"),
			},
			expectedType:  entity.ContentTypeText,
			expectedText:  "multi content text",
			expectedMulti: true,
		},
		{
			name: "with_multi_content_multipart",
			multiContent: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("text part"),
					},
					{
						ContentType: gptr.Of(entity.ContentTypeImage),
						Image: &entity.Image{
							URL: gptr.Of("http://example.com/image.jpg"),
						},
					},
				},
			},
			expectedType:  entity.ContentTypeMultipart,
			expectedMulti: true,
		},
		{
			name:          "without_multi_content_nil",
			multiContent:  nil,
			expectedMulti: false,
		},
		{
			name:         "with_content_and_multi_content",
			content:      gptr.Of("regular content"),
			multiContent: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of("multi content"),
			},
			expectedType:  entity.ContentTypeText,
			expectedText:  "multi content",
			expectedMulti: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ExecutePromptResult{
				Content:      tt.content,
				ToolCalls:    tt.toolCalls,
				TokenUsage:   tt.tokenUsage,
				MultiContent: tt.multiContent,
			}

			// Test that MultiContent field is correctly set
			if tt.expectedMulti {
				assert.NotNil(t, result.MultiContent)
				assert.Equal(t, tt.multiContent, result.MultiContent)
				assert.Equal(t, tt.expectedType, gptr.Indirect(result.MultiContent.ContentType))
				if tt.expectedText != "" {
					assert.Equal(t, tt.expectedText, gptr.Indirect(result.MultiContent.Text))
				}
			} else {
				assert.Nil(t, result.MultiContent)
			}

			// Test that other fields are preserved
			assert.Equal(t, tt.content, result.Content)
			assert.Equal(t, tt.toolCalls, result.ToolCalls)
			assert.Equal(t, tt.tokenUsage, result.TokenUsage)
		})
	}
}

func TestExecutePromptParam_RuntimeParam_JSON_Scenarios(t *testing.T) {
	tests := []struct {
		name         string
		runtimeParam *string
		description  string
	}{
		{
			name:         "complex_runtime_param_json",
			runtimeParam: stringPtr(`{"model_config":{"model_id":"gpt-4","temperature":0.8,"max_tokens":2048,"top_p":0.9},"other_config":{"timeout":30}}`),
			description:  "Complex JSON with multiple nested objects",
		},
		{
			name:         "simple_runtime_param_json",
			runtimeParam: stringPtr(`{"model_config":{"model_id":"simple_model"}}`),
			description:  "Simple JSON with minimal config",
		},
		{
			name:         "runtime_param_with_special_chars",
			runtimeParam: stringPtr(`{"model_config":{"model_id":"test\"model","description":"A model with \"quotes\" and \n newlines"}}`),
			description:  "JSON with escaped characters",
		},
		{
			name:         "runtime_param_empty_object",
			runtimeParam: stringPtr(`{}`),
			description:  "Empty JSON object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := &ExecutePromptParam{
				PromptID:      99999,
				PromptVersion: "test",
				RuntimeParam:  tt.runtimeParam,
			}

			// Test that RuntimeParam preserves the exact string value
			assert.Equal(t, tt.runtimeParam, param.RuntimeParam)

			// Test that the field can be accessed and is not modified
			if tt.runtimeParam != nil {
				assert.Equal(t, *tt.runtimeParam, *param.RuntimeParam)
				assert.True(t, len(*param.RuntimeParam) > 0 || *tt.runtimeParam == "{}")
			}
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
