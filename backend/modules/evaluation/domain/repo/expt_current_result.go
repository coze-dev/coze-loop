// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IExptCurrentResultRepo is implemented by the existing item repository.
// Every write holds the canonical row lock and checks the frozen source in one transaction.
//
//go:generate mockgen -destination=mocks/expt_current_result.go -package=mocks . IExptCurrentResultRepo
type IExptCurrentResultRepo interface {
	GetCurrentItemRunSource(context.Context, int64, int64, int64, int64) (*entity.ExptCurrentItemRunSource, error)
	StoreCurrentTurnRunResult(context.Context, *entity.ExptCurrentItemRunSource, *entity.ExptTurnResultRunLog) (bool, error)
	CompleteCurrentItemRun(context.Context, *entity.ExptCurrentItemRunSource, *entity.ExptCurrentItemRunWrite) (bool, error)
}
