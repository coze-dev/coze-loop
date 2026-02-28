// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/maps"
)

func NewExptTemplateManager(
	templateRepo repo.IExptTemplateRepo,
	idgen idgen.IIDGenerator,
	evaluatorService EvaluatorService,
	evalTargetService IEvalTargetService,
	evaluationSetService IEvaluationSetService,
	evaluationSetVersionService EvaluationSetVersionService,
	lwt platestwrite.ILatestWriteTracker,
) IExptTemplateManager {
	return &ExptTemplateManagerImpl{
		templateRepo:                templateRepo,
		idgen:                       idgen,
		evaluatorService:            evaluatorService,
		evalTargetService:           evalTargetService,
		evaluationSetService:        evaluationSetService,
		evaluationSetVersionService: evaluationSetVersionService,
		lwt:                         lwt,
	}
}

type ExptTemplateManagerImpl struct {
	templateRepo                repo.IExptTemplateRepo
	idgen                       idgen.IIDGenerator
	evaluatorService            EvaluatorService
	evalTargetService           IEvalTargetService
	evaluationSetService        IEvaluationSetService
	evaluationSetVersionService EvaluationSetVersionService
	lwt                         platestwrite.ILatestWriteTracker
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

	// 解析并回填评估器版本ID（如果缺失）
	// 注意：FieldMappingConfig 中的 EvaluatorFieldMapping 会在 buildFieldMappingConfigAndEnableScoreWeight 中从 TemplateConf 构建
	// 所以只需要回填 TemplateConf 中的 EvaluatorConf 即可
	if err := e.resolveAndFillEvaluatorVersionIDs(ctx, param.SpaceID, param.TemplateConf, param.EvaluatorIDVersionItems); err != nil {
		return nil, err
	}

	// 验证模板配置
	if param.TemplateConf != nil {
		if err := param.TemplateConf.Valid(ctx); err != nil {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(err.Error()))
		}
	}

	// 从 EvaluatorIDVersionItems 构建 evaluatorVersionRefs
	evaluatorVersionRefs := e.buildEvaluatorVersionRefs(param.EvaluatorIDVersionItems)

	// 生成模板ID
	templateID, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, errorx.Wrapf(err, "gen template id fail")
	}

	// 处理创建评测对象参数
	finalTargetID, finalTargetVersionID, targetType, err := e.resolveTargetForCreate(ctx, param)
	if err != nil {
		return nil, err
	}

	// 构建模板实体
	now := time.Now()
	template := &entity.ExptTemplate{
		Meta: &entity.ExptTemplateMeta{
			ID:          templateID,
			WorkspaceID: param.SpaceID,
			Name:        param.Name,
			Desc:        param.Description,
			ExptType:    param.ExptType,
		},
		TripleConfig: &entity.ExptTemplateTuple{
			EvalSetID:               param.EvalSetID,
			EvalSetVersionID:        param.EvalSetVersionID,
			TargetID:                finalTargetID,
			TargetVersionID:         finalTargetVersionID,
			TargetType:              targetType,
			EvaluatorVersionIds:     e.extractEvaluatorVersionIDs(param.EvaluatorIDVersionItems),
			EvaluatorIDVersionItems: param.EvaluatorIDVersionItems,
		},
		EvaluatorVersionRef: evaluatorVersionRefs,
		TemplateConf:        param.TemplateConf,
		BaseInfo: &entity.BaseInfo{
			CreatedAt: gptr.Of(now.UnixMilli()),
			UpdatedAt: gptr.Of(now.UnixMilli()),
			CreatedBy: &entity.UserInfo{UserID: gptr.Of(session.UserID)},
			UpdatedBy: &entity.UserInfo{UserID: gptr.Of(session.UserID)},
		},
	}

	// 从 TemplateConf 构建 FieldMappingConfig，并根据 EvaluatorConf.ScoreWeight 设置是否启用分数权重
	e.buildFieldMappingConfigAndEnableScoreWeight(template, param.TemplateConf)

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

	// 设置写标志，用于主从延迟兜底
	e.lwt.SetWriteFlag(ctx, platestwrite.ResourceTypeExptTemplate, templateID)

	// 填充关联数据（EvalSet、EvalTarget、Evaluators）
	// 如果创建了新的 EvalTarget，需要从主库读取以避免主从延迟
	queryCtx := ctx
	if param.CreateEvalTargetParam != nil && !param.CreateEvalTargetParam.IsNull() {
		queryCtx = contexts.WithCtxWriteDB(ctx)
	}
	tupleID := e.packTemplateTupleID(template)
	exptTuples, err := e.mgetExptTupleByID(queryCtx, []*entity.ExptTupleID{tupleID}, param.SpaceID, session)
	if err != nil {
		return nil, err
	}
	if len(exptTuples) > 0 {
		template.EvalSet = exptTuples[0].EvalSet
		template.Target = exptTuples[0].Target
		template.Evaluators = exptTuples[0].Evaluators
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
	// 参考 ExptMangerImpl.MGet 的方式，如果只有一个模板ID且有写标志，则从主库读取
	if len(templateIDs) == 1 && e.lwt.CheckWriteFlagByID(ctx, platestwrite.ResourceTypeExptTemplate, templateIDs[0]) {
		ctx = contexts.WithCtxWriteDB(ctx)
	}

	templates, err := e.templateRepo.MGetByID(ctx, templateIDs, spaceID)
	if err != nil {
		return nil, err
	}

	if len(templates) == 0 {
		return templates, nil
	}

	// 构建 ExptTupleID 列表，用于批量查询关联数据
	tupleIDs := make([]*entity.ExptTupleID, 0, len(templates))
	for _, template := range templates {
		tupleIDs = append(tupleIDs, e.packTemplateTupleID(template))
	}

	// 批量查询关联数据
	exptTuples, err := e.mgetExptTupleByID(ctx, tupleIDs, spaceID, session)
	if err != nil {
		return nil, err
	}

	// 填充关联数据
	for idx := range exptTuples {
		templates[idx].EvalSet = exptTuples[idx].EvalSet
		templates[idx].Target = exptTuples[idx].Target
		templates[idx].Evaluators = exptTuples[idx].Evaluators
	}

	return templates, nil
}

