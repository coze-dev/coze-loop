// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// TestHTTPFaaSIntegration 测试HTTP FaaS的集成功能
func TestHTTPFaaSIntegration(t *testing.T) {
	// 检查是否设置了FaaS URL
	faasURL := os.Getenv("COZE_LOOP_FAAS_URL")
	if faasURL == "" {
		t.Skip("跳过HTTP FaaS集成测试：未设置COZE_LOOP_FAAS_URL环境变量")
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// 创建HTTP FaaS运行时适配器
	config := &HTTPFaaSRuntimeConfig{
		BaseURL:        faasURL,
		Timeout:        30 * time.Second,
		MaxRetries:     2,
		RetryInterval:  500 * time.Millisecond,
		EnableEnhanced: true,
	}

	jsAdapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, config, logger)
	require.NoError(t, err)

	pythonAdapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypePython, config, logger)
	require.NoError(t, err)

	t.Run("JavaScript代码执行", func(t *testing.T) {
		ctx := context.Background()
		code := `
			console.log("Hello from JavaScript");
			const result = 2 + 3;
			console.log("Result:", result);
			return result;
		`

		result, err := jsAdapter.RunCode(ctx, code, "javascript", 10000, make(map[string]string))
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Output)
		
		// 检查输出包含预期内容
		t.Logf("JavaScript输出: stdout=%s, stderr=%s, ret_val=%s", 
			result.Output.Stdout, result.Output.Stderr, result.Output.RetVal)
		
	assert.Contains(t, result.Output.Stdout, "Hello from JavaScript")
	})

	t.Run("Python代码执行", func(t *testing.T) {
		ctx := context.Background()
		code := `
print("Hello from Python")
x = 10
y = 20
result = x + y
print(f"Result: {result}")
		`

		result, err := pythonAdapter.RunCode(ctx, code, "python", 10000, make(map[string]string))
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Output)
		
		// 检查输出包含预期内容
		t.Logf("Python输出: stdout=%s, stderr=%s, ret_val=%s", 
			result.Output.Stdout, result.Output.Stderr, result.Output.RetVal)
		
			assert.Contains(t, result.Output.Stdout, "Hello from Python")
	assert.Contains(t, result.Output.Stdout, "Result: 30")
	})

	t.Run("错误代码处理", func(t *testing.T) {
		ctx := context.Background()
		code := `
			console.log("Before error");
			throw new Error("Test error");
			console.log("After error");
		`

		result, err := jsAdapter.RunCode(ctx, code, "javascript", 10000, make(map[string]string))
		// 即使代码有错误，HTTP FaaS也应该返回结果而不是错误
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Output)
		
		t.Logf("错误代码输出: stdout=%s, stderr=%s", 
			result.Output.Stdout, result.Output.Stderr)
	})

	t.Run("超时处理", func(t *testing.T) {
		ctx := context.Background()
		code := `
			// 模拟长时间运行的代码
			let sum = 0;
			for (let i = 0; i < 1000000; i++) {
				sum += i;
			}
			return sum;
		`

		start := time.Now()
		result, err := jsAdapter.RunCode(ctx, code, "javascript", 1000, make(map[string]string)) // 1秒超时
		duration := time.Since(start)
		
		// 应该在合理时间内完成（可能超时或正常完成）
		assert.True(t, duration < 5*time.Second, "执行时间应该在5秒内")
		
		if err != nil {
			t.Logf("超时测试产生错误（预期）: %v", err)
		} else {
			t.Logf("超时测试完成: %+v", result)
		}
	})

	t.Run("并发执行", func(t *testing.T) {
		ctx := context.Background()
		
		// 启动多个并发执行
		const concurrency = 5
		results := make(chan error, concurrency)
		
		for i := 0; i < concurrency; i++ {
			go func(index int) {
				code := fmt.Sprintf(`
					console.log("Task %d started");
					const result = %d * 2;
					console.log("Task %d result:", result);
					return result;
				`, index, index, index)
				
				_, err := jsAdapter.RunCode(ctx, code, "javascript", 5000, make(map[string]string))
				results <- err
			}(i)
		}
		
		// 等待所有任务完成
		for i := 0; i < concurrency; i++ {
			err := <-results
			assert.NoError(t, err, "并发任务%d应该成功", i)
		}
	})

	// 清理
	t.Run("清理资源", func(t *testing.T) {
		err := jsAdapter.Cleanup()
		assert.NoError(t, err)
		
		err = pythonAdapter.Cleanup()
		assert.NoError(t, err)
	})
}