// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strconv"
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
	exptRepo           repo.IExperimentRepo
	exptTurnResultRepo repo.IExptTurnResultRepo
	notifyRPCAdapter   rpc.INotifyRPCAdapter
	userProvider       rpc.IUserProvider
	webhookSender      rpc.IWebhookSender
}

func NewExptLifecycleEventHandler(
	exptRepo repo.IExperimentRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	notifyRPCAdapter rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider,
	webhookSender rpc.IWebhookSender,
) ExptLifecycleEventHandler {
	return &ExptLifecycleEventHandlerImpl{
		exptRepo:           exptRepo,
		exptTurnResultRepo: exptTurnResultRepo,
		notifyRPCAdapter:   notifyRPCAdapter,
		userProvider:       userProvider,
		webhookSender:      webhookSender,
	}
}

func (h *ExptLifecycleEventHandlerImpl) HandleLifecycleEvent(ctx context.Context, event *entity.ExptLifecycleEvent) error {
	expt, err := h.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		return err
	}

	// 状态→对外事件映射；非通知相关状态直接忽略（向前兼容 default 分支）。
	notifyEvent, ok := entity.MapExptStatusToNotificationEvent(event.ToStatus)
	if !ok {
		return nil
	}

	// notifications 为空走默认配置（飞书✅ + 条件 [started, failed, success]），向前兼容。
	cfg := expt.Notifications
	if cfg == nil {
		cfg = entity.DefaultNotificationConfig()
	}

	// 匹配规则并聚合命中的渠道：飞书去重为单次，webhook 收集所有命中规则的 URL（跨规则去重）。
	feishuMatched := false
	webhookURLs := make([]string, 0)
	seenURL := make(map[string]bool)
	for _, rule := range cfg.GetRules() {
		if rule == nil {
			continue
		}
		if !entity.MatchNotificationCondition(rule.GetCondition(), event.ToStatus) {
			continue
		}
		for _, action := range rule.GetActions() {
			if action == nil {
				continue
			}
			switch action.GetType() {
			case entity.NotificationChannelType_Feishu:
				feishuMatched = true
			case entity.NotificationChannelType_Webhook:
				if action.Webhook == nil {
					continue
				}
				for _, u := range action.Webhook.GetUrls() {
					if u == "" || seenURL[u] {
						continue
					}
					seenURL[u] = true
					webhookURLs = append(webhookURLs, u)
				}
			default:
				// 未知渠道类型：忽略（向前兼容）。
			}
		}
	}

	// 飞书动作：复用既有 SendMessageCard 链路，接收人=创建人，无创建人不发。
	if feishuMatched {
		if err := h.sendNotifyCard(ctx, event, expt); err != nil {
			logs.CtxWarn(ctx, "[Notify] sendNotifyCard fail, expt_id: %v, err: %v", expt.ID, err)
		}
	}

	// Webhook 动作：构造 payload，每个 URL 独立投递（独立 delivery_id）。
	if len(webhookURLs) > 0 && h.webhookSender != nil {
		payloadBase := h.buildWebhookPayload(ctx, expt, notifyEvent)
		for _, u := range webhookURLs {
			payload := *payloadBase
			payload.DeliveryID = "d_" + uuid.NewString()
			deliveryEvent := &entity.WebhookDeliveryEvent{
				DeliveryID: payload.DeliveryID,
				SpaceID:    expt.SpaceID,
				ExptID:     expt.ID,
				URL:        u,
				Payload:    &payload,
				Attempt:    0,
			}
			if err := h.webhookSender.SendWebhookDelivery(ctx, deliveryEvent); err != nil {
				logs.CtxWarn(ctx, "[Webhook] SendWebhookDelivery fail, expt_id: %v, url: %v, err: %v", expt.ID, u, err)
			}
		}
	}

	return nil
}

// buildWebhookPayload 构造 webhook 回调 payload（仅基础字段，progress 为 turn 维度）。
// 返回的 payload 不含 delivery_id（由调用方按 URL 各自填充）。
func (h *ExptLifecycleEventHandlerImpl) buildWebhookPayload(ctx context.Context, expt *entity.Experiment, event entity.NotificationEvent) *entity.WebhookPayload {
	progress := h.calcTurnProgress(ctx, expt)
	return &entity.WebhookPayload{
		Event:     string(event),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Experiment: &entity.NotificationExperiment{
			ID:       strconv.FormatInt(expt.ID, 10),
			Name:     expt.Name,
			Status:   string(event),
			Progress: progress,
		},
	}
}

// calcTurnProgress 统计 turn 维度进度（决策2 / D9）：
// succeeded=success_turn_cnt, failed=fail_turn_cnt, total=各状态 turn 计数之和。
// 扫描失败时返回已统计的 progress（不阻塞通知，payload 反映触发时快照）。
func (h *ExptLifecycleEventHandlerImpl) calcTurnProgress(ctx context.Context, expt *entity.Experiment) *entity.NotificationProgress {
	progress := &entity.NotificationProgress{}
	if h.exptTurnResultRepo == nil {
		return progress
	}
	const limit int64 = 200
	var cursor int64
	for {
		results, ncursor, err := h.exptTurnResultRepo.ScanTurnResults(ctx, expt.ID, nil, cursor, limit, expt.SpaceID)
		if err != nil {
			logs.CtxWarn(ctx, "[Webhook] ScanTurnResults fail for progress, expt_id: %v, err: %v", expt.ID, err)
			return progress
		}
		for _, tr := range results {
			if tr == nil {
				continue
			}
			progress.Total++
			switch entity.TurnRunState(tr.Status) {
			case entity.TurnRunState_Success:
				progress.Succeeded++
			case entity.TurnRunState_Fail:
				progress.Failed++
			default:
			}
		}
		if len(results) == 0 || ncursor <= cursor {
			break
		}
		cursor = ncursor
	}
	return progress
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
