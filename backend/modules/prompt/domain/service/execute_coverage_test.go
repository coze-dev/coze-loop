// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

func TestShouldContinue(t *testing.T) {
	makeReplyWithToolCalls := func(toolCalls []*entity.ToolCall) *entity.Reply {
		return &entity.Reply{
			Item: &entity.ReplyItem{
				Message: &entity.Message{
					ToolCalls: toolCalls,
				},
			},
		}
	}

	t.Run("singleStep=true returns false", func(t *testing.T) {
		reply := makeReplyWithToolCalls([]*entity.ToolCall{{ID: "1"}})
		assert.False(t, shouldContinue(true, time.Now(), 1, reply))
	})

	t.Run("currentStep >= maxIterations returns false", func(t *testing.T) {
		reply := makeReplyWithToolCalls([]*entity.ToolCall{{ID: "1"}})
		assert.False(t, shouldContinue(false, time.Now(), maxIterations, reply))
	})

	t.Run("exceeded maxDuration returns false", func(t *testing.T) {
		reply := makeReplyWithToolCalls([]*entity.ToolCall{{ID: "1"}})
		pastStart := time.Now().Add(-maxDuration - 1*time.Minute)
		assert.False(t, shouldContinue(false, pastStart, 1, reply))
	})

	t.Run("nil reply returns false", func(t *testing.T) {
		assert.False(t, shouldContinue(false, time.Now(), 1, nil))
	})

	t.Run("nil reply.Item returns false", func(t *testing.T) {
		assert.False(t, shouldContinue(false, time.Now(), 1, &entity.Reply{}))
	})

	t.Run("nil reply.Item.Message returns false", func(t *testing.T) {
		assert.False(t, shouldContinue(false, time.Now(), 1, &entity.Reply{
			Item: &entity.ReplyItem{},
		}))
	})

	t.Run("empty ToolCalls returns false", func(t *testing.T) {
		reply := makeReplyWithToolCalls(nil)
		assert.False(t, shouldContinue(false, time.Now(), 1, reply))
	})

	t.Run("non-empty ToolCalls returns true", func(t *testing.T) {
		reply := makeReplyWithToolCalls([]*entity.ToolCall{{ID: "1"}})
		assert.True(t, shouldContinue(false, time.Now(), 1, reply))
	})
}

func TestEncodeDecodeDebugIDAndStep(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		debugID := int64(123456789)
		debugStep := int32(7)

		encoded, err := encodeDebugIDAndStep(debugID, debugStep)
		assert.NoError(t, err)
		assert.NotEmpty(t, encoded)

		decodedID, decodedStep, err := decodeDebugIDAndStep(encoded)
		assert.NoError(t, err)
		assert.Equal(t, debugID, decodedID)
		assert.Equal(t, debugStep, decodedStep)
	})

	t.Run("decode invalid string returns error", func(t *testing.T) {
		_, _, err := decodeDebugIDAndStep("not-valid-base32")
		assert.Error(t, err)
	})
}
