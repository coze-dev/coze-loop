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
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// JavaScriptRuntime JavaScript运行时实现，专门处理JavaScript代码执行
type JavaScriptRuntime struct {
	logger          *logrus.Logger
	config          *entity.SandboxConfig
	httpFaaSAdapter *HTTPFaaSRuntimeAdapter
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
	jsFaaSDomain := os.Getenv("COZE_LOOP_JS_FAAS_DOMAIN")
	jsFaaSPort := os.Getenv("COZE_LOOP_JS_FAAS_PORT")
	if jsFaaSDomain == "" || jsFaaSPort == "" {
		return nil, fmt.Errorf("必须配置JavaScript FaaS服务URL，请设置COZE_LOOP_JS_FAAS_DOMAIN和COZE_LOOP_JS_FAAS_PORT环境变量")
	}
	jsFaaSURL := "http://" + jsFaaSDomain + ":" + jsFaaSPort

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
	logs.CtxInfo(context.Background(), "JavaScript FaaS适配器配置: %+v, httpFaaSAdapter: %+v", faasConfig, httpFaaSAdapter)

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
func (jr *JavaScriptRuntime) RunCode(ctx context.Context, code string, language string, timeoutMS int64, ext map[string]string) (*entity.ExecutionResult, error) {
	if code == "" {
		return nil, fmt.Errorf("代码不能为空")
	}

	jr.logger.WithFields(logrus.Fields{
		"language":   language,
		"timeout_ms": timeoutMS,
	}).Debug("开始执行JavaScript代码")

	// 使用HTTP FaaS适配器执行代码
	return jr.httpFaaSAdapter.RunCode(ctx, code, "js", timeoutMS, ext)
}

// ValidateCode 验证JavaScript代码语法
func (jr *JavaScriptRuntime) ValidateCode(ctx context.Context, code string, language string) bool {
	if code == "" {
		return false
	}

	// 使用基本语法验证
	return basicSyntaxValidation(code)
}

// GetSupportedLanguages 获取支持的语言类型列表
func (jr *JavaScriptRuntime) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{entity.LanguageTypeJS}
}

// GetHealthStatus 获取健康状态
func (jr *JavaScriptRuntime) GetHealthStatus() map[string]interface{} {
	status := map[string]interface{}{
		"status":              "healthy",
		"language":            "javascript",
		"supported_languages": jr.GetSupportedLanguages(),
		"js_faas_url":         os.Getenv("COZE_LOOP_JS_FAAS_URL"),
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

// GetReturnValFunction 获取JavaScript return_val函数实现
func (jr *JavaScriptRuntime) GetReturnValFunction() string {
	return `
// return_val函数实现
function return_val(value) {
    /**
     * 修复后的return_val函数实现 - 只输出ret_val内容
     * @param {string} value - 要返回的值，通常是JSON字符串
     */
    
    // 处理输入值
    const ret_val = (value === null || value === undefined) ? "" : String(value);
    
    // 直接输出ret_val内容，供JavaScript FaaS服务器捕获
    console.log(ret_val);
}
`
}

// 确保JavaScriptRuntime实现IRuntime接口
var _ component.IRuntime = (*JavaScriptRuntime)(nil)
