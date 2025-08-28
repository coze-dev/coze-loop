// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package evaluationset

import (
	"context"
	"strconv"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	dataset_domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	eval_set_domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluationsetservice"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/rpcerror"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
)

type EvaluationSetProvider struct {
	client          evaluationsetservice.Client
	datasetProvider *dataset.DatasetProvider
}

var _ rpc.IDatasetProvider = (*EvaluationSetProvider)(nil)

func NewEvaluationSetProvider(client evaluationsetservice.Client, datasetProvider *dataset.DatasetProvider) *EvaluationSetProvider {
	return &EvaluationSetProvider{client: client, datasetProvider: datasetProvider}
}

// CreateDataset 创建数据集
func (d *EvaluationSetProvider) CreateDataset(ctx context.Context, dataset *entity.Dataset) (int64, error) {
	if dataset.WorkspaceID == 0 {
		return 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("workspace ID is required"))
	}
	if dataset.Name == "" {
		return 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset name is required"))
	}

	userIDStr, _ := session.UserIDInCtx(ctx)
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		return 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("userid is required"))
	}
	// 构造请求
	req := &eval_set.CreateEvaluationSetRequest{
		WorkspaceID: dataset.WorkspaceID,
		Name:        &dataset.Name,
		Description: &dataset.Description,
		Session: &common.Session{
			UserID: lo.ToPtr(userID),
		},
	}

	// 设置BizCategory
	if dataset.EvaluationBizCategory != nil {
		bizCategory := eval_set_domain.BizCategory(*dataset.EvaluationBizCategory)
		req.BizCategory = &bizCategory
	}

	// 转换DatasetSchema
	if len(dataset.DatasetVersion.DatasetSchema.FieldSchemas) > 0 {
		req.EvaluationSetSchema = datasetSchemaDO2DTO(&dataset.DatasetVersion.DatasetSchema)
	}

	resp, err := d.client.CreateEvaluationSet(ctx, req)
	if err != nil {
		logs.CtxError(ctx, "CreateEvaluationSet failed, workspace_id=%d, err=%#v", dataset.WorkspaceID, err)
		return 0, rpcerror.UnwrapRPCError(err)
	}

	datasetID := resp.GetEvaluationSetID()
	logs.CtxInfo(ctx, "CreateDataset success, workspace_id=%d, dataset_id=%d", dataset.WorkspaceID, datasetID)
	return datasetID, nil
}

// UpdateDatasetSchema 更新数据集模式
func (d *EvaluationSetProvider) UpdateDatasetSchema(ctx context.Context, dataset *entity.Dataset) error {
	if dataset.WorkspaceID == 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("workspace ID is required"))
	}
	if dataset.ID == 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset ID is required"))
	}

	// 转换FieldSchemas
	fieldSchemas := make([]*eval_set_domain.FieldSchema, 0, len(dataset.DatasetVersion.DatasetSchema.FieldSchemas))
	for _, fs := range dataset.DatasetVersion.DatasetSchema.FieldSchemas {
		fieldSchemas = append(fieldSchemas, fieldSchemaDO2DTO(fs))
	}

	req := &eval_set.UpdateEvaluationSetSchemaRequest{
		WorkspaceID:     dataset.WorkspaceID,
		EvaluationSetID: dataset.ID,
		Fields:          fieldSchemas,
	}

	_, err := d.client.UpdateEvaluationSetSchema(ctx, req)
	if err != nil {
		logs.CtxError(ctx, "UpdateEvaluationSetSchema failed, workspace_id=%d, dataset_id=%d, err=%#v", dataset.WorkspaceID, dataset.ID, err)
		return rpcerror.UnwrapRPCError(err)
	}

	logs.CtxInfo(ctx, "UpdateDatasetSchema success, workspace_id=%d, dataset_id=%d", dataset.WorkspaceID, dataset.ID)
	return nil
}

// GetDataset 获取数据集
func (d *EvaluationSetProvider) GetDataset(ctx context.Context, workspaceID, datasetID int64, category entity.DatasetCategory) (*entity.Dataset, error) {
	if workspaceID == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("workspace ID is required"))
	}
	if datasetID == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset ID is required"))
	}

	req := &eval_set.GetEvaluationSetRequest{
		WorkspaceID:     workspaceID,
		EvaluationSetID: datasetID,
	}

	resp, err := d.client.GetEvaluationSet(ctx, req)
	if err != nil {
		logs.CtxError(ctx, "GetEvaluationSet failed, workspace_id=%d, dataset_id=%d, err=%v", workspaceID, datasetID, err)
		return nil, rpcerror.UnwrapRPCError(err)
	}

	dataset := evaluationSetDTO2DO(resp.EvaluationSet)
	logs.CtxInfo(ctx, "GetDataset success, workspace_id=%d, dataset_id=%d", workspaceID, datasetID)
	return dataset, nil
}

