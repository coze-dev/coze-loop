// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/collector"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	traced "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/trace"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics"

	"github.com/coze-dev/coze-loop/backend/modules/observability/application/utils"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/workspace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/otel"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/openapi"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type IAnnotationQueueConsumer interface {
	Send(context.Context, *entity.AnnotationEvent) error
}

type IObservabilityOpenAPIApplication interface {
	openapi.OpenAPIService
	IAnnotationQueueConsumer
}

func NewOpenAPIApplication(
	traceService service.ITraceService,
	auth rpc.IAuthProvider,
	benefit benefit.IBenefitService,
	tenant tenant.ITenantProvider,
	workspace workspace.IWorkSpaceProvider,
	rateLimiter limiter.IRateLimiterFactory,
	traceConfig config.ITraceConfig,
	metrics metrics.ITraceMetrics,
	collector collector.ICollectorProvider,
) (IObservabilityOpenAPIApplication, error) {
	return &OpenAPIApplication{
		traceService: traceService,
		auth:         auth,
		benefit:      benefit,
		tenant:       tenant,
		workspace:    workspace,
		rateLimiter:  rateLimiter.NewRateLimiter(),
		traceConfig:  traceConfig,
		metrics:      metrics,
		collector:    collector,
	}, nil
}

type OpenAPIApplication struct {
	traceService service.ITraceService
	auth         rpc.IAuthProvider
	benefit      benefit.IBenefitService
	tenant       tenant.ITenantProvider
	workspace    workspace.IWorkSpaceProvider
	rateLimiter  limiter.IRateLimiter
	traceConfig  config.ITraceConfig
	metrics      metrics.ITraceMetrics
	collector    collector.ICollectorProvider
}

func (o *OpenAPIApplication) IngestTraces(ctx context.Context, req *openapi.IngestTracesRequest) (*openapi.IngestTracesResponse, error) {
	if err := o.validateIngestTracesReq(ctx, req); err != nil {
		return nil, err
	}
	// unpack
	spanMap := o.unpackSpace(ctx, req.Spans)
	connectorUid := session.UserIDInCtxOrEmpty(ctx)
	for workspaceId := range spanMap {
		workSpaceIdNum, err := strconv.ParseInt(workspaceId, 10, 64)
		if err != nil {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
		}
		// check permission
		if err = o.auth.CheckIngestPermission(ctx, workspaceId); err != nil {
			return nil, err
		}
		// check benefit
		benefitRes, err := o.benefit.CheckTraceBenefit(ctx, &benefit.CheckTraceBenefitParams{
			ConnectorUID: connectorUid,
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
		if !benefitRes.IsEnough {
			return nil, errorx.NewByCode(obErrorx.TraceNoCapacityAvailableErrorCode)
		} else if !benefitRes.AccountAvailable {
			return nil, errorx.NewByCode(obErrorx.AccountNotAvailableErrorCode)
		}

		spans := tconv.SpanListDTO2DO(spanMap[workspaceId])
		for i := range spans {
			spans[i].CallType = "Custom"
		}
		tenantSpanMap := o.unpackTenant(ctx, spans)
		for ingestTenant := range tenantSpanMap {
			if err = o.validateIngestTracesReqByTenant(ctx, ingestTenant, req); err != nil {
				return nil, err
			}
			if err = o.traceService.IngestTraces(ctx, &service.IngestTracesReq{
				Tenant:           ingestTenant,
				TTL:              loop_span.TTLFromInteger(benefitRes.StorageDuration),
				WhichIsEnough:    benefitRes.WhichIsEnough,
				CozeAccountId:    connectorUid,
				VolcanoAccountID: benefitRes.VolcanoAccountID,
				Spans:            tenantSpanMap[ingestTenant],
			}); err != nil {
				return nil, err
			}
		}
	}
	return openapi.NewIngestTracesResponse(), nil
}

func (o *OpenAPIApplication) unpackSpace(ctx context.Context, spans []*span.InputSpan) map[string][]*span.InputSpan {
	if spans == nil {
		return nil
	}
	spansMap := make(map[string][]*span.InputSpan)
	for i := range spans {
		workspaceID := o.workspace.GetIngestWorkSpaceID(ctx, []*span.InputSpan{spans[i]})
		if workspaceID == "" {
			continue
		}
		spans[i].WorkspaceID = workspaceID
		if spansMap[workspaceID] == nil {
			spansMap[workspaceID] = make([]*span.InputSpan, 0)
		}
		spansMap[workspaceID] = append(spansMap[workspaceID], spans[i])
	}
	return spansMap
}

func (o *OpenAPIApplication) unpackTenant(ctx context.Context, spans []*loop_span.Span) map[string][]*loop_span.Span {
	if spans == nil {
		return nil
	}
	spansMap := make(map[string][]*loop_span.Span)
	for i := range spans {
		ingestTenant := o.tenant.GetIngestTenant(ctx, []*loop_span.Span{spans[i]})
		if spansMap[ingestTenant] == nil {
			spansMap[ingestTenant] = make([]*loop_span.Span, 0)
		}
		spansMap[ingestTenant] = append(spansMap[ingestTenant], spans[i])
	}
	return spansMap
}

func (o *OpenAPIApplication) validateIngestTracesReq(ctx context.Context, req *openapi.IngestTracesRequest) error {
	if req == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if len(req.Spans) < 1 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no spans provided"))
	}
	workspaceId := req.Spans[0].WorkspaceID
	for i := 1; i < len(req.Spans); i++ {
		if req.Spans[i].WorkspaceID != workspaceId {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("spans space id is not the same"))
		}
	}
	return nil
}

