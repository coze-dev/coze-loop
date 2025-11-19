// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"sync"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	domain_eval_set "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

var (
	evaluationSetApplicationOnce = sync.Once{}
	evaluationSetApplication     evaluation.EvaluationSetService
)

func NewEvaluationSetApplicationImpl(auth rpc.IAuthProvider,
	evaluationSetService service.IEvaluationSetService,
	evaluationSetSchemaService service.EvaluationSetSchemaService,
	evaluationSetVersionService service.EvaluationSetVersionService,
	evaluationSetItemService service.EvaluationSetItemService,
	metric metrics.EvaluationSetMetrics,
	userInfoService userinfo.UserInfoService,
) evaluation.EvaluationSetService {
	evaluationSetApplicationOnce.Do(func() {
		evaluationSetApplication = &EvaluationSetApplicationImpl{
			auth:                        auth,
			evaluationSetService:        evaluationSetService,
			evaluationSetSchemaService:  evaluationSetSchemaService,
			evaluationSetVersionService: evaluationSetVersionService,
			evaluationSetItemService:    evaluationSetItemService,
			metric:                      metric,
			userInfoService:             userInfoService,
		}
	})

	return evaluationSetApplication
}

type EvaluationSetApplicationImpl struct {
	auth                        rpc.IAuthProvider
	metric                      metrics.EvaluationSetMetrics
	evaluationSetService        service.IEvaluationSetService
	evaluationSetSchemaService  service.EvaluationSetSchemaService
	evaluationSetVersionService service.EvaluationSetVersionService
	evaluationSetItemService    service.EvaluationSetItemService
	userInfoService             userinfo.UserInfoService
}

func (e *EvaluationSetApplicationImpl) CreateEvaluationSet(ctx context.Context, req *eval_set.CreateEvaluationSetRequest) (resp *eval_set.CreateEvaluationSetResponse, err error) {
	defer func() {
		e.metric.EmitCreate(req.GetWorkspaceID(), err)
	}()
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	if err = req.IsValid(); err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(err.Error()))
	}

	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluationSet"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	var session *entity.Session
	if req.Session != nil {
		session = &entity.Session{
			UserID: strconv.FormatInt(gptr.Indirect(req.Session.UserID), 10),
			AppID:  gptr.Indirect(req.Session.AppID),
		}
	}
	id, err := e.evaluationSetService.CreateEvaluationSet(ctx, &entity.CreateEvaluationSetParam{
		SpaceID:             req.WorkspaceID,
		Name:                gptr.Indirect(req.Name),
		Description:         req.Description,
		EvaluationSetSchema: evaluation_set.SchemaDTO2DO(req.EvaluationSetSchema),
		BizCategory:         req.BizCategory,
		Session:             session,
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.CreateEvaluationSetResponse{
		EvaluationSetID: &id,
	}, nil
}

func (e *EvaluationSetApplicationImpl) UpdateEvaluationSet(ctx context.Context, req *eval_set.UpdateEvaluationSetRequest) (resp *eval_set.UpdateEvaluationSetResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	err = e.evaluationSetService.UpdateEvaluationSet(ctx, &entity.UpdateEvaluationSetParam{
		SpaceID:         req.WorkspaceID,
		EvaluationSetID: req.EvaluationSetID,
		Name:            req.Name,
		Description:     req.Description,
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.UpdateEvaluationSetResponse{}, nil
}

func (e *EvaluationSetApplicationImpl) DeleteEvaluationSet(ctx context.Context, req *eval_set.DeleteEvaluationSetRequest) (resp *eval_set.DeleteEvaluationSetResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	err = e.evaluationSetService.DeleteEvaluationSet(ctx, req.WorkspaceID, req.EvaluationSetID)
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.DeleteEvaluationSetResponse{}, nil
}

func (e *EvaluationSetApplicationImpl) GetEvaluationSet(ctx context.Context, req *eval_set.GetEvaluationSetRequest) (resp *eval_set.GetEvaluationSetResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, req.DeletedAt)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("experiment set not found"))
	}
	var ownerID *string
	if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
		ownerID = set.BaseInfo.CreatedBy.UserID
	}
	err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	dto := evaluation_set.EvaluationSetDO2DTO(set)
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier([]*domain_eval_set.EvaluationSet{dto}))
	return &eval_set.GetEvaluationSetResponse{
		EvaluationSet: dto,
	}, nil
}

