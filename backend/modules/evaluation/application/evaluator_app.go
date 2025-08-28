// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
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
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// NewEvaluatorHandlerImpl 创建 EvaluatorService 实例
func NewEvaluatorHandlerImpl(idgen idgen.IIDGenerator,
	configer conf.IConfiger,
	auth rpc.IAuthProvider,
	evaluatorService service.EvaluatorService,
	evaluatorRecordService service.EvaluatorRecordService,
	metrics metrics.EvaluatorExecMetrics,
	userInfoService userinfo.UserInfoService,
	auditClient audit.IAuditService,
	benefitService benefit.IBenefitService,
	fileProvider rpc.IFileProvider,
) evaluation.EvaluatorService {
	handler := &EvaluatorHandlerImpl{
		idgen:                  idgen,
		auth:                   auth,
		auditClient:            auditClient,
		configer:               configer,
		evaluatorService:       evaluatorService,
		evaluatorRecordService: evaluatorRecordService,
		metrics:                metrics,
		userInfoService:        userInfoService,
		benefitService:         benefitService,
		fileProvider:           fileProvider,
	}
	return handler
}

// EvaluatorHandlerImpl 实现 EvaluatorService 接口
type EvaluatorHandlerImpl struct {
	idgen                  idgen.IIDGenerator
	auth                   rpc.IAuthProvider
	auditClient            audit.IAuditService
	configer               conf.IConfiger
	evaluatorService       service.EvaluatorService
	evaluatorRecordService service.EvaluatorRecordService
	metrics                metrics.EvaluatorExecMetrics
	userInfoService        userinfo.UserInfoService
	benefitService         benefit.IBenefitService
	fileProvider           rpc.IFileProvider
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
	evaluatorDOS, total, err := e.evaluatorService.ListEvaluator(ctx, buildSrvListEvaluatorRequest(request))
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
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(drafts[0].SpaceID, 10),
		SpaceID:       drafts[0].SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
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
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(draft.ID, 10),
		SpaceID:       draft.SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
	})
	if err != nil {
		return nil, err
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
	defer func() {
		e.metrics.EmitCreate(request.GetEvaluator().GetWorkspaceID(), err)
	}()
	// 转换请求参数为领域对象
	evaluatorDO := evaluatorconvertor.ConvertEvaluatorDTO2DO(request.GetEvaluator())
	evaluatorID, err := e.evaluatorService.CreateEvaluator(ctx, evaluatorDO, request.GetCid())
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
	// 机审
	auditTexts := make([]string, 0)
	auditTexts = append(auditTexts, request.GetName())
	auditTexts = append(auditTexts, request.GetDescription())
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
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	if err = e.evaluatorService.UpdateEvaluatorMeta(ctx, request.GetEvaluatorID(), request.GetWorkspaceID(), request.GetName(), request.GetDescription(), userIDInContext); err != nil {
		return nil, err
	}
	return &evaluatorservice.UpdateEvaluatorResponse{}, nil
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
	evaluatorDTO := evaluatorconvertor.ConvertEvaluatorDO2DTO(evaluatorDO)
	evaluatorDTO.CurrentVersion.EvaluatorContent = request.EvaluatorContent
	evaluatorDTO.DraftSubmitted = ptr.Of(false)
	err = e.evaluatorService.UpdateEvaluatorDraft(ctx, evaluatorconvertor.ConvertEvaluatorDTO2DO(evaluatorDTO))
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
	evaluatorDO, err := e.evaluatorService.GetEvaluatorVersion(ctx, request.GetEvaluatorVersionID(), request.GetIncludeDeleted())
	if err != nil {
		return nil, err
	}
	if evaluatorDO == nil {
		return &evaluatorservice.GetEvaluatorVersionResponse{}, nil
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(evaluatorDO.ID, 10),
		SpaceID:       evaluatorDO.SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
	})
	if err != nil {
		return nil, err
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
	evaluatorDOList, err := e.evaluatorService.BatchGetEvaluatorVersion(ctx, ptr.Of(request.WorkspaceID), request.GetEvaluatorVersionIds(), request.GetIncludeDeleted())
	if err != nil {
		return nil, err
	}
	if len(evaluatorDOList) == 0 {
		return &evaluatorservice.BatchGetEvaluatorVersionsResponse{}, nil
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(evaluatorDOList[0].SpaceID, 10),
		SpaceID:       evaluatorDOList[0].SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of("listLoopEvaluator"), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
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
	builtinTemplates := e.configer.GetEvaluatorTemplateConf(ctx)[strings.ToLower(request.GetBuiltinTemplateType().String())]
	return &evaluatorservice.ListTemplatesResponse{
		BuiltinTemplateKeys: buildTemplateKeys(builtinTemplates),
	}, nil
}

func buildTemplateKeys(origins map[string]*evaluatordto.EvaluatorContent) []*evaluatordto.EvaluatorContent {
	keys := make([]*evaluatordto.EvaluatorContent, 0, len(origins))
	for _, origin := range origins {
		keys = append(keys, &evaluatordto.EvaluatorContent{
			PromptEvaluator: &evaluatordto.PromptEvaluator{
				PromptTemplateKey:  origin.GetPromptEvaluator().PromptTemplateKey,
				PromptTemplateName: origin.GetPromptEvaluator().PromptTemplateName,
			},
		})
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].GetPromptEvaluator().GetPromptTemplateKey() < keys[j].GetPromptEvaluator().GetPromptTemplateKey()
	})
	return keys
}