// ClearDatasetItems 清空数据集项
func (d *EvaluationSetProvider) ClearDatasetItems(ctx context.Context, workspaceID, datasetID int64, category entity.DatasetCategory) error {
	if workspaceID == 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("workspace ID is required"))
	}
	if datasetID == 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset ID is required"))
	}

	req := &eval_set.ClearEvaluationSetDraftItemRequest{
		WorkspaceID:     workspaceID,
		EvaluationSetID: datasetID,
	}

	_, err := d.client.ClearEvaluationSetDraftItem(ctx, req)
	if err != nil {
		logs.CtxError(ctx, "ClearEvaluationSetDraftItem failed, workspace_id=%d, dataset_id=%d, err=%v", workspaceID, datasetID, err)
		return rpcerror.UnwrapRPCError(err)
	}

	logs.CtxInfo(ctx, "ClearDatasetItems success, workspace_id=%d, dataset_id=%d", workspaceID, datasetID)
	return nil
}

// AddDatasetItems 添加数据集项
func (d *EvaluationSetProvider) AddDatasetItems(ctx context.Context, datasetID int64, category entity.DatasetCategory, items []*entity.DatasetItem) ([]*entity.DatasetItem, []entity.ItemErrorGroup, error) {
	if len(items) == 0 {
		return []*entity.DatasetItem{}, []entity.ItemErrorGroup{}, nil
	}

	// 验证所有items属于同一个workspace和dataset
	workspaceID := items[0].WorkspaceID
	for _, item := range items {
		if item.WorkspaceID != workspaceID || item.DatasetID != datasetID {
			return nil, nil, errorx.NewByCode(errno.CommonInvalidParamCode,
				errorx.WithExtraMsg("all items must belong to the same workspace and dataset"))
		}
	}

	successItems := make([]*entity.DatasetItem, 0, len(items))
	errorGroups := make([]entity.ItemErrorGroup, 0)
	errorGroupsMap := make(map[int64]entity.ItemErrorGroup)

	const batchSize = 100
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batchItems := items[i:end]

		logs.CtxInfo(ctx, "Processing batch %d-%d of %d items", i, end-1, len(items))

		// 转换为EvaluationSetItem
		evalSetItems := datasetItemsDO2DTO(batchItems)

		req := &eval_set.BatchCreateEvaluationSetItemsRequest{
			WorkspaceID:      workspaceID,
			EvaluationSetID:  datasetID,
			Items:            evalSetItems,
			SkipInvalidItems: lo.ToPtr(true),
			AllowPartialAdd:  lo.ToPtr(true),
		}

		resp, err := d.client.BatchCreateEvaluationSetItems(ctx, req)
		if err != nil {
			logs.CtxError(ctx, "BatchCreateEvaluationSetItems failed, workspace_id=%d, dataset_id=%d, batch=%d-%d, err=%v", workspaceID, datasetID, i, end-1, err)
			return successItems, errorGroups, rpcerror.UnwrapRPCError(err)
		}

		// 处理成功的items
		for batchSpecificIndex, itemID := range resp.GetAddedItems() {
			originalIndex := i + int(batchSpecificIndex)
			if originalIndex < len(items) {
				items[originalIndex].ID = itemID
				successItems = append(successItems, items[originalIndex])
			}
		}

		// 处理错误
		for _, group := range resp.GetErrors() {
			for _, detail := range group.GetDetails() {
				originalMessage := detail.GetMessage()
				originalErrorType := int64(group.GetType())
				if detail.Index != nil {
					batchSpecificIndex := int(detail.GetIndex())
					originalIndex := i + batchSpecificIndex
					if originalIndex >= 0 && originalIndex < len(items) && batchSpecificIndex >= 0 && batchSpecificIndex < len(batchItems) {
						items[originalIndex].AddError(originalMessage, originalErrorType, nil)
					} else {
						logs.CtxError(ctx, "Error index out of bounds when adding error. OriginalIndex: %d, BatchSpecificIndex: %d", originalIndex, batchSpecificIndex)
					}
				} else if detail.StartIndex != nil && detail.EndIndex != nil {
					startBatchIndex := int(detail.GetStartIndex())
					endBatchIndex := int(detail.GetEndIndex())
					for k := startBatchIndex; k <= endBatchIndex; k++ {
						originalIndex := i + k
						if originalIndex >= 0 && originalIndex < len(items) && k >= 0 && k < len(batchItems) {
							items[originalIndex].AddError(originalMessage, originalErrorType, nil)
						} else {
							logs.CtxError(ctx, "Error index out of bounds in range when adding error. OriginalIndex: %d, BatchSpecificIndex_k: %d", originalIndex, k)
						}
					}
				}
			}

			if group.Type == nil || group.ErrorCount == nil {
				logs.CtxError(ctx, "Invalid errorGroup: %#v", group)
				continue
			}
			errorType := int64(*group.Type)
			if errorGroup, ok := errorGroupsMap[errorType]; !ok {
				errorGroupsMap[errorType] = entity.ItemErrorGroup{
					Type:       errorType,
					Summary:    group.GetSummary(),
					ErrorCount: *group.ErrorCount,
				}
			} else {
				errorGroup.ErrorCount += *group.ErrorCount
				errorGroupsMap[errorType] = errorGroup
			}
		}
	}

	errorGroups = lo.MapToSlice(errorGroupsMap, func(key int64, value entity.ItemErrorGroup) entity.ItemErrorGroup {
		return value
	})

	logs.CtxInfo(ctx, "AddDatasetItems completed, success_count=%d, error_groups=%d", len(successItems), len(errorGroups))
	return successItems, errorGroups, nil
}

