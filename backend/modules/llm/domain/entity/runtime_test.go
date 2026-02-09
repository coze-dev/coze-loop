// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessage_TokenGetters(t *testing.T) {
	t.Run("nil_message", func(t *testing.T) {
		var m *Message
		assert.Equal(t, 0, m.GetInputToken())
		assert.Equal(t, 0, m.GetOutputToken())
	})

	t.Run("nil_response_meta", func(t *testing.T) {
		m := &Message{}
		assert.Equal(t, 0, m.GetInputToken())
		assert.Equal(t, 0, m.GetOutputToken())
	})

	t.Run("nil_usage", func(t *testing.T) {
		m := &Message{ResponseMeta: &ResponseMeta{}}
		assert.Equal(t, 0, m.GetInputToken())
		assert.Equal(t, 0, m.GetOutputToken())
	})

	t.Run("success", func(t *testing.T) {
		m := &Message{
			ResponseMeta: &ResponseMeta{
				Usage: &TokenUsage{
					PromptTokens:     10,
					CompletionTokens: 20,
				},
			},
		}
		assert.Equal(t, 10, m.GetInputToken())
		assert.Equal(t, 20, m.GetOutputToken())
	})
}

func TestMessage_MultiModal(t *testing.T) {
	t.Run("has_multi_modal", func(t *testing.T) {
		assert.False(t, (*Message)(nil).HasMultiModalContent())
		assert.False(t, (&Message{}).HasMultiModalContent())

		m := &Message{
			MultiModalContent: []*ChatMessagePart{
				{Type: ChatMessagePartTypeText, Text: "text"},
			},
		}
		assert.False(t, m.HasMultiModalContent())

		m.MultiModalContent = append(m.MultiModalContent, &ChatMessagePart{
			Type: ChatMessagePartTypeImageURL,
		})
		assert.True(t, m.HasMultiModalContent())
	})

	t.Run("get_image_count_and_max_size", func(t *testing.T) {
		m := &Message{
			MultiModalContent: []*ChatMessagePart{
				{
					Type: ChatMessagePartTypeImageURL,
					ImageURL: &ChatMessageImageURL{
						URL: "http://example.com/a.jpg",
					},
				},
				{
					Type: ChatMessagePartTypeImageURL,
					ImageURL: &ChatMessageImageURL{
						URL:      "base64data", // simplified base64
						MIMEType: "image/png",
					},
				},
			},
		}
		hasUrl, hasBinary, cnt, maxSize := m.GetImageCountAndMaxSize()
		assert.True(t, hasUrl)
		assert.True(t, hasBinary)
		assert.Equal(t, int64(2), cnt)
		assert.True(t, maxSize > 0)
	})
}

func TestChatMessagePart_Checks(t *testing.T) {
	t.Run("is_multi_modal", func(t *testing.T) {
		assert.False(t, (*ChatMessagePart)(nil).IsMultiModal())
		assert.False(t, (&ChatMessagePart{Type: ChatMessagePartTypeText}).IsMultiModal())
		assert.True(t, (&ChatMessagePart{Type: ChatMessagePartTypeImageURL}).IsMultiModal())
	})

	t.Run("is_url_binary", func(t *testing.T) {
		assert.False(t, (*ChatMessagePart)(nil).IsURL())
		assert.False(t, (*ChatMessagePart)(nil).IsBinary())

		p := &ChatMessagePart{
			Type: ChatMessagePartTypeImageURL,
			ImageURL: &ChatMessageImageURL{
				URL: "url",
			},
		}
		assert.True(t, p.IsURL())
		assert.False(t, p.IsBinary())

		p.ImageURL.MIMEType = "image/png"
		assert.False(t, p.IsURL())
		assert.True(t, p.IsBinary())
	})
}

func TestParamValue_GetValue(t *testing.T) {
	tests := []struct {
		name      string
		paramType ParamType
		value     string
		want      interface{}
		wantErr   bool
	}{
		{"bool_true", ParamTypeBoolean, "true", true, false},
		{"bool_invalid", ParamTypeBoolean, "not_bool", nil, true},
		{"float", ParamTypeFloat, "1.23", 1.23, false},
		{"int", ParamTypeInt, "123", int64(123), false},
		{"string", ParamTypeString, "hello", "hello", false},
		{"unsupported", ParamType("unknown"), "val", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ParamValue{
				ParamType: tt.paramType,
				Value:     tt.value,
			}
			got, err := p.GetValue()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
