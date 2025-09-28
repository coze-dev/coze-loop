// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// StubRuntime 是一个简单的运行时存根实现，用于替代被删除的 runtime 包
type StubRuntime struct {
	languageType entity.LanguageType
}

// NewStubRuntime 创建一个新的存根运行时实例
func NewStubRuntime(languageType entity.LanguageType) *StubRuntime {
	return &StubRuntime{
		languageType: languageType,
	}
}

// RunCode 在沙箱中执行文本格式的代码（存根实现）
func (r *StubRuntime) RunCode(ctx context.Context, code, language string, timeoutMS int64, ext map[string]string) (*entity.ExecutionResult, error) {
	// 这是一个存根实现，实际的代码执行功能已被移除
	return &entity.ExecutionResult{
		Output: &entity.ExecutionOutput{
			Stderr: "Runtime functionality has been removed",
		},
	}, fmt.Errorf("runtime functionality has been removed")
}

// GetLanguageType 获取支持的语言类型
func (r *StubRuntime) GetLanguageType() entity.LanguageType {
	return r.languageType
}

// GetReturnValFunction 获取语言特定的return_val函数实现
func (r *StubRuntime) GetReturnValFunction() string {
	switch r.languageType {
	case entity.LanguageTypePython:
		return `
# return_val函数实现
def return_val(value):
    """
    标准return_val函数实现 - 设置返回值到ret_val字段
    Args:
        value: 要返回的值，通常是JSON字符串
    """
    # 这里不使用print，而是设置一个全局变量
    # 该变量会被FaaS服务器捕获到ret_val字段
    global _return_val_output
    _return_val_output = value
`
	case entity.LanguageTypeJS:
		return `
// return_val函数实现
function return_val(value) {
    /**
     * 标准return_val函数实现 - 输出返回值供FaaS服务捕获
     * @param {string} value - 要返回的值，通常是JSON字符串
     */
    console.log(value);
}
`
	default:
		return ""
	}
}

// StubRuntimeFactory 是一个简单的运行时工厂存根实现
type StubRuntimeFactory struct {
	logger        *logrus.Logger
	sandboxConfig *entity.SandboxConfig
}

// NewStubRuntimeFactory 创建一个新的存根运行时工厂实例
func NewStubRuntimeFactory(logger *logrus.Logger, sandboxConfig *entity.SandboxConfig) component.IRuntimeFactory {
	return &StubRuntimeFactory{
		logger:        logger,
		sandboxConfig: sandboxConfig,
	}
}

// CreateRuntime 根据语言类型创建Runtime实例（存根实现）
func (f *StubRuntimeFactory) CreateRuntime(languageType entity.LanguageType) (component.IRuntime, error) {
	return NewStubRuntime(languageType), nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (f *StubRuntimeFactory) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}
}

// StubRuntimeManager 是一个简单的运行时管理器存根实现
type StubRuntimeManager struct {
	factory component.IRuntimeFactory
	logger  *logrus.Logger
	cache   map[entity.LanguageType]component.IRuntime
}

// NewStubRuntimeManager 创建一个新的存根运行时管理器实例
func NewStubRuntimeManager(factory component.IRuntimeFactory, logger *logrus.Logger) component.IRuntimeManager {
	return &StubRuntimeManager{
		factory: factory,
		logger:  logger,
		cache:   make(map[entity.LanguageType]component.IRuntime),
	}
}

// GetRuntime 获取指定语言类型的Runtime实例
func (m *StubRuntimeManager) GetRuntime(languageType entity.LanguageType) (component.IRuntime, error) {
	if runtime, exists := m.cache[languageType]; exists {
		return runtime, nil
	}

	runtime, err := m.factory.CreateRuntime(languageType)
	if err != nil {
		return nil, err
	}

	m.cache[languageType] = runtime
	return runtime, nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (m *StubRuntimeManager) GetSupportedLanguages() []entity.LanguageType {
	return m.factory.GetSupportedLanguages()
}

// ClearCache 清空缓存
func (m *StubRuntimeManager) ClearCache() {
	m.cache = make(map[entity.LanguageType]component.IRuntime)
}
