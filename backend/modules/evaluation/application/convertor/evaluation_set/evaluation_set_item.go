// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	eval_set_svc "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
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
		Turns:           TurnDTO2DOs(gptr.Indirect(dto.EvaluationSetID), gptr.Indirect(dto.ItemID), dto.Turns),
		BaseInfo:        common.ConvertBaseInfoDTO2DO(dto.BaseInfo),
	}
}

func TurnDTO2DOs(evalSetID, itemID int64, dtos []*eval_set.Turn) []*entity.Turn {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.Turn, 0)
	for _, dto := range dtos {
		result = append(result, TurnDTO2DO(evalSetID, itemID, dto))
	}
	return result
}

func TurnDTO2DO(evalSetID, itemID int64, dto *eval_set.Turn) *entity.Turn {
	if dto == nil {
		return nil
	}
	return &entity.Turn{
		ID:            gptr.Indirect(dto.ID),
		FieldDataList: FieldDataDTO2DOs(dto.FieldDataList),
		ItemID:        itemID,
		EvalSetID:     evalSetID,
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
		TraceID: gptr.Indirect(dto.TraceID),
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
		TraceID: gptr.Of(do.TraceID),
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

func CreateDatasetItemOutputDO2DTOs(dos []*entity.DatasetItemOutput) []*dataset.CreateDatasetItemOutput {
	if dos == nil {
		return nil
	}
	result := make([]*dataset.CreateDatasetItemOutput, 0)
	for _, do := range dos {
		result = append(result, CreateDatasetItemOutputDO2DTO(do))
	}
	return result
}

func CreateDatasetItemOutputDO2DTO(do *entity.DatasetItemOutput) *dataset.CreateDatasetItemOutput {
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

func ItemDefDO2DTOs(dos []*entity.EvaluationSetItemDef) []*eval_set.EvaluationItemDef {
	if dos == nil {
		return nil
	}
	result := make([]*eval_set.EvaluationItemDef, 0, len(dos))
	for _, do := range dos {
		result = append(result, ItemDefDO2DTO(do))
	}
	return result
}

func ItemDefDO2DTO(do *entity.EvaluationSetItemDef) *eval_set.EvaluationItemDef {
	if do == nil {
		return nil
	}
	return &eval_set.EvaluationItemDef{
		ItemID:          gptr.Of(do.ItemID),
		WorkspaceID:     gptr.Of(do.SpaceID),
		EvaluationSetID: gptr.Of(do.EvaluationSetID),
		ItemKey:         gptr.Of(do.ItemKey),
		Status:          gptr.Of(do.Status),
		LatestVersion:   gptr.Of(do.LatestVersion),
		BaseInfo:        common.ConvertBaseInfoDO2DTO(do.BaseInfo),
	}
}

func ItemVersionDO2DTOs(dos []*entity.EvaluationSetItemVersion) []*eval_set.EvaluationItemVersion {
	if dos == nil {
		return nil
	}
	result := make([]*eval_set.EvaluationItemVersion, 0, len(dos))
	for _, do := range dos {
		result = append(result, ItemVersionDO2DTO(do))
	}
	return result
}

func ItemVersionDO2DTO(do *entity.EvaluationSetItemVersion) *eval_set.EvaluationItemVersion {
	if do == nil {
		return nil
	}
	return &eval_set.EvaluationItemVersion{
		ItemVersionID: gptr.Of(do.ItemVersionID),
		ItemID:        gptr.Of(do.ItemID),
		Version:       gptr.Of(do.Version),
		VersionNum:    gptr.Of(do.VersionNum),
		Description:   gptr.Of(do.Description),
		Turns:         TurnDO2DTOs(do.Turns),
		Status:        gptr.Of(do.Status),
		BaseInfo:      common.ConvertBaseInfoDO2DTO(do.BaseInfo),
	}
}

func ItemVersionRefDTO2DOs(dtos []*eval_set_svc.EvaluationItemVersionRef) []*entity.EvaluationItemVersionRef {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.EvaluationItemVersionRef, 0, len(dtos))
	for _, dto := range dtos {
		result = append(result, ItemVersionRefDTO2DO(dto))
	}
	return result
}

func ItemVersionRefDTO2DO(dto *eval_set_svc.EvaluationItemVersionRef) *entity.EvaluationItemVersionRef {
	if dto == nil {
		return nil
	}
	return &entity.EvaluationItemVersionRef{
		ItemID:        dto.ItemID,
		ItemVersionID: dto.ItemVersionID,
		ItemVersion:   dto.ItemVersion,
	}
}

func ItemVersionRefDO2DTOs(dos []*entity.EvaluationItemVersionRef) []*eval_set_svc.EvaluationItemVersionRef {
	if dos == nil {
		return nil
	}
	result := make([]*eval_set_svc.EvaluationItemVersionRef, 0, len(dos))
	for _, do := range dos {
		result = append(result, ItemVersionRefDO2DTO(do))
	}
	return result
}

func ItemVersionRefDO2DTO(do *entity.EvaluationItemVersionRef) *eval_set_svc.EvaluationItemVersionRef {
	if do == nil {
		return nil
	}
	return &eval_set_svc.EvaluationItemVersionRef{
		ItemID:        do.ItemID,
		ItemVersionID: do.ItemVersionID,
		ItemVersion:   do.ItemVersion,
	}
}
