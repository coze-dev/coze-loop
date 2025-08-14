// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"

	"golang.org/x/sync/errgroup"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/view"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/application/utils"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	commdo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	MaxSpanLength     = 100
	MaxListSpansLimit = 1000
	QueryLimitDefault = 100
)

type ITraceApplication interface {
	trace.TraceService
}

func NewTraceApplication(
	traceService service.ITraceService,
	viewRepo repo.IViewRepo,
	benefitService benefit.IBenefitService,
	traceMetrics metrics.ITraceMetrics,
	traceConfig config.ITraceConfig,
	authService rpc.IAuthProvider,
	evalService rpc.IEvaluatorRPCAdapter,
	userService rpc.IUserProvider,
	tagService rpc.ITagRPCAdapter,
) (ITraceApplication, error) {
	return &TraceApplication{
		traceService: traceService,
		viewRepo:     viewRepo,
		traceConfig:  traceConfig,
		metrics:      traceMetrics,
		benefit:      benefitService,
		authSvc:      authService,
		evalSvc:      evalService,
		userSvc:      userService,
		tagSvc:       tagService,
	}, nil
}

type TraceApplication struct {
	traceService service.ITraceService
	viewRepo     repo.IViewRepo
	traceConfig  config.ITraceConfig
	metrics      metrics.ITraceMetrics
	benefit      benefit.IBenefitService
	authSvc      rpc.IAuthProvider
	evalSvc      rpc.IEvaluatorRPCAdapter
	userSvc      rpc.IUserProvider
	tagSvc       rpc.ITagRPCAdapter
}

func (t *TraceApplication) ListSpans(ctx context.Context, req *trace.ListSpansRequest) (*trace.ListSpansResponse, error) {
	if err := t.validateListSpansReq(ctx, req); err != nil {
		return nil, err
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceRead,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	sReq, err := t.buildListSpansSvcReq(req)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("list spans req is invalid"))
	}
	sResp, err := t.traceService.ListSpans(ctx, sReq)
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "List spans successfully, spans count: %d", len(sResp.Spans))
	userMap, evalMap, tagMap := t.getAnnoDisplayInfo(ctx,
		req.GetWorkspaceID(),
		nil,
		sResp.Spans.GetEvaluatorVersionIDs(),
		sResp.Spans.GetAnnotationTagIDs())
	return &trace.ListSpansResponse{
		Spans:         tconv.SpanListDO2DTO(sResp.Spans, userMap, evalMap, tagMap),
		NextPageToken: sResp.NextPageToken,
		HasMore:       sResp.HasMore,
	}, nil
}

func (t *TraceApplication) validateListSpansReq(ctx context.Context, req *trace.ListSpansRequest) error {
	if req == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	} else if pageSize := req.GetPageSize(); pageSize < 0 || pageSize > MaxListSpansLimit {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid limit"))
	} else if len(req.GetOrderBys()) > 1 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid order by %s"))
	}
	v := utils.DateValidator{
		Start:        req.GetStartTime(),
		End:          req.GetEndTime(),
		EarliestDays: t.traceConfig.GetTraceDataMaxDurationDay(ctx, req.PlatformType),
	}
	newStartTime, newEndTime, err := v.CorrectDate()
	if err != nil {
		return err
	}
	req.SetStartTime(newStartTime)
	req.SetEndTime(newEndTime)
	return nil
}

