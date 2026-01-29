// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	evaluation_set_convertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	evaluatorconvertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var (
	onboardApplicationOnce = sync.Once{}
	onboardApplication     expt.OnboardService
)

func NewOnboardApplicationImpl(
	auth rpc.IAuthProvider,
	configer conf.IOnboardConfiger,
	evaluationSetService service.IEvaluationSetService,
	evaluationSetVersionService service.EvaluationSetVersionService,
	evaluationSetItemService service.EvaluationSetItemService,
	evaluatorService service.EvaluatorService,
	templateManager service.IExptTemplateManager,
) expt.OnboardService {
	onboardApplicationOnce.Do(func() {
		onboardApplication = &OnboardApplicationImpl{
			auth:                        auth,
			configer:                    configer,
			evaluationSetService:        evaluationSetService,
			evaluationSetVersionService: evaluationSetVersionService,
			evaluationSetItemService:    evaluationSetItemService,
			evaluatorService:            evaluatorService,
			templateManager:             templateManager,
		}
	})
	return onboardApplication
}

type OnboardApplicationImpl struct {
	auth                        rpc.IAuthProvider
	configer                    conf.IOnboardConfiger
	evaluationSetService        service.IEvaluationSetService
	evaluationSetVersionService service.EvaluationSetVersionService
	evaluationSetItemService    service.EvaluationSetItemService
	evaluatorService            service.EvaluatorService
	templateManager             service.IExptTemplateManager
}

// Onboard 实现 OnboardService：根据模板ID从配置中心读取配置，创建评测集、评估器和实验模板
func (o *OnboardApplicationImpl) Onboard(ctx context.Context, req *expt.OnboardRequest) (resp *expt.OnboardResponse, err error) {
	// 1. 参数校验
	templateID, err := validateOnboardRequest(req)
	if err != nil {
		return nil, err
	}

	// 2. 鉴权
	if err = o.authorizeOnboard(ctx, req.WorkspaceID); err != nil {
		return nil, err
	}

	// 3. 从配置中心读取 Onboard 配置
	onboardConfig, err := o.configer.GetOnboardConfigByTemplateID(ctx, templateID)
	if err != nil {
		logs.CtxError(ctx, "failed to get onboard config for template_id %s: %v, skip onboard process", templateID, err)
		// 配置读取失败，直接返回成功，跳过后续流程
		return &expt.OnboardResponse{
			BaseResp: base.NewBaseResp(),
		}, nil
	}

	// 4. 创建 session
	sessionDO := o.buildSessionFromCtx(ctx)

	// 5. 评测集相关流程（创建评测集、items 和版本）
	evalSetID, evalSetVersionID := o.setupEvaluationSetFlow(ctx, req.WorkspaceID, onboardConfig, sessionDO)

	// 6. 评估器相关流程（即使评测集创建失败，也尝试创建评估器）
	evaluatorIDVersionItems, scoreWeights := o.setupEvaluatorsFlow(ctx, req.WorkspaceID, templateID, onboardConfig)

	// 7. 创建实验模板（基于创建的评测集、评估器和source_target_id）
	sourceTargetID := req.GetSourceTargetID()
	if evalSetID > 0 && evalSetVersionID > 0 && len(evaluatorIDVersionItems) > 0 && sourceTargetID != "" {
		o.setupExptTemplateFlow(ctx, req.WorkspaceID, sourceTargetID, evalSetID, evalSetVersionID, evaluatorIDVersionItems, scoreWeights, onboardConfig, sessionDO)
	}

	return &expt.OnboardResponse{
		BaseResp: base.NewBaseResp(),
	}, nil
}

// validateOnboardRequest 校验请求参数并返回模板 ID
func validateOnboardRequest(req *expt.OnboardRequest) (string, error) {
	if req == nil {
		return "", errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.WorkspaceID <= 0 {
		return "", errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("workspace_id is required"))
	}
	templateID := req.GetTemplateID()
	if templateID == "" {
		return "", errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("template_id is required"))
	}
	return templateID, nil
}

