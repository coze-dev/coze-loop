// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/mocks"
)

func TestGetEvaluatorTemplateSpaceConf(t *testing.T) {
	tests := []struct {
		name           string
		configData     map[string]interface{}
		expectedResult []string
	}{
		{
			name: "valid config with space IDs",
			configData: map[string]interface{}{
				"evaluator_template_space": map[string]interface{}{
					"evaluator_template_space": []string{"7565071389755228204", "1234567890123456789"},
				},
			},
			expectedResult: []string{"7565071389755228204", "1234567890123456789"},
		},
		{
			name: "empty config",
			configData: map[string]interface{}{
				"evaluator_template_space": map[string]interface{}{
					"evaluator_template_space": []string{},
				},
			},
			expectedResult: []string{},
		},
		{
			name:           "missing config",
			configData:     map[string]interface{}{},
			expectedResult: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		// 创建mock configer
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockConfiger := mocks.NewMockIConfiger(ctrl)

		// 设置mock期望
		mockConfiger.EXPECT().GetEvaluatorTemplateSpaceConf(gomock.Any()).Return(tt.expectedResult)

			// 调用方法
			result := mockConfiger.GetEvaluatorTemplateSpaceConf(context.Background())

			// 验证结果
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestDefaultEvaluatorTemplateSpaceConf(t *testing.T) {
	result := DefaultEvaluatorTemplateSpaceConf()
	assert.NotNil(t, result)
	assert.Equal(t, 0, len(result))
}
