// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/backoff"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/utils"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type IExperimentApplication interface {
	evaluation.ExperimentService
	service.ExptSchedulerEvent
	service.ExptItemEvalEvent
	service.ExptAggrResultService
	service.IExptResultExportService
	service.IExptInsightAnalysisService
	// StartDeadlineDispatcher 启动 Redis ZSET 到点派发 Worker（与 consumer 同进程时调用）
	StartDeadlineDispatcher(ctx context.Context)
}

type experimentApplication struct {
	idgen         idgen.IIDGenerator
	manager       service.IExptManager
	resultSvc     service.ExptResultService
	configer      component.IConfiger
	auth          rpc.IAuthProvider
	tagRPCAdapter rpc.ITagRPCAdapter

	service.ExptSchedulerEvent
	service.ExptItemEvalEvent
	service.ExptAggrResultService
	service.IExptResultExportService
	userInfoService userinfo.UserInfoService
	service.IExptInsightAnalysisService

	evalTargetService        service.IEvalTargetService
	evaluationSetItemService service.EvaluationSetItemService
	annotateService          service.IExptAnnotateService

	// 新增：EvaluatorService 用于查询内置评估器版本
	evaluatorService service.EvaluatorService

	// 实验模板管理服务
	templateManager service.IExptTemplateManager

	// 到点派发 Worker，与 consumer 同进程时启动
	deadlineDispatcher *service.ExptDeadlineDispatcher
}

func NewExperimentApplication(
	aggResultSvc service.ExptAggrResultService,
	resultSvc service.ExptResultService,
	manager service.IExptManager,
	scheduler service.ExptSchedulerEvent,
	recordEval service.ExptItemEvalEvent,
	idgen idgen.IIDGenerator,
	configer component.IConfiger,
	auth rpc.IAuthProvider,
	userInfoService userinfo.UserInfoService,
	evalTargetService service.IEvalTargetService,
	evaluationSetItemService service.EvaluationSetItemService,
	annotateService service.IExptAnnotateService,
	tagRPCAdapter rpc.ITagRPCAdapter,
	exptResultExportService service.IExptResultExportService,
	exptInsightAnalysisService service.IExptInsightAnalysisService,
	evaluatorService service.EvaluatorService,
	templateManager service.IExptTemplateManager,
	deadlineDispatcher *service.ExptDeadlineDispatcher,
) IExperimentApplication {
	return &experimentApplication{
		resultSvc:                   resultSvc,
		manager:                     manager,
		idgen:                       idgen,
		configer:                    configer,
		ExptAggrResultService:       aggResultSvc,
		ExptSchedulerEvent:          scheduler,
		ExptItemEvalEvent:           recordEval,
		auth:                        auth,
		userInfoService:             userInfoService,
		evalTargetService:           evalTargetService,
		evaluationSetItemService:    evaluationSetItemService,
		annotateService:             annotateService,
		tagRPCAdapter:               tagRPCAdapter,
		IExptResultExportService:    exptResultExportService,
		IExptInsightAnalysisService: exptInsightAnalysisService,
		evaluatorService:            evaluatorService,
		templateManager:             templateManager,
		deadlineDispatcher:          deadlineDispatcher,
	}
}

func (e *experimentApplication) StartDeadlineDispatcher(ctx context.Context) {
	if e.deadlineDispatcher == nil {
		return
	}
	go e.deadlineDispatcher.Run(ctx)
}

func (e *experimentApplication) CreateExperiment(ctx context.Context, req *expt.CreateExperimentRequest) (r *expt.CreateExperimentResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}
	logs.CtxInfo(ctx, "CreateExperiment userIDInContext: %s", session.UserID)

	// 收集 evaluator_version_id（包含顺序解析 EvaluatorIDVersionList）、runconfig 和 score weight
	evalVersionIDs, evaluatorVersionRunConfigs, evaluatorScoreWeights, err := e.resolveEvaluatorVersionIDsFromCreateReq(ctx, req)
	if err != nil {
		return nil, err
	}

	// 去重
	if len(evalVersionIDs) > 1 {
		seen := map[int64]struct{}{}
		uniq := make([]int64, 0, len(evalVersionIDs))
		for _, id := range evalVersionIDs {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			uniq = append(uniq, id)
		}
		evalVersionIDs = uniq
	}
	req.EvaluatorVersionIds = evalVersionIDs

	// 将解析出的权重配置合并到请求中（如果请求中没有显式设置）
	if len(evaluatorScoreWeights) > 0 && len(req.EvaluatorScoreWeights) == 0 {
		req.EvaluatorScoreWeights = evaluatorScoreWeights
	}

	param, err := experiment.ConvertCreateReq(req, evaluatorVersionRunConfigs)
	if err != nil {
		return nil, err
	}
	createExpt, err := e.manager.CreateExpt(ctx, param, session)
	if err != nil {
		return nil, err
	}

	return &expt.CreateExperimentResponse{
		Experiment: experiment.ToExptDTO(createExpt),
		BaseResp:   base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) CreateExperimentTemplate(ctx context.Context, req *expt.CreateExperimentTemplateRequest) (r *expt.CreateExperimentTemplateResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}
	logs.CtxInfo(ctx, "CreateExperimentTemplate userIDInContext: %s", session.UserID)

	// 权限校验
	if err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionCreateExptTemplate), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}

	param, err := experiment.ConvertCreateExptTemplateReq(req)
	if err != nil {
		return nil, err
	}

	// 业务逻辑已下沉到 service 层，在 Create 方法中会自动解析并回填 evaluator_version_id
	createTemplate, err := e.templateManager.Create(ctx, param, session)
	if err != nil {
		return nil, err
	}

	dto := experiment.ToExptTemplateDTO(createTemplate)
	// 填充完整的用户信息
	e.mPackExptTemplateUserInfo(ctx, []*domain_expt.ExptTemplate{dto})

	return &expt.CreateExperimentTemplateResponse{
		ExperimentTemplate: dto,
		BaseResp:           base.NewBaseResp(),
	}, nil
}

