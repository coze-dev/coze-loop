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
	evaluationSetServiceOnce = sync.Once{}
	evaluationSetServiceImpl IEvaluationSetService
)

type EvaluationSetServiceImpl struct {
	datasetRPCAdapter rpc.IDatasetRPCAdapter
}

func NewEvaluationSetServiceImpl(datasetRPCAdapter rpc.IDatasetRPCAdapter) IEvaluationSetService {
	evaluationSetServiceOnce.Do(func() {
		evaluationSetServiceImpl = &EvaluationSetServiceImpl{
			datasetRPCAdapter: datasetRPCAdapter,
		}
	})
	return evaluationSetServiceImpl
}

func (d *EvaluationSetServiceImpl) CreateEvaluationSet(ctx context.Context, param *entity.CreateEvaluationSetParam) (id int64, err error) {
	if param == nil {
		return 0, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	// 依赖数据集服务
	return d.datasetRPCAdapter.CreateDataset(ctx, &rpc.CreateDatasetParam{
		SpaceID:            param.SpaceID,
		Name:               param.Name,
		Desc:               param.Description,
		EvaluationSetItems: param.EvaluationSetSchema,
		BizCategory:        param.BizCategory,
		Session:            param.Session,
	})
}

func (d *EvaluationSetServiceImpl) UpdateEvaluationSet(ctx context.Context, param *entity.UpdateEvaluationSetParam) (err error) {
	if param == nil {
		return errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	// 依赖数据集服务
	return d.datasetRPCAdapter.UpdateDataset(ctx, param.SpaceID, param.EvaluationSetID, param.Name, param.Description)
}

func (d *EvaluationSetServiceImpl) DeleteEvaluationSet(ctx context.Context, spaceID, evaluationSetID int64) (err error) {
	// 依赖数据集服务
	return d.datasetRPCAdapter.DeleteDataset(ctx, spaceID, evaluationSetID)
}

func (d *EvaluationSetServiceImpl) GetEvaluationSet(ctx context.Context, spaceID *int64, evaluationSetID int64, deletedAt *bool) (set *entity.EvaluationSet, err error) {
	// 依赖数据集服务
	return d.datasetRPCAdapter.GetDataset(ctx, spaceID, evaluationSetID, deletedAt)
}

func (d *EvaluationSetServiceImpl) BatchGetEvaluationSets(ctx context.Context, spaceID *int64, evaluationSetID []int64, deletedAt *bool) (set []*entity.EvaluationSet, err error) {
	// 依赖数据集服务
	return d.datasetRPCAdapter.BatchGetDatasets(ctx, spaceID, evaluationSetID, deletedAt)
}

func (d *EvaluationSetServiceImpl) ListEvaluationSets(ctx context.Context, param *entity.ListEvaluationSetsParam) (sets []*entity.EvaluationSet, total *int64, nextPageToken *string, err error) {
	if param == nil {
		return nil, nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	// 依赖数据集服务
	return d.datasetRPCAdapter.ListDatasets(ctx, &rpc.ListDatasetsParam{
		SpaceID:          param.SpaceID,
		EvaluationSetIDs: param.EvaluationSetIDs,
		Name:             param.Name,
		Creators:         param.Creators,
		PageNumber:       param.PageNumber,
		PageSize:         param.PageSize,
		PageToken:        param.PageToken,
		OrderBys:         param.OrderBys,
	})
}

func (d *EvaluationSetServiceImpl) QueryItemSnapshotMappings(ctx context.Context, spaceID, datasetID int64, versionID *int64) (fieldMappings []*entity.ItemSnapshotFieldMapping, syncCkDate string, err error) {
	return d.datasetRPCAdapter.QueryItemSnapshotMappings(ctx, spaceID, datasetID, versionID)
}
