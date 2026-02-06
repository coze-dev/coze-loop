// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	druntime "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/runtime"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
)

func TestMessagesDTO2DO(t *testing.T) {
	dtos := []*druntime.Message{
		{
			Role:    druntime.RoleUser,
			Content: gptr.Of("hello"),
		},
	}
	got := MessagesDTO2DO(dtos)
	assert.Len(t, got, 1)
	assert.Equal(t, entity.RoleUser, got[0].Role)
	assert.Equal(t, "hello", got[0].Content)
}

func TestMessageDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, MessageDTO2DO(nil))
	})
	t.Run("full input", func(t *testing.T) {
		dto := &druntime.Message{
			Role:             druntime.RoleAssistant,
			Content:          gptr.Of("content"),
			ReasoningContent: gptr.Of("reasoning"),
			MultimodalContents: []*druntime.ChatMessagePart{
				{
					Type: gptr.Of(druntime.ChatMessagePartTypeText),
					Text: gptr.Of("text"),
				},
			},
			ToolCalls: []*druntime.ToolCall{
				{
					ID: gptr.Of("call1"),
				},
			},
			ToolCallID: gptr.Of("tc1"),
			ResponseMeta: &druntime.ResponseMeta{
				FinishReason: gptr.Of("stop"),
				Usage: &druntime.TokenUsage{
					PromptTokens: gptr.Of(int64(10)),
				},
			},
		}
		got := MessageDTO2DO(dto)
		assert.NotNil(t, got)
		assert.Equal(t, entity.RoleAssistant, got.Role)
		assert.Equal(t, "content", got.Content)
		assert.Equal(t, "reasoning", got.ReasoningContent)
		assert.Len(t, got.MultiModalContent, 1)
		assert.Len(t, got.ToolCalls, 1)
		assert.Equal(t, "tc1", got.ToolCallID)
		assert.Equal(t, "stop", got.ResponseMeta.FinishReason)
		assert.Equal(t, 10, got.ResponseMeta.Usage.PromptTokens)
	})
}

func TestMessageDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, MessageDO2DTO(nil))
	})
	t.Run("full input", func(t *testing.T) {
		do := &entity.Message{
			Role:             entity.RoleUser,
			Content:          "content",
			ReasoningContent: "reasoning",
			MultiModalContent: []*entity.ChatMessagePart{
				{
					Type: entity.ChatMessagePartTypeText,
					Text: "text",
				},
			},
			ToolCalls: []*entity.ToolCall{
				{
					ID: "call1",
				},
			},
			ToolCallID: "tc1",
			ResponseMeta: &entity.ResponseMeta{
				FinishReason: "stop",
				Usage: &entity.TokenUsage{
					PromptTokens: 10,
				},
			},
		}
		got := MessageDO2DTO(do)
		assert.NotNil(t, got)
		assert.Equal(t, druntime.RoleUser, got.Role)
		assert.Equal(t, "content", *got.Content)
		assert.Equal(t, "reasoning", *got.ReasoningContent)
		assert.Len(t, got.MultimodalContents, 1)
		assert.Len(t, got.ToolCalls, 1)
		assert.Equal(t, "tc1", *got.ToolCallID)
		assert.Equal(t, "stop", *got.ResponseMeta.FinishReason)
		assert.Equal(t, int64(10), *got.ResponseMeta.Usage.PromptTokens)
	})
}

func TestToolCallConvert(t *testing.T) {
	dto := &druntime.ToolCall{
		Index: gptr.Of(int64(0)),
		ID:    gptr.Of("id1"),
		Type:  gptr.Of(druntime.ToolTypeFunction),
		FunctionCall: &druntime.FunctionCall{
			Name:      gptr.Of("func1"),
			Arguments: gptr.Of("{}"),
		},
	}
	do := ToolCallDTO2DO(dto)
	assert.Equal(t, "id1", do.ID)
	assert.Equal(t, "func1", do.Function.Name)

	dto2 := ToolCallDO2DTO(do)
	assert.Equal(t, "id1", *dto2.ID)
	assert.Equal(t, "func1", *dto2.FunctionCall.Name)
}

func TestChatMessagePartConvert(t *testing.T) {
	dto := &druntime.ChatMessagePart{
		Type: gptr.Of(druntime.ChatMessagePartTypeImageURL),
		ImageURL: &druntime.ChatMessageImageURL{
			URL:      gptr.Of("http://img.com"),
			Detail:   gptr.Of(druntime.ImageURLDetailHigh),
			MimeType: gptr.Of("image/png"),
		},
	}
	do := ChatMessagePartDTO2DO(dto)
	assert.Equal(t, entity.ChatMessagePartTypeImageURL, do.Type)
	assert.Equal(t, "http://img.com", do.ImageURL.URL)
	assert.Equal(t, entity.ImageURLDetailHigh, do.ImageURL.Detail)

	dto2 := ChatMessagePartDO2DTO(do)
	assert.Equal(t, druntime.ChatMessagePartTypeImageURL, *dto2.Type)
	assert.Equal(t, "http://img.com", *dto2.ImageURL.URL)
	assert.Equal(t, druntime.ImageURLDetailHigh, *dto2.ImageURL.Detail)
}
