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
	decimal10_2Min       = -999999.99
	decimal10_2Max       = 999999.99
	decimal10_2Precision = 2
)

func clampScoreToDecimal10_2(ctx context.Context, score float64) float64 {
	multiplier := math.Pow10(decimal10_2Precision)
	rounded := math.Round(score*multiplier) / multiplier

	if rounded < decimal10_2Min {
		logs.CtxWarn(ctx, "Score value %f (rounded from %f) exceeds decimal(10,2) minimum limit for experiment_id: %d, clamping to %f", rounded, score, decimal10_2Min)
		return decimal10_2Min
	}
	if rounded > decimal10_2Max {
		logs.CtxWarn(ctx, "Score value %f (rounded from %f) exceeds decimal(10,2) maximum limit for experiment_id: %d, clamping to %f", rounded, score, decimal10_2Max)
		return decimal10_2Max
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
		Score:        gptr.Of(clampScoreToDecimal10_2(ctx, do.Score)),
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
