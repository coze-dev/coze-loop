// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	eval_common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	dataset0 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
)

// ExportRequestDTO2DO 将导出请求从 DTO 转换为 DO
func ExportRequestDTO2DO(req *trace.ExportTracesToDatasetRequest) *service.ExportTracesToDatasetRequest {
	if req == nil {
		return nil
	}

	result := &service.ExportTracesToDatasetRequest{
		WorkspaceID: req.GetWorkspaceID(),
		SpanIds:     convertSpanIdsDTO2DO(req.GetSpanIds()),
		Category:    convertDatasetCategoryDTO2DO(req.GetCategory()),
		Config:      convertDatasetConfigDTO2DO(req.GetConfig()),
	}

	result.StartTime = req.GetStartTime()
	result.EndTime = req.GetEndTime()

	if req.IsSetPlatformType() {
		result.PlatformType = loop_span.PlatformType(req.GetPlatformType())
	} else {
		result.PlatformType = loop_span.PlatformCozeLoop
	}

	// 转换导出类型
	switch req.GetExportType() {
	case dataset0.ExportTypeAppend:
		result.ExportType = service.ExportType_Append
	case dataset0.ExportTypeOverwrite:
		result.ExportType = service.ExportType_Overwrite
	default:
		result.ExportType = service.ExportType_Append
	}

	// 转换字段映射
	if req.IsSetFieldMappings() {
		result.FieldMappings = convertFieldMappingsDTO2DO(req.GetFieldMappings())
	}

	return result
}

// ExportResponseDO2DTO 将导出响应从 DO 转换为 DTO
func ExportResponseDO2DTO(resp *service.ExportTracesToDatasetResponse) *trace.ExportTracesToDatasetResponse {
	if resp == nil {
		return nil
	}

	result := trace.NewExportTracesToDatasetResponse()
	result.SuccessCount = &resp.SuccessCount
	result.DatasetID = &resp.DatasetID
	result.DatasetName = &resp.DatasetName

	// 转换错误信息
	if len(resp.Errors) > 0 {
		result.Errors = convertItemErrorGroupsDO2DTO(resp.Errors)
	}

	return result
}

// PreviewRequestDTO2DO 将预览请求从 DTO 转换为 DO
func PreviewRequestDTO2DO(req *trace.PreviewExportTracesToDatasetRequest) *service.ExportTracesToDatasetRequest {
	if req == nil {
		return nil
	}

	result := &service.ExportTracesToDatasetRequest{
		WorkspaceID: req.GetWorkspaceID(),
		SpanIds:     convertSpanIdsDTO2DO(req.GetSpanIds()),
		Category:    convertDatasetCategoryDTO2DO(req.GetCategory()),
		Config:      convertDatasetConfigDTO2DO(req.GetConfig()),
	}
	result.StartTime = req.GetStartTime()
	result.EndTime = req.GetEndTime()

	if req.IsSetPlatformType() {
		result.PlatformType = loop_span.PlatformType(req.GetPlatformType())
	} else {
		result.PlatformType = loop_span.PlatformCozeLoop
	}

	// 转换导出类型
	switch req.GetExportType() {
	case dataset0.ExportTypeAppend:
		result.ExportType = service.ExportType_Append
	case dataset0.ExportTypeOverwrite:
		result.ExportType = service.ExportType_Overwrite
	default:
		result.ExportType = service.ExportType_Append
	}

	// 转换字段映射
	if req.IsSetFieldMappings() {
		result.FieldMappings = convertFieldMappingsDTO2DO(req.GetFieldMappings())
	}

	return result
}

