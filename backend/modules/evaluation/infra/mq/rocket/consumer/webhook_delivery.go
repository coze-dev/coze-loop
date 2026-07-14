// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	componentwebhook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/webhook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// NewWebhookDeliveryEventConsumer wraps a WebhookDeliveryConsumer handler with
// the standard `IConsumerWorker` shim that reads the `webhook_delivery_event_rmq`
// key from the config loader (topic + consumer group + delay support).
func NewWebhookDeliveryEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &WebhookDeliveryEventConsumer{
		IConsumerHandler: handler,
		IConfigLoader:    loader,
	}
}

type WebhookDeliveryEventConsumer struct {
	mq.IConsumerHandler
	conf.IConfigLoader
}

// webhookDeliveryTopicMissingMarker is the log/metric marker emitted when the
// RMQ config for the webhook_delivery consumer is missing or invalid. The
// registry short-circuits via cfg.IsEnabled=false so the pod boots to
// readiness even before ops provisions the topic. Grep for this string in
// TCE logs to confirm the graceful-degrade path fired.
const webhookDeliveryTopicMissingMarker = "webhook_delivery_consumer_topic_missing_skip_start"

// ConsumerCfg loads the webhook delivery RMQ config. When the config key is
// entirely absent or the resulting cfg is not Valid() (topic / addr /
// consumer_group empty — the exact scenario we saw on boe before topic
// provisioning), it degrades gracefully by returning an explicitly disabled
// ConsumerConfig plus a warning log. Genuine loader errors still propagate so
// yaml parse failures / auth failures do not silently vanish.
func (c *WebhookDeliveryEventConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	rmqCfg := &rocket.RMQConf{}
	if err := c.UnmarshalKey(ctx, rocket.WebhookDeliveryEventRMQKey, rmqCfg); err != nil {
		return nil, err
	}
	if !rmqCfg.Valid() {
		logs.CtxWarn(ctx, "%s key=%s addr=%q topic=%q consumer_group=%q",
			webhookDeliveryTopicMissingMarker,
			rocket.WebhookDeliveryEventRMQKey,
			rmqCfg.Addr, rmqCfg.Topic, rmqCfg.ConsumerGroup)
		return &mq.ConsumerConfig{IsEnabled: gptr.Of(false)}, nil
	}
	return gptr.Of(rmqCfg.ToConsumerCfg()), nil
}

// NewNilWebhookDeliveryEventConsumer returns a rollback stub that satisfies
// mq.IConsumerWorker but declares itself disabled so registry.StartAll skips
// subscribe entirely. Mirrors the provideNilWebhookDispatcher pattern from
// commercial wire (E-I-03 rollback). Callers reach for this when a webhook
// dep is unavailable at boot or WebhookGlobalConf.DisableConsumer is set.
func NewNilWebhookDeliveryEventConsumer() mq.IConsumerWorker {
	return &nilWebhookDeliveryEventConsumer{}
}

type nilWebhookDeliveryEventConsumer struct{}

func (n *nilWebhookDeliveryEventConsumer) ConsumerCfg(_ context.Context) (*mq.ConsumerConfig, error) {
	return &mq.ConsumerConfig{IsEnabled: gptr.Of(false)}, nil
}

func (n *nilWebhookDeliveryEventConsumer) HandleMessage(_ context.Context, _ *mq.MessageExt) error {
	return nil
}

// NewWebhookDeliveryConsumer wires the retry state-machine handler. Signature
// is frozen because `cozeloop-commercial/cmd/evaluation/consumer.go` already
// invokes it with the 7-tuple (sender, deliveryRepo, publisher, configer,
// exptRepo, exptResultSvc, exptAggrResultSvc). The last three are held for
// future BITs / downstream reporting hooks (T5+) and are currently unused by
// the base retry loop.
func NewWebhookDeliveryConsumer(
	sender componentwebhook.IWebhookSender,
	deliveryRepo repo.IWebhookDeliveryRepo,
	publisher events.WebhookDeliveryEventPublisher,
	configer component.IWebhookConfiger,
	exptRepo repo.IExperimentRepo,
	exptResultSvc service.ExptResultService,
	exptAggrResultSvc service.ExptAggrResultService,
) mq.IConsumerHandler {
	return &WebhookDeliveryConsumer{
		sender:            sender,
		deliveryRepo:      deliveryRepo,
		publisher:         publisher,
		configer:          configer,
		exptRepo:          exptRepo,
		exptResultSvc:     exptResultSvc,
		exptAggrResultSvc: exptAggrResultSvc,
		now:               time.Now,
	}
}

// WebhookDeliveryConsumer is the retry state machine for the
// `evaluation_webhook_delivery` topic. See T2.5 in SDD for the contract:
//
//   - Consume WebhookDeliveryEvent → look up delivery row by DeliveryID.
//   - Skip terminal / dry_run rows (idempotent re-delivery guard).
//   - Call sender.Send → advance status per response:
//   - 2xx → succeeded, clear last_error, set finished_at (last_sent_at).
//   - non-2xx / transport error → retrying, enqueue PublishDelay(next backoff).
//   - When attempt_count == max_attempts (default 4) → final_failed, no re-enqueue.
//   - Same delivery_id row is updated across all attempts (R5 idempotency).
type WebhookDeliveryConsumer struct {
	sender            componentwebhook.IWebhookSender
	deliveryRepo      repo.IWebhookDeliveryRepo
	publisher         events.WebhookDeliveryEventPublisher
	configer          component.IWebhookConfiger
	exptRepo          repo.IExperimentRepo
	exptResultSvc     service.ExptResultService
	exptAggrResultSvc service.ExptAggrResultService
	now               func() time.Time
}

