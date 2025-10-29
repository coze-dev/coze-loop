// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func TestPromptServiceImpl_FormatPrompt(t *testing.T) {
	type fields struct {
		idgen            idgen.IIDGenerator
		debugLogRepo     repo.IDebugLogRepo
		debugContextRepo repo.IDebugContextRepo
		manageRepo       repo.IManageRepo
		configProvider   conf.IConfigProvider
		llm              rpc.ILLMProvider
		file             rpc.IFileProvider
	}
	type args struct {
		ctx          context.Context
		prompt       *entity.Prompt
		messages     []*entity.Message
		variableVals []*entity.VariableVal
	}
	tests := []struct {
		name                  string
		fieldsGetter          func(ctrl *gomock.Controller) fields
		args                  args
		wantFormattedMessages []*entity.Message
		wantErr               error
	}{
		{
			name: "success_simple_prompt",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
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
										Desc: "Your name",
										Type: entity.VariableTypeString,
									},
								},
							},
						},
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "1.0.0",
							IsModified:  true,
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
			wantErr: nil,
		},
		{
			name: "success_with_additional_messages",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
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
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "1.0.0",
							IsModified:  true,
						},
					},
				},
				messages: []*entity.Message{
					{
						Role:    entity.RoleUser,
						Content: ptr.Of("Hello!"),
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
					Content: ptr.Of("Hello!"),
				},
			},
			wantErr: nil,
		},
		{
			name: "success_with_multimodal_content",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
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
												Text: ptr.Of("Describe this picture:"),
											},
											{
												Type: entity.ContentTypeImageURL,
												ImageURL: &entity.ImageURL{
													URI: "test-image-uri",
													URL: "https://example.com/image.jpg",
												},
											},
										},
									},
								},
							},
						},
						DraftInfo: &entity.DraftInfo{
							UserID:      "test_user",
							BaseVersion: "1.0.0",
							IsModified:  true,
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
							Text: ptr.Of("Describe this picture:"),
						},
						{
							Type: entity.ContentTypeImageURL,
							ImageURL: &entity.ImageURL{
								URI: "test-image-uri",
								URL: "https://example.com/image.jpg",
							},
						},
					},
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			p := &PromptServiceImpl{
				formatter:        NewPromptFormatter(),
				idgen:            ttFields.idgen,
				debugLogRepo:     ttFields.debugLogRepo,
				debugContextRepo: ttFields.debugContextRepo,
				manageRepo:       ttFields.manageRepo,
				configProvider:   ttFields.configProvider,
				llm:              ttFields.llm,
				file:             ttFields.file,
			}
			gotFormattedMessages, err := p.FormatPrompt(tt.args.ctx, tt.args.prompt, tt.args.messages, tt.args.variableVals)

			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr == nil {
				assert.Equal(t, tt.wantFormattedMessages, gotFormattedMessages)
			}
		})
	}
}

