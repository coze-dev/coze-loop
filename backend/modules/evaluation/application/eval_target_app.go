// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/bytedance/gg/gmap"
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	eval_target_dto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var _ evaluation.EvalTargetService = &EvalTargetApplicationImpl{}

type EvalTargetApplicationImpl struct {
	auth                     rpc.IAuthProvider
	evalTargetService        service.IEvalTargetService
	typedOperators           map[entity.EvalTargetType]service.ISourceEvalTargetOperateService
	evalAsyncRepo            repo.IEvalAsyncRepo
	resourceAccessAuthorizer service.ResourceAccessAuthorizer
}

func sharedTargetOptionDTO2DO(option *common.SharedResourceOption) *entity.SharedResourceOption {
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

// buildEvalTargetAuthorizeRequest 组装评测对象读授权入参。
// 评测对象读取即返回其配置（相当于内容），共享读要求 readable（execute 黑盒会被拒）。
func buildEvalTargetAuthorizeRequest(callerSpaceID, evalTargetID int64, sharedOption *entity.SharedResourceOption, versionID *int64) *entity.AuthorizeResourceRequest {
	return &entity.AuthorizeResourceRequest{
		CallerSpaceID:      callerSpaceID,
		ResourceType:       entity.SharedResourceTypeEvalTarget,
		ResourceID:         evalTargetID,
		VersionID:          versionID,
		SharedOption:       sharedOption,
		Action:             consts.Read,
		RequireContentRead: true,
	}
}

func evalTargetVersionID(evalTarget *entity.EvalTarget) *int64 {
	if evalTarget == nil || evalTarget.EvalTargetVersion == nil {
		return nil
	}
	return gptr.Of(evalTarget.EvalTargetVersion.ID)
}

var (
	evalTargetHandlerOnce = sync.Once{}
	evalTargetHandler     evaluation.EvalTargetService
)

func NewEvalTargetHandlerImpl(
	auth rpc.IAuthProvider,
	evalTargetService service.IEvalTargetService,
	typedOperators map[entity.EvalTargetType]service.ISourceEvalTargetOperateService,
	evalAsyncRepo repo.IEvalAsyncRepo,
	resourceAccessAuthorizer service.ResourceAccessAuthorizer,
) evaluation.EvalTargetService {
	evalTargetHandlerOnce.Do(func() {
		evalTargetHandler = &EvalTargetApplicationImpl{
			auth:                     auth,
			evalTargetService:        evalTargetService,
			typedOperators:           typedOperators,
			evalAsyncRepo:            evalAsyncRepo,
			resourceAccessAuthorizer: resourceAccessAuthorizer,
		}
	})
	return evalTargetHandler
}

// resolveOperatorType 仅记录型映射到 base 类型以复用 operator，其他类型原样返回
func resolveOperatorType(targetType entity.EvalTargetType) entity.EvalTargetType {
	if baseType, ok := targetType.RecordOnlyTypeToBaseType(); ok {
		return baseType
	}
	return targetType
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
	opts := make([]entity.Option, 0)
	opts = append(opts, entity.WithCozeBotPublishVersion(request.Param.BotPublishVersion),
		entity.WithCozeBotInfoType(entity.CozeBotInfoType(request.Param.GetBotInfoType())),
		entity.WithRegion(request.Param.Region),
		entity.WithEnv(request.Param.Env),
		entity.WithOperationInstruction(request.Param.OperationInstruction),
		entity.WithCluster(request.Param.Cluster))
	if request.GetParam().CustomEvalTarget != nil {
		opts = append(opts, entity.WithCustomEvalTarget(&entity.CustomEvalTarget{
			ID:        request.GetParam().GetCustomEvalTarget().ID,
			Name:      request.GetParam().GetCustomEvalTarget().Name,
			AvatarURL: request.GetParam().GetCustomEvalTarget().AvatarURL,
			Ext:       request.GetParam().GetCustomEvalTarget().Ext,
		}))
	}
	if request.GetParam().AgentConnection != nil {
		opts = append(opts, entity.WithAgentConnection(target.AgentConnectionDTO2DO(request.GetParam().AgentConnection)))
	}
	id, versionID, err := e.evalTargetService.CreateEvalTarget(ctx, request.WorkspaceID, request.Param.GetSourceTargetID(), request.Param.GetSourceTargetVersion(),
		entity.EvalTargetType(request.Param.GetEvalTargetType()), opts...)
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
	sharedOption := sharedTargetOptionDTO2DO(request.SharedOption)
	// 共享读:评测对象存在于来源空间,DAO 按传入 spaceID 硬过滤,
	// 故先用来源空间加载(此时仅为"按调用方声明"加载,尚未鉴权),
	// 授权由下方 authorizer 前置把关(命中白名单才返回,否则 fail-closed 拒绝)。
	querySpaceID := request.WorkspaceID
	if sharedOption.Enabled() {
		querySpaceID = gptr.Indirect(sharedOption.SourceSpaceID)
	}
	evalTarget, err := e.evalTargetService.GetEvalTargetVersion(ctx, querySpaceID, request.GetEvalTargetVersionID(), sharedOption == nil)
	if err != nil {
		return nil, err
	}
	if evalTarget == nil {
		return &eval_target.GetEvalTargetVersionResponse{}, nil
	}
	accessCtx, err := e.resourceAccessAuthorizer.AuthorizeRead(ctx, buildEvalTargetAuthorizeRequest(request.WorkspaceID, evalTarget.ID, sharedOption, evalTargetVersionID(evalTarget)))
	if err != nil {
		return nil, err
	}
	if evalTarget.EvalTargetVersion != nil && accessCtx.IsShared() && !service.IsSharedVersionAllowed(evalTarget.EvalTargetVersion.ID, "", "", accessCtx.VersionPolicy, accessCtx.SpecifiedIDs) {
		return &eval_target.GetEvalTargetVersionResponse{}, nil
	}
	evalTarget.SharedInfo = accessCtx.SharedInfo()
	if evalTarget.EvalTargetVersion != nil {
		evalTarget.EvalTargetVersion.SharedInfo = accessCtx.SharedInfo()
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
	sharedOption := sharedTargetOptionDTO2DO(request.SharedOption)
	needSourceInfo := gptr.Indirect(request.NeedSourceInfo) || sharedOption != nil
	// 共享读:按来源空间加载,鉴权前置(见 GetEvalTargetVersion 注释)
	querySpaceID := request.WorkspaceID
	if sharedOption.Enabled() {
		querySpaceID = gptr.Indirect(sharedOption.SourceSpaceID)
	}
	evalTargets, err := e.evalTargetService.BatchGetEvalTargetVersion(ctx, querySpaceID, request.GetEvalTargetVersionIds(), needSourceInfo)
	if err != nil {
		return nil, err
	}
	if len(evalTargets) == 0 {
		return &eval_target.BatchGetEvalTargetVersionsResponse{}, nil
	}
	res := make([]*eval_target_dto.EvalTarget, 0)
	for _, evalTarget := range evalTargets {
		accessCtx, authErr := e.resourceAccessAuthorizer.AuthorizeRead(ctx, buildEvalTargetAuthorizeRequest(request.WorkspaceID, evalTarget.ID, sharedOption, evalTargetVersionID(evalTarget)))
		if authErr != nil {
			return nil, authErr
		}
		if evalTarget.EvalTargetVersion != nil && accessCtx.IsShared() && !service.IsSharedVersionAllowed(evalTarget.EvalTargetVersion.ID, "", "", accessCtx.VersionPolicy, accessCtx.SpecifiedIDs) {
			continue
		}
		evalTarget.SharedInfo = accessCtx.SharedInfo()
		if evalTarget.EvalTargetVersion != nil {
			evalTarget.EvalTargetVersion.SharedInfo = accessCtx.SharedInfo()
		}
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
	// is_shared=true：按共享配置枚举「共享给调用方空间」的评测对象来源空间，
	// 逐来源空间重定向 ListSource（用 source space 而非 request.WorkspaceID）。
	if request.SharedOption != nil && request.SharedOption.GetIsShared() {
		return e.listSharedSourceEvalTargets(ctx, request)
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
	opType := resolveOperatorType(param.TargetType)
	if e.typedOperators[opType] == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	res, nextCursor, hasMore, err = e.typedOperators[opType].ListSource(ctx, param)
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

// listSharedSourceEvalTargets 枚举「共享给调用方空间」的评测对象来源空间，逐来源空间列举其可选来源。
// source_space_id 为空 → 跨全部来源空间；有值 → 限定该来源空间。空配置 → 空列表（fail-closed）。
func (e EvalTargetApplicationImpl) listSharedSourceEvalTargets(ctx context.Context, request *eval_target.ListSourceEvalTargetsRequest) (*eval_target.ListSourceEvalTargetsResponse, error) {
	targetType := entity.EvalTargetType(request.GetTargetType())
	opType := resolveOperatorType(targetType)
	if e.typedOperators[opType] == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	sourceSpaces, err := e.sharedSourceSpaces(ctx, request.WorkspaceID, request.SharedOption)
	if err != nil {
		return nil, err
	}
	dtos := make([]*eval_target_dto.EvalTarget, 0)
	for _, src := range sourceSpaces {
		sharedInfo := entity.BuildSharedResourceInfo(request.WorkspaceID, src, entity.SharedAccessLevelReadable, entity.SharedVersionPolicyAll)
		param := &entity.ListSourceParam{
			SpaceID:      gptr.Of(src),
			PageSize:     request.PageSize,
			Cursor:       request.PageToken,
			KeyWord:      request.Name,
			TargetType:   targetType,
			SharedOption: &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(src)},
		}
		res, _, _, listErr := e.typedOperators[opType].ListSource(ctx, param)
		if listErr != nil {
			return nil, listErr
		}
		for _, do := range res {
			if do == nil {
				continue
			}
			do.SharedInfo = sharedInfo
			if do.EvalTargetVersion != nil {
				do.EvalTargetVersion.SharedInfo = sharedInfo
			}
			dtos = append(dtos, target.EvalTargetDO2DTO(do))
		}
	}
	return &eval_target.ListSourceEvalTargetsResponse{
		EvalTargets: dtos,
		HasMore:     gptr.Of(false),
	}, nil
}

// sharedSourceSpaces 返回「共享评测对象给调用方空间」的去重来源空间列表。
func (e EvalTargetApplicationImpl) sharedSourceSpaces(ctx context.Context, callerSpaceID int64, sharedOption *common.SharedResourceOption) ([]int64, error) {
	var sourceFilter *int64
	if sharedOption != nil && sharedOption.SourceSpaceID != nil && sharedOption.GetSourceSpaceID() > 0 {
		sourceFilter = sharedOption.SourceSpaceID
	}
	accessCtxs, err := e.resourceAccessAuthorizer.ListSharedResources(ctx, &entity.ListSharedResourcesRequest{
		CallerSpaceID:     callerSpaceID,
		ResourceType:      entity.SharedResourceTypeEvalTarget,
		SourceSpaceFilter: sourceFilter,
	})
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]struct{})
	spaces := make([]int64, 0)
	for _, accessCtx := range accessCtxs {
		if accessCtx == nil || accessCtx.ResourceSpaceID <= 0 {
			continue
		}
		if _, ok := seen[accessCtx.ResourceSpaceID]; ok {
			continue
		}
		seen[accessCtx.ResourceSpaceID] = struct{}{}
		spaces = append(spaces, accessCtx.ResourceSpaceID)
	}
	return spaces, nil
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

	sharedOption := sharedTargetOptionDTO2DO(request.SharedOption)
	// 共享读：先校验该来源目标共享给调用方，再重定向到来源空间列举版本并按版本策略过滤。
	var accessCtx *entity.ResourceAccessContext
	querySpaceID := request.WorkspaceID
	if sharedOption.Enabled() {
		sourceTargetID, parseErr := strconv.ParseInt(request.SourceTargetID, 10, 64)
		if parseErr != nil {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid source target id"))
		}
		accessCtx, err = e.resourceAccessAuthorizer.AuthorizeRead(ctx, buildEvalTargetAuthorizeRequest(request.WorkspaceID, sourceTargetID, sharedOption, nil))
		if err != nil {
			return nil, err
		}
		querySpaceID = accessCtx.QuerySpaceID()
	}

	var res []*entity.EvalTargetVersion
	var nextCursor string
	var hasMore bool
	param := &entity.ListSourceVersionParam{
		SpaceID:        gptr.Of(querySpaceID),
		PageSize:       request.PageSize,
		Cursor:         request.PageToken,
		SourceTargetID: request.SourceTargetID,
		TargetType:     entity.EvalTargetType(request.GetTargetType()),
		SharedOption:   sharedOption,
	}
	opType := resolveOperatorType(param.TargetType)
	if e.typedOperators[opType] == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	res, nextCursor, hasMore, err = e.typedOperators[opType].ListSourceVersion(ctx, param)
	if err != nil {
		return nil, err
	}
	dtos := make([]*eval_target_dto.EvalTargetVersion, 0)
	for _, do := range res {
		if do == nil {
			continue
		}
		if accessCtx.IsShared() {
			if !service.IsSharedVersionAllowed(do.ID, do.SourceTargetVersion, "", accessCtx.VersionPolicy, accessCtx.SpecifiedIDs) {
				continue
			}
			do.SharedInfo = accessCtx.SharedInfo()
		}
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

func (e EvalTargetApplicationImpl) AsyncExecuteEvalTarget(ctx context.Context, request *eval_target.AsyncExecuteEvalTargetRequest) (r *eval_target.AsyncExecuteEvalTargetResponse, err error) {
	if err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.EvalTargetID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationTarget)}},
	}); err != nil {
		return nil, err
	}

	record, _, err := e.evalTargetService.AsyncExecuteTarget(ctx, request.WorkspaceID, request.EvalTargetID, request.EvalTargetVersionID, &entity.ExecuteTargetCtx{
		ExperimentRunID: request.ExperimentRunID,
	}, target.InputDTO2ToDO(request.InputData))
	if err != nil {
		return nil, err
	}

	return &eval_target.AsyncExecuteEvalTargetResponse{
		InvokeID: gptr.Of(record.ID),
		BaseResp: base.NewBaseResp(),
	}, nil
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

