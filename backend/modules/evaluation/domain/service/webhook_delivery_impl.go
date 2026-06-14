// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	webhookMaxRetry       = 3
	webhookRequestTimeout = 10 * time.Second
)

// WebhookDeliveryHandlerImpl handles webhook delivery: HTTP POST + retry on failure
type WebhookDeliveryHandlerImpl struct {
	signatureProvider IWebhookSignatureProvider
	eventPublisher    events.ExptEventPublisher
	httpClient        *http.Client
}

// NewWebhookDeliveryHandler creates a new WebhookDeliveryHandlerImpl
func NewWebhookDeliveryHandler(
	signatureProvider IWebhookSignatureProvider,
	eventPublisher events.ExptEventPublisher,
) WebhookDeliveryHandler {
	return &WebhookDeliveryHandlerImpl{
		signatureProvider: signatureProvider,
		eventPublisher:    eventPublisher,
		httpClient: &http.Client{
			Timeout: webhookRequestTimeout,
		},
	}
}

// HandleWebhookDelivery sends the webhook HTTP POST and retries on failure
func (h *WebhookDeliveryHandlerImpl) HandleWebhookDelivery(ctx context.Context, event *entity.WebhookDeliveryEvent) error {
	// Compute signature
	signature, err := h.signatureProvider.Sign(ctx, event.SpaceID, event.Timestamp, []byte(event.Payload))
	if err != nil {
		logs.CtxError(ctx, "webhook signature computation failed, delivery_id: %v, err: %v", event.DeliveryID, err)
		// Signature failure is not retryable
		return err
	}

	// Send HTTP POST
	err = h.sendWebhook(ctx, event, signature)
	if err != nil {
		logs.CtxWarn(ctx, "webhook delivery failed, delivery_id: %v, url: %v, retry_count: %v, err: %v",
			event.DeliveryID, event.WebhookURL, event.RetryCount, err)

		// Schedule retry if under max
		if event.RetryCount < event.MaxRetry {
			retryEvent := &entity.WebhookDeliveryEvent{
				DeliveryID: event.DeliveryID,
				ExptID:     event.ExptID,
				SpaceID:    event.SpaceID,
				WebhookURL: event.WebhookURL,
				Payload:    event.Payload,
				RetryCount: event.RetryCount + 1,
				MaxRetry:   event.MaxRetry,
				Timestamp:  event.Timestamp,
			}
			delayLevel := event.RetryCount + 1 // 1-based for delay mapping
			if pubErr := h.eventPublisher.PublishWebhookDeliveryEvent(ctx, retryEvent, delayLevel); pubErr != nil {
				logs.CtxError(ctx, "failed to publish webhook retry event, delivery_id: %v, retry_count: %v, err: %v",
					retryEvent.DeliveryID, retryEvent.RetryCount, pubErr)
				return errorx.Wrapf(pubErr, "publish webhook retry event fail")
			}
			logs.CtxInfo(ctx, "webhook retry scheduled, delivery_id: %v, next_retry: %v, delay_level: %v",
				retryEvent.DeliveryID, retryEvent.RetryCount, delayLevel)
		} else {
			logs.CtxWarn(ctx, "webhook delivery exhausted all retries, delivery_id: %v, url: %v, max_retry: %v",
				event.DeliveryID, event.WebhookURL, event.MaxRetry)
		}
		// Return nil to ACK the message (retry is handled via new message)
		return nil
	}

	logs.CtxInfo(ctx, "webhook delivery succeeded, delivery_id: %v, url: %v, retry_count: %v",
		event.DeliveryID, event.WebhookURL, event.RetryCount)
	return nil
}

func (h *WebhookDeliveryHandlerImpl) sendWebhook(ctx context.Context, event *entity.WebhookDeliveryEvent, signature string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, event.WebhookURL,
		strings.NewReader(event.Payload))
	if err != nil {
		return errorx.Wrapf(err, "create webhook request fail")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Coze-Timestamp", event.Timestamp)
	req.Header.Set("X-Coze-Signature", signature)
	req.Header.Set("X-Coze-Delivery-ID", event.DeliveryID)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return errorx.Wrapf(err, "webhook HTTP request fail")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
}

