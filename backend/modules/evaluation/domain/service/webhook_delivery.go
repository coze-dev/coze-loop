// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// WebhookDeliveryHandler handles webhook delivery events (both first attempt and retries)
type WebhookDeliveryHandler interface {
	HandleWebhookDelivery(ctx context.Context, event *entity.WebhookDeliveryEvent) error
}

// WebhookLifecycleEventHandler handles lifecycle events for webhook notification dispatch
type WebhookLifecycleEventHandler interface {
	HandleLifecycleEventForWebhook(ctx context.Context, event *entity.ExptLifecycleEvent) error
}
