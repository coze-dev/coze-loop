// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"sync"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
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
	}

	return resp, nil
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

// BatchUpsertEvaluationSetItemColumns 单列/部分列 upsert（D5 决策：evaluation 自实现）。
// 行为：对每条 patch
//   - 若提供 item_id 命中老行：BatchGet 整行 → 合并 patch 列（按 Turn ID + FieldData.Key 覆盖）→ UpdateEvaluationSetItem 整行（含乐观锁由底层负责）；patched_count 累加该 patch 涉及的列数。
//   - 若未提供 item_id：转入 BatchCreate 路径新增；created_count 累加。
//
// 失败语义：单条失败收集到 errors，整体不中断；SkipInvalidItems / AllowPartialAdd 与 BatchCreate 一致。
func (e *EvaluationSetApplicationImpl) BatchUpsertEvaluationSetItemColumns(ctx context.Context, req *eval_set.BatchUpsertEvaluationSetItemColumnsRequest) (resp *eval_set.BatchUpsertEvaluationSetItemColumnsResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if len(req.Patches) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("patches is empty"))
	}
	// 鉴权
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil)
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

	// 预收集需要 BatchGet 的 item_id
	existingItemIDs := make([]int64, 0, len(req.Patches))
	for _, patch := range req.Patches {
		if patch != nil && patch.ItemID != nil && *patch.ItemID > 0 {
			existingItemIDs = append(existingItemIDs, *patch.ItemID)
		}
	}
	existingItemsByID := make(map[int64]*entity.EvaluationSetItem, len(existingItemIDs))
	if len(existingItemIDs) > 0 {
		items, getErr := e.evaluationSetItemService.BatchGetEvaluationSetItems(ctx, &entity.BatchGetEvaluationSetItemsParam{
			SpaceID:         req.WorkspaceID,
			EvaluationSetID: req.EvaluationSetID,
			ItemIDs:         existingItemIDs,
		})
		if getErr != nil {
			return nil, getErr
		}
		for _, it := range items {
			if it != nil {
				existingItemsByID[it.ItemID] = it
			}
		}
	}

	upsertedItems := make(map[int64]int64)
	errGroups := make([]*entity.ItemErrorGroup, 0)
	var patchedCount int32
	var createdCount int32

	// 收集需要 BatchCreate 的新增 patch
	type pendingCreate struct {
		index int64
		item  *entity.EvaluationSetItem
	}
	pendingCreates := make([]pendingCreate, 0)

	for idx, patch := range req.Patches {
		if patch == nil {
			continue
		}
		// 命中老行：Get→merge→Update
		if patch.ItemID != nil && *patch.ItemID > 0 {
			existing, ok := existingItemsByID[*patch.ItemID]
			if !ok || existing == nil {
				notFound := entity.ItemErrorType_InternalError
				summary := "evaluation set item not found"
				errCount := int32(1)
				errGroups = append(errGroups, &entity.ItemErrorGroup{
					Type:       &notFound,
					Summary:    &summary,
					ErrorCount: &errCount,
				})
				continue
			}

			mergedTurns := mergeTurnsForPatch(existing.Turns, patch.Turns, existing.EvaluationSetID, existing.ItemID)
			updErr := e.evaluationSetItemService.UpdateEvaluationSetItem(ctx, req.WorkspaceID, req.EvaluationSetID, existing.ItemID, mergedTurns)
			if updErr != nil {
				internal := entity.ItemErrorType_InternalError
				summary := updErr.Error()
				errCount := int32(1)
				errGroups = append(errGroups, &entity.ItemErrorGroup{
					Type:       &internal,
					Summary:    &summary,
					ErrorCount: &errCount,
				})
				continue
			}
			upsertedItems[int64(idx)] = existing.ItemID
			patchedCount += countPatchColumns(patch.Turns)
			continue
		}

		// 新增行：转入 BatchCreate 路径
		newItem := &entity.EvaluationSetItem{
			SpaceID:         req.WorkspaceID,
			EvaluationSetID: req.EvaluationSetID,
			ItemKey:         gptr.Indirect(patch.ItemKey),
			Turns:           evaluation_set.TurnDTO2DOs(req.EvaluationSetID, 0, patch.Turns),
		}
		pendingCreates = append(pendingCreates, pendingCreate{index: int64(idx), item: newItem})
	}

	// 批量新增（保持顺序）
	if len(pendingCreates) > 0 {
		toCreate := make([]*entity.EvaluationSetItem, 0, len(pendingCreates))
		for _, pc := range pendingCreates {
			toCreate = append(toCreate, pc.item)
		}
		idMap, createErrs, _, createErr := e.evaluationSetItemService.BatchCreateEvaluationSetItems(ctx, &entity.BatchCreateEvaluationSetItemsParam{
			SpaceID:          req.WorkspaceID,
			EvaluationSetID:  req.EvaluationSetID,
			Items:            toCreate,
			SkipInvalidItems: req.SkipInvalidItems,
			AllowPartialAdd:  req.AllowPartialAdd,
		})
		if createErr != nil {
			return nil, createErr
		}
		// idMap key 是 BatchCreate 输入下标，需要映射回 patches 的全局下标
		for localIdx, pc := range pendingCreates {
			if itemID, ok := idMap[int64(localIdx)]; ok {
				upsertedItems[pc.index] = itemID
				createdCount++
			}
		}
		errGroups = append(errGroups, createErrs...)
	}

	resp = &eval_set.BatchUpsertEvaluationSetItemColumnsResponse{
		UpsertedItems: upsertedItems,
		Errors:        evaluation_set.ItemErrorGroupDO2DTOs(errGroups),
		PatchedCount:  gptr.Of(patchedCount),
		CreatedCount:  gptr.Of(createdCount),
	}
	return resp, nil
}

