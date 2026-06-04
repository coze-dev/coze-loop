// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

type IWebhookDispatcher interface {
	Dispatch(ctx context.Context, expt *entity.Experiment, event *entity.ExptLifecycleEvent) error
}

type dispatcherImpl struct {
	deliveryRepo repo.IWebhookDeliveryRepo
	publisher    events.WebhookDeliveryEventPublisher
	configer     component.IWebhookConfiger
}

type deliveryTarget struct {
	channelType string
	sourceType  string
	url         string
}

func NewWebhookDispatcher(
	deliveryRepo repo.IWebhookDeliveryRepo,
	publisher events.WebhookDeliveryEventPublisher,
	configer component.IWebhookConfiger,
) IWebhookDispatcher {
	return &dispatcherImpl{
		deliveryRepo: deliveryRepo,
		publisher:    publisher,
		configer:     configer,
	}
}

func (d *dispatcherImpl) Dispatch(ctx context.Context, expt *entity.Experiment, event *entity.ExptLifecycleEvent) error {
	if d == nil || expt == nil || event == nil {
		return nil
	}
	if event.ToStatus != expt.Status {
		return nil
	}
	eventType, ok := entity.ExptStatusToWebhookEvent(event.ToStatus)
	if !ok {
		return nil
	}
	webhookConf := entity.DefaultWebhookGlobalConf()
	if d.configer != nil {
		webhookConf = d.configer.GetWebhookConf(ctx)
	}
	if !webhookConf.IsEnabled(expt.SpaceID) {
		return nil
	}
	targets := make([]*deliveryTarget, 0)
	notificationConf := expt.NotificationConf
	if notificationConf != nil && notificationConf.Webhook != nil && notificationConf.Webhook.Enable && notificationConf.MatchStatus(event.ToStatus) {
		for _, webhookURL := range notificationConf.Webhook.GetWebhookURLs() {
			targets = append(targets, &deliveryTarget{
				channelType: "webhook",
				sourceType:  "user",
				url:         webhookURL,
			})
		}
	}
	if bitsURL := buildBitsCallbackURL(webhookConf, expt, eventType); bitsURL != nil {
		targets = append(targets, &deliveryTarget{
			channelType: "bits_callback",
			sourceType:  "bits_callback",
			url:         *bitsURL,
		})
	}
	if len(targets) == 0 {
		return nil
	}

	maxAttempts := entity.DefaultWebhookRetryConf().MaxRetries + 1
	if d.configer != nil {
		if retryConf := d.configer.GetWebhookRetryConf(ctx); retryConf != nil {
			maxAttempts = retryConf.MaxRetries + 1
		}
	}
	now := time.Now()
	for _, target := range targets {
		deliveryID := uuid.NewString()
		delivery := &entity.WebhookDelivery{
			SpaceID:      expt.SpaceID,
			ExptID:       expt.ID,
			DeliveryID:   deliveryID,
			EventType:    eventType,
			ChannelType:  target.channelType,
			WebhookURL:   target.url,
			Status:       entity.DeliveryStatusPending,
			AttemptCount: 0,
			MaxAttempts:  maxAttempts,
			CreatedAt:    now,
			UpdatedAt:    now,
			CreatedBy:    expt.CreatedBy,
			UpdatedBy:    expt.CreatedBy,
		}
		if err := d.deliveryRepo.Create(ctx, delivery); err != nil {
			return err
		}
		message := &entity.WebhookDeliveryMessage{
			DeliveryID: deliveryID,
			ExptID:     expt.ID,
			SpaceID:    expt.SpaceID,
			EventType:  eventType,
			WebhookURL: target.url,
			Attempt:    0,
			CreatedAt:  now.Unix(),
			SourceType: target.sourceType,
		}
		if err := d.publisher.PublishWebhookDeliveryEvent(ctx, message, nil); err != nil {
			return err
		}
	}
	return nil
}

func buildBitsCallbackURL(conf *entity.WebhookGlobalConf, expt *entity.Experiment, eventType entity.WebhookEventType) *string {
	if expt == nil || expt.SourceType != entity.SourceType_Workflow || eventType == entity.WebhookEventStarted {
		return nil
	}
	return conf.BuildBitsCallbackURL(expt.SpaceID, expt.ID, expt.SourceID)
}
