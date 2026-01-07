// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/looptracer"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
	loopentity "github.com/coze-dev/cozeloop-go/entity"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
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
				formatter:            NewPromptFormatter(),
				toolConfigProvider:   NewToolConfigProvider(),
				toolResultsCollector: NewToolResultsCollector(),
				idgen:                ttFields.idgen,
				debugLogRepo:         ttFields.debugLogRepo,
				debugContextRepo:     ttFields.debugContextRepo,
				manageRepo:           ttFields.manageRepo,
				configProvider:       ttFields.configProvider,
				llm:                  ttFields.llm,
				file:                 ttFields.file,
			}
			gotFormattedMessages, err := p.FormatPrompt(tt.args.ctx, tt.args.prompt, tt.args.messages, tt.args.variableVals)

			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr == nil {
				assert.Equal(t, normalizeSkipRender(tt.wantFormattedMessages), normalizeSkipRender(gotFormattedMessages))
			}
		})
	}
}

func normalizeSkipRender(messages []*entity.Message) []*entity.Message {
	for _, message := range messages {
		if message == nil {
			continue
		}
		message.SkipRender = nil
	}
	return messages
}

func TestPromptServiceImpl_ExecuteStreaming(t *testing.T) {
	t.Run("nil prompt", func(t *testing.T) {
		t.Parallel()

		p := &PromptServiceImpl{
			formatter:            NewPromptFormatter(),
			toolConfigProvider:   NewToolConfigProvider(),
			toolResultsCollector: NewToolResultsCollector(),
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
			formatter:            NewPromptFormatter(),
			toolConfigProvider:   NewToolConfigProvider(),
			toolResultsCollector: NewToolResultsCollector(),
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
			formatter:            NewPromptFormatter(),
			toolConfigProvider:   NewToolConfigProvider(),
			toolResultsCollector: NewToolResultsCollector(),
			idgen:                mockIDGen,
			llm:                  mockLLM,
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
			formatter:            NewPromptFormatter(),
			toolConfigProvider:   NewToolConfigProvider(),
			toolResultsCollector: NewToolResultsCollector(),
			idgen:                mockIDGen,
			llm:                  mockLLM,
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
		{
			name: "error_llm_call_failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123456789), nil)
				mockLLM := rpcmocks.NewMockILLMProvider(ctrl)
				mockLLM.EXPECT().Call(gomock.Any(), gomock.Any()).Return(nil, errorx.New("llm call failed"))
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
			wantErr: errorx.New("llm call failed"),
		},
		{
			name: "error_format_prompt_failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(int64(123456789), nil)
				return fields{
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
									TemplateType: entity.TemplateTypeGoTemplate,
									Messages: []*entity.Message{
										{
											Role:    entity.RoleSystem,
											Content: ptr.Of("You are a {{.InvalidSyntax"), // Invalid template
										},
									},
								},
							},
						},
					},
					SingleStep: true,
				},
			},
			wantReply: nil,
			wantErr:   errorx.NewByCode(prompterr.TemplateParseErrorCode),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptServiceImpl{
				formatter:            NewPromptFormatter(),
				toolConfigProvider:   NewToolConfigProvider(),
				toolResultsCollector: NewToolResultsCollector(),
				idgen:                ttFields.idgen,
				debugLogRepo:         ttFields.debugLogRepo,
				debugContextRepo:     ttFields.debugContextRepo,
				manageRepo:           ttFields.manageRepo,
				configProvider:       ttFields.configProvider,
				llm:                  ttFields.llm,
				file:                 ttFields.file,
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
		formatter:            NewPromptFormatter(),
		toolConfigProvider:   NewToolConfigProvider(),
		toolResultsCollector: NewToolResultsCollector(),
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

func TestPromptServiceImpl_prepareLLMCallParam_ValidationErrors(t *testing.T) {
	t.Parallel()
	svc := &PromptServiceImpl{
		formatter:            NewPromptFormatter(),
		toolConfigProvider:   NewToolConfigProvider(),
		toolResultsCollector: NewToolResultsCollector(),
	}

	tests := []struct {
		name        string
		param       ExecuteParam
		wantErr     bool
		errContains string
	}{
		{
			name: "specific tool choice without single step mode - should error",
			param: ExecuteParam{
				Prompt: &entity.Prompt{
					ID:        1,
					SpaceID:   42,
					PromptKey: "test_prompt",
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{
							Version: "v1",
						},
						PromptDetail: &entity.PromptDetail{
							ToolCallConfig: &entity.ToolCallConfig{
								ToolChoice: entity.ToolChoiceTypeSpecific,
								ToolChoiceSpecification: &entity.ToolChoiceSpecification{
									Type: entity.ToolTypeFunction,
									Name: "get_weather",
								},
							},
							Tools: []*entity.Tool{
								{
									Type: entity.ToolTypeFunction,
									Function: &entity.Function{
										Name:        "get_weather",
										Description: "Get weather",
										Parameters:  "{}",
									},
								},
							},
							ModelConfig: &entity.ModelConfig{
								ModelID: 1,
							},
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleSystem,
										Content: ptr.Of("Test"),
									},
								},
							},
						},
					},
				},
				Messages:   []*entity.Message{},
				SingleStep: false, // Should be true for specific tool choice
				Scenario:   entity.ScenarioPromptDebug,
			},
			wantErr:     true,
			errContains: "single step mode",
		},
		{
			name: "specific tool choice without specification - should error",
			param: ExecuteParam{
				Prompt: &entity.Prompt{
					ID:        1,
					SpaceID:   42,
					PromptKey: "test_prompt",
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{
							Version: "v1",
						},
						PromptDetail: &entity.PromptDetail{
							ToolCallConfig: &entity.ToolCallConfig{
								ToolChoice:              entity.ToolChoiceTypeSpecific,
								ToolChoiceSpecification: nil, // Should not be nil
							},
							Tools: []*entity.Tool{
								{
									Type: entity.ToolTypeFunction,
									Function: &entity.Function{
										Name:        "get_weather",
										Description: "Get weather",
										Parameters:  "{}",
									},
								},
							},
							ModelConfig: &entity.ModelConfig{
								ModelID: 1,
							},
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleSystem,
										Content: ptr.Of("Test"),
									},
								},
							},
						},
					},
				},
				Messages:   []*entity.Message{},
				SingleStep: true,
				Scenario:   entity.ScenarioPromptDebug,
			},
			wantErr:     true,
			errContains: "must not be empty",
		},
		{
			name: "specific tool choice with single step and specification - should succeed",
			param: ExecuteParam{
				Prompt: &entity.Prompt{
					ID:        1,
					SpaceID:   42,
					PromptKey: "test_prompt",
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{
							Version: "v1",
						},
						PromptDetail: &entity.PromptDetail{
							ToolCallConfig: &entity.ToolCallConfig{
								ToolChoice: entity.ToolChoiceTypeSpecific,
								ToolChoiceSpecification: &entity.ToolChoiceSpecification{
									Type: entity.ToolTypeFunction,
									Name: "get_weather",
								},
							},
							Tools: []*entity.Tool{
								{
									Type: entity.ToolTypeFunction,
									Function: &entity.Function{
										Name:        "get_weather",
										Description: "Get weather",
										Parameters:  "{}",
									},
								},
							},
							ModelConfig: &entity.ModelConfig{
								ModelID: 1,
							},
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleSystem,
										Content: ptr.Of("Test"),
									},
								},
							},
						},
					},
				},
				Messages:   []*entity.Message{},
				SingleStep: true,
				Scenario:   entity.ScenarioPromptDebug,
			},
			wantErr: false,
		},
		{
			name: "specific tool choice with google_search - should succeed",
			param: ExecuteParam{
				Prompt: &entity.Prompt{
					ID:        1,
					SpaceID:   42,
					PromptKey: "test_prompt",
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{
							Version: "v1",
						},
						PromptDetail: &entity.PromptDetail{
							ToolCallConfig: &entity.ToolCallConfig{
								ToolChoice: entity.ToolChoiceTypeSpecific,
								ToolChoiceSpecification: &entity.ToolChoiceSpecification{
									Type: entity.ToolTypeGoogleSearch,
									Name: "search",
								},
							},
							Tools: []*entity.Tool{
								{
									Type: entity.ToolTypeGoogleSearch,
								},
							},
							ModelConfig: &entity.ModelConfig{
								ModelID: 1,
							},
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleSystem,
										Content: ptr.Of("Test"),
									},
								},
							},
						},
					},
				},
				Messages:   []*entity.Message{},
				SingleStep: true,
				Scenario:   entity.ScenarioPromptDebug,
			},
			wantErr: false,
		},
		{
			name: "auto tool choice - should succeed without validation",
			param: ExecuteParam{
				Prompt: &entity.Prompt{
					ID:        1,
					SpaceID:   42,
					PromptKey: "test_prompt",
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{
							Version: "v1",
						},
						PromptDetail: &entity.PromptDetail{
							ToolCallConfig: &entity.ToolCallConfig{
								ToolChoice: entity.ToolChoiceTypeAuto,
							},
							Tools: []*entity.Tool{
								{
									Type: entity.ToolTypeFunction,
									Function: &entity.Function{
										Name:        "get_weather",
										Description: "Get weather",
										Parameters:  "{}",
									},
								},
							},
							ModelConfig: &entity.ModelConfig{
								ModelID: 1,
							},
							PromptTemplate: &entity.PromptTemplate{
								TemplateType: entity.TemplateTypeNormal,
								Messages: []*entity.Message{
									{
										Role:    entity.RoleSystem,
										Content: ptr.Of("Test"),
									},
								},
							},
						},
					},
				},
				Messages:   []*entity.Message{},
				SingleStep: false,
				Scenario:   entity.ScenarioPromptDebug,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := svc.prepareLLMCallParam(context.Background(), tt.param)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				if tt.param.Prompt.PromptCommit.PromptDetail.ToolCallConfig != nil {
					assert.Equal(t, tt.param.Prompt.PromptCommit.PromptDetail.ToolCallConfig, got.ToolCallConfig)
				}
			}
		})
	}
}

