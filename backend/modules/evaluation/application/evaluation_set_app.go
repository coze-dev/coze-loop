// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	common_domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
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
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
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
	resourceAccessAuthorizer service.ResourceAccessAuthorizer,
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
			resourceAccessAuthorizer:    resourceAccessAuthorizer,
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
	resourceAccessAuthorizer    service.ResourceAccessAuthorizer
}

func sharedOptionDTO2DO(option *common_domain.SharedResourceOption) *entity.SharedResourceOption {
	if option == nil {
		return nil
	}
	sharedOption := &entity.SharedResourceOption{
		IsShared: option.GetIsShared(),
	}
	if option.SourceSpaceID != nil {
		sharedOption.SourceSpaceID = option.SourceSpaceID
	}
	if !sharedOption.Enabled() {
		return nil
	}
	return sharedOption
}

// buildEvalSetAuthorizeRequest 组装评测集读授权入参：从 set 取 owner，供 domain 层统一鉴权。
func buildEvalSetAuthorizeRequest(callerSpaceID int64, set *entity.EvaluationSet, sharedOption *entity.SharedResourceOption, versionID *int64, versionName *string, requireContentRead bool) *entity.AuthorizeResourceRequest {
	req := &entity.AuthorizeResourceRequest{
		CallerSpaceID:      callerSpaceID,
		ResourceType:       entity.SharedResourceTypeEvalSet,
		SharedOption:       sharedOption,
		Action:             consts.Read,
		VersionID:          versionID,
		VersionName:        versionName,
		RequireContentRead: requireContentRead,
	}
	if set != nil {
		req.ResourceID = set.ID
		if set.BaseInfo != nil && set.BaseInfo.CreatedBy != nil {
			req.OwnerID = set.BaseInfo.CreatedBy.UserID
		}
	}
	return req
}

func shouldRedactSharedEvaluationSetSchema(accessCtx *entity.ResourceAccessContext) bool {
	return accessCtx != nil && accessCtx.IsShared() && accessCtx.AccessLevel == entity.SharedAccessLevelExecute
}

func redactEvaluationSetVersionSchemas(accessCtx *entity.ResourceAccessContext, versions []*domain_eval_set.EvaluationSetVersion) {
	if !shouldRedactSharedEvaluationSetSchema(accessCtx) {
		return
	}
	for _, version := range versions {
		if version != nil {
			version.EvaluationSetSchema = nil
		}
	}
}

func (e *EvaluationSetApplicationImpl) CreateEvaluationSet(ctx context.Context, req *eval_set.CreateEvaluationSetRequest) (resp *eval_set.CreateEvaluationSetResponse, err error) {
	// TODO: remove debug logging after versioned_item feature is stable
	logs.CtxInfo(ctx, "CreateEvaluationSet req: %v", json.Jsonify(req))
	defer func() {
		logs.CtxInfo(ctx, "CreateEvaluationSet resp: %v, err: %v", json.Jsonify(resp), err)
		e.metric.EmitCreate(req.GetWorkspaceID(), err)
	}()
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.Name == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("name is nil"))
	}
	if req.EvaluationSetSchema == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("schema is nil"))
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
		DatasetType:         req.Type,
		Tags:                evaluation_set.ResourceTagRefDTO2DOs(req.Tags),
		DatasetKey:          req.DatasetKey,
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.CreateEvaluationSetResponse{
		EvaluationSetID: &id,
	}, nil
}

