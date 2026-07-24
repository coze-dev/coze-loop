// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
)

// fixedSecretProvider 返回固定 secret，用于验签
type fixedSecretProvider struct{ secret string }

func (p *fixedSecretProvider) GetSecret(ctx context.Context, spaceID int64) (string, error) {
	return p.secret, nil
}

func TestEvaluatorCallbackDispatcher_EmptyURL_Skips(t *testing.T) {
	var called int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
	}))
	defer srv.Close()

	d := NewEvaluatorCallbackDispatcher(&fixedSecretProvider{secret: "s"})
	err := d.Dispatch(context.Background(), 1, "", &openapi.EvaluatorCallbackPayloadOApi{InvokeID: gptr.Of(int64(1))})
	assert.NoError(t, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&called))
}

func TestEvaluatorCallbackDispatcher_Success_PostsSignedPayload(t *testing.T) {
	var gotBody []byte
	var gotSig, gotTs, gotNonce string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotSig = r.Header.Get("X-CozeLoop-Signature")
		gotTs = r.Header.Get("X-CozeLoop-Timestamp")
		gotNonce = r.Header.Get("X-CozeLoop-Nonce")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	secret := "test-secret"
	d := NewEvaluatorCallbackDispatcher(&fixedSecretProvider{secret: secret})
	payload := &openapi.EvaluatorCallbackPayloadOApi{
		InvokeID:           gptr.Of(int64(100)),
		WorkspaceID:        gptr.Of(int64(200)),
		EvaluatorVersionID: gptr.Of(int64(300)),
		Status:             gptr.Of("success"),
		TimeConsumingMs:    gptr.Of(int64(42)),
	}
	err := d.Dispatch(context.Background(), 200, srv.URL, payload)
	assert.NoError(t, err)

	var decoded openapi.EvaluatorCallbackPayloadOApi
	assert.NoError(t, json.Unmarshal(gotBody, &decoded))
	assert.Equal(t, int64(100), decoded.GetInvokeID())
	assert.Equal(t, "success", decoded.GetStatus())
	assert.NotEmpty(t, decoded.GetCid())
	// 验签：服务端用相同 secret 复算
	assert.Equal(t, ComputeHMACSHA256(secret, gotTs+"\n"+gotNonce+"\n"), gotSig)
}

func TestEvaluatorCallbackDispatcher_Non2xx_RetriesThenReturnsNil(t *testing.T) {
	var called int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := NewEvaluatorCallbackDispatcher(&fixedSecretProvider{secret: "s"})
	err := d.Dispatch(context.Background(), 1, srv.URL, &openapi.EvaluatorCallbackPayloadOApi{InvokeID: gptr.Of(int64(1))})
	// 3s 窗口耗尽仍失败 → 不抛错
	assert.NoError(t, err)
	// 至少调用了一次（退避会重试多次）
	assert.Greater(t, atomic.LoadInt32(&called), int32(1))
}
