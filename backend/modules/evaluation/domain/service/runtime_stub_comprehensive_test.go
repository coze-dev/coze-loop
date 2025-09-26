// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestNewStubRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		languageType entity.LanguageType
	}{
		{
			name:         "Python运行时",
			languageType: entity.LanguageTypePython,
		},
		{
			name:         "JavaScript运行时",
			languageType: entity.LanguageTypeJS,
		},
		{
			name:         "自定义语言类型",
			languageType: "CustomLang",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runtime := NewStubRuntime(tt.languageType)

			assert.NotNil(t, runtime)
			assert.Equal(t, tt.languageType, runtime.languageType)
			assert.Equal(t, tt.languageType, runtime.GetLanguageType())
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
		expectErr    bool
	}{
		{
			name:         "Python代码执行",
			languageType: entity.LanguageTypePython,
			code:         "print('hello world')",
			language:     "python",
			timeoutMS:    5000,
			ext:          map[string]string{"key": "value"},
			expectErr:    true,
		},
		{
			name:         "JavaScript代码执行",
			languageType: entity.LanguageTypeJS,
			code:         "console.log('hello world')",
			language:     "javascript",
			timeoutMS:    3000,
			ext:          nil,
			expectErr:    true,
		},
		{
			name:         "空代码",
			languageType: entity.LanguageTypePython,
			code:         "",
			language:     "python",
			timeoutMS:    1000,
			ext:          make(map[string]string),
			expectErr:    true,
		},
		{
			name:         "长代码",
			languageType: entity.LanguageTypePython,
			code:         strings.Repeat("print('test')\n", 100),
			language:     "python",
			timeoutMS:    10000,
			ext:          map[string]string{"debug": "true"},
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runtime := NewStubRuntime(tt.languageType)
			ctx := context.Background()

			result, err := runtime.RunCode(ctx, tt.code, tt.language, tt.timeoutMS, tt.ext)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "runtime functionality has been removed")
			} else {
				assert.NoError(t, err)
			}

			// 验证返回结果结构
			assert.NotNil(t, result)
			assert.NotNil(t, result.Output)
			assert.Equal(t, "Runtime functionality has been removed", result.Output.Stderr)
			assert.Empty(t, result.Output.Stdout)
			assert.Empty(t, result.Output.RetVal)
		})
	}
}

func TestStubRuntime_GetLanguageType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		languageType entity.LanguageType
	}{
		{
			name:         "Python语言类型",
			languageType: entity.LanguageTypePython,
		},
		{
			name:         "JavaScript语言类型",
			languageType: entity.LanguageTypeJS,
		},
		{
			name:         "空语言类型",
			languageType: "",
		},
		{
			name:         "自定义语言类型",
			languageType: "Golang",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runtime := NewStubRuntime(tt.languageType)
			result := runtime.GetLanguageType()

			assert.Equal(t, tt.languageType, result)
		})
	}
}

func TestStubRuntime_GetReturnValFunction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		languageType entity.LanguageType
		expectEmpty  bool
		contains     []string
	}{
		{
			name:         "Python return_val函数",
			languageType: entity.LanguageTypePython,
			expectEmpty:  false,
			contains:     []string{"def return_val(value):", "global _return_val_output", "_return_val_output = value"},
		},
		{
			name:         "JavaScript return_val函数",
			languageType: entity.LanguageTypeJS,
			expectEmpty:  false,
			contains:     []string{"function return_val(value)", "console.log(value);"},
		},
		{
			name:         "不支持的语言类型",
			languageType: "Golang",
			expectEmpty:  true,
			contains:     nil,
		},
		{
			name:         "空语言类型",
			languageType: "",
			expectEmpty:  true,
			contains:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runtime := NewStubRuntime(tt.languageType)
			result := runtime.GetReturnValFunction()

			if tt.expectEmpty {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
				for _, expected := range tt.contains {
					assert.Contains(t, result, expected)
				}
			}
		})
	}
}

