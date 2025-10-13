// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// mockRuntimeFactory 模拟运行时工厂
type mockRuntimeFactory struct {
	createRuntimeFunc      func(languageType entity.LanguageType) (component.IRuntime, error)
	getSupportedLanguagesFunc func() []entity.LanguageType
	getHealthStatusFunc    func() map[string]interface{}
	getMetricsFunc         func() map[string]interface{}
}

func (m *mockRuntimeFactory) CreateRuntime(languageType entity.LanguageType) (component.IRuntime, error) {
	if m.createRuntimeFunc != nil {
		return m.createRuntimeFunc(languageType)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRuntimeFactory) GetSupportedLanguages() []entity.LanguageType {
	if m.getSupportedLanguagesFunc != nil {
		return m.getSupportedLanguagesFunc()
	}
	return []entity.LanguageType{}
}

func (m *mockRuntimeFactory) GetHealthStatus() map[string]interface{} {
	if m.getHealthStatusFunc != nil {
		return m.getHealthStatusFunc()
	}
	return map[string]interface{}{}
}

func (m *mockRuntimeFactory) GetMetrics() map[string]interface{} {
	if m.getMetricsFunc != nil {
		return m.getMetricsFunc()
	}
	return map[string]interface{}{}
}

// mockRuntime 模拟运行时
type mockRuntime struct {
	getLanguageTypeFunc      func() entity.LanguageType
	runCodeFunc              func(ctx context.Context, code string, language string, timeoutMS int64, ext map[string]string) (*entity.ExecutionResult, error)
	validateCodeFunc         func(ctx context.Context, code string, language string) bool
	getSupportedLanguagesFunc func() []entity.LanguageType
	getHealthStatusFunc      func() map[string]interface{}
	getMetricsFunc           func() map[string]interface{}
	getReturnValFunctionFunc func() string
}

func (m *mockRuntime) GetLanguageType() entity.LanguageType {
	if m.getLanguageTypeFunc != nil {
		return m.getLanguageTypeFunc()
	}
	return entity.LanguageType("")
}

func (m *mockRuntime) RunCode(ctx context.Context, code string, language string, timeoutMS int64, ext map[string]string) (*entity.ExecutionResult, error) {
	if m.runCodeFunc != nil {
		return m.runCodeFunc(ctx, code, language, timeoutMS, ext)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRuntime) ValidateCode(ctx context.Context, code string, language string) bool {
	if m.validateCodeFunc != nil {
		return m.validateCodeFunc(ctx, code, language)
	}
	return false
}

func (m *mockRuntime) GetSupportedLanguages() []entity.LanguageType {
	if m.getSupportedLanguagesFunc != nil {
		return m.getSupportedLanguagesFunc()
	}
	return []entity.LanguageType{}
}

func (m *mockRuntime) GetHealthStatus() map[string]interface{} {
	if m.getHealthStatusFunc != nil {
		return m.getHealthStatusFunc()
	}
	return map[string]interface{}{}
}

func (m *mockRuntime) GetMetrics() map[string]interface{} {
	if m.getMetricsFunc != nil {
		return m.getMetricsFunc()
	}
	return map[string]interface{}{}
}

func (m *mockRuntime) GetReturnValFunction() string {
	if m.getReturnValFunctionFunc != nil {
		return m.getReturnValFunctionFunc()
	}
	return ""
}

func TestRuntimeManager_NewRuntimeManager(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	factory := &mockRuntimeFactory{}
	manager := NewRuntimeManager(factory, logger)
	
	assert.NotNil(t, manager)
	assert.Equal(t, factory, manager.factory)
	assert.NotNil(t, manager.cache)
	assert.NotNil(t, manager.logger)
}

func TestRuntimeManager_NewRuntimeManager_NilLogger(t *testing.T) {
	factory := &mockRuntimeFactory{}
	manager := NewRuntimeManager(factory, nil)
	
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.logger) // 应该创建默认logger
}

func TestRuntimeManager_GetRuntime_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	mockRuntime := &mockRuntime{
		getLanguageTypeFunc: func() entity.LanguageType {
			return entity.LanguageTypePython
		},
	}
	
	mockFactory := &mockRuntimeFactory{
		createRuntimeFunc: func(languageType entity.LanguageType) (component.IRuntime, error) {
			if languageType == entity.LanguageTypePython {
				return mockRuntime, nil
			}
			return nil, errors.New("unsupported language")
		},
	}
	
	manager := NewRuntimeManager(mockFactory, logger)
	
	// 第一次获取 - 应该创建新实例
	runtime1, err := manager.GetRuntime(entity.LanguageTypePython)
	require.NoError(t, err)
	assert.Equal(t, mockRuntime, runtime1)
	
	// 第二次获取 - 应该从缓存获取
	runtime2, err := manager.GetRuntime(entity.LanguageTypePython)
	require.NoError(t, err)
	assert.Equal(t, runtime1, runtime2) // 应该是同一个实例
}