func (e EvalTargetApplicationImpl) GetEvalTargetOutputFieldContent(ctx context.Context, request *eval_target.GetEvalTargetOutputFieldContentRequest) (r *eval_target.GetEvalTargetOutputFieldContentResponse, err error) {
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if request.WorkspaceID == 0 || request.EvalTargetRecordID == 0 || len(request.FieldKeys) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("workspace_id, eval_target_record_id and field_keys are required"))
	}
	resp := &eval_target.GetEvalTargetOutputFieldContentResponse{}
	// 通过 eval_target_record_id 查询 target_record
	record, err := e.evalTargetService.GetRecordByID(ctx, request.WorkspaceID, request.EvalTargetRecordID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("eval target record not found"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(record.TargetID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationTarget)}},
	})
	if err != nil {
		return nil, err
	}
	// 加载大对象完整内容
	if err := e.evalTargetService.LoadRecordOutputFields(ctx, record, request.FieldKeys); err != nil {
		return nil, err
	}
	// 提取请求的字段内容
	fieldContents := make(map[string]*entity.Content)
	if record.EvalTargetOutputData != nil && record.EvalTargetOutputData.OutputFields != nil {
		keySet := make(map[string]struct{}, len(request.FieldKeys))
		for _, k := range request.FieldKeys {
			keySet[k] = struct{}{}
		}
		for k, c := range record.EvalTargetOutputData.OutputFields {
			if _, ok := keySet[k]; ok {
				fieldContents[k] = c
			}
		}
	}
	if len(fieldContents) == 0 {
		resp.FieldContents = map[string]*common.Content{}
		return resp, nil
	}
	resp.FieldContents = target.ContentDOToDTOs(fieldContents)
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
	targetType := entity.EvalTargetType(request.GetTargetType())
	opType := resolveOperatorType(targetType)
	if e.typedOperators[opType] == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	res, err := e.typedOperators[opType].BatchGetSource(ctx, request.WorkspaceID, request.SourceTargetIds)
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

