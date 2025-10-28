// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// EvaluatorTemplateService 定义 EvaluatorTemplate 的 Service 接口
//
//go:generate mockgen -destination mocks/evaluator_template_service_mock.go -package=mocks . EvaluatorTemplateService
type EvaluatorTemplateService interface {
	// CreateEvaluatorTemplate 创建评估器模板
	CreateEvaluatorTemplate(ctx context.Context, req *entity.CreateEvaluatorTemplateRequest) (*entity.CreateEvaluatorTemplateResponse, error)

	// UpdateEvaluatorTemplate 更新评估器模板
	UpdateEvaluatorTemplate(ctx context.Context, req *entity.UpdateEvaluatorTemplateRequest) (*entity.UpdateEvaluatorTemplateResponse, error)

	// DeleteEvaluatorTemplate 删除评估器模板
	DeleteEvaluatorTemplate(ctx context.Context, req *entity.DeleteEvaluatorTemplateRequest) (*entity.DeleteEvaluatorTemplateResponse, error)

	// GetEvaluatorTemplate 获取评估器模板详情
	GetEvaluatorTemplate(ctx context.Context, req *entity.GetEvaluatorTemplateRequest) (*entity.GetEvaluatorTemplateResponse, error)

	// ListEvaluatorTemplate 查询评估器模板列表
	ListEvaluatorTemplate(ctx context.Context, req *entity.ListEvaluatorTemplateRequest) (*entity.ListEvaluatorTemplateResponse, error)
}