func (o *OpenAPIApplication) validateIngestTracesReqByTenant(ctx context.Context, tenant string, req *openapi.IngestTracesRequest) error {
	tenantIngestConfig, err := o.traceConfig.GetTraceIngestTenantProducerCfg(ctx)
	if err != nil {
		logs.CtxWarn(ctx, "get tenantIngestConfig failed")
		return nil
	}
	maxSpanLength := MaxSpanLength
	if cfg := tenantIngestConfig[tenant]; cfg != nil {
		maxSpanLength = cfg.MaxSpanLength
	}

	if req == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if len(req.Spans) > maxSpanLength {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("max span length exceeded"))
	}
	return nil
}

func (o *OpenAPIApplication) OtelIngestTraces(ctx context.Context, req *openapi.OtelIngestTracesRequest) (*openapi.OtelIngestTracesResponse, error) {
	if err := o.validateOtelIngestTracesReq(ctx, req); err != nil {
		return nil, err
	}
	spanSrc, err := ungzip(req.ContentEncoding, req.Body)
	if err != nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonBadRequestCodeCode, errorx.WithExtraMsg("ungzip span failed"))
	}
	reqSpanProto, err := unmarshalOtelSpan(spanSrc, req.ContentType)
	if err != nil {
		return nil, err
	}
	spansMap := unpackSpace(req.WorkspaceID, reqSpanProto)
	partialFailSpanNumber := 0
	partialErrMessage := ""
	for workspaceId, otelSpans := range spansMap {
		workSpaceIdNum, e := strconv.ParseInt(workspaceId, 10, 64)
		if e != nil {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
		}
		if e = o.auth.CheckIngestPermission(ctx, workspaceId); e != nil {
			return nil, e
		}
		connectorUid := session.UserIDInCtxOrEmpty(ctx)
		benefitRes, e := o.benefit.CheckTraceBenefit(ctx, &benefit.CheckTraceBenefitParams{
			ConnectorUID: connectorUid,
			SpaceID:      workSpaceIdNum,
		})
		if e != nil {
			logs.CtxError(ctx, "Fail to check benefit, %v", e)
		}
		if benefitRes == nil {
			benefitRes = &benefit.CheckTraceBenefitResult{
				AccountAvailable: true,
				IsEnough:         true,
				StorageDuration:  3,
				WhichIsEnough:    -1,
			}
		}
		if !benefitRes.IsEnough {
			return nil, errorx.NewByCode(obErrorx.TraceNoCapacityAvailableErrorCode)
		} else if !benefitRes.AccountAvailable {
			return nil, errorx.NewByCode(obErrorx.AccountNotAvailableErrorCode)
		}

		spans := otel.OtelSpansConvertToSendSpans(ctx, workspaceId, otelSpans)

		tenantSpanMap := o.unpackTenant(ctx, spans)
		for ingestTenant := range tenantSpanMap {
			if e = o.traceService.IngestTraces(ctx, &service.IngestTracesReq{
				Tenant:           ingestTenant,
				TTL:              loop_span.TTLFromInteger(benefitRes.StorageDuration),
				WhichIsEnough:    benefitRes.WhichIsEnough,
				CozeAccountId:    connectorUid,
				VolcanoAccountID: benefitRes.VolcanoAccountID,
				Spans:            tenantSpanMap[ingestTenant],
			}); e != nil {
				logs.CtxError(ctx, "IngestTraces err: %v", e)
				partialFailSpanNumber += len(tenantSpanMap[ingestTenant])
				partialErrMessage = fmt.Sprintf("SendTraceInner err: %v", e)
				continue
			}
		}
	}
	respSpanProto := &coltracepb.ExportTraceServiceResponse{
		PartialSuccess: &coltracepb.ExportTracePartialSuccess{
			RejectedSpans: int64(partialFailSpanNumber),
			ErrorMessage:  partialErrMessage,
		},
	}
	rawResp, err := proto.Marshal(respSpanProto)
	if err != nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode, errorx.WithExtraMsg("proto Marshal err"))
	}
	return &openapi.OtelIngestTracesResponse{
		Body:        rawResp,
		ContentType: gptr.Of(otel.ContentTypeProtoBuf),
	}, nil
}

