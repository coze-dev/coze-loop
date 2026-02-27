// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"

	domaincommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	evaluator_convertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	experiment_convertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/kitexutil"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type IEvalOpenAPIApplication = evaluation.EvalOpenAPIService

type EvalOpenAPIApplication struct {
	targetSvc                   service.IEvalTargetService
	asyncRepo                   repo.IEvalAsyncRepo
	publisher                   events.ExptEventPublisher
	auth                        rpc.IAuthProvider
	evaluationSetService        service.IEvaluationSetService
	evaluationSetVersionService service.EvaluationSetVersionService
	evaluationSetItemService    service.EvaluationSetItemService
	evaluationSetSchemaService  service.EvaluationSetSchemaService
	metric                      metrics.OpenAPIEvaluationMetrics
	userInfoService             userinfo.UserInfoService
	experimentApp               IExperimentApplication
	manager                     service.IExptManager
	resultSvc                   service.ExptResultService
	service.ExptAggrResultService
	evaluatorService       service.EvaluatorService
	evaluatorRecordService service.EvaluatorRecordService
	exptTemplateManager    service.IExptTemplateManager
	configer               component.IConfiger
}

func NewEvalOpenAPIApplication(asyncRepo repo.IEvalAsyncRepo, publisher events.ExptEventPublisher,
	targetSvc service.IEvalTargetService,
	auth rpc.IAuthProvider,
	evaluationSetService service.IEvaluationSetService,
	evaluationSetVersionService service.EvaluationSetVersionService,
	evaluationSetItemService service.EvaluationSetItemService,
	evaluationSetSchemaService service.EvaluationSetSchemaService,
	metric metrics.OpenAPIEvaluationMetrics,
	userInfoService userinfo.UserInfoService,
	experimentApp IExperimentApplication,
	manager service.IExptManager,
	resultSvc service.ExptResultService,
	aggResultSvc service.ExptAggrResultService,
	evaluatorService service.EvaluatorService,
	evaluatorRecordService service.EvaluatorRecordService,
	exptTemplateManager service.IExptTemplateManager,
	configer component.IConfiger,
) IEvalOpenAPIApplication {
	return &EvalOpenAPIApplication{
		asyncRepo:                   asyncRepo,
		publisher:                   publisher,
		targetSvc:                   targetSvc,
		auth:                        auth,
		evaluationSetService:        evaluationSetService,
		evaluationSetVersionService: evaluationSetVersionService,
		evaluationSetItemService:    evaluationSetItemService,
		evaluationSetSchemaService:  evaluationSetSchemaService,
		metric:                      metric,
		userInfoService:             userInfoService,
		experimentApp:               experimentApp,
		manager:                     manager,
		resultSvc:                   resultSvc,
		ExptAggrResultService:       aggResultSvc,
		evaluatorService:            evaluatorService,
		evaluatorRecordService:      evaluatorRecordService,
		exptTemplateManager:         exptTemplateManager,
		configer:                    configer,
	}
}

func (e *EvalOpenAPIApplication) CreateEvaluationSetOApi(ctx context.Context, req *openapi.CreateEvaluationSetOApiRequest) (r *openapi.CreateEvaluationSetOApiResponse, err error) {
	var evaluationSetID int64
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), evaluationSetID, kitexutil.GetTOMethod(ctx), startTime, err)
	}()
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.GetName() == "" {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("name is required"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluationSet"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	// 调用domain服务
	id, err := e.evaluationSetService.CreateEvaluationSet(ctx, &entity.CreateEvaluationSetParam{
		SpaceID:             req.GetWorkspaceID(),
		Name:                req.GetName(),
		Description:         req.Description,
		EvaluationSetSchema: evaluation_set.OpenAPIEvaluationSetSchemaDTO2DO(req.EvaluationSetSchema),
	})
	if err != nil {
		return nil, err
	}

	evaluationSetID = id

	// 构建响应
	return &openapi.CreateEvaluationSetOApiResponse{
		Data: &openapi.CreateEvaluationSetOpenAPIData{
			EvaluationSetID: gptr.Of(id),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) GetEvaluationSetOApi(ctx context.Context, req *openapi.GetEvaluationSetOApiRequest) (r *openapi.GetEvaluationSetOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	// 调用domain服务
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, req.WorkspaceID, req.GetEvaluationSetID(), nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluation set not found"))
	}
	var ownerID *string
	if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
		ownerID = set.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}

	// 数据转换
	dto := evaluation_set.OpenAPIEvaluationSetDO2DTO(set)
	// 构建响应
	return &openapi.GetEvaluationSetOApiResponse{
		Data: &openapi.GetEvaluationSetOpenAPIData{
			EvaluationSet: dto,
		},
	}, nil
}

func (e *EvalOpenAPIApplication) UpdateEvaluationSetOApi(ctx context.Context, req *openapi.UpdateEvaluationSetOApiRequest) (r *openapi.UpdateEvaluationSetOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	// 调用domain服务
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, req.WorkspaceID, req.GetEvaluationSetID(), nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluation set not found"))
	}
	var ownerID *string
	if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
		ownerID = set.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	err = e.evaluationSetService.UpdateEvaluationSet(ctx, &entity.UpdateEvaluationSetParam{
		SpaceID:         req.GetWorkspaceID(),
		EvaluationSetID: req.GetEvaluationSetID(),
		Name:            req.Name,
		Description:     req.Description,
	})
	if err != nil {
		return nil, err
	}
	// 构建响应
	return &openapi.UpdateEvaluationSetOApiResponse{
		Data: &openapi.UpdateEvaluationSetOpenAPIData{},
	}, nil
}

func (e *EvalOpenAPIApplication) DeleteEvaluationSetOApi(ctx context.Context, req *openapi.DeleteEvaluationSetOApiRequest) (r *openapi.DeleteEvaluationSetOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 调用domain服务
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, req.WorkspaceID, req.GetEvaluationSetID(), nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluation set not found"))
	}
	var ownerID *string
	if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
		ownerID = set.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	err = e.evaluationSetService.DeleteEvaluationSet(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID())
	if err != nil {
		return nil, err
	}
	// 构建响应
	return &openapi.DeleteEvaluationSetOApiResponse{
		Data: &openapi.DeleteEvaluationSetOpenAPIData{},
	}, nil
}

