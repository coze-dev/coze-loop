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

func SchemaDTO2DO(dto *eval_set.EvaluationSetSchema) *entity.EvaluationSetSchema {
	if dto == nil {
		return nil
	}
	return &entity.EvaluationSetSchema{
		ID:              gptr.Indirect(dto.ID),
		AppID:           gptr.Indirect(dto.AppID),
		SpaceID:         gptr.Indirect(dto.WorkspaceID),
		EvaluationSetID: gptr.Indirect(dto.EvaluationSetID),
		FieldSchemas:    FieldSchemaDTO2DOs(dto.FieldSchemas),
		BaseInfo:        common.ConvertBaseInfoDTO2DO(dto.BaseInfo),
	}
}

func FieldSchemaDTO2DOs(dtos []*eval_set.FieldSchema) []*entity.FieldSchema {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.FieldSchema, 0)
	for _, dto := range dtos {
		result = append(result, FieldSchemaDTO2DO(dto))
	}
	return result
}

func FieldSchemaDTO2DO(dto *eval_set.FieldSchema) *entity.FieldSchema {
	if dto == nil {
		return nil
	}
	var multiModelSpec *entity.MultiModalSpec
	if dto.MultiModelSpec != nil {
		multiModelSpec = &entity.MultiModalSpec{
			MaxFileCount:     gptr.Indirect(dto.MultiModelSpec.MaxFileCount),
			MaxFileSize:      gptr.Indirect(dto.MultiModelSpec.MaxFileSize),
			SupportedFormats: dto.MultiModelSpec.SupportedFormats,
			MaxPartCount:     gptr.Indirect(dto.MultiModelSpec.MaxPartCount),
		}
	}
	return &entity.FieldSchema{
		Key:                    gptr.Indirect(dto.Key),
		Name:                   gptr.Indirect(dto.Name),
		Description:            gptr.Indirect(dto.Description),
		ContentType:            common.ConvertContentTypeDTO2DO(gptr.Indirect(dto.ContentType)),
		DefaultDisplayFormat:   entity.FieldDisplayFormat(gptr.Indirect(dto.DefaultDisplayFormat)),
		Status:                 entity.FieldStatus(gptr.Indirect(dto.Status)),
		TextSchema:             gptr.Indirect(dto.TextSchema),
		MultiModelSpec:         multiModelSpec,
		Hidden:                 gptr.Indirect(dto.Hidden),
		IsRequired:             gptr.Indirect(dto.IsRequired),
		DefaultTransformations: dto.DefaultTransformations,
	}
}

func SchemaDO2DTO(do *entity.EvaluationSetSchema) *eval_set.EvaluationSetSchema {
	if do == nil {
		return nil
	}
	return &eval_set.EvaluationSetSchema{
		ID:              gptr.Of(do.ID),
		AppID:           gptr.Of(do.AppID),
		WorkspaceID:     gptr.Of(do.SpaceID),
		EvaluationSetID: gptr.Of(do.EvaluationSetID),
		FieldSchemas:    FieldSchemaDO2DTOs(do.FieldSchemas),
		BaseInfo:        common.ConvertBaseInfoDO2DTO(do.BaseInfo),
	}
}

func FieldSchemaDO2DTOs(dos []*entity.FieldSchema) []*eval_set.FieldSchema {
	if dos == nil {
		return nil
	}
	result := make([]*eval_set.FieldSchema, 0)
	for _, do := range dos {
		result = append(result, FieldSchemaDO2DTO(do))
	}
	return result
}

func MultiModalSpecDO2DTO(do *entity.MultiModalSpec) *dataset.MultiModalSpec {
	if do == nil {
		return nil
	}
	return &dataset.MultiModalSpec{
		MaxFileCount:     gptr.Of(do.MaxFileCount),
		MaxFileSize:      gptr.Of(do.MaxFileSize),
		SupportedFormats: do.SupportedFormats,
		MaxPartCount:     gptr.Of(do.MaxPartCount),
	}
}

func FieldSchemaDO2DTO(do *entity.FieldSchema) *eval_set.FieldSchema {
	if do == nil {
		return nil
	}
	return &eval_set.FieldSchema{
		Key:                    gptr.Of(do.Key),
		Name:                   gptr.Of(do.Name),
		Description:            gptr.Of(do.Description),
		ContentType:            gptr.Of(common.ConvertContentTypeDO2DTO(do.ContentType)),
		DefaultDisplayFormat:   gptr.Of(dataset.FieldDisplayFormat(do.DefaultDisplayFormat)),
		Status:                 gptr.Of(dataset.FieldStatus(do.Status)),
		TextSchema:             gptr.Of(do.TextSchema),
		MultiModelSpec:         MultiModalSpecDO2DTO(do.MultiModelSpec),
		Hidden:                 gptr.Of(do.Hidden),
		IsRequired:             gptr.Of(do.IsRequired),
		DefaultTransformations: do.DefaultTransformations,
	}
}
