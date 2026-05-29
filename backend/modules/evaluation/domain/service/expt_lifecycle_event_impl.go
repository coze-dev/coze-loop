// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/bytedance/gg/gptr"

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
	webhookDispatcher IWebhookDispatcher
}

func NewExptLifecycleEventHandler(exptRepo repo.IExperimentRepo, notifyRPCAdapter rpc.INotifyRPCAdapter, userProvider rpc.IUserProvider, webhookDispatcher IWebhookDispatcher) ExptLifecycleEventHandler {
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

	// Feishu notification
	h.handleFeishuNotification(ctx, event, expt)

	// Webhook dispatch
	h.dispatchWebhook(ctx, event, expt)

	return nil
}

func (h *ExptLifecycleEventHandlerImpl) handleFeishuNotification(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) {
	// Check if feishu notification is configured with filter
	if expt.NotificationConf == nil || expt.NotificationConf.FeishuNotification == nil || !expt.NotificationConf.FeishuNotification.Enable {
		return
	}

	filter := expt.NotificationConf.Filter
	if filter != nil && len(filter.FilterConditions) > 0 {
		if matchNotificationFilter(filter, event.ToStatus) {
			logs.CtxInfo(ctx, "feishu_notify: filter matched, sending card, expt_id: %v, to_status: %v", expt.ID, event.ToStatus)
			h.sendNotifyCard(ctx, event, expt)
		} else {
			logs.CtxInfo(ctx, "feishu_notify: filter not matched, skip, expt_id: %v, to_status: %v", expt.ID, event.ToStatus)
		}
		return
	}

	// No filter configured but feishu notification is enabled, send for all status changes
	logs.CtxInfo(ctx, "feishu_notify: no filter, sending card, expt_id: %v, to_status: %v", expt.ID, event.ToStatus)
	h.sendNotifyCard(ctx, event, expt)
}

func (h *ExptLifecycleEventHandlerImpl) dispatchWebhook(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) {
	if h.webhookDispatcher == nil {
		return
	}
	if err := h.webhookDispatcher.Dispatch(ctx, event, expt); err != nil {
		logs.CtxWarn(ctx, "webhook_dispatcher: dispatch failed, expt_id: %d, err: %v", event.ExptID, err)
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
