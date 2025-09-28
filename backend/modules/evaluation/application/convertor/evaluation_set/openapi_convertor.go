// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"fmt"

	"github.com/bytedance/gg/gptr"

	openapi_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// convertOpenAPIContentTypeToDO 将OpenAPI的ContentType转换为Domain Entity的ContentType
func convertOpenAPIContentTypeToDO(contentType *common.ContentType) entity.ContentType {
	if contentType == nil {
		return entity.ContentTypeText // 默认值
	}
	
	switch *contentType {
	case common.ContentTypeText:
		return entity.ContentTypeText
	case common.ContentTypeImage:
		return entity.ContentTypeImage
	case common.ContentTypeAudio:
		return entity.ContentTypeAudio
	case common.ContentTypeMultiPart:
		return entity.ContentTypeMultipart
	default:
		return entity.ContentTypeText // 默认使用Text类型
	}
}

// convertDOContentTypeToOpenAPI 将Domain Entity的ContentType转换为OpenAPI的ContentType
func convertDOContentTypeToOpenAPI(contentType entity.ContentType) *common.ContentType {
	if contentType == "" {
		return nil
	}
	
	switch contentType {
	case entity.ContentTypeText:
		ct := common.ContentTypeText
		return &ct
	case entity.ContentTypeImage:
		ct := common.ContentTypeImage
		return &ct
	case entity.ContentTypeAudio:
		ct := common.ContentTypeAudio
		return &ct
	case entity.ContentTypeMultipart, entity.ContentTypeMultipartVariable:
		ct := common.ContentTypeMultiPart
		return &ct
	default:
		// 默认使用text类型
		ct := common.ContentTypeText
		return &ct
	}
}

// convertOpenAPIDisplayFormatToDO 将OpenAPI的DefaultDisplayFormat转换为Domain Entity的DefaultDisplayFormat
func convertOpenAPIDisplayFormatToDO(format *openapi_eval_set.FieldDisplayFormat) entity.FieldDisplayFormat {
	if format == nil {
		return entity.FieldDisplayFormat_PlainText // 默认值
	}
	
	switch *format {
	case openapi_eval_set.FieldDisplayFormatPlainText:
		return entity.FieldDisplayFormat_PlainText
	case openapi_eval_set.FieldDisplayFormatMarkdown:
		return entity.FieldDisplayFormat_Markdown
	case openapi_eval_set.FieldDisplayFormatJSON:
		return entity.FieldDisplayFormat_JSON
	case openapi_eval_set.FieldDisplayFormateYAML:
		return entity.FieldDisplayFormat_YAML
	case openapi_eval_set.FieldDisplayFormateCode:
		return entity.FieldDisplayFormat_Code
	default:
		return entity.FieldDisplayFormat_PlainText
	}
}

// convertDODisplayFormatToOpenAPI 将Domain Entity的DefaultDisplayFormat转换为OpenAPI的DefaultDisplayFormat
func convertDODisplayFormatToOpenAPI(format entity.FieldDisplayFormat) *openapi_eval_set.FieldDisplayFormat {
	var displayFormat *openapi_eval_set.FieldDisplayFormat
	
	switch format {
	case entity.FieldDisplayFormat_PlainText:
		f := openapi_eval_set.FieldDisplayFormatPlainText
		displayFormat = &f
	case entity.FieldDisplayFormat_Markdown:
		f := openapi_eval_set.FieldDisplayFormatMarkdown
		displayFormat = &f
	case entity.FieldDisplayFormat_JSON:
		f := openapi_eval_set.FieldDisplayFormatJSON
		displayFormat = &f
	case entity.FieldDisplayFormat_YAML:
		f := openapi_eval_set.FieldDisplayFormateYAML
		displayFormat = &f
	case entity.FieldDisplayFormat_Code:
		f := openapi_eval_set.FieldDisplayFormateCode
		displayFormat = &f
	}
	
	return displayFormat
}

// OpenAPI Schema 转换
func OpenAPISchemaDTO2DO(dto *openapi_eval_set.EvaluationSetSchema) *entity.EvaluationSetSchema {
	if dto == nil {
		return nil
	}
	return &entity.EvaluationSetSchema{
		FieldSchemas: OpenAPIFieldSchemaDTO2DOs(dto.FieldSchemas),
	}
}

func OpenAPIFieldSchemaDTO2DOs(dtos []*openapi_eval_set.FieldSchema) []*entity.FieldSchema {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.FieldSchema, 0)
	for _, dto := range dtos {
		result = append(result, OpenAPIFieldSchemaDTO2DO(dto))
	}
	return result
}

func OpenAPIFieldSchemaDTO2DO(dto *openapi_eval_set.FieldSchema) *entity.FieldSchema {
	if dto == nil {
		return nil
	}
	
	var description string
	if dto.Description != nil {
		description = *dto.Description
	}
	
	var textSchema string
	if dto.TextSchema != nil {
		textSchema = *dto.TextSchema
	}
	
	contentType := convertOpenAPIContentTypeToDO(dto.ContentType)
	
	displayFormat := convertOpenAPIDisplayFormatToDO(dto.DefaultDisplayFormat)
	
	return &entity.FieldSchema{
		Name:                 gptr.Indirect(dto.Name),
		Description:          description,
		ContentType:          contentType,
		DefaultDisplayFormat: displayFormat,
		IsRequired:           gptr.Indirect(dto.IsRequired),
		TextSchema:           textSchema,
	}
}

// OpenAPI OrderBy 转换
func ConvertOpenAPIOrderByDTO2DOs(dtos []*common.OrderBy) []*entity.OrderBy {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.OrderBy, 0)
	for _, dto := range dtos {
		result = append(result, ConvertOpenAPIOrderByDTO2DO(dto))
	}
	return result
}

