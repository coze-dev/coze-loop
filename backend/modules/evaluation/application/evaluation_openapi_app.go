// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	domainCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domaindoEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiCommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	openapiEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_target"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	domainEvalTarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	experiment_convertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/kitexutil"
)

var (
	evaluationOpenApiApplicationOnce = sync.Once{}
	evaluationOpenApiApplication     evaluation.EvaluationOpenAPIService
)

func NewEvaluationOpenApiApplicationImpl(auth rpc.IAuthProvider,
	evaluationSetService service.IEvaluationSetService,
	evaluationSetVersionService service.EvaluationSetVersionService,
	evaluationSetItemService service.EvaluationSetItemService,
	evaluationSetSchemaService service.EvaluationSetSchemaService,
	metric metrics.OpenAPIEvaluationSetMetrics,
	userInfoService userinfo.UserInfoService,
	experimentApp IExperimentApplication,
) evaluation.EvaluationOpenAPIService {
	evaluationOpenApiApplicationOnce.Do(func() {
		evaluationOpenApiApplication = &EvaluationOpenApiApplicationImpl{
			auth:                        auth,
			evaluationSetService:        evaluationSetService,
			evaluationSetVersionService: evaluationSetVersionService,
			evaluationSetItemService:    evaluationSetItemService,
			evaluationSetSchemaService:  evaluationSetSchemaService,
			metric:                      metric,
			userInfoService:             userInfoService,
			experimentApp:               experimentApp,
		}
	})

	return evaluationOpenApiApplication
}

type EvaluationOpenApiApplicationImpl struct {
	auth                        rpc.IAuthProvider
	evaluationSetService        service.IEvaluationSetService
	evaluationSetVersionService service.EvaluationSetVersionService
	evaluationSetItemService    service.EvaluationSetItemService
	evaluationSetSchemaService  service.EvaluationSetSchemaService
	metric                      metrics.OpenAPIEvaluationSetMetrics
	userInfoService             userinfo.UserInfoService
	experimentApp               IExperimentApplication
}