// ValidateDatasetItems 验证数据集项
func (d *EvaluationSetProvider) ValidateDatasetItems(ctx context.Context, ds *entity.Dataset, items []*entity.DatasetItem, ignoreCurrentCount *bool) ([]*entity.DatasetItem, []entity.ItemErrorGroup, error) {
	return d.datasetProvider.ValidateDatasetItems(ctx, ds, items, ignoreCurrentCount)
}

// datasetSchemaDO2DTO 转换DatasetSchema到EvaluationSetSchema
func datasetSchemaDO2DTO(schema *entity.DatasetSchema) *eval_set_domain.EvaluationSetSchema {
	if schema == nil {
		return nil
	}

	evalSetSchema := &eval_set_domain.EvaluationSetSchema{
		ID:              &schema.ID,
		WorkspaceID:     &schema.WorkspaceID,
		EvaluationSetID: &schema.DatasetID,
	}

	if len(schema.FieldSchemas) > 0 {
		fieldSchemas := make([]*eval_set_domain.FieldSchema, 0, len(schema.FieldSchemas))
		for _, fs := range schema.FieldSchemas {
			fieldSchemas = append(fieldSchemas, fieldSchemaDO2DTO(fs))
		}
		evalSetSchema.FieldSchemas = fieldSchemas
	}

	return evalSetSchema
}

// fieldSchemaDO2DTO 转换FieldSchema
func fieldSchemaDO2DTO(fs entity.FieldSchema) *eval_set_domain.FieldSchema {
	contentType := common.ContentType(fs.ContentType)
	defaultDisplayFormat := gptr.Of(FieldDisplayFormatDO2DTO(fs.DisplayFormat))
	return &eval_set_domain.FieldSchema{
		Key:                  fs.Key,
		Name:                 &fs.Name,
		Description:          &fs.Description,
		ContentType:          &contentType,
		TextSchema:           &fs.TextSchema,
		DefaultDisplayFormat: defaultDisplayFormat,
	}
}

// evaluationSetDTO2DO 转换EvaluationSet到Dataset
func evaluationSetDTO2DO(evalSet *eval_set_domain.EvaluationSet) *entity.Dataset {
	if evalSet == nil {
		return nil
	}

	dataset := &entity.Dataset{
		ID:              evalSet.GetID(),
		WorkspaceID:     evalSet.GetWorkspaceID(),
		Name:            evalSet.GetName(),
		Description:     evalSet.GetDescription(),
		DatasetCategory: entity.DatasetCategory_Evaluation,
	}
	if evalSet.IsSetBizCategory() {
		bizCategory := entity.EvaluationBizCategory(evalSet.GetBizCategory())
		dataset.EvaluationBizCategory = &bizCategory
	}

	// 转换DatasetVersion
	if evalSet.EvaluationSetVersion != nil {
		dataset.DatasetVersion = entity.DatasetVersion{
			ID:          evalSet.EvaluationSetVersion.GetID(),
			WorkspaceID: evalSet.GetWorkspaceID(),
			DatasetID:   evalSet.GetID(),
			Version:     evalSet.EvaluationSetVersion.GetVersion(),
			Description: evalSet.EvaluationSetVersion.GetDescription(),
		}

		// 转换DatasetSchema
		if evalSet.EvaluationSetVersion.EvaluationSetSchema != nil {
			dataset.DatasetVersion.DatasetSchema = evaluationSetSchemaDTO2DO(evalSet.EvaluationSetVersion.EvaluationSetSchema)
		}
	}

	return dataset
}

