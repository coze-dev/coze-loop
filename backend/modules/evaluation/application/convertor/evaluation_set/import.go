// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset_job"
	domain_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	evalsetpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func SourceTypeDTO2DO(src *dataset_job.SourceType) *entity.SetSourceType {
	if src == nil {
		return nil
	}
	v := entity.SetSourceType(*src)
	return &v
}

func DatasetIOEndpointDTO2DO(dto *dataset_job.DatasetIOEndpoint) *entity.DatasetIOEndpoint {
	if dto == nil {
		return nil
	}
	return &entity.DatasetIOEndpoint{
		File:    DatasetIOFileDTO2DO(dto.File),
		Dataset: DatasetIODatasetDTO2DO(dto.Dataset),
	}
}

func DatasetIOFileDTO2DO(dto *dataset_job.DatasetIOFile) *entity.DatasetIOFile {
	if dto == nil {
		return nil
	}
	res := &entity.DatasetIOFile{
		Provider:         entity.StorageProvider(dto.Provider),
		Path:             dto.Path,
		Files:            dto.Files,
		OriginalFileName: dto.OriginalFileName,
		DownloadURL:      dto.DownloadURL,
		ProviderID:       dto.ProviderID,
		ProviderAuth:     ProviderAuthDTO2DO(dto.ProviderAuth),
	}
	if dto.Format != nil {
		format := entity.FileFormat(*dto.Format)
		res.Format = &format
	}
	if dto.CompressFormat != nil {
		compress := entity.FileFormat(*dto.CompressFormat)
		res.CompressFormat = &compress
	}
	return res
}

func DatasetIODatasetDTO2DO(dto *dataset_job.DatasetIODataset) *entity.DatasetIODataset {
	if dto == nil {
		return nil
	}
	return &entity.DatasetIODataset{
		SpaceID:   dto.SpaceID,
		DatasetID: dto.DatasetID,
		VersionID: dto.VersionID,
	}
}

func ProviderAuthDTO2DO(dto *dataset_job.ProviderAuth) *entity.ProviderAuth {
	if dto == nil {
		return nil
	}
	return &entity.ProviderAuth{
		ProviderAccountID: dto.ProviderAccountID,
	}
}

func FieldMappingsDTO2DOs(dtos []*dataset_job.FieldMapping) []*entity.FieldMapping {
	if len(dtos) == 0 {
		return nil
	}
	res := make([]*entity.FieldMapping, 0, len(dtos))
	for _, dto := range dtos {
		res = append(res, FieldMappingDTO2DO(dto))
	}
	return res
}

func FieldMappingDTO2DO(dto *dataset_job.FieldMapping) *entity.FieldMapping {
	if dto == nil {
		return nil
	}
	return &entity.FieldMapping{
		Source: dto.Source,
		Target: dto.Target,
	}
}

func ConflictFieldDO2DTOs(dos []*entity.ConflictField) []*evalsetpb.ConflictField {
	if len(dos) == 0 {
		return nil
	}
	res := make([]*evalsetpb.ConflictField, 0, len(dos))
	for _, do := range dos {
		res = append(res, ConflictFieldDO2DTO(do))
	}
	return res
}

func ConflictFieldDO2DTO(do *entity.ConflictField) *evalsetpb.ConflictField {
	if do == nil {
		return nil
	}
	var detail map[string]*domain_eval_set.FieldSchema
	if len(do.Detail) > 0 {
		detail = make(map[string]*domain_eval_set.FieldSchema, len(do.Detail))
		for k, v := range do.Detail {
			detail[k] = FieldSchemaDO2DTO(v)
		}
	}
	return &evalsetpb.ConflictField{
		FieldName: gptr.Of(do.FieldName),
		DetailM:   detail,
	}
}
