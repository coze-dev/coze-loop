// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"
	"github.com/kaptinlin/jsonrepair"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/mocks"
)

// TestEvaluatorSourcePromptServiceImpl_Run 测试 Run 方法
func TestEvaluatorSourcePromptServiceImpl_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// These mocks will be shared across all test cases due to the singleton nature of the service
	sharedMockLLMProvider := rpcmocks.NewMockILLMProvider(ctrl)
	sharedMockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
	sharedMockConfiger := configmocks.NewMockIConfiger(ctrl)

	// Instantiate the service once with the shared mocks
	service := &EvaluatorSourcePromptServiceImpl{
		llmProvider: sharedMockLLMProvider,
		metric:      sharedMockMetric,
		configer:    sharedMockConfiger,
	}

	ctx := context.Background()
	baseMockEvaluator := &entity.Evaluator{
		ID:            100,
		SpaceID:       1,
		Name:          "Test Evaluator",
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:                100,
			EvaluatorID:       100,
			SpaceID:           1,
			PromptTemplateKey: "test-template-key",
			PromptSuffix:      "test-prompt-suffix",
			ModelConfig: &entity.ModelConfig{
				ModelID: 1,
			},
			ParseType: entity.ParseTypeFunctionCall,
			MessageList: []*entity.Message{
				{
					Role: entity.RoleSystem,
					Content: &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("{{test-content}}"),
					},
				},
			},
			Tools: []*entity.Tool{
				{
					Type: entity.ToolTypeFunction,
					Function: &entity.Function{
						Name:        "test_function",
						Description: "test description",
						Parameters:  "{\"type\": \"object\", \"properties\": {\"score\": {\"type\": \"number\"}, \"reasoning\": {\"type\": \"string\"}}}",
					},
				},
			},
		},
	}

	baseMockInput := &entity.EvaluatorInputData{
		InputFields: map[string]*entity.Content{
			"input": {
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of("test input"),
			},
		},
	}

	testCases := []struct {
		name            string
		evaluator       *entity.Evaluator
		input           *entity.EvaluatorInputData
		setupMocks      func()
		expectedOutput  *entity.EvaluatorOutputData
		expectedStatus  entity.EvaluatorRunStatus
		checkOutputFunc func(t *testing.T, output *entity.EvaluatorOutputData, expected *entity.EvaluatorOutputData)
	}{
		{
			name:      "成功运行评估器",
			evaluator: baseMockEvaluator,
			input:     baseMockInput,
			setupMocks: func() {
				sharedMockLLMProvider.EXPECT().Call(gomock.Any(), gomock.Any()).Return(
					&entity.ReplyItem{
						ToolCalls: []*entity.ToolCall{
							{
								Type: entity.ToolTypeFunction,
								FunctionCall: &entity.FunctionCall{
									Name:      "test_function",
									Arguments: gptr.Of("{\"score\": 1.0, \"reason\": \"test response\"}"),
								},
							},
						},
						TokenUsage: &entity.TokenUsage{InputTokens: 10, OutputTokens: 10},
					}, nil)
				sharedMockMetric.EXPECT().EmitRun(int64(1), gomock.Any(), gomock.Any(), gomock.Any())
			},
			expectedOutput: &entity.EvaluatorOutputData{
				EvaluatorResult:   &entity.EvaluatorResult{Score: gptr.Of(1.0), Reasoning: "test response"},
				EvaluatorUsage:    &entity.EvaluatorUsage{InputTokens: 10, OutputTokens: 10},
				EvaluatorRunError: nil,
			},
			expectedStatus: entity.EvaluatorRunStatusSuccess,
			checkOutputFunc: func(t *testing.T, output *entity.EvaluatorOutputData, expected *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorResult)
				assert.Equal(t, expected.EvaluatorResult.Score, output.EvaluatorResult.Score)
				assert.Equal(t, expected.EvaluatorResult.Reasoning, output.EvaluatorResult.Reasoning)
				assert.NotNil(t, output.EvaluatorUsage)
				assert.Equal(t, expected.EvaluatorUsage.InputTokens, output.EvaluatorUsage.InputTokens)
				assert.Equal(t, expected.EvaluatorUsage.OutputTokens, output.EvaluatorUsage.OutputTokens)
				assert.Nil(t, output.EvaluatorRunError)
				assert.GreaterOrEqual(t, output.TimeConsumingMS, int64(0))
			},
		},
		{
			name:      "LLM调用失败",
			evaluator: baseMockEvaluator,
			input:     baseMockInput,
			setupMocks: func() {
				expectedLlmError := errors.New("llm call failed")
				sharedMockLLMProvider.EXPECT().Call(gomock.Any(), gomock.Any()).Return(nil, expectedLlmError)
				sharedMockMetric.EXPECT().EmitRun(int64(1), expectedLlmError, gomock.Any(), gomock.Any())
			},
			expectedOutput: &entity.EvaluatorOutputData{
				EvaluatorRunError: &entity.EvaluatorRunError{Message: "llm call failed"},
				EvaluatorResult:   nil,
				EvaluatorUsage:    &entity.EvaluatorUsage{},
			},
			expectedStatus: entity.EvaluatorRunStatusFail,
			checkOutputFunc: func(t *testing.T, output *entity.EvaluatorOutputData, expected *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorRunError)
				assert.Contains(t, output.EvaluatorRunError.Message, expected.EvaluatorRunError.Message)
				assert.Nil(t, output.EvaluatorResult)
				assert.GreaterOrEqual(t, output.TimeConsumingMS, int64(0))
			},
		},
		{
			name:      "LLM返回ToolCalls为空",
			evaluator: baseMockEvaluator,
			input:     baseMockInput,
			setupMocks: func() {
				sharedMockLLMProvider.EXPECT().Call(gomock.Any(), gomock.Any()).Return(
					&entity.ReplyItem{
						ToolCalls: nil,
					}, nil)
				sharedMockMetric.EXPECT().EmitRun(int64(1), gomock.Any(), gomock.Any(), gomock.Any())
			},
			expectedOutput: &entity.EvaluatorOutputData{
				EvaluatorRunError: &entity.EvaluatorRunError{Message: "no tool calls returned from LLM"},
				EvaluatorResult:   nil,
				EvaluatorUsage:    &entity.EvaluatorUsage{InputTokens: 5, OutputTokens: 5},
			},
			expectedStatus: entity.EvaluatorRunStatusFail,
			checkOutputFunc: func(t *testing.T, output *entity.EvaluatorOutputData, expected *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorRunError)
				assert.Nil(t, output.EvaluatorResult)
			},
		},
		{
			name:      "LLM返回FunctionCall Arguments 字段为空",
			evaluator: baseMockEvaluator,
			input:     baseMockInput,
			setupMocks: func() {
				sharedMockLLMProvider.EXPECT().Call(gomock.Any(), gomock.Any()).Return(
					&entity.ReplyItem{
						ToolCalls: []*entity.ToolCall{{Type: entity.ToolTypeFunction, FunctionCall: &entity.FunctionCall{
							Name:      "test_function",
							Arguments: gptr.Of(""),
						}}},
						TokenUsage: &entity.TokenUsage{InputTokens: 8, OutputTokens: 8},
					}, nil)
				sharedMockMetric.EXPECT().EmitRun(int64(1), gomock.Any(), gomock.Any(), gomock.Any())
			},
			expectedOutput: &entity.EvaluatorOutputData{
				EvaluatorRunError: &entity.EvaluatorRunError{Message: "function call arguments are nil"},
				EvaluatorResult:   nil,
				EvaluatorUsage:    &entity.EvaluatorUsage{InputTokens: 8, OutputTokens: 8},
			},
			expectedStatus: entity.EvaluatorRunStatusFail,
			checkOutputFunc: func(t *testing.T, output *entity.EvaluatorOutputData, expected *entity.EvaluatorOutputData) {
				assert.NotNil(t, output.EvaluatorRunError)
				assert.Nil(t, output.EvaluatorResult)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupMocks != nil {
				tc.setupMocks()
			}

			output, status, _ := service.Run(ctx, tc.evaluator, tc.input)

			assert.Equal(t, tc.expectedStatus, status)
			if tc.checkOutputFunc != nil {
				tc.checkOutputFunc(t, output, tc.expectedOutput)
			}
		})
	}
}

