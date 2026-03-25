// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	taskfilter "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	taskdomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
)

func NewExptTemplateManager(
	templateRepo repo.IExptTemplateRepo,
	idgen idgen.IIDGenerator,
	evaluatorService EvaluatorService,
	evalTargetService IEvalTargetService,
	evaluationSetService IEvaluationSetService,
	evaluationSetVersionService EvaluationSetVersionService,
	lwt platestwrite.ILatestWriteTracker,
	taskRPCAdapter rpc.ITaskRPCAdapter,
	pipelineRPCAdapter rpc.IPipelineListAdapter,
	exptRepo repo.IExperimentRepo,
) IExptTemplateManager {
	return &ExptTemplateManagerImpl{
		templateRepo:                templateRepo,
		idgen:                       idgen,
		evaluatorService:            evaluatorService,
		evalTargetService:           evalTargetService,
		evaluationSetService:        evaluationSetService,
		evaluationSetVersionService: evaluationSetVersionService,
		lwt:                         lwt,
		taskRPCAdapter:              taskRPCAdapter,
		pipelineRPCAdapter:          pipelineRPCAdapter,
		exptRepo:                    exptRepo,
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
	taskRPCAdapter              rpc.ITaskRPCAdapter
	pipelineRPCAdapter          rpc.IPipelineListAdapter
	exptRepo                    repo.IExperimentRepo
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
		ExptInfo: &entity.ExptInfo{
			CronActivate: param.CronActivate,
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
		ExptSource: param.ExptSource,
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

	if err := e.enrichExptSourceFromPipeline(ctx, templates, spaceID); err != nil {
		return nil, errorx.Wrapf(err, "enrich expt source from pipeline fail, workspace_id: %d", spaceID)
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

	// 合并 ExptInfo（cron_activate 等运行态字段）
	var mergedExptInfo *entity.ExptInfo
	if existingTemplate.ExptInfo != nil {
		cpy := *existingTemplate.ExptInfo
		mergedExptInfo = &cpy
	} else {
		mergedExptInfo = &entity.ExptInfo{}
	}
	if param.CronActivate != nil {
		mergedExptInfo.CronActivate = *param.CronActivate
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
		ExptInfo:            mergedExptInfo,
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
	if param.CronActivate != nil {
		ufields["cron_activate"] = *param.CronActivate
		exptInfo := existingTemplate.ExptInfo
		if exptInfo == nil {
			exptInfo = &entity.ExptInfo{}
		} else {
			cpy := *exptInfo
			exptInfo = &cpy
		}
		exptInfo.CronActivate = *param.CronActivate
		exptInfoBytes, mErr := json.Marshal(exptInfo)
		if mErr != nil {
			return nil, errorx.Wrapf(mErr, "marshal ExptInfo fail for update_meta, template_id: %d", param.TemplateID)
		}
		ufields["expt_info"] = exptInfoBytes
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
// latestExptStartTime: 最新实验开始时间（毫秒时间戳），创建实验时传入，其他场景传 nil
func (e *ExptTemplateManagerImpl) UpdateExptInfo(ctx context.Context, templateID, spaceID, exptID int64, exptStatus entity.ExptStatus, adjustCount int64, latestExptStartTime *int64) error {
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

	// 更新最新实验开始时间（创建实验时传入）
	if latestExptStartTime != nil && *latestExptStartTime > 0 {
		exptInfo.LatestExptStartTime = *latestExptStartTime
	}

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

	// ListExperimentTemplates 走 List 分支时也需要填充：ExptSource 来自 DB，span_filter / scheduler 依赖 Pipeline
	if err := e.enrichExptSourceFromPipeline(ctx, templates, spaceID); err != nil {
		return nil, 0, errorx.Wrapf(err, "enrich expt source from pipeline fail, workspace_id: %d", spaceID)
	}

	return templates, count, nil
}

// ListOnline 查询在线实验模板（Task 与 ExptTemplate 同级）
// 1. 通过 taskRPCAdapter.ListTasks 查询所有 task，转换为 ExptTemplate（直接用 task.Rule.SpanFilters，Scheduler 从 task 取）
// 2. 查询当前空间下所有在线 ExptTemplate（expt_type=Online）
// 3. 合并后内存筛选、排序、分页
func (e *ExptTemplateManagerImpl) ListOnline(ctx context.Context, page, pageSize int32, spaceID int64, filter *entity.ExptTemplateListFilter, orderBys []*entity.OrderBy, session *entity.Session) ([]*entity.ExptTemplate, int64, error) {
	// Step 1: 通过 ITaskRPCAdapter 查询当前空间下所有的 task
	limit := int32(200)
	offset := int32(0)
	var allTasks []*taskdomain.Task
	for {
		tasks, total, err := e.taskRPCAdapter.ListTasks(ctx, &rpc.ListTasksParam{
			WorkspaceID: spaceID,
			Limit:       gptr.Of(limit),
			Offset:      gptr.Of(offset),
		})
		if err != nil {
			return nil, 0, errorx.Wrapf(err, "list tasks fail, workspace_id: %d", spaceID)
		}
		if tasks == nil || len(tasks) == 0 {
			break
		}
		allTasks = append(allTasks, tasks...)
		if total != nil && int64(len(allTasks)) >= *total {
			break
		}
		offset += limit
	}

	// Step 2: 将 task 转换为 ExptTemplate（使用 task.Rule.SpanFilters，Scheduler 从 task 取，Task 无 Scheduler 则为 nil）
	taskTemplates := make([]*entity.ExptTemplate, 0, len(allTasks))
	for _, task := range allTasks {
		if task == nil || task.ID == nil {
			continue
		}
		tpl := taskToExptTemplate(task, spaceID)
		if tpl != nil {
			taskTemplates = append(taskTemplates, tpl)
		}
	}

	// Step 2.5: 填充 task 转换的模板的 Evaluators（通过 BatchGetEvaluatorVersion）
	if len(taskTemplates) > 0 {
		taskTupleIDs := make([]*entity.ExptTupleID, 0, len(taskTemplates))
		for _, t := range taskTemplates {
			taskTupleIDs = append(taskTupleIDs, e.packTemplateTupleID(t))
		}
		taskExptTuples, err := e.mgetExptTupleByID(ctx, taskTupleIDs, spaceID, session)
		if err != nil {
			return nil, 0, errorx.Wrapf(err, "mget expt tuple for task templates fail")
		}
		for idx := range taskExptTuples {
			taskTemplates[idx].Evaluators = taskExptTuples[idx].Evaluators
		}
	}

	// Step 3: 查询当前空间下所有在线 ExptTemplate（expt_type=Online）
	onlineFilter := &entity.ExptTemplateListFilter{
		Includes: &entity.ExptTemplateFilterFields{
			ExptType: []int64{int64(entity.ExptType_Online)},
		},
	}
	if filter != nil {
		onlineFilter.FuzzyName = filter.FuzzyName
		onlineFilter.Excludes = filter.Excludes
		if filter.Includes != nil {
			onlineFilter.Includes = &entity.ExptTemplateFilterFields{
				ExptType:       []int64{int64(entity.ExptType_Online)},
				CreatedBy:      filter.Includes.CreatedBy,
				UpdatedBy:      filter.Includes.UpdatedBy,
				EvalSetIDs:     filter.Includes.EvalSetIDs,
				TargetIDs:      filter.Includes.TargetIDs,
				EvaluatorIDs:   filter.Includes.EvaluatorIDs,
				TargetType:     filter.Includes.TargetType,
				CronActivate:   filter.Includes.CronActivate,
			}
		}
	}
	// 分页拉取所有在线模板（templateRepo.List 在 page/size 为 0 时只返回 defaultLimit 条）
	listPageSize := int32(200)
	listPageNum := int32(1)
	var dbTemplates []*entity.ExptTemplate
	for {
		pageTemplates, total, err := e.templateRepo.List(ctx, listPageNum, listPageSize, onlineFilter, nil, spaceID)
		if err != nil {
			return nil, 0, errorx.Wrapf(err, "list online templates fail, workspace_id: %d", spaceID)
		}
		dbTemplates = append(dbTemplates, pageTemplates...)
		if total == 0 || int64(len(dbTemplates)) >= total {
			break
		}
		listPageNum++
	}

	// Step 4: 填充 DB 模板的关联数据（EvalSet、Target、Evaluators）
	if len(dbTemplates) > 0 {
		tupleIDs := make([]*entity.ExptTupleID, 0, len(dbTemplates))
		for _, t := range dbTemplates {
			tupleIDs = append(tupleIDs, e.packTemplateTupleID(t))
		}
		exptTuples, err := e.mgetExptTupleByID(ctx, tupleIDs, spaceID, session)
		if err != nil {
			return nil, 0, errorx.Wrapf(err, "mget expt tuple by id fail")
		}
		for idx := range exptTuples {
			dbTemplates[idx].EvalSet = exptTuples[idx].EvalSet
			dbTemplates[idx].Target = exptTuples[idx].Target
			dbTemplates[idx].Evaluators = exptTuples[idx].Evaluators
		}
	}

	// Step 5: 对 DB 模板（非 task 来源）从 Pipeline 填充 ExptSource.SpanFilterFields 和 Scheduler
	// 来自 task 的模板已在 taskToExptTemplate 中填充，此处只处理 DB 中的在线模板
	if err := e.enrichExptSourceFromPipeline(ctx, dbTemplates, spaceID); err != nil {
		return nil, 0, errorx.Wrapf(err, "enrich expt source from pipeline fail, workspace_id: %d", spaceID)
	}

	// Step 6: 合并 task 转换的模板 + DB 模板
	allTemplates := make([]*entity.ExptTemplate, 0, len(taskTemplates)+len(dbTemplates))
	allTemplates = append(allTemplates, taskTemplates...)
	allTemplates = append(allTemplates, dbTemplates...)

	// Step 7: 内存筛选
	filteredTemplates := e.applyTemplateFilters(allTemplates, filter)

	// Step 8: 内存排序
	e.applyTemplateOrderBy(filteredTemplates, orderBys)

	// Step 9: 内存分页
	total := int64(len(filteredTemplates))
	start := int((page - 1) * pageSize)
	end := start + int(pageSize)
	if start > len(filteredTemplates) {
		return []*entity.ExptTemplate{}, total, nil
	}
	if end > len(filteredTemplates) {
		end = len(filteredTemplates)
	}

	return filteredTemplates[start:end], total, nil
}

// taskToExptTemplate 将 task 转换为 ExptTemplate，直接使用 task.Rule.SpanFilters 和 task 中的 Scheduler
// 将 task.TaskConfig.AutoEvaluateConfigs 转为 ExptTemplate 的评估器配置
func taskToExptTemplate(task *taskdomain.Task, spaceID int64) *entity.ExptTemplate {
	if task == nil || task.ID == nil {
		return nil
	}
	taskID := strconv.FormatInt(*task.ID, 10)
	meta := &entity.ExptTemplateMeta{
		ID:          -*task.ID, // 使用负数区分 task 来源，避免与 DB 模板 ID 冲突
		WorkspaceID: spaceID,
		Name:        task.Name,
		Desc:        "",
		ExptType:    entity.ExptType_Online,
	}
	exptSource := &entity.ExptSource{
		SourceType: entity.SourceType_AutoTask,
		SourceID:   taskID,
	}
	if task.Rule != nil && task.Rule.SpanFilters != nil {
		exptSource.SpanFilterFields = spanFilterFieldsFromTaskRule(task.Rule.SpanFilters)
	}
	exptSource.Scheduler = taskRuleToExptScheduler(task.Rule)

	baseInfo := &entity.BaseInfo{}
	if task.BaseInfo != nil {
		baseInfo.CreatedAt = task.BaseInfo.CreatedAt
		baseInfo.UpdatedAt = task.BaseInfo.UpdatedAt
		if task.BaseInfo.CreatedBy != nil && task.BaseInfo.CreatedBy.IsSetUserID() {
			uid := task.BaseInfo.CreatedBy.GetUserID()
			baseInfo.CreatedBy = &entity.UserInfo{UserID: &uid}
		}
		if task.BaseInfo.UpdatedBy != nil && task.BaseInfo.UpdatedBy.IsSetUserID() {
			uid := task.BaseInfo.UpdatedBy.GetUserID()
			baseInfo.UpdatedBy = &entity.UserInfo{UserID: &uid}
		}
	}

	// 从 task.TaskConfig.AutoEvaluateConfigs 转为 ExptTemplate 的 TripleConfig 和 TemplateConf.ConnectorConf.EvaluatorsConf
	tripleConfig, connectorConf := autoEvaluateConfigsToExptTemplateConf(task.TaskConfig)

	return &entity.ExptTemplate{
		Meta:         meta,
		TripleConfig: tripleConfig,
		ExptSource:   exptSource,
		BaseInfo:     baseInfo,
		TemplateConf: &entity.ExptTemplateConfiguration{
			ExptSource:    exptSource,
			ConnectorConf: connectorConf,
		},
	}
}

// autoEvaluateConfigsToExptTemplateConf 将 task.TaskConfig.AutoEvaluateConfigs 转为 TripleConfig 和 ConnectorConf
func autoEvaluateConfigsToExptTemplateConf(taskConfig *taskdomain.TaskConfig) (*entity.ExptTemplateTuple, entity.Connector) {
	connector := entity.Connector{}
	if taskConfig == nil || len(taskConfig.AutoEvaluateConfigs) == 0 {
		return nil, connector
	}

	evaluatorConfs := make([]*entity.EvaluatorConf, 0, len(taskConfig.AutoEvaluateConfigs))
	evaluatorIDVersionItems := make([]*entity.EvaluatorIDVersionItem, 0, len(taskConfig.AutoEvaluateConfigs))

	for _, cfg := range taskConfig.AutoEvaluateConfigs {
		if cfg == nil || cfg.EvaluatorVersionID <= 0 {
			continue
		}
		ec := &entity.EvaluatorConf{
			EvaluatorVersionID: cfg.EvaluatorVersionID,
			EvaluatorID:        cfg.EvaluatorID,
			Version:            "", // Task 无 Version 字段，留空由后续回填
			IngressConf:        evaluateFieldMappingsToIngressConf(cfg.FieldMappings),
		}
		evaluatorConfs = append(evaluatorConfs, ec)
		evaluatorIDVersionItems = append(evaluatorIDVersionItems, &entity.EvaluatorIDVersionItem{
			EvaluatorID:        cfg.EvaluatorID,
			EvaluatorVersionID: cfg.EvaluatorVersionID,
			Version:            "",
			ScoreWeight:        1.0,
		})
	}

	if len(evaluatorConfs) == 0 {
		return nil, connector
	}

	connector.EvaluatorsConf = &entity.EvaluatorsConf{
		EvaluatorConf: evaluatorConfs,
	}

	tripleConfig := &entity.ExptTemplateTuple{
		EvaluatorIDVersionItems: evaluatorIDVersionItems,
		EvaluatorVersionIds:     extractEvaluatorVersionIDs(evaluatorIDVersionItems),
	}
	return tripleConfig, connector
}

// evaluateFieldMappingsToIngressConf 将 task.EvaluateFieldMapping 转为 EvaluatorIngressConf
// Task 的字段映射来自 trace，使用 EvalSetAdapter 承载（trace 数据会流入 eval set）
func evaluateFieldMappingsToIngressConf(mappings []*taskdomain.EvaluateFieldMapping) *entity.EvaluatorIngressConf {
	if len(mappings) == 0 {
		return &entity.EvaluatorIngressConf{
			EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
		}
	}
	fieldConfs := make([]*entity.FieldConf, 0, len(mappings))
	for _, m := range mappings {
		if m == nil {
			continue
		}
		fieldName := ""
		if m.FieldSchema != nil && m.FieldSchema.Key != nil {
			fieldName = *m.FieldSchema.Key
		} else if m.FieldSchema != nil && m.FieldSchema.Name != nil {
			fieldName = *m.FieldSchema.Name
		}
		if fieldName == "" {
			continue
		}
		// FromField: trace 来源使用 trace_field_key，eval_set 来源使用 eval_set_name
		fromField := m.TraceFieldKey
		if m.EvalSetName != nil && *m.EvalSetName != "" {
			fromField = *m.EvalSetName
		}
		fieldConfs = append(fieldConfs, &entity.FieldConf{
			FieldName: fieldName,
			FromField: fromField,
			Value:     "",
		})
	}
	return &entity.EvaluatorIngressConf{
		EvalSetAdapter: &entity.FieldAdapter{FieldConfs: fieldConfs},
	}
}

func extractEvaluatorVersionIDs(items []*entity.EvaluatorIDVersionItem) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		if item != nil && item.EvaluatorVersionID > 0 {
			ids = append(ids, item.EvaluatorVersionID)
		}
	}
	return ids
}

// taskRuleToExptScheduler 将 task.Rule (Sampler + EffectiveTime) 转为 entity.ExptSchedulerDO
// 参考 convertScheduler/convertFrequency 逻辑
func taskRuleToExptScheduler(rule *taskdomain.Rule) *entity.ExptSchedulerDO {
	if rule == nil {
		return nil
	}
	sampler := rule.Sampler
	effectiveTime := rule.EffectiveTime
	if sampler == nil && effectiveTime == nil {
		return nil
	}
	out := &entity.ExptSchedulerDO{}
	if sampler != nil {
		out.Enabled = sampler.IsCycle
		if sampler.IsCycle != nil && *sampler.IsCycle {
			if freq := convertTaskFrequency(sampler, effectiveTime); freq != nil {
				out.Frequency = freq
			}
		}
	}
	if effectiveTime != nil {
		if effectiveTime.StartAt != nil && *effectiveTime.StartAt != 0 {
			out.StartTime = effectiveTime.StartAt
		}
		if effectiveTime.EndAt != nil && *effectiveTime.EndAt != 0 {
			out.EndTime = effectiveTime.EndAt
		}
	}
	if out.Enabled == nil && out.Frequency == nil && out.TriggerAt == nil && out.StartTime == nil && out.EndTime == nil {
		return nil
	}
	return out
}

// convertTaskFrequency 根据 task.Sampler.CycleTimeUnit 和 EffectiveTime 计算 Frequency
func convertTaskFrequency(sampler *taskdomain.Sampler, effectiveTime *taskdomain.EffectiveTime) *string {
	if sampler == nil || sampler.IsCycle == nil || !*sampler.IsCycle {
		return nil
	}
	cycleTimeUnit := ""
	if sampler.CycleTimeUnit != nil {
		cycleTimeUnit = *sampler.CycleTimeUnit
	}
	switch cycleTimeUnit {
	case taskdomain.TimeUnitDay, taskdomain.TimeUnitNull, "":
		f := "every_day"
		return &f
	case taskdomain.TimeUnitWeek:
		if effectiveTime == nil || effectiveTime.StartAt == nil || *effectiveTime.StartAt == 0 {
			return nil
		}
		wd := time.UnixMilli(*effectiveTime.StartAt).Weekday()
		var f string
		switch wd {
		case time.Monday:
			f = "monday"
		case time.Tuesday:
			f = "tuesday"
		case time.Wednesday:
			f = "wednesday"
		case time.Thursday:
			f = "thursday"
		case time.Friday:
			f = "friday"
		case time.Saturday:
			f = "saturday"
		case time.Sunday:
			f = "sunday"
		default:
			return nil
		}
		return &f
	default:
		return nil
	}
}

// spanFilterFieldsFromTaskRule 将 task.Rule.SpanFilters (filter.SpanFilterFields) 转为 entity.SpanFilterFieldsDO
func spanFilterFieldsFromTaskRule(sf *taskfilter.SpanFilterFields) *entity.SpanFilterFieldsDO {
	if sf == nil {
		return nil
	}
	do := &entity.SpanFilterFieldsDO{}
	if sf.PlatformType != nil {
		s := string(*sf.PlatformType)
		do.PlatformType = &s
	}
	if sf.SpanListType != nil {
		s := string(*sf.SpanListType)
		do.SpanListType = &s
	}
	if sf.Filters != nil {
		do.Filters = filterFieldsFromTaskRule(sf.Filters)
	}
	return do
}

// filterFieldsFromTaskRule 将 filter.FilterFields 转为 entity.FilterFieldsDO
func filterFieldsFromTaskRule(ff *taskfilter.FilterFields) *entity.FilterFieldsDO {
	if ff == nil {
		return nil
	}
	do := &entity.FilterFieldsDO{}
	if ff.QueryAndOr != nil {
		s := string(*ff.QueryAndOr)
		do.QueryAndOr = &s
	}
	if len(ff.FilterFields) > 0 {
		do.FilterFields = make([]*entity.FilterFieldDO, 0, len(ff.FilterFields))
		for _, f := range ff.FilterFields {
			if fd := filterFieldFromTaskRule(f); fd != nil {
				do.FilterFields = append(do.FilterFields, fd)
			}
		}
	}
	return do
}

// filterFieldFromTaskRule 将 filter.FilterField 转为 entity.FilterFieldDO
func filterFieldFromTaskRule(f *taskfilter.FilterField) *entity.FilterFieldDO {
	if f == nil {
		return nil
	}
	fd := &entity.FilterFieldDO{
		FieldName: f.FieldName,
		Values:    f.Values,
	}
	if f.FieldType != nil {
		s := string(*f.FieldType)
		fd.FieldType = &s
	}
	if f.QueryType != nil {
		s := string(*f.QueryType)
		fd.QueryType = &s
	}
	if f.QueryAndOr != nil {
		s := string(*f.QueryAndOr)
		fd.QueryAndOr = &s
	}
	if f.SubFilter != nil {
		fd.SubFilter = filterFieldsFromTaskRule(f.SubFilter)
	}
	return fd
}

// enrichExptSourceFromPipeline 对在线实验模板，根据 source_id 和 space_id 调用 ListPipeline，
// 从 Flow 中 node_template_type=data_reflow 的节点提取 task.rule.span_filters 到 ExptSource.SpanFilterFields，
// 提取 Pipeline.Scheduler 到 ExptSource.Scheduler
func (e *ExptTemplateManagerImpl) enrichExptSourceFromPipeline(ctx context.Context, templates []*entity.ExptTemplate, spaceID int64) error {
	if e.pipelineRPCAdapter == nil {
		return nil
	}
	// 收集需要查询的 pipeline ID（source_id 解析为 int64）
	pipelineIDs := make([]int64, 0)
	templateByPipelineID := make(map[int64][]*entity.ExptTemplate)
	for _, t := range templates {
		if t.ExptSource == nil || t.ExptSource.SourceType != entity.SourceType_AutoTask || t.ExptSource.SourceID == "" {
			continue
		}
		pid, err := strconv.ParseInt(t.ExptSource.SourceID, 10, 64)
		if err != nil || pid <= 0 {
			continue
		}
		pipelineIDs = append(pipelineIDs, pid)
		templateByPipelineID[pid] = append(templateByPipelineID[pid], t)
	}
	if len(pipelineIDs) == 0 {
		return nil
	}
	// 去重
	pipelineIDs = gslice.Uniq(pipelineIDs)

	resp, err := e.pipelineRPCAdapter.ListPipelineFlow(ctx, &rpc.ListPipelineFlowRequest{
		SpaceID:    gptr.Of(spaceID),
		IDList:     pipelineIDs,
		WithDetail: true,
	})
	if err != nil {
		return err
	}
	if resp == nil || len(resp.Items) == 0 {
		return nil
	}

	for _, p := range resp.Items {
		if p == nil || p.ID == nil {
			continue
		}
		pid := *p.ID
		targets := templateByPipelineID[pid]
		if len(targets) == 0 {
			continue
		}
		spanFilterFields := extractSpanFilterFieldsFromPipeline(p)
		scheduler := extractSchedulerFromPipeline(p)
		for _, tpl := range targets {
			if tpl.ExptSource != nil {
				tpl.ExptSource.SpanFilterFields = spanFilterFields
				tpl.ExptSource.Scheduler = scheduler
			}
		}
	}
	return nil
}

// extractSpanFilterFieldsFromPipeline 从 Pipeline Flow 中 node_template_type=data_reflow 的节点提取 span_filters
func extractSpanFilterFieldsFromPipeline(p *entity.Pipeline) *entity.SpanFilterFieldsDO {
	if p == nil || p.Flow == nil || len(p.Flow.Nodes) == 0 {
		return nil
	}
	for _, node := range p.Flow.Nodes {
		if node == nil || string(node.NodeTemplateType) != "data_reflow" {
			continue
		}
		if node.Refs == nil {
			continue
		}
		taskRef, ok := node.Refs["task"]
		if !ok || taskRef == nil || taskRef.Content == "" {
			continue
		}
		return parseSpanFilterFieldsFromTaskJSON(taskRef.Content)
	}
	return nil
}

// taskRuleJSON 解析 task JSON 中的 rule.span_filters 结构
type taskRuleSpanFiltersJSON struct {
	Rule *struct {
		SpanFilters *spanFiltersJSON `json:"span_filters"`
	} `json:"rule"`
}

type spanFiltersJSON struct {
	SpanListType *string      `json:"span_list_type"`
	PlatformType *string      `json:"platform_type"`
	Filters      *filtersJSON `json:"filters"`
}

type filtersJSON struct {
	QueryAndOr   *string            `json:"query_and_or"`
	FilterFields []*filterFieldJSON `json:"filter_fields"`
}

type filterFieldJSON struct {
	FieldName  *string      `json:"field_name"`
	FieldType  *string      `json:"field_type"`
	Values     []string     `json:"values"`
	QueryType  *string      `json:"query_type"`
	QueryAndOr *string      `json:"query_and_or"`
	SubFilter  *filtersJSON `json:"sub_filter"`
}

func parseSpanFilterFieldsFromTaskJSON(content string) *entity.SpanFilterFieldsDO {
	var parsed taskRuleSpanFiltersJSON
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil
	}
	if parsed.Rule == nil || parsed.Rule.SpanFilters == nil {
		return nil
	}
	sf := parsed.Rule.SpanFilters
	result := &entity.SpanFilterFieldsDO{
		PlatformType: sf.PlatformType,
		SpanListType: sf.SpanListType,
	}
	if sf.Filters != nil {
		result.Filters = &entity.FilterFieldsDO{
			QueryAndOr: sf.Filters.QueryAndOr,
		}
		if len(sf.Filters.FilterFields) > 0 {
			result.Filters.FilterFields = make([]*entity.FilterFieldDO, 0, len(sf.Filters.FilterFields))
			for _, ff := range sf.Filters.FilterFields {
				if ff == nil {
					continue
				}
				fd := &entity.FilterFieldDO{
					FieldName:  ff.FieldName,
					FieldType:  ff.FieldType,
					Values:     ff.Values,
					QueryType:  ff.QueryType,
					QueryAndOr: ff.QueryAndOr,
				}
				if ff.SubFilter != nil {
					fd.SubFilter = &entity.FilterFieldsDO{
						QueryAndOr:   ff.SubFilter.QueryAndOr,
						FilterFields: nil, // 递归简化，暂不展开
					}
				}
				result.Filters.FilterFields = append(result.Filters.FilterFields, fd)
			}
		}
	}
	return result
}

// extractSchedulerFromPipeline 从 Pipeline 提取 Scheduler
func extractSchedulerFromPipeline(p *entity.Pipeline) *entity.ExptSchedulerDO {
	if p == nil || p.Scheduler == nil {
		return nil
	}
	s := p.Scheduler
	return &entity.ExptSchedulerDO{
		Enabled:   s.Enabled,
		Frequency: s.Frequency,
		TriggerAt: s.TriggerAt,
		StartTime: s.StartTime,
		EndTime:   s.EndTime,
	}
}

// applyTemplateFilters 应用筛选条件
func (e *ExptTemplateManagerImpl) applyTemplateFilters(templates []*entity.ExptTemplate, filters *entity.ExptTemplateListFilter) []*entity.ExptTemplate {
	if filters == nil {
		return templates
	}

	var result []*entity.ExptTemplate
	for _, template := range templates {
		if e.matchesTemplateFilter(template, filters) {
			result = append(result, template)
		}
	}
	return result
}

// matchesTemplateFilter 检查模板是否匹配筛选条件
func (e *ExptTemplateManagerImpl) matchesTemplateFilter(template *entity.ExptTemplate, filters *entity.ExptTemplateListFilter) bool {
	if filters == nil {
		return true
	}

	includes := filters.Includes
	excludes := filters.Excludes

	// 检查 Includes
	if includes != nil {
		// CreatedBy
		if len(includes.CreatedBy) > 0 {
			createdBy := ""
			if template.BaseInfo != nil && template.BaseInfo.CreatedBy != nil && template.BaseInfo.CreatedBy.UserID != nil {
				createdBy = *template.BaseInfo.CreatedBy.UserID
			}
			if !slices.Contains(includes.CreatedBy, createdBy) {
				return false
			}
		}

		// UpdatedBy
		if len(includes.UpdatedBy) > 0 {
			updatedBy := ""
			if template.BaseInfo != nil && template.BaseInfo.UpdatedBy != nil && template.BaseInfo.UpdatedBy.UserID != nil {
				updatedBy = *template.BaseInfo.UpdatedBy.UserID
			}
			if !slices.Contains(includes.UpdatedBy, updatedBy) {
				return false
			}
		}

		// EvalSetIDs
		if len(includes.EvalSetIDs) > 0 {
			evalSetID := template.GetEvalSetID()
			if !slices.Contains(includes.EvalSetIDs, evalSetID) {
				return false
			}
		}

		// TargetIDs
		if len(includes.TargetIDs) > 0 {
			targetID := template.GetTargetID()
			if !slices.Contains(includes.TargetIDs, targetID) {
				return false
			}
		}

		// EvaluatorIDs
		if len(includes.EvaluatorIDs) > 0 {
			// 需要查询评估器ID，这里简化处理，暂时跳过这个筛选条件
			// TODO: 实现评估器ID的精确匹配
		}

		// TargetType
		if len(includes.TargetType) > 0 {
			targetType := int64(template.GetTargetType())
			if !slices.Contains(includes.TargetType, targetType) {
				return false
			}
		}

		// ExptType (应该都是 Online，但检查一下)
		if len(includes.ExptType) > 0 {
			exptType := int64(template.GetExptType())
			if !slices.Contains(includes.ExptType, exptType) {
				return false
			}
		}

		// FuzzyName
		if len(filters.FuzzyName) > 0 {
			name := template.Meta.Name
			if !strings.Contains(strings.ToLower(name), strings.ToLower(filters.FuzzyName)) {
				return false
			}
		}
	}

	// 检查 Excludes
	if excludes != nil {
		// CreatedBy
		if len(excludes.CreatedBy) > 0 {
			createdBy := ""
			if template.BaseInfo != nil && template.BaseInfo.CreatedBy != nil && template.BaseInfo.CreatedBy.UserID != nil {
				createdBy = *template.BaseInfo.CreatedBy.UserID
			}
			if slices.Contains(excludes.CreatedBy, createdBy) {
				return false
			}
		}

		// UpdatedBy
		if len(excludes.UpdatedBy) > 0 {
			updatedBy := ""
			if template.BaseInfo != nil && template.BaseInfo.UpdatedBy != nil && template.BaseInfo.UpdatedBy.UserID != nil {
				updatedBy = *template.BaseInfo.UpdatedBy.UserID
			}
			if slices.Contains(excludes.UpdatedBy, updatedBy) {
				return false
			}
		}

		// EvalSetIDs
		if len(excludes.EvalSetIDs) > 0 {
			evalSetID := template.GetEvalSetID()
			if slices.Contains(excludes.EvalSetIDs, evalSetID) {
				return false
			}
		}

		// TargetIDs
		if len(excludes.TargetIDs) > 0 {
			targetID := template.GetTargetID()
			if slices.Contains(excludes.TargetIDs, targetID) {
				return false
			}
		}

		// TargetType
		if len(excludes.TargetType) > 0 {
			targetType := int64(template.GetTargetType())
			if slices.Contains(excludes.TargetType, targetType) {
				return false
			}
		}

		// ExptType
		if len(excludes.ExptType) > 0 {
			exptType := int64(template.GetExptType())
			if slices.Contains(excludes.ExptType, exptType) {
				return false
			}
		}

		// CronActivate
		if len(excludes.CronActivate) > 0 {
			cv := int64(0)
			if template.ExptInfo != nil && template.ExptInfo.CronActivate {
				cv = 1
			}
			if slices.Contains(excludes.CronActivate, cv) {
				return false
			}
		}
	}

	return true
}

// applyTemplateOrderBy 应用排序
func (e *ExptTemplateManagerImpl) applyTemplateOrderBy(templates []*entity.ExptTemplate, orderBys []*entity.OrderBy) {
	if len(orderBys) == 0 {
		return
	}

	// 使用标准库的 sort.Slice 进行排序
	sort.Slice(templates, func(i, j int) bool {
		a, b := templates[i], templates[j]
		for _, orderBy := range orderBys {
			field := gptr.Indirect(orderBy.Field)
			isAsc := gptr.Indirect(orderBy.IsAsc)

			var cmp int
			switch field {
			case entity.OrderByUpdatedAt:
				updatedAtA := int64(0)
				updatedAtB := int64(0)
				if a.BaseInfo != nil && a.BaseInfo.UpdatedAt != nil {
					updatedAtA = *a.BaseInfo.UpdatedAt
				}
				if b.BaseInfo != nil && b.BaseInfo.UpdatedAt != nil {
					updatedAtB = *b.BaseInfo.UpdatedAt
				}
				if updatedAtA < updatedAtB {
					cmp = -1
				} else if updatedAtA > updatedAtB {
					cmp = 1
				}
			case entity.OrderByCreatedAt:
				createdAtA := int64(0)
				createdAtB := int64(0)
				if a.BaseInfo != nil && a.BaseInfo.CreatedAt != nil {
					createdAtA = *a.BaseInfo.CreatedAt
				}
				if b.BaseInfo != nil && b.BaseInfo.CreatedAt != nil {
					createdAtB = *b.BaseInfo.CreatedAt
				}
				if createdAtA < createdAtB {
					cmp = -1
				} else if createdAtA > createdAtB {
					cmp = 1
				}
			default:
				continue
			}

			if cmp != 0 {
				if !isAsc {
					return cmp > 0
				}
				return cmp < 0
			}
		}
		return false
	})
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
