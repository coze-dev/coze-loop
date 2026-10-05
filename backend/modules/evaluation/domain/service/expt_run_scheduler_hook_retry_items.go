// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func (s *ExptSchedulerImpl) boundRetryItems(event *entity.ExptScheduleEvent) bool {
	return s.hookBoundInitializer != nil && s.hookBoundMode == entity.EvaluationModeRetryItems && event.ExptRunMode == entity.EvaluationModeRetryItems
}

func (s *ExptSchedulerImpl) finishRetryItemsTick(ctx context.Context, event *entity.ExptScheduleEvent) (bool, error) {
	err := s.Manager.CompleteExpt(ctx, event.ExptID, &event.ExptRunID, event.SpaceID, event.Session)
	if errors.Is(err, entity.ErrHookFinalizationUnsettled) || errors.Is(err, entity.ErrHookStoreConflict) {
		return true, nil
	}
	return false, err
}
