// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestHTTPFaaSRuntimeAdapter(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// 创建HTTP FaaS运行时适配器
	config := &HTTPFaaSRuntimeConfig{
		BaseURL:        "http://localhost:8890", // 使用测试端口
		Timeout:        30 * time.Second,
		MaxRetries:     1, // 减少重试次数以加快测试
		RetryInterval:  100 * time.Millisecond,
		EnableEnhanced: true,
	}

	adapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)

	t.Run("GetLanguageType", func(t *testing.T) {
		langType := adapter.GetLanguageType()
		assert.Equal(t, entity.LanguageTypeJS, langType)
	})

	t.Run("ValidateCode", func(t *testing.T) {
		ctx := context.Background()

		// 测试有效的JavaScript代码
		valid := adapter.ValidateCode(ctx, "console.log('hello');", "javascript")
		assert.True(t, valid)

		// 测试无效的代码（括号不匹配）
		invalid := adapter.ValidateCode(ctx, "console.log('hello';", "javascript")
		assert.False(t, invalid)

		// 测试空代码
		empty := adapter.ValidateCode(ctx, "", "javascript")
		assert.False(t, empty)
	})

	t.Run("Cleanup", func(t *testing.T) {
		err := adapter.Cleanup()
		assert.NoError(t, err)
	})
}

func TestHTTPFaaSRuntimeAdapter_BasicValidation(t *testing.T) {
	logger := logrus.New()
	config := &HTTPFaaSRuntimeConfig{
		BaseURL:        "http://localhost:8890",
		Timeout:        30 * time.Second,
		MaxRetries:     1,
		RetryInterval:  100 * time.Millisecond,
		EnableEnhanced: true,
	}

	adapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, config, logger)
	assert.NoError(t, err)

	tests := []struct {
		name     string
		code     string
		language string
		expected bool
	}{
		{
			name:     "Valid JavaScript",
			code:     "function test() { return 42; }",
			language: "javascript",
			expected: true,
		},
		{
			name:     "Valid TypeScript",
			code:     "const x: number = 42;",
			language: "typescript",
			expected: true,
		},
		{
			name:     "Valid Python",
			code:     "def test(): return 42",
			language: "python",
			expected: true,
		},
		{
			name:     "Invalid brackets",
			code:     "function test() { return 42;",
			language: "javascript",
			expected: false,
		},
		{
			name:     "Invalid parentheses",
			code:     "console.log('hello';",
			language: "javascript",
			expected: false,
		},
		{
			name:     "Empty code",
			code:     "",
			language: "javascript",
			expected: false,
		},
		{
			name:     "Unsupported language",
			code:     "valid code",
			language: "java",
			expected: false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.ValidateCode(ctx, tt.code, tt.language)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHTTPFaaSRuntimeConfig_Default(t *testing.T) {
	logger := logrus.New()

	// 测试默认配置
	adapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, nil, logger)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)
	assert.Equal(t, "http://coze-loop-faas-enhanced:8000", adapter.config.BaseURL)
	assert.Equal(t, 30*time.Second, adapter.config.Timeout)
	assert.Equal(t, 3, adapter.config.MaxRetries)
	assert.Equal(t, 1*time.Second, adapter.config.RetryInterval)
	assert.True(t, adapter.config.EnableEnhanced)
}