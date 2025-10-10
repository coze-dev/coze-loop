// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestNewStubRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		languageType entity.LanguageType
		want         *StubRuntime
	}{
		{
			name:         "创建Python存根运行时",
			languageType: entity.LanguageTypePython,
			want: &StubRuntime{
				languageType: entity.LanguageTypePython,
			},
		},
		{
			name:         "创建JavaScript存根运行时",
			languageType: entity.LanguageTypeJS,
			want: &StubRuntime{
				languageType: entity.LanguageTypeJS,
			},
		},
		{
			name:         "创建未知语言类型存根运行时",
			languageType: entity.LanguageType("Unknown"),
			want: &StubRuntime{
				languageType: entity.LanguageType("Unknown"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := NewStubRuntime(tt.languageType)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestStubRuntime_RunCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		languageType entity.LanguageType
		code         string
		language     string
		timeoutMS    int64
		ext          map[string]string
		wantOutput   string
		wantErr      bool
	}{
		{
			name:         "Python代码执行存根",
			languageType: entity.LanguageTypePython,
			code:         "print('hello world')",
			language:     "python",
			timeoutMS:    5000,
			ext:          map[string]string{"key": "value"},
			wantOutput:   "Runtime functionality has been removed",
			wantErr:      true,
		},
		{
			name:         "JavaScript代码执行存根",
			languageType: entity.LanguageTypeJS,
			code:         "console.log('hello world')",
			language:     "javascript",
			timeoutMS:    3000,
			ext:          nil,
			wantOutput:   "Runtime functionality has been removed",
			wantErr:      true,
		},
		{
			name:         "空代码执行存根",
			languageType: entity.LanguageTypePython,
			code:         "",
			language:     "python",
			timeoutMS:    1000,
			ext:          map[string]string{},
			wantOutput:   "Runtime functionality has been removed",
			wantErr:      true,
		},
		{
			name:         "超时时间为0",
			languageType: entity.LanguageTypeJS,
			code:         "var x = 1;",
			language:     "javascript",
			timeoutMS:    0,
			ext:          nil,
			wantOutput:   "Runtime functionality has been removed",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runtime := NewStubRuntime(tt.languageType)
			result, err := runtime.RunCode(context.Background(), tt.code, tt.language, tt.timeoutMS, tt.ext)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "runtime functionality has been removed")
			} else {
				assert.NoError(t, err)
			}

			assert.NotNil(t, result)
			assert.NotNil(t, result.Output)
			assert.Equal(t, tt.wantOutput, result.Output.Stderr)
		})
	}
}

func TestStubRuntime_GetLanguageType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		languageType entity.LanguageType
		want         entity.LanguageType
	}{
		{
			name:         "获取Python语言类型",
			languageType: entity.LanguageTypePython,
			want:         entity.LanguageTypePython,
		},
		{
			name:         "获取JavaScript语言类型",
			languageType: entity.LanguageTypeJS,
			want:         entity.LanguageTypeJS,
		},
		{
			name:         "获取未知语言类型",
			languageType: entity.LanguageType("Unknown"),
			want:         entity.LanguageType("Unknown"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runtime := NewStubRuntime(tt.languageType)
			result := runtime.GetLanguageType()
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestStubRuntime_GetReturnValFunction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		languageType entity.LanguageType
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:         "Python return_val函数",
			languageType: entity.LanguageTypePython,
			wantContains: []string{
				"def return_val(value):",
				"global _return_val_output",
				"_return_val_output = value",
			},
			wantEmpty: false,
		},
		{
			name:         "JavaScript return_val函数",
			languageType: entity.LanguageTypeJS,
			wantContains: []string{
				"function return_val(value)",
				"console.log(value);",
			},
			wantEmpty: false,
		},
		{
			name:         "不支持的语言类型",
			languageType: entity.LanguageType("Unknown"),
			wantContains: nil,
			wantEmpty:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runtime := NewStubRuntime(tt.languageType)
			result := runtime.GetReturnValFunction()

			if tt.wantEmpty {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
				for _, contain := range tt.wantContains {
					assert.Contains(t, result, contain)
				}
			}
		})
	}
}

