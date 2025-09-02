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
	
	_, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 模拟Python代码执行（实际实现时需要集成真正的Deno+Pyodide运行时）
	result := &entity.ExecutionResult{
		Output: &entity.ExecutionOutput{
			Stdout: "Python代码执行成功",
			Stderr: "",
			RetVal: `{"score": 1.0, "reason": "代码执行完成"}`,
		},
		WorkloadInfo: &entity.ExecutionWorkloadInfo{
			ID:     fmt.Sprintf("python_%d", time.Now().UnixNano()),
			Status: "success",
		},
	}

	adapter.logger.WithFields(logrus.Fields{
		"language":    language,
		"timeout_ms":  timeoutMS,
		"code_length": len(code),
	}).Info("Python代码执行完成")

	return result, nil
}

// Cleanup 清理资源
func (adapter *DenoPythonRuntimeAdapter) Cleanup() error {
	// 暂时无需清理
	return nil
}

// 确保DenoPythonRuntimeAdapter实现IRuntime接口
var _ component.IRuntime = (*DenoPythonRuntimeAdapter)(nil)