func (e *EvalOpenAPIApplication) ListEvaluationSetsOApi(ctx context.Context, req *openapi.ListEvaluationSetsOApiRequest) (r *openapi.ListEvaluationSetsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		// ListEvaluationSets没有单个evaluationSetID，使用0作为占位符
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationSet"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	// 调用domain服务
	sets, total, nextPageToken, err := e.evaluationSetService.ListEvaluationSets(ctx, &entity.ListEvaluationSetsParam{
		SpaceID:          req.GetWorkspaceID(),
		EvaluationSetIDs: req.EvaluationSetIds,
		Name:             req.Name,
		Creators:         req.Creators,
		PageSize:         req.PageSize,
		PageToken:        req.PageToken,
	})
	if err != nil {
		return nil, err
	}

	// 数据转换
	dtos := evaluation_set.OpenAPIEvaluationSetDO2DTOs(sets)

	// 构建响应
	hasMore := sets != nil && len(sets) == int(req.GetPageSize())
	return &openapi.ListEvaluationSetsOApiResponse{
		Data: &openapi.ListEvaluationSetsOpenAPIData{
			Sets:          dtos,
			HasMore:       gptr.Of(hasMore),
			NextPageToken: nextPageToken,
			Total:         total,
		},
	}, nil
}

func (e *EvalOpenAPIApplication) CreateEvaluationSetVersionOApi(ctx context.Context, req *openapi.CreateEvaluationSetVersionOApiRequest) (r *openapi.CreateEvaluationSetVersionOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.Version == nil || *req.Version == "" {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("version is required"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, req.WorkspaceID, req.GetEvaluationSetID(), nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluation set not found"))
	}
	var ownerID *string
	if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
		ownerID = set.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.CreateVersion), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// 调用domain服务
	id, err := e.evaluationSetVersionService.CreateEvaluationSetVersion(ctx, &entity.CreateEvaluationSetVersionParam{
		SpaceID:         req.GetWorkspaceID(),
		EvaluationSetID: req.GetEvaluationSetID(),
		Version:         *req.Version,
		Description:     req.Description,
	})
	if err != nil {
		return nil, err
	}

	// 构建响应
	return &openapi.CreateEvaluationSetVersionOApiResponse{
		Data: &openapi.CreateEvaluationSetVersionOpenAPIData{
			VersionID: gptr.Of(id),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) ListEvaluationSetVersionsOApi(ctx context.Context, req *openapi.ListEvaluationSetVersionsOApiRequest) (r *openapi.ListEvaluationSetVersionsOApiResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, req.WorkspaceID, req.GetEvaluationSetID(), nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("errno set not found"))
	}
	var ownerID *string
	if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
		ownerID = set.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	versions, total, nextCursor, err := e.evaluationSetVersionService.ListEvaluationSetVersions(ctx, &entity.ListEvaluationSetVersionsParam{
		SpaceID:         req.GetWorkspaceID(),
		EvaluationSetID: req.GetEvaluationSetID(),
		PageSize:        req.PageSize,
		PageToken:       req.PageToken,
		VersionLike:     req.VersionLike,
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &openapi.ListEvaluationSetVersionsOApiResponse{
		Data: &openapi.ListEvaluationSetVersionsOpenAPIData{
			Versions:      evaluation_set.OpenAPIEvaluationSetVersionDO2DTOs(versions),
			Total:         total,
			NextPageToken: nextCursor,
		},
	}, nil
}

func (e *EvalOpenAPIApplication) BatchCreateEvaluationSetItemsOApi(ctx context.Context, req *openapi.BatchCreateEvaluationSetItemsOApiRequest) (r *openapi.BatchCreateEvaluationSetItemsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if len(req.Items) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("items is required"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, req.WorkspaceID, req.GetEvaluationSetID(), nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("errno set not found"))
	}
	var ownerID *string
	if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
		ownerID = set.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.AddItem), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// 调用domain服务
	_, errors, itemOutputs, err := e.evaluationSetItemService.BatchCreateEvaluationSetItems(ctx, &entity.BatchCreateEvaluationSetItemsParam{
		SpaceID:          req.GetWorkspaceID(),
		EvaluationSetID:  req.GetEvaluationSetID(),
		Items:            evaluation_set.OpenAPIItemDTO2DOs(req.GetEvaluationSetID(), req.Items),
		SkipInvalidItems: req.IsSkipInvalidItems,
		AllowPartialAdd:  req.IsAllowPartialAdd,
	})
	if err != nil {
		return nil, err
	}

	// 构建响应
	return &openapi.BatchCreateEvaluationSetItemsOApiResponse{
		Data: &openapi.BatchCreateEvaluationSetItemsOpenAPIData{
			ItemOutputs: evaluation_set.OpenAPIDatasetItemOutputDO2DTOs(itemOutputs),
			Errors:      evaluation_set.OpenAPIItemErrorGroupDO2DTOs(errors),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) BatchUpdateEvaluationSetItemsOApi(ctx context.Context, req *openapi.BatchUpdateEvaluationSetItemsOApiRequest) (r *openapi.BatchUpdateEvaluationSetItemsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if len(req.Items) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("items is required"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, req.WorkspaceID, req.GetEvaluationSetID(), nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("errno set not found"))
	}
	var ownerID *string
	if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
		ownerID = set.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.UpdateItem), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}

	// 调用domain服务
	errors, itemOutputs, err := e.evaluationSetItemService.BatchUpdateEvaluationSetItems(ctx, &entity.BatchUpdateEvaluationSetItemsParam{
		SpaceID:          req.GetWorkspaceID(),
		EvaluationSetID:  req.GetEvaluationSetID(),
		Items:            evaluation_set.OpenAPIItemDTO2DOs(req.GetEvaluationSetID(), req.Items),
		SkipInvalidItems: req.IsSkipInvalidItems,
	})
	if err != nil {
		return nil, err
	}

	// 构建响应
	return &openapi.BatchUpdateEvaluationSetItemsOApiResponse{
		Data: &openapi.BatchUpdateEvaluationSetItemsOpenAPIData{
			ItemOutputs: evaluation_set.OpenAPIDatasetItemOutputDO2DTOs(itemOutputs),
			Errors:      evaluation_set.OpenAPIItemErrorGroupDO2DTOs(errors),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) BatchDeleteEvaluationSetItemsOApi(ctx context.Context, req *openapi.BatchDeleteEvaluationSetItemsOApiRequest) (r *openapi.BatchDeleteEvaluationSetItemsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if !req.GetIsDeleteAll() && (len(req.ItemIds) == 0) {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("item_ids is required"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, req.WorkspaceID, req.GetEvaluationSetID(), nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("errno set not found"))
	}
	var ownerID *string
	if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
		ownerID = set.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.DeleteItem), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	if req.GetIsDeleteAll() {
		// 清除所有
		err = e.evaluationSetItemService.ClearEvaluationSetDraftItem(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID())
		if err != nil {
			return nil, err
		}
	} else {
		// 调用domain服务
		err = e.evaluationSetItemService.BatchDeleteEvaluationSetItems(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), req.ItemIds)
		if err != nil {
			return nil, err
		}
	}
	// 构建响应
	return &openapi.BatchDeleteEvaluationSetItemsOApiResponse{}, nil
}

