// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type ExptItemResultRunLogConverter struct{}

func NewExptItemResultRunLogConverter() *ExptItemResultRunLogConverter {
	return &ExptItemResultRunLogConverter{}
}

func (c *ExptItemResultRunLogConverter) PO2DO(rl *model.ExptItemResultRunLog) *entity.ExptItemResultRunLog {
	if rl == nil {
		return nil
	}
	do := &entity.ExptItemResultRunLog{
		ID:          rl.ID,
		SpaceID:     rl.SpaceID,
		ExptID:      rl.ExptID,
		ExptRunID:   rl.ExptRunID,
		ItemID:      rl.ItemID,
		Status:      rl.Status,
		ErrMsg:      gptr.Indirect(rl.ErrMsg),
		LogID:       rl.LogID,
		ResultState: gptr.Indirect(rl.ResultState),
		UpdatedAt:   gptr.Of(rl.UpdatedAt),
	}

	return do
}

func (c *ExptItemResultRunLogConverter) DO2PO(do *entity.ExptItemResultRunLog) *model.ExptItemResultRunLog {
	if do == nil {
		return nil
	}
	po := &model.ExptItemResultRunLog{
		ID:          do.ID,
		SpaceID:     do.SpaceID,
		ExptID:      do.ExptID,
		ExptRunID:   do.ExptRunID,
		ItemID:      do.ItemID,
		Status:      do.Status,
		ErrMsg:      gptr.Of(do.ErrMsg),
		LogID:       do.LogID,
		ResultState: gptr.Of(do.ResultState),
	}

	if do.UpdatedAt != nil {
		po.UpdatedAt = gptr.Indirect(do.UpdatedAt)
	}

	return po
}