// authorizeOnboard 进行 Workspace 级创建评测集鉴权
func (o *OnboardApplicationImpl) authorizeOnboard(ctx context.Context, workspaceID int64) error {
	return o.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(workspaceID, 10),
		SpaceID:       workspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluationSet"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
}

// buildSessionFromCtx 从上下文中构造 session 对象
func (o *OnboardApplicationImpl) buildSessionFromCtx(ctx context.Context) *entity.Session {
	userID := session.UserIDInCtxOrEmpty(ctx)
	return &entity.Session{
		UserID: userID,
	}
}

// setupEvaluationSetFlow 负责评测集全流程：创建评测集 -> 批量新增 items -> 创建版本
// 行为与原 Onboard 方法保持一致：任一步失败都会跳过后续评测集步骤，但不会影响评估器流程。
// 返回创建的评测集ID和版本ID（如果创建失败则返回0）
func (o *OnboardApplicationImpl) setupEvaluationSetFlow(
	ctx context.Context,
	workspaceID int64,
	onboardConfig *conf.OnboardTemplateConfig,
	sessionDO *entity.Session,
) (evalSetID, evalSetVersionID int64) {
	// 1. 检查评测集配置是否存在
	if onboardConfig.EvaluationSet == nil {
		logs.CtxError(ctx, "evaluation set config is nil, skip evaluation set related steps and proceed to evaluator creation")
		return 0, 0
	}

	// 2. 尝试复用同名评测集（按 Name + SpaceID 唯一）
	if existingID, existingVersionID, ok := o.findExistingEvaluationSetByName(ctx, workspaceID, onboardConfig.EvaluationSet.Name); ok {
		logs.CtxInfo(ctx, "reusing existing evaluation set, id: %d, version_id: %d, name: %s", existingID, existingVersionID, onboardConfig.EvaluationSet.Name)
		return existingID, existingVersionID
	}

	// 3. 构造评测集 schema 和业务类目
	schemaDO := buildEvaluationSetSchema(onboardConfig)
	bizCategory := buildBizCategory(onboardConfig)

	// 4. 创建评测集
	evalSetID, err := o.evaluationSetService.CreateEvaluationSet(ctx, &entity.CreateEvaluationSetParam{
		SpaceID:             workspaceID,
		Name:                onboardConfig.EvaluationSet.Name,
		Description:         gptr.Of(onboardConfig.EvaluationSet.Description),
		EvaluationSetSchema: schemaDO,
		BizCategory:         bizCategory,
		Session:             sessionDO,
	})
	if err != nil {
		logs.CtxError(ctx, "failed to create evaluation set: %v, skip evaluation set related steps and proceed to evaluator creation", err)
		return 0, 0
	}
	logs.CtxInfo(ctx, "created evaluation set with id: %d", evalSetID)

	// 5. 批量创建评测集 items
	itemsCreated := o.batchCreateEvaluationSetItems(ctx, workspaceID, evalSetID, onboardConfig)

	// 6. 创建评测集版本（仅在 items 创建成功时执行）
	if itemsCreated {
		versionID := o.createEvaluationSetVersion(ctx, workspaceID, evalSetID, onboardConfig)
		return evalSetID, versionID
	}
	return evalSetID, 0
}

