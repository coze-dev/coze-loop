package pyodide

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/domain/entity"
	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/pkg/errors"
	"code.byted.org/flowdevops/cozeloop/backend/modules/evaluation/sandbox/pkg/utils"
)

// DenoPyodideRuntime 基于Deno的Pyodide运行时实现
// 基于 pyodide_runner.py 的简洁设计
type DenoPyodideRuntime struct {
	config       *entity.SandboxConfig
	logger       *logrus.Logger
	tempDir      string
	runnerScript string
}

// SandboxConfig 简化的沙箱配置
type SandboxConfig struct {

	// 基本配置（兼容 pyodide_runner.py）
	AllowEnv       interface{} `json:"allow_env,omitempty"`
	AllowRead      interface{} `json:"allow_read,omitempty"`
	AllowWrite     interface{} `json:"allow_write,omitempty"`
	AllowNet       interface{} `json:"allow_net,omitempty"`
	AllowRun       interface{} `json:"allow_run,omitempty"`
	AllowFFI       interface{} `json:"allow_ffi,omitempty"`
	NodeModulesDir string      `json:"node_modules_dir,omitempty"`
	MemoryLimitMB  int64       `json:"memory_limit_mb,omitempty"`
	TimeoutSeconds float64     `json:"timeout_seconds,omitempty"`
}

