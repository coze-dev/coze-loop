// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// UserCodeBuilder 用户代码构建器接口
type UserCodeBuilder interface {
	// BuildCode 构建可执行代码
	BuildCode(input *entity.EvaluatorInputData, codeVersion *entity.CodeEvaluatorVersion) (string, error)
	// GetLanguageType 获取支持的语言类型
	GetLanguageType() entity.LanguageType
}

// CodeBuilderFactory 代码构建器工厂接口
type CodeBuilderFactory interface {
	// CreateBuilder 根据语言类型创建代码构建器
	CreateBuilder(languageType entity.LanguageType) (UserCodeBuilder, error)
	// GetSupportedLanguages 获取支持的语言类型列表
	GetSupportedLanguages() []entity.LanguageType
}

// CodeBuilderFactoryImpl 代码构建器工厂实现
type CodeBuilderFactoryImpl struct{}

// NewCodeBuilderFactory 创建代码构建器工厂实例
func NewCodeBuilderFactory() CodeBuilderFactory {
	return &CodeBuilderFactoryImpl{}
}

// CreateBuilder 根据语言类型创建代码构建器
func (f *CodeBuilderFactoryImpl) CreateBuilder(languageType entity.LanguageType) (UserCodeBuilder, error) {
	switch languageType {
	case entity.LanguageTypePython:
		return NewPythonCodeBuilder(), nil
	case entity.LanguageTypeJS:
		return NewJavaScriptCodeBuilder(), nil
	default:
		return nil, fmt.Errorf("unsupported language type: %s", languageType)
	}
}

// GetSupportedLanguages 获取支持的语言类型列表
func (f *CodeBuilderFactoryImpl) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}
}