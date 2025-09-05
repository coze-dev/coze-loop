// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package enhanced

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// DenoProcess Deno进程实例
type DenoProcess struct {
	ID       string
	Cmd      *exec.Cmd
	Stdin    io.WriteCloser
	Stdout   io.ReadCloser
	Stderr   io.ReadCloser
	Language entity.LanguageType
	Status   ProcessStatus
	Created  time.Time
	LastUsed time.Time
	mutex    sync.RWMutex
}

// ProcessStatus 进程状态
type ProcessStatus int

const (
	ProcessStatusStarting ProcessStatus = iota
	ProcessStatusReady
	ProcessStatusBusy
	ProcessStatusError
	ProcessStatusStopped
)

// DenoProcessManager Deno进程管理器
type DenoProcessManager struct {
	processes map[string]*DenoProcess
	mutex     sync.RWMutex
	logger    *logrus.Logger
	config    *entity.SandboxConfig
}

// NewDenoProcessManager 创建Deno进程管理器
func NewDenoProcessManager(config *entity.SandboxConfig, logger *logrus.Logger) *DenoProcessManager {
	return &DenoProcessManager{
		processes: make(map[string]*DenoProcess),
		logger:    logger,
		config:    config,
	}
}

// CreateProcess 创建Deno进程
func (pm *DenoProcessManager) CreateProcess(ctx context.Context, language entity.LanguageType) (*DenoProcess, error) {
	processID := fmt.Sprintf("deno_%s_%d", language, time.Now().UnixNano())
	
	// 确定脚本路径
	scriptPath, err := pm.getScriptPath(language)
	if err != nil {
		return nil, fmt.Errorf("获取脚本路径失败: %w", err)
	}
	
	// 创建Deno命令
	cmd := exec.CommandContext(ctx, "deno", "run", "--allow-all", scriptPath)
	
	// 设置管道
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("创建stdin管道失败: %w", err)
	}
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("创建stdout管道失败: %w", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("创建stderr管道失败: %w", err)
	}
	
	// 创建进程对象
	process := &DenoProcess{
		ID:       processID,
		Cmd:      cmd,
		Stdin:    stdin,
		Stdout:   stdout,
		Stderr:   stderr,
		Language: language,
		Status:   ProcessStatusStarting,
		Created:  time.Now(),
		LastUsed: time.Now(),
	}
	
	// 启动进程
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("启动Deno进程失败: %w", err)
	}
	
	// 注册进程
	pm.mutex.Lock()
	pm.processes[processID] = process
	pm.mutex.Unlock()
	
	// 启动监控协程
	go pm.monitorProcess(process)
	
	// 等待进程就绪
	if err := pm.waitForReady(process, 10*time.Second); err != nil {
		pm.StopProcess(processID)
		return nil, fmt.Errorf("等待进程就绪失败: %w", err)
	}
	
	pm.logger.WithFields(logrus.Fields{
		"process_id": processID,
		"language":   language,
		"script":     scriptPath,
	}).Info("Deno进程创建成功")
	
	return process, nil
}

// getScriptPath 获取脚本路径
func (pm *DenoProcessManager) getScriptPath(language entity.LanguageType) (string, error) {
	// 获取当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取工作目录失败: %w", err)
	}
	
	var scriptName string
	switch language {
	case entity.LanguageTypeJS:
		scriptName = "deno_runner.ts"
	case entity.LanguageTypePython:
		scriptName = "pyodide_runner.ts"
	default:
		return "", fmt.Errorf("不支持的语言类型: %s", language)
	}
	
	// 查找脚本文件的可能路径
	possiblePaths := []string{
		filepath.Join(wd, "backend", "modules", "evaluation", "infra", "runtime", "enhanced", scriptName),
		filepath.Join(wd, "backend", "modules", "evaluation", "infra", "runtime", "pyodide", scriptName),
		filepath.Join(wd, "modules", "evaluation", "infra", "runtime", "enhanced", scriptName),
		filepath.Join(wd, "modules", "evaluation", "infra", "runtime", "pyodide", scriptName),
		filepath.Join(wd, "infra", "runtime", "enhanced", scriptName),
		filepath.Join(wd, "infra", "runtime", "pyodide", scriptName),
	}
	
	// 如果是Python，还要检查pyodide目录
	if language == entity.LanguageTypePython {
		possiblePaths = append(possiblePaths,
			filepath.Join(wd, "backend", "modules", "evaluation", "infra", "runtime", "pyodide", "pyodide_runner.ts"),
			filepath.Join(wd, "modules", "evaluation", "infra", "runtime", "pyodide", "pyodide_runner.ts"),
		)
	}
	
	// 从当前文件路径推导
	_, currentFile, _, ok := runtime.Caller(0)
	if ok {
		currentDir := filepath.Dir(currentFile)
		if language == entity.LanguageTypePython {
			possiblePaths = append(possiblePaths,
				filepath.Join(currentDir, "..", "pyodide", "pyodide_runner.ts"),
				filepath.Join(currentDir, "pyodide_runner.ts"),
			)
		} else {
			possiblePaths = append(possiblePaths,
				filepath.Join(currentDir, "deno_runner.ts"),
			)
		}
	}
	
	// 查找存在的脚本文件
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	
	return "", fmt.Errorf("未找到%s脚本文件，搜索路径: %v", scriptName, possiblePaths)
}