func (e *EvalOpenAPIApplication) ListEvaluationSetVersionItemsOApi(ctx context.Context, req *openapi.ListEvaluationSetVersionItemsOApiRequest) (r *openapi.ListEvaluationSetVersionItemsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, req.WorkspaceID, req.GetEvaluationSetID(), gptr.Of(true))
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("errno set not found"))
	}
	var ownerID *string
	if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
		ownerID = set.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.ReadItem), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}

	// 调用domain服务
	items, total, _, nextPageToken, err := e.evaluationSetItemService.ListEvaluationSetItems(ctx, &entity.ListEvaluationSetItemsParam{
		SpaceID:         req.GetWorkspaceID(),
		EvaluationSetID: req.GetEvaluationSetID(),
		VersionID:       req.VersionID,
		PageSize:        req.PageSize,
		PageToken:       req.PageToken,
	})
	if err != nil {
		return nil, err
	}

	// 数据转换
	dtos := evaluation_set.OpenAPIItemDO2DTOs(items)

	// 构建响应
	hasMore := items != nil && len(items) == int(req.GetPageSize())
	return &openapi.ListEvaluationSetVersionItemsOApiResponse{
		Data: &openapi.ListEvaluationSetVersionItemsOpenAPIData{
			Items:         dtos,
			HasMore:       gptr.Of(hasMore),
			NextPageToken: nextPageToken,
			Total:         total,
		},
	}, nil
}

func (e *EvalOpenAPIApplication) GetEvaluationItemFieldOApi(ctx context.Context, req *openapi.GetEvaluationItemFieldOApiRequest) (r *openapi.GetEvaluationItemFieldOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, req.WorkspaceID, req.GetEvaluationSetID(), gptr.Of(true))
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("errno set not found"))
	}
	var ownerID *string
	if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
		ownerID = set.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.ReadItem), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}

	items, err := e.evaluationSetItemService.BatchGetEvaluationSetItems(ctx, &entity.BatchGetEvaluationSetItemsParam{
		SpaceID:         req.GetWorkspaceID(),
		EvaluationSetID: req.GetEvaluationSetID(),
		VersionID:       req.VersionID,
		ItemIDs:         []int64{req.GetItemID()},
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 || items[0] == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("item not found"))
	}
	// 调用domain服务
	fieldData, err := e.evaluationSetItemService.GetEvaluationSetItemField(ctx, &entity.GetEvaluationSetItemFieldParam{
		SpaceID:         req.GetWorkspaceID(),
		EvaluationSetID: req.GetEvaluationSetID(),
		ItemPK:          items[0].ID,
		FieldName:       req.GetFieldName(),
		TurnID:          req.TurnID,
	})
	if err != nil {
		return nil, err
	}

	// 构建响应
	return &openapi.GetEvaluationItemFieldOApiResponse{
		FieldData: evaluation_set.OpenAPIFieldDataDO2DTO(fieldData),
	}, nil
}

func (e *EvalOpenAPIApplication) UpdateEvaluationSetSchemaOApi(ctx context.Context, req *openapi.UpdateEvaluationSetSchemaOApiRequest) (r *openapi.UpdateEvaluationSetSchemaOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, req.WorkspaceID, req.GetEvaluationSetID(), nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("errno set not found"))
	}
	var ownerID *string
	if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
		ownerID = set.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.EditSchema), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	err = e.evaluationSetSchemaService.UpdateEvaluationSetSchema(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), evaluation_set.OpenAPIFieldSchemaDTO2DOs(req.Fields))
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &openapi.UpdateEvaluationSetSchemaOApiResponse{}, nil
}

func (e *EvalOpenAPIApplication) ReportEvalTargetInvokeResult_(ctx context.Context, req *openapi.ReportEvalTargetInvokeResultRequest) (r *openapi.ReportEvalTargetInvokeResultResponse, err error) {
	logs.CtxInfo(ctx, "ReportEvalTargetInvokeResult receive req: %v", json.Jsonify(req))

	actx, err := e.asyncRepo.GetEvalAsyncCtx(ctx, strconv.FormatInt(req.GetInvokeID(), 10))
	if err != nil {
		return nil, err
	}

	logs.CtxInfo(ctx, "report target record, record_id: %v, space_id: %v, expt_id: %v, expt_run_id: %v, item_id: %v", req.GetInvokeID(), req.GetWorkspaceID(), actx.Event.GetExptID(), actx.Event.GetExptRunID(), actx.Event.GetEvalSetItemID())

	outputData := target.ToInvokeOutputDataDO(req)
	outputData.TimeConsumingMS = gptr.Of(time.Now().UnixMilli() - actx.AsyncUnixMS)
	if err := e.targetSvc.ReportInvokeRecords(ctx, &entity.ReportTargetRecordParam{
		SpaceID:    req.GetWorkspaceID(),
		RecordID:   req.GetInvokeID(),
		OutputData: outputData,
		Status:     target.ToTargetRunStatsDO(req.GetStatus()),
		Session:    actx.Session,
	}); err != nil {
		return nil, err
	}

	if actx.Event != nil {
		if err := e.publisher.PublishExptRecordEvalEvent(ctx, actx.Event, gptr.Of(e.configer.GetTargetTrajectoryConf(ctx).GetExtractInterval(req.GetWorkspaceID())+time.Second*3)); err != nil {
			return nil, err
		}
	}

	return &openapi.ReportEvalTargetInvokeResultResponse{BaseResp: base.NewBaseResp()}, nil
}

