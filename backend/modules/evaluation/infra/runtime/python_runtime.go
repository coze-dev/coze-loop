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


// PythonRuntime Python代码执行实现
// PythonRuntime Python代码执行实现
type PythonRuntime struct {
	httpClient http.IClient
	baseURL    string
}

// NewPythonRuntime 创建PythonRuntime实例
func NewPythonRuntime(httpClient http.IClient) *PythonRuntime {
	return &PythonRuntime{
		httpClient: httpClient,
		baseURL:    "https://zl8v0obi.fn-boe.bytedance.net/run_code/python",
	}
}

// GetLanguageType 获取支持的语言类型
func (r *PythonRuntime) GetLanguageType() entity.LanguageType {
	return entity.LanguageTypePython
}

// RunCode 在沙箱中执行Python代码 - 返回原始响应结构
func (r *PythonRuntime) RunCode(ctx context.Context, code string, language string, timeoutMS int64) (*entity.ExecutionResult, error) {
	if code == "" {
		return nil, fmt.Errorf("code is empty")
	}

	// 设置默认超时时间
	timeout := time.Duration(timeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	// 构建HTTP请求参数，使用文本格式
	response := entity.ExecutionResult{
		Output:       &entity.ExecutionOutput{},
		WorkloadInfo: &entity.ExecutionWorkloadInfo{},
	}
	requestParam := &http.RequestParam{
		RequestURI: r.baseURL,
		Method:     "POST",
		Header: map[string]string{
			"Content-Type": "text/plain",
		},
		Body:     bytes.NewReader([]byte(code)),
		Response: &response,
		Timeout:  timeout,
	}

	// 发送HTTP请求
	err := r.httpClient.DoHTTPRequest(ctx, requestParam)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}

	return &response, nil
}

// 确保PythonRuntime实现IRuntime接口
var _ component.IRuntime = (*PythonRuntime)(nil)