func (e *ExptTemplateManagerImpl) Update(ctx context.Context, param *entity.UpdateExptTemplateParam, session *entity.Session) (*entity.ExptTemplate, error) {
	// 获取现有模板
	existingTemplate, err := e.templateRepo.GetByID(ctx, param.TemplateID, &param.SpaceID)
	if err != nil {
		return nil, err
	}
	if existingTemplate == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("template %d not found", param.TemplateID)))
	}

	// 如果名称改变，检查新名称是否可用（允许和当前名称重复）
	if param.Name != "" && param.Name != existingTemplate.GetName() {
		pass, err := e.CheckName(ctx, param.Name, param.SpaceID, session)
		if !pass {
			return nil, errorx.NewByCode(errno.ExperimentNameExistedCode, errorx.WithExtraMsg(fmt.Sprintf("template name %s already exists", param.Name)))
		}
		if err != nil {
			return nil, err
		}
	}

	// 解析并回填评估器版本ID（如果缺失），保持与 Create 一致的行为
	// 注意：FieldMappingConfig 中的 EvaluatorFieldMapping 会在 buildFieldMappingConfigAndEnableScoreWeight 中从 TemplateConf 构建
	// 所以只需要回填 TemplateConf 中的 EvaluatorConf 即可
	if err := e.resolveAndFillEvaluatorVersionIDs(ctx, param.SpaceID, param.TemplateConf, param.EvaluatorIDVersionItems); err != nil {
		return nil, err
	}

	// 验证模板配置
	if param.TemplateConf != nil {
		if err := param.TemplateConf.Valid(ctx); err != nil {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(err.Error()))
		}
	}

	// 从 EvaluatorIDVersionItems 构建 evaluatorVersionRefs
	evaluatorVersionRefs := e.buildEvaluatorVersionRefs(param.EvaluatorIDVersionItems)

	// 处理创建评测对象参数（需要校验 SourceTargetID 与现有 Target 的 SourceTargetID 一致）
	var finalTargetID, finalTargetVersionID int64
	var targetType entity.EvalTargetType
	if param.CreateEvalTargetParam != nil && !param.CreateEvalTargetParam.IsNull() {
		// 获取现有的 Target 以校验 SourceTargetID
		existingTargetID := existingTemplate.GetTargetID()
		existingTarget, err := e.evalTargetService.GetEvalTarget(ctx, existingTargetID)
		if err != nil {
			return nil, errorx.Wrapf(err, "get existing eval target fail, target_id: %d", existingTargetID)
		}
		if existingTarget == nil {
			return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("existing target %d not found", existingTargetID)))
		}
		// 校验 SourceTargetID 必须与现有的 Target 的 SourceTargetID 一致
		sourceTargetID := gptr.Indirect(param.CreateEvalTargetParam.SourceTargetID)
		if sourceTargetID != existingTarget.SourceTargetID {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(fmt.Sprintf("SourceTargetID %s must match existing Target SourceTargetID %s", sourceTargetID, existingTarget.SourceTargetID)))
		}
		// 创建新的评测对象版本
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
		targetID, targetVersionID, err := e.evalTargetService.CreateEvalTarget(ctx, param.SpaceID, sourceTargetID, gptr.Indirect(param.CreateEvalTargetParam.SourceTargetVersion), gptr.Indirect(param.CreateEvalTargetParam.EvalTargetType), opts...)
		if err != nil {
			return nil, errorx.Wrapf(err, "CreateEvalTarget failed, param: %v", param.CreateEvalTargetParam)
		}
		finalTargetID = targetID
		finalTargetVersionID = targetVersionID
		targetType = gptr.Indirect(param.CreateEvalTargetParam.EvalTargetType)
	} else {
		// 保持原有 TargetID，不允许修改
		finalTargetID = existingTemplate.GetTargetID()
		finalTargetVersionID = param.TargetVersionID
		if finalTargetVersionID == 0 {
			finalTargetVersionID = existingTemplate.GetTargetVersionID()
		}
		targetType = existingTemplate.GetTargetType()
	}

	// 准备更新后的 Meta
	updatedMeta := &entity.ExptTemplateMeta{
		ID:          existingTemplate.GetID(),
		WorkspaceID: param.SpaceID,
		Name:        param.Name,
		Desc:        param.Description,
		ExptType:    param.ExptType,
	}

	// 如果某些字段为空，保持原有值
	if updatedMeta.Name == "" {
		updatedMeta.Name = existingTemplate.GetName()
	}
	if updatedMeta.Desc == "" {
		updatedMeta.Desc = existingTemplate.GetDescription()
	}
	if updatedMeta.ExptType == 0 {
		updatedMeta.ExptType = existingTemplate.GetExptType()
	}

	// 准备更新后的 TripleConfig
	updatedTripleConfig := &entity.ExptTemplateTuple{
		EvalSetID:               existingTemplate.GetEvalSetID(), // 不允许修改
		EvalSetVersionID:        param.EvalSetVersionID,
		TargetID:                finalTargetID,
		TargetVersionID:         finalTargetVersionID,
		TargetType:              targetType,
		EvaluatorVersionIds:     e.extractEvaluatorVersionIDs(param.EvaluatorIDVersionItems),
		EvaluatorIDVersionItems: param.EvaluatorIDVersionItems,
	}

	// 如果某些字段为空，保持原有值
	if updatedTripleConfig.EvalSetVersionID == 0 {
		updatedTripleConfig.EvalSetVersionID = existingTemplate.GetEvalSetVersionID()
	}
	if updatedTripleConfig.TargetVersionID == 0 {
		updatedTripleConfig.TargetVersionID = existingTemplate.GetTargetVersionID()
	}

	// 如果创建了评测对象，更新 TemplateConf 中的 TargetVersionID
	if param.CreateEvalTargetParam != nil && !param.CreateEvalTargetParam.IsNull() && param.TemplateConf != nil && param.TemplateConf.ConnectorConf.TargetConf != nil {
		param.TemplateConf.ConnectorConf.TargetConf.TargetVersionID = finalTargetVersionID
	} else if param.TemplateConf != nil && param.TemplateConf.ConnectorConf.TargetConf != nil && finalTargetVersionID > 0 {
		// 更新 TemplateConf 中的 TargetVersionID（如果提供了新版本）
		param.TemplateConf.ConnectorConf.TargetConf.TargetVersionID = finalTargetVersionID
	}

	// 构建更新后的模板实体（默认沿用原有 EnableScoreWeight）
	now := time.Now()
	baseInfo := &entity.BaseInfo{
		UpdatedAt: gptr.Of(now.UnixMilli()),
		UpdatedBy: &entity.UserInfo{UserID: gptr.Of(session.UserID)},
	}
	// 如果原有模板有 BaseInfo，保留 CreatedAt 和 CreatedBy
	if existingTemplate.BaseInfo != nil {
		baseInfo.CreatedAt = existingTemplate.BaseInfo.CreatedAt
		baseInfo.CreatedBy = existingTemplate.BaseInfo.CreatedBy
	}
	updatedTemplate := &entity.ExptTemplate{
		Meta:                updatedMeta,
		TripleConfig:        updatedTripleConfig,
		EvaluatorVersionRef: evaluatorVersionRefs,
		TemplateConf:        param.TemplateConf,
		BaseInfo:            baseInfo,
	}

	// 如果 TemplateConf 为空，保持原有值
	if updatedTemplate.TemplateConf == nil {
		updatedTemplate.TemplateConf = existingTemplate.TemplateConf
	}

	// 从 TemplateConf 构建 FieldMappingConfig，并根据 EvaluatorConf.ScoreWeight 设置是否启用分数权重
	e.buildFieldMappingConfigAndEnableScoreWeight(updatedTemplate, updatedTemplate.TemplateConf)

	// 转换为评估器引用DO
	refs := updatedTemplate.ToEvaluatorRefDO()

	// 更新数据库
	if err := e.templateRepo.UpdateWithRefs(ctx, updatedTemplate, refs); err != nil {
		return nil, err
	}

	// 重新获取更新后的模板
	updatedTemplate, err = e.templateRepo.GetByID(ctx, param.TemplateID, &param.SpaceID)
	if err != nil {
		return nil, err
	}
	if updatedTemplate == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("template %d not found after update", param.TemplateID)))
	}

	// 填充关联数据（EvalSet、EvalTarget、Evaluators）
	// 如果创建了新的 EvalTarget，需要从主库读取以避免主从延迟
	queryCtx := ctx
	if param.CreateEvalTargetParam != nil && !param.CreateEvalTargetParam.IsNull() {
		queryCtx = contexts.WithCtxWriteDB(ctx)
	}
	tupleID := e.packTemplateTupleID(updatedTemplate)
	exptTuples, err := e.mgetExptTupleByID(queryCtx, []*entity.ExptTupleID{tupleID}, param.SpaceID, session)
	if err != nil {
		return nil, err
	}
	if len(exptTuples) > 0 {
		updatedTemplate.EvalSet = exptTuples[0].EvalSet
		updatedTemplate.Target = exptTuples[0].Target
		updatedTemplate.Evaluators = exptTuples[0].Evaluators
	}

	return updatedTemplate, nil
}