func (e *EvaluationSetApplicationImpl) CreateEvaluationSetWithImport(ctx context.Context, req *eval_set.CreateEvaluationSetWithImportRequest) (r *eval_set.CreateEvaluationSetWithImportResponse, err error) {
	defer func() {
		e.metric.EmitCreate(req.GetWorkspaceID(), err)
	}()
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.Name == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("name is nil"))
	}
	if req.EvaluationSetSchema == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("schema is nil"))
	}
	if req.Source == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("source is nil"))
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
	id, jobID, err := e.evaluationSetService.CreateEvaluationSetWithImport(ctx, &entity.CreateEvaluationSetWithImportParam{
		SpaceID:             req.WorkspaceID,
		Name:                gptr.Indirect(req.Name),
		Description:         req.Description,
		EvaluationSetSchema: evaluation_set.SchemaDTO2DO(req.EvaluationSetSchema),
		BizCategory:         req.BizCategory,
		SourceType:          evaluation_set.SourceTypeDTO2DO(req.SourceType),
		Source:              evaluation_set.DatasetIOEndpointDTO2DO(req.Source),
		FieldMappings:       evaluation_set.FieldMappingsDTO2DOs(req.FieldMappings),
		Session:             session,
		Option:              evaluation_set.DatasetIOJobOptionDTO2DO(req.Option),
		DatasetType:         req.Type,
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.CreateEvaluationSetWithImportResponse{
		EvaluationSetID: gptr.Of(id),
		JobID:           gptr.Of(jobID),
	}, nil
}

func (e *EvaluationSetApplicationImpl) ParseImportSourceFile(ctx context.Context, req *eval_set.ParseImportSourceFileRequest) (r *eval_set.ParseImportSourceFileResponse, err error) {
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.File == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("file is nil"))
	}

	if err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluationSet"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}

	param := &entity.ParseImportSourceFileParam{
		SpaceID: req.WorkspaceID,
		File:    evaluation_set.DatasetIOFileDTO2DO(req.GetFile()),
	}

	result, err := e.evaluationSetService.ParseImportSourceFile(ctx, param)
	if err != nil {
		return nil, err
	}

	resp := &eval_set.ParseImportSourceFileResponse{
		BaseResp: base.NewBaseResp(),
	}
	if result != nil {
		resp.Bytes = gptr.Of(result.Bytes)
		resp.FieldSchemas = evaluation_set.FieldSchemaDO2DTOs(result.FieldSchemas)
		resp.Conflicts = evaluation_set.ConflictFieldDO2DTOs(result.Conflicts)
		resp.FilesWithAmbiguousColumn = result.FilesWithAmbiguousColumn
		resp.UntypedURLFields = result.UntypedURLFields
		resp.PrecheckDataByField = result.PrecheckDataByField
	}

	return resp, nil
}

func (e *EvaluationSetApplicationImpl) ValidateEvaluationSetMultiPartData(ctx context.Context, req *eval_set.ValidateEvaluationSetMultiPartDataRequest) (r *eval_set.ValidateEvaluationSetMultiPartDataResponse, err error) {
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	if err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.SpaceID, 10),
		SpaceID:       req.SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluationSet"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}

	details, err := e.evaluationSetService.ValidateMultiPartData(ctx, req.SpaceID, req.GetPreviewData(), evaluation_set.MultiModalStoreOptionDTO2DO(req.GetStoreOption()))
	if err != nil {
		return nil, err
	}

	return &eval_set.ValidateEvaluationSetMultiPartDataResponse{
		BaseResp:                  base.NewBaseResp(),
		AttachmentUrlsCheckDetail: evaluation_set.UploadAttachmentDetailsDO2DTOs(details),
	}, nil
}