func TestNewStubRuntimeFactory(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	sandboxConfig := &entity.SandboxConfig{
		MemoryLimit:  512,
		TimeoutLimit: 5000,
	}

	factory := NewStubRuntimeFactory(logger, sandboxConfig)
	assert.NotNil(t, factory)

	stubFactory, ok := factory.(*StubRuntimeFactory)
	assert.True(t, ok)
	assert.Equal(t, logger, stubFactory.logger)
	assert.Equal(t, sandboxConfig, stubFactory.sandboxConfig)
}

func TestStubRuntimeFactory_CreateRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		languageType entity.LanguageType
		wantErr      bool
	}{
		{
			name:         "创建Python运行时",
			languageType: entity.LanguageTypePython,
			wantErr:      false,
		},
		{
			name:         "创建JavaScript运行时",
			languageType: entity.LanguageTypeJS,
			wantErr:      false,
		},
		{
			name:         "创建未知语言类型运行时",
			languageType: entity.LanguageType("Unknown"),
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			factory := NewStubRuntimeFactory(logrus.New(), &entity.SandboxConfig{})
			runtime, err := factory.CreateRuntime(tt.languageType)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, runtime)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, runtime)

				stubRuntime, ok := runtime.(*StubRuntime)
				assert.True(t, ok)
				assert.Equal(t, tt.languageType, stubRuntime.languageType)
			}
		})
	}
}

func TestStubRuntimeFactory_GetSupportedLanguages(t *testing.T) {
	t.Parallel()

	factory := NewStubRuntimeFactory(logrus.New(), &entity.SandboxConfig{})
	languages := factory.GetSupportedLanguages()

	expected := []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}

	assert.Equal(t, expected, languages)
	assert.Len(t, languages, 2)
	assert.Contains(t, languages, entity.LanguageTypePython)
	assert.Contains(t, languages, entity.LanguageTypeJS)
}

func TestNewStubRuntimeManager(t *testing.T) {
	t.Parallel()

	factory := NewStubRuntimeFactory(logrus.New(), &entity.SandboxConfig{})
	logger := logrus.New()

	manager := NewStubRuntimeManager(factory, logger)
	assert.NotNil(t, manager)

	stubManager, ok := manager.(*StubRuntimeManager)
	assert.True(t, ok)
	assert.Equal(t, factory, stubManager.factory)
	assert.Equal(t, logger, stubManager.logger)
	assert.NotNil(t, stubManager.cache)
}

func TestStubRuntimeManager_GetRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		languageType entity.LanguageType
		wantErr      bool
	}{
		{
			name:         "获取Python运行时",
			languageType: entity.LanguageTypePython,
			wantErr:      false,
		},
		{
			name:         "获取JavaScript运行时",
			languageType: entity.LanguageTypeJS,
			wantErr:      false,
		},
		{
			name:         "获取未知语言类型运行时",
			languageType: entity.LanguageType("Unknown"),
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			factory := NewStubRuntimeFactory(logrus.New(), &entity.SandboxConfig{})
			manager := NewStubRuntimeManager(factory, logrus.New())

			// 第一次获取
			runtime1, err1 := manager.GetRuntime(tt.languageType)
			if tt.wantErr {
				assert.Error(t, err1)
				assert.Nil(t, runtime1)
				return
			}

			assert.NoError(t, err1)
			assert.NotNil(t, runtime1)

			// 第二次获取，应该返回缓存的实例
			runtime2, err2 := manager.GetRuntime(tt.languageType)
			assert.NoError(t, err2)
			assert.NotNil(t, runtime2)
			assert.Same(t, runtime1, runtime2) // 验证是同一个实例

			// 验证运行时类型
			stubRuntime, ok := runtime1.(*StubRuntime)
			assert.True(t, ok)
			assert.Equal(t, tt.languageType, stubRuntime.languageType)
		})
	}
}

func TestStubRuntimeManager_GetSupportedLanguages(t *testing.T) {
	t.Parallel()

	factory := NewStubRuntimeFactory(logrus.New(), &entity.SandboxConfig{})
	manager := NewStubRuntimeManager(factory, logrus.New())

	languages := manager.GetSupportedLanguages()

	expected := []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}

	assert.Equal(t, expected, languages)
	assert.Len(t, languages, 2)
}

