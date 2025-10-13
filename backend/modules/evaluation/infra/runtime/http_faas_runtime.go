// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// HTTPFaaSRequest HTTP FaaS请求结构
type HTTPFaaSRequest struct {
	Language string            `json:"language"`
	Code     string            `json:"code"`
	Input    interface{}       `json:"input,omitempty"`
	Timeout  int64             `json:"timeout,omitempty"`
	Priority string            `json:"priority,omitempty"`
	Ext      map[string]string `json:"ext,omitempty"`
}

// HTTPFaaSResponse HTTP FaaS响应结构
type HTTPFaaSResponse struct {
	Output struct {
		Stdout string `json:"stdout"`
		Stderr string `json:"stderr"`
		RetVal string `json:"ret_val"`
	} `json:"output"`
	Metadata *struct {
		TaskID     string `json:"task_id"`
		InstanceID string `json:"instance_id"`
		Duration   int64  `json:"duration"`
		PoolStats  struct {
			TotalInstances  int `json:"totalInstances"`
			IdleInstances   int `json:"idleInstances"`
			ActiveInstances int `json:"activeInstances"`
		} `json:"pool_stats"`
	} `json:"metadata,omitempty"`
	Error   string `json:"error,omitempty"`
	Details string `json:"details,omitempty"`
}

// HTTPFaaSRuntimeConfig HTTP FaaS运行时配置
type HTTPFaaSRuntimeConfig struct {
	BaseURL        string        `json:"base_url"`        // FaaS服务基础URL
	Timeout        time.Duration `json:"timeout"`         // HTTP请求超时
	MaxRetries     int           `json:"max_retries"`     // 最大重试次数
	RetryInterval  time.Duration `json:"retry_interval"`  // 重试间隔
	EnableEnhanced bool          `json:"enable_enhanced"` // 是否启用增强版FaaS
}

// HTTPFaaSRuntimeAdapter 基于HTTP调用的FaaS运行时适配器
type HTTPFaaSRuntimeAdapter struct {
	config       *HTTPFaaSRuntimeConfig
	logger       *logrus.Logger
	httpClient   *http.Client
	languageType entity.LanguageType
}

// NewHTTPFaaSRuntimeAdapter 创建HTTP FaaS运行时适配器
func NewHTTPFaaSRuntimeAdapter(languageType entity.LanguageType, config *HTTPFaaSRuntimeConfig, logger *logrus.Logger) (*HTTPFaaSRuntimeAdapter, error) {
	if config == nil {
		// 根据语言类型选择对应的FaaS服务
		baseURL := "http://coze-loop-js-faas:8000" // 默认值
		switch languageType {
		case entity.LanguageTypePython:
			baseURL = "http://coze-loop-python-faas:8000"
		case entity.LanguageTypeJS:
			baseURL = "http://coze-loop-js-faas:8000"
		}

		config = &HTTPFaaSRuntimeConfig{
			BaseURL:        baseURL,
			Timeout:        30 * time.Second,
			MaxRetries:     3,
			RetryInterval:  1 * time.Second,
			EnableEnhanced: true,
		}
	}

	// 创建HTTP客户端
	httpClient := &http.Client{
		Timeout: config.Timeout,
	}

	return &HTTPFaaSRuntimeAdapter{
		config:       config,
		logger:       logger,
		httpClient:   httpClient,
		languageType: languageType,
	}, nil
}

// GetLanguageType 获取支持的语言类型
func (adapter *HTTPFaaSRuntimeAdapter) GetLanguageType() entity.LanguageType {
	return adapter.languageType
}

// GetReturnValFunction 获取return_val函数实现
func (adapter *HTTPFaaSRuntimeAdapter) GetReturnValFunction() string {
	// HTTPFaaSRuntimeAdapter 作为通用适配器，不提供语言特定的return_val函数
	// 应该由具体的语言运行时（PythonRuntime、JavaScriptRuntime）来实现
	switch adapter.languageType {
	case entity.LanguageTypePython:
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
	case entity.LanguageTypeJS:
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
	default:
		return ""
	}
}