// BatchGetExperimentTemplate 批量获取实验模板
func (e *experimentApplication) BatchGetExperimentTemplate(ctx context.Context, req *expt.BatchGetExperimentTemplateRequest) (r *expt.BatchGetExperimentTemplateResponse, err error) {
	session := entity.NewSession(ctx)
	logs.CtxInfo(ctx, "BatchGetExperimentTemplate template_ids: %v, workspace_id: %d", req.GetTemplateIds(), req.GetWorkspaceID())

	// 权限校验，与 ListExperimentTemplates 一致：空间级 listLoopExptTemplate
	if err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExptTemplate), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}

	templateIDs := req.GetTemplateIds()
	if len(templateIDs) == 0 {
		return &expt.BatchGetExperimentTemplateResponse{
			ExperimentTemplates: nil,
			BaseResp:            base.NewBaseResp(),
		}, nil
	}

	templates, err := e.templateManager.MGet(ctx, templateIDs, req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	dtos := experiment.ToExptTemplateDTOs(templates)
	// 填充完整的用户信息
	e.mPackExptTemplateUserInfo(ctx, dtos)

	return &expt.BatchGetExperimentTemplateResponse{
		ExperimentTemplates: dtos,
		BaseResp:            base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) UpdateExperimentTemplate(ctx context.Context, req *expt.UpdateExperimentTemplateRequest) (r *expt.UpdateExperimentTemplateResponse, err error) {
	session := entity.NewSession(ctx)

	// 从顶层字段提取 template_id 和 workspace_id
	templateID := req.GetTemplateID()
	workspaceID := req.GetWorkspaceID()
	if templateID == 0 || workspaceID == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("template_id and workspace_id are required"))
	}

	logs.CtxInfo(ctx, "UpdateExperimentTemplate template_id: %d, workspace_id: %d", templateID, workspaceID)

	// 权限校验，与 ListExperimentTemplates 一致：空间级 listLoopExptTemplate
	if err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(workspaceID, 10),
		SpaceID:       workspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExptTemplate), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}

	// 获取现有模板用于业务逻辑
	got, err := e.templateManager.Get(ctx, templateID, workspaceID, session)
	if err != nil {
		return nil, err
	}
	if got == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("template not found"))
	}

	// 转换请求参数
	param, err := experiment.ConvertUpdateExptTemplateReq(req)
	if err != nil {
		return nil, err
	}

	// 更新模板
	updatedTemplate, err := e.templateManager.Update(ctx, param, session)
	if err != nil {
		return nil, err
	}

	dto := experiment.ToExptTemplateDTO(updatedTemplate)
	// 填充完整的用户信息
	e.mPackExptTemplateUserInfo(ctx, []*domain_expt.ExptTemplate{dto})

	return &expt.UpdateExperimentTemplateResponse{
		ExperimentTemplate: dto,
		BaseResp:           base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) UpdateExperimentTemplateMeta(ctx context.Context, req *expt.UpdateExperimentTemplateMetaRequest) (r *expt.UpdateExperimentTemplateMetaResponse, err error) {
	session := entity.NewSession(ctx)

	templateID := req.GetTemplateID()
	workspaceID := req.GetWorkspaceID()
	if templateID == 0 || workspaceID == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("template_id and workspace_id are required"))
	}

	logs.CtxInfo(ctx, "UpdateExperimentTemplateMeta template_id: %d, workspace_id: %d", templateID, workspaceID)

	// 权限校验，与 ListExperimentTemplates 一致：空间级 listLoopExptTemplate
	if err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(workspaceID, 10),
		SpaceID:       workspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExptTemplate), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}

	// 获取现有模板用于业务逻辑
	got, err := e.templateManager.Get(ctx, templateID, workspaceID, session)
	if err != nil {
		return nil, err
	}
	if got == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("template not found"))
	}

	// 转换请求参数
	param, err := experiment.ConvertUpdateExptTemplateMetaReq(req)
	if err != nil {
		return nil, err
	}

	// 更新模板 meta
	updatedTemplate, err := e.templateManager.UpdateMeta(ctx, param, session)
	if err != nil {
		return nil, err
	}

	// 转换为 Meta DTO
	var metaDTO *domain_expt.ExptTemplateMeta
	if updatedTemplate.Meta != nil {
		metaDTO = &domain_expt.ExptTemplateMeta{
			ID:          gptr.Of(updatedTemplate.Meta.ID),
			WorkspaceID: gptr.Of(updatedTemplate.Meta.WorkspaceID),
			Name:        gptr.Of(updatedTemplate.Meta.Name),
			Desc:        gptr.Of(updatedTemplate.Meta.Desc),
			ExptType:    gptr.Of(domain_expt.ExptType(updatedTemplate.Meta.ExptType)),
		}
	}

	return &expt.UpdateExperimentTemplateMetaResponse{
		Meta:     metaDTO,
		BaseResp: base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) DeleteExperimentTemplate(ctx context.Context, req *expt.DeleteExperimentTemplateRequest) (r *expt.DeleteExperimentTemplateResponse, err error) {
	session := entity.NewSession(ctx)
	logs.CtxInfo(ctx, "DeleteExperimentTemplate template_id: %d, workspace_id: %d", req.GetTemplateID(), req.GetWorkspaceID())

	// 权限校验，与 ListExperimentTemplates 一致：空间级 listLoopExptTemplate
	if err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExptTemplate), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}

	// 删除模板
	if err := e.templateManager.Delete(ctx, req.GetTemplateID(), req.GetWorkspaceID(), session); err != nil {
		return nil, err
	}

	return &expt.DeleteExperimentTemplateResponse{
		BaseResp: base.NewBaseResp(),
	}, nil
}

// ListExperimentTemplates 列出实验模板
func (e *experimentApplication) ListExperimentTemplates(ctx context.Context, req *expt.ListExperimentTemplatesRequest) (r *expt.ListExperimentTemplatesResponse, err error) {
	session := entity.NewSession(ctx)
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExptTemplate), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	// 当 FilterOption 为空时，直接不下发过滤条件，避免在 Filter 转换过程中出现空指针等异常
	var filters *entity.ExptTemplateListFilter
	if req.GetFilterOption() != nil {
		filters, err = experiment.NewExptTemplateFilterConvertor(e.evalTargetService).Convert(ctx, req.GetFilterOption(), req.GetWorkspaceID())
		if err != nil {
			return nil, err
		}
	}

	// 检查是否需要分流处理在线实验模板
	needOnlineExptTemplates := false
	if filters != nil && filters.Includes != nil && len(filters.Includes.ExptType) > 0 {
		// 检查是否包含在线实验类型 (ExptType_Online = 2)
		for _, exptType := range filters.Includes.ExptType {
			if exptType == int64(entity.ExptType_Online) {
				needOnlineExptTemplates = true
				break
			}
		}
	}

	orderBys := slices.Transform(req.GetOrderBys(), func(e *common.OrderBy, _ int) *entity.OrderBy {
		return &entity.OrderBy{Field: gptr.Of(e.GetField()), IsAsc: gptr.Of(e.GetIsAsc())}
	})
	// 如果没有显式指定排序字段，默认按 updated_at 倒序
	if len(orderBys) == 0 {
		orderBys = []*entity.OrderBy{
			{
				Field: gptr.Of(entity.OrderByUpdatedAt),
				IsAsc: gptr.Of(false),
			},
		}
	}

	var templates []*entity.ExptTemplate
	var count int64

	if needOnlineExptTemplates {
		// 在线实验模板：通过 Task 查询
		templates, count, err = e.templateManager.ListOnline(ctx, req.GetPageNumber(), req.GetPageSize(), req.GetWorkspaceID(), filters, orderBys, session)
		if err != nil {
			return nil, err
		}
	} else {
		// 离线实验模板：使用原有逻辑
		templates, count, err = e.templateManager.List(ctx, req.GetPageNumber(), req.GetPageSize(), req.GetWorkspaceID(), filters, orderBys, session)
		if err != nil {
			return nil, err
		}
	}

	dtos := experiment.ToExptTemplateDTOs(templates)
	// 填充完整的用户信息
	e.mPackExptTemplateUserInfo(ctx, dtos)

	return &expt.ListExperimentTemplatesResponse{
		ExperimentTemplates: dtos,
		Total:               gptr.Of(int32(count)),
		BaseResp:            base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) SubmitExperiment(ctx context.Context, req *expt.SubmitExperimentRequest) (r *expt.SubmitExperimentResponse, err error) {
	logs.CtxInfo(ctx, "SubmitExperiment req: %v", json.Jsonify(req))
	if hasDuplicates(req.EvaluatorVersionIds) {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("duplicate evaluator version ids"))
	}

	if err := e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionCreateExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}

	// 构建 CreateExperimentRequest，resolveEvaluatorVersionIDs 流程已在 CreateExperiment 中完成
	createReq := &expt.CreateExperimentRequest{
		WorkspaceID:            req.GetWorkspaceID(),
		EvalSetVersionID:       req.EvalSetVersionID,
		EvalSetID:              req.EvalSetID,
		EvaluatorVersionIds:    req.EvaluatorVersionIds,
		Name:                   req.Name,
		Desc:                   req.Desc,
		TargetFieldMapping:     req.TargetFieldMapping,
		EvaluatorFieldMapping:  req.EvaluatorFieldMapping,
		ItemConcurNum:          req.ItemConcurNum,
		EvaluatorsConcurNum:    req.EvaluatorsConcurNum,
		CreateEvalTargetParam:  req.CreateEvalTargetParam,
		ExptType:               req.ExptType,
		MaxAliveTime:           req.MaxAliveTime,
		SourceType:             req.SourceType,
		SourceID:               req.SourceID,
		TargetRuntimeParam:     req.TargetRuntimeParam,
		EvaluatorIDVersionList: req.EvaluatorIDVersionList,
		Session:                req.Session,
		EnableWeightedScore:    req.EnableWeightedScore,
		// EvaluatorScoreWeights 会在 CreateExperiment 的 resolveEvaluatorVersionIDsFromCreateReq 中解析
	}
	if req.IsSetExptTemplateID() {
		createReq.ExptTemplateID = gptr.Of(req.GetExptTemplateID())
	}
	cresp, err := e.CreateExperiment(ctx, createReq)
	if err != nil {
		return nil, err
	}

	rresp, err := e.RunExperiment(ctx, &expt.RunExperimentRequest{
		WorkspaceID: gptr.Of(req.GetWorkspaceID()),
		ExptID:      cresp.GetExperiment().ID,
		ExptType:    req.ExptType,
		Session:     req.Session,
		Ext:         req.Ext,
	})
	if err != nil {
		return nil, err
	}

	// 如果有关联的实验模板，更新模板的 ExptInfo（创建实验，数量 +1）
	if req.IsSetExptTemplateID() && req.GetExptTemplateID() > 0 {
		exptID := gptr.Indirect(cresp.GetExperiment().ID)
		exptStatus := entity.ExptStatus_Pending // Submit 时实验状态为 Pending
		if err := e.templateManager.UpdateExptInfo(ctx, req.GetExptTemplateID(), req.GetWorkspaceID(), exptID, exptStatus, 1); err != nil {
			// 记录错误但不影响主流程
			logs.CtxError(ctx, "[ExptEval] UpdateExptInfo failed after SubmitExperiment, template_id: %v, expt_id: %v, err: %v",
				req.GetExptTemplateID(), exptID, err)
		}
	}

	return &expt.SubmitExperimentResponse{
		Experiment: cresp.GetExperiment(),
		RunID:      gptr.Of(rresp.GetRunID()),
		BaseResp:   base.NewBaseResp(),
	}, nil
}