func (e EvalTargetApplicationImpl) GetSourceEvalTargetVersion(ctx context.Context, request *eval_target.GetSourceEvalTargetVersionRequest) (r *eval_target.GetSourceEvalTargetVersionResponse, err error) {
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if request.TargetType == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type is nil"))
	}
	if strings.TrimSpace(request.GetSourceTargetID()) == "" {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("source target id is nil"))
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
	targetType := entity.EvalTargetType(request.GetTargetType())
	typedOperator := e.typedOperators[targetType]
	if typedOperator == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	evalTarget, err := typedOperator.BuildBySource(ctx, request.WorkspaceID, request.GetSourceTargetID(), request.GetSourceTargetVersion())
	if err != nil {
		return nil, err
	}
	if evalTarget == nil {
		return &eval_target.GetSourceEvalTargetVersionResponse{}, nil
	}
	if err := typedOperator.PackSourceVersionInfo(ctx, request.WorkspaceID, []*entity.EvalTarget{evalTarget}); err != nil {
		return nil, err
	}
	return &eval_target.GetSourceEvalTargetVersionResponse{
		EvalTargetVersion: target.EvalTargetVersionDO2DTO(evalTarget.EvalTargetVersion),
	}, nil
}

func (e EvalTargetApplicationImpl) SearchCustomEvalTarget(ctx context.Context, req *eval_target.SearchCustomEvalTargetRequest) (r *eval_target.SearchCustomEvalTargetResponse, err error) {
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.WorkspaceID == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("spaceID is nil"))
	}
	if req.ApplicationID == nil && req.CustomRPCServer == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("app info is nil"))
	}
	if req.Region == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("region is nil"))
	}
	if e.typedOperators[entity.EvalTargetTypeCustomRPCServer] == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(gptr.Indirect(req.WorkspaceID), 10),
		SpaceID:       gptr.Indirect(req.WorkspaceID),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	res, nextCursor, hasMore, err := e.typedOperators[entity.EvalTargetTypeCustomRPCServer].SearchCustomEvalTarget(ctx, &entity.SearchCustomEvalTargetParam{
		WorkspaceID:     req.WorkspaceID,
		Keyword:         req.Keyword,
		ApplicationID:   req.ApplicationID,
		CustomRPCServer: target.CustomRPCServerDTO2DO(req.CustomRPCServer),
		Region:          req.Region,
		Env:             req.Env,
		PageSize:        req.PageSize,
		PageToken:       req.PageToken,
	})
	if err != nil {
		return nil, err
	}
	return &eval_target.SearchCustomEvalTargetResponse{
		CustomEvalTargets: target.CustomEvalTargetDO2DTOs(res),
		NextPageToken:     &nextCursor,
		HasMore:           &hasMore,
	}, nil
}

