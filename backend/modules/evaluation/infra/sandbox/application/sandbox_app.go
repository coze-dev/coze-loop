package application

import (
	"context"
	"fmt"
	"time"

	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/domain/entity"
	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/domain/service"
	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/infra/deno"
	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/infra/pyodide"
	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/pkg/errors"
	"github.com/sirupsen/logrus"
)

// SandboxApp 沙箱应用服务
type SandboxApp struct {
	sandboxManager service.SandboxManager
	logger         *logrus.Logger
}

// NewSandboxApp 创建沙箱应用服务
func NewSandboxApp(logger *logrus.Logger) (*SandboxApp, error) {
	// 创建默认配置
	config := entity.DefaultSandboxConfig()

	// 创建Deno运行时
	denoRuntime, err := deno.NewDenoRuntime(config, logger)
	if err != nil {
		return nil, errors.NewSystemError("初始化Deno运行时失败", err)
	}

	// 创建基于Deno的Pyodide运行时
	pyodideRuntime, err := pyodide.NewDenoPyodideRuntime(config, logger)
	if err != nil {
		return nil, errors.NewSystemError("初始化Deno Pyodide运行时失败", err)
	}

	// 创建沙箱管理器
	sandboxManager := service.NewSandboxManager(denoRuntime, pyodideRuntime, config, logger)

	return &SandboxApp{
		sandboxManager: sandboxManager,
		logger:         logger,
	}, nil
}

// RunCode 执行代码
func (app *SandboxApp) RunCode(ctx context.Context, req *ExecuteCodeRequest) (*ExecuteCodeResponse, error) {
	// 转换请求
	execReq := &entity.ExecutionRequest{
		Code:        req.Code,
		Language:    req.Language,
		Input:       req.Input,
		Config:      req.Config,
		Environment: req.Environment,
	}

	// 执行代码
	result, err := app.sandboxManager.RunCode(ctx, execReq)
	if err != nil {
		app.logger.WithFields(logrus.Fields{
			"language": req.Language,
			"error":    err.Error(),
		}).Error("代码执行失败")

		return &ExecuteCodeResponse{
			Success: false,
			Error:   err.Error(),
			Result:  result,
		}, err
	}

	return &ExecuteCodeResponse{
		Success: true,
		Result:  result,
	}, nil
}
// ValidateCode 验证代码是否可编译通过且执行
func (app *SandboxApp) ValidateCode(ctx context.Context, code string, language string) bool {
	return app.sandboxManager.ValidateCode(ctx, code, language)
}

// GetSupportedLanguages 获取支持的语言
// GetSupportedLanguages 获取支持的语言
func (app *SandboxApp) GetSupportedLanguages() []string {
	return app.sandboxManager.GetSupportedLanguages()
}

// GetHealthStatus 获取健康状态
func (app *SandboxApp) GetHealthStatus(ctx context.Context) *HealthStatusResponse {
	supportedLanguages := app.GetSupportedLanguages()

	status := &HealthStatusResponse{
		Status:             "healthy",
		SupportedLanguages: supportedLanguages,
		Timestamp:          time.Now(),
	}

	// 测试各个运行时
	for _, lang := range supportedLanguages {
		testReq := &entity.ExecutionRequest{
			Code:     getTestCode(lang),
			Language: lang,
			Config:   &entity.SandboxConfig{TimeoutLimit: 5 * time.Second},
		}

		_, err := app.sandboxManager.RunCode(ctx, testReq)
		if err != nil {
			status.Status = "degraded"
			status.Issues = append(status.Issues, fmt.Sprintf("%s运行时异常: %v", lang, err))
		}
	}

	return status
}

// Shutdown 关闭应用
func (app *SandboxApp) Shutdown() error {
	return app.sandboxManager.Shutdown()
}

// getTestCode 获取测试代码
func getTestCode(language string) string {
	switch language {
	case "python":
		return "score = 1.0\nreason = 'test'"
	case "javascript", "typescript":
		return "const score = 1.0; const reason = 'test';"
	default:
		return "console.log('test');"
	}
}

// ExecuteCodeRequest 执行代码请求
type ExecuteCodeRequest struct {
	Code        string                `json:"code" binding:"required"`
	Language    string                `json:"language" binding:"required"`
	Input       *entity.EvalInput     `json:"input,omitempty"`
	Config      *entity.SandboxConfig `json:"config,omitempty"`
	Environment map[string]string     `json:"environment,omitempty"`
}

// ExecuteCodeResponse 执行代码响应
type ExecuteCodeResponse struct {
	Success bool                    `json:"success"`
	Error   string                  `json:"error,omitempty"`
	Result  *entity.ExecutionResult `json:"result,omitempty"`
}

// HealthStatusResponse 健康状态响应
type HealthStatusResponse struct {
	Status             string    `json:"status"`
	SupportedLanguages []string  `json:"supported_languages"`
	Issues             []string  `json:"issues,omitempty"`
	Timestamp          time.Time `json:"timestamp"`
}

// ValidateCodeRequest 验证代码请求
type ValidateCodeRequest struct {
	Code     string `json:"code" binding:"required"`
	Language string `json:"language" binding:"required"`
}

// ValidateCodeResponse 验证代码响应
type ValidateCodeResponse struct {
	Success bool `json:"success"`
	Valid   bool `json:"valid"`
}