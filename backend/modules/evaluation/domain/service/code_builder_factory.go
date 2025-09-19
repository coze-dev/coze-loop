// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// UserCodeBuilder 用户代码构建器接口
type UserCodeBuilder interface {
	// GetLanguageType 获取支持的语言类型
	GetLanguageType() entity.LanguageType
	// BuildCode 构建可执行的代码
	BuildCode(input *entity.EvaluatorInputData, codeVersion *entity.CodeEvaluatorVersion) (string, error)
	// BuildSyntaxCheckCode 构建语法检查代码
	BuildSyntaxCheckCode(userCode string) string
	// SetRuntime 设置运行时实例
	SetRuntime(runtime component.IRuntime)
}

// CodeBuilderFactory 代码构建器工厂接口
type CodeBuilderFactory interface {
	// CreateBuilder 根据语言类型创建代码构建器
	CreateBuilder(languageType entity.LanguageType) (UserCodeBuilder, error)
	// GetSupportedLanguages 获取支持的语言类型列表
	GetSupportedLanguages() []entity.LanguageType
	// SetRuntimeManager 设置运行时管理器（用于依赖注入runtime）
	SetRuntimeManager(manager component.IRuntimeManager)
}

// CodeBuilderFactoryImpl 代码构建器工厂实现
type CodeBuilderFactoryImpl struct {
	runtimeManager component.IRuntimeManager
}

// NewCodeBuilderFactory 创建代码构建器工厂实例
func NewCodeBuilderFactory() CodeBuilderFactory {
	return &CodeBuilderFactoryImpl{}
}

// SetRuntimeManager 设置运行时管理器
func (f *CodeBuilderFactoryImpl) SetRuntimeManager(manager component.IRuntimeManager) {
	f.runtimeManager = manager
}

// CreateBuilder 根据语言类型创建代码构建器
func (f *CodeBuilderFactoryImpl) CreateBuilder(languageType entity.LanguageType) (UserCodeBuilder, error) {
	var builder UserCodeBuilder
	
	switch languageType {
	case entity.LanguageTypeJS:
		builder = NewJavaScriptCodeBuilder()
	case entity.LanguageTypePython:
		builder = NewPythonCodeBuilder()
	default:
		return nil, fmt.Errorf("unsupported language type: %s", languageType)
	}
	
	// 如果有运行时管理器，为构建器注入相应的runtime
	if f.runtimeManager != nil {
		runtime, err := f.runtimeManager.GetRuntime(languageType)
		if err == nil {
			builder.SetRuntime(runtime)
		}
	}
	
	return builder, nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (f *CodeBuilderFactoryImpl) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{
		entity.LanguageTypeJS,
		entity.LanguageTypePython,
	}
}