// findExistingEvaluationSetByName 尝试按名称在当前空间中复用已存在的评测集
func (o *OnboardApplicationImpl) findExistingEvaluationSetByName(ctx context.Context, workspaceID int64, name string) (evalSetID, evalSetVersionID int64, ok bool) {
	if name == "" {
		return 0, 0, false
	}
	pageNum := int32(1)
	pageSize := int32(50)
	// 按更新时间倒序，优先拉取最近更新的一批同名评测集
	orderBys := []*entity.OrderBy{
		{
			Field: gptr.Of(entity.OrderByUpdatedAt),
			IsAsc: gptr.Of(false),
		},
	}
	param := &entity.ListEvaluationSetsParam{
		SpaceID:    workspaceID,
		Name:       gptr.Of(name),
		PageNumber: &pageNum,
		PageSize:   &pageSize,
		OrderBys:   orderBys,
	}
	sets, _, _, err := o.evaluationSetService.ListEvaluationSets(ctx, param)
	if err != nil || len(sets) == 0 {
		return 0, 0, false
	}

	// 先选出最近更新的评测集
	latestSet := sets[0]
	if latestSet == nil || latestSet.ID == 0 {
		return 0, 0, false
	}

	// 使用 ListEvaluationSetVersions 查询该评测集的所有版本，并按创建时间倒排
	var (
		allVersions []*entity.EvaluationSetVersion
		pageToken   *string
		versions    []*entity.EvaluationSetVersion
		nextCursor  *string
	)
	verPageSize := int32(50)
	pageNum = int32(1)
	for {
		verParam := &entity.ListEvaluationSetVersionsParam{
			SpaceID:         workspaceID,
			EvaluationSetID: latestSet.ID,
			PageToken:       pageToken,
			PageSize:        &verPageSize,
			PageNumber:      &pageNum,
		}
		versions, _, nextCursor, err = o.evaluationSetVersionService.ListEvaluationSetVersions(ctx, verParam)
		if err != nil {
			// 查询失败时，只复用评测集本身
			return latestSet.ID, 0, true
		}
		if len(versions) > 0 {
			allVersions = append(allVersions, versions...)
		}
		if nextCursor == nil || *nextCursor == "" {
			break
		}
		pageToken = nextCursor
		pageNum++
	}

	if len(allVersions) == 0 {
		// 查不到版本时，只复用评测集本身
		return latestSet.ID, 0, true
	}

	// 按创建时间倒排（CreatedAt 可能为空，视为 0）
	sort.Slice(allVersions, func(i, j int) bool {
		var ci, cj int64
		if allVersions[i].BaseInfo != nil && allVersions[i].BaseInfo.CreatedAt != nil {
			ci = *allVersions[i].BaseInfo.CreatedAt
		}
		if allVersions[j].BaseInfo != nil && allVersions[j].BaseInfo.CreatedAt != nil {
			cj = *allVersions[j].BaseInfo.CreatedAt
		}
		return ci > cj
	})

	return latestSet.ID, allVersions[0].ID, true
}

// buildEvaluationSetSchema 根据配置构建评测集 schema
func buildEvaluationSetSchema(onboardConfig *conf.OnboardTemplateConfig) *entity.EvaluationSetSchema {
	if onboardConfig.EvaluationSet == nil || onboardConfig.EvaluationSet.EvaluationSetSchema == nil {
		return nil
	}
	return evaluation_set_convertor.SchemaDTO2DO(onboardConfig.EvaluationSet.EvaluationSetSchema)
}

// buildBizCategory 根据配置构建业务类目指针
func buildBizCategory(onboardConfig *conf.OnboardTemplateConfig) *entity.BizCategory {
	if onboardConfig.EvaluationSet == nil || onboardConfig.EvaluationSet.BizCategory == "" {
		return nil
	}
	bizCategoryVal := onboardConfig.EvaluationSet.BizCategory
	return &bizCategoryVal
}

