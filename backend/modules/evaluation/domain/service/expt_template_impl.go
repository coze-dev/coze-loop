// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/bytedance/gg/gptr"

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
	evaluationSetService IEvaluationSetService,
) IExptTemplateManager {
	return &ExptTemplateManagerImpl{
		templateRepo:          templateRepo,
		idgen:                 idgen,
		evaluatorService:      evaluatorService,
		evalTargetService:     evalTargetService,
		evaluationSetService:  evaluationSetService,
	}
}

type ExptTemplateManagerImpl struct {
	templateRepo          repo.IExptTemplateRepo
	idgen                 idgen.IIDGenerator
	evaluatorService      EvaluatorService
	evalTargetService     IEvalTargetService
	evaluationSetService  IEvaluationSetService
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

	// 处理创建评测对象参数
	var targetType entity.EvalTargetType
	var finalTargetID, finalTargetVersionID int64
	if param.CreateEvalTargetParam != nil && !param.CreateEvalTargetParam.IsNull() {
		// 如果提供了创建评测对象参数，则创建评测对象
		opts := make([]entity.Option, 0)
		opts = append(opts, entity.WithCozeBotPublishVersion(param.CreateEvalTargetParam.BotPublishVersion),
			entity.WithCozeBotInfoType(gptr.Indirect(param.CreateEvalTargetParam.BotInfoType)),
			entity.WithRegion(param.CreateEvalTargetParam.Region),
			entity.WithEnv(param.CreateEvalTargetParam.Env))
		if param.CreateEvalTargetParam.CustomEvalTarget != nil {
			opts = append(opts, entity.WithCustomEvalTarget(&entity.CustomEvalTarget{
				ID:        param.CreateEvalTargetParam.CustomEvalTarget.ID,
				Name:      param.CreateEvalTargetParam.CustomEvalTarget.Name,
				AvatarURL: param.CreateEvalTargetParam.CustomEvalTarget.AvatarURL,
				Ext:       param.CreateEvalTargetParam.CustomEvalTarget.Ext,
			}))
		}
		targetID, targetVersionID, err := e.evalTargetService.CreateEvalTarget(ctx, param.SpaceID, gptr.Indirect(param.CreateEvalTargetParam.SourceTargetID), gptr.Indirect(param.CreateEvalTargetParam.SourceTargetVersion), gptr.Indirect(param.CreateEvalTargetParam.EvalTargetType), opts...)
		if err != nil {
			return nil, errorx.Wrapf(err, "CreateEvalTarget failed, param: %v", param.CreateEvalTargetParam)
		}
		finalTargetID = targetID
		finalTargetVersionID = targetVersionID
		targetType = gptr.Indirect(param.CreateEvalTargetParam.EvalTargetType)
	} else if param.TargetID > 0 {
		// 如果提供了 target_id，则获取现有的评测对象
		target, err := e.evalTargetService.GetEvalTarget(ctx, param.TargetID)
		if err != nil {
			return nil, errorx.Wrapf(err, "get eval target fail, target_id: %d", param.TargetID)
		}
		if target == nil {
			return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("target %d not found", param.TargetID)))
		}
		finalTargetID = param.TargetID
		finalTargetVersionID = param.TargetVersionID
		targetType = target.EvalTargetType
	}

	// 构建模板实体
	template := &entity.ExptTemplate{
		ID:                 templateID,
		SpaceID:            param.SpaceID,
		CreatedBy:          session.UserID,
		Name:               param.Name,
		Description:        param.Description,
		EvalSetID:          param.EvalSetID,
		EvalSetVersionID:   param.EvalSetVersionID,
		TargetID:           finalTargetID,
		TargetType:         targetType,
		TargetVersionID:    finalTargetVersionID,
		EvaluatorVersionRef: evaluatorVersionRefs,
		TemplateConf:       param.TemplateConf,
		ExptType:            param.ExptType,
	}

	// 如果创建了评测对象，更新 TemplateConf 中的 TargetVersionID
	if param.CreateEvalTargetParam != nil && !param.CreateEvalTargetParam.IsNull() && template.TemplateConf != nil && template.TemplateConf.ConnectorConf.TargetConf != nil {
		template.TemplateConf.ConnectorConf.TargetConf.TargetVersionID = finalTargetVersionID
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

func (e *ExptTemplateManagerImpl) Update(ctx context.Context, param *entity.UpdateExptTemplateParam, session *entity.Session) (*entity.ExptTemplate, error) {
	// 获取现有模板
	existingTemplate, err := e.templateRepo.GetByID(ctx, param.TemplateID, param.SpaceID)
	if err != nil {
		return nil, err
	}
	if existingTemplate == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("template %d not found", param.TemplateID)))
	}

	// 如果名称改变，检查新名称是否可用
	if param.Name != "" && param.Name != existingTemplate.Name {
		pass, err := e.CheckName(ctx, param.Name, param.SpaceID, session)
		if !pass {
			return nil, errorx.NewByCode(errno.ExperimentNameExistedCode, errorx.WithExtraMsg(fmt.Sprintf("template name %s already exists", param.Name)))
		}
		if err != nil {
			return nil, err
		}
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

	// 处理创建评测对象参数（更新模板时）
	var finalTargetID, finalTargetVersionID int64
	finalTargetID = existingTemplate.TargetID // 默认保持原有 TargetID
	finalTargetVersionID = param.TargetVersionID
	if param.CreateEvalTargetParam != nil && !param.CreateEvalTargetParam.IsNull() {
		// 如果提供了创建评测对象参数，则创建新的评测对象
		// 注意：这会导致 TargetID 改变，但根据业务需求，更新模板时允许创建新的评测对象
		opts := make([]entity.Option, 0)
		opts = append(opts, entity.WithCozeBotPublishVersion(param.CreateEvalTargetParam.BotPublishVersion),
			entity.WithCozeBotInfoType(gptr.Indirect(param.CreateEvalTargetParam.BotInfoType)),
			entity.WithRegion(param.CreateEvalTargetParam.Region),
			entity.WithEnv(param.CreateEvalTargetParam.Env))
		if param.CreateEvalTargetParam.CustomEvalTarget != nil {
			opts = append(opts, entity.WithCustomEvalTarget(&entity.CustomEvalTarget{
				ID:        param.CreateEvalTargetParam.CustomEvalTarget.ID,
				Name:      param.CreateEvalTargetParam.CustomEvalTarget.Name,
				AvatarURL: param.CreateEvalTargetParam.CustomEvalTarget.AvatarURL,
				Ext:       param.CreateEvalTargetParam.CustomEvalTarget.Ext,
			}))
		}
		targetID, targetVersionID, err := e.evalTargetService.CreateEvalTarget(ctx, param.SpaceID, gptr.Indirect(param.CreateEvalTargetParam.SourceTargetID), gptr.Indirect(param.CreateEvalTargetParam.SourceTargetVersion), gptr.Indirect(param.CreateEvalTargetParam.EvalTargetType), opts...)
		if err != nil {
			return nil, errorx.Wrapf(err, "CreateEvalTarget failed, param: %v", param.CreateEvalTargetParam)
		}
		finalTargetID = targetID
		finalTargetVersionID = targetVersionID
	}

	// 构建更新后的模板实体
	updatedTemplate := &entity.ExptTemplate{
		ID:                 param.TemplateID,
		SpaceID:            param.SpaceID,
		CreatedBy:          existingTemplate.CreatedBy, // 保持原有创建者
		Name:               param.Name,
		Description:        param.Description,
		EvalSetID:          existingTemplate.EvalSetID, // 不允许修改
		EvalSetVersionID:   param.EvalSetVersionID,
		TargetID:           finalTargetID,
		TargetType:         existingTemplate.TargetType, // 如果创建了新评测对象，类型应该保持一致或从 CreateEvalTargetParam 获取
		TargetVersionID:    finalTargetVersionID,
		EvaluatorVersionRef: evaluatorVersionRefs,
		TemplateConf:       param.TemplateConf,
		ExptType:            param.ExptType,
	}

	// 如果创建了新的评测对象，更新 TargetType
	if param.CreateEvalTargetParam != nil && !param.CreateEvalTargetParam.IsNull() {
		updatedTemplate.TargetType = gptr.Indirect(param.CreateEvalTargetParam.EvalTargetType)
		// 更新 TemplateConf 中的 TargetVersionID
		if updatedTemplate.TemplateConf != nil && updatedTemplate.TemplateConf.ConnectorConf.TargetConf != nil {
			updatedTemplate.TemplateConf.ConnectorConf.TargetConf.TargetVersionID = finalTargetVersionID
		}
	}

	// 如果某些字段为空，保持原有值
	if updatedTemplate.Name == "" {
		updatedTemplate.Name = existingTemplate.Name
	}
	if updatedTemplate.Description == "" {
		updatedTemplate.Description = existingTemplate.Description
	}
	if updatedTemplate.EvalSetVersionID == 0 {
		updatedTemplate.EvalSetVersionID = existingTemplate.EvalSetVersionID
	}
	if updatedTemplate.TargetVersionID == 0 {
		updatedTemplate.TargetVersionID = existingTemplate.TargetVersionID
	}
	if updatedTemplate.ExptType == 0 {
		updatedTemplate.ExptType = existingTemplate.ExptType
	}
	if updatedTemplate.TemplateConf == nil {
		updatedTemplate.TemplateConf = existingTemplate.TemplateConf
	}

	// 转换为评估器引用DO
	refs := updatedTemplate.ToEvaluatorRefDO()

	// 更新数据库
	if err := e.templateRepo.UpdateWithRefs(ctx, updatedTemplate, refs); err != nil {
		return nil, err
	}

	// 重新获取更新后的模板
	return e.templateRepo.GetByID(ctx, param.TemplateID, param.SpaceID)
}

