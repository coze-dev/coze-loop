// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/bytedance/gg/gptr"

	componentwebhook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/webhook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptLifecycleEventHandlerImpl struct {
	exptRepo          repo.IExperimentRepo
	notifyRPCAdapter  rpc.INotifyRPCAdapter
	userProvider      rpc.IUserProvider
	webhookDispatcher componentwebhook.IWebhookDispatcher
}

// NewExptLifecycleEventHandler extends the 3-arg signature to 4 args so the
// commercial wire graph can inject a real dispatcher (nil-safe: a nil
// dispatcher makes the webhook path a no-op and leaves the feishu path
// untouched).
func NewExptLifecycleEventHandler(
	exptRepo repo.IExperimentRepo,
	notifyRPCAdapter rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider,
	webhookDispatcher componentwebhook.IWebhookDispatcher,
) ExptLifecycleEventHandler {
	return &ExptLifecycleEventHandlerImpl{
		exptRepo:          exptRepo,
		notifyRPCAdapter:  notifyRPCAdapter,
		userProvider:      userProvider,
		webhookDispatcher: webhookDispatcher,
	}
}

func (h *ExptLifecycleEventHandlerImpl) HandleLifecycleEvent(ctx context.Context, event *entity.ExptLifecycleEvent) error {
	expt, err := h.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		return err
	}

	switch event.ToStatus {
	case entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
		if err := h.sendNotifyCard(ctx, event, expt); err != nil {
			logs.CtxWarn(ctx, "expt %d feishu notify failed, err=%v", expt.ID, err)
		}
		h.dispatchWebhook(ctx, event, expt)
		return nil
	default:
		return nil
	}
}

func (h *ExptLifecycleEventHandlerImpl) dispatchWebhook(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) {
	if h.webhookDispatcher == nil {
		return
	}
	req := &componentwebhook.DispatchRequest{
		SpaceID:    event.SpaceID,
		Experiment: expt,
		Event:      lifecycleStatusToWebhookEvent(event.ToStatus),
		NotifyConf: expt.NotificationConf,
	}
	if err := h.webhookDispatcher.Dispatch(ctx, req); err != nil {
		logs.CtxWarn(ctx, "expt %d webhook dispatch failed, err=%v", expt.ID, err)
	}
}

func lifecycleStatusToWebhookEvent(status entity.ExptStatus) string {
	switch status {
	case entity.ExptStatus_Success:
		return entity.WebhookDeliveryEventSucceeded
	case entity.ExptStatus_Failed:
		return entity.WebhookDeliveryEventFailed
	case entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
		return entity.WebhookDeliveryEventTerminated
	default:
		return entity.WebhookDeliveryEventStarted
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