// waitForReady 等待进程就绪
func (pm *DenoProcessManager) waitForReady(process *DenoProcess, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	// 发送ping请求测试进程是否就绪
	testRequest := map[string]interface{}{
		"code":     "console.log('ping');",
		"language": "javascript",
	}
	
	if process.Language == entity.LanguageTypePython {
		testRequest["code"] = "print('ping')"
		testRequest["language"] = "python"
	}
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待进程就绪超时")
		default:
			// 尝试执行测试请求
			_, err := pm.executeInProcess(process, testRequest, 5*time.Second)
			if err == nil {
				process.mutex.Lock()
				process.Status = ProcessStatusReady
				process.mutex.Unlock()
				return nil
			}
			
			// 等待一段时间后重试
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// ExecuteCode 在进程中执行代码
func (pm *DenoProcessManager) ExecuteCode(processID string, code string, language string, timeout time.Duration) (*entity.ExecutionResult, error) {
	pm.mutex.RLock()
	process, exists := pm.processes[processID]
	pm.mutex.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("进程不存在: %s", processID)
	}
	
	// 检查进程状态
	process.mutex.RLock()
	status := process.Status
	process.mutex.RUnlock()
	
	if status != ProcessStatusReady {
		return nil, fmt.Errorf("进程状态不可用: %v", status)
	}
	
	// 标记进程为忙碌状态
	process.mutex.Lock()
	process.Status = ProcessStatusBusy
	process.LastUsed = time.Now()
	process.mutex.Unlock()
	
	defer func() {
		process.mutex.Lock()
		process.Status = ProcessStatusReady
		process.mutex.Unlock()
	}()
	
	// 构建执行请求
	request := map[string]interface{}{
		"code":     code,
		"language": language,
		"config": map[string]interface{}{
			"timeout_seconds":  timeout.Seconds(),
			"memory_limit_mb":  pm.config.MemoryLimit,
			"allow_net":        pm.config.NetworkEnabled,
		},
	}
	
	// 执行代码
	result, err := pm.executeInProcess(process, request, timeout)
	if err != nil {
		return nil, fmt.Errorf("执行代码失败: %w", err)
	}
	
	return result, nil
}

// executeInProcess 在进程中执行请求
func (pm *DenoProcessManager) executeInProcess(process *DenoProcess, request map[string]interface{}, timeout time.Duration) (*entity.ExecutionResult, error) {
	// 序列化请求
	requestData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}
	
	// 创建超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	// 发送请求
	if _, err := process.Stdin.Write(requestData); err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	
	// 读取响应
	responseChan := make(chan []byte, 1)
	errorChan := make(chan error, 1)
	
	go func() {
		buffer := make([]byte, 1024*1024) // 1MB缓冲区
		n, err := process.Stdout.Read(buffer)
		if err != nil {
			errorChan <- err
			return
		}
		responseChan <- buffer[:n]
	}()
	
	// 等待响应或超时
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("执行超时")
	case err := <-errorChan:
		return nil, fmt.Errorf("读取响应失败: %w", err)
	case responseData := <-responseChan:
		// 解析响应
		return pm.parseResponse(responseData)
	}
}