// PreviewResponseDO2DTO 将预览响应从 DO 转换为 DTO
func PreviewResponseDO2DTO(resp *service.PreviewExportTracesToDatasetResponse) *trace.PreviewExportTracesToDatasetResponse {
	if resp == nil {
		return nil
	}

	result := trace.NewPreviewExportTracesToDatasetResponse()

	// 转换数据项
	if len(resp.Items) > 0 {
		result.Items = convertDatasetItemsDO2DTO(resp.Items)
	}

	// 转换错误信息
	if len(resp.Errors) > 0 {
		result.Errors = convertItemErrorGroupsDO2DTO(resp.Errors)
	}

	return result
}

// convertDatasetConfigDTO2DO 转换数据集配置
func convertDatasetConfigDTO2DO(config *trace.DatasetConfig) service.DatasetConfig {
	if config == nil {
		return service.DatasetConfig{}
	}

	result := service.DatasetConfig{
		IsNewDataset: config.GetIsNewDataset(),
	}

	if config.IsSetDatasetID() {
		result.DatasetID = config.DatasetID
	}
	if config.IsSetDatasetName() {
		result.DatasetName = config.DatasetName
	}
	if config.IsSetDatasetSchema() {
		result.DatasetSchema = convertDatasetSchemaDTO2DO(config.GetDatasetSchema())
	}

	return result
}

// convertDatasetSchemaDTO2DO 转换数据集模式
func convertDatasetSchemaDTO2DO(schema *dataset0.DatasetSchema) entity.DatasetSchema {
	if schema == nil {
		return entity.DatasetSchema{}
	}

	result := entity.DatasetSchema{}

	if schema.IsSetFieldSchemas() {
		fieldSchemas := schema.GetFieldSchemas()
		result.FieldSchemas = make([]entity.FieldSchema, len(fieldSchemas))
		for i, fs := range fieldSchemas {
			key := fs.GetKey()
			name := fs.GetName()
			description := fs.GetDescription()
			textSchema := fs.GetTextSchema()
			result.FieldSchemas[i] = entity.FieldSchema{
				Key:         &key,
				Name:        name,
				Description: description,
				ContentType: convertContentTypeDTO2DO(fs.GetContentType()),
				TextSchema:  textSchema,
			}
		}
	}

	return result
}

// convertFieldMappingsDTO2DO 转换字段映射
func convertFieldMappingsDTO2DO(mappings []*dataset0.FieldMapping) []entity.FieldMapping {
	if len(mappings) == 0 {
		return nil
	}

	result := make([]entity.FieldMapping, len(mappings))
	for i, mapping := range mappings {
		result[i] = entity.FieldMapping{
			FieldSchema: entity.FieldSchema{
				Key:         mapping.GetFieldSchema().Key,
				Name:        mapping.GetFieldSchema().GetName(),
				Description: mapping.GetFieldSchema().GetDescription(),
				ContentType: convertContentTypeDTO2DO(mapping.GetFieldSchema().GetContentType()),
				TextSchema:  mapping.GetFieldSchema().GetTextSchema(),
			},
			TraceFieldKey:      mapping.GetTraceFieldKey(),
			TraceFieldJsonpath: mapping.GetTraceFieldJsonpath(),
		}
	}

	return result
}

// convertItemErrorGroupsDO2DTO 转换错误组
func convertItemErrorGroupsDO2DTO(errors []entity.ItemErrorGroup) []*dataset.ItemErrorGroup {
	if len(errors) == 0 {
		return nil
	}

	result := make([]*dataset.ItemErrorGroup, len(errors))
	for i, err := range errors {
		errorType := dataset.ItemErrorType(err.Type)
		result[i] = &dataset.ItemErrorGroup{
			Type:       &errorType,
			Summary:    &err.Summary,
			ErrorCount: &err.ErrorCount,
		}

		if len(err.Details) > 0 {
			details := make([]*dataset.ItemErrorDetail, len(err.Details))
			for j, detail := range err.Details {
				details[j] = &dataset.ItemErrorDetail{
					Message: &detail.Message,
				}
				if detail.Index != nil {
					details[j].Index = detail.Index
				}
				if detail.StartIndex != nil {
					details[j].StartIndex = detail.StartIndex
				}
				if detail.EndIndex != nil {
					details[j].EndIndex = detail.EndIndex
				}
			}
			result[i].Details = details
		}
	}

	return result
}