// RunCode 通过HTTP调用FaaS服务执行代码
func (adapter *HTTPFaaSRuntimeAdapter) RunCode(ctx context.Context, code string, language string, timeoutMS int64, ext map[string]string) (*entity.ExecutionResult, error) {
	if code == "" {
		return nil, fmt.Errorf("代码不能为空")
	}

	// 构建请求
	request := HTTPFaaSRequest{
		Language: language,
		Code:     code,
		Timeout:  timeoutMS,
		Priority: "normal",
		Ext:      ext,
	}

	// 执行HTTP请求（带重试）
	var response *HTTPFaaSResponse
	var err error

	for retry := 0; retry <= adapter.config.MaxRetries; retry++ {
		if retry > 0 {
			adapter.logger.WithFields(logrus.Fields{
				"retry":    retry,
				"language": language,
			}).Warn("重试HTTP FaaS请求")

			// 等待重试间隔
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(adapter.config.RetryInterval):
			}
		}

		response, err = adapter.executeHTTPRequest(ctx, &request)
		if err == nil {
			break
		}

		adapter.logger.WithError(err).WithFields(logrus.Fields{
			"retry":    retry,
			"language": language,
		}).Error("HTTP FaaS请求失败")
	}

	if err != nil {
		return nil, fmt.Errorf("HTTP FaaS请求失败（已重试%d次）: %w", adapter.config.MaxRetries, err)
	}

	// 检查响应错误
	if response.Error != "" {
		return &entity.ExecutionResult{
			Output: &entity.ExecutionOutput{
				Stdout: response.Output.Stdout,
				Stderr: response.Output.Stderr + "\n" + response.Error,
				RetVal: "",
			},
		}, fmt.Errorf("FaaS执行错误: %s", response.Error)
	}

	// 转换结果
	result := &entity.ExecutionResult{
		Output: &entity.ExecutionOutput{
			Stdout: response.Output.Stdout,
			Stderr: response.Output.Stderr,
			RetVal: response.Output.RetVal,
		},
	}

	// 记录执行统计信息
	if response.Metadata != nil {
		adapter.logger.WithFields(logrus.Fields{
			"task_id":          response.Metadata.TaskID,
			"instance_id":      response.Metadata.InstanceID,
			"duration_ms":      response.Metadata.Duration,
			"total_instances":  response.Metadata.PoolStats.TotalInstances,
			"idle_instances":   response.Metadata.PoolStats.IdleInstances,
			"active_instances": response.Metadata.PoolStats.ActiveInstances,
		}).Info("FaaS执行完成")
	}

	return result, nil
}

// Cleanup 清理资源
func (adapter *HTTPFaaSRuntimeAdapter) Cleanup() error {
	// HTTP客户端无需特殊清理
	return nil
}

// executeHTTPRequest 执行HTTP请求
func (adapter *HTTPFaaSRuntimeAdapter) executeHTTPRequest(ctx context.Context, request *HTTPFaaSRequest) (*HTTPFaaSResponse, error) {
	// 序列化请求
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 构建HTTP请求
	url := fmt.Sprintf("%s/run_code", adapter.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// 执行请求
	resp, err := adapter.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(responseBody))
	}

	// 解析响应
	var response HTTPFaaSResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 添加详细的调试日志
	codePreview := request.Code
	if len(codePreview) > 100 {
		codePreview = codePreview[:100] + "..."
	}

	adapter.logger.WithFields(logrus.Fields{
		"request_code":     codePreview,
		"response_stdout":  response.Output.Stdout,
		"response_stderr":  response.Output.Stderr,
		"response_ret_val": response.Output.RetVal,
		"response_error":   response.Error,
		"response_details": response.Details,
	}).Debug("FaaS执行详细信息")

	return &response, nil
}

// getTaskID 获取任务ID
func (adapter *HTTPFaaSRuntimeAdapter) getTaskID(response *HTTPFaaSResponse) string {
	if response.Metadata != nil && response.Metadata.TaskID != "" {
		return response.Metadata.TaskID
	}
	return fmt.Sprintf("http_faas_%d", time.Now().UnixNano())
}

// basicSyntaxValidation 基本的语法检查：检查括号匹配
func basicSyntaxValidation(code string) bool {
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

// normalizeLanguage 标准化语言名称
func normalizeLanguage(language string) string {
	switch strings.ToLower(language) {
	case "javascript", "js", "typescript", "ts":
		return "js"
	case "python", "py":
		return "python"
	default:
		return strings.ToLower(language)
	}
}

// 确保HTTPFaaSRuntimeAdapter实现IRuntime接口
var _ component.IRuntime = (*HTTPFaaSRuntimeAdapter)(nil)
