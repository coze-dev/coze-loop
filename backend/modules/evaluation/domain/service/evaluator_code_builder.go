// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
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