func TestRuntimeManager_GetRuntime_FactoryError(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	expectedError := errors.New("factory error")
	
	mockFactory := &mockRuntimeFactory{
		createRuntimeFunc: func(languageType entity.LanguageType) (component.IRuntime, error) {
			return nil, expectedError
		},
	}
	
	manager := NewRuntimeManager(mockFactory, logger)
	
	runtime, err := manager.GetRuntime(entity.LanguageTypePython)
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	assert.Nil(t, runtime)
}

func TestRuntimeManager_GetRuntime_ConcurrentAccess(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	mockRuntime := &mockRuntime{
		getLanguageTypeFunc: func() entity.LanguageType {
			return entity.LanguageTypePython
		},
	}
	
	callCount := 0
	mockFactory := &mockRuntimeFactory{
		createRuntimeFunc: func(languageType entity.LanguageType) (component.IRuntime, error) {
			callCount++
			if languageType == entity.LanguageTypePython {
				return mockRuntime, nil
			}
			return nil, errors.New("unsupported language")
		},
	}
	
	manager := NewRuntimeManager(mockFactory, logger)
	
	// 并发获取同一个运行时
	done := make(chan bool, 10)
	results := make([]component.IRuntime, 10)
	errors := make([]error, 10)
	
	for i := 0; i < 10; i++ {
		go func(idx int) {
			defer func() { done <- true }()
			
			runtime, err := manager.GetRuntime(entity.LanguageTypePython)
			results[idx] = runtime
		errors[idx] = err
		}(i)
	}
	
	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// 验证所有结果都相同且没有错误
	for i := 0; i < 10; i++ {
		assert.NoError(t, errors[i])
		assert.Equal(t, mockRuntime, results[i])
	}
	
	// 由于双重检查锁，CreateRuntime应该只被调用一次
	assert.Equal(t, 1, callCount)
}

func TestRuntimeManager_GetSupportedLanguages(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	supportedLanguages := []entity.LanguageType{entity.LanguageTypePython, entity.LanguageTypeJS}
	
	mockFactory := &mockRuntimeFactory{
		getSupportedLanguagesFunc: func() []entity.LanguageType {
			return supportedLanguages
		},
	}
	
	manager := NewRuntimeManager(mockFactory, logger)
	
	languages := manager.GetSupportedLanguages()
	assert.Equal(t, supportedLanguages, languages)
}

func TestRuntimeManager_ClearCache(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	mockRuntime := &mockRuntime{
		getLanguageTypeFunc: func() entity.LanguageType {
			return entity.LanguageTypePython
		},
	}
	
	mockFactory := &mockRuntimeFactory{
		createRuntimeFunc: func(languageType entity.LanguageType) (component.IRuntime, error) {
			if languageType == entity.LanguageTypePython {
				return mockRuntime, nil
			}
			return nil, errors.New("unsupported language")
		},
	}
	
	manager := NewRuntimeManager(mockFactory, logger)
	
	// 先获取运行时，填充缓存
	runtime1, err := manager.GetRuntime(entity.LanguageTypePython)
	require.NoError(t, err)
	assert.Equal(t, mockRuntime, runtime1)
	
	// 验证缓存不为空
	assert.Equal(t, 1, len(manager.cache))
	
	// 清空缓存
	manager.ClearCache()
	
	// 验证缓存已清空
	assert.Equal(t, 0, len(manager.cache))
}