func (e EvalTargetApplicationImpl) MockEvalTargetOutput(ctx context.Context, request *eval_target.MockEvalTargetOutputRequest) (r *eval_target.MockEvalTargetOutputResponse, err error) {
	// 参数验证
	if request == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("request is nil"))
	}

	// 验证targetType是否支持（仅记录型复用 base 的 operator）
	targetType := entity.EvalTargetType(request.TargetType)
	opType := resolveOperatorType(targetType)
	if e.typedOperators[opType] == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("target type not support"))
	}

	// 使用BuildBySource构建target实体（不保存）
	sourceTargetID := strconv.FormatInt(request.SourceTargetID, 10)
	evalTarget, err := e.typedOperators[opType].BuildBySource(ctx, request.WorkspaceID, sourceTargetID, request.EvalTargetVersion)
	if err != nil {
		return nil, err
	}
	if evalTarget == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("failed to build eval target from source"))
	}

	// 鉴权 - 与CreateEvalTarget保持一致，基于workspace进行鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluationTarget"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	// 使用构建的target实体的output schema生成mock数据
	var mockOutput map[string]string
	if evalTarget.EvalTargetVersion != nil && len(evalTarget.EvalTargetVersion.OutputSchema) > 0 {
		mockOutput, err = e.evalTargetService.GenerateMockOutputData(evalTarget.EvalTargetVersion.OutputSchema)
		if err != nil {
			return nil, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("failed to generate mock data: "+err.Error()))
		}
	} else {
		// 如果没有输出schema，返回空对象
		mockOutput = map[string]string{}
	}

	return &eval_target.MockEvalTargetOutputResponse{
		EvalTarget: target.EvalTargetDO2DTO(evalTarget),
		MockOutput: mockOutput,
	}, nil
}