// TestEvaluatorSourcePromptServiceImpl_PreHandle 测试 PreHandle 方法
func TestEvaluatorSourcePromptServiceImpl_PreHandle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLLMProvider := rpcmocks.NewMockILLMProvider(ctrl)
	mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
	mockConfiger := configmocks.NewMockIConfiger(ctrl)

	service := &EvaluatorSourcePromptServiceImpl{
		llmProvider: mockLLMProvider,
		metric:      mockMetric,
		configer:    mockConfiger,
	}
	ctx := context.Background()

	testCases := []struct {
		name        string
		evaluator   *entity.Evaluator
		setupMocks  func()
		expectedErr error
	}{
		{
			name: "成功预处理评估器",
			evaluator: &entity.Evaluator{
				ID:            100,
				SpaceID:       1,
				Name:          "Test Evaluator",
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					ID:                100,
					EvaluatorID:       100,
					SpaceID:           1,
					PromptTemplateKey: "test-template-key",
					PromptSuffix:      "test-prompt-suffix",
					ModelConfig: &entity.ModelConfig{
						ModelID: 1,
					},
					ParseType: entity.ParseTypeFunctionCall,
				},
			},
			setupMocks: func() {
				mockConfiger.EXPECT().GetEvaluatorPromptSuffix(gomock.Any()).Return(map[string]string{
					"test-template-key": "test-prompt-suffix",
				}).Times(1)
				mockConfiger.EXPECT().GetEvaluatorToolConf(gomock.Any()).Return(map[string]*evaluator.Tool{
					"test_function": {
						Type: evaluator.ToolType(entity.ToolTypeFunction),
						Function: &evaluator.Function{
							Name:        "test_function",
							Description: gptr.Of("test description"),
							Parameters:  gptr.Of("{\"type\": \"object\", \"properties\": {\"score\": {\"type\": \"number\"}, \"reasoning\": {\"type\": \"string\"}}}"),
						},
					},
				}).Times(2)
				mockConfiger.EXPECT().GetEvaluatorToolMapping(gomock.Any()).Return(map[string]string{
					"test-template-key": "test-function",
				}).Times(1)
				mockConfiger.EXPECT().GetEvaluatorPromptSuffixMapping(gomock.Any()).Return(map[string]string{
					"1": "test-prompt-suffix",
				}).Times(1)
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupMocks != nil {
				tc.setupMocks()
			}

			err := service.PreHandle(ctx, tc.evaluator)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewEvaluatorSourcePromptServiceImpl(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLLMProvider := rpcmocks.NewMockILLMProvider(ctrl)
	mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
	mockConfiger := configmocks.NewMockIConfiger(ctrl)

	service := NewEvaluatorSourcePromptServiceImpl(
		mockLLMProvider,
		mockMetric,
		mockConfiger,
	)
	assert.NotNil(t, service)
	assert.Implements(t, (*EvaluatorSourceService)(nil), service)
}

func TestEvaluatorSourcePromptServiceImpl_Debug(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLLMProvider := rpcmocks.NewMockILLMProvider(ctrl)
	mockMetric := metricsmocks.NewMockEvaluatorExecMetrics(ctrl)
	mockConfiger := configmocks.NewMockIConfiger(ctrl)

	service := &EvaluatorSourcePromptServiceImpl{
		llmProvider: mockLLMProvider,
		metric:      mockMetric,
		configer:    mockConfiger,
	}
	ctx := context.Background()

	baseMockEvaluator := &entity.Evaluator{
		ID:            100,
		SpaceID:       1,
		Name:          "Test Evaluator",
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:                100,
			EvaluatorID:       100,
			SpaceID:           1,
			PromptTemplateKey: "test-template-key",
			PromptSuffix:      "test-prompt-suffix",
			ModelConfig: &entity.ModelConfig{
				ModelID: 1,
			},
			ParseType: entity.ParseTypeFunctionCall,
			MessageList: []*entity.Message{
				{
					Role: entity.RoleSystem,
					Content: &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("{{test-content}}"),
					},
				},
			},
			Tools: []*entity.Tool{
				{
					Type: entity.ToolTypeFunction,
					Function: &entity.Function{
						Name:        "test_function",
						Description: "test description",
						Parameters:  "{\"type\": \"object\", \"properties\": {\"score\": {\"type\": \"number\"}, \"reasoning\": {\"type\": \"string\"}}}",
					},
				},
			},
		},
	}

	baseMockInput := &entity.EvaluatorInputData{
		InputFields: map[string]*entity.Content{
			"input": {
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of("test input"),
			},
		},
	}

	t.Run("成功调试评估器", func(t *testing.T) {
		mockLLMProvider.EXPECT().Call(gomock.Any(), gomock.Any()).Return(
			&entity.ReplyItem{
				ToolCalls: []*entity.ToolCall{
					{
						Type: entity.ToolTypeFunction,
						FunctionCall: &entity.FunctionCall{
							Name:      "test_function",
							Arguments: gptr.Of("{\"score\": 1.0, \"reason\": \"test response\"}"),
						},
					},
				},
				TokenUsage: &entity.TokenUsage{InputTokens: 10, OutputTokens: 10},
			}, nil)
		mockMetric.EXPECT().EmitRun(int64(1), gomock.Any(), gomock.Any(), gomock.Any())
		output, err := service.Debug(ctx, baseMockEvaluator, baseMockInput)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.NotNil(t, output.EvaluatorResult)
		assert.Equal(t, 1.0, *output.EvaluatorResult.Score)
		assert.Equal(t, "test response", output.EvaluatorResult.Reasoning)
	})

	t.Run("调试评估器失败", func(t *testing.T) {
		mockLLMProvider.EXPECT().Call(gomock.Any(), gomock.Any()).Return(nil, errors.New("llm call failed"))
		mockMetric.EXPECT().EmitRun(int64(1), gomock.Any(), gomock.Any(), gomock.Any())
		output, err := service.Debug(ctx, baseMockEvaluator, baseMockInput)
		assert.Error(t, err)
		assert.Nil(t, output)
	})
}

