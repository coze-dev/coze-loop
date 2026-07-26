// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func VersionDO2DTOs(dos []*entity.EvaluationSetVersion) []*eval_set.EvaluationSetVersion {
	if dos == nil {
		return nil
	}
	result := make([]*eval_set.EvaluationSetVersion, 0)
	for _, do := range dos {
		result = append(result, VersionDO2DTO(do))
	}
	return result
}

func VersionDO2DTO(do *entity.EvaluationSetVersion) *eval_set.EvaluationSetVersion {
	if do == nil {
		return nil
	}
	return &eval_set.EvaluationSetVersion{
		ID:                  gptr.Of(do.ID),
		AppID:               gptr.Of(do.AppID),
		WorkspaceID:         gptr.Of(do.SpaceID),
		EvaluationSetID:     gptr.Of(do.EvaluationSetID),
		Version:             gptr.Of(do.Version),
		VersionNum:          gptr.Of(do.VersionNum),
		Description:         gptr.Of(do.Description),
		EvaluationSetSchema: SchemaDO2DTO(do.EvaluationSetSchema),
		ItemCount:           gptr.Of(do.ItemCount),
		BaseInfo:            common.ConvertBaseInfoDO2DTO(do.BaseInfo),
		SharedInfo:          SharedResourceInfoDO2DTO(do.SharedInfo),
	}
}
