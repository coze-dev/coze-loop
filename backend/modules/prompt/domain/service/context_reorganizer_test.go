// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestContextReorganizer_ReorganizeContexts(t *testing.T) {
	t.Run("no reply returns original contexts", func(t *testing.T) {
		reorganizer := NewContextReorganizer()
		contexts := []*entity.Message{
			{
				Role:    entity.RoleUser,
				Content: ptr.Of("hello"),
			},
		}

		got, err := reorganizer.ReorganizeContexts(context.Background(), ReorganizeContextParam{
			Messages: contexts,
			Reply:    nil,
		})
		assert.NoError(t, err)
		assert.Equal(t, contexts, got)
	})

	t.Run("reply without tool calls appends reply message", func(t *testing.T) {
		reorganizer := NewContextReorganizer()
		contexts := []*entity.Message{
			{
				Role:    entity.RoleUser,
				Content: ptr.Of("hi"),
			},
		}
		replyMessage := &entity.Message{
			Role:    entity.RoleAssistant,
			Content: ptr.Of("ok"),
		}

		got, err := reorganizer.ReorganizeContexts(context.Background(), ReorganizeContextParam{
			Messages: contexts,
			Reply: &entity.Reply{
				Item: &entity.ReplyItem{Message: replyMessage},
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, []*entity.Message{contexts[0], replyMessage}, got)
	})

	t.Run("reply with tool calls appends mock tool responses", func(t *testing.T) {
		reorganizer := NewContextReorganizer()
		contexts := []*entity.Message{
			{
				Role:    entity.RoleUser,
				Content: ptr.Of("hi"),
			},
		}
		replyMessage := &entity.Message{
			Role:    entity.RoleAssistant,
			Content: ptr.Of("call tool"),
			ToolCalls: []*entity.ToolCall{
				{
					ID: "call_1",
					FunctionCall: &entity.FunctionCall{
						Name: "tool_a",
					},
				},
				{
					ID: "call_2",
				},
			},
		}

		got, err := reorganizer.ReorganizeContexts(context.Background(), ReorganizeContextParam{
			Messages: contexts,
			MockTools: []*entity.MockTool{
				{
					Name:         "tool_a",
					MockResponse: "mocked",
				},
			},
			Reply: &entity.Reply{
				Item: &entity.ReplyItem{Message: replyMessage},
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, []*entity.Message{
			contexts[0],
			replyMessage,
			{
				Role:       entity.RoleTool,
				ToolCallID: ptr.Of("call_1"),
				Content:    ptr.Of("mocked"),
			},
		}, got)
	})
}