// batchCreateEvaluationSetItems 按配置批量创建评测集 items，返回是否创建成功
func (o *OnboardApplicationImpl) batchCreateEvaluationSetItems(
	ctx context.Context,
	workspaceID, evalSetID int64,
	onboardConfig *conf.OnboardTemplateConfig,
) bool {
	if onboardConfig.EvaluationSet == nil || len(onboardConfig.EvaluationSet.Items) == 0 {
		return true
	}

	// 将 DTO 转换为 DO
	items := make([]*entity.EvaluationSetItem, 0, len(onboardConfig.EvaluationSet.Items))
	for _, itemDTO := range onboardConfig.EvaluationSet.Items {
		itemDO := evaluation_set_convertor.ItemDTO2DO(itemDTO)
		if itemDO == nil {
			continue
		}
		itemDO.SpaceID = workspaceID
		itemDO.EvaluationSetID = evalSetID
		items = append(items, itemDO)
	}

	if len(items) == 0 {
		return true
	}

	skipInvalid := gptr.Of(onboardConfig.EvaluationSet.SkipInvalidItems)
	if !onboardConfig.EvaluationSet.SkipInvalidItems {
		skipInvalid = gptr.Of(true) // 默认 true
	}
	allowPartial := gptr.Of(onboardConfig.EvaluationSet.AllowPartialAdd)
	if !onboardConfig.EvaluationSet.AllowPartialAdd {
		allowPartial = gptr.Of(true) // 默认 true
	}

	_, errors, _, err := o.evaluationSetItemService.BatchCreateEvaluationSetItems(ctx, &entity.BatchCreateEvaluationSetItemsParam{
		SpaceID:          workspaceID,
		EvaluationSetID:  evalSetID,
		Items:            items,
		SkipInvalidItems: skipInvalid,
		AllowPartialAdd:  allowPartial,
	})
	if err != nil {
		logs.CtxError(ctx, "failed to batch create evaluation set items: %v, skip version creation and proceed to evaluator creation", err)
		return false
	}
	if len(errors) > 0 {
		logs.CtxWarn(ctx, "some items failed to create, errors: %v, skip version creation and proceed to evaluator creation", errors)
		return false
	}

	logs.CtxInfo(ctx, "batch created %d evaluation set items", len(items))
	return true
}

// createEvaluationSetVersion 创建评测集版本，失败只记录日志，不影响后续评估器流程
// 返回创建的版本ID（如果创建失败则返回0）
func (o *OnboardApplicationImpl) createEvaluationSetVersion(
	ctx context.Context,
	workspaceID, evalSetID int64,
	onboardConfig *conf.OnboardTemplateConfig,
) int64 {
	versionDesc := onboardConfig.EvaluationSet.VersionDesc
	if versionDesc == "" {
		versionDesc = "onboard initial version"
	}
	versionID, err := o.evaluationSetVersionService.CreateEvaluationSetVersion(ctx, &entity.CreateEvaluationSetVersionParam{
		SpaceID:         workspaceID,
		EvaluationSetID: evalSetID,
		Version:         onboardConfig.EvaluationSet.Version,
		Description:     gptr.Of(versionDesc),
	})
	if err != nil {
		logs.CtxError(ctx, "failed to create evaluation set version: %v, proceed to evaluator creation", err)
		return 0
	}
	logs.CtxInfo(ctx, "created evaluation set version with id: %d", versionID)
	return versionID
}

