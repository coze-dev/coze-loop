// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
)

type NoopWebhookDeliveryAdapter struct{}

func NewNoopWebhookDeliveryAdapter() rpc.IWebhookDeliveryAdapter {
	return &NoopWebhookDeliveryAdapter{}
}

func (n *NoopWebhookDeliveryAdapter) Deliver(ctx context.Context, url string, spaceID int64, payload *rpc.WebhookPayload) error {
	return nil
}
