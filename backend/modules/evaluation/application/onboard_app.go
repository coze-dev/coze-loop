// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/config"
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
	configer config.IOnboardConfiger,
	evaluationSetService service.IEvaluationSetService,
	evaluationSetVersionService service.EvaluationSetVersionService,
	evaluationSetItemService service.EvaluationSetItemService,
	evaluatorService service.EvaluatorService,
) expt.OnboardService {
	onboardApplicationOnce.Do(func() {
		onboardApplication = &OnboardApplicationImpl{
			auth:                        auth,
			configer:                    configer,
			evaluationSetService:        evaluationSetService,
			evaluationSetVersionService: evaluationSetVersionService,
			evaluationSetItemService:    evaluationSetItemService,
			evaluatorService:            evaluatorService,
		}
	})
	return onboardApplication
}

type OnboardApplicationImpl struct {
	auth                        rpc.IAuthProvider
	configer                    config.IOnboardConfiger
	evaluationSetService        service.IEvaluationSetService
	evaluationSetVersionService service.EvaluationSetVersionService
	evaluationSetItemService    service.EvaluationSetItemService
	evaluatorService            service.EvaluatorService
}

// Onboard 实现onboard接口：根据模板id从tcc读取配置，创建评测集，提交版本，创建评估器
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

	// 3. 从 tcc 读取配置
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
	o.setupEvaluationSetFlow(ctx, req.WorkspaceID, onboardConfig, sessionDO)

	// 6. 评估器相关流程（即使评测集创建失败，也尝试创建评估器）
	o.setupEvaluatorsFlow(ctx, req.WorkspaceID, templateID, onboardConfig)

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
func (o *OnboardApplicationImpl) setupEvaluationSetFlow(
	ctx context.Context,
	workspaceID int64,
	onboardConfig *config.OnboardTemplateConfig,
	sessionDO *entity.Session,
) {
	// 1. 构造评测集 schema 和业务类目
	schemaDO := buildEvaluationSetSchema(onboardConfig)
	bizCategory := buildBizCategory(onboardConfig)

	// 2. 创建评测集
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
		return
	}
	logs.CtxInfo(ctx, "created evaluation set with id: %d", evalSetID)

	// 3. 批量创建评测集 items
	itemsCreated := o.batchCreateEvaluationSetItems(ctx, workspaceID, evalSetID, onboardConfig)

	// 4. 创建评测集版本（仅在 items 创建成功时执行）
	if itemsCreated {
		o.createEvaluationSetVersion(ctx, workspaceID, evalSetID, onboardConfig)
	}
}

// buildEvaluationSetSchema 根据配置构建评测集 schema
func buildEvaluationSetSchema(onboardConfig *config.OnboardTemplateConfig) *entity.EvaluationSetSchema {
	if onboardConfig.EvaluationSet.EvaluationSetSchema == nil {
		return nil
	}
	return evaluation_set_convertor.SchemaDTO2DO(onboardConfig.EvaluationSet.EvaluationSetSchema)
}

// buildBizCategory 根据配置构建业务类目指针
func buildBizCategory(onboardConfig *config.OnboardTemplateConfig) *entity.BizCategory {
	if onboardConfig.EvaluationSet.BizCategory == "" {
		return nil
	}
	bizCategoryVal := onboardConfig.EvaluationSet.BizCategory
	return &bizCategoryVal
}

// batchCreateEvaluationSetItems 按配置批量创建评测集 items，返回是否创建成功
func (o *OnboardApplicationImpl) batchCreateEvaluationSetItems(
	ctx context.Context,
	workspaceID, evalSetID int64,
	onboardConfig *config.OnboardTemplateConfig,
) bool {
	if len(onboardConfig.EvaluationSet.Items) == 0 {
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
func (o *OnboardApplicationImpl) createEvaluationSetVersion(
	ctx context.Context,
	workspaceID, evalSetID int64,
	onboardConfig *config.OnboardTemplateConfig,
) {
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
		return
	}
	logs.CtxInfo(ctx, "created evaluation set version with id: %d", versionID)
}

// setupEvaluatorsFlow 按配置创建评估器列表
// 行为保持不变：单个评估器失败只记录日志并跳过，其余继续。
func (o *OnboardApplicationImpl) setupEvaluatorsFlow(
	ctx context.Context,
	workspaceID int64,
	templateID string,
	onboardConfig *config.OnboardTemplateConfig,
) {
	for i, evaluatorConfig := range onboardConfig.Evaluators {
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
	}
}

// buildEvaluatorDO 根据配置构建评估器DO
func (o *OnboardApplicationImpl) buildEvaluatorDO(ctx context.Context, workspaceID int64, cfg *config.OnboardEvaluatorConfig) (*entity.Evaluator, error) {
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
