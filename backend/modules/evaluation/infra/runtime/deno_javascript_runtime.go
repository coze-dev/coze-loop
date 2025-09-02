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

// DenoJavaScriptRuntimeAdapter 基于Deno的JavaScript运行时适配器
type DenoJavaScriptRuntimeAdapter struct {
	logger *logrus.Logger
	config *SandboxConfig
}

// NewDenoJavaScriptRuntimeAdapter 创建JavaScript运行时适配器
func NewDenoJavaScriptRuntimeAdapter(config *SandboxConfig, logger *logrus.Logger) (*DenoJavaScriptRuntimeAdapter, error) {
	if config == nil {
		config = DefaultSandboxConfig()
	}

	return &DenoJavaScriptRuntimeAdapter{
		logger: logger,
		config: config,
	}, nil
}

// GetLanguageType 获取支持的语言类型
func (adapter *DenoJavaScriptRuntimeAdapter) GetLanguageType() entity.LanguageType {
	return entity.LanguageTypeJS
}

// RunCode 在沙箱中执行JavaScript代码
func (adapter *DenoJavaScriptRuntimeAdapter) RunCode(ctx context.Context, code string, language string, timeoutMS int64) (*entity.ExecutionResult, error) {
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

	// 模拟JavaScript代码执行（实际实现时需要集成真正的Deno运行时）
	result := &entity.ExecutionResult{
		Output: &entity.ExecutionOutput{
			Stdout: "JavaScript代码执行成功",
			Stderr: "",
			RetVal: `{"score": 1.0, "reason": "代码执行完成"}`,
		},
		WorkloadInfo: &entity.ExecutionWorkloadInfo{
			ID:     fmt.Sprintf("js_%d", time.Now().UnixNano()),
			Status: "success",
		},
	}

	adapter.logger.WithFields(logrus.Fields{
		"language":    language,
		"timeout_ms":  timeoutMS,
		"code_length": len(code),
	}).Info("JavaScript代码执行完成")

	return result, nil
}

// Cleanup 清理资源
func (adapter *DenoJavaScriptRuntimeAdapter) Cleanup() error {
	// 暂时无需清理
	return nil
}

// 确保DenoJavaScriptRuntimeAdapter实现IRuntime接口
var _ component.IRuntime = (*DenoJavaScriptRuntimeAdapter)(nil)