func TestPromptServiceImpl_reorganizeContexts_ToolResultMap(t *testing.T) {
	t.Parallel()

	p := &PromptServiceImpl{}
	reply := &entity.Reply{
		Item: &entity.ReplyItem{
			Message: &entity.Message{
				Role:    entity.RoleAssistant,
				Content: ptr.Of("assistant"),
				ToolCalls: []*entity.ToolCall{
					{
						ID: "call_1",
						FunctionCall: &entity.FunctionCall{
							Name:      "tool_a",
							Arguments: ptr.Of(`{"k":"v"}`),
						},
					},
				},
			},
		},
	}

	got, err := p.reorganizeContexts(
		[]*entity.Message{{Role: entity.RoleUser, Content: ptr.Of("user")}},
		map[string]string{"tool_a": "tool output"},
		reply,
	)
	assert.NoError(t, err)
	assert.Len(t, got, 3)
	assert.Equal(t, entity.RoleUser, got[0].Role)
	assert.Equal(t, entity.RoleAssistant, got[1].Role)
	assert.Equal(t, entity.RoleTool, got[2].Role)
	assert.Equal(t, ptr.Of("call_1"), got[2].ToolCallID)
	assert.Equal(t, ptr.Of("tool output"), got[2].Content)
}

func TestPromptServiceImpl_reportToolSpan_UsesToolResultMap(t *testing.T) {
	t.Parallel()

	originalTracer := looptracer.GetTracer()
	recorder := &recordingTracer{}
	looptracer.InitTracer(recorder)
	t.Cleanup(func() { looptracer.InitTracer(originalTracer) })

	p := &PromptServiceImpl{}
	prompt := &entity.Prompt{
		SpaceID:   42,
		PromptKey: "pk",
		PromptCommit: &entity.PromptCommit{
			CommitInfo: &entity.CommitInfo{Version: "v1"},
		},
	}
	args := ptr.Of(`{"a":1}`)
	replyItem := &entity.ReplyItem{
		Message: &entity.Message{
			ToolCalls: []*entity.ToolCall{
				{
					ID: "call_1",
					FunctionCall: &entity.FunctionCall{
						Name:      "tool_a",
						Arguments: args,
					},
				},
			},
		},
	}

	p.reportToolSpan(context.Background(), prompt, map[string]string{"tool_a": "tool output"}, replyItem)

	if assert.Len(t, recorder.spans, 1) {
		assert.Equal(t, "tool output", recorder.spans[0].output)
		assert.Same(t, args, recorder.spans[0].input)
		assert.Equal(t, loopentity.Prompt{PromptKey: "pk", Version: "v1"}, recorder.spans[0].prompt)
		assert.True(t, recorder.spans[0].finished)
	}
}

