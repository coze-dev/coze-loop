// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestSimpleStatusError_ErrorAndCode(t *testing.T) {
	t.Parallel()

	err := &SimpleStatusError{
		code: 123,
		msg:  "test message",
	}

	assert.Equal(t, "test message", err.Error())
	assert.Equal(t, int32(123), err.Code())
}

func TestSanitizeErrorForTrace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		inputErr       error
		expectedCode   int32
		expectedErrStr string
	}{
		{
			name:           "nil error",
			inputErr:       nil,
			expectedCode:   0,
			expectedErrStr: "",
		},
		{
			name:           "plain error without status code",
			inputErr:       errors.New("plain error"),
			expectedCode:   0,
			expectedErrStr: "plain error",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := SanitizeErrorForTrace(tt.inputErr)

			if tt.inputErr == nil {
				assert.Nil(t, result)
				return
			}

			// 返回的一定是 *SimpleStatusError
			sanitized, ok := result.(*SimpleStatusError)
			assert.True(t, ok, "result should be *SimpleStatusError")

			assert.Equal(t, tt.expectedCode, sanitized.code)
			assert.Equal(t, tt.expectedErrStr, sanitized.msg)
			// Error() 也应该返回相同的字符串
			assert.Equal(t, tt.expectedErrStr, sanitized.Error())
		})
	}

	// 单独测试 status error，因为需要动态计算期望值
	t.Run("status error from errorx with code and message", func(t *testing.T) {
		t.Parallel()

		inputErr := errorx.NewByCode(
			errno.InvalidInputDataCode,
			errorx.WithExtraMsg("invalid input"),
		)

		// 获取期望的字符串：通过 FromStatusError 获取 statusError，然后调用其 Error() 方法
		statusErr, ok := errorx.FromStatusError(inputErr)
		assert.True(t, ok, "inputErr should be a StatusError")
		expectedErrStr := statusErr.Error()
		expectedCode := int32(errno.InvalidInputDataCode)

		result := SanitizeErrorForTrace(inputErr)

		// 返回的一定是 *SimpleStatusError
		sanitized, ok := result.(*SimpleStatusError)
		assert.True(t, ok, "result should be *SimpleStatusError")

		assert.Equal(t, expectedCode, sanitized.code)
		assert.Equal(t, expectedErrStr, sanitized.msg)
		// Error() 也应该返回相同的字符串
		assert.Equal(t, expectedErrStr, sanitized.Error())
	})
}