// HandleMessage decodes and processes one WebhookDeliveryEvent. Panics /
// returns are converted to nil so RocketMQ ack-s the message and the retry
// state machine keeps the source of truth in `webhook_delivery` (never rely
// on MQ redelivery — we drive our own backoff via PublishDelay).
func (c *WebhookDeliveryConsumer) HandleMessage(ctx context.Context, ext *mq.MessageExt) (err error) {
	defer func() {
		if err != nil {
			logs.CtxError(ctx, "WebhookDeliveryConsumer HandleMessage fail, err: %v", err)
		}
	}()

	evt := &events.WebhookDeliveryEvent{}
	if err := sonic.Unmarshal(ext.Body, evt); err != nil {
		logs.CtxError(ctx, "WebhookDeliveryEvent unmarshal fail, raw: %s, err: %s", string(ext.Body), err)
		return nil
	}
	if evt.DeliveryID == "" {
		logs.CtxWarn(ctx, "WebhookDeliveryEvent missing delivery_id, dropping msg_id=%v", ext.MsgID)
		return nil
	}
	return c.Process(ctx, evt)
}

// Process is the per-message logic exposed for unit tests without wiring a
// mq.MessageExt. All state changes go through `deliveryRepo.Update` so tests
// can assert the DB projection via the DAO layer.
func (c *WebhookDeliveryConsumer) Process(ctx context.Context, evt *events.WebhookDeliveryEvent) error {
	delivery, err := c.deliveryRepo.GetByDeliveryID(ctx, evt.DeliveryID)
	if err != nil {
		logs.CtxError(ctx, "webhook delivery lookup fail, id=%s err=%v", evt.DeliveryID, err)
		return nil
	}
	if delivery == nil {
		logs.CtxWarn(ctx, "webhook delivery not found, id=%s (dispatcher out of sync?)", evt.DeliveryID)
		return nil
	}
	if delivery.IsFinal() {
		logs.CtxInfo(ctx, "webhook delivery already terminal, id=%s status=%s", delivery.DeliveryID, delivery.Status)
		return nil
	}

	global := c.configer.GetWebhookConf(ctx)
	if global != nil && !global.Enabled {
		logs.CtxWarn(ctx, "webhook globally disabled, skip delivery_id=%s", delivery.DeliveryID)
		return nil
	}

	retry := c.configer.GetWebhookRetryConf(ctx)
	if retry == nil {
		retry = entity.DefaultWebhookRetryConf()
	}

	now := c.now()
	if delivery.FirstSentAt == nil {
		firstCopy := now
		delivery.FirstSentAt = &firstCopy
	}
	delivery.LastSentAt = &now
	delivery.AttemptCount++
	delivery.UpdatedAt = now

	status, sendErr := c.sender.Send(ctx, delivery)
	delivery.LastResponseCode = status
	if sendErr == nil {
		delivery.Status = entity.WebhookDeliveryStatusSucceeded
		delivery.LastError = ""
		if err := c.deliveryRepo.Update(ctx, delivery); err != nil {
			logs.CtxWarn(ctx, "webhook delivery mark succeeded failed, id=%s err=%v", delivery.DeliveryID, err)
		}
		return nil
	}

	delivery.LastError = truncateErr(sendErr.Error())
	// max_attempts is the total attempt count (dispatcher's initial POST +
	// consumer's retries). Once we've reached it, no more re-enqueue.
	maxAttempts := retry.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 4
	}
	if delivery.AttemptCount >= maxAttempts {
		delivery.Status = entity.WebhookDeliveryStatusFinalFailed
		if err := c.deliveryRepo.Update(ctx, delivery); err != nil {
			logs.CtxWarn(ctx, "webhook delivery mark final_failed failed, id=%s err=%v", delivery.DeliveryID, err)
		}
		logs.CtxWarn(ctx, "webhook delivery final_failed, id=%s attempts=%d last_status=%d",
			delivery.DeliveryID, delivery.AttemptCount, status)
		return nil
	}

	delivery.Status = entity.WebhookDeliveryStatusRetrying
	if err := c.deliveryRepo.Update(ctx, delivery); err != nil {
		logs.CtxWarn(ctx, "webhook delivery mark retrying failed, id=%s err=%v", delivery.DeliveryID, err)
	}

	delay := nextBackoff(retry.BackoffSec, delivery.AttemptCount)
	nextAttempt := delivery.AttemptCount + 1
	nextEvt := &events.WebhookDeliveryEvent{
		DeliveryID:   delivery.DeliveryID,
		SpaceID:      delivery.SpaceID,
		ExperimentID: delivery.ExperimentID,
		Event:        delivery.Event,
		Attempt:      nextAttempt,
	}
	if err := c.publisher.PublishDelay(ctx, nextEvt, delay); err != nil {
		logs.CtxWarn(ctx, "webhook delivery re-enqueue failed, id=%s attempt=%d err=%v",
			delivery.DeliveryID, nextAttempt, err)
	}
	return nil
}

// nextBackoff picks the delay for the *next* attempt. `backoff` is the
// zero-indexed schedule (e.g. [60, 300, 1800] seconds). If the schedule is
// exhausted the last entry is reused.
func nextBackoff(backoff []int, currentAttempt int) time.Duration {
	if len(backoff) == 0 {
		return 60 * time.Second
	}
	idx := currentAttempt - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(backoff) {
		idx = len(backoff) - 1
	}
	return time.Duration(backoff[idx]) * time.Second
}

// truncateErr caps error message length to fit the `last_error` VARCHAR(2048)
// column without silently overflowing in MySQL.
func truncateErr(msg string) string {
	const maxLen = 2000
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen] + "…"
}
