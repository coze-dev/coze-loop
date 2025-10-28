// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/convertor"
)

// EvaluatorTemplateRepoImpl 实现 EvaluatorTemplateRepo 接口
type EvaluatorTemplateRepoImpl struct {
	tagDAO      mysql.EvaluatorTagDAO
	templateDAO mysql.EvaluatorTemplateDAO
}

// NewEvaluatorTemplateRepo 创建 EvaluatorTemplateRepoImpl 实例
func NewEvaluatorTemplateRepo(tagDAO mysql.EvaluatorTagDAO, templateDAO mysql.EvaluatorTemplateDAO) repo.EvaluatorTemplateRepo {
	return &EvaluatorTemplateRepoImpl{
		tagDAO:      tagDAO,
		templateDAO: templateDAO,
	}
}

// ListEvaluatorTemplate 根据筛选条件查询evaluator_template列表，支持tag筛选和分页
func (r *EvaluatorTemplateRepoImpl) ListEvaluatorTemplate(ctx context.Context, req *repo.ListEvaluatorTemplateRequest) (*repo.ListEvaluatorTemplateResponse, error) {
	templateIDs := []int64{}
	var err error

	// 处理筛选条件
	if req.FilterOption != nil {
		// 检查是否有有效的筛选条件
		hasValidFilters := false

		// 检查SearchKeyword是否有效
		if req.FilterOption.SearchKeyword != nil && *req.FilterOption.SearchKeyword != "" {
			hasValidFilters = true
		}

		// 检查FilterConditions是否有效
		if req.FilterOption.Filters != nil && len(req.FilterOption.Filters.FilterConditions) > 0 {
			hasValidFilters = true
		}

		// 如果有有效的筛选条件，进行标签查询
		if hasValidFilters {
			// 使用EvaluatorTagDAO查询符合条件的template IDs（不分页）
			filteredIDs, _, err := r.tagDAO.GetSourceIDsByFilterConditions(ctx, int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), req.FilterOption, 0, 0)
			if err != nil {
				return nil, err
			}

			if len(filteredIDs) == 0 {
				return &repo.ListEvaluatorTemplateResponse{
					TotalCount: 0,
					Templates:  []*entity.EvaluatorTemplate{},
				}, nil
			}

			// 使用筛选后的IDs
			templateIDs = filteredIDs
		}
	}

	// 构建DAO层查询请求
	daoReq := &mysql.ListEvaluatorTemplateRequest{
		IDs:            templateIDs,
		PageSize:       req.PageSize,
		PageNum:        req.PageNum,
		IncludeDeleted: req.IncludeDeleted,
	}

	// 调用DAO层查询
	daoResp, err := r.templateDAO.ListEvaluatorTemplate(ctx, daoReq)
	if err != nil {
		return nil, err
	}

	// 转换响应格式
	templates := make([]*entity.EvaluatorTemplate, 0, len(daoResp.Templates))
	for _, templatePO := range daoResp.Templates {
		templateDO, err := convertor.ConvertEvaluatorTemplatePO2DOWithBaseInfo(templatePO)
		if err != nil {
			return nil, err
		}
		templates = append(templates, templateDO)
	}

	return &repo.ListEvaluatorTemplateResponse{
		TotalCount: daoResp.TotalCount,
		Templates:  templates,
	}, nil
}

// CreateEvaluatorTemplate 创建评估器模板
func (r *EvaluatorTemplateRepoImpl) CreateEvaluatorTemplate(ctx context.Context, template *entity.EvaluatorTemplate) (*entity.EvaluatorTemplate, error) {
	if template == nil {
		return nil, errors.New("template cannot be nil")
	}

	// 转换DO到PO
	templatePO, err := convertor.ConvertEvaluatorTemplateDO2PO(template)
	if err != nil {
		return nil, err
	}

	// 调用DAO层创建
	createdPO, err := r.templateDAO.CreateEvaluatorTemplate(ctx, templatePO)
	if err != nil {
		return nil, err
	}

	// 转换PO到DO
	createdDO, err := convertor.ConvertEvaluatorTemplatePO2DOWithBaseInfo(createdPO)
	if err != nil {
		return nil, err
	}

	return createdDO, nil
}

// UpdateEvaluatorTemplate 更新评估器模板
func (r *EvaluatorTemplateRepoImpl) UpdateEvaluatorTemplate(ctx context.Context, template *entity.EvaluatorTemplate) (*entity.EvaluatorTemplate, error) {
	if template == nil {
		return nil, errors.New("template cannot be nil")
	}

	// 转换DO到PO
	templatePO, err := convertor.ConvertEvaluatorTemplateDO2PO(template)
	if err != nil {
		return nil, err
	}

	// 调用DAO层更新
	updatedPO, err := r.templateDAO.UpdateEvaluatorTemplate(ctx, templatePO)
	if err != nil {
		return nil, err
	}

	// 转换PO到DO
	updatedDO, err := convertor.ConvertEvaluatorTemplatePO2DOWithBaseInfo(updatedPO)
	if err != nil {
		return nil, err
	}

	return updatedDO, nil
}

// DeleteEvaluatorTemplate 删除评估器模板（软删除）
func (r *EvaluatorTemplateRepoImpl) DeleteEvaluatorTemplate(ctx context.Context, id int64, userID string) error {
	return r.templateDAO.DeleteEvaluatorTemplate(ctx, id, userID)
}

// GetEvaluatorTemplate 根据ID获取评估器模板
func (r *EvaluatorTemplateRepoImpl) GetEvaluatorTemplate(ctx context.Context, id int64, includeDeleted bool) (*entity.EvaluatorTemplate, error) {
	// 调用DAO层查询
	templatePO, err := r.templateDAO.GetEvaluatorTemplate(ctx, id, includeDeleted)
	if err != nil {
		return nil, err
	}

	if templatePO == nil {
		return nil, nil
	}

	// 转换PO到DO
	templateDO, err := convertor.ConvertEvaluatorTemplatePO2DOWithBaseInfo(templatePO)
	if err != nil {
		return nil, err
	}

	return templateDO, nil
}
