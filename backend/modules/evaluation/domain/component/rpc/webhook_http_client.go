// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"time"
)

// IWebhookHTTPClient Webhook HTTP 投递客户端接口（domain port）
//
//go:generate mockgen -destination=mocks/webhook_http_client.go -package=mocks . IWebhookHTTPClient
type IWebhookHTTPClient interface {
	DoPost(ctx context.Context, url string, payload []byte, headers map[string]string) (statusCode int, err error)
}

// RetryDelayForAttempt 根据重试次数返回延迟时间
// retry_count=0: 1min, retry_count=1: 5min, retry_count=2: 30min
func RetryDelayForAttempt(retryCount int) time.Duration {
	switch retryCount {
	case 0:
		return 1 * time.Minute
	case 1:
		return 5 * time.Minute
	case 2:
		return 30 * time.Minute
	default:
		return 30 * time.Minute
	}
}
