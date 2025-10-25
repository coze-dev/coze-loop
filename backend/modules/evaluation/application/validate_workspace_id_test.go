// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestEvaluatorHandlerImpl_validateWorkspaceID(t *testing.T) {
	tests := []struct {
		name           string
		workspaceID    int64
		allowedSpaces  []string
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name:          "valid workspace ID in allowed list",
			workspaceID:   7565071389755228204,
			allowedSpaces: []string{"7565071389755228204", "1234567890123456789"},
			expectedError: false,
		},
		{
			name:          "invalid workspace ID not in allowed list",
			workspaceID:   999999999999999999,
			allowedSpaces: []string{"7565071389755228204", "1234567890123456789"},
			expectedError: true,
			expectedErrMsg: "workspace_id not in allowed evaluator template spaces",
		},
		{
			name:          "empty allowed spaces list",
			workspaceID:   7565071389755228204,
			allowedSpaces: []string{},
			expectedError: true,
			expectedErrMsg: "evaluator template space not configured",
		},
		{
			name:          "nil allowed spaces list",
			workspaceID:   7565071389755228204,
			allowedSpaces: nil,
			expectedError: true,
			expectedErrMsg: "evaluator template space not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建mock controller
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// 创建mock configer
			mockConfiger := mocks.NewMockIConfiger(ctrl)
			mockConfiger.EXPECT().GetEvaluatorTemplateSpaceConf(gomock.Any()).Return(tt.allowedSpaces)

			// 创建EvaluatorHandlerImpl实例
			handler := &EvaluatorHandlerImpl{
				configer: mockConfiger,
			}

			// 调用校验方法
			err := handler.validateWorkspaceID(context.Background(), tt.workspaceID)

			// 验证结果
			if tt.expectedError {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
				// 验证错误码
				if statusErr, ok := errorx.FromStatusError(err); ok {
					assert.Equal(t, int32(errno.CommonInvalidParamCode), statusErr.Code())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
