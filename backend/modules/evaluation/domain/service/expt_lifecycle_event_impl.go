// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptLifecycleEventHandlerImpl struct {
	exptRepo         repo.IExperimentRepo
	notifyRPCAdapter rpc.INotifyRPCAdapter
	userProvider     rpc.IUserProvider
	eventPublisher   events.ExptEventPublisher
}

func NewExptLifecycleEventHandler(exptRepo repo.IExperimentRepo, notifyRPCAdapter rpc.INotifyRPCAdapter, userProvider rpc.IUserProvider, eventPublisher events.ExptEventPublisher) ExptLifecycleEventHandler {
	return &ExptLifecycleEventHandlerImpl{
		exptRepo:         exptRepo,
		notifyRPCAdapter: notifyRPCAdapter,
		userProvider:     userProvider,
		eventPublisher:   eventPublisher,
	}
}

func (h *ExptLifecycleEventHandlerImpl) HandleLifecycleEvent(ctx context.Context, event *entity.ExptLifecycleEvent) error {
	expt, err := h.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		return err
	}

	switch event.ToStatus {
	case entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
		// Send internal notification card
		if err := h.sendNotifyCard(ctx, event, expt); err != nil {
			logs.CtxError(ctx, "[ExptLifecycleEventHandler] sendNotifyCard failed, expt_id: %v, err: %v", event.ExptID, err)
		}
		// Dispatch webhook/feishu delivery events based on notification configs
		h.dispatchNotificationActions(ctx, event, expt)
		return nil
	default:
		return nil
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

// dispatchNotificationActions checks experiment notification configs and publishes
// WebhookDeliveryEvent messages for matching triggers.
func (h *ExptLifecycleEventHandlerImpl) dispatchNotificationActions(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) {
	if len(expt.Notifications) == 0 {
		return
	}

	toStatusStr := exptStatusToString(event.ToStatus)
	fromStatusStr := exptStatusToString(event.FromStatus)

	for _, nc := range expt.Notifications {
		if nc == nil {
			continue
		}
		if !matchesTrigger(nc.Trigger, toStatusStr) {
			continue
		}
		for _, action := range nc.Actions {
			if action == nil {
				continue
			}
			deliveryEvent := &entity.WebhookDeliveryEvent{
				DeliveryID:     fmt.Sprintf("%d-%d-%d", expt.ID, event.ToStatus, time.Now().UnixNano()),
				WorkspaceID:    expt.SpaceID,
				ExperimentID:   expt.ID,
				ExperimentName: expt.Name,
				FromStatus:     fromStatusStr,
				ToStatus:       toStatusStr,
				RetryCount:     0,
				MaxRetries:     3,
				CreatedAt:      time.Now().UnixMilli(),
				ActionType:     action.Type,
			}
			switch action.Type {
			case entity.NotificationActionTypeWebhook:
				if action.Webhook != nil {
					deliveryEvent.WebhookURL = action.Webhook.URL
					deliveryEvent.Secret = action.Webhook.Secret
				}
			case entity.NotificationActionTypeFeishu:
				if action.Feishu != nil {
					deliveryEvent.FeishuWebhookURL = action.Feishu.WebhookURL
					deliveryEvent.MessageTemplate = action.Feishu.MessageTemplate
				}
			default:
				logs.CtxWarn(ctx, "[ExptLifecycleEventHandler] unknown action type: %v, expt_id: %v", action.Type, expt.ID)
				continue
			}

			if err := h.eventPublisher.PublishWebhookDeliveryEvent(ctx, deliveryEvent, nil); err != nil {
				logs.CtxError(ctx, "[ExptLifecycleEventHandler] PublishWebhookDeliveryEvent failed, expt_id: %v, delivery_id: %v, err: %v",
					expt.ID, deliveryEvent.DeliveryID, err)
			} else {
				logs.CtxInfo(ctx, "[ExptLifecycleEventHandler] PublishWebhookDeliveryEvent success, expt_id: %v, delivery_id: %v, action_type: %v",
					expt.ID, deliveryEvent.DeliveryID, action.Type)
			}
		}
	}
}

// matchesTrigger checks whether a NotificationTrigger matches the given status string.
func matchesTrigger(trigger *entity.NotificationTrigger, toStatus string) bool {
	if trigger == nil {
		// No trigger means always match
		return true
	}
	if !strings.EqualFold(trigger.Field, "status") {
		return false
	}
	if !strings.EqualFold(trigger.Operator, "in") {
		return false
	}
	for _, v := range trigger.Values {
		if strings.EqualFold(v, toStatus) {
			return true
		}
	}
	return false
}

// exptStatusToString converts an ExptStatus to a human-readable string for trigger matching.
func exptStatusToString(status entity.ExptStatus) string {
	switch status {
	case entity.ExptStatus_Pending:
		return "pending"
	case entity.ExptStatus_Processing:
		return "processing"
	case entity.ExptStatus_Success:
		return "success"
	case entity.ExptStatus_Failed:
		return "failed"
	case entity.ExptStatus_Terminated:
		return "terminated"
	case entity.ExptStatus_SystemTerminated:
		return "system_terminated"
	case entity.ExptStatus_Terminating:
		return "terminating"
	case entity.ExptStatus_Draining:
		return "draining"
	default:
		return "unknown"
	}
}
