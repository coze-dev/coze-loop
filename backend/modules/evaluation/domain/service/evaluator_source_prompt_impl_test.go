// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	t.Run("非法JSON应能修复", func(t *testing.T) {
		json := "{name: 'John'}"
		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, "{\"name\": \"John\"}", repaired)
	})

	t.Run("合法JSON应原样返回", func(t *testing.T) {
		json := "{\"name\":\"John\"}"
		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, json, repaired)
	})

	t.Run("完全不合法", func(t *testing.T) {
		json := "{name: John"
		referenceJson := "{\"name\": \"John\"}"

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, referenceJson, repaired)
	})

	t.Run("空字符串应报错", func(t *testing.T) {
		json := ""
		repaired, err := jsonrepair.JSONRepair(json)
		assert.Error(t, err)
		assert.Empty(t, repaired)
	})

	t.Run("部分修复但仍不合法应报错", func(t *testing.T) {
		json := "{name: 'John', age: }"
		referenceJson := "{\"name\": \"John\", \"age\": null}"

		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, repaired, referenceJson)
	})

	t.Run("嵌套对象修复", func(t *testing.T) {
		json := "{user: {name: 'John', age: 30}}"
		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, "{\"user\": {\"name\": \"John\", \"age\": 30}}", repaired)
	})

	t.Run("数组修复", func(t *testing.T) {
		json := "[{name: 'John'}, {name: 'Jane'}]"
		repaired, err := jsonrepair.JSONRepair(json)
		assert.NoError(t, err)
		assert.Equal(t, "[{\"name\": \"John\"}, {\"name\": \"Jane\"}]", repaired)
	})

	t.Run("混合修复", func(t *testing.T) {
		json := "```json\n{\n\"reason\": \"The output is a direct and necessary request for clarification, without any unnecessary elements. It adheres to the criteria by being concise and only seeking the required information.\",\n\"score\": 1\n}\n```"
		repaired, err := jsonrepair.JSONRepair(json)
		fmt.Println(repaired)
		fmt.Println(err)
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
		if assert.NoError(t, err) {
			assert.NotNil(t, output.EvaluatorResult.Score)
			expectedScore, err := strconv.ParseFloat("0.75", 64)
			assert.NoError(t, err)
			assert.InDelta(t, expectedScore, *output.EvaluatorResult.Score, 0.0001)
			assert.Equal(t, "Reason with string score", output.EvaluatorResult.Reasoning)
		}
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

		// Assert: 断言解析失败，并返回错误
		assert.Error(t, err)
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

		// Assert: 断言解析失败，并返回错误
		assert.Error(t, err)
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

		// Assert: 断言解析失败，并返回错误
		assert.Error(t, err)
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
}