func (t *TraceApplication) buildListSpansSvcReq(req *trace.ListSpansRequest) (*service.ListSpansReq, error) {
	ret := &service.ListSpansReq{
		WorkspaceID:     req.GetWorkspaceID(),
		StartTime:       req.GetStartTime(),
		EndTime:         req.GetEndTime(),
		Limit:           QueryLimitDefault,
		DescByStartTime: len(req.GetOrderBys()) > 0,
		PageToken:       req.GetPageToken(),
	}
	if req.PageSize != nil {
		ret.Limit = *req.PageSize
	}
	platformType := loop_span.PlatformType(req.GetPlatformType())
	if req.PlatformType == nil {
		platformType = loop_span.PlatformCozeLoop
	}
	ret.PlatformType = platformType
	switch req.GetSpanListType() {
	case common.SpanListTypeRootSpan:
		ret.SpanListType = loop_span.SpanListTypeRootSpan
	case common.SpanListTypeAllSpan:
		ret.SpanListType = loop_span.SpanListTypeAllSpan
	case common.SpanListTypeLlmSpan:
		ret.SpanListType = loop_span.SpanListTypeLLMSpan
	default:
		ret.SpanListType = loop_span.SpanListTypeRootSpan
	}
	if req.Filters != nil {
		ret.Filters = tconv.FilterFieldsDTO2DO(req.Filters)
		if err := ret.Filters.Validate(); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func (t *TraceApplication) GetTrace(ctx context.Context, req *trace.GetTraceRequest) (*trace.GetTraceResponse, error) {
	if err := t.validateGetTraceReq(ctx, req); err != nil {
		return nil, err
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceRead,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	sReq := t.buildGetTraceSvcReq(req)
	sResp, err := t.traceService.GetTrace(ctx, sReq)
	if err != nil {
		return nil, err
	}
	inTokens, outTokens, err := sResp.Spans.Stat(ctx)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	logs.CtxInfo(ctx, "Get trace successfully, spans count %d", len(sResp.Spans))
	userMap, evalMap, tagMap := t.getAnnoDisplayInfo(ctx,
		req.GetWorkspaceID(),
		sResp.Spans.GetUserIDs(),
		sResp.Spans.GetEvaluatorVersionIDs(),
		sResp.Spans.GetAnnotationTagIDs())
	return &trace.GetTraceResponse{
		Spans: tconv.SpanListDO2DTO(sResp.Spans, userMap, evalMap, tagMap),
		TracesAdvanceInfo: &trace.TraceAdvanceInfo{
			TraceID: sResp.TraceId,
			Tokens: &trace.TokenCost{
				Input:  inTokens,
				Output: outTokens,
			},
		},
	}, nil
}

func (t *TraceApplication) validateGetTraceReq(ctx context.Context, req *trace.GetTraceRequest) error {
	if req == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	} else if req.GetTraceID() == "" {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid trace_id"))
	}
	v := utils.DateValidator{
		Start:        req.GetStartTime(),
		End:          req.GetEndTime(),
		EarliestDays: t.traceConfig.GetTraceDataMaxDurationDay(ctx, req.PlatformType),
	}
	newStartTime, newEndTime, err := v.CorrectDate()
	if err != nil {
		return err
	}
	req.SetStartTime(newStartTime)
	req.SetEndTime(newEndTime)
	return nil
}

func (t *TraceApplication) buildGetTraceSvcReq(req *trace.GetTraceRequest) *service.GetTraceReq {
	ret := &service.GetTraceReq{
		WorkspaceID: req.GetWorkspaceID(),
		TraceID:     req.GetTraceID(),
		StartTime:   req.GetStartTime(),
		EndTime:     req.GetEndTime(),
		SpanIDs:     req.GetSpanIds(),
	}
	platformType := loop_span.PlatformType(req.GetPlatformType())
	if req.PlatformType == nil {
		platformType = loop_span.PlatformCozeLoop
	}
	ret.PlatformType = platformType
	return ret
}

func (t *TraceApplication) BatchGetTracesAdvanceInfo(ctx context.Context, req *trace.BatchGetTracesAdvanceInfoRequest) (*trace.BatchGetTracesAdvanceInfoResponse, error) {
	if err := t.validateGetTracesAdvanceInfoReq(ctx, req); err != nil {
		return nil, err
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceRead,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "Batch get traces advance info request: %+v", req)
	sReq := t.buildBatchGetTraceAdvanceInfoSvcReq(req)
	sResp, err := t.traceService.GetTracesAdvanceInfo(ctx, sReq)
	if err != nil {
		return nil, err
	}
	return &trace.BatchGetTracesAdvanceInfoResponse{
		TracesAdvanceInfo: tconv.BatchAdvanceInfoDO2DTO(sResp.Infos),
	}, nil
}

func (t *TraceApplication) validateGetTracesAdvanceInfoReq(ctx context.Context, req *trace.BatchGetTracesAdvanceInfoRequest) error {
	if req == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	} else if len(req.GetTraces()) < 1 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid traces"))
	}
	for _, tReq := range req.Traces {
		if tReq.GetTraceID() == "" {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid trace_id"))
		}
		v := utils.DateValidator{
			Start:        tReq.GetStartTime(),
			End:          tReq.GetEndTime(),
			EarliestDays: t.traceConfig.GetTraceDataMaxDurationDay(ctx, req.PlatformType),
		}
		newStartTime, newEndTime, err := v.CorrectDate()
		if err != nil {
			return err
		}
		tReq.SetStartTime(newStartTime)
		tReq.SetEndTime(newEndTime)
	}
	return nil
}

