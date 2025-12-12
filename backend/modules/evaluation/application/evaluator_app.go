// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/Masterminds/semver/v3"
	"github.com/bytedance/gg/gptr"
	"golang.org/x/sync/errgroup"

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	evaluatorcommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatorservice "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
	evaluatorconvertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/encoding"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// NewEvaluatorHandlerImpl 创建 EvaluatorService 实例
func NewEvaluatorHandlerImpl(idgen idgen.IIDGenerator,
	configer conf.IConfiger,
	auth rpc.IAuthProvider,
	evaluatorService service.EvaluatorService,
	evaluatorRecordService service.EvaluatorRecordService,
	evaluatorTemplateService service.EvaluatorTemplateService,
	metrics metrics.EvaluatorExecMetrics,
	userInfoService userinfo.UserInfoService,
	auditClient audit.IAuditService,
	benefitService benefit.IBenefitService,
	fileProvider rpc.IFileProvider,
	evaluatorSourceServices map[entity.EvaluatorType]service.EvaluatorSourceService,
) evaluation.EvaluatorService {
	handler := &EvaluatorHandlerImpl{
		idgen:                    idgen,
		auth:                     auth,
		auditClient:              auditClient,
		configer:                 configer,
		evaluatorService:         evaluatorService,
		evaluatorRecordService:   evaluatorRecordService,
		evaluatorTemplateService: evaluatorTemplateService,
		metrics:                  metrics,
		userInfoService:          userInfoService,
		benefitService:           benefitService,
		fileProvider:             fileProvider,
		evaluatorSourceServices:  evaluatorSourceServices,
	}
	return handler
}

// EvaluatorHandlerImpl 实现 EvaluatorService 接口
type EvaluatorHandlerImpl struct {
	idgen                    idgen.IIDGenerator
	auth                     rpc.IAuthProvider
	auditClient              audit.IAuditService
	configer                 conf.IConfiger
	evaluatorService         service.EvaluatorService
	evaluatorRecordService   service.EvaluatorRecordService
	evaluatorTemplateService service.EvaluatorTemplateService
	metrics                  metrics.EvaluatorExecMetrics
	userInfoService          userinfo.UserInfoService
	benefitService           benefit.IBenefitService
	fileProvider             rpc.IFileProvider
	evaluatorSourceServices  map[entity.EvaluatorType]service.EvaluatorSourceService
}

// ListEvaluators 按查询条件查询 evaluator
func (e *EvaluatorHandlerImpl) ListEvaluators(ctx context.Context, request *evaluatorservice.ListEvaluatorsRequest) (resp *evaluatorservice.ListEvaluatorsResponse, err error) {
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	var evaluatorDOS []*entity.Evaluator
	var total int64

	// 根据Builtin参数进行分流
	if request.GetBuiltin() {
		// 查询内置评估器
		evaluatorDOS, total, err = e.evaluatorService.ListBuiltinEvaluator(ctx, buildSrvListBuiltinEvaluatorRequest(request))
	} else {
		// 查询普通评估器
		evaluatorDOS, total, err = e.evaluatorService.ListEvaluator(ctx, buildSrvListEvaluatorRequest(request))
	}

	if err != nil {
		return nil, err
	}

	dtoList := make([]*evaluatordto.Evaluator, 0, len(evaluatorDOS))
	for _, evaluatorDO := range evaluatorDOS {
		dtoList = append(dtoList, evaluatorconvertor.ConvertEvaluatorDO2DTO(evaluatorDO))
	}
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier(dtoList))
	return &evaluatorservice.ListEvaluatorsResponse{
		Total:      gptr.Of(total),
		Evaluators: dtoList,
	}, nil
}

func buildSrvListEvaluatorRequest(request *evaluatorservice.ListEvaluatorsRequest) *entity.ListEvaluatorRequest {
	srvReq := &entity.ListEvaluatorRequest{
		SpaceID:     request.WorkspaceID,
		SearchName:  request.GetSearchName(),
		CreatorIDs:  request.GetCreatorIds(),
		PageSize:    request.GetPageSize(),
		PageNum:     request.GetPageNumber(),
		WithVersion: request.GetWithVersion(),
	}
	evaluatorType := make([]entity.EvaluatorType, 0, len(request.GetEvaluatorType()))
	for _, et := range request.GetEvaluatorType() {
		evaluatorType = append(evaluatorType, entity.EvaluatorType(et))
	}
	srvReq.EvaluatorType = evaluatorType
	orderBys := make([]*entity.OrderBy, 0, len(request.GetOrderBys()))
	for _, ob := range request.GetOrderBys() {
		orderBys = append(orderBys, &entity.OrderBy{
			Field: ob.Field,
			IsAsc: ob.IsAsc,
		})
	}
	srvReq.OrderBys = orderBys

	// 转换FilterOption
	if request.GetFilterOption() != nil {
		srvReq.FilterOption = evaluatorconvertor.ConvertEvaluatorFilterOptionDTO2DO(request.GetFilterOption())
	}

	return srvReq
}

func buildSrvListBuiltinEvaluatorRequest(request *evaluatorservice.ListEvaluatorsRequest) *entity.ListBuiltinEvaluatorRequest {
	srvReq := &entity.ListBuiltinEvaluatorRequest{
		PageSize:    request.GetPageSize(),
		PageNum:     request.GetPageNumber(),
		WithVersion: request.GetWithVersion(),
	}

	// 转换FilterOption
	if request.GetFilterOption() != nil {
		srvReq.FilterOption = evaluatorconvertor.ConvertEvaluatorFilterOptionDTO2DO(request.GetFilterOption())
	}

	return srvReq
}

// BatchGetEvaluator 按 id 批量查询 evaluator草稿
func (e *EvaluatorHandlerImpl) BatchGetEvaluators(ctx context.Context, request *evaluatorservice.BatchGetEvaluatorsRequest) (resp *evaluatorservice.BatchGetEvaluatorsResponse, err error) {
	// 获取元信息和草稿
	drafts, err := e.evaluatorService.BatchGetEvaluator(ctx, request.GetWorkspaceID(), request.GetEvaluatorIds(), request.GetIncludeDeleted())
	if err != nil {
		return nil, err
	}
	if len(drafts) == 0 {
		return &evaluatorservice.BatchGetEvaluatorsResponse{}, nil
	}
	// 对预置评估器/普通评估器需要分别鉴权
	builtinSpaceIDs := make(map[int64]struct{})
	normalSpaceIDs := make(map[int64]struct{})
	for _, draft := range drafts {
		if draft == nil {
			continue
		}
		if draft.Builtin {
			builtinSpaceIDs[draft.SpaceID] = struct{}{}
		} else {
			normalSpaceIDs[draft.SpaceID] = struct{}{}
		}
	}
	if len(builtinSpaceIDs) > 0 {
		for spaceID := range builtinSpaceIDs {
			// 预置评估器鉴权
			err = e.authBuiltinManagement(ctx, spaceID, spaceTypeBuiltin, false)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(normalSpaceIDs) > 0 {
		for spaceID := range normalSpaceIDs {
			// 普通评估器鉴权
			err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
				ObjectID:      strconv.FormatInt(spaceID, 10),
				SpaceID:       spaceID,
				ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
			})
			if err != nil {
				return nil, err
			}
		}
	}
	dtoList := make([]*evaluatordto.Evaluator, 0, len(drafts))
	for _, draft := range drafts {
		dtoList = append(dtoList, evaluatorconvertor.ConvertEvaluatorDO2DTO(draft))
	}
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier(dtoList))
	return &evaluatorservice.BatchGetEvaluatorsResponse{
		Evaluators: dtoList,
	}, nil
}

// GetEvaluator 按 id 单个查询 evaluator元信息和草稿
func (e *EvaluatorHandlerImpl) GetEvaluator(ctx context.Context, request *evaluatorservice.GetEvaluatorRequest) (resp *evaluatorservice.GetEvaluatorResponse, err error) {
	// 获取对应草稿版本
	draft, err := e.evaluatorService.GetEvaluator(ctx, request.GetWorkspaceID(), request.GetEvaluatorID(), request.GetIncludeDeleted())
	if err != nil {
		return nil, err
	}
	if draft == nil {
		return &evaluatorservice.GetEvaluatorResponse{}, nil
	}
	if draft.Builtin {
		// 预置评估器鉴权
		err = e.authBuiltinManagement(ctx, draft.SpaceID, spaceTypeBuiltin, false)
		if err != nil {
			return nil, err
		}
	} else {
		// 普通评估器鉴权
		err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
			ObjectID:      strconv.FormatInt(draft.ID, 10),
			SpaceID:       draft.SpaceID,
			ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
		})
		if err != nil {
			return nil, err
		}
	}
	dto := evaluatorconvertor.ConvertEvaluatorDO2DTO(draft)
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier([]*evaluatordto.Evaluator{dto}))
	return &evaluatorservice.GetEvaluatorResponse{
		Evaluator: dto,
	}, nil
}

