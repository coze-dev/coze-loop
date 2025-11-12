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
	evaluationSetVersionServiceOnce = sync.Once{}
	evaluationSetVersionServiceImpl EvaluationSetVersionService
)

type EvaluationSetVersionServiceImpl struct {
	datasetRPCAdapter rpc.IDatasetRPCAdapter
}

func NewEvaluationSetVersionServiceImpl(datasetRPCAdapter rpc.IDatasetRPCAdapter) EvaluationSetVersionService {
	evaluationSetVersionServiceOnce.Do(func() {
		evaluationSetVersionServiceImpl = &EvaluationSetVersionServiceImpl{
			datasetRPCAdapter: datasetRPCAdapter,
		}
	})
	return evaluationSetVersionServiceImpl
}

func (d *EvaluationSetVersionServiceImpl) CreateEvaluationSetVersion(ctx context.Context, param *entity.CreateEvaluationSetVersionParam) (id int64, err error) {
	if param == nil {
		return 0, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	// 依赖数据集服务
	return d.datasetRPCAdapter.CreateDatasetVersion(ctx, param.SpaceID, param.EvaluationSetID, param.Version, param.Description)
}

func (d *EvaluationSetVersionServiceImpl) GetEvaluationSetVersion(ctx context.Context, spaceID, versionID int64, deletedAt *bool) (version *entity.EvaluationSetVersion, set *entity.EvaluationSet, err error) {
	// 依赖数据集服务
	return d.datasetRPCAdapter.GetDatasetVersion(ctx, spaceID, versionID, deletedAt)
}

func (d *EvaluationSetVersionServiceImpl) ListEvaluationSetVersions(ctx context.Context, param *entity.ListEvaluationSetVersionsParam) (sets []*entity.EvaluationSetVersion, total *int64, nextCursor *string, err error) {
	if param == nil {
		return nil, nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	// 依赖数据集服务
	return d.datasetRPCAdapter.ListDatasetVersions(ctx, param.SpaceID, param.EvaluationSetID, param.PageToken, param.PageNumber, param.PageSize, param.VersionLike, param.Versions)
}

func (d *EvaluationSetVersionServiceImpl) BatchGetEvaluationSetVersions(ctx context.Context, spaceID *int64, versionIDs []int64, deletedAt *bool) (sets []*entity.BatchGetEvaluationSetVersionsResult, err error) {
	// 依赖数据集服务
	datasets, err := d.datasetRPCAdapter.BatchGetVersionedDatasets(ctx, spaceID, versionIDs, deletedAt)
	if err != nil {
		return nil, err
	}
	sets = make([]*entity.BatchGetEvaluationSetVersionsResult, 0)
	for _, dataset := range datasets {
		sets = append(sets, &entity.BatchGetEvaluationSetVersionsResult{
			Version:       dataset.Version,
			EvaluationSet: dataset.EvaluationSet,
		})
	}
	return sets, nil
}