// setupEvaluatorsFlow 按配置创建评估器列表
// 行为保持不变：单个评估器失败只记录日志并跳过，其余继续。
// 返回创建的评估器ID版本项列表（包含EvaluatorID、Version、EvaluatorVersionID）和评估器ID到权重的映射
func (o *OnboardApplicationImpl) setupEvaluatorsFlow(
	ctx context.Context,
	workspaceID int64,
	templateID string,
	onboardConfig *conf.OnboardTemplateConfig,
) ([]*entity.EvaluatorIDVersionItem, map[int64]float64) {
	evaluatorIDVersionItems := make([]*entity.EvaluatorIDVersionItem, 0)
	scoreWeights := make(map[int64]float64) // evaluatorID -> scoreWeight
	for i, evaluatorConfig := range onboardConfig.Evaluators {
		// 1. 优先复用同名 + 同版本评估器（按 Name + Version + SpaceID）
		if existingVer, ok := o.findExistingEvaluatorVersion(ctx, workspaceID, evaluatorConfig); ok {
			item := &entity.EvaluatorIDVersionItem{
				EvaluatorID:        existingVer.GetEvaluatorID(),
				Version:            existingVer.GetVersion(),
				EvaluatorVersionID: existingVer.GetEvaluatorVersionID(),
			}
			evaluatorIDVersionItems = append(evaluatorIDVersionItems, item)
			if evaluatorConfig.ScoreWeight != nil && *evaluatorConfig.ScoreWeight > 0 {
				scoreWeights[item.EvaluatorID] = *evaluatorConfig.ScoreWeight
			}
			logs.CtxInfo(ctx, "reusing existing evaluator, id: %d, version_id: %d, name: %s, version: %s",
				item.EvaluatorID, item.EvaluatorVersionID, evaluatorConfig.Name, evaluatorConfig.Version)
			continue
		}

		// 2. 不存在则创建新的评估器
		evaluatorDO, err := o.buildEvaluatorDO(ctx, workspaceID, evaluatorConfig)
		if err != nil {
			logs.CtxError(ctx, "failed to build evaluator DO for evaluator %d: %v, skip this evaluator", i, err)
			continue
		}

		evaluatorID, err := o.evaluatorService.CreateEvaluator(ctx, evaluatorDO, fmt.Sprintf("onboard_%s_%d", templateID, i))
		if err != nil {
			logs.CtxError(ctx, "failed to create evaluator %d: %v, skip this evaluator", i, err)
			continue
		}
		logs.CtxInfo(ctx, "created evaluator with id: %d", evaluatorID)

		// 获取创建的评估器元信息以获取 LatestVersion
		evaluators, err := o.evaluatorService.BatchGetEvaluator(ctx, workspaceID, []int64{evaluatorID}, false)
		if err != nil || len(evaluators) == 0 {
			logs.CtxWarn(ctx, "failed to get evaluator %d after creation: %v, skip adding to template", evaluatorID, err)
			continue
		}
		evaluatorMeta := evaluators[0]
		if evaluatorMeta == nil {
			logs.CtxWarn(ctx, "evaluator %d is nil after creation, skip adding to template", evaluatorID)
			continue
		}

		// 使用 LatestVersion 获取最新已提交版本（而不是 draft 版本）
		if evaluatorMeta.LatestVersion == "" {
			logs.CtxWarn(ctx, "evaluator %d has no latest version, skip adding to template", evaluatorID)
			continue
		}
		pairs := [][2]interface{}{{evaluatorID, evaluatorMeta.LatestVersion}}
		versions, err := o.evaluatorService.BatchGetEvaluatorByIDAndVersion(ctx, pairs)
		if err != nil || len(versions) == 0 || versions[0] == nil {
			logs.CtxWarn(ctx, "failed to get latest version for evaluator %d: %v, skip adding to template", evaluatorID, err)
			continue
		}
		evaluator := versions[0]

		// 构建 EvaluatorIDVersionItem
		item := &entity.EvaluatorIDVersionItem{
			EvaluatorID:        evaluator.GetEvaluatorID(),
			Version:            evaluator.GetVersion(),
			EvaluatorVersionID: evaluator.GetEvaluatorVersionID(),
		}
		evaluatorIDVersionItems = append(evaluatorIDVersionItems, item)

		// 保存权重信息（如果配置中有）
		if evaluatorConfig.ScoreWeight != nil && *evaluatorConfig.ScoreWeight > 0 {
			scoreWeights[evaluatorID] = *evaluatorConfig.ScoreWeight
		}
	}
	return evaluatorIDVersionItems, scoreWeights
}

// findExistingEvaluatorVersion 按名称在当前空间中查找已存在的评估器，并返回最新已提交版本
func (o *OnboardApplicationImpl) findExistingEvaluatorVersion(ctx context.Context, workspaceID int64, cfg *conf.OnboardEvaluatorConfig) (*entity.Evaluator, bool) {
	if cfg == nil || cfg.Name == "" {
		return nil, false
	}

	// 1. 先按名称在当前空间中查找评估器元信息，按更新时间倒序保证取到最新的同名评估器
	orderBys := []*entity.OrderBy{
		{
			Field: gptr.Of(entity.OrderByUpdatedAt),
			IsAsc: gptr.Of(false),
		},
	}
	listReq := &entity.ListEvaluatorRequest{
		SpaceID:     workspaceID,
		SearchName:  cfg.Name,
		PageSize:    1,
		PageNum:     1,
		OrderBys:    orderBys,
		WithVersion: false,
	}
	evals, _, err := o.evaluatorService.ListEvaluator(ctx, listReq)
	if err != nil || len(evals) == 0 || evals[0] == nil {
		return nil, false
	}
	meta := evals[0]

	// 2. 使用 LatestVersion 获取最新已提交版本（而不是 draft 版本）
	if meta.LatestVersion == "" {
		return nil, false
	}
	pairs := [][2]interface{}{{meta.ID, meta.LatestVersion}}
	versions, err := o.evaluatorService.BatchGetEvaluatorByIDAndVersion(ctx, pairs)
	if err != nil || len(versions) == 0 || versions[0] == nil {
		return nil, false
	}
	return versions[0], true
}

