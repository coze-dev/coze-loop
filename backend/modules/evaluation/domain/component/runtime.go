// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/runtime.go -package=mocks . IRuntime,IRuntimeManager,IRuntimeFactory

// IRuntime 代码执行沙箱接口
type IRuntime interface {
	// RunCode 在沙箱中执行文本格式的代码
	RunCode(ctx context.Context, code string, language string, timeoutMS int64) (*entity.ExecutionResult, error)
	// GetLanguageType 获取支持的语言类型
	GetLanguageType() entity.LanguageType
}

// IRuntimeManager Runtime管理器接口
type IRuntimeManager interface {
	// GetRuntime 获取指定语言类型的Runtime实例
	GetRuntime(languageType entity.LanguageType) (IRuntime, error)
	// GetSupportedLanguages 获取支持的语言类型列表
	GetSupportedLanguages() []entity.LanguageType
	// ClearCache 清空缓存
	ClearCache()
}

// IRuntimeFactory Runtime工厂接口
type IRuntimeFactory interface {
	// CreateRuntime 根据语言类型创建Runtime实例
	CreateRuntime(languageType entity.LanguageType) (IRuntime, error)
	// GetSupportedLanguages 获取支持的语言类型列表
	GetSupportedLanguages() []entity.LanguageType
}