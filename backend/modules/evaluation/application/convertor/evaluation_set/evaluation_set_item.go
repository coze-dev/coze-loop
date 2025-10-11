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

func ItemDTO2DOs(dtos []*eval_set.EvaluationSetItem) []*entity.EvaluationSetItem {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.EvaluationSetItem, 0)
	for _, dto := range dtos {
		result = append(result, ItemDTO2DO(dto))
	}
	return result
}

func ItemDTO2DO(dto *eval_set.EvaluationSetItem) *entity.EvaluationSetItem {
	if dto == nil {
		return nil
	}
	return &entity.EvaluationSetItem{
		ID:              gptr.Indirect(dto.ID),
		AppID:           gptr.Indirect(dto.AppID),
		SpaceID:         gptr.Indirect(dto.WorkspaceID),
		EvaluationSetID: gptr.Indirect(dto.EvaluationSetID),
		SchemaID:        gptr.Indirect(dto.SchemaID),
		ItemID:          gptr.Indirect(dto.ItemID),
		ItemKey:         gptr.Indirect(dto.ItemKey),
		Turns:           TurnDTO2DOs(dto.Turns),
		BaseInfo:        common.ConvertBaseInfoDTO2DO(dto.BaseInfo),
	}
}

func TurnDTO2DOs(dtos []*eval_set.Turn) []*entity.Turn {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.Turn, 0)
	for _, dto := range dtos {
		result = append(result, TurnDTO2DO(dto))
	}
	return result
}

func TurnDTO2DO(dto *eval_set.Turn) *entity.Turn {
	if dto == nil {
		return nil
	}
	return &entity.Turn{
		ID:            gptr.Indirect(dto.ID),
		FieldDataList: FieldDataDTO2DOs(dto.FieldDataList),
	}
}

func FieldDataDTO2DOs(dtos []*eval_set.FieldData) []*entity.FieldData {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.FieldData, 0)
	for _, dto := range dtos {
		result = append(result, FieldDataDTO2DO(dto))
	}
	return result
}

func FieldDataDTO2DO(dto *eval_set.FieldData) *entity.FieldData {
	if dto == nil {
		return nil
	}
	return &entity.FieldData{
		Key:     gptr.Indirect(dto.Key),
		Name:    gptr.Indirect(dto.Name),
		Content: common.ConvertContentDTO2DO(dto.Content),
	}
}

func ItemDO2DTOs(dos []*entity.EvaluationSetItem) []*eval_set.EvaluationSetItem {
	if dos == nil {
		return nil
	}
	result := make([]*eval_set.EvaluationSetItem, 0)
	for _, do := range dos {
		result = append(result, ItemDO2DTO(do))
	}
	return result
}

func ItemDO2DTO(do *entity.EvaluationSetItem) *eval_set.EvaluationSetItem {
	if do == nil {
		return nil
	}
	return &eval_set.EvaluationSetItem{
		ID:              gptr.Of(do.ID),
		AppID:           gptr.Of(do.AppID),
		WorkspaceID:     gptr.Of(do.SpaceID),
		EvaluationSetID: gptr.Of(do.EvaluationSetID),
		SchemaID:        gptr.Of(do.SchemaID),
		ItemID:          gptr.Of(do.ItemID),
		ItemKey:         gptr.Of(do.ItemKey),
		Turns:           TurnDO2DTOs(do.Turns),
		BaseInfo:        common.ConvertBaseInfoDO2DTO(do.BaseInfo),
	}
}

func TurnDO2DTOs(dos []*entity.Turn) []*eval_set.Turn {
	if dos == nil {
		return nil
	}
	result := make([]*eval_set.Turn, 0)
	for _, do := range dos {
		result = append(result, TurnDO2DTO(do))
	}
	return result
}

func TurnDO2DTO(do *entity.Turn) *eval_set.Turn {
	if do == nil {
		return nil
	}
	return &eval_set.Turn{
		ID:            gptr.Of(do.ID),
		FieldDataList: FieldDataDO2DTOs(do.FieldDataList),
	}
}

func FieldDataDO2DTOs(dos []*entity.FieldData) []*eval_set.FieldData {
	if dos == nil {
		return nil
	}
	result := make([]*eval_set.FieldData, 0)
	for _, do := range dos {
		result = append(result, FieldDataDO2DTO(do))
	}
	return result
}

func FieldDataDO2DTO(do *entity.FieldData) *eval_set.FieldData {
	if do == nil {
		return nil
	}
	return &eval_set.FieldData{
		Key:     gptr.Of(do.Key),
		Name:    gptr.Of(do.Name),
		Content: common.ConvertContentDO2DTO(do.Content),
	}
}

func ItemErrorGroupDO2DTOs(dos []*entity.ItemErrorGroup) []*dataset.ItemErrorGroup {
	if dos == nil {
		return nil
	}
	result := make([]*dataset.ItemErrorGroup, 0)
	for _, do := range dos {
		result = append(result, ItemErrorGroupDO2DTO(do))
	}
	return result
}

func CreateDatasetItemOutputDO2DTOs(dos []*entity.CreateDatasetItemOutput) []*dataset.CreateDatasetItemOutput {
	if dos == nil {
		return nil
	}
	result := make([]*dataset.CreateDatasetItemOutput, 0)
	for _, do := range dos {
		result = append(result, CreateDatasetItemOutputDO2DTO(do))
	}
	return result
}
func CreateDatasetItemOutputDO2DTO(do *entity.CreateDatasetItemOutput) *dataset.CreateDatasetItemOutput {
	if do == nil {
		return nil
	}
	return &dataset.CreateDatasetItemOutput{
		ItemIndex: do.ItemIndex,
		ItemKey:   do.ItemKey,
		ItemID:    do.ItemID,
		IsNewItem: do.IsNewItem,
	}
}

func ItemErrorGroupDO2DTO(do *entity.ItemErrorGroup) *dataset.ItemErrorGroup {
	if do == nil {
		return nil
	}
	return &dataset.ItemErrorGroup{
		Type:       gptr.Of(dataset.ItemErrorType(gptr.Indirect(do.Type))),
		Summary:    do.Summary,
		ErrorCount: do.ErrorCount,
		Details:    ItemErrorDetailDO2DTOs(do.Details),
	}
}

func ItemErrorDetailDO2DTOs(dos []*entity.ItemErrorDetail) []*dataset.ItemErrorDetail {
	if dos == nil {
		return nil
	}
	result := make([]*dataset.ItemErrorDetail, 0)
	for _, do := range dos {
		result = append(result, ItemErrorDetailDO2DTO(do))
	}
	return result
}

func ItemErrorDetailDO2DTO(do *entity.ItemErrorDetail) *dataset.ItemErrorDetail {
	if do == nil {
		return nil
	}
	return &dataset.ItemErrorDetail{
		Message:    do.Message,
		Index:      do.Index,
		StartIndex: do.StartIndex,
		EndIndex:   do.EndIndex,
	}
}