// TestEvaluatorSourcePromptServiceImpl_ComplexBusinessLogic 测试复杂业务逻辑
func TestEvaluatorSourcePromptServiceImpl_ComplexBusinessLogic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "复杂模板渲染测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				evaluatorVersion := &entity.PromptEvaluatorVersion{
					SpaceID: 123,
					MessageList: []*entity.Message{
						{
							Role: entity.RoleSystem,
							Content: &entity.Content{
								ContentType: gptr.Of(entity.ContentTypeMultipart),
								MultiPart: []*entity.Content{
									{
										ContentType: gptr.Of(entity.ContentTypeText),
										Text:        gptr.Of("请评估以下内容：{{content}}"),
									},
									{
										ContentType: gptr.Of(entity.ContentTypeMultipartVariable),
										Text:        gptr.Of("images"),
									},
									{
										ContentType: gptr.Of(entity.ContentTypeText),
										Text:        gptr.Of("评分标准：{{criteria}}"),
									},
								},
							},
						},
					},
					PromptSuffix: " 请提供详细分析。",
				}

				input := &entity.EvaluatorInputData{
					InputFields: map[string]*entity.Content{
						"content": {
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("这是一个测试文本"),
						},
						"criteria": {
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("准确性、完整性、清晰度"),
						},
						"images": {
							ContentType: gptr.Of(entity.ContentTypeMultipart),
							MultiPart: []*entity.Content{
								{
									ContentType: gptr.Of(entity.ContentTypeImage),
									Image: &entity.Image{
										URI: gptr.Of("image1.jpg"),
										URL: gptr.Of("https://example.com/image1.jpg"),
									},
								},
								{
									ContentType: gptr.Of(entity.ContentTypeImage),
									Image: &entity.Image{
										URI: gptr.Of("image2.jpg"),
										URL: gptr.Of("https://example.com/image2.jpg"),
									},
								},
							},
						},
					},
				}

				ctx := context.Background()
				err := renderTemplate(ctx, evaluatorVersion, input)

				assert.NoError(t, err)
				assert.Len(t, evaluatorVersion.MessageList, 1)

				multiPart := evaluatorVersion.MessageList[0].Content.MultiPart
				assert.Len(t, multiPart, 4) // 原来3个部分，images变量展开为2个图片

				// 验证文本替换
				assert.Equal(t, "请评估以下内容：这是一个测试文本", gptr.Indirect(multiPart[0].Text))
				assert.Equal(t, "评分标准：准确性、完整性、清晰度", gptr.Indirect(multiPart[3].Text))

				// 验证图片变量展开
				assert.Equal(t, entity.ContentTypeImage, gptr.Indirect(multiPart[1].ContentType))
				assert.Equal(t, entity.ContentTypeImage, gptr.Indirect(multiPart[2].ContentType))
				assert.Equal(t, "image1.jpg", gptr.Indirect(multiPart[1].Image.URI))
				assert.Equal(t, "image2.jpg", gptr.Indirect(multiPart[2].Image.URI))
			},
		},
		{
			name: "大数据量处理测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				// 测试处理大量输入字段
				largeInput := &entity.EvaluatorInputData{
					InputFields: make(map[string]*entity.Content),
				}

				// 创建1000个输入字段
				for i := 0; i < 1000; i++ {
					key := fmt.Sprintf("field_%d", i)
					largeInput.InputFields[key] = &entity.Content{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of(fmt.Sprintf("value_%d", i)),
					}
				}

				evaluatorVersion := &entity.PromptEvaluatorVersion{
					SpaceID: 123,
					MessageList: []*entity.Message{
						{
							Role: entity.RoleSystem,
							Content: &entity.Content{
								ContentType: gptr.Of(entity.ContentTypeText),
								Text:        gptr.Of("Process large data: {{field_0}} ... {{field_999}}"),
							},
						},
					},
					PromptSuffix: "",
				}

				ctx := context.Background()
				start := time.Now()
				err := renderTemplate(ctx, evaluatorVersion, largeInput)
				duration := time.Since(start)

				assert.NoError(t, err)
				assert.Less(t, duration, 1*time.Second) // 确保处理时间合理

				// 验证模板渲染结果
				expectedText := "Process large data: value_0 ... value_999"
				assert.Equal(t, expectedText, gptr.Indirect(evaluatorVersion.MessageList[0].Content.Text))
			},
		},
		{
			name: "边界条件测试",
			testFunc: func(t *testing.T) {
				t.Parallel()

				tests := []struct {
					name        string
					content     *entity.Content
					inputFields map[string]*entity.Content
					expectError bool
				}{
					{
						name:        "空内容",
						content:     nil,
						inputFields: map[string]*entity.Content{},
						expectError: false,
					},
					{
						name: "空文本",
						content: &entity.Content{
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of(""),
						},
						inputFields: map[string]*entity.Content{},
						expectError: false,
					},
					{
						name: "嵌套变量",
						content: &entity.Content{
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("{{var1}} contains {{var2}}"),
						},
						inputFields: map[string]*entity.Content{
							"var1": {
								ContentType: gptr.Of(entity.ContentTypeText),
								Text:        gptr.Of("{{var2}}"),
							},
							"var2": {
								ContentType: gptr.Of(entity.ContentTypeText),
								Text:        gptr.Of("nested value"),
							},
						},
						expectError: false,
					},
					{
						name: "循环引用",
						content: &entity.Content{
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("{{var1}}"),
						},
						inputFields: map[string]*entity.Content{
							"var1": {
								ContentType: gptr.Of(entity.ContentTypeText),
								Text:        gptr.Of("{{var2}}"),
							},
							"var2": {
								ContentType: gptr.Of(entity.ContentTypeText),
								Text:        gptr.Of("{{var1}}"),
							},
						},
						expectError: false, // 不会无限循环，只会替换一次
					},
				}

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						err := processMessageContent(tt.content, tt.inputFields)
						if tt.expectError {
							assert.Error(t, err)
						} else {
							assert.NoError(t, err)
						}
					})
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

