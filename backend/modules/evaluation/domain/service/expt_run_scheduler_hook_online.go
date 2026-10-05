// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func (e *ExptSchedulerImpl) boundOnline(event *entity.ExptScheduleEvent) bool {
	return event != nil && event.ExptRunMode == entity.EvaluationModeAppend && e.hookBoundInitializer != nil && e.hookBoundMode == entity.EvaluationModeAppend
}

func (e *ExptSchedulerImpl) finishOnlineTick(ctx context.Context, event *entity.ExptScheduleEvent, toSubmit, incomplete int) (bool, error) {
	key := entity.HookRunKey{WorkspaceID: event.SpaceID, ExperimentID: event.ExptID, RunID: event.ExptRunID}
	source, err := e.hookScheduler.ReadFinalizationSource(ctx, key)
	if err != nil {
		return true, err
	}
	if source == nil || source.RunLog == nil {
		return true, entity.ErrHookStoreCorrupt
	}
	if entity.ExptStatus(source.RunLog.Status) != entity.ExptStatus_Draining {
		return toSubmit > 0 || incomplete > 0, nil
	}
	err = e.Manager.CompleteExpt(ctx, event.ExptID, &event.ExptRunID, event.SpaceID, event.Session)
	if errors.Is(err, entity.ErrHookFinalizationUnsettled) || errors.Is(err, entity.ErrHookStoreConflict) {
		return true, nil
	}
	return false, err
}
