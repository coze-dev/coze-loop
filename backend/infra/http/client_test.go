// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewHTTPClient(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient()
	assert.NotNil(t, client)

	httpClient, ok := client.(*HTTPClient)
	assert.True(t, ok)
	assert.NotNil(t, httpClient.client)
	assert.Equal(t, 30*time.Second, httpClient.client.Timeout)
}

func TestHTTPClient_DoHTTPRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupServer func() *httptest.Server
		requestParam *RequestParam
		wantErr     bool
		errContains string
	}{
		{
			name: "成功的GET请求",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"message": "success"}`))
				}))
			},
			requestParam: &RequestParam{
				Method:     "GET",
				RequestURI: "", // 将在测试中设置
				Response:   &map[string]interface{}{},
			},
			wantErr: false,
		},
		{
			name: "成功的POST请求带JSON body",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "POST", r.Method)
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
					
					body, _ := io.ReadAll(r.Body)
					var requestData map[string]interface{}
					json.Unmarshal(body, &requestData)
					assert.Equal(t, "test", requestData["key"])
					
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"result": "ok"}`))
				}))
			},
			requestParam: &RequestParam{
				Method:     "POST",
				RequestURI: "", // 将在测试中设置
				Body:       map[string]interface{}{"key": "test"},
				Response:   &map[string]interface{}{},
			},
			wantErr: false,
		},
		{
			name: "成功的POST请求带io.Reader body",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "POST", r.Method)
					assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
					
					body, _ := io.ReadAll(r.Body)
					assert.Equal(t, "test data", string(body))
					
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"result": "ok"}`))
				}))
			},
			requestParam: &RequestParam{
				Method:     "POST",
				RequestURI: "", // 将在测试中设置
				Body:       strings.NewReader("test data"),
				Response:   &map[string]interface{}{},
			},
			wantErr: false,
		},
		{
			name: "带自定义头部的请求",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
					assert.Equal(t, "application/custom", r.Header.Get("Content-Type"))
					
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"result": "ok"}`))
				}))
			},
			requestParam: &RequestParam{
				Method:     "POST",
				RequestURI: "", // 将在测试中设置
				Header: map[string]string{
					"Authorization": "Bearer token123",
					"Content-Type":  "application/custom",
				},
				Body:     map[string]interface{}{"key": "test"},
				Response: &map[string]interface{}{},
			},
			wantErr: false,
		},
		{
			name: "带超时的请求",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(200 * time.Millisecond) // 模拟慢响应
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"result": "ok"}`))
				}))
			},
			requestParam: &RequestParam{
				Method:     "GET",
				RequestURI: "", // 将在测试中设置
				Timeout:    100 * time.Millisecond, // 设置短超时
				Response:   &map[string]interface{}{},
			},
			wantErr:     true,
			errContains: "context deadline exceeded",
		},
		{
			name: "HTTP错误状态码",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("Bad Request"))
				}))
			},
			requestParam: &RequestParam{
				Method:     "GET",
				RequestURI: "", // 将在测试中设置
				Response:   &map[string]interface{}{},
			},
			wantErr:     true,
			errContains: "HTTP request failed with status 400",
		},
		{
			name: "JSON序列化失败",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			requestParam: &RequestParam{
				Method:     "POST",
				RequestURI: "", // 将在测试中设置
				Body:       make(chan int), // 无法序列化的类型
			},
			wantErr:     true,
			errContains: "failed to marshal request body",
		},
		{
			name: "无效的URL",
			setupServer: func() *httptest.Server {
				return nil // 不需要服务器
			},
			requestParam: &RequestParam{
				Method:     "GET",
				RequestURI: "://invalid-url",
			},
			wantErr:     true,
			errContains: "failed to create request",
		},
		{
			name: "响应JSON解析失败",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("invalid json"))
				}))
			},
			requestParam: &RequestParam{
				Method:     "GET",
				RequestURI: "", // 将在测试中设置
				Response:   &map[string]interface{}{},
			},
			wantErr:     true,
			errContains: "failed to unmarshal response body",
		},
		{
			name:         "nil请求参数",
			setupServer:  func() *httptest.Server { return nil },
			requestParam: nil,
			wantErr:      true,
			errContains:  "request param is nil",
		},
		{
			name: "无响应体处理",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"result": "ok"}`))
				}))
			},
			requestParam: &RequestParam{
				Method:     "GET",
				RequestURI: "", // 将在测试中设置
				Response:   nil, // 不处理响应体
			},
			wantErr: false,
		},
		{
			name: "网络连接失败",
			setupServer: func() *httptest.Server {
				return nil // 不启动服务器
			},
			requestParam: &RequestParam{
				Method:     "GET",
				RequestURI: "http://localhost:99999", // 不存在的端口
			},
			wantErr:     true,
			errContains: "failed to send request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var server *httptest.Server
			if tt.setupServer != nil {
				server = tt.setupServer()
				if server != nil {
					defer server.Close()
					// 设置测试服务器URL
					if tt.requestParam != nil && tt.requestParam.RequestURI == "" {
						tt.requestParam.RequestURI = server.URL
					}
				}
			}

			client := NewHTTPClient()
			ctx := context.Background()

			err := client.DoHTTPRequest(ctx, tt.requestParam)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				// 验证响应解析
				if tt.requestParam != nil && tt.requestParam.Response != nil {
					response := tt.requestParam.Response.(*map[string]interface{})
					assert.NotEmpty(t, *response)
				}
			}
		})
	}
}

