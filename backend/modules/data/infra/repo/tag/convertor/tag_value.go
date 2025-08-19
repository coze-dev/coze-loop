// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	entity2 "github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/tag/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/consts"
)

func TagValuePO2DO(val *model.TagValue) *entity2.TagValue {
	if val == nil {
		return nil
	}
	return &entity2.TagValue{
		ID:            val.ID,
		AppID:         val.AppID,
		SpaceID:       val.SpaceID,
		VersionNum:    val.VersionNum,
		TagKeyID:      val.TagKeyID,
		TagValueID:    val.TagValueID,
		TagValueName:  val.TagValueName,
		Description:   val.Description,
		Status:        entity2.TagStatus(val.Status),
		ParentValueID: val.ParentValueID,
		CreatedBy:     val.CreatedBy,
		CreatedAt:     val.CreatedAt,
		UpdatedBy:     val.UpdatedBy,
		UpdatedAt:     val.UpdatedAt,
		IsSystem:      val.TagValueName == consts.FallbackTagValueDefaultName,
	}
}
