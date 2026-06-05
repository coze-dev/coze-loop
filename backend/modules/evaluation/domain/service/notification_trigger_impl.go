// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// NotificationTriggerServiceImpl 通知触发服务实现
type NotificationTriggerServiceImpl struct {
	webhookDeliverySvc component.IWebhookDeliveryService
	notifyRPC          rpc.INotifyRPCAdapter
	userProvider       rpc.IUserProvider
	exptRepo           repo.IExperimentRepo
	exptStatsRepo      repo.IExptStatsRepo
}

// NewNotificationTriggerService 创建通知触发服务
func NewNotificationTriggerService(
	webhookDeliverySvc component.IWebhookDeliveryService,
	notifyRPC rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider,
	exptRepo repo.IExperimentRepo,
	exptStatsRepo repo.IExptStatsRepo,
) INotificationTriggerService {
	return &NotificationTriggerServiceImpl{
		webhookDeliverySvc: webhookDeliverySvc,
		notifyRPC:          notifyRPC,
		userProvider:       userProvider,
		exptRepo:           exptRepo,
		exptStatsRepo:      exptStatsRepo,
	}
}

// TriggerNotification 匹配通知规则并分发到各渠道
func (s *NotificationTriggerServiceImpl) TriggerNotification(ctx context.Context, event *entity.ExptLifecycleEvent, notificationConf *entity.NotificationConf) error {
	if notificationConf == nil || len(notificationConf.Rules) == 0 {
		return nil
	}

	// 获取实验详情用于构造通知内容
	expt, err := s.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		logs.CtxError(ctx, "notification trigger: get experiment failed, expt_id: %v, error: %v", event.ExptID, err)
		return err
	}

	// 获取实验统计用于 webhook payload 的 progress
	stats, err := s.exptStatsRepo.Get(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		logs.CtxWarn(ctx, "notification trigger: get expt stats failed, expt_id: %v, error: %v", event.ExptID, err)
		// stats 获取失败不阻塞通知投递，progress 字段将为空
	}
	expt.Stats = stats

	for _, rule := range notificationConf.Rules {
		if rule == nil || rule.Condition == nil {
			continue
		}

		if !MatchFilterCondition(rule.Condition, event.ToStatus) {
			continue
		}

		// Webhook 渠道
		if rule.Webhook != nil && rule.Webhook.Enabled && len(rule.Webhook.URLs) > 0 {
			s.triggerWebhook(ctx, event, expt, rule.Webhook.URLs)
		}

		// 飞书渠道：复用现有飞书通知逻辑
		if rule.Feishu != nil && rule.Feishu.Enabled {
			s.triggerFeishu(ctx, expt)
		}
	}

	return nil
}

// triggerWebhook 对每个 URL 构造消息并投递到 MQ
func (s *NotificationTriggerServiceImpl) triggerWebhook(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment, urls []string) {
	for _, url := range urls {
		msg := BuildWebhookDeliveryMessage(event, expt, url, expt.SpaceID)
		if msg == nil {
			continue
		}
		if err := s.webhookDeliverySvc.DeliverWebhook(ctx, msg); err != nil {
			logs.CtxError(ctx, "notification trigger: deliver webhook failed, expt_id: %v, url: %v, error: %v", event.ExptID, url, err)
			// 投递失败不阻塞后续通知
		}
	}
}

// triggerFeishu 复用现有飞书消息卡片通知逻辑
func (s *NotificationTriggerServiceImpl) triggerFeishu(ctx context.Context, expt *entity.Experiment) {
	if len(expt.CreatedBy) == 0 {
		logs.CtxWarn(ctx, "notification trigger: skip feishu, no creator for expt_id: %v", expt.ID)
		return
	}

	userInfos, err := s.userProvider.MGetUserInfo(ctx, []string{expt.CreatedBy})
	if err != nil {
		logs.CtxWarn(ctx, "notification trigger: get user info failed for feishu, expt_id: %v, error: %v", expt.ID, err)
		return
	}

	if len(userInfos) != 1 || userInfos[0] == nil || userInfos[0].Email == nil || len(*userInfos[0].Email) == 0 {
		logs.CtxWarn(ctx, "notification trigger: skip feishu, no target email for expt_id: %v", expt.ID)
		return
	}

	cardID, param := buildExptNotifyParam(expt)
	if cardID == "" {
		return
	}

	if err := s.notifyRPC.SendMessageCard(ctx, *userInfos[0].Email, cardID, param); err != nil {
		logs.CtxWarn(ctx, "notification trigger: send feishu card failed, expt_id: %v, error: %v", expt.ID, err)
	}
}

// MatchFilterCondition 判断当前实验状态是否匹配过滤条件
// 特殊处理：当 values 包含 Terminated 时，SystemTerminated 也视为匹配
func MatchFilterCondition(condition *entity.NotificationFilterCondition, status entity.ExptStatus) bool {
	if condition == nil || len(condition.Values) == 0 {
		return false
	}

	matched := statusInValues(status, condition.Values)

	switch condition.Operator {
	case entity.NotificationOperatorIncludes:
		return matched
	case entity.NotificationOperatorExcludes:
		return !matched
	default:
		return false
	}
}

// statusInValues 检查 status 是否在 values 列表中，支持 Terminated 覆盖 SystemTerminated
func statusInValues(status entity.ExptStatus, values []entity.ExptStatus) bool {
	for _, v := range values {
		if v == status {
			return true
		}
		// 特殊处理：Terminated 覆盖 SystemTerminated
		if v == entity.ExptStatus_Terminated && status == entity.ExptStatus_SystemTerminated {
			return true
		}
	}
	return false
}

// exptStatusToEventString 将 ExptStatus 映射为 webhook event 字符串
func exptStatusToEventString(status entity.ExptStatus) string {
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
		return ""
	}
}

// exptStatusToString 将 ExptStatus 映射为 payload 中的 status 字符串
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
	default:
		return "unknown"
	}
}

// BuildWebhookDeliveryMessage 构造 Webhook 投递消息
func BuildWebhookDeliveryMessage(event *entity.ExptLifecycleEvent, expt *entity.Experiment, url string, spaceID int64) *entity.WebhookDeliveryMessage {
	eventStr := exptStatusToEventString(event.ToStatus)
	if eventStr == "" {
		return nil
	}

	deliveryID := uuid.New().String()

	var progress *entity.WebhookProgress
	if expt.Stats != nil {
		progress = &entity.WebhookProgress{
			Total:     expt.Stats.SuccessItemCnt + expt.Stats.FailItemCnt + expt.Stats.PendingItemCnt + expt.Stats.ProcessingItemCnt + expt.Stats.TerminatedItemCnt,
			Succeeded: expt.Stats.SuccessItemCnt,
			Failed:    expt.Stats.FailItemCnt,
		}
	}

	payload := &entity.WebhookDeliveryPayload{
		DeliveryID: deliveryID,
		Event:      eventStr,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Experiment: &entity.WebhookExperimentInfo{
			ID:       expt.ID,
			Name:     expt.Name,
			Status:   exptStatusToString(expt.Status),
			Progress: progress,
		},
	}

	return &entity.WebhookDeliveryMessage{
		DeliveryID: deliveryID,
		URL:        url,
		Payload:    payload,
		RetryCount: 0,
		SpaceID:    spaceID,
	}
}
