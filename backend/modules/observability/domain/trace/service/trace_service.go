// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_processor"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	time_util "github.com/coze-dev/coze-loop/backend/pkg/time"
)

type ListSpansReq struct {
	WorkspaceID     int64
	StartTime       int64 // ms
	EndTime         int64 // ms
	Filters         *loop_span.FilterFields
	Limit           int32
	DescByStartTime bool
	PageToken       string
	PlatformType    loop_span.PlatformType
	SpanListType    loop_span.SpanListType
}

type ListSpansResp struct {
	Spans         loop_span.SpanList
	NextPageToken string
	HasMore       bool
}

type GetTraceReq struct {
	WorkspaceID  int64
	LogID        string
	TraceID      string
	StartTime    int64 // ms
	EndTime      int64 // ms
	PlatformType loop_span.PlatformType
	SpanIDs      []string
}

type GetTraceResp struct {
	TraceId string
	Spans   loop_span.SpanList
}

type SearchTraceOApiReq struct {
	WorkspaceID  int64
	Tenants      []string
	TraceID      string
	LogID        string
	StartTime    int64 // ms
	EndTime      int64 // ms
	Limit        int32
	PlatformType loop_span.PlatformType
}

type SearchTraceOApiResp struct {
	Spans loop_span.SpanList
}

type ListSpansOApiReq struct {
	WorkspaceID     int64
	Tenants         []string
	StartTime       int64 // ms
	EndTime         int64 // ms
	Filters         *loop_span.FilterFields
	Limit           int32
	DescByStartTime bool
	PageToken       string
	PlatformType    loop_span.PlatformType
	SpanListType    loop_span.SpanListType
}

type ListSpansOApiResp struct {
	Spans         loop_span.SpanList
	NextPageToken string
	HasMore       bool
}

type TraceQueryParam struct {
	TraceID   string
	StartTime int64 // ms
	EndTime   int64 // ms
}

type GetTracesAdvanceInfoReq struct {
	WorkspaceID  int64
	Traces       []*TraceQueryParam
	PlatformType loop_span.PlatformType
}

type GetTracesAdvanceInfoResp struct {
	Infos []*loop_span.TraceAdvanceInfo
}

type IngestTracesReq struct {
	Tenant           string
	TTL              loop_span.TTL
	WhichIsEnough    int
	CozeAccountId    string
	VolcanoAccountID int64
	Spans            loop_span.SpanList
}

type SendTraceResp struct{}

type GetTracesMetaInfoReq struct {
	WorkspaceID  int64
	PlatformType loop_span.PlatformType
	SpanListType loop_span.SpanListType
}

type GetTracesMetaInfoResp struct {
	FilesMetas map[string]*config.FieldMeta
}

type CreateAnnotationReq struct {
	WorkspaceID   int64
	SpanID        string
	TraceID       string
	AnnotationKey string
	AnnotationVal loop_span.AnnotationValue
	Reasoning     string
	QueryDays     int64
	Caller        string
}
type DeleteAnnotationReq struct {
	WorkspaceID   int64
	SpanID        string
	TraceID       string
	AnnotationKey string
	QueryDays     int64
	Caller        string
}

type CreateManualAnnotationReq struct {
	PlatformType loop_span.PlatformType
	Annotation   *loop_span.Annotation
}

type CreateManualAnnotationResp struct {
	AnnotationID string
}

type UpdateManualAnnotationReq struct {
	AnnotationID string
	Annotation   *loop_span.Annotation
	PlatformType loop_span.PlatformType
}

type DeleteManualAnnotationReq struct {
	AnnotationID  string
	WorkspaceID   int64
	TraceID       string
	SpanID        string
	StartTime     int64 // ms
	AnnotationKey string
	PlatformType  loop_span.PlatformType
}

type ListAnnotationsReq struct {
	WorkspaceID     int64
	TraceID         string
	SpanID          string
	StartTime       int64
	DescByUpdatedAt bool
	PlatformType    loop_span.PlatformType
}

type ListAnnotationsResp struct {
	Annotations loop_span.AnnotationList
}

type IAnnotationEvent interface {
	Send(ctx context.Context, msg *entity.AnnotationEvent) error
}

//go:generate mockgen -destination=mocks/trace_service.go -package=mocks . ITraceService
type ITraceService interface {
	ListSpans(ctx context.Context, req *ListSpansReq) (*ListSpansResp, error)
	GetTrace(ctx context.Context, req *GetTraceReq) (*GetTraceResp, error)
	SearchTraceOApi(ctx context.Context, req *SearchTraceOApiReq) (*SearchTraceOApiResp, error)
	ListSpansOApi(ctx context.Context, req *ListSpansOApiReq) (*ListSpansOApiResp, error)
	GetTracesAdvanceInfo(ctx context.Context, req *GetTracesAdvanceInfoReq) (*GetTracesAdvanceInfoResp, error)
	IngestTraces(ctx context.Context, req *IngestTracesReq) error
	GetTracesMetaInfo(ctx context.Context, req *GetTracesMetaInfoReq) (*GetTracesMetaInfoResp, error)
	ListAnnotations(ctx context.Context, req *ListAnnotationsReq) (*ListAnnotationsResp, error)
	CreateAnnotation(ctx context.Context, req *CreateAnnotationReq) error
	DeleteAnnotation(ctx context.Context, req *DeleteAnnotationReq) error
	CreateManualAnnotation(ctx context.Context, req *CreateManualAnnotationReq) (*CreateManualAnnotationResp, error)
	UpdateManualAnnotation(ctx context.Context, req *UpdateManualAnnotationReq) error
	DeleteManualAnnotation(ctx context.Context, req *DeleteManualAnnotationReq) error
	IAnnotationEvent
}

