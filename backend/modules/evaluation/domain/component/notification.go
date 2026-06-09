// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IWebhookDeliverer handles webhook HTTP delivery.
// Open-source provides a noop implementation; commercial provides a real HTTP client.
type IWebhookDeliverer interface {
	// DeliverWebhook sends the payload to the given URL with HMAC-SHA256 signature.
	// Returns nil on success (2xx). On failure, returns an error with HTTP status code context.
	DeliverWebhook(ctx context.Context, deliveryID string, webhookURL string, payload []byte, spaceID int64) error
}

// IFeishuNotifier handles Feishu (Lark) message card notifications.
// Open-source provides a noop implementation; commercial provides a real Feishu SDK client.
type IFeishuNotifier interface {
	// SendExptNotification sends a Feishu notification for an experiment lifecycle event.
	SendExptNotification(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) error
}

// INotificationDeliveryLogRepo persists webhook delivery logs.
type INotificationDeliveryLogRepo interface {
	Create(ctx context.Context, log *entity.WebhookDeliveryLog) error
	UpdateStatus(ctx context.Context, deliveryID string, status entity.WebhookDeliveryStatus, httpCode int, errMsg string) error
	GetByDeliveryID(ctx context.Context, deliveryID string) (*entity.WebhookDeliveryLog, error)
}
