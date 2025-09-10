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

// Runtime 统一的运行时实现，仅支持HTTP FaaS模式
type Runtime struct {
	logger             *logrus.Logger
	config             *entity.SandboxConfig
	supportedLanguages []entity.LanguageType
	useHTTPFaaS        bool
}

// NewRuntime 创建统一运行时实例
func NewRuntime(config *entity.SandboxConfig, logger *logrus.Logger) (*Runtime, error) {
	if config == nil {
		config = entity.DefaultSandboxConfig()
	}
	
	if logger == nil {
		logger = logrus.New()
	}

	runtime := &Runtime{
		logger:             logger,
		config:             config,
		supportedLanguages: []entity.LanguageType{entity.LanguageTypeJS, entity.LanguageTypePython},
	}

	// 检查是否使用HTTP FaaS模式
	pythonFaaSURL := os.Getenv("COZE_LOOP_PYTHON_FAAS_URL")
	jsFaaSURL := os.Getenv("COZE_LOOP_JS_FAAS_URL")
	
	// 只支持HTTP FaaS模式，移除本地增强运行时
	if pythonFaaSURL == "" && jsFaaSURL == "" {
		return nil, fmt.Errorf("必须配置FaaS服务URL，请设置COZE_LOOP_PYTHON_FAAS_URL和COZE_LOOP_JS_FAAS_URL环境变量")
	}
	
	runtime.useHTTPFaaS = true
	logger.Info("使用HTTP FaaS模式")
	
	if pythonFaaSURL != "" {
		logger.WithField("python_faas_url", pythonFaaSURL).Info("配置Python FaaS服务")
	}
	if jsFaaSURL != "" {
		logger.WithField("js_faas_url", jsFaaSURL).Info("配置JavaScript FaaS服务")
	}

	return runtime, nil
}

// GetLanguageType 获取主要支持的语言类型
func (ur *Runtime) GetLanguageType() entity.LanguageType {
	return entity.LanguageTypeJS
}

// RunCode 执行代码
func (ur *Runtime) RunCode(ctx context.Context, code string, language string, timeoutMS int64) (*entity.ExecutionResult, error) {
	if code == "" {
		return nil, fmt.Errorf("代码不能为空")
	}

	// 验证语言类型
	if !ur.isLanguageSupported(language) {
		return nil, fmt.Errorf("不支持的语言类型: %s", language)
	}

	ur.logger.WithFields(logrus.Fields{
		"language":     language,
		"timeout_ms":   timeoutMS,
		"use_http_faas": ur.useHTTPFaaS,
	}).Debug("开始执行代码")

	// 使用HTTP FaaS执行代码
	return ur.executeWithHTTPFaaS(ctx, code, language, timeoutMS)
}

// executeWithHTTPFaaS 使用HTTP FaaS执行代码
func (ur *Runtime) executeWithHTTPFaaS(ctx context.Context, code string, language string, timeoutMS int64) (*entity.ExecutionResult, error) {
	// 根据语言类型选择对应的FaaS服务
	var faasURL string
	normalizedLang := normalizeLanguage(language)
	
	switch normalizedLang {
	case "python":
		faasURL = os.Getenv("COZE_LOOP_PYTHON_FAAS_URL")
		if faasURL == "" {
			return nil, fmt.Errorf("Python FaaS服务未配置，请设置COZE_LOOP_PYTHON_FAAS_URL环境变量")
		}
	case "js":
		faasURL = os.Getenv("COZE_LOOP_JS_FAAS_URL")
		if faasURL == "" {
			return nil, fmt.Errorf("JavaScript FaaS服务未配置，请设置COZE_LOOP_JS_FAAS_URL环境变量")
		}
	default:
		return nil, fmt.Errorf("不支持的语言类型: %s", language)
	}

	// 创建对应语言的HTTP FaaS适配器
	var languageType entity.LanguageType
	if normalizedLang == "python" {
		languageType = entity.LanguageTypePython
	} else {
		languageType = entity.LanguageTypeJS
	}

	faasConfig := &HTTPFaaSRuntimeConfig{
		BaseURL:        faasURL,
		Timeout:        30 * time.Second,
		MaxRetries:     3,
		RetryInterval:  1 * time.Second,
		EnableEnhanced: true,
	}

	httpRuntime, err := NewHTTPFaaSRuntimeAdapter(languageType, faasConfig, ur.logger)
	if err != nil {
		return nil, fmt.Errorf("初始化%s FaaS运行时失败: %w", language, err)
	}

	// 执行代码
	return httpRuntime.RunCode(ctx, code, language, timeoutMS)
}

// ValidateCode 验证代码语法
func (ur *Runtime) ValidateCode(ctx context.Context, code string, language string) bool {
	if code == "" {
		return false
	}

	// 验证语言类型
	if !ur.isLanguageSupported(language) {
		ur.logger.WithField("language", language).Warn("不支持的语言类型")
		return false
	}

	// HTTP FaaS模式下使用基本语法验证
	return basicSyntaxValidation(code)
}

// Cleanup 清理资源
func (ur *Runtime) Cleanup() error {
	ur.logger.Info("HTTP FaaS运行时无需特殊清理")
	return nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (ur *Runtime) GetSupportedLanguages() []entity.LanguageType {
	return ur.supportedLanguages
}

// isLanguageSupported 检查是否支持指定语言
func (ur *Runtime) isLanguageSupported(language string) bool {
	normalizedLang := normalizeLanguage(language)
	for _, supportedLang := range ur.supportedLanguages {
		if string(supportedLang) == normalizedLang {
			return true
		}
	}
	return false
}



// GetHealthStatus 获取健康状态
func (ur *Runtime) GetHealthStatus() map[string]interface{} {
	status := map[string]interface{}{
		"status":             "healthy",
		"supported_languages": ur.supportedLanguages,
		"use_http_faas":      ur.useHTTPFaaS,
	}

	status["mode"] = "http_faas"
	status["python_faas_url"] = os.Getenv("COZE_LOOP_PYTHON_FAAS_URL")
	status["js_faas_url"] = os.Getenv("COZE_LOOP_JS_FAAS_URL")

	return status
}

// GetMetrics 获取运行时指标
func (ur *Runtime) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"mode":         "http_faas",
		"runtime_type": "http_faas",
		"python_faas_configured": os.Getenv("COZE_LOOP_PYTHON_FAAS_URL") != "",
		"js_faas_configured":     os.Getenv("COZE_LOOP_JS_FAAS_URL") != "",
	}
}

// 确保Runtime实现IRuntime接口
var _ component.IRuntime = (*Runtime)(nil)