// buildEvaluatorDO 根据配置构建评估器DO
func (o *OnboardApplicationImpl) buildEvaluatorDO(ctx context.Context, workspaceID int64, cfg *conf.OnboardEvaluatorConfig) (*entity.Evaluator, error) {
	userID := session.UserIDInCtxOrEmpty(ctx)

	// 构建评估器DTO
	evaluatorDTO := &evaluatordto.Evaluator{
		WorkspaceID:   gptr.Of(workspaceID),
		Name:          gptr.Of(cfg.Name),
		Description:   gptr.Of(cfg.Description),
		EvaluatorType: evaluatordto.EvaluatorTypePtr(cfg.Type),
		CurrentVersion: &evaluatordto.EvaluatorVersion{
			Version:          gptr.Of(cfg.Version),
			Description:      gptr.Of("onboard initial version"),
			EvaluatorContent: cfg.Content,
		},
	}

	// 转换为DO
	evaluatorDO, err := evaluatorconvertor.ConvertEvaluatorDTO2DO(evaluatorDTO)
	if err != nil {
		return nil, fmt.Errorf("failed to convert evaluator DTO to DO: %w", err)
	}

	// 设置基础信息
	evaluatorDO.BaseInfo = &entity.BaseInfo{
		CreatedBy: &entity.UserInfo{
			UserID: &userID,
		},
		UpdatedBy: &entity.UserInfo{
			UserID: &userID,
		},
	}

	return evaluatorDO, nil
}

// setupExptTemplateFlow 创建实验模板流程
// 基于创建的评测集、评估器和source_target_id，以及TCC中的模板配置创建实验模板
func (o *OnboardApplicationImpl) setupExptTemplateFlow(
	ctx context.Context,
	workspaceID int64,
	sourceTargetID string,
	evalSetID, evalSetVersionID int64,
	evaluatorIDVersionItems []*entity.EvaluatorIDVersionItem,
	scoreWeights map[int64]float64,
	onboardConfig *conf.OnboardTemplateConfig,
	sessionDO *entity.Session,
) {
	// 如果没有模板配置，跳过创建模板流程
	if onboardConfig.Template == nil {
		logs.CtxInfo(ctx, "no template config in onboard config, skip creating experiment template")
		return
	}

	templateConfig := onboardConfig.Template

	// 1. 构建 CreateEvalTargetParam，直接注入 source_target_id 和 target_type
	// 如果配置中指定了 eval_target_type，使用配置值；否则使用默认值
	evalTargetType := entity.EvalTargetTypeVolcengineAgent
	if templateConfig.EvalTargetType != nil {
		evalTargetType = entity.EvalTargetType(*templateConfig.EvalTargetType)
	}
	createEvalTargetParam := &entity.CreateEvalTargetParam{
		SourceTargetID: gptr.Of(sourceTargetID),
		EvalTargetType: gptr.Of(evalTargetType),
	}

	// 2. 构建 CreateExptTemplateParam
	param := &entity.CreateExptTemplateParam{
		SpaceID:                 workspaceID,
		Name:                    templateConfig.Name,
		Description:             templateConfig.Description,
		EvalSetID:               evalSetID,
		EvalSetVersionID:        evalSetVersionID,
		EvaluatorIDVersionItems: evaluatorIDVersionItems,
		ExptType:                entity.ExptType(templateConfig.ExptType),
		CreateEvalTargetParam:   createEvalTargetParam,
	}

	// 2.1 如果同名模板已经存在，则直接跳过创建（保持幂等），按当前逻辑视为成功
	if pass, err := o.templateManager.CheckName(ctx, param.Name, workspaceID, sessionDO); err == nil && !pass {
		logs.CtxInfo(ctx, "template %s already exists in workspace %d, skip creating", param.Name, workspaceID)
		return
	}

	// 3. 构建 TemplateConf（从 FieldMappingConfig）
	// 注意：targetVersionID 传入 0，因为 target 会在 service 层创建时自动填充
	param.TemplateConf = o.buildTemplateConfFromOnboardConfig(
		templateConfig,
		0, // targetVersionID 会在创建 target 后自动填充
		evaluatorIDVersionItems,
		scoreWeights,
	)

	// 5. 创建实验模板
	template, err := o.templateManager.Create(ctx, param, sessionDO)
	if err != nil {
		logs.CtxError(ctx, "failed to create experiment template: %v", err)
		return
	}
	logs.CtxInfo(ctx, "created experiment template with id: %d", template.GetID())
}

