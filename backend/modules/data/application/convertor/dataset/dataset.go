// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
)

func DatasetDO2DTO(d *entity.Dataset, schema *entity.DatasetSchema) (dto *dataset.Dataset, err error) {
	if d == nil {
		return nil, nil
	}
	dto = &dataset.Dataset{
		ID:                d.ID,
		AppID:             gptr.Of(d.AppID),
		SpaceID:           d.SpaceID,
		SchemaID:          d.SchemaID,
		Name:              gptr.Of(d.Name),
		Description:       d.Description,
		Status:            gptr.Of(DatasetStatusDO2DTO(d.Status)),
		Category:          gptr.Of(DatasetCategoryDO2DTO(d.Category)),
		BizCategory:       gptr.Of(d.BizCategory),
		Tag:               gptr.Of(d.Tag),
		SecurityLevel:     gptr.Of(SecurityLevelDO2DTO(d.SecurityLevel)),
		Visibility:        gptr.Of(DatasetVisibilityDO2DTO(d.Visibility)),
		Spec:              DatasetSpecDO2DTO(d.Spec),
		Features:          DatasetFeaturesDO2DTO(d.Features),
		LatestVersion:     gptr.Of(d.LatestVersion),
		NextVersionNum:    gptr.Of(d.NextVersionNum),
		ChangeUncommitted: gptr.Of(d.IsChangeUncommitted()),
		ItemCount:         gptr.Of(int64(0)), // 需要额外从 redis 读取，此处不填充
		CreatedBy:         gptr.Of(d.CreatedBy),
		CreatedAt:         gptr.Of(d.CreatedAt.UnixMilli()),
		UpdatedBy:         gptr.Of(d.UpdatedBy),
		UpdatedAt:         gptr.Of(d.UpdatedAt.UnixMilli()),
	}
	if d.ExpiredAt != nil {
		dto.ExpiredAt = gptr.Of(d.ExpiredAt.UnixMilli())
	}
	if schema != nil {
		dto.Schema, err = SchemaDO2DTO(schema)
		if err != nil {
			return nil, err
		}
	}
	return dto, nil
}

func SchemaDO2DTO(s *entity.DatasetSchema) (*dataset.DatasetSchema, error) {
	fields, err := gslice.TryMap(s.Fields, FieldSchemaDO2DTO).Get()
	if err != nil {
		return nil, err
	}
	return &dataset.DatasetSchema{
		ID:            gptr.Of(s.ID),
		AppID:         gptr.Of(s.AppID),
		SpaceID:       gptr.Of(s.SpaceID),
		DatasetID:     gptr.Of(s.DatasetID),
		Fields:        fields,
		Immutable:     gptr.Of(s.Immutable),
		CreatedBy:     gptr.Of(s.CreatedBy),
		CreatedAt:     gptr.Of(s.CreatedAt.UnixMilli()),
		UpdatedBy:     gptr.Of(s.UpdatedBy),
		UpdatedAt:     gptr.Of(s.UpdatedAt.UnixMilli()),
		UpdateVersion: gptr.Of(s.UpdateVersion),
	}, nil
}

func DatasetStatusDO2DTO(dc entity.DatasetStatus) dataset.DatasetStatus {
	switch dc {
	case entity.DatasetStatusAvailable:
		return dataset.DatasetStatus_Available
	case entity.DatasetStatusDeleted:
		return dataset.DatasetStatus_Deleted
	case entity.DatasetStatusExpired:
		return dataset.DatasetStatus_Expired
	case entity.DatasetStatusImporting:
		return dataset.DatasetStatus_Importing
	case entity.DatasetStatusExporting:
		return dataset.DatasetStatus_Exporting
	case entity.DatasetStatusIndexing:
		return dataset.DatasetStatus_Indexing
	default:
		return dataset.Dataset_Status_DEFAULT
	}
}

func DatasetCategoryDO2DTO(dc entity.DatasetCategory) dataset.DatasetCategory {
	switch dc {
	case entity.DatasetCategoryGeneral:
		return dataset.DatasetCategory_General
	case entity.DatasetCategoryTraining:
		return dataset.DatasetCategory_Training
	case entity.DatasetCategoryValidation:
		return dataset.DatasetCategory_Validation
	case entity.DatasetCategoryEvaluation:
		return dataset.DatasetCategory_Evaluation
	default:
		return dataset.Dataset_Category_DEFAULT
	}
}

