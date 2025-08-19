package service

import (
	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/domain/entity"
	"context"
)

// Runtime 沙箱运行时接口
type Runtime interface {
	RunCode(ctx context.Context, req *entity.ExecutionRequest) (*entity.ExecutionResult, error)
	ValidateCode(ctx context.Context, code string, language string) bool
	IsSupported(language string) bool
	Cleanup() error
}

// SandboxManager 沙箱管理器接口
type SandboxManager interface {
	RunCode(ctx context.Context, req *entity.ExecutionRequest) (*entity.ExecutionResult, error)
	ValidateCode(ctx context.Context, code string, language string) bool
	GetSupportedLanguages() []string
	GetRuntimeForLanguage(language string) (Runtime, error)
	Shutdown() error
}