// ExecutionRequest 执行请求
type ExecutionRequest struct {
	Config *SandboxConfig         `json:"config,omitempty"`
	Code   string                 `json:"code"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Success       bool                   `json:"success"`
	Result        interface{}            `json:"result,omitempty"`
	Stdout        string                 `json:"stdout,omitempty"`
	Stderr        string                 `json:"stderr,omitempty"`
	ExecutionTime float64                `json:"execution_time"`
	SandboxError  string                 `json:"sandbox_error,omitempty"`
	Status        string                 `json:"status"`
}

// NewDenoPyodideRuntime 创建基于Deno的Pyodide运行时
func NewDenoPyodideRuntime(config *entity.SandboxConfig, logger *logrus.Logger) (*DenoPyodideRuntime, error) {
	if config == nil {
		return nil, errors.NewSystemError("配置不能为空", nil)
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "deno_pyodide_sandbox_*")
	if err != nil {
		return nil, errors.NewSystemError("创建临时目录失败", err)
	}

	// 获取当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		return nil, errors.NewSystemError("获取工作目录失败", err)
	}

	// 查找pyodide_runner.ts路径
	possiblePaths := []string{
		filepath.Join(wd, "infra", "pyodide", "pyodide_runner.ts"),                                                    // 当前目录下的相对路径
		filepath.Join(wd, "backend", "modules", "evaluation", "sandbox", "infra", "pyodide", "pyodide_runner.ts"),   // 完整路径
		filepath.Join(wd, "modules", "evaluation", "sandbox", "infra", "pyodide", "pyodide_runner.ts"),              // 相对路径
		filepath.Join(wd, "sandbox", "infra", "pyodide", "pyodide_runner.ts"),                                        // sandbox目录
		"./infra/pyodide/pyodide_runner.ts",                                                                           // 相对于当前工作目录
	}

	var runnerScript string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			runnerScript = path
			break
		}
	}

	// 如果没有找到，尝试使用绝对路径
	if runnerScript == "" {
		// 从当前文件路径推导
		_, currentFile, _, ok := runtime.Caller(0)
		if ok {
			currentDir := filepath.Dir(currentFile)
			absolutePath := filepath.Join(currentDir, "pyodide_runner.ts")
			if _, err := os.Stat(absolutePath); err == nil {
				runnerScript = absolutePath
			}
		}
	}

	// 检查Deno和pyodide_runner.ts是否存在
	if err := checkDenoAndRunner(runnerScript); err != nil {
		return nil, err
	}

	return &DenoPyodideRuntime{
		config:       config,
		logger:       logger,
		tempDir:      tempDir,
		runnerScript: runnerScript,
	}, nil
}

// checkDenoAndRunner 检查Deno和Runner脚本是否可用
func checkDenoAndRunner(runnerScript string) error {
	// 检查Deno是否安装
	if _, err := exec.LookPath("deno"); err != nil {
		return errors.NewSystemError("Deno未安装，请先安装Deno", err)
	}

	// 检查pyodide_runner.ts是否存在
	if runnerScript == "" {
		return errors.NewSystemError("未找到pyodide_runner.ts文件，请确保文件存在", nil)
	}
	
	if _, err := os.Stat(runnerScript); os.IsNotExist(err) {
		return errors.NewSystemError(fmt.Sprintf("pyodide_runner.ts不存在: %s", runnerScript), err)
	}

	return nil
}

// RunCode 执行Python代码
func (dr *DenoPyodideRuntime) RunCode(ctx context.Context, req *entity.ExecutionRequest) (*entity.ExecutionResult, error) {
	startTime := time.Now()

	// 验证请求
	if req == nil {
		return &entity.ExecutionResult{
			Error:    "执行请求不能为空",
			ExitCode: 1,
			Success:  false,
			Duration: time.Since(startTime),
		}, errors.NewValidationError("执行请求不能为空")
	}

	// 使用管道通信执行
	result, err := dr.executeWithPipeComm(ctx, req, startTime)
	if err != nil {
		return result, err
	}

	return result, nil
}

// executeWithPipeComm 使用管道通信执行
// 基于 runner.go 的管道通信机制和 pyodide_runner.py 的配置系统
func (dr *DenoPyodideRuntime) executeWithPipeComm(ctx context.Context, req *entity.ExecutionRequest, startTime time.Time) (*entity.ExecutionResult, error) {
	// 构建配置
	config := dr.buildConfig(req)
	
	// 创建执行请求
	execReq := &ExecutionRequest{
		Config: config,
		Code:   req.Code,
		Params: dr.convertParams(req.Input),
	}

	// 序列化请求
	requestData, err := json.Marshal(execReq)
	if err != nil {
		return dr.createErrorResult("请求序列化失败", err, startTime), 
			   errors.NewSystemError("请求序列化失败", err)
	}

	// 执行
	result, err := dr.executeAttempt(ctx, requestData, startTime)
	return result, err
}

// executeAttempt 执行单次尝试
func (dr *DenoPyodideRuntime) executeAttempt(
	ctx context.Context, 
	requestData []byte, 
	startTime time.Time,
) (*entity.ExecutionResult, error) {
	// 创建管道
	pr, pw, err := os.Pipe()
	if err != nil {
		return dr.createErrorResult("创建管道失败", err, startTime), 
			   errors.NewSystemError("创建管道失败", err)
	}
	defer pr.Close()

	r, w, err := os.Pipe()
	if err != nil {
		pw.Close()
		return dr.createErrorResult("创建输出管道失败", err, startTime), 
			   errors.NewSystemError("创建输出管道失败", err)
	}
	defer r.Close()

	// 写入请求数据
	go func() {
		defer pw.Close()
		if _, writeErr := pw.Write(requestData); writeErr != nil {
			dr.logger.WithError(writeErr).Error("写入请求数据失败")
		}
	}()

	// 构建Deno命令
	cmd := exec.CommandContext(ctx, "deno", "run", "--allow-all", dr.runnerScript)
	cmd.Stdin = pr
	cmd.Stdout = w
	cmd.Stderr = w

	// 启动进程
	if err = cmd.Start(); err != nil {
		w.Close()
		return dr.createErrorResult("启动进程失败", err, startTime), 
			   errors.NewSystemError("启动进程失败", err)
	}
	w.Close()

	// 设置超时
	timeoutSeconds := dr.getTimeoutSeconds()
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// 在goroutine中等待进程完成
	processErrChan := make(chan error, 1)
	go func() {
		processErrChan <- cmd.Wait()
	}()

	// 读取结果
	result := &ExecutionResult{}
	resultChan := make(chan error, 1)
	go func() {
		decoder := json.NewDecoder(r)
		resultChan <- decoder.Decode(result)
	}()

	// 等待完成或超时
	select {
	case <-timeoutCtx.Done():
		// 超时处理
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return dr.createErrorResult(fmt.Sprintf("执行超时（%v秒）", timeoutSeconds), timeoutCtx.Err(), startTime),
			   errors.NewSystemError("执行超时", timeoutCtx.Err())

	case err := <-resultChan:
		if err != nil {
			// 等待进程完成
			<-processErrChan
			return dr.createErrorResult("解析结果失败", err, startTime),
				   errors.NewSystemError("解析结果失败", err)
		}

		// 等待进程完成
		processErr := <-processErrChan
		if processErr != nil {
			dr.logger.WithError(processErr).Warn("进程执行完成但返回错误")
		}

		duration := time.Since(startTime)

		// 转换结果格式
		return dr.convertExecutionResult(result, duration), nil
	}
}

// createErrorResult 创建错误结果
func (dr *DenoPyodideRuntime) createErrorResult(message string, err error, startTime time.Time) *entity.ExecutionResult {
	errorMsg := message
	if err != nil {
		errorMsg = fmt.Sprintf("%s: %v", message, err)
	}
	
	return &entity.ExecutionResult{
		Error:    errorMsg,
		ExitCode: 1,
		Success:  false,
		Duration: time.Since(startTime),
	}
}

// buildConfig 构建配置
// 基于 pyodide_runner.py 的配置系统
func (dr *DenoPyodideRuntime) buildConfig(req *entity.ExecutionRequest) *SandboxConfig {
	memoryLimitMB := dr.getMemoryLimitMB()
	timeoutSeconds := dr.getTimeoutSeconds()
	
	config := &SandboxConfig{
		AllowEnv:       false,
		AllowRead:      []string{"node_modules"},
		AllowWrite:     []string{"node_modules"},
		AllowNet:       dr.config.NetworkEnabled,
		AllowRun:       false, // 始终禁用运行外部命令
		AllowFFI:       false, // 始终禁用FFI
		NodeModulesDir: "auto",
		MemoryLimitMB:  int64(memoryLimitMB),
		TimeoutSeconds: timeoutSeconds,
	}

	return config
}

// getMemoryLimitMB 获取内存限制
func (dr *DenoPyodideRuntime) getMemoryLimitMB() int {
	baseLimit := int(dr.config.MemoryLimit)
	
	// 确保最小内存限制
	if baseLimit < 32 {
		baseLimit = 32
	}
	
	// 确保最大内存限制
	if baseLimit > 2048 {
		baseLimit = 2048
	}
	
	return baseLimit
}

// getTimeoutSeconds 获取超时时间
func (dr *DenoPyodideRuntime) getTimeoutSeconds() float64 {
	baseTimeout := dr.config.TimeoutLimit.Seconds()
	
	// 确保最小超时时间
	if baseTimeout < 1 {
		baseTimeout = 1
	}
	
	// 确保最大超时时间
	if baseTimeout > 300 {
		baseTimeout = 300
	}
	
	return baseTimeout
}

// convertParams 转换参数格式
func (dr *DenoPyodideRuntime) convertParams(input interface{}) map[string]interface{} {
	if input == nil {
		return make(map[string]interface{})
	}

	// 尝试转换为map
	if params, ok := input.(map[string]interface{}); ok {
		return params
	}

	// 如果不是map，包装成参数
	return map[string]interface{}{
		"input": input,
	}
}

// convertExecutionResult 转换执行结果格式
func (dr *DenoPyodideRuntime) convertExecutionResult(result *ExecutionResult, duration time.Duration) *entity.ExecutionResult {
	entityResult := &entity.ExecutionResult{
		Success:  result.Success,
		Duration: duration,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}

	if result.Success {
		entityResult.ExitCode = 0
		// 解析输出结果
		if result.Result != nil {
			entityResult.Output = dr.parseExecutionOutput(result.Result)
		}
	} else {
		entityResult.ExitCode = 1
		entityResult.Error = result.SandboxError
		if entityResult.Error == "" && result.Stderr != "" {
			entityResult.Error = result.Stderr
		}
	}

	dr.logger.WithFields(logrus.Fields{
		"success":        result.Success,
		"duration_ms":    duration.Milliseconds(),
		"execution_time": result.ExecutionTime,
		"output_size":    len(result.Stdout),
	}).Info("Deno Pyodide执行完成")

	return entityResult
}

// parseExecutionOutput 解析执行输出
func (dr *DenoPyodideRuntime) parseExecutionOutput(result interface{}) *entity.EvalOutput {
	// 尝试直接解析为EvalOutput
	if evalOutput, ok := result.(map[string]interface{}); ok {
		score := 1.0
		reason := "代码执行完成"

		if scoreVal, exists := evalOutput["score"]; exists {
			if scoreFloat, ok := scoreVal.(float64); ok {
				score = scoreFloat
			}
		}

		if reasonVal, exists := evalOutput["reason"]; exists {
			if reasonStr, ok := reasonVal.(string); ok {
				reason = reasonStr
			}
		}

		return &entity.EvalOutput{
			Score:  score,
			Reason: reason,
		}
	}

	// 如果结果是字符串，尝试解析JSON
	if resultStr, ok := result.(string); ok {
		var evalOutput entity.EvalOutput
		if err := json.Unmarshal([]byte(resultStr), &evalOutput); err == nil {
			return &evalOutput
		}

		// 尝试解析为通用map
		var resultMap map[string]interface{}
		if err := json.Unmarshal([]byte(resultStr), &resultMap); err == nil {
			score := 1.0
			reason := "代码执行完成"

			if scoreVal, exists := resultMap["score"]; exists {
				if scoreFloat, ok := scoreVal.(float64); ok {
					score = scoreFloat
				}
			}

			if reasonVal, exists := resultMap["reason"]; exists {
				if reasonStr, ok := reasonVal.(string); ok {
					reason = reasonStr
				}
			}

			return &entity.EvalOutput{
				Score:  score,
				Reason: reason,
			}
		}
	}

	// 默认结果
	return &entity.EvalOutput{
		Score:  1.0,
		Reason: "代码执行完成",
	}
}

// ValidateCode 验证Python代码编译（不执行）
func (dr *DenoPyodideRuntime) ValidateCode(ctx context.Context, code string, language string) bool {
	// 检查是否支持该语言
	if !dr.IsSupported(language) {
		return false
	}

	// 基础安全验证（需要先创建validator）
	validator := utils.NewCodeValidator()
	if err := validator.ValidateCompilation(code, language); err != nil {
		dr.logger.WithError(err).Debug("Python代码安全验证失败")
		return false
	}

	// Python语法验证
	return dr.validatePythonSyntax(ctx, code)
}

// IsSupported 检查是否支持指定语言
func (dr *DenoPyodideRuntime) IsSupported(language string) bool {
	normalizedLang := strings.ToLower(strings.TrimSpace(language))
	return normalizedLang == "python" || normalizedLang == "py"
}

// validatePythonSyntax 验证Python语法
func (dr *DenoPyodideRuntime) validatePythonSyntax(ctx context.Context, code string) bool {
	// 创建Python语法检查脚本
	syntaxChecker := dr.createPythonSyntaxChecker(code)
	
	// 创建临时文件
	tempFile := filepath.Join(dr.tempDir, fmt.Sprintf("syntax_check_%d.ts", time.Now().UnixNano()))
	
	if err := os.WriteFile(tempFile, []byte(syntaxChecker), 0644); err != nil {
		dr.logger.WithError(err).Error("创建Python语法检查文件失败")
		return false
	}
	defer os.Remove(tempFile)

	// 创建验证超时上下文
	validateCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 执行语法检查
	cmd := exec.CommandContext(validateCtx, "deno", "run", "--allow-all", tempFile)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		dr.logger.WithError(err).WithField("output", string(output)).Error("Python语法检查执行失败")
		return false
	}

	// 解析结果
	return dr.parseSyntaxCheckResult(string(output))
}

// createPythonSyntaxChecker 创建Python语法检查脚本
func (dr *DenoPyodideRuntime) createPythonSyntaxChecker(code string) string {
	// 转义代码中的特殊字符
	escapedCode := strings.ReplaceAll(code, "\\", "\\\\")
	escapedCode = strings.ReplaceAll(escapedCode, "`", "\\`")
	escapedCode = strings.ReplaceAll(escapedCode, "$", "\\$")
	escapedCode = strings.ReplaceAll(escapedCode, `"""`, `\"\"\"`)
	
	syntaxChecker := `
import { loadPyodide } from "https://cdn.jsdelivr.net/pyodide/v0.24.1/full/pyodide.mjs";

async function checkPythonSyntax() {
    try {
        const pyodide = await loadPyodide({
            indexURL: "https://cdn.jsdelivr.net/pyodide/v0.24.1/full/",
            stdout: () => {}, // 禁用输出
            stderr: () => {}, // 禁用错误输出
        });

        // Python语法检查代码
        const syntaxCheckCode = ` + "`" + `
import ast
import json
import sys

code = """` + escapedCode + `"""

try:
    # 尝试解析AST
    ast.parse(code)
    result = {"valid": True, "error": None}
except SyntaxError as e:
    result = {"valid": False, "error": f"语法错误: {str(e)}"}
except Exception as e:
    result = {"valid": False, "error": f"解析错误: {str(e)}"}

print(json.dumps(result))
` + "`" + `;

        // 执行语法检查
        const result = pyodide.runPython(syntaxCheckCode);
        console.log(result);
        
    } catch (error) {
        console.log(JSON.stringify({
            "valid": false, 
            "error": "Pyodide初始化失败: " + error.message
        }));
    }
}

checkPythonSyntax();
`

	return syntaxChecker
}

// parseSyntaxCheckResult 解析语法检查结果
func (dr *DenoPyodideRuntime) parseSyntaxCheckResult(output string) bool {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	
	// 查找最后一行的JSON输出
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			var result struct {
				Valid bool   `json:"valid"`
				Error string `json:"error"`
			}
			
			if err := json.Unmarshal([]byte(line), &result); err == nil {
				if !result.Valid && result.Error != "" {
					dr.logger.WithField("syntax_error", result.Error).Debug("Python语法验证失败")
				}
				return result.Valid
			}
		}
	}
	
	// 如果无法解析结果，默认返回false
	dr.logger.WithField("output", output).Warn("无法解析Python语法检查结果")
	return false
}


// Cleanup 清理资源
func (dr *DenoPyodideRuntime) Cleanup() error {
	// 清理临时目录
	if dr.tempDir != "" {
		return os.RemoveAll(dr.tempDir)
	}
	
	return nil
}