// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"context"
	"fmt"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset/datasetservice"
	domain_dataset "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type DatasetRPCAdapter struct {
	client datasetservice.Client
}

func NewDatasetRPCAdapter(client datasetservice.Client) rpc.IDatasetRPCAdapter {
	return &DatasetRPCAdapter{
		client: client,
	}
}

func (a *DatasetRPCAdapter) CreateDataset(ctx context.Context, param *rpc.CreateDatasetParam) (id int64, err error) {
	fields := make([]*domain_dataset.FieldSchema, 0)
	if param.EvaluationSetItems != nil {
		fields, err = convert2DatasetFieldSchemas(ctx, param.EvaluationSetItems.FieldSchemas)
		if err != nil {
			return 0, err
		}
	}
	resp, err := a.client.CreateDataset(ctx, &dataset.CreateDatasetRequest{
		WorkspaceID: param.SpaceID,
		Name:        param.Name,
		AppID:       &param.EvaluationSetItems.AppID,
		Description: param.Desc,
		Category:    domain_dataset.DatasetCategoryPtr(domain_dataset.DatasetCategory_Evaluation),
		BizCategory: param.BizCategory,
		Visibility:  domain_dataset.DatasetVisibilityPtr(domain_dataset.DatasetVisibility_Space),
		Fields:      fields,
		Features: &domain_dataset.DatasetFeatures{
			EditSchema: gptr.Of(true),
		},
	})
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return 0, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return resp.GetDatasetID(), nil
}

