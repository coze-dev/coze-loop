// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"sync"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	eval_target_dto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

var _ evaluation.EvalTargetService = &EvalTargetApplicationImpl{}

type EvalTargetApplicationImpl struct {
	auth              rpc.IAuthProvider
	evalTargetService service.IEvalTargetService
	typedOperators    map[entity.EvalTargetType]service.ISourceEvalTargetOperateService
}

var (
	evalTargetHandlerOnce = sync.Once{}
	evalTargetHandler     evaluation.EvalTargetService
)

func NewEvalTargetHandlerImpl(auth rpc.IAuthProvider, evalTargetService service.IEvalTargetService,
	typedOperators map[entity.EvalTargetType]service.ISourceEvalTargetOperateService,
) evaluation.EvalTargetService {
	evalTargetHandlerOnce.Do(func() {
		evalTargetHandler = &EvalTargetApplicationImpl{
			auth:              auth,
			evalTargetService: evalTargetService,
			typedOperators:    typedOperators,
		}
	})
	return evalTargetHandler
}

func (e EvalTargetApplicationImpl) CreateEvalTarget(ctx context.Context, request *eval_target.CreateEvalTargetRequest) (r *eval_target.CreateEvalTargetResponse, err error) {
	// 校验参数是否为空
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if request.Param == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req param is nil"))
	}
	if request.Param.SourceTargetID == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("source target id is nil"))
	}
	if request.Param.SourceTargetVersion == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("source target version is nil"))
	}
	if request.Param.EvalTargetType == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("source target type is nil"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	id, versionID, err := e.evalTargetService.CreateEvalTarget(ctx, request.WorkspaceID, request.Param.GetSourceTargetID(), request.Param.GetSourceTargetVersion(),
		entity.EvalTargetType(request.Param.GetEvalTargetType()),
		entity.WithCozeBotPublishVersion(request.Param.BotPublishVersion),
		entity.WithCozeBotInfoType(entity.CozeBotInfoType(request.Param.GetBotInfoType())))
	if err != nil {
		return nil, err
	}
	return &eval_target.CreateEvalTargetResponse{
		ID:        &id,
		VersionID: &versionID,
	}, nil
}

func (e EvalTargetApplicationImpl) BatchGetEvalTargetsBySource(ctx context.Context, request *eval_target.BatchGetEvalTargetsBySourceRequest) (r *eval_target.BatchGetEvalTargetsBySourceResponse, err error) {
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if len(request.SourceTargetIds) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("source target id is nil"))
	}
	if request.EvalTargetType == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("source target type is nil"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	evalTargets, err := e.evalTargetService.BatchGetEvalTargetBySource(ctx, &entity.BatchGetEvalTargetBySourceParam{
		SpaceID:        request.WorkspaceID,
		SourceTargetID: request.GetSourceTargetIds(),
		TargetType:     entity.EvalTargetType(request.GetEvalTargetType()),
	})
	if err != nil {
		return nil, err
	}
	if len(evalTargets) == 0 {
		return &eval_target.BatchGetEvalTargetsBySourceResponse{}, nil
	}
	// 包装source info信息
	if gptr.Indirect(request.NeedSourceInfo) {
		for _, op := range e.typedOperators {
			err = op.PackSourceInfo(ctx, request.WorkspaceID, evalTargets)
			if err != nil {
				return nil, err
			}
		}
	}
	res := make([]*eval_target_dto.EvalTarget, 0)
	for _, evalTarget := range evalTargets {
		res = append(res, target.EvalTargetDO2DTO(evalTarget))
	}
	return &eval_target.BatchGetEvalTargetsBySourceResponse{
		EvalTargets: res,
	}, nil
}

func (e EvalTargetApplicationImpl) GetEvalTargetVersion(ctx context.Context, request *eval_target.GetEvalTargetVersionRequest) (r *eval_target.GetEvalTargetVersionResponse, err error) {
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if request.EvalTargetVersionID == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target version id is nil"))
	}
	evalTarget, err := e.evalTargetService.GetEvalTargetVersion(ctx, request.WorkspaceID, request.GetEvalTargetVersionID(), false)
	if err != nil {
		return nil, err
	}
	if evalTarget == nil {
		return &eval_target.GetEvalTargetVersionResponse{}, nil
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(evalTarget.ID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationTarget)}},
	})
	if err != nil {
		return nil, err
	}
	return &eval_target.GetEvalTargetVersionResponse{
		EvalTarget: target.EvalTargetDO2DTO(evalTarget),
	}, nil
}

func (e EvalTargetApplicationImpl) BatchGetEvalTargetVersions(ctx context.Context, request *eval_target.BatchGetEvalTargetVersionsRequest) (r *eval_target.BatchGetEvalTargetVersionsResponse, err error) {
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if len(request.EvalTargetVersionIds) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target ids is nil"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	evalTargets, err := e.evalTargetService.BatchGetEvalTargetVersion(ctx, request.WorkspaceID, request.GetEvalTargetVersionIds(), gptr.Indirect(request.NeedSourceInfo))
	if err != nil {
		return nil, err
	}
	if len(evalTargets) == 0 {
		return &eval_target.BatchGetEvalTargetVersionsResponse{}, nil
	}
	res := make([]*eval_target_dto.EvalTarget, 0)
	for _, evalTarget := range evalTargets {
		res = append(res, target.EvalTargetDO2DTO(evalTarget))
	}
	return &eval_target.BatchGetEvalTargetVersionsResponse{
		EvalTargets: res,
	}, nil
}