func (t *TraceApplication) buildBatchGetTraceAdvanceInfoSvcReq(req *trace.BatchGetTracesAdvanceInfoRequest) *service.GetTracesAdvanceInfoReq {
	ret := &service.GetTracesAdvanceInfoReq{
		WorkspaceID: req.GetWorkspaceID(),
		Traces:      make([]*service.TraceQueryParam, len(req.GetTraces())),
	}
	for i, traceInfo := range req.GetTraces() {
		ret.Traces[i] = &service.TraceQueryParam{
			TraceID:   traceInfo.GetTraceID(),
			StartTime: traceInfo.GetStartTime(),
			EndTime:   traceInfo.GetEndTime(),
		}
	}
	platformType := loop_span.PlatformType(req.GetPlatformType())
	if req.PlatformType == nil {
		platformType = loop_span.PlatformCozeLoop
	}
	ret.PlatformType = platformType
	return ret
}

func (t *TraceApplication) IngestTracesInner(ctx context.Context, req *trace.IngestTracesRequest) (r *trace.IngestTracesResponse, err error) {
	if err := t.validateIngestTracesInnerReq(ctx, req); err != nil {
		return nil, err
	}
	// spaceId/UserId
	spansMap := make(map[string]map[string][]*span.InputSpan)
	for _, inputSpan := range req.Spans {
		if inputSpan == nil {
			continue
		}
		spaceId := inputSpan.WorkspaceID
		userId := inputSpan.TagsString[loop_span.SpanFieldUserID]
		if spansMap[spaceId] == nil {
			spansMap[spaceId] = make(map[string][]*span.InputSpan)
		}
		if spansMap[spaceId][userId] == nil {
			spansMap[spaceId][userId] = make([]*span.InputSpan, 0)
		}
		spansMap[spaceId][userId] = append(spansMap[spaceId][userId], inputSpan)
	}
	for spaceID, userIdSpansMap := range spansMap {
		for userId, spans := range userIdSpansMap {
			workspaceId := spaceID
			workSpaceIdNum, err := strconv.ParseInt(workspaceId, 10, 64)
			if err != nil {
				return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
			}
			benefitRes, err := t.benefit.CheckTraceBenefit(ctx, &benefit.CheckTraceBenefitParams{
				ConnectorUID: userId,
				SpaceID:      workSpaceIdNum,
			})
			if err != nil {
				logs.CtxError(ctx, "Fail to check benefit, %v", err)
			}
			if benefitRes == nil {
				benefitRes = &benefit.CheckTraceBenefitResult{
					AccountAvailable: true,
					IsEnough:         true,
					StorageDuration:  3,
					WhichIsEnough:    -1,
				}
			}
			if !benefitRes.IsEnough || !benefitRes.AccountAvailable {
				benefitRes.StorageDuration = 3
				logs.CtxWarn(ctx, "check benefit err: resource not enough")
			}
			spans := tconv.SpanListDTO2DO(spans)
			for _, s := range spans {
				callType, ok := s.TagsString[loop_span.SpanFieldCallType]
				if ok {
					s.CallType = callType
					delete(s.TagsString, loop_span.SpanFieldCallType)
				}
			}
			if err := t.traceService.IngestTraces(ctx, &service.IngestTracesReq{
				TTL:              loop_span.TTLFromInteger(benefitRes.StorageDuration),
				WhichIsEnough:    benefitRes.WhichIsEnough,
				CozeAccountId:    userId,
				VolcanoAccountID: benefitRes.VolcanoAccountID,
				Spans:            spans,
			}); err != nil {
				return nil, err
			}
		}
	}
	return trace.NewIngestTracesResponse(), nil
}

