// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"time"
)

// WebhookDeliveryEvent is the MQ envelope for a pending / retryable
// `webhook_delivery` row. The consumer looks up the row by `DeliveryID`, calls
// `IWebhookSender.Send`, and re-publishes with an exponential delay on
// non-2xx / transport errors.
type WebhookDeliveryEvent struct {
	DeliveryID   string
	SpaceID      int64
	ExperimentID int64
	Event        string
	Attempt      int
}

//go:generate mockgen -destination mocks/webhook_delivery_publisher.go -package mocks . WebhookDeliveryEventPublisher
type WebhookDeliveryEventPublisher interface {
	Publish(ctx context.Context, evt *WebhookDeliveryEvent) error
	PublishDelay(ctx context.Context, evt *WebhookDeliveryEvent, delay time.Duration) error
}