// mergeTurnsForPatch 将 patch turns 合并到 existing turns 上：
//   - Turn 通过 ID 匹配；未命中视为新增 Turn 追加。
//   - 每个 Turn 内按 FieldData.Key 覆盖；未命中的 key 视为新增列。
func mergeTurnsForPatch(existing []*entity.Turn, patchDTOs []*domain_eval_set.Turn, evalSetID, itemID int64) []*entity.Turn {
	merged := make([]*entity.Turn, 0, len(existing))
	turnByID := make(map[int64]*entity.Turn, len(existing))
	for _, t := range existing {
		if t == nil {
			continue
		}
		clone := &entity.Turn{
			ID:            t.ID,
			ItemID:        t.ItemID,
			EvalSetID:     t.EvalSetID,
			FieldDataList: append([]*entity.FieldData{}, t.FieldDataList...),
		}
		turnByID[t.ID] = clone
		merged = append(merged, clone)
	}

	for _, pt := range patchDTOs {
		if pt == nil {
			continue
		}
		patchTurnID := gptr.Indirect(pt.ID)
		patchFields := evaluation_set.FieldDataDTO2DOs(pt.FieldDataList)
		if existingTurn, ok := turnByID[patchTurnID]; ok && patchTurnID > 0 {
			// 按 key 覆盖
			fieldIdxByKey := make(map[string]int, len(existingTurn.FieldDataList))
			for i, f := range existingTurn.FieldDataList {
				if f != nil {
					fieldIdxByKey[f.Key] = i
				}
			}
			for _, pf := range patchFields {
				if pf == nil {
					continue
				}
				if i, ok := fieldIdxByKey[pf.Key]; ok {
					existingTurn.FieldDataList[i] = pf
				} else {
					existingTurn.FieldDataList = append(existingTurn.FieldDataList, pf)
				}
			}
		} else {
			// 新增 Turn
			merged = append(merged, &entity.Turn{
				ID:            patchTurnID,
				ItemID:        itemID,
				EvalSetID:     evalSetID,
				FieldDataList: patchFields,
			})
		}
	}
	return merged
}

// countPatchColumns 统计 patch 涉及的列数（每个 Turn 内的 FieldData 单元累加）。
func countPatchColumns(patchTurns []*domain_eval_set.Turn) int32 {
	var n int32
	for _, t := range patchTurns {
		if t == nil {
			continue
		}
		n += int32(len(t.FieldDataList))
	}
	return n
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
	err = e.evaluationSetItemService.UpdateEvaluationSetItem(ctx, req.WorkspaceID, req.EvaluationSetID, req.ItemID, evaluation_set.TurnDTO2DOs(req.GetEvaluationSetID(), req.GetItemID(), req.Turns))
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

func (e *EvaluationSetApplicationImpl) GetEvaluationSetItemField(ctx context.Context, req *eval_set.GetEvaluationSetItemFieldRequest) (r *eval_set.GetEvaluationSetItemFieldResponse, err error) {
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
	fieldData, err := e.evaluationSetItemService.GetEvaluationSetItemField(ctx, &entity.GetEvaluationSetItemFieldParam{
		SpaceID:         req.WorkspaceID,
		EvaluationSetID: req.EvaluationSetID,
		ItemPK:          req.GetItemPk(),
		FieldName:       req.GetFieldName(),
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