func (e *EvalOpenAPIApplication) SubmitExperimentOApi(ctx context.Context, req *openapi.SubmitExperimentOApiRequest) (r *openapi.SubmitExperimentOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	if req.GetWorkspaceID() <= 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("workspace_id is required"))
	}

	if req.EvalSetParam == nil || !req.EvalSetParam.IsSetVersion() || req.EvalSetParam.GetVersion() == "" {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("eval_set_param.version is required"))
	}

	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionCreateExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	session := entity.NewSession(ctx)
	// 检查是否重名
	pass, err := e.manager.CheckName(ctx, req.GetName(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}
	if !pass {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("experiment name already exists"))
	}
	versions, _, _, err := e.evaluationSetVersionService.ListEvaluationSetVersions(ctx, &entity.ListEvaluationSetVersionsParam{
		SpaceID:         req.GetWorkspaceID(),
		EvaluationSetID: req.GetEvalSetParam().GetEvalSetID(),
		PageSize:        gptr.Of(int32(1)),
		Versions:        []string{req.GetEvalSetParam().GetVersion()},
	})
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("eval set not found"))
	}
	evaluatorVersionIDs := make([]int64, 0)
	evaluatorMap := make(map[string]int64)
	for _, evaluator := range req.GetEvaluatorParams() {
		version, _, err := e.evaluatorService.ListEvaluatorVersion(ctx, &entity.ListEvaluatorVersionRequest{
			SpaceID:       req.GetWorkspaceID(),
			EvaluatorID:   evaluator.GetEvaluatorID(),
			QueryVersions: []string{evaluator.GetVersion()},
			PageSize:      int32(1),
			PageNum:       int32(1),
		})
		if err != nil {
			return nil, err
		}
		if len(version) == 0 {
			return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluator not found"))
		}
		var versionID int64
		switch version[0].EvaluatorType {
		case entity.EvaluatorTypePrompt:
			if version[0].PromptEvaluatorVersion != nil {
				versionID = version[0].PromptEvaluatorVersion.ID
			}
		case entity.EvaluatorTypeCode:
			if version[0].CodeEvaluatorVersion != nil {
				versionID = version[0].CodeEvaluatorVersion.ID
			}
		case entity.EvaluatorTypeCustomRPC:
			if version[0].CustomRPCEvaluatorVersion != nil {
				versionID = version[0].CustomRPCEvaluatorVersion.ID
			}
		}
		evaluatorVersionIDs = append(evaluatorVersionIDs, versionID)
		evaluatorMap[fmt.Sprintf("%d_%s", evaluator.GetEvaluatorID(), evaluator.GetVersion())] = versionID
	}

	createReq := &exptpb.SubmitExperimentRequest{
		WorkspaceID:            req.GetWorkspaceID(),
		EvalSetVersionID:       gptr.Of(versions[0].ID),
		EvalSetID:              req.GetEvalSetParam().EvalSetID,
		EvaluatorVersionIds:    evaluatorVersionIDs,
		Name:                   req.Name,
		Desc:                   req.Description,
		TargetFieldMapping:     experiment_convertor.OpenAPITargetFieldMappingDTO2Domain(req.TargetFieldMapping),
		EvaluatorFieldMapping:  experiment_convertor.OpenAPIEvaluatorFieldMappingDTO2Domain(req.EvaluatorFieldMapping, evaluatorMap),
		ItemConcurNum:          req.ItemConcurNum,
		TargetRuntimeParam:     experiment_convertor.OpenAPIRuntimeParamDTO2Domain(req.TargetRuntimeParam),
		CreateEvalTargetParam:  experiment_convertor.OpenAPICreateEvalTargetParamDTO2Domain(req.EvalTargetParam),
		EvaluatorIDVersionList: experiment_convertor.OpenAPIEvaluatorParamsDTO2Domain(req.EvaluatorParams),
	}

	cresp, err := e.experimentApp.SubmitExperiment(ctx, createReq)
	if err != nil {
		return nil, err
	}
	if cresp == nil || cresp.GetExperiment() == nil || cresp.GetExperiment().ID == nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("experiment create failed"))
	}

	return &openapi.SubmitExperimentOApiResponse{
		Data: &openapi.SubmitExperimentOpenAPIData{
			Experiment: experiment_convertor.DomainExperimentDTO2OpenAPI(cresp.GetExperiment()),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) GetExperimentsOApi(ctx context.Context, req *openapi.GetExperimentsOApiRequest) (r *openapi.GetExperimentsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()
	session := entity.NewSession(ctx)
	do, err := e.manager.GetDetail(ctx, req.GetExperimentID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}
	// 鉴权
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(req.GetExperimentID(), 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		OwnerID:         gptr.Of(do.CreatedBy),
		ResourceSpaceID: req.GetWorkspaceID(),
	})
	if err != nil {
		return nil, err
	}
	return &openapi.GetExperimentsOApiResponse{
		Data: &openapi.GetExperimentsOpenAPIDataData{
			Experiment: experiment_convertor.OpenAPIExptDO2DTO(do),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) ListExperimentResultOApi(ctx context.Context, req *openapi.ListExperimentResultOApiRequest) (r *openapi.ListExperimentResultOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetExperimentID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
	})
	if err != nil {
		return nil, err
	}
	param := &entity.MGetExperimentResultParam{
		SpaceID:        req.GetWorkspaceID(),
		BaseExptID:     req.ExperimentID,
		ExptIDs:        []int64{req.GetExperimentID()},
		Page:           entity.NewPage(int(req.GetPageNum()), int(req.GetPageSize())),
		UseAccelerator: true,
	}

	result, err := e.resultSvc.MGetExperimentResult(ctx, param)
	if err != nil {
		return nil, err
	}

	res := &openapi.ListExperimentResultOApiResponse{
		Data: &openapi.ListExperimentResultOpenAPIData{
			ColumnEvalSetFields: experiment_convertor.OpenAPIColumnEvalSetFieldsDO2DTOs(result.ColumnEvalSetFields),
			ColumnEvaluators:    experiment_convertor.OpenAPIColumnEvaluatorsDO2DTOs(result.ColumnEvaluators),
			Total:               gptr.Of(result.Total),
			ItemResults:         experiment_convertor.OpenAPIItemResultsDO2DTOs(result.ItemResults),
		},
	}
	if len(result.ExptColumnsEvalTarget) > 0 {
		res.Data.ColumnEvalTargets = experiment_convertor.OpenAPIColumnEvalTargetDO2DTOs(result.ExptColumnsEvalTarget[0].Columns)
	}
	return res, nil
}

