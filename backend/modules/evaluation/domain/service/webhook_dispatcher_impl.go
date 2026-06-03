// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	webhookcomp "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/webhook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type WebhookDispatcherImpl struct {
	deliveryRepo repo.IWebhookDeliveryRepo
	publisher    events.ExptEventPublisher
	configer     component.IConfiger
}

func NewWebhookDispatcher(
	deliveryRepo repo.IWebhookDeliveryRepo,
	publisher events.ExptEventPublisher,
	configer component.IConfiger,
) webhookcomp.IWebhookDispatcher {
	return &WebhookDispatcherImpl{
		deliveryRepo: deliveryRepo,
		publisher:    publisher,
		configer:     configer,
	}
}

func (d *WebhookDispatcherImpl) Dispatch(ctx context.Context, expt *entity.Experiment, eventType entity.WebhookEventType) error {
	globalConf := d.configer.GetWebhookConf(ctx)
	if globalConf == nil || !globalConf.Enable {
		logs.CtxInfo(ctx, "[Webhook] global webhook disabled, skip dispatch for expt %d", expt.ID)
		return nil
	}

	notifConf := expt.NotificationConf
	if notifConf == nil || notifConf.Webhook == nil || !notifConf.Webhook.Enable {
		return nil
	}

	urls := parseWebhookURLs(notifConf.Webhook.URLs)
	if len(urls) == 0 {
		return nil
	}

	if globalConf.MaxURLsPerExperiment > 0 && len(urls) > globalConf.MaxURLsPerExperiment {
		urls = urls[:globalConf.MaxURLsPerExperiment]
	}

	now := time.Now()
	for _, webhookURL := range urls {
		deliveryID := uuid.New().String()

		delivery := &entity.WebhookDelivery{
			SpaceID:      expt.SpaceID,
			DeliveryID:   deliveryID,
			ExperimentID: expt.ID,
			EventType:    eventType,
			WebhookURL:   webhookURL,
			Status:       entity.DeliveryStatus_Pending,
			RetryCount:   0,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := d.deliveryRepo.Create(ctx, delivery); err != nil {
			logs.CtxWarn(ctx, "[Webhook] create delivery record failed, expt_id=%d url=%s err=%v", expt.ID, webhookURL, err)
			continue
		}

		event := &entity.WebhookDeliveryEvent{
			DeliveryID:   deliveryID,
			SpaceID:      expt.SpaceID,
			ExperimentID: expt.ID,
			EventType:    eventType,
			WebhookURL:   webhookURL,
			RetryCount:   0,
		}
		if err := d.publisher.PublishWebhookDeliveryEvent(ctx, event, nil); err != nil {
			logs.CtxWarn(ctx, "[Webhook] publish delivery event failed, delivery_id=%s expt_id=%d err=%v", deliveryID, expt.ID, err)
		}
	}

	return nil
}

func parseWebhookURLs(urlStr string) []string {
	if urlStr == "" {
		return nil
	}
	parts := strings.Split(urlStr, ",")
	urls := make([]string, 0, len(parts))
	for _, p := range parts {
		u := strings.TrimSpace(p)
		if u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}
