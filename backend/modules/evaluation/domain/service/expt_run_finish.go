// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// Retained only for existing adapters. Standard CompleteRun/CompleteExpt own
// the SQL boundary, admission release and all business completion duties.
//
//go:generate mockgen -destination=mocks/expt_run_finish.go -package=mocks . IExptNaturalRunFinisher
type IExptNaturalRunFinisher interface {
	FinishCurrentRun(context.Context, *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error)
	FinishCurrentRunWithHeldTickLock(context.Context, *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error)
}

func (e *ExptMangerImpl) FinishCurrentRun(ctx context.Context, event *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error) {
	if event == nil || e.runFinishRepo == nil {
		return nil, errno.NewMQRetryErr("current completion provider missing")
	}
	result := &entity.ExptRunFinishResult{}
	ctx = context.WithValue(ctx, completeCaptureKey{}, result)
	err := e.CompleteRun(ctx, event.ExptID, event.ExptRunID, event.SpaceID, event.Session)
	if result.Disposition == entity.ExptRunFinishPending {
		return result, nil
	}
	if result.Disposition == 0 {
		return nil, err
	}
	return result, err
}

func (e *ExptMangerImpl) FinishCurrentRunWithHeldTickLock(ctx context.Context, event *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error) {
	if event == nil {
		return nil, errno.NewMQRetryErr("completion event missing")
	}
	// Reuse actual caller ownership, or let standard Complete acquire the tick.
	// This compatibility promise cannot create ownership from identity alone.
	ctx = context.WithValue(ctx, completeCalibratedKey{}, true)
	return e.FinishCurrentRun(ctx, event)
}