func (e *EvaluationSetApplicationImpl) UpdateEvaluationSet(ctx context.Context, req *eval_set.UpdateEvaluationSetRequest) (resp *eval_set.UpdateEvaluationSetResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil, nil)
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
		Tags:            evaluation_set.ResourceTagRefDTO2DOs(req.Tags),
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
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil, nil)
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
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, req.DeletedAt, nil)
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
	if err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(set.ID, 10),
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	}); err != nil {
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
	tagFilter, err := evaluation_set.TagFilterDTO2DO(req.TagFilter)
	if err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(err.Error()))
	}
	// is_shared=true：按共享配置枚举「共享给调用方空间」的评测集（source 为空则跨全部来源空间）。
	if req.SharedOption != nil && req.SharedOption.GetIsShared() {
		return e.listSharedEvaluationSets(ctx, req)
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
		TagFilter:        tagFilter,
		DatasetKeys:      req.DatasetKeys,
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

// listSharedEvaluationSets 按共享配置枚举「共享给调用方空间」的评测集。
// source_space_id 为空 → 跨全部来源空间枚举；有值 → 限定该来源空间。
// 逐来源空间加载资源并回填共享元信息；空配置 → 空列表（fail-closed）。
func (e *EvaluationSetApplicationImpl) listSharedEvaluationSets(ctx context.Context, req *eval_set.ListEvaluationSetsRequest) (*eval_set.ListEvaluationSetsResponse, error) {
	if (req.Name != nil && strings.TrimSpace(*req.Name) != "") ||
		len(req.Creators) > 0 ||
		req.Type != nil ||
		len(req.DatasetKeys) > 0 ||
		req.TagFilter != nil ||
		len(req.OrderBys) > 0 {
		return nil, errorx.NewByCode(
			errno.CommonInvalidParamCode,
			errorx.WithExtraMsg("content filters and order_bys are not supported for shared evaluation sets"),
		)
	}
	var sourceFilter *int64
	if req.SharedOption != nil && req.SharedOption.SourceSpaceID != nil && req.SharedOption.GetSourceSpaceID() > 0 {
		sourceFilter = req.SharedOption.SourceSpaceID
	}
	accessCtxs, err := e.resourceAccessAuthorizer.ListSharedResources(ctx, &entity.ListSharedResourcesRequest{
		CallerSpaceID:     req.WorkspaceID,
		ResourceType:      entity.SharedResourceTypeEvalSet,
		SourceSpaceFilter: sourceFilter,
	})
	if err != nil {
		return nil, err
	}
	// 按来源空间分组批量加载，并记录每个资源的授权上下文用于回填共享元信息。
	pageToken := req.PageToken
	if pageToken == nil || strings.TrimSpace(*pageToken) == "" {
		pageToken, err = sharedPageTokenForPageNumber(req.PageNumber, req.PageSize)
		if err != nil {
			return nil, err
		}
	}
	pagedAccessCtxs, total, nextPageToken, _, err := paginateSharedAccessContexts(
		accessCtxs,
		req.EvaluationSetIds,
		req.PageSize,
		pageToken,
	)
	if err != nil {
		return nil, err
	}
	sets, err := batchGetSharedEvaluationSets(ctx, e.evaluationSetService, req.WorkspaceID, pagedAccessCtxs)
	if err != nil {
		return nil, err
	}

	dtos := evaluation_set.EvaluationSetDO2DTOs(sets)
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier(dtos))
	return &eval_set.ListEvaluationSetsResponse{
		EvaluationSets: dtos,
		Total:          &total,
		NextPageToken:  nextPageToken,
	}, nil
}

