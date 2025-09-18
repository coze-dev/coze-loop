// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
)

func (h *TraceHubServiceImpl) CallBack(ctx context.Context, event *entity.AutoEvalEvent) error {
	err := h.upsertAnnotation(ctx, event.TurnEvalResults, false)
	if err != nil {
		logs.CtxError(ctx, "upsertAnnotation err:%v", err)
		return err
	}
	return nil
}
func (h *TraceHubServiceImpl) Correction(ctx context.Context, event *entity.CorrectionEvent) error {
	spanID := event.Ext["span_id"]
	traceID := event.Ext["trace_id"]
	startTimeStr := event.Ext["start_time"]
	startTime, err := strconv.ParseInt(startTimeStr, 10, 64)
	if err != nil {
		return err
	}
	logs.CtxInfo(ctx, "startTime: %v", startTime)
	workspaceIDStr := event.Ext["workspace_id"]
	workspaceID, err := strconv.ParseInt(workspaceIDStr, 10, 64)
	if err != nil {
		return err
	}
	//todo：loopspan下
	platform_type := event.Ext["platform_type"]
	tenants, err := h.getTenants(ctx, loop_span.PlatformType(platform_type))
	if err != nil {
		return err
	}
	spans, err := h.getSpan(ctx,
		tenants,
		[]string{spanID},
		traceID,
		workspaceIDStr,
		startTime/1000-time.Second.Milliseconds(),
		startTime/1000+time.Second.Milliseconds(),
	)
	if err != nil {
		return err
	}
	if event.EvaluatorResult.Correction == nil || event.EvaluatorResult == nil {
		return err
	}
	if len(spans) == 0 {
		return fmt.Errorf("span not found, span_id: %s", spanID)
	}
	span := spans[0]
	annotations, err := h.traceRepo.ListAnnotations(ctx, &repo.ListAnnotationsParam{
		Tenants:     tenants,
		SpanID:      spanID,
		TraceID:     traceID,
		WorkspaceId: workspaceID,
		StartAt:     startTime - 5*time.Second.Milliseconds(),
		EndAt:       startTime + 5*time.Second.Milliseconds(),
	})
	if err != nil {
		return err
	}
	var annotation *loop_span.Annotation
	for _, a := range annotations {
		meta := a.GetAutoEvaluateMetadata()
		if meta != nil && meta.EvaluatorRecordID == event.EvaluatorRecordID {
			annotation = a
			break
		}
	}

	updateBy := session.UserIDInCtxOrEmpty(ctx)
	if updateBy == "" {
		return err
	}
	annotation.CorrectAutoEvaluateScore(event.EvaluatorResult.Correction.Score, event.EvaluatorResult.Correction.Explain, updateBy)

	// 再同步修改观测数据
	param := &repo.UpsertAnnotationParam{
		Tenant:      span.GetTenant(),
		TTL:         span.GetTTL(ctx),
		Annotations: []*loop_span.Annotation{annotation},
		IsSync:      true,
	}
	if err = h.traceRepo.UpsertAnnotation(ctx, param); err != nil {
		recordID := lo.Ternary(annotation.GetAutoEvaluateMetadata() != nil, annotation.GetAutoEvaluateMetadata().EvaluatorRecordID, 0)
		// 如果同步修改失败，异步补偿
		// todo 异步有问题，会重复
		logs.CtxWarn(ctx, "Sync upsert annotation failed, try async upsert. span_id=[%v], recored_id=[%v], err:%v",
			annotation.SpanID, recordID, err)
		return nil
	}
	return nil
}

func (h *TraceHubServiceImpl) upsertAnnotation(ctx context.Context, turnEvalResults []*entity.OnlineExptTurnEvalResult, isSync bool) (err error) {
	for _, turn := range turnEvalResults {
		spanID := turn.Ext["span_id"]
		traceID := turn.Ext["trace_id"]
		startTimeStr := turn.Ext["start_time"]
		startTime, err := strconv.ParseInt(startTimeStr, 10, 64)
		if err != nil {
			return err
		}
		logs.CtxInfo(ctx, "startTime: %v", startTime)
		taskIDStr := turn.Ext["task_id"]
		taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
		if err != nil {
			return err
		}
		workspaceIDStr := turn.Ext["workspace_id"]
		platform_type := turn.Ext["platform_type"]
		tenants, err := h.getTenants(ctx, loop_span.PlatformType(platform_type))
		if err != nil {
			return err
		}
		spans, err := h.getSpan(ctx,
			tenants,
			[]string{spanID},
			traceID,
			workspaceIDStr,
			startTime/1000-5*time.Second.Milliseconds(),
			startTime/1000+5*time.Second.Milliseconds(),
		)
		if len(spans) == 0 {
			return fmt.Errorf("span not found, span_id: %s", spanID)
		}
		span := spans[0]
		annotation := &loop_span.Annotation{
			SpanID:         spanID,
			TraceID:        span.TraceID,
			WorkspaceID:    workspaceIDStr,
			AnnotationType: loop_span.AnnotationTypeAutoEvaluate,
			Key:            fmt.Sprintf("%d:%d", taskID, turn.EvaluatorVersionID),
			Value: loop_span.AnnotationValue{
				ValueType:  loop_span.AnnotationValueTypeDouble,
				FloatValue: turn.Score,
			},
			Reasoning: turn.Reasoning,
			Status:    loop_span.AnnotationStatusNormal,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err = h.traceRepo.InsertAnnotations(ctx, &repo.InsertAnnotationParam{
			Tenant:      span.GetTenant(),
			TTL:         span.GetTTL(ctx),
			Annotations: []*loop_span.Annotation{annotation},
		})
		if err != nil {
			return err
		}

	}
	return nil
}
func (h *TraceHubServiceImpl) getTenants(ctx context.Context, platform loop_span.PlatformType) ([]string, error) {
	return h.tenantProvider.GetTenantsByPlatformType(ctx, platform)
}
func (h *TraceHubServiceImpl) getSpan(ctx context.Context, tenants []string, spanIds []string, traceId, workspaceId string, startAt, endAt int64) ([]*loop_span.Span, error) {
	if len(spanIds) == 0 || workspaceId == "" {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	var filterFields []*loop_span.FilterField
	filterFields = append(filterFields, &loop_span.FilterField{
		FieldName: loop_span.SpanFieldSpanId,
		FieldType: loop_span.FieldTypeString,
		Values:    spanIds,
		QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
	})
	filterFields = append(filterFields, &loop_span.FilterField{
		FieldName: loop_span.SpanFieldSpaceId,
		FieldType: loop_span.FieldTypeString,
		Values:    []string{workspaceId},
		QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
	})
	if traceId != "" {
		filterFields = append(filterFields, &loop_span.FilterField{
			FieldName: loop_span.SpanFieldTraceId,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{traceId},
			QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
		})
	}
	res, err := h.traceRepo.ListSpans(ctx, &repo.ListSpansParam{
		Tenants: tenants,
		Filters: &loop_span.FilterFields{
			FilterFields: filterFields,
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
	return res.Spans, nil
}