func (e *ExptTemplateManagerImpl) UpdateMeta(ctx context.Context, param *entity.UpdateExptTemplateMetaParam, session *entity.Session) (*entity.ExptTemplate, error) {
	// 获取现有模板
	existingTemplate, err := e.templateRepo.GetByID(ctx, param.TemplateID, &param.SpaceID)
	if err != nil {
		return nil, err
	}
	if existingTemplate == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("template %d not found", param.TemplateID)))
	}

	// 如果名称改变，检查新名称是否可用（允许和当前名称重复）
	if param.Name != "" && param.Name != existingTemplate.GetName() {
		pass, err := e.CheckName(ctx, param.Name, param.SpaceID, session)
		if !pass {
			return nil, errorx.NewByCode(errno.ExperimentNameExistedCode, errorx.WithExtraMsg(fmt.Sprintf("template name %s already exists", param.Name)))
		}
		if err != nil {
			return nil, err
		}
	}

	// 构建更新字段
	ufields := make(map[string]any)
	if param.Name != "" {
		ufields["name"] = param.Name
	}
	if param.Description != "" {
		ufields["description"] = param.Description
	}
	if param.ExptType > 0 {
		ufields["expt_type"] = int32(param.ExptType)
	}

	// 更新 updated_at 和 updated_by
	now := time.Now()
	ufields["updated_at"] = now
	if session != nil && session.UserID != "" {
		ufields["updated_by"] = session.UserID
	}

	// 更新数据库
	if len(ufields) > 0 {
		if err := e.templateRepo.UpdateFields(ctx, param.TemplateID, ufields); err != nil {
			return nil, err
		}
	}

	// 重新获取更新后的模板
	updatedTemplate, err := e.templateRepo.GetByID(ctx, param.TemplateID, &param.SpaceID)
	if err != nil {
		return nil, err
	}
	if updatedTemplate == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("template %d not found after update", param.TemplateID)))
	}

	// 更新 BaseInfo
	if updatedTemplate.BaseInfo == nil {
		updatedTemplate.BaseInfo = &entity.BaseInfo{}
	}
	updatedTemplate.BaseInfo.UpdatedAt = gptr.Of(now.UnixMilli())
	if session != nil && session.UserID != "" {
		updatedTemplate.BaseInfo.UpdatedBy = &entity.UserInfo{UserID: gptr.Of(session.UserID)}
	}

	return updatedTemplate, nil
}