func (t *TraceApplication) validateIngestTracesInnerReq(ctx context.Context, req *trace.IngestTracesRequest) error {
	if req == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if len(req.Spans) > MaxSpanLength {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("max span length exceeded"))
	} else if len(req.Spans) < 1 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no spans provided"))
	}
	return nil
}

func (t *TraceApplication) GetTracesMetaInfo(ctx context.Context, req *trace.GetTracesMetaInfoRequest) (*trace.GetTracesMetaInfoResponse, error) {
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceRead,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "Get traces meta info request: %+v", req)
	sReq := t.buildGetTracesMetaInfoReq(req)
	sResp, err := t.traceService.GetTracesMetaInfo(ctx, sReq)
	if err != nil {
		return nil, err
	}
	fMeta := make(map[string]*trace.FieldMeta)
	for k, v := range sResp.FilesMetas {
		fMeta[k] = &trace.FieldMeta{
			ValueType:                 filter.FieldType(v.FieldType),
			SupportCustomizableOption: ptr.Of(v.SupportCustom),
		}
		if v.FieldOptions != nil {
			fMeta[k].FieldOptions = &filter.FieldOptions{
				I64List:    v.FieldOptions.I64List,
				F64List:    v.FieldOptions.F64List,
				StringList: v.FieldOptions.StringList,
			}
		}
		fTypes := make([]filter.FieldType, 0)
		for _, t := range v.FilterTypes {
			fTypes = append(fTypes, filter.FieldType(t))
		}
		fMeta[k].FilterTypes = fTypes
	}
	return &trace.GetTracesMetaInfoResponse{
		FieldMetas: fMeta,
	}, nil
}

func (t *TraceApplication) buildGetTracesMetaInfoReq(req *trace.GetTracesMetaInfoRequest) *service.GetTracesMetaInfoReq {
	ret := &service.GetTracesMetaInfoReq{
		WorkspaceID: req.GetWorkspaceID(),
	}
	platformType := loop_span.PlatformType(req.GetPlatformType())
	if req.PlatformType == nil {
		platformType = loop_span.PlatformCozeLoop
	}
	ret.PlatformType = platformType
	switch req.GetSpanListType() {
	case common.SpanListTypeRootSpan:
		ret.SpanListType = loop_span.SpanListTypeRootSpan
	case common.SpanListTypeAllSpan:
		ret.SpanListType = loop_span.SpanListTypeAllSpan
	case common.SpanListTypeLlmSpan:
		ret.SpanListType = loop_span.SpanListTypeLLMSpan
	default:
		ret.SpanListType = loop_span.SpanListTypeRootSpan
	}
	return ret
}

func (t *TraceApplication) CreateView(ctx context.Context, req *trace.CreateViewRequest) (*trace.CreateViewResponse, error) {
	if req == nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	} else if req.ViewName == "" {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid view_name"))
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceViewCreate,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}
	viewPO := tconv.CreateViewDTO2PO(req, userID)
	id, err := t.viewRepo.CreateView(ctx, viewPO)
	if err != nil {
		return nil, err
	}
	return &trace.CreateViewResponse{
		ID: id,
	}, nil
}

func (t *TraceApplication) UpdateView(ctx context.Context, req *trace.UpdateViewRequest) (*trace.UpdateViewResponse, error) {
	if req == nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	if err := t.authSvc.CheckViewPermission(ctx,
		rpc.AuthActionTraceViewEdit,
		strconv.FormatInt(req.GetWorkspaceID(), 10),
		strconv.FormatInt(req.GetID(), 10)); err != nil {
		return nil, err
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}
	viewDo, err := t.viewRepo.GetView(ctx, req.GetID(), ptr.Of(req.GetWorkspaceID()), ptr.Of(userID))
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "Get original view %v", *viewDo)
	if req.ViewName != nil {
		viewDo.ViewName = *req.ViewName
	}
	if req.Filters != nil {
		viewDo.Filters = *req.Filters
	}
	if req.PlatformType != nil {
		viewDo.PlatformType = *req.PlatformType
	}
	if req.SpanListType != nil {
		viewDo.SpanListType = *req.SpanListType
	}
	logs.CtxInfo(ctx, "Update view %d into %v", req.GetID(), *viewDo)
	if err := t.viewRepo.UpdateView(ctx, viewDo); err != nil {
		return nil, err
	}
	return trace.NewUpdateViewResponse(), nil
}