// GetEvaluatorTemplate 按 key 单个查询内置评估器模板详情
func (e *EvaluatorHandlerImpl) GetTemplateInfo(ctx context.Context, request *evaluatorservice.GetTemplateInfoRequest) (resp *evaluatorservice.GetTemplateInfoResponse, err error) {
	if template, ok := e.configer.GetEvaluatorTemplateConf(ctx)[strings.ToLower(request.GetBuiltinTemplateType().String())][request.GetBuiltinTemplateKey()]; !ok {
		return nil, errorx.NewByCode(errno.TemplateNotFoundCode, errorx.WithExtraMsg("builtin template not found"))
	} else {
		return &evaluatorservice.GetTemplateInfoResponse{
			EvaluatorContent: template,
		}, nil
	}
}

// RunEvaluator evaluator_version 运行
func (e *EvaluatorHandlerImpl) RunEvaluator(ctx context.Context, request *evaluatorservice.RunEvaluatorRequest) (resp *evaluatorservice.RunEvaluatorResponse, err error) {
	evaluatorDO, err := e.evaluatorService.GetEvaluatorVersion(ctx, request.GetEvaluatorVersionID(), false)
	if err != nil {
		return nil, err
	}
	if evaluatorDO == nil {
		return nil, errorx.NewByCode(errno.EvaluatorNotExistCode)
	}
	// 鉴权
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(evaluatorDO.ID, 10),
		SpaceID:       evaluatorDO.SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Run), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
	})
	if err != nil {
		return nil, err
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
	}
	inputData := evaluatorconvertor.ConvertEvaluatorInputDataDTO2DO(request.GetInputData())
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
	do := evaluatorconvertor.ConvertEvaluatorDTO2DO(dto)
	inputData := evaluatorconvertor.ConvertEvaluatorInputDataDTO2DO(request.GetInputData())
	outputData, err := e.evaluatorService.DebugEvaluator(ctx, do, inputData)
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
	evaluatorDO, err := e.evaluatorService.GetEvaluatorVersion(ctx, evaluatorRecord.EvaluatorVersionID, false)
	if err != nil {
		return nil, err
	}
	if evaluatorDO == nil {
		return &evaluatorservice.UpdateEvaluatorRecordResponse{}, nil
	}
	err = e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(evaluatorDO.ID, 10),
		SpaceID:       evaluatorDO.SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Edit), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
	})
	if err != nil {
		return nil, err
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
	evaluatorDO, err := e.evaluatorService.GetEvaluatorVersion(ctx, evaluatorRecord.EvaluatorVersionID, request.GetIncludeDeleted())
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
		return fmt.Errorf("failed to get file URLs: %w", err)
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