// resolveEvaluatorVersionIDsFromCreateReq 汇总 evaluator_version_ids、runconfig 和权重配置：
// 1) 先取请求中的 EvaluatorVersionIds
// 2) 从有序 EvaluatorIDVersionList 中批量解析并按输入顺序回填版本ID
// 3) 从 EvaluatorIDVersionList 中提取 runconfig 和权重配置，构建 evaluator_version_id 到 runconfig/权重的映射
// 注意：runconfig 用于评估器运行时配置，score weight 用于加权分数计算
func (e *experimentApplication) resolveEvaluatorVersionIDsFromCreateReq(ctx context.Context, req *expt.CreateExperimentRequest) ([]int64, map[int64]*evaluatordto.EvaluatorRunConfig, map[int64]float64, error) {
	evalVersionIDs := make([]int64, 0, len(req.EvaluatorVersionIds))
	evalVersionIDs = append(evalVersionIDs, req.EvaluatorVersionIds...)

	// 权重映射：key 为 evaluator_version_id，value 为权重（用于加权分数计算）
	evaluatorScoreWeights := make(map[int64]float64)
	// 如果请求中已经显式设置了权重，优先使用
	if len(req.EvaluatorScoreWeights) > 0 {
		for k, v := range req.EvaluatorScoreWeights {
			if v > 0 {
				evaluatorScoreWeights[k] = v
			}
		}
	}

	// 解析有序列表并批量查询：将 BuiltinVisible 与普通版本分离，分别批量查，最后按输入顺序回填版本ID
	items := req.GetEvaluatorIDVersionList()
	builtinIDs := make([]int64, 0)
	normalPairs := make([][2]interface{}, 0)
	for _, it := range items {
		if it == nil {
			continue
		}
		eid := it.GetEvaluatorID()
		ver := it.GetVersion()
		if eid == 0 || ver == "" {
			continue
		}
		if ver == "BuiltinVisible" {
			builtinIDs = append(builtinIDs, eid)
		} else {
			normalPairs = append(normalPairs, [2]interface{}{eid, ver})
		}
	}

	// 批量获取内置与普通版本
	id2Builtin := make(map[int64]*entity.Evaluator, len(builtinIDs))
	if len(builtinIDs) > 0 {
		evs, err := e.evaluatorService.BatchGetBuiltinEvaluator(ctx, builtinIDs)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, ev := range evs {
			if ev != nil {
				id2Builtin[ev.ID] = ev
			}
		}
	}

	pair2Eval := make(map[string]*entity.Evaluator, len(normalPairs))
	if len(normalPairs) > 0 {
		evs, err := e.evaluatorService.BatchGetEvaluatorByIDAndVersion(ctx, normalPairs)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, ev := range evs {
			if ev == nil {
				continue
			}
			key := fmt.Sprintf("%d#%s", ev.ID, ev.GetVersion())
			pair2Eval[key] = ev
		}
	}

	// 按输入顺序回填版本ID，同时提取 runconfig 和权重配置
	// runconfig: 用于评估器运行时配置（如超时、重试等）
	// score weight: 用于加权分数计算
	evaluatorVersionRunConfigs := make(map[int64]*evaluatordto.EvaluatorRunConfig)
	for _, it := range items {
		if it == nil {
			continue
		}
		eid := it.GetEvaluatorID()
		ver := it.GetVersion()
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
		if ev == nil {
			continue
		}
		if verID := ev.GetEvaluatorVersionID(); verID != 0 {
			evalVersionIDs = append(evalVersionIDs, verID)
			// 提取 runconfig（如果存在）- 用于评估器运行时配置
			if it.RunConfig != nil {
				evaluatorVersionRunConfigs[verID] = it.RunConfig
			}
			// 提取权重配置（如果存在且请求中未显式设置）- 用于加权分数计算
			if it.ScoreWeight != nil {
				weight := *it.ScoreWeight
				if weight > 0 {
					// 如果请求中已经显式设置了权重，则不覆盖
					if _, exists := evaluatorScoreWeights[verID]; !exists {
						evaluatorScoreWeights[verID] = weight
					}
				}
			}
		}
	}

	// 回填 EvaluatorFieldMapping 中缺失的 evaluator_version_id
	if fm := req.GetEvaluatorFieldMapping(); len(fm) > 0 {
		for _, m := range fm {
			if m == nil || m.GetEvaluatorVersionID() != 0 {
				continue
			}
			if item := m.GetEvaluatorIDVersionItem(); item != nil {
				eid := item.GetEvaluatorID()
				ver := item.GetVersion()
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
					if vid := ev.GetEvaluatorVersionID(); vid != 0 {
						m.SetEvaluatorVersionID(vid)
					}
				}
			}
		}
	}

	return evalVersionIDs, evaluatorVersionRunConfigs, evaluatorScoreWeights, nil
}

func (e *experimentApplication) CheckExperimentName(ctx context.Context, req *expt.CheckExperimentNameRequest) (r *expt.CheckExperimentNameResponse, err error) {
	if err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionCreateExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}
	session := entity.NewSession(ctx)
	pass, err := e.manager.CheckName(ctx, req.GetName(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}
	var message string
	if !pass {
		message = fmt.Sprintf("experiment name %s already exist", req.GetName())
	}

	return &expt.CheckExperimentNameResponse{
		Pass:    gptr.Of(pass),
		Message: &message,
	}, nil
}

