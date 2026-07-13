// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// DispatchRequest is what the lifecycle handler passes to the dispatcher when
// the experiment status transitions to a terminal state. `NotifyConf` is the
// user-provided config; `InternalRules` is appended by BITs / gray injection
// paths and carries `internal_source=bits` so the sender can bypass the SSRF
// private-network guard.
type DispatchRequest struct {
	SpaceID       int64
	Experiment    *entity.Experiment
	Event         string
	NotifyConf    *entity.ExptNotificationConf
	InternalRules []entity.ExptNotificationRule
}

// IWebhookDispatcher fans out an event through the notification config,
// persisting one `webhook_delivery` row per URL and enqueueing the first
// delivery attempt via the event publisher.
//
//go:generate mockgen -destination=mocks/dispatcher.go -package=mocks . IWebhookDispatcher
type IWebhookDispatcher interface {
	Dispatch(ctx context.Context, req *DispatchRequest) error
}

// IWebhookSender performs a single HTTP send with HMAC signing. Consumer /
// retry loops call this once per attempt; result flows back into
// `webhook_delivery` fields.
//
//go:generate mockgen -destination=mocks/sender.go -package=mocks . IWebhookSender
type IWebhookSender interface {
	Send(ctx context.Context, delivery *entity.WebhookDelivery) (statusCode int, err error)
}