func (e *EvaluationSetApplicationImpl) ListEvaluationSets(ctx context.Context, req *eval_set.ListEvaluationSetsRequest) (resp *eval_set.ListEvaluationSetsResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationSet"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	sets, total, nextPageToken, err := e.evaluationSetService.ListEvaluationSets(ctx, &entity.ListEvaluationSetsParam{
		SpaceID:          req.WorkspaceID,
		EvaluationSetIDs: req.EvaluationSetIds,
		Name:             req.Name,
		Creators:         req.Creators,
		PageNumber:       req.PageNumber,
		PageSize:         req.PageSize,
		PageToken:        req.PageToken,
		OrderBys:         common.ConvertOrderByDTO2DOs(req.OrderBys),
	})
	if err != nil {
		return nil, err
	}
	dtos := evaluation_set.EvaluationSetDO2DTOs(sets)
	// 返回结果构建、错误处理
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier(dtos))
	return &eval_set.ListEvaluationSetsResponse{
		EvaluationSets: dtos,
		Total:          total,
		NextPageToken:  nextPageToken,
	}, nil
}

func (e *EvaluationSetApplicationImpl) BatchCreateEvaluationSetItems(ctx context.Context, req *eval_set.BatchCreateEvaluationSetItemsRequest) (resp *eval_set.BatchCreateEvaluationSetItemsResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if len(req.Items) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("items is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	idMap, errors, itemOutputs, err := e.evaluationSetItemService.BatchCreateEvaluationSetItems(ctx, &entity.BatchCreateEvaluationSetItemsParam{
		SpaceID:          req.WorkspaceID,
		EvaluationSetID:  req.EvaluationSetID,
		Items:            evaluation_set.ItemDTO2DOs(req.Items),
		SkipInvalidItems: req.SkipInvalidItems,
		AllowPartialAdd:  req.AllowPartialAdd,
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.BatchCreateEvaluationSetItemsResponse{
		AddedItems:  idMap,
		Errors:      evaluation_set.ItemErrorGroupDO2DTOs(errors),
		ItemOutputs: evaluation_set.CreateDatasetItemOutputDO2DTOs(itemOutputs),
	}, nil
}

// UpsertEvaluationSetItem implements the EvaluationSetServiceImpl interface.
func (e *EvaluationSetApplicationImpl) UpdateEvaluationSetItem(ctx context.Context, req *eval_set.UpdateEvaluationSetItemRequest) (resp *eval_set.UpdateEvaluationSetItemResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	err = e.evaluationSetItemService.UpdateEvaluationSetItem(ctx, req.WorkspaceID, req.EvaluationSetID, req.ItemID, evaluation_set.TurnDTO2DOs(req.Turns))
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.UpdateEvaluationSetItemResponse{}, nil
}

// BatchDeleteEvaluationSetItems implements the EvaluationSetServiceImpl interface.
func (e *EvaluationSetApplicationImpl) BatchDeleteEvaluationSetItems(ctx context.Context, req *eval_set.BatchDeleteEvaluationSetItemsRequest) (resp *eval_set.BatchDeleteEvaluationSetItemsResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	err = e.evaluationSetItemService.BatchDeleteEvaluationSetItems(ctx, req.WorkspaceID, req.EvaluationSetID, req.ItemIds)
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.BatchDeleteEvaluationSetItemsResponse{}, nil
}

// ListEvaluationSetItems implements the EvaluationSetServiceImpl interface.
func (e *EvaluationSetApplicationImpl) ListEvaluationSetItems(ctx context.Context, req *eval_set.ListEvaluationSetItemsRequest) (resp *eval_set.ListEvaluationSetItemsResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, gptr.Of(true))
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	items, total, nextCursor, err := e.evaluationSetItemService.ListEvaluationSetItems(ctx, &entity.ListEvaluationSetItemsParam{
		SpaceID:         req.WorkspaceID,
		EvaluationSetID: req.EvaluationSetID,
		VersionID:       req.VersionID,
		PageNumber:      req.PageNumber,
		PageSize:        req.PageSize,
		OrderBys:        common.ConvertOrderByDTO2DOs(req.OrderBys),
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.ListEvaluationSetItemsResponse{
		Items:         evaluation_set.ItemDO2DTOs(items),
		Total:         total,
		NextPageToken: nextCursor,
	}, nil
}

func (e *EvaluationSetApplicationImpl) BatchGetEvaluationSetItems(ctx context.Context, req *eval_set.BatchGetEvaluationSetItemsRequest) (resp *eval_set.BatchGetEvaluationSetItemsResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, gptr.Of(true))
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	items, err := e.evaluationSetItemService.BatchGetEvaluationSetItems(ctx, &entity.BatchGetEvaluationSetItemsParam{
		SpaceID:         req.WorkspaceID,
		EvaluationSetID: req.EvaluationSetID,
		VersionID:       req.VersionID,
		ItemIDs:         req.ItemIds,
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.BatchGetEvaluationSetItemsResponse{
		Items: evaluation_set.ItemDO2DTOs(items),
	}, nil
}

func (e *EvaluationSetApplicationImpl) UpdateEvaluationSetSchema(ctx context.Context, req *eval_set.UpdateEvaluationSetSchemaRequest) (resp *eval_set.UpdateEvaluationSetSchemaResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	err = e.evaluationSetSchemaService.UpdateEvaluationSetSchema(ctx, req.WorkspaceID, req.EvaluationSetID, evaluation_set.FieldSchemaDTO2DOs(req.Fields))
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.UpdateEvaluationSetSchemaResponse{}, nil
}

func (e *EvaluationSetApplicationImpl) CreateEvaluationSetVersion(ctx context.Context, req *eval_set.CreateEvaluationSetVersionRequest) (resp *eval_set.CreateEvaluationSetVersionResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.Version == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("version is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	id, err := e.evaluationSetVersionService.CreateEvaluationSetVersion(ctx, &entity.CreateEvaluationSetVersionParam{
		SpaceID:         req.WorkspaceID,
		EvaluationSetID: req.EvaluationSetID,
		Version:         gptr.Indirect(req.Version),
		Description:     req.Desc,
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.CreateEvaluationSetVersionResponse{
		ID: &id,
	}, nil
}

func (e *EvaluationSetApplicationImpl) GetEvaluationSetVersion(ctx context.Context, req *eval_set.GetEvaluationSetVersionRequest) (resp *eval_set.GetEvaluationSetVersionResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, gptr.Indirect(req.EvaluationSetID), req.DeletedAt)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	version, set, err := e.evaluationSetVersionService.GetEvaluationSetVersion(ctx, req.WorkspaceID, req.VersionID, req.DeletedAt)
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	dto := evaluation_set.EvaluationSetDO2DTO(set)
	versionDTO := evaluation_set.VersionDO2DTO(version)
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier([]*domain_eval_set.EvaluationSetVersion{versionDTO}))
	return &eval_set.GetEvaluationSetVersionResponse{
		Version:       versionDTO,
		EvaluationSet: dto,
	}, nil
}

func (e *EvaluationSetApplicationImpl) ListEvaluationSetVersions(ctx context.Context, req *eval_set.ListEvaluationSetVersionsRequest) (resp *eval_set.ListEvaluationSetVersionsResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	versions, total, nextCursor, err := e.evaluationSetVersionService.ListEvaluationSetVersions(ctx, &entity.ListEvaluationSetVersionsParam{
		SpaceID:         req.WorkspaceID,
		EvaluationSetID: req.EvaluationSetID,
		PageSize:        req.PageSize,
		PageNumber:      req.PageNumber,
		PageToken:       req.PageToken,
		VersionLike:     req.VersionLike,
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	versionDTOs := evaluation_set.VersionDO2DTOs(versions)
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier(versionDTOs))
	return &eval_set.ListEvaluationSetVersionsResponse{
		Versions:      versionDTOs,
		Total:         total,
		NextPageToken: nextCursor,
	}, nil
}

func (e *EvaluationSetApplicationImpl) BatchGetEvaluationSetVersions(ctx context.Context, req *eval_set.BatchGetEvaluationSetVersionsRequest) (resp *eval_set.BatchGetEvaluationSetVersionsResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationSet"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	sets, err := e.evaluationSetVersionService.BatchGetEvaluationSetVersions(ctx, &req.WorkspaceID, req.VersionIds, req.DeletedAt)
	if err != nil {
		return nil, err
	}
	res := make([]*eval_set.VersionedEvaluationSet, 0)
	for _, set := range sets {
		res = append(res, &eval_set.VersionedEvaluationSet{
			EvaluationSet: evaluation_set.EvaluationSetDO2DTO(set.EvaluationSet),
			Version:       evaluation_set.VersionDO2DTO(set.Version),
		})
	}
	return &eval_set.BatchGetEvaluationSetVersionsResponse{
		VersionedEvaluationSets: res,
	}, nil
}

func (e *EvaluationSetApplicationImpl) ClearEvaluationSetDraftItem(ctx context.Context, req *eval_set.ClearEvaluationSetDraftItemRequest) (r *eval_set.ClearEvaluationSetDraftItemResponse, err error) {
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationSet"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	// domain调用
	err = e.evaluationSetItemService.ClearEvaluationSetDraftItem(ctx, req.WorkspaceID, req.EvaluationSetID)
	if err != nil {
		return nil, err
	}
	return &eval_set.ClearEvaluationSetDraftItemResponse{}, nil
}
