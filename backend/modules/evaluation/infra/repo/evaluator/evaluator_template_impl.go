// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/contexts"
)

// EvaluatorTemplateRepoImpl 实现 EvaluatorTemplateRepo 接口
type EvaluatorTemplateRepoImpl struct {
	tagDAO      mysql.EvaluatorTagDAO
	templateDAO mysql.EvaluatorTemplateDAO
	idgen       idgen.IIDGenerator
}

// NewEvaluatorTemplateRepo 创建 EvaluatorTemplateRepoImpl 实例
func NewEvaluatorTemplateRepo(tagDAO mysql.EvaluatorTagDAO, templateDAO mysql.EvaluatorTemplateDAO, idgen idgen.IIDGenerator) repo.EvaluatorTemplateRepo {
	return &EvaluatorTemplateRepoImpl{
		tagDAO:      tagDAO,
		templateDAO: templateDAO,
		idgen:       idgen,
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

		// 检查FilterConditions或SubFilters是否有效
		if req.FilterOption.Filters != nil {
			if len(req.FilterOption.Filters.FilterConditions) > 0 {
				hasValidFilters = true
			}
			if len(req.FilterOption.Filters.SubFilters) > 0 {
				hasValidFilters = true
			}
		}

		// 如果有有效的筛选条件，进行标签查询
		if hasValidFilters {
			// 使用EvaluatorTagDAO查询符合条件的template IDs（不分页）
			filteredIDs, _, err := r.tagDAO.GetSourceIDsByFilterConditions(ctx, int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), req.FilterOption, 0, 0, contexts.CtxLocale(ctx))
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
	ids := make([]int64, 0, len(daoResp.Templates))
	for _, templatePO := range daoResp.Templates {
		templateDO, err := convertor.ConvertEvaluatorTemplatePO2DOWithBaseInfo(templatePO)
		if err != nil {
			return nil, err
		}
		templates = append(templates, templateDO)
		ids = append(ids, templatePO.ID)
	}

	// 批量查询并填充标签（以模板ID为source_id）
	if len(ids) > 0 {
		allTags, tagErr := r.tagDAO.BatchGetTagsBySourceIDsAndType(ctx, ids, int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), contexts.CtxLocale(ctx))
		if tagErr == nil && len(allTags) > 0 {
			tagsBySourceID := make(map[int64][]*model.EvaluatorTag)
			for _, tag := range allTags {
				tagsBySourceID[tag.SourceID] = append(tagsBySourceID[tag.SourceID], tag)
			}
			for _, tpl := range templates {
				r.setTemplateTags(tpl, tpl.ID, tagsBySourceID)
			}
		}
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

	// 生成并填充主键ID（使用 idgen），避免依赖数据库自增
	ids, err := r.idgen.GenMultiIDs(ctx, 1)
	if err != nil {
		return nil, err
	}
	templatePO.ID = ids[0]

	// 调用DAO层创建
	createdPO, err := r.templateDAO.CreateEvaluatorTemplate(ctx, templatePO)
	if err != nil {
		return nil, err
	}

	// 若携带了标签，则为模板创建tags（以模板ID作为 source_id）
	if len(template.Tags) > 0 {
		userID := session.UserIDInCtxOrEmpty(ctx)
		// 统计总标签数（所有语言）
		total := 0
		for _, langMap := range template.Tags {
			for _, vals := range langMap {
				total += len(vals)
			}
		}
		if total > 0 {
			ids, err := r.idgen.GenMultiIDs(ctx, total)
			if err != nil {
				return nil, err
			}
			idx := 0
			evaluatorTags := make([]*model.EvaluatorTag, 0, total)
			for lang, langMap := range template.Tags {
				for tagKey, tagValues := range langMap {
					for _, tagValue := range tagValues {
						evaluatorTags = append(evaluatorTags, &model.EvaluatorTag{
							ID:        ids[idx],
							SourceID:  createdPO.ID,
							TagType:   int32(entity.EvaluatorTagKeyType_EvaluatorTemplate),
							TagKey:    string(tagKey),
							TagValue:  tagValue,
							LangType:  string(lang),
							CreatedBy: userID,
							UpdatedBy: userID,
						})
						idx++
					}
				}
			}
			if err := r.tagDAO.BatchCreateEvaluatorTags(ctx, evaluatorTags); err != nil {
				return nil, err
			}
		}
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

	// 标签全量对齐：新增补充、删除不在集合内的，保持未变化的不动
	// 此处 template 已在入参校验中判空，无需再判空
	// 针对每种语言分别全量对齐
	userID := session.UserIDInCtxOrEmpty(ctx)
	for lang, tagMap := range template.Tags {
		existingTags, err := r.tagDAO.BatchGetTagsBySourceIDsAndType(ctx, []int64{template.ID}, int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), string(lang))
		if err != nil {
			return nil, err
		}
		existing := make(map[string]map[string]bool)
		for _, t := range existingTags {
			if _, ok := existing[t.TagKey]; !ok {
				existing[t.TagKey] = make(map[string]bool)
			}
			existing[t.TagKey][t.TagValue] = true
		}
		target := make(map[string]map[string]bool)
		for k, vs := range tagMap {
			kstr := string(k)
			if _, ok := target[kstr]; !ok {
				target[kstr] = make(map[string]bool)
			}
			for _, v := range vs {
				target[kstr][v] = true
			}
		}
		del := make(map[string][]string)
		for k, vals := range existing {
			for v := range vals {
				if !target[k][v] {
					del[k] = append(del[k], v)
				}
			}
		}
		if len(del) > 0 {
			if err := r.tagDAO.DeleteEvaluatorTagsByConditions(ctx, template.ID, int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), string(lang), del); err != nil {
				return nil, err
			}
		}
		add := make(map[string][]string)
		for k, vals := range target {
			for v := range vals {
				if !existing[k][v] {
					add[k] = append(add[k], v)
				}
			}
		}
		if len(add) > 0 {
			total := 0
			for _, vs := range add {
				total += len(vs)
			}
			if total > 0 {
				ids, err := r.idgen.GenMultiIDs(ctx, total)
				if err != nil {
					return nil, err
				}
				idx := 0
				evaluatorTags := make([]*model.EvaluatorTag, 0, total)
				for k, vs := range add {
					for _, v := range vs {
						evaluatorTags = append(evaluatorTags, &model.EvaluatorTag{
							ID:        ids[idx],
							SourceID:  template.ID,
							TagType:   int32(entity.EvaluatorTagKeyType_EvaluatorTemplate),
							TagKey:    k,
							TagValue:  v,
							LangType:  string(lang),
							CreatedBy: userID,
							UpdatedBy: userID,
						})
						idx++
					}
				}
				if err := r.tagDAO.BatchCreateEvaluatorTags(ctx, evaluatorTags); err != nil {
					return nil, err
				}
			}
		}
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

	// 补充查询：回填模板标签（按当前语言）
	allTags, tagErr := r.tagDAO.BatchGetTagsBySourceIDsAndType(ctx, []int64{id}, int32(entity.EvaluatorTagKeyType_EvaluatorTemplate), contexts.CtxLocale(ctx))
	if tagErr == nil && len(allTags) > 0 {
		tagsBySourceID := map[int64][]*model.EvaluatorTag{id: allTags}
		r.setTemplateTags(templateDO, id, tagsBySourceID)
	}

	return templateDO, nil
}

// IncrPopularityByID 基于ID将 popularity + 1
func (r *EvaluatorTemplateRepoImpl) IncrPopularityByID(ctx context.Context, id int64) error {
	return r.templateDAO.IncrPopularityByID(ctx, id)
}

// setTemplateTags 将查询到的标签填充到模板DO（当前按单语言过滤后回填到对应语言key下）
func (r *EvaluatorTemplateRepoImpl) setTemplateTags(tpl *entity.EvaluatorTemplate, templateID int64, tagsBySourceID map[int64][]*model.EvaluatorTag) {
	if tags, exists := tagsBySourceID[templateID]; exists && len(tags) > 0 {
		tagMap := make(map[entity.EvaluatorTagKey][]string)
		for _, t := range tags {
			key := entity.EvaluatorTagKey(t.TagKey)
			if tagMap[key] == nil {
				tagMap[key] = make([]string, 0)
			}
			tagMap[key] = append(tagMap[key], t.TagValue)
		}
		if tpl.Tags == nil {
			tpl.Tags = make(map[entity.EvaluatorTagLangType]map[entity.EvaluatorTagKey][]string)
		}
		lang := entity.EvaluatorTagLangType(tags[0].LangType)
		tpl.Tags[lang] = tagMap
	}
}
