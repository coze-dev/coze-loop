// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/evaluation_set.go -package=mocks . IEvaluationSetService
type IEvaluationSetService interface {
	CreateEvaluationSet(ctx context.Context, param *entity.CreateEvaluationSetParam) (id int64, err error)
	UpdateEvaluationSet(ctx context.Context, param *entity.UpdateEvaluationSetParam) (err error)
	DeleteEvaluationSet(ctx context.Context, spaceID, evaluationSetID int64) (err error)
	GetEvaluationSet(ctx context.Context, spaceID *int64, evaluationSetID int64, deletedAt *bool) (set *entity.EvaluationSet, err error)
	BatchGetEvaluationSets(ctx context.Context, spaceID *int64, evaluationSetID []int64, deletedAt *bool) (set []*entity.EvaluationSet, err error)
	ListEvaluationSets(ctx context.Context, param *entity.ListEvaluationSetsParam) (sets []*entity.EvaluationSet, total *int64, nextPageToken *string, err error)
	QueryItemSnapshotMappings(ctx context.Context, spaceID, datasetID int64, versionID *int64) (fieldMappings []*entity.ItemSnapshotFieldMapping, syncCkDate string, err error)
}

//type CreateEvaluationSetParam struct {
//	SpaceID             int64
//	Name                string
//	Description         *string
//	EvaluationSetSchema *entity.EvaluationSetSchema
//	BizCategory         *entity.BizCategory
//	Session             *entity.Session
//}
//
//type UpdateEvaluationSetParam struct {
//	SpaceID         int64
//	EvaluationSetID int64
//	Name            *string
//	Description     *string
//}
//
//type ListEvaluationSetsParam struct {
//	SpaceID          int64
//	EvaluationSetIDs []int64
//	Name             *string
//	Creators         []string
//	PageNumber       *int32
//	PageSize         *int32
//	PageToken        *string
//	OrderBys         []*entity.OrderBy
//}
