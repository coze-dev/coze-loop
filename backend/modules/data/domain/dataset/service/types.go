// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
)

type DatasetWithSchema struct {
	*entity.Dataset
	Schema *entity.DatasetSchema
}

type VersionedDatasetWithSchema struct {
	*entity.Dataset
	Version *entity.DatasetVersion
	Schema  *entity.DatasetSchema
}

type IndexedItem struct {
	Index int // 批量写入 items 时保存原 item 的索引信息
	*entity.Item
}

type MAddItemOpt struct {
	PartialAdd bool
}

type GetOpt struct {
	WithDeleted bool
}

func WithDeleted(d bool) *GetOpt {
	return &GetOpt{WithDeleted: d}
}

type SearchDatasetsParam struct {
	SpaceID    int64
	DatasetIDs []int64
	Category   entity.DatasetCategory
	// 支持模糊搜索
	Name       *string
	CreatedBys []string
	/* pagination */
	Page *int32
	// 分页大小(0, 200]，默认为 20
	PageSize *int32
	// 与 page 同时提供时，优先使用 cursor
	Cursor       *string
	OrderBy      *OrderBy
	BizCategorys []string
}

type OrderBy struct {
	// 排序字段
	Field *string
	// 升序，默认倒序
	IsAsc *bool
}

type SearchDatasetsResults struct {
	DatasetWithSchemas []*DatasetWithSchema
	HasMore            bool
	NextCursor         string
	Total              int64
}

type UpdateDatasetParam struct {
	SpaceID     int64
	DatasetID   int64
	Name        string
	Description *string
	UpdatedBy   string
}

type ValidateDatasetItemsParam struct {
	SpaceID                int64
	DatasetID              int64
	DatasetCategory        entity.DatasetCategory
	DatasetFields          []*entity.FieldSchema
	Items                  []*entity.Item
	IgnoreCurrentItemCount bool
}

type ValidateDatasetItemsResult struct {
	ValidItemIndices []int32
	ErrorGroups      []*entity.ItemErrorGroup
}