func convertContentDO2DTO(content *entity.Content) *dataset0.Content {
	var result *dataset0.Content
	if content == nil {
		return result
	}
	var multiPart []*dataset0.Content
	if content.MultiPart != nil {
		for _, part := range content.MultiPart {
			multiPart = append(multiPart, convertContentDO2DTO(part))
		}
	}
	result = &dataset0.Content{
		ContentType: entity.CommonContentTypeDO2DTO(content.GetContentType()),
		Text:        gptr.Of(content.GetText()),
		Image: &dataset0.Image{
			Name: gptr.Of(content.GetImage().GetName()),
			URL:  gptr.Of(content.GetImage().GetUrl()),
		},
		MultiPart: multiPart,
	}
	return result
}

func convertFieldListDO2DTO(fieldList []*entity.FieldData) []*dataset0.FieldData {
	result := make([]*dataset0.FieldData, len(fieldList))
	for i, field := range fieldList {
		result[i] = &dataset0.FieldData{
			Key:     gptr.Of(field.Key),
			Name:    gptr.Of(field.Name),
			Content: convertContentDO2DTO(field.Content),
		}
	}
	return result
}

// convertDatasetItemsDO2DTO 转换数据集项
func convertDatasetItemsDO2DTO(items []*entity.DatasetItem) []*dataset0.Item {
	if len(items) == 0 {
		return nil
	}

	result := make([]*dataset0.Item, len(items))
	for i, item := range items {
		result[i] = &dataset0.Item{
			Status: dataset0.ItemStatusSuccess,
		}

		// 转换字段数据为 map
		if len(item.FieldData) > 0 {
			result[i].FieldList = convertFieldListDO2DTO(item.FieldData)
		}

		// 转换错误信息
		if len(item.Error) > 0 {
			result[i].Status = dataset0.ItemStatusError
			errors := make([]*dataset0.ItemError, len(item.Error))
			for j, err := range item.Error {
				errorType := dataset.ItemErrorType(err.Type)
				errors[j] = &dataset0.ItemError{
					Type:       &errorType,
					FieldNames: err.FieldNames,
				}
			}
			result[i].Errors = errors
		}
	}

	return result
}

// convertDatasetCategoryDTO2DO 转换数据集分类
func convertDatasetCategoryDTO2DO(category dataset.DatasetCategory) entity.DatasetCategory {
	switch category {
	case dataset.DatasetCategory_General:
		return entity.DatasetCategory_General
	case dataset.DatasetCategory_Evaluation:
		return entity.DatasetCategory_Evaluation
	default:
		return entity.DatasetCategory_General
	}
}

// convertSpanIdsDTO2DO 将DTO中的Span ID字符串转换为DO中的SpanID结构体
func convertSpanIdsDTO2DO(spanIDs []*trace.SpanID) []service.SpanID {
	if spanIDs == nil {
		return nil
	}
	result := make([]service.SpanID, 0, len(spanIDs))
	for _, spanID := range spanIDs {
		result = append(result, service.SpanID{
			TraceID: spanID.GetTraceID(),
			SpanID:  spanID.GetSpanID(),
		})
	}
	return result
}

// convertContentTypeDTO2DO 转换内容类型
func convertContentTypeDTO2DO(contentType eval_common.ContentType) entity.ContentType {
	switch contentType {
	case eval_common.ContentTypeText:
		return entity.ContentType_Text
	case eval_common.ContentTypeImage:
		return entity.ContentType_Image
	case eval_common.ContentTypeAudio:
		return entity.ContentType_Audio
	case eval_common.ContentTypeMultiPart:
		return entity.ContentType_MultiPart
	default:
		return entity.ContentType_Text
	}
}
