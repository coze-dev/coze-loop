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

	switch event.ToStatus {
	case entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated:
		return h.sendNotifyCard(ctx, event, expt)
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
