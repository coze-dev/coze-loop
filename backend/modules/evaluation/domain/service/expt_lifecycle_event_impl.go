// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/google/uuid"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptLifecycleEventHandlerImpl struct {
	exptRepo               repo.IExperimentRepo
	notifyRPCAdapter       rpc.INotifyRPCAdapter
	userProvider           rpc.IUserProvider
	webhookDeliveryService rpc.IWebhookDeliveryService
}

func NewExptLifecycleEventHandler(
	exptRepo repo.IExperimentRepo,
	notifyRPCAdapter rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider,
	webhookDeliveryService rpc.IWebhookDeliveryService,
) ExptLifecycleEventHandler {
	return &ExptLifecycleEventHandlerImpl{
		exptRepo:               exptRepo,
		notifyRPCAdapter:       notifyRPCAdapter,
		userProvider:           userProvider,
		webhookDeliveryService: webhookDeliveryService,
	}
}

func (h *ExptLifecycleEventHandlerImpl) HandleLifecycleEvent(ctx context.Context, event *entity.ExptLifecycleEvent) error {
	expt, err := h.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		return err
	}

	// Load notification config; use default if not set.
	notifConfig := getNotificationConfig(expt)

	// Check if the status transition matches the notification condition.
	if !matchesCondition(notifConfig, event.ToStatus) {
		return nil
	}

	// Dispatch to Lark channel if enabled.
	if notifConfig.LarkChannel == nil || notifConfig.LarkChannel.Enabled {
		if err := h.sendNotifyCard(ctx, event, expt); err != nil {
			logs.CtxWarn(ctx, "sendNotifyCard failed for expt %v, err: %v", expt.ID, err)
		}
	}

	// Dispatch to Webhook channel if enabled.
	if notifConfig.WebhookChannel != nil && notifConfig.WebhookChannel.Enabled && len(notifConfig.WebhookChannel.URLs) > 0 {
		if err := h.publishWebhookDeliveryEvents(ctx, event, expt, notifConfig.WebhookChannel.URLs); err != nil {
			logs.CtxWarn(ctx, "publishWebhookDeliveryEvents failed for expt %v, err: %v", expt.ID, err)
		}
	}

	return nil
}

func getNotificationConfig(expt *entity.Experiment) *entity.NotificationConfig {
	if expt.EvalConf != nil && expt.EvalConf.NotificationConfig != nil {
		return expt.EvalConf.NotificationConfig
	}
	return entity.DefaultNotificationConfig()
}

func (h *ExptLifecycleEventHandlerImpl) sendNotifyCard(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) error {
	if event.ToStatus != expt.Status {
		return nil
	}
	userInfos, err := h.userProvider.MGetUserInfo(ctx, []string{expt.CreatedBy})
	if err != nil {
		return err
	}
	if len(userInfos) != 1 || userInfos[0] == nil || len(gptr.Indirect(userInfos[0].Email)) == 0 {
		logs.CtxWarn(ctx, "expt %v notify card without target email", expt.ID)
		return nil
	}
	cardID, param := buildExptNotifyParam(expt)
	return h.notifyRPCAdapter.SendMessageCard(ctx, ptr.From(userInfos[0].Email), cardID, param)
}

func (h *ExptLifecycleEventHandlerImpl) publishWebhookDeliveryEvents(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment, urls []string) error {
	if h.webhookDeliveryService == nil {
		return nil
	}

	now := time.Now().Unix()
	events := make([]*entity.WebhookDeliveryEvent, 0, len(urls))
	for _, u := range urls {
		events = append(events, &entity.WebhookDeliveryEvent{
			DeliveryID: fmt.Sprintf("d_%s", uuid.New().String()),
			WebhookURL: u,
			ExptID:     expt.ID,
			SpaceID:    expt.SpaceID,
			ExptStatus: event.ToStatus,
			RetryCount: 0,
			MaxRetries: 3,
			CreatedAt:  now,
		})
	}

	return h.webhookDeliveryService.PublishWebhookDelivery(ctx, events)
}