// CreateEvaluator 创建 evaluator_version
func (e *EvaluatorHandlerImpl) CreateEvaluator(ctx context.Context, request *evaluatorservice.CreateEvaluatorRequest) (resp *evaluatorservice.CreateEvaluatorResponse, err error) {
	// 校验参数
	if err = e.checkCreateEvaluatorRequest(ctx, request); err != nil {
		return nil, err
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.GetEvaluator().GetWorkspaceID(), 10),
		SpaceID:       request.GetEvaluator().GetWorkspaceID(),
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("createLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	if request.GetEvaluator().GetEvaluatorType() == evaluatordto.EvaluatorType_CustomRPC {
		err = e.authCustomRPCEvaluatorContentWritable(ctx, request.GetEvaluator().GetWorkspaceID())
		if err != nil {
			return nil, err
		}
	}

	defer func() {
		e.metrics.EmitCreate(request.GetEvaluator().GetWorkspaceID(), err)
	}()
	// 转换请求参数为领域对象
	evaluatorDO, err := evaluatorconvertor.ConvertEvaluatorDTO2DO(request.GetEvaluator())
	if err != nil {
		return nil, err
	}

	// 统一走 CreateEvaluator，是否创建tag由repo层依据 do.Builtin 决定
	var evaluatorID int64
	evaluatorID, err = e.evaluatorService.CreateEvaluator(ctx, evaluatorDO, request.GetCid())
	if err != nil {
		return nil, err
	}

	// 返回创建结果
	return &evaluatorservice.CreateEvaluatorResponse{
		EvaluatorID: gptr.Of(evaluatorID),
	}, nil
}

func (e *EvaluatorHandlerImpl) checkCreateEvaluatorRequest(ctx context.Context, request *evaluatorservice.CreateEvaluatorRequest) (err error) {
	if request == nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if request.Evaluator == nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluator_version is nil"))
	}
	if request.Evaluator.Name == nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("name is nil"))
	}
	if request.Evaluator.WorkspaceID == nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("workspace id is nil"))
	}
	if request.Evaluator.EvaluatorType == nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluator_version type is nil"))
	}
	if request.Evaluator.CurrentVersion == nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("current version is nil"))
	}
	if request.Evaluator.CurrentVersion.EvaluatorContent == nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluator_version content is nil"))
	}
	if request.Evaluator.GetEvaluatorType() == evaluatordto.EvaluatorType_Prompt {
		if request.Evaluator.CurrentVersion.EvaluatorContent.PromptEvaluator == nil {
			return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("prompt evaluator_version is nil"))
		}
	}
	if utf8.RuneCountInString(request.Evaluator.GetName()) > consts.MaxEvaluatorNameLength {
		return errorx.NewByCode(errno.EvaluatorNameExceedMaxLengthCode, errorx.WithExtraMsg("name is too long"))
	}
	if utf8.RuneCountInString(request.Evaluator.GetDescription()) > consts.MaxEvaluatorDescLength {
		return errorx.NewByCode(errno.EvaluatorDescriptionExceedMaxLengthCode, errorx.WithExtraMsg("description is too long"))
	}
	// 机审
	auditTexts := make([]string, 0)
	auditTexts = append(auditTexts, request.Evaluator.GetName())
	auditTexts = append(auditTexts, request.Evaluator.GetDescription())
	auditTexts = append(auditTexts, request.Evaluator.GetCurrentVersion().GetDescription())
	data := map[string]string{
		"texts": strings.Join(auditTexts, ","),
	}
	record, err := e.auditClient.Audit(ctx, audit.AuditParam{
		AuditData: data,
		ReqID:     encoding.Encode(ctx, data),
		AuditType: audit.AuditType_CozeLoopEvaluatorModify,
	})
	if err != nil {
		logs.CtxError(ctx, "audit: failed to audit, err=%v", err) // 审核服务不可用，默认通过
	}
	if record.AuditStatus == audit.AuditStatus_Rejected {
		return errorx.NewByCode(errno.RiskContentDetectedCode)
	}
	return nil
}

// UpdateEvaluator 修改 evaluator_version
func (e *EvaluatorHandlerImpl) UpdateEvaluator(ctx context.Context, request *evaluatorservice.UpdateEvaluatorRequest) (resp *evaluatorservice.UpdateEvaluatorResponse, err error) {
	err = validateUpdateEvaluatorRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	// 鉴权
	evaluatorDO, err := e.evaluatorService.GetEvaluator(ctx, request.GetWorkspaceID(), request.GetEvaluatorID(), false)
	if err != nil {
		return nil, err
	}
	if evaluatorDO == nil {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(evaluatorDO.ID, 10),
		SpaceID:       evaluatorDO.SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
	})
	if err != nil {
		return nil, err
	}
	// 如果是builtin分支，补充管理空间校验
	if request.GetBuiltin() {
		if err := e.authBuiltinManagement(ctx, request.GetWorkspaceID(), spaceTypeBuiltin, true); err != nil {
			return nil, err
		}
	}
	// 机审
	if err := e.auditEvaluatorModify(ctx, evaluatorDO.ID, []string{request.GetName(), request.GetDescription()}); err != nil {
		return nil, err
	}
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	// 组装请求
	req := &entity.UpdateEvaluatorMetaRequest{
		ID:                    request.GetEvaluatorID(),
		SpaceID:               request.GetWorkspaceID(),
		Name:                  request.Name,
		Description:           request.Description,
		Builtin:               request.Builtin,
		EvaluatorInfo:         nil,
		BuiltinVisibleVersion: request.BuiltinVisibleVersion,
		UpdatedBy:             userIDInContext,
	}
	// 转换 EvaluatorInfo
	if request.IsSetEvaluatorInfo() && request.GetEvaluatorInfo() != nil {
		req.EvaluatorInfo = &entity.EvaluatorInfo{
			Benchmark:     request.GetEvaluatorInfo().Benchmark,
			Vendor:        request.GetEvaluatorInfo().Vendor,
			VendorURL:     request.GetEvaluatorInfo().VendorURL,
			UserManualURL: request.GetEvaluatorInfo().UserManualURL,
		}
	}
	// box_type 映射（White/Black -> 1/2）
	if request.IsSetBoxType() {
		bt := entity.EvaluatorBoxTypeWhite
		switch request.GetBoxType() {
		case "Black":
			bt = entity.EvaluatorBoxTypeBlack
		case "White":
			bt = entity.EvaluatorBoxTypeWhite
		}
		req.BoxType = &bt
	}

	if err = e.evaluatorService.UpdateEvaluatorMeta(ctx, req); err != nil {
		return nil, err
	}
	return &evaluatorservice.UpdateEvaluatorResponse{}, nil
}

// auditEvaluatorModify 抽取的机审逻辑：传入要审计的文本列表
func (e *EvaluatorHandlerImpl) auditEvaluatorModify(ctx context.Context, objectID int64, texts []string) error {
	data := map[string]string{
		"texts": strings.Join(texts, ","),
	}
	record, err := e.auditClient.Audit(ctx, audit.AuditParam{
		ObjectID:  objectID,
		AuditData: data,
		ReqID:     encoding.Encode(ctx, data),
		AuditType: audit.AuditType_CozeLoopEvaluatorModify,
	})
	if err != nil {
		logs.CtxError(ctx, "audit: failed to audit, err=%v", err) // 审核服务不可用，默认通过
		return nil
	}
	if record.AuditStatus == audit.AuditStatus_Rejected {
		return errorx.NewByCode(errno.RiskContentDetectedCode)
	}
	return nil
}

func validateUpdateEvaluatorRequest(ctx context.Context, request *evaluatorservice.UpdateEvaluatorRequest) error {
	if request == nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if request.GetEvaluatorID() == 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("id is 0"))
	}
	if request.WorkspaceID == 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("space id is 0"))
	}
	if utf8.RuneCountInString(request.GetName()) > consts.MaxEvaluatorNameLength {
		return errorx.NewByCode(errno.EvaluatorNameExceedMaxLengthCode)
	}
	if utf8.RuneCountInString(request.GetDescription()) > consts.MaxEvaluatorDescLength {
		return errorx.NewByCode(errno.EvaluatorDescriptionExceedMaxLengthCode)
	}
	return nil
}

// UpdateEvaluatorDraft 修改 evaluator_version
func (e *EvaluatorHandlerImpl) UpdateEvaluatorDraft(ctx context.Context, request *evaluatorservice.UpdateEvaluatorDraftRequest) (resp *evaluatorservice.UpdateEvaluatorDraftResponse, err error) {
	// 鉴权
	evaluatorDO, err := e.evaluatorService.GetEvaluator(ctx, request.GetWorkspaceID(), request.GetEvaluatorID(), false)
	if err != nil {
		return nil, err
	}
	if evaluatorDO == nil {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(evaluatorDO.ID, 10),
		SpaceID:       evaluatorDO.SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
	})
	if err != nil {
		return nil, err
	}
	if request.GetEvaluatorType() == evaluatordto.EvaluatorType_CustomRPC {
		err = e.authCustomRPCEvaluatorContentWritable(ctx, evaluatorDO.SpaceID)
		if err != nil {
			return nil, err
		}
	}
	evaluatorDTO := evaluatorconvertor.ConvertEvaluatorDO2DTO(evaluatorDO)
	evaluatorDTO.CurrentVersion.EvaluatorContent = request.EvaluatorContent
	evaluatorDTO.DraftSubmitted = ptr.Of(false)
	evaluatorDO, err = evaluatorconvertor.ConvertEvaluatorDTO2DO(evaluatorDTO)
	if err != nil {
		return nil, err
	}
	err = e.evaluatorService.UpdateEvaluatorDraft(ctx, evaluatorDO)
	if err != nil {
		return nil, err
	}
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier([]*evaluatordto.Evaluator{evaluatorDTO}))
	return &evaluatorservice.UpdateEvaluatorDraftResponse{
		Evaluator: evaluatorDTO,
	}, nil
}

