// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"time"

	mtr "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptLifecycleEventHandlerImpl struct {
	exptRepo            repo.IExperimentRepo
	notifyRPCAdapter    rpc.INotifyRPCAdapter
	userProvider        rpc.IUserProvider
	webhookDispatcher   IWebhookDispatcher
	sandboxAgentMetrics mtr.SandboxAgentMetrics
}

func NewExptLifecycleEventHandler(exptRepo repo.IExperimentRepo, notifyRPCAdapter rpc.INotifyRPCAdapter, userProvider rpc.IUserProvider, webhookDispatcher IWebhookDispatcher, sandboxAgentMetrics mtr.SandboxAgentMetrics) ExptLifecycleEventHandler {
	return &ExptLifecycleEventHandlerImpl{
		exptRepo:            exptRepo,
		notifyRPCAdapter:    notifyRPCAdapter,
		userProvider:        userProvider,
		webhookDispatcher:   webhookDispatcher,
		sandboxAgentMetrics: sandboxAgentMetrics,
	}
}

func (h *ExptLifecycleEventHandlerImpl) HandleLifecycleEvent(ctx context.Context, event *entity.ExptLifecycleEvent) error {
	logs.CtxInfo(ctx, "HandleLifecycleEvent: received event, expt_id: %d, space_id: %d, to_status: %s", event.ExptID, event.SpaceID, event.ToStatus)
	expt, err := h.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		logs.CtxError(ctx, "HandleLifecycleEvent: GetByID failed, expt_id=%d, space_id=%d, to_status=%v, err=%v",
			event.ExptID, event.SpaceID, event.ToStatus, err)
		return err
	}

	// Sandbox agent 评测实验的稳定性打点（experiment_started / experiment_finished / experiment_duration）
	h.emitSandboxAgentExperimentMetric(ctx, event, expt)

	// Feishu notification
	h.handleFeishuNotification(ctx, event, expt)

	// Webhook dispatch
	h.dispatchWebhook(ctx, event, expt)

	return nil
}

// emitSandboxAgentExperimentMetric 仅对沙箱 agent 类型的实验打生命周期指标。
// 说明:
//   - experiment_started: ToStatus == Processing 时上报
//   - experiment_finished + experiment_duration: 终态 (Success/Failed/Terminated/SystemTerminated) 时上报
//   - duration 使用 expt.StartAt / expt.EndAt 计算; 若字段缺失, 由实现层容忍为 0
func (h *ExptLifecycleEventHandlerImpl) emitSandboxAgentExperimentMetric(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) {
	if h == nil || h.sandboxAgentMetrics == nil || expt == nil {
		logs.CtxWarn(ctx, "[sandbox_agent_metrics] emitExperimentMetric skipped, handler_nil=%v, metrics_nil=%v, expt_nil=%v",
			h == nil, h == nil || h.sandboxAgentMetrics == nil, expt == nil)
		return
	}
	if !isSandboxAgentExperiment(expt) {
		logs.CtxInfo(ctx, "[sandbox_agent_metrics] emitExperimentMetric skipped, not sandbox agent expt, expt_id=%d, to_status=%v",
			expt.ID, event.ToStatus)
		return
	}
	tags := mtr.SandboxAgentExperimentTags{
		ExperimentID:   expt.ID,
		DatasetID:      expt.EvalSetID,
		DatasetVersion: expt.EvalSetVersionID,
	}
	switch {
	case event.ToStatus == entity.ExptStatus_Processing:
		logs.CtxInfo(ctx, "[sandbox_agent_metrics] emit experiment_started, expt_id=%d, dataset_id=%d, dataset_version=%d",
			tags.ExperimentID, tags.DatasetID, tags.DatasetVersion)
		h.sandboxAgentMetrics.EmitExperimentStarted(tags)
	case entity.IsExptFinished(event.ToStatus):
		var startAt, endAt time.Time
		if expt.StartAt != nil {
			startAt = *expt.StartAt
		}
		if expt.EndAt != nil {
			endAt = *expt.EndAt
		}
		if endAt.IsZero() {
			endAt = time.Now()
		}
		logs.CtxInfo(ctx, "[sandbox_agent_metrics] emit experiment_finished, expt_id=%d, to_status=%v, start_at=%v, end_at=%v",
			tags.ExperimentID, event.ToStatus, startAt.UnixMilli(), endAt.UnixMilli())
		h.sandboxAgentMetrics.EmitExperimentFinished(tags, statusToErr(event.ToStatus), startAt, endAt)
	default:
		logs.CtxInfo(ctx, "[sandbox_agent_metrics] emitExperimentMetric no-op, expt_id=%d, to_status=%v (not Processing / terminal)",
			expt.ID, event.ToStatus)
	}
}

// isSandboxAgentExperiment 判断实验是否属于沙箱 agent 类型。
// 优先看 Target.EvalTargetVersion.EvalTargetType, 兼容部分历史记录仅落 SandboxAgent 指针的场景。
func isSandboxAgentExperiment(expt *entity.Experiment) bool {
	if expt == nil || expt.Target == nil || expt.Target.EvalTargetVersion == nil {
		return false
	}
	if expt.Target.EvalTargetVersion.EvalTargetType == entity.EvalTargetTypeSandboxAgent {
		return true
	}
	return expt.Target.EvalTargetVersion.SandboxAgent != nil
}

// statusToErr 将终态状态映射为一个"错误标记"error, 供 metrics 侧判定 success/error_type;
// 终态非 Success 视为 error, 但不携带具体分类 (由业务侧 invoke 级错误码承载)。
func statusToErr(status entity.ExptStatus) error {
	if status == entity.ExptStatus_Success {
		return nil
	}
	return errExptTerminatedWithFailure
}

// errExptTerminatedWithFailure 表征实验以非成功状态终结, 仅用于 metrics 打点分类。
var errExptTerminatedWithFailure = &exptFailureError{}

type exptFailureError struct{}

func (e *exptFailureError) Error() string { return "experiment terminated with non-success status" }

func (h *ExptLifecycleEventHandlerImpl) handleFeishuNotification(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) {
	logs.CtxInfo(ctx, "feishu_notification: enter, expt_id: %d, to_status: %v, has_notification_conf: %v",
		expt.ID, event.ToStatus, expt.NotificationConf != nil)

	// 兼容旧实验：NotificationConf 为 nil 时，保持旧行为（仅终态发送）
	if expt.NotificationConf == nil {
		switch event.ToStatus {
		case entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
			logs.CtxInfo(ctx, "feishu_notification: legacy expt, sending card for terminal status, expt_id: %d, to_status: %v", expt.ID, event.ToStatus)
			_ = h.sendNotifyCard(ctx, event, expt)
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

	matched := matchNotificationFilter(ctx, filter, event.ToStatus)
	logs.CtxInfo(ctx, "feishu_notification: expt_id: %d, to_status: %v, filter_matched: %v, will_send: %v",
		expt.ID, event.ToStatus, matched, matched)

	if matched {
		_ = h.sendNotifyCard(ctx, event, expt)
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
	receiveID, receiveIDType := resolveNotifyTarget(ctx, h.userProvider, expt)
	if receiveID == "" {
		logs.CtxWarn(ctx, "expt %v notify card without target", expt.ID)
		return nil
	}
	cardID, param := buildExptNotifyParam(expt, event.ToStatus)
	return h.notifyRPCAdapter.SendMessageCard(ctx, receiveID, receiveIDType, cardID, param)
}
