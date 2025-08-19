// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"sync"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// RuntimeManager Runtime管理器，提供线程安全的Runtime实例缓存和管理
type RuntimeManager struct {
	factory component.IRuntimeFactory
	cache   map[entity.LanguageType]component.IRuntime
	mutex   sync.RWMutex
}

// NewRuntimeManager 创建RuntimeManager实例
func NewRuntimeManager(factory component.IRuntimeFactory) *RuntimeManager {
	return &RuntimeManager{
		factory: factory,
		cache:   make(map[entity.LanguageType]component.IRuntime),
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
		return nil, err
	}

	// 缓存Runtime实例
	m.cache[languageType] = runtime
	return runtime, nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (m *RuntimeManager) GetSupportedLanguages() []entity.LanguageType {
	return m.factory.GetSupportedLanguages()
}

// ClearCache 清空缓存（主要用于测试）
func (m *RuntimeManager) ClearCache() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.cache = make(map[entity.LanguageType]component.IRuntime)
}

// 确保RuntimeManager实现IRuntimeManager接口
var _ component.IRuntimeManager = (*RuntimeManager)(nil)