// UpdateExptInfo 更新实验模板的 ExptInfo
// adjustCount: 实验数量的增量（创建实验时为 +1，删除实验时为 -1，状态变更时为 0）
func (e *ExptTemplateManagerImpl) UpdateExptInfo(ctx context.Context, templateID, spaceID int64, exptID int64, exptStatus entity.ExptStatus, adjustCount int64) error {
	// 获取现有模板
	existingTemplate, err := e.templateRepo.GetByID(ctx, templateID, &spaceID)
	if err != nil {
		return errorx.Wrapf(err, "get template fail, template_id: %d", templateID)
	}
	if existingTemplate == nil {
		return errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("template %d not found", templateID)))
	}

	// 初始化或更新 ExptInfo
	var exptInfo *entity.ExptInfo
	if existingTemplate.ExptInfo != nil {
		exptInfo = existingTemplate.ExptInfo
	} else {
		exptInfo = &entity.ExptInfo{
			CreatedExptCount: 0,
			LatestExptID:     0,
			LatestExptStatus: entity.ExptStatus_Unknown,
		}
	}

	// 根据 adjustCount 调整创建实验数量
	if adjustCount != 0 {
		exptInfo.CreatedExptCount += adjustCount
		if exptInfo.CreatedExptCount < 0 {
			exptInfo.CreatedExptCount = 0
		}
	}

	// 更新最新实验ID和状态
	exptInfo.LatestExptID = exptID
	exptInfo.LatestExptStatus = exptStatus

	// 序列化 ExptInfo
	exptInfoBytes, err := json.Marshal(exptInfo)
	if err != nil {
		return errorx.Wrapf(err, "marshal ExptInfo fail, template_id: %d", templateID)
	}

	// 更新数据库
	ufields := map[string]any{
		"expt_info": exptInfoBytes,
	}
	if err := e.templateRepo.UpdateFields(ctx, templateID, ufields); err != nil {
		return errorx.Wrapf(err, "update ExptInfo fail, template_id: %d", templateID)
	}

	return nil
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

	// 构建 ExptTupleID 列表，用于批量查询关联数据
	tupleIDs := make([]*entity.ExptTupleID, 0, len(templates))
	for _, template := range templates {
		tupleIDs = append(tupleIDs, e.packTemplateTupleID(template))
	}

	// 批量查询关联数据
	exptTuples, err := e.mgetExptTupleByID(ctx, tupleIDs, spaceID, session)
	if err != nil {
		return nil, 0, err
	}

	// 填充关联数据
	for idx := range exptTuples {
		templates[idx].EvalSet = exptTuples[idx].EvalSet
		templates[idx].Target = exptTuples[idx].Target
		templates[idx].Evaluators = exptTuples[idx].Evaluators
	}

	return templates, count, nil
}

