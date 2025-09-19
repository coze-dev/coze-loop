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

func TestPythonRuntime_Creation(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("COZE_LOOP_PYTHON_FAAS_URL", "http://localhost:8001")
	defer os.Unsetenv("COZE_LOOP_PYTHON_FAAS_URL")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewPythonRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	
	// 测试基本属性
	assert.Equal(t, entity.LanguageTypePython, runtime.GetLanguageType())
	assert.Equal(t, []entity.LanguageType{entity.LanguageTypePython}, runtime.GetSupportedLanguages())
}

func TestJavaScriptRuntime_Creation(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("COZE_LOOP_JS_FAAS_URL", "http://localhost:8002")
	defer os.Unsetenv("COZE_LOOP_JS_FAAS_URL")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewJavaScriptRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	
	// 测试基本属性
	assert.Equal(t, entity.LanguageTypeJS, runtime.GetLanguageType())
	assert.Equal(t, []entity.LanguageType{entity.LanguageTypeJS}, runtime.GetSupportedLanguages())
}

func TestRuntimeFactory_CreatePythonRuntime(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("COZE_LOOP_PYTHON_FAAS_URL", "http://localhost:8001")
	defer os.Unsetenv("COZE_LOOP_PYTHON_FAAS_URL")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	factory := NewRuntimeFactory(logger, config).(*RuntimeFactory)
	require.NotNil(t, factory)
	
	runtime, err := factory.CreateRuntime(entity.LanguageTypePython)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	// 测试缓存功能
	runtime2, err := factory.CreateRuntime(entity.LanguageTypePython)
	require.NoError(t, err)
	assert.Equal(t, runtime, runtime2) // 应该返回同一个实例
	

}

func TestRuntimeFactory_CreateJavaScriptRuntime(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("COZE_LOOP_JS_FAAS_URL", "http://localhost:8002")
	defer os.Unsetenv("COZE_LOOP_JS_FAAS_URL")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	factory := NewRuntimeFactory(logger, config).(*RuntimeFactory)
	require.NotNil(t, factory)
	
	runtime, err := factory.CreateRuntime(entity.LanguageTypeJS)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	// 测试缓存功能
	runtime2, err := factory.CreateRuntime(entity.LanguageTypeJS)
	require.NoError(t, err)
	assert.Equal(t, runtime, runtime2) // 应该返回同一个实例
	

}

func TestRuntimeFactory_UnsupportedLanguage(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	factory := NewRuntimeFactory(logger, config)
	require.NotNil(t, factory)
	
	runtime, err := factory.CreateRuntime("unsupported")
	assert.Error(t, err)
	assert.Nil(t, runtime)
	assert.Contains(t, err.Error(), "不支持的语言类型")
}

func TestPythonRuntime_ValidateCode(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("COZE_LOOP_PYTHON_FAAS_URL", "http://localhost:8001")
	defer os.Unsetenv("COZE_LOOP_PYTHON_FAAS_URL")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewPythonRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	ctx := context.Background()
	
	// 测试空代码
	assert.False(t, runtime.ValidateCode(ctx, "", "python"))
	
	// 测试简单有效代码
	assert.True(t, runtime.ValidateCode(ctx, "print('hello')", "python"))
	
	// 测试括号不匹配的代码
	assert.False(t, runtime.ValidateCode(ctx, "print('hello'", "python"))
}

func TestJavaScriptRuntime_ValidateCode(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("COZE_LOOP_JS_FAAS_URL", "http://localhost:8002")
	defer os.Unsetenv("COZE_LOOP_JS_FAAS_URL")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewJavaScriptRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	ctx := context.Background()
	
	// 测试空代码
	assert.False(t, runtime.ValidateCode(ctx, "", "javascript"))
	
	// 测试简单有效代码
	assert.True(t, runtime.ValidateCode(ctx, "console.log('hello');", "javascript"))
	
	// 测试括号不匹配的代码
	assert.False(t, runtime.ValidateCode(ctx, "console.log('hello'", "javascript"))
}