// DeleteEvaluator 删除 evaluator_version
func (e *EvaluatorHandlerImpl) DeleteEvaluator(ctx context.Context, request *evaluatorservice.DeleteEvaluatorRequest) (resp *evaluatorservice.DeleteEvaluatorResponse, err error) {
	// 鉴权
	evaluatorDOS, err := e.evaluatorService.BatchGetEvaluator(ctx, request.GetWorkspaceID(), []int64{request.GetEvaluatorID()}, false)
	if err != nil {
		return nil, err
	}
	g, gCtx := errgroup.WithContext(ctx)
	for _, evaluatorDO := range evaluatorDOS {
		if evaluatorDO == nil {
			continue
		}
		curEvaluator := evaluatorDO
		g.Go(func() error {
			defer func() {
				if r := recover(); r != nil {
					logs.CtxError(ctx, "goroutine panic: %v", r)
				}
			}()
			return e.auth.Authorization(gCtx, &rpc.AuthorizationParam{
				ObjectID:      strconv.FormatInt(curEvaluator.ID, 10),
				SpaceID:       curEvaluator.SpaceID,
				ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
			})
		})

	}
	if err = g.Wait(); err != nil {
		return nil, err
	}
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	err = e.evaluatorService.DeleteEvaluator(ctx, []int64{request.GetEvaluatorID()}, userIDInContext)
	if err != nil {
		return nil, err
	}
	return &evaluatorservice.DeleteEvaluatorResponse{}, nil
}

// ListEvaluatorVersions 按查询条件查询 evaluator_version version
func (e *EvaluatorHandlerImpl) ListEvaluatorVersions(ctx context.Context, request *evaluatorservice.ListEvaluatorVersionsRequest) (resp *evaluatorservice.ListEvaluatorVersionsResponse, err error) {
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	evaluatorDOList, total, err := e.evaluatorService.ListEvaluatorVersion(ctx, buildListEvaluatorVersionRequest(request))
	if err != nil {
		return nil, err
	}
	// 转换结果集
	dtoList := make([]*evaluatordto.EvaluatorVersion, 0, len(evaluatorDOList))
	for _, evaluatorDO := range evaluatorDOList {
		dtoList = append(dtoList, evaluatorconvertor.ConvertEvaluatorDO2DTO(evaluatorDO).GetCurrentVersion())
	}
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier(dtoList))
	// 返回查询结果
	return &evaluatorservice.ListEvaluatorVersionsResponse{
		EvaluatorVersions: dtoList,
		Total:             gptr.Of(total),
	}, nil
}

func buildListEvaluatorVersionRequest(request *evaluatorservice.ListEvaluatorVersionsRequest) *entity.ListEvaluatorVersionRequest {
	// 转换请求参数为repo层结构
	req := &entity.ListEvaluatorVersionRequest{
		EvaluatorID:   request.GetEvaluatorID(),
		QueryVersions: request.GetQueryVersions(),
	}
	if request.PageSize == nil {
		req.PageSize = consts.DefaultListEvaluatorVersionPageSize
	} else {
		req.PageSize = request.GetPageSize()
	}
	if request.PageNumber == nil {
		req.PageNum = consts.DefaultListEvaluatorVersionPageNum
	} else {
		req.PageNum = request.GetPageNumber()
	}
	if len(request.GetOrderBys()) == 0 {
		req.OrderBys = []*entity.OrderBy{
			{
				Field: gptr.Of("updated_at"),
				IsAsc: gptr.Of(false),
			},
		}
	} else {
		orderBy := make([]*entity.OrderBy, 0, len(request.GetOrderBys()))
		for _, ob := range request.GetOrderBys() {
			orderBy = append(orderBy, &entity.OrderBy{
				Field: ob.Field,
				IsAsc: ob.IsAsc,
			})
		}
		req.OrderBys = orderBy
	}
	return req
}

// GetEvaluatorVersion 按 id 和版本号单个查询 evaluator_version version
func (e *EvaluatorHandlerImpl) GetEvaluatorVersion(ctx context.Context, request *evaluatorservice.GetEvaluatorVersionRequest) (resp *evaluatorservice.GetEvaluatorVersionResponse, err error) {
	var spaceID *int64
	if !request.GetBuiltin() {
		spaceID = gptr.Of(request.WorkspaceID)
	}
	evaluatorDO, err := e.evaluatorService.GetEvaluatorVersion(ctx, spaceID, request.GetEvaluatorVersionID(), request.GetIncludeDeleted(), request.GetBuiltin())
	if err != nil {
		return nil, err
	}
	if evaluatorDO == nil {
		return &evaluatorservice.GetEvaluatorVersionResponse{}, nil
	}
	// 鉴权
	if request.GetBuiltin() {
		err = e.authBuiltinManagement(ctx, evaluatorDO.SpaceID, spaceTypeBuiltin, false)
		if err != nil {
			return nil, err
		}
		// 预置评估器复用空间下列表查询鉴权
		err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
			ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
			SpaceID:       request.WorkspaceID,
			ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
		})
		if err != nil {
			return nil, err
		}
	} else {
		err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
			ObjectID:      strconv.FormatInt(evaluatorDO.ID, 10),
			SpaceID:       evaluatorDO.SpaceID,
			ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
		})
		if err != nil {
			return nil, err
		}
	}

	dto := evaluatorconvertor.ConvertEvaluatorDO2DTO(evaluatorDO)
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier([]*evaluatordto.Evaluator{dto}))
	evaluatorVersionDTO := dto.CurrentVersion
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier([]*evaluatordto.EvaluatorVersion{evaluatorVersionDTO}))
	// 返回查询结果
	return &evaluatorservice.GetEvaluatorVersionResponse{
		Evaluator: dto,
	}, nil
}

func (e *EvaluatorHandlerImpl) BatchGetEvaluatorVersions(ctx context.Context, request *evaluatorservice.BatchGetEvaluatorVersionsRequest) (resp *evaluatorservice.BatchGetEvaluatorVersionsResponse, err error) {
	// 查询时不传 space_id，允许查询所有空间的 evaluator
	evaluatorDOList, err := e.evaluatorService.BatchGetEvaluatorVersion(ctx, nil, request.GetEvaluatorVersionIds(), request.GetIncludeDeleted())
	if err != nil {
		return nil, err
	}
	if len(evaluatorDOList) == 0 {
		return &evaluatorservice.BatchGetEvaluatorVersionsResponse{}, nil
	}
	// 按 SpaceID 分组进行权限校验
	requestWorkspaceID := request.WorkspaceID
	checkedSpaceIDs := make(map[int64]bool)
	for _, evaluatorDO := range evaluatorDOList {
		spaceID := evaluatorDO.SpaceID
		// 如果已经校验过该空间，跳过
		if checkedSpaceIDs[spaceID] {
			continue
		}
		checkedSpaceIDs[spaceID] = true

		// 如果是请求的空间下的，使用原来的权限校验逻辑
		if spaceID == requestWorkspaceID {
			err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
				ObjectID:      strconv.FormatInt(spaceID, 10),
				SpaceID:       spaceID,
				ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
			})
			if err != nil {
				return nil, err
			}
		} else {
			// 如果不是，使用预置空间校验的 authBuiltinManagement 函数进行校验
			err = e.authBuiltinManagement(ctx, spaceID, spaceTypeBuiltin, false)
			if err != nil {
				return nil, err
			}
		}
	}
	dtoList := make([]*evaluatordto.Evaluator, 0, len(evaluatorDOList))
	for _, evaluatorDO := range evaluatorDOList {
		dtoList = append(dtoList, evaluatorconvertor.ConvertEvaluatorDO2DTO(evaluatorDO))
	}
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier(dtoList))
	evaluatorVersionDTOList := make([]*evaluatordto.EvaluatorVersion, 0, len(dtoList))
	for _, dto := range dtoList {
		evaluatorVersionDTOList = append(evaluatorVersionDTOList, dto.CurrentVersion)
	}
	e.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier(evaluatorVersionDTOList))
	return &evaluatorservice.BatchGetEvaluatorVersionsResponse{
		Evaluators: dtoList,
	}, nil
}

// SubmitEvaluatorVersion 提交 evaluator_version 版本
func (e *EvaluatorHandlerImpl) SubmitEvaluatorVersion(ctx context.Context, request *evaluatorservice.SubmitEvaluatorVersionRequest) (resp *evaluatorservice.SubmitEvaluatorVersionResponse, err error) {
	// 校验参数
	err = e.validateSubmitEvaluatorVersionRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	// 鉴权
	evaluatorDO, err := e.evaluatorService.GetEvaluator(ctx, request.GetWorkspaceID(), request.GetEvaluatorID(), false)
	if err != nil {
		return nil, err
	}
	if evaluatorDO == nil {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(evaluatorDO.ID, 10),
		SpaceID:       evaluatorDO.SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
	})
	if err != nil {
		return nil, err
	}
	evaluatorDO, err = e.evaluatorService.SubmitEvaluatorVersion(ctx, evaluatorDO, request.GetVersion(), request.GetDescription(), request.GetCid())
	if err != nil {
		return nil, err
	}

	return &evaluatorservice.SubmitEvaluatorVersionResponse{
		Evaluator: evaluatorconvertor.ConvertEvaluatorDO2DTO(evaluatorDO),
	}, nil
}

