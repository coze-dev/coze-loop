// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func EvaluationSetDO2DTOs(dos []*entity.EvaluationSet) []*eval_set.EvaluationSet {
	if dos == nil {
		return nil
	}
	result := make([]*eval_set.EvaluationSet, 0)
	for _, do := range dos {
		result = append(result, EvaluationSetDO2DTO(do))
	}
	return result
}

func EvaluationSetDO2DTO(do *entity.EvaluationSet) *eval_set.EvaluationSet {
	if do == nil {
		return nil
	}
	var spec *dataset.DatasetSpec
	if do.Spec != nil {
		spec = &dataset.DatasetSpec{
			MaxItemCount:           gptr.Of(do.Spec.MaxItemCount),
			MaxFieldCount:          gptr.Of(do.Spec.MaxFieldCount),
			MaxItemSize:            gptr.Of(do.Spec.MaxItemSize),
			MaxItemDataNestedDepth: gptr.Of(do.Spec.MaxItemDataNestedDepth),
		}
	}
	var features *dataset.DatasetFeatures
	if do.Features != nil {
		features = &dataset.DatasetFeatures{
			EditSchema:   gptr.Of(do.Features.EditSchema),
			RepeatedData: gptr.Of(do.Features.RepeatedData),
			MultiModal:   gptr.Of(do.Features.MultiModal),
		}
	}

	return &eval_set.EvaluationSet{
		ID:                   gptr.Of(do.ID),
		AppID:                gptr.Of(do.AppID),
		WorkspaceID:          gptr.Of(do.SpaceID),
		Name:                 gptr.Of(do.Name),
		Description:          gptr.Of(do.Description),
		Status:               gptr.Of(dataset.DatasetStatus(do.Status)),
		Spec:                 spec,
		Features:             features,
		ItemCount:            gptr.Of(do.ItemCount),
		ChangeUncommitted:    gptr.Of(do.ChangeUncommitted),
		EvaluationSetVersion: VersionDO2DTO(do.EvaluationSetVersion),
		LatestVersion:        gptr.Of(do.LatestVersion),
		NextVersionNum:       gptr.Of(do.NextVersionNum),
		BaseInfo:             common.ConvertBaseInfoDO2DTO(do.BaseInfo),
	}
}