func TestJSONRepair(t *testing.T) {
	t.Run("场景1: 非法JSON应能修复", func(t *testing.T) {
		json := `{name: 'John'}`
		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, `{"name": "John"}`, repaired)
	})

	t.Run("场景2: 合法JSON应原样返回", func(t *testing.T) {
		json := `{"name":"John"}`
		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, json, repaired)
	})

	t.Run("场景3: 完全不合法", func(t *testing.T) {
		json := `{name: John`
		referenceJson := `{"name": "John"}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, referenceJson, repaired)
	})

	t.Run("场景4: 空字符串应报错", func(t *testing.T) {
		json := ""
		repaired, err := jsonrepair.JSONRepair(json)
		assert.Error(t, err)
		assert.Empty(t, repaired)
	})

	t.Run("场景5: 部分修复但仍不合法应报错", func(t *testing.T) {
		json := `{name: 'John', age: }`
		referenceJson := `{"name": "John", "age": null}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, repaired, referenceJson)
	})

	t.Run("场景6: 嵌套对象修复", func(t *testing.T) {
		json := `{user: {name: 'John', age: 30}}`
		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, `{"user": {"name": "John", "age": 30}}`, repaired)
	})

	t.Run("场景7: 数组修复", func(t *testing.T) {
		json := `[{name: 'John'}, {name: 'Jane'}]`
		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, `[{"name": "John"}, {"name": "Jane"}]`, repaired)
	})

	t.Run("场景8: 混合修复", func(t *testing.T) {
		json := "```json\n{\n\"reason\": \"The output is a direct and necessary request for clarification, without any unnecessary elements. It adheres to the criteria by being concise and only seeking the required information.\",\n\"score\": 1\n}\n```"
		repaired, err := jsonrepair.JSONRepair(json)
		fmt.Println(repaired)
		fmt.Println(err)
	})

	t.Run("场景9: 空值修复", func(t *testing.T) {
		json := `{name: 'John', age: }`
		referenceJson := `{"name": "John", "age": null}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, repaired, referenceJson)
	})

	t.Run("场景10: 字符串值包含未转义双引号", func(t *testing.T) {
		json := `{name: 'John "The Coder" Doe'}`
		expected := `{"name": "John \"The Coder\" Doe"}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, expected, repaired)
	})

	t.Run("场景11: 多个字符串值包含双引号", func(t *testing.T) {
		json := `{name: 'John "Johnny" Doe', nickname: 'The "Master" Coder'}`
		expected := `{"name": "John \"Johnny\" Doe", "nickname": "The \"Master\" Coder"}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, expected, repaired)
	})

	t.Run("场景12: 嵌套对象字符串包含双引号", func(t *testing.T) {
		json := `{user: {name: 'John "The Great" Doe', title: 'Senior "Backend" Engineer'}}`
		expected := `{"user": {"name": "John \"The Great\" Doe", "title": "Senior \"Backend\" Engineer"}}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, expected, repaired)
	})

	t.Run("场景13: 数组字符串包含双引号", func(t *testing.T) {
		json := `[{name: 'John "Johnny" Doe'}, {name: 'Jane "Janie" Smith'}]`
		expected := `[{"name": "John \"Johnny\" Doe"}, {"name": "Jane \"Janie\" Smith"}]`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, expected, repaired)
	})

	t.Run("场景14: 字符串同时包含单双引号", func(t *testing.T) {
		json := `{message: 'He said "Hello, it\'s me!"'}`
		expected := `{"message": "He said \"Hello, it's me!\""}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, expected, repaired)
	})

	// 中文双引号相关测试用例
	t.Run("场景15: 字符串值包含中文双引号", func(t *testing.T) {
		json := `{name: '张三"程序员"李四'}`
		expected := `{"name": "张三\"程序员\"李四"}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, expected, repaired)
	})

	t.Run("场景16: 字符串值包含中文左右双引号", func(t *testing.T) {
		json := `{name: '张三"高级"程序员"}`
		expected := `{"name": "张三\"高级\"程序员\""}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, expected, repaired)
	})

	t.Run("场景17: 多个字符串值包含中文双引号", func(t *testing.T) {
		json := `{name: '张三"小明"王五', title: '高级"后端"工程师'}`
		expected := `{"name": "张三\"小明\"王五", "title": "高级\"后端\"工程师"}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, expected, repaired)
	})

	t.Run("场景18: 嵌套对象中文双引号", func(t *testing.T) {
		json := `{user: {name: '张三"大神"李四', position: '资深"架构师"'}}`
		expected := `{"user": {"name": "张三\"大神\"李四", "position": "资深\"架构师\""}}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, expected, repaired)
	})

	t.Run("场景19: 数组中文双引号", func(t *testing.T) {
		json := `[{name: '张三"小明"王五'}, {name: '李四"小红"赵六'}]`
		expected := `[{"name": "张三\"小明\"王五"}, {"name": "李四\"小红\"赵六"}]`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, expected, repaired)
	})

	t.Run("场景20: 混合中英文双引号", func(t *testing.T) {
		json := `{message: '他说"Hello"和"世界"'}`
		expected := `{"message": "他说\"Hello\"和\"世界\""}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, expected, repaired)
	})

	t.Run("场景21: 字符串内容包含中文双引号", func(t *testing.T) {
		json := `{content: '这是一个"测试"字符串，包含"中文"内容'}`
		expected := `{"content": "这是一个\"测试\"字符串，包含\"中文\"内容"}`

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, expected, repaired)
	})

	t.Run("场景22: 未转义双引号后出现数字", func(t *testing.T) {
		// Arrange: 准备一个reason字段包含转义字符的JSON
		content := `{
  "reason": "首句通过麦肯锡与自身咨询公司的鲜明对比，直击了咨询行业创业者的核心痛点（收费与客户量的巨大差距），使用了商业人士熟悉的行业对比表达方式，明确体现了咨询公司经营者的身份特征，激发了受众的好奇心和解决问题的紧迫感。包含了"麦肯锡"、"咨询公司"等筛选关键词，避免了泛泛而谈的通用开场白。但相比参考输出中"50万见面费都给不了，那就不是我的客户"这种更直接、更具筛选性的表达，原首句对高端客户的筛选精准度稍逊，且对非咨询行业的企业主吸引力较弱。",
  "score": 0.7
}`
		_, err := jsonrepair.JSONRepair(content)
		assert.NoError(t, err)
	})
}