// CheckExperimentTemplateName 校验实验模板名称是否可用。
// 如果传入了 template_id，且名称与该模板当前名称相同，则认为可用。
func (e *experimentApplication) CheckExperimentTemplateName(ctx context.Context, req *expt.CheckExperimentTemplateNameRequest) (r *expt.CheckExperimentTemplateNameResponse, err error) {
	// 空间级别创建模板权限校验，沿用 CreateExperimentTemplate 的策略
	if err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionCreateExptTemplate), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}

	session := entity.NewSession(ctx)

	// 如果传了 template_id，且名称与当前模板名称相同，则直接返回可用
	if req.IsSetTemplateID() && req.GetTemplateID() > 0 {
		tpl, err := e.templateManager.Get(ctx, req.GetTemplateID(), req.GetWorkspaceID(), session)
		if err != nil {
			return nil, err
		}
		if tpl != nil && tpl.Meta != nil && tpl.Meta.Name == req.GetName() {
			isAvailable := true
			return &expt.CheckExperimentTemplateNameResponse{
				IsAvailable: &isAvailable,
				BaseResp:    base.NewBaseResp(),
			}, nil
		}
	}

	// 否则走正常的重名校验：模板名在该空间下是否已存在
	pass, err := e.templateManager.CheckName(ctx, req.GetName(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	return &expt.CheckExperimentTemplateNameResponse{
		IsAvailable: &pass,
		BaseResp:    base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) BatchGetExperiments(ctx context.Context, req *expt.BatchGetExperimentsRequest) (r *expt.BatchGetExperimentsResponse, err error) {
	session := entity.NewSession(ctx)

	dos, err := e.manager.MGetDetail(ctx, req.GetExptIds(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	if err := e.AuthReadExperiments(ctx, dos, req.GetWorkspaceID()); err != nil {
		return nil, err
	}

	dtos := experiment.ToExptDTOs(dos)

	vos, err := e.mPackUserInfo(ctx, dtos)
	if err != nil {
		return nil, err
	}

	return &expt.BatchGetExperimentsResponse{
		Experiments: vos,
		BaseResp:    base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) ListExperiments(ctx context.Context, req *expt.ListExperimentsRequest) (r *expt.ListExperimentsResponse, err error) {
	session := entity.NewSession(ctx)
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	filters, err := experiment.NewExptFilterConvertor(e.evalTargetService).Convert(ctx, req.GetFilterOption(), req.GetWorkspaceID())
	if err != nil {
		return nil, err
	}

	orderBys := slices.Transform(req.GetOrderBys(), func(e *common.OrderBy, _ int) *entity.OrderBy {
		return &entity.OrderBy{Field: gptr.Of(e.GetField()), IsAsc: gptr.Of(e.GetIsAsc())}
	})
	expts, count, err := e.manager.List(ctx, req.GetPageNumber(), req.GetPageSize(), req.GetWorkspaceID(), filters, orderBys, session)
	if err != nil {
		return nil, err
	}

	dtos := experiment.ToExptDTOs(expts)
	vos, err := e.mPackUserInfo(ctx, dtos)
	if err != nil {
		return nil, err
	}

	return &expt.ListExperimentsResponse{
		Experiments: vos,
		Total:       gptr.Of(int32(count)),
		BaseResp:    base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) ListExperimentStats(ctx context.Context, req *expt.ListExperimentStatsRequest) (r *expt.ListExperimentStatsResponse, err error) {
	session := &entity.Session{UserID: strconv.FormatInt(req.GetSession().GetUserID(), 10)}
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	filters, err := experiment.NewExptFilterConvertor(e.evalTargetService).Convert(ctx, req.GetFilterOption(), req.GetWorkspaceID())
	if err != nil {
		return nil, err
	}

	expts, total, err := e.manager.ListExptRaw(ctx, req.GetPageNumber(), req.GetPageSize(), req.GetWorkspaceID(), filters)
	if err != nil {
		return nil, err
	}

	exptIDs := slices.Transform(expts, func(e *entity.Experiment, _ int) int64 { return e.ID })
	stats, err := e.resultSvc.MGetStats(ctx, exptIDs, req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}
	exptID2Stats := slices.ToMap(stats, func(e *entity.ExptStats) (int64, *entity.ExptStats) { return e.ExptID, e })
	dtos := make([]*domain_expt.ExptStatsInfo, 0, len(stats))
	for _, exptDO := range expts {
		dtos = append(dtos, experiment.ToExptStatsInfoDTO(exptDO, exptID2Stats[exptDO.ID]))
	}
	return &expt.ListExperimentStatsResponse{
		ExptStatsInfos: dtos,
		Total:          gptr.Of(int32(total)),
	}, nil
}

func (e *experimentApplication) UpdateExperiment(ctx context.Context, req *expt.UpdateExperimentRequest) (r *expt.UpdateExperimentResponse, err error) {
	session := entity.NewSession(ctx)

	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	if got.Name != req.GetName() {
		pass, err := e.manager.CheckName(ctx, req.GetName(), req.GetWorkspaceID(), session)
		if err != nil {
			return nil, err
		}

		if !pass {
			return nil, errorx.NewByCode(errno.ExperimentNameExistedCode, errorx.WithExtraMsg(fmt.Sprintf("name %v", req.Name)))
		}
	}

	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.ExptID, 10),
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(got.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	})
	if err != nil {
		return nil, err
	}

	if err := e.manager.Update(ctx, &entity.Experiment{
		ID:          req.GetExptID(),
		SpaceID:     req.WorkspaceID,
		Name:        req.GetName(),
		Description: req.GetDesc(),
	}, session); err != nil {
		return nil, err
	}

	resp, err := e.manager.Get(contexts.WithCtxWriteDB(ctx), req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	return &expt.UpdateExperimentResponse{
		Experiment: experiment.ToExptDTO(resp),
		BaseResp:   base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) DeleteExperiment(ctx context.Context, req *expt.DeleteExperimentRequest) (r *expt.DeleteExperimentResponse, err error) {
	session := entity.NewSession(ctx)

	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.GetExptID(), 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(got.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	})
	if err != nil {
		return nil, err
	}

	if err := e.manager.Delete(ctx, req.GetExptID(), req.GetWorkspaceID(), session); err != nil {
		return nil, err
	}

	return &expt.DeleteExperimentResponse{BaseResp: base.NewBaseResp()}, nil
}

func (e *experimentApplication) BatchDeleteExperiments(ctx context.Context, req *expt.BatchDeleteExperimentsRequest) (r *expt.BatchDeleteExperimentsResponse, err error) {
	session := entity.NewSession(ctx)

	got, err := e.manager.MGet(ctx, req.GetExptIds(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}
	exptMap := slices.ToMap(got, func(e *entity.Experiment) (int64, *entity.Experiment) {
		return e.ID, e
	})

	var authParams []*rpc.AuthorizationWithoutSPIParam
	for _, exptID := range req.GetExptIds() {
		if exptMap[exptID] == nil {
			continue
		}
		authParams = append(authParams, &rpc.AuthorizationWithoutSPIParam{
			ObjectID:        strconv.FormatInt(exptID, 10),
			SpaceID:         req.WorkspaceID,
			ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
			OwnerID:         gptr.Of(exptMap[exptID].CreatedBy),
			ResourceSpaceID: req.WorkspaceID,
		})
	}

	err = e.auth.MAuthorizeWithoutSPI(ctx, req.WorkspaceID, authParams)
	if err != nil {
		return nil, err
	}

	if err := e.manager.MDelete(ctx, req.GetExptIds(), req.GetWorkspaceID(), session); err != nil {
		return nil, err
	}

	return &expt.BatchDeleteExperimentsResponse{BaseResp: base.NewBaseResp()}, nil
}

func (e *experimentApplication) CloneExperiment(ctx context.Context, req *expt.CloneExperimentRequest) (r *expt.CloneExperimentResponse, err error) {
	session := entity.NewSession(ctx)

	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetExptID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionCreateExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	exptDO, err := e.manager.Clone(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	id, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, err
	}

	if err := e.resultSvc.CreateStats(ctx, &entity.ExptStats{
		ID:      id,
		SpaceID: req.GetWorkspaceID(),
		ExptID:  exptDO.ID,
	}, session); err != nil {
		return nil, err
	}

	return &expt.CloneExperimentResponse{
		Experiment: experiment.ToExptDTO(exptDO),
		BaseResp:   base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) RunExperiment(ctx context.Context, req *expt.RunExperimentRequest) (r *expt.RunExperimentResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}

	runID, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, err
	}

	evalMode := experiment.ExptType2EvalMode(req.GetExptType())

	if err := e.manager.LogRun(ctx, req.GetExptID(), runID, evalMode, req.GetWorkspaceID(), session); err != nil {
		return nil, err
	}

	if err := e.manager.Run(ctx, req.GetExptID(), runID, req.GetWorkspaceID(), session, evalMode, req.GetExt()); err != nil {
		return nil, err
	}
	return &expt.RunExperimentResponse{
		RunID:    gptr.Of(runID),
		BaseResp: base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) RetryExperiment(ctx context.Context, req *expt.RetryExperimentRequest) (r *expt.RetryExperimentResponse, err error) {
	session := entity.NewSession(ctx)

	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	if err := e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.GetExptID(), 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(got.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	}); err != nil {
		return nil, err
	}

	runID, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, err
	}

	if err := e.manager.LogRun(ctx, req.GetExptID(), runID, entity.EvaluationModeFailRetry, req.GetWorkspaceID(), session); err != nil {
		return nil, err
	}

	if err := e.manager.RetryUnSuccess(ctx, req.GetExptID(), runID, req.GetWorkspaceID(), session, req.GetExt()); err != nil {
		return nil, err
	}

	return &expt.RetryExperimentResponse{
		RunID:    gptr.Of(runID),
		BaseResp: base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) KillExperiment(ctx context.Context, req *expt.KillExperimentRequest) (r *expt.KillExperimentResponse, err error) {
	session := entity.NewSession(ctx)
	logs.CtxInfo(ctx, "KillExperiment receive req, expt_id: %v, user_id: %v", req.GetExptID(), session.UserID)

	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	if got.Status != entity.ExptStatus_Processing {
		return nil, errorx.NewByCode(errno.TerminateNonRunningExperimentErrorCode)
	}

	if !e.configer.GetMaintainerUserIDs(ctx)[session.UserID] {
		if err := e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
			ObjectID:        strconv.FormatInt(req.GetExptID(), 10),
			SpaceID:         req.GetWorkspaceID(),
			ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
			OwnerID:         gptr.Of(got.CreatedBy),
			ResourceSpaceID: req.GetWorkspaceID(),
		}); err != nil {
			return nil, err
		}
	}

	if err := e.manager.SetExptTerminating(ctx, req.GetExptID(), got.LatestRunID, req.GetWorkspaceID(), session); err != nil {
		return nil, err
	}

	kill := func(ctx context.Context, exptID, exptRunID, spaceID int64, session *entity.Session) error {
		if err := e.manager.CompleteRun(ctx, exptID, exptRunID, spaceID, session, entity.WithStatus(entity.ExptStatus_Terminated)); err != nil {
			return err
		}
		return e.manager.CompleteExpt(ctx, exptID, spaceID, session,
			entity.WithStatus(entity.ExptStatus_Terminated), entity.WithCompleteInterval(time.Second), entity.NoAggrCalculate())
	}

	goroutine.Go(ctx, func() {
		if err := backoff.RetryWithElapsedTime(ctx, time.Minute*3, func() error {
			return kill(ctx, req.GetExptID(), got.LatestRunID, req.GetWorkspaceID(), session)
		}); err != nil {
			logs.CtxInfo(ctx, "kill expt failed, expt_id: %v, err: %v", req.GetExptID(), err)
		}
	})

	return &expt.KillExperimentResponse{BaseResp: base.NewBaseResp()}, nil
}

func (e *experimentApplication) BatchGetExperimentResult_(ctx context.Context, req *expt.BatchGetExperimentResultRequest) (r *expt.BatchGetExperimentResultResponse, err error) {
	// 1. 如果指定了 BaselineExperimentID，先查出其真实的 SpaceID
	var actualSpaceID int64
	if req.BaselineExperimentID != nil {
		session := entity.NewSession(ctx)
		baseExpt, err := e.manager.Get(ctx, *req.BaselineExperimentID, req.WorkspaceID, session)
		if err != nil {
			return nil, err
		}
		actualSpaceID = baseExpt.SpaceID // 从实验信息中提取 SpaceID
	} else {
		// 如果没有指定 BaselineExperimentID，使用请求中的 WorkspaceID
		actualSpaceID = req.WorkspaceID
	}

	// 2. 使用查出的真实 SpaceID 进行权限校验
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(actualSpaceID, 10),
		SpaceID:       actualSpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	page := entity.NewPage(int(req.GetPageNumber()), int(req.GetPageSize()))
	// 3. 构建查询参数，使用真实的 SpaceID
	param := &entity.MGetExperimentResultParam{
		SpaceID:        actualSpaceID, // 使用查出的真实 SpaceID
		ExptIDs:        req.GetExperimentIds(),
		BaseExptID:     req.BaselineExperimentID,
		Page:           page,
		UseAccelerator: req.GetUseAccelerator(),
		FullTrajectory: req.GetFullTrajectory(),
	}
	if err = buildExptTurnResultFilter(req, param); err != nil {
		return nil, err
	}

	result, err := e.resultSvc.MGetExperimentResult(ctx, param)
	if err != nil {
		return nil, err
	}

	resp := &expt.BatchGetExperimentResultResponse{
		ColumnEvalSetFields:   experiment.ColumnEvalSetFieldsDO2DTOs(result.ColumnEvalSetFields),
		ColumnEvaluators:      experiment.ColumnEvaluatorsDO2DTOs(result.ColumnEvaluators),
		ExptColumnEvaluators:  experiment.ExptColumnEvaluatorsDO2DTOs(result.ExptColumnEvaluators),
		ExptColumnAnnotations: experiment.ExptColumnAnnotationDO2DTOs(result.ExptColumnAnnotations),
		ExptColumnEvalTarget:  experiment.ExptColumnEvalTargetDO2DTOs(result.ExptColumnsEvalTarget),
		Total:                 gptr.Of(result.Total),
		ItemResults:           experiment.ItemResultsDO2DTOs(result.ItemResults),
		BaseResp:              base.NewBaseResp(),
	}

	return resp, nil
}

func buildExptTurnResultFilter(req *expt.BatchGetExperimentResultRequest, param *entity.MGetExperimentResultParam) error {
	if req.GetUseAccelerator() {
		filterAccelerators := make(map[int64]*entity.ExptTurnResultFilterAccelerator, len(req.GetFilters()))
		for exptID, f := range req.GetFilters() {
			filter, err := experiment.ConvertExptTurnResultFilterAccelerator(f)
			if err != nil {
				return err
			}
			filterAccelerators[exptID] = filter
		}
		param.FilterAccelerators = filterAccelerators
		param.UseAccelerator = true
	} else {
		filters := make(map[int64]*entity.ExptTurnResultFilter, len(req.GetFilters()))
		for exptID, f := range req.GetFilters() {
			filter, err := experiment.ConvertExptTurnResultFilter(f.GetFilters())
			if err != nil {
				return err
			}
			filters[exptID] = filter
		}
		param.Filters = filters
		param.UseAccelerator = false
	}
	return nil
}

func (e *experimentApplication) BatchGetExperimentAggrResult_(ctx context.Context, req *expt.BatchGetExperimentAggrResultRequest) (r *expt.BatchGetExperimentAggrResultResponse, err error) {
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	aggrResults, err := e.BatchGetExptAggrResultByExperimentIDs(ctx, req.WorkspaceID, req.ExperimentIds)
	if err != nil {
		return nil, err
	}

	exptAggregateResultDTOs := make([]*domain_expt.ExptAggregateResult_, 0, len(aggrResults))
	for _, aggrResult := range aggrResults {
		exptAggregateResultDTOs = append(exptAggregateResultDTOs, experiment.ExptAggregateResultDOToDTO(aggrResult))
	}

	return &expt.BatchGetExperimentAggrResultResponse{
		ExptAggregateResult_: exptAggregateResultDTOs,
	}, nil
}

func (e *experimentApplication) mPackUserInfo(ctx context.Context, expts []*domain_expt.Experiment) ([]*domain_expt.Experiment, error) {
	if len(expts) == 0 {
		return expts, nil
	}

	userCarriers := make([]userinfo.UserInfoCarrier, 0, len(expts))
	for _, exptVO := range expts {
		exptVO.BaseInfo = &common.BaseInfo{
			CreatedBy: &common.UserInfo{
				UserID: exptVO.CreatorBy,
			},
		}
		userCarriers = append(userCarriers, exptVO)
	}

	e.userInfoService.PackUserInfo(ctx, userCarriers)

	return expts, nil
}

func (e *experimentApplication) mPackExptTemplateUserInfo(ctx context.Context, templates []*domain_expt.ExptTemplate) {
	if len(templates) == 0 {
		return
	}

	userCarriers := make([]userinfo.UserInfoCarrier, 0, len(templates))
	for _, template := range templates {
		if template != nil && template.GetBaseInfo() != nil {
			userCarriers = append(userCarriers, template)
		}
	}

	if len(userCarriers) > 0 {
		e.userInfoService.PackUserInfo(ctx, userCarriers)
	}
}

func (e *experimentApplication) AuthReadExperiments(ctx context.Context, dos []*entity.Experiment, spaceID int64) error {
	var authParams []*rpc.AuthorizationWithoutSPIParam
	for _, do := range dos {
		if do == nil {
			continue
		}
		exptID := do.ID
		authParams = append(authParams, &rpc.AuthorizationWithoutSPIParam{
			ObjectID:        strconv.FormatInt(exptID, 10),
			SpaceID:         spaceID,
			ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
			OwnerID:         gptr.Of(do.CreatedBy),
			ResourceSpaceID: spaceID,
		})
	}
	return e.auth.MAuthorizeWithoutSPI(ctx, spaceID, authParams)
}

func (e *experimentApplication) AuthReadExptTemplates(ctx context.Context, templates []*entity.ExptTemplate, spaceID int64) error {
	var authParams []*rpc.AuthorizationWithoutSPIParam
	for _, tpl := range templates {
		if tpl == nil {
			continue
		}
		templateID := tpl.GetID()
		authParams = append(authParams, &rpc.AuthorizationWithoutSPIParam{
			ObjectID:        strconv.FormatInt(templateID, 10),
			SpaceID:         spaceID,
			ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExptTemplate)}},
			OwnerID:         gptr.Of(tpl.GetCreatedBy()),
			ResourceSpaceID: spaceID,
		})
	}
	return e.auth.MAuthorizeWithoutSPI(ctx, spaceID, authParams)
}