// WebhookLifecycleEventHandlerImpl dispatches lifecycle events to webhook URLs
type WebhookLifecycleEventHandlerImpl struct {
	exptRepo       repo.IExperimentRepo
	eventPublisher events.ExptEventPublisher
	larkNotifier   ILarkNotifier
}

// NewWebhookLifecycleEventHandler creates a new WebhookLifecycleEventHandlerImpl
func NewWebhookLifecycleEventHandler(
	exptRepo repo.IExperimentRepo,
	eventPublisher events.ExptEventPublisher,
	larkNotifier ILarkNotifier,
) WebhookLifecycleEventHandler {
	return &WebhookLifecycleEventHandlerImpl{
		exptRepo:       exptRepo,
		eventPublisher: eventPublisher,
		larkNotifier:   larkNotifier,
	}
}

// HandleLifecycleEventForWebhook processes a lifecycle event and dispatches webhook + lark notifications
func (h *WebhookLifecycleEventHandlerImpl) HandleLifecycleEventForWebhook(ctx context.Context, event *entity.ExptLifecycleEvent) error {
	expt, err := h.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		return errorx.Wrapf(err, "get experiment fail, expt_id: %v", event.ExptID)
	}

	notifications := expt.Notifications
	if notifications == nil {
		logs.CtxInfo(ctx, "no notification config for expt %v, skip", event.ExptID)
		return nil
	}

	// Check if the event matches notification conditions
	if !entity.MatchNotificationCondition(notifications, event.ToStatus) {
		logs.CtxInfo(ctx, "lifecycle event does not match notification condition, expt_id: %v, to_status: %v", event.ExptID, event.ToStatus)
		return nil
	}

	userStatus := entity.MapExptStatusToNotificationEvent(event.ToStatus)

	// Dispatch webhook notifications
	if notifications.HasWebhookURLs() {
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		payload := buildWebhookPayload(expt, userStatus)

		for _, url := range notifications.Channels.Webhook.URLs {
			deliveryID := fmt.Sprintf("%d-%s-%s-%d", event.ExptID, userStatus, shortHash(url), time.Now().UnixNano())
			deliveryEvent := &entity.WebhookDeliveryEvent{
				DeliveryID: deliveryID,
				ExptID:     event.ExptID,
				SpaceID:    event.SpaceID,
				WebhookURL: url,
				Payload:    payload,
				RetryCount: 0,
				MaxRetry:   webhookMaxRetry,
				Timestamp:  timestamp,
			}
			if pubErr := h.eventPublisher.PublishWebhookDeliveryEvent(ctx, deliveryEvent, 0); pubErr != nil {
				logs.CtxError(ctx, "failed to publish webhook delivery event, expt_id: %v, url: %v, err: %v",
					event.ExptID, url, pubErr)
				// Continue to other URLs, don't fail the whole event
			}
		}
	}

	// Dispatch lark notifications
	if notifications.IsLarkEnabled() {
		if larkErr := h.larkNotifier.NotifyExperimentStatusChange(ctx, event.SpaceID, event.ExptID, userStatus, expt.CreatedBy); larkErr != nil {
			logs.CtxError(ctx, "lark notification failed, expt_id: %v, err: %v", event.ExptID, larkErr)
			// Don't fail the event for lark notification failure
		}
	}

	return nil
}

// buildWebhookPayload constructs the JSON payload for webhook delivery
func buildWebhookPayload(expt *entity.Experiment, status string) string {
	return fmt.Sprintf(`{"event_type":"experiment.status_changed","experiment_id":%d,"space_id":%d,"name":"%s","status":"%s"}`,
		expt.ID, expt.SpaceID, expt.Name, status)
}

// shortHash returns a short hash of a string for use in delivery IDs
func shortHash(s string) string {
	if len(s) <= 8 {
		return s
	}
	// Simple hash: take first 8 chars
	return s[len(s)-8:]
}