func (e *EvaluatorHandlerImpl) validateSubmitEvaluatorVersionRequest(ctx context.Context, request *evaluatorservice.SubmitEvaluatorVersionRequest) error {
	if request.GetEvaluatorID() == 0 {
		return errorx.NewByCode(errno.InvalidEvaluatorIDCode, errorx.WithExtraMsg("[validateSubmitEvaluatorVersionRequest] evaluator_version id is empty"))
	}
	if len(request.GetVersion()) == 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("[validateSubmitEvaluatorVersionRequest] evaluator_version version is empty"))
	}
	if len(request.GetVersion()) > consts.MaxEvaluatorVersionLength {
		return errorx.NewByCode(errno.EvaluatorVersionExceedMaxLengthCode)
	}
	_, err := semver.StrictNewVersion(request.GetVersion())
	if err != nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("[validateSubmitEvaluatorVersionRequest] evaluator_version version does not follow SemVer specification"))
	}
	if len(request.GetDescription()) > consts.MaxEvaluatorVersionDescLength {
		return errorx.NewByCode(errno.EvaluatorVersionDescriptionExceedMaxLengthCode)
	}
	// 机审
	auditTexts := make([]string, 0)
	auditTexts = append(auditTexts, request.GetDescription())
	data := map[string]string{
		"texts": strings.Join(auditTexts, ","),
	}
	record, err := e.auditClient.Audit(ctx, audit.AuditParam{
		ObjectID:  request.GetEvaluatorID(),
		AuditData: data,
		ReqID:     encoding.Encode(ctx, data),
		AuditType: audit.AuditType_CozeLoopEvaluatorModify,
	})
	if err != nil {
		logs.CtxError(ctx, "audit: failed to audit, err=%v", err) // 审核服务不可用，默认通过
	}
	if record.AuditStatus == audit.AuditStatus_Rejected {
		return errorx.NewByCode(errno.RiskContentDetectedCode)
	}
	return nil
}

// ListBuiltinTemplate 获取内置评估器模板列表
func (e *EvaluatorHandlerImpl) ListTemplates(ctx context.Context, request *evaluatorservice.ListTemplatesRequest) (resp *evaluatorservice.ListTemplatesResponse, err error) {
	templateType := strings.ToLower(request.GetBuiltinTemplateType().String())

	// 针对Code类型使用新的配置方法
	if templateType == "code" {
		codeTemplates := e.configer.GetCodeEvaluatorTemplateConf(ctx)

		if codeTemplates == nil {
			return &evaluatorservice.ListTemplatesResponse{
				BuiltinTemplateKeys: make([]*evaluatordto.EvaluatorContent, 0),
			}, nil
		}

		// 仅返回template_key构建的list结果，不进行language_type筛选
		return &evaluatorservice.ListTemplatesResponse{
			BuiltinTemplateKeys: buildCodeTemplateKeys(codeTemplates),
		}, nil
	}

	// 其他类型保持原有逻辑
	builtinTemplates := e.configer.GetEvaluatorTemplateConf(ctx)[templateType]

	if builtinTemplates == nil {
		return &evaluatorservice.ListTemplatesResponse{
			BuiltinTemplateKeys: make([]*evaluatordto.EvaluatorContent, 0),
		}, nil
	}

	return &evaluatorservice.ListTemplatesResponse{
		BuiltinTemplateKeys: buildTemplateKeys(builtinTemplates, request.GetBuiltinTemplateType()),
	}, nil
}

// buildTemplateKeys 构建Prompt类型的模板键列表
// 注意：此函数只处理Prompt类型的Evaluator，Code类型请使用buildCodeTemplateKeys函数
func buildTemplateKeys(origins map[string]*evaluatordto.EvaluatorContent, templateType evaluatordto.TemplateType) []*evaluatordto.EvaluatorContent {
	keys := make([]*evaluatordto.EvaluatorContent, 0, len(origins))

	for _, origin := range origins {
		evaluatorContent := &evaluatordto.EvaluatorContent{}

		// 只处理Prompt类型的模板
		if templateType == evaluatordto.TemplateType_Prompt && origin.GetPromptEvaluator() != nil {
			evaluatorContent.PromptEvaluator = &evaluatordto.PromptEvaluator{
				PromptTemplateKey:  origin.GetPromptEvaluator().PromptTemplateKey,
				PromptTemplateName: origin.GetPromptEvaluator().PromptTemplateName,
			}
			keys = append(keys, evaluatorContent)
		}
	}

	// 按PromptTemplateKey排序
	sort.Slice(keys, func(i, j int) bool {
		keyI := keys[i].GetPromptEvaluator().GetPromptTemplateKey()
		keyJ := keys[j].GetPromptEvaluator().GetPromptTemplateKey()
		return keyI < keyJ
	})
	return keys
}

func buildCodeTemplateKeys(codeTemplates map[string]map[string]*evaluatordto.EvaluatorContent) []*evaluatordto.EvaluatorContent {
	// 用于去重的map，key为template_key
	templateKeyMap := make(map[string]*evaluatordto.EvaluatorContent)

	// 遍历所有模板，按template_key去重
	for templateKey, languageMap := range codeTemplates {
		if templateKey != "" {
			// 如果已存在相同template_key，保留第一个
			if _, exists := templateKeyMap[templateKey]; !exists {
				// 取第一个语言类型的模板作为代表
				for _, template := range languageMap {
					if template.GetCodeEvaluator() != nil {
						templateKeyMap[templateKey] = &evaluatordto.EvaluatorContent{
							CodeEvaluator: &evaluatordto.CodeEvaluator{
								CodeTemplateKey:  template.GetCodeEvaluator().CodeTemplateKey,
								CodeTemplateName: template.GetCodeEvaluator().CodeTemplateName,
								// 不包含LanguageType，因为只返回template_key
							},
						}
						break // 只取第一个
					}
				}
			}
		}
	}

	// 转换为slice并排序
	keys := make([]*evaluatordto.EvaluatorContent, 0, len(templateKeyMap))
	for _, template := range templateKeyMap {
		keys = append(keys, template)
	}

	// 按template_key排序
	sort.Slice(keys, func(i, j int) bool {
		keyI := keys[i].GetCodeEvaluator().GetCodeTemplateKey()
		keyJ := keys[j].GetCodeEvaluator().GetCodeTemplateKey()
		return keyI < keyJ
	})

	return keys
}

// GetEvaluatorTemplate 按 key 单个查询内置评估器模板详情
func (e *EvaluatorHandlerImpl) GetTemplateInfo(ctx context.Context, request *evaluatorservice.GetTemplateInfoRequest) (resp *evaluatorservice.GetTemplateInfoResponse, err error) {
	templateType := strings.ToLower(request.GetBuiltinTemplateType().String())
	templateKey := request.GetBuiltinTemplateKey()

	var template *evaluatordto.EvaluatorContent
	var ok bool

	if templateType == "code" {
		// 检查是否为custom类型
		if templateKey == "custom" {
			// 使用custom配置方法
			customTemplates := e.configer.GetCustomCodeEvaluatorTemplateConf(ctx)
			if languageMap, exists := customTemplates[templateKey]; exists {
				if request.GetLanguageType() != "" {
					// 指定了语言类型，查找对应的模板
					template, ok = languageMap[request.GetLanguageType()]
				} else {
					// 未指定语言类型，返回第一个可用的模板
					for _, t := range languageMap {
						template = t
						ok = true
						break
					}
				}
			}
		} else {
			// Code类型使用新的配置方法
			codeTemplates := e.configer.GetCodeEvaluatorTemplateConf(ctx)
			if languageMap, exists := codeTemplates[templateKey]; exists {
				if request.GetLanguageType() != "" {
					// 指定了语言类型，查找对应的模板
					template, ok = languageMap[request.GetLanguageType()]
				} else {
					template, ok = languageMap[evaluatordto.LanguageTypePython]
				}
			}
		}
	} else {
		// 其他类型使用原有逻辑
		allTemplates := e.configer.GetEvaluatorTemplateConf(ctx)[templateType]
		template, ok = allTemplates[templateKey]
	}

	if !ok || template == nil {
		return nil, errorx.NewByCode(errno.TemplateNotFoundCode, errorx.WithExtraMsg("builtin template not found"))
	}

	return &evaluatorservice.GetTemplateInfoResponse{
		EvaluatorContent: template,
	}, nil
}

// RunEvaluator evaluator_version 运行
func (e *EvaluatorHandlerImpl) RunEvaluator(ctx context.Context, request *evaluatorservice.RunEvaluatorRequest) (resp *evaluatorservice.RunEvaluatorResponse, err error) {
	evaluatorDO, err := e.evaluatorService.GetEvaluatorVersion(ctx, nil, request.GetEvaluatorVersionID(), false, false)
	if err != nil {
		return nil, err
	}
	if evaluatorDO == nil {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	// 若为预置评估器则跳过鉴权
	if !evaluatorDO.Builtin {
		// 鉴权
		err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
			ObjectID:      strconv.FormatInt(evaluatorDO.ID, 10),
			SpaceID:       evaluatorDO.SpaceID,
			ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
		})
		if err != nil {
			return nil, err
		}
	}
	recordDO, err := e.evaluatorService.RunEvaluator(ctx, buildRunEvaluatorRequest(evaluatorDO.Name, request))
	if err != nil {
		return nil, err
	}
	return &evaluatorservice.RunEvaluatorResponse{
		Record: evaluatorconvertor.ConvertEvaluatorRecordDO2DTO(recordDO),
	}, nil
}

