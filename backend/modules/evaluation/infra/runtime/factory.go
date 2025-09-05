// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// RuntimeFactoryImpl Runtime工厂实现
type RuntimeFactoryImpl struct {
	logger        *logrus.Logger
	sandboxConfig *entity.SandboxConfig
}

// NewRuntimeFactory 创建Runtime工厂实例
func NewRuntimeFactory(logger *logrus.Logger, sandboxConfig *entity.SandboxConfig) component.IRuntimeFactory {
	if sandboxConfig == nil {
		sandboxConfig = entity.DefaultSandboxConfig()
	}
	
	// 默认使用统一运行时工厂（整合了所有运行时功能）
	return NewUnifiedRuntimeFactory(logger, sandboxConfig)
}

// CreateRuntime 根据语言类型创建Runtime实例（已废弃，使用统一运行时）
func (f *RuntimeFactoryImpl) CreateRuntime(languageType entity.LanguageType) (component.IRuntime, error) {
	// 这个实现已经被废弃，统一使用 UnifiedRuntimeFactory
	// 为了向后兼容，这里返回错误提示
	return nil, fmt.Errorf("RuntimeFactoryImpl已废弃，请使用NewRuntimeFactory或NewUnifiedRuntimeFactory创建工厂")
}

// GetSupportedLanguages 获取支持的语言类型列表
func (f *RuntimeFactoryImpl) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}
}

// 确保RuntimeFactoryImpl实现IRuntimeFactory接口
var _ component.IRuntimeFactory = (*RuntimeFactoryImpl)(nil)