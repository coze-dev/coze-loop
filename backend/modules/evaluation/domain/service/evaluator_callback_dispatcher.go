// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/backoff"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate mockgen -destination mocks/evaluator_callback_dispatcher_mock.go -package mocks . IEvaluatorCallbackDispatcher

// IEvaluatorCallbackDispatcher 评估器异步执行完成回调分发器
type IEvaluatorCallbackDispatcher interface {
	// Dispatch posts the signed payload to callbackURL. A callbackURL of "" is a no-op.
	// If payload.Cid is empty, it is generated and written back into payload.
	// Delivery failures are logged and never returned — they must not block the caller's report flow.
	Dispatch(ctx context.Context, spaceID int64, callbackURL string, payload *openapi.EvaluatorCallbackPayloadOApi) error
}

// EvaluatorCallbackDispatcher 同步 POST + backoff 重试投递；失败仅记日志，不进 MQ
type EvaluatorCallbackDispatcher struct {
	httpClient     *http.Client
	secretProvider IWebhookSecretProvider
}

func NewEvaluatorCallbackDispatcher(secretProvider IWebhookSecretProvider) *EvaluatorCallbackDispatcher {
	return &EvaluatorCallbackDispatcher{
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		secretProvider: secretProvider,
	}
}

func (d *EvaluatorCallbackDispatcher) Dispatch(ctx context.Context, spaceID int64, callbackURL string, payload *openapi.EvaluatorCallbackPayloadOApi) error {
	if callbackURL == "" {
		return nil
	}
	if payload.GetCid() == "" {
		payload.Cid = gptr.Of(GenerateNonce())
	}

	body, err := json.Marshal(payload)
	if err != nil {
		logs.CtxError(ctx, "[EvaluatorCallbackDispatcher] marshal payload fail, invoke_id: %v, err: %v", payload.GetInvokeID(), err)
		return nil // 不阻塞主流程
	}

	var secret string
	if d.secretProvider != nil {
		secret, _ = d.secretProvider.GetSecret(ctx, spaceID)
	}

	// 同步 POST + 3s 窗口退避重试；每次重试重算签名
	if rerr := backoff.RetryThreeSeconds(ctx, func() error {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := GenerateNonce()
		signature := ComputeHMACSHA256(secret, timestamp+"\n"+nonce+"\n")
		return d.doPost(ctx, callbackURL, body, timestamp, nonce, signature)
	}); rerr != nil {
		logs.CtxError(ctx, "[EvaluatorCallbackDispatcher] post fail after retry, invoke_id: %v, url: %v, err: %v", payload.GetInvokeID(), callbackURL, rerr)
	} else {
		logs.CtxInfo(ctx, "[EvaluatorCallbackDispatcher] post success, invoke_id: %v, url: %v, cid: %v", payload.GetInvokeID(), callbackURL, payload.GetCid())
	}
	return nil
}

func (d *EvaluatorCallbackDispatcher) doPost(ctx context.Context, turl string, body []byte, timestamp, nonce, signature string) error {
	u, err := url.Parse(turl)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid callback_url")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turl, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CozeLoop-Timestamp", timestamp)
	req.Header.Set("X-CozeLoop-Nonce", nonce)
	req.Header.Set("X-CozeLoop-Signature", signature)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()        //nolint:errcheck
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("evaluator callback returned non-2xx status: %d", resp.StatusCode)
	}
	return nil
}
