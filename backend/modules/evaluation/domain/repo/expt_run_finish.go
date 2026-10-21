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