// evaluationSetSchemaDTO2DO 转换EvaluationSetSchema到DatasetSchema
func evaluationSetSchemaDTO2DO(evalSetSchema *eval_set_domain.EvaluationSetSchema) entity.DatasetSchema {
	schema := entity.DatasetSchema{
		ID:          evalSetSchema.GetID(),
		WorkspaceID: evalSetSchema.GetWorkspaceID(),
		DatasetID:   evalSetSchema.GetEvaluationSetID(),
	}

	if len(evalSetSchema.FieldSchemas) > 0 {
		fieldSchemas := make([]entity.FieldSchema, 0, len(evalSetSchema.FieldSchemas))
		for _, fs := range evalSetSchema.FieldSchemas {
			fieldSchemas = append(fieldSchemas, fieldSchemaDTO2DO(fs))
		}
		schema.FieldSchemas = fieldSchemas
	}

	return schema
}

// fieldSchemaDTO2DO 转换FieldSchema
func fieldSchemaDTO2DO(fs *eval_set_domain.FieldSchema) entity.FieldSchema {
	fieldSchema := entity.FieldSchema{
		Key:         fs.Key,
		Name:        fs.GetName(),
		Description: fs.GetDescription(),
		TextSchema:  fs.GetTextSchema(),
	}

	if fs.ContentType != nil {
		fieldSchema.ContentType = entity.ContentType(*fs.ContentType)
	}

	return fieldSchema
}

func convertContentDO2DTO(content *entity.Content) *common.Content {
	var result *common.Content
	if content == nil {
		return result
	}
	var multiPart []*common.Content
	if content.MultiPart != nil {
		for _, part := range content.MultiPart {
			multiPart = append(multiPart, convertContentDO2DTO(part))
		}
	}
	result = &common.Content{
		ContentType: entity.CommonContentTypeDO2DTO(content.GetContentType()),
		Text:        gptr.Of(content.GetText()),
		Image: &common.Image{
			Name: gptr.Of(content.GetImage().GetName()),
			URL:  gptr.Of(content.GetImage().GetUrl()),
		},
		MultiPart: multiPart,
	}
	return result
}

// datasetItemsDO2DTO 转换DatasetItem到EvaluationSetItem
func datasetItemsDO2DTO(items []*entity.DatasetItem) []*eval_set_domain.EvaluationSetItem {
	evalSetItems := make([]*eval_set_domain.EvaluationSetItem, 0, len(items))

	for _, item := range items {
		if item == nil {
			continue
		}

		evalSetItem := &eval_set_domain.EvaluationSetItem{
			WorkspaceID:     &item.WorkspaceID,
			EvaluationSetID: &item.DatasetID,
			ItemKey:         item.ItemKey,
		}

		// 转换FieldData到Turns，增加空值检查
		if len(item.FieldData) > 0 {
			fieldDataList := make([]*eval_set_domain.FieldData, 0, len(item.FieldData))
			for _, fd := range item.FieldData {
				if fd != nil && fd.Key != "" && fd.Content != nil {
					fieldDataList = append(fieldDataList, &eval_set_domain.FieldData{
						Key:     &fd.Key,
						Name:    &fd.Name,
						Content: convertContentDO2DTO(fd.Content),
					})
				}
			}

			if len(fieldDataList) > 0 {
				evalSetItem.Turns = []*eval_set_domain.Turn{
					{
						FieldDataList: fieldDataList,
					},
				}
			}
		}

		evalSetItems = append(evalSetItems, evalSetItem)
	}

	return evalSetItems
}

func FieldDisplayFormatDO2DTO(df entity.FieldDisplayFormat) dataset_domain.FieldDisplayFormat {
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
