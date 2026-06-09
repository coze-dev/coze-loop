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

var webhookRetryIntervals = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	30 * time.Minute,
}

type ExptLifecycleEventHandlerImpl struct {
	exptRepo         repo.IExperimentRepo
	notifyRPCAdapter rpc.INotifyRPCAdapter
	userProvider     rpc.IUserProvider
	webhookAdapter   rpc.IWebhookDeliveryAdapter
}

func NewExptLifecycleEventHandler(
	exptRepo repo.IExperimentRepo,
	notifyRPCAdapter rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider,
	webhookAdapter rpc.IWebhookDeliveryAdapter,
) ExptLifecycleEventHandler {
	return &ExptLifecycleEventHandlerImpl{
		exptRepo:         exptRepo,
		notifyRPCAdapter: notifyRPCAdapter,
		userProvider:     userProvider,
		webhookAdapter:   webhookAdapter,
	}
}

func (h *ExptLifecycleEventHandlerImpl) HandleLifecycleEvent(ctx context.Context, event *entity.ExptLifecycleEvent) error {
	expt, err := h.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		return err
	}

	nc := h.getNotificationConfig(expt)

	// Backward compatibility: no config means default Lark behavior for terminal states
	if nc == nil {
		switch event.ToStatus {
		case entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
			return h.sendNotifyCard(ctx, event, expt)
		default:
			return nil
		}
	}

	if !nc.ShouldNotify(event.ToStatus) {
		return nil
	}

	// Send Feishu card
	if nc.Channels != nil && nc.Channels.FeishuEnabled {
		if err := h.sendNotifyCard(ctx, event, expt); err != nil {
			logs.CtxWarn(ctx, "sendNotifyCard failed for expt %v: %v", expt.ID, err)
		}
	}

	// Send webhooks
	if nc.Channels != nil && len(nc.Channels.Webhooks) > 0 {
		h.deliverWebhooks(ctx, expt, event)
	}

	return nil
}

func (h *ExptLifecycleEventHandlerImpl) getNotificationConfig(expt *entity.Experiment) *entity.NotificationConfig {
	if expt == nil || expt.EvalConf == nil {
		return nil
	}
	return expt.EvalConf.NotificationConfig
}

func (h *ExptLifecycleEventHandlerImpl) deliverWebhooks(ctx context.Context, expt *entity.Experiment, event *entity.ExptLifecycleEvent) {
	payload := buildWebhookPayload(expt, event)

	nc := h.getNotificationConfig(expt)
	for _, wh := range nc.Channels.Webhooks {
		deliveryID := uuid.New().String()
		payloadCopy := *payload
		payloadCopy.DeliveryID = deliveryID
		whURL := wh.URL
		spaceID := expt.SpaceID
		go func() {
			h.deliverWithRetry(ctx, whURL, spaceID, &payloadCopy)
		}()
	}
}

func (h *ExptLifecycleEventHandlerImpl) deliverWithRetry(ctx context.Context, url string, spaceID int64, payload *rpc.WebhookPayload) {
	if err := h.webhookAdapter.Deliver(ctx, url, spaceID, payload); err == nil {
		return
	}
	for i, interval := range webhookRetryIntervals {
		logs.CtxWarn(ctx, "webhook delivery attempt %d failed, retrying in %v, url=%s, delivery_id=%s", i+1, interval, url, payload.DeliveryID)
		time.Sleep(interval)
		if err := h.webhookAdapter.Deliver(ctx, url, spaceID, payload); err == nil {
			logs.CtxInfo(ctx, "webhook delivery succeeded on retry %d, url=%s, delivery_id=%s", i+1, url, payload.DeliveryID)
			return
		}
	}
	logs.CtxError(ctx, "webhook delivery exhausted all retries, url=%s, delivery_id=%s", url, payload.DeliveryID)
}

func buildWebhookPayload(expt *entity.Experiment, event *entity.ExptLifecycleEvent) *rpc.WebhookPayload {
	eventName := exptStatusToEventName(event.ToStatus)
	payload := &rpc.WebhookPayload{
		Event:     eventName,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Experiment: &rpc.WebhookExptInfo{
			ID:     expt.ID,
			Name:   expt.Name,
			Status: eventName,
		},
	}
	if expt.Stats != nil {
		payload.Experiment.Progress = &rpc.WebhookExptProgress{
			Total:     expt.Stats.PendingItemCnt + expt.Stats.SuccessItemCnt + expt.Stats.FailItemCnt + expt.Stats.ProcessingItemCnt + expt.Stats.TerminatedItemCnt,
			Succeeded: expt.Stats.SuccessItemCnt,
			Failed:    expt.Stats.FailItemCnt,
		}
	}
	return payload
}

func exptStatusToEventName(status entity.ExptStatus) string {
	switch status {
	case entity.ExptStatus_Processing:
		return "started"
	case entity.ExptStatus_Success:
		return "succeeded"
	case entity.ExptStatus_Failed:
		return "failed"
	case entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
		return "terminated"
	default:
		return fmt.Sprintf("unknown_%d", status)
	}
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
