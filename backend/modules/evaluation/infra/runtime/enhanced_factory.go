// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/runtime/enhanced"
)

// EnhancedRuntimeFactory 增强版Runtime工厂实现
type EnhancedRuntimeFactory struct {
	logger        *logrus.Logger
	sandboxConfig *entity.SandboxConfig
	enhancedRuntime component.IRuntime // 单例增强运行时
}

// NewEnhancedRuntimeFactory 创建增强版Runtime工厂实例
func NewEnhancedRuntimeFactory(logger *logrus.Logger, sandboxConfig *entity.SandboxConfig) component.IRuntimeFactory {
	if sandboxConfig == nil {
		sandboxConfig = entity.DefaultSandboxConfig()
	}
	
	return &EnhancedRuntimeFactory{
		logger:        logger,
		sandboxConfig: sandboxConfig,
	}
}

// CreateRuntime 根据语言类型创建Runtime实例
func (f *EnhancedRuntimeFactory) CreateRuntime(languageType entity.LanguageType) (component.IRuntime, error) {
	// 检查是否启用HTTP FaaS模式
	faasURL := os.Getenv("COZE_LOOP_FAAS_URL")
	if faasURL != "" {
		// 使用HTTP FaaS运行时
		f.logger.WithFields(logrus.Fields{
			"language_type": languageType,
			"faas_url":      faasURL,
		}).Info("使用HTTP FaaS运行时")

		config := &HTTPFaaSRuntimeConfig{
			BaseURL:        faasURL,
			Timeout:        30 * time.Second,
			MaxRetries:     3,
			RetryInterval:  1 * time.Second,
			EnableEnhanced: true,
		}

		return NewHTTPFaaSRuntimeAdapter(languageType, config, f.logger)
	}

	// 对于增强版运行时，我们使用单例模式
	// 因为增强运行时内部已经包含了沙箱池和任务调度器，可以处理多种语言
	if f.enhancedRuntime == nil {
		enhancedRuntime, err := enhanced.NewEnhancedRuntime(f.sandboxConfig, f.logger)
		if err != nil {
			return nil, fmt.Errorf("创建增强运行时失败: %w", err)
		}
		f.enhancedRuntime = enhancedRuntime
		
		f.logger.WithFields(logrus.Fields{
			"supported_languages": enhancedRuntime.GetSupportedLanguages(),
		}).Info("增强运行时创建成功")
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
		return nil, fmt.Errorf("增强运行时不支持语言类型: %s", languageType)
	}
	
	return f.enhancedRuntime, nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (f *EnhancedRuntimeFactory) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}
}

// Cleanup 清理工厂资源
func (f *EnhancedRuntimeFactory) Cleanup() error {
	if f.enhancedRuntime != nil {
		if cleanupRuntime, ok := f.enhancedRuntime.(interface{ Cleanup() error }); ok {
			if err := cleanupRuntime.Cleanup(); err != nil {
				return fmt.Errorf("清理增强运行时失败: %w", err)
			}
		}
		f.enhancedRuntime = nil
	}
	return nil
}

// 确保EnhancedRuntimeFactory实现IRuntimeFactory接口
var _ component.IRuntimeFactory = (*EnhancedRuntimeFactory)(nil)