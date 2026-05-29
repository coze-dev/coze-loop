// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// newTestSender 创建不带 SSRF 防护的 sender（用于 httptest 127.0.0.1）
func newTestSender() *Sender {
	return NewSenderWithClient(&http.Client{Timeout: defaultTimeout})
}

func TestSender_Send_Success(t *testing.T) {
	// Arrange: httptest server returning 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get(HeaderTimestamp))
		assert.NotEmpty(t, r.Header.Get(HeaderNonce))
		assert.NotEmpty(t, r.Header.Get(HeaderSignature))
		assert.Equal(t, http.MethodPost, r.Method)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Act
	sender := newTestSender()
	result := sender.Send(context.Background(), server.URL, []byte(`{"test":"data"}`), "secret-key")

	// Assert
	assert.Nil(t, result.Err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.Equal(t, `{"status":"ok"}`, result.ResponseBody)
	assert.True(t, result.IsSuccess())
}

func TestSender_Send_ServerError(t *testing.T) {
	// Arrange: httptest server returning 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	// Act
	sender := newTestSender()
	result := sender.Send(context.Background(), server.URL, []byte(`{"test":"data"}`), "secret-key")

	// Assert
	assert.Nil(t, result.Err)
	assert.Equal(t, http.StatusInternalServerError, result.StatusCode)
	assert.Equal(t, `{"error":"internal server error"}`, result.ResponseBody)
	assert.False(t, result.IsSuccess())
}

func TestSender_Send_Timeout(t *testing.T) {
	// Arrange: httptest server that sleeps longer than the 5s timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(6 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Act: use a sender with a shorter timeout for test efficiency
	sender := &Sender{
		client: &http.Client{
			Timeout: 100 * time.Millisecond,
		},
	}
	result := sender.Send(context.Background(), server.URL, []byte(`{"test":"data"}`), "secret-key")

	// Assert
	assert.NotNil(t, result.Err)
	assert.Contains(t, result.Err.Error(), "send request failed")
	assert.Equal(t, 0, result.StatusCode)
	assert.False(t, result.IsSuccess())
}

func TestSender_Send_InvalidURL(t *testing.T) {
	// Arrange: use a sender with short timeout and an unreachable URL
	sender := &Sender{
		client: &http.Client{
			Timeout: 100 * time.Millisecond,
		},
	}

	// Act
	result := sender.Send(context.Background(), "http://192.0.2.1:9999/webhook", []byte(`{}`), "secret")

	// Assert
	assert.NotNil(t, result.Err)
	assert.Contains(t, result.Err.Error(), "send request failed")
	assert.False(t, result.IsSuccess())
}

func TestSender_Send_ContextCanceled(t *testing.T) {
	// Arrange: server that is slow + canceled context
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Act
	sender := NewSender()
	result := sender.Send(ctx, server.URL, []byte(`{}`), "secret")

	// Assert
	assert.NotNil(t, result.Err)
	assert.False(t, result.IsSuccess())
}

func TestSendResult_IsSuccess(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{name: "200 OK is success", statusCode: 200, want: true},
		{name: "201 Created is success", statusCode: 201, want: true},
		{name: "204 No Content is success", statusCode: 204, want: true},
		{name: "299 is success", statusCode: 299, want: true},
		{name: "199 is not success", statusCode: 199, want: false},
		{name: "300 is not success", statusCode: 300, want: false},
		{name: "400 Bad Request is not success", statusCode: 400, want: false},
		{name: "404 Not Found is not success", statusCode: 404, want: false},
		{name: "500 Internal Server Error is not success", statusCode: 500, want: false},
		{name: "0 (no response) is not success", statusCode: 0, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &SendResult{StatusCode: tt.statusCode}
			assert.Equal(t, tt.want, r.IsSuccess())
		})
	}
}
