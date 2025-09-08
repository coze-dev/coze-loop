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
)

// RuntimeFactory 简化的运行时工厂实现
type RuntimeFactory struct {
	logger        *logrus.Logger
	sandboxConfig *entity.SandboxConfig
	runtime       component.IRuntime // 单例运行时实例
}

// NewRuntimeFactory 创建Runtime工厂实例
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
	}
}

// CreateRuntime 根据语言类型创建Runtime实例
func (f *RuntimeFactory) CreateRuntime(languageType entity.LanguageType) (component.IRuntime, error) {
	// 使用单例模式，所有语言类型共享同一个运行时实例
	if f.runtime == nil {
		var err error
		
		// 检查是否使用HTTP FaaS模式
		faasURL := os.Getenv("COZE_LOOP_FAAS_URL")
		if faasURL == "" {
			// 默认使用Docker Compose中的FaaS服务
			faasURL = "http://coze-loop-faas:8000"
		}
		
		// 使用HTTP FaaS运行时
		config := &HTTPFaaSRuntimeConfig{
			BaseURL:        faasURL,
			Timeout:        30 * time.Second,
			MaxRetries:     3,
			RetryInterval:  1 * time.Second,
			EnableEnhanced: true,
		}
		f.runtime, err = NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, config, f.logger)
		
		if err != nil {
			return nil, fmt.Errorf("创建运行时失败: %w", err)
		}
	}
	
	// 验证是否支持请求的语言类型
	supported := false
	for _, supportedLang := range f.GetSupportedLanguages() {
		if supportedLang == languageType {
			supported = true
			break
		}
	}
	
	if !supported {
		return nil, fmt.Errorf("不支持的语言类型: %s", languageType)
	}
	
	return f.runtime, nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (f *RuntimeFactory) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}
}

// 确保RuntimeFactory实现IRuntimeFactory接口
var _ component.IRuntimeFactory = (*RuntimeFactory)(nil)