// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate mockgen -destination=mocks/dataset_provider_mock.go -package=mocks . IDatasetProvider
type IDatasetProvider interface {
	CreateDataset(ctx context.Context, dataset *entity.Dataset) (int64, error)
	UpdateDatasetSchema(ctx context.Context, dataset *entity.Dataset) error
	GetDataset(ctx context.Context, workspaceID, datasetID int64, category entity.DatasetCategory) (*entity.Dataset, error)
	ClearDatasetItems(ctx context.Context, workspaceID, datasetID int64, category entity.DatasetCategory) error
	AddDatasetItems(ctx context.Context, datasetID int64, category entity.DatasetCategory, items []*entity.DatasetItem) ([]*entity.DatasetItem, []entity.ItemErrorGroup, error)
	ValidateDatasetItems(ctx context.Context, dataset *entity.Dataset, items []*entity.DatasetItem, ignoreCurrentCount *bool) ([]*entity.DatasetItem, []entity.ItemErrorGroup, error)
}

var NoopDatasetProvider = &noopDatasetProvider{}

type noopDatasetProvider struct{}

var _ IDatasetProvider = (*noopDatasetProvider)(nil)

func (d *noopDatasetProvider) CreateDataset(ctx context.Context, dataset *entity.Dataset) (int64, error) {
	return 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset category is invalid"))
}

func (d *noopDatasetProvider) UpdateDatasetSchema(ctx context.Context, dataset *entity.Dataset) error {
	return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset category is invalid"))
}

// GetDataset 获取数据集
func (d *noopDatasetProvider) GetDataset(ctx context.Context, workspaceID, datasetID int64, category entity.DatasetCategory) (*entity.Dataset, error) {
	return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset category is invalid"))
}

// ClearDatasetItems 清空数据集项
func (d *noopDatasetProvider) ClearDatasetItems(ctx context.Context, workspaceID, datasetID int64, category entity.DatasetCategory) error {
	return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset category is invalid"))
}

// AddDatasetItems 添加数据集项
func (d *noopDatasetProvider) AddDatasetItems(ctx context.Context, datasetID int64, category entity.DatasetCategory, items []*entity.DatasetItem) ([]*entity.DatasetItem, []entity.ItemErrorGroup, error) {
	return nil, nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset category is invalid"))
}

// ValidateDatasetItems 验证数据集项
func (d *noopDatasetProvider) ValidateDatasetItems(ctx context.Context, dataset *entity.Dataset, items []*entity.DatasetItem, ignoreCurrentCount *bool) ([]*entity.DatasetItem, []entity.ItemErrorGroup, error) {
	return nil, nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset category is invalid"))
}
