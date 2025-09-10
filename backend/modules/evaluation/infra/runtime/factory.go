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
	runtime component.IRuntime // 单例统一运行时
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

	// 对于统一运行时，我们使用单例模式
	// 因为统一运行时内部已经可以处理多种语言
	if f.runtime == nil {
		runtime, err := NewRuntime(f.sandboxConfig, f.logger)
		if err != nil {
			return nil, fmt.Errorf("创建统一运行时失败: %w", err)
		}
		f.runtime = runtime
		
		f.logger.WithFields(logrus.Fields{
			"supported_languages": runtime.GetSupportedLanguages(),
		}).Info("统一运行时创建成功")
	}
	
	// 检查是否支持请求的语言类型
	supported := false
	for _, supportedLang := range f.GetSupportedLanguages() {
		if supportedLang == languageType {
			supported = true
			break
		}
	}
	
	if !supported {
		return nil, fmt.Errorf("统一运行时不支持语言类型: %s", languageType)
	}
	
	// 缓存运行时实例（所有语言共享同一个实例）
	f.runtimeCache[languageType] = f.runtime
	
	return f.runtime, nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (f *RuntimeFactory) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}
}

// Cleanup 清理工厂资源
func (f *RuntimeFactory) Cleanup() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	if f.runtime != nil {
		if cleanupRuntime, ok := f.runtime.(interface{ Cleanup() error }); ok {
			if err := cleanupRuntime.Cleanup(); err != nil {
				return fmt.Errorf("清理统一运行时失败: %w", err)
			}
		}
		f.runtime = nil
	}
	
	// 清空缓存
	f.runtimeCache = make(map[entity.LanguageType]component.IRuntime)
	
	return nil
}

// GetHealthStatus 获取工厂健康状态
func (f *RuntimeFactory) GetHealthStatus() map[string]interface{} {
	status := map[string]interface{}{
		"status":             "healthy",
		"supported_languages": f.GetSupportedLanguages(),
		"cache_size":         len(f.runtimeCache),
	}
	
	if f.runtime != nil {
		if healthRuntime, ok := f.runtime.(interface{ GetHealthStatus() map[string]interface{} }); ok {
			status["runtime_health"] = healthRuntime.GetHealthStatus()
		}
	}
	
	return status
}

// 确保RuntimeFactory实现IRuntimeFactory接口
var _ component.IRuntimeFactory = (*RuntimeFactory)(nil)