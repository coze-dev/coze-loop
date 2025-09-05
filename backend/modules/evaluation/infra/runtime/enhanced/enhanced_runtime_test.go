// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package enhanced

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestNewEnhancedRuntime(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewEnhancedRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	// 测试基本属性
	assert.Equal(t, entity.LanguageTypeJS, runtime.GetLanguageType())
	assert.Contains(t, runtime.GetSupportedLanguages(), entity.LanguageTypeJS)
	assert.Contains(t, runtime.GetSupportedLanguages(), entity.LanguageTypePython)
	
	// 清理资源
	err = runtime.Cleanup()
	assert.NoError(t, err)
}

func TestEnhancedRuntime_RunCode_JavaScript(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	runtime, err := NewEnhancedRuntime(config, logger)
	require.NoError(t, err)
	defer runtime.Cleanup()
	
	ctx := context.Background()
	
	tests := []struct {
		name        string
		code        string
		language    string
		timeoutMS   int64
		expectError bool
	}{
		{
			name:        "简单JavaScript代码",
			code:        "console.log('Hello World');",
			language:    "javascript",
			timeoutMS:   5000,
			expectError: false,
		},
		{
			name:        "JavaScript计算",
			code:        "const result = 1 + 2; console.log(result);",
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
			name:        "不支持的语言",
			code:        "print('Hello')",
			language:    "unsupported",
			timeoutMS:   5000,
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := runtime.RunCode(ctx, tt.code, tt.language, tt.timeoutMS)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotNil(t, result.Output)
				assert.NotNil(t, result.WorkloadInfo)
			}
		})
	}
}

func TestEnhancedRuntime_RunCode_Python(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	runtime, err := NewEnhancedRuntime(config, logger)
	require.NoError(t, err)
	defer runtime.Cleanup()
	
	ctx := context.Background()
	
	tests := []struct {
		name        string
		code        string
		language    string
		timeoutMS   int64
		expectError bool
	}{
		{
			name:        "简单Python代码",
			code:        "print('Hello World')",
			language:    "python",
			timeoutMS:   5000,
			expectError: false,
		},
		{
			name:        "Python计算",
			code:        "result = 1 + 2\nprint(result)",
			language:    "python",
			timeoutMS:   5000,
			expectError: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := runtime.RunCode(ctx, tt.code, tt.language, tt.timeoutMS)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotNil(t, result.Output)
				assert.NotNil(t, result.WorkloadInfo)
			}
		})
	}
}

func TestEnhancedRuntime_ValidateCode(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	runtime, err := NewEnhancedRuntime(config, logger)
	require.NoError(t, err)
	defer runtime.Cleanup()
	
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
		{
			name:     "不支持的语言",
			code:     "some code",
			language: "unsupported",
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runtime.ValidateCode(ctx, tt.code, tt.language)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEnhancedRuntime_ConcurrentExecution(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	runtime, err := NewEnhancedRuntime(config, logger)
	require.NoError(t, err)
	defer runtime.Cleanup()
	
	// 并发执行多个任务
	const numTasks = 10
	results := make(chan error, numTasks)
	
	for i := 0; i < numTasks; i++ {
		go func(taskID int) {
			ctx := context.Background()
			code := "console.log('Task " + string(rune('0'+taskID)) + " executed');"
			
			_, err := runtime.RunCode(ctx, code, "javascript", 5000)
			results <- err
		}(i)
	}
	
	// 等待所有任务完成
	for i := 0; i < numTasks; i++ {
		select {
		case err := <-results:
			assert.NoError(t, err)
		case <-time.After(30 * time.Second):
			t.Fatal("任务执行超时")
		}
	}
}

func TestEnhancedRuntime_GetMetrics(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	runtime, err := NewEnhancedRuntime(config, logger)
	require.NoError(t, err)
	defer runtime.Cleanup()
	
	// 获取池指标
	poolMetrics := runtime.GetPoolMetrics()
	assert.NotNil(t, poolMetrics)
	assert.GreaterOrEqual(t, poolMetrics.TotalInstances, int64(0))
	
	// 获取调度器指标
	schedulerMetrics := runtime.GetSchedulerMetrics()
	assert.NotNil(t, schedulerMetrics)
	assert.GreaterOrEqual(t, schedulerMetrics.TotalTasks, int64(0))
	
	// 获取健康状态
	healthStatus := runtime.GetHealthStatus()
	assert.NotNil(t, healthStatus)
	assert.Equal(t, "healthy", healthStatus["status"])
	assert.Contains(t, healthStatus, "pool")
	assert.Contains(t, healthStatus, "scheduler")
	assert.Contains(t, healthStatus, "supported_languages")
}

func TestEnhancedRuntime_ResourceManagement(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	runtime, err := NewEnhancedRuntime(config, logger)
	require.NoError(t, err)
	
	// 执行一些任务以创建实例
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := runtime.RunCode(ctx, "console.log('test');", "javascript", 5000)
		assert.NoError(t, err)
	}
	
	// 检查池指标
	poolMetrics := runtime.GetPoolMetrics()
	assert.Greater(t, poolMetrics.TotalInstances, int64(0))
	
	// 清理资源
	err = runtime.Cleanup()
	assert.NoError(t, err)
}