func (a *DatasetRPCAdapter) UpdateDataset(ctx context.Context, spaceID int64, evaluationSetID int64, name *string, desc *string) (err error) {
	resp, err := a.client.UpdateDataset(ctx, &dataset.UpdateDatasetRequest{
		WorkspaceID: &spaceID,
		DatasetID:   evaluationSetID,
		Name:        name,
		Description: desc,
	})
	if err != nil {
		return err
	}
	if resp == nil {
		return errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return nil
}

func (a *DatasetRPCAdapter) DeleteDataset(ctx context.Context, spaceID int64, evaluationSetID int64) (err error) {
	resp, err := a.client.DeleteDataset(ctx, &dataset.DeleteDatasetRequest{
		WorkspaceID: &spaceID,
		DatasetID:   evaluationSetID,
	})
	if err != nil {
		return err
	}
	if resp == nil {
		return errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return nil
}

func (a *DatasetRPCAdapter) GetDataset(ctx context.Context, spaceID *int64, evaluationSetID int64, deletedAt *bool) (set *entity.EvaluationSet, err error) {
	resp, err := a.client.GetDataset(ctx, &dataset.GetDatasetRequest{
		WorkspaceID: spaceID,
		DatasetID:   evaluationSetID,
		WithDeleted: deletedAt,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return convert2EvaluationSet(ctx, resp.Dataset), nil
}

func (a *DatasetRPCAdapter) BatchGetDatasets(ctx context.Context, spaceID *int64, evaluationSetID []int64, deletedAt *bool) (sets []*entity.EvaluationSet, err error) {
	resp, err := a.client.BatchGetDatasets(ctx, &dataset.BatchGetDatasetsRequest{
		WorkspaceID: gptr.Indirect(spaceID),
		DatasetIds:  evaluationSetID,
		WithDeleted: deletedAt,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return convert2EvaluationSets(ctx, resp.Datasets), nil
}

func (a *DatasetRPCAdapter) ListDatasets(ctx context.Context, param *rpc.ListDatasetsParam) (sets []*entity.EvaluationSet, total *int64, nextPageToken *string, err error) {
	resp, err := a.client.ListDatasets(ctx, &dataset.ListDatasetsRequest{
		WorkspaceID: param.SpaceID,
		DatasetIds:  param.EvaluationSetIDs,
		Name:        param.Name,
		CreatedBys:  param.Creators,
		PageNumber:  param.PageNumber,
		PageSize:    param.PageSize,
		PageToken:   param.PageToken,
		OrderBys:    convert2DatasetOrderBys(ctx, param.OrderBys),
		Category:    domain_dataset.DatasetCategoryPtr(domain_dataset.DatasetCategory_Evaluation),
	})
	if err != nil {
		return nil, nil, nil, err
	}
	if resp == nil {
		return nil, nil, nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, nil, nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return convert2EvaluationSets(ctx, resp.Datasets), resp.Total, resp.NextPageToken, nil
}

func (a *DatasetRPCAdapter) CreateDatasetVersion(ctx context.Context, spaceID int64, evaluationSetID int64, version string, desc *string) (id int64, err error) {
	resp, err := a.client.CreateDatasetVersion(ctx, &dataset.CreateDatasetVersionRequest{
		WorkspaceID: &spaceID,
		DatasetID:   evaluationSetID,
		Version:     version,
		Desc:        desc,
	})
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return 0, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return resp.GetID(), nil
}

func (a *DatasetRPCAdapter) GetDatasetVersion(ctx context.Context, spaceID int64, versionID int64, deletedAt *bool) (version *entity.EvaluationSetVersion, set *entity.EvaluationSet, err error) {
	resp, err := a.client.GetDatasetVersion(ctx, &dataset.GetDatasetVersionRequest{
		WorkspaceID: &spaceID,
		VersionID:   versionID,
		WithDeleted: deletedAt,
	})
	if err != nil {
		return nil, nil, err
	}
	if resp == nil {
		return nil, nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	version = convert2EvaluationSetVersion(ctx, resp.Version, resp.Dataset)
	set = convert2EvaluationSet(ctx, resp.Dataset)
	// 数据集返回的dataset结构体中version的值是草稿版本的值，这里需要替换一下
	if set != nil {
		set.EvaluationSetVersion = version
	}
	return version, set, nil
}

func (a *DatasetRPCAdapter) BatchGetVersionedDatasets(ctx context.Context, spaceID *int64, versionIDs []int64, deletedAt *bool) (sets []*rpc.BatchGetVersionedDatasetsResult, err error) {
	resp, err := a.client.BatchGetDatasetVersions(ctx, &dataset.BatchGetDatasetVersionsRequest{
		WorkspaceID: spaceID,
		VersionIds:  versionIDs,
		WithDeleted: deletedAt,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	sets = make([]*rpc.BatchGetVersionedDatasetsResult, 0)
	for _, v := range resp.VersionedDataset {
		version := convert2EvaluationSetVersion(ctx, v.Version, v.Dataset)
		set := convert2EvaluationSet(ctx, v.Dataset)
		// 数据集返回的dataset结构体中version的值是草稿版本的值，这里需要替换一下
		if set != nil {
			set.EvaluationSetVersion = version
		}
		sets = append(sets, &rpc.BatchGetVersionedDatasetsResult{
			EvaluationSet: set,
			Version:       version,
		})
	}
	return sets, nil
}

func (a *DatasetRPCAdapter) ListDatasetVersions(ctx context.Context, spaceID int64, evaluationSetID int64, pageToken *string, pageNumber, pageSize *int32, versionLike *string) (version []*entity.EvaluationSetVersion, total *int64, nextPageToken *string, err error) {
	resp, err := a.client.ListDatasetVersions(ctx, &dataset.ListDatasetVersionsRequest{
		WorkspaceID: &spaceID,
		DatasetID:   evaluationSetID,
		PageToken:   pageToken,
		PageSize:    pageSize,
		PageNumber:  pageNumber,
		VersionLike: versionLike,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	if resp == nil {
		return nil, nil, nil, errorx.NewByCode(errno.CommonRPCErrorCode, errorx.WithExtraMsg(fmt.Sprintf("ListDatasetVersions return nil, WorkspaceID: %v, evaluationSetID: %v", spaceID, evaluationSetID)))
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, nil, nil, errorx.NewByCode(errno.CommonRPCErrorCode, errorx.WithExtraMsg(fmt.Sprintf("ListDatasetVersions err, WorkspaceID: %v, evaluationSetID: %v, rpc code: %v", spaceID, evaluationSetID, resp.BaseResp.StatusCode)))
	}
	return convert2EvaluationSetVersions(ctx, resp.Versions), resp.Total, resp.NextPageToken, nil
}

func (a *DatasetRPCAdapter) UpdateDatasetSchema(ctx context.Context, spaceID int64, evaluationSetID int64, schemas []*entity.FieldSchema) (err error) {
	fieldSchemas, err := convert2DatasetFieldSchemas(ctx, schemas)
	if err != nil {
		return err
	}
	resp, err := a.client.UpdateDatasetSchema(ctx, &dataset.UpdateDatasetSchemaRequest{
		WorkspaceID: &spaceID,
		DatasetID:   evaluationSetID,
		Fields:      fieldSchemas,
	})
	if err != nil {
		return err
	}
	if resp == nil {
		return errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return nil
}

func (a *DatasetRPCAdapter) BatchCreateDatasetItems(ctx context.Context, param *rpc.BatchCreateDatasetItemsParam) (idMap map[int64]int64, errorGroup []*entity.ItemErrorGroup, err error) {
	datasetItems, err := convert2DatasetItems(ctx, param.Items)
	if err != nil {
		return nil, nil, err
	}
	resp, err := a.client.BatchCreateDatasetItems(ctx, &dataset.BatchCreateDatasetItemsRequest{
		WorkspaceID:      &param.SpaceID,
		DatasetID:        param.EvaluationSetID,
		Items:            datasetItems,
		SkipInvalidItems: param.SkipInvalidItems,
		AllowPartialAdd:  param.AllowPartialAdd,
	})
	if err != nil {
		return nil, nil, err
	}
	if resp == nil {
		return nil, nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		logs.CtxInfo(ctx, "BatchCreateDatasetItems resp: %v", json.Jsonify(resp))
		return nil, nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return resp.GetAddedItems(), convert2EvaluationSetErrorGroups(ctx, resp.GetErrors()), nil
}

func (a *DatasetRPCAdapter) UpdateDatasetItem(ctx context.Context, spaceID int64, evaluationSetID int64, itemID int64, turns []*entity.Turn) (err error) {
	data, err := convert2DatasetData(ctx, turns)
	if err != nil {
		return err
	}
	resp, err := a.client.UpdateDatasetItem(ctx, &dataset.UpdateDatasetItemRequest{
		WorkspaceID: &spaceID,
		DatasetID:   evaluationSetID,
		ItemID:      itemID,
		Data:        data,
	})
	if err != nil {
		return err
	}
	if resp == nil {
		return errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return nil
}

func (a *DatasetRPCAdapter) BatchDeleteDatasetItems(ctx context.Context, spaceID int64, evaluationSetID int64, itemIDs []int64) (err error) {
	resp, err := a.client.BatchDeleteDatasetItems(ctx, &dataset.BatchDeleteDatasetItemsRequest{
		WorkspaceID: &spaceID,
		DatasetID:   evaluationSetID,
		ItemIds:     itemIDs,
	})
	if err != nil {
		return err
	}
	if resp == nil {
		return errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return nil
}

func (a *DatasetRPCAdapter) ListDatasetItems(ctx context.Context, param *rpc.ListDatasetItemsParam) (items []*entity.EvaluationSetItem, total *int64, nextPageToken *string, err error) {
	resp, err := a.client.ListDatasetItems(ctx, &dataset.ListDatasetItemsRequest{
		WorkspaceID: &param.SpaceID,
		DatasetID:   param.EvaluationSetID,
		PageNumber:  param.PageNumber,
		PageSize:    param.PageSize,
		PageToken:   param.PageToken,
		OrderBys:    convert2DatasetOrderBys(ctx, param.OrderBys),
		// todo
		// ItemIDsNotIn: param.ItemIDsNotIn,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	if resp == nil {
		return nil, nil, nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, nil, nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return convert2EvaluationSetItems(ctx, resp.Items), resp.Total, resp.NextPageToken, nil
}

func (a *DatasetRPCAdapter) ListDatasetItemsByVersion(ctx context.Context, param *rpc.ListDatasetItemsParam) (items []*entity.EvaluationSetItem, total *int64, nextPageToken *string, err error) {
	resp, err := a.client.ListDatasetItemsByVersion(ctx, &dataset.ListDatasetItemsByVersionRequest{
		WorkspaceID: &param.SpaceID,
		DatasetID:   param.EvaluationSetID,
		VersionID:   gptr.Indirect(param.VersionID),
		PageNumber:  param.PageNumber,
		PageSize:    param.PageSize,
		PageToken:   param.PageToken,
		OrderBys:    convert2DatasetOrderBys(ctx, param.OrderBys),
	})
	if err != nil {
		return nil, nil, nil, err
	}
	if resp == nil {
		return nil, nil, nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, nil, nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return convert2EvaluationSetItems(ctx, resp.Items), resp.Total, resp.NextPageToken, nil
}

func (a *DatasetRPCAdapter) BatchGetDatasetItems(ctx context.Context, param *rpc.BatchGetDatasetItemsParam) (items []*entity.EvaluationSetItem, err error) {
	resp, err := a.client.BatchGetDatasetItems(ctx, &dataset.BatchGetDatasetItemsRequest{
		WorkspaceID: &param.SpaceID,
		DatasetID:   param.EvaluationSetID,
		ItemIds:     param.ItemIDs,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return convert2EvaluationSetItems(ctx, resp.Items), nil
}

func (a *DatasetRPCAdapter) BatchGetDatasetItemsByVersion(ctx context.Context, param *rpc.BatchGetDatasetItemsParam) (items []*entity.EvaluationSetItem, err error) {
	resp, err := a.client.BatchGetDatasetItemsByVersion(ctx, &dataset.BatchGetDatasetItemsByVersionRequest{
		WorkspaceID: &param.SpaceID,
		DatasetID:   param.EvaluationSetID,
		ItemIds:     param.ItemIDs,
		VersionID:   gptr.Indirect(param.VersionID),
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return convert2EvaluationSetItems(ctx, resp.Items), nil
}

func (a *DatasetRPCAdapter) ClearEvaluationSetDraftItem(ctx context.Context, spaceID, evaluationSetID int64) (err error) {
	_, err = a.client.ClearDatasetItem(ctx, &dataset.ClearDatasetItemRequest{WorkspaceID: &spaceID, DatasetID: evaluationSetID})
	if err != nil {
		return err
	}
	return nil
}

func (a *DatasetRPCAdapter) QueryItemSnapshotMappings(ctx context.Context, spaceID, datasetID int64, versionID *int64) (fieldMappings []*entity.ItemSnapshotFieldMapping, syncCkDate string, err error) {
	return nil, "", nil
}
