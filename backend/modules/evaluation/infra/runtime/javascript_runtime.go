// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/http"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// JavaScriptRuntime JavaScript代码执行实现
type JavaScriptRuntime struct {
	httpClient http.IClient
	baseURL    string
}

// NewJavaScriptRuntime 创建JavaScriptRuntime实例
func NewJavaScriptRuntime(httpClient http.IClient) *JavaScriptRuntime {
	return &JavaScriptRuntime{
		httpClient: httpClient,
		baseURL:    "https://zl8v0obi.fn-boe.bytedance.net/run_code/js",
	}
}

// GetLanguageType 获取支持的语言类型
func (r *JavaScriptRuntime) GetLanguageType() entity.LanguageType {
	return entity.LanguageTypeJS
}

// RunCode 在沙箱中执行JavaScript代码
func (r *JavaScriptRuntime) RunCode(ctx context.Context, code string, language string, timeoutMS int64) (*entity.ExecutionResult, error) {
	if code == "" {
		return nil, fmt.Errorf("code is empty")
	}

	// 设置默认超时时间
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// 构建HTTP请求参数，使用文本格式
	requestParam := &http.RequestParam{
		RequestURI: r.baseURL,
		Method:     "POST",
		Header: map[string]string{
			"Content-Type": "text/plain",
		},
		Body:     bytes.NewReader([]byte(code)),
		Response: &entity.ExecutionResult{},
		Timeout:  timeout,
	}

	// 发送HTTP请求
	err := r.httpClient.DoHTTPRequest(ctx, requestParam)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}

	// 返回结果
	result := requestParam.Response.(*entity.ExecutionResult)
	return result, nil
}

// 确保JavaScriptRuntime实现IRuntime接口
var _ component.IRuntime = (*JavaScriptRuntime)(nil)