// buildTemplateConfFromOnboardConfig 从 onboard 配置构建 TemplateConf
func (o *OnboardApplicationImpl) buildTemplateConfFromOnboardConfig(
	templateConfig *conf.OnboardExptTemplateConfig,
	targetVersionID int64,
	evaluatorIDVersionItems []*entity.EvaluatorIDVersionItem,
	scoreWeights map[int64]float64,
) *entity.ExptTemplateConfiguration {
	templateConf := &entity.ExptTemplateConfiguration{
		ItemConcurNum:       convertInt32PtrToIntPtr(templateConfig.ItemConcurNum),
		EvaluatorsConcurNum: convertInt32PtrToIntPtr(templateConfig.EvaluatorsConcurNum),
	}

	// 构建 TargetConf
	var targetIngressConf *entity.TargetIngressConf
	if templateConfig.FieldMappingConfig != nil && templateConfig.FieldMappingConfig.TargetFieldMapping != nil {
		targetIngressConf = o.buildTargetIngressConf(templateConfig.FieldMappingConfig.TargetFieldMapping, templateConfig.FieldMappingConfig.TargetRuntimeParam)
	}

	// 构建 EvaluatorsConf
	var evaluatorConfs []*entity.EvaluatorConf
	if templateConfig.FieldMappingConfig != nil && len(templateConfig.FieldMappingConfig.EvaluatorFieldMapping) > 0 {
		evaluatorConfs = o.buildEvaluatorConfs(templateConfig.FieldMappingConfig.EvaluatorFieldMapping, evaluatorIDVersionItems, scoreWeights)
	}

	// 构建 ConnectorConf
	if targetIngressConf != nil || len(evaluatorConfs) > 0 {
		templateConf.ConnectorConf = entity.Connector{
			TargetConf: &entity.TargetConf{
				TargetVersionID: targetVersionID,
				IngressConf:     targetIngressConf,
			},
		}
		if len(evaluatorConfs) > 0 {
			templateConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{
				EvaluatorConf: evaluatorConfs,
			}
		}
	}

	return templateConf
}

// buildTargetIngressConf 构建 TargetIngressConf
func (o *OnboardApplicationImpl) buildTargetIngressConf(
	targetMapping *conf.OnboardTargetFieldMapping,
	runtimeParam *conf.OnboardRuntimeParam,
) *entity.TargetIngressConf {
	tic := &entity.TargetIngressConf{
		EvalSetAdapter: &entity.FieldAdapter{},
	}

	if targetMapping != nil {
		// 构建 FromEvalSet
		if len(targetMapping.FromEvalSet) > 0 {
			fc := make([]*entity.FieldConf, 0, len(targetMapping.FromEvalSet))
			for _, fm := range targetMapping.FromEvalSet {
				fc = append(fc, &entity.FieldConf{
					FieldName: fm.FieldName,
					FromField: fm.FromFieldName,
					Value:     fm.ConstValue,
				})
			}
			tic.EvalSetAdapter.FieldConfs = fc
		}

		// 注意：TargetIngressConf 没有 TargetAdapter 字段，只有 EvalSetAdapter 和 CustomConf
		// FromTarget 字段映射通常用于评估器，而不是目标本身
		// 如果需要支持 FromTarget，可能需要调整配置结构或使用其他方式
	}

	// 构建运行时参数
	if runtimeParam != nil && runtimeParam.JSONValue != "" {
		tic.CustomConf = &entity.FieldAdapter{
			FieldConfs: []*entity.FieldConf{
				{
					FieldName: "builtin_runtime_param",
					Value:     runtimeParam.JSONValue,
				},
			},
		}
	}

	return tic
}

