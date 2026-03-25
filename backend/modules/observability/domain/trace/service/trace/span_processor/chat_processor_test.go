// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TestChatProcessor_Transform(t *testing.T) {
	tests := []struct {
		name           string
		spans          loop_span.SpanList
		wantUserMsg    string
		shouldContains bool
	}{
		{
			name: "standard chat completion - single user message",
			spans: loop_span.SpanList{
				{
					SpanType: loop_span.SpanTypeModel,
					Input:    `{"messages":[{"role":"user","content":"Hello"}]}`,
				},
			},
			wantUserMsg: "Hello",
		},
		{
			name: "standard chat completion - multiple messages, last is user",
			spans: loop_span.SpanList{
				{
					SpanType: loop_span.SpanTypeModel,
					Input:    `{"messages":[{"role":"system","content":"You are helpful"},{"role":"user","content":"First question"},{"role":"assistant","content":"First answer"},{"role":"user","content":"Second question"}]}`,
				},
			},
			wantUserMsg: "Second question",
		},
		{
			name: "standard chat completion - last is assistant",
			spans: loop_span.SpanList{
				{
					SpanType: loop_span.SpanTypeModel,
					Input:    `{"messages":[{"role":"user","content":"Question"},{"role":"assistant","content":"Answer"}]}`,
				},
			},
			wantUserMsg: "Question",
		},
		{
			name: "responses API - string input",
			spans: loop_span.SpanList{
				{
					SpanType: loop_span.SpanTypeModel,
					Input:    `{"input":"Hello from responses API"}`,
				},
			},
			wantUserMsg: "Hello from responses API",
		},
		{
			name: "responses API - message array input",
			spans: loop_span.SpanList{
				{
					SpanType: loop_span.SpanTypeModel,
					Input:    `{"input":[{"type":"message","role":"user","content":"First"},{"type":"message","role":"assistant","content":"Response"},{"type":"message","role":"user","content":"Second"}]}`,
				},
			},
			wantUserMsg: "Second",
		},
		{
			name: "non-model span - should not be processed",
			spans: loop_span.SpanList{
				{
					SpanType: loop_span.SpanTypeFunction,
					Input:    `{"messages":[{"role":"user","content":"Hello"}]}`,
				},
			},
			shouldContains: true,
			wantUserMsg:    "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ChatProcessor{}
			result, err := p.Transform(context.Background(), tt.spans)
			assert.NoError(t, err)
			assert.Equal(t, len(tt.spans), len(result))
			if len(result) > 0 && tt.wantUserMsg != "" {
				assert.Contains(t, result[0].Input, tt.wantUserMsg)
				if !tt.shouldContains {
					var inputMap map[string]interface{}
					err := json.Unmarshal([]byte(result[0].Input), &inputMap)
					require.NoError(t, err)
					messages, ok := inputMap["messages"].([]interface{})
					require.True(t, ok)
					assert.Len(t, messages, 1)
				}
			}
		})
	}
}

func TestChatProcessor_Transform_EdgeCases(t *testing.T) {
	p := &ChatProcessor{}

	t.Run("empty input - should not change", func(t *testing.T) {
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: ""}}
		result, err := p.Transform(context.Background(), spans)
		assert.NoError(t, err)
		assert.Equal(t, "", result[0].Input)
	})

	t.Run("invalid json - should not change", func(t *testing.T) {
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: "not a json"}}
		result, err := p.Transform(context.Background(), spans)
		assert.NoError(t, err)
		assert.Equal(t, "not a json", result[0].Input)
	})

	t.Run("no user message - should not change", func(t *testing.T) {
		originalInput := `{"messages":[{"role":"assistant","content":"Hello"}]}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: originalInput}}
		result, err := p.Transform(context.Background(), spans)
		assert.NoError(t, err)
		assert.Equal(t, originalInput, result[0].Input)
	})

	t.Run("nil span - should skip", func(t *testing.T) {
		spans := loop_span.SpanList{nil}
		result, err := p.Transform(context.Background(), spans)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(result))
	})
}

func TestChatProcessor_ExtractLastUserMessage(t *testing.T) {
	p := &ChatProcessor{}

	tests := []struct {
		name        string
		input       string
		wantEmpty   bool
		wantContent string
	}{
		{
			name:      "empty input",
			input:     "",
			wantEmpty: true,
		},
		{
			name:      "invalid json",
			input:     "not json",
			wantEmpty: true,
		},
		{
			name:      "no messages field",
			input:     `{"other":"field"}`,
			wantEmpty: true,
		},
		{
			name:      "empty messages array",
			input:     `{"messages":[]}`,
			wantEmpty: true,
		},
		{
			name:        "single user message",
			input:       `{"messages":[{"role":"user","content":"Hello"}]}`,
			wantContent: "Hello",
		},
		{
			name:        "multiple messages with user last",
			input:       `{"messages":[{"role":"system","content":"System"},{"role":"user","content":"User"}]}`,
			wantContent: "User",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.extractLastUserMessage(context.Background(), tt.input)
			if tt.wantEmpty {
				assert.Empty(t, got)
			} else {
				assert.Contains(t, got, tt.wantContent)
				var inputMap map[string]interface{}
				err := json.Unmarshal([]byte(got), &inputMap)
				require.NoError(t, err)
				_, ok := inputMap["messages"]
				assert.True(t, ok)
			}
		})
	}
}

func TestNewChatProcessorFactory(t *testing.T) {
	factory := NewChatProcessorFactory()
	assert.NotNil(t, factory)

	processor, err := factory.CreateProcessor(context.Background(), Settings{})
	assert.NoError(t, err)
	assert.NotNil(t, processor)
	_, ok := processor.(*ChatProcessor)
	assert.True(t, ok)
}
