// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// NewWebhookHTTPClient 创建 Webhook HTTP Client
func NewWebhookHTTPClient() rpc.IWebhookHTTPClient {
	return &webhookHTTPClient{
		client: &http.Client{
			Timeout: entity.WebhookTimeout,
		},
	}
}

type webhookHTTPClient struct {
	client *http.Client
}

func (c *webhookHTTPClient) DoPost(ctx context.Context, url string, payload []byte, headers map[string]string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("create http request fail: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("webhook http post fail: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// drain body to reuse connection
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("webhook http post non-2xx: %d", resp.StatusCode)
	}

	return resp.StatusCode, nil
}
