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
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
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
		logs.CtxInfo(ctx, "[Webhook] dispatch skipped: nil receiver/expt/event, d_nil=%v expt_nil=%v event_nil=%v", d == nil, expt == nil, event == nil)
		return nil
	}
	logs.CtxInfo(ctx, "[Webhook] dispatch start, expt_id=%d space_id=%d to_status=%d", expt.ID, expt.SpaceID, event.ToStatus)
	if event.ToStatus != expt.Status {
		logs.CtxInfo(ctx, "[Webhook] dispatch skipped: status mismatch, expt_id=%d event_to_status=%d expt_status=%d", expt.ID, event.ToStatus, expt.Status)
		return nil
	}
	eventType, ok := entity.ExptStatusToWebhookEvent(event.ToStatus)
	if !ok {
		logs.CtxInfo(ctx, "[Webhook] dispatch skipped: status not mapped to webhook event, expt_id=%d to_status=%d", expt.ID, event.ToStatus)
		return nil
	}
	webhookConf := entity.DefaultWebhookGlobalConf()
	if d.configer != nil {
		webhookConf = d.configer.GetWebhookConf(ctx)
	}
	if !webhookConf.IsEnabled(expt.SpaceID) {
		globalEnable := webhookConf != nil && webhookConf.Enable
		logs.CtxInfo(ctx, "[Webhook] dispatch skipped: webhook disabled at config level, expt_id=%d space_id=%d global_enable=%v configer_nil=%v", expt.ID, expt.SpaceID, globalEnable, d.configer == nil)
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
		notifConfNil := notificationConf == nil
		userWebhookEnable := false
		userURLsCount := 0
		matchStatus := false
		if !notifConfNil {
			matchStatus = notificationConf.MatchStatus(event.ToStatus)
			if notificationConf.Webhook != nil {
				userWebhookEnable = notificationConf.Webhook.Enable
				userURLsCount = len(notificationConf.Webhook.GetWebhookURLs())
			}
		}
		logs.CtxInfo(ctx, "[Webhook] dispatch skipped: no targets, expt_id=%d space_id=%d source_type=%d event_type=%v notif_conf_nil=%v user_webhook_enable=%v user_url_count=%d match_status=%v",
			expt.ID, expt.SpaceID, expt.SourceType, eventType, notifConfNil, userWebhookEnable, userURLsCount, matchStatus)
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
