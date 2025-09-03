// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
)

// PromptLabelDO2PO converts PromptLabel DO to PO
func PromptLabelDO2PO(do *entity.PromptLabel) *model.PromptLabel {
	if do == nil {
		return nil
	}
	return &model.PromptLabel{
		ID:        do.ID,
		SpaceID:   do.SpaceID,
		LabelKey:  do.LabelKey,
		CreatedBy: do.CreatedBy,
		CreatedAt: do.CreatedAt,
		UpdatedBy: do.UpdatedBy,
		UpdatedAt: do.UpdatedAt,
	}
}

// PromptLabelPO2DO converts PromptLabel PO to DO
func PromptLabelPO2DO(po *model.PromptLabel) *entity.PromptLabel {
	if po == nil {
		return nil
	}
	return &entity.PromptLabel{
		ID:        po.ID,
		SpaceID:   po.SpaceID,
		LabelKey:  po.LabelKey,
		CreatedBy: po.CreatedBy,
		CreatedAt: po.CreatedAt,
		UpdatedBy: po.UpdatedBy,
		UpdatedAt: po.UpdatedAt,
	}
}

// BatchPromptLabelPO2DO converts batch PromptLabel PO to DO
func BatchPromptLabelPO2DO(pos []*model.PromptLabel) []*entity.PromptLabel {
	if len(pos) == 0 {
		return nil
	}
	dos := make([]*entity.PromptLabel, 0, len(pos))
	for _, po := range pos {
		if po == nil {
			continue
		}
		dos = append(dos, PromptLabelPO2DO(po))
	}
	return dos
}