func sharedResourceKey(spaceID, resourceID int64) string {
	return strconv.FormatInt(spaceID, 10) + ":" + strconv.FormatInt(resourceID, 10)
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
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil, nil)
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
		SpaceID:           req.WorkspaceID,
		EvaluationSetID:   req.EvaluationSetID,
		Items:             evaluation_set.ItemDTO2DOs(req.Items),
		SkipInvalidItems:  req.SkipInvalidItems,
		AllowPartialAdd:   req.AllowPartialAdd,
		FieldWriteOptions: evaluation_set.FieldWriteOptionDTO2DOs(req.FieldWriteOptions),
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
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil, nil)
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
	err = e.evaluationSetItemService.UpdateEvaluationSetItem(ctx, req.WorkspaceID, req.EvaluationSetID, req.ItemID, evaluation_set.TurnDTO2DOs(req.GetEvaluationSetID(), req.GetItemID(), req.Turns), evaluation_set.FieldWriteOptionDTO2DOs(req.FieldWriteOptions), evaluation_set.ResourceTagRefDTO2DOs(req.Tags))
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
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil, nil)
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
	isShared := req.SharedOption != nil && req.SharedOption.GetIsShared()
	var sharedOption *entity.SharedResourceOption
	if isShared {
		sharedOption = sharedOptionDTO2DO(req.SharedOption)
		if sharedOption == nil {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("source_space_id is required when shared_option.is_shared is true"))
		}
		// 共享评测集只开放已发布版本快照，不允许读取 draft/current items。
		if req.VersionID == nil || req.GetVersionID() <= 0 {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("version_id is required for shared evaluation set items"))
		}
	}

	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, gptr.Of(true), sharedOption)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("errno set not found"))
	}

	querySpaceID := req.WorkspaceID
	if isShared {
		// item 内容路径要求 readable（execute 黑盒不可读内容）
		accessCtx, err := e.resourceAccessAuthorizer.AuthorizeRead(ctx, buildEvalSetAuthorizeRequest(req.WorkspaceID, set, sharedOption, req.VersionID, nil, true))
		if err != nil {
			return nil, err
		}
		// AuthorizeRead 只能对 specified + version_id 做加载前早拒；latest 必须在版本加载后
		// 与评测集的 LatestVersion 比较。这里同时校验版本确实属于当前评测集，防止跨集读取。
		version, versionSet, err := e.evaluationSetVersionService.GetEvaluationSetVersion(
			ctx, req.WorkspaceID, req.GetVersionID(), gptr.Of(true), sharedOption,
		)
		if err != nil {
			return nil, err
		}
		if version == nil || versionSet == nil || versionSet.ID != set.ID ||
			!service.IsSharedVersionAllowed(version.ID, version.Version, versionSet.LatestVersion, accessCtx.VersionPolicy, accessCtx.SpecifiedIDs) {
			return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluation set version not shared"))
		}
		querySpaceID = accessCtx.QuerySpaceID()
	} else {
		// 非共享场景保持 main 分支原有鉴权逻辑。
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
	}

	tagFilter, err := evaluation_set.TagFilterDTO2DO(req.TagFilter)
	if err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(err.Error()))
	}
	// domain调用：共享时用来源空间查询 item
	items, total, filterTotal, nextCursor, err := e.evaluationSetItemService.ListEvaluationSetItems(ctx, &entity.ListEvaluationSetItemsParam{
		SpaceID:         querySpaceID,
		EvaluationSetID: req.EvaluationSetID,
		VersionID:       req.VersionID,
		PageNumber:      req.PageNumber,
		PageSize:        req.PageSize,
		OrderBys:        common.ConvertOrderByDTO2DOs(req.OrderBys),
		Filter:          req.Filter,
		TagFilter:       tagFilter,
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.ListEvaluationSetItemsResponse{
		Items:         evaluation_set.ItemDO2DTOs(items),
		Total:         total,
		FilterTotal:   filterTotal,
		NextPageToken: nextCursor,
	}, nil
}

func (e *EvaluationSetApplicationImpl) BatchGetEvaluationSetItems(ctx context.Context, req *eval_set.BatchGetEvaluationSetItemsRequest) (resp *eval_set.BatchGetEvaluationSetItemsResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, gptr.Of(true), nil)
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
	tagFilter, err := evaluation_set.TagFilterDTO2DO(req.TagFilter)
	if err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(err.Error()))
	}
	items, err := e.evaluationSetItemService.BatchGetEvaluationSetItems(ctx, &entity.BatchGetEvaluationSetItemsParam{
		SpaceID:            req.WorkspaceID,
		EvaluationSetID:    req.EvaluationSetID,
		VersionID:          req.VersionID,
		ItemIDs:            req.ItemIds,
		ItemVersionQueries: evaluation_set.ItemVersionRefDTO2DOs(req.ItemVersionQueries),
		Filter:             req.Filter,
		TagFilter:          tagFilter,
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
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil, nil)
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
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil, nil)
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
	sharedOption := sharedOptionDTO2DO(req.SharedOption)
	if req.SharedOption == nil || !req.SharedOption.GetIsShared() {
		// 非共享场景严格保持 main：先加载评测集、鉴权，再加载版本。
		set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, gptr.Indirect(req.EvaluationSetID), req.DeletedAt, nil)
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
		if err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
			ObjectID:        strconv.FormatInt(set.ID, 10),
			SpaceID:         req.WorkspaceID,
			ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
			OwnerID:         ownerID,
			ResourceSpaceID: set.SpaceID,
		}); err != nil {
			return nil, err
		}
		version, versionSet, err := e.evaluationSetVersionService.GetEvaluationSetVersion(ctx, req.WorkspaceID, req.VersionID, req.DeletedAt, nil)
		if err != nil {
			return nil, err
		}
		versionDTO := evaluation_set.VersionDO2DTO(version)
		e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier([]*domain_eval_set.EvaluationSetVersion{versionDTO}))
		return &eval_set.GetEvaluationSetVersionResponse{
			Version: versionDTO, EvaluationSet: evaluation_set.EvaluationSetDO2DTO(versionSet),
		}, nil
	}
	if sharedOption == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("source_space_id is required when shared_option.is_shared is true"))
	}
	// 共享场景允许仅凭 version_id 从来源空间加载版本及所属评测集。
	version, set, err := e.evaluationSetVersionService.GetEvaluationSetVersion(ctx, req.WorkspaceID, req.VersionID, req.DeletedAt, sharedOption)
	if err != nil {
		return nil, err
	}
	if set == nil || version == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("errno set version not found"))
	}
	// 若显式传入了 evaluation_set_id，则校验与版本所属评测集一致,防止跨集越权
	if req.EvaluationSetID != nil && gptr.Indirect(req.EvaluationSetID) != 0 && gptr.Indirect(req.EvaluationSetID) != set.ID {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluation_set_id mismatch with version"))
	}
	accessCtx, err := e.resourceAccessAuthorizer.AuthorizeRead(ctx, buildEvalSetAuthorizeRequest(req.WorkspaceID, set, sharedOption, gptr.Of(req.VersionID), gptr.Of(version.Version), false))
	if err != nil {
		return nil, err
	}
	if accessCtx.IsShared() && !service.IsSharedVersionAllowed(version.ID, version.Version, set.LatestVersion, accessCtx.VersionPolicy, accessCtx.SpecifiedIDs) {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluation set version not shared"))
	}
	version.SharedInfo = accessCtx.SharedInfo()
	set.SharedInfo = accessCtx.SharedInfo()
	// 返回结果构建、错误处理
	dto := evaluation_set.EvaluationSetDO2DTO(set)
	versionDTO := evaluation_set.VersionDO2DTO(version)
	redactEvaluationSetVersionSchemas(accessCtx, []*domain_eval_set.EvaluationSetVersion{versionDTO})
	if dto != nil {
		redactEvaluationSetVersionSchemas(accessCtx, []*domain_eval_set.EvaluationSetVersion{dto.EvaluationSetVersion})
	}
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
	sharedOption := sharedOptionDTO2DO(req.SharedOption)
	if req.SharedOption == nil || !req.SharedOption.GetIsShared() {
		// 非共享场景严格保持 main 的加载、鉴权和分页调用。
		set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil, nil)
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
		if err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
			ObjectID:        strconv.FormatInt(set.ID, 10),
			SpaceID:         req.WorkspaceID,
			ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
			OwnerID:         ownerID,
			ResourceSpaceID: set.SpaceID,
		}); err != nil {
			return nil, err
		}
		versions, total, nextCursor, err := e.evaluationSetVersionService.ListEvaluationSetVersions(ctx, &entity.ListEvaluationSetVersionsParam{
			SpaceID: req.WorkspaceID, EvaluationSetID: req.EvaluationSetID, PageSize: req.PageSize,
			PageNumber: req.PageNumber, PageToken: req.PageToken, VersionLike: req.VersionLike,
		})
		if err != nil {
			return nil, err
		}
		versionDTOs := evaluation_set.VersionDO2DTOs(versions)
		e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier(versionDTOs))
		return &eval_set.ListEvaluationSetVersionsResponse{Versions: versionDTOs, Total: total, NextPageToken: nextCursor}, nil
	}
	if sharedOption == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("source_space_id is required when shared_option.is_shared is true"))
	}
	// 共享场景按来源空间加载并执行共享版本策略。
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil, sharedOption)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("errno set not found"))
	}
	accessCtx, err := e.resourceAccessAuthorizer.AuthorizeRead(ctx, buildEvalSetAuthorizeRequest(req.WorkspaceID, set, sharedOption, nil, nil, false))
	if err != nil {
		return nil, err
	}
	set.SharedInfo = accessCtx.SharedInfo()

	var (
		versions   []*entity.EvaluationSetVersion
		total      *int64
		nextCursor *string
	)
	// 非共享和共享 all 的可见集合与底层集合一致，严格保留原分页调用。
	if !accessCtx.IsShared() || accessCtx.VersionPolicy == "" || accessCtx.VersionPolicy == entity.SharedVersionPolicyAll {
		versions, total, nextCursor, err = e.evaluationSetVersionService.ListEvaluationSetVersions(ctx, &entity.ListEvaluationSetVersionsParam{
			SpaceID:         req.WorkspaceID,
			EvaluationSetID: req.EvaluationSetID,
			PageSize:        req.PageSize,
			PageNumber:      req.PageNumber,
			PageToken:       req.PageToken,
			VersionLike:     req.VersionLike,
			SharedOption:    sharedOption,
		})
		if err != nil {
			return nil, err
		}
		if accessCtx.IsShared() {
			for _, version := range versions {
				if version != nil {
					version.SharedInfo = accessCtx.SharedInfo()
				}
			}
		}
	} else {
		switch accessCtx.VersionPolicy {
		case entity.SharedVersionPolicyLatest, entity.SharedVersionPolicySpecified:
		default:
			return nil, errorx.NewByCode(errno.CommonNoPermissionCode, errorx.WithExtraMsg("unsupported shared version policy"))
		}

		// latest/specified 必须先在完整匹配集合上做权限过滤，再对授权集合分页。
		// 扫描时使用底层最大页长，保持底层排序和 VersionLike 语义。
		filteredVersions := make([]*entity.EvaluationSetVersion, 0)
		scanPageSize := int32(maxSharedPageSize)
		var scanPageToken *string
		for {
			page, _, nextScanPageToken, listErr := e.evaluationSetVersionService.ListEvaluationSetVersions(ctx, &entity.ListEvaluationSetVersionsParam{
				SpaceID:         req.WorkspaceID,
				EvaluationSetID: req.EvaluationSetID,
				PageSize:        &scanPageSize,
				PageToken:       scanPageToken,
				VersionLike:     req.VersionLike,
				SharedOption:    sharedOption,
			})
			if listErr != nil {
				return nil, listErr
			}
			for _, version := range page {
				if version != nil && service.IsSharedVersionAllowed(version.ID, version.Version, set.LatestVersion, accessCtx.VersionPolicy, accessCtx.SpecifiedIDs) {
					version.SharedInfo = accessCtx.SharedInfo()
					filteredVersions = append(filteredVersions, version)
				}
			}
			if nextScanPageToken == nil || strings.TrimSpace(*nextScanPageToken) == "" {
				break
			}
			if scanPageToken != nil && *nextScanPageToken == *scanPageToken {
				return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("list evaluation set versions returned repeated page token"))
			}
			scanPageToken = nextScanPageToken
		}

		pageToken := req.PageToken
		if pageToken == nil || strings.TrimSpace(*pageToken) == "" {
			pageToken, err = sharedPageTokenForPageNumber(req.PageNumber, req.PageSize)
			if err != nil {
				return nil, err
			}
		}
		versions, nextCursor, _, err = paginateShared(filteredVersions, req.PageSize, pageToken)
		if err != nil {
			return nil, err
		}
		total = gptr.Of(int64(len(filteredVersions)))
	}

	// 返回结果构建、错误处理
	versionDTOs := evaluation_set.VersionDO2DTOs(versions)
	redactEvaluationSetVersionSchemas(accessCtx, versionDTOs)
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
	sets, err := e.evaluationSetVersionService.BatchGetEvaluationSetVersions(ctx, &req.WorkspaceID, req.VersionIds, req.DeletedAt, nil)
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