func (e *EvalOpenAPIApplication) GetExperimentAggrResultOApi(ctx context.Context, req *openapi.GetExperimentAggrResultOApiRequest) (r *openapi.GetExperimentAggrResultOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetExperimentID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
	})
	if err != nil {
		return nil, err
	}
	aggrResults, err := e.BatchGetExptAggrResultByExperimentIDs(ctx, req.GetWorkspaceID(), []int64{req.GetExperimentID()})
	if err != nil {
		return nil, err
	}
	if len(aggrResults) == 0 {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("experiment aggr result not found"))
	}
	aggrResult := aggrResults[0]
	res := make([]*experiment.EvaluatorAggregateResult_, 0)
	for _, v := range aggrResult.EvaluatorResults {
		res = append(res, &experiment.EvaluatorAggregateResult_{
			EvaluatorID:        &v.EvaluatorID,
			EvaluatorVersionID: &v.EvaluatorVersionID,
			Name:               v.Name,
			Version:            v.Version,
			AggregatorResults:  experiment_convertor.OpenAPIAggregatorResultsDO2DTOs(v.AggregatorResults),
		})
	}
	return &openapi.GetExperimentAggrResultOApiResponse{
		Data: &openapi.GetExperimentAggrResultOpenAPIData{
			EvaluatorResults:      res,
			EvalTargetAggrResult_: experiment_convertor.OpenTargetAggrResultDO2DTO(aggrResult.TargetResults),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) ListEvaluatorsOApi(ctx context.Context, req *openapi.ListEvaluatorsOApiRequest) (r *openapi.ListEvaluatorsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	var dos []*entity.Evaluator
	var total int64

	if req.GetBuiltin() {
		// 查询预置评估器（与 EvaluatorHandlerImpl.ListEvaluators 一致）
		dos, total, err = e.evaluatorService.ListBuiltinEvaluator(ctx, &entity.ListBuiltinEvaluatorRequest{
			PageSize:     req.GetPageSize(),
			PageNum:      req.GetPageNumber(),
			WithVersion:  req.GetWithVersion(),
			FilterOption: evaluator_convertor.OpenAPIEvaluatorFilterOptionDTO2DO(req.FilterOption),
		})
	} else {
		// 查询普通评估器
		evalTypes := make([]entity.EvaluatorType, 0, len(req.EvaluatorType))
		for _, t := range req.EvaluatorType {
			evalTypes = append(evalTypes, evaluator_convertor.OpenAPIEvaluatorTypeDTO2DO(gptr.Of(t)))
		}
		dos, total, err = e.evaluatorService.ListEvaluator(ctx, &entity.ListEvaluatorRequest{
			SpaceID:       req.GetWorkspaceID(),
			SearchName:    req.GetSearchName(),
			CreatorIDs:    req.CreatorIds,
			EvaluatorType: evalTypes,
			PageSize:      req.GetPageSize(),
			PageNum:       req.GetPageNumber(),
			OrderBys:      common.OpenAPIOrderBysDTO2DO(req.OrderBys),
			WithVersion:   req.GetWithVersion(),
			FilterOption:  evaluator_convertor.OpenAPIEvaluatorFilterOptionDTO2DO(req.FilterOption),
		})
	}
	if err != nil {
		return nil, err
	}

	return &openapi.ListEvaluatorsOApiResponse{
		Data: &openapi.ListEvaluatorsOpenAPIData{
			Evaluators: evaluator_convertor.OpenAPIEvaluatorDO2DTOs(dos),
			Total:      gptr.Of(total),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) BatchGetEvaluatorsOApi(ctx context.Context, req *openapi.BatchGetEvaluatorsOApiRequest) (r *openapi.BatchGetEvaluatorsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	dos, err := e.evaluatorService.BatchGetEvaluator(ctx, req.GetWorkspaceID(), req.EvaluatorIds, req.GetIncludeDeleted())
	if err != nil {
		return nil, err
	}

	return &openapi.BatchGetEvaluatorsOApiResponse{
		Data: &openapi.BatchGetEvaluatorsOpenAPIData{
			Evaluators: evaluator_convertor.OpenAPIEvaluatorDO2DTOs(dos),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) CreateEvaluatorOApi(ctx context.Context, req *openapi.CreateEvaluatorOApiRequest) (r *openapi.CreateEvaluatorOApiResponse, err error) {
	var evaluatorID int64
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		workspaceID := req.GetWorkspaceID()
		e.metric.EmitOpenAPIMetric(ctx, workspaceID, evaluatorID, kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil || req.Evaluator == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req or evaluator is nil"))
	}

	// 如果 Evaluator 中的 WorkspaceID 为 0，则使用请求中的 WorkspaceID
	if req.GetEvaluator() != nil && req.GetEvaluator().GetWorkspaceID() == 0 {
		req.Evaluator.WorkspaceID = req.WorkspaceID
	}

	workspaceID := req.GetWorkspaceID()
	if workspaceID == 0 {
		// 如果请求中没有 workspace_id，尝试从 Evaluator 中获取
		if req.GetEvaluator() != nil {
			workspaceID = req.GetEvaluator().GetWorkspaceID()
		}
		if workspaceID == 0 {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("workspace_id is required"))
		}
	}

	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(workspaceID, 10),
		SpaceID:       workspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	do, err := evaluator_convertor.OpenAPIEvaluatorDTO2DO(req.Evaluator)
	if err != nil {
		return nil, err
	}
	do.SpaceID = workspaceID

	id, err := e.evaluatorService.CreateEvaluator(ctx, do, "")
	if err != nil {
		return nil, err
	}
	evaluatorID = id

	return &openapi.CreateEvaluatorOApiResponse{
		Data: &openapi.CreateEvaluatorOpenAPIData{
			EvaluatorID: gptr.Of(id),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) UpdateEvaluatorOApi(ctx context.Context, req *openapi.UpdateEvaluatorOApiRequest) (r *openapi.UpdateEvaluatorOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluatorID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	evaluator, err := e.evaluatorService.GetEvaluator(ctx, req.GetWorkspaceID(), req.GetEvaluatorID(), false)
	if err != nil {
		return nil, err
	}
	if evaluator == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluator not found"))
	}

	var ownerID *string
	if evaluator.BaseInfo != nil && evaluator.BaseInfo.CreatedBy != nil {
		ownerID = evaluator.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(evaluator.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
		OwnerID:         ownerID,
		ResourceSpaceID: evaluator.SpaceID,
	})
	if err != nil {
		return nil, err
	}

	updateReq := &entity.UpdateEvaluatorMetaRequest{
		ID:          req.GetEvaluatorID(),
		SpaceID:     req.GetWorkspaceID(),
		Name:        req.Name,
		Description: req.Description,
	}

	err = e.evaluatorService.UpdateEvaluatorMeta(ctx, updateReq)
	if err != nil {
		return nil, err
	}

	return &openapi.UpdateEvaluatorOApiResponse{
		Data: &openapi.UpdateEvaluatorOpenAPIData{},
	}, nil
}

func (e *EvalOpenAPIApplication) UpdateEvaluatorDraftOApi(ctx context.Context, req *openapi.UpdateEvaluatorDraftOApiRequest) (r *openapi.UpdateEvaluatorDraftOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluatorID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	evaluator, err := e.evaluatorService.GetEvaluator(ctx, req.GetWorkspaceID(), req.GetEvaluatorID(), false)
	if err != nil {
		return nil, err
	}
	if evaluator == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluator not found"))
	}

	var ownerID *string
	if evaluator.BaseInfo != nil && evaluator.BaseInfo.CreatedBy != nil {
		ownerID = evaluator.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(evaluator.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
		OwnerID:         ownerID,
		ResourceSpaceID: evaluator.SpaceID,
	})
	if err != nil {
		return nil, err
	}

	evalType := evaluator_convertor.OpenAPIEvaluatorTypeDTO2DO(req.EvaluatorType)
	verDO, err := evaluator_convertor.OpenAPIEvaluatorContentDTO2DO(req.EvaluatorContent, evalType)
	if err != nil {
		return nil, err
	}

	evaluator.EvaluatorType = evalType
	evaluator.SetEvaluatorVersion(verDO)

	err = e.evaluatorService.UpdateEvaluatorDraft(ctx, evaluator)
	if err != nil {
		return nil, err
	}

	return &openapi.UpdateEvaluatorDraftOApiResponse{
		Data: &openapi.UpdateEvaluatorDraftOpenAPIData{
			Evaluator: evaluator_convertor.OpenAPIEvaluatorDO2DTO(evaluator),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) DeleteEvaluatorOApi(ctx context.Context, req *openapi.DeleteEvaluatorOApiRequest) (r *openapi.DeleteEvaluatorOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluatorID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	evaluator, err := e.evaluatorService.GetEvaluator(ctx, req.GetWorkspaceID(), req.GetEvaluatorID(), false)
	if err != nil {
		return nil, err
	}
	if evaluator == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluator not found"))
	}

	var ownerID *string
	if evaluator.BaseInfo != nil && evaluator.BaseInfo.CreatedBy != nil {
		ownerID = evaluator.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(evaluator.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
		OwnerID:         ownerID,
		ResourceSpaceID: evaluator.SpaceID,
	})
	if err != nil {
		return nil, err
	}

	err = e.evaluatorService.DeleteEvaluator(ctx, []int64{req.GetEvaluatorID()}, "")
	if err != nil {
		return nil, err
	}

	return &openapi.DeleteEvaluatorOApiResponse{
		Data: &openapi.DeleteEvaluatorOpenAPIData{},
	}, nil
}

func (e *EvalOpenAPIApplication) ListEvaluatorVersionsOApi(ctx context.Context, req *openapi.ListEvaluatorVersionsOApiRequest) (r *openapi.ListEvaluatorVersionsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluatorID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	evaluator, err := e.evaluatorService.GetEvaluator(ctx, req.GetWorkspaceID(), req.GetEvaluatorID(), false)
	if err != nil {
		return nil, err
	}
	if evaluator == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluator not found"))
	}

	var ownerID *string
	if evaluator.BaseInfo != nil && evaluator.BaseInfo.CreatedBy != nil {
		ownerID = evaluator.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(evaluator.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
		OwnerID:         ownerID,
		ResourceSpaceID: evaluator.SpaceID,
	})
	if err != nil {
		return nil, err
	}

	dos, total, err := e.evaluatorService.ListEvaluatorVersion(ctx, &entity.ListEvaluatorVersionRequest{
		SpaceID:       req.GetWorkspaceID(),
		EvaluatorID:   req.GetEvaluatorID(),
		QueryVersions: req.QueryVersions,
		PageSize:      req.GetPageSize(),
		PageNum:       req.GetPageNumber(),
		OrderBys:      common.OpenAPIOrderBysDTO2DO(req.OrderBys),
	})
	if err != nil {
		return nil, err
	}

	return &openapi.ListEvaluatorVersionsOApiResponse{
		Data: &openapi.ListEvaluatorVersionsOpenAPIData{
			EvaluatorVersions: evaluator_convertor.OpenAPIEvaluatorVersionDO2DTOs(dos),
			Total:             gptr.Of(total),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) BatchGetEvaluatorVersionsOApi(ctx context.Context, req *openapi.BatchGetEvaluatorVersionsOApiRequest) (r *openapi.BatchGetEvaluatorVersionsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	dos, err := e.evaluatorService.BatchGetEvaluatorVersion(ctx, gptr.Of(req.GetWorkspaceID()), req.EvaluatorVersionIds, req.GetIncludeDeleted())
	if err != nil {
		return nil, err
	}

	return &openapi.BatchGetEvaluatorVersionsOApiResponse{
		Data: &openapi.BatchGetEvaluatorVersionsOpenAPIData{
			Evaluators: evaluator_convertor.OpenAPIEvaluatorDO2DTOs(dos),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) SubmitEvaluatorVersionOApi(ctx context.Context, req *openapi.SubmitEvaluatorVersionOApiRequest) (r *openapi.SubmitEvaluatorVersionOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluatorID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	evaluator, err := e.evaluatorService.GetEvaluator(ctx, req.GetWorkspaceID(), req.GetEvaluatorID(), false)
	if err != nil {
		return nil, err
	}
	if evaluator == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluator not found"))
	}

	var ownerID *string
	if evaluator.BaseInfo != nil && evaluator.BaseInfo.CreatedBy != nil {
		ownerID = evaluator.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(evaluator.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.CreateVersion), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
		OwnerID:         ownerID,
		ResourceSpaceID: evaluator.SpaceID,
	})
	if err != nil {
		return nil, err
	}

	res, err := e.evaluatorService.SubmitEvaluatorVersion(ctx, evaluator, req.GetVersion(), req.GetDescription(), "")
	if err != nil {
		return nil, err
	}

	return &openapi.SubmitEvaluatorVersionOApiResponse{
		Data: &openapi.SubmitEvaluatorVersionOpenAPIData{
			Evaluator: evaluator_convertor.OpenAPIEvaluatorDO2DTO(res),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) RunEvaluatorOApi(ctx context.Context, req *openapi.RunEvaluatorOApiRequest) (r *openapi.RunEvaluatorOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluatorVersionID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	// 校验评估器版本是否存在且有权限
	evaluator, err := e.evaluatorService.GetEvaluatorVersion(ctx, gptr.Of(req.GetWorkspaceID()), req.GetEvaluatorVersionID(), false, false)
	if err != nil {
		return nil, err
	}
	if evaluator == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluator version not found"))
	}

	var ownerID *string
	if evaluator.BaseInfo != nil && evaluator.BaseInfo.CreatedBy != nil {
		ownerID = evaluator.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(evaluator.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
		OwnerID:         ownerID,
		ResourceSpaceID: evaluator.SpaceID,
	})
	if err != nil {
		return nil, err
	}

	inputData := evaluator_convertor.OpenAPIEvaluatorInputDataDTO2DO(req.InputData)
	runConf := evaluator_convertor.OpenAPIEvaluatorRunConfigDTO2DO(req.EvaluatorRunConf)
	// 与 EvaluatorHandlerImpl.buildRunEvaluatorRequest 一致：将 evaluator_runtime_param 注入到 InputData.Ext，供下游执行时使用
	if runConf != nil && runConf.EvaluatorRuntimeParam != nil && runConf.EvaluatorRuntimeParam.JSONValue != nil && len(*runConf.EvaluatorRuntimeParam.JSONValue) > 0 {
		if inputData == nil {
			inputData = &entity.EvaluatorInputData{}
		}
		if inputData.Ext == nil {
			inputData.Ext = make(map[string]string)
		}
		inputData.Ext[consts.FieldAdapterBuiltinFieldNameRuntimeParam] = *runConf.EvaluatorRuntimeParam.JSONValue
	}

	record, err := e.evaluatorService.RunEvaluator(ctx, &entity.RunEvaluatorRequest{
		SpaceID:            req.GetWorkspaceID(),
		EvaluatorVersionID: req.GetEvaluatorVersionID(),
		InputData:          inputData,
		EvaluatorRunConf:   runConf,
		Ext:                req.Ext,
	})
	if err != nil {
		return nil, err
	}

	return &openapi.RunEvaluatorOApiResponse{
		Data: &openapi.RunEvaluatorOpenAPIData{
			Record: evaluator_convertor.OpenAPIEvaluatorRecordDO2DTO(record),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) CorrectEvaluatorRecordOApi(ctx context.Context, req *openapi.CorrectEvaluatorRecordOApiRequest) (r *openapi.CorrectEvaluatorRecordOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluatorRecordID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	record, err := e.evaluatorRecordService.GetEvaluatorRecord(ctx, req.GetEvaluatorRecordID(), false)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluator record not found"))
	}

	// 鉴权，评估记录属于某个实验，这里检查实验的编辑权限
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(record.ExperimentID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExperiment)}},
		ResourceSpaceID: record.SpaceID,
	})
	if err != nil {
		return nil, err
	}

	correction := evaluator_convertor.OpenAPICorrectionDTO2DO(req.Correction)
	err = e.evaluatorRecordService.CorrectEvaluatorRecord(ctx, record, correction)
	if err != nil {
		return nil, err
	}

	return &openapi.CorrectEvaluatorRecordOApiResponse{
		Data: &openapi.CorrectEvaluatorRecordOpenAPIData{
			Record: evaluator_convertor.OpenAPIEvaluatorRecordDO2DTO(record),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) BatchGetEvaluatorRecordsOApi(ctx context.Context, req *openapi.BatchGetEvaluatorRecordsOApiRequest) (r *openapi.BatchGetEvaluatorRecordsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	// 批量查询评估记录，与非 OpenAPI 接口一致，按空间 listLoopEvaluator 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	dos, err := e.evaluatorRecordService.BatchGetEvaluatorRecord(ctx, req.EvaluatorRecordIds, req.GetIncludeDeleted())
	if err != nil {
		return nil, err
	}

	return &openapi.BatchGetEvaluatorRecordsOApiResponse{
		Data: &openapi.BatchGetEvaluatorRecordsOpenAPIData{
			Records: evaluator_convertor.OpenAPIEvaluatorRecordDO2DTOs(dos),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) CreateExptTemplateOApi(ctx context.Context, req *openapi.CreateExptTemplateOApiRequest) (r *openapi.CreateExptTemplateOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionCreateExptTemplate), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	param, err := experiment_convertor.OpenAPICreateExptTemplateReq2Domain(req)
	if err != nil {
		return nil, err
	}

	session := entity.NewSession(ctx)
	do, err := e.exptTemplateManager.Create(ctx, param, session)
	if err != nil {
		return nil, err
	}

	return &openapi.CreateExptTemplateOApiResponse{
		Data: &openapi.CreateExptTemplateOpenAPIData{
			ExperimentTemplate: experiment_convertor.OpenAPIExptTemplateDO2DTO(do),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) BatchGetExptTemplatesOApi(ctx context.Context, req *openapi.BatchGetExptTemplatesOApiRequest) (r *openapi.BatchGetExptTemplatesOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExptTemplate), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	session := entity.NewSession(ctx)
	dos, err := e.exptTemplateManager.MGet(ctx, req.TemplateIds, req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	return &openapi.BatchGetExptTemplatesOApiResponse{
		Data: &openapi.BatchGetExptTemplatesOpenAPIData{
			ExperimentTemplates: experiment_convertor.OpenAPIExptTemplateDO2DTOs(dos),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) SubmitExptFromTemplateOApi(ctx context.Context, req *openapi.SubmitExptFromTemplateOApiRequest) (r *openapi.SubmitExptFromTemplateOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.GetWorkspaceID() <= 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("workspace_id is required"))
	}
	if req.GetTemplateID() <= 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("template_id is required"))
	}

	name := req.GetName()
	if name == "" {
		name = fmt.Sprintf("实验模板_%d", time.Now().Unix())
	}

	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionCreateExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	session := entity.NewSession(ctx)
	template, err := e.exptTemplateManager.Get(ctx, req.GetTemplateID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("experiment template not found"))
	}

	// 检查实验名称是否重复
	pass, err := e.manager.CheckName(ctx, name, req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}
	if !pass {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("experiment name already exists"))
	}

	submitReq := experiment_convertor.TemplateToSubmitExperimentRequest(template, name, req.GetWorkspaceID())
	if submitReq == nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("failed to build submit request from template"))
	}
	submitReq.Session = &domaincommon.Session{}
	if session.UserID != "" {
		if userID, parseErr := strconv.ParseInt(session.UserID, 10, 64); parseErr == nil {
			submitReq.Session.UserID = gptr.Of(userID)
		}
	}

	cresp, err := e.experimentApp.SubmitExperiment(ctx, submitReq)
	if err != nil {
		return nil, err
	}
	if cresp == nil || cresp.GetExperiment() == nil || cresp.GetExperiment().ID == nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("experiment create failed"))
	}

	return &openapi.SubmitExptFromTemplateOApiResponse{
		Data: &openapi.SubmitExptFromTemplateOpenAPIData{
			Experiment: experiment_convertor.DomainExperimentDTO2OpenAPI(cresp.GetExperiment()),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) UpdateExptTemplateMetaOApi(ctx context.Context, req *openapi.UpdateExptTemplateMetaOApiRequest) (r *openapi.UpdateExptTemplateMetaOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetTemplateID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	session := entity.NewSession(ctx)
	template, err := e.exptTemplateManager.Get(ctx, req.GetTemplateID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("experiment template not found"))
	}

	var ownerID *string
	if template.BaseInfo != nil && template.BaseInfo.CreatedBy != nil {
		ownerID = template.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(template.Meta.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExptTemplate)}},
		OwnerID:         ownerID,
		ResourceSpaceID: template.Meta.WorkspaceID,
	})
	if err != nil {
		return nil, err
	}

	param := &entity.UpdateExptTemplateMetaParam{
		TemplateID:  req.GetTemplateID(),
		SpaceID:     req.GetWorkspaceID(),
		Name:        req.GetMeta().GetName(),
		Description: req.GetMeta().GetDescription(),
		ExptType:    experiment_convertor.OpenAPIExptTypeDTO2DO(req.GetMeta().ExptType),
	}

	do, err := e.exptTemplateManager.UpdateMeta(ctx, param, session)
	if err != nil {
		return nil, err
	}

	return &openapi.UpdateExptTemplateMetaOApiResponse{
		Data: &openapi.UpdateExptTemplateMetaOpenAPIData{
			Meta: &experiment.ExptTemplateMeta{
				ID:          gptr.Of(do.Meta.ID),
				WorkspaceID: gptr.Of(do.Meta.WorkspaceID),
				Name:        gptr.Of(do.Meta.Name),
				Description: gptr.Of(do.Meta.Desc),
				ExptType:    experiment_convertor.OpenAPIExptTypeDO2DTO(do.Meta.ExptType),
			},
		},
	}, nil
}