func TestParseOutput_ParseTypeContent(t *testing.T) {
	t.Run("ParseTypeContent-正常修复", func(t *testing.T) {
		evaluatorVersion := &entity.PromptEvaluatorVersion{
			ParseType: entity.ParseTypeContent,
			SpaceID:   1,
			Tools: []*entity.Tool{
				{
					Function: &entity.Function{
						Parameters: "{\"type\": \"object\", \"properties\": {\"score\": {\"type\": \"number\"}, \"reason\": {\"type\": \"string\"}}}",
					},
				},
			},
		}
		replyItem := &entity.ReplyItem{
			Content:    gptr.Of("{score: 1.5, reason: 'good'}"),
			TokenUsage: &entity.TokenUsage{InputTokens: 5, OutputTokens: 6},
		}
		output, err := parseOutput(context.Background(), evaluatorVersion, replyItem)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.NotNil(t, output.EvaluatorResult)
		assert.Equal(t, 1.5, *output.EvaluatorResult.Score)
		assert.Equal(t, "good", output.EvaluatorResult.Reasoning)
		assert.Equal(t, int64(5), output.EvaluatorUsage.InputTokens)
		assert.Equal(t, int64(6), output.EvaluatorUsage.OutputTokens)
	})
}

func Test_parseContentOutput(t *testing.T) {
	// 公共测试设置
	ctx := context.Background()
	// evaluatorVersion 在被测函数中未被使用，可为空
	evaluatorVersion := &entity.PromptEvaluatorVersion{}

	t.Run("场景1: 内容是标准的JSON字符串", func(t *testing.T) {
		// Arrange: 准备一个标准的JSON字符串作为输入
		content := `{"score": 0.8, "reason": "This is a good reason."}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 断言无错误，并且输出被正确填充
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.8, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, "This is a good reason.", output.EvaluatorResult.Reasoning)
	})

	t.Run("场景2: JSON被包裹在Markdown代码块中", func(t *testing.T) {
		// Arrange: 准备一个被Markdown代码块包裹的JSON字符串
		content := "Some text before.\n```json\n{\"score\": 0.9, \"reason\": \"Another reason.\"}\n```\nSome text after."
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 断言函数能通过正则提取并解析JSON
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.9, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, "Another reason.", output.EvaluatorResult.Reasoning)
	})

	t.Run("场景3: score字段是字符串类型", func(t *testing.T) {
		// Arrange: 准备一个score字段为字符串的JSON
		content := `{"score": "0.75", "reason": "Reason with string score"}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 断言能够处理从字符串到浮点数的转换
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		expectedScore, err := strconv.ParseFloat("0.75", 64)
		assert.NoError(t, err)
		assert.InDelta(t, expectedScore, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, "Reason with string score", output.EvaluatorResult.Reasoning)
	})

	t.Run("场景4: 存在多个JSON块，第一个是有效的", func(t *testing.T) {
		// Arrange: 准备一个包含多个JSON的字符串，第一个即有效
		content := "First block: {\"score\": 1.0, \"reason\": \"First valid JSON\"}. Second block: {\"score\": 0.1, \"reason\": \"Second JSON\"}"
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 断言函数使用第一个有效的JSON并返回
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 1.0, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, "First valid JSON", output.EvaluatorResult.Reasoning)
	})

	t.Run("场景6: 内容中不包含有效的JSON", func(t *testing.T) {
		// Arrange: 准备一个不含JSON的普通字符串
		content := "This is just a plain string with no JSON."
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 由于有兜底策略，不会返回错误，但会使用兜底解析
		// 兜底策略：score设置为0，使用完整内容作为reason
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.0, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, content, output.EvaluatorResult.Reasoning)
	})

	t.Run("场景7: JSON中的score字段值不是数字", func(t *testing.T) {
		// Arrange: 准备一个score字段格式错误的JSON
		content := `{"score": "not-a-number", "reason": "bad score"}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 由于有兜底策略，不会返回错误，但会使用兜底解析
		// 兜底策略：score设置为0，使用完整内容作为reason
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.0, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, content, output.EvaluatorResult.Reasoning)
	})

	t.Run("场景8: 内容为空字符串", func(t *testing.T) {
		// Arrange: 准备一个空字符串
		content := ""
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 由于有兜底策略，不会返回错误，但会使用兜底解析
		// 兜底策略：score设置为0，使用完整内容作为reason
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.0, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, content, output.EvaluatorResult.Reasoning)
	})

	t.Run("场景9: JSON的reason字段中包含转义字符", func(t *testing.T) {
		// Arrange: 准备一个reason字段包含转义字符的JSON
		content := `{"score": 0.5, "reason": "This is a reason with a \"quote\" and a \\ backslash."}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 断言转义字符被正确解析
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.5, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, `This is a reason with a "quote" and a \ backslash.`, output.EvaluatorResult.Reasoning)
	})

	t.Run("场景10: reason在前", func(t *testing.T) {
		// Arrange: 准备一个reason字段包含转义字符的JSON
		content := `### 步骤1：图片理解描述清单
- 可识别对象：美国地图、标注的州（如CALIFORNIA、COLORADO、MINNESOTA、IOWA、PENNSYLVANIA）、标注的城市（如SAN DIEGO、ATLANTA、ORLANDO、CHICAGO）
- 场景：美国地图的室外地理场景
- 文字信息：标注的州名和城市名，如“CALIFORNIA”“SAN DIEGO”“MINNESOTA”“CHICAGO”“PENNSYLVANIA”“ATLANTA”“ORLANDO”
- 属性：各州用不同颜色标注，城市用圆点标注
- 空间关系：各城市和州在地图上的位置关系

### 步骤2：问题理解拆解清单
- 核心意图：找出标注城市中最北的那个
- 考察点：地理空间位置的比较
- 解答步骤：需要对比各标注城市在地图上的纬度位置，判断哪个最靠北

### 步骤3：再次图片理解
通过图片可知，MINNESOTA所在位置比CHICAGO更北，模型回答CHICAGO错误，信息不足支持正确判断

### 步骤4：回答评估
模型回答CHICAGO是错误的，因为MINNESOTA比CHICAGO更靠北，所以得分应为0.0
{
    "reason": "模型回答CHICAGO错误，实际上MINNESOTA所在位置比CHICAGO更北，回答不符合问题要求",
    "score": 0.0
}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 这个复杂的内容能够通过正则表达式提取JSON并解析成功
		// 成功原因：虽然内容包含大量Markdown文本，但末尾的JSON结构格式正确，正则表达式能够匹配并提取有效的JSON片段
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.0, *output.EvaluatorResult.Score, 0.0001)
		assert.Contains(t, output.EvaluatorResult.Reasoning, "MINNESOTA")

	})

	t.Run("场景11: 未转义双引号后出现数字", func(t *testing.T) {
		// Arrange: 准备一个reason字段包含转义字符的JSON
		content := `{
  "reason": "首句通过麦肯锡与自身咨询公司的鲜明对比，直击了咨询行业创业者的核心痛点（收费与客户量的巨大差距），使用了商业人士熟悉的行业对比表达方式，明确体现了咨询公司经营者的身份特征，激发了受众的好奇心和解决问题的紧迫感。包含了"麦肯锡"、"咨询公司"等筛选关键词，避免了泛泛而谈的通用开场白。但相比参考输出中"50万见面费都给不了，那就不是我的客户"这种更直接、更具筛选性的表达，原首句对高端客户的筛选精准度稍逊，且对非咨询行业的企业主吸引力较弱。",
  "score": 0.7
}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 这个内容包含未转义引号，但通过正则解析策略成功解析
		// 成功原因：虽然reason字段包含未转义的双引号导致JSON格式错误，但parseScoreWithRegex策略能够通过正则表达式提取score值，并使用完整内容作为reason
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.7, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, content, output.EvaluatorResult.Reasoning) // 使用完整内容作为reason

	})

	// 基于 CSV 失败记录添加的新测试场景
	t.Run("场景12: reason中包含未转义双引号", func(t *testing.T) {
		// 基于 CSV 第1行记录：包含 "thought" 等字段，但双引号未转义
		// 失败原因：字符串中包含未转义的双引号导致 JSON 解析失败
		content := `{"score": 0.7, "reason": "首句通过"麦肯锡"与自身咨询公司的对比，直击了咨询行业创业者的核心痛点"}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 这个JSON包含未转义的双引号，但jsonrepair能够成功修复
		// 成功原因：虽然reason字段包含未转义的双引号（"麦肯锡"），但jsonrepair.JSONRepair能够智能识别并修复这种简单的引号问题，将其转换为正确的转义格式
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.7, *output.EvaluatorResult.Score, 0.0001)
	})

	t.Run("场景13: 缺少必需字段的JSON", func(t *testing.T) {
		// 基于 CSV 第8-9行记录：地址信息的 JSON 对象，缺少 score 和 reason 字段
		// 失败原因：JSON 结构正确但缺少必需字段 score 或 reason
		content := `{"city": "上海市", "province": "上海市", "address": "申昆路2377号4幢"}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 由于有兜底策略，不会返回错误，但会使用兜底解析
		// 兜底策略：score设置为0，使用完整内容作为reason
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.0, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, content, output.EvaluatorResult.Reasoning)
	})

	t.Run("场景14: score字段为特殊值", func(t *testing.T) {
		// 基于 CSV 第52-57行记录：图像分析结果中 score 为 "无"
		// 失败原因：score 字段值为非数字字符串，无法转换为浮点数
		content := `{"reason": "图中无文字", "score": "无"}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 由于有兜底策略，不会返回错误，但会使用兜底解析
		// 兜底策略：score设置为0，使用完整内容作为reason
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.0, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, content, output.EvaluatorResult.Reasoning)
	})

	t.Run("场景15: 复杂嵌套JSON结构", func(t *testing.T) {
		// 基于 CSV 第11-15行记录：包含嵌套结构的评分结果
		// 实际测试发现这个JSON能够通过正则表达式提取嵌套的score和reason
		content := `{
			"1.5模型": {
				"reason": "输出准确完整",
				"score": 1.0
			}
		}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 正则表达式能够提取嵌套JSON中的score和reason
		// 成功原因：正则表达式能够匹配嵌套JSON结构中的score和reason字段
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 1.0, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, "输出准确完整", output.EvaluatorResult.Reasoning)
	})

	t.Run("场景16: 超长reason文本", func(t *testing.T) {
		// 基于 CSV 第10行记录：包含详细分析过程的长文本
		// 测试函数对超长 reason 文本的处理能力
		longReason := "要解决AI助手回复是否正确的问题，需**对比AI回复与专家标准答案的核心要点覆盖情况**：### **1. 明确专家标准答案的核心要点** 专家给出的解决路径共6点：① 与机构协商解决；② 向主管部门投诉；③ 申请第三方调解；④ 寻求法律援助；⑤ 媒体曝光；⑥ 更换服务机构。### **2. 分析AI助手回复的内容** 根据输入，**AI助手回复的答案为空**（即未提供任何解决措施）。"
		content := fmt.Sprintf(`{"score": 0.8, "reason": "%s"}`, longReason)
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 预期解析成功，处理长文本
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.8, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, longReason, output.EvaluatorResult.Reasoning)
	})

	t.Run("场景17: Markdown格式混合JSON", func(t *testing.T) {
		// 基于 CSV 记录中包含步骤分析的格式
		// 测试从复杂文本中提取 JSON 的能力
		content := `### 步骤1：分析
		详细分析过程...
		
		### 步骤2：评估结果
		{"reason": "分析结果显示模型回答准确", "score": 0.9}
		
		### 步骤3：总结
		综合评估完成`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 这个Markdown格式的内容能够通过正则表达式提取JSON并解析成功
		// 成功原因：虽然内容包含Markdown格式文本，但中间的JSON片段格式正确，正则表达式能够匹配并提取有效的JSON结构
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.9, *output.EvaluatorResult.Score, 0.0001)
		assert.Contains(t, output.EvaluatorResult.Reasoning, "分析结果显示模型回答准确")
	})

	t.Run("场景18: 纯文本无JSON结构", func(t *testing.T) {
		// 基于 CSV 第58-62行记录：技术文档和说明
		// 失败原因：纯文本内容，无 JSON 结构
		content := `### 步骤解释
		1. **发布模式定义**：发布模式是正常启动服务的模式，不支持热部署和单步调试，属于稳定运行服务。
		2. **命令示例**：通过设置RUN_MODE=release并执行docker compose up --build命令来启动服务。`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 由于有兜底策略，不会返回错误，但会使用兜底解析
		// 兜底策略：score设置为0，使用完整内容作为reason
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.0, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, content, output.EvaluatorResult.Reasoning)
	})

	t.Run("场景19: JSON结构不完整", func(t *testing.T) {
		// 模拟 CSV 中结构不完整的情况
		// 失败原因：JSON 缺少闭合括号或格式错误
		content := `{"score": 0.6, "reason": "评估结果` // 缺少闭合引号和括号
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: JSON结构不完整，但jsonrepair能够修复
		// 成功原因：虽然JSON缺少闭合引号和括号，但jsonrepair.JSONRepair能够智能补全缺失的语法元素，将不完整的JSON修复为有效格式
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.6, *output.EvaluatorResult.Score, 0.0001)
	})

	t.Run("场景20: 多层嵌套的评分分析", func(t *testing.T) {
		// 基于 CSV 第63-77行记录：复杂嵌套的评分分析
		// 实际测试发现正则表达式会匹配第一个找到的score和reason
		content := `{
			"1.5模型评估": {
				"reason": "模型输出准确",
				"score": 0.8
			},
			"1.6模型评估": {
				"reason": "模型输出更准确",
				"score": 1.0
			}
		}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 正则表达式会提取第一个匹配到的score和reason
		// 成功原因：正则表达式能够匹配嵌套JSON中的第一个score和reason字段
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.8, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, "模型输出准确", output.EvaluatorResult.Reasoning)
	})

	// 新增测试场景：复杂JSON字符串解析
	t.Run("场景21: 用户提供的复杂JSON字符串", func(t *testing.T) {
		// 用户提供的实际复杂JSON字符串，包含SQL语句、复杂转义、多层引号嵌套
		content := `{"reason": "用户问题要求查询前端集群\"tos-1az-front-azb-2\"主要访问的BS池，核心意图是筛选该集群的日志并按BS池字段分组统计访问次数。\n\n1. **筛选条件**：SQL使用` + "`" + `Cluster: \"\"tos-1az-front-azb-2\"\"` + "`" + `精准匹配用户指定的前端集群，符合筛选意图。\n2. **分析目标**：通过` + "`" + `SELECT \"\"LastAccessBsPoolName\"\", COUNT(*) AS access_count GROUP BY \"\"LastAccessBsPoolName\"\"` + "`" + `实现按BS池分组统计访问次数，与\"主要访问哪些BS池\"的分析需求一致。\n3. **字段依赖**：使用\"LastAccessBsPoolName\"字段对应BS池信息，假设该字段存在（用户问题未提及字段缺失，分析内容中已使用该字段）。\n\n**校验维度检查**：\n- 条件完整性：无遗漏筛选条件（仅需集群筛选）。\n- 条件准确性：集群名称匹配，无矛盾。\n- 分析目标一致性：GROUP BY+COUNT(*)符合统计访问次数需求。\n- 函数正确性：COUNT(*)参数类型正确。\n- 语法规范：字段\"LastAccessBsPoolName\"用双引号包裹（含大写字母，符合特殊字符处理规则），别名access_count合法，逻辑正确。\n\n**特殊场景**：用户未要求时间范围，SQL未包含时间筛选，符合默认规则；无歧义或未定义字段问题。", "score": 1.0}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 这个复杂的JSON字符串应该能够被成功解析
		// 成功原因：虽然包含复杂的转义字符、SQL语句和多层引号嵌套，但JSON结构本身是正确的，
		// parseDirectJSON策略能够直接解析成功
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 1.0, *output.EvaluatorResult.Score, 0.0001)
		assert.Contains(t, output.EvaluatorResult.Reasoning, "用户问题要求查询前端集群")
		assert.Contains(t, output.EvaluatorResult.Reasoning, "SQL使用")
		assert.Contains(t, output.EvaluatorResult.Reasoning, "LastAccessBsPoolName")
	})

	t.Run("场景22: 复杂SQL语句嵌入JSON", func(t *testing.T) {
		// 测试reason字段包含完整SQL语句和复杂转义的情况
		content := `{"score": 0.85, "reason": "查询语句：SELECT \"table_name\", COUNT(*) FROM database WHERE condition = 'value' AND status IN ('active', 'pending') GROUP BY \"table_name\" ORDER BY COUNT(*) DESC; 该SQL语句结构正确，使用了适当的引号转义。"}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: SQL语句嵌入的JSON应该能够被正确解析
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.85, *output.EvaluatorResult.Score, 0.0001)
		assert.Contains(t, output.EvaluatorResult.Reasoning, "SELECT")
		assert.Contains(t, output.EvaluatorResult.Reasoning, "GROUP BY")
	})

	t.Run("场景23: 多层引号嵌套复杂情况", func(t *testing.T) {
		// 测试reason字段包含多层嵌套引号的复杂情况
		content := `{"score": 0.9, "reason": "分析结果显示：字段\"name\"的值为\"John \\\"The Great\\\" Doe\"，其中包含转义的双引号。同时，配置项\"config\":{\"key\":\"value\"}也被正确解析。"}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 多层引号嵌套应该能够被正确解析
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.9, *output.EvaluatorResult.Score, 0.0001)
		assert.Contains(t, output.EvaluatorResult.Reasoning, "John \\\"The Great\\\" Doe") // 转义后的格式
		assert.Contains(t, output.EvaluatorResult.Reasoning, "config")
	})

	t.Run("场景24: Markdown格式文本嵌入", func(t *testing.T) {
		// 测试reason字段包含Markdown格式文本的解析
		content := `{"score": 0.75, "reason": "## 分析结果\n\n### 1. 代码质量\n- **优点**：代码结构清晰\n- **缺点**：缺少注释\n\n### 2. 性能评估\n使用` + "`" + `SELECT * FROM table` + "`" + `查询可能存在性能问题。\n\n**总结**：整体质量良好，但需要优化。"}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: Markdown格式文本应该能够被正确解析
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.75, *output.EvaluatorResult.Score, 0.0001)
		assert.Contains(t, output.EvaluatorResult.Reasoning, "## 分析结果")
		assert.Contains(t, output.EvaluatorResult.Reasoning, "**优点**")
		assert.Contains(t, output.EvaluatorResult.Reasoning, "SELECT * FROM table")
	})

	t.Run("场景25: 极长复杂文本性能测试", func(t *testing.T) {
		// 构造类似用户提供JSON的超长复杂文本，测试解析性能
		longReason := strings.Repeat("这是一个包含\"引号\"和复杂内容的长文本段落。", 100) +
			"SQL查询：SELECT \"field1\", \"field2\" FROM \"table\" WHERE \"condition\" = 'value'。" +
			strings.Repeat("更多复杂内容包含各种特殊字符：@#$%^&*()_+{}|:<>?[]\\;'\".,/", 50)
		
		content := fmt.Sprintf(`{"score": 0.95, "reason": "%s"}`, strings.ReplaceAll(longReason, `"`, `\"`))
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// 测试解析性能
		start := time.Now()
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)
		duration := time.Since(start)

		// Assert: 超长文本应该能够被正确解析，且性能合理
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.95, *output.EvaluatorResult.Score, 0.0001)
		assert.Contains(t, output.EvaluatorResult.Reasoning, "SQL查询")
		assert.Less(t, duration, 100*time.Millisecond) // 确保解析时间在合理范围内
	})

	t.Run("场景26: 特殊字符组合测试", func(t *testing.T) {
		// 测试各种特殊字符组合的解析能力
		content := `{"score": 0.8, "reason": "测试内容包含各种特殊字符：\n换行符、\t制表符、\"双引号\"、'单引号'、\\反斜杠、/正斜杠、@邮箱符号、#井号、$美元符号、%百分号、^异或、&与符号、*星号、()括号、[]方括号、{}花括号、|管道符、:冒号、;分号、<>尖括号、?问号、中文字符、数字123、emoji😀等。"}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 特殊字符组合应该能够被正确解析
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.8, *output.EvaluatorResult.Score, 0.0001)
		assert.Contains(t, output.EvaluatorResult.Reasoning, "换行符")
		assert.Contains(t, output.EvaluatorResult.Reasoning, "双引号")
		assert.Contains(t, output.EvaluatorResult.Reasoning, "emoji😀")
	})

	t.Run("场景27: 复杂转义失败场景", func(t *testing.T) {
		// 测试复杂转义字符导致解析失败的场景
		content := `{"score": 0.6, "reason": "这是一个包含未正确转义的"引号"和其他"复杂内容"的文本，可能导致JSON解析失败。包含SQL：SELECT "field" FROM "table" WHERE "condition" = "value"。"}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 这种未正确转义的情况应该被jsonrepair修复后成功解析
		// 成功原因：虽然reason字段包含未转义的双引号，但jsonrepair.JSONRepair能够智能识别并修复这些转义问题
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.6, *output.EvaluatorResult.Score, 0.0001)
	})

	t.Run("场景28: 边界情况-空reason但有score", func(t *testing.T) {
		// 测试reason为空但score存在的边界情况
		content := `{"score": 1.0, "reason": ""}`
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// Act: 调用被测函数
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)

		// Assert: 正则解析策略能够提取score，使用完整内容作为reason
		// 成功原因：parseScoreWithRegex策略能够通过正则表达式提取score值，并使用完整内容作为reason
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 1.0, *output.EvaluatorResult.Score, 0.0001)
		assert.Equal(t, content, output.EvaluatorResult.Reasoning)
	})

	t.Run("场景29: 性能边界测试-超大JSON", func(t *testing.T) {
		// 测试超大JSON的解析性能和稳定性
		hugeReason := strings.Repeat("这是一个非常长的文本内容，用于测试解析器的性能边界。", 1000)
		content := fmt.Sprintf(`{"score": 0.5, "reason": "%s"}`, strings.ReplaceAll(hugeReason, `"`, `\"`))
		replyItem := &entity.ReplyItem{Content: &content}
		output := &entity.EvaluatorOutputData{
			EvaluatorResult: &entity.EvaluatorResult{},
		}

		// 测试解析性能
		start := time.Now()
		err := parseContentOutput(ctx, evaluatorVersion, replyItem, output)
		duration := time.Since(start)

		// Assert: 超大JSON应该能够被正确解析，且性能在可接受范围内
		assert.NoError(t, err)
		assert.NotNil(t, output.EvaluatorResult.Score)
		assert.InDelta(t, 0.5, *output.EvaluatorResult.Score, 0.0001)
		assert.Less(t, duration, 500*time.Millisecond) // 允许更长的解析时间用于超大内容
		assert.Greater(t, len(output.EvaluatorResult.Reasoning), 10000) // 确保长文本被完整保留
	})
}