// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// EnhancedRuntimeManager 增强版Runtime管理器，提供线程安全的Runtime实例缓存和管理
type EnhancedRuntimeManager struct {
	factory component.IRuntimeFactory
	cache   map[entity.LanguageType]component.IRuntime
	mutex   sync.RWMutex
	logger  *logrus.Logger
}

// NewEnhancedRuntimeManager 创建增强版RuntimeManager实例
func NewEnhancedRuntimeManager(factory component.IRuntimeFactory, logger *logrus.Logger) component.IRuntimeManager {
	return &EnhancedRuntimeManager{
		factory: factory,
		cache:   make(map[entity.LanguageType]component.IRuntime),
		logger:  logger,
	}
}

// GetRuntime 获取指定语言类型的Runtime实例，支持缓存和线程安全
func (m *EnhancedRuntimeManager) GetRuntime(languageType entity.LanguageType) (component.IRuntime, error) {
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
		m.logger.WithError(err).WithField("language_type", languageType).Error("创建增强运行时实例失败")
		return nil, err
	}

	// 缓存Runtime实例
	m.cache[languageType] = runtime
	
	m.logger.WithField("language_type", languageType).Info("增强运行时实例创建并缓存成功")
	
	return runtime, nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (m *EnhancedRuntimeManager) GetSupportedLanguages() []entity.LanguageType {
	return m.factory.GetSupportedLanguages()
}

// ClearCache 清空缓存（主要用于测试和资源清理）
func (m *EnhancedRuntimeManager) ClearCache() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// 清理现有实例
	for languageType, runtime := range m.cache {
		if err := runtime.Cleanup(); err != nil {
			m.logger.WithError(err).WithField("language_type", languageType).Error("清理运行时实例失败")
		}
	}
	
	m.cache = make(map[entity.LanguageType]component.IRuntime)
	m.logger.Info("增强运行时管理器缓存已清空")
}

// Shutdown 关闭管理器并清理所有资源
func (m *EnhancedRuntimeManager) Shutdown() error {
	m.logger.Info("开始关闭增强运行时管理器...")
	
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// 清理所有缓存的实例
	for languageType, runtime := range m.cache {
		if err := runtime.Cleanup(); err != nil {
			m.logger.WithError(err).WithField("language_type", languageType).Error("关闭运行时实例失败")
		}
	}
	
	// 清理工厂（如果工厂支持清理）
	if cleanupFactory, ok := m.factory.(interface{ Cleanup() error }); ok {
		if err := cleanupFactory.Cleanup(); err != nil {
			m.logger.WithError(err).Error("清理运行时工厂失败")
			return err
		}
	}
	
	m.cache = make(map[entity.LanguageType]component.IRuntime)
	m.logger.Info("增强运行时管理器已关闭")
	
	return nil
}

// GetHealthStatus 获取管理器健康状态
func (m *EnhancedRuntimeManager) GetHealthStatus() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	status := map[string]interface{}{
		"status":             "healthy",
		"cached_runtimes":    len(m.cache),
		"supported_languages": m.GetSupportedLanguages(),
	}
	
	// 如果有缓存的运行时，尝试获取其健康状态
	runtimeStatus := make(map[string]interface{})
	for languageType, runtime := range m.cache {
		if healthyRuntime, ok := runtime.(interface{ GetHealthStatus() map[string]interface{} }); ok {
			runtimeStatus[string(languageType)] = healthyRuntime.GetHealthStatus()
		} else {
			runtimeStatus[string(languageType)] = "available"
		}
	}
	
	if len(runtimeStatus) > 0 {
		status["runtime_status"] = runtimeStatus
	}
	
	return status
}

// 确保EnhancedRuntimeManager实现IRuntimeManager接口
var _ component.IRuntimeManager = (*EnhancedRuntimeManager)(nil)