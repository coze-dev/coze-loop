// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func NewExptStatsConverter() ExptStatsConverter {
	return ExptStatsConverter{}
}

type ExptStatsConverter struct{}

func (ExptStatsConverter) DO2PO(stats *entity.ExptStats) *model.ExptStats {
	return &model.ExptStats{
		ID:              stats.ID,
		SpaceID:         stats.SpaceID,
		ExptID:          stats.ExptID,
		PendingCnt:      stats.PendingItemCnt,
		SuccessCnt:      stats.SuccessItemCnt,
		FailCnt:         stats.FailItemCnt,
		TerminatedCnt:   stats.TerminatedItemCnt,
		ProcessingCnt:   stats.ProcessingItemCnt,
		CreditCost:      stats.CreditCost,
		InputTokenCost:  gptr.Of(stats.InputTokenCost),
		OutputTokenCost: gptr.Of(stats.OutputTokenCost),
		CreatedAt:       stats.CreatedAt,
		UpdatedAt:       stats.UpdatedAt,
	}
}

func (ExptStatsConverter) PO2DO(stats *model.ExptStats) *entity.ExptStats {
	return &entity.ExptStats{
		ID:                stats.ID,
		SpaceID:           stats.SpaceID,
		ExptID:            stats.ExptID,
		PendingItemCnt:    stats.PendingCnt,
		SuccessItemCnt:    stats.SuccessCnt,
		FailItemCnt:       stats.FailCnt,
		TerminatedItemCnt: stats.TerminatedCnt,
		ProcessingItemCnt: stats.ProcessingCnt,
		CreditCost:        stats.CreditCost,
		InputTokenCost:    gptr.Indirect(stats.InputTokenCost),
		OutputTokenCost:   gptr.Indirect(stats.OutputTokenCost),
		CreatedAt:         stats.CreatedAt,
		UpdatedAt:         stats.UpdatedAt,
	}
}
