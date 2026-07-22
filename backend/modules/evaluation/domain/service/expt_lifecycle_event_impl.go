// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
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
		logs.CtxError(ctx, "HandleLifecycleEvent: GetByID failed, expt_id=%d, space_id=%d, to_status=%v, err=%v",
			event.ExptID, event.SpaceID, event.ToStatus, err)
		return err
	}

	// 注意: 沙箱 agent 的 experiment_started / experiment_finished 打点已从此处迁出。
	// 现在打在 SubmitExperiment (application/experiment_app.go) 和 CompleteExpt
	// (domain/service/expt_manage_execution_impl.go) 里, 走同步路径以避免 rocket MQ
	// consumer group 竞争导致灰度实例采集不到。此处仅保留飞书通知 + Webhook 分发。

	// Feishu notification
	h.handleFeishuNotification(ctx, event, expt)

	// Webhook dispatch
	h.dispatchWebhook(ctx, event, expt)

	return nil
}

// isSandboxAgentExperiment 判断实验是否属于沙箱 agent 类型。
// 优先看 Target.EvalTargetVersion.EvalTargetType, 兼容部分历史记录仅落 SandboxAgent 指针的场景。
// 保留在此文件是因为同 package 的 CompleteExpt (expt_manage_execution_impl.go) 复用。
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
// 保留在此文件是因为同 package 的 CompleteExpt (expt_manage_execution_impl.go) 复用。
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
