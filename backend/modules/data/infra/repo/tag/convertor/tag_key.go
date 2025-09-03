// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"strings"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"github.com/bytedance/sonic"

	entity2 "github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/tag/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
)

func TagKeyPO2DO(val *model.TagKey) (*entity2.TagKey, error) {
	if val == nil {
		return nil, nil
	}
	res := &entity2.TagKey{
		ID:      val.ID,
		AppID:   val.AppID,
		SpaceID: val.SpaceID,
		// TODO need remove
		Version:        gptr.Of(val.Version),
		VersionNum:     val.VersionNum,
		TagKeyID:       val.TagKeyID,
		TagKeyName:     val.TagKeyName,
		Description:    val.Description,
		Status:         entity2.TagStatus(val.Status),
		TagType:        entity2.TagType(val.TagType),
		ParentKeyID:    val.ParentKeyID,
		CreatedBy:      val.CreatedBy,
		CreatedAt:      val.CreatedAt,
		UpdatedBy:      val.UpdatedBy,
		UpdatedAt:      val.UpdatedAt,
		TagContentType: entity2.TagContentType(*val.ContentType),
	}
	if val.ChangeLog != nil {
		if err := sonic.Unmarshal(val.ChangeLog, &res.ChangeLogs); err != nil {
			return nil, errno.JSONErr(err, "unmarshal tag key change log failed")
		}
	}
	targetTypes := strings.Split(val.TagTargetType, ",")
	res.TagTargetType = gslice.Map(targetTypes, func(val string) entity2.TagTargetType {
		return entity2.TagTargetType(val)
	})
	if val.Spec != nil {
		if err := sonic.Unmarshal(val.Spec, &res.ContentSpec); err != nil {
			return nil, errno.JSONErr(err, "unmarshal tag key content spec failed")
		}
	}
	return res, nil
}