func buildRunEvaluatorRequest(evaluatorName string, request *evaluatorservice.RunEvaluatorRequest) *entity.RunEvaluatorRequest {
	srvReq := &entity.RunEvaluatorRequest{
		SpaceID:            request.WorkspaceID,
		Name:               evaluatorName,
		EvaluatorVersionID: request.EvaluatorVersionID,
		ExperimentID:       request.GetExperimentID(),
		ExperimentRunID:    request.GetExperimentRunID(),
		ItemID:             request.GetItemID(),
		TurnID:             request.GetTurnID(),
		EvaluatorRunConf:   evaluatorconvertor.ConvertEvaluatorRunConfDTO2DO(request.GetEvaluatorRunConf()),
	}
	inputData := evaluatorconvertor.ConvertEvaluatorInputDataDTO2DO(request.GetInputData())
	if request.IsSetEvaluatorRunConf() && request.GetEvaluatorRunConf().IsSetEvaluatorRuntimeParam() &&
		request.GetEvaluatorRunConf().GetEvaluatorRuntimeParam().IsSetJSONValue() {
		if inputData == nil {
			inputData = &entity.EvaluatorInputData{}
		}
		if inputData.Ext == nil {
			inputData.Ext = make(map[string]string)
		}
		inputData.Ext[consts.FieldAdapterBuiltinFieldNameRuntimeParam] = request.GetEvaluatorRunConf().GetEvaluatorRuntimeParam().GetJSONValue()
	}

	srvReq.InputData = inputData
	return srvReq
}

// DebugEvaluator 调试 evaluator_version
func (e *EvaluatorHandlerImpl) DebugEvaluator(ctx context.Context, request *evaluatorservice.DebugEvaluatorRequest) (resp *evaluatorservice.DebugEvaluatorResponse, err error) {
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("debugLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	userID := session.UserIDInCtxOrEmpty(ctx)

	req := &benefit.CheckEvaluatorBenefitParams{
		ConnectorUID: userID,
		SpaceID:      request.GetWorkspaceID(),
	}
	result, err := e.benefitService.CheckEvaluatorBenefit(ctx, req)
	if err != nil {
		return nil, err
	}

	logs.CtxInfo(ctx, "DebugEvaluator CheckEvaluatorBenefit result: %v,", json.Jsonify(result))

	if result != nil && result.DenyReason != nil {
		return nil, errorx.NewByCode(errno.EvaluatorBenefitDenyCode)
	}

	// URI转换处理
	if request.InputData != nil {
		err = e.transformURIsToURLs(ctx, request.InputData.InputFields)
		if err != nil {
			logs.CtxError(ctx, "failed to transform URIs to URLs: %v", err)
			return nil, err
		}
	}

	dto := &evaluatordto.Evaluator{
		WorkspaceID:   gptr.Of(request.WorkspaceID),
		EvaluatorType: gptr.Of(request.EvaluatorType),
		CurrentVersion: &evaluatordto.EvaluatorVersion{
			EvaluatorContent: request.EvaluatorContent,
		},
	}
	do, err := evaluatorconvertor.ConvertEvaluatorDTO2DO(dto)
	if err != nil {
		return nil, err
	}
	inputData := evaluatorconvertor.ConvertEvaluatorInputDataDTO2DO(request.GetInputData())
	evaluatorRunConf := evaluatorconvertor.ConvertEvaluatorRunConfDTO2DO(request.GetEvaluatorRunConf())
	if request.IsSetEvaluatorRunConf() && request.GetEvaluatorRunConf().IsSetEvaluatorRuntimeParam() &&
		request.GetEvaluatorRunConf().GetEvaluatorRuntimeParam().IsSetJSONValue() {
		if inputData == nil {
			inputData = &entity.EvaluatorInputData{}
		}
		if inputData.Ext == nil {
			inputData.Ext = make(map[string]string)
		}
		inputData.Ext[consts.FieldAdapterBuiltinFieldNameRuntimeParam] = request.GetEvaluatorRunConf().GetEvaluatorRuntimeParam().GetJSONValue()
	}
	outputData, err := e.evaluatorService.DebugEvaluator(ctx, do, inputData, evaluatorRunConf, request.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return &evaluatorservice.DebugEvaluatorResponse{
		EvaluatorOutputData: evaluatorconvertor.ConvertEvaluatorOutputDataDO2DTO(outputData),
	}, nil
}

// UpdateEvaluatorRecord 创建 evaluator_version 运行结果
func (e *EvaluatorHandlerImpl) UpdateEvaluatorRecord(ctx context.Context, request *evaluatorservice.UpdateEvaluatorRecordRequest) (resp *evaluatorservice.UpdateEvaluatorRecordResponse, err error) {
	evaluatorRecord, err := e.evaluatorRecordService.GetEvaluatorRecord(ctx, request.GetEvaluatorRecordID(), false)
	if err != nil {
		return nil, err
	}
	if evaluatorRecord == nil {
		return nil, errorx.NewByCode(errno.EvaluatorRecordNotFoundCode)
	}
	// 鉴权
	evaluatorDO, err := e.evaluatorService.GetEvaluatorVersion(ctx, nil, evaluatorRecord.EvaluatorVersionID, false, false)
	if err != nil {
		return nil, err
	}
	if evaluatorDO == nil {
		return &evaluatorservice.UpdateEvaluatorRecordResponse{}, nil
	}
	if evaluatorDO.Builtin {
		if err := e.authBuiltinManagement(ctx, evaluatorDO.SpaceID, spaceTypeBuiltin, false); err != nil {
			return nil, err
		}
	} else {
		err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
			ObjectID:      strconv.FormatInt(evaluatorDO.ID, 10),
			SpaceID:       evaluatorDO.SpaceID,
			ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
		})
		if err != nil {
			return nil, err
		}
	}
	// 机审
	auditTexts := make([]string, 0)
	if request.Correction != nil {
		auditTexts = append(auditTexts, request.GetCorrection().GetExplain())
	}
	data := map[string]string{
		"texts": strings.Join(auditTexts, ","),
	}
	record, err := e.auditClient.Audit(ctx, audit.AuditParam{
		ObjectID:  evaluatorDO.ID,
		AuditData: data,
		ReqID:     encoding.Encode(ctx, data),
		AuditType: audit.AuditType_CozeLoopEvaluatorModify,
	})
	if err != nil {
		logs.CtxError(ctx, "audit: failed to audit, err=%v", err) // 审核服务不可用，默认通过
	}
	if record.AuditStatus == audit.AuditStatus_Rejected {
		return nil, errorx.NewByCode(errno.RiskContentDetectedCode)
	}
	correctionDO := evaluatorconvertor.ConvertCorrectionDTO2DO(request.GetCorrection())
	err = e.evaluatorRecordService.CorrectEvaluatorRecord(ctx, evaluatorRecord, correctionDO)
	if err != nil {
		return nil, err
	}
	return &evaluatorservice.UpdateEvaluatorRecordResponse{
		Record: evaluatorconvertor.ConvertEvaluatorRecordDO2DTO(evaluatorRecord),
	}, nil
}

func (e *EvaluatorHandlerImpl) GetEvaluatorRecord(ctx context.Context, request *evaluatorservice.GetEvaluatorRecordRequest) (resp *evaluatorservice.GetEvaluatorRecordResponse, err error) {
	evaluatorRecord, err := e.evaluatorRecordService.GetEvaluatorRecord(ctx, request.GetEvaluatorRecordID(), request.GetIncludeDeleted())
	if err != nil {
		return nil, err
	}
	if evaluatorRecord == nil {
		return &evaluatorservice.GetEvaluatorRecordResponse{}, nil
	}
	// 鉴权
	evaluatorDO, err := e.evaluatorService.GetEvaluatorVersion(ctx, nil, evaluatorRecord.EvaluatorVersionID, request.GetIncludeDeleted(), false)
	if err != nil {
		return nil, err
	}
	if evaluatorDO == nil {
		return &evaluatorservice.GetEvaluatorRecordResponse{}, nil
	}
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(evaluatorDO.ID, 10),
		SpaceID:       evaluatorDO.SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
	})
	if err != nil {
		return nil, err
	}
	dto := evaluatorconvertor.ConvertEvaluatorRecordDO2DTO(evaluatorRecord)
	e.userInfoService.PackUserInfo(ctx, []userinfo.UserInfoCarrier{dto})
	return &evaluatorservice.GetEvaluatorRecordResponse{
		Record: dto,
	}, nil
}

