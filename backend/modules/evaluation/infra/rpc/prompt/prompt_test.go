// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestPromptRPCAdapter_convMsgToContent(t *testing.T) {
	adapter := &PromptRPCAdapter{}

	tests := []struct {
		name     string
		msg      *prompt.Message
		expected *entity.Content
	}{
		{
			name: "message_with_empty_parts",
			msg: &prompt.Message{
				Content: gptr.Of("text content"),
				Parts:   []*prompt.ContentPart{},
			},
			expected: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of("text content"),
			},
		},
		{
			name: "message_with_nil_parts",
			msg: &prompt.Message{
				Content: gptr.Of("text content"),
				Parts:   nil,
			},
			expected: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of("text content"),
			},
		},
		{
			name: "message_with_text_parts",
			msg: &prompt.Message{
				Content: gptr.Of("ignored content"),
				Parts: []*prompt.ContentPart{
					{
						Type: gptr.Of(prompt.ContentTypeText),
						Text: gptr.Of("text part 1"),
					},
					{
						Type: gptr.Of(prompt.ContentTypeText),
						Text: gptr.Of("text part 2"),
					},
				},
			},
			expected: &entity.Content{
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
			name: "message_with_mixed_parts",
			msg: &prompt.Message{
				Content: gptr.Of("ignored content"),
				Parts: []*prompt.ContentPart{
					{
						Type: gptr.Of(prompt.ContentTypeText),
						Text: gptr.Of("describe this image"),
					},
					{
						Type: gptr.Of(prompt.ContentTypeImageURL),
						ImageURL: &prompt.ImageURL{
							URL: gptr.Of("http://example.com/image.jpg"),
						},
					},
				},
			},
			expected: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("describe this image"),
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
		{
			name: "message_with_image_uri",
			msg: &prompt.Message{
				Content: gptr.Of("ignored content"),
				Parts: []*prompt.ContentPart{
					{
						Type: gptr.Of(prompt.ContentTypeImageURL),
						ImageURL: &prompt.ImageURL{
							URL: gptr.Of("http://example.com/image.jpg"),
							URI: gptr.Of("local://image.jpg"),
						},
					},
				},
			},
			expected: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: gptr.Of(entity.ContentTypeImage),
						Image: &entity.Image{
							URL: gptr.Of("http://example.com/image.jpg"),
							URI: gptr.Of("local://image.jpg"),
						},
					},
				},
			},
		},
		{
			name: "message_with_unknown_part_type",
			msg: &prompt.Message{
				Content: gptr.Of("ignored content"),
				Parts: []*prompt.ContentPart{
					{
						Type: gptr.Of("unknown_type"),
						Text: gptr.Of("unknown content"),
					},
					{
						Type: gptr.Of(prompt.ContentTypeText),
						Text: gptr.Of("valid text"),
					},
				},
			},
			expected: &entity.Content{
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
			name: "message_with_nil_parts_elements",
			msg: &prompt.Message{
				Content: gptr.Of("ignored content"),
				Parts: []*prompt.ContentPart{
					nil,
					{
						Type: gptr.Of(prompt.ContentTypeText),
						Text: gptr.Of("valid text"),
					},
					nil,
				},
			},
			expected: &entity.Content{
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
			result := adapter.convMsgToContent(tt.msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPromptRPCAdapter_ExecutePrompt_WithUserQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name      string
		spaceID   int64
		param     *rpc.ExecutePromptParam
		mockSetup func(mockPromptRPCAdapter *mocks.MockIPromptRPCAdapter)
		want      *rpc.ExecutePromptResult
		wantErr   bool
	}{
		{
			name:    "successful execution with user query",
			spaceID: 123,
			param: &rpc.ExecutePromptParam{
				PromptID:      456,
				PromptVersion: "v1",
				Variables: []*entity.VariableVal{
					{
						Key:   gptr.Of("var1"),
						Value: gptr.Of("test value"),
					},
				},
				History: []*entity.Message{
					{
						Role: entity.RoleSystem,
						Content: &entity.Content{
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("system message"),
						},
					},
				},
				UserQuery: &entity.Message{
					Role: entity.RoleUser,
					Content: &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("user query"),
					},
				},
			},
			mockSetup: func(mockPromptRPCAdapter *mocks.MockIPromptRPCAdapter) {
				mockPromptRPCAdapter.EXPECT().
					ExecutePrompt(gomock.Any(), int64(123), gomock.Any()).
					Return(&rpc.ExecutePromptResult{
						Content: gptr.Of("test response"),
						TokenUsage: &entity.TokenUsage{
							InputTokens:  100,
							OutputTokens: 50,
						},
					}, nil)
			},
			want: &rpc.ExecutePromptResult{
				Content: gptr.Of("test response"),
				TokenUsage: &entity.TokenUsage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
			wantErr: false,
		},
		{
			name:    "successful execution with multipart user query",
			spaceID: 123,
			param: &rpc.ExecutePromptParam{
				PromptID:      456,
				PromptVersion: "v1",
				Variables:     []*entity.VariableVal{},
				History:       []*entity.Message{},
				UserQuery: &entity.Message{
					Role: entity.RoleUser,
					Content: &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeMultipart),
						MultiPart: []*entity.Content{
							{
								ContentType: gptr.Of(entity.ContentTypeText),
								Text:        gptr.Of("describe this"),
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
			mockSetup: func(mockPromptRPCAdapter *mocks.MockIPromptRPCAdapter) {
				multiContentResult := &entity.Content{
					ContentType: gptr.Of(entity.ContentTypeMultipart),
					MultiPart: []*entity.Content{
						{
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("text response"),
						},
					},
				}
				mockPromptRPCAdapter.EXPECT().
					ExecutePrompt(gomock.Any(), int64(123), gomock.Any()).
					Return(&rpc.ExecutePromptResult{
						Content:      gptr.Of("multipart response"),
						MultiContent: multiContentResult,
						TokenUsage: &entity.TokenUsage{
							InputTokens:  150,
							OutputTokens: 75,
						},
					}, nil)
			},
			want: &rpc.ExecutePromptResult{
				Content: gptr.Of("multipart response"),
				MultiContent: &entity.Content{
					ContentType: gptr.Of(entity.ContentTypeMultipart),
					MultiPart: []*entity.Content{
						{
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("text response"),
						},
					},
				},
				TokenUsage: &entity.TokenUsage{
					InputTokens:  150,
					OutputTokens: 75,
				},
			},
			wantErr: false,
		},
		{
			name:    "successful execution with user query and history",
			spaceID: 123,
			param: &rpc.ExecutePromptParam{
				PromptID:      456,
				PromptVersion: "v1",
				Variables:     []*entity.VariableVal{},
				History: []*entity.Message{
					{
						Role: entity.RoleSystem,
						Content: &entity.Content{
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("system context"),
						},
					},
					{
						Role: entity.RoleUser,
						Content: &entity.Content{
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("previous user message"),
						},
					},
					{
						Role: entity.RoleAssistant,
						Content: &entity.Content{
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("previous assistant response"),
						},
					},
				},
				UserQuery: &entity.Message{
					Role: entity.RoleUser,
					Content: &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("current user query"),
					},
				},
			},
			mockSetup: func(mockPromptRPCAdapter *mocks.MockIPromptRPCAdapter) {
				mockPromptRPCAdapter.EXPECT().
					ExecutePrompt(gomock.Any(), int64(123), gomock.Any()).
					Return(&rpc.ExecutePromptResult{
						Content: gptr.Of("response with context"),
						TokenUsage: &entity.TokenUsage{
							InputTokens:  200,
							OutputTokens: 100,
						},
					}, nil)
			},
			want: &rpc.ExecutePromptResult{
				Content: gptr.Of("response with context"),
				TokenUsage: &entity.TokenUsage{
					InputTokens:  200,
					OutputTokens: 100,
				},
			},
			wantErr: false,
		},
		{
			name:    "execution error",
			spaceID: 123,
			param: &rpc.ExecutePromptParam{
				PromptID:      456,
				PromptVersion: "v1",
				Variables:     []*entity.VariableVal{},
				History:       []*entity.Message{},
				UserQuery: &entity.Message{
					Role: entity.RoleUser,
					Content: &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("user query"),
					},
				},
			},
			mockSetup: func(mockPromptRPCAdapter *mocks.MockIPromptRPCAdapter) {
				mockPromptRPCAdapter.EXPECT().
					ExecutePrompt(gomock.Any(), int64(123), gomock.Any()).
					Return(nil, assert.AnError)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPromptRPCAdapter := mocks.NewMockIPromptRPCAdapter(ctrl)
			tt.mockSetup(mockPromptRPCAdapter)

			got, err := mockPromptRPCAdapter.ExecutePrompt(context.Background(), tt.spaceID, tt.param)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}