func (e EvalTargetApplicationImpl) DebugEvalTarget(ctx context.Context, request *eval_target.DebugEvalTargetRequest) (r *eval_target.DebugEvalTargetResponse, err error) {
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.GetWorkspaceID(), 10),
		SpaceID:       request.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionDebugEvalTarget), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	logID := logs.GetLogID(ctx)

	inputFields := make(map[string]*spi.Content)
	if err := json.Unmarshal([]byte(request.GetParam()), &inputFields); err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(fmt.Sprintf("logid: %s, param json unmarshal fail", logID)))
	}

	switch request.GetEvalTargetType() {
	case eval_target_dto.EvalTargetType_CustomRPCServer:
		record, err := e.evalTargetService.DebugTarget(ctx, &entity.DebugTargetParam{
			SpaceID: request.GetWorkspaceID(),
			PatchyTarget: &entity.EvalTarget{
				SpaceID:        request.GetWorkspaceID(),
				EvalTargetType: entity.EvalTargetTypeCustomRPCServer,
				EvalTargetVersion: &entity.EvalTargetVersion{
					SpaceID:         request.GetWorkspaceID(),
					EvalTargetType:  entity.EvalTargetTypeCustomRPCServer,
					CustomRPCServer: target.CustomRPCServerDTO2DO(request.GetCustomRPCServer()),
				},
			},
			InputData: &entity.EvalTargetInputData{
				InputFields: gmap.Map(inputFields, func(k string, v *spi.Content) (string, *entity.Content) {
					return k, target.ToSPIContentDO(v)
				}),
				Ext: map[string]string{
					consts.FieldAdapterBuiltinFieldNameRuntimeParam: request.GetTargetRuntimeParam().GetJSONValue(),
				},
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("logid: %s", logID))
		}
		if record != nil && record.Status != nil && *record.Status == entity.EvalTargetRunStatusFail {
			if record.EvalTargetOutputData != nil && record.EvalTargetOutputData.EvalTargetRunError != nil {
				record.EvalTargetOutputData.EvalTargetRunError.Message = fmt.Sprintf("logid: %s, %s", logID, record.EvalTargetOutputData.EvalTargetRunError.Message)
			}
		}
		return &eval_target.DebugEvalTargetResponse{
			EvalTargetRecord: target.EvalTargetRecordDO2DTO(record),
			BaseResp:         base.NewBaseResp(),
		}, err
	default:
		return nil, errorx.New("logid: %s, unsupported eval target type %v", logID, request.GetEvalTargetType())
	}
}

