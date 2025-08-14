// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// ExptTurnResultFilterKeyMappingDO2PO DO2PO 将 domain 层的 ExptTurnResultFilterKeyMapping 转换为 persistence 层的 ExptTurnResultFilterKeyMapping
func ExptTurnResultFilterKeyMappingDO2PO(do *entity.ExptTurnResultFilterKeyMapping) *model.ExptTurnResultFilterKeyMapping {
	if do == nil {
		return nil
	}
	return &model.ExptTurnResultFilterKeyMapping{
		SpaceID:   do.SpaceID,
		ExptID:    do.ExptID,
		FromField: do.FromField,
		ToKey:     do.ToKey,
		FieldType: int32(do.FieldType),
	}
}

// ExptTurnResultFilterKeyMappingPO2DO PO2DO 将 persistence 层的 ExptTurnResultFilterKeyMapping 转换为 domain 层的 ExptTurnResultFilterKeyMapping
func ExptTurnResultFilterKeyMappingPO2DO(po *model.ExptTurnResultFilterKeyMapping) *entity.ExptTurnResultFilterKeyMapping {
	if po == nil {
		return nil
	}
	return &entity.ExptTurnResultFilterKeyMapping{
		SpaceID:   po.SpaceID,
		ExptID:    po.ExptID,
		FromField: po.FromField,
		ToKey:     po.ToKey,
		FieldType: entity.FieldTypeMapping(po.FieldType),
	}
}
