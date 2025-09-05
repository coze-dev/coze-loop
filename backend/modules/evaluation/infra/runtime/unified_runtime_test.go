// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestUnifiedRuntime_Basic(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewUnifiedRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	defer func() {
		err := runtime.Cleanup()
		assert.NoError(t, err)
	}()
	
	// 测试基本属性
	assert.Equal(t, entity.LanguageTypeJS, runtime.GetLanguageType())
	
	supportedLanguages := runtime.GetSupportedLanguages()
	assert.Contains(t, supportedLanguages, entity.LanguageTypeJS)
	assert.Contains(t, supportedLanguages, entity.LanguageTypePython)
}

func TestUnifiedRuntime_ValidateCode(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewUnifiedRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	defer func() {
		err := runtime.Cleanup()
		assert.NoError(t, err)
	}()
	
	ctx := context.Background()
	
	// 测试JavaScript代码验证
	t.Run("ValidJavaScript", func(t *testing.T) {
		validJS := `
			function add(a, b) {
				return a + b;
			}
			console.log(add(1, 2));
		`
		assert.True(t, runtime.ValidateCode(ctx, validJS, "javascript"))
	})
	
	// 测试Python代码验证
	t.Run("ValidPython", func(t *testing.T) {
		validPython := `
def add(a, b):
    return a + b

print(add(1, 2))
		`
		assert.True(t, runtime.ValidateCode(ctx, validPython, "python"))
	})
	
	// 测试无效代码
	t.Run("InvalidCode", func(t *testing.T) {
		invalidCode := `function test() { console.log("unclosed`
		assert.False(t, runtime.ValidateCode(ctx, invalidCode, "javascript"))
	})
	
	// 测试空代码
	t.Run("EmptyCode", func(t *testing.T) {
		assert.False(t, runtime.ValidateCode(ctx, "", "javascript"))
	})
	
	// 测试不支持的语言
	t.Run("UnsupportedLanguage", func(t *testing.T) {
		assert.False(t, runtime.ValidateCode(ctx, "print('test')", "unsupported"))
	})
}

func TestUnifiedRuntime_HTTPFaaSMode(t *testing.T) {
	// 设置环境变量以启用HTTP FaaS模式
	originalURL := os.Getenv("COZE_LOOP_FAAS_URL")
	os.Setenv("COZE_LOOP_FAAS_URL", "http://localhost:8000")
	defer func() {
		if originalURL == "" {
			os.Unsetenv("COZE_LOOP_FAAS_URL")
		} else {
			os.Setenv("COZE_LOOP_FAAS_URL", originalURL)
		}
	}()
	
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewUnifiedRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	defer func() {
		err := runtime.Cleanup()
		assert.NoError(t, err)
	}()
	
	// 验证HTTP FaaS模式
	healthStatus := runtime.GetHealthStatus()
	assert.Equal(t, true, healthStatus["use_http_faas"])
	assert.Equal(t, "http_faas", healthStatus["mode"])
}

func TestUnifiedRuntime_EnhancedMode(t *testing.T) {
	// 确保没有设置HTTP FaaS环境变量
	originalURL := os.Getenv("COZE_LOOP_FAAS_URL")
	os.Unsetenv("COZE_LOOP_FAAS_URL")
	defer func() {
		if originalURL != "" {
			os.Setenv("COZE_LOOP_FAAS_URL", originalURL)
		}
	}()
	
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewUnifiedRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	defer func() {
		err := runtime.Cleanup()
		assert.NoError(t, err)
	}()
	
	// 验证增强模式
	healthStatus := runtime.GetHealthStatus()
	assert.Equal(t, false, healthStatus["use_http_faas"])
	assert.Equal(t, "enhanced_local", healthStatus["mode"])
}

func TestUnifiedRuntime_HealthStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewUnifiedRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	defer func() {
		err := runtime.Cleanup()
		assert.NoError(t, err)
	}()
	
	healthStatus := runtime.GetHealthStatus()
	assert.Equal(t, "healthy", healthStatus["status"])
	assert.NotNil(t, healthStatus["supported_languages"])
	assert.NotNil(t, healthStatus["use_http_faas"])
}

func TestUnifiedRuntime_Metrics(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewUnifiedRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	defer func() {
		err := runtime.Cleanup()
		assert.NoError(t, err)
	}()
	
	metrics := runtime.GetMetrics()
	assert.Equal(t, "unified", metrics["mode"])
	assert.NotNil(t, metrics["runtime_type"])
}

func TestUnifiedRuntime_LanguageNormalization(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"javascript", "js"},
		{"js", "js"},
		{"typescript", "js"},
		{"ts", "js"},
		{"python", "python"},
		{"py", "python"},
		{"unknown", "unknown"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizeLanguage(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestUnifiedRuntimeFactory_Basic(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	
	factory := NewUnifiedRuntimeFactory(logger, config)
	require.NotNil(t, factory)
	
	defer func() {
		if cleanupFactory, ok := factory.(*UnifiedRuntimeFactory); ok {
			err := cleanupFactory.Cleanup()
			assert.NoError(t, err)
		}
	}()
	
	// 测试支持的语言
	supportedLanguages := factory.GetSupportedLanguages()
	assert.Contains(t, supportedLanguages, entity.LanguageTypeJS)
	assert.Contains(t, supportedLanguages, entity.LanguageTypePython)
	
	// 测试创建JavaScript运行时
	jsRuntime, err := factory.CreateRuntime(entity.LanguageTypeJS)
	assert.NoError(t, err)
	assert.NotNil(t, jsRuntime)
	
	// 测试创建Python运行时（应该返回同一个实例）
	pythonRuntime, err := factory.CreateRuntime(entity.LanguageTypePython)
	assert.NoError(t, err)
	assert.NotNil(t, pythonRuntime)
	
	// 验证是同一个实例（统一运行时）
	assert.Equal(t, jsRuntime, pythonRuntime)
}

func TestUnifiedRuntimeManager_Basic(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	
	config := entity.DefaultSandboxConfig()
	
	manager := NewDefaultRuntimeManager(logger, config)
	require.NotNil(t, manager)
	
	// 测试支持的语言
	supportedLanguages := manager.GetSupportedLanguages()
	assert.Contains(t, supportedLanguages, entity.LanguageTypeJS)
	assert.Contains(t, supportedLanguages, entity.LanguageTypePython)
	
	// 测试获取运行时
	jsRuntime, err := manager.GetRuntime(entity.LanguageTypeJS)
	assert.NoError(t, err)
	assert.NotNil(t, jsRuntime)
	
	// 测试缓存（第二次获取应该返回相同实例）
	jsRuntime2, err := manager.GetRuntime(entity.LanguageTypeJS)
	assert.NoError(t, err)
	assert.Equal(t, jsRuntime, jsRuntime2)
	
	// 测试健康状态
	if healthManager, ok := manager.(*UnifiedRuntimeManager); ok {
		healthStatus := healthManager.GetHealthStatus()
		assert.Equal(t, "healthy", healthStatus["status"])
		assert.Equal(t, 1, healthStatus["cached_runtimes"]) // 应该有一个缓存的运行时
	}
	
	// 简化清理：只清空缓存，不进行复杂的资源清理
	manager.ClearCache()
}