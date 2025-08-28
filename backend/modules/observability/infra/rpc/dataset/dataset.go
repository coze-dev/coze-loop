// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"context"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/datasetservice"
	dataset_domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/rpcerror"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type DatasetProvider struct {
	client datasetservice.Client
}

func NewDatasetProvider(client datasetservice.Client) *DatasetProvider {
	return &DatasetProvider{client: client}
}

// ValidateDatasetItems 验证数据集项
func (d *DatasetProvider) ValidateDatasetItems(ctx context.Context, ds *entity.Dataset, items []*entity.DatasetItem, ignoreCurrentCount *bool) ([]*entity.DatasetItem, []entity.ItemErrorGroup, error) {
	if len(items) == 0 {
		return []*entity.DatasetItem{}, []entity.ItemErrorGroup{}, nil
	}

	// 转换输入参数
	itemsDTO := datasetItemsDO2DTO(items)

	// 构建请求
	req := &dataset.ValidateDatasetItemsReq{
		WorkspaceID:            &ds.WorkspaceID,
		DatasetID:              &ds.ID,
		Items:                  itemsDTO,
		IgnoreCurrentItemCount: ignoreCurrentCount,
		DatasetCategory:        datasetCategoryDO2DTO(ds.DatasetCategory),
		DatasetFields:          fieldSchemasDO2DTO(ds.DatasetVersion.DatasetSchema.FieldSchemas),
	}

	// 调用RPC方法
	resp, err := d.client.ValidateDatasetItems(ctx, req)
	if err != nil {
		return nil, nil, rpcerror.UnwrapRPCError(err)
	}

	validItems := make([]*entity.DatasetItem, 0)
	for _, index := range resp.GetValidItemIndices() {
		if index >= 0 && int(index) < len(items) {
			validItems = append(validItems, items[index])
		}
	}
	errorGroups := itemErrorGroupsDTO2DO(resp.GetErrors())

	// 转换响应 - 根据有效索引提取有效项目
	for _, group := range resp.GetErrors() {
		for _, detail := range group.GetDetails() {
			if detail.Index != nil && *detail.Index >= 0 && *detail.Index < int32(len(items)) {
				item := items[detail.GetIndex()]
				item.AddError(detail.GetMessage(), int64(group.GetType()), nil)
			} else if detail.StartIndex != nil && detail.EndIndex != nil {
				for i := detail.GetStartIndex(); i <= detail.GetEndIndex(); i++ {
					if i >= 0 && int(i) < len(items) {
						item := items[i]
						item.AddError(detail.GetMessage(), int64(group.GetType()), nil)
					}
				}
			} else {
				logs.CtxError(ctx, "Validate evaluation set item return invalid detail index: %#v", detail)
				continue
			}
		}
	}
	return validItems, errorGroups, nil
}

func datasetItemsDO2DTO(items []*entity.DatasetItem) []*dataset_domain.DatasetItem {
	if len(items) == 0 {
		return nil
	}

	result := make([]*dataset_domain.DatasetItem, 0, len(items))
	for _, item := range items {
		result = append(result, datasetItemDO2DTO(item))
	}
	return result
}

func datasetItemDO2DTO(item *entity.DatasetItem) *dataset_domain.DatasetItem {
	if item == nil {
		return nil
	}

	return &dataset_domain.DatasetItem{
		ID:        &item.ID,
		SpaceID:   &item.WorkspaceID,
		DatasetID: &item.DatasetID,
		ItemKey:   item.ItemKey,
		Data:      fieldDataListDO2DTO(item.FieldData),
	}
}

func fieldDataListDO2DTO(data []*entity.FieldData) []*dataset_domain.FieldData {
	if len(data) == 0 {
		return nil
	}

	result := make([]*dataset_domain.FieldData, 0, len(data))
	for _, fieldData := range data {
		result = append(result, fieldDataDO2DTO(fieldData))
	}
	return result
}

func fieldDataDO2DTO(fieldData *entity.FieldData) *dataset_domain.FieldData {
	if fieldData == nil {
		return nil
	}
	content, err := json.Marshal(fieldData.Content)
	if err != nil {
		return nil
	}
	return &dataset_domain.FieldData{
		Key:     &fieldData.Key,
		Name:    &fieldData.Name,
		Content: gptr.Of(string(content)),
	}
}

func itemErrorGroupsDTO2DO(errors []*dataset_domain.ItemErrorGroup) []entity.ItemErrorGroup {
	if len(errors) == 0 {
		return nil
	}

	result := make([]entity.ItemErrorGroup, 0, len(errors))
	for _, errorGroup := range errors {
		result = append(result, itemErrorGroupDTO2DO(errorGroup))
	}
	return result
}

