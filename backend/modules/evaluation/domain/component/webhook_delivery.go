// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/webhook_delivery.go -package=mocks . IWebhookDeliveryService

// IWebhookDeliveryService Webhook 投递服务，负责将 Webhook 消息投递到目标 URL
type IWebhookDeliveryService interface {
	DeliverWebhook(ctx context.Context, msg *entity.WebhookDeliveryMessage) error
}