func NewTraceServiceImpl(
	tRepo repo.ITraceRepo,
	traceConfig config.ITraceConfig,
	traceProducer mq.ITraceProducer,
	annotationProducer mq.IAnnotationProducer,
	metrics metrics.ITraceMetrics,
	buildHelper TraceFilterProcessorBuilder,
	tenantProvider tenant.ITenantProvider,
) (ITraceService, error) {
	return &TraceServiceImpl{
		traceRepo:          tRepo,
		traceConfig:        traceConfig,
		traceProducer:      traceProducer,
		annotationProducer: annotationProducer,
		buildHelper:        buildHelper,
		tenantProvider:     tenantProvider,
		metrics:            metrics,
	}, nil
}

type TraceServiceImpl struct {
	traceRepo          repo.ITraceRepo
	traceConfig        config.ITraceConfig
	traceProducer      mq.ITraceProducer
	annotationProducer mq.IAnnotationProducer
	metrics            metrics.ITraceMetrics
	buildHelper        TraceFilterProcessorBuilder
	tenantProvider     tenant.ITenantProvider
}

func (r *TraceServiceImpl) GetTrace(ctx context.Context, req *GetTraceReq) (*GetTraceResp, error) {
	tenants, err := r.getTenants(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	st := time.Now()
	spans, err := r.traceRepo.GetTrace(ctx, &repo.GetTraceParam{
		Tenants: tenants,
		LogID:   req.LogID,
		TraceID: req.TraceID,
		StartAt: req.StartTime,
		EndAt:   req.EndTime,
		Limit:   1000,
		SpanIDs: req.SpanIDs,
	})
	r.metrics.EmitGetTrace(req.WorkspaceID, st, err != nil)
	if err != nil {
		return nil, err
	}
	processors, err := r.buildHelper.BuildGetTraceProcessors(ctx, span_processor.Settings{
		WorkspaceId:    req.WorkspaceID,
		PlatformType:   req.PlatformType,
		QueryStartTime: req.StartTime,
		QueryEndTime:   req.EndTime,
	})
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	for _, p := range processors {
		spans, err = p.Transform(ctx, spans)
		if err != nil {
			return nil, err
		}
	}
	spans.SortByStartTime(false)
	return &GetTraceResp{
		TraceId: req.TraceID,
		Spans:   spans,
	}, nil
}

func (r *TraceServiceImpl) ListSpans(ctx context.Context, req *ListSpansReq) (*ListSpansResp, error) {
	if err := req.Filters.Traverse(processSpecificFilter); err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid filter"))
	}
	platformFilter, err := r.buildHelper.BuildPlatformRelatedFilter(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	builtinFilter, err := r.buildBuiltinFilters(ctx, platformFilter, req)
	if err != nil {
		return nil, err
	} else if builtinFilter == nil {
		return &ListSpansResp{Spans: loop_span.SpanList{}}, nil
	}
	filters := r.combineFilters(builtinFilter, req.Filters)
	tenants, err := r.getTenants(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	st := time.Now()
	tRes, err := r.traceRepo.ListSpans(ctx, &repo.ListSpansParam{
		Tenants:         tenants,
		Filters:         filters,
		StartAt:         req.StartTime,
		EndAt:           req.EndTime,
		Limit:           req.Limit,
		DescByStartTime: req.DescByStartTime,
		PageToken:       req.PageToken,
	})
	r.metrics.EmitListSpans(req.WorkspaceID, string(req.SpanListType), st, err != nil)
	if err != nil {
		return nil, err
	}
	spans := tRes.Spans
	processors, err := r.buildHelper.BuildListSpansProcessors(ctx, span_processor.Settings{
		WorkspaceId:    req.WorkspaceID,
		PlatformType:   req.PlatformType,
		QueryStartTime: req.StartTime,
		QueryEndTime:   req.EndTime,
	})
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	for _, p := range processors {
		spans, err = p.Transform(ctx, spans)
		if err != nil {
			return nil, err
		}
	}
	return &ListSpansResp{
		Spans:         spans,
		NextPageToken: tRes.PageToken,
		HasMore:       tRes.HasMore,
	}, nil
}

func (r *TraceServiceImpl) SearchTraceOApi(ctx context.Context, req *SearchTraceOApiReq) (*SearchTraceOApiResp, error) {
	spans, err := r.traceRepo.GetTrace(ctx, &repo.GetTraceParam{
		Tenants:            req.Tenants,
		TraceID:            req.TraceID,
		LogID:              req.LogID,
		StartAt:            req.StartTime,
		EndAt:              req.EndTime,
		Limit:              req.Limit,
		NotQueryAnnotation: false,
	})
	if err != nil {
		return nil, err
	}
	processors, err := r.buildHelper.BuildSearchTraceOApiProcessors(ctx, span_processor.Settings{
		WorkspaceId:    req.WorkspaceID,
		QueryStartTime: req.StartTime,
		QueryEndTime:   req.EndTime,
		PlatformType:   req.PlatformType,
	})
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	for _, p := range processors {
		spans, err = p.Transform(ctx, spans)
		if err != nil {
			return nil, err
		}
	}
	spans.SortByStartTime(false)
	return &SearchTraceOApiResp{
		Spans: spans,
	}, nil
}

func (r *TraceServiceImpl) ListSpansOApi(ctx context.Context, req *ListSpansOApiReq) (*ListSpansOApiResp, error) {
	if err := req.Filters.Traverse(processSpecificFilter); err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid filter"))
	}
	platformFilter, err := r.buildHelper.BuildPlatformRelatedFilter(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	builtinFilter, err := r.buildBuiltinFilters(ctx, platformFilter, &ListSpansReq{
		WorkspaceID:  req.WorkspaceID,
		SpanListType: req.SpanListType,
	})
	if err != nil {
		return nil, err
	} else if builtinFilter == nil {
		return &ListSpansOApiResp{Spans: loop_span.SpanList{}}, nil
	}
	filters := r.combineFilters(builtinFilter, req.Filters)
	tRes, err := r.traceRepo.ListSpans(ctx, &repo.ListSpansParam{
		Tenants:         req.Tenants,
		Filters:         filters,
		StartAt:         req.StartTime,
		EndAt:           req.EndTime,
		Limit:           req.Limit,
		DescByStartTime: req.DescByStartTime,
		PageToken:       req.PageToken,
	})
	if err != nil {
		return nil, err
	}

	spans := tRes.Spans
	processors, err := r.buildHelper.BuildListSpansOApiProcessors(ctx, span_processor.Settings{
		WorkspaceId:    req.WorkspaceID,
		QueryStartTime: req.StartTime,
		QueryEndTime:   req.EndTime,
	})
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	for _, p := range processors {
		spans, err = p.Transform(ctx, spans)
		if err != nil {
			return nil, err
		}
	}
	return &ListSpansOApiResp{
		Spans:         spans,
		NextPageToken: tRes.PageToken,
		HasMore:       tRes.HasMore,
	}, nil
}

func (r *TraceServiceImpl) IngestTraces(ctx context.Context, req *IngestTracesReq) error {
	processors, err := r.buildHelper.BuildIngestTraceProcessors(ctx, span_processor.Settings{
		Tenant: req.Tenant,
	})
	if err != nil {
		return errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	for _, p := range processors {
		req.Spans, err = p.Transform(ctx, req.Spans)
		if err != nil {
			return err
		}
	}

	traceData := &entity.TraceData{
		Tenant: req.Tenant,
		TenantInfo: entity.TenantInfo{
			TTL:              req.TTL,
			WorkspaceId:      req.Spans[0].WorkspaceID,
			CozeAccountID:    req.CozeAccountId,
			WhichIsEnough:    req.WhichIsEnough,
			VolcanoAccountID: req.VolcanoAccountID,
		},
		SpanList: req.Spans,
	}
	if err := r.traceProducer.IngestSpans(ctx, traceData); err != nil {
		return err
	}
	logs.CtxInfo(ctx, "Send msg successfully, spans count %d", len(req.Spans))
	return nil
}

func (r *TraceServiceImpl) GetTracesAdvanceInfo(ctx context.Context, req *GetTracesAdvanceInfoReq) (*GetTracesAdvanceInfoResp, error) {
	var (
		g                errgroup.Group
		lock             sync.Mutex
		defaultTimeRange = int64(15 * 60 * 1000) // ms
	)
	tenants, err := r.getTenants(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	resp := &GetTracesAdvanceInfoResp{
		Infos: []*loop_span.TraceAdvanceInfo{},
	}
	// use one processor...
	processors, err := r.buildHelper.BuildAdvanceInfoProcessors(ctx, span_processor.Settings{
		WorkspaceId:  req.WorkspaceID,
		PlatformType: req.PlatformType,
	})
	if err != nil {
		logs.CtxError(ctx, "Fail to build advance info processor, %v", err)
		return nil, err
	}
	for _, v := range req.Traces {
		g.Go(func() error {
			defer goroutine.Recovery(ctx)
			qReq := &repo.GetTraceParam{
				Tenants:            tenants,
				TraceID:            v.TraceID,
				StartAt:            v.StartTime,
				EndAt:              v.StartTime + defaultTimeRange,
				Limit:              1000,
				NotQueryAnnotation: true, // no need to query annotation
				OmitColumns: []string{
					loop_span.SpanFieldInput,
					loop_span.SpanFieldOutput,
				},
			}
			st := time.Now()
			spans, err := r.traceRepo.GetTrace(ctx, qReq)
			r.metrics.EmitGetTrace(req.WorkspaceID, st, err != nil)
			if err != nil {
				logs.CtxError(ctx, "Fail to get trace %v, %v", *qReq, err)
				return err
			}
			for _, p := range processors {
				spans, err = p.Transform(ctx, spans)
				if err != nil {
					logs.CtxWarn(ctx, "Fail to transform span, %v", err)
					return nil
				}
			}
			inputTokens, outputTokens, err := spans.Stat(ctx)
			if err != nil {
				logs.CtxWarn(ctx, "Fail to get spans stat, %v", err)
				return nil
			}
			lock.Lock()
			defer lock.Unlock()
			resp.Infos = append(resp.Infos, &loop_span.TraceAdvanceInfo{
				TraceId:    qReq.TraceID,
				InputCost:  inputTokens,
				OutputCost: outputTokens,
			})
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		logs.CtxError(ctx, "fail to get all trace advance info, %v", err)
		return nil, err
	}
	return resp, nil
}

func (r *TraceServiceImpl) GetTracesMetaInfo(ctx context.Context, req *GetTracesMetaInfoReq) (*GetTracesMetaInfoResp, error) {
	cfg, err := r.traceConfig.GetTraceFieldMetaInfo(ctx)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	fields, ok := cfg.FieldMetas[req.PlatformType][req.SpanListType]
	if !ok {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("meta info not found"))
	}
	fieldMetas := make(map[string]*config.FieldMeta)
	for _, field := range fields {
		fieldMta, ok := cfg.AvailableFields[field]
		if !ok || fieldMta == nil {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode)
		}
		fieldMetas[field] = fieldMta
	}
	return &GetTracesMetaInfoResp{
		FilesMetas: fieldMetas,
	}, nil
}

func (r *TraceServiceImpl) ListAnnotations(ctx context.Context, req *ListAnnotationsReq) (*ListAnnotationsResp, error) {
	tenants, err := r.getTenants(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	annotations, err := r.traceRepo.ListAnnotations(ctx, &repo.ListAnnotationsParam{
		Tenants:         tenants,
		SpanID:          req.SpanID,
		TraceID:         req.TraceID,
		WorkspaceId:     req.WorkspaceID,
		DescByUpdatedAt: req.DescByUpdatedAt,
		StartAt:         req.StartTime - time.Second.Milliseconds(),
		EndAt:           req.StartTime + time.Second.Milliseconds(),
	})
	if err != nil {
		return nil, err
	}
	return &ListAnnotationsResp{
		Annotations: annotations,
	}, nil
}

func (r *TraceServiceImpl) CreateManualAnnotation(ctx context.Context, req *CreateManualAnnotationReq) (*CreateManualAnnotationResp, error) {
	tenants, err := r.getTenants(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	span, err := r.getSpan(ctx,
		tenants,
		req.Annotation.SpanID,
		req.Annotation.TraceID,
		req.Annotation.WorkspaceID,
		req.Annotation.StartTime.Add(-time.Second).UnixMilli(),
		req.Annotation.StartTime.Add(time.Second).UnixMilli(),
	)
	if err != nil {
		return nil, err
	} else if span == nil {
		logs.CtxWarn(ctx, "no span found for span_id %s trace_id %s", req.Annotation.SpanID, req.Annotation.TraceID)
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	annotation, err := span.BuildFeedback(
		loop_span.AnnotationTypeManualFeedback,
		req.Annotation.Key,
		req.Annotation.Value,
		req.Annotation.Reasoning,
		session.UserIDInCtxOrEmpty(ctx),
		false,
	)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid annotation"))
	}
	if err := r.traceRepo.InsertAnnotations(ctx, &repo.InsertAnnotationParam{
		Tenant:      span.GetTenant(),
		TTL:         span.GetTTL(ctx),
		Annotations: []*loop_span.Annotation{annotation},
	}); err != nil {
		return nil, err
	}
	return &CreateManualAnnotationResp{
		AnnotationID: annotation.ID,
	}, nil
}

func (r *TraceServiceImpl) UpdateManualAnnotation(ctx context.Context, req *UpdateManualAnnotationReq) error {
	tenants, err := r.getTenants(ctx, req.PlatformType)
	if err != nil {
		return err
	}
	span, err := r.getSpan(ctx,
		tenants,
		req.Annotation.SpanID,
		req.Annotation.TraceID,
		req.Annotation.WorkspaceID,
		req.Annotation.StartTime.Add(-time.Second).UnixMilli(),
		req.Annotation.StartTime.Add(time.Second).UnixMilli(),
	)
	if err != nil {
		return err
	} else if span == nil {
		logs.CtxWarn(ctx, "no span found for span_id %s trace_id %s", req.Annotation.SpanID, req.Annotation.TraceID)
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	annotation, err := span.BuildFeedback(
		loop_span.AnnotationTypeManualFeedback,
		req.Annotation.Key,
		req.Annotation.Value,
		req.Annotation.Reasoning,
		session.UserIDInCtxOrEmpty(ctx),
		false,
	)
	fmt.Println(annotation.ID, req.AnnotationID)
	if err != nil || annotation.ID != req.AnnotationID {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	existedAnno, err := r.traceRepo.GetAnnotation(ctx, &repo.GetAnnotationParam{
		Tenants: tenants,
		ID:      req.AnnotationID,
		StartAt: time.UnixMicro(span.StartTime).Add(-time.Second).UnixMilli(),
		EndAt:   time.UnixMicro(span.StartTime).Add(time.Second).UnixMilli(),
	})
	if err != nil {
		logs.CtxError(ctx, "get annotation %s err %v", req.AnnotationID, err)
		return err
	} else if existedAnno != nil {
		annotation.CreatedBy = existedAnno.CreatedBy
		annotation.CreatedAt = existedAnno.CreatedAt
	}
	return r.traceRepo.InsertAnnotations(ctx, &repo.InsertAnnotationParam{
		Tenant:      span.GetTenant(),
		TTL:         span.GetTTL(ctx),
		Annotations: []*loop_span.Annotation{annotation},
	})
}

func (r *TraceServiceImpl) DeleteManualAnnotation(ctx context.Context, req *DeleteManualAnnotationReq) error {
	tenants, err := r.getTenants(ctx, req.PlatformType)
	if err != nil {
		return err
	}
	span, err := r.getSpan(ctx,
		tenants,
		req.SpanID,
		req.TraceID,
		strconv.FormatInt(req.WorkspaceID, 10),
		req.StartTime-time.Second.Milliseconds(),
		req.StartTime+time.Second.Milliseconds(),
	)
	if err != nil {
		return err
	} else if span == nil {
		logs.CtxWarn(ctx, "no span found for span_id %s trace_id %s", req.SpanID, req.TraceID)
		return errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	annotation, err := span.BuildFeedback(
		loop_span.AnnotationTypeManualFeedback,
		req.AnnotationKey,
		loop_span.AnnotationValue{},
		"",
		session.UserIDInCtxOrEmpty(ctx),
		true,
	)
	if err != nil || annotation.ID != req.AnnotationID {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid annotation"))
	}
	return r.traceRepo.InsertAnnotations(ctx, &repo.InsertAnnotationParam{
		Tenant:      span.GetTenant(),
		TTL:         span.GetTTL(ctx),
		Annotations: []*loop_span.Annotation{annotation},
	})
}

func (r *TraceServiceImpl) CreateAnnotation(ctx context.Context, req *CreateAnnotationReq) error {
	cfg, err := r.getAnnotationCallerCfg(ctx, req.Caller)
	if err != nil {
		return err
	}
	span, err := r.getSpan(ctx,
		cfg.Tenants,
		req.SpanID,
		req.TraceID,
		strconv.FormatInt(req.WorkspaceID, 10),
		time.Now().Add(-time.Duration(req.QueryDays)*24*time.Hour).UnixMilli(),
		time.Now().UnixMilli(),
	)
	if err != nil {
		return err
	} else if span == nil {
		return r.annotationProducer.SendAnnotation(ctx, &entity.AnnotationEvent{
			Annotation: &loop_span.Annotation{
				SpanID:         req.SpanID,
				TraceID:        req.TraceID,
				WorkspaceID:    strconv.FormatInt(req.WorkspaceID, 10),
				AnnotationType: loop_span.AnnotationType(cfg.AnnotationType),
				Key:            req.AnnotationKey,
				Value:          req.AnnotationVal,
				Reasoning:      req.Reasoning,
				Status:         loop_span.AnnotationStatusNormal,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			},
			Caller:     req.Caller,
			StartAt:    time.Now().Add(-24 * time.Hour).UnixMilli(),
			EndAt:      time.Now().Add(1 * time.Hour).UnixMilli(),
			RetryTimes: 3,
		})
	}
	annotation, err := span.BuildFeedback(
		loop_span.AnnotationType(cfg.AnnotationType),
		req.AnnotationKey,
		req.AnnotationVal,
		req.Reasoning, "", false,
	)
	if err != nil {
		return errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid annotation"))
	}
	existedAnno, err := r.traceRepo.GetAnnotation(ctx, &repo.GetAnnotationParam{
		Tenants: cfg.Tenants,
		ID:      annotation.ID,
		StartAt: time.UnixMicro(span.StartTime).Add(-time.Second).UnixMilli(),
		EndAt:   time.UnixMicro(span.StartTime).Add(time.Second).UnixMilli(),
	})
	if err != nil {
		return err
	} else if existedAnno != nil {
		annotation.CreatedBy = existedAnno.CreatedBy
		annotation.CreatedAt = existedAnno.CreatedAt
	}
	return r.traceRepo.InsertAnnotations(ctx, &repo.InsertAnnotationParam{
		Tenant:      span.GetTenant(),
		TTL:         span.GetTTL(ctx),
		Annotations: []*loop_span.Annotation{annotation},
	})
}

func (r *TraceServiceImpl) DeleteAnnotation(ctx context.Context, req *DeleteAnnotationReq) error {
	cfg, err := r.getAnnotationCallerCfg(ctx, req.Caller)
	if err != nil {
		return err
	}
	span, err := r.getSpan(ctx,
		cfg.Tenants,
		req.SpanID,
		req.TraceID,
		strconv.FormatInt(req.WorkspaceID, 10),
		time.Now().Add(-time.Duration(req.QueryDays)*24*time.Hour).UnixMilli(),
		time.Now().UnixMilli(),
	)
	if err != nil {
		return err
	} else if span == nil {
		return r.annotationProducer.SendAnnotation(ctx, &entity.AnnotationEvent{
			Annotation: &loop_span.Annotation{
				SpanID:         req.SpanID,
				TraceID:        req.TraceID,
				WorkspaceID:    strconv.FormatInt(req.WorkspaceID, 10),
				AnnotationType: loop_span.AnnotationType(cfg.AnnotationType),
				Key:            req.AnnotationKey,
				Status:         loop_span.AnnotationStatusDeleted,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
				IsDeleted:      true,
			},
			Caller:     req.Caller,
			StartAt:    time.Now().Add(-24 * time.Hour).UnixMilli(),
			EndAt:      time.Now().Add(1 * time.Hour).UnixMilli(),
			RetryTimes: 3,
		})
	}
	annotation, err := span.BuildFeedback(
		loop_span.AnnotationType(cfg.AnnotationType),
		req.AnnotationKey,
		loop_span.AnnotationValue{}, "", "",
		true,
	)
	if err != nil {
		return errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid annotation"))
	}
	return r.traceRepo.InsertAnnotations(ctx, &repo.InsertAnnotationParam{
		Tenant:      span.GetTenant(),
		TTL:         span.GetTTL(ctx),
		Annotations: []*loop_span.Annotation{annotation},
	})
}

func (r *TraceServiceImpl) Send(ctx context.Context, event *entity.AnnotationEvent) error {
	shouldReSend := false
	defer func() {
		event.RetryTimes--
		// resend if not success
		if !shouldReSend || event.RetryTimes <= 0 {
			return
		}
		logs.CtxInfo(ctx, "resend annotation event")
		_ = r.annotationProducer.SendAnnotation(ctx, event)
	}()
	cfg, err := r.getAnnotationCallerCfg(ctx, event.Caller)
	if err != nil { // retry
		return err
	}
	span, err := r.getSpan(ctx,
		cfg.Tenants,
		event.Annotation.SpanID,
		event.Annotation.TraceID,
		event.Annotation.WorkspaceID,
		event.StartAt,
		event.EndAt,
	)
	if err != nil || span == nil { // retry if not found yet
		shouldReSend = true
		return nil
	}
	event.Annotation.StartTime = time.UnixMicro(span.StartTime)
	if err := event.Annotation.GenID(); err != nil {
		logs.CtxWarn(ctx, "failed to generate annotation id for %+v, %v", event.Annotation, err)
		return nil
	}
	// retry if failed
	return r.traceRepo.InsertAnnotations(ctx, &repo.InsertAnnotationParam{
		Tenant:      span.GetTenant(),
		TTL:         span.GetTTL(ctx),
		Annotations: []*loop_span.Annotation{event.Annotation},
	})
}

func (r *TraceServiceImpl) getSpan(ctx context.Context, tenants []string, spanId, traceId, workspaceId string, startAt, endAt int64) (*loop_span.Span, error) {
	if spanId == "" || traceId == "" || workspaceId == "" {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	res, err := r.traceRepo.ListSpans(ctx, &repo.ListSpansParam{
		Tenants: tenants,
		Filters: &loop_span.FilterFields{
			FilterFields: []*loop_span.FilterField{
				{
					FieldName: loop_span.SpanFieldSpanId,
					FieldType: loop_span.FieldTypeString,
					Values:    []string{spanId},
					QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
				},
				{
					FieldName: loop_span.SpanFieldSpaceId,
					FieldType: loop_span.FieldTypeString,
					Values:    []string{workspaceId},
					QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
				},
				{
					FieldName: loop_span.SpanFieldTraceId,
					FieldType: loop_span.FieldTypeString,
					Values:    []string{traceId},
					QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
				},
			},
		},
		StartAt:            startAt,
		EndAt:              endAt,
		NotQueryAnnotation: true,
		Limit:              2,
	})
	if err != nil {
		logs.CtxError(ctx, "failed to list span, %v", err)
		return nil, err
	} else if len(res.Spans) == 0 {
		return nil, nil
	}
	return res.Spans[0], nil
}

func (r *TraceServiceImpl) getAnnotationCallerCfg(ctx context.Context, caller string) (*config.AnnotationConfig, error) {
	cfg, err := r.traceConfig.GetAnnotationSourceCfg(ctx)
	if err != nil {
		return nil, err
	}
	callerCfg, ok := cfg.SourceCfg[caller]
	if ok {
		return &callerCfg, nil
	}
	callerCfg, ok = cfg.SourceCfg["default"]
	if ok {
		return &callerCfg, nil
	}
	return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
}

func (r *TraceServiceImpl) buildBuiltinFilters(ctx context.Context, f span_filter.Filter, req *ListSpansReq) (*loop_span.FilterFields, error) {
	filters := make([]*loop_span.FilterField, 0)
	env := &span_filter.SpanEnv{
		WorkspaceId: req.WorkspaceID,
	}
	basicFilter, forceQuery, err := f.BuildBasicSpanFilter(ctx, env)
	if err != nil {
		return nil, err
	} else if len(basicFilter) == 0 && !forceQuery { // if it's null, no need to query from ck
		return nil, nil
	}
	filters = append(filters, basicFilter...)
	switch req.SpanListType {
	case loop_span.SpanListTypeRootSpan:
		subFilter, err := f.BuildRootSpanFilter(ctx, env)
		if err != nil {
			return nil, err
		}
		filters = append(filters, subFilter...)
	case loop_span.SpanListTypeLLMSpan:
		subFilter, err := f.BuildLLMSpanFilter(ctx, env)
		if err != nil {
			return nil, err
		}
		filters = append(filters, subFilter...)
	case loop_span.SpanListTypeAllSpan:
		subFilter, err := f.BuildALLSpanFilter(ctx, env)
		if err != nil {
			return nil, err
		}
		filters = append(filters, subFilter...)
	default:
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid span list type: %s"))
	}
	filterAggr := &loop_span.FilterFields{
		QueryAndOr:   ptr.Of(loop_span.QueryAndOrEnumAnd),
		FilterFields: filters,
	}
	return filterAggr, nil
}

func (r *TraceServiceImpl) combineFilters(filters ...*loop_span.FilterFields) *loop_span.FilterFields {
	filterAggr := &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
	}
	for _, f := range filters {
		if f == nil {
			continue
		}
		filterAggr.FilterFields = append(filterAggr.FilterFields, &loop_span.FilterField{
			QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
			SubFilter:  f,
		})
	}
	return filterAggr
}

func (r *TraceServiceImpl) getTenants(ctx context.Context, platform loop_span.PlatformType) ([]string, error) {
	return r.tenantProvider.GetTenantsByPlatformType(ctx, platform)
}

func processSpecificFilter(f *loop_span.FilterField) error {
	switch f.FieldName {
	case loop_span.SpanFieldStatus:
		if err := processStatusFilter(f); err != nil {
			return err
		}
	case loop_span.SpanFieldDuration,
		loop_span.SpanFieldLatencyFirstResp,
		loop_span.SpanFieldStartTimeFirstResp,
		loop_span.SpanFieldStartTimeFirstTokenResp,
		loop_span.SpanFieldLatencyFirstTokenResp,
		loop_span.SpanFieldReasoningDuration:
		if err := processLatencyFilter(f); err != nil {
			return err
		}
	}
	return nil
}

func processStatusFilter(f *loop_span.FilterField) error {
	if f.QueryType == nil || *f.QueryType != loop_span.QueryTypeEnumIn {
		return fmt.Errorf("status filter should use in operator")
	}
	f.FieldName = loop_span.SpanFieldStatusCode
	f.FieldType = loop_span.FieldTypeLong
	checkSuccess, checkError := false, false
	for _, val := range f.Values {
		switch val {
		case loop_span.SpanStatusSuccess:
			checkSuccess = true
		case loop_span.SpanStatusError:
			checkError = true
		default:
			return fmt.Errorf("invalid status code field value")
		}
	}
	if checkSuccess && checkError {
		f.QueryType = ptr.Of(loop_span.QueryTypeEnumAlwaysTrue)
		f.Values = nil
	} else if checkSuccess {
		f.Values = []string{"0"}
	} else if checkError {
		f.QueryType = ptr.Of(loop_span.QueryTypeEnumNotIn)
		f.Values = []string{"0"}
	} else {
		return fmt.Errorf("invalid status code query")
	}
	return nil
}

// ms -> us
func processLatencyFilter(f *loop_span.FilterField) error {
	if f.FieldType != loop_span.FieldTypeLong {
		return fmt.Errorf("latency field type should be long ")
	}
	micros := make([]string, 0)
	for _, val := range f.Values {
		integer, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("fail to parse long value %s, %v", val, err)
		}
		integer = time_util.MillSec2MicroSec(integer)
		micros = append(micros, strconv.FormatInt(integer, 10))
	}
	f.Values = micros
	return nil
}

//go:generate mockgen -destination=mocks/span_processor.go -package=mocks . TraceFilterProcessorBuilder
type TraceFilterProcessorBuilder interface {
	BuildPlatformRelatedFilter(context.Context, loop_span.PlatformType) (span_filter.Filter, error)
	BuildGetTraceProcessors(context.Context, span_processor.Settings) ([]span_processor.Processor, error)
	BuildListSpansProcessors(context.Context, span_processor.Settings) ([]span_processor.Processor, error)
	BuildAdvanceInfoProcessors(context.Context, span_processor.Settings) ([]span_processor.Processor, error)
	BuildIngestTraceProcessors(context.Context, span_processor.Settings) ([]span_processor.Processor, error)
	BuildSearchTraceOApiProcessors(context.Context, span_processor.Settings) ([]span_processor.Processor, error)
	BuildListSpansOApiProcessors(context.Context, span_processor.Settings) ([]span_processor.Processor, error)
}

type TraceFilterProcessorBuilderImpl struct {
	platformFilterFactory             span_filter.PlatformFilterFactory
	getTraceProcessorFactories        []span_processor.Factory
	listSpansProcessorFactories       []span_processor.Factory
	advanceInfoProcessorFactories     []span_processor.Factory
	ingestTraceProcessorFactories     []span_processor.Factory
	searchTraceOApiProcessorFactories []span_processor.Factory
	listSpansOApiProcessorFactories   []span_processor.Factory
}

func (t *TraceFilterProcessorBuilderImpl) BuildPlatformRelatedFilter(
	ctx context.Context,
	platformType loop_span.PlatformType,
) (span_filter.Filter, error) {
	return t.platformFilterFactory.GetFilter(ctx, platformType)
}

func (t *TraceFilterProcessorBuilderImpl) BuildGetTraceProcessors(
	ctx context.Context,
	set span_processor.Settings,
) ([]span_processor.Processor, error) {
	ret := make([]span_processor.Processor, 0)
	for _, factory := range t.getTraceProcessorFactories {
		p, err := factory.CreateProcessor(ctx, set)
		if err != nil {
			return nil, err
		}
		ret = append(ret, p)
	}
	return ret, nil
}

func (t *TraceFilterProcessorBuilderImpl) BuildListSpansProcessors(
	ctx context.Context,
	set span_processor.Settings,
) ([]span_processor.Processor, error) {
	ret := make([]span_processor.Processor, 0)
	for _, factory := range t.listSpansProcessorFactories {
		p, err := factory.CreateProcessor(ctx, set)
		if err != nil {
			return nil, err
		}
		ret = append(ret, p)
	}
	return ret, nil
}

func (t *TraceFilterProcessorBuilderImpl) BuildAdvanceInfoProcessors(
	ctx context.Context,
	set span_processor.Settings,
) ([]span_processor.Processor, error) {
	ret := make([]span_processor.Processor, 0)
	for _, factory := range t.advanceInfoProcessorFactories {
		p, err := factory.CreateProcessor(ctx, set)
		if err != nil {
			return nil, err
		}
		ret = append(ret, p)
	}
	return ret, nil
}

func (t *TraceFilterProcessorBuilderImpl) BuildIngestTraceProcessors(
	ctx context.Context,
	set span_processor.Settings,
) ([]span_processor.Processor, error) {
	ret := make([]span_processor.Processor, 0)
	for _, factory := range t.ingestTraceProcessorFactories {
		p, err := factory.CreateProcessor(ctx, set)
		if err != nil {
			return nil, err
		}
		ret = append(ret, p)
	}
	return ret, nil
}

func (t *TraceFilterProcessorBuilderImpl) BuildSearchTraceOApiProcessors(
	ctx context.Context,
	set span_processor.Settings,
) ([]span_processor.Processor, error) {
	ret := make([]span_processor.Processor, 0)
	for _, factory := range t.searchTraceOApiProcessorFactories {
		p, err := factory.CreateProcessor(ctx, set)
		if err != nil {
			return nil, err
		}
		ret = append(ret, p)
	}
	return ret, nil
}

func (t *TraceFilterProcessorBuilderImpl) BuildListSpansOApiProcessors(
	ctx context.Context,
	set span_processor.Settings,
) ([]span_processor.Processor, error) {
	ret := make([]span_processor.Processor, 0)
	for _, factory := range t.listSpansOApiProcessorFactories {
		p, err := factory.CreateProcessor(ctx, set)
		if err != nil {
			return nil, err
		}
		ret = append(ret, p)
	}
	return ret, nil
}

func NewTraceFilterProcessorBuilder(
	platformFilterFactory span_filter.PlatformFilterFactory,
	getTraceProcessorFactories []span_processor.Factory,
	listSpansProcessorFactories []span_processor.Factory,
	advanceInfoProcessorFactories []span_processor.Factory,
	ingestTraceProcessorFactories []span_processor.Factory,
	searchTraceOApiProcessorFactories []span_processor.Factory,
	listSpansOApiProcessorFactories []span_processor.Factory,
) TraceFilterProcessorBuilder {
	return &TraceFilterProcessorBuilderImpl{
		platformFilterFactory:             platformFilterFactory,
		getTraceProcessorFactories:        getTraceProcessorFactories,
		listSpansProcessorFactories:       listSpansProcessorFactories,
		advanceInfoProcessorFactories:     advanceInfoProcessorFactories,
		ingestTraceProcessorFactories:     ingestTraceProcessorFactories,
		searchTraceOApiProcessorFactories: searchTraceOApiProcessorFactories,
		listSpansOApiProcessorFactories:   listSpansOApiProcessorFactories,
	}
}