func (o *OpenAPIApplication) validateOtelIngestTracesReq(ctx context.Context, req *openapi.OtelIngestTracesRequest) error {
	if req == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if len(req.Body) == 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("req body is nil"))
	}
	if !strings.Contains(req.ContentType, otel.ContentTypeJson) && !strings.Contains(req.ContentType, otel.ContentTypeProtoBuf) {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("contentType is invalid"))
	}
	return nil
}

func ungzip(contentEncoding string, data []byte) ([]byte, error) {
	if !strings.Contains(contentEncoding, "gzip") {
		return data, nil
	}
	reader := bytes.NewReader(data)

	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = gzipReader.Close()
	}()

	var uncompressedData bytes.Buffer
	_, err = io.Copy(&uncompressedData, gzipReader)
	if err != nil {
		return nil, err
	}

	return uncompressedData.Bytes(), nil
}

func unpackSpace(outerSpaceID string, reqSpanProto *otel.ExportTraceServiceRequest) map[string][]*otel.ResourceScopeSpan {
	if reqSpanProto == nil {
		return nil
	}
	spansMap := make(map[string][]*otel.ResourceScopeSpan)
	for _, resourceSpans := range reqSpanProto.ResourceSpans {
		for _, scopeSpans := range resourceSpans.ScopeSpans {
			for _, span := range scopeSpans.Spans {
				spaceID := ""
				for _, attribute := range span.Attributes {
					if attribute.Key == otel.OtelAttributeWorkSpaceID {
						spaceID = attribute.Value.GetStringValue()
						break
					}
				}
				if spaceID == "" {
					spaceID = outerSpaceID
				}
				if spansMap[spaceID] == nil {
					spansMap[spaceID] = make([]*otel.ResourceScopeSpan, 0)
				}
				spansMap[spaceID] = append(spansMap[spaceID], &otel.ResourceScopeSpan{
					Resource: resourceSpans.Resource,
					Scope:    scopeSpans.Scope,
					Span:     span,
				})

			}
		}
	}

	return spansMap
}

func unmarshalOtelSpan(spanSrc []byte, contentType string) (*otel.ExportTraceServiceRequest, error) {
	finalResult := &otel.ExportTraceServiceRequest{}
	if strings.Contains(contentType, otel.ContentTypeProtoBuf) {
		tempReq := &coltracepb.ExportTraceServiceRequest{}
		if err := proto.Unmarshal(spanSrc, tempReq); err != nil {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode, errorx.WithExtraMsg("proto Unmarshal err"))
		}
		finalResult = otel.OtelTraceRequestPbToJson(tempReq)
	} else if strings.Contains(contentType, otel.ContentTypeJson) {
		if err := sonic.Unmarshal(spanSrc, finalResult); err != nil {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode, errorx.WithExtraMsg("json Unmarshal err"))
		}
	} else {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg(fmt.Sprintf("unsupported content type: %s", contentType)))
	}

	return finalResult, nil
}