// parseResponse 解析响应
func (pm *DenoProcessManager) parseResponse(data []byte) (*entity.ExecutionResult, error) {
	var response struct {
		Success       bool        `json:"success"`
		Result        interface{} `json:"result"`
		Stdout        string      `json:"stdout"`
		Stderr        string      `json:"stderr"`
		ExecutionTime float64     `json:"execution_time"`
		SandboxError  string      `json:"sandbox_error"`
		Status        string      `json:"status"`
	}
	
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	
	// 构建执行结果
	result := &entity.ExecutionResult{
		Output: &entity.ExecutionOutput{
			Stdout: response.Stdout,
			Stderr: response.Stderr,
		},
		WorkloadInfo: &entity.ExecutionWorkloadInfo{
			ID:     fmt.Sprintf("workload_%d", time.Now().UnixNano()),
			Status: response.Status,
		},
	}
	
	// 处理结果数据
	if response.Success && response.Result != nil {
		// 尝试解析为评估输出
		if resultMap, ok := response.Result.(map[string]interface{}); ok {
			score := 1.0
			reason := "代码执行成功"
			
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
			
			result.Output.RetVal = fmt.Sprintf(`{"score": %f, "reason": "%s"}`, score, reason)
		} else {
			result.Output.RetVal = `{"score": 1.0, "reason": "代码执行成功"}`
		}
	} else {
		// 执行失败
		errorMsg := response.SandboxError
		if errorMsg == "" && response.Stderr != "" {
			errorMsg = response.Stderr
		}
		if errorMsg == "" {
			errorMsg = "执行失败"
		}
		
		result.Output.RetVal = fmt.Sprintf(`{"score": 0.0, "reason": "%s"}`, errorMsg)
		result.WorkloadInfo.Status = "error"
	}
	
	return result, nil
}

// monitorProcess 监控进程
func (pm *DenoProcessManager) monitorProcess(process *DenoProcess) {
	defer func() {
		pm.mutex.Lock()
		delete(pm.processes, process.ID)
		pm.mutex.Unlock()
		
		pm.logger.WithField("process_id", process.ID).Info("进程监控结束")
	}()
	
	// 等待进程结束
	err := process.Cmd.Wait()
	
	process.mutex.Lock()
	process.Status = ProcessStatusStopped
	process.mutex.Unlock()
	
	if err != nil {
		pm.logger.WithError(err).WithField("process_id", process.ID).Error("进程异常结束")
	} else {
		pm.logger.WithField("process_id", process.ID).Info("进程正常结束")
	}
}

// StopProcess 停止进程
func (pm *DenoProcessManager) StopProcess(processID string) error {
	pm.mutex.RLock()
	process, exists := pm.processes[processID]
	pm.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("进程不存在: %s", processID)
	}
	
	// 关闭管道
	if process.Stdin != nil {
		process.Stdin.Close()
	}
	if process.Stdout != nil {
		process.Stdout.Close()
	}
	if process.Stderr != nil {
		process.Stderr.Close()
	}
	
	// 终止进程
	if process.Cmd != nil && process.Cmd.Process != nil {
		if err := process.Cmd.Process.Kill(); err != nil {
			pm.logger.WithError(err).WithField("process_id", processID).Warn("强制终止进程失败")
		}
	}
	
	// 从进程列表中移除
	pm.mutex.Lock()
	delete(pm.processes, processID)
	pm.mutex.Unlock()
	
	pm.logger.WithField("process_id", processID).Info("进程已停止")
	return nil
}

// StopAllProcesses 停止所有进程
func (pm *DenoProcessManager) StopAllProcesses() error {
	pm.mutex.RLock()
	processIDs := make([]string, 0, len(pm.processes))
	for id := range pm.processes {
		processIDs = append(processIDs, id)
	}
	pm.mutex.RUnlock()
	
	var errors []error
	for _, id := range processIDs {
		if err := pm.StopProcess(id); err != nil {
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("停止进程时出现错误: %v", errors)
	}
	
	pm.logger.Info("所有进程已停止")
	return nil
}

// GetProcessCount 获取进程数量
func (pm *DenoProcessManager) GetProcessCount() int {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return len(pm.processes)
}

// GetProcessStatus 获取进程状态
func (pm *DenoProcessManager) GetProcessStatus(processID string) (ProcessStatus, error) {
	pm.mutex.RLock()
	process, exists := pm.processes[processID]
	pm.mutex.RUnlock()
	
	if !exists {
		return ProcessStatusStopped, fmt.Errorf("进程不存在: %s", processID)
	}
	
	process.mutex.RLock()
	defer process.mutex.RUnlock()
	return process.Status, nil
}