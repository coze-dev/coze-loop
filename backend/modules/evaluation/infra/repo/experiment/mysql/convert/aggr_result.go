// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func ExptAggrResultDOToPO(do *entity.ExptAggrResult) *model.ExptAggrResult {
	po := &model.ExptAggrResult{
		ID:           do.ID,
		SpaceID:      do.SpaceID,
		ExperimentID: do.ExperimentID,
		FieldType:    gptr.Of(do.FieldType),
		FieldKey:     do.FieldKey,
		Score:        gptr.Of(do.Score),
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