func TestStubRuntime_GetReturnValFunction_PythonDetails(t *testing.T) {
	t.Parallel()

	runtime := NewStubRuntime(entity.LanguageTypePython)
	result := runtime.GetReturnValFunction()

	// 验证Python函数的详细内容
	assert.Contains(t, result, "def return_val(value):")
	assert.Contains(t, result, "标准return_val函数实现")
	assert.Contains(t, result, "global _return_val_output")
	assert.Contains(t, result, "_return_val_output = value")
	assert.Contains(t, result, "Args:")
	assert.Contains(t, result, "value: 要返回的值，通常是JSON字符串")

	// 验证不包含print语句（注释中提到不使用print）
	assert.NotContains(t, result, "print(")
}

func TestStubRuntime_GetReturnValFunction_JavaScriptDetails(t *testing.T) {
	t.Parallel()

	runtime := NewStubRuntime(entity.LanguageTypeJS)
	result := runtime.GetReturnValFunction()

	// 验证JavaScript函数的详细内容
	assert.Contains(t, result, "function return_val(value)")
	assert.Contains(t, result, "标准return_val函数实现")
	assert.Contains(t, result, "console.log(value);")
	assert.Contains(t, result, "@param {string} value")
	assert.Contains(t, result, "要返回的值，通常是JSON字符串")
}

func TestNewStubRuntimeFactory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		logger        *logrus.Logger
		sandboxConfig *entity.SandboxConfig
	}{
		{
			name:          "完整参数",
			logger:        logrus.New(),
			sandboxConfig: &entity.SandboxConfig{},
		},
		{
			name:          "nil logger",
			logger:        nil,
			sandboxConfig: &entity.SandboxConfig{},
		},
		{
			name:          "nil sandboxConfig",
			logger:        logrus.New(),
			sandboxConfig: nil,
		},
		{
			name:          "所有参数为nil",
			logger:        nil,
			sandboxConfig: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			factory := NewStubRuntimeFactory(tt.logger, tt.sandboxConfig)

			assert.NotNil(t, factory)
			stubFactory, ok := factory.(*StubRuntimeFactory)
			assert.True(t, ok)
			assert.Equal(t, tt.logger, stubFactory.logger)
			assert.Equal(t, tt.sandboxConfig, stubFactory.sandboxConfig)
		})
	}
}

func TestStubRuntimeFactory_CreateRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		languageType entity.LanguageType
		expectErr    bool
	}{
		{
			name:         "创建Python运行时",
			languageType: entity.LanguageTypePython,
			expectErr:    false,
		},
		{
			name:         "创建JavaScript运行时",
			languageType: entity.LanguageTypeJS,
			expectErr:    false,
		},
		{
			name:         "创建自定义语言运行时",
			languageType: "CustomLang",
			expectErr:    false,
		},
		{
			name:         "创建空语言类型运行时",
			languageType: "",
			expectErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := logrus.New()
					sandboxConfig := &entity.SandboxConfig{}
			factory := NewStubRuntimeFactory(logger, sandboxConfig)

			runtime, err := factory.CreateRuntime(tt.languageType)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, runtime)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, runtime)
				assert.Equal(t, tt.languageType, runtime.GetLanguageType())
			}
		})
	}
}

func TestStubRuntimeFactory_GetSupportedLanguages(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	sandboxConfig := &entity.SandboxConfig{}
	factory := NewStubRuntimeFactory(logger, sandboxConfig)

	supportedLanguages := factory.GetSupportedLanguages()

	// 验证支持的语言列表
	assert.Len(t, supportedLanguages, 2)
	assert.Contains(t, supportedLanguages, entity.LanguageTypePython)
	assert.Contains(t, supportedLanguages, entity.LanguageTypeJS)

	// 验证顺序
	assert.Equal(t, entity.LanguageTypePython, supportedLanguages[0])
	assert.Equal(t, entity.LanguageTypeJS, supportedLanguages[1])
}