// resolveAndFillEvaluatorVersionIDs 解析并回填评估器版本ID
// 如果 EvaluatorIDVersionItems 中的项缺少 evaluator_version_id，则根据 evaluator_id 和 version 解析并回填
// 同时回填 TemplateConf 中 EvaluatorConf 缺失的 evaluator_version_id
// 注意：FieldMappingConfig 中的 EvaluatorFieldMapping 会在 buildFieldMappingConfigAndEnableScoreWeight 中从 TemplateConf 构建
func (e *ExptTemplateManagerImpl) resolveAndFillEvaluatorVersionIDs(
	ctx context.Context,
	spaceID int64,
	templateConf *entity.ExptTemplateConfiguration,
	evaluatorIDVersionItems []*entity.EvaluatorIDVersionItem,
) error {
	// 收集需要查询的 evaluator_id 和 version
	builtinIDs := make([]int64, 0)
	normalPairs := make([][2]interface{}, 0)
	itemsNeedResolve := make([]*entity.EvaluatorIDVersionItem, 0)

	// 1. 从 EvaluatorIDVersionItems 中收集
	for _, item := range evaluatorIDVersionItems {
		if item == nil {
			continue
		}
		// 如果已经有 evaluator_version_id，跳过
		if item.EvaluatorVersionID > 0 {
			continue
		}
		eid := item.EvaluatorID
		ver := item.Version
		if eid == 0 || ver == "" {
			continue
		}
		itemsNeedResolve = append(itemsNeedResolve, item)
		if ver == "BuiltinVisible" {
			builtinIDs = append(builtinIDs, eid)
		} else {
			normalPairs = append(normalPairs, [2]interface{}{eid, ver})
		}
	}

	// 2. 从 TemplateConf.EvaluatorsConf.EvaluatorConf 中收集缺失 evaluator_version_id 的项
	if templateConf != nil && templateConf.ConnectorConf.EvaluatorsConf != nil {
		for _, ec := range templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf {
			if ec == nil || ec.EvaluatorVersionID > 0 {
				continue
			}
			if ec.EvaluatorID > 0 && ec.Version != "" {
				// 检查是否已存在
				found := false
				if ec.Version == "BuiltinVisible" {
					for _, id := range builtinIDs {
						if id == ec.EvaluatorID {
							found = true
							break
						}
					}
					if !found {
						builtinIDs = append(builtinIDs, ec.EvaluatorID)
					}
				} else {
					for _, pair := range normalPairs {
						if pair[0].(int64) == ec.EvaluatorID && pair[1].(string) == ec.Version {
							found = true
							break
						}
					}
					if !found {
						normalPairs = append(normalPairs, [2]interface{}{ec.EvaluatorID, ec.Version})
					}
				}
			}
		}
	}

	// 如果没有需要解析的项，直接返回
	if len(itemsNeedResolve) == 0 && len(builtinIDs) == 0 && len(normalPairs) == 0 {
		return nil
	}

	// 批量获取内置与普通版本
	id2Builtin := make(map[int64]*entity.Evaluator, len(builtinIDs))
	if len(builtinIDs) > 0 {
		evs, err := e.evaluatorService.BatchGetBuiltinEvaluator(ctx, builtinIDs)
		if err != nil {
			return errorx.Wrapf(err, "batch get builtin evaluator fail")
		}
		for _, ev := range evs {
			if ev != nil {
				// 预置评估器允许跨空间复用，这里不做 SpaceID 校验
				id2Builtin[ev.ID] = ev
			}
		}
	}

	pair2Eval := make(map[string]*entity.Evaluator, len(normalPairs))
	if len(normalPairs) > 0 {
		evs, err := e.evaluatorService.BatchGetEvaluatorByIDAndVersion(ctx, normalPairs)
		if err != nil {
			return errorx.Wrapf(err, "batch get evaluator by id and version fail")
		}
		for _, ev := range evs {
			if ev == nil {
				continue
			}
			// 非预置评估器必须与模板 SpaceID 一致，防止绑定其他空间的评估器
			if !ev.Builtin && ev.GetSpaceID() != spaceID {
				return errorx.NewByCode(
					errno.EvaluatorVersionNotFoundCode,
					errorx.WithExtraMsg(fmt.Sprintf("evaluator %d version %s does not belong to workspace %d", ev.ID, ev.GetVersion(), spaceID)),
				)
			}
			key := fmt.Sprintf("%d#%s", ev.ID, ev.GetVersion())
			pair2Eval[key] = ev
		}
	}

	// 回填 EvaluatorIDVersionItems 中缺失的版本ID
	for _, item := range itemsNeedResolve {
		if item == nil {
			continue
		}
		eid := item.EvaluatorID
		ver := item.Version
		if eid == 0 || ver == "" {
			continue
		}
		var ev *entity.Evaluator
		if ver == "BuiltinVisible" {
			ev = id2Builtin[eid]
		} else {
			key := fmt.Sprintf("%d#%s", eid, ver)
			ev = pair2Eval[key]
		}
		if ev != nil {
			if verID := ev.GetEvaluatorVersionID(); verID != 0 {
				item.EvaluatorVersionID = verID
			}
		}
	}

	// 构建 evaluator_id + version -> evaluator_version_id 的映射（用于回填 EvaluatorConf）
	eidVer2VersionID := make(map[string]int64)
	// 从已回填的 items 中构建映射
	for _, item := range evaluatorIDVersionItems {
		if item != nil && item.EvaluatorVersionID > 0 {
			key := fmt.Sprintf("%d#%s", item.EvaluatorID, item.Version)
			eidVer2VersionID[key] = item.EvaluatorVersionID
		}
	}
	// 从查询结果中补充映射
	for _, ev := range id2Builtin {
		if ev != nil && ev.GetEvaluatorVersionID() > 0 {
			key := fmt.Sprintf("%d#%s", ev.ID, "BuiltinVisible")
			eidVer2VersionID[key] = ev.GetEvaluatorVersionID()
		}
	}
	for _, ev := range pair2Eval {
		if ev != nil && ev.GetEvaluatorVersionID() > 0 {
			key := fmt.Sprintf("%d#%s", ev.ID, ev.GetVersion())
			eidVer2VersionID[key] = ev.GetEvaluatorVersionID()
		}
	}

	// 回填 TemplateConf 中 EvaluatorConf 缺失的 evaluator_version_id
	if templateConf != nil && templateConf.ConnectorConf.EvaluatorsConf != nil {
		evaluatorConfs := templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf
		for _, ec := range evaluatorConfs {
			if ec == nil {
				continue
			}
			// 如果已经有 evaluator_version_id，跳过
			if ec.EvaluatorVersionID > 0 {
				continue
			}
			// 从映射中查找并回填
			if ec.EvaluatorID > 0 && ec.Version != "" {
				key := fmt.Sprintf("%d#%s", ec.EvaluatorID, ec.Version)
				if verID, ok := eidVer2VersionID[key]; ok && verID > 0 {
					ec.EvaluatorVersionID = verID
				}
			}
		}
	}

	return nil
}

