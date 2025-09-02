// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/sandbox/infra/pyodide"
	sandboxEntity "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/sandbox/domain/entity"
)

// DenoPythonRuntimeAdapter 基于Deno+Pyodide的Python运行时适配器
type DenoPythonRuntimeAdapter struct {
	logger *logrus.Logger
	config *SandboxConfig
}

// NewDenoPythonRuntimeAdapter 创建Python运行时适配器
func NewDenoPythonRuntimeAdapter(config *SandboxConfig, logger *logrus.Logger) (*DenoPythonRuntimeAdapter, error) {
	if config == nil {
		config = DefaultSandboxConfig()
	}

	return &DenoPythonRuntimeAdapter{
		logger: logger,
		config: config,
	}, nil
}

// GetLanguageType 获取支持的语言类型
func (adapter *DenoPythonRuntimeAdapter) GetLanguageType() entity.LanguageType {
	return entity.LanguageTypePython
}

// RunCode 在沙箱中执行Python代码
func (adapter *DenoPythonRuntimeAdapter) RunCode(ctx context.Context, code string, language string, timeoutMS int64) (*entity.ExecutionResult, error) {
	if code == "" {
		return nil, fmt.Errorf("代码不能为空")
	}

	// 设置超时上下文
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = adapter.config.TimeoutLimit
	}
	
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 创建sandbox实例
	pyodideRuntime, err := pyodide.NewDenoPyodideRuntime(adapter.convertConfig(), adapter.logger)
	if err != nil {
		return nil, fmt.Errorf("创建Pyodide运行时失败: %w", err)
	}
	defer pyodideRuntime.Cleanup()

	// 构建执行请求
	req := &sandboxEntity.ExecutionRequest{
		Code:     code,
		Language: language,
		Config:   adapter.convertConfig(),
	}

	// 执行代码
	result, err := pyodideRuntime.RunCode(timeoutCtx, req)
	if err != nil {
		return nil, fmt.Errorf("代码执行失败: %w", err)
	}

	// 转换结果格式
	return adapter.convertResult(result), nil
}

// ValidateCode 验证Python代码语法（不执行）
func (adapter *DenoPythonRuntimeAdapter) ValidateCode(ctx context.Context, code string, language string) bool {
	if code == "" {
		return false
	}

	// 创建sandbox实例
	pyodideRuntime, err := pyodide.NewDenoPyodideRuntime(adapter.convertConfig(), adapter.logger)
	if err != nil {
		adapter.logger.WithError(err).Error("创建Pyodide运行时失败")
		return false
	}
	defer pyodideRuntime.Cleanup()

	// 验证代码
	return pyodideRuntime.ValidateCode(ctx, code, language)
}

// Cleanup 清理资源
func (adapter *DenoPythonRuntimeAdapter) Cleanup() error {
	// 暂时无需清理
	return nil
}

// convertConfig 转换配置格式
func (adapter *DenoPythonRuntimeAdapter) convertConfig() *sandboxEntity.SandboxConfig {
	return &sandboxEntity.SandboxConfig{
		MemoryLimit:    adapter.config.MemoryLimit,
		TimeoutLimit:   adapter.config.TimeoutLimit,
		MaxOutputSize:  adapter.config.MaxOutputSize,
		NetworkEnabled: adapter.config.NetworkEnabled,
	}
}

// convertResult 转换结果格式
func (adapter *DenoPythonRuntimeAdapter) convertResult(sandboxResult *sandboxEntity.ExecutionResult) *entity.ExecutionResult {
	var retVal string
	if sandboxResult.Output != nil {
		retVal = fmt.Sprintf(`{"score": %f, "reason": "%s"}`, sandboxResult.Output.Score, sandboxResult.Output.Reason)
	} else {
		retVal = `{"score": 0.0, "reason": "执行失败"}`
	}

	status := "error"
	if sandboxResult.Success {
		status = "success"
	}

	return &entity.ExecutionResult{
		Output: &entity.ExecutionOutput{
			Stdout: sandboxResult.Stdout,
			Stderr: sandboxResult.Stderr,
			RetVal: retVal,
		},
		WorkloadInfo: &entity.ExecutionWorkloadInfo{
			ID:     fmt.Sprintf("python_%d", time.Now().UnixNano()),
			Status: status,
		},
	}
}

// 确保DenoPythonRuntimeAdapter实现IRuntime接口
var _ component.IRuntime = (*DenoPythonRuntimeAdapter)(nil)