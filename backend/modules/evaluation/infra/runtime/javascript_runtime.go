// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// JavaScriptRuntime JavaScript运行时实现，专门处理JavaScript代码执行
type JavaScriptRuntime struct {
	logger             *logrus.Logger
	config             *entity.SandboxConfig
	httpFaaSAdapter    *HTTPFaaSRuntimeAdapter
}

// NewJavaScriptRuntime 创建JavaScript运行时实例
func NewJavaScriptRuntime(config *entity.SandboxConfig, logger *logrus.Logger) (*JavaScriptRuntime, error) {
	if config == nil {
		config = entity.DefaultSandboxConfig()
	}
	
	if logger == nil {
		logger = logrus.New()
	}

	// 检查JavaScript FaaS服务配置
	jsFaaSURL := os.Getenv("COZE_LOOP_JS_FAAS_URL")
	if jsFaaSURL == "" {
		return nil, fmt.Errorf("必须配置JavaScript FaaS服务URL，请设置COZE_LOOP_JS_FAAS_URL环境变量")
	}

	// 创建HTTP FaaS适配器配置
	faasConfig := &HTTPFaaSRuntimeConfig{
		BaseURL:        jsFaaSURL,
		Timeout:        30 * time.Second,
		MaxRetries:     3,
		RetryInterval:  1 * time.Second,
		EnableEnhanced: true,
	}

	// 创建HTTP FaaS适配器
	httpFaaSAdapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, faasConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("初始化JavaScript FaaS适配器失败: %w", err)
	}

	runtime := &JavaScriptRuntime{
		logger:          logger,
		config:          config,
		httpFaaSAdapter: httpFaaSAdapter,
	}

	logger.WithField("js_faas_url", jsFaaSURL).Info("JavaScript运行时创建成功")
	
	return runtime, nil
}

// GetLanguageType 获取语言类型
func (jr *JavaScriptRuntime) GetLanguageType() entity.LanguageType {
	return entity.LanguageTypeJS
}

// RunCode 执行JavaScript代码
func (jr *JavaScriptRuntime) RunCode(ctx context.Context, code string, language string, timeoutMS int64) (*entity.ExecutionResult, error) {
	if code == "" {
		return nil, fmt.Errorf("代码不能为空")
	}

	jr.logger.WithFields(logrus.Fields{
		"language":   language,
		"timeout_ms": timeoutMS,
	}).Debug("开始执行JavaScript代码")

	// 使用HTTP FaaS适配器执行代码
	return jr.httpFaaSAdapter.RunCode(ctx, code, "js", timeoutMS)
}

// ValidateCode 验证JavaScript代码语法
func (jr *JavaScriptRuntime) ValidateCode(ctx context.Context, code string, language string) bool {
	if code == "" {
		return false
	}

	// 使用基本语法验证
	return basicSyntaxValidation(code)
}

// Cleanup 清理资源
func (jr *JavaScriptRuntime) Cleanup() error {
	jr.logger.Info("JavaScript运行时清理完成")
	if jr.httpFaaSAdapter != nil {
		return jr.httpFaaSAdapter.Cleanup()
	}
	return nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (jr *JavaScriptRuntime) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{entity.LanguageTypeJS}
}

// GetHealthStatus 获取健康状态
func (jr *JavaScriptRuntime) GetHealthStatus() map[string]interface{} {
	status := map[string]interface{}{
		"status":             "healthy",
		"language":           "javascript",
		"supported_languages": jr.GetSupportedLanguages(),
		"js_faas_url":        os.Getenv("COZE_LOOP_JS_FAAS_URL"),
	}

	return status
}

// GetMetrics 获取运行时指标
func (jr *JavaScriptRuntime) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"runtime_type":       "javascript",
		"language":           "javascript",
		"js_faas_configured": os.Getenv("COZE_LOOP_JS_FAAS_URL") != "",
	}
}

// 确保JavaScriptRuntime实现IRuntime接口
var _ component.IRuntime = (*JavaScriptRuntime)(nil)