func (e *EvaluationSetApplicationImpl) GetEvaluationSetItemField(ctx context.Context, req *eval_set.GetEvaluationSetItemFieldRequest) (r *eval_set.GetEvaluationSetItemFieldResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, gptr.Of(true), nil)
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
	fieldData, err := e.evaluationSetItemService.GetEvaluationSetItemField(ctx, &entity.GetEvaluationSetItemFieldParam{
		SpaceID:         req.WorkspaceID,
		EvaluationSetID: req.EvaluationSetID,
		ItemPK:          req.GetItemPk(),
		FieldName:       req.GetFieldName(),
		FieldKey:        gptr.Of(req.GetFieldKey()),
		TurnID:          req.TurnID,
	})
	if err != nil {
		return nil, err
	}
	// 返回结果构建、错误处理
	return &eval_set.GetEvaluationSetItemFieldResponse{
		FieldData: evaluation_set.FieldDataDO2DTO(fieldData),
	}, nil
}

func (e *EvaluationSetApplicationImpl) GetEvaluationSetItemDef(ctx context.Context, req *eval_set.GetEvaluationSetItemDefRequest) (resp *eval_set.GetEvaluationSetItemDefResponse, err error) {
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, gptr.Of(true), nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	itemDef, err := e.evaluationSetItemService.GetEvaluationSetItemDef(ctx, req.WorkspaceID, req.EvaluationSetID, req.ItemID)
	if err != nil {
		return nil, err
	}
	return &eval_set.GetEvaluationSetItemDefResponse{
		ItemDef: evaluation_set.ItemDefDO2DTO(itemDef),
	}, nil
}

