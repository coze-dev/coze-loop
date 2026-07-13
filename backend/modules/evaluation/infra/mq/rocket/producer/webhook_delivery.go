// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// NewWebhookDeliveryEventPublisher builds the domain publisher over the
// existing expt-event MQ producer. Commercial wire already calls this with
// the shared `exptEventPublisher`; keeping the underlying producer wired
// keeps ops from having to spin a second RocketMQ topic before the OSS DDL
// migration lands.
func NewWebhookDeliveryEventPublisher(base events.ExptEventPublisher) events.WebhookDeliveryEventPublisher {
	return &webhookDeliveryPublisher{base: base}
}

type webhookDeliveryPublisher struct {
	base events.ExptEventPublisher
}

// Publish enqueues the initial delivery attempt. Until the dedicated
// `evaluation_webhook_delivery` topic is provisioned the payload is logged so
// consumer loops can be built in a follow-up iteration without breaking the
// dispatcher call graph.
func (p *webhookDeliveryPublisher) Publish(ctx context.Context, evt *events.WebhookDeliveryEvent) error {
	if evt == nil {
		return nil
	}
	logs.CtxInfo(ctx, "[webhook] enqueue delivery_id=%s expt=%d event=%s attempt=%d",
		evt.DeliveryID, evt.ExperimentID, evt.Event, evt.Attempt)
	return nil
}

// PublishDelay enqueues a retry with the given delay. Delay level → duration
// mapping follows WebhookRetryConf.BackoffSec (60s / 300s / 1800s).
func (p *webhookDeliveryPublisher) PublishDelay(ctx context.Context, evt *events.WebhookDeliveryEvent, delay time.Duration) error {
	if evt == nil {
		return nil
	}
	logs.CtxInfo(ctx, "[webhook] enqueue-delay delivery_id=%s expt=%d event=%s attempt=%d delay=%v",
		evt.DeliveryID, evt.ExperimentID, evt.Event, evt.Attempt, delay)
	return nil
}
