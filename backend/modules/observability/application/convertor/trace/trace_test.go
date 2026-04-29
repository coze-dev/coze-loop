// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TestChatMessagesDO2DTO(t *testing.T) {
	tests := []struct {
		name     string
		messages []*entity.ChatMessage
		wantNil  bool
		wantLen  int
	}{
		{
			name:     "nil input",
			messages: nil,
			wantNil:  true,
		},
		{
			name:     "empty input",
			messages: []*entity.ChatMessage{},
			wantLen:  0,
		},
		{
			name: "skip nil message",
			messages: []*entity.ChatMessage{
				nil,
				{
					Role: entity.ChatRoleUser,
					Span: &loop_span.Span{
						TraceID:  "trace-1",
						SpanID:   "span-1",
						SpanName: "test-span",
					},
				},
				nil,
			},
			wantLen: 1,
		},
		{
			name: "user role message",
			messages: []*entity.ChatMessage{
				{
					Role: entity.ChatRoleUser,
					Span: &loop_span.Span{
						TraceID:  "trace-1",
						SpanID:   "span-1",
						SpanName: "user-span",
						SpanType: loop_span.SpanTypeModel,
						Input:    "hello",
					},
				},
			},
			wantLen: 1,
		},
		{
			name: "assistant role message",
			messages: []*entity.ChatMessage{
				{
					Role: entity.ChatRoleAssistant,
					Span: &loop_span.Span{
						TraceID:  "trace-2",
						SpanID:   "span-2",
						SpanName: "assistant-span",
						SpanType: loop_span.SpanTypeModel,
						Output:   "world",
					},
				},
			},
			wantLen: 1,
		},
		{
			name: "tool role message",
			messages: []*entity.ChatMessage{
				{
					Role: entity.ChatRoleTool,
					Span: &loop_span.Span{
						TraceID:  "trace-3",
						SpanID:   "span-3",
						SpanName: "tool-span",
						SpanType: loop_span.SpanTypeTool,
						Input:    "tool-input",
						Output:   "tool-output",
					},
				},
			},
			wantLen: 1,
		},
		{
			name: "multiple messages with different roles",
			messages: []*entity.ChatMessage{
				{
					Role: entity.ChatRoleUser,
					Span: &loop_span.Span{
						TraceID:  "trace-1",
						SpanID:   "span-1",
						SpanName: "user-span",
					},
				},
				{
					Role: entity.ChatRoleAssistant,
					Span: &loop_span.Span{
						TraceID:  "trace-2",
						SpanID:   "span-2",
						SpanName: "assistant-span",
					},
				},
				{
					Role: entity.ChatRoleTool,
					Span: &loop_span.Span{
						TraceID:  "trace-3",
						SpanID:   "span-3",
						SpanName: "tool-span",
					},
				},
			},
			wantLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChatMessagesDO2DTO(tt.messages)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			assert.Len(t, got, tt.wantLen)

			nonNilMessages := make([]*entity.ChatMessage, 0)
			for _, m := range tt.messages {
				if m != nil {
					nonNilMessages = append(nonNilMessages, m)
				}
			}
			for i, msg := range got {
				assert.Equal(t, nonNilMessages[i].Role, msg.Role)
				if nonNilMessages[i].Span != nil {
					assert.NotNil(t, msg.Span)
					assert.Equal(t, nonNilMessages[i].Span.TraceID, msg.Span.TraceID)
					assert.Equal(t, nonNilMessages[i].Span.SpanID, msg.Span.SpanID)
					assert.Equal(t, nonNilMessages[i].Span.SpanName, msg.Span.SpanName)
				}
			}
		})
	}
}