func (e *EvaluatorHandlerImpl) BatchGetEvaluatorRecords(ctx context.Context, request *evaluatorservice.BatchGetEvaluatorRecordsRequest) (resp *evaluatorservice.BatchGetEvaluatorRecordsResponse, err error) {
	evaluatorRecordIDs := request.GetEvaluatorRecordIds()
	evaluatorRecords, err := e.evaluatorRecordService.BatchGetEvaluatorRecord(ctx, evaluatorRecordIDs, request.GetIncludeDeleted())
	if err != nil {
		return nil, err
	}
	if len(evaluatorRecords) == 0 {
		return &evaluatorservice.BatchGetEvaluatorRecordsResponse{}, nil
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(evaluatorRecords[0].SpaceID, 10),
		SpaceID:       evaluatorRecords[0].SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	dtoList := make([]*evaluatordto.EvaluatorRecord, 0, len(evaluatorRecords))
	for _, evaluatorRecord := range evaluatorRecords {
		dto := evaluatorconvertor.ConvertEvaluatorRecordDO2DTO(evaluatorRecord)
		dtoList = append(dtoList, dto)
	}
	return &evaluatorservice.BatchGetEvaluatorRecordsResponse{
		Records: dtoList,
	}, nil
}

func (e *EvaluatorHandlerImpl) GetDefaultPromptEvaluatorTools(ctx context.Context, request *evaluatorservice.GetDefaultPromptEvaluatorToolsRequest) (resp *evaluatorservice.GetDefaultPromptEvaluatorToolsResponse, err error) {
	return &evaluatorservice.GetDefaultPromptEvaluatorToolsResponse{
		Tools: []*evaluatordto.Tool{e.configer.GetEvaluatorToolConf(ctx)[consts.DefaultEvaluatorToolKey]},
	}, nil
}

func (e *EvaluatorHandlerImpl) CheckEvaluatorName(ctx context.Context, request *evaluatorservice.CheckEvaluatorNameRequest) (resp *evaluatorservice.CheckEvaluatorNameResponse, err error) {
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	exist, err := e.evaluatorService.CheckNameExist(ctx, request.GetWorkspaceID(), request.GetEvaluatorID(), request.GetName())
	if err != nil {
		return nil, err
	}
	if exist {
		return &evaluatorservice.CheckEvaluatorNameResponse{
			Pass:    gptr.Of(false),
			Message: gptr.Of(fmt.Sprintf("evaluator_version name %s already exists", request.GetName())),
		}, nil
	}
	return &evaluatorservice.CheckEvaluatorNameResponse{
		Pass: gptr.Of(true),
	}, nil
}

// transformURIsToURLs 将InputFields中的URI转换为URL
func (e *EvaluatorHandlerImpl) transformURIsToURLs(ctx context.Context, inputFields map[string]*evaluatorcommon.Content) error {
	if len(inputFields) == 0 {
		return nil
	}

	// 收集所有需要转换的URI
	uriToContentMap := make(map[string][]*evaluatorcommon.Image)
	e.collectURIs(inputFields, uriToContentMap)

	if len(uriToContentMap) == 0 {
		return nil
	}

	// 批量获取URL
	uris := make([]string, 0, len(uriToContentMap))
	for uri := range uriToContentMap {
		uris = append(uris, uri)
	}

	urlMap, err := e.fileProvider.MGetFileURL(ctx, uris)
	if err != nil {
		return errorx.NewByCode(errno.FileURLRetrieveFailedCode, errorx.WithExtraMsg(err.Error()))
	}

	// 回填URL到原始数据
	e.fillURLs(uriToContentMap, urlMap)

	return nil
}

// collectURIs 递归收集所有需要转换的URI
func (e *EvaluatorHandlerImpl) collectURIs(inputFields map[string]*evaluatorcommon.Content, uriToContentMap map[string][]*evaluatorcommon.Image) {
	for _, content := range inputFields {
		e.collectURIsFromContent(content, uriToContentMap)
	}
}

// collectURIsFromContent 从单个Content中收集URI
func (e *EvaluatorHandlerImpl) collectURIsFromContent(content *evaluatorcommon.Content, uriToContentMap map[string][]*evaluatorcommon.Image) {
	if content == nil {
		return
	}

	switch content.GetContentType() {
	case evaluatorcommon.ContentTypeImage:
		if content.Image != nil && content.Image.URI != nil && *content.Image.URI != "" {
			uri := *content.Image.URI
			uriToContentMap[uri] = append(uriToContentMap[uri], content.Image)
		}
	case evaluatorcommon.ContentTypeMultiPart:
		for _, subContent := range content.MultiPart {
			e.collectURIsFromContent(subContent, uriToContentMap)
		}
	}
}

// fillURLs 将转换后的URL填充回原始数据
func (e *EvaluatorHandlerImpl) fillURLs(uriToContentMap map[string][]*evaluatorcommon.Image, urlMap map[string]string) {
	for uri, images := range uriToContentMap {
		if url, exists := urlMap[uri]; exists {
			for _, image := range images {
				image.URL = &url
			}
		}
	}
}

// ValidateEvaluator 验证评估器
func (e *EvaluatorHandlerImpl) ValidateEvaluator(ctx context.Context, request *evaluatorservice.ValidateEvaluatorRequest) (resp *evaluatorservice.ValidateEvaluatorResponse, err error) {
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("debugLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	// 转换请求参数
	evaluator, err := evaluatorconvertor.ConvertEvaluatorContent2DO(request.EvaluatorContent, request.EvaluatorType)
	if err != nil {
		return &evaluatorservice.ValidateEvaluatorResponse{
			Valid:        gptr.Of(false),
			ErrorMessage: gptr.Of(errorx.ErrorWithoutStack(err)),
		}, nil
	}

	// 设置基本信息
	evaluator.SpaceID = request.WorkspaceID

	// 验证基本信息
	if err := evaluator.ValidateBaseInfo(); err != nil {
		return &evaluatorservice.ValidateEvaluatorResponse{
			Valid:        gptr.Of(false),
			ErrorMessage: gptr.Of(errorx.ErrorWithoutStack(err)),
		}, nil
	}

	// 获取评估器源服务
	evaluatorSourceService, ok := e.evaluatorSourceServices[evaluator.EvaluatorType]
	if !ok {
		return &evaluatorservice.ValidateEvaluatorResponse{
			Valid:        gptr.Of(false),
			ErrorMessage: gptr.Of(fmt.Sprintf("unsupported evaluator type: %d", evaluator.EvaluatorType)),
		}, nil
	}

	// 验证评估器（语法检查等）
	if err := evaluatorSourceService.Validate(ctx, evaluator); err != nil {
		return &evaluatorservice.ValidateEvaluatorResponse{
			Valid:        gptr.Of(false),
			ErrorMessage: gptr.Of(errorx.ErrorWithoutStack(err)),
		}, nil
	}

	// 构造响应
	response := &evaluatorservice.ValidateEvaluatorResponse{
		Valid: gptr.Of(true),
	}

	return response, nil
}

// BatchDebugEvaluator 批量调试评估器
func (e *EvaluatorHandlerImpl) BatchDebugEvaluator(ctx context.Context, request *evaluatorservice.BatchDebugEvaluatorRequest) (resp *evaluatorservice.BatchDebugEvaluatorResponse, err error) {
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("debugLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	userID := session.UserIDInCtxOrEmpty(ctx)

	// 权益检查
	req := &benefit.CheckEvaluatorBenefitParams{
		ConnectorUID: userID,
		SpaceID:      request.GetWorkspaceID(),
	}
	result, err := e.benefitService.CheckEvaluatorBenefit(ctx, req)
	if err != nil {
		return nil, err
	}

	logs.CtxInfo(ctx, "BatchDebugEvaluator CheckEvaluatorBenefit result: %v,", json.Jsonify(result))

	if result != nil && result.DenyReason != nil {
		return nil, errorx.NewByCode(errno.EvaluatorBenefitDenyCode)
	}

	// 批量URI转换处理
	for _, inputData := range request.InputData {
		if inputData != nil {
			err = e.transformURIsToURLs(ctx, inputData.InputFields)
			if err != nil {
				logs.CtxError(ctx, "failed to transform URIs to URLs: %v", err)
				return nil, err
			}
		}
	}

	// 构建评估器对象
	dto := &evaluatordto.Evaluator{
		WorkspaceID:   gptr.Of(request.WorkspaceID),
		EvaluatorType: gptr.Of(request.EvaluatorType),
		CurrentVersion: &evaluatordto.EvaluatorVersion{
			EvaluatorContent: request.EvaluatorContent,
		},
	}
	evaluatorDO, err := evaluatorconvertor.ConvertEvaluatorDTO2DO(dto)
	if err != nil {
		return nil, err
	}

	// 构建运行配置
	evaluatorRunConf := evaluatorconvertor.ConvertEvaluatorRunConfDTO2DO(request.GetEvaluatorRunConf())

	// 并发调试处理
	return e.batchDebugWithConcurrency(ctx, evaluatorDO, request.InputData, evaluatorRunConf, request.WorkspaceID)
}

// batchDebugWithConcurrency 使用并发池进行批量调试
func (e *EvaluatorHandlerImpl) batchDebugWithConcurrency(ctx context.Context, evaluatorDO *entity.Evaluator, inputDataList []*evaluatordto.EvaluatorInputData, evaluatorRunConf *entity.EvaluatorRunConfig, exptSpaceID int64) (*evaluatorservice.BatchDebugEvaluatorResponse, error) {
	// 创建并发池，并发度为10
	pool, err := goroutine.NewPool(10)
	if err != nil {
		return nil, errorx.NewByCode(errno.GoroutinePoolCreateFailedCode, errorx.WithExtraMsg(err.Error()))
	}

	// 初始化结果数组
	results := make([]*evaluatordto.EvaluatorOutputData, len(inputDataList))
	var mutex sync.Mutex

	// 为每个输入数据创建调试任务
	for i, inputData := range inputDataList {
		index := i
		currentInputData := inputData

		pool.Add(func() error {
			// 转换输入数据
			inputDataDO := evaluatorconvertor.ConvertEvaluatorInputDataDTO2DO(currentInputData)
			if evaluatorRunConf != nil && evaluatorRunConf.EvaluatorRuntimeParam != nil && evaluatorRunConf.EvaluatorRuntimeParam.JSONValue != nil {
				if inputDataDO == nil {
					inputDataDO = &entity.EvaluatorInputData{}
				}
				if inputDataDO.Ext == nil {
					inputDataDO.Ext = make(map[string]string)
				}
				inputDataDO.Ext[consts.FieldAdapterBuiltinFieldNameRuntimeParam] = ptr.From(evaluatorRunConf.EvaluatorRuntimeParam.JSONValue)
			}

			// 调用单个评估器调试逻辑
			outputDataDO, debugErr := e.evaluatorService.DebugEvaluator(ctx, evaluatorDO, inputDataDO, evaluatorRunConf, exptSpaceID)

			// 保护结果收集过程
			mutex.Lock()
			defer mutex.Unlock()

			// 首先转换输出数据
			if outputDataDO != nil {
				results[index] = evaluatorconvertor.ConvertEvaluatorOutputDataDO2DTO(outputDataDO)

				// 检查是否需要使用debugErr作为EvaluatorRunError
				if results[index].EvaluatorRunError == nil && debugErr != nil {
					results[index].EvaluatorRunError = &evaluatordto.EvaluatorRunError{
						Code:    gptr.Of(int32(500)),
						Message: gptr.Of(errorx.ErrorWithoutStack(debugErr)),
					}
				}
			} else if debugErr != nil {
				// outputDataDO为nil但有debugErr的情况
				results[index] = &evaluatordto.EvaluatorOutputData{
					EvaluatorRunError: &evaluatordto.EvaluatorRunError{
						Code:    gptr.Of(int32(500)),
						Message: gptr.Of(errorx.ErrorWithoutStack(debugErr)),
					},
				}
			}

			return nil // 总是返回nil，确保单个失败不影响其他任务
		})
	}

	// 执行所有任务，使用ExecAll确保单个失败不影响其他任务
	err = pool.ExecAll(ctx)
	if err != nil {
		return nil, errorx.NewByCode(errno.BatchTaskExecutionFailedCode, errorx.WithExtraMsg(err.Error()))
	}

	return &evaluatorservice.BatchDebugEvaluatorResponse{
		EvaluatorOutputData: results,
	}, nil
}

// ListTemplatesV2 查询评估器模板列表
func (e *EvaluatorHandlerImpl) ListTemplatesV2(ctx context.Context, request *evaluatorservice.ListTemplatesV2Request) (resp *evaluatorservice.ListTemplatesV2Response, err error) {
	// 构建service层请求
	serviceReq := &entity.ListEvaluatorTemplateRequest{
		SpaceID:        0,   // 模板查询不需要空间ID
		FilterOption:   nil, // 默认无筛选条件
		PageSize:       20,  // 默认分页大小
		PageNum:        1,   // 默认页码
		IncludeDeleted: false,
	}

	// 转换FilterOption
	if request.GetFilterOption() != nil {
		serviceReq.FilterOption = evaluatorconvertor.ConvertEvaluatorFilterOptionDTO2DO(request.GetFilterOption())
	}

	// 覆盖分页参数（若请求携带）
	if ps := request.GetPageSize(); ps > 0 {
		serviceReq.PageSize = ps
	}
	if pn := request.GetPageNumber(); pn > 0 {
		serviceReq.PageNum = pn
	}

	// 调用service层
	serviceResp, err := e.evaluatorTemplateService.ListEvaluatorTemplate(ctx, serviceReq)
	if err != nil {
		return nil, err
	}

	// 转换结果
	templates := make([]*evaluatordto.EvaluatorTemplate, 0, len(serviceResp.Templates))
	for _, template := range serviceResp.Templates {
		templates = append(templates, evaluatorconvertor.ConvertEvaluatorTemplateDO2DTO(template))
	}

	return &evaluatorservice.ListTemplatesV2Response{
		EvaluatorTemplates: templates,
		Total:              gptr.Of(serviceResp.TotalCount),
	}, nil
}

// GetTemplateV2 获取评估器模板详情
func (e *EvaluatorHandlerImpl) GetTemplateV2(ctx context.Context, request *evaluatorservice.GetTemplateV2Request) (resp *evaluatorservice.GetTemplateV2Response, err error) {
	// 若请求指定custom_code，则直接返回自定义code评估器模板（包含所有语言），无需查询DB
	if request.GetCustomCode() {
		customTemplates := e.configer.GetCustomCodeEvaluatorTemplateConf(ctx)
		lang2 := make(map[evaluatordto.LanguageType]string)
		if len(customTemplates) > 0 {
			if langMap, ok := customTemplates["custom"]; ok {
				for lang, content := range langMap {
					if content == nil || content.CodeEvaluator == nil || content.CodeEvaluator.CodeContent == nil {
						continue
					}
					lt := lang
					lang2[lt] = content.CodeEvaluator.GetCodeContent()
				}
			}
		}
		template := &evaluatordto.EvaluatorTemplate{
			EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Code),
			EvaluatorContent: &evaluatordto.EvaluatorContent{
				CodeEvaluator: &evaluatordto.CodeEvaluator{Lang2CodeContent: lang2},
			},
		}
		return &evaluatorservice.GetTemplateV2Response{EvaluatorTemplate: template}, nil
	}

	// 构建service层请求
	serviceReq := &entity.GetEvaluatorTemplateRequest{
		ID:             request.GetEvaluatorTemplateID(),
		IncludeDeleted: false,
	}

	// 调用service层
	serviceResp, err := e.evaluatorTemplateService.GetEvaluatorTemplate(ctx, serviceReq)
	if err != nil {
		return nil, err
	}

	if serviceResp.Template == nil {
		return &evaluatorservice.GetTemplateV2Response{}, nil
	}

	// 转换结果
	template := evaluatorconvertor.ConvertEvaluatorTemplateDO2DTO(serviceResp.Template)

	return &evaluatorservice.GetTemplateV2Response{
		EvaluatorTemplate: template,
	}, nil
}

// CreateEvaluatorTemplate 创建评估器模板
func (e *EvaluatorHandlerImpl) CreateEvaluatorTemplate(ctx context.Context, request *evaluatorservice.CreateEvaluatorTemplateRequest) (resp *evaluatorservice.CreateEvaluatorTemplateResponse, err error) {
	// 参数验证
	if request.GetEvaluatorTemplate() == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluator_template is nil"))
	}

	// 校验评估器模板管理权限
	err = e.authBuiltinManagement(ctx, request.GetEvaluatorTemplate().GetWorkspaceID(), spaceTypeTemplate, true)
	if err != nil {
		return nil, err
	}

	// 转换DTO到DO
	templateDO := evaluatorconvertor.ConvertEvaluatorTemplateDTO2DO(request.GetEvaluatorTemplate())

	// 构建service层请求
	serviceReq := &entity.CreateEvaluatorTemplateRequest{
		SpaceID:                templateDO.SpaceID,
		Name:                   templateDO.Name,
		Description:            templateDO.Description,
		EvaluatorType:          templateDO.EvaluatorType,
		EvaluatorInfo:          templateDO.EvaluatorInfo,
		InputSchemas:           templateDO.InputSchemas,
		OutputSchemas:          templateDO.OutputSchemas,
		ReceiveChatHistory:     templateDO.ReceiveChatHistory,
		Tags:                   templateDO.Tags,
		PromptEvaluatorContent: templateDO.PromptEvaluatorContent,
		CodeEvaluatorContent:   templateDO.CodeEvaluatorContent,
	}

	// 调用service层
	serviceResp, err := e.evaluatorTemplateService.CreateEvaluatorTemplate(ctx, serviceReq)
	if err != nil {
		return nil, err
	}

	// 转换结果
	template := evaluatorconvertor.ConvertEvaluatorTemplateDO2DTO(serviceResp.Template)

	return &evaluatorservice.CreateEvaluatorTemplateResponse{
		EvaluatorTemplate: template,
	}, nil
}

// UpdateEvaluatorTemplate 更新评估器模板
func (e *EvaluatorHandlerImpl) UpdateEvaluatorTemplate(ctx context.Context, request *evaluatorservice.UpdateEvaluatorTemplateRequest) (resp *evaluatorservice.UpdateEvaluatorTemplateResponse, err error) {
	// 参数验证
	if request.GetEvaluatorTemplate() == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluator_template is nil"))
	}

	// 校验评估器模板管理权限
	err = e.authBuiltinManagement(ctx, request.GetEvaluatorTemplate().GetWorkspaceID(), spaceTypeTemplate, true)
	if err != nil {
		return nil, err
	}

	// 转换DTO到DO
	templateDO := evaluatorconvertor.ConvertEvaluatorTemplateDTO2DO(request.GetEvaluatorTemplate())

	// 构建service层请求
	serviceReq := &entity.UpdateEvaluatorTemplateRequest{
		ID:                     request.EvaluatorTemplateID,
		Name:                   gptr.Of(templateDO.Name),
		Description:            gptr.Of(templateDO.Description),
		EvaluatorInfo:          templateDO.EvaluatorInfo,
		InputSchemas:           templateDO.InputSchemas,
		OutputSchemas:          templateDO.OutputSchemas,
		ReceiveChatHistory:     templateDO.ReceiveChatHistory,
		Tags:                   templateDO.Tags,
		PromptEvaluatorContent: templateDO.PromptEvaluatorContent,
		CodeEvaluatorContent:   templateDO.CodeEvaluatorContent,
	}

	// 调用service层
	serviceResp, err := e.evaluatorTemplateService.UpdateEvaluatorTemplate(ctx, serviceReq)
	if err != nil {
		return nil, err
	}

	// 转换结果
	template := evaluatorconvertor.ConvertEvaluatorTemplateDO2DTO(serviceResp.Template)

	return &evaluatorservice.UpdateEvaluatorTemplateResponse{
		EvaluatorTemplate: template,
	}, nil
}

