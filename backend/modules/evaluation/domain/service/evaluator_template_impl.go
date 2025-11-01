// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// EvaluatorTemplateServiceImpl 实现 EvaluatorTemplateService 接口
type EvaluatorTemplateServiceImpl struct {
	templateRepo repo.EvaluatorTemplateRepo
}

// NewEvaluatorTemplateService 创建 EvaluatorTemplateService 实例
func NewEvaluatorTemplateService(templateRepo repo.EvaluatorTemplateRepo) EvaluatorTemplateService {
	return &EvaluatorTemplateServiceImpl{
		templateRepo: templateRepo,
	}
}

// CreateEvaluatorTemplate 创建评估器模板
func (s *EvaluatorTemplateServiceImpl) CreateEvaluatorTemplate(ctx context.Context, req *entity.CreateEvaluatorTemplateRequest) (*entity.CreateEvaluatorTemplateResponse, error) {
	// 参数验证
	if err := s.validateCreateRequest(req); err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	// 从context获取用户ID
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	// 构建模板实体
	template := &entity.EvaluatorTemplate{
		SpaceID:                req.SpaceID,
		Name:                   req.Name,
		Description:            req.Description,
		EvaluatorType:          req.EvaluatorType,
		Benchmark:              req.Benchmark,
		Vendor:                 req.Vendor,
		InputSchemas:           req.InputSchemas,
		OutputSchemas:          req.OutputSchemas,
		ReceiveChatHistory:     req.ReceiveChatHistory,
		Tags:                   req.Tags,
		PromptEvaluatorContent: req.PromptEvaluatorContent,
		CodeEvaluatorContent:   req.CodeEvaluatorContent,
	}

	// 设置基础信息
	baseInfo := &entity.BaseInfo{
		CreatedBy: &entity.UserInfo{
			UserID: &userID,
		},
		UpdatedBy: &entity.UserInfo{
			UserID: &userID,
		},
	}
	template.SetBaseInfo(baseInfo)

	// 调用repo层创建
	createdTemplate, err := s.templateRepo.CreateEvaluatorTemplate(ctx, template)
	if err != nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}

	return &entity.CreateEvaluatorTemplateResponse{
		Template: createdTemplate,
	}, nil
}

// UpdateEvaluatorTemplate 更新评估器模板
func (s *EvaluatorTemplateServiceImpl) UpdateEvaluatorTemplate(ctx context.Context, req *entity.UpdateEvaluatorTemplateRequest) (*entity.UpdateEvaluatorTemplateResponse, error) {
	// 参数验证
	if err := s.validateUpdateRequest(req); err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	// 从context获取用户ID
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	// 先获取现有模板
	existingTemplate, err := s.templateRepo.GetEvaluatorTemplate(ctx, req.ID, false)
	if err != nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	if existingTemplate == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
	}

	// 更新字段
	if req.Name != nil {
		existingTemplate.Name = *req.Name
	}
	if req.Description != nil {
		existingTemplate.Description = *req.Description
	}
	if req.Benchmark != nil {
		existingTemplate.Benchmark = *req.Benchmark
	}
	if req.Vendor != nil {
		existingTemplate.Vendor = *req.Vendor
	}
	if req.InputSchemas != nil {
		existingTemplate.InputSchemas = req.InputSchemas
	}
	if req.OutputSchemas != nil {
		existingTemplate.OutputSchemas = req.OutputSchemas
	}
	if req.ReceiveChatHistory != nil {
		existingTemplate.ReceiveChatHistory = req.ReceiveChatHistory
	}
	if req.Tags != nil {
		existingTemplate.Tags = req.Tags
	}
	if req.PromptEvaluatorContent != nil {
		existingTemplate.PromptEvaluatorContent = req.PromptEvaluatorContent
	}
	if req.CodeEvaluatorContent != nil {
		existingTemplate.CodeEvaluatorContent = req.CodeEvaluatorContent
	}

	// 更新基础信息
	if existingTemplate.BaseInfo != nil {
		existingTemplate.BaseInfo.UpdatedBy = &entity.UserInfo{
			UserID: &userID,
		}
	}

	// 调用repo层更新
	updatedTemplate, err := s.templateRepo.UpdateEvaluatorTemplate(ctx, existingTemplate)
	if err != nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}

	return &entity.UpdateEvaluatorTemplateResponse{
		Template: updatedTemplate,
	}, nil
}

// DeleteEvaluatorTemplate 删除评估器模板
func (s *EvaluatorTemplateServiceImpl) DeleteEvaluatorTemplate(ctx context.Context, req *entity.DeleteEvaluatorTemplateRequest) (*entity.DeleteEvaluatorTemplateResponse, error) {
	// 参数验证
	if err := s.validateDeleteRequest(req); err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	// 从context获取用户ID
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	// 检查模板是否存在
	existingTemplate, err := s.templateRepo.GetEvaluatorTemplate(ctx, req.ID, false)
	if err != nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	if existingTemplate == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
	}

	// 调用repo层删除
	err = s.templateRepo.DeleteEvaluatorTemplate(ctx, req.ID, userID)
	if err != nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}

	return &entity.DeleteEvaluatorTemplateResponse{
		Success: true,
	}, nil
}