func (e EvalTargetApplicationImpl) AsyncDebugEvalTarget(ctx context.Context, request *eval_target.AsyncDebugEvalTargetRequest) (r *eval_target.AsyncDebugEvalTargetResponse, err error) {
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.GetWorkspaceID(), 10),
		SpaceID:       request.GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionDebugEvalTarget), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	userID := session.UserIDInCtxOrEmpty(ctx)
	inputFields := make(map[string]*spi.Content)
	if err := json.Unmarshal([]byte(request.GetParam()), &inputFields); err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("param json unmarshal fail"))
	}

	switch request.GetEvalTargetType() {
	case eval_target_dto.EvalTargetType_CustomRPCServer:
		record, callee, err := e.evalTargetService.AsyncDebugTarget(ctx, &entity.DebugTargetParam{
			SpaceID: request.GetWorkspaceID(),
			PatchyTarget: &entity.EvalTarget{
				SpaceID:        request.GetWorkspaceID(),
				EvalTargetType: entity.EvalTargetTypeCustomRPCServer,
				EvalTargetVersion: &entity.EvalTargetVersion{
					SpaceID:         request.GetWorkspaceID(),
					EvalTargetType:  entity.EvalTargetTypeCustomRPCServer,
					CustomRPCServer: target.CustomRPCServerDTO2DO(request.GetCustomRPCServer()),
				},
			},
			InputData: &entity.EvalTargetInputData{
				InputFields: gmap.Map(inputFields, func(k string, v *spi.Content) (string, *entity.Content) {
					return k, target.ToSPIContentDO(v)
				}),
				Ext: map[string]string{
					consts.FieldAdapterBuiltinFieldNameRuntimeParam: request.GetTargetRuntimeParam().GetJSONValue(),
				},
			},
		})
		if err != nil {
			return nil, err
		}

		recordID := record.ID
		if err := e.evalAsyncRepo.SetEvalAsyncCtx(ctx, strconv.FormatInt(recordID, 10), &entity.EvalAsyncCtx{
			RecordID:    recordID,
			AsyncUnixMS: startTime.UnixMilli(),
			Session:     &entity.Session{UserID: userID},
			Callee:      callee,
		}); err != nil {
			return nil, err
		}

		return &eval_target.AsyncDebugEvalTargetResponse{
			InvokeID: record.ID,
			Callee:   gptr.Of(callee),
			BaseResp: base.NewBaseResp(),
		}, err
	default:
		return nil, errorx.New("unsupported eval target type %v", request.GetEvalTargetType())
	}
}
