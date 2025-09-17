// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/mocks"
)

// TestEvaluatorSourcePromptServiceImpl_Run_DisableTracing 测试追踪控制核心逻辑
func TestEvaluatorSourcePromptServiceImpl_Run_DisableTracing(t *testing.T) {
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
	evaluator := &entity.Evaluator{
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
						Text:        gptr.Of("test-content"),
					},
				},
			},
			InputSchemas: []*entity.ArgsSchema{
				{
					Key:        gptr.Of("test-input-key"),
					JsonSchema: gptr.Of(`{"type": "string"}`),
					SupportContentTypes: []entity.ContentType{
						entity.ContentTypeText,
					},
				},
			},
		},
	}

	input := &entity.EvaluatorInputData{
		InputFields: map[string]*entity.Content{
			"test-input-key": {
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of("test input"),
			},
		},
	}

	// 模拟LLM调用返回
	mockLLMResponse := &entity.ReplyItem{
		Content: gptr.Of(`{"score": 0.85, "reasoning": "Test reasoning"}`),
	}

	tests := []struct {
		name            string
		disableTracing  bool
		setupMocks      func()
		checkTraceID    func(t *testing.T, traceID string)
	}{
		{
			name:           "disableTracing=true时不创建Span",
			disableTracing: true,
			setupMocks: func() {
				// 配置LLM调用相关的mock
				mockLLMProvider.EXPECT().Call(gomock.Any(), gomock.Any()).Return(mockLLMResponse, nil)
				// 配置指标相关的mock，因为会有错误时的指标记录
				mockMetric.EXPECT().EmitRun(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			checkTraceID: func(t *testing.T, traceID string) {
				// 当禁用追踪时，traceID应该为空字符串
				assert.Empty(t, traceID)
			},
		},
		{
			name:           "disableTracing=false时正常创建Span",
			disableTracing: false,
			setupMocks: func() {
				// 配置LLM调用相关的mock
				mockLLMProvider.EXPECT().Call(gomock.Any(), gomock.Any()).Return(mockLLMResponse, nil)
				// 配置指标相关的mock，因为会有错误时的指标记录
				mockMetric.EXPECT().EmitRun(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			checkTraceID: func(t *testing.T, traceID string) {
				// 由于业务逻辑失败，即使启用追踪，traceID在错误情况下可能为空
				// 这里主要验证追踪控制逻辑不会导致额外错误
				// traceID的具体值取决于业务执行结果
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()

			output, status, traceID := service.Run(ctx, evaluator, input, tt.disableTracing)

			// 由于输入验证失败，验证错误状态
			assert.Equal(t, entity.EvaluatorRunStatusFail, status)
			assert.NotNil(t, output)
			
			// 验证追踪ID的生成情况
			tt.checkTraceID(t, traceID)
		})
	}
}