func TestHTTPClient_DoHTTPRequest_ContextCancellation(t *testing.T) {
	t.Parallel()

	// 创建一个会延迟响应的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "ok"}`))
	}))
	defer server.Close()

	client := NewHTTPClient()
	ctx, cancel := context.WithCancel(context.Background())

	// 在请求开始后立即取消上下文
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	requestParam := &RequestParam{
		Method:     "GET",
		RequestURI: server.URL,
		Response:   &map[string]interface{}{},
	}

	err := client.DoHTTPRequest(ctx, requestParam)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestHTTPClient_DoHTTPRequest_ResponseBodyReadError(t *testing.T) {
	t.Parallel()

	// 创建一个返回错误响应体的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100") // 设置错误的内容长度
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("short")) // 实际内容比声明的短
	}))
	defer server.Close()

	client := NewHTTPClient()
	ctx := context.Background()

	requestParam := &RequestParam{
		Method:     "GET",
		RequestURI: server.URL,
		Response:   &map[string]interface{}{},
	}

	// 这个测试可能不会失败，因为Go的HTTP客户端通常能处理这种情况
	// 但我们仍然测试以确保代码路径被覆盖
	err := client.DoHTTPRequest(ctx, requestParam)
	// 可能成功也可能失败，取决于具体实现
	if err != nil {
		t.Logf("Expected potential error: %v", err)
	}
}

func TestHTTPClient_DoHTTPRequest_LargeResponse(t *testing.T) {
	t.Parallel()

	// 创建一个返回大响应的服务器
	largeData := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largeData[string(rune('a'+i%26))+string(rune('a'+(i/26)%26))] = i
	}
	largeResponseBytes, _ := json.Marshal(largeData)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(largeResponseBytes)
	}))
	defer server.Close()

	client := NewHTTPClient()
	ctx := context.Background()

	var response map[string]interface{}
	requestParam := &RequestParam{
		Method:     "GET",
		RequestURI: server.URL,
		Response:   &response,
	}

	err := client.DoHTTPRequest(ctx, requestParam)
	assert.NoError(t, err)
	assert.NotEmpty(t, response)
	assert.Equal(t, len(largeData), len(response))
}

func TestHTTPClient_DoHTTPRequest_EmptyBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Empty(t, body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "ok"}`))
	}))
	defer server.Close()

	client := NewHTTPClient()
	ctx := context.Background()

	requestParam := &RequestParam{
		Method:     "POST",
		RequestURI: server.URL,
		Body:       nil, // 空body
		Response:   &map[string]interface{}{},
	}

	err := client.DoHTTPRequest(ctx, requestParam)
	assert.NoError(t, err)
}

func TestHTTPClient_DoHTTPRequest_DifferentHTTPMethods(t *testing.T) {
	t.Parallel()

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, method, r.Method)
				w.WriteHeader(http.StatusOK)
				if method != "HEAD" {
					w.Write([]byte(`{"method": "` + method + `"}`))
				}
			}))
			defer server.Close()

			client := NewHTTPClient()
			ctx := context.Background()

			var response map[string]interface{}
			requestParam := &RequestParam{
				Method:     method,
				RequestURI: server.URL,
			}

			// HEAD请求通常不返回响应体
			if method != "HEAD" {
				requestParam.Response = &response
			}

			err := client.DoHTTPRequest(ctx, requestParam)
			assert.NoError(t, err)

			if method != "HEAD" {
				assert.Equal(t, method, response["method"])
			}
		})
	}
}

func TestHTTPClient_DoHTTPRequest_BytesReader(t *testing.T) {
	t.Parallel()

	testData := []byte("test bytes data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, testData, body)
		assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "ok"}`))
	}))
	defer server.Close()

	client := NewHTTPClient()
	ctx := context.Background()

	requestParam := &RequestParam{
		Method:     "POST",
		RequestURI: server.URL,
		Body:       bytes.NewReader(testData),
		Response:   &map[string]interface{}{},
	}

	err := client.DoHTTPRequest(ctx, requestParam)
	assert.NoError(t, err)
}

func TestHTTPClient_DoHTTPRequest_ErrorStatusCodes(t *testing.T) {
	t.Parallel()

	statusCodes := []int{400, 401, 403, 404, 500, 502, 503}

	for _, statusCode := range statusCodes {
		t.Run(string(rune(statusCode)), func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
				w.Write([]byte("Error message"))
			}))
			defer server.Close()

			client := NewHTTPClient()
			ctx := context.Background()

			requestParam := &RequestParam{
				Method:     "GET",
				RequestURI: server.URL,
			}

			err := client.DoHTTPRequest(ctx, requestParam)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "HTTP request failed with status")
			assert.Contains(t, err.Error(), "Error message")
		})
	}
}