func (e *EvalOpenAPIApplication) UpdateExptTemplateOApi(ctx context.Context, req *openapi.UpdateExptTemplateOApiRequest) (r *openapi.UpdateExptTemplateOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetTemplateID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	session := entity.NewSession(ctx)
	template, err := e.exptTemplateManager.Get(ctx, req.GetTemplateID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("experiment template not found"))
	}

	var ownerID *string
	if template.BaseInfo != nil && template.BaseInfo.CreatedBy != nil {
		ownerID = template.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(template.Meta.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExptTemplate)}},
		OwnerID:         ownerID,
		ResourceSpaceID: template.Meta.WorkspaceID,
	})
	if err != nil {
		return nil, err
	}

	param, err := experiment_convertor.OpenAPIUpdateExptTemplateReq2Domain(req)
	if err != nil {
		return nil, err
	}

	do, err := e.exptTemplateManager.Update(ctx, param, session)
	if err != nil {
		return nil, err
	}

	return &openapi.UpdateExptTemplateOApiResponse{
		Data: &openapi.UpdateExptTemplateOpenAPIData{
			ExperimentTemplate: experiment_convertor.OpenAPIExptTemplateDO2DTO(do),
		},
	}, nil
}

func (e *EvalOpenAPIApplication) DeleteExptTemplateOApi(ctx context.Context, req *openapi.DeleteExptTemplateOApiRequest) (r *openapi.DeleteExptTemplateOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetTemplateID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	session := entity.NewSession(ctx)
	template, err := e.exptTemplateManager.Get(ctx, req.GetTemplateID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("experiment template not found"))
	}

	var ownerID *string
	if template.BaseInfo != nil && template.BaseInfo.CreatedBy != nil {
		ownerID = template.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(template.Meta.ID, 10),
		SpaceID:         req.GetWorkspaceID(),
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationExptTemplate)}},
		OwnerID:         ownerID,
		ResourceSpaceID: template.Meta.WorkspaceID,
	})
	if err != nil {
		return nil, err
	}

	err = e.exptTemplateManager.Delete(ctx, req.GetTemplateID(), req.GetWorkspaceID(), session)
	if err != nil {
		return nil, err
	}

	return &openapi.DeleteExptTemplateOApiResponse{
		Data: &openapi.DeleteExptTemplateOpenAPIData{},
	}, nil
}

func (e *EvalOpenAPIApplication) ListExptTemplatesOApi(ctx context.Context, req *openapi.ListExptTemplatesOApiRequest) (r *openapi.ListExptTemplatesOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExptTemplate), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	var filter *entity.ExptTemplateListFilter
	if req.FilterOption != nil {
		filter = experiment_convertor.OpenAPIExptTemplateFilterDTO2DO(req.FilterOption)
	}

	session := entity.NewSession(ctx)
	dos, total, err := e.exptTemplateManager.List(ctx, req.GetPageNumber(), req.GetPageSize(), req.GetWorkspaceID(), filter, common.OpenAPIOrderBysDTO2DO(req.OrderBys), session)
	if err != nil {
		return nil, err
	}

	return &openapi.ListExptTemplatesOApiResponse{
		Data: &openapi.ListExptTemplatesOpenAPIData{
			ExperimentTemplates: experiment_convertor.OpenAPIExptTemplateDO2DTOs(dos),
			Total:               gptr.Of(int32(total)),
		},
	}, nil
}
