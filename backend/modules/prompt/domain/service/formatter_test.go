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

func TestPromptFormatter_FormatPrompt(t *testing.T) {
	type args struct {
		ctx          context.Context
		prompt       *entity.Prompt
		messages     []*entity.Message
		variableVals []*entity.VariableVal
	}
	tests := []struct {
		name                  string
		args                  args
		wantFormattedMessages []*entity.Message
		wantErr               bool
	}{
		{
			name: "success_simple_template",
			args: args{
				ctx: context.Background(),
				prompt: &entity.Prompt{
					ID:        123,
					SpaceID:   456,
					PromptKey: "test_key",
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleSystem,
										Content: ptr.Of("You are a helpful assistant."),
									},
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("Hello {{name}}"),
									},
								},
								VariableDefs: []*entity.VariableDef{
									{
										Key:  "name",
										Desc: "User name",
										Type: entity.VariableTypeString,
									},
								},
							},
						},
					},
				},
				variableVals: []*entity.VariableVal{
					{
						Key:   "name",
						Value: ptr.Of("World"),
					},
				},
			},
			wantFormattedMessages: []*entity.Message{
				{
					Role:    entity.RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
				},
				{
					Role:    entity.RoleUser,
					Content: ptr.Of("Hello World"),
				},
			},
			wantErr: false,
		},
		{
			name: "success_with_additional_messages",
			args: args{
				ctx: context.Background(),
				prompt: &entity.Prompt{
					ID:        123,
					SpaceID:   456,
					PromptKey: "test_key",
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleSystem,
										Content: ptr.Of("You are a helpful assistant."),
									},
								},
							},
						},
					},
				},
				messages: []*entity.Message{
					{
						Role:    entity.RoleUser,
						Content: ptr.Of("What is AI?"),
					},
				},
			},
			wantFormattedMessages: []*entity.Message{
				{
					Role:    entity.RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
				},
				{
					Role:    entity.RoleUser,
					Content: ptr.Of("What is AI?"),
				},
			},
			wantErr: false,
		},
		{
			name: "success_multimodal_content",
			args: args{
				ctx: context.Background(),
				prompt: &entity.Prompt{
					ID:        123,
					SpaceID:   456,
					PromptKey: "test_key",
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role: entity.RoleUser,
										Parts: []*entity.ContentPart{
											{
												Type: entity.ContentTypeText,
												Text: ptr.Of("Describe this image:"),
											},
											{
												Type: entity.ContentTypeImageURL,
												ImageURL: &entity.ImageURL{
													URI: "test-image-uri",
													URL: "https://example.com/test.jpg",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantFormattedMessages: []*entity.Message{
				{
					Role: entity.RoleUser,
					Parts: []*entity.ContentPart{
						{
							Type: entity.ContentTypeText,
							Text: ptr.Of("Describe this image:"),
						},
						{
							Type: entity.ContentTypeImageURL,
							ImageURL: &entity.ImageURL{
								URI: "test-image-uri",
								URL: "https://example.com/test.jpg",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			formatter := NewPromptFormatter()
			gotFormattedMessages, err := formatter.FormatPrompt(tt.args.ctx, tt.args.prompt, tt.args.messages, tt.args.variableVals)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantFormattedMessages, gotFormattedMessages)
			}
		})
	}
}

func TestNewPromptFormatter(t *testing.T) {
	formatter := NewPromptFormatter()
	assert.NotNil(t, formatter)
	// Verify it implements the interface
	_ = formatter
}
