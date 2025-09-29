// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// RuntimeFactory 统一的运行时工厂实现
type RuntimeFactory struct {
	logger        *logrus.Logger
	sandboxConfig *entity.SandboxConfig
	runtimeCache  map[entity.LanguageType]component.IRuntime
	mutex         sync.RWMutex
}

// NewRuntimeFactory 创建统一运行时工厂实例
func NewRuntimeFactory(logger *logrus.Logger, sandboxConfig *entity.SandboxConfig) component.IRuntimeFactory {
	if sandboxConfig == nil {
		sandboxConfig = entity.DefaultSandboxConfig()
	}
	
	if logger == nil {
		logger = logrus.New()
	}
	
	return &RuntimeFactory{
		logger:        logger,
		sandboxConfig: sandboxConfig,
		runtimeCache:  make(map[entity.LanguageType]component.IRuntime),
	}
}

// CreateRuntime 根据语言类型创建Runtime实例
func (f *RuntimeFactory) CreateRuntime(languageType entity.LanguageType) (component.IRuntime, error) {
	// 检查缓存
	f.mutex.RLock()
	if runtime, exists := f.runtimeCache[languageType]; exists {
		f.mutex.RUnlock()
		return runtime, nil
	}
	f.mutex.RUnlock()

	// 双重检查锁
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	if runtime, exists := f.runtimeCache[languageType]; exists {
		return runtime, nil
	}

	// 根据语言类型创建对应的Runtime实例
	var runtime component.IRuntime
	var err error

	switch languageType {
	case entity.LanguageTypePython:
		runtime, err = NewPythonRuntime(f.sandboxConfig, f.logger)
		if err != nil {
			return nil, fmt.Errorf("创建Python运行时失败: %w", err)
		}
		f.logger.Info("Python运行时创建成功")

	case entity.LanguageTypeJS:
		runtime, err = NewJavaScriptRuntime(f.sandboxConfig, f.logger)
		if err != nil {
			return nil, fmt.Errorf("创建JavaScript运行时失败: %w", err)
		}
		f.logger.Info("JavaScript运行时创建成功")

	default:
		return nil, fmt.Errorf("不支持的语言类型: %s", languageType)
	}
	
	// 缓存运行时实例
	f.runtimeCache[languageType] = runtime
	
	return runtime, nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (f *RuntimeFactory) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}
}


// GetHealthStatus 获取工厂健康状态
func (f *RuntimeFactory) GetHealthStatus() map[string]interface{} {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	
	status := map[string]interface{}{
		"status":             "healthy",
		"supported_languages": f.GetSupportedLanguages(),
		"cache_size":         len(f.runtimeCache),
	}
	
	// 添加缓存的运行时健康状态
	runtimeHealth := make(map[string]interface{})
	for languageType, runtime := range f.runtimeCache {
		if healthRuntime, ok := runtime.(interface{ GetHealthStatus() map[string]interface{} }); ok {
			runtimeHealth[string(languageType)] = healthRuntime.GetHealthStatus()
		} else {
			runtimeHealth[string(languageType)] = map[string]interface{}{
				"status": "cached",
			}
		}
	}
	if len(runtimeHealth) > 0 {
		status["runtime_health"] = runtimeHealth
	}
	
	return status
}

// GetMetrics 获取工厂指标
func (f *RuntimeFactory) GetMetrics() map[string]interface{} {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	
	metrics := map[string]interface{}{
		"factory_type":       "language_specific",
		"cache_size":         len(f.runtimeCache),
		"supported_languages": len(f.GetSupportedLanguages()),
	}
	
	// 添加运行时指标
	runtimeMetrics := make(map[string]interface{})
	for languageType, runtime := range f.runtimeCache {
		if metricsRuntime, ok := runtime.(interface{ GetMetrics() map[string]interface{} }); ok {
			runtimeMetrics[string(languageType)] = metricsRuntime.GetMetrics()
		}
	}
	if len(runtimeMetrics) > 0 {
		metrics["runtime_metrics"] = runtimeMetrics
	}
	
	return metrics
}

// 确保RuntimeFactory实现IRuntimeFactory接口
var _ component.IRuntimeFactory = (*RuntimeFactory)(nil)