func SecurityLevelDO2DTO(sl entity.SecurityLevel) dataset.SecurityLevel {
	switch sl {
	case entity.SecurityLevelL1:
		return dataset.SecurityLevel_L1
	case entity.SecurityLevelL2:
		return dataset.SecurityLevel_L2
	case entity.SecurityLevelL3:
		return dataset.SecurityLevel_L3
	case entity.SecurityLevelL4:
		return dataset.SecurityLevel_L4
	default:
		return dataset.Dataset_SecurityLevel_DEFAULT
	}
}

func DatasetVisibilityDO2DTO(v entity.DatasetVisibility) dataset.DatasetVisibility {
	switch v {
	case entity.DatasetVisibilityPublic:
		return dataset.DatasetVisibility_Public
	case entity.DatasetVisibilitySpace:
		return dataset.DatasetVisibility_Space
	case entity.DatasetVisibilitySystem:
		return dataset.DatasetVisibility_System
	default:
		return dataset.Dataset_Visibility_DEFAULT
	}
}

func DatasetSpecDO2DTO(s *entity.DatasetSpec) *dataset.DatasetSpec {
	if s == nil {
		return nil
	}
	return &dataset.DatasetSpec{
		MaxItemCount:  gptr.Of(s.MaxItemCount),
		MaxFieldCount: gptr.Of(s.MaxFieldCount),
		MaxItemSize:   gptr.Of(s.MaxItemSize),
	}
}

func DatasetFeaturesDO2DTO(fs *entity.DatasetFeatures) *dataset.DatasetFeatures {
	if fs == nil {
		return nil
	}
	return &dataset.DatasetFeatures{
		EditSchema:   gptr.Of(fs.EditSchema),
		RepeatedData: gptr.Of(fs.RepeatedData),
		MultiModal:   gptr.Of(fs.MultiModal),
	}
}

func ConvertCategoryDTO2DO(category dataset.DatasetCategory) entity.DatasetCategory {
	switch category {
	case dataset.DatasetCategory_General:
		return entity.DatasetCategoryGeneral
	case dataset.DatasetCategory_Training:
		return entity.DatasetCategoryTraining
	case dataset.DatasetCategory_Validation:
		return entity.DatasetCategoryValidation
	case dataset.DatasetCategory_Evaluation:
		return entity.DatasetCategoryEvaluation
	default:
		return entity.DatasetCategoryUnknown
	}
}

func SecurityLevelDTO2DO(securityLevel dataset.SecurityLevel) entity.SecurityLevel {
	switch securityLevel {
	case dataset.SecurityLevel_L1:
		return entity.SecurityLevelL1
	case dataset.SecurityLevel_L2:
		return entity.SecurityLevelL2
	case dataset.SecurityLevel_L3:
		return entity.SecurityLevelL3
	case dataset.SecurityLevel_L4:
		return entity.SecurityLevelL4
	default:
		return entity.SecurityLevelUnknown
	}
}

func VisibilityDTO2DO(visibility dataset.DatasetVisibility) entity.DatasetVisibility {
	switch visibility {
	case dataset.DatasetVisibility_Public:
		return entity.DatasetVisibilityPublic
	case dataset.DatasetVisibility_Space:
		return entity.DatasetVisibilitySpace
	case dataset.DatasetVisibility_System:
		return entity.DatasetVisibilitySystem
	default:
		return entity.DatasetVisibilityUnknown
	}
}

func SpecDTO2DO(datasetSpec *dataset.DatasetSpec) *entity.DatasetSpec {
	if datasetSpec == nil {
		return nil
	}
	return &entity.DatasetSpec{
		MaxItemCount:  datasetSpec.GetMaxItemCount(),
		MaxFieldCount: datasetSpec.GetMaxFieldCount(),
		MaxItemSize:   datasetSpec.GetMaxItemSize(),
	}
}

func FeaturesDTO2DO(dto *dataset.DatasetFeatures) *entity.DatasetFeatures {
	if dto == nil {
		return nil
	}
	return &entity.DatasetFeatures{
		EditSchema:   dto.GetEditSchema(),
		RepeatedData: dto.GetRepeatedData(),
		MultiModal:   dto.GetMultiModal(),
	}
}