func (o *OpenAPIApplication) CreateAnnotation(ctx context.Context, req *openapi.CreateAnnotationRequest) (*openapi.CreateAnnotationResponse, error) {
	var val loop_span.AnnotationValue
	switch loop_span.AnnotationValueType(req.GetAnnotationValueType()) {
	case loop_span.AnnotationValueTypeLong:
		i, err := strconv.ParseInt(req.AnnotationValue, 10, 64)
		if err != nil {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid annotation_value"))
		}
		val = loop_span.NewLongValue(i)
	case loop_span.AnnotationValueTypeString, loop_span.AnnotationValueTypeCategory:
		val = loop_span.NewStringValue(req.AnnotationValue)
	case loop_span.AnnotationValueTypeBool:
		b, err := strconv.ParseBool(req.AnnotationValue)
		if err != nil {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid annotation_value"))
		}
		val = loop_span.NewBoolValue(b)
	case loop_span.AnnotationValueTypeDouble, loop_span.AnnotationValueTypeNumber:
		f, err := strconv.ParseFloat(req.AnnotationValue, 64)
		if err != nil {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid annotation_value"))
		}
		val = loop_span.NewDoubleValue(f)
	default:
		val = loop_span.NewStringValue(req.AnnotationValue)
	}
	if err := o.auth.CheckWorkspacePermission(ctx,
		rpc.AuthActionAnnotationCreate,
		strconv.FormatInt(req.WorkspaceID, 10), true); err != nil {
		return nil, err
	}
	res, err := o.benefit.CheckTraceBenefit(ctx, &benefit.CheckTraceBenefitParams{
		ConnectorUID: session.UserIDInCtxOrEmpty(ctx),
		SpaceID:      req.WorkspaceID,
	})
	if err != nil {
		return nil, err
	}
	err = o.traceService.CreateAnnotation(ctx, &service.CreateAnnotationReq{
		WorkspaceID:   req.GetWorkspaceID(),
		SpanID:        req.GetSpanID(),
		TraceID:       req.GetTraceID(),
		AnnotationKey: req.GetAnnotationKey(),
		AnnotationVal: val,
		Reasoning:     req.GetReasoning(),
		QueryDays:     res.StorageDuration,
		Caller:        req.GetBase().GetCaller(),
	})
	if err != nil {
		return nil, err
	}
	return openapi.NewCreateAnnotationResponse(), nil
}

func (o *OpenAPIApplication) DeleteAnnotation(ctx context.Context, req *openapi.DeleteAnnotationRequest) (*openapi.DeleteAnnotationResponse, error) {
	if err := o.auth.CheckWorkspacePermission(ctx,
		rpc.AuthActionAnnotationCreate,
		strconv.FormatInt(req.WorkspaceID, 10), true); err != nil {
		return nil, err
	}
	res, err := o.benefit.CheckTraceBenefit(ctx, &benefit.CheckTraceBenefitParams{
		ConnectorUID: session.UserIDInCtxOrEmpty(ctx),
		SpaceID:      req.WorkspaceID,
	})
	if err != nil {
		return nil, err
	}
	err = o.traceService.DeleteAnnotation(ctx, &service.DeleteAnnotationReq{
		WorkspaceID:   req.GetWorkspaceID(),
		SpanID:        req.GetSpanID(),
		TraceID:       req.GetTraceID(),
		AnnotationKey: req.GetAnnotationKey(),
		QueryDays:     res.StorageDuration,
		Caller:        req.GetBase().GetCaller(),
	})
	if err != nil {
		return nil, err
	}
	return openapi.NewDeleteAnnotationResponse(), nil
}

