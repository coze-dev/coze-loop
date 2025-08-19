// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"

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
) (IObservabilityOpenAPIApplication, error) {
	return &OpenAPIApplication{
		traceService: traceService,
		auth:         auth,
		benefit:      benefit,
	}, nil
}

type OpenAPIApplication struct {
	traceService service.ITraceService
	auth         rpc.IAuthProvider
	benefit      benefit.IBenefitService
}

func (o *OpenAPIApplication) IngestTraces(ctx context.Context, req *openapi.IngestTracesRequest) (*openapi.IngestTracesResponse, error) {
	if err := o.validateIngestTracesReq(ctx, req); err != nil {
		return nil, err
	}
	workspaceId := req.GetSpans()[0].WorkspaceID
	if err := o.auth.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceIngest,
		workspaceId); err != nil {
		return nil, err
	}
	workSpaceIdNum, err := strconv.ParseInt(workspaceId, 10, 64)
	if err != nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	connectorUid := session.UserIDInCtxOrEmpty(ctx)
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
	spans := tconv.SpanListDTO2DO(req.Spans)
	for _, s := range spans {
		s.CallType = "Custom"
	}
	if err := o.traceService.IngestTraces(ctx, &service.IngestTracesReq{
		TTL:              loop_span.TTLFromInteger(benefitRes.StorageDuration),
		WhichIsEnough:    benefitRes.WhichIsEnough,
		CozeAccountId:    connectorUid,
		VolcanoAccountID: benefitRes.VolcanoAccountID,
		Spans:            spans,
	}); err != nil {
		return nil, err
	}
	return openapi.NewIngestTracesResponse(), nil
}

func (o *OpenAPIApplication) validateIngestTracesReq(ctx context.Context, req *openapi.IngestTracesRequest) error {
	if req == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if len(req.Spans) > MaxSpanLength {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("max span length exceeded"))
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

func (o *OpenAPIApplication) CreateAnnotation(ctx context.Context, req *openapi.CreateAnnotationRequest) (*openapi.CreateAnnotationResponse, error) {
	var val loop_span.AnnotationValue
	switch loop_span.AnnotationValueType(req.GetAnnotationValueType()) {
	case loop_span.AnnotationValueTypeLong:
		i, err := strconv.ParseInt(req.AnnotationValue, 10, 64)
		if err != nil {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid annotation_value"))
		}
		val = loop_span.NewLongValue(i)
	case loop_span.AnnotationValueTypeString:
		val = loop_span.NewStringValue(req.AnnotationValue)
	case loop_span.AnnotationValueTypeBool:
		b, err := strconv.ParseBool(req.AnnotationValue)
		if err != nil {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid annotation_value"))
		}
		val = loop_span.NewBoolValue(b)
	case loop_span.AnnotationValueTypeDouble:
		f, err := strconv.ParseFloat(req.AnnotationValue, 64)
		if err != nil {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid annotation_value"))
		}
		val = loop_span.NewDoubleValue(f)
	default:
		val = loop_span.NewStringValue(req.AnnotationValue)
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

func (o *OpenAPIApplication) Send(ctx context.Context, event *entity.AnnotationEvent) error {
	return o.traceService.Send(ctx, event)
}
