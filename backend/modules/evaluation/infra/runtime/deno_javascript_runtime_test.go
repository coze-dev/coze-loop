// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestNewDenoJavaScriptRuntimeAdapter(t *testing.T) {
	logger := logrus.New()
	config := DefaultSandboxConfig()

	adapter, err := NewDenoJavaScriptRuntimeAdapter(config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)
	assert.Equal(t, entity.LanguageTypeJS, adapter.GetLanguageType())

	// 测试使用nil配置
	adapterWithNilConfig, err := NewDenoJavaScriptRuntimeAdapter(nil, logger)
	assert.NoError(t, err)
	assert.NotNil(t, adapterWithNilConfig)
}

func TestDenoJavaScriptRuntimeAdapter_RunCode(t *testing.T) {
	logger := logrus.New()
	config := DefaultSandboxConfig()
	adapter, err := NewDenoJavaScriptRuntimeAdapter(config, logger)
	assert.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name        string
		code        string
		language    string
		timeoutMS   int64
		expectError bool
	}{
		{
			name:        "正常JavaScript代码",
			code:        "console.log('Hello World');",
			language:    "javascript",
			timeoutMS:   5000,
			expectError: false,
		},
		{
			name:        "空代码",
			code:        "",
			language:    "javascript",
			timeoutMS:   5000,
			expectError: true,
		},
		{
			name:        "使用默认超时",
			code:        "console.log('Test');",
			language:    "javascript",
			timeoutMS:   0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.RunCode(ctx, tt.code, tt.language, tt.timeoutMS)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotNil(t, result.Output)
				assert.NotNil(t, result.WorkloadInfo)
				assert.Equal(t, "success", result.WorkloadInfo.Status)
			}
		})
	}
}

func TestDenoJavaScriptRuntimeAdapter_ValidateCode(t *testing.T) {
	logger := logrus.New()
	config := DefaultSandboxConfig()
	adapter, err := NewDenoJavaScriptRuntimeAdapter(config, logger)
	assert.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name     string
		code     string
		language string
		expected bool
	}{
		{
			name:     "有效的JavaScript代码",
			code:     "console.log('Hello World');",
			language: "javascript",
			expected: true,
		},
		{
			name:     "有效的TypeScript代码",
			code:     "const x: number = 42; console.log(x);",
			language: "typescript",
			expected: true,
		},
		{
			name:     "无效的JavaScript代码",
			code:     "console.log('unclosed string",
			language: "javascript",
			expected: false,
		},
		{
			name:     "空代码",
			code:     "",
			language: "javascript",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.ValidateCode(ctx, tt.code, tt.language)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDenoJavaScriptRuntimeAdapter_Cleanup(t *testing.T) {
	logger := logrus.New()
	config := DefaultSandboxConfig()
	adapter, err := NewDenoJavaScriptRuntimeAdapter(config, logger)
	assert.NoError(t, err)

	err = adapter.Cleanup()
	assert.NoError(t, err)
}