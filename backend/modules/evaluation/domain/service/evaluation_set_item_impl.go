// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"sync"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

var (
	evaluationSetItemServiceOnce = sync.Once{}
	evaluationSetItemServiceImpl EvaluationSetItemService
)

type EvaluationSetItemServiceImpl struct {
	datasetRPCAdapter rpc.IDatasetRPCAdapter
}

func NewEvaluationSetItemServiceImpl(datasetRPCAdapter rpc.IDatasetRPCAdapter) EvaluationSetItemService {
	evaluationSetItemServiceOnce.Do(func() {
		evaluationSetItemServiceImpl = &EvaluationSetItemServiceImpl{
			datasetRPCAdapter: datasetRPCAdapter,
		}
	})
	return evaluationSetItemServiceImpl
}

func (d *EvaluationSetItemServiceImpl) BatchCreateEvaluationSetItems(ctx context.Context, param *entity.BatchCreateEvaluationSetItemsParam) (idMap map[int64]int64, errors []*entity.ItemErrorGroup, itemOutputs []*entity.DatasetItemOutput, err error) {
	if param == nil {
		return nil, nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	return d.datasetRPCAdapter.BatchCreateDatasetItems(ctx, &rpc.BatchCreateDatasetItemsParam{
		SpaceID:           param.SpaceID,
		EvaluationSetID:   param.EvaluationSetID,
		Items:             param.Items,
		SkipInvalidItems:  param.SkipInvalidItems,
		AllowPartialAdd:   param.AllowPartialAdd,
		FieldWriteOptions: param.FieldWriteOptions,
	})
}

func (d *EvaluationSetItemServiceImpl) BatchUpdateEvaluationSetItems(ctx context.Context, param *entity.BatchUpdateEvaluationSetItemsParam) (errors []*entity.ItemErrorGroup, itemOutputs []*entity.DatasetItemOutput, err error) {
	if param == nil {
		return nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	return d.datasetRPCAdapter.BatchUpdateDatasetItems(ctx, &rpc.BatchUpdateDatasetItemsParam{
		SpaceID:           param.SpaceID,
		EvaluationSetID:   param.EvaluationSetID,
		Items:             param.Items,
		SkipInvalidItems:  param.SkipInvalidItems,
		FieldWriteOptions: param.FieldWriteOptions,
	})
}

func (d *EvaluationSetItemServiceImpl) UpdateEvaluationSetItem(ctx context.Context, spaceID, evaluationSetID, itemID int64, turns []*entity.Turn, fieldWriteOptions []*entity.FieldWriteOption, tags []*entity.ResourceTagRef) (err error) {
	return d.datasetRPCAdapter.UpdateDatasetItem(ctx, spaceID, evaluationSetID, itemID, turns, fieldWriteOptions, tags)
}

func (d *EvaluationSetItemServiceImpl) BatchDeleteEvaluationSetItems(ctx context.Context, spaceID, evaluationSetID int64, itemIDs []int64) (err error) {
	return d.datasetRPCAdapter.BatchDeleteDatasetItems(ctx, spaceID, evaluationSetID, itemIDs)
}

func (d *EvaluationSetItemServiceImpl) ListEvaluationSetItems(ctx context.Context, param *entity.ListEvaluationSetItemsParam) (items []*entity.EvaluationSetItem, total, filterTotal *int64, nextPageToken *string, err error) {
	if param == nil {
		return nil, nil, nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	listParam := &rpc.ListDatasetItemsParam{
		SpaceID:         param.SpaceID,
		EvaluationSetID: param.EvaluationSetID,
		VersionID:       param.VersionID,
		PageNumber:      param.PageNumber,
		PageSize:        param.PageSize,
		PageToken:       param.PageToken,
		OrderBys:        param.OrderBys,
		ItemIDsNotIn:    param.ItemIDsNotIn,
		Filter:          param.Filter,
		TagFilter:       param.TagFilter,
	}
	if param.VersionID == nil {
		return d.datasetRPCAdapter.ListDatasetItems(ctx, listParam)
	}
	return d.datasetRPCAdapter.ListDatasetItemsByVersion(ctx, listParam)
}

func (d *EvaluationSetItemServiceImpl) BatchGetEvaluationSetItems(ctx context.Context, param *entity.BatchGetEvaluationSetItemsParam) (items []*entity.EvaluationSetItem, err error) {
	if param == nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}

	// 按版本引用批量获取 items，按 100 一批分页查询，区分草稿版本走不同的下游接口
	if len(param.ItemVersionQueries) > 0 {
		return d.batchGetByVersionQueries(ctx, param)
	}

	if len(param.ItemIDs) == 0 {
		return nil, nil
	}

	// 按 ItemIDs 批量获取，下游有单次数量限制，按 100 条分批查询
	return d.batchGetByItemIDs(ctx, param)
}

func (d *EvaluationSetItemServiceImpl) batchGetByVersionQueries(ctx context.Context, param *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	const batchSize = 100
	total := len(param.ItemVersionQueries)
	var items []*entity.EvaluationSetItem

	for start := 0; start < total; start += batchSize {
		end := start + batchSize
		if end > total {
			end = total
		}
		listParam := &rpc.BatchGetDatasetItemsParam{
			SpaceID:            param.SpaceID,
			EvaluationSetID:    param.EvaluationSetID,
			ItemVersionQueries: param.ItemVersionQueries[start:end],
			VersionID:          param.VersionID,
			Filter:             param.Filter,
			TagFilter:          param.TagFilter,
		}
		batchItems, err := d.batchGetDatasetItems(ctx, listParam, param.VersionID)
		if err != nil {
			return nil, err
		}
		items = append(items, batchItems...)
	}
	return items, nil
}

func (d *EvaluationSetItemServiceImpl) batchGetByItemIDs(ctx context.Context, param *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	const batchSize = 100
	totalIDs := len(param.ItemIDs)
	var items []*entity.EvaluationSetItem

	for start := 0; start < totalIDs; start += batchSize {
		end := start + batchSize
		if end > totalIDs {
			end = totalIDs
		}
		listParam := &rpc.BatchGetDatasetItemsParam{
			SpaceID:         param.SpaceID,
			EvaluationSetID: param.EvaluationSetID,
			ItemIDs:         param.ItemIDs[start:end],
			VersionID:       param.VersionID,
			Filter:          param.Filter,
			TagFilter:       param.TagFilter,
		}
		batchItems, err := d.batchGetDatasetItems(ctx, listParam, param.VersionID)
		if err != nil {
			return nil, err
		}
		items = append(items, batchItems...)
	}
	return items, nil
}

func (d *EvaluationSetItemServiceImpl) batchGetDatasetItems(ctx context.Context, param *rpc.BatchGetDatasetItemsParam, versionID *int64) ([]*entity.EvaluationSetItem, error) {
	if versionID == nil {
		return d.datasetRPCAdapter.BatchGetDatasetItems(ctx, param)
	}
	return d.datasetRPCAdapter.BatchGetDatasetItemsByVersion(ctx, param)
}

func (d *EvaluationSetItemServiceImpl) ClearEvaluationSetDraftItem(ctx context.Context, spaceID, evaluationSetID int64) (err error) {
	return d.datasetRPCAdapter.ClearEvaluationSetDraftItem(ctx, spaceID, evaluationSetID)
}

func (d *EvaluationSetItemServiceImpl) GetEvaluationSetItemField(ctx context.Context, param *entity.GetEvaluationSetItemFieldParam) (fieldData *entity.FieldData, err error) {
	if param == nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	return d.datasetRPCAdapter.GetDatasetItemField(ctx, &rpc.GetDatasetItemFieldParam{
		SpaceID:         param.SpaceID,
		EvaluationSetID: param.EvaluationSetID,
		ItemPK:          param.ItemPK,
		FieldName:       param.FieldName,
		FieldKey:        param.FieldKey,
		TurnID:          param.TurnID,
	})
}

func (d *EvaluationSetItemServiceImpl) UpdateEvaluationSetItemDef(ctx context.Context, spaceID, evaluationSetID, itemID int64, itemKey, status *string) error {
	return d.datasetRPCAdapter.UpdateDatasetItemDef(ctx, spaceID, evaluationSetID, itemID, itemKey, status)
}

func (d *EvaluationSetItemServiceImpl) GetEvaluationSetItemDef(ctx context.Context, spaceID, evaluationSetID, itemID int64) (*entity.EvaluationSetItemDef, error) {
	return d.datasetRPCAdapter.GetDatasetItemDef(ctx, spaceID, evaluationSetID, itemID)
}

func (d *EvaluationSetItemServiceImpl) ListEvaluationSetItemDefs(ctx context.Context, param *entity.ListEvaluationSetItemDefsParam) ([]*entity.EvaluationSetItemDef, *int64, *string, error) {
	if param == nil {
		return nil, nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	return d.datasetRPCAdapter.ListDatasetItemDefs(ctx, &rpc.ListDatasetItemDefsParam{
		SpaceID:         param.SpaceID,
		EvaluationSetID: param.EvaluationSetID,
		PageNumber:      param.PageNumber,
		PageSize:        param.PageSize,
		PageToken:       param.PageToken,
		OrderBys:        param.OrderBys,
	})
}

func (d *EvaluationSetItemServiceImpl) ListEvaluationSetItemVersions(ctx context.Context, param *entity.ListEvaluationSetItemVersionsParam) ([]*entity.EvaluationSetItemVersion, *int64, *string, error) {
	if param == nil {
		return nil, nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	return d.datasetRPCAdapter.ListDatasetItemVersions(ctx, &rpc.ListDatasetItemVersionsParam{
		SpaceID:         param.SpaceID,
		EvaluationSetID: param.EvaluationSetID,
		ItemID:          param.ItemID,
		PageNumber:      param.PageNumber,
		PageSize:        param.PageSize,
		PageToken:       param.PageToken,
		OrderBys:        param.OrderBys,
	})
}

func (d *EvaluationSetItemServiceImpl) GetEvaluationSetItemVersion(ctx context.Context, spaceID, evaluationSetID, itemID int64, itemVersionID *int64, itemVersion *string) (*entity.EvaluationSetItemVersion, error) {
	return d.datasetRPCAdapter.GetDatasetItemVersion(ctx, spaceID, evaluationSetID, itemID, itemVersionID, itemVersion)
}

func (d *EvaluationSetItemServiceImpl) UpdateEvaluationSetItemVersion(ctx context.Context, spaceID, evaluationSetID, itemID int64, itemVersionID *int64, status, description, itemVersion *string) error {
	return d.datasetRPCAdapter.UpdateDatasetItemVersion(ctx, spaceID, evaluationSetID, itemID, itemVersionID, status, description, itemVersion)
}

func (d *EvaluationSetItemServiceImpl) BatchAddExistEvaluationSetItems(ctx context.Context, param *entity.BatchAddExistEvaluationSetItemsParam) (*entity.BatchAddExistEvaluationSetItemsResult, error) {
	if param == nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	return d.datasetRPCAdapter.BatchAddExistDatasetItems(ctx, &rpc.BatchAddExistDatasetItemsParam{
		SpaceID:         param.SpaceID,
		EvaluationSetID: param.EvaluationSetID,
		Items:           param.Items,
		AllowPartialAdd: param.AllowPartialAdd,
	})
}
