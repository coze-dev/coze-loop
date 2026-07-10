// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	common_entity "github.com/coze-dev/coze-loop/backend/modules/data/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

//go:generate mockgen -destination=mocks/repo.go -package=mocks . IDatasetAPI
type IDatasetAPI interface {
	IDatasetRepo
	ISchemaRepo
	IVersionRepo
	IOperationRepo
	IItemRepo
	IItemSnapshotRepo
	IIOJobRepo
}

type IDatasetRepo interface {
	GetItemCount(ctx context.Context, datasetID int64) (int64, error)
	MGetItemCount(ctx context.Context, datasetID ...int64) (map[int64]int64, error)
	SetItemCount(ctx context.Context, datasetID int64, n int64) error
	IncrItemCount(ctx context.Context, datasetID int64, n int64) (int64, error)

	CreateDatasetAndSchema(ctx context.Context, dataset *entity.Dataset, fields []*entity.FieldSchema) error
	GetDataset(ctx context.Context, spaceID, id int64, opt ...Option) (*entity.Dataset, error)
	MGetDatasets(ctx context.Context, spaceID int64, ids []int64, opt ...Option) ([]*entity.Dataset, error)
	PatchDataset(ctx context.Context, patch, where *entity.Dataset, opt ...Option) error
	DeleteDataset(ctx context.Context, spaceID, id int64, opt ...Option) error
	ListDatasets(ctx context.Context, params *ListDatasetsParams, opt ...Option) ([]*entity.Dataset, *pagination.PageResult, error)
	// ListDatasetIDs 仅返回满足条件的 dataset id（游标分页），用于 item_count 阈值统计等无需完整行的场景。
	ListDatasetIDs(ctx context.Context, params *ListDatasetsParams, opt ...Option) ([]int64, *pagination.PageResult, error)
	CountDatasets(ctx context.Context, params *ListDatasetsParams, opt ...Option) (int64, error)
}

type ISchemaRepo interface {
	GetSchema(ctx context.Context, spaceID, id int64, opt ...Option) (*entity.DatasetSchema, error)
	MGetSchema(ctx context.Context, spaceID int64, ids []int64, opt ...Option) ([]*entity.DatasetSchema, error)
	CreateSchema(ctx context.Context, schema *entity.DatasetSchema, opt ...Option) error
	UpdateSchema(ctx context.Context, updateVersion int64, schema *entity.DatasetSchema, opt ...Option) error
}

type IItemRepo interface {
	MSetItemData(ctx context.Context, items []*entity.Item, provider common_entity.Provider) (int, error)
	MGetItemData(ctx context.Context, items []*entity.Item, provider common_entity.Provider) error

	MCreateItems(ctx context.Context, items []*entity.Item, opt ...Option) ( /*新写入的 item 数量，不包含 update on conflict 的数量*/ int64, error)
	ListItems(ctx context.Context, params *ListItemsParams, opt ...Option) ([]*entity.Item, *pagination.PageResult, error)
	CountItems(ctx context.Context, params *ListItemsParams, opt ...Option) (int64, error)

	UpdateItem(ctx context.Context, item *entity.Item, opt ...Option) error
	DeleteItems(ctx context.Context, spaceID int64, ids []int64, opt ...Option) error
	ArchiveItems(ctx context.Context, spaceID, delVN int64, ids []int64, opt ...Option) error
	// ClearDataset 清空 dataset 所有 items
	ClearDataset(ctx context.Context, spaceID, datasetID, delVN int64, opt ...Option) ([]*entity.ItemIdentity, error)
}

type IVersionRepo interface {
	GetItemCountOfVersion(ctx context.Context, versionID int64) (*int64, error)
	SetItemCountOfVersion(ctx context.Context, datasetID int64, n int64) error

	CreateVersion(ctx context.Context, version *entity.DatasetVersion, opt ...Option) error
	GetVersion(ctx context.Context, spaceID, versionID int64, opt ...Option) (*entity.DatasetVersion, error)
	MGetVersions(ctx context.Context, spaceID int64, ids []int64, opt ...Option) ([]*entity.DatasetVersion, error)
	ListVersions(ctx context.Context, params *ListDatasetVersionsParams, opt ...Option) ([]*entity.DatasetVersion, *pagination.PageResult, error)
	CountVersions(ctx context.Context, params *ListDatasetVersionsParams, opt ...Option) (int64, error)
	PatchVersion(ctx context.Context, patch, where *entity.DatasetVersion, opt ...Option) error
}

type IOperationRepo interface {
	AddDatasetOperation(ctx context.Context, datasetID int64, op *entity.DatasetOperation) error
	DelDatasetOperation(ctx context.Context, datasetID int64, opType entity.DatasetOpType, id string) error
	MGetDatasetOperations(ctx context.Context, datasetID int64, opTypes []entity.DatasetOpType) (map[entity.DatasetOpType][]*entity.DatasetOperation, error)
}

type IIOJobRepo interface {
	CreateIOJob(ctx context.Context, job *entity.IOJob, opt ...Option) error
	GetIOJob(ctx context.Context, id int64, opt ...Option) (*entity.IOJob, error)
	UpdateIOJob(ctx context.Context, id int64, delta *DeltaDatasetIOJob, opt ...Option) error
	ListIOJobs(ctx context.Context, params *ListIOJobsParams, opt ...Option) ([]*entity.IOJob, error)
}

type IItemSnapshotRepo interface {
	BatchUpsertItemSnapshots(ctx context.Context, snapshots []*entity.ItemSnapshot, opt ...Option) (int64, error)
	ListItemSnapshots(ctx context.Context, params *ListItemSnapshotsParams, opt ...Option) ([]*entity.ItemSnapshot, *pagination.PageResult, error)
	CountItemSnapshots(ctx context.Context, params *ListItemSnapshotsParams, opt ...Option) (int64, error)
}
