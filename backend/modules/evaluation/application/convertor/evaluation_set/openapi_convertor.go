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
	
	var contentType entity.ContentType
	if dto.ContentType != nil {
		// 正确映射OpenAPI DTO的ContentType枚举值到entity的ContentType枚举值
		switch *dto.ContentType {
		case common.ContentTypeText:
			contentType = entity.ContentTypeText
		case common.ContentTypeImage:
			contentType = entity.ContentTypeImage
		case common.ContentTypeAudio:
			contentType = entity.ContentTypeAudio
		case common.ContentTypeMultiPart:
			contentType = entity.ContentTypeMultipart
		default:
			// 默认使用Text类型
			contentType = entity.ContentTypeText
		}
	}
	
	var displayFormat entity.FieldDisplayFormat
	if dto.DefaultDisplayFormat != nil {
		// 简单字符串映射，实际应根据具体枚举值映射
		switch *dto.DefaultDisplayFormat {
		case "plain_text":
			displayFormat = entity.FieldDisplayFormat_PlainText
		case "markdown":
			displayFormat = entity.FieldDisplayFormat_Markdown
		case "json":
			displayFormat = entity.FieldDisplayFormat_JSON
		case "yaml":
			displayFormat = entity.FieldDisplayFormat_YAML
		case "code":
			displayFormat = entity.FieldDisplayFormat_Code
		default:
			displayFormat = entity.FieldDisplayFormat_PlainText
		}
	}
	
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
	
	var displayFormat *openapi_eval_set.FieldDisplayFormat
	switch do.DefaultDisplayFormat {
	case entity.FieldDisplayFormat_PlainText:
		format := openapi_eval_set.FieldDisplayFormatPlainText
		displayFormat = &format
	case entity.FieldDisplayFormat_Markdown:
		format := openapi_eval_set.FieldDisplayFormatMarkdown
		displayFormat = &format
	case entity.FieldDisplayFormat_JSON:
		format := openapi_eval_set.FieldDisplayFormatJSON
		displayFormat = &format
	case entity.FieldDisplayFormat_YAML:
		format := openapi_eval_set.FieldDisplayFormateYAML
		displayFormat = &format
	case entity.FieldDisplayFormat_Code:
		format := openapi_eval_set.FieldDisplayFormateCode
		displayFormat = &format
	}
	
	var contentType *common.ContentType
	if do.ContentType != "" {
		// 正确映射entity的ContentType枚举值到OpenAPI DTO的ContentType枚举值
		switch do.ContentType {
		case entity.ContentTypeText:
			ct := common.ContentTypeText
			contentType = &ct
		case entity.ContentTypeImage:
			ct := common.ContentTypeImage
			contentType = &ct
		case entity.ContentTypeAudio:
			ct := common.ContentTypeAudio
			contentType = &ct
		case entity.ContentTypeMultipart, entity.ContentTypeMultipartVariable:
			ct := common.ContentTypeMultiPart
			contentType = &ct
		default:
			// 默认使用text类型
			ct := common.ContentTypeText
			contentType = &ct
		}
	}
	
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