// DeleteEvaluatorTemplate 删除评估器模板
func (e *EvaluatorHandlerImpl) DeleteEvaluatorTemplate(ctx context.Context, request *evaluatorservice.DeleteEvaluatorTemplateRequest) (resp *evaluatorservice.DeleteEvaluatorTemplateResponse, err error) {
	// 参数验证
	if request.GetEvaluatorTemplateID() == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluator_template_id is 0"))
	}

	// 鉴权 - 需要先获取模板信息来确定空间ID
	templateDO, err := e.evaluatorTemplateService.GetEvaluatorTemplate(ctx, &entity.GetEvaluatorTemplateRequest{
		ID:             request.GetEvaluatorTemplateID(),
		IncludeDeleted: false,
	})
	if err != nil {
		return nil, err
	}
	if templateDO.Template == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
	}

	// 校验评估器模板管理权限
	err = e.authBuiltinManagement(ctx, templateDO.Template.SpaceID, spaceTypeTemplate, true)
	if err != nil {
		return nil, err
	}

	// 构建service层请求
	serviceReq := &entity.DeleteEvaluatorTemplateRequest{
		ID: request.GetEvaluatorTemplateID(),
	}

	// 调用service层
	_, err = e.evaluatorTemplateService.DeleteEvaluatorTemplate(ctx, serviceReq)
	if err != nil {
		return nil, err
	}

	return &evaluatorservice.DeleteEvaluatorTemplateResponse{}, nil
}