func (e EvalTargetApplicationImpl) ListSourceEvalTargets(ctx context.Context, request *eval_target.ListSourceEvalTargetsRequest) (r *eval_target.ListSourceEvalTargetsResponse, err error) {
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if request.TargetType == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type is nil"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	var res []*entity.EvalTarget
	var nextCursor string
	var hasMore bool
	param := &entity.ListSourceParam{
		SpaceID:    &request.WorkspaceID,
		PageSize:   request.PageSize,
		Cursor:     request.PageToken,
		KeyWord:    request.Name,
		TargetType: entity.EvalTargetType(request.GetTargetType()),
	}
	if e.typedOperators[param.TargetType] == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	res, nextCursor, hasMore, err = e.typedOperators[param.TargetType].ListSource(ctx, param)
	if err != nil {
		return nil, err
	}

	dtos := make([]*eval_target_dto.EvalTarget, 0)
	for _, do := range res {
		dtos = append(dtos, target.EvalTargetDO2DTO(do))
	}
	return &eval_target.ListSourceEvalTargetsResponse{
		EvalTargets:   dtos,
		NextPageToken: &nextCursor,
		HasMore:       &hasMore,
	}, nil
}

func (e EvalTargetApplicationImpl) ListSourceEvalTargetVersions(ctx context.Context, request *eval_target.ListSourceEvalTargetVersionsRequest) (r *eval_target.ListSourceEvalTargetVersionsResponse, err error) {
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if request.TargetType == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type is nil"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	var res []*entity.EvalTargetVersion
	var nextCursor string
	var hasMore bool
	param := &entity.ListSourceVersionParam{
		SpaceID:        &request.WorkspaceID,
		PageSize:       request.PageSize,
		Cursor:         request.PageToken,
		SourceTargetID: request.SourceTargetID,
		TargetType:     entity.EvalTargetType(request.GetTargetType()),
	}
	if e.typedOperators[param.TargetType] == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	res, nextCursor, hasMore, err = e.typedOperators[param.TargetType].ListSourceVersion(ctx, param)
	if err != nil {
		return nil, err
	}
	dtos := make([]*eval_target_dto.EvalTargetVersion, 0)
	for _, do := range res {
		dtos = append(dtos, target.EvalTargetVersionDO2DTO(do))
	}
	return &eval_target.ListSourceEvalTargetVersionsResponse{
		Versions:      dtos,
		NextPageToken: &nextCursor,
		HasMore:       &hasMore,
	}, nil
}

func (e EvalTargetApplicationImpl) ExecuteEvalTarget(ctx context.Context, request *eval_target.ExecuteEvalTargetRequest) (r *eval_target.ExecuteEvalTargetResponse, err error) {
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if request.InputData == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("inputData is nil"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.EvalTargetID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationTarget)}},
	})
	if err != nil {
		return nil, err
	}
	targetRecord, err := e.evalTargetService.ExecuteTarget(ctx, request.WorkspaceID, request.EvalTargetID, request.EvalTargetVersionID, &entity.ExecuteTargetCtx{
		ExperimentRunID: request.ExperimentRunID,
		ItemID:          0,
		TurnID:          0,
	}, target.InputDTO2ToDO(request.InputData))
	if err != nil {
		return nil, err
	}
	resp := &eval_target.ExecuteEvalTargetResponse{
		EvalTargetRecord: target.EvalTargetRecordDO2DTO(targetRecord),
	}
	return resp, nil
}

func (e EvalTargetApplicationImpl) GetEvalTargetRecord(ctx context.Context, request *eval_target.GetEvalTargetRecordRequest) (r *eval_target.GetEvalTargetRecordResponse, err error) {
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	resp := &eval_target.GetEvalTargetRecordResponse{}
	targetRecord, err := e.evalTargetService.GetRecordByID(ctx, request.WorkspaceID, request.EvalTargetRecordID)
	if err != nil {
		return nil, err
	}
	if targetRecord == nil {
		return resp, nil
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(targetRecord.TargetID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationTarget)}},
	})
	if err != nil {
		return nil, err
	}
	resp.EvalTargetRecord = target.EvalTargetRecordDO2DTO(targetRecord)
	return resp, nil
}

func (e EvalTargetApplicationImpl) BatchGetEvalTargetRecords(ctx context.Context, request *eval_target.BatchGetEvalTargetRecordsRequest) (r *eval_target.BatchGetEvalTargetRecordsResponse, err error) {
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	resp := &eval_target.BatchGetEvalTargetRecordsResponse{}
	recordList, err := e.evalTargetService.BatchGetRecordByIDs(ctx, request.WorkspaceID, request.EvalTargetRecordIds)
	if err != nil {
		return nil, err
	}
	dtoList := make([]*eval_target_dto.EvalTargetRecord, 0)
	for _, record := range recordList {
		dtoList = append(dtoList, target.EvalTargetRecordDO2DTO(record))
	}
	resp.EvalTargetRecords = dtoList
	return resp, nil
}

func (e EvalTargetApplicationImpl) BatchGetSourceEvalTargets(ctx context.Context, request *eval_target.BatchGetSourceEvalTargetsRequest) (r *eval_target.BatchGetSourceEvalTargetsResponse, err error) {
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if request.TargetType == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type is nil"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	var res []*entity.EvalTarget
	if e.typedOperators[entity.EvalTargetType(request.GetTargetType())] == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	res, err = e.typedOperators[entity.EvalTargetType(request.GetTargetType())].BatchGetSource(ctx, request.WorkspaceID, request.SourceTargetIds)
	if err != nil {
		return nil, err
	}

	dtos := make([]*eval_target_dto.EvalTarget, 0)
	for _, do := range res {
		dtos = append(dtos, target.EvalTargetDO2DTO(do))
	}
	return &eval_target.BatchGetSourceEvalTargetsResponse{
		EvalTargets: dtos,
	}, nil
}
