// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient HTTP客户端实现
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient 创建HTTP客户端实例
func NewHTTPClient() IClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DoHTTPRequest 执行HTTP请求
func (c *HTTPClient) DoHTTPRequest(ctx context.Context, requestParam *RequestParam) error {
	if requestParam == nil {
		return fmt.Errorf("request param is nil")
	}

	// 设置超时
	if requestParam.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, requestParam.Timeout)
		defer cancel()
	}

	// 序列化请求体
	var body io.Reader
	if requestParam.Body != nil {
		// 如果Body已经是io.Reader类型，直接使用
		if reader, ok := requestParam.Body.(io.Reader); ok {
			body = reader
		} else {
			// 否则进行JSON序列化
			bodyBytes, err := json.Marshal(requestParam.Body)
			if err != nil {
				return fmt.Errorf("failed to marshal request body: %w", err)
			}
			body = bytes.NewReader(bodyBytes)
		}
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, requestParam.Method, requestParam.RequestURI, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	if requestParam.Header != nil {
		for key, value := range requestParam.Header {
			req.Header.Set(key, value)
		}
	}

	// 如果没有设置Content-Type且有body，设置为application/json
	if requestParam.Body != nil && req.Header.Get("Content-Type") == "" {
		// 如果Body是io.Reader类型，默认设置为text/plain
		if _, ok := requestParam.Body.(io.Reader); ok {
			req.Header.Set("Content-Type", "text/plain")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查HTTP状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应体
	if requestParam.Response != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if err := json.Unmarshal(bodyBytes, requestParam.Response); err != nil {
			return fmt.Errorf("failed to unmarshal response body: %w", err)
		}
	}

	return nil
}
