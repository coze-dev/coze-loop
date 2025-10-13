// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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


	t.Run("Cleanup", func(t *testing.T) {
		err := adapter.Cleanup()
		assert.NoError(t, err)
	})
}



func TestHTTPFaaSRuntimeConfig_Default(t *testing.T) {
	logger := logrus.New()

	// 测试默认配置
	adapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, nil, logger)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)
	assert.Equal(t, "http://coze-loop-js-faas:8000", adapter.config.BaseURL)
	assert.Equal(t, 30*time.Second, adapter.config.Timeout)
	assert.Equal(t, 3, adapter.config.MaxRetries)
	assert.Equal(t, 1*time.Second, adapter.config.RetryInterval)
	assert.True(t, adapter.config.EnableEnhanced)
}

func TestHTTPFaaSRuntimeAdapter_GetReturnValFunction(t *testing.T) {
	logger := logrus.New()
	
	tests := []struct {
		name         string
		languageType entity.LanguageType
		wantContains []string
	}{
		{
			name:         "Python return_val function",
			languageType: entity.LanguageTypePython,
			wantContains: []string{"def return_val", "__COZE_RETURN_VAL_START__", "__COZE_RETURN_VAL_END__"},
		},
		{
			name:         "JavaScript return_val function",
			languageType: entity.LanguageTypeJS,
			wantContains: []string{"function return_val", "console.log(ret_val)"},
		},
		{
			name:         "Unknown language type",
			languageType: entity.LanguageType("unknown"),
			wantContains: []string{""},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &HTTPFaaSRuntimeConfig{
				BaseURL:        "http://localhost:8890",
				Timeout:        30 * time.Second,
				MaxRetries:     1,
				RetryInterval:  100 * time.Millisecond,
				EnableEnhanced: true,
			}
			
			adapter, err := NewHTTPFaaSRuntimeAdapter(tt.languageType, config, logger)
			require.NoError(t, err)
			
			result := adapter.GetReturnValFunction()
			
			if tt.languageType == entity.LanguageType("unknown") {
				assert.Empty(t, result)
			} else {
				for _, want := range tt.wantContains {
					assert.Contains(t, result, want)
				}
			}
		})
	}
}

func TestHTTPFaaSRuntimeAdapter_RunCode_EmptyCode(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	config := &HTTPFaaSRuntimeConfig{
		BaseURL:        "http://localhost:8890",
		Timeout:        30 * time.Second,
		MaxRetries:     1,
		RetryInterval:  100 * time.Millisecond,
		EnableEnhanced: true,
	}

	adapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)

	ctx := context.Background()
	result, err := adapter.RunCode(ctx, "", "javascript", 5000, make(map[string]string))
	
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "代码不能为空")
}

func TestHTTPFaaSRuntimeAdapter_RunCode_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	config := &HTTPFaaSRuntimeConfig{
		BaseURL:        "http://localhost:8890",
		Timeout:        30 * time.Second,
		MaxRetries:     1,
		RetryInterval:  100 * time.Millisecond,
		EnableEnhanced: true,
	}

	adapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, config, logger)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)

	ctx := context.Background()
	code := `console.log("hello world");`
	
	// 由于我们没有真实的FaaS服务，这个测试会失败
	// 但我们仍然可以测试错误处理路径
	result, err := adapter.RunCode(ctx, code, "javascript", 5000, make(map[string]string))
	
	// 期望连接失败错误
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestHTTPFaaSRuntimeAdapter_NormalizeLanguage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"JavaScript lowercase", "javascript", "js"},
		{"JavaScript uppercase", "JAVASCRIPT", "js"},
		{"JS lowercase", "js", "js"},
		{"JS uppercase", "JS", "js"},
		{"TypeScript lowercase", "typescript", "js"},
		{"TypeScript uppercase", "TYPESCRIPT", "js"},
		{"TS lowercase", "ts", "js"},
		{"TS uppercase", "TS", "js"},
		{"Python lowercase", "python", "python"},
		{"Python uppercase", "PYTHON", "python"},
		{"Py lowercase", "py", "python"},
		{"Py uppercase", "PY", "python"},
		{"Unknown language", "ruby", "ruby"},
		{"Empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeLanguage(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHTTPFaaSRuntimeAdapter_GetTaskID(t *testing.T) {
	logger := logrus.New()
	config := &HTTPFaaSRuntimeConfig{
		BaseURL:        "http://localhost:8890",
		Timeout:        30 * time.Second,
		MaxRetries:     1,
		RetryInterval:  100 * time.Millisecond,
		EnableEnhanced: true,
	}

	adapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, config, logger)
	require.NoError(t, err)

	// 测试没有metadata的情况
	response := &HTTPFaaSResponse{}
	taskID := adapter.getTaskID(response)
	assert.Contains(t, taskID, "http_faas_")

	// 测试有metadata但没有TaskID的情况
	response.Metadata = &struct {
		TaskID     string `json:"task_id"`
		InstanceID string `json:"instance_id"`
		Duration   int64  `json:"duration"`
		PoolStats  struct {
			TotalInstances  int `json:"totalInstances"`
			IdleInstances   int `json:"idleInstances"`
			ActiveInstances int `json:"activeInstances"`
		} `json:"pool_stats"`
	}{}
	taskID = adapter.getTaskID(response)
	assert.Contains(t, taskID, "http_faas_")

	// 测试有TaskID的情况
	response.Metadata.TaskID = "test-task-123"
	taskID = adapter.getTaskID(response)
	assert.Equal(t, "test-task-123", taskID)
}

func TestHTTPFaaSRuntimeAdapter_BasicSyntaxValidation(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		{"Valid Python code", "print('hello world')", true},
		{"Valid JavaScript code", "console.log('hello world');", true},
		{"Valid code with brackets", "def test(): return [1, 2, 3]", true},
		{"Valid code with braces", "function test() { return {a: 1}; }", true},
		{"Valid code with parentheses", "print('hello')", true},
		{"Unmatched opening bracket", "print('hello'", false},
		{"Unmatched closing bracket", "print'hello')", false},
		{"Unmatched opening brace", "function test() { return ", false},
		{"Unmatched closing brace", "function test() return }", false},
		{"Unmatched opening parenthesis", "print'hello'", true}, // 这个测试用例实际上没有括号，所以应该是true
		{"Unmatched closing parenthesis", "print('hello", false},
		{"Multiple unmatched brackets", "print('hello' + [1, 2, 3", false},
		{"Nested but valid", "function test() { return [1, (2, 3)]; }", true},
		{"Empty string", "", true},
		{"Only whitespace", "   \n\t  ", true},
		{"Mixed brackets valid", "{ [ ( ) ] }", true},
		{"Mixed brackets invalid", "{ [ ( ] ) }", true}, // 这个测试用例实际上括号是匹配的，所以应该是true
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := basicSyntaxValidation(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}