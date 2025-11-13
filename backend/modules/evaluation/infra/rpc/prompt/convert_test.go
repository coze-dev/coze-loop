// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestConvertMessages2Prompt_WithUserQuery(t *testing.T) {
	tests := []struct {
		name     string
		messages []*entity.Message
		want     []*prompt.Message
	}{
		{
			name: "text_content_message",
			messages: []*entity.Message{
				{
					Role: entity.RoleUser,
					Content: &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test text"),
					},
				},
			},
			want: []*prompt.Message{
				{
					Role:    gptr.Of(prompt.RoleUser),
					Content: gptr.Of("test text"),
				},
			},
		},
		{
			name: "multipart_content_message",
			messages: []*entity.Message{
				{
					Role: entity.RoleUser,
					Content: &entity.Content{
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
				},
			},
			want: []*prompt.Message{
				{
					Role: gptr.Of(prompt.RoleUser),
					Parts: []*prompt.ContentPart{
						{
							Type: gptr.Of(prompt.ContentTypeText),
							Text: gptr.Of("text part"),
						},
						{
							Type: gptr.Of(prompt.ContentTypeImageURL),
							ImageURL: &prompt.ImageURL{
								URL: gptr.Of("http://example.com/image.jpg"),
							},
						},
					},
				},
			},
		},
		{
			name: "mixed_messages_with_user_query",
			messages: []*entity.Message{
				{
					Role: entity.RoleSystem,
					Content: &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("system message"),
					},
				},
				{
					Role: entity.RoleUser,
					Content: &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("user query message"),
					},
				},
				{
					Role: entity.RoleAssistant,
					Content: &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("assistant response"),
					},
				},
			},
			want: []*prompt.Message{
				{
					Role:    gptr.Of(prompt.RoleSystem),
					Content: gptr.Of("system message"),
				},
				{
					Role:    gptr.Of(prompt.RoleUser),
					Content: gptr.Of("user query message"),
				},
				{
					Role:    gptr.Of(prompt.RoleAssistant),
					Content: gptr.Of("assistant response"),
				},
			},
		},
		{
			name: "nil_content_message",
			messages: []*entity.Message{
				{
					Role:    entity.RoleUser,
					Content: nil,
				},
			},
			want: []*prompt.Message{},
		},
		{
			name:     "empty_messages",
			messages: []*entity.Message{},
			want:     nil,
		},
		{
			name: "nil_message_in_slice",
			messages: []*entity.Message{
				nil,
				{
					Role: entity.RoleUser,
					Content: &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("valid message"),
					},
				},
			},
			want: []*prompt.Message{
				{
					Role:    gptr.Of(prompt.RoleUser),
					Content: gptr.Of("valid message"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertMessages2Prompt(tt.messages)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConvertFromContent(t *testing.T) {
	tests := []struct {
		name  string
		parts []*prompt.ContentPart
		want  *entity.Content
	}{
		{
			name: "text_parts_only",
			parts: []*prompt.ContentPart{
				{
					Type: gptr.Of(prompt.ContentTypeText),
					Text: gptr.Of("text part 1"),
				},
				{
					Type: gptr.Of(prompt.ContentTypeText),
					Text: gptr.Of("text part 2"),
				},
			},
			want: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("text part 1"),
					},
					{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("text part 2"),
					},
				},
			},
		},
		{
			name: "mixed_text_and_image_parts",
			parts: []*prompt.ContentPart{
				{
					Type: gptr.Of(prompt.ContentTypeText),
					Text: gptr.Of("describe this image"),
				},
				{
					Type: gptr.Of(prompt.ContentTypeImageURL),
					ImageURL: &prompt.ImageURL{
						URL: gptr.Of("http://example.com/image1.jpg"),
					},
				},
				{
					Type: gptr.Of(prompt.ContentTypeImageURL),
					ImageURL: &prompt.ImageURL{
						URL: gptr.Of("http://example.com/image2.jpg"),
						URI: gptr.Of("local://image2.jpg"),
					},
				},
			},
			want: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("describe this image"),
					},
					{
						ContentType: gptr.Of(entity.ContentTypeImage),
						Image: &entity.Image{
							URL: gptr.Of("http://example.com/image1.jpg"),
						},
					},
					{
						ContentType: gptr.Of(entity.ContentTypeImage),
						Image: &entity.Image{
							URL: gptr.Of("http://example.com/image2.jpg"),
							URI: gptr.Of("local://image2.jpg"),
						},
					},
				},
			},
		},
		{
			name:  "empty_parts",
			parts: []*prompt.ContentPart{},
			want:  nil,
		},
		{
			name:  "nil_parts",
			parts: nil,
			want:  nil,
		},
		{
			name: "parts_with_nil_elements",
			parts: []*prompt.ContentPart{
				nil,
				{
					Type: gptr.Of(prompt.ContentTypeText),
					Text: gptr.Of("valid text"),
				},
				nil,
			},
			want: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("valid text"),
					},
				},
			},
		},
		{
			name: "unknown_content_type",
			parts: []*prompt.ContentPart{
				{
					Type: gptr.Of("unknown_type"),
					Text: gptr.Of("unknown content"),
				},
				{
					Type: gptr.Of(prompt.ContentTypeText),
					Text: gptr.Of("valid text"),
				},
			},
			want: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("valid text"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertFromContent(tt.parts)
			assert.Equal(t, tt.want, got)
		})
	}
}