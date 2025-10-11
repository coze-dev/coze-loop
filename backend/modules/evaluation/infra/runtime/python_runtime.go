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

// PythonRuntime Python运行时实现，专门处理Python代码执行
type PythonRuntime struct {
	logger          *logrus.Logger
	config          *entity.SandboxConfig
	httpFaaSAdapter *HTTPFaaSRuntimeAdapter
}

// NewPythonRuntime 创建Python运行时实例
func NewPythonRuntime(config *entity.SandboxConfig, logger *logrus.Logger) (*PythonRuntime, error) {
	if config == nil {
		config = entity.DefaultSandboxConfig()
	}

	if logger == nil {
		logger = logrus.New()
	}

	// 检查Python FaaS服务配置
	pythonFaaSURL := os.Getenv("COZE_LOOP_PYTHON_FAAS_URL")
	if pythonFaaSURL == "" {
		return nil, fmt.Errorf("必须配置Python FaaS服务URL，请设置COZE_LOOP_PYTHON_FAAS_URL环境变量")
	}

	// 创建HTTP FaaS适配器配置
	faasConfig := &HTTPFaaSRuntimeConfig{
		BaseURL:        pythonFaaSURL,
		Timeout:        30 * time.Second,
		MaxRetries:     3,
		RetryInterval:  1 * time.Second,
		EnableEnhanced: true,
	}

	// 创建HTTP FaaS适配器
	httpFaaSAdapter, err := NewHTTPFaaSRuntimeAdapter(entity.LanguageTypePython, faasConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("初始化Python FaaS适配器失败: %w", err)
	}

	runtime := &PythonRuntime{
		logger:          logger,
		config:          config,
		httpFaaSAdapter: httpFaaSAdapter,
	}

	logger.WithField("python_faas_url", pythonFaaSURL).Info("Python运行时创建成功")

	return runtime, nil
}

// GetLanguageType 获取语言类型
func (pr *PythonRuntime) GetLanguageType() entity.LanguageType {
	return entity.LanguageTypePython
}

// RunCode 执行Python代码
func (pr *PythonRuntime) RunCode(ctx context.Context, code string, language string, timeoutMS int64, ext map[string]string) (*entity.ExecutionResult, error) {
	if code == "" {
		return nil, fmt.Errorf("代码不能为空")
	}

	pr.logger.WithFields(logrus.Fields{
		"language":   language,
		"timeout_ms": timeoutMS,
	}).Debug("开始执行Python代码")

	// 使用HTTP FaaS适配器执行代码
	return pr.httpFaaSAdapter.RunCode(ctx, code, "python", timeoutMS, ext)
}

// ValidateCode 验证Python代码语法
func (pr *PythonRuntime) ValidateCode(ctx context.Context, code string, language string) bool {
	if code == "" {
		return false
	}

	// 使用基本语法验证
	return basicSyntaxValidation(code)
}

// GetSupportedLanguages 获取支持的语言类型列表
func (pr *PythonRuntime) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{entity.LanguageTypePython}
}

// GetHealthStatus 获取健康状态
func (pr *PythonRuntime) GetHealthStatus() map[string]interface{} {
	status := map[string]interface{}{
		"status":              "healthy",
		"language":            "python",
		"supported_languages": pr.GetSupportedLanguages(),
		"python_faas_url":     os.Getenv("COZE_LOOP_PYTHON_FAAS_URL"),
	}

	return status
}

// GetMetrics 获取运行时指标
func (pr *PythonRuntime) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"runtime_type":           "python",
		"language":               "python",
		"python_faas_configured": os.Getenv("COZE_LOOP_PYTHON_FAAS_URL") != "",
	}
}

// GetReturnValFunction 获取Python return_val函数实现
func (pr *PythonRuntime) GetReturnValFunction() string {
	return `
# return_val函数实现
def return_val(value):
    """
    修复后的return_val函数实现 - 只输出ret_val内容
    Args:
        value: 要返回的值，通常是JSON字符串
    """
    # 处理输入值
    if value is None:
        ret_val = ""
    else:
        ret_val = str(value)
    
    # 使用特殊标记输出ret_val内容，供FaaS服务器提取
    print(f"__COZE_RETURN_VAL_START__")
    print(ret_val)
    print(f"__COZE_RETURN_VAL_END__")
`
}

// 确保PythonRuntime实现IRuntime接口
var _ component.IRuntime = (*PythonRuntime)(nil)