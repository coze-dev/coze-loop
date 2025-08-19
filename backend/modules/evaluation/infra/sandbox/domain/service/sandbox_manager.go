package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/domain/entity"
	"github.com/sirupsen/logrus"
)

// SandboxManagerImpl 沙箱管理器实现
type SandboxManagerImpl struct {
	denoRuntime    Runtime
	pyodideRuntime Runtime
	config         *entity.SandboxConfig
	mu             sync.RWMutex
	logger         *logrus.Logger
}

// NewSandboxManager 创建沙箱管理器
func NewSandboxManager(denoRuntime, pyodideRuntime Runtime, config *entity.SandboxConfig, logger *logrus.Logger) *SandboxManagerImpl {
	if config == nil {
		config = entity.DefaultSandboxConfig()
	}

	return &SandboxManagerImpl{
		denoRuntime:    denoRuntime,
		pyodideRuntime: pyodideRuntime,
		config:         config,
		logger:         logger,
	}
}

// RunCode 执行代码
func (sm *SandboxManagerImpl) RunCode(ctx context.Context, req *entity.ExecutionRequest) (*entity.ExecutionResult, error) {
	startTime := time.Now()

	// 应用默认配置
	if req.Config == nil {
		req.Config = sm.config
	}

	// 选择运行时
	runtime, err := sm.GetRuntimeForLanguage(req.Language)
	if err != nil {
		return &entity.ExecutionResult{
			Error:    fmt.Sprintf("不支持的语言: %s", req.Language),
			ExitCode: 1,
			Success:  false,
			Duration: time.Since(startTime),
		}, err
	}

	// 创建超时上下文
	execCtx, cancel := context.WithTimeout(ctx, req.Config.TimeoutLimit)
	defer cancel()

	// 执行代码
	result, err := runtime.RunCode(execCtx, req)
	if err != nil {
		sm.logger.WithFields(logrus.Fields{
			"language": req.Language,
			"error":    err.Error(),
			"duration": time.Since(startTime),
		}).Error("代码执行失败")

		if result == nil {
			result = &entity.ExecutionResult{
				Error:    err.Error(),
				ExitCode: 1,
				Success:  false,
				Duration: time.Since(startTime),
			}
		}
	}

	// 记录执行日志
	sm.logExecution(req, result, startTime)

	return result, err
}
// ValidateCode 验证代码是否可编译通过且执行
func (sm *SandboxManagerImpl) ValidateCode(ctx context.Context, code string, language string) bool {
	// 选择运行时
	runtime, err := sm.GetRuntimeForLanguage(language)
	if err != nil {
		return false
	}

	// 调用运行时的验证方法
	return runtime.ValidateCode(ctx, code, language)
}

// GetSupportedLanguages 获取支持的语言
// GetSupportedLanguages 获取支持的语言
func (sm *SandboxManagerImpl) GetSupportedLanguages() []string {
	languages := make([]string, 0)

	// 手动添加支持的语言
	if sm.denoRuntime != nil {
		languages = append(languages, "javascript", "typescript")
	}

	if sm.pyodideRuntime != nil {
		languages = append(languages, "python")
	}

	return languages
}

// GetRuntimeForLanguage 根据语言获取运行时
func (sm *SandboxManagerImpl) GetRuntimeForLanguage(language string) (Runtime, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	normalizedLang := strings.ToLower(strings.TrimSpace(language))

	switch normalizedLang {
	case "python", "py":
		if sm.pyodideRuntime == nil {
			return nil, fmt.Errorf("Pyodide运行时未初始化")
		}
		return sm.pyodideRuntime, nil
	case "javascript", "js", "typescript", "ts":
		if sm.denoRuntime == nil {
			return nil, fmt.Errorf("Deno运行时未初始化")
		}
		return sm.denoRuntime, nil
	default:
		return nil, fmt.Errorf("不支持的语言: %s", language)
	}
}

// Shutdown 关闭沙箱管理器
func (sm *SandboxManagerImpl) Shutdown() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var errors []string

	if sm.denoRuntime != nil {
		if err := sm.denoRuntime.Cleanup(); err != nil {
			errors = append(errors, fmt.Sprintf("Deno cleanup error: %v", err))
		}
	}

	if sm.pyodideRuntime != nil {
		if err := sm.pyodideRuntime.Cleanup(); err != nil {
			errors = append(errors, fmt.Sprintf("Pyodide cleanup error: %v", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// logExecution 记录执行日志
func (sm *SandboxManagerImpl) logExecution(req *entity.ExecutionRequest, result *entity.ExecutionResult, startTime time.Time) {
	logFields := logrus.Fields{
		"language":     req.Language,
		"duration_ms":  time.Since(startTime).Milliseconds(),
		"success":      result.Success,
		"memory_usage": result.MemoryUsage,
		"exit_code":    result.ExitCode,
	}

	if result.Error != "" {
		logFields["error"] = result.Error
	}

	if result.Success {
		sm.logger.WithFields(logFields).Info("代码执行成功")
	} else {
		sm.logger.WithFields(logFields).Warn("代码执行失败")
	}
}