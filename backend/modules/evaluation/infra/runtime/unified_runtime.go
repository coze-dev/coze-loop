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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/runtime/enhanced"
)

// UnifiedRuntime 统一的运行时实现，整合所有运行时功能
type UnifiedRuntime struct {
	logger           *logrus.Logger
	config           *entity.SandboxConfig
	enhancedRuntime  *enhanced.EnhancedRuntime
	httpFaaSRuntime  *HTTPFaaSRuntimeAdapter
	supportedLanguages []entity.LanguageType
	useHTTPFaaS      bool
}

// NewUnifiedRuntime 创建统一运行时实例
func NewUnifiedRuntime(config *entity.SandboxConfig, logger *logrus.Logger) (*UnifiedRuntime, error) {
	if config == nil {
		config = entity.DefaultSandboxConfig()
	}
	
	if logger == nil {
		logger = logrus.New()
	}

	runtime := &UnifiedRuntime{
		logger:             logger,
		config:             config,
		supportedLanguages: []entity.LanguageType{entity.LanguageTypeJS, entity.LanguageTypePython},
	}

	// 检查是否使用HTTP FaaS模式
	faasURL := os.Getenv("COZE_LOOP_FAAS_URL")
	if faasURL != "" {
		runtime.useHTTPFaaS = true
		
		// 初始化HTTP FaaS运行时
		faasConfig := &HTTPFaaSRuntimeConfig{
			BaseURL:        faasURL,
			Timeout:        30 * time.Second,
			MaxRetries:     3,
			RetryInterval:  1 * time.Second,
			EnableEnhanced: true,
		}
		
		httpRuntime, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypeJS, faasConfig, logger)
		if err != nil {
			return nil, fmt.Errorf("初始化HTTP FaaS运行时失败: %w", err)
		}
		runtime.httpFaaSRuntime = httpRuntime
		
		logger.WithField("faas_url", faasURL).Info("使用HTTP FaaS模式")
	} else {
		// 使用本地增强运行时
		enhancedRuntime, err := enhanced.NewEnhancedRuntime(config, logger)
		if err != nil {
			return nil, fmt.Errorf("初始化增强运行时失败: %w", err)
		}
		runtime.enhancedRuntime = enhancedRuntime
		
		logger.Info("使用本地增强运行时模式")
	}

	return runtime, nil
}

// GetLanguageType 获取主要支持的语言类型
func (ur *UnifiedRuntime) GetLanguageType() entity.LanguageType {
	return entity.LanguageTypeJS
}

// RunCode 执行代码
func (ur *UnifiedRuntime) RunCode(ctx context.Context, code string, language string, timeoutMS int64) (*entity.ExecutionResult, error) {
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

	// 根据配置选择执行方式
	if ur.useHTTPFaaS {
		return ur.httpFaaSRuntime.RunCode(ctx, code, language, timeoutMS)
	} else {
		return ur.enhancedRuntime.RunCode(ctx, code, language, timeoutMS)
	}
}

// ValidateCode 验证代码语法
func (ur *UnifiedRuntime) ValidateCode(ctx context.Context, code string, language string) bool {
	if code == "" {
		return false
	}

	// 验证语言类型
	if !ur.isLanguageSupported(language) {
		ur.logger.WithField("language", language).Warn("不支持的语言类型")
		return false
	}

	// 根据配置选择验证方式
	if ur.useHTTPFaaS {
		return ur.httpFaaSRuntime.ValidateCode(ctx, code, language)
	} else {
		return ur.enhancedRuntime.ValidateCode(ctx, code, language)
	}
}

// Cleanup 清理资源
func (ur *UnifiedRuntime) Cleanup() error {
	ur.logger.Info("开始清理统一运行时资源...")
	
	var errors []error

	// 清理HTTP FaaS运行时
	if ur.httpFaaSRuntime != nil {
		if err := ur.httpFaaSRuntime.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("清理HTTP FaaS运行时失败: %w", err))
		}
	}

	// 清理增强运行时
	if ur.enhancedRuntime != nil {
		if err := ur.enhancedRuntime.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("清理增强运行时失败: %w", err))
		}
	}

	if len(errors) > 0 {
		ur.logger.WithField("errors", errors).Error("清理过程中出现错误")
		return fmt.Errorf("清理过程中出现 %d 个错误: %v", len(errors), errors)
	}

	ur.logger.Info("统一运行时资源清理完成")
	return nil
}

// GetSupportedLanguages 获取支持的语言类型列表
func (ur *UnifiedRuntime) GetSupportedLanguages() []entity.LanguageType {
	return ur.supportedLanguages
}

// isLanguageSupported 检查是否支持指定语言
func (ur *UnifiedRuntime) isLanguageSupported(language string) bool {
	normalizedLang := normalizeLanguage(language)
	for _, supportedLang := range ur.supportedLanguages {
		if string(supportedLang) == normalizedLang {
			return true
		}
	}
	return false
}

// normalizeLanguage 标准化语言名称
func normalizeLanguage(language string) string {
	switch language {
	case "javascript", "js", "typescript", "ts":
		return "js"
	case "python", "py":
		return "python"
	default:
		return language
	}
}

// GetHealthStatus 获取健康状态
func (ur *UnifiedRuntime) GetHealthStatus() map[string]interface{} {
	status := map[string]interface{}{
		"status":             "healthy",
		"supported_languages": ur.supportedLanguages,
		"use_http_faas":      ur.useHTTPFaaS,
	}

	if ur.useHTTPFaaS {
		status["mode"] = "http_faas"
		status["faas_url"] = os.Getenv("COZE_LOOP_FAAS_URL")
	} else if ur.enhancedRuntime != nil {
		status["mode"] = "enhanced_local"
		// 如果增强运行时有健康状态方法，可以添加详细信息
		if healthStatus := ur.enhancedRuntime.GetHealthStatus(); healthStatus != nil {
			status["enhanced_details"] = healthStatus
		}
	}

	return status
}

// GetMetrics 获取运行时指标
func (ur *UnifiedRuntime) GetMetrics() map[string]interface{} {
	metrics := map[string]interface{}{
		"mode": "unified",
	}

	if ur.useHTTPFaaS {
		metrics["runtime_type"] = "http_faas"
	} else if ur.enhancedRuntime != nil {
		metrics["runtime_type"] = "enhanced_local"
		
		// 添加增强运行时的指标
		if poolMetrics := ur.enhancedRuntime.GetPoolMetrics(); poolMetrics != nil {
			metrics["pool_metrics"] = poolMetrics
		}
		if schedulerMetrics := ur.enhancedRuntime.GetSchedulerMetrics(); schedulerMetrics != nil {
			metrics["scheduler_metrics"] = schedulerMetrics
		}
	}

	return metrics
}

// 确保UnifiedRuntime实现IRuntime接口
var _ component.IRuntime = (*UnifiedRuntime)(nil)