func (t *TraceApplication) DeleteView(ctx context.Context, req *trace.DeleteViewRequest) (*trace.DeleteViewResponse, error) {
	if req == nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetID() <= 0 || req.GetWorkspaceID() <= 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	if err := t.authSvc.CheckViewPermission(ctx,
		rpc.AuthActionTraceViewEdit,
		strconv.FormatInt(req.GetWorkspaceID(), 10),
		strconv.FormatInt(req.GetID(), 10)); err != nil {
		return nil, err
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}
	logs.CtxInfo(ctx, "Delete view %d at %d by %s", req.GetID(), req.GetWorkspaceID(), userID)
	if err := t.viewRepo.DeleteView(ctx, req.GetID(), req.GetWorkspaceID(), userID); err != nil {
		return nil, err
	}
	return trace.NewDeleteViewResponse(), nil
}

func (t *TraceApplication) ListViews(ctx context.Context, req *trace.ListViewsRequest) (*trace.ListViewsResponse, error) {
	if req == nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceViewList,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	systemViews, err := t.getSystemViews(ctx)
	if err != nil {
		return nil, err
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}
	logs.CtxInfo(ctx, "List views for %s at %d", userID, req.GetWorkspaceID())
	viewList, err := t.viewRepo.ListViews(ctx, req.WorkspaceID, userID)
	if err != nil {
		return nil, err
	}
	return &trace.ListViewsResponse{
		Views:    append(systemViews, tconv.BatchViewPO2DTO(viewList)...),
		BaseResp: nil,
	}, nil
}

func (t *TraceApplication) getSystemViews(ctx context.Context) ([]*view.View, error) {
	systemViews, err := t.traceConfig.GetSystemViews(ctx)
	if err != nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode, errorx.WithExtraMsg("get system views failed"))
	}
	ret := make([]*view.View, 0)
	for _, v := range systemViews {
		ret = append(ret, &view.View{
			ID:           v.ID,
			ViewName:     v.ViewName,
			Filters:      v.Filters,
			PlatformType: ptr.Of(common.PlatformTypeCozeloop),
			SpanListType: ptr.Of(common.SpanListTypeRootSpan),
			IsSystem:     true,
		})
	}
	return ret, nil
}

func (t *TraceApplication) CreateManualAnnotation(ctx context.Context, req *trace.CreateManualAnnotationRequest) (*trace.CreateManualAnnotationResponse, error) {
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionAnnotationCreate,
		req.GetAnnotation().GetWorkspaceID()); err != nil {
		return nil, err
	}
	platformType := loop_span.PlatformType(req.GetPlatformType())
	if req.PlatformType == nil {
		platformType = loop_span.PlatformCozeLoop
	}
	annotation, err := tconv.AnnotationDTO2DO(req.Annotation)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	workspaceId, err := strconv.ParseInt(annotation.WorkspaceID, 10, 64)
	if err != nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	tagInfo, err := t.tagSvc.GetTagInfo(ctx, workspaceId, annotation.Key)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode)
	} else if err = tagInfo.CheckAnnotation(annotation); err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	resp, err := t.traceService.CreateManualAnnotation(ctx, &service.CreateManualAnnotationReq{
		PlatformType: platformType,
		Annotation:   annotation,
	})
	if err != nil {
		return nil, err
	}
	return &trace.CreateManualAnnotationResponse{
		AnnotationID: ptr.Of(resp.AnnotationID),
	}, nil
}

