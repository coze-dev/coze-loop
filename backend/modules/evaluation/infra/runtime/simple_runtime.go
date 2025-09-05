// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// SimpleRuntime 简化的运行时实现，直接调用Deno执行代码
type SimpleRuntime struct {
	logger           *logrus.Logger
	config           *entity.SandboxConfig
	supportedLanguages []entity.LanguageType
	tempDir          string
}

// NewSimpleRuntime 创建简化运行时实例
func NewSimpleRuntime(config *entity.SandboxConfig, logger *logrus.Logger) (*SimpleRuntime, error) {
	if config == nil {
		config = entity.DefaultSandboxConfig()
	}
	
	if logger == nil {
		logger = logrus.New()
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "simple_runtime_*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	runtime := &SimpleRuntime{
		logger:             logger,
		config:             config,
		supportedLanguages: []entity.LanguageType{entity.LanguageTypeJS, entity.LanguageTypePython},
		tempDir:            tempDir,
	}

	return runtime, nil
}

// GetLanguageType 获取主要支持的语言类型
func (sr *SimpleRuntime) GetLanguageType() entity.LanguageType {
	return entity.LanguageTypeJS
}

// RunCode 执行代码
func (sr *SimpleRuntime) RunCode(ctx context.Context, code string, language string, timeoutMS int64) (*entity.ExecutionResult, error) {
	if code == "" {
		return nil, fmt.Errorf("代码不能为空")
	}

	// 验证语言类型
	if !sr.isLanguageSupported(language) {
		return nil, fmt.Errorf("不支持的语言类型: %s", language)
	}

	sr.logger.WithFields(logrus.Fields{
		"language":   language,
		"timeout_ms": timeoutMS,
	}).Debug("开始执行代码")

	// 根据语言类型执行代码
	switch normalizeLanguage(language) {
	case "js":
		return sr.executeJavaScript(ctx, code, timeoutMS)
	case "python":
		return sr.executePython(ctx, code, timeoutMS)
	default:
		return nil, fmt.Errorf("不支持的语言类型: %s", language)
	}
}

// ValidateCode 验证代码语法
func (sr *SimpleRuntime) ValidateCode(ctx context.Context, code string, language string) bool {
	if code == "" {
		return false
	}

	// 验证语言类型
	if !sr.isLanguageSupported(language) {
		sr.logger.WithField("language", language).Warn("不支持的语言类型")
		return false
	}

	// 简单的语法检查
	switch normalizeLanguage(language) {
	case "js":
		return sr.basicJSValidation(code)
	case "python":
		return sr.basicPythonValidation(code)
	default:
		return false
	}
}

// Cleanup 清理资源
func (sr *SimpleRuntime) Cleanup() error {
	sr.logger.Info("开始清理简化运行时资源...")
	
	// 清理临时目录
	if sr.tempDir != "" {
		if err := os.RemoveAll(sr.tempDir); err != nil {
			sr.logger.WithError(err).Error("清理临时目录失败")
			return fmt.Errorf("清理临时目录失败: %w", err)
		}
	}

	sr.logger.Info("简化运行时资源清理完成")
	return nil
}

// executeJavaScript 执行JavaScript代码
func (sr *SimpleRuntime) executeJavaScript(ctx context.Context, code string, timeoutMS int64) (*entity.ExecutionResult, error) {
	// 创建临时文件
	tempFile := filepath.Join(sr.tempDir, fmt.Sprintf("script_%d.js", time.Now().UnixNano()))
	if err := os.WriteFile(tempFile, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tempFile)

	// 设置超时
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 执行Deno命令
	cmd := exec.CommandContext(ctx, "deno", "run", 
		"--allow-read", 
		"--allow-write", 
		"--no-prompt", 
		tempFile)
	
	output, err := cmd.CombinedOutput()
	
	result := &entity.ExecutionResult{
		Output: &entity.ExecutionOutput{
			Stdout: string(output),
			Stderr: "",
			RetVal: "",
		},
		WorkloadInfo: &entity.ExecutionWorkloadInfo{
			ID:     fmt.Sprintf("simple_js_%d", time.Now().UnixNano()),
			Status: "success",
		},
	}

	if err != nil {
		result.Output.Stderr = err.Error()
		result.WorkloadInfo.Status = "error"
		return result, fmt.Errorf("JavaScript执行失败: %w", err)
	}

	return result, nil
}

// executePython 执行Python代码（通过Deno + Pyodide）
func (sr *SimpleRuntime) executePython(ctx context.Context, code string, timeoutMS int64) (*entity.ExecutionResult, error) {
	// 创建Python执行脚本
	pythonScript := fmt.Sprintf(`
import { loadPyodide } from "https://cdn.jsdelivr.net/pyodide/v0.24.1/full/pyodide.mjs";

const pyodide = await loadPyodide();

try {
    const result = pyodide.runPython(%q);
    console.log(result);
} catch (error) {
    console.error("Python execution error:", error);
    Deno.exit(1);
}
`, code)

	// 创建临时文件
	tempFile := filepath.Join(sr.tempDir, fmt.Sprintf("python_script_%d.js", time.Now().UnixNano()))
	if err := os.WriteFile(tempFile, []byte(pythonScript), 0644); err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tempFile)

	// 设置超时
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 执行Deno命令
	cmd := exec.CommandContext(ctx, "deno", "run", 
		"--allow-net", 
		"--allow-read", 
		"--allow-write", 
		"--no-prompt", 
		tempFile)
	
	output, err := cmd.CombinedOutput()
	
	result := &entity.ExecutionResult{
		Output: &entity.ExecutionOutput{
			Stdout: string(output),
			Stderr: "",
			RetVal: "",
		},
		WorkloadInfo: &entity.ExecutionWorkloadInfo{
			ID:     fmt.Sprintf("simple_python_%d", time.Now().UnixNano()),
			Status: "success",
		},
	}

	if err != nil {
		result.Output.Stderr = err.Error()
		result.WorkloadInfo.Status = "error"
		return result, fmt.Errorf("Python执行失败: %w", err)
	}

	return result, nil
}

// isLanguageSupported 检查是否支持指定语言
func (sr *SimpleRuntime) isLanguageSupported(language string) bool {
	normalizedLang := normalizeLanguage(language)
	for _, supportedLang := range sr.supportedLanguages {
		if string(supportedLang) == normalizedLang {
			return true
		}
	}
	return false
}



// basicJSValidation 基本的JavaScript语法检查
func (sr *SimpleRuntime) basicJSValidation(code string) bool {
	// 简单的语法检查：检查括号匹配
	brackets := 0
	braces := 0
	parentheses := 0

	for _, char := range code {
		switch char {
		case '[':
			brackets++
		case ']':
			brackets--
		case '{':
			braces++
		case '}':
			braces--
		case '(':
			parentheses++
		case ')':
			parentheses--
		}
	}

	return brackets == 0 && braces == 0 && parentheses == 0
}

// basicPythonValidation 基本的Python语法检查
func (sr *SimpleRuntime) basicPythonValidation(code string) bool {
	// 简单的语法检查：检查括号匹配
	brackets := 0
	braces := 0
	parentheses := 0

	for _, char := range code {
		switch char {
		case '[':
			brackets++
		case ']':
			brackets--
		case '{':
			braces++
		case '}':
			braces--
		case '(':
			parentheses++
		case ')':
			parentheses--
		}
	}

	return brackets == 0 && braces == 0 && parentheses == 0
}

// 确保SimpleRuntime实现IRuntime接口
var _ component.IRuntime = (*SimpleRuntime)(nil)