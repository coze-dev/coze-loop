// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination mocks/evaluator_service_mock.go -package mocks . EvaluatorService
type EvaluatorService interface {
	// ListEvaluator 按查询条件查询 evaluator_version
	ListEvaluator(ctx context.Context, request *entity.ListEvaluatorRequest) ([]*entity.Evaluator, int64, error)
	// BatchGetEvaluator 按 id 批量查询 evaluator_version
	BatchGetEvaluator(ctx context.Context, spaceID int64, evaluatorIDs []int64, includeDeleted bool) ([]*entity.Evaluator, error)
	// GetEvaluator 按 id 单个查询 evaluator_version
	GetEvaluator(ctx context.Context, spaceID int64, evaluatorID int64, includeDeleted bool) (*entity.Evaluator, error)
	// CreateEvaluator 创建 evaluator_version
	CreateEvaluator(ctx context.Context, evaluator *entity.Evaluator, cid string) (int64, error)
	// UpdateEvaluatorMeta 修改 evaluator_version
	UpdateEvaluatorMeta(ctx context.Context, id, spaceID int64, name, description, userID string) error
	// UpdateEvaluatorDraft 修改 evaluator_version draft
	UpdateEvaluatorDraft(ctx context.Context, versionDO *entity.Evaluator) error
	// DeleteEvaluator 删除 evaluator_version
	DeleteEvaluator(ctx context.Context, evaluatorIDs []int64, userID string) error
	// RunEvaluator evaluator_version 运行
	RunEvaluator(ctx context.Context, request *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error)
	// DebugEvaluator 调试 evaluator_version
	DebugEvaluator(ctx context.Context, evaluatorDO *entity.Evaluator, inputData *entity.EvaluatorInputData) (*entity.EvaluatorOutputData, error)
	// GetEvaluatorVersion 按 version id 单个查询 evaluator_version version
	GetEvaluatorVersion(ctx context.Context, evaluatorVersionID int64, includeDeleted bool) (*entity.Evaluator, error)
	// BatchGetEvaluatorVersion 按 version id 批量查询 evaluator_version version
	BatchGetEvaluatorVersion(ctx context.Context, spaceID *int64, evaluatorVersionIDs []int64, includeDeleted bool) ([]*entity.Evaluator, error)
	// ListEvaluatorVersion 按条件查询 evaluator_version version
	ListEvaluatorVersion(ctx context.Context, request *entity.ListEvaluatorVersionRequest) (evaluatorVersions []*entity.Evaluator, total int64, err error)
	// SubmitEvaluatorVersion 提交 evaluator_version 版本
	SubmitEvaluatorVersion(ctx context.Context, evaluatorVersionDO *entity.Evaluator, version, description, cid string) (*entity.Evaluator, error)
	// CheckNameExist
	CheckNameExist(ctx context.Context, spaceID, evaluatorID int64, name string) (bool, error)
}

//go:generate mockgen -destination mocks/evaluator_record_service_mock.go -package mocks . EvaluatorRecordService
type EvaluatorRecordService interface {
	// CorrectEvaluatorRecord 创建 evaluator_version 运行结果
	CorrectEvaluatorRecord(ctx context.Context, evaluatorRecordDO *entity.EvaluatorRecord, correctionDO *entity.Correction) error
	// GetEvaluatorRecord 按 id 查询单个 evaluator_version 运行结果
	GetEvaluatorRecord(ctx context.Context, evaluatorRecordID int64, includeDeleted bool) (*entity.EvaluatorRecord, error)
	// BatchGetEvaluatorRecord 按 id 批量查询 evaluator_version 运行结果
	BatchGetEvaluatorRecord(ctx context.Context, evaluatorRecordIDs []int64, includeDeleted bool) ([]*entity.EvaluatorRecord, error)
}

//type ListEvaluatorRequest struct {
//	SpaceID       int64                  `json:"space_id"`
//	SearchName    string                 `json:"search_name,omitempty"`
//	CreatorIDs    []int64                `json:"creator_ids,omitempty"`
//	EvaluatorType []entity.EvaluatorType `json:"evaluator_type,omitempty"`
//	PageSize      int32                  `json:"page_size,omitempty"`
//	PageNum       int32                  `json:"page_num,omitempty"`
//	OrderBys      []*entity.OrderBy      `json:"order_bys,omitempty"`
//	WithVersion   bool                   `json:"with_version,omitempty"`
//}
//
//type ListEvaluatorVersionRequest struct {
//	SpaceID       int64             `json:"space_id"`
//	EvaluatorID   int64             `json:"evaluator_id,omitempty"`
//	QueryVersions []string          `json:"query_versions,omitempty"`
//	PageSize      int32             `json:"page_size,omitempty"`
//	PageNum       int32             `json:"page_num,omitempty"`
//	OrderBys      []*entity.OrderBy `json:"order_bys,omitempty"`
//}
//
//type ListEvaluatorVersionResponse struct {
//	EvaluatorVersions []*entity.Evaluator `json:"evaluator_versions,omitempty"`
//	Total             int64               `json:"total,omitempty"`
//}
//
//type RunEvaluatorRequest struct {
//	SpaceID            int64                      `json:"space_id"`
//	Name               string                     `json:"name"`
//	EvaluatorVersionID int64                      `json:"evaluator_version_id"`
//	InputData          *entity.EvaluatorInputData `json:"input_data"`
//	ExperimentID       int64                      `json:"experiment_id,omitempty"`
//	ExperimentRunID    int64                      `json:"experiment_run_id,omitempty"`
//	ItemID             int64                      `json:"item_id,omitempty"`
//	TurnID             int64                      `json:"turn_id,omitempty"`
//	Ext                map[string]string          `json:"ext,omitempty"`
//}