func TestPromptServiceImpl_ExecuteStreaming(t *testing.T) {
	t.Run("nil prompt", func(t *testing.T) {
		t.Parallel()

		p := &PromptServiceImpl{
			formatter: NewPromptFormatter(),
		}
		param := ExecuteStreamingParam{
			ExecuteParam: ExecuteParam{
				Prompt: nil,
			},
			ResultStream: make(chan<- *entity.Reply),
		}
		_, err := p.ExecuteStreaming(context.Background(), param)
		unittest.AssertErrorEqual(t, err, errorx.New("invalid param"))
	})

	t.Run("nil result stream", func(t *testing.T) {
		t.Parallel()

		p := &PromptServiceImpl{
			formatter: NewPromptFormatter(),
		}
		param := ExecuteStreamingParam{
			ExecuteParam: ExecuteParam{
				Prompt: &entity.Prompt{},
			},
			ResultStream: nil,
		}
		_, err := p.ExecuteStreaming(context.Background(), param)
		unittest.AssertErrorEqual(t, err, errorx.New("invalid param"))
	})

	t.Run("single step execution success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
		mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123456789), nil)
		mockLLM := rpcmocks.NewMockILLMProvider(ctrl)
		mockContent := "Hello!"
		mockLLM.EXPECT().StreamingCall(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, param rpc.LLMStreamingCallParam) (*entity.ReplyItem, error) {
			for _, v := range mockContent {
				param.ResultStream <- &entity.ReplyItem{
					Message: &entity.Message{
						Role:    entity.RoleAssistant,
						Content: ptr.Of(string(v)),
					},
				}
			}
			finishReason := "stop"
			tokenUsage := &entity.TokenUsage{
				InputTokens:  10,
				OutputTokens: 5,
			}
			param.ResultStream <- &entity.ReplyItem{
				FinishReason: finishReason,
			}
			param.ResultStream <- &entity.ReplyItem{
				TokenUsage: tokenUsage,
			}
			return &entity.ReplyItem{
				Message: &entity.Message{
					Role:    entity.RoleAssistant,
					Content: ptr.Of(mockContent),
				},
				FinishReason: finishReason,
				TokenUsage:   tokenUsage,
			}, nil
		})
		wantReplyItem := &entity.Reply{
			Item: &entity.ReplyItem{
				Message: &entity.Message{
					Role:    entity.RoleAssistant,
					Content: ptr.Of(mockContent),
				},
				FinishReason: "stop",
				TokenUsage: &entity.TokenUsage{
					InputTokens:  10,
					OutputTokens: 5,
				},
			},
			DebugID:   123456789,
			DebugStep: 1,
		}
		p := &PromptServiceImpl{
			formatter: NewPromptFormatter(),
			idgen:     mockIDGen,
			llm:       mockLLM,
		}

		stream := make(chan *entity.Reply)
		param := ExecuteStreamingParam{
			ExecuteParam: ExecuteParam{
				Prompt: &entity.Prompt{
					ID:        1,
					SpaceID:   123,
					PromptKey: "test_prompt",
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
				Messages: []*entity.Message{
					{
						Role:    entity.RoleUser,
						Content: ptr.Of("Hello"),
					},
				},
				SingleStep: true,
			},
			ResultStream: stream,
		}
		go func() {
			defer close(stream)
			gotReply, err := p.ExecuteStreaming(context.Background(), param)
			assert.Nil(t, err)
			assert.NotEmpty(t, gotReply.DebugTraceKey)
			assert.Equal(t, wantReplyItem.Item, gotReply.Item)
			assert.Equal(t, wantReplyItem.DebugID, gotReply.DebugID)
			assert.Equal(t, wantReplyItem.DebugStep, gotReply.DebugStep)
		}()
		var content string
		for reply := range stream {
			assert.NotEmpty(t, reply.DebugTraceKey)
			assert.Equal(t, wantReplyItem.DebugID, reply.DebugID)
			assert.Equal(t, wantReplyItem.DebugStep, reply.DebugStep)
			if reply.Item != nil {
				if reply.Item.Message != nil {
					content += ptr.From(reply.Item.Message.Content)
				}
				if reply.Item.FinishReason != "" {
					assert.Equal(t, wantReplyItem.Item.FinishReason, reply.Item.FinishReason)
				}
				if reply.Item.TokenUsage != nil {
					assert.Equal(t, wantReplyItem.Item.TokenUsage, reply.Item.TokenUsage)
				}
			}
		}
	})

	t.Run("multi-step execution success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
		mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123456789), nil)
		mockLLM := rpcmocks.NewMockILLMProvider(ctrl)
		mockLLM.EXPECT().StreamingCall(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, param rpc.LLMStreamingCallParam) (*entity.ReplyItem, error) {
			param.ResultStream <- &entity.ReplyItem{
				Message: &entity.Message{
					Role: entity.RoleAssistant,
					ToolCalls: []*entity.ToolCall{
						{
							Index: 0,
							ID:    "call_123456",
							Type:  entity.ToolTypeFunction,
						},
					},
				},
			}
			param.ResultStream <- &entity.ReplyItem{
				Message: &entity.Message{
					Role: entity.RoleAssistant,
					ToolCalls: []*entity.ToolCall{
						{
							Index: 0,
							FunctionCall: &entity.FunctionCall{
								Name: "get_weather",
							},
						},
					},
				},
			}
			param.ResultStream <- &entity.ReplyItem{
				Message: &entity.Message{
					Role: entity.RoleAssistant,
					ToolCalls: []*entity.ToolCall{
						{
							Index: 0,
							FunctionCall: &entity.FunctionCall{
								Arguments: ptr.Of(`{"location": "New York", `),
							},
						},
					},
				},
			}
			param.ResultStream <- &entity.ReplyItem{
				Message: &entity.Message{
					Role: entity.RoleAssistant,
					ToolCalls: []*entity.ToolCall{
						{
							Index: 0,
							FunctionCall: &entity.FunctionCall{
								Arguments: ptr.Of(`"unit": "c"}`),
							},
						},
					},
				},
			}
			finishReason := "tool_calls"
			tokenUsage := &entity.TokenUsage{
				InputTokens:  20,
				OutputTokens: 10,
			}
			param.ResultStream <- &entity.ReplyItem{
				FinishReason: finishReason,
			}
			param.ResultStream <- &entity.ReplyItem{
				TokenUsage: tokenUsage,
			}
			return &entity.ReplyItem{
				Message: &entity.Message{
					Role: entity.RoleAssistant,
					ToolCalls: []*entity.ToolCall{
						{
							Index: 0,
							ID:    "call_123456",
							Type:  entity.ToolTypeFunction,
							FunctionCall: &entity.FunctionCall{
								Name:      "get_weather",
								Arguments: ptr.Of(`{"location": "New York", "unit": "c"}`),
							},
						},
					},
				},
				FinishReason: finishReason,
				TokenUsage:   tokenUsage,
			}, nil
		})
		mockLLM.EXPECT().StreamingCall(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, param rpc.LLMStreamingCallParam) (*entity.ReplyItem, error) {
			assert.Equal(t, 4, len(param.Messages))
			mockContent := "sunny"
			for _, v := range mockContent {
				param.ResultStream <- &entity.ReplyItem{
					Message: &entity.Message{
						Role:    entity.RoleAssistant,
						Content: ptr.Of(string(v)),
					},
				}
			}
			finishReason := "stop"
			tokenUsage := &entity.TokenUsage{
				InputTokens:  10,
				OutputTokens: 5,
			}
			param.ResultStream <- &entity.ReplyItem{
				FinishReason: finishReason,
			}
			param.ResultStream <- &entity.ReplyItem{
				TokenUsage: tokenUsage,
			}
			return &entity.ReplyItem{
				Message: &entity.Message{
					Role:    entity.RoleAssistant,
					Content: ptr.Of(mockContent),
				},
				FinishReason: finishReason,
				TokenUsage:   tokenUsage,
			}, nil
		})
		wantReplyItem := &entity.Reply{
			Item: &entity.ReplyItem{
				Message: &entity.Message{
					Role:    entity.RoleAssistant,
					Content: ptr.Of("sunny"),
				},
				FinishReason: "stop",
				TokenUsage: &entity.TokenUsage{
					InputTokens:  30,
					OutputTokens: 15,
				},
			},
			DebugID:   123456789,
			DebugStep: 2,
		}
		p := &PromptServiceImpl{
			formatter: NewPromptFormatter(),
			idgen:     mockIDGen,
			llm:       mockLLM,
		}

		stream := make(chan *entity.Reply)
		param := ExecuteStreamingParam{
			ExecuteParam: ExecuteParam{
				Prompt: &entity.Prompt{
					ID:        1,
					SpaceID:   123,
					PromptKey: "test_prompt",
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
				Messages: []*entity.Message{
					{
						Role:    entity.RoleUser,
						Content: ptr.Of("What's the weather in New York?"),
					},
				},
				MockTools: []*entity.MockTool{
					{
						Name:         "get_weather",
						MockResponse: "sunny",
					},
				},
				SingleStep: false,
			},
			ResultStream: stream,
		}
		go func() {
			defer close(stream)
			gotReply, err := p.ExecuteStreaming(context.Background(), param)
			assert.Nil(t, err)
			assert.NotEmpty(t, gotReply.DebugTraceKey)
			assert.Equal(t, wantReplyItem.Item, gotReply.Item)
			assert.Equal(t, wantReplyItem.DebugID, gotReply.DebugID)
			assert.Equal(t, wantReplyItem.DebugStep, gotReply.DebugStep)
		}()
		var toolCallArguments string
		var finalContent string
		for reply := range stream {
			assert.NotEmpty(t, reply.DebugTraceKey)
			assert.Equal(t, wantReplyItem.DebugID, reply.DebugID)
			if reply.DebugStep == 1 {
				assert.Equal(t, reply.DebugStep, int32(1))
				if reply.Item != nil {
					if reply.Item.FinishReason != "" {
						assert.Equal(t, "tool_calls", reply.Item.FinishReason)
					}
					if reply.Item.TokenUsage != nil {
						assert.Equal(t, &entity.TokenUsage{InputTokens: 20, OutputTokens: 10}, reply.Item.TokenUsage)
					}
					if reply.Item.Message != nil && len(reply.Item.Message.ToolCalls) > 0 {
						toolCall := reply.Item.Message.ToolCalls[0]
						if toolCall.ID != "" {
							assert.Equal(t, "call_123456", toolCall.ID)
						}
						if toolCall.Type != "" {
							assert.Equal(t, entity.ToolTypeFunction, toolCall.Type)
						}
						if toolCall.FunctionCall != nil {
							if toolCall.FunctionCall.Name != "" {
								assert.Equal(t, "get_weather", toolCall.FunctionCall.Name)
							}
							if arguments := ptr.From(toolCall.FunctionCall.Arguments); arguments != "" {
								toolCallArguments += arguments
							}
						}
					}
				}
			} else {
				assert.Equal(t, reply.DebugStep, int32(2))
				if reply.Item != nil {
					if reply.Item.Message != nil {
						finalContent += ptr.From(reply.Item.Message.Content)
					}
					if reply.Item.FinishReason != "" {
						assert.Equal(t, wantReplyItem.Item.FinishReason, reply.Item.FinishReason)
					}
					if reply.Item.TokenUsage != nil {
						assert.Equal(t, &entity.TokenUsage{InputTokens: 10, OutputTokens: 5}, reply.Item.TokenUsage)
					}
				}
			}
		}
		assert.Equal(t, `{"location": "New York", "unit": "c"}`, toolCallArguments)
		assert.Equal(t, "sunny", finalContent)
	})
}

