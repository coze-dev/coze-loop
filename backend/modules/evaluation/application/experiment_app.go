// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	loopsession "github.com/coze-dev/coze-loop/backend/infra/middleware/session"

	"github.com/coze-dev/coze-loop/backend/infra/backoff"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
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
}

type experimentApplication struct {
	idgen idgen.IIDGenerator
	// tupleSvc  service.IExptTupleService
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
) IExperimentApplication {
	return &experimentApplication{
		resultSvc: resultSvc,
		manager:   manager,
		// tupleSvc:                 tupleSvc,
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
	}
}

func (e *experimentApplication) CreateExperiment(ctx context.Context, req *expt.CreateExperimentRequest) (r *expt.CreateExperimentResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	}
	logs.CtxInfo(ctx, "CreateExperiment userIDInContext: %s", session.UserID)

	param, err := experiment.ConvertCreateReq(req)
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

func (e *experimentApplication) SubmitExperiment(ctx context.Context, req *expt.SubmitExperimentRequest) (r *expt.SubmitExperimentResponse, err error) {
	logs.CtxInfo(ctx, "SubmitExperiment req: %v", json.Jsonify(req))
	if hasDuplicates(req.EvaluatorVersionIds) {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("duplicate evaluator version ids"))
	}

	// 收集 evaluator_version_id（包含顺序解析 EvaluatorIDVersionList）
	evalVersionIDs, err := e.resolveEvaluatorVersionIDs(ctx, req)
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

	cresp, err := e.CreateExperiment(ctx, &expt.CreateExperimentRequest{
		WorkspaceID:           req.GetWorkspaceID(),
		EvalSetVersionID:      req.EvalSetVersionID,
		EvalSetID:             req.EvalSetID,
		EvaluatorVersionIds:   evalVersionIDs,
		Name:                  req.Name,
		Desc:                  req.Desc,
		TargetFieldMapping:    req.TargetFieldMapping,
		EvaluatorFieldMapping: req.EvaluatorFieldMapping,
		ItemConcurNum:         req.ItemConcurNum,
		EvaluatorsConcurNum:   req.EvaluatorsConcurNum,
		CreateEvalTargetParam: req.CreateEvalTargetParam,
		ExptType:              req.ExptType,
		MaxAliveTime:          req.MaxAliveTime,
		SourceType:            req.SourceType,
		SourceID:              req.SourceID,
		TargetRuntimeParam:    req.TargetRuntimeParam,
		Session:               req.Session,
	})
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

	return &expt.SubmitExperimentResponse{
		Experiment: cresp.GetExperiment(),
		RunID:      gptr.Of(rresp.GetRunID()),
		BaseResp:   base.NewBaseResp(),
	}, nil
}

// resolveEvaluatorVersionIDs 汇总 evaluator_version_ids：
// 1) 先取请求中的 EvaluatorVersionIds
// 2) 从有序 EvaluatorIDVersionList 中批量解析并按输入顺序回填版本ID
func (e *experimentApplication) resolveEvaluatorVersionIDs(ctx context.Context, req *expt.SubmitExperimentRequest) ([]int64, error) {
	evalVersionIDs := make([]int64, 0, len(req.EvaluatorVersionIds))
	evalVersionIDs = append(evalVersionIDs, req.EvaluatorVersionIds...)

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
			return nil, err
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
			return nil, err
		}
		for _, ev := range evs {
			if ev == nil {
				continue
			}
			key := fmt.Sprintf("%d#%s", ev.ID, ev.GetVersion())
			pair2Eval[key] = ev
		}
	}

	// 按输入顺序回填版本ID
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

	return evalVersionIDs, nil
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
		ExptAggregateResults: exptAggregateResultDTOs,
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
		recordDO.AnnotateData.Score = &score
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
		recordDO.AnnotateData.Score = &score
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
		ExptResultExportRecord: experiment.ExportRecordDO2DTO(record),
		BaseResp:               base.NewBaseResp(),
	}, nil
}

func (e *experimentApplication) InsightAnalysisExperiment(ctx context.Context, req *expt.InsightAnalysisExperimentRequest) (r *expt.InsightAnalysisExperimentResponse, err error) {
	session := entity.NewSession(ctx)
	if req.Session != nil && req.Session.UserID != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
		}
	} else {
		logs.CtxInfo(ctx, "InsightAnalysisExperiment found empty userID, expt_id: %v, workspace_id: %v", req.GetExptID(), req.GetWorkspaceID())
		userId := loopsession.UserIDInCtxOrEmpty(ctx)
		session = &entity.Session{
			UserID: userId,
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

	var startTime, endTime *int64
	if got.StartAt != nil {
		startTime = gptr.Of(got.StartAt.UnixMilli())
	}
	if got.EndAt != nil {
		endTime = gptr.Of(got.EndAt.UnixMilli())
	}

	recordID, err := e.CreateAnalysisRecord(ctx, &entity.ExptInsightAnalysisRecord{
		SpaceID:   req.GetWorkspaceID(),
		ExptID:    req.GetExptID(),
		CreatedBy: session.UserID,
		Status:    entity.InsightAnalysisStatus_Running,
	}, session, gptr.Indirect(startTime), gptr.Indirect(endTime))
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