func TestStubRuntimeManager_ClearCache(t *testing.T) {
	t.Parallel()

	factory := NewStubRuntimeFactory(logrus.New(), &entity.SandboxConfig{})
	manager := NewStubRuntimeManager(factory, logrus.New())

	// 先获取一个运行时实例，填充缓存
	runtime1, err := manager.GetRuntime(entity.LanguageTypePython)
	assert.NoError(t, err)
	assert.NotNil(t, runtime1)

	// 验证缓存中有实例
	stubManager := manager.(*StubRuntimeManager)
	assert.Len(t, stubManager.cache, 1)

	// 清空缓存
	manager.ClearCache()
	assert.Len(t, stubManager.cache, 0)

	// 再次获取，应该创建新的实例
	runtime2, err := manager.GetRuntime(entity.LanguageTypePython)
	assert.NoError(t, err)
	assert.NotNil(t, runtime2)
	assert.NotSame(t, runtime1, runtime2) // 验证不是同一个实例
}

func TestStubRuntimeIntegration(t *testing.T) {
	t.Parallel()

	// 集成测试：测试整个存根运行时系统的协作
	logger := logrus.New()
	sandboxConfig := &entity.SandboxConfig{
		MemoryLimit:  512,
		TimeoutLimit: 5000,
	}

	// 创建工厂
	factory := NewStubRuntimeFactory(logger, sandboxConfig)

	// 创建管理器
	manager := NewStubRuntimeManager(factory, logger)

	// 测试支持的语言
	languages := manager.GetSupportedLanguages()
	assert.Contains(t, languages, entity.LanguageTypePython)
	assert.Contains(t, languages, entity.LanguageTypeJS)

	// 测试Python运行时
	pythonRuntime, err := manager.GetRuntime(entity.LanguageTypePython)
	assert.NoError(t, err)
	assert.NotNil(t, pythonRuntime)

	pythonResult, err := pythonRuntime.RunCode(context.Background(), "print('test')", "python", 5000, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "runtime functionality has been removed")
	assert.NotNil(t, pythonResult)
	assert.Equal(t, "Runtime functionality has been removed", pythonResult.Output.Stderr)

	pythonReturnVal := pythonRuntime.GetReturnValFunction()
	assert.Contains(t, pythonReturnVal, "def return_val(value):")

	// 测试JavaScript运行时
	jsRuntime, err := manager.GetRuntime(entity.LanguageTypeJS)
	assert.NoError(t, err)
	assert.NotNil(t, jsRuntime)

	jsResult, err := jsRuntime.RunCode(context.Background(), "console.log('test')", "javascript", 3000, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "runtime functionality has been removed")
	assert.NotNil(t, jsResult)

	jsReturnVal := jsRuntime.GetReturnValFunction()
	assert.Contains(t, jsReturnVal, "function return_val(value)")

	// 测试缓存机制
	pythonRuntime2, err := manager.GetRuntime(entity.LanguageTypePython)
	assert.NoError(t, err)
	assert.Same(t, pythonRuntime, pythonRuntime2)

	// 测试清空缓存
	manager.ClearCache()
	pythonRuntime3, err := manager.GetRuntime(entity.LanguageTypePython)
	assert.NoError(t, err)
	assert.NotSame(t, pythonRuntime, pythonRuntime3)
}

func TestStubRuntimeErrorHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupFunc    func() (component.IRuntime, error)
		testFunc     func(runtime component.IRuntime) error
		wantErr      bool
		errorMessage string
	}{
		{
			name: "运行时执行总是返回错误",
			setupFunc: func() (component.IRuntime, error) {
				return NewStubRuntime(entity.LanguageTypePython), nil
			},
			testFunc: func(runtime component.IRuntime) error {
				_, err := runtime.RunCode(context.Background(), "print('test')", "python", 5000, nil)
				return err
			},
			wantErr:      true,
			errorMessage: "runtime functionality has been removed",
		},
		{
			name: "不支持的语言类型返回空函数",
			setupFunc: func() (component.IRuntime, error) {
				return NewStubRuntime(entity.LanguageType("Unknown")), nil
			},
			testFunc: func(runtime component.IRuntime) error {
				result := runtime.GetReturnValFunction()
				if result != "" {
					return assert.AnError
				}
				return nil
			},
			wantErr:      false,
			errorMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runtime, err := tt.setupFunc()
			assert.NoError(t, err)
			assert.NotNil(t, runtime)

			err = tt.testFunc(runtime)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
