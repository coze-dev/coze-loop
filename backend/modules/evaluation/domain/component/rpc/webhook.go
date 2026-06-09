// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/webhook.go -package=mocks . IWebhookDeliveryService,IWebhookSigner

// IWebhookDeliveryService publishes webhook delivery events to the MQ topic.
type IWebhookDeliveryService interface {
	PublishWebhookDelivery(ctx context.Context, events []*entity.WebhookDeliveryEvent) error
}

// IWebhookSigner signs webhook payloads using workspace secret keys.
type IWebhookSigner interface {
	Sign(ctx context.Context, workspaceID int64, timestamp int64, body []byte) (string, error)
}