func TestPromptServiceImpl_Execute(t *testing.T) {
	type fields struct {
		idgen            idgen.IIDGenerator
		debugLogRepo     repo.IDebugLogRepo
		debugContextRepo repo.IDebugContextRepo
		manageRepo       repo.IManageRepo
		configProvider   conf.IConfigProvider
		llm              rpc.ILLMProvider
		file             rpc.IFileProvider
	}
	type args struct {
		ctx   context.Context
		param ExecuteParam
	}
	mockContent := "Hello!"
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantReply    *entity.Reply
		wantErr      error
	}{
		{
			name: "nil prompt",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				param: ExecuteParam{
					Prompt: nil,
				},
			},
			wantErr: errorx.New("invalid param"),
		},
		{
			name: "single step execution success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLLM := rpcmocks.NewMockILLMProvider(ctrl)
				mockLLM.EXPECT().Call(gomock.Any(), gomock.Any()).Return(&entity.ReplyItem{
					Message: &entity.Message{
						Role:    entity.RoleAssistant,
						Content: ptr.Of(mockContent),
					},
					FinishReason: "stop",
					TokenUsage: &entity.TokenUsage{
						InputTokens:  10,
						OutputTokens: 5,
					},
				}, nil)
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123456789), nil)
				return fields{
					llm:   mockLLM,
					idgen: mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				param: ExecuteParam{
					Prompt: &entity.Prompt{
						ID:        1,
						SpaceID:   123,
						PromptKey: "test_prompt",
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
					Messages: []*entity.Message{
						{
							Role:    entity.RoleUser,
							Content: ptr.Of("Hello"),
						},
					},
					SingleStep: true,
				},
			},
			wantReply: &entity.Reply{
				Item: &entity.ReplyItem{
					Message: &entity.Message{
						Role:    entity.RoleAssistant,
						Content: ptr.Of(mockContent),
					},
					FinishReason: "stop",
					TokenUsage: &entity.TokenUsage{
						InputTokens:  10,
						OutputTokens: 5,
					},
				},
				DebugID:   123456789,
				DebugStep: 1,
			},
		},
		{
			name: "multi-step execution success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123456789), nil)
				mockLLM := rpcmocks.NewMockILLMProvider(ctrl)
				mockLLM.EXPECT().Call(gomock.Any(), gomock.Any()).Return(&entity.ReplyItem{
					Message: &entity.Message{
						Role: entity.RoleAssistant,
						ToolCalls: []*entity.ToolCall{
							{
								Index: 0,
								ID:    "call_123456",
								Type:  entity.ToolTypeFunction,
								FunctionCall: &entity.FunctionCall{
									Name:      "get_weather",
									Arguments: ptr.Of(`{"location": "New York", "unit": "c"}`),
								},
							},
						},
					},
					FinishReason: "tool_calls",
					TokenUsage: &entity.TokenUsage{
						InputTokens:  20,
						OutputTokens: 10,
					},
				}, nil)
				mockLLM.EXPECT().Call(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, param rpc.LLMCallParam) (*entity.ReplyItem, error) {
					assert.Equal(t, 4, len(param.Messages))
					return &entity.ReplyItem{
						Message: &entity.Message{
							Role:    entity.RoleAssistant,
							Content: ptr.Of("sunny"),
						},
						FinishReason: "stop",
						TokenUsage: &entity.TokenUsage{
							InputTokens:  10,
							OutputTokens: 5,
						},
					}, nil
				})
				return fields{
					llm:   mockLLM,
					idgen: mockIDGen,
				}
			},
			args: args{
				ctx: context.Background(),
				param: ExecuteParam{
					Prompt: &entity.Prompt{
						ID:        1,
						SpaceID:   123,
						PromptKey: "test_prompt",
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
					Messages: []*entity.Message{
						{
							Role:    entity.RoleUser,
							Content: ptr.Of("What's the weather in New York?"),
						},
					},
					SingleStep: false,
				},
			},
			wantReply: &entity.Reply{
				Item: &entity.ReplyItem{
					Message: &entity.Message{
						Role:    entity.RoleAssistant,
						Content: ptr.Of("sunny"),
					},
					FinishReason: "stop",
					TokenUsage: &entity.TokenUsage{
						InputTokens:  30,
						OutputTokens: 15,
					},
				},
				DebugID:   123456789,
				DebugStep: 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptServiceImpl{
				formatter:        NewPromptFormatter(),
				idgen:            ttFields.idgen,
				debugLogRepo:     ttFields.debugLogRepo,
				debugContextRepo: ttFields.debugContextRepo,
				manageRepo:       ttFields.manageRepo,
				configProvider:   ttFields.configProvider,
				llm:              ttFields.llm,
				file:             ttFields.file,
			}

			gotReply, err := p.Execute(tt.args.ctx, tt.args.param)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr == nil {
				assert.NotEmpty(t, gotReply.DebugTraceKey)
				assert.Equal(t, tt.wantReply.Item, gotReply.Item)
				assert.Equal(t, tt.wantReply.DebugID, gotReply.DebugID)
				assert.Equal(t, tt.wantReply.DebugStep, gotReply.DebugStep)
			}
		})
	}
}

