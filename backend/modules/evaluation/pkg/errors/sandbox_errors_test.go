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