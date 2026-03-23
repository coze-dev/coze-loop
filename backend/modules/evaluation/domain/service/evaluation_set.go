// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
)

//go:generate mockgen -destination=mocks/evaluation_set.go -package=mocks . IEvaluationSetService
type IEvaluationSetService interface {
	CreateEvaluationSet(ctx context.Context, param *entity.CreateEvaluationSetParam) (id int64, err error)
	CreateEvaluationSetWithImport(ctx context.Context, param *entity.CreateEvaluationSetWithImportParam) (id, jobID int64, err error)
	ParseImportSourceFile(ctx context.Context, param *entity.ParseImportSourceFileParam) (*entity.ParseImportSourceFileResult, error)
	ValidateMultiPartData(ctx context.Context, spaceID int64, previewData []string, storeOption *entity.MultiModalStoreOption) ([]*entity.UploadAttachmentDetail, error)
	UpdateEvaluationSet(ctx context.Context, param *entity.UpdateEvaluationSetParam) (err error)
	DeleteEvaluationSet(ctx context.Context, spaceID, evaluationSetID int64) (err error)
	GetEvaluationSet(ctx context.Context, spaceID *int64, evaluationSetID int64, deletedAt *bool) (set *entity.EvaluationSet, err error)
	BatchGetEvaluationSets(ctx context.Context, spaceID *int64, evaluationSetID []int64, deletedAt *bool) (set []*entity.EvaluationSet, err error)
	ListEvaluationSets(ctx context.Context, param *entity.ListEvaluationSetsParam) (sets []*entity.EvaluationSet, total *int64, nextPageToken *string, err error)
	ImportEvaluationSet(ctx context.Context, param *entity.ImportEvaluationSetParam) (jobID int64, err error)
	GetEvaluationSetIOJob(ctx context.Context, spaceID, jobID int64) (job *entity.DatasetIOJob, err error)
	QueryItemSnapshotMappings(ctx context.Context, req *rpc.QueryItemSnapshotMappingRequest) (fieldMappings []*entity.ItemSnapshotFieldMapping, syncCkDate string, err error)
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