// buildEvaluatorVersionRefs 从 EvaluatorIDVersionItems 构建 evaluatorVersionRefs
func (e *ExptTemplateManagerImpl) buildEvaluatorVersionRefs(items []*entity.EvaluatorIDVersionItem) []*entity.ExptTemplateEvaluatorVersionRef {
	refs := make([]*entity.ExptTemplateEvaluatorVersionRef, 0)
	for _, item := range items {
		if item != nil && item.EvaluatorVersionID > 0 {
			refs = append(refs, &entity.ExptTemplateEvaluatorVersionRef{
				EvaluatorID:        item.EvaluatorID,
				EvaluatorVersionID: item.EvaluatorVersionID,
			})
		}
	}
	return refs
}

// extractEvaluatorVersionIDs 从 EvaluatorIDVersionItems 中提取 EvaluatorVersionID 列表
func (e *ExptTemplateManagerImpl) extractEvaluatorVersionIDs(items []*entity.EvaluatorIDVersionItem) []int64 {
	ids := make([]int64, 0)
	idSet := make(map[int64]bool)
	for _, item := range items {
		if item != nil && item.EvaluatorVersionID > 0 {
			if !idSet[item.EvaluatorVersionID] {
				ids = append(ids, item.EvaluatorVersionID)
				idSet[item.EvaluatorVersionID] = true
			}
		}
	}
	return ids
}

// resolveTargetForCreate 解析创建模板时的 target 信息
func (e *ExptTemplateManagerImpl) resolveTargetForCreate(ctx context.Context, param *entity.CreateExptTemplateParam) (targetID, targetVersionID int64, targetType entity.EvalTargetType, err error) {
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
			return 0, 0, 0, errorx.Wrapf(err, "CreateEvalTarget failed, param: %v", param.CreateEvalTargetParam)
		}
		return targetID, targetVersionID, gptr.Indirect(param.CreateEvalTargetParam.EvalTargetType), nil
	}
	if param.TargetID > 0 {
		// 如果提供了 target_id，则获取现有的评测对象
		target, err := e.evalTargetService.GetEvalTarget(ctx, param.TargetID)
		if err != nil {
			return 0, 0, 0, errorx.Wrapf(err, "get eval target fail, target_id: %d", param.TargetID)
		}
		if target == nil {
			return 0, 0, 0, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("target %d not found", param.TargetID)))
		}
		return param.TargetID, param.TargetVersionID, target.EvalTargetType, nil
	}
	return 0, 0, 0, nil
}

