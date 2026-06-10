// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

type ExptLifecycleEventHandlerImpl struct {
	exptRepo   repo.IExperimentRepo
	dispatcher *NotificationDispatcher
}

func NewExptLifecycleEventHandler(exptRepo repo.IExperimentRepo, dispatcher *NotificationDispatcher) ExptLifecycleEventHandler {
	return &ExptLifecycleEventHandlerImpl{
		exptRepo:   exptRepo,
		dispatcher: dispatcher,
	}
}

func (h *ExptLifecycleEventHandlerImpl) HandleLifecycleEvent(ctx context.Context, event *entity.ExptLifecycleEvent) error {
	expt, err := h.exptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
	if err != nil {
		return err
	}

	if event.ToStatus != expt.Status {
		return nil
	}

	h.dispatcher.Dispatch(ctx, event, expt)
	return nil
}
