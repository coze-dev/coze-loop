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

// UnifiedRuntimeFactory 统一的运行时工厂实现
type UnifiedRuntimeFactory struct {
	logger        *logrus.Logger
	sandboxConfig *entity.SandboxConfig
	runtimeCache  map[entity.LanguageType]component.IRuntime
	mutex         sync.RWMutex
	unifiedRuntime component.IRuntime // 单例统一运行时
}

// NewUnifiedRuntimeFactory 创建统一运行时工厂实例
func NewUnifiedRuntimeFactory(logger *logrus.Logger, sandboxConfig *entity.SandboxConfig) component.IRuntimeFactory {
	if sandboxConfig == nil {
		sandboxConfig = entity.DefaultSandboxConfig()
	}
	
	if logger == nil {
		logger = logrus.New()
	}
	
	return &UnifiedRuntimeFactory{
		logger:        logger,
		sandboxConfig: sandboxConfig,
		runtimeCache:  make(map[entity.LanguageType]component.IRuntime),
	}
}

// CreateRuntime 根据语言类型创建Runtime实例
func (f *UnifiedRuntimeFactory) CreateRuntime(languageType entity.LanguageType) (component.IRuntime, error) {
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
	if f.unifiedRuntime == nil {
		unifiedRuntime, err := NewUnifiedRuntime(f.sandboxConfig, f.logger)
		if err != nil {
			return nil, fmt.Errorf("创建统一运行时失败: %w", err)
		}
		f.unifiedRuntime = unifiedRuntime
		
		f.logger.WithFields(logrus.Fields{
			"supported_languages": unifiedRuntime.GetSupportedLanguages(),
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
	f.runtimeCache[languageType] = f.unifiedRuntime
	
	return f.unifiedRuntime, nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (f *UnifiedRuntimeFactory) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}
}

// Cleanup 清理工厂资源
func (f *UnifiedRuntimeFactory) Cleanup() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	
	if f.unifiedRuntime != nil {
		if cleanupRuntime, ok := f.unifiedRuntime.(interface{ Cleanup() error }); ok {
			if err := cleanupRuntime.Cleanup(); err != nil {
				return fmt.Errorf("清理统一运行时失败: %w", err)
			}
		}
		f.unifiedRuntime = nil
	}
	
	// 清空缓存
	f.runtimeCache = make(map[entity.LanguageType]component.IRuntime)
	
	return nil
}

// GetHealthStatus 获取工厂健康状态
func (f *UnifiedRuntimeFactory) GetHealthStatus() map[string]interface{} {
	status := map[string]interface{}{
		"status":             "healthy",
		"supported_languages": f.GetSupportedLanguages(),
		"cache_size":         len(f.runtimeCache),
	}
	
	if f.unifiedRuntime != nil {
		if healthRuntime, ok := f.unifiedRuntime.(interface{ GetHealthStatus() map[string]interface{} }); ok {
			status["runtime_health"] = healthRuntime.GetHealthStatus()
		}
	}
	
	return status
}

// 确保UnifiedRuntimeFactory实现IRuntimeFactory接口
var _ component.IRuntimeFactory = (*UnifiedRuntimeFactory)(nil)