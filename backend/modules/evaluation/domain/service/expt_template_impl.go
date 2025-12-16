// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func NewExptTemplateManager(
	templateRepo repo.IExptTemplateRepo,
	idgen idgen.IIDGenerator,
	evaluatorService EvaluatorService,
	evalTargetService IEvalTargetService,
) IExptTemplateManager {
	return &ExptTemplateManagerImpl{
		templateRepo:     templateRepo,
		idgen:            idgen,
		evaluatorService: evaluatorService,
		evalTargetService: evalTargetService,
	}
}

type ExptTemplateManagerImpl struct {
	templateRepo     repo.IExptTemplateRepo
	idgen            idgen.IIDGenerator
	evaluatorService EvaluatorService
	evalTargetService IEvalTargetService
}

func (e *ExptTemplateManagerImpl) CheckName(ctx context.Context, name string, spaceID int64, session *entity.Session) (bool, error) {
	_, exists, err := e.templateRepo.GetByName(ctx, name, spaceID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

func (e *ExptTemplateManagerImpl) Create(ctx context.Context, param *entity.CreateExptTemplateParam, session *entity.Session) (*entity.ExptTemplate, error) {
	// 验证名称
	pass, err := e.CheckName(ctx, param.Name, param.SpaceID, session)
	if !pass {
		return nil, errorx.NewByCode(errno.ExperimentNameExistedCode, errorx.WithExtraMsg(fmt.Sprintf("template name %s already exists", param.Name)))
	}
	if err != nil {
		return nil, err
	}

	// 验证模板配置
	if param.TemplateConf != nil {
		if err := param.TemplateConf.Valid(ctx); err != nil {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(err.Error()))
		}
	}

	// 解析评估器版本ID，获取评估器ID
	evaluatorVersionRefs := make([]*entity.ExptTemplateEvaluatorVersionRef, 0, len(param.EvaluatorVersionIDs))
	if len(param.EvaluatorVersionIDs) > 0 {
		spaceIDPtr := &param.SpaceID
		evaluators, err := e.evaluatorService.BatchGetEvaluatorVersion(ctx, spaceIDPtr, param.EvaluatorVersionIDs, false)
		if err != nil {
			return nil, errorx.Wrapf(err, "get evaluators by version_ids fail")
		}

		evaluatorMap := make(map[int64]*entity.Evaluator)
		for _, ev := range evaluators {
			if ev != nil {
				// 使用EvaluatorVersion的ID作为key
				versionID := ev.GetEvaluatorVersionID()
				if versionID > 0 {
					evaluatorMap[versionID] = ev
				}
			}
		}

		for _, versionID := range param.EvaluatorVersionIDs {
			ev, ok := evaluatorMap[versionID]
			if !ok {
				return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("evaluator version %d not found", versionID)))
			}
			evaluatorVersionRefs = append(evaluatorVersionRefs, &entity.ExptTemplateEvaluatorVersionRef{
				EvaluatorID:        ev.ID,
				EvaluatorVersionID: versionID,
			})
		}
	}

	// 生成模板ID
	templateID, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, errorx.Wrapf(err, "gen template id fail")
	}

	// 获取target_type
	var targetType entity.EvalTargetType
	if param.TargetID > 0 {
		target, err := e.evalTargetService.GetEvalTarget(ctx, param.TargetID)
		if err != nil {
			return nil, errorx.Wrapf(err, "get eval target fail, target_id: %d", param.TargetID)
		}
		if target == nil {
			return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("target %d not found", param.TargetID)))
		}
		targetType = target.EvalTargetType
	}

	// 构建模板实体
	template := &entity.ExptTemplate{
		ID:                templateID,
		SpaceID:           param.SpaceID,
		CreatedBy:         session.UserID,
		Name:              param.Name,
		Description:       param.Description,
		EvalSetID:         param.EvalSetID,
		EvalSetVersionID:  param.EvalSetVersionID,
		TargetID:          param.TargetID,
		TargetType:        targetType,
		TargetVersionID:   param.TargetVersionID,
		EvaluatorVersionRef: evaluatorVersionRefs,
		TemplateConf:      param.TemplateConf,
		ExptType:          param.ExptType,
	}

	// 转换为评估器引用DO
	refs := template.ToEvaluatorRefDO()

	// 保存到数据库
	if err := e.templateRepo.Create(ctx, template, refs); err != nil {
		return nil, err
	}

	return template, nil
}

func (e *ExptTemplateManagerImpl) Get(ctx context.Context, templateID, spaceID int64, session *entity.Session) (*entity.ExptTemplate, error) {
	templates, err := e.MGet(ctx, []int64{templateID}, spaceID, session)
	if err != nil {
		return nil, err
	}

	if len(templates) == 0 {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("template %d not found", templateID)))
	}

	return templates[0], nil
}

func (e *ExptTemplateManagerImpl) MGet(ctx context.Context, templateIDs []int64, spaceID int64, session *entity.Session) ([]*entity.ExptTemplate, error) {
	return e.templateRepo.MGetByID(ctx, templateIDs, spaceID)
}

func (e *ExptTemplateManagerImpl) Update(ctx context.Context, template *entity.ExptTemplate, session *entity.Session) error {
	return e.templateRepo.Update(ctx, template)
}

func (e *ExptTemplateManagerImpl) Delete(ctx context.Context, templateID, spaceID int64, session *entity.Session) error {
	return e.templateRepo.Delete(ctx, templateID, spaceID)
}
