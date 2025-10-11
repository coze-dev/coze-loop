// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/evaluation_set_item.go -package=mocks . EvaluationSetItemService
type EvaluationSetItemService interface {
	BatchCreateEvaluationSetItems(ctx context.Context, param *entity.BatchCreateEvaluationSetItemsParam) (idMap map[int64]int64, errors []*entity.ItemErrorGroup, itemOutputs []*entity.CreateDatasetItemOutput, err error)
	UpdateEvaluationSetItem(ctx context.Context, spaceID, evaluationSetID, itemID int64, turns []*entity.Turn) (err error)
	BatchDeleteEvaluationSetItems(ctx context.Context, spaceID int64, evaluationSetID int64, itemIDs []int64) (err error)
	ListEvaluationSetItems(ctx context.Context, param *entity.ListEvaluationSetItemsParam) (items []*entity.EvaluationSetItem, total *int64, nextPageToken *string, err error)
	BatchGetEvaluationSetItems(ctx context.Context, param *entity.BatchGetEvaluationSetItemsParam) (items []*entity.EvaluationSetItem, err error)
	ClearEvaluationSetDraftItem(ctx context.Context, spaceID, evaluationSetID int64) (err error)
}

//type ListEvaluationSetItemsParam struct {
//	SpaceID         int64
//	EvaluationSetID int64
//	VersionID       *int64
//	PageNumber      *int32
//	PageSize        *int32
//	PageToken       *string
//	OrderBys        []*entity.OrderBy
//	ItemIDsNotIn    []int64
//}
//type BatchGetEvaluationSetItemsParam struct {
//	SpaceID         int64
//	EvaluationSetID int64
//	ItemIDs         []int64
//	VersionID       *int64
//	PageNumber      *int32
//	PageSize        *int32
//	PageToken       *string
//	OrderBys        []*entity.OrderBy
//}
//
//type BatchCreateEvaluationSetItemsParam struct {
//	SpaceID         int64
//	EvaluationSetID int64
//	Items           []*entity.EvaluationSetItem
//	// items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据
//	SkipInvalidItems *bool
//	// 批量写入 items 如果超出数据集容量限制，默认不会写入任何数据；设置 partialAdd=true 会写入不超出容量限制的前 N 条
//	AllowPartialAdd *bool
//}
