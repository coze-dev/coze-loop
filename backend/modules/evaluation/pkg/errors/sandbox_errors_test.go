// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSandboxError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *SandboxError
		expected string
	}{
		{
			name: "错误包含Cause",
			err: &SandboxError{
				Code:    "TEST_CODE",
				Message: "Test message",
				Cause:   errors.New("underlying error"),
			},
			expected: "[TEST_CODE] Test message: underlying error",
		},
		{
			name: "错误不包含Cause",
			err: &SandboxError{
				Code:    "TEST_CODE",
				Message: "Test message",
				Cause:   nil,
			},
			expected: "[TEST_CODE] Test message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSandboxError_Unwrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *SandboxError
		expected error
	}{
		{
			name: "有Cause的错误",
			err: &SandboxError{
				Code:    "TEST_CODE",
				Message: "Test message",
				Cause:   errors.New("underlying error"),
			},
			expected: errors.New("underlying error"),
		},
		{
			name: "无Cause的错误",
			err: &SandboxError{
				Code:    "TEST_CODE",
				Message: "Test message",
				Cause:   nil,
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.err.Unwrap()
			if tt.expected != nil {
				assert.Error(t, result)
				assert.Equal(t, tt.expected.Error(), result.Error())
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestNewSandboxError(t *testing.T) {
	t.Parallel()

	err := NewSandboxError("TEST_CODE", "Test message", errors.New("test cause"))
	assert.Equal(t, "TEST_CODE", err.Code)
	assert.Equal(t, "Test message", err.Message)
	assert.Error(t, err.Cause)
	assert.Equal(t, "test cause", err.Cause.Error())
}

func TestNewUnsupportedLanguageError(t *testing.T) {
	t.Parallel()

	err := NewUnsupportedLanguageError("python")
	assert.Equal(t, ErrCodeUnsupportedLanguage, err.Code)
	assert.Contains(t, err.Message, "python")
	assert.Nil(t, err.Cause)
}

func TestNewExecutionTimeoutError(t *testing.T) {
	t.Parallel()

	err := NewExecutionTimeoutError("30s")
	assert.Equal(t, ErrCodeExecutionTimeout, err.Code)
	assert.Contains(t, err.Message, "30s")
	assert.Nil(t, err.Cause)
}

func TestNewMemoryLimitError(t *testing.T) {
	t.Parallel()

	err := NewMemoryLimitError(512)
	assert.Equal(t, ErrCodeMemoryLimit, err.Code)
	assert.Contains(t, err.Message, "512MB")
	assert.Nil(t, err.Cause)
}

func TestNewSecurityViolationError(t *testing.T) {
	t.Parallel()

	err := NewSecurityViolationError("尝试访问文件系统")
	assert.Equal(t, ErrCodeSecurityViolation, err.Code)
	assert.Contains(t, err.Message, "尝试访问文件系统")
	assert.Nil(t, err.Cause)
}

func TestNewRuntimeError(t *testing.T) {
	t.Parallel()

	cause := errors.New("division by zero")
	err := NewRuntimeError("执行失败", cause)
	assert.Equal(t, ErrCodeRuntimeError, err.Code)
	assert.Equal(t, "执行失败", err.Message)
	assert.Equal(t, cause, err.Cause)
}

func TestNewInvalidCodeError(t *testing.T) {
	t.Parallel()

	err := NewInvalidCodeError("语法错误")
	assert.Equal(t, ErrCodeInvalidCode, err.Code)
	assert.Equal(t, "语法错误", err.Message)
	assert.Nil(t, err.Cause)
}

func TestNewSystemError(t *testing.T) {
	t.Parallel()

	cause := errors.New("out of memory")
	err := NewSystemError("系统资源不足", cause)
	assert.Equal(t, ErrCodeSystemError, err.Code)
	assert.Equal(t, "系统资源不足", err.Message)
	assert.Equal(t, cause, err.Cause)
}

func TestNewCompilationError(t *testing.T) {
	t.Parallel()

	err := NewCompilationError("编译失败", "syntax error details")
	assert.Equal(t, "编译失败", err.Message)
	assert.Equal(t, "syntax error details", err.Details)
	assert.Contains(t, err.Error(), "编译失败")
	assert.Contains(t, err.Error(), "syntax error details")
}

func TestNewSyntaxValidationError(t *testing.T) {
	t.Parallel()

	err := NewSyntaxValidationError("python", 10, "语法验证失败")
	assert.Equal(t, "python", err.Language)
	assert.Equal(t, 10, err.Line)
	assert.Equal(t, "语法验证失败", err.Message)
	assert.Contains(t, err.Error(), "python")
	assert.Contains(t, err.Error(), "第10行")
	assert.Contains(t, err.Error(), "语法验证失败")
}

func TestNewValidationError(t *testing.T) {
	t.Parallel()

	err := NewValidationError("参数验证失败")
	assert.Equal(t, ErrCodeInvalidCode, err.Code)
	assert.Equal(t, "参数验证失败", err.Message)
	assert.Nil(t, err.Cause)
}

func TestNewNotFoundError(t *testing.T) {
	t.Parallel()

	err := NewNotFoundError("资源未找到")
	assert.Equal(t, "NOT_FOUND", err.Code)
	assert.Equal(t, "资源未找到", err.Message)
	assert.Nil(t, err.Cause)
}

func TestIsNotFoundError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "是NotFound错误",
			err:      NewNotFoundError("资源未找到"),
			expected: true,
		},
		{
			name:     "不是NotFound错误",
			err:      NewSandboxError("OTHER_CODE", "其他错误", nil),
			expected: false,
		},
		{
			name:     "非SandboxError类型",
			err:      errors.New("普通错误"),
			expected: false,
		},
		{
			name:     "nil错误",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := IsNotFoundError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompilationError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *CompilationError
		expected string
	}{
		{
			name: "包含Details",
			err: &CompilationError{
				Message: "编译失败",
				Details: "语法错误详情",
			},
			expected: "编译错误: 编译失败 - 语法错误详情",
		},
		{
			name: "不包含Details",
			err: &CompilationError{
				Message: "编译失败",
				Details: "",
			},
			expected: "编译错误: 编译失败",
		},
		{
			name: "Details为空字符串",
			err: &CompilationError{
				Message: "编译失败",
				Details: "",
			},
			expected: "编译错误: 编译失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyntaxValidationError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *SyntaxValidationError
		expected string
	}{
		{
			name: "包含行号",
			err: &SyntaxValidationError{
				Language: "python",
				Line:     10,
				Message:  "语法错误",
			},
			expected: "语法错误 (python, 第10行): 语法错误",
		},
		{
			name: "不包含行号",
			err: &SyntaxValidationError{
				Language: "javascript",
				Line:     0,
				Message:  "语法错误",
			},
			expected: "语法错误 (javascript): 语法错误",
		},
		{
			name: "负数行号",
			err: &SyntaxValidationError{
				Language: "python",
				Line:     -1,
				Message:  "语法错误",
			},
			expected: "语法错误 (python): 语法错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestErrorConstants(t *testing.T) {
	t.Parallel()

	// 验证错误常量的值
	assert.Equal(t, "UNSUPPORTED_LANGUAGE", ErrCodeUnsupportedLanguage)
	assert.Equal(t, "EXECUTION_TIMEOUT", ErrCodeExecutionTimeout)
	assert.Equal(t, "MEMORY_LIMIT_EXCEEDED", ErrCodeMemoryLimit)
	assert.Equal(t, "SECURITY_VIOLATION", ErrCodeSecurityViolation)
	assert.Equal(t, "RUNTIME_ERROR", ErrCodeRuntimeError)
	assert.Equal(t, "INVALID_CODE", ErrCodeInvalidCode)
	assert.Equal(t, "SYSTEM_ERROR", ErrCodeSystemError)
	assert.Equal(t, "COMPILATION_ERROR", ErrCodeCompilationError)
	assert.Equal(t, "SYNTAX_VALIDATION_ERROR", ErrCodeSyntaxValidation)
}

func TestSandboxErrorChaining(t *testing.T) {
	t.Parallel()

	// 测试错误链
	rootCause := errors.New("root cause")
	middleErr := NewRuntimeError("middle error", rootCause)
	topErr := NewSystemError("top error", middleErr)

	// 验证错误链
	assert.Equal(t, middleErr, topErr.Unwrap())
	assert.Equal(t, rootCause, middleErr.Unwrap())

	// 验证错误消息包含完整链
	assert.Contains(t, topErr.Error(), "top error")
	assert.Contains(t, topErr.Error(), middleErr.Error())
}

func TestErrorEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "空字符串参数",
			test: func(t *testing.T) {
				err := NewUnsupportedLanguageError("")
				assert.Equal(t, ErrCodeUnsupportedLanguage, err.Code)
				assert.Contains(t, err.Message, "不支持的编程语言:")
			},
		},
		{
			name: "零值内存限制",
			test: func(t *testing.T) {
				err := NewMemoryLimitError(0)
				assert.Equal(t, ErrCodeMemoryLimit, err.Code)
				assert.Contains(t, err.Message, "0MB")
			},
		},
		{
			name: "负数内存限制",
			test: func(t *testing.T) {
				err := NewMemoryLimitError(-1)
				assert.Equal(t, ErrCodeMemoryLimit, err.Code)
				assert.Contains(t, err.Message, "-1MB")
			},
		},
		{
			name: "空消息编译错误",
			test: func(t *testing.T) {
				err := NewCompilationError("", "")
				assert.Equal(t, "", err.Message)
				assert.Equal(t, "", err.Details)
				assert.Equal(t, "编译错误: ", err.Error())
			},
		},
		{
			name: "空语言语法错误",
			test: func(t *testing.T) {
				err := NewSyntaxValidationError("", 0, "")
				assert.Equal(t, "", err.Language)
				assert.Equal(t, 0, err.Line)
				assert.Equal(t, "", err.Message)
				assert.Equal(t, "语法错误 (): ", err.Error())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.test(t)
		})
	}
}