// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"fmt"

	"github.com/coze-dev/coze-loop/backend/infra/http"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// RuntimeFactoryImpl Runtime工厂实现
type RuntimeFactoryImpl struct {
	httpClient http.IClient
}

// NewRuntimeFactory 创建Runtime工厂实例
func NewRuntimeFactory(httpClient http.IClient) component.IRuntimeFactory {
	return &RuntimeFactoryImpl{
		httpClient: httpClient,
	}
}

// CreateRuntime 根据语言类型创建Runtime实例
func (f *RuntimeFactoryImpl) CreateRuntime(languageType entity.LanguageType) (component.IRuntime, error) {
	switch languageType {
	case entity.LanguageTypePython:
		return NewPythonRuntime(f.httpClient), nil
	case entity.LanguageTypeJS:
		return NewJavaScriptRuntime(f.httpClient), nil
	default:
		return nil, fmt.Errorf("unsupported language type: %s", languageType)
	}
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