func TestStubRuntime_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	runtime := NewStubRuntime(entity.LanguageTypePython)
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	// 测试并发访问安全性
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			ctx := context.Background()
			code := "print('hello from goroutine')"
			ext := map[string]string{"goroutine_id": string(rune(id))}

			result, err := runtime.RunCode(ctx, code, "python", 1000, ext)

			assert.Error(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, entity.LanguageTypePython, runtime.GetLanguageType())

			returnValFunc := runtime.GetReturnValFunction()
			assert.NotEmpty(t, returnValFunc)
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestStubRuntime_ContextCancellation(t *testing.T) {
	t.Parallel()

	runtime := NewStubRuntime(entity.LanguageTypePython)

	// 测试context取消
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	result, err := runtime.RunCode(ctx, "print('test')", "python", 5000, nil)

	// 即使context被取消，存根实现仍然返回错误（因为功能被移除）
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, err.Error(), "runtime functionality has been removed")
}

func TestStubRuntimeFactory_Interface(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	sandboxConfig := &entity.SandboxConfig{}
	factory := NewStubRuntimeFactory(logger, sandboxConfig)

	// 验证工厂实现了正确的接口
	assert.NotNil(t, factory)

	// 测试创建运行时
	runtime, err := factory.CreateRuntime(entity.LanguageTypePython)
	assert.NoError(t, err)
	assert.NotNil(t, runtime)

	// 验证运行时实现了正确的接口
	assert.Equal(t, entity.LanguageTypePython, runtime.GetLanguageType())

	ctx := context.Background()
	result, err := runtime.RunCode(ctx, "test", "python", 1000, nil)
	assert.Error(t, err)
	assert.NotNil(t, result)

	returnValFunc := runtime.GetReturnValFunction()
	assert.NotEmpty(t, returnValFunc)
}

func TestStubRuntime_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		languageType entity.LanguageType
		code         string
		language     string
		timeoutMS    int64
		ext          map[string]string
	}{
		{
			name:         "超长代码",
			languageType: entity.LanguageTypePython,
			code:         strings.Repeat("a", 10000),
			language:     "python",
			timeoutMS:    1,
			ext:          nil,
		},
		{
			name:         "负数超时",
			languageType: entity.LanguageTypeJS,
			code:         "console.log('test')",
			language:     "javascript",
			timeoutMS:    -1000,
			ext:          map[string]string{},
		},
		{
			name:         "巨大的ext map",
			languageType: entity.LanguageTypePython,
			code:         "print('test')",
			language:     "python",
			timeoutMS:    5000,
			ext:          func() map[string]string {
				ext := make(map[string]string)
				for i := 0; i < 1000; i++ {
					ext[string(rune(i))] = strings.Repeat("value", 100)
				}
				return ext
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runtime := NewStubRuntime(tt.languageType)
			ctx := context.Background()

			result, err := runtime.RunCode(ctx, tt.code, tt.language, tt.timeoutMS, tt.ext)

			// 所有情况都应该返回错误（因为是存根实现）
			assert.Error(t, err)
			assert.NotNil(t, result)
			assert.Contains(t, err.Error(), "runtime functionality has been removed")
		})
	}
}

// 基准测试
func BenchmarkStubRuntime_RunCode(b *testing.B) {
	runtime := NewStubRuntime(entity.LanguageTypePython)
	ctx := context.Background()
	code := "print('hello world')"
	ext := map[string]string{"key": "value"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = runtime.RunCode(ctx, code, "python", 5000, ext)
	}
}

func BenchmarkStubRuntime_GetReturnValFunction(b *testing.B) {
	runtime := NewStubRuntime(entity.LanguageTypePython)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = runtime.GetReturnValFunction()
	}
}

func BenchmarkStubRuntimeFactory_CreateRuntime(b *testing.B) {
	logger := logrus.New()
	sandboxConfig := &entity.SandboxConfig{}
	factory := NewStubRuntimeFactory(logger, sandboxConfig)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = factory.CreateRuntime(entity.LanguageTypePython)
	}
}