func (e *EvaluationOpenApiApplicationImpl) CreateEvaluationSetOApi(ctx context.Context, req *openapi.CreateEvaluationSetOApiRequest) (r *openapi.CreateEvaluationSetOApiResponse, err error) {
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

func (e *EvaluationOpenApiApplicationImpl) GetEvaluationSetOApi(ctx context.Context, req *openapi.GetEvaluationSetOApiRequest) (r *openapi.GetEvaluationSetOApiResponse, err error) {
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

func (e *EvaluationOpenApiApplicationImpl) ListEvaluationSetsOApi(ctx context.Context, req *openapi.ListEvaluationSetsOApiRequest) (r *openapi.ListEvaluationSetsOApiResponse, err error) {
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

func (e *EvaluationOpenApiApplicationImpl) CreateEvaluationSetVersionOApi(ctx context.Context, req *openapi.CreateEvaluationSetVersionOApiRequest) (r *openapi.CreateEvaluationSetVersionOApiResponse, err error) {
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
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("errno set not found"))
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

func (e *EvaluationOpenApiApplicationImpl) ListEvaluationSetVersionsOApi(ctx context.Context, req *openapi.ListEvaluationSetVersionsOApiRequest) (r *openapi.ListEvaluationSetVersionsOApiResponse, err error) {
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

func (e *EvaluationOpenApiApplicationImpl) BatchCreateEvaluationSetItemsOApi(ctx context.Context, req *openapi.BatchCreateEvaluationSetItemsOApiRequest) (r *openapi.BatchCreateEvaluationSetItemsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.Items == nil || len(req.Items) == 0 {
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
		Items:            evaluation_set.OpenAPIItemDTO2DOs(req.Items),
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

func (e *EvaluationOpenApiApplicationImpl) BatchUpdateEvaluationSetItemsOApi(ctx context.Context, req *openapi.BatchUpdateEvaluationSetItemsOApiRequest) (r *openapi.BatchUpdateEvaluationSetItemsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.Items == nil || len(req.Items) == 0 {
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
		Items:            evaluation_set.OpenAPIItemDTO2DOs(req.Items),
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

func (e *EvaluationOpenApiApplicationImpl) BatchDeleteEvaluationSetItemsOApi(ctx context.Context, req *openapi.BatchDeleteEvaluationSetItemsOApiRequest) (r *openapi.BatchDeleteEvaluationSetItemsOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.GetIsDeleteAll() == false && (req.ItemIds == nil || len(req.ItemIds) == 0) {
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
	if req.GetIsDeleteAll() == true {
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

func (e *EvaluationOpenApiApplicationImpl) ListEvaluationSetVersionItemsOApi(ctx context.Context, req *openapi.ListEvaluationSetVersionItemsOApiRequest) (r *openapi.ListEvaluationSetVersionItemsOApiResponse, err error) {
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
	items, total, nextPageToken, err := e.evaluationSetItemService.ListEvaluationSetItems(ctx, &entity.ListEvaluationSetItemsParam{
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

func (e *EvaluationOpenApiApplicationImpl) UpdateEvaluationSetSchemaOApi(ctx context.Context, req *openapi.UpdateEvaluationSetSchemaOApiRequest) (r *openapi.UpdateEvaluationSetSchemaOApiResponse, err error) {
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

func (e *EvaluationOpenApiApplicationImpl) SubmitExperimentOApi(ctx context.Context, req *openapi.SubmitExperimentOApiRequest) (r *openapi.SubmitExperimentOApiResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	var spaceID, evaluationSetID int64
	if req != nil {
		spaceID = req.GetWorkspaceID()
	}
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, spaceID, evaluationSetID, kitexutil.GetTOMethod(ctx), startTime, err)
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

	versionID, convErr := strconv.ParseInt(req.EvalSetParam.GetVersion(), 10, 64)
	if convErr != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid eval_set_param.version"))
	}

	version, set, getErr := e.evaluationSetVersionService.GetEvaluationSetVersion(ctx, req.GetWorkspaceID(), versionID, nil)
	if getErr != nil {
		return nil, getErr
	}
	if version == nil || set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluation set version not found"))
	}
	if set.SpaceID != req.GetWorkspaceID() {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluation set version not found"))
	}
	evaluationSetID = set.ID

	if err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
		SpaceID:       req.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionCreateExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}

	if e.experimentApp == nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("experiment app not initialized"))
	}

	params := req.GetEvaluatorParams()
	if len(params) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluator versions is required"))
	}

	evaluatorVersionIDs, parseErr := collectEvaluatorVersionIDs(params)
	if parseErr != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(parseErr.Error()))
	}
	if len(evaluatorVersionIDs) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluator versions is required"))
	}
	if hasDuplicateInt64(evaluatorVersionIDs) {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("duplicate evaluator version ids"))
	}

	targetMapping := convertOpenAPITargetFieldMapping(req.TargetFieldMapping)
	evaluatorFieldMapping := convertOpenAPIEvaluatorFieldMapping(req.EvaluatorFieldMapping)
	runtimeParam := convertOpenAPIRuntimeParam(req.TargetRuntimeParam)
	evalTargetParam, targetErr := convertOpenAPIEvalTargetParam(req.GetEvalTargetParam())
	if targetErr != nil {
		return nil, targetErr
	}

	createResp, err := e.experimentApp.CreateExperiment(ctx, &exptpb.CreateExperimentRequest{
		WorkspaceID:           req.GetWorkspaceID(),
		EvalSetVersionID:      gptr.Of(versionID),
		EvalSetID:             gptr.Of(set.ID),
		EvaluatorVersionIds:   evaluatorVersionIDs,
		Name:                  req.Name,
		Desc:                  req.Description,
		TargetFieldMapping:    targetMapping,
		EvaluatorFieldMapping: evaluatorFieldMapping,
		ItemConcurNum:         req.ItemConcurNum,
		TargetRuntimeParam:    runtimeParam,
		CreateEvalTargetParam: evalTargetParam,
	})
	if err != nil {
		return nil, err
	}
	if createResp == nil || createResp.GetExperiment() == nil || createResp.GetExperiment().ID == nil {
		return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("experiment create failed"))
	}

	_, err = e.experimentApp.RunExperiment(ctx, &exptpb.RunExperimentRequest{
		WorkspaceID: gptr.Of(req.GetWorkspaceID()),
		ExptID:      createResp.GetExperiment().ID,
	})
	if err != nil {
		return nil, err
	}

	return &openapi.SubmitExperimentOApiResponse{
		Data: &openapi.SubmitExperimentOpenAPIData{
			Experiment: experiment_convertor.DomainExperimentDTO2OpenAPI(createResp.GetExperiment()),
		},
	}, nil
}

func collectEvaluatorVersionIDs(params []*openapi.SubmitExperimentEvaluatorParam) ([]int64, error) {
	ids := make([]int64, 0)
	for _, param := range params {
		if param == nil || !param.IsSetVersions() {
			continue
		}
		splits := strings.Split(param.GetVersions(), ",")
		parsed, err := parseEvaluatorVersionStrings(splits)
		if err != nil {
			return nil, err
		}
		ids = append(ids, parsed...)
	}
	return ids, nil
}

func parseEvaluatorVersionStrings(versions []string) ([]int64, error) {
	ids := make([]int64, 0, len(versions))
	for _, v := range versions {
		value := strings.TrimSpace(v)
		if value == "" {
			continue
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func hasDuplicateInt64(values []int64) bool {
	seen := make(map[int64]struct{}, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			return true
		}
		seen[v] = struct{}{}
	}
	return false
}

func convertOpenAPITargetFieldMapping(mapping *openapiExperiment.TargetFieldMapping) *domainExpt.TargetFieldMapping {
	if mapping == nil {
		return nil
	}
	result := &domainExpt.TargetFieldMapping{}
	for _, fm := range mapping.FromEvalSet {
		if fm == nil {
			continue
		}
		field := fm.GetFieldName()
		fromField := fm.GetFromFieldName()
		result.FromEvalSet = append(result.FromEvalSet, &domainExpt.FieldMapping{
			FieldName:     gptr.Of(field),
			FromFieldName: gptr.Of(fromField),
		})
	}
	return result
}

func convertOpenAPIEvaluatorFieldMapping(mappings []*openapiExperiment.EvaluatorFieldMapping) []*domainExpt.EvaluatorFieldMapping {
	if len(mappings) == 0 {
		return nil
	}
	result := make([]*domainExpt.EvaluatorFieldMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}
		domainMapping := &domainExpt.EvaluatorFieldMapping{EvaluatorVersionID: mapping.GetEvaluatorVersionID()}
		for _, fromEval := range mapping.FromEvalSet {
			if fromEval == nil {
				continue
			}
			field := fromEval.GetFieldName()
			fromField := fromEval.GetFromFieldName()
			domainMapping.FromEvalSet = append(domainMapping.FromEvalSet, &domainExpt.FieldMapping{
				FieldName:     gptr.Of(field),
				FromFieldName: gptr.Of(fromField),
			})
		}
		for _, fromTarget := range mapping.FromTarget {
			if fromTarget == nil {
				continue
			}
			field := fromTarget.GetFieldName()
			fromField := fromTarget.GetFromFieldName()
			domainMapping.FromTarget = append(domainMapping.FromTarget, &domainExpt.FieldMapping{
				FieldName:     gptr.Of(field),
				FromFieldName: gptr.Of(fromField),
			})
		}
		result = append(result, domainMapping)
	}
	return result
}

func convertOpenAPIRuntimeParam(param *openapiCommon.RuntimeParam) *domainCommon.RuntimeParam {
	if param == nil {
		return nil
	}
	if !param.IsSetJSONValue() {
		return &domainCommon.RuntimeParam{}
	}
	return &domainCommon.RuntimeParam{JSONValue: gptr.Of(param.GetJSONValue())}
}

func convertOpenAPIEvalTargetParam(param *openapi.SubmitExperimentEvalTargetParam) (*domainEvalTarget.CreateEvalTargetParam, error) {
	if param == nil {
		return nil, nil
	}
	result := &domainEvalTarget.CreateEvalTargetParam{
		SourceTargetID:      param.SourceTargetID,
		SourceTargetVersion: param.SourceTargetVersion,
		BotPublishVersion:   param.BotPublishVersion,
	}
	if param.EvalTargetType != nil {
		typ, err := mapEvalTargetType(*param.EvalTargetType)
		if err != nil {
			return nil, err
		}
		result.EvalTargetType = gptr.Of(typ)
	}
	if param.BotInfoType != nil {
		infoType, err := mapCozeBotInfoType(*param.BotInfoType)
		if err != nil {
			return nil, err
		}
		result.BotInfoType = gptr.Of(infoType)
	}
	return result, nil
}

func mapEvalTargetType(openapiType openapiEvalTarget.EvalTargetType) (domaindoEvalTarget.EvalTargetType, error) {
	switch openapiType {
	case openapiEvalTarget.EvalTargetTypeCozeBot:
		return domaindoEvalTarget.EvalTargetType_CozeBot, nil
	case openapiEvalTarget.EvalTargetTypeCozeLoopPrompt:
		return domaindoEvalTarget.EvalTargetType_CozeLoopPrompt, nil
	case openapiEvalTarget.EvalTargetTypeTrace:
		return domaindoEvalTarget.EvalTargetType_Trace, nil
	case openapiEvalTarget.EvalTargetTypeCozeWorkflow:
		return domaindoEvalTarget.EvalTargetType_CozeWorkflow, nil
	case openapiEvalTarget.EvalTargetTypeVolcengineAgent:
		return domaindoEvalTarget.EvalTargetType_VolcengineAgent, nil
	default:
		return 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("unsupported eval target type"))
	}
}

func mapCozeBotInfoType(openapiType openapiEvalTarget.CozeBotInfoType) (domaindoEvalTarget.CozeBotInfoType, error) {
	switch openapiType {
	case openapiEvalTarget.CozeBotInfoTypeDraftBot:
		return domaindoEvalTarget.CozeBotInfoType_DraftBot, nil
	case openapiEvalTarget.CozeBotInfoTypeProductBot:
		return domaindoEvalTarget.CozeBotInfoType_ProductBot, nil
	default:
		return 0, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("unsupported coze bot info type"))
	}
}

func (e *EvaluationOpenApiApplicationImpl) GetExperimentsOApi(ctx context.Context, req *openapi.GetExperimentsOApiRequest) (r *openapi.GetExperimentsOApiResponse, err error) {
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) ListExperimentResultOApi(ctx context.Context, req *openapi.ListExperimentResultOApiRequest) (r *openapi.ListExperimentResultOApiResponse, err error) {
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) GetExperimentAggrResultOApi(ctx context.Context, req *openapi.GetExperimentAggrResultOApiRequest) (r *openapi.GetExperimentAggrResultOApiResponse, err error) {
	panic("implement me")
}
