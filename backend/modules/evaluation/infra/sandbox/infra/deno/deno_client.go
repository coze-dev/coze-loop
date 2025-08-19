package deno

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/domain/entity"
	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/pkg/errors"
	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/pkg/utils"
	"github.com/sirupsen/logrus"
)

// DenoRuntime Deno运行时实现
type DenoRuntime struct {
	config    *entity.SandboxConfig
	validator *utils.CodeValidator
	logger    *logrus.Logger
	tempDir   string
}

// NewDenoRuntime 创建Deno运行时
func NewDenoRuntime(config *entity.SandboxConfig, logger *logrus.Logger) (*DenoRuntime, error) {
	if config == nil {
		config = entity.DefaultSandboxConfig()
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "deno_sandbox_*")
	if err != nil {
		return nil, errors.NewSystemError("创建临时目录失败", err)
	}

	return &DenoRuntime{
		config:    config,
		validator: utils.NewCodeValidator(),
		logger:    logger,
		tempDir:   tempDir,
	}, nil
}

// RunCode 执行JavaScript/TypeScript代码
func (dr *DenoRuntime) RunCode(ctx context.Context, req *entity.ExecutionRequest) (*entity.ExecutionResult, error) {
	startTime := time.Now()

	// 验证代码
	if err := dr.validator.Validate(req.Code, req.Language); err != nil {
		return &entity.ExecutionResult{
			Error:    err.Error(),
			ExitCode: 1,
			Success:  false,
			Duration: time.Since(startTime),
		}, err
	}

	// 创建临时文件
	var ext string
	switch strings.ToLower(req.Language) {
	case "typescript", "ts":
		ext = ".ts"
	default:
		ext = ".js"
	}

	tempFile := filepath.Join(dr.tempDir, fmt.Sprintf("code_%d%s", time.Now().UnixNano(), ext))

	// 包装代码以支持评估输入输出
	wrappedCode := dr.wrapCode(req.Code, req.Input)

	if err := os.WriteFile(tempFile, []byte(wrappedCode), 0644); err != nil {
		return &entity.ExecutionResult{
			Error:    "创建临时文件失败",
			ExitCode: 1,
			Success:  false,
			Duration: time.Since(startTime),
		}, errors.NewSystemError("创建临时文件失败", err)
	}

	defer os.Remove(tempFile)

	// 构建Deno命令
	cmd := dr.buildDenoCommand(tempFile)

	// 执行命令
	result, err := dr.executeCommand(ctx, cmd, startTime)
	if err != nil {
		return result, err
	}

	// 解析输出
	if result.Success {
		result.Output = dr.parseOutput(result.Stdout)
	}

	return result, nil
}

// ValidateCode 验证JavaScript/TypeScript代码编译（不执行）
func (dr *DenoRuntime) ValidateCode(ctx context.Context, code string, language string) bool {
	// 检查是否支持该语言
	if !dr.IsSupported(language) {
		return false
	}

	// 基础安全验证
	if err := dr.validator.ValidateCompilation(code, language); err != nil {
		dr.logger.WithError(err).Debug("代码安全验证失败")
		return false
	}

	// 编译验证
	return dr.validateCodeCompilation(ctx, code, language)
}

// IsSupported 检查是否支持指定语言
func (dr *DenoRuntime) IsSupported(language string) bool {
	normalizedLang := strings.ToLower(strings.TrimSpace(language))
	return normalizedLang == "javascript" || normalizedLang == "js" ||
		normalizedLang == "typescript" || normalizedLang == "ts"
}

// validateCodeCompilation 只验证代码编译，不执行
func (dr *DenoRuntime) validateCodeCompilation(ctx context.Context, code string, language string) bool {
	// 确定文件扩展名
	var ext string
	switch strings.ToLower(language) {
	case "typescript", "ts":
		ext = ".ts"
	default:
		ext = ".js"
	}

	// 创建临时文件
	tempFile := filepath.Join(dr.tempDir, fmt.Sprintf("validate_%d%s", time.Now().UnixNano(), ext))
	
	if err := os.WriteFile(tempFile, []byte(code), 0644); err != nil {
		dr.logger.WithError(err).Error("创建验证临时文件失败")
		return false
	}
	defer os.Remove(tempFile)

	// 创建验证超时上下文
	validateCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 构建Deno check命令
	cmd := exec.CommandContext(validateCtx, "deno", "check", "--quiet", tempFile)
	
	// 执行编译检查
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		// 记录编译错误（调试级别）
		dr.logger.WithFields(logrus.Fields{
			"language": language,
			"error":    err.Error(),
			"output":   string(output),
		}).Debug("代码编译验证失败")
		return false
	}

	dr.logger.WithFields(logrus.Fields{
		"language": language,
		"file_ext": ext,
	}).Debug("代码编译验证成功")

	return true
}