func (o *OpenAPIApplication) SearchTraceOApi(ctx context.Context, req *openapi.SearchTraceOApiRequest) (*openapi.SearchTraceOApiResponse, error) {
	var err error
	st := time.Now()
	spansSize := 0
	errCode := 0
	defer func() {
		if req != nil {
			o.metrics.EmitTraceOapi("SearchTraceOApi", req.WorkspaceID, req.GetPlatformType(), "", int64(spansSize), errCode, st, err != nil)
			o.collector.CollectTraceOpenAPIEvent(ctx, "SearchTraceOApi", req.WorkspaceID, req.GetPlatformType(), "", int64(spansSize), errCode, st, err != nil)
		}
	}()

	if err = o.validateSearchTraceOApiReq(ctx, req); err != nil {
		errCode = obErrorx.CommercialCommonInvalidParamCodeCode
		return nil, err
	}
	if err = o.auth.CheckQueryPermission(ctx, strconv.FormatInt(req.GetWorkspaceID(), 10), req.GetPlatformType()); err != nil {
		errCode = obErrorx.CommonNoPermissionCode
		return nil, err
	}
	limitKey := strconv.FormatInt(req.GetWorkspaceID(), 10)
	if !o.AllowByKey(ctx, limitKey) {
		err = errorx.NewByCode(obErrorx.CommonRequestRateLimitCode, errorx.WithExtraMsg("qps limit exceeded"))
		errCode = obErrorx.CommonRequestRateLimitCode
		return nil, err
	}
	sReq, err := o.buildSearchTraceOApiReq(ctx, req)
	if err != nil {
		errCode = obErrorx.CommonInternalErrorCode
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("search trace req is invalid"))
	}
	sResp, err := o.traceService.SearchTraceOApi(ctx, sReq)
	if err != nil {
		return nil, err
	}
	inTokens, outTokens, err := sResp.Spans.Stat(ctx)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	spansSize = loop_span.SizeofSpans(sResp.Spans)
	logs.CtxInfo(ctx, "SearchTrace successfully, spans count %d", len(sResp.Spans))
	return &openapi.SearchTraceOApiResponse{
		Data: &openapi.SearchTraceOApiData{
			Spans: tconv.SpanListDO2DTO(sResp.Spans, nil, nil, nil),
			TracesAdvanceInfo: &trace.TraceAdvanceInfo{
				Tokens: &trace.TokenCost{
					Input:  inTokens,
					Output: outTokens,
				},
			},
		},
	}, nil
}

func (o *OpenAPIApplication) validateSearchTraceOApiReq(ctx context.Context, req *openapi.SearchTraceOApiRequest) error {
	if req == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetTraceID() == "" && req.GetLogid() == "" {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("at least need trace_id or log_id"))
	} else if req.Limit > MaxListSpansLimit || req.Limit < 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid limit"))
	}
	v := utils.DateValidator{
		Start:        req.GetStartTime(),
		End:          req.GetEndTime(),
		EarliestDays: 365,
	}
	newStartTime, newEndTime, err := v.CorrectDate()
	if err != nil {
		return err
	}
	req.SetStartTime(newStartTime)
	req.SetEndTime(newEndTime)
	return nil
}

func (o *OpenAPIApplication) buildSearchTraceOApiReq(ctx context.Context, req *openapi.SearchTraceOApiRequest) (*service.SearchTraceOApiReq, error) {
	platformType := loop_span.PlatformType(req.GetPlatformType())
	if req.PlatformType == nil {
		platformType = loop_span.PlatformCozeLoop
	}

	ret := &service.SearchTraceOApiReq{
		WorkspaceID:           req.WorkspaceID,
		ThirdPartyWorkspaceID: o.workspace.GetThirdPartyQueryWorkSpaceID(ctx, req.WorkspaceID),
		Tenants:               o.tenant.GetOAPIQueryTenants(ctx, platformType),
		TraceID:               req.GetTraceID(),
		LogID:                 req.GetLogid(),
		StartTime:             req.GetStartTime(),
		EndTime:               req.GetEndTime(),
		Limit:                 req.GetLimit(),
		PlatformType:          platformType,
		WithDetail:            true,
		SpanIDs:               req.SpanIds,
	}
	if len(ret.Tenants) == 0 {
		logs.CtxError(ctx, "fail to get platform tenants")
		return nil, errorx.WrapByCode(errors.New("fail to get platform tenants"), obErrorx.CommercialCommonInternalErrorCodeCode)
	}

	return ret, nil
}