func ConvertOpenAPIOrderByDTO2DO(dto *common.OrderBy) *entity.OrderBy {
	if dto == nil {
		return nil
	}
	
	return &entity.OrderBy{
		Field: dto.Field,
		IsAsc: dto.IsAsc,
	}
}

// 内部DTO转OpenAPI DTO
func EvaluationSetDO2OpenAPIDTO(do *entity.EvaluationSet) *openapi_eval_set.EvaluationSet {
	if do == nil {
		return nil
	}

	// 正确映射DatasetStatus到EvaluationSetStatus
	var status openapi_eval_set.EvaluationSetStatus
	switch do.Status {
	case entity.DatasetStatus_Available:
		status = openapi_eval_set.EvaluationSetStatusActive
	case entity.DatasetStatus_Deleted, entity.DatasetStatus_Expired:
		status = openapi_eval_set.EvaluationSetStatusArchived
	default:
		// 默认使用active状态
		status = openapi_eval_set.EvaluationSetStatusActive
	}

	return &openapi_eval_set.EvaluationSet{
		ID:                gptr.Of(do.ID),
		Name:              gptr.Of(do.Name),
		Description:       gptr.Of(do.Description),
		Status:            gptr.Of(status),
		ItemCount:         gptr.Of(do.ItemCount),
		LatestVersion:     gptr.Of(do.LatestVersion),
		ChangeUncommitted: gptr.Of(do.ChangeUncommitted),
		CurrentVersion:    EvaluationSetVersionDO2OpenAPIDTO(do.EvaluationSetVersion),
		BaseInfo:          ConvertBaseInfoDO2OpenAPIDTO(do.BaseInfo),
	}
}

func EvaluationSetDO2OpenAPIDTOs(dos []*entity.EvaluationSet) []*openapi_eval_set.EvaluationSet {
	if dos == nil {
		return nil
	}
	result := make([]*openapi_eval_set.EvaluationSet, 0)
	for _, do := range dos {
		result = append(result, EvaluationSetDO2OpenAPIDTO(do))
	}
	return result
}

func EvaluationSetVersionDO2OpenAPIDTO(do *entity.EvaluationSetVersion) *openapi_eval_set.EvaluationSetVersion {
	if do == nil {
		return nil
	}
	
	var description *string
	if do.Description != "" {
		description = &do.Description
	}
	
	return &openapi_eval_set.EvaluationSetVersion{
		ID:                   gptr.Of(do.ID),
		Version:              gptr.Of(do.Version),
		Description:          description,
		EvaluationSetSchema:  EvaluationSetSchemaDO2OpenAPIDTO(do.EvaluationSetSchema),
		ItemCount:            gptr.Of(do.ItemCount),
		BaseInfo:             ConvertBaseInfoDO2OpenAPIDTO(do.BaseInfo),
	}
}

func EvaluationSetSchemaDO2OpenAPIDTO(do *entity.EvaluationSetSchema) *openapi_eval_set.EvaluationSetSchema {
	if do == nil {
		return nil
	}
	return &openapi_eval_set.EvaluationSetSchema{
		FieldSchemas: FieldSchemaDO2OpenAPIDTOs(do.FieldSchemas),
	}
}

func FieldSchemaDO2OpenAPIDTOs(dos []*entity.FieldSchema) []*openapi_eval_set.FieldSchema {
	if dos == nil {
		return nil
	}
	result := make([]*openapi_eval_set.FieldSchema, 0)
	for _, do := range dos {
		result = append(result, FieldSchemaDO2OpenAPIDTO(do))
	}
	return result
}

func FieldSchemaDO2OpenAPIDTO(do *entity.FieldSchema) *openapi_eval_set.FieldSchema {
	if do == nil {
		return nil
	}
	
	var description *string
	if do.Description != "" {
		description = &do.Description
	}
	
	var textSchema *string
	if do.TextSchema != "" {
		textSchema = &do.TextSchema
	}
	
	displayFormat := convertDODisplayFormatToOpenAPI(do.DefaultDisplayFormat)
	
	contentType := convertDOContentTypeToOpenAPI(do.ContentType)
	
	return &openapi_eval_set.FieldSchema{
		Name:                 gptr.Of(do.Name),
		Description:          description,
		ContentType:          contentType,
		DefaultDisplayFormat: displayFormat,
		IsRequired:           gptr.Of(do.IsRequired),
		TextSchema:           textSchema,
	}
}

func ConvertBaseInfoDO2OpenAPIDTO(do *entity.BaseInfo) *common.BaseInfo {
	if do == nil {
		return nil
	}
	
	var createdAt *string
	if do.CreatedAt != nil {
		// 将时间戳转换为ISO 8601格式字符串，这里简化处理
		timestamp := fmt.Sprintf("%d", *do.CreatedAt)
		createdAt = &timestamp
	}
	
	var updatedAt *string
	if do.UpdatedAt != nil {
		timestamp := fmt.Sprintf("%d", *do.UpdatedAt)
		updatedAt = &timestamp
	}
	
	return &common.BaseInfo{
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		CreatedBy: ConvertUserInfoDO2OpenAPIDTO(do.CreatedBy),
		UpdatedBy: ConvertUserInfoDO2OpenAPIDTO(do.UpdatedBy),
	}
}

func ConvertUserInfoDO2OpenAPIDTO(do *entity.UserInfo) *common.UserInfo {
	if do == nil {
		return nil
	}
	return &common.UserInfo{
		UserID:    do.UserID,
		Name:      do.Name,
		AvatarURL: do.AvatarURL,
		Email:     do.Email,
	}
}