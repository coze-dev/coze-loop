// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/data_provider.go -package=mocks . IDatasetRPCAdapter
type IDatasetRPCAdapter interface {
	CreateDataset(ctx context.Context, param *CreateDatasetParam) (id int64, err error)
	UpdateDataset(ctx context.Context, spaceID int64, evaluationSetID int64, name *string, desc *string) (err error)
	DeleteDataset(ctx context.Context, spaceID int64, evaluationSetID int64) (err error)
	GetDataset(ctx context.Context, spaceID *int64, evaluationSetID int64, deletedAt *bool) (set *entity.EvaluationSet, err error)
	BatchGetDatasets(ctx context.Context, spaceID *int64, evaluationSetID []int64, deletedAt *bool) (sets []*entity.EvaluationSet, err error)
	ListDatasets(ctx context.Context, param *ListDatasetsParam) (sets []*entity.EvaluationSet, total *int64, nextPageToken *string, err error)

	CreateDatasetVersion(ctx context.Context, spaceID int64, evaluationSetID int64, version string, desc *string) (id int64, err error)
	GetDatasetVersion(ctx context.Context, spaceID int64, versionID int64, deletedAt *bool) (version *entity.EvaluationSetVersion, set *entity.EvaluationSet, err error)
	BatchGetVersionedDatasets(ctx context.Context, spaceID *int64, versionIDs []int64, deletedAt *bool) (sets []*BatchGetVersionedDatasetsResult, err error)
	ListDatasetVersions(ctx context.Context, spaceID int64, evaluationSetID int64, pageToken *string, pageNumber, pageSize *int32, versionLike *string) (version []*entity.EvaluationSetVersion, total *int64, nextPageToken *string, err error)

	UpdateDatasetSchema(ctx context.Context, spaceID int64, evaluationSetID int64, schemas []*entity.FieldSchema) (err error)

	BatchCreateDatasetItems(ctx context.Context, param *BatchCreateDatasetItemsParam) (idMap map[int64]int64, errorGroup []*entity.ItemErrorGroup, err error)
	UpdateDatasetItem(ctx context.Context, spaceID int64, evaluationSetID int64, itemID int64, turns []*entity.Turn) (err error)
	BatchDeleteDatasetItems(ctx context.Context, spaceID int64, evaluationSetID int64, itemIDs []int64) (err error)
	ListDatasetItems(ctx context.Context, param *ListDatasetItemsParam) (items []*entity.EvaluationSetItem, total *int64, nextPageToken *string, err error)
	ListDatasetItemsByVersion(ctx context.Context, param *ListDatasetItemsParam) (items []*entity.EvaluationSetItem, total *int64, nextPageToken *string, err error)
	BatchGetDatasetItems(ctx context.Context, param *BatchGetDatasetItemsParam) (items []*entity.EvaluationSetItem, err error)
	BatchGetDatasetItemsByVersion(ctx context.Context, param *BatchGetDatasetItemsParam) (items []*entity.EvaluationSetItem, err error)
	ClearEvaluationSetDraftItem(ctx context.Context, spaceID, evaluationSetID int64) (err error)
	QueryItemSnapshotMappings(ctx context.Context, spaceID, datasetID int64, versionID *int64) (fieldMappings []*entity.ItemSnapshotFieldMapping, syncCkDate string, err error)
}

type CreateDatasetParam struct {
	SpaceID            int64
	Name               string
	Desc               *string
	EvaluationSetItems *entity.EvaluationSetSchema
	BizCategory        *entity.BizCategory
	Session            *entity.Session
}

type ListDatasetsParam struct {
	SpaceID          int64
	EvaluationSetIDs []int64
	Name             *string
	Creators         []string
	PageNumber       *int32
	PageSize         *int32
	PageToken        *string
	OrderBys         []*entity.OrderBy
}

type ListDatasetItemsParam struct {
	SpaceID         int64
	EvaluationSetID int64
	VersionID       *int64
	PageNumber      *int32
	PageSize        *int32
	PageToken       *string
	OrderBys        []*entity.OrderBy
	ItemIDsNotIn    []int64
}

type BatchGetDatasetItemsParam struct {
	SpaceID         int64
	EvaluationSetID int64
	ItemIDs         []int64
	VersionID       *int64
}

type BatchCreateDatasetItemsParam struct {
	SpaceID         int64
	EvaluationSetID int64
	Items           []*entity.EvaluationSetItem
	// items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据
	SkipInvalidItems *bool
	// 批量写入 items 如果超出数据集容量限制，默认不会写入任何数据；设置 partialAdd=true 会写入不超出容量限制的前 N 条
	AllowPartialAdd *bool
}

type BatchGetVersionedDatasetsResult struct {
	Version       *entity.EvaluationSetVersion
	EvaluationSet *entity.EvaluationSet
}
