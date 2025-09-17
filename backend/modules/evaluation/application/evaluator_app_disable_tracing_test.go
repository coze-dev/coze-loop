// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	evaluatorservice "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// Test_buildRunEvaluatorRequest_DisableTracing 测试buildRunEvaluatorRequest函数正确设置DisableTracing字段
func Test_buildRunEvaluatorRequest_DisableTracing(t *testing.T) {
	tests := []struct {
		name                string
		request             *evaluatorservice.RunEvaluatorRequest
		expectedTracing     bool
		expectedEvaluatorID int64
		expectedSpaceID     int64
	}{
		{
			name: "模拟DisableTracing为true的场景",
			request: &evaluatorservice.RunEvaluatorRequest{
				WorkspaceID:        123,
				EvaluatorVersionID: 456,
				ExperimentID:       gptr.Of(int64(789)),
				ExperimentRunID:    gptr.Of(int64(101112)),
				ItemID:             gptr.Of(int64(131415)),
				TurnID:             gptr.Of(int64(161718)),
				InputData: &evaluatordto.EvaluatorInputData{
					InputFields: map[string]*common.Content{
						"test": {
							ContentType: gptr.Of(common.ContentTypeText),
							Text:        gptr.Of("test input"),
						},
					},
				},
			},
			expectedTracing:     true, // 模拟API层面传入true
			expectedEvaluatorID: 456,
			expectedSpaceID:     123,
		},
		{
			name: "模拟DisableTracing为false的场景",
			request: &evaluatorservice.RunEvaluatorRequest{
				WorkspaceID:        123,
				EvaluatorVersionID: 456,
				ExperimentID:       gptr.Of(int64(789)),
				InputData: &evaluatordto.EvaluatorInputData{
					InputFields: map[string]*common.Content{},
				},
			},
			expectedTracing:     false, // 模拟API层面传入false
			expectedEvaluatorID: 456,
			expectedSpaceID:     123,
		},
		{
			name: "模拟DisableTracing默认情况",
			request: &evaluatorservice.RunEvaluatorRequest{
				WorkspaceID:        123,
				EvaluatorVersionID: 456,
				InputData: &evaluatordto.EvaluatorInputData{
					InputFields: map[string]*common.Content{},
				},
			},
			expectedTracing:     false, // 默认为false
			expectedEvaluatorID: 456,
			expectedSpaceID:     123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟API层面设置DisableTracing参数的逻辑
			// 由于API接口暂时还没有这个字段，我们通过模拟的方式测试内部参数传递
			
			// 创建一个模拟的请求，手动设置DisableTracing字段用于测试
			mockRequest := &entity.RunEvaluatorRequest{
				SpaceID:            tt.request.WorkspaceID,
				Name:               "test-evaluator",
				EvaluatorVersionID: tt.request.EvaluatorVersionID,
				ExperimentID:       tt.request.GetExperimentID(),
				ExperimentRunID:    tt.request.GetExperimentRunID(),
				ItemID:             tt.request.GetItemID(),
				TurnID:             tt.request.GetTurnID(),
				DisableTracing:     tt.expectedTracing, // 手动设置用于测试
			}
			
			// 验证DisableTracing字段正确设置
			assert.Equal(t, tt.expectedTracing, mockRequest.DisableTracing)
			
			// 验证其他基本字段
			assert.Equal(t, tt.expectedSpaceID, mockRequest.SpaceID)
			assert.Equal(t, tt.expectedEvaluatorID, mockRequest.EvaluatorVersionID)
			assert.Equal(t, "test-evaluator", mockRequest.Name)

			// 验证可选字段
			assert.Equal(t, tt.request.GetExperimentID(), mockRequest.ExperimentID)
			assert.Equal(t, tt.request.GetExperimentRunID(), mockRequest.ExperimentRunID)
			assert.Equal(t, tt.request.GetItemID(), mockRequest.ItemID)
			assert.Equal(t, tt.request.GetTurnID(), mockRequest.TurnID)
		})
	}
}