// buildFieldMappingConfigAndEnableScoreWeight 从 TemplateConf 构建 FieldMappingConfig，并根据 EvaluatorConf.ScoreWeight 设置是否启用分数权重
func (e *ExptTemplateManagerImpl) buildFieldMappingConfigAndEnableScoreWeight(template *entity.ExptTemplate, templateConf *entity.ExptTemplateConfiguration) {
	if templateConf == nil {
		return
	}

	fieldMappingConfig := &entity.ExptFieldMapping{
		ItemConcurNum: templateConf.ItemConcurNum,
	}

	// 从 ConnectorConf 转换字段映射
	if templateConf.ConnectorConf.TargetConf != nil && templateConf.ConnectorConf.TargetConf.IngressConf != nil {
		ingressConf := templateConf.ConnectorConf.TargetConf.IngressConf
		targetMapping := &entity.TargetFieldMapping{}
		if ingressConf.EvalSetAdapter != nil {
			for _, fc := range ingressConf.EvalSetAdapter.FieldConfs {
				targetMapping.FromEvalSet = append(targetMapping.FromEvalSet, &entity.ExptTemplateFieldMapping{
					FieldName:     fc.FieldName,
					FromFieldName: fc.FromField,
					ConstValue:    fc.Value,
				})
			}
		}
		fieldMappingConfig.TargetFieldMapping = targetMapping

		// 提取运行时参数
		if ingressConf.CustomConf != nil {
			for _, fc := range ingressConf.CustomConf.FieldConfs {
				if fc.FieldName == "builtin_runtime_param" {
					fieldMappingConfig.TargetRuntimeParam = &entity.RuntimeParam{
						JSONValue: gptr.Of(fc.Value),
					}
					break
				}
			}
		}
	}

	if templateConf.ConnectorConf.EvaluatorsConf != nil {
		evaluatorMappings := make([]*entity.EvaluatorFieldMapping, 0, len(templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf))
		for _, ec := range templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf {
			if ec.IngressConf == nil {
				continue
			}
			em := &entity.EvaluatorFieldMapping{
				EvaluatorVersionID: ec.EvaluatorVersionID,
				EvaluatorID:        ec.EvaluatorID,
				Version:            ec.Version,
			}
			if ec.IngressConf.EvalSetAdapter != nil {
				for _, fc := range ec.IngressConf.EvalSetAdapter.FieldConfs {
					em.FromEvalSet = append(em.FromEvalSet, &entity.ExptTemplateFieldMapping{
						FieldName:     fc.FieldName,
						FromFieldName: fc.FromField,
						ConstValue:    fc.Value,
					})
				}
			}
			if ec.IngressConf.TargetAdapter != nil {
				for _, fc := range ec.IngressConf.TargetAdapter.FieldConfs {
					em.FromTarget = append(em.FromTarget, &entity.ExptTemplateFieldMapping{
						FieldName:     fc.FieldName,
						FromFieldName: fc.FromField,
						ConstValue:    fc.Value,
					})
				}
			}
			evaluatorMappings = append(evaluatorMappings, em)
		}
		fieldMappingConfig.EvaluatorFieldMapping = evaluatorMappings

		// 如果有任一评估器配置了分数权重，则标记模板支持分数权重
		if templateConf.ConnectorConf.EvaluatorsConf != nil {
			templateConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight = false
			for _, ec := range templateConf.ConnectorConf.EvaluatorsConf.EvaluatorConf {
				if ec != nil && ec.ScoreWeight != nil && *ec.ScoreWeight > 0 {
					templateConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight = true
					break
				}
			}
		}
	}

	template.FieldMappingConfig = fieldMappingConfig
}

// packTemplateTupleID 从 ExptTemplate 构建 ExptTupleID
func (e *ExptTemplateManagerImpl) packTemplateTupleID(template *entity.ExptTemplate) *entity.ExptTupleID {
	exptTupleID := &entity.ExptTupleID{
		VersionedEvalSetID: &entity.VersionedEvalSetID{
			EvalSetID: template.GetEvalSetID(),
			VersionID: template.GetEvalSetVersionID(),
		},
	}

	if template.GetTargetID() > 0 || template.GetTargetVersionID() > 0 {
		exptTupleID.VersionedTargetID = &entity.VersionedTargetID{
			TargetID:  template.GetTargetID(),
			VersionID: template.GetTargetVersionID(),
		}
	}

	// 从 EvaluatorVersionRef 或 EvaluatorIDVersionItems 中提取 EvaluatorVersionIDs
	if len(template.EvaluatorVersionRef) > 0 {
		evaluatorVersionIDs := make([]int64, 0, len(template.EvaluatorVersionRef))
		for _, ref := range template.EvaluatorVersionRef {
			if ref.EvaluatorVersionID > 0 {
				evaluatorVersionIDs = append(evaluatorVersionIDs, ref.EvaluatorVersionID)
			}
		}
		exptTupleID.EvaluatorVersionIDs = evaluatorVersionIDs
	} else if template.TripleConfig != nil && len(template.TripleConfig.EvaluatorVersionIds) > 0 {
		exptTupleID.EvaluatorVersionIDs = template.TripleConfig.EvaluatorVersionIds
	}

	return exptTupleID
}