type recordingTracer struct {
	spans []*recordingSpan
}

func (r *recordingTracer) StartSpan(ctx context.Context, name, spanType string, _ ...looptracer.StartSpanOption) (context.Context, looptracer.Span) {
	span := &recordingSpan{name: name, spanType: spanType, startTime: time.Now()}
	r.spans = append(r.spans, span)
	return ctx, span
}

func (r *recordingTracer) GetSpanFromContext(ctx context.Context) looptracer.Span { return nil }
func (r *recordingTracer) Flush(ctx context.Context)                              {}
func (r *recordingTracer) Inject(ctx context.Context) context.Context             { return ctx }
func (r *recordingTracer) InjectW3CTraceContext(ctx context.Context) map[string]string {
	return map[string]string{}
}

type recordingSpan struct {
	name      string
	spanType  string
	startTime time.Time

	input    any
	output   any
	prompt   loopentity.Prompt
	finished bool
}

func (s *recordingSpan) SetServiceName(ctx context.Context, serviceName string) {}
func (s *recordingSpan) SetLogID(ctx context.Context, logID string)             {}
func (s *recordingSpan) SetFinishTime(finishTime time.Time)                     {}
func (s *recordingSpan) SetSystemTags(ctx context.Context, systemTags map[string]interface{}) {
}
func (s *recordingSpan) SetDeploymentEnv(ctx context.Context, deploymentEnv string) {}
func (s *recordingSpan) GetSpanID() string                                          { return "" }
func (s *recordingSpan) GetTraceID() string                                         { return "" }
func (s *recordingSpan) GetBaggage() map[string]string                              { return nil }
func (s *recordingSpan) SetInput(ctx context.Context, input interface{})            { s.input = input }
func (s *recordingSpan) SetOutput(ctx context.Context, output interface{})          { s.output = output }
func (s *recordingSpan) SetError(ctx context.Context, err error)                    {}
func (s *recordingSpan) SetStatusCode(ctx context.Context, code int)                {}
func (s *recordingSpan) SetUserID(ctx context.Context, userID string)               {}
func (s *recordingSpan) SetUserIDBaggage(ctx context.Context, userID string)        {}
func (s *recordingSpan) SetMessageID(ctx context.Context, messageID string)         {}
func (s *recordingSpan) SetMessageIDBaggage(ctx context.Context, messageID string)  {}
func (s *recordingSpan) SetThreadID(ctx context.Context, threadID string)           {}
func (s *recordingSpan) SetThreadIDBaggage(ctx context.Context, threadID string)    {}
func (s *recordingSpan) SetPrompt(ctx context.Context, prompt loopentity.Prompt)    { s.prompt = prompt }
func (s *recordingSpan) SetModelProvider(ctx context.Context, modelProvider string) {
}
func (s *recordingSpan) SetModelName(ctx context.Context, modelName string) {}
func (s *recordingSpan) SetModelCallOptions(ctx context.Context, callOptions interface{}) {
}
func (s *recordingSpan) SetInputTokens(ctx context.Context, inputTokens int) {}
func (s *recordingSpan) SetOutputTokens(ctx context.Context, outputTokens int) {
}
func (s *recordingSpan) SetStartTimeFirstResp(ctx context.Context, startTimeFirstResp int64) {
}
func (s *recordingSpan) SetRuntime(ctx context.Context, runtime tracespec.Runtime) {}
func (s *recordingSpan) SetTags(ctx context.Context, tagKVs map[string]interface{}) {
}
func (s *recordingSpan) SetBaggage(ctx context.Context, baggageItems map[string]string) {}
func (s *recordingSpan) Finish(ctx context.Context)                                     { s.finished = true }
func (s *recordingSpan) GetStartTime() time.Time                                        { return s.startTime }
func (s *recordingSpan) ToHeader() (map[string]string, error)                           { return map[string]string{}, nil }
func (s *recordingSpan) SetCallType(callType string)                                    {}