func (e *EvaluationSetApplicationImpl) ListEvaluationSetItemDefs(ctx context.Context, req *eval_set.ListEvaluationSetItemDefsRequest) (resp *eval_set.ListEvaluationSetItemDefsResponse, err error) {
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, gptr.Of(true), nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	itemDefs, total, nextPageToken, err := e.evaluationSetItemService.ListEvaluationSetItemDefs(ctx, &entity.ListEvaluationSetItemDefsParam{
		SpaceID:         req.WorkspaceID,
		EvaluationSetID: req.EvaluationSetID,
		PageNumber:      req.PageNumber,
		PageSize:        req.PageSize,
		PageToken:       req.PageToken,
		OrderBys:        common.ConvertOrderByDTO2DOs(req.OrderBys),
	})
	if err != nil {
		return nil, err
	}
	return &eval_set.ListEvaluationSetItemDefsResponse{
		ItemDefs:      evaluation_set.ItemDefDO2DTOs(itemDefs),
		Total:         total,
		NextPageToken: nextPageToken,
	}, nil
}

func (e *EvaluationSetApplicationImpl) ListEvaluationSetItemVersions(ctx context.Context, req *eval_set.ListEvaluationSetItemVersionsRequest) (resp *eval_set.ListEvaluationSetItemVersionsResponse, err error) {
	// TODO: remove debug logging after versioned_item feature is stable
	logs.CtxInfo(ctx, "ListEvaluationSetItemVersions req: %v", json.Jsonify(req))
	defer func() {
		logs.CtxInfo(ctx, "ListEvaluationSetItemVersions resp: %v, err: %v", json.Jsonify(resp), err)
	}()
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, gptr.Of(true), nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	versions, total, nextPageToken, err := e.evaluationSetItemService.ListEvaluationSetItemVersions(ctx, &entity.ListEvaluationSetItemVersionsParam{
		SpaceID:         req.WorkspaceID,
		EvaluationSetID: req.EvaluationSetID,
		ItemID:          req.ItemID,
		PageNumber:      req.PageNumber,
		PageSize:        req.PageSize,
		PageToken:       req.PageToken,
		OrderBys:        common.ConvertOrderByDTO2DOs(req.OrderBys),
	})
	if err != nil {
		return nil, err
	}
	return &eval_set.ListEvaluationSetItemVersionsResponse{
		Versions:      evaluation_set.ItemVersionDO2DTOs(versions),
		Total:         total,
		NextPageToken: nextPageToken,
	}, nil
}

