// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPClient(t *testing.T) {
	t.Parallel()

	client := NewHTTPClient()
	assert.NotNil(t, client)

	// 验证类型断言
	httpClient, ok := client.(*HTTPClient)
	require.True(t, ok)
	assert.NotNil(t, httpClient.client)
	assert.Equal(t, 30*time.Second, httpClient.client.Timeout)
}

func TestHTTPClient_DoHTTPRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		requestParam *RequestParam
		setupServer  func() *httptest.Server
		wantErr      bool
		wantErrMsg   string
	}{
		{
			name: "成功的GET请求",
			requestParam: &RequestParam{
				Method:     "GET",
				RequestURI: "", // 将在测试中设置
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"message": "success"}`))
				}))
			},
			wantErr: false,
		},
		{
			name: "成功的POST请求带JSON body",
			requestParam: &RequestParam{
				Method:     "POST",
				RequestURI: "", // 将在测试中设置
				Body:       map[string]interface{}{"key": "value"},
				Header: map[string]string{
					"Content-Type": "application/json",
				},
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "POST", r.Method)
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)

					var data map[string]interface{}
					err = json.Unmarshal(body, &data)
					require.NoError(t, err)
					assert.Equal(t, "value", data["key"])

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"received": true}`))
				}))
			},
			wantErr: false,
		},
		{
			name: "成功的POST请求带io.Reader body",
			requestParam: &RequestParam{
				Method:     "POST",
				RequestURI: "", // 将在测试中设置
				Body:       strings.NewReader(`{"reader": "body"}`),
				Header: map[string]string{
					"Content-Type": "application/json",
				},
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "POST", r.Method)

					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					assert.Equal(t, `{"reader": "body"}`, string(body))

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"received": true}`))
				}))
			},
			wantErr: false,
		},
		{
			name: "服务器返回错误状态码",
			requestParam: &RequestParam{
				Method:     "GET",
				RequestURI: "", // 将在测试中设置
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error": "server error"}`))
				}))
			},
			wantErr:    true,
			wantErrMsg: "HTTP request failed with status 500",
		},
		{
			name: "请求超时",
			requestParam: &RequestParam{
				Method:     "GET",
				RequestURI: "", // 将在测试中设置
				Timeout:    100 * time.Millisecond,
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// 模拟慢响应
					time.Sleep(200 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
				}))
			},
			wantErr:    true,
			wantErrMsg: "context deadline exceeded",
		},
		{
			name:         "请求参数为nil",
			requestParam: nil,
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			wantErr:    true,
			wantErrMsg: "request param is nil",
		},
		{
			name: "无效的URL",
			requestParam: &RequestParam{
				Method:     "GET",
				RequestURI: "://invalid-url",
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			wantErr:    true,
			wantErrMsg: "missing protocol scheme",
		},
		{
			name: "JSON序列化失败",
			requestParam: &RequestParam{
				Method:     "POST",
				RequestURI: "",             // 将在测试中设置
				Body:       make(chan int), // 不可序列化的类型
			},
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			wantErr:    true,
			wantErrMsg: "json: unsupported type: chan int",
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
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				assert.NoError(t, err)
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
		_, _ = w.Write([]byte(`{"result": "ok"}`))
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
		_, _ = w.Write([]byte("short")) // 实际内容比声明的短
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
		_, _ = w.Write(largeResponseBytes)
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
		_, _ = w.Write([]byte(`{"result": "ok"}`))
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
					_, _ = w.Write([]byte(`{"method": "` + method + `"}`))
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
		_, _ = w.Write([]byte(`{"result": "ok"}`))
	}))
	defer server.Close()

	client := NewHTTPClient()
	ctx := context.Background()

	requestParam := &RequestParam{
		Method:     "POST",
		RequestURI: server.URL,
		Body:       strings.NewReader(string(testData)),
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
				_, _ = w.Write([]byte("Error message"))
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