// buildEvaluatorConfs 构建 EvaluatorConf 列表
func (o *OnboardApplicationImpl) buildEvaluatorConfs(
	evaluatorMappings []*conf.OnboardEvaluatorFieldMapping,
	evaluatorIDVersionItems []*entity.EvaluatorIDVersionItem,
	scoreWeights map[int64]float64,
) []*entity.EvaluatorConf {
	// 构建 evaluatorIDVersionID 映射（用于匹配字段映射）
	itemMap := make(map[string]*entity.EvaluatorIDVersionItem)
	for _, item := range evaluatorIDVersionItems {
		if item != nil && item.EvaluatorID > 0 && item.Version != "" {
			key := fmt.Sprintf("%d#%s", item.EvaluatorID, item.Version)
			itemMap[key] = item
		}
	}

	evaluatorConfs := make([]*entity.EvaluatorConf, 0, len(evaluatorMappings))
	for _, em := range evaluatorMappings {
		if em == nil {
			continue
		}

		// 查找对应的 EvaluatorIDVersionItem
		key := fmt.Sprintf("%d#%s", em.EvaluatorID, em.Version)
		item, ok := itemMap[key]
		if !ok {
			logs.CtxWarn(context.Background(), "evaluator mapping %s not found in created evaluators, skip", key)
			continue
		}

		// 构建 IngressConf
		ingressConf := &entity.EvaluatorIngressConf{}
		if len(em.FromEvalSet) > 0 {
			ingressConf.EvalSetAdapter = &entity.FieldAdapter{
				FieldConfs: make([]*entity.FieldConf, 0, len(em.FromEvalSet)),
			}
			for _, fm := range em.FromEvalSet {
				ingressConf.EvalSetAdapter.FieldConfs = append(ingressConf.EvalSetAdapter.FieldConfs, &entity.FieldConf{
					FieldName: fm.FieldName,
					FromField: fm.FromFieldName,
					Value:     fm.ConstValue,
				})
			}
		}
		if len(em.FromTarget) > 0 {
			ingressConf.TargetAdapter = &entity.FieldAdapter{
				FieldConfs: make([]*entity.FieldConf, 0, len(em.FromTarget)),
			}
			for _, fm := range em.FromTarget {
				ingressConf.TargetAdapter.FieldConfs = append(ingressConf.TargetAdapter.FieldConfs, &entity.FieldConf{
					FieldName: fm.FieldName,
					FromField: fm.FromFieldName,
					Value:     fm.ConstValue,
				})
			}
		}

		// 构建 EvaluatorConf
		conf := &entity.EvaluatorConf{
			EvaluatorID:        item.EvaluatorID,
			Version:            item.Version,
			EvaluatorVersionID: item.EvaluatorVersionID,
			IngressConf:        ingressConf,
		}

		// 应用评分权重（从评估器ID获取）
		if scoreWeights != nil {
			if w, ok := scoreWeights[item.EvaluatorID]; ok && w > 0 {
				conf.ScoreWeight = gptr.Of(w)
			}
		}

		evaluatorConfs = append(evaluatorConfs, conf)
	}

	return evaluatorConfs
}

// convertInt32PtrToIntPtr 将 *int32 转换为 *int
func convertInt32PtrToIntPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	val := int(*v)
	return &val
}
