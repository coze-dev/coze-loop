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
	UpdateEvaluatorMeta(ctx context.Context, req *entity.UpdateEvaluatorMetaRequest) error
	// UpdateEvaluatorVersionTags 根据版本ID全量更新标签：不存在的新增，不在传入列表中的删除
	UpdateEvaluatorVersionTags(ctx context.Context, versionID int64, tags map[entity.EvaluatorTagKey][]string) error

	BatchGetEvaluatorMetaByID(ctx context.Context, ids []int64, includeDeleted bool) ([]*entity.Evaluator, error)
	BatchGetEvaluatorByVersionID(ctx context.Context, spaceID *int64, ids []int64, includeDeleted bool) ([]*entity.Evaluator, error)
	BatchGetEvaluatorDraftByEvaluatorID(ctx context.Context, spaceID int64, ids []int64, includeDeleted bool) ([]*entity.Evaluator, error)
	BatchGetEvaluatorVersionsByEvaluatorIDs(ctx context.Context, evaluatorIDs []int64, includeDeleted bool) ([]*entity.Evaluator, error)
	ListEvaluator(ctx context.Context, req *ListEvaluatorRequest) (*ListEvaluatorResponse, error)
	ListEvaluatorVersion(ctx context.Context, req *ListEvaluatorVersionRequest) (*ListEvaluatorVersionResponse, error)

	CheckNameExist(ctx context.Context, spaceID, evaluatorID int64, name string) (bool, error)
	CheckVersionExist(ctx context.Context, evaluatorID int64, version string) (bool, error)

	// ListBuiltinEvaluator 根据筛选条件查询内置评估器列表，支持tag筛选和分页
	ListBuiltinEvaluator(ctx context.Context, req *ListBuiltinEvaluatorRequest) (*ListBuiltinEvaluatorResponse, error)
	// BatchGetBuiltinEvaluatorByVersionID 批量根据版本ID获取内置评估器，包含tag信息
	BatchGetBuiltinEvaluatorByVersionID(ctx context.Context, spaceID *int64, ids []int64, includeDeleted bool) ([]*entity.Evaluator, error)
}

type ListEvaluatorRequest struct {
	SpaceID       int64
	SearchName    string
	CreatorIDs    []int64
	EvaluatorType []entity.EvaluatorType
	FilterOption  *entity.EvaluatorFilterOption `json:"filter_option,omitempty"` // 标签筛选条件
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

// ListBuiltinEvaluatorRequest 查询内置评估器的请求参数
type ListBuiltinEvaluatorRequest struct {
	SpaceID        int64                         `json:"space_id"`
	FilterOption   *entity.EvaluatorFilterOption `json:"filter_option,omitempty"`   // 标签筛选条件
	PageSize       int32                         `json:"page_size"`                 // 分页大小
	PageNum        int32                         `json:"page_num"`                  // 页码
	IncludeDeleted bool                          `json:"include_deleted,omitempty"` // 是否包含已删除记录
}

// ListBuiltinEvaluatorResponse 查询内置评估器的响应
type ListBuiltinEvaluatorResponse struct {
	TotalCount int64               `json:"total_count"` // 总数量
	Evaluators []*entity.Evaluator `json:"evaluators"`  // 评估器列表
}