// Cleanup 清理资源
func (dr *DenoRuntime) Cleanup() error {
	if dr.tempDir != "" {
		return os.RemoveAll(dr.tempDir)
	}
	return nil
}

// wrapCode 包装代码以支持评估
func (dr *DenoRuntime) wrapCode(code string, input *entity.EvalInput) string {
	var inputJSON string
	if input != nil {
		if data, err := json.Marshal(input); err == nil {
			inputJSON = string(data)
		} else {
			inputJSON = "{}"
		}
	} else {
		inputJSON = "{}"
	}

	wrapper := fmt.Sprintf(`
// 沙箱环境初始化
const evalInput = %s;

// 用户代码
try {
	%s
	
	// 如果没有显式输出，尝试获取最后的表达式结果
	if (typeof result === 'undefined' && typeof score === 'undefined') {
		console.log(JSON.stringify({score: 1.0, reason: "代码执行成功"}));
	} else if (typeof score !== 'undefined') {
		console.log(JSON.stringify({score: score, reason: typeof reason !== 'undefined' ? reason : ""}));
	} else if (typeof result !== 'undefined') {
		console.log(JSON.stringify({score: 1.0, reason: String(result)}));
	}
} catch (error) {
	console.error("执行错误:", error.message);
	console.log(JSON.stringify({score: 0.0, reason: "执行错误: " + error.message}));
}
`, inputJSON, code)

	return wrapper
}

// buildDenoCommand 构建Deno命令
func (dr *DenoRuntime) buildDenoCommand(tempFile string) *exec.Cmd {
	args := []string{
		"run",
		"--no-prompt",
		"--quiet",
	}

	// 安全权限设置
	if !dr.config.NetworkEnabled {
		args = append(args, "--deny-net")
	}

	args = append(args,
		"--deny-read",
		"--deny-write",
		"--deny-env",
		"--deny-run",
		"--deny-ffi",
		"--deny-hrtime",
		tempFile,
	)

	return exec.Command("deno", args...)
}

// executeCommand 执行命令
func (dr *DenoRuntime) executeCommand(ctx context.Context, cmd *exec.Cmd, startTime time.Time) (*entity.ExecutionResult, error) {
	// 设置资源限制
	dr.setResourceLimits(cmd)

	// 执行命令
	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	result := &entity.ExecutionResult{
		Duration: duration,
	}

	if err != nil {
		result.Error = err.Error()
		result.ExitCode = 1
		result.Success = false
		result.Stderr = string(output)

		// 检查是否是超时错误
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = "执行超时"
			return result, errors.NewExecutionTimeoutError(dr.config.TimeoutLimit.String())
		}

		return result, errors.NewRuntimeError("Deno执行失败", err)
	}

	result.Stdout = string(output)
	result.ExitCode = 0
	result.Success = true

	return result, nil
}

// setResourceLimits 设置资源限制
func (dr *DenoRuntime) setResourceLimits(cmd *exec.Cmd) {
	// 设置环境变量限制内存
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}

	// Deno V8 内存限制
	memoryLimitMB := dr.config.MemoryLimit
	cmd.Env = append(cmd.Env, fmt.Sprintf("DENO_V8_FLAGS=--max-old-space-size=%d", memoryLimitMB))
}

// parseOutput 解析输出
func (dr *DenoRuntime) parseOutput(stdout string) *entity.EvalOutput {
	lines := strings.Split(strings.TrimSpace(stdout), "\n")

	// 查找最后一行的JSON输出
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			var output entity.EvalOutput
			if err := json.Unmarshal([]byte(line), &output); err == nil {
				return &output
			}
		}
	}

	// 如果没有找到JSON输出，返回默认结果
	return &entity.EvalOutput{
		Score:  1.0,
		Reason: "代码执行完成",
	}
}