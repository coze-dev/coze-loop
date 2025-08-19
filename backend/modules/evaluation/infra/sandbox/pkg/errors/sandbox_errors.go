package errors

import "fmt"

// SandboxError 沙箱错误类型
type SandboxError struct {
	Code    string
	Message string
	Cause   error
}

func (e *SandboxError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *SandboxError) Unwrap() error {
	return e.Cause
}

// 错误代码常量
const (
	ErrCodeUnsupportedLanguage = "UNSUPPORTED_LANGUAGE"
	ErrCodeExecutionTimeout    = "EXECUTION_TIMEOUT"
	ErrCodeMemoryLimit         = "MEMORY_LIMIT_EXCEEDED"
	ErrCodeSecurityViolation   = "SECURITY_VIOLATION"
	ErrCodeRuntimeError        = "RUNTIME_ERROR"
	ErrCodeInvalidCode         = "INVALID_CODE"
	ErrCodeSystemError         = "SYSTEM_ERROR"
	ErrCodeCompilationError    = "COMPILATION_ERROR"
	ErrCodeSyntaxValidation    = "SYNTAX_VALIDATION_ERROR"
)

// NewSandboxError 创建沙箱错误
func NewSandboxError(code, message string, cause error) *SandboxError {
	return &SandboxError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// NewUnsupportedLanguageError 不支持的语言错误
func NewUnsupportedLanguageError(language string) *SandboxError {
	return NewSandboxError(
		ErrCodeUnsupportedLanguage,
		fmt.Sprintf("不支持的编程语言: %s", language),
		nil,
	)
}

// NewExecutionTimeoutError 执行超时错误
func NewExecutionTimeoutError(timeout string) *SandboxError {
	return NewSandboxError(
		ErrCodeExecutionTimeout,
		fmt.Sprintf("代码执行超时: %s", timeout),
		nil,
	)
}

// NewMemoryLimitError 内存限制错误
func NewMemoryLimitError(limit int64) *SandboxError {
	return NewSandboxError(
		ErrCodeMemoryLimit,
		fmt.Sprintf("内存使用超过限制: %dMB", limit),
		nil,
	)
}

// NewSecurityViolationError 安全违规错误
func NewSecurityViolationError(violation string) *SandboxError {
	return NewSandboxError(
		ErrCodeSecurityViolation,
		fmt.Sprintf("安全违规: %s", violation),
		nil,
	)
}

// NewRuntimeError 运行时错误
func NewRuntimeError(message string, cause error) *SandboxError {
	return NewSandboxError(
		ErrCodeRuntimeError,
		message,
		cause,
	)
}

// NewInvalidCodeError 无效代码错误
func NewInvalidCodeError(message string) *SandboxError {
	return NewSandboxError(
		ErrCodeInvalidCode,
		message,
		nil,
	)
}

// NewSystemError 系统错误
func NewSystemError(message string, cause error) *SandboxError {
	return NewSandboxError(
		ErrCodeSystemError,
		message,
		cause,
	)
}

// NewValidationError 验证错误
func NewValidationError(message string) *SandboxError {
	return NewSandboxError(
		ErrCodeInvalidCode,
		message,
		nil,
	)
}

// NewNotFoundError 资源未找到错误
func NewNotFoundError(message string) *SandboxError {
	return NewSandboxError(
		"NOT_FOUND",
		message,
		nil,
	)
}

// IsNotFoundError 检查是否为未找到错误
func IsNotFoundError(err error) bool {
	if sandboxErr, ok := err.(*SandboxError); ok {
		return sandboxErr.Code == "NOT_FOUND"
	}
	return false
}

// CompilationError 编译错误
type CompilationError struct {
	Message string
	Details string
}

func (e *CompilationError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("编译错误: %s - %s", e.Message, e.Details)
	}
	return fmt.Sprintf("编译错误: %s", e.Message)
}

// NewCompilationError 创建编译错误
func NewCompilationError(message, details string) *CompilationError {
	return &CompilationError{
		Message: message,
		Details: details,
	}
}

// SyntaxValidationError 语法验证错误
type SyntaxValidationError struct {
	Language string
	Line     int
	Message  string
}

func (e *SyntaxValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("语法错误 (%s, 第%d行): %s", e.Language, e.Line, e.Message)
	}
	return fmt.Sprintf("语法错误 (%s): %s", e.Language, e.Message)
}

// NewSyntaxValidationError 创建语法验证错误
func NewSyntaxValidationError(language string, line int, message string) *SyntaxValidationError {
	return &SyntaxValidationError{
		Language: language,
		Line:     line,
		Message:  message,
	}
}