func itemErrorGroupDTO2DO(errorGroup *dataset_domain.ItemErrorGroup) entity.ItemErrorGroup {
	if errorGroup == nil {
		return entity.ItemErrorGroup{}
	}
	return entity.ItemErrorGroup{
		Type:       itemErrorTypeDTO2DO(errorGroup.Type),
		Summary:    errorGroup.GetSummary(),
		ErrorCount: errorGroup.GetErrorCount(),
		Details:    itemErrorDetailsDTO2DO(errorGroup.Details),
	}
}

func itemErrorTypeDTO2DO(errorType *dataset_domain.ItemErrorType) int64 {
	if errorType == nil {
		return entity.DatasetErrorType_InternalError
	}
	return int64(*errorType)
}

func itemErrorDetailsDTO2DO(details []*dataset_domain.ItemErrorDetail) []*entity.ItemErrorDetail {
	if len(details) == 0 {
		return nil
	}

	result := make([]*entity.ItemErrorDetail, 0, len(details))
	for _, detail := range details {
		result = append(result, itemErrorDetailDTO2DO(detail))
	}
	return result
}

func itemErrorDetailDTO2DO(detail *dataset_domain.ItemErrorDetail) *entity.ItemErrorDetail {
	if detail == nil {
		return nil
	}

	return &entity.ItemErrorDetail{
		Message:    detail.GetMessage(),
		Index:      detail.Index,
		StartIndex: detail.StartIndex,
		EndIndex:   detail.EndIndex,
	}
}

func fieldDisplayFormatDO2DTO(df entity.FieldDisplayFormat) dataset_domain.FieldDisplayFormat {
	switch df {
	case entity.FieldDisplayFormat_PlainText:
		return dataset_domain.FieldDisplayFormat_PlainText
	case entity.FieldDisplayFormat_Markdown:
		return dataset_domain.FieldDisplayFormat_Markdown
	case entity.FieldDisplayFormat_JSON:
		return dataset_domain.FieldDisplayFormat_JSON
	case entity.FieldDisplayFormat_YAML:
		return dataset_domain.FieldDisplayFormat_YAML
	case entity.FieldDisplayFormat_Code:
		return dataset_domain.FieldDisplayFormat_Code
	default:
		return dataset_domain.FieldDisplayFormat_PlainText
	}
}

func datasetCategoryDO2DTO(category entity.DatasetCategory) *dataset_domain.DatasetCategory {
	switch category {
	case entity.DatasetCategory_Evaluation:
		return dataset_domain.DatasetCategoryPtr(dataset_domain.DatasetCategory_Evaluation)
	case entity.DatasetCategory_General:
		return dataset_domain.DatasetCategoryPtr(dataset_domain.DatasetCategory_General)
	default:
		return dataset_domain.DatasetCategoryPtr(dataset_domain.DatasetCategory_Evaluation)
	}
}

func fieldSchemasDO2DTO(schemas []entity.FieldSchema) []*dataset_domain.FieldSchema {
	if len(schemas) == 0 {
		return nil
	}

	result := make([]*dataset_domain.FieldSchema, 0, len(schemas))
	for _, schema := range schemas {
		result = append(result, fieldSchemaDO2DTO(&schema))
	}
	return result
}

func fieldSchemaDO2DTO(schema *entity.FieldSchema) *dataset_domain.FieldSchema {
	if schema == nil {
		return nil
	}

	return &dataset_domain.FieldSchema{
		Key:           schema.Key,
		Name:          &schema.Name,
		Description:   &schema.Description,
		ContentType:   ContentTypeDO2DTO(schema.ContentType),
		DefaultFormat: ptr.Of(fieldDisplayFormatDO2DTO(schema.DisplayFormat)),
	}
}

func ContentTypeDO2DTO(contentType entity.ContentType) *dataset_domain.ContentType {
	switch contentType {
	case entity.ContentType_Text:
		return dataset_domain.ContentTypePtr(dataset_domain.ContentType_Text)
	case entity.ContentType_Image:
		return dataset_domain.ContentTypePtr(dataset_domain.ContentType_Image)
	case entity.ContentType_Audio:
		return dataset_domain.ContentTypePtr(dataset_domain.ContentType_Audio)
	case entity.ContentType_MultiPart:
		return dataset_domain.ContentTypePtr(dataset_domain.ContentType_MultiPart)
	default:
		return dataset_domain.ContentTypePtr(dataset_domain.ContentType_Text)
	}
}
