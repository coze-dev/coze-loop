// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewExptItemRefConvertor() *ExptItemRefConvertor {
	return &ExptItemRefConvertor{}
}

type ExptItemRefConvertor struct{}

func (ExptItemRefConvertor) DO2PO(do *entity.ExptItemRef) *model.ExptItemRef {
	if do == nil {
		return nil
	}
	po := &model.ExptItemRef{
		ID:               do.ID,
		SpaceID:          do.SpaceID,
		ExptID:           do.ExptID,
		ItemID:           do.ItemID,
		ItemVersionID:    do.ItemVersionID,
		EvalSetID:        do.EvalSetID,
		EvalSetVersionID: do.EvalSetVersionID,
		OrderIdx:         do.OrderIdx,
	}
	if do.ItemConfig != nil {
		b, err := json.Marshal(do.ItemConfig)
		if err == nil {
			po.ItemConfig = &b
		}
	}
	return po
}

func (ExptItemRefConvertor) PO2DO(po *model.ExptItemRef) *entity.ExptItemRef {
	if po == nil {
		return nil
	}
	do := &entity.ExptItemRef{
		ID:               po.ID,
		SpaceID:          po.SpaceID,
		ExptID:           po.ExptID,
		ItemID:           po.ItemID,
		ItemVersionID:    po.ItemVersionID,
		EvalSetID:        po.EvalSetID,
		EvalSetVersionID: po.EvalSetVersionID,
		OrderIdx:         po.OrderIdx,
	}
	if po.ItemConfig != nil && len(*po.ItemConfig) > 0 {
		cfg := &entity.ExptItemConfig{}
		if err := json.Unmarshal(*po.ItemConfig, cfg); err != nil {
			logs.Warn("ExptItemRefConvertor PO2DO unmarshal item_config fail: %v", err)
		} else {
			do.ItemConfig = cfg
		}
	}
	return do
}
