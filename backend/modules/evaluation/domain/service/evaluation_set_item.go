// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/evaluation_set_item.go -package=mocks . EvaluationSetItemService
type EvaluationSetItemService interface {
	BatchCreateEvaluationSetItems(ctx context.Context, param *entity.BatchCreateEvaluationSetItemsParam) (idMap map[int64]int64, errors []*entity.ItemErrorGroup, itemOutputs []*entity.DatasetItemOutput, err error)
	UpdateEvaluationSetItem(ctx context.Context, spaceID, evaluationSetID, itemID int64, turns []*entity.Turn) (err error)
	BatchUpdateEvaluationSetItems(ctx context.Context, param *entity.BatchUpdateEvaluationSetItemsParam) (errors []*entity.ItemErrorGroup, itemOutputs []*entity.DatasetItemOutput, err error)
	BatchDeleteEvaluationSetItems(ctx context.Context, spaceID, evaluationSetID int64, itemIDs []int64) (err error)
	ListEvaluationSetItems(ctx context.Context, param *entity.ListEvaluationSetItemsParam) (items []*entity.EvaluationSetItem, total *int64, nextPageToken *string, err error)
	BatchGetEvaluationSetItems(ctx context.Context, param *entity.BatchGetEvaluationSetItemsParam) (items []*entity.EvaluationSetItem, err error)
	ClearEvaluationSetDraftItem(ctx context.Context, spaceID, evaluationSetID int64) (err error)
	GetEvaluationSetItemField(ctx context.Context, param *entity.GetEvaluationSetItemFieldParam) (fieldData *entity.FieldData, err error)
}
