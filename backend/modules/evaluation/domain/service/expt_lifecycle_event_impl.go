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
	exptRepo         repo.IExperimentRepo
	notifyRPCAdapter rpc.INotifyRPCAdapter
	userProvider     rpc.IUserProvider
}

func NewExptLifecycleEventHandler(exptRepo repo.IExperimentRepo, notifyRPCAdapter rpc.INotifyRPCAdapter, userProvider rpc.IUserProvider) ExptLifecycleEventHandler {
	return &ExptLifecycleEventHandlerImpl{
		exptRepo:         exptRepo,
		notifyRPCAdapter: notifyRPCAdapter,
		userProvider:     userProvider,
	}
}

func (h *ExptLifecycleEventHandlerImpl) HandleLifecycleEvent(ctx context.Context, event *entity.ExptLifecycleEvent) error {
	expt, err := h.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		return err
	}

	// 飞书侧仍仅对 4 终态发卡片：Processing 等中间态（含本期新增的「开始执行」事件）
	// 不发飞书，但 HandleLifecycleEvent 仍正常返回，以便 commercial 层继续走 webhook fan-out。
	switch event.ToStatus {
	case entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
		return h.sendNotifyCard(ctx, event, expt)
	default:
		// 非终态：飞书不发，但不中断 fan-out（webhook 在 commercial handler 消费 Processing 等事件）。
		return nil
	}
}

func (h *ExptLifecycleEventHandlerImpl) sendNotifyCard(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) error {
	if event.ToStatus != expt.Status {
		return nil
	}
	// 飞书发送由统一通知条件驱动（feishu.enable=true 且状态命中 filter 才发）。
	// null-safe：未配置通知（历史实验/模板）走默认配置 -> 等价现有终态飞书行为（向后兼容）。
	if !expt.NotificationConf.ShouldNotifyFeishu(event.ToStatus) {
		logs.CtxInfo(ctx, "expt %v feishu notify skipped by notification_conf, status: %v", expt.ID, event.ToStatus)
		return nil
	}
	// 接收人逻辑不变：默认发实验创建人；CLI/API 创建无创建人 email 时跳过（保留原 logs.CtxWarn）。
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