// DebugBuiltinEvaluator 调试预置评估器
func (e *EvaluatorHandlerImpl) DebugBuiltinEvaluator(ctx context.Context, request *evaluatorservice.DebugBuiltinEvaluatorRequest) (resp *evaluatorservice.DebugBuiltinEvaluatorResponse, err error) {
	// 预置评估器复用空间下列表查询鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(request.WorkspaceID, 10),
		SpaceID:       request.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	// 1) 通过 evaluator_id 查询预置评估器（按 builtin_visible_version 组装）
	builtinEvaluatorDO, err := e.evaluatorService.GetBuiltinEvaluator(ctx, request.GetEvaluatorID())
	if err != nil {
		return nil, err
	}
	if builtinEvaluatorDO == nil {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}

	// 2) 调用调试逻辑
	inputDataDO := evaluatorconvertor.ConvertEvaluatorInputDataDTO2DO(request.GetInputData())
	outputDataDO, err := e.evaluatorService.DebugEvaluator(ctx, builtinEvaluatorDO, inputDataDO, nil, request.WorkspaceID) // 预置评估器无运行配置
	if err != nil {
		return nil, err
	}

	// 3) 返回结果
	return &evaluatorservice.DebugBuiltinEvaluatorResponse{
		OutputData: evaluatorconvertor.ConvertEvaluatorOutputDataDO2DTO(outputDataDO),
	}, nil
}

// UpdateBuiltinEvaluatorTags 发布预置评估器
func (e *EvaluatorHandlerImpl) UpdateBuiltinEvaluatorTags(ctx context.Context, request *evaluatorservice.UpdateBuiltinEvaluatorTagsRequest) (resp *evaluatorservice.UpdateBuiltinEvaluatorTagsResponse, err error) {
	// 1) 查 Evaluator 实体，判断是否属于预置评估器管理空间
	evaluatorDO, err := e.evaluatorService.GetEvaluator(ctx, request.GetWorkspaceID(), request.GetEvaluatorID(), false)
	if err != nil {
		return nil, err
	}
	if evaluatorDO == nil {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	// 校验是否在builtin管理空间
	if err := e.authBuiltinManagement(ctx, request.GetWorkspaceID(), spaceTypeBuiltin, true); err != nil {
		return nil, err
	}

	// 2) 调用 service，按 evaluatorID 更新标签（不再使用 version 参数）
	err = e.evaluatorService.UpdateBuiltinEvaluatorTags(ctx, request.GetEvaluatorID(), evaluatorconvertor.ConvertEvaluatorLangTagsDTO2DO(request.GetTags()))
	if err != nil {
		return nil, err
	}

	// 3) 组装更新后的标签并返回 Evaluator（最终标签集合等于请求中的标签集合）
	evaluatorDO.Tags = evaluatorconvertor.ConvertEvaluatorLangTagsDTO2DO(request.GetTags())
	return &evaluatorservice.UpdateBuiltinEvaluatorTagsResponse{
		Evaluator: evaluatorconvertor.ConvertEvaluatorDO2DTO(evaluatorDO),
	}, nil
}

func (e *EvaluatorHandlerImpl) ListEvaluatorTags(ctx context.Context, request *evaluatorservice.ListEvaluatorTagsRequest) (resp *evaluatorservice.ListEvaluatorTagsResponse, err error) {
	tagType := convertListEvaluatorTagType(request.GetTagType())
	tags, err := e.evaluatorService.ListEvaluatorTags(ctx, tagType)
	if err != nil {
		return nil, err
	}
	return &evaluatorservice.ListEvaluatorTagsResponse{
		Tags: convertEvaluatorTagsDO2DTO(tags),
	}, nil
}

func convertListEvaluatorTagType(tagType evaluatordto.EvaluatorTagType) entity.EvaluatorTagKeyType {
	switch tagType {
	case evaluatordto.EvaluatorTagTypeTemplate:
		return entity.EvaluatorTagKeyType_Template
	default:
		return entity.EvaluatorTagKeyType_Evaluator
	}
}

func convertEvaluatorTagsDO2DTO(tags map[entity.EvaluatorTagKey][]string) map[evaluatordto.EvaluatorTagKey][]string {
	if len(tags) == 0 {
		return map[evaluatordto.EvaluatorTagKey][]string{}
	}
	result := make(map[evaluatordto.EvaluatorTagKey][]string, len(tags))
	for key, values := range tags {
		dtoKey := evaluatordto.EvaluatorTagKey(key)
		if len(values) == 0 {
			result[dtoKey] = []string{}
			continue
		}
		copied := make([]string, len(values))
		copy(copied, values)
		result[dtoKey] = copied
	}
	return result
}

type SpaceType string

const (
	spaceTypeBuiltin  SpaceType = "builtin"
	spaceTypeTemplate SpaceType = "template"
)

// validate 校验评估器管理权限
func (e *EvaluatorHandlerImpl) authBuiltinManagement(ctx context.Context, workspaceID int64, spaceType SpaceType, authWrite bool) error {
	if authWrite {
		// 鉴权
		err := e.auth.Authorization(ctx, &rpc.AuthorizationParam{
			ObjectID:      strconv.FormatInt(workspaceID, 10),
			SpaceID:       workspaceID,
			ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}}, // listLoopEvaluator为暂时复用的权限点
		})
		if err != nil {
			return err
		}
	}

	var allowedSpaceIDs []string
	switch spaceType {
	case spaceTypeBuiltin:
		allowedSpaceIDs = e.configer.GetBuiltinEvaluatorSpaceConf(ctx)
	default:
		allowedSpaceIDs = e.configer.GetEvaluatorTemplateSpaceConf(ctx)
	}

	// 如果配置为空，则不允许任何空间ID
	if len(allowedSpaceIDs) == 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("evaluator template space not configured"))
	}

	// 将空间ID转换为字符串进行比较
	workspaceIDStr := strconv.FormatInt(workspaceID, 10)

	// 检查空间ID是否在允许列表中
	for _, allowedID := range allowedSpaceIDs {
		if allowedID == workspaceIDStr {
			return nil
		}
	}

	return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("workspace_id not in allowed evaluator template spaces"))
}

func (e *EvaluatorHandlerImpl) authCustomRPCEvaluatorContentWritable(ctx context.Context, workspaceID int64) error {
	allowedSpaceIDs := e.configer.GetBuiltinEvaluatorSpaceConf(ctx)

	ok, err := e.configer.CheckCustomRPCEvaluatorWritable(ctx, strconv.FormatInt(workspaceID, 10), allowedSpaceIDs)
	if err != nil {
		return err
	}
	if !ok {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("current space does not support custom RPC evaluator"))
	}
	return nil
}
