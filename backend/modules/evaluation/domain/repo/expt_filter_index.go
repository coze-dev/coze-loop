// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IExptFilterIndexRepo is optional; existing repositories retain their legacy path.
type IExptFilterIndexRepo interface {
	SnapshotPlan(context.Context, []int64) (*entity.ExptFilterPlan, error)
	SaveIndex(context.Context, []*entity.ExptTurnResultFilterEntity, *entity.ExptFilterPlan, entity.ExptFilterOperation) error
	ReadIndex(context.Context, entity.ExptFilterTarget, entity.ExptFilterReadQuery) ([]*entity.ExptFilterRow, error)
	ScanIndexKeys(context.Context, entity.ExptFilterTarget, *entity.ExptFilterTargetCursor, int) ([]entity.ExptFilterStoredKey, *entity.ExptFilterTargetCursor, error)
}

// IExptFilterSourceScanner reads current non-deleted sources from the primary.
// Empty pages mean exhaustion; lookup and decoding failures must return an error.
// ID bounds are scan bounds, not transaction commit boundaries.
type IExptFilterSourceScanner interface {
	BeginScan(context.Context) (entity.ExptFilterScanBounds, error)
	ScanExperiments(context.Context, entity.ExptFilterScanBounds, int64, int) (entity.ExptFilterExperimentPage, error)
	TurnUpperBound(context.Context, entity.ExptFilterScope) (int64, error)
	ScanTurns(context.Context, entity.ExptFilterScope, int64, int64, int) ([]*entity.ExptTurnResult, int64, error)
}
