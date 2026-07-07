// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package sender 提供 webhook 首投 / 重试统一 HTTP 发送与结果分类。
// 契约（tech_design 已锁 + test_cases 12/14/15/16/17）：
//   - method=POST，Content-Type=application/json，client timeout=5s；
//   - 4 件套 header：X-Fornax-Delivery-Id / X-Fornax-Event / X-Fornax-Timestamp / X-Fornax-Signature；
//   - Classify(err|resp) → Outcome：
//     2xx → Succeeded；5xx / timeout / network_error → Retryable；4xx → NonRetryable。
package sender

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	DefaultTimeout = 5 * time.Second

	HeaderDeliveryID = "X-Fornax-Delivery-Id"
	HeaderEvent      = "X-Fornax-Event"
	HeaderTimestamp  = "X-Fornax-Timestamp"
	HeaderSignature  = "X-Fornax-Signature"
	HeaderContent    = "Content-Type"
	ContentJSON      = "application/json"
)

// Outcome 是一次投递的分类结果。tech_design 已锁三档：
//   - Succeeded：2xx；
//   - Retryable：timeout / network error / 5xx，交由 retry topic 走三档延迟；
//   - NonRetryable：4xx（含 429，默认口径不重试；后续如需特殊化可在此扩展）。
type Outcome int

const (
	OutcomeSucceeded Outcome = iota
	OutcomeRetryable
	OutcomeNonRetryable
)

// Request 是一次 webhook 投递需要的最小上下文。
// Body 已经过 canonical JSON 处理（签名前 canonicalize，body 内容与签名一致）。
type Request struct {
	URL        string
	Body       []byte
	DeliveryID string
	Event      string
	Timestamp  string
	Signature  string
}

// Result 汇总一次投递的分类 + 观测字段，供 dispatcher 写 webhook_delivery 行。
// StatusCode=0 表示网络层/超时错误（未拿到 HTTP 响应）。
type Result struct {
	Outcome    Outcome
	StatusCode int
	Err        error
}

// Send 发送单次 HTTP POST 并按 tech_design 契约分类结果。
// client 允许注入（便于测试 / mock），传 nil 时使用内建 5s timeout client。
func Send(ctx context.Context, client *http.Client, req Request) Result {
	if client == nil {
		client = &http.Client{Timeout: DefaultTimeout}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return Result{Outcome: OutcomeNonRetryable, Err: fmt.Errorf("build request: %w", err)}
	}
	httpReq.Header.Set(HeaderContent, ContentJSON)
	httpReq.Header.Set(HeaderDeliveryID, req.DeliveryID)
	httpReq.Header.Set(HeaderEvent, req.Event)
	httpReq.Header.Set(HeaderTimestamp, req.Timestamp)
	httpReq.Header.Set(HeaderSignature, req.Signature)

	resp, err := client.Do(httpReq)
	if err != nil {
		return Result{Outcome: OutcomeRetryable, Err: err}
	}
	defer resp.Body.Close()
	return Result{Outcome: classify(resp.StatusCode), StatusCode: resp.StatusCode}
}

func classify(code int) Outcome {
	switch {
	case code >= 200 && code < 300:
		return OutcomeSucceeded
	case code >= 500:
		return OutcomeRetryable
	default:
		return OutcomeNonRetryable
	}
}