func (o *OpenAPIApplication) SearchTraceTreeOApi(ctx context.Context, req *openapi.SearchTraceTreeOApiRequest) (*openapi.SearchTraceTreeOApiResponse, error) {
	var err error
	st := time.Now()
	spansSize := 0
	errCode := 0
	defer func() {
		if req != nil {
			o.metrics.EmitTraceOapi("SearchTraceTreeOApi", req.GetWorkspaceID(), req.GetPlatformType(), "", int64(spansSize), errCode, st, err != nil)
			o.collector.CollectTraceOpenAPIEvent(ctx, "SearchTraceTreeOApi", req.GetWorkspaceID(), req.GetPlatformType(), "", int64(spansSize), errCode, st, err != nil)
		}
	}()

	if err = o.validateSearchTraceTreeOApiReq(ctx, req); err != nil {
		errCode = obErrorx.CommercialCommonInvalidParamCodeCode
		return nil, err
	}
	if err = o.auth.CheckQueryPermission(ctx, strconv.FormatInt(req.GetWorkspaceID(), 10), req.GetPlatformType()); err != nil {
		errCode = obErrorx.CommonNoPermissionCode
		return nil, err
	}
	limitKey := strconv.FormatInt(req.GetWorkspaceID(), 10)
	if !o.AllowByKey(ctx, limitKey) {
		err = errorx.NewByCode(obErrorx.CommonRequestRateLimitCode, errorx.WithExtraMsg("qps limit exceeded"))
		errCode = obErrorx.CommonRequestRateLimitCode
		return nil, err
	}
	sReq, err := o.buildSearchTraceTreeOApiReq(ctx, req)
	if err != nil {
		errCode = obErrorx.CommonInternalErrorCode
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("search trace req is invalid"))
	}
	sResp, err := o.traceService.SearchTraceOApi(ctx, sReq)
	if err != nil {
		return nil, err
	}
	inTokens, outTokens, err := sResp.Spans.Stat(ctx)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	if sResp != nil {
		spansSize = loop_span.SizeofSpans(sResp.Spans)
		logs.CtxInfo(ctx, "SearchTrace successfully, spans count %d", len(sResp.Spans))
	}
	return &openapi.SearchTraceTreeOApiResponse{
		Data: &openapi.SearchTraceOApiData{
			Spans: tconv.SpanListDO2DTO(sResp.Spans, nil, nil, nil),
			TracesAdvanceInfo: &trace.TraceAdvanceInfo{
				Tokens: &trace.TokenCost{
					Input:  inTokens,
					Output: outTokens,
				},
			},
		},
	}, nil
}

func (o *OpenAPIApplication) validateSearchTraceTreeOApiReq(ctx context.Context, req *openapi.SearchTraceTreeOApiRequest) error {
	if req == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetTraceID() == "" {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("at least need trace_id or log_id"))
	} else if req.Limit > MaxTraceTreeLength || req.Limit < 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid limit"))
	}
	v := utils.DateValidator{
		Start:        req.GetStartTime(),
		End:          req.GetEndTime(),
		EarliestDays: 365,
	}
	newStartTime, newEndTime, err := v.CorrectDate()
	if err != nil {
		return err
	}
	req.SetStartTime(&newStartTime)
	req.SetEndTime(&newEndTime)
	return nil
}

