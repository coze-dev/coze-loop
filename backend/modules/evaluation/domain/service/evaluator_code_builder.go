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
	// BuildCode 构建可执行代码
	BuildCode(input *entity.EvaluatorInputData, codeVersion *entity.CodeEvaluatorVersion) (string, error)
	// BuildSyntaxCheckCode 构建语法检查代码
	BuildSyntaxCheckCode(userCode string) string
	// GetLanguageType 获取支持的语言类型
	GetLanguageType() entity.LanguageType
	// SetRuntime 设置运行时实例（用于获取return_val函数）
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
type CodeBuilderFactoryImpl struct{
	runtimeManager component.IRuntimeManager // 运行时管理器，用于获取runtime实例
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
	var err error
	
	switch languageType {
	case entity.LanguageTypePython:
		builder = NewPythonCodeBuilder()
	case entity.LanguageTypeJS:
		builder = NewJavaScriptCodeBuilder()
	default:
		return nil, fmt.Errorf("unsupported language type: %s", languageType)
	}
	
	// 如果有runtimeManager，尝试获取对应的runtime并设置到builder中
	if f.runtimeManager != nil {
		if runtime, runtimeErr := f.runtimeManager.GetRuntime(languageType); runtimeErr == nil {
			builder.SetRuntime(runtime)
		}
		// 如果获取runtime失败，不影响builder的创建，只是无法使用runtime的return_val函数
	}
	
	return builder, err
}

// GetSupportedLanguages 获取支持的语言类型列表
func (f *CodeBuilderFactoryImpl) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}
}