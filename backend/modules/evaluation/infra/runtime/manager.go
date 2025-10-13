// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// RuntimeManager 统一的运行时管理器，提供线程安全的Runtime实例缓存和管理
type RuntimeManager struct {
	factory component.IRuntimeFactory
	cache   map[entity.LanguageType]component.IRuntime
	mutex   sync.RWMutex
	logger  *logrus.Logger
}

// NewRuntimeManager 创建统一运行时管理器实例
func NewRuntimeManager(factory component.IRuntimeFactory, logger *logrus.Logger) *RuntimeManager {
	if logger == nil {
		logger = logrus.New()
	}

	return &RuntimeManager{
		factory: factory,
		cache:   make(map[entity.LanguageType]component.IRuntime),
		logger:  logger,
	}
}

// GetRuntime 获取指定语言类型的Runtime实例，支持缓存和线程安全
func (m *RuntimeManager) GetRuntime(languageType entity.LanguageType) (component.IRuntime, error) {
	// 先尝试从缓存获取
	m.mutex.RLock()
	if runtime, exists := m.cache[languageType]; exists {
		m.mutex.RUnlock()
		return runtime, nil
	}
	m.mutex.RUnlock()

	// 缓存中不存在，创建新的Runtime
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 双重检查，防止并发创建
	if runtime, exists := m.cache[languageType]; exists {
		return runtime, nil
	}

	// 通过工厂创建Runtime
	runtime, err := m.factory.CreateRuntime(languageType)
	if err != nil {
		m.logger.WithError(err).WithField("language_type", languageType).Error("创建运行时失败")
		return nil, err
	}

	// 缓存Runtime实例
	m.cache[languageType] = runtime

	m.logger.WithField("language_type", languageType).Info("运行时实例创建并缓存成功")
	return runtime, nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (m *RuntimeManager) GetSupportedLanguages() []entity.LanguageType {
	return m.factory.GetSupportedLanguages()
}

// ClearCache 清空缓存（主要用于测试和重置）
func (m *RuntimeManager) ClearCache() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.cache = make(map[entity.LanguageType]component.IRuntime)
	m.logger.Info("运行时缓存已清空")
}

// GetHealthStatus 获取管理器健康状态
func (m *RuntimeManager) GetHealthStatus() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	status := map[string]interface{}{
		"status":              "healthy",
		"supported_languages": m.GetSupportedLanguages(),
		"cached_runtimes":     len(m.cache),
	}

	// 添加工厂健康状态
	if healthFactory, ok := m.factory.(interface{ GetHealthStatus() map[string]interface{} }); ok {
		status["factory_health"] = healthFactory.GetHealthStatus()
	}

	// 添加缓存的运行时状态
	runtimeStatus := make(map[string]interface{})
	for languageType, runtime := range m.cache {
		if healthRuntime, ok := runtime.(interface{ GetHealthStatus() map[string]interface{} }); ok {
			runtimeStatus[string(languageType)] = healthRuntime.GetHealthStatus()
		} else {
			runtimeStatus[string(languageType)] = map[string]interface{}{
				"status": "cached",
			}
		}
	}
	status["runtime_status"] = runtimeStatus

	return status
}

// GetMetrics 获取管理器指标
func (m *RuntimeManager) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	metrics := map[string]interface{}{
		"manager_type":        "unified",
		"cached_runtimes":     len(m.cache),
		"supported_languages": len(m.GetSupportedLanguages()),
	}

	// 添加运行时指标
	runtimeMetrics := make(map[string]interface{})
	for languageType, runtime := range m.cache {
		if metricsRuntime, ok := runtime.(interface{ GetMetrics() map[string]interface{} }); ok {
			runtimeMetrics[string(languageType)] = metricsRuntime.GetMetrics()
		}
	}
	if len(runtimeMetrics) > 0 {
		metrics["runtime_metrics"] = runtimeMetrics
	}

	return metrics
}

// 确保RuntimeManager实现IRuntimeManager接口
var _ component.IRuntimeManager = (*RuntimeManager)(nil)