func (e *EvaluationSetApplicationImpl) GetEvaluationSetItemVersion(ctx context.Context, req *eval_set.GetEvaluationSetItemVersionRequest) (resp *eval_set.GetEvaluationSetItemVersionResponse, err error) {
	// TODO: remove debug logging after versioned_item feature is stable
	logs.CtxInfo(ctx, "GetEvaluationSetItemVersion req: %v", json.Jsonify(req))
	defer func() {
		logs.CtxInfo(ctx, "GetEvaluationSetItemVersion resp: %v, err: %v", json.Jsonify(resp), err)
	}()
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, gptr.Of(true), nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	version, err := e.evaluationSetItemService.GetEvaluationSetItemVersion(ctx, req.WorkspaceID, req.EvaluationSetID, req.ItemID, req.ItemVersionID, req.ItemVersion)
	if err != nil {
		return nil, err
	}
	return &eval_set.GetEvaluationSetItemVersionResponse{
		Version: evaluation_set.ItemVersionDO2DTO(version),
	}, nil
}

func (e *EvaluationSetApplicationImpl) UpdateEvaluationSetItemVersion(ctx context.Context, req *eval_set.UpdateEvaluationSetItemVersionRequest) (resp *eval_set.UpdateEvaluationSetItemVersionResponse, err error) {
	// TODO: remove debug logging after versioned_item feature is stable
	logs.CtxInfo(ctx, "UpdateEvaluationSetItemVersion req: %v", json.Jsonify(req))
	defer func() {
		logs.CtxInfo(ctx, "UpdateEvaluationSetItemVersion resp: %v, err: %v", json.Jsonify(resp), err)
	}()
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil, nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	err = e.evaluationSetItemService.UpdateEvaluationSetItemVersion(ctx, req.WorkspaceID, req.EvaluationSetID, req.ItemID, req.ItemVersionID, req.Status, req.Description, req.ItemVersion)
	if err != nil {
		return nil, err
	}
	return &eval_set.UpdateEvaluationSetItemVersionResponse{}, nil
}

func (e *EvaluationSetApplicationImpl) BatchAddExistEvaluationSetItems(ctx context.Context, req *eval_set.BatchAddExistEvaluationSetItemsRequest) (resp *eval_set.BatchAddExistEvaluationSetItemsResponse, err error) {
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if len(req.Items) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("items is empty"))
	}
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil, nil)
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
		SpaceID:         req.WorkspaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         ownerID,
		ResourceSpaceID: set.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	result, err := e.evaluationSetItemService.BatchAddExistEvaluationSetItems(ctx, &entity.BatchAddExistEvaluationSetItemsParam{
		SpaceID:         req.WorkspaceID,
		EvaluationSetID: req.EvaluationSetID,
		Items:           evaluation_set.ItemVersionRefDTO2DOs(req.Items),
		AllowPartialAdd: req.AllowPartialAdd,
	})
	if err != nil {
		return nil, err
	}
	resp = &eval_set.BatchAddExistEvaluationSetItemsResponse{}
	if result != nil {
		resp.SuccessCount = result.SuccessCount
		resp.FailedCount = result.FailedCount
		resp.FailedItems = evaluation_set.ItemVersionRefDO2DTOs(result.FailedItems)
	}
	return resp, nil
}
