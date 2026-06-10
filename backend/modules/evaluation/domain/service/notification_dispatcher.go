// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// NotificationDispatcher 通知分发服务
type NotificationDispatcher struct {
	webhookService   *WebhookDeliveryService
	notifyRPCAdapter rpc.INotifyRPCAdapter
	userProvider     rpc.IUserProvider
}

// NewNotificationDispatcher 创建 NotificationDispatcher
func NewNotificationDispatcher(
	webhookService *WebhookDeliveryService,
	notifyRPCAdapter rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider,
) *NotificationDispatcher {
	return &NotificationDispatcher{
		webhookService:   webhookService,
		notifyRPCAdapter: notifyRPCAdapter,
		userProvider:     userProvider,
	}
}

// Dispatch 根据 notification_conf 分发通知
// notification_conf 为空时使用默认行为（Success/Failed/Terminated/SystemTerminated 发飞书）
func (d *NotificationDispatcher) Dispatch(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) {
	trigger, ok := entity.StatusToTrigger(event.ToStatus)
	if !ok {
		return
	}

	// notification_conf 为空时使用默认行为
	if len(expt.NotificationConf) == 0 {
		d.dispatchDefaultFeishu(ctx, event, expt)
		return
	}

	// 遍历规则，匹配 trigger
	for _, rule := range expt.NotificationConf {
		if rule.Trigger != trigger {
			continue
		}
		for _, action := range rule.Actions {
			if action == nil {
				continue
			}
			switch action.Type {
			case entity.NotificationActionTypeWebhook:
				if action.URL != "" {
					d.webhookService.DeliverFirstAttempt(ctx, expt, trigger, action.URL)
				}
			case entity.NotificationActionTypeFeishu:
				d.sendFeishuNotification(ctx, expt)
			}
		}
	}
}

// dispatchDefaultFeishu 默认行为：Success/Failed/Terminated/SystemTerminated 发飞书
func (d *NotificationDispatcher) dispatchDefaultFeishu(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) {
	switch event.ToStatus {
	case entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
		d.sendFeishuNotification(ctx, expt)
	default:
		// Processing 等状态在默认行为下不发飞书
	}
}

// sendFeishuNotification 发送飞书消息卡片（复用现有逻辑）
func (d *NotificationDispatcher) sendFeishuNotification(ctx context.Context, expt *entity.Experiment) {
	if d.notifyRPCAdapter == nil || d.userProvider == nil {
		return
	}

	userInfos, err := d.userProvider.MGetUserInfo(ctx, []string{expt.CreatedBy})
	if err != nil {
		logs.CtxWarn(ctx, "expt %v notification get user info fail: %v", expt.ID, err)
		return
	}
	if len(userInfos) != 1 || userInfos[0] == nil || len(gptr.Indirect(userInfos[0].Email)) == 0 {
		logs.CtxWarn(ctx, "expt %v notify card without target email", expt.ID)
		return
	}
	cardID, param := buildExptNotifyParam(expt)
	if cardID == "" {
		return
	}
	if err := d.notifyRPCAdapter.SendMessageCard(ctx, ptr.From(userInfos[0].Email), cardID, param); err != nil {
		logs.CtxWarn(ctx, "expt %v feishu notification fail: %v", expt.ID, err)
	}
}