func (e *experimentApplication) InvokeExperiment(ctx context.Context, req *expt.InvokeExperimentRequest) (r *expt.InvokeExperimentResponse, err error) {
	logs.CtxInfo(ctx, "experimentApplication InvokeExperiment, req: %v", json.Jsonify(req))
	session := &entity.Session{UserID: strconv.FormatInt(req.GetSession().GetUserID(), 10)}

	got, err := e.manager.Get(ctx, req.GetExperimentID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	if err := e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.GetExperimentID(), 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(got.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	}); err != nil {
		return nil, err
	}

	logs.CtxInfo(ctx, "InvokeExperiment expt: %v", json.Jsonify(got))
	if got.Status != entity.ExptStatus_Processing && got.Status != entity.ExptStatus_Pending {
		logs.CtxInfo(ctx, "expt status not allow to invoke, expt_id: %v, status: %v", req.GetExperimentID(), got.Status)
		return nil, errorx.NewByCode(errno.ExperimentStatusNotAllowedToInvokeCode, errorx.WithExtraMsg(fmt.Sprintf("expt status not allow to invoke, expt_id: %v, status: %v", req.GetExperimentID(), got.Status)))
	}
	itemDOS := evaluation_set.ItemDTO2DOs(req.Items)
	idMap, evalSetErrors, itemOutputs, err := e.evaluationSetItemService.BatchCreateEvaluationSetItems(ctx, &entity.BatchCreateEvaluationSetItemsParam{
		SpaceID:          req.GetWorkspaceID(),
		EvaluationSetID:  req.GetEvaluationSetID(),
		Items:            itemDOS,
		SkipInvalidItems: req.SkipInvalidItems,
		AllowPartialAdd:  req.AllowPartialAdd,
	})
	if err != nil {
		return nil, err
	}
	validItemDOS := make([]*entity.EvaluationSetItem, 0, len(itemDOS))
	for idx, itemID := range idMap {
		itemDOS[idx].ItemID = itemID
		validItemDOS = append(validItemDOS, itemDOS[idx])
	}
	err = e.manager.Invoke(ctx, &entity.InvokeExptReq{
		ExptID:  req.GetExperimentID(),
		RunID:   req.GetExperimentRunID(),
		SpaceID: req.GetWorkspaceID(),
		Session: session,
		Items:   validItemDOS,
		Ext:     req.Ext,
	})
	if err != nil {
		return nil, err
	}
	err = e.resultSvc.UpsertExptTurnResultFilter(ctx, req.GetWorkspaceID(), req.GetExperimentID(), maps.ToSlice(idMap, func(k, v int64) int64 {
		return v
	}))
	if err != nil {
		return nil, err
	}

	return &expt.InvokeExperimentResponse{
		AddedItems:  idMap,
		Errors:      evaluation_set.ItemErrorGroupDO2DTOs(evalSetErrors),
		ItemOutputs: evaluation_set.CreateDatasetItemOutputDO2DTOs(itemOutputs),
		BaseResp:    base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) FinishExperiment(ctx context.Context, req *expt.FinishExperimentRequest) (r *expt.FinishExperimentResponse, err error) {
	session := &entity.Session{UserID: strconv.FormatInt(req.GetSession().GetUserID(), 10)}

	got, err := e.manager.Get(ctx, req.GetExperimentID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	if entity.IsExptFinished(got.Status) {
		return &expt.FinishExperimentResponse{BaseResp: base.NewBaseResp()}, nil
	}

	if err := e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.GetExperimentID(), 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(got.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	}); err != nil {
		return nil, err
	}

	if err := e.manager.Finish(ctx, got, req.GetExperimentRunID(), session); err != nil {
		return nil, err
	}

	return &expt.FinishExperimentResponse{BaseResp: base.NewBaseResp()}, nil
}

func (e *experimentApplication) UpsertExptTurnResultFilter(ctx context.Context, req *expt.UpsertExptTurnResultFilterRequest) (r *expt.UpsertExptTurnResultFilterResponse, err error) {
	if req.GetFilterType() == expt.UpsertExptTurnResultFilterTypeMANUAL {
		logs.CtxInfo(ctx, "ManualUpsertExptTurnResultFilter, req: %v", json.Jsonify(req))
		err = e.resultSvc.ManualUpsertExptTurnResultFilter(ctx, req.GetWorkspaceID(), req.GetExperimentID(), req.GetItemIds())
		if err != nil {
			logs.CtxWarn(ctx, "ManualUpsertExptTurnResultFilter fail, err: %v", err)
			return nil, err
		}
	} else if req.GetFilterType() == expt.UpsertExptTurnResultFilterTypeCHECK {
		err = e.resultSvc.CompareExptTurnResultFilters(ctx, req.GetWorkspaceID(), req.GetExperimentID(), req.GetItemIds(), req.GetRetryTimes())
		if err != nil {
			return nil, err
		}
	} else {
		err = e.resultSvc.UpsertExptTurnResultFilter(ctx, req.GetWorkspaceID(), req.GetExperimentID(), req.GetItemIds())
		if err != nil {
			return nil, err
		}
	}

	return &expt.UpsertExptTurnResultFilterResponse{}, nil
}

func hasDuplicates(slice []int64) bool {
	elementMap := make(map[int64]bool)
	for _, value := range slice {
		if elementMap[value] {
			return true
		}
		elementMap[value] = true
	}

	return false
}

func (e *experimentApplication) AssociateAnnotationTag(ctx context.Context, req *expt.AssociateAnnotationTagReq) (r *expt.AssociateAnnotationTagResp, err error) {
	session := entity.NewSession(ctx)
	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.GetExptID(), 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(got.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	})
	if err != nil {
		return nil, err
	}

	tagRef := &entity.ExptTurnResultTagRef{
		SpaceID:  req.GetWorkspaceID(),
		ExptID:   req.GetExptID(),
		TagKeyID: req.GetTagKeyID(),
	}
	err = e.annotateService.CreateExptTurnResultTagRefs(ctx, []*entity.ExptTurnResultTagRef{tagRef})
	if err != nil {
		return nil, err
	}
	return &expt.AssociateAnnotationTagResp{
		BaseResp: base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) CreateAnnotateRecord(ctx context.Context, req *expt.CreateAnnotateRecordReq) (r *expt.CreateAnnotateRecordResp, err error) {
	session := entity.NewSession(ctx)
	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.GetExptID(), 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(got.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	})
	if err != nil {
		return nil, err
	}

	id, err := e.idgen.GenID(ctx)
	if err != nil {
		return nil, err
	}
	record := req.AnnotateRecord
	recordDO := &entity.AnnotateRecord{
		ID:           id,
		TagKeyID:     record.GetTagKeyID(),
		SpaceID:      req.GetWorkspaceID(),
		ExperimentID: req.GetExptID(),
		TagValueID:   record.GetTagValueID(),
		AnnotateData: &entity.AnnotateData{
			TextValue:      record.PlainText,
			BoolValue:      record.BooleanOption,
			Option:         record.CategoricalOption,
			TagContentType: entity.TagContentType(record.GetTagContentType()),
		},
	}

	if record.Score != nil {
		score, err := strconv.ParseFloat(ptr.From(record.Score), 64)
		if err != nil {
			return nil, err
		}
		roundedScore := utils.RoundScoreToTwoDecimals(score)
		recordDO.AnnotateData.Score = &roundedScore
	}

	err = e.annotateService.SaveAnnotateRecord(ctx, req.GetExptID(), req.GetItemID(), req.GetTurnID(), recordDO)
	if err != nil {
		return nil, err
	}
	return &expt.CreateAnnotateRecordResp{
		AnnotateRecordID: id,
		BaseResp:         base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) UpdateAnnotateRecord(ctx context.Context, req *expt.UpdateAnnotateRecordReq) (r *expt.UpdateAnnotateRecordResp, err error) {
	session := entity.NewSession(ctx)
	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.GetExptID(), 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(got.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	})
	if err != nil {
		return nil, err
	}

	record := req.AnnotateRecords
	recordDO := &entity.AnnotateRecord{
		ID:           record.GetAnnotateRecordID(),
		TagKeyID:     record.GetTagKeyID(),
		SpaceID:      req.GetWorkspaceID(),
		ExperimentID: req.GetExptID(),
		TagValueID:   record.GetTagValueID(),
		AnnotateData: &entity.AnnotateData{
			TextValue:      record.PlainText,
			BoolValue:      record.BooleanOption,
			Option:         record.CategoricalOption,
			TagContentType: entity.TagContentType(record.GetTagContentType()),
		},
	}
	if record.Score != nil {
		score, err := strconv.ParseFloat(ptr.From(record.Score), 64)
		if err != nil {
			return nil, err
		}
		roundedScore := utils.RoundScoreToTwoDecimals(score)
		recordDO.AnnotateData.Score = &roundedScore
	}
	err = e.annotateService.UpdateAnnotateRecord(ctx, req.GetItemID(), req.GetTurnID(), recordDO)
	if err != nil {
		return nil, err
	}
	return &expt.UpdateAnnotateRecordResp{
		BaseResp: base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) DeleteAnnotationTag(ctx context.Context, req *expt.DeleteAnnotationTagReq) (r *expt.DeleteAnnotationTagResp, err error) {
	session := entity.NewSession(ctx)
	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.GetExptID(), 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(got.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	})
	if err != nil {
		return nil, err
	}

	err = e.annotateService.DeleteExptTurnResultTagRef(ctx, req.GetExptID(), req.GetWorkspaceID(), req.GetTagKeyID())
	if err != nil {
		return nil, err
	}

	return &expt.DeleteAnnotationTagResp{
		BaseResp: base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) ExportExptResult_(ctx context.Context, req *expt.ExportExptResultRequest) (r *expt.ExportExptResultResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}
	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	if !e.configer.GetExptExportWhiteList(ctx).IsUserIDInWhiteList(session.UserID) {
		err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
			ObjectID:        strconv.FormatInt(req.GetExptID(), 10),
			SpaceID:         req.GetWorkspaceID(),
			ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
			OwnerID:         gptr.Of(got.CreatedBy),
			ResourceSpaceID: req.GetWorkspaceID(),
		})
		if err != nil {
			return nil, err
		}
	}

	exportID, err := e.ExportCSV(ctx, req.GetWorkspaceID(), req.GetExptID(), session)
	if err != nil {
		return nil, err
	}

	return &expt.ExportExptResultResponse{
		ExportID: exportID,
		BaseResp: base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) ListExptResultExportRecord(ctx context.Context, req *expt.ListExptResultExportRecordRequest) (r *expt.ListExptResultExportRecordResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}
	if !e.configer.GetExptExportWhiteList(ctx).IsUserIDInWhiteList(session.UserID) {
		err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
			ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
			SpaceID:       req.WorkspaceID,
			ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
		})
		if err != nil {
			return nil, err
		}
	}

	page := entity.NewPage(int(req.GetPageNumber()), int(req.GetPageSize()))
	records, total, err := e.ListExportRecord(ctx, req.GetWorkspaceID(), req.GetExptID(), page)
	if err != nil {
		return nil, err
	}

	dtos := make([]*domain_expt.ExptResultExportRecord, 0)
	for _, record := range records {
		dtos = append(dtos, experiment.ExportRecordDO2DTO(record))
	}

	userCarriers := make([]userinfo.UserInfoCarrier, 0, len(dtos))
	for _, dto := range dtos {
		userCarriers = append(userCarriers, dto)
	}

	e.userInfoService.PackUserInfo(ctx, userCarriers)

	return &expt.ListExptResultExportRecordResponse{
		ExptResultExportRecords: dtos,
		Total:                   ptr.Of(total),
		BaseResp:                base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) GetExptResultExportRecord(ctx context.Context, req *expt.GetExptResultExportRecordRequest) (r *expt.GetExptResultExportRecordResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}
	if !e.configer.GetExptExportWhiteList(ctx).IsUserIDInWhiteList(session.UserID) {
		err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
			ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
			SpaceID:       req.WorkspaceID,
			ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
		})
		if err != nil {
			return nil, err
		}
	}

	record, err := e.GetExptExportRecord(ctx, req.WorkspaceID, req.ExportID)
	if err != nil {
		return nil, err
	}

	return &expt.GetExptResultExportRecordResponse{
		ExptResultExportRecords: experiment.ExportRecordDO2DTO(record),
		BaseResp:                base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) InsightAnalysisExperiment(ctx context.Context, req *expt.InsightAnalysisExperimentRequest) (r *expt.InsightAnalysisExperimentResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}
	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	// 验证 expt_id 是否属于请求的 workspace_id，防止权限绕过
	if got.SpaceID != req.GetWorkspaceID() {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("experiment %d does not belong to workspace %d", req.GetExptID(), req.GetWorkspaceID())))
	}

	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.GetExptID(), 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(got.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	})
	if err != nil {
		return nil, err
	}

	recordID, err := e.CreateAnalysisRecord(ctx, &entity.ExptInsightAnalysisRecord{
		SpaceID:   req.GetWorkspaceID(),
		ExptID:    req.GetExptID(),
		CreatedBy: session.UserID,
		Status:    entity.InsightAnalysisStatus_Running,
	}, session)
	if err != nil {
		return nil, err
	}
	return &expt.InsightAnalysisExperimentResponse{
		InsightAnalysisRecordID: recordID,
		BaseResp:                base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) ListExptInsightAnalysisRecord(ctx context.Context, req *expt.ListExptInsightAnalysisRecordRequest) (r *expt.ListExptInsightAnalysisRecordResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	// First record contains the upvote/downvote count info for display purpose,
	// Other records' feedback is not necessary for this list api
	records, total, err := e.ListAnalysisRecord(ctx, req.GetWorkspaceID(), req.GetExptID(), entity.NewPage(int(req.GetPageNumber()), int(req.GetPageSize())), session)
	if err != nil {
		return nil, err
	}
	dtos := make([]*domain_expt.ExptInsightAnalysisRecord, 0)
	for _, record := range records {
		dtos = append(dtos, experiment.ExptInsightAnalysisRecordDO2DTO(record))
	}
	return &expt.ListExptInsightAnalysisRecordResponse{
		ExptInsightAnalysisRecords: dtos,
		Total:                      ptr.Of(total),
		BaseResp:                   base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) DeleteExptInsightAnalysisRecord(ctx context.Context, req *expt.DeleteExptInsightAnalysisRecordRequest) (r *expt.DeleteExptInsightAnalysisRecordResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}
	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.GetExptID(), 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(got.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	})
	if err != nil {
		return nil, err
	}
	err = e.DeleteAnalysisRecord(ctx, req.GetWorkspaceID(), req.GetExptID(), req.GetInsightAnalysisRecordID())
	if err != nil {
		return nil, err
	}
	return &expt.DeleteExptInsightAnalysisRecordResponse{
		BaseResp: base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) GetExptInsightAnalysisRecord(ctx context.Context, req *expt.GetExptInsightAnalysisRecordRequest) (r *expt.GetExptInsightAnalysisRecordResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	record, err := e.GetAnalysisRecordByID(ctx, req.GetWorkspaceID(), req.GetExptID(), req.GetInsightAnalysisRecordID(), session)
	if err != nil {
		return nil, err
	}
	return &expt.GetExptInsightAnalysisRecordResponse{
		ExptInsightAnalysisRecord: experiment.ExptInsightAnalysisRecordDO2DTO(record),
		BaseResp:                  base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) FeedbackExptInsightAnalysisReport(ctx context.Context, req *expt.FeedbackExptInsightAnalysisReportRequest) (r *expt.FeedbackExptInsightAnalysisReportResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}
	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	// 验证 expt_id 是否属于请求的 workspace_id，防止权限绕过
	if got.SpaceID != req.GetWorkspaceID() {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("experiment %d does not belong to workspace %d", req.GetExptID(), req.GetWorkspaceID())))
	}

	// 验证 insight_analysis_record_id 是否属于该 expt_id 和 workspace_id，防止水平越权
	record, err := e.GetAnalysisRecordByID(ctx, req.GetWorkspaceID(), req.GetExptID(), req.GetInsightAnalysisRecordID(), session)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("insight analysis record %d not found for experiment %d in workspace %d", req.GetInsightAnalysisRecordID(), req.GetExptID(), req.GetWorkspaceID())))
	}

	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.GetExptID(), 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(got.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	})
	if err != nil {
		return nil, err
	}
	actionType, err := experiment.FeedbackActionType2DO(req.GetFeedbackActionType())
	if err != nil {
		return nil, err
	}
	param := &entity.ExptInsightAnalysisFeedbackParam{
		SpaceID:            req.GetWorkspaceID(),
		ExptID:             req.GetExptID(),
		AnalysisRecordID:   req.GetInsightAnalysisRecordID(),
		FeedbackActionType: actionType,
		Comment:            req.Comment,
		CommentID:          req.CommentID,
		Session:            session,
	}
	err = e.FeedbackExptInsightAnalysis(ctx, param)
	if err != nil {
		return nil, err
	}
	return &expt.FeedbackExptInsightAnalysisReportResponse{
		BaseResp: base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) ListExptInsightAnalysisComment(ctx context.Context, req *expt.ListExptInsightAnalysisCommentRequest) (r *expt.ListExptInsightAnalysisCommentResponse, err error) {
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	comments, total, err := e.ListExptInsightAnalysisFeedbackComment(ctx, req.GetWorkspaceID(), req.GetExptID(), req.GetInsightAnalysisRecordID(), entity.NewPage(int(req.GetPageNumber()), int(req.GetPageSize())))
	if err != nil {
		return nil, err
	}
	dtos := make([]*domain_expt.ExptInsightAnalysisFeedbackComment, 0)
	for _, comment := range comments {
		dtos = append(dtos, experiment.ExptInsightAnalysisFeedbackCommentDO2DTO(comment))
	}
	return &expt.ListExptInsightAnalysisCommentResponse{
		ExptInsightAnalysisFeedbackComments: dtos,
		Total:                               ptr.Of(total),
		BaseResp:                            base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) GetAnalysisRecordFeedbackVote(ctx context.Context, req *expt.GetAnalysisRecordFeedbackVoteRequest) (r *expt.GetAnalysisRecordFeedbackVoteResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}

	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	vote, err := e.GetAnalysisRecordFeedbackVoteByUser(ctx, req.GetWorkspaceID(), req.GetExptID(), req.GetInsightAnalysisRecordID(), session)
	if err != nil {
		return nil, err
	}

	return &expt.GetAnalysisRecordFeedbackVoteResponse{
		Vote:     experiment.ExptInsightAnalysisFeedbackVoteDO2DTO(vote),
		BaseResp: base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) CalculateExperimentAggrResult_(ctx context.Context, req *expt.CalculateExperimentAggrResultRequest) (r *expt.CalculateExperimentAggrResultResponse, err error) {
	session := entity.NewSession(ctx)

	if err := e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionCreateExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}

	got, err := e.manager.Get(ctx, req.GetExptID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	if !entity.IsExptFinished(got.Status) {
		return nil, errorx.NewByCode(errno.IncompleteExptCalcAggrResultErrorCode)
	}

	if err := e.PublishExptAggrResultEvent(ctx, &entity.AggrCalculateEvent{
		SpaceID:       req.GetWorkspaceID(),
		ExperimentID:  req.GetExptID(),
		CalculateMode: entity.CreateAllFields,
	}, gptr.Of(time.Second*3)); err != nil {
		return nil, err
	}

	return &expt.CalculateExperimentAggrResultResponse{BaseResp: base.NewBaseResp()}, nil
}
