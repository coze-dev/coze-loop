// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/bytedance/gg/gptr"

	webhookcomp "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/webhook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptLifecycleEventHandlerImpl struct {
	exptRepo            repo.IExperimentRepo
	notifyRPCAdapter    rpc.INotifyRPCAdapter
	userProvider        rpc.IUserProvider
	webhookDispatcher   webhookcomp.IWebhookDispatcher
}

func NewExptLifecycleEventHandler(exptRepo repo.IExperimentRepo, notifyRPCAdapter rpc.INotifyRPCAdapter, userProvider rpc.IUserProvider, webhookDispatcher webhookcomp.IWebhookDispatcher) ExptLifecycleEventHandler {
	return &ExptLifecycleEventHandlerImpl{
		exptRepo:            exptRepo,
		notifyRPCAdapter:    notifyRPCAdapter,
		userProvider:        userProvider,
		webhookDispatcher:   webhookDispatcher,
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
			logs.CtxWarn(ctx, "[Lifecycle] sendNotifyCard failed, expt_id=%d err=%v", expt.ID, err)
		}
		h.dispatchWebhook(ctx, expt, event.ToStatus)
		return nil
	case entity.ExptStatus_Processing:
		h.dispatchWebhook(ctx, expt, event.ToStatus)
		return nil
	default:
		return nil
	}
}

func (h *ExptLifecycleEventHandlerImpl) dispatchWebhook(ctx context.Context, expt *entity.Experiment, status entity.ExptStatus) {
	eventType, ok := entity.ExptStatusToWebhookEvent(status)
	if !ok {
		return
	}
	if err := h.webhookDispatcher.Dispatch(ctx, expt, eventType); err != nil {
		logs.CtxWarn(ctx, "[Lifecycle] webhook dispatch failed, expt_id=%d err=%v", expt.ID, err)
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