func (o *OpenAPIApplication) buildSearchTraceTreeOApiReq(ctx context.Context, req *openapi.SearchTraceTreeOApiRequest) (*service.SearchTraceOApiReq, error) {
	platformType := loop_span.PlatformType(req.GetPlatformType())
	if req.PlatformType == nil {
		platformType = loop_span.PlatformCozeLoop
	}

	ret := &service.SearchTraceOApiReq{
		WorkspaceID:           req.GetWorkspaceID(),
		ThirdPartyWorkspaceID: o.workspace.GetThirdPartyQueryWorkSpaceID(ctx, req.GetWorkspaceID()),
		Tenants:               o.tenant.GetOAPIQueryTenants(ctx, platformType),
		TraceID:               req.GetTraceID(),
		StartTime:             req.GetStartTime(),
		EndTime:               req.GetEndTime(),
		Limit:                 req.GetLimit(),
		PlatformType:          platformType,
		WithDetail:            false,
	}

	if len(ret.Tenants) == 0 {
		logs.CtxError(ctx, "fail to get platform tenants")
		return nil, errorx.WrapByCode(errors.New("fail to get platform tenants"), obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	if req.Filters != nil {
		ret.Filters = tconv.FilterFieldsDTO2DO(req.Filters)
		if err := ret.Filters.Validate(); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func (o *OpenAPIApplication) ListSpansOApi(ctx context.Context, req *openapi.ListSpansOApiRequest) (*openapi.ListSpansOApiResponse, error) {
	var err error
	st := time.Now()
	spansSize := 0
	errCode := 0
	resp := openapi.NewListSpansOApiResponse()
	defer func() {
		if req != nil {
			o.metrics.EmitTraceOapi("ListSpansOApi", req.WorkspaceID, req.GetPlatformType(), req.GetSpanListType(), int64(spansSize), errCode, st, err != nil)
			o.collector.CollectTraceOpenAPIEvent(ctx, "ListSpansOApi", req.WorkspaceID, req.GetPlatformType(), req.GetSpanListType(), int64(spansSize), errCode, st, err != nil)
		}
	}()
	if err = o.validateListSpansOApi(ctx, req); err != nil {
		errCode = obErrorx.CommercialCommonInvalidParamCodeCode
		return nil, err
	}
	if err = o.auth.CheckQueryPermission(ctx, strconv.FormatInt(req.GetWorkspaceID(), 10), req.GetPlatformType()); err != nil {
		errCode = obErrorx.CommonNoPermissionCode
		return nil, err
	}

	limitKey := strconv.FormatInt(req.GetWorkspaceID(), 10)
	if !o.AllowByKey(ctx, limitKey) {
		err = errorx.NewByCode(obErrorx.CommonRequestRateLimitCode, errorx.WithExtraMsg("qps limit exceeded"))
		errCode = obErrorx.CommonRequestRateLimitCode
		return nil, err
	}
	sReq, err := o.buildListSpansOApiReq(ctx, req)
	if err != nil {
		errCode = obErrorx.CommercialCommonInvalidParamCodeCode
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("list spans req is invalid"))
	}
	sResp, err := o.traceService.ListSpansOApi(ctx, sReq)
	if err != nil {
		errCode = obErrorx.CommonInternalErrorCode
		return nil, err
	}
	logs.CtxInfo(ctx, "List spans successfully, spans count: %d", len(sResp.Spans))
	spansSize = loop_span.SizeofSpans(sResp.Spans)

	resp.Data = &openapi.ListSpansOApiData{
		Spans:         tconv.SpanListDO2DTO(sResp.Spans, nil, nil, nil),
		NextPageToken: sResp.NextPageToken,
		HasMore:       sResp.HasMore,
	}
	resp.BaseResp = base.NewBaseResp()
	return resp, nil
}

func (o *OpenAPIApplication) validateListSpansOApi(ctx context.Context, req *openapi.ListSpansOApiRequest) error {
	if req == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if pageSize := req.GetPageSize(); pageSize < 0 || pageSize > MaxOApiListSpansLimit {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid limit"))
	} else if len(req.GetOrderBys()) > 1 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid order by %s"))
	}
	v := utils.DateValidator{
		Start:        req.GetStartTime(),
		End:          req.GetEndTime(),
		EarliestDays: 365,
	}
	newStartTime, newEndTime, err := v.CorrectDate()
	if err != nil {
		return err
	}
	req.SetStartTime(newStartTime)
	req.SetEndTime(newEndTime)
	return nil
}

func (o *OpenAPIApplication) buildListSpansOApiReq(ctx context.Context, req *openapi.ListSpansOApiRequest) (*service.ListSpansOApiReq, error) {
	ret := &service.ListSpansOApiReq{
		WorkspaceID:           req.WorkspaceID,
		ThirdPartyWorkspaceID: o.workspace.GetThirdPartyQueryWorkSpaceID(ctx, req.WorkspaceID),
		StartTime:             req.GetStartTime(),
		EndTime:               req.GetEndTime(),
		Limit:                 QueryLimitDefault,
		DescByStartTime:       len(req.GetOrderBys()) > 0,
		PageToken:             req.GetPageToken(),
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
	tenants := o.tenant.GetOAPIQueryTenants(ctx, platformType)
	if len(tenants) == 0 {
		logs.CtxError(ctx, "fail to get platform tenants")
		return nil, errorx.WrapByCode(errors.New("fail to get platform tenants"), obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	ret.Tenants = tenants
	return ret, nil
}

func (o *OpenAPIApplication) ListTracesOApi(ctx context.Context, req *openapi.ListTracesOApiRequest) (*openapi.ListTracesOApiResponse, error) {
	var err error
	st := time.Now()
	errCode := 0
	defer func() {
		o.metrics.EmitTraceOapi("ListTracesOApi", req.WorkspaceID, "", "", 0, errCode, st, err != nil)
		o.collector.CollectTraceOpenAPIEvent(ctx, "ListTracesOApi", req.WorkspaceID, "", "", 0, errCode, st, err != nil)
	}()

	if err = o.validateListTracesOApiReq(ctx, req); err != nil {
		errCode = obErrorx.CommercialCommonInvalidParamCodeCode
		return nil, err
	}
	if err = o.auth.CheckQueryPermission(ctx, strconv.FormatInt(req.GetWorkspaceID(), 10), req.GetPlatformType()); err != nil {
		errCode = obErrorx.CommonNoPermissionCode
		return nil, err
	}

	limitKey := strconv.FormatInt(req.GetWorkspaceID(), 10)
	if !o.AllowByKey(ctx, limitKey) {
		err = errorx.NewByCode(obErrorx.CommonRequestRateLimitCode, errorx.WithExtraMsg("qps limit exceeded"))
		errCode = obErrorx.CommonRequestRateLimitCode
		return nil, err
	}

	logs.CtxInfo(ctx, "ListTracesOApi request: %+v", req)
	sReq := o.buildListTracesOApiReq(ctx, req)
	sResp, err := o.traceService.GetTracesAdvanceInfo(ctx, sReq)
	if err != nil {
		errCode = obErrorx.CommonInternalErrorCode
		return nil, err
	}
	traces := make([]*traced.Trace, 0)
	for _, info := range sResp.Infos {
		traces = append(traces, tconv.AdvanceInfoDO2TraceDTO(info))
	}
	return &openapi.ListTracesOApiResponse{
		Data: &openapi.ListTracesData{
			Traces: traces,
		},
	}, nil
}

func (o *OpenAPIApplication) validateListTracesOApiReq(ctx context.Context, req *openapi.ListTracesOApiRequest) error {
	if req == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	} else if len(req.GetTraceIds()) < 1 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid traces"))
	}
	for _, id := range req.TraceIds {
		if id == "" {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid trace_id"))
		}
	}
	v := utils.DateValidator{
		Start:        req.GetStartTime(),
		End:          req.GetEndTime(),
		EarliestDays: 365,
	}
	newStartTime, newEndTime, err := v.CorrectDate()
	logs.CtxInfo(ctx, "newStartTime: %d, newEndTime: %d", newStartTime, newEndTime)
	if err != nil {
		return err
	}
	req.SetStartTime(newStartTime)
	req.SetEndTime(newEndTime)
	return nil
}

func (o *OpenAPIApplication) buildListTracesOApiReq(ctx context.Context, req *openapi.ListTracesOApiRequest) *service.GetTracesAdvanceInfoReq {
	ret := &service.GetTracesAdvanceInfoReq{
		WorkspaceID:           req.GetWorkspaceID(),
		ThirdPartyWorkspaceID: o.workspace.GetThirdPartyQueryWorkSpaceID(ctx, req.WorkspaceID),
		Traces:                make([]*service.TraceQueryParam, len(req.GetTraceIds())),
	}
	for i, id := range req.GetTraceIds() {
		ret.Traces[i] = &service.TraceQueryParam{
			TraceID:   id,
			StartTime: req.GetStartTime(),
			EndTime:   req.GetEndTime(),
		}
	}
	platformType := loop_span.PlatformType(req.GetPlatformType())
	if req.PlatformType == nil {
		platformType = loop_span.PlatformCozeLoop
	}
	ret.PlatformType = platformType
	return ret
}

func (o *OpenAPIApplication) Send(ctx context.Context, event *entity.AnnotationEvent) error {
	return o.traceService.Send(ctx, event)
}

func (p *OpenAPIApplication) AllowByKey(ctx context.Context, key string) bool {
	maxQPS, err := p.traceConfig.GetQueryMaxQPS(ctx, key)
	if err != nil {
		logs.CtxError(ctx, "get query max qps failed, err=%v, key=%s", err, key)
		return true
	}
	result, err := p.rateLimiter.AllowN(ctx, key, 1,
		limiter.WithLimit(&limiter.Limit{
			Rate:   maxQPS,
			Burst:  maxQPS,
			Period: time.Second,
		}))
	if err != nil {
		logs.CtxError(ctx, "allow rate limit failed, err=%v", err)
		return true
	}
	if result == nil || result.Allowed {
		return true
	}
	return false
}