func TestPromptServiceImpl_prepareLLMCallParam_PreservesExtra(t *testing.T) {
	t.Parallel()
	extra := ptr.Of(`{"foo":"bar"}`)
	prompt := &entity.Prompt{
		ID:        1,
		SpaceID:   42,
		PromptKey: "test_prompt",
		PromptCommit: &entity.PromptCommit{
			CommitInfo: &entity.CommitInfo{
				Version: "v1",
			},
			PromptDetail: &entity.PromptDetail{
				ModelConfig: &entity.ModelConfig{
					ModelID:  99,
					Extra:    extra,
					JSONMode: ptr.Of(true),
				},
				PromptTemplate: &entity.PromptTemplate{
					TemplateType: entity.TemplateTypeNormal,
					Messages: []*entity.Message{
						{
							Role:    entity.RoleSystem,
							Content: ptr.Of("System prompt"),
						},
					},
				},
			},
		},
	}
	svc := &PromptServiceImpl{
		formatter: NewPromptFormatter(),
	}
	param := ExecuteParam{
		Prompt: prompt,
		Messages: []*entity.Message{
			{
				Role:    entity.RoleUser,
				Content: ptr.Of("Hi"),
			},
		},
		VariableVals: nil,
		Scenario:     entity.ScenarioPromptDebug,
	}
	got, err := svc.prepareLLMCallParam(context.Background(), param)
	assert.NoError(t, err)
	if assert.NotNil(t, got.ModelConfig) {
		assert.Equal(t, extra, got.ModelConfig.Extra)
		assert.Equal(t, prompt.PromptCommit.PromptDetail.ModelConfig.Extra, got.ModelConfig.Extra)
	}
}
