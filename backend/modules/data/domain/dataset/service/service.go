// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"io/fs"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/vfs"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/component/mq"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo"
	common_entity "github.com/coze-dev/coze-loop/backend/modules/data/domain/entity"
)

//go:generate mockgen -destination=mocks/service.go -package=mocks . IDatasetAPI
type IDatasetAPI interface {
	IDatasetService
	ISchemaService
	IVersionService
	IItemService
	IItemSnapshotService
	IFileStoreService
	IIOJobService
}

type IDatasetService interface {
	CreateDataset(ctx context.Context, dataset *entity.Dataset, fields []*entity.FieldSchema) error
	UpdateDataset(ctx context.Context, param *UpdateDatasetParam) error
	DeleteDataset(ctx context.Context, spaceID, id int64) error
	GetDataset(ctx context.Context, spaceID, id int64) (*DatasetWithSchema, error)
	BatchGetDataset(ctx context.Context, spaceID int64, ids []int64) ([]*DatasetWithSchema, error)
	GetDatasetWithOpt(ctx context.Context, spaceID, id int64, opt *GetOpt) (*DatasetWithSchema, error)
	BatchGetDatasetWithOpt(ctx context.Context, spaceID int64, ids []int64, opt *GetOpt) ([]*DatasetWithSchema, error)
	SearchDataset(ctx context.Context, req *SearchDatasetsParam) (*SearchDatasetsResults, error)
}

type ISchemaService interface {
	UpdateSchema(ctx context.Context, ds *entity.Dataset, fields []*entity.FieldSchema, updatedBy string) error
}

type IVersionService interface {
	CreateVersion(ctx context.Context, ds *DatasetWithSchema, version *entity.DatasetVersion) error
	GetVersionWithOpt(ctx context.Context, spaceID, versionID int64, opt *GetOpt) (*entity.DatasetVersion, *DatasetWithSchema, error)
	GetOrSetItemCountOfVersion(ctx context.Context, version *entity.DatasetVersion) (int64, error)
	BatchGetVersionedDatasetsWithOpt(ctx context.Context, spaceID int64, versionIDs []int64, opt *GetOpt) ([]*VersionedDatasetWithSchema, error)
}

type IItemService interface {
	// ValidateDatasetItems 校验 items 是否符合 dataset 的规格
	ValidateDatasetItems(ctx context.Context, param *ValidateDatasetItemsParam) (*ValidateDatasetItemsResult, error)
	// BatchCreateItems 创建新 items，支持 itemKey 幂等锁
	BatchCreateItems(ctx context.Context, ds *DatasetWithSchema, items []*IndexedItem, opt *MAddItemOpt) (added []*IndexedItem, err error)
	// BatchGetItems 获取 items
	BatchGetItems(ctx context.Context, spaceID, datasetID int64, itemIDs []int64) ([]*entity.Item, error)
	// GetItem 获取 item
	GetItem(ctx context.Context, spaceID, datasetID, itemID int64) (*entity.Item, error)
	// LoadItemData 填充 item 的数据内容，为多模态文件签发 URL
	LoadItemData(ctx context.Context, items ...*entity.Item) error
	// ArchiveAndCreateItem 归档并创建新的 item，用于更新有版本引用的 item
	ArchiveAndCreateItem(ctx context.Context, ds *DatasetWithSchema, oldID int64, item *entity.Item) error
	// UpdateItem 更新 item，用于无版本引用的 item
	UpdateItem(ctx context.Context, ds *DatasetWithSchema, item *entity.Item) error
	// BatchDeleteItems 删除 items
	BatchDeleteItems(ctx context.Context, ds *DatasetWithSchema, items ...*entity.Item) error
	// ClearDataset 清空 dataset 所有 items
	ClearDataset(ctx context.Context, ds *DatasetWithSchema) error
}

type IItemSnapshotService interface {
	RunSnapshotItemJob(ctx context.Context, msg *entity.JobRunMessage) error
}

type IIOJobService interface {
	GetIOJob(ctx context.Context, jobID int64) (*entity.IOJob, error)

	// CreateIOJob 创建 job, 并发送执行 job 的消息
	CreateIOJob(ctx context.Context, job *entity.IOJob) error
	RunIOJob(ctx context.Context, msg *entity.JobRunMessage) error
}

type IFileStoreService interface {
	StatFile(ctx context.Context, provider common_entity.Provider, path string) (fs.FileInfo, error)
}

var _ IDatasetAPI = (*DatasetServiceImpl)(nil)

type DatasetServiceImpl struct {
	txDB     db.Provider
	idgen    idgen.IIDGenerator
	repo     repo.IDatasetAPI
	fsUnion  vfs.IUnionFS
	producer mq.IDatasetJobPublisher
	configer conf.IConfig
	locker   lock.ILocker

	storageConfig func() *conf.DatasetItemStorage
	specConfig    func() *conf.DatasetSpec
	featConfig    func() *conf.DatasetFeature
	retryCfg      func() *conf.SnapshotRetry
}

func NewDatasetServiceImpl(db db.Provider, idgen idgen.IIDGenerator, repo repo.IDatasetAPI, configer conf.IConfig,
	producer mq.IDatasetJobPublisher, fsUnion vfs.IUnionFS, locker lock.ILocker,
) IDatasetAPI {
	return &DatasetServiceImpl{
		txDB:          db,
		idgen:         idgen,
		repo:          repo,
		fsUnion:       fsUnion,
		producer:      producer,
		configer:      configer,
		locker:        locker,
		storageConfig: configer.GetDatasetItemStorage,
		specConfig:    configer.GetDatasetSpec,
		featConfig:    configer.GetDatasetFeature,
		retryCfg:      configer.GetSnapshotRetry,
	}
}
