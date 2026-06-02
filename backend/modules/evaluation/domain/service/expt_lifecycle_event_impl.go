// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"

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
	logs.CtxInfo(ctx, "HandleLifecycleEvent: received event, expt_id: %d, space_id: %d, to_status: %s", event.ExptID, event.SpaceID, event.ToStatus)
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
	logs.CtxInfo(ctx, "feishu_notification: enter, expt_id: %d, to_status: %v, has_notification_conf: %v",
		expt.ID, event.ToStatus, expt.NotificationConf != nil)

	// 兼容旧实验：NotificationConf 为 nil 时，保持旧行为（仅终态发送）
	if expt.NotificationConf == nil {
		switch event.ToStatus {
		case entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
			logs.CtxInfo(ctx, "feishu_notification: legacy expt, sending card for terminal status, expt_id: %d, to_status: %v", expt.ID, event.ToStatus)
			h.sendNotifyCard(ctx, event, expt)
		default:
			logs.CtxInfo(ctx, "feishu_notification: legacy expt, skip non-terminal status, expt_id: %d, to_status: %v", expt.ID, event.ToStatus)
		}
		return
	}

	// 新实验：显式检查飞书通知配置
	feishuConf := expt.NotificationConf.FeishuNotification
	logs.CtxInfo(ctx, "feishu_notification: expt_id: %d, has_feishu_conf: %v, enable: %v",
		expt.ID, feishuConf != nil, feishuConf != nil && feishuConf.Enable)

	if feishuConf == nil || !feishuConf.Enable {
		logs.CtxInfo(ctx, "feishu_notification: not configured or disabled, skip notify, expt_id: %d", expt.ID)
		return
	}

	filter := expt.NotificationConf.Filter
	filterJSON, _ := json.Marshal(filter)
	logs.CtxInfo(ctx, "feishu_notification: expt_id: %d, to_status: %v, filter: %s", expt.ID, event.ToStatus, string(filterJSON))

	matched := matchNotificationFilter(filter, event.ToStatus)
	logs.CtxInfo(ctx, "feishu_notification: expt_id: %d, to_status: %v, filter_matched: %v, will_send: %v",
		expt.ID, event.ToStatus, matched, matched)

	if matched {
		h.sendNotifyCard(ctx, event, expt)
	}
}

func (h *ExptLifecycleEventHandlerImpl) dispatchWebhook(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) {
	if h.webhookDispatcher == nil {
		logs.CtxInfo(ctx, "webhook_dispatcher: not configured, skip dispatch, expt_id: %d", event.ExptID)
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
