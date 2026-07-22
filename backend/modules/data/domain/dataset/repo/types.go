// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"time"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/consts"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

type ListDatasetsParams struct {
	Paginator       *pagination.Paginator
	SpaceID         int64 `validate:"required,gt=0"` // 分片键
	IDs             []int64
	Category        entity.DatasetCategory
	CreatedBys      []string
	NameLike        string // 按名称模糊搜索，
	DescriptionLike string // 按描述模糊搜索
	BizCategorys    []string
}

type ListItemsParams struct {
	Paginator *pagination.Paginator
	SpaceID   int64 `validate:"required,gt=0"`
	DatasetID int64 `validate:"required,gt=0"`
	ItemKeys  []string
	ItemIDs   []int64
	AddVNEq   int64
	DelVNEq   int64
	AddVNLte  int64
	DelVNGt   int64
	ItemIDGt  int64
}

func NewListItemsParamsFromVersion(version *entity.DatasetVersion, taps ...func(*ListItemsParams)) *ListItemsParams {
	params := &ListItemsParams{
		SpaceID:   version.SpaceID,
		DatasetID: version.DatasetID,
		DelVNGt:   version.VersionNum,
		AddVNLte:  version.VersionNum,
	}
	for _, tap := range taps {
		tap(params)
	}
	return params
}

func NewListItemsParamsOfDataset(spaceID, datasetID int64, taps ...func(*ListItemsParams)) *ListItemsParams {
	params := &ListItemsParams{
		SpaceID:   spaceID,
		DatasetID: datasetID,
		DelVNEq:   consts.MaxVersionNum,
	}
	for _, tap := range taps {
		tap(params)
	}
	return params
}

type ListDatasetVersionsParams struct {
	Paginator   *pagination.Paginator
	SpaceID     int64 `validate:"required,gt=0"` // 分片键
	DatasetID   int64
	IDs         []int64
	Versions    []string
	VersionNums []int64
	VersionLike string // 按版本号模糊搜索
}

type ListItemSnapshotsParams struct {
	Paginator *pagination.Paginator
	SpaceID   int64 `validate:"required,gt=0"`
	VersionID int64 `validate:"required,gt=0"` // index, must set
}

type DeltaDatasetIOJob struct {
	Total          *int64
	Status         *entity.JobStatus
	PreProcessed   *int64 // DeltaProcessed 不为 0 时需设置
	DeltaProcessed int64
	DeltaAdded     int64
	SubProgresses  []*entity.DatasetIOJobProgress // 非空时覆盖现有进度
	Errors         []*entity.ItemErrorGroup       // 非空时覆盖现有错误
	StartedAt      *time.Time
	EndedAt        *time.Time
}

type ListIOJobsParams struct {
	SpaceID   int64 `validate:"required,gt=0"`
	DatasetID int64 `validate:"required,gt=0"`
	Types     []entity.JobType
	Statuses  []entity.JobStatus
}

type Opt struct {
	TX          *gorm.DB
	WithMaster  bool
	WithDeleted bool
}

type Option func(option *Opt)

func WithDeleted() Option {
	return func(option *Opt) {
		option.WithDeleted = true
	}
}

func WithMaster() Option {
	return func(option *Opt) {
		option.WithMaster = true
	}
}

func WithTransaction(tx *gorm.DB) Option {
	return func(option *Opt) {
		option.TX = tx
	}
}
