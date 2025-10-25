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
	CreateEvaluatorTemplate(ctx context.Context, req *CreateEvaluatorTemplateRequest) (*CreateEvaluatorTemplateResponse, error)

	// UpdateEvaluatorTemplate 更新评估器模板
	UpdateEvaluatorTemplate(ctx context.Context, req *UpdateEvaluatorTemplateRequest) (*UpdateEvaluatorTemplateResponse, error)

	// DeleteEvaluatorTemplate 删除评估器模板
	DeleteEvaluatorTemplate(ctx context.Context, req *DeleteEvaluatorTemplateRequest) (*DeleteEvaluatorTemplateResponse, error)

	// GetEvaluatorTemplate 获取评估器模板详情
	GetEvaluatorTemplate(ctx context.Context, req *GetEvaluatorTemplateRequest) (*GetEvaluatorTemplateResponse, error)

	// ListEvaluatorTemplate 查询评估器模板列表
	ListEvaluatorTemplate(ctx context.Context, req *ListEvaluatorTemplateRequest) (*ListEvaluatorTemplateResponse, error)
}

// CreateEvaluatorTemplateRequest 创建评估器模板请求
type CreateEvaluatorTemplateRequest struct {
	SpaceID        int64                         `json:"space_id" validate:"required,gt=0"`                    // 空间ID
	Name           string                        `json:"name" validate:"required,min=1,max=100"`              // 模板名称
	Description    string                        `json:"description" validate:"max=500"`                     // 模板描述
	EvaluatorType  entity.EvaluatorType         `json:"evaluator_type" validate:"required"`                  // 评估器类型
	Benchmark      string                        `json:"benchmark,omitempty" validate:"max=100"`               // 基准
	Vendor         string                        `json:"vendor,omitempty" validate:"max=100"`                 // 供应商
	InputSchemas   []*entity.ArgsSchema          `json:"input_schemas,omitempty"`                            // 输入模式
	OutputSchemas  []*entity.ArgsSchema          `json:"output_schemas,omitempty"`                           // 输出模式
	ReceiveChatHistory *bool                    `json:"receive_chat_history,omitempty"`                      // 是否接收聊天历史
	Tags           map[entity.EvaluatorTagKey][]string `json:"tags,omitempty"`                              // 标签

	// 评估器内容
	PromptEvaluatorContent *entity.PromptEvaluatorContent `json:"prompt_evaluator_content,omitempty"`        // Prompt评估器内容
	CodeEvaluatorContent   *entity.CodeEvaluatorContent   `json:"code_evaluator_content,omitempty"`          // Code评估器内容
}

// CreateEvaluatorTemplateResponse 创建评估器模板响应
type CreateEvaluatorTemplateResponse struct {
	Template *entity.EvaluatorTemplate `json:"template"` // 创建的模板
}

// UpdateEvaluatorTemplateRequest 更新评估器模板请求
type UpdateEvaluatorTemplateRequest struct {
	ID             int64                         `json:"id" validate:"required,gt=0"`                        // 模板ID
	Name           *string                       `json:"name,omitempty" validate:"omitempty,min=1,max=100"`  // 模板名称
	Description    *string                       `json:"description,omitempty" validate:"omitempty,max=500"`  // 模板描述
	Benchmark      *string                       `json:"benchmark,omitempty" validate:"omitempty,max=100"`   // 基准
	Vendor         *string                       `json:"vendor,omitempty" validate:"omitempty,max=100"`      // 供应商
	InputSchemas   []*entity.ArgsSchema          `json:"input_schemas,omitempty"`                            // 输入模式
	OutputSchemas  []*entity.ArgsSchema          `json:"output_schemas,omitempty"`                           // 输出模式
	ReceiveChatHistory *bool                    `json:"receive_chat_history,omitempty"`                      // 是否接收聊天历史
	Tags           map[entity.EvaluatorTagKey][]string `json:"tags,omitempty"`                              // 标签

	// 评估器内容
	PromptEvaluatorContent *entity.PromptEvaluatorContent `json:"prompt_evaluator_content,omitempty"`        // Prompt评估器内容
	CodeEvaluatorContent   *entity.CodeEvaluatorContent   `json:"code_evaluator_content,omitempty"`          // Code评估器内容
}

// UpdateEvaluatorTemplateResponse 更新评估器模板响应
type UpdateEvaluatorTemplateResponse struct {
	Template *entity.EvaluatorTemplate `json:"template"` // 更新后的模板
}

// DeleteEvaluatorTemplateRequest 删除评估器模板请求
type DeleteEvaluatorTemplateRequest struct {
	ID     int64  `json:"id" validate:"required,gt=0"`     // 模板ID
}

// DeleteEvaluatorTemplateResponse 删除评估器模板响应
type DeleteEvaluatorTemplateResponse struct {
	Success bool `json:"success"` // 删除是否成功
}

// GetEvaluatorTemplateRequest 获取评估器模板请求
type GetEvaluatorTemplateRequest struct {
	ID             int64 `json:"id" validate:"required,gt=0"`                // 模板ID
	IncludeDeleted bool  `json:"include_deleted,omitempty"`                  // 是否包含已删除记录
}

// GetEvaluatorTemplateResponse 获取评估器模板响应
type GetEvaluatorTemplateResponse struct {
	Template *entity.EvaluatorTemplate `json:"template"` // 模板详情
}

// ListEvaluatorTemplateRequest 查询评估器模板列表请求
type ListEvaluatorTemplateRequest struct {
	SpaceID        int64                         `json:"space_id" validate:"required,gt=0"`                // 空间ID
	FilterOption   *entity.EvaluatorFilterOption `json:"filter_option,omitempty"`                           // 标签筛选条件
	PageSize       int32                         `json:"page_size" validate:"required,min=1,max=100"`        // 分页大小
	PageNum        int32                         `json:"page_num" validate:"required,min=1"`                // 页码
	IncludeDeleted bool                          `json:"include_deleted,omitempty"`                        // 是否包含已删除记录
}

// ListEvaluatorTemplateResponse 查询评估器模板列表响应
type ListEvaluatorTemplateResponse struct {
	TotalCount int64                       `json:"total_count"` // 总数量
	Templates  []*entity.EvaluatorTemplate `json:"templates"`   // 模板列表
	PageSize   int32                       `json:"page_size"`   // 分页大小
	PageNum    int32                       `json:"page_num"`    // 页码
	TotalPages int32                       `json:"total_pages"` // 总页数
}
