// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// EvaluatorTemplateRepo 定义 EvaluatorTemplate 的 Repo 接口
//
//go:generate mockgen -destination mocks/evaluator_template_mock.go -package=mocks . EvaluatorTemplateRepo
type EvaluatorTemplateRepo interface {
	// CreateEvaluatorTemplate 创建评估器模板
	CreateEvaluatorTemplate(ctx context.Context, template *entity.EvaluatorTemplate) (*entity.EvaluatorTemplate, error)

	// UpdateEvaluatorTemplate 更新评估器模板
	UpdateEvaluatorTemplate(ctx context.Context, template *entity.EvaluatorTemplate) (*entity.EvaluatorTemplate, error)

	// DeleteEvaluatorTemplate 删除评估器模板（软删除）
	DeleteEvaluatorTemplate(ctx context.Context, id int64, userID string) error

	// GetEvaluatorTemplate 根据ID获取评估器模板
	GetEvaluatorTemplate(ctx context.Context, id int64, includeDeleted bool) (*entity.EvaluatorTemplate, error)

	// ListEvaluatorTemplate 根据筛选条件查询evaluator_template列表，支持tag筛选和分页
	ListEvaluatorTemplate(ctx context.Context, req *ListEvaluatorTemplateRequest) (*ListEvaluatorTemplateResponse, error)

	// IncrPopularityByID 基于ID将 popularity + 1
	IncrPopularityByID(ctx context.Context, id int64) error
}

// ListEvaluatorTemplateRequest 查询evaluator_template的请求参数
type ListEvaluatorTemplateRequest struct {
	SpaceID        int64                         `json:"space_id"`
	FilterOption   *entity.EvaluatorFilterOption `json:"filter_option,omitempty"`   // 标签筛选条件
	PageSize       int32                         `json:"page_size"`                 // 分页大小
	PageNum        int32                         `json:"page_num"`                  // 页码
	IncludeDeleted bool                          `json:"include_deleted,omitempty"` // 是否包含已删除记录
}

// ListEvaluatorTemplateResponse 查询evaluator_template的响应
type ListEvaluatorTemplateResponse struct {
	TotalCount int64                       `json:"total_count"` // 总数量
	Templates  []*entity.EvaluatorTemplate `json:"templates"`   // 模板列表
}
