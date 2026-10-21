// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IExptRunFinishRepo atomically closes a current run after primary rechecks.
//
//go:generate mockgen -destination=mocks/expt_run_finish.go -package=mocks . IExptRunFinishRepo
type IExptRunFinishRepo interface {
	FinishCurrentRun(context.Context, *entity.ExptRunFinishRequest) (*entity.ExptRunFinishResult, error)
}

// IExptRunTerminationRepo shares the production finish provider without
// widening legacy finish-only implementations.
//
//go:generate mockgen -destination=mocks/expt_run_termination.go -package=mocks . IExptRunTerminationRepo
type IExptRunTerminationRepo interface {
	TerminateCurrentRun(context.Context, *entity.ExptRunTerminationRequest) (*entity.ExptRunTerminationResult, error)
	GetTerminationItemSource(context.Context, int64, int64, int64, int64) (*entity.ExptCurrentItemRunSource, error)
}