func TestRuntimeManager_GetHealthStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	mockRuntime := &mockRuntime{
		getLanguageTypeFunc: func() entity.LanguageType {
			return entity.LanguageTypePython
		},
		getHealthStatusFunc: func() map[string]interface{} {
			return map[string]interface{}{
				"status":   "healthy",
				"language": "python",
			}
		},
	}
	
	supportedLanguages := []entity.LanguageType{entity.LanguageTypePython, entity.LanguageTypeJS}
	
	mockFactory := &mockRuntimeFactory{
		createRuntimeFunc: func(languageType entity.LanguageType) (component.IRuntime, error) {
			if languageType == entity.LanguageTypePython {
				return mockRuntime, nil
			}
			return nil, errors.New("unsupported language")
		},
		getSupportedLanguagesFunc: func() []entity.LanguageType {
			return supportedLanguages
		},
		getHealthStatusFunc: func() map[string]interface{} {
			return map[string]interface{}{
				"status":  "healthy",
				"factory": "test",
			}
		},
	}
	
	manager := NewRuntimeManager(mockFactory, logger)
	
	// 先获取运行时，填充缓存
	runtime, err := manager.GetRuntime(entity.LanguageTypePython)
	require.NoError(t, err)
	assert.Equal(t, mockRuntime, runtime)
	
	// 获取健康状态
	status := manager.GetHealthStatus()
	assert.NotNil(t, status)
	assert.Equal(t, "healthy", status["status"])
	assert.Equal(t, 1, status["cached_runtimes"])
	assert.Equal(t, supportedLanguages, status["supported_languages"])
	
	// 验证工厂健康状态
	factoryHealth, ok := status["factory_health"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "healthy", factoryHealth["status"])
	
	// 验证运行时状态
	runtimeStatus, ok := status["runtime_status"].(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, runtimeStatus, "Python") // 注意：键是string(entity.LanguageTypePython) = "Python"
}

func TestRuntimeManager_GetMetrics(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	mockRuntime := &mockRuntime{
		getLanguageTypeFunc: func() entity.LanguageType {
			return entity.LanguageTypePython
		},
		getMetricsFunc: func() map[string]interface{} {
			return map[string]interface{}{
				"runtime_type": "python",
				"executions":   100,
			}
		},
	}
	
	supportedLanguages := []entity.LanguageType{entity.LanguageTypePython, entity.LanguageTypeJS}
	
	mockFactory := &mockRuntimeFactory{
		createRuntimeFunc: func(languageType entity.LanguageType) (component.IRuntime, error) {
			if languageType == entity.LanguageTypePython {
				return mockRuntime, nil
			}
			return nil, errors.New("unsupported language")
		},
		getSupportedLanguagesFunc: func() []entity.LanguageType {
			return supportedLanguages
		},
	}
	
	manager := NewRuntimeManager(mockFactory, logger)
	
	// 先获取运行时，填充缓存
	runtime, err := manager.GetRuntime(entity.LanguageTypePython)
	require.NoError(t, err)
	assert.Equal(t, mockRuntime, runtime)
	
	// 获取指标
	metrics := manager.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, "unified", metrics["manager_type"])
	assert.Equal(t, 1, metrics["cached_runtimes"])
	assert.Equal(t, 2, metrics["supported_languages"])
	
	// 验证运行时指标
	runtimeMetrics, ok := metrics["runtime_metrics"].(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, runtimeMetrics, "Python") // 注意：键是string(entity.LanguageTypePython) = "Python"
	
	pythonMetrics, ok := runtimeMetrics["Python"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "python", pythonMetrics["runtime_type"])
	assert.Equal(t, 100, pythonMetrics["executions"])
}

func TestRuntimeManager_GetHealthStatus_EmptyCache(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	supportedLanguages := []entity.LanguageType{entity.LanguageTypePython, entity.LanguageTypeJS}
	
	mockFactory := &mockRuntimeFactory{
		getSupportedLanguagesFunc: func() []entity.LanguageType {
			return supportedLanguages
		},
		getHealthStatusFunc: func() map[string]interface{} {
			return map[string]interface{}{
				"status":  "healthy",
				"factory": "test",
			}
		},
	}
	
	manager := NewRuntimeManager(mockFactory, logger)
	
	// 获取健康状态（空缓存）
	status := manager.GetHealthStatus()
	assert.NotNil(t, status)
	assert.Equal(t, "healthy", status["status"])
	assert.Equal(t, 0, status["cached_runtimes"])
	assert.Equal(t, supportedLanguages, status["supported_languages"])
	
	// 验证运行时状态为空
	runtimeStatus, ok := status["runtime_status"].(map[string]interface{})
	assert.True(t, ok)
	assert.Empty(t, runtimeStatus)
}

func TestRuntimeManager_GetMetrics_EmptyCache(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	
	supportedLanguages := []entity.LanguageType{entity.LanguageTypePython, entity.LanguageTypeJS}
	
	mockFactory := &mockRuntimeFactory{
		getSupportedLanguagesFunc: func() []entity.LanguageType {
			return supportedLanguages
		},
	}
	
	manager := NewRuntimeManager(mockFactory, logger)
	
	// 获取指标（空缓存）
	metrics := manager.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, "unified", metrics["manager_type"])
	assert.Equal(t, 0, metrics["cached_runtimes"])
	assert.Equal(t, 2, metrics["supported_languages"])
	
	// 验证运行时指标为空 - 当没有缓存的运行时时，runtime_metrics字段不存在
	_, exists := metrics["runtime_metrics"]
	assert.False(t, exists)
}