// GetEvaluatorTemplate 获取评估器模板详情
func (s *EvaluatorTemplateServiceImpl) GetEvaluatorTemplate(ctx context.Context, req *entity.GetEvaluatorTemplateRequest) (*entity.GetEvaluatorTemplateResponse, error) {
	// 参数验证
	if err := s.validateGetRequest(req); err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	// 调用repo层获取
	template, err := s.templateRepo.GetEvaluatorTemplate(ctx, req.ID, req.IncludeDeleted)
	if err != nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}
	if template == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
	}

	return &entity.GetEvaluatorTemplateResponse{
		Template: template,
	}, nil
}

// ListEvaluatorTemplate 查询评估器模板列表
func (s *EvaluatorTemplateServiceImpl) ListEvaluatorTemplate(ctx context.Context, req *entity.ListEvaluatorTemplateRequest) (*entity.ListEvaluatorTemplateResponse, error) {
	// 参数验证
	if err := s.validateListRequest(req); err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	// 构建repo层请求
	repoReq := &repo.ListEvaluatorTemplateRequest{
		SpaceID:        req.SpaceID,
		FilterOption:   req.FilterOption,
		PageSize:       req.PageSize,
		PageNum:        req.PageNum,
		IncludeDeleted: req.IncludeDeleted,
	}

	// 调用repo层查询
	repoResp, err := s.templateRepo.ListEvaluatorTemplate(ctx, repoReq)
	if err != nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode)
	}

	// 计算总页数
	totalPages := int32((repoResp.TotalCount + int64(req.PageSize) - 1) / int64(req.PageSize))

	return &entity.ListEvaluatorTemplateResponse{
		TotalCount: repoResp.TotalCount,
		Templates:  repoResp.Templates,
		PageSize:   req.PageSize,
		PageNum:    req.PageNum,
		TotalPages: totalPages,
	}, nil
}

// validateCreateRequest 验证创建请求
func (s *EvaluatorTemplateServiceImpl) validateCreateRequest(req *entity.CreateEvaluatorTemplateRequest) error {
	if req.SpaceID <= 0 {
		return errors.New("空间ID必须大于0")
	}
	if req.Name == "" {
		return errors.New("模板名称不能为空")
	}
	if len(req.Name) > 100 {
		return errors.New("模板名称长度不能超过100个字符")
	}
	if len(req.Description) > 500 {
		return errors.New("模板描述长度不能超过500个字符")
	}
	if req.Benchmark != "" && len(req.Benchmark) > 100 {
		return errors.New("基准长度不能超过100个字符")
	}
	if req.Vendor != "" && len(req.Vendor) > 100 {
		return errors.New("供应商长度不能超过100个字符")
	}

	// 验证评估器类型和内容匹配
	if req.EvaluatorType == entity.EvaluatorTypePrompt && req.PromptEvaluatorContent == nil {
		return errors.New("Prompt类型评估器必须提供PromptEvaluatorContent")
	}
	if req.EvaluatorType == entity.EvaluatorTypeCode && req.CodeEvaluatorContent == nil {
		return errors.New("Code类型评估器必须提供CodeEvaluatorContent")
	}

	return nil
}

// validateUpdateRequest 验证更新请求
func (s *EvaluatorTemplateServiceImpl) validateUpdateRequest(req *entity.UpdateEvaluatorTemplateRequest) error {
	if req.ID <= 0 {
		return errors.New("模板ID必须大于0")
	}
	if req.Name != nil && *req.Name == "" {
		return errors.New("模板名称不能为空")
	}
	if req.Name != nil && len(*req.Name) > 100 {
		return errors.New("模板名称长度不能超过100个字符")
	}
	if req.Description != nil && len(*req.Description) > 500 {
		return errors.New("模板描述长度不能超过500个字符")
	}
	if req.Benchmark != nil && len(*req.Benchmark) > 100 {
		return errors.New("基准长度不能超过100个字符")
	}
	if req.Vendor != nil && len(*req.Vendor) > 100 {
		return errors.New("供应商长度不能超过100个字符")
	}

	return nil
}

// validateDeleteRequest 验证删除请求
func (s *EvaluatorTemplateServiceImpl) validateDeleteRequest(req *entity.DeleteEvaluatorTemplateRequest) error {
	if req.ID <= 0 {
		return errors.New("模板ID必须大于0")
	}
	return nil
}

// validateGetRequest 验证获取请求
func (s *EvaluatorTemplateServiceImpl) validateGetRequest(req *entity.GetEvaluatorTemplateRequest) error {
	if req.ID <= 0 {
		return errors.New("模板ID必须大于0")
	}
	return nil
}

// validateListRequest 验证列表请求
func (s *EvaluatorTemplateServiceImpl) validateListRequest(req *entity.ListEvaluatorTemplateRequest) error {
	if req.PageSize <= 0 || req.PageSize > 100 {
		return errors.New("分页大小必须在1-100之间")
	}
	if req.PageNum <= 0 {
		return errors.New("页码必须大于0")
	}
	return nil
}
