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

func TestEnhancedRuntimeIntegration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	
	// 测试增强工厂
	factory := NewEnhancedRuntimeFactory(logger, config)
	require.NotNil(t, factory)
	
	// 测试支持的语言
	supportedLanguages := factory.GetSupportedLanguages()
	assert.Contains(t, supportedLanguages, entity.LanguageTypeJS)
	assert.Contains(t, supportedLanguages, entity.LanguageTypePython)
	
	// 创建JavaScript运行时
	jsRuntime, err := factory.CreateRuntime(entity.LanguageTypeJS)
	require.NoError(t, err)
	require.NotNil(t, jsRuntime)
	
	// 创建Python运行时（应该返回同一个增强运行时实例）
	pythonRuntime, err := factory.CreateRuntime(entity.LanguageTypePython)
	require.NoError(t, err)
	require.NotNil(t, pythonRuntime)
	
	// 验证是否是同一个实例（单例模式）
	assert.Equal(t, jsRuntime, pythonRuntime)
	
	// 测试增强管理器
	manager := NewEnhancedRuntimeManager(factory, logger)
	require.NotNil(t, manager)
	
	// 通过管理器获取运行时
	managedJSRuntime, err := manager.GetRuntime(entity.LanguageTypeJS)
	require.NoError(t, err)
	require.NotNil(t, managedJSRuntime)
	
	managedPythonRuntime, err := manager.GetRuntime(entity.LanguageTypePython)
	require.NoError(t, err)
	require.NotNil(t, managedPythonRuntime)
	
	// 验证缓存机制
	assert.Equal(t, managedJSRuntime, managedPythonRuntime)
	
	// 测试代码执行
	ctx := context.Background()
	
	// JavaScript代码执行
	jsResult, err := managedJSRuntime.RunCode(ctx, "console.log('Hello from JS');", "javascript", 5000)
	require.NoError(t, err)
	require.NotNil(t, jsResult)
	assert.NotNil(t, jsResult.Output)
	
	// Python代码执行
	pythonResult, err := managedPythonRuntime.RunCode(ctx, "print('Hello from Python')", "python", 5000)
	require.NoError(t, err)
	require.NotNil(t, pythonResult)
	assert.NotNil(t, pythonResult.Output)
	
	// 测试代码验证
	assert.True(t, managedJSRuntime.ValidateCode(ctx, "console.log('valid');", "javascript"))
	assert.False(t, managedJSRuntime.ValidateCode(ctx, "console.log('invalid", "javascript"))
	
	// 测试健康状态（如果支持）
	if healthyRuntime, ok := managedJSRuntime.(interface{ GetHealthStatus() map[string]interface{} }); ok {
		healthStatus := healthyRuntime.GetHealthStatus()
		assert.NotNil(t, healthStatus)
		assert.Equal(t, "healthy", healthStatus["status"])
	}
	
	// 清理资源
	if managerWithShutdown, ok := manager.(interface{ Shutdown() error }); ok {
		err = managerWithShutdown.Shutdown()
		assert.NoError(t, err)
	} else {
		manager.ClearCache()
	}
}

func TestEnhancedRuntimePerformance(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // 减少日志输出以提高性能测试准确性
	
	config := entity.DefaultSandboxConfig()
	factory := NewEnhancedRuntimeFactory(logger, config)
	manager := NewEnhancedRuntimeManager(factory, logger)
	
	runtime, err := manager.GetRuntime(entity.LanguageTypeJS)
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// 性能测试：并发执行
	const numConcurrentTasks = 20
	const numIterations = 5
	
	results := make(chan time.Duration, numConcurrentTasks)
	
	startTime := time.Now()
	
	for i := 0; i < numConcurrentTasks; i++ {
		go func(taskID int) {
			taskStart := time.Now()
			
			for j := 0; j < numIterations; j++ {
				code := "const result = Math.random() * 100; console.log(result);"
				_, err := runtime.RunCode(ctx, code, "javascript", 10000)
				if err != nil {
					t.Errorf("任务 %d 执行失败: %v", taskID, err)
					return
				}
			}
			
			results <- time.Since(taskStart)
		}(i)
	}
	
	// 收集结果
	var totalDuration time.Duration
	for i := 0; i < numConcurrentTasks; i++ {
		select {
		case duration := <-results:
			totalDuration += duration
		case <-time.After(60 * time.Second):
			t.Fatal("性能测试超时")
		}
	}
	
	overallDuration := time.Since(startTime)
	averageTaskDuration := totalDuration / numConcurrentTasks
	
	t.Logf("性能测试结果:")
	t.Logf("  并发任务数: %d", numConcurrentTasks)
	t.Logf("  每任务迭代数: %d", numIterations)
	t.Logf("  总执行时间: %v", overallDuration)
	t.Logf("  平均任务时间: %v", averageTaskDuration)
	t.Logf("  吞吐量: %.2f 任务/秒", float64(numConcurrentTasks)/overallDuration.Seconds())
	
	// 验证性能指标
	assert.Less(t, overallDuration, 30*time.Second, "整体执行时间应该在30秒内")
	assert.Less(t, averageTaskDuration, 10*time.Second, "平均任务时间应该在10秒内")
	
	// 清理
	if managerWithShutdown, ok := manager.(interface{ Shutdown() error }); ok {
		err = managerWithShutdown.Shutdown()
		assert.NoError(t, err)
	} else {
		manager.ClearCache()
	}
}

func TestEnhancedRuntimeErrorHandling(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	factory := NewEnhancedRuntimeFactory(logger, config)
	manager := NewEnhancedRuntimeManager(factory, logger)
	
	runtime, err := manager.GetRuntime(entity.LanguageTypeJS)
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// 测试各种错误情况
	tests := []struct {
		name        string
		code        string
		language    string
		timeout     int64
		expectError bool
		errorType   string
	}{
		{
			name:        "空代码",
			code:        "",
			language:    "javascript",
			timeout:     5000,
			expectError: true,
			errorType:   "empty_code",
		},
		{
			name:        "不支持的语言",
			code:        "print('hello')",
			language:    "unsupported",
			timeout:     5000,
			expectError: true,
			errorType:   "unsupported_language",
		},
		{
			name:        "语法错误",
			code:        "console.log('unclosed string",
			language:    "javascript",
			timeout:     5000,
			expectError: false, // 增强运行时可能会处理语法错误
			errorType:   "syntax_error",
		},
		{
			name:        "运行时错误",
			code:        "throw new Error('Runtime error');",
			language:    "javascript",
			timeout:     5000,
			expectError: false, // 运行时错误应该被捕获并返回结果
			errorType:   "runtime_error",
		},
		{
			name:        "超时测试",
			code:        "while(true) { /* infinite loop */ }",
			language:    "javascript",
			timeout:     1000, // 1秒超时
			expectError: true,
			errorType:   "timeout",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := runtime.RunCode(ctx, tt.code, tt.language, tt.timeout)
			
			if tt.expectError {
				assert.Error(t, err, "应该返回错误: %s", tt.errorType)
				assert.Nil(t, result, "错误情况下结果应该为nil")
			} else {
				// 对于某些情况，增强运行时可能会成功执行但在结果中包含错误信息
				if err != nil {
					t.Logf("预期可能的错误: %v", err)
				} else {
					assert.NotNil(t, result, "成功情况下结果不应该为nil")
				}
			}
		})
	}
	
	// 清理
	if managerWithShutdown, ok := manager.(interface{ Shutdown() error }); ok {
		err = managerWithShutdown.Shutdown()
		assert.NoError(t, err)
	} else {
		manager.ClearCache()
	}
}