// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"context"
	"math"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	decimal10_4Min       = -999999.9999
	decimal10_4Max       = 999999.9999
	decimal10_4Precision = 4
)

func clampScoreToDecimal10_4(ctx context.Context, score float64) float64 {
	multiplier := math.Pow10(decimal10_4Precision)
	rounded := math.Round(score*multiplier) / multiplier

	if rounded < decimal10_4Min {
		logs.CtxWarn(ctx, "Score value %f (rounded from %f) exceeds decimal(10,4) minimum limit for experiment_id: %d, clamping to %f", rounded, score, decimal10_4Min)
		return decimal10_4Min
	}
	if rounded > decimal10_4Max {
		logs.CtxWarn(ctx, "Score value %f (rounded from %f) exceeds decimal(10,4) maximum limit for experiment_id: %d, clamping to %f", rounded, score, decimal10_4Max)
		return decimal10_4Max
	}

	return rounded
}

func ExptAggrResultDOToPO(ctx context.Context, do *entity.ExptAggrResult) *model.ExptAggrResult {
	po := &model.ExptAggrResult{
		ID:           do.ID,
		SpaceID:      do.SpaceID,
		ExperimentID: do.ExperimentID,
		FieldType:    gptr.Of(do.FieldType),
		FieldKey:     do.FieldKey,
		Score:        gptr.Of(clampScoreToDecimal10_4(ctx, do.Score)),
		AggrResult:   gptr.Of(do.AggrResult),
		Version:      do.Version,
		Status:       do.Status,
	}

	return po
}

func ExptAggrResultPOToDO(po *model.ExptAggrResult) *entity.ExptAggrResult {
	do := &entity.ExptAggrResult{
		ID:           po.ID,
		SpaceID:      po.SpaceID,
		ExperimentID: po.ExperimentID,
		FieldType:    gptr.Indirect(po.FieldType),
		FieldKey:     po.FieldKey,
		Score:        gptr.Indirect(po.Score),
		AggrResult:   gptr.Indirect(po.AggrResult),
		Version:      po.Version,
		Status:       po.Status,
		UpdateAt:     gptr.Of(po.UpdatedAt),
	}

	return do
}
