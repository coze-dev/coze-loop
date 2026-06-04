// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type SendResult struct {
	Success    bool
	StatusCode int
	Error      error
}

type IWebhookSender interface {
	Send(ctx context.Context, url string, payload *entity.WebhookPayload, secret string) *SendResult
}
