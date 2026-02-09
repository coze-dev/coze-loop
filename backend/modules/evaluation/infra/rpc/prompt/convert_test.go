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

func TestConvertToLoopPrompts(t *testing.T) {
	assert.Nil(t, ConvertToLoopPrompts(nil))
	res := ConvertToLoopPrompts([]*prompt.Prompt{{ID: gptr.Of(int64(1))}})
	assert.Len(t, res, 1)
	assert.Equal(t, int64(1), res[0].ID)
}

func TestConvertToLoopPrompt(t *testing.T) {
	assert.Nil(t, ConvertToLoopPrompt(nil))
	p := &prompt.Prompt{
		ID:        gptr.Of(int64(1)),
		PromptKey: gptr.Of("key"),
		PromptBasic: &prompt.PromptBasic{
			DisplayName:   gptr.Of("name"),
			Description:   gptr.Of("desc"),
			LatestVersion: gptr.Of("v1"),
		},
		PromptCommit: &prompt.PromptCommit{
			Detail: &prompt.PromptDetail{
				PromptTemplate: &prompt.PromptTemplate{
					VariableDefs: []*prompt.VariableDef{
						{Key: gptr.Of("k1"), Type: gptr.Of("t1"), TypeTags: []string{"tag1"}},
					},
				},
			},
			CommitInfo: &prompt.CommitInfo{
				Version:     gptr.Of("v1"),
				BaseVersion: gptr.Of("v0"),
				Description: gptr.Of("commit desc"),
				CommittedAt: gptr.Of(int64(123456789)),
				CommittedBy: gptr.Of("1001"),
			},
		},
	}
	res := ConvertToLoopPrompt(p)
	assert.NotNil(t, res)
	assert.Equal(t, int64(1), res.ID)
	assert.Equal(t, "key", res.PromptKey)
	assert.Equal(t, "name", *res.PromptBasic.DisplayName)
	assert.Len(t, res.PromptCommit.Detail.PromptTemplate.VariableDefs, 1)
	assert.Equal(t, "v1", *res.PromptCommit.CommitInfo.Version)
}

func TestConvertVariables2Prompt(t *testing.T) {
	assert.Nil(t, ConvertVariables2Prompt(nil))
	vars := []*entity.VariableVal{
		{
			Key:   gptr.Of("k1"),
			Value: gptr.Of("v1"),
			PlaceholderMessages: []*entity.Message{
				{Role: entity.RoleUser, Content: &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("msg")}},
			},
		},
	}
	res := ConvertVariables2Prompt(vars)
	assert.Len(t, res, 1)
	assert.Equal(t, "k1", *res[0].Key)
	assert.Len(t, res[0].PlaceholderMessages, 1)
}

func TestConvertPromptToolCalls2Eval(t *testing.T) {
	assert.Nil(t, ConvertPromptToolCalls2Eval(nil))
	calls := []*prompt.ToolCall{
		{
			Index: gptr.Of(int64(0)),
			ID:    gptr.Of("id1"),
			FunctionCall: &prompt.FunctionCall{
				Name:      gptr.Of("func1"),
				Arguments: gptr.Of(`{"a":1}`),
			},
		},
	}
	res := ConvertPromptToolCalls2Eval(calls)
	assert.Len(t, res, 1)
	assert.Equal(t, int64(0), res[0].Index)
	assert.Equal(t, "id1", res[0].ID)
	assert.Equal(t, "func1", res[0].FunctionCall.Name)
}

func TestRole2PromptRole(t *testing.T) {
	assert.Equal(t, prompt.RoleSystem, Role2PromptRole(entity.RoleSystem))
	assert.Equal(t, prompt.RoleUser, Role2PromptRole(entity.RoleUser))
	assert.Equal(t, prompt.RoleAssistant, Role2PromptRole(entity.RoleAssistant))
	assert.Equal(t, prompt.RoleTool, Role2PromptRole(entity.RoleTool))
	assert.Equal(t, prompt.RoleUser, Role2PromptRole(entity.Role(99)))
}

func TestConvertContent(t *testing.T) {
	assert.Nil(t, ConvertContent(nil))

	t.Run("text", func(t *testing.T) {
		c := &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("text")}
		res := ConvertContent(c)
		assert.Len(t, res, 1)
		assert.Equal(t, prompt.ContentTypeText, *res[0].Type)
	})

	t.Run("image", func(t *testing.T) {
		c := &entity.Content{
			ContentType: gptr.Of(entity.ContentTypeImage),
			Image: &entity.Image{
				URL: gptr.Of("url"),
				URI: gptr.Of("uri"),
			},
		}
		res := ConvertContent(c)
		assert.Len(t, res, 1)
		assert.Equal(t, prompt.ContentTypeImageURL, *res[0].Type)
		assert.Equal(t, "url", *res[0].ImageURL.URL)
	})

	t.Run("multipart", func(t *testing.T) {
		c := &entity.Content{
			ContentType: gptr.Of(entity.ContentTypeMultipart),
			MultiPart: []*entity.Content{
				{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("t1")},
				{ContentType: gptr.Of(entity.ContentTypeImage), Image: &entity.Image{URL: gptr.Of("u1")}},
			},
		}
		res := ConvertContent(c)
		assert.Len(t, res, 2)
		assert.Equal(t, prompt.ContentTypeText, *res[0].Type)
		assert.Equal(t, prompt.ContentTypeImageURL, *res[1].Type)
	})

	t.Run("default", func(t *testing.T) {
		c := &entity.Content{ContentType: gptr.Of(entity.ContentType("unknown"))}
		res := ConvertContent(c)
		assert.Len(t, res, 0)
	})
}