func (t *TraceApplication) UpdateManualAnnotation(ctx context.Context, req *trace.UpdateManualAnnotationRequest) (*trace.UpdateManualAnnotationResponse, error) {
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionAnnotationCreate,
		req.GetAnnotation().GetWorkspaceID()); err != nil {
		return nil, err
	}
	platformType := loop_span.PlatformType(req.GetPlatformType())
	if req.PlatformType == nil {
		platformType = loop_span.PlatformCozeLoop
	}
	annotation, err := tconv.AnnotationDTO2DO(req.Annotation)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	workspaceId, err := strconv.ParseInt(annotation.WorkspaceID, 10, 64)
	if err != nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	tagInfo, err := t.tagSvc.GetTagInfo(ctx, workspaceId, annotation.Key)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode)
	} else if err = tagInfo.CheckAnnotation(annotation); err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	err = t.traceService.UpdateManualAnnotation(ctx, &service.UpdateManualAnnotationReq{
		AnnotationID: req.AnnotationID,
		PlatformType: platformType,
		Annotation:   annotation,
	})
	if err != nil {
		return nil, err
	}
	return &trace.UpdateManualAnnotationResponse{}, nil
}

func (t *TraceApplication) DeleteManualAnnotation(ctx context.Context, req *trace.DeleteManualAnnotationRequest) (*trace.DeleteManualAnnotationResponse, error) {
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionAnnotationCreate,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	platformType := loop_span.PlatformType(req.GetPlatformType())
	if req.PlatformType == nil {
		platformType = loop_span.PlatformCozeLoop
	}
	if _, err := t.tagSvc.GetTagInfo(ctx, req.WorkspaceID, req.AnnotationKey); err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	err := t.traceService.DeleteManualAnnotation(ctx, &service.DeleteManualAnnotationReq{
		AnnotationID:  req.AnnotationID,
		WorkspaceID:   req.WorkspaceID,
		TraceID:       req.TraceID,
		SpanID:        req.SpanID,
		StartTime:     req.StartTime,
		AnnotationKey: req.AnnotationKey,
		PlatformType:  platformType,
	})
	if err != nil {
		return nil, err
	}
	return &trace.DeleteManualAnnotationResponse{}, nil
}

func (t *TraceApplication) ListAnnotations(ctx context.Context, req *trace.ListAnnotationsRequest) (*trace.ListAnnotationsResponse, error) {
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceRead,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	platformType := loop_span.PlatformType(req.GetPlatformType())
	if req.PlatformType == nil {
		platformType = loop_span.PlatformCozeLoop
	}
	resp, err := t.traceService.ListAnnotations(ctx, &service.ListAnnotationsReq{
		WorkspaceID:     req.WorkspaceID,
		SpanID:          req.SpanID,
		TraceID:         req.TraceID,
		StartTime:       req.StartTime,
		DescByUpdatedAt: ptr.From(req.DescByUpdatedAt),
		PlatformType:    platformType,
	})
	if err != nil {
		return nil, err
	}
	userMap, evalMap, tagMap := t.getAnnoDisplayInfo(ctx,
		req.GetWorkspaceID(),
		resp.Annotations.GetUserIDs(),
		resp.Annotations.GetEvaluatorVersionIDs(),
		resp.Annotations.GetAnnotationTagIDs())
	return &trace.ListAnnotationsResponse{
		Annotations: tconv.AnnotationListDO2DTO(resp.Annotations, userMap, evalMap, tagMap),
	}, nil
}

func (t *TraceApplication) getAnnoDisplayInfo(ctx context.Context, workspaceId int64, userIds []string, evalIds []int64, tagKeyIds []string,
) (userMap map[string]*commdo.UserInfo, evalMap map[int64]*rpc.Evaluator, tagMap map[int64]*rpc.TagInfo) {
	if len(userIds) == 0 && len(tagKeyIds) == 0 && len(evalIds) == 0 {
		return
	}
	g := errgroup.Group{}
	g.Go(func() error {
		defer goroutine.Recovery(ctx)
		_, userMap, _ = t.userSvc.GetUserInfo(ctx, userIds)
		return nil
	})
	g.Go(func() error {
		defer goroutine.Recovery(ctx)
		_, evalMap, _ = t.evalSvc.BatchGetEvaluatorVersions(ctx, &rpc.BatchGetEvaluatorVersionsParam{
			WorkspaceID:         workspaceId,
			EvaluatorVersionIds: evalIds,
		})
		return nil
	})
	g.Go(func() error {
		defer goroutine.Recovery(ctx)
		tagMap, _ = t.tagSvc.BatchGetTagInfo(ctx, workspaceId, tagKeyIds)
		return nil
	})
	_ = g.Wait()
	return
}
