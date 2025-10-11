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

func (d *EvaluationSetItemServiceImpl) BatchCreateEvaluationSetItems(ctx context.Context, param *entity.BatchCreateEvaluationSetItemsParam) (idMap map[int64]int64, errors []*entity.ItemErrorGroup, itemOutputs []*entity.CreateDatasetItemOutput, err error) {
	if param == nil {
		return nil, nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	return d.datasetRPCAdapter.BatchCreateDatasetItems(ctx, &rpc.BatchCreateDatasetItemsParam{
		SpaceID:          param.SpaceID,
		EvaluationSetID:  param.EvaluationSetID,
		Items:            param.Items,
		SkipInvalidItems: param.SkipInvalidItems,
		AllowPartialAdd:  param.AllowPartialAdd,
	})
}

func (d *EvaluationSetItemServiceImpl) UpdateEvaluationSetItem(ctx context.Context, spaceID, evaluationSetID, itemID int64, turns []*entity.Turn) (err error) {
	return d.datasetRPCAdapter.UpdateDatasetItem(ctx, spaceID, evaluationSetID, itemID, turns)
}

func (d *EvaluationSetItemServiceImpl) BatchDeleteEvaluationSetItems(ctx context.Context, spaceID int64, evaluationSetID int64, itemIDs []int64) (err error) {
	return d.datasetRPCAdapter.BatchDeleteDatasetItems(ctx, spaceID, evaluationSetID, itemIDs)
}

func (d *EvaluationSetItemServiceImpl) ListEvaluationSetItems(ctx context.Context, param *entity.ListEvaluationSetItemsParam) (items []*entity.EvaluationSetItem, total *int64, nextPageToken *string, err error) {
	if param == nil {
		return nil, nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode)
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
	listParam := &rpc.BatchGetDatasetItemsParam{
		SpaceID:         param.SpaceID,
		EvaluationSetID: param.EvaluationSetID,
		ItemIDs:         param.ItemIDs,
		VersionID:       param.VersionID,
	}
	if param.VersionID == nil {
		return d.datasetRPCAdapter.BatchGetDatasetItems(ctx, listParam)
	}
	return d.datasetRPCAdapter.BatchGetDatasetItemsByVersion(ctx, listParam)
}

func (d *EvaluationSetItemServiceImpl) ClearEvaluationSetDraftItem(ctx context.Context, spaceID, evaluationSetID int64) (err error) {
	return d.datasetRPCAdapter.ClearEvaluationSetDraftItem(ctx, spaceID, evaluationSetID)
}
