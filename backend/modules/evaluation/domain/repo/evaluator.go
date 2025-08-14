// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IEvaluatorRepo 定义 Evaluator 的 Repo 接口
//
//go:generate mockgen -destination mocks/evaluator_mock.go -package mocks . IEvaluatorRepo
type IEvaluatorRepo interface {
	CreateEvaluator(ctx context.Context, evaluator *entity.Evaluator) (evaluatorID int64, err error)
	SubmitEvaluatorVersion(ctx context.Context, evaluatorVersionDO *entity.Evaluator) error

	BatchDeleteEvaluator(ctx context.Context, ids []int64, userID string) error

	UpdateEvaluatorDraft(ctx context.Context, version *entity.Evaluator) error
	UpdateEvaluatorMeta(ctx context.Context, id int64, name, description, userID string) error

	BatchGetEvaluatorMetaByID(ctx context.Context, ids []int64, includeDeleted bool) ([]*entity.Evaluator, error)
	BatchGetEvaluatorByVersionID(ctx context.Context, spaceID *int64, ids []int64, includeDeleted bool) ([]*entity.Evaluator, error)
	BatchGetEvaluatorDraftByEvaluatorID(ctx context.Context, spaceID int64, ids []int64, includeDeleted bool) ([]*entity.Evaluator, error)
	BatchGetEvaluatorVersionsByEvaluatorIDs(ctx context.Context, evaluatorIDs []int64, includeDeleted bool) ([]*entity.Evaluator, error)
	ListEvaluator(ctx context.Context, req *ListEvaluatorRequest) (*ListEvaluatorResponse, error)
	ListEvaluatorVersion(ctx context.Context, req *ListEvaluatorVersionRequest) (*ListEvaluatorVersionResponse, error)

	CheckNameExist(ctx context.Context, spaceID, evaluatorID int64, name string) (bool, error)
	CheckVersionExist(ctx context.Context, evaluatorID int64, version string) (bool, error)
}

type ListEvaluatorRequest struct {
	SpaceID       int64
	SearchName    string
	CreatorIDs    []int64
	EvaluatorType []entity.EvaluatorType
	PageSize      int32
	PageNum       int32
	OrderBy       []*entity.OrderBy
}

type ListEvaluatorResponse struct {
	TotalCount int64
	Evaluators []*entity.Evaluator
}

type ListEvaluatorVersionRequest struct {
	PageSize      int32
	PageNum       int32
	EvaluatorID   int64
	QueryVersions []string
	OrderBy       []*entity.OrderBy
}

type ListEvaluatorVersionResponse struct {
	TotalCount int64
	Versions   []*entity.Evaluator
}