func TestPythonRuntime_RunCode_EmptyCode(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("COZE_LOOP_PYTHON_FAAS_URL", "http://localhost:8001")
	defer os.Unsetenv("COZE_LOOP_PYTHON_FAAS_URL")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewPythonRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	ctx := context.Background()
	
	// 测试空代码
	result, err := runtime.RunCode(ctx, "", "python", 5000, make(map[string]string))
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "代码不能为空")
}

func TestJavaScriptRuntime_RunCode_EmptyCode(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("COZE_LOOP_JS_FAAS_URL", "http://localhost:8002")
	defer os.Unsetenv("COZE_LOOP_JS_FAAS_URL")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewJavaScriptRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	
	ctx := context.Background()
	
	// 测试空代码
	result, err := runtime.RunCode(ctx, "", "javascript", 5000, make(map[string]string))
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "代码不能为空")
}

func TestPythonRuntime_HealthStatus(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("COZE_LOOP_PYTHON_FAAS_URL", "http://localhost:8001")
	defer os.Unsetenv("COZE_LOOP_PYTHON_FAAS_URL")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewPythonRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	

	
	status := runtime.GetHealthStatus()
	assert.NotNil(t, status)
	assert.Equal(t, "healthy", status["status"])
	assert.Equal(t, "python", status["language"])
}

func TestJavaScriptRuntime_HealthStatus(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("COZE_LOOP_JS_FAAS_URL", "http://localhost:8002")
	defer os.Unsetenv("COZE_LOOP_JS_FAAS_URL")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewJavaScriptRuntime(config, logger)
	require.NoError(t, err)
	require.NotNil(t, runtime)
	

	
	status := runtime.GetHealthStatus()
	assert.NotNil(t, status)
	assert.Equal(t, "healthy", status["status"])
	assert.Equal(t, "javascript", status["language"])
}

func TestRuntimeFactory_GetSupportedLanguages(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	factory := NewRuntimeFactory(logger, config)
	require.NotNil(t, factory)
	
	languages := factory.GetSupportedLanguages()
	assert.Len(t, languages, 2)
	assert.Contains(t, languages, entity.LanguageTypePython)
	assert.Contains(t, languages, entity.LanguageTypeJS)
}

func TestRuntimeFactory_HealthStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	factory := NewRuntimeFactory(logger, config).(*RuntimeFactory)
	require.NotNil(t, factory)
	
	status := factory.GetHealthStatus()
	assert.NotNil(t, status)
	assert.Equal(t, "healthy", status["status"])
	assert.Equal(t, 0, status["cache_size"])
}

func TestRuntimeFactory_Metrics(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	factory := NewRuntimeFactory(logger, config).(*RuntimeFactory)
	require.NotNil(t, factory)
	
	metrics := factory.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, "language_specific", metrics["factory_type"])
	assert.Equal(t, 0, metrics["cache_size"])
	assert.Equal(t, 2, metrics["supported_languages"])
}

func TestPythonRuntime_MissingEnvironmentVariable(t *testing.T) {
	// 确保环境变量不存在
	os.Unsetenv("COZE_LOOP_PYTHON_FAAS_URL")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewPythonRuntime(config, logger)
	assert.Error(t, err)
	assert.Nil(t, runtime)
	assert.Contains(t, err.Error(), "必须配置Python FaaS服务URL")
}

func TestJavaScriptRuntime_MissingEnvironmentVariable(t *testing.T) {
	// 确保环境变量不存在
	os.Unsetenv("COZE_LOOP_JS_FAAS_URL")

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	config := entity.DefaultSandboxConfig()
	
	runtime, err := NewJavaScriptRuntime(config, logger)
	assert.Error(t, err)
	assert.Nil(t, runtime)
	assert.Contains(t, err.Error(), "必须配置JavaScript FaaS服务URL")
}