func (e *ExptTemplateManagerImpl) Delete(ctx context.Context, templateID, spaceID int64, session *entity.Session) error {
	return e.templateRepo.Delete(ctx, templateID, spaceID)
}

func (e *ExptTemplateManagerImpl) List(ctx context.Context, page, pageSize int32, spaceID int64, filter *entity.ExptTemplateListFilter, orderBys []*entity.OrderBy, session *entity.Session) ([]*entity.ExptTemplate, int64, error) {
	templates, count, err := e.templateRepo.List(ctx, page, pageSize, filter, orderBys, spaceID)
	if err != nil {
		return nil, 0, err
	}

	if len(templates) == 0 {
		return templates, count, nil
	}

	// 填充关联数据（类似 ListExperiments 的处理方式）
	// 收集需要查询的ID
	var (
		evalSetIDs      []int64
		targetIDs       []int64
		evaluatorIDs    []int64
		evaluatorIDMap  = make(map[int64]bool)
	)

	for _, template := range templates {
		if template.EvalSetID > 0 {
			evalSetIDs = append(evalSetIDs, template.EvalSetID)
		}
		if template.TargetID > 0 {
			targetIDs = append(targetIDs, template.TargetID)
		}
		for _, ref := range template.EvaluatorVersionRef {
			if ref.EvaluatorID > 0 && !evaluatorIDMap[ref.EvaluatorID] {
				evaluatorIDs = append(evaluatorIDs, ref.EvaluatorID)
				evaluatorIDMap[ref.EvaluatorID] = true
			}
		}
	}

	// 并发查询关联数据
	type result struct {
		evalSets   map[int64]*entity.EvaluationSet
		targets    map[int64]*entity.EvalTarget
		evaluators map[int64]*entity.Evaluator
		err        error
	}

	resultChan := make(chan result, 1)
	go func() {
		var res result
		res.evalSets = make(map[int64]*entity.EvaluationSet)
		res.targets = make(map[int64]*entity.EvalTarget)
		res.evaluators = make(map[int64]*entity.Evaluator)

		// 查询评测集
		if len(evalSetIDs) > 0 {
			spaceIDPtr := &spaceID
			evalSets, err := e.evaluationSetService.BatchGetEvaluationSets(ctx, spaceIDPtr, evalSetIDs, nil)
			if err != nil {
				res.err = err
				resultChan <- res
				return
			}
			for _, es := range evalSets {
				res.evalSets[es.ID] = es
			}
		}

		// 查询评估对象
		if len(targetIDs) > 0 {
			for _, targetID := range targetIDs {
				target, err := e.evalTargetService.GetEvalTarget(ctx, targetID)
				if err != nil {
					res.err = err
					resultChan <- res
					return
				}
				if target != nil {
					res.targets[target.ID] = target
				}
			}
		}

		// 查询评估器
		if len(evaluatorIDs) > 0 {
			evaluators, err := e.evaluatorService.BatchGetEvaluator(ctx, spaceID, evaluatorIDs, false)
			if err != nil {
				res.err = err
				resultChan <- res
				return
			}
			for _, ev := range evaluators {
				res.evaluators[ev.ID] = ev
			}
		}

		resultChan <- res
	}()

	res := <-resultChan
	if res.err != nil {
		return nil, 0, res.err
	}

	// 填充关联数据
	for _, template := range templates {
		if template.EvalSetID > 0 {
			if es, ok := res.evalSets[template.EvalSetID]; ok {
				template.EvalSet = es
			}
		}
		if template.TargetID > 0 {
			if t, ok := res.targets[template.TargetID]; ok {
				template.Target = t
			}
		}
		if len(template.EvaluatorVersionRef) > 0 {
			template.Evaluators = make([]*entity.Evaluator, 0, len(template.EvaluatorVersionRef))
			for _, ref := range template.EvaluatorVersionRef {
				if ev, ok := res.evaluators[ref.EvaluatorID]; ok {
					template.Evaluators = append(template.Evaluators, ev)
				}
			}
		}
	}

	return templates, count, nil
}
