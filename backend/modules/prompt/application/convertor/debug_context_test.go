// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type debugContextTestCase struct {
	name     string
	promptID int64
	userID   string
	dto      *prompt.DebugContext
	do       *entity.DebugContext
}

func mockDebugContextCases() []debugContextTestCase {
	// 定义共享的测试用例
	return []debugContextTestCase{
		{
			name:     "nil input",
			promptID: 123,
			userID:   "test_user",
			dto:      &prompt.DebugContext{},
			do: &entity.DebugContext{
				PromptID: 123,
				UserID:   "test_user",
			},
		},
		{
			name:     "empty debug context",
			promptID: 123,
			userID:   "test_user",
			dto:      &prompt.DebugContext{},
			do: &entity.DebugContext{
				PromptID: 123,
				UserID:   "test_user",
			},
		},
		{
			name:     "debug context with debug config only",
			promptID: 123,
			userID:   "test_user",
			dto: &prompt.DebugContext{
				DebugConfig: &prompt.DebugConfig{
					SingleStepDebug: ptr.Of(true),
				},
			},
			do: &entity.DebugContext{
				PromptID: 123,
				UserID:   "test_user",
				DebugConfig: &entity.DebugConfig{
					SingleStepDebug: ptr.Of(true),
				},
			},
		},
		{
			name:     "debug context with debug core only",
			promptID: 123,
			userID:   "test_user",
			dto: &prompt.DebugContext{
				DebugCore: &prompt.DebugCore{
					MockContexts: []*prompt.DebugMessage{
						{
							Role:    ptr.Of(prompt.RoleUser),
							Content: ptr.Of("Test message"),
						},
					},
					MockVariables: []*prompt.VariableVal{
						{
							Key:   ptr.Of("test_var"),
							Value: ptr.Of("test_value"),
						},
					},
					MockTools: []*prompt.MockTool{
						{
							Name:         ptr.Of("test_tool"),
							MockResponse: ptr.Of("test_response"),
						},
					},
				},
			},
			do: &entity.DebugContext{
				PromptID: 123,
				UserID:   "test_user",
				DebugCore: &entity.DebugCore{
					MockContexts: []*entity.DebugMessage{
						{
							Role:    entity.RoleUser,
							Content: ptr.Of("Test message"),
						},
					},
					MockVariables: []*entity.VariableVal{
						{
							Key:   "test_var",
							Value: ptr.Of("test_value"),
						},
					},
					MockTools: []*entity.MockTool{
						{
							Name:         "test_tool",
							MockResponse: "test_response",
						},
					},
				},
			},
		},
		{
			name:     "debug context with compare config only",
			promptID: 123,
			userID:   "test_user",
			dto: &prompt.DebugContext{
				CompareConfig: &prompt.CompareConfig{
					Groups: []*prompt.CompareGroup{
						{
							PromptDetail: &prompt.PromptDetail{
								PromptTemplate: &prompt.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									HasSnippet:   ptr.Of(false),
									Messages: []*prompt.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
								},
							},
							DebugCore: &prompt.DebugCore{
								MockContexts: []*prompt.DebugMessage{
									{
										Role:    ptr.Of(prompt.RoleUser),
										Content: ptr.Of("Compare test message"),
									},
								},
							},
						},
					},
				},
			},
			do: &entity.DebugContext{
				PromptID: 123,
				UserID:   "test_user",
				CompareConfig: &entity.CompareConfig{
					Groups: []*entity.CompareGroup{
						{
							PromptDetail: &entity.PromptDetail{
								PromptTemplate: &entity.PromptTemplate{
									TemplateType: entity.TemplateTypeNormal,
									HasSnippets:  false,
									Messages: []*entity.Message{
										{
											Role:    entity.RoleSystem,
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
								},
							},
							DebugCore: &entity.DebugCore{
								MockContexts: []*entity.DebugMessage{
									{
										Role:    entity.RoleUser,
										Content: ptr.Of("Compare test message"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:     "complete debug context",
			promptID: 123,
			userID:   "test_user",
			dto: &prompt.DebugContext{
				DebugCore: &prompt.DebugCore{
					MockContexts: []*prompt.DebugMessage{
						{
							Role:         ptr.Of(prompt.RoleUser),
							Content:      ptr.Of("Test message"),
							DebugID:      ptr.Of("debug-123"),
							InputTokens:  ptr.Of(int64(10)),
							OutputTokens: ptr.Of(int64(20)),
							CostMs:       ptr.Of(int64(100)),
						},
					},
					MockTools: []*prompt.MockTool{
						{
							Name:         ptr.Of("test_tool"),
							MockResponse: ptr.Of("test_response"),
						},
					},
				},
				DebugConfig: &prompt.DebugConfig{
					SingleStepDebug: ptr.Of(true),
				},
				CompareConfig: &prompt.CompareConfig{
					Groups: []*prompt.CompareGroup{
						{
							PromptDetail: &prompt.PromptDetail{
								PromptTemplate: &prompt.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
									HasSnippet:   ptr.Of(false),
									Messages: []*prompt.Message{
										{
											Role:    ptr.Of(prompt.RoleSystem),
											Content: ptr.Of("You are a helpful assistant."),
										},
									},
								},
							},
						},
					},
				},
			},
			do: &entity.DebugContext{
				PromptID: 123,
				UserID:   "test_user",
				DebugCore: &entity.DebugCore{
					MockContexts: []*entity.DebugMessage{
						{
							Role:         entity.RoleUser,
							Content:      ptr.Of("Test message"),
							DebugID:      ptr.Of("debug-123"),
							InputTokens:  ptr.Of(int64(10)),
							OutputTokens: ptr.Of(int64(20)),
							CostMS:       ptr.Of(int64(100)),
						},
					},
					MockTools: []*entity.MockTool{
						{
							Name:         "test_tool",
							MockResponse: "test_response",
						},
					},
				},
				DebugConfig: &entity.DebugConfig{
					SingleStepDebug: ptr.Of(true),
				},
				CompareConfig: &entity.CompareConfig{
					Groups: []*entity.CompareGroup{
						{
							PromptDetail: &entity.PromptDetail{
								PromptTemplate: &entity.PromptTemplate{
									TemplateType: entity.TemplateTypeNormal,
									HasSnippets:  false,
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
				},
			},
		},
		{
			name:     "debug context with complex debug message",
			promptID: 123,
			userID:   "test_user",
			dto: &prompt.DebugContext{
				DebugCore: &prompt.DebugCore{
					MockContexts: []*prompt.DebugMessage{
						{
							Role:             ptr.Of(prompt.RoleAssistant),
							Content:          ptr.Of("Final answer"),
							ReasoningContent: ptr.Of("This is my reasoning process..."),
							ToolCalls: []*prompt.DebugToolCall{
								{
									ToolCall: &prompt.ToolCall{
										Index: ptr.Of(int64(0)),
										ID:    ptr.Of("tool-call-123"),
										Type:  ptr.Of(prompt.ToolTypeFunction),
										FunctionCall: &prompt.FunctionCall{
											Name:      ptr.Of("get_weather"),
											Arguments: ptr.Of(`{"location": "New York"}`),
										},
									},
									MockResponse:  ptr.Of(`{"temperature": 25, "condition": "sunny"}`),
									DebugTraceKey: ptr.Of("trace-key-123"),
								},
							},
						},
						{
							Role:       ptr.Of(prompt.RoleTool),
							Content:    ptr.Of(`{"temperature": 25, "condition": "sunny"}`),
							ToolCallID: ptr.Of("tool-call-123"),
						},
					},
				},
			},
			do: &entity.DebugContext{
				PromptID: 123,
				UserID:   "test_user",
				DebugCore: &entity.DebugCore{
					MockContexts: []*entity.DebugMessage{
						{
							Role:             entity.RoleAssistant,
							Content:          ptr.Of("Final answer"),
							ReasoningContent: ptr.Of("This is my reasoning process..."),
							ToolCalls: []*entity.DebugToolCall{
								{
									ToolCall: entity.ToolCall{
										Index: 0,
										ID:    "tool-call-123",
										Type:  entity.ToolTypeFunction,
										FunctionCall: &entity.FunctionCall{
											Name:      "get_weather",
											Arguments: ptr.Of(`{"location": "New York"}`),
										},
									},
									MockResponse:  `{"temperature": 25, "condition": "sunny"}`,
									DebugTraceKey: "trace-key-123",
								},
							},
						},
						{
							Role:       entity.RoleTool,
							Content:    ptr.Of(`{"temperature": 25, "condition": "sunny"}`),
							ToolCallID: ptr.Of("tool-call-123"),
						},
					},
				},
			},
		},
	}
}

func TestDebugContextDTO2DO(t *testing.T) {
	for _, tt := range mockDebugContextCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.do, DebugContextDTO2DO(tt.promptID, tt.userID, tt.dto))
		})
	}
}

func TestDebugContextDO2DTO(t *testing.T) {
	for _, tt := range mockDebugContextCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// 注意：DO2DTO不会包含promptID和userID
			assert.Equal(t, tt.dto, DebugContextDO2DTO(tt.do))
		})
	}
}
