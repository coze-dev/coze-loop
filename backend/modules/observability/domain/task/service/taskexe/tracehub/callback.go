// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
)

func (h *TraceHubServiceImpl) CallBack(ctx context.Context, event *entity.AutoEvalEvent) error {
	logs.CtxInfo(ctx, "CallBack msg %+v", event)
	for _, turn := range event.TurnEvalResults {
		workspaceIDStr, workspaceID := turn.GetWorkspaceIDFromExt()
		tenants, err := h.getTenants(ctx, loop_span.PlatformType("loop_all"))
		if err != nil {
			return err
		}
		var storageDuration int64 = 1
		//缓存做了吗？
		res, err := h.benefitSvc.CheckTraceBenefit(ctx, &benefit.CheckTraceBenefitParams{
			ConnectorUID: session.UserIDInCtxOrEmpty(ctx),
			SpaceID:      workspaceID,
		})
		if err != nil {
			logs.CtxWarn(ctx, "fail to check trace benefit, %v", err)
		} else if res == nil {
			logs.CtxWarn(ctx, "fail to get trace benefit, got nil response")
		} else if res != nil {
			storageDuration = res.StorageDuration
		}

		spans, err := h.getSpan(ctx,
			tenants,
			[]string{turn.GetSpanIDFromExt()},
			turn.GetTraceIDFromExt(),
			workspaceIDStr,
			turn.GetStartTimeFromExt()/1000-(24*time.Duration(storageDuration)*time.Hour).Milliseconds(),
			turn.GetStartTimeFromExt()/1000+10*time.Minute.Milliseconds(),
		)
		if err != nil {
			return err
		}
		if len(spans) == 0 {
			return fmt.Errorf("span not found, span_id: %s", turn.GetSpanIDFromExt())
		}
		span := spans[0]

		// Newly added: write Redis counters based on the Status
		err = h.updateTaskRunDetailsCount(ctx, turn.GetTaskIDFromExt(), turn)
		if err != nil {
			logs.CtxWarn(ctx, "更新TaskRun状态计数失败: taskID=%d, status=%d, err=%v",
				turn.GetTaskIDFromExt(), turn.Status, err)
			// Continue processing without interrupting the flow
		}

		annotation := &loop_span.Annotation{
			SpanID:         turn.GetSpanIDFromExt(),
			TraceID:        span.TraceID,
			WorkspaceID:    workspaceIDStr,
			AnnotationType: loop_span.AnnotationTypeAutoEvaluate,
			StartTime:      time.UnixMicro(span.StartTime),
			Key:            fmt.Sprintf("%d:%d", turn.GetTaskIDFromExt(), turn.EvaluatorVersionID),
			Value: loop_span.AnnotationValue{
				ValueType:  loop_span.AnnotationValueTypeDouble,
				FloatValue: turn.Score,
			},
			Reasoning: turn.Reasoning,
			Status:    loop_span.AnnotationStatusNormal,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err = annotation.GenID(); err != nil {
			return err
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

func (h *TraceHubServiceImpl) Correction(ctx context.Context, event *entity.CorrectionEvent) error {
	workspaceIDStr, workspaceID := event.GetWorkspaceIDFromExt()
	if workspaceID == 0 {
		return fmt.Errorf("workspace_id is empty")
	}
	tenants, err := h.getTenants(ctx, loop_span.PlatformType("loop_all"))
	if err != nil {
		return err
	}
	spans, err := h.getSpan(ctx,
		tenants,
		[]string{event.GetSpanIDFromExt()},
		event.GetTraceIDFromExt(),
		workspaceIDStr,
		event.GetStartTimeFromExt()/1000-time.Second.Milliseconds(),
		event.GetStartTimeFromExt()/1000+time.Second.Milliseconds(),
	)
	if err != nil {
		return err
	}
	if event.EvaluatorResult.Correction == nil || event.EvaluatorResult == nil {
		return err
	}
	if len(spans) == 0 {
		return fmt.Errorf("span not found, span_id: %s", event.GetSpanIDFromExt())
	}
	span := spans[0]
	annotations, err := h.traceRepo.ListAnnotations(ctx, &repo.ListAnnotationsParam{
		Tenants:     tenants,
		SpanID:      event.GetSpanIDFromExt(),
		TraceID:     event.GetTraceIDFromExt(),
		WorkspaceId: workspaceID,
		StartAt:     event.GetStartTimeFromExt() - 5*time.Second.Milliseconds(),
		EndAt:       event.GetStartTimeFromExt() + 5*time.Second.Milliseconds(),
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

	// Then synchronize the observability data
	param := &repo.UpsertAnnotationParam{
		Tenant:      span.GetTenant(),
		TTL:         span.GetTTL(ctx),
		Annotations: []*loop_span.Annotation{annotation},
		IsSync:      true,
	}
	if err = h.traceRepo.UpsertAnnotation(ctx, param); err != nil {
		recordID := lo.Ternary(annotation.GetAutoEvaluateMetadata() != nil, annotation.GetAutoEvaluateMetadata().EvaluatorRecordID, 0)
		// If the synchronous update fails, compensate asynchronously
		// TODO: asynchronous processing has issues and may duplicate
		logs.CtxWarn(ctx, "Sync upsert annotation failed, try async upsert. span_id=[%v], recored_id=[%v], err:%v",
			annotation.SpanID, recordID, err)
		return nil
	}
	return nil
}
