// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate  mockgen -destination  ./mocks/expt_run.go  --package mocks . ExptSchedulerEvent,ExptItemEvalEvent,QuotaService
type ExptSchedulerEvent interface {
	Schedule(ctx context.Context, event *entity.ExptScheduleEvent) error
}

type ExptItemEvalEvent interface {
	Eval(ctx context.Context, event *entity.ExptItemEvalEvent) error
}

type QuotaService interface {
	AllowExptRun(ctx context.Context, exptID, spaceID int64, session *entity.Session) error
	ReleaseExptRun(ctx context.Context, exptID, spaceID int64, session *entity.Session) error
}