// mgetExptTupleByID 批量查询关联数据（参考 ExptMangerImpl.mgetExptTupleByID）
func (e *ExptTemplateManagerImpl) mgetExptTupleByID(ctx context.Context, tupleIDs []*entity.ExptTupleID, spaceID int64, session *entity.Session) ([]*entity.ExptTuple, error) {
	var (
		versionedTargetIDs  = make([]*entity.VersionedTargetID, 0, len(tupleIDs))
		versionedEvalSetIDs = make([]*entity.VersionedEvalSetID, 0, len(tupleIDs))
		evaluatorVersionIDs []int64

		targets    []*entity.EvalTarget
		evalSets   []*entity.EvaluationSet
		evaluators []*entity.Evaluator
	)

	for _, etids := range tupleIDs {
		if etids.VersionedEvalSetID != nil {
			versionedEvalSetIDs = append(versionedEvalSetIDs, etids.VersionedEvalSetID)
		}
		if etids.VersionedTargetID != nil {
			versionedTargetIDs = append(versionedTargetIDs, etids.VersionedTargetID)
		}
		if len(etids.EvaluatorVersionIDs) > 0 {
			evaluatorVersionIDs = append(evaluatorVersionIDs, etids.EvaluatorVersionIDs...)
		}
	}

	pool, err := goroutine.NewPool(3)
	if err != nil {
		return nil, err
	}

	// 查询 Target
	if len(versionedTargetIDs) > 0 {
		pool.Add(func() error {
			// 去重
			targetVersionIDs := make([]int64, 0, len(versionedTargetIDs))
			for _, tids := range versionedTargetIDs {
				targetVersionIDs = append(targetVersionIDs, tids.VersionID)
			}
			targetVersionIDs = maps.ToSlice(gslice.ToMap(targetVersionIDs, func(t int64) (int64, bool) { return t, true }), func(k int64, v bool) int64 { return k })
			var poolErr error
			targets, poolErr = e.evalTargetService.BatchGetEvalTargetVersion(ctx, spaceID, targetVersionIDs, true)
			if poolErr != nil {
				return poolErr
			}
			return nil
		})
	}

	// 查询 EvalSet
	if len(versionedEvalSetIDs) > 0 {
		evalSetVersionIDs := make([]int64, 0, len(versionedEvalSetIDs))
		for _, ids := range versionedEvalSetIDs {
			if ids.EvalSetID != ids.VersionID {
				evalSetVersionIDs = append(evalSetVersionIDs, ids.VersionID)
			}
		}
		if len(evalSetVersionIDs) > 0 {
			pool.Add(func() error {
				verIDs := maps.ToSlice(gslice.ToMap(evalSetVersionIDs, func(t int64) (int64, bool) { return t, true }), func(k int64, v bool) int64 { return k })
				// 仅查询未删除版本，避免带出已删除列
				got, poolErr := e.evaluationSetVersionService.BatchGetEvaluationSetVersions(ctx, gptr.Of(spaceID), verIDs, gptr.Of(false))
				if poolErr != nil {
					return poolErr
				}
				for _, elem := range got {
					if elem == nil {
						continue
					}
					elem.EvaluationSet.EvaluationSetVersion = elem.Version
					evalSets = append(evalSets, elem.EvaluationSet)
				}
				return nil
			})
		}
		// 草稿的evalSetID和versionID相同
		evalSetIDs := make([]int64, 0, len(versionedEvalSetIDs))
		for _, ids := range versionedEvalSetIDs {
			if ids.EvalSetID == ids.VersionID {
				evalSetIDs = append(evalSetIDs, ids.EvalSetID)
			}
		}
		if len(evalSetIDs) > 0 {
			pool.Add(func() error {
				setIDs := maps.ToSlice(gslice.ToMap(evalSetIDs, func(t int64) (int64, bool) { return t, true }), func(k int64, v bool) int64 { return k })
				got, poolErr := e.evaluationSetService.BatchGetEvaluationSets(ctx, gptr.Of(spaceID), setIDs, gptr.Of(false))
				if poolErr != nil {
					return poolErr
				}
				for _, elem := range got {
					if elem == nil {
						continue
					}
					evalSets = append(evalSets, elem)
				}
				return nil
			})
		}
	}

	// 查询 Evaluators
	if len(evaluatorVersionIDs) > 0 {
		pool.Add(func() error {
			var poolErr error
			evaluators, poolErr = e.evaluatorService.BatchGetEvaluatorVersion(ctx, nil, evaluatorVersionIDs, true)
			if poolErr != nil {
				return poolErr
			}
			return nil
		})
	}

	if err := pool.Exec(ctx); err != nil {
		return nil, err
	}

	// 构建结果映射（参考 ExptMangerImpl.mgetExptTupleByID）
	targetMap := gslice.ToMap(targets, func(t *entity.EvalTarget) (int64, *entity.EvalTarget) {
		if t == nil || t.EvalTargetVersion == nil {
			return 0, nil
		}
		return t.EvalTargetVersion.ID, t
	})
	evalSetMap := gslice.ToMap(evalSets, func(t *entity.EvaluationSet) (int64, *entity.EvaluationSet) {
		if t == nil {
			return 0, nil
		}
		// 对于版本化的 EvalSet，使用 VersionID 作为 key
		if t.EvaluationSetVersion != nil {
			return t.EvaluationSetVersion.ID, t
		}
		// 对于草稿 EvalSet，使用 EvalSetID 作为 key（此时 EvalSetID == VersionID）
		return t.ID, t
	})
	evaluatorMap := gslice.ToMap(evaluators, func(t *entity.Evaluator) (int64, *entity.Evaluator) {
		return t.GetEvaluatorVersionID(), t
	})

	// 构建结果列表
	res := make([]*entity.ExptTuple, 0, len(tupleIDs))
	for _, tupleIDs := range tupleIDs {
		tuple := &entity.ExptTuple{
			EvalSet: evalSetMap[tupleIDs.VersionedEvalSetID.VersionID],
		}
		if tupleIDs.VersionedTargetID != nil {
			tuple.Target = targetMap[tupleIDs.VersionedTargetID.VersionID]
		}
		if len(tupleIDs.EvaluatorVersionIDs) > 0 {
			cevaluators := make([]*entity.Evaluator, 0, len(tupleIDs.EvaluatorVersionIDs))
			for _, evaluatorVersionID := range tupleIDs.EvaluatorVersionIDs {
				if ev, ok := evaluatorMap[evaluatorVersionID]; ok && ev != nil {
					cevaluators = append(cevaluators, ev)
				}
			}
			tuple.Evaluators = cevaluators
		}
		res = append(res, tuple)
	}

	return res, nil
}
