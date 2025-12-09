// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	tracerepo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
)

//go:generate mockgen -destination=mocks/task_callback_service.go -package=mocks . ITaskCallbackService
type ITaskCallbackService interface {
	AutoEvalCallback(ctx context.Context, event *entity.AutoEvalEvent) error
	AutoEvalCorrection(ctx context.Context, event *entity.CorrectionEvent) error
}

type TaskCallbackServiceImpl struct {
	taskRepo       repo.ITaskRepo
	traceRepo      tracerepo.ITraceRepo
	taskProcessor  processor.TaskProcessor
	tenantProvider tenant.ITenantProvider
	config         config.ITraceConfig
	benefitSvc     benefit.IBenefitService
}

func NewTaskCallbackServiceImpl(
	taskRepo repo.ITaskRepo,
	traceRepo tracerepo.ITraceRepo,
	taskProcessor processor.TaskProcessor,
	tenantProvider tenant.ITenantProvider,
	config config.ITraceConfig,
	benefitSvc benefit.IBenefitService,
) ITaskCallbackService {
	return &TaskCallbackServiceImpl{
		taskRepo:       taskRepo,
		traceRepo:      traceRepo,
		taskProcessor:  taskProcessor,
		tenantProvider: tenantProvider,
		config:         config,
		benefitSvc:     benefitSvc,
	}
}

func (t *TaskCallbackServiceImpl) AutoEvalCallback(ctx context.Context, event *entity.AutoEvalEvent) error {
	for _, turn := range event.TurnEvalResults {
		workspaceIDStr, workspaceID := turn.GetWorkspaceIDFromExt()
		platformType, ok := turn.GetPlatformType()
		if !ok {
			task, err := t.taskRepo.GetTask(ctx, turn.GetTaskIDFromExt(), nil, nil)
			if err != nil {
				return err
			}
			logs.CtxInfo(ctx, "PlatformType not found in message, get from task [%#v]", task)
			if task != nil && task.SpanFilter != nil {
				platformType = task.SpanFilter.PlatformType
			}
		}
		tenants, err := t.tenantProvider.GetTenantsByPlatformType(ctx, platformType)
		if err != nil {
			return err
		}
		storageDuration := t.config.GetTraceDataMaxDurationDay(ctx, lo.ToPtr(string(loop_span.PlatformDefault)))
		res, err := t.benefitSvc.CheckTraceBenefit(ctx, &benefit.CheckTraceBenefitParams{
			ConnectorUID: turn.GetUserID(),
			SpaceID:      workspaceID,
		})
		if err != nil {
			logs.CtxWarn(ctx, "fail to check trace benefit, %v", err)
		} else if res == nil {
			logs.CtxWarn(ctx, "fail to get trace benefit, got nil response")
		} else {
			storageDuration = res.StorageDuration
		}

		spans, err := t.getSpan(ctx,
			tenants,
			[]string{turn.GetSpanIDFromExt()},
			turn.GetTraceIDFromExt(),
			workspaceIDStr,
			turn.GetStartTimeFromExt(storageDuration),
			turn.GetEndTimeFromExt(),
		)
		if err != nil {
			return err
		}
		if len(spans) == 0 {
			logs.CtxWarn(ctx, "span not found, span_id: %s", turn.GetSpanIDFromExt())
			return fmt.Errorf("span not found, span_id: %s", turn.GetSpanIDFromExt())
		}
		span := spans[0]

		// Newly added: write Redis counters based on the Status
		err = t.updateTaskRunDetailsCount(ctx, turn.GetTaskIDFromExt(), turn, storageDuration*24*60*60*1000)
		if err != nil {
			logs.CtxWarn(ctx, "Update TaskRun count failed: taskID=%d, status=%d, err=%v",
				turn.GetTaskIDFromExt(), turn.Status, err)
			// Continue processing without interrupting the flow
		}

		_, err = span.AddAutoEvalAnnotation(
			turn.GetTaskIDFromExt(),
			turn.EvaluatorRecordID,
			turn.EvaluatorVersionID,
			turn.Score,
			turn.Reasoning,
			turn.GetUserID(),
		)
		if err != nil {
			return err
		}

		err = t.traceRepo.InsertAnnotations(ctx, &tracerepo.InsertAnnotationParam{
			WorkSpaceID:    workspaceIDStr,
			Tenant:         span.GetTenant(),
			TTL:            span.GetTTL(ctx),
			Span:           span,
			AnnotationType: gptr.Of(loop_span.AnnotationTypeAutoEvaluate),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *TaskCallbackServiceImpl) AutoEvalCorrection(ctx context.Context, event *entity.CorrectionEvent) error {
	workspaceIDStr, workspaceID := event.GetWorkspaceIDFromExt()
	if workspaceID == 0 {
		return fmt.Errorf("workspace_id is empty")
	}
	platformType, ok := event.GetPlatformType()
	if !ok {
		task, err := t.taskRepo.GetTask(ctx, event.GetTaskIDFromExt(), nil, nil)
		if err != nil {
			return err
		}
		logs.CtxInfo(ctx, "PlatformType not found in message, get from task [%#v]", task)
		if task != nil && task.SpanFilter != nil {
			platformType = task.SpanFilter.PlatformType
		}
	}
	tenants, err := t.tenantProvider.GetTenantsByPlatformType(ctx, platformType)
	if err != nil {
		return err
	}
	spans, err := t.getSpan(ctx,
		tenants,
		[]string{event.GetSpanIDFromExt()},
		event.GetTraceIDFromExt(),
		workspaceIDStr,
		event.GetStartTimeFromExt(),
		event.GetEndTimeFromExt(),
	)
	if err != nil {
		return err
	}
	if len(spans) == 0 {
		return fmt.Errorf("span not found, span_id: %s", event.GetSpanIDFromExt())
	}
	span := spans[0]

	annotations, err := t.traceRepo.ListAnnotations(ctx, &tracerepo.ListAnnotationsParam{
		Tenants:     tenants,
		SpanID:      event.GetSpanIDFromExt(),
		TraceID:     event.GetTraceIDFromExt(),
		WorkspaceId: workspaceID,
		StartAt:     event.GetStartTimeFromExt(),
		EndAt:       event.GetEndTimeFromExt(),
	})
	if err != nil {
		return err
	}

	annotation, ok := annotations.FindByEvaluatorRecordID(event.EvaluatorRecordID)
	if !ok {
		logs.CtxError(ctx, "annotation not found, evaluator_record_id: %d", event.EvaluatorRecordID)
		return fmt.Errorf("annotation not found, evaluator_record_id: %d", event.EvaluatorRecordID)
	}

	annotation.CorrectAutoEvaluateScore(event.EvaluatorResult.Correction.Score, event.EvaluatorResult.Correction.Explain, event.GetUpdateBy())
	span.Annotations = make(loop_span.AnnotationList, 0)
	span.Annotations[0] = annotation

	// Then synchronize the observability data
	param := &tracerepo.InsertAnnotationParam{
		WorkSpaceID:    workspaceIDStr,
		Tenant:         span.GetTenant(),
		TTL:            span.GetTTL(ctx),
		Span:           span,
		AnnotationType: gptr.Of(loop_span.AnnotationTypeAutoEvaluate),
	}
	if err = t.traceRepo.InsertAnnotations(ctx, param); err != nil {
		recordID := lo.Ternary(annotation.GetAutoEvaluateMetadata() != nil, annotation.GetAutoEvaluateMetadata().EvaluatorRecordID, 0)
		// If the synchronous update fails, compensate asynchronously
		// TODO: asynchronous processing has issues and may duplicate
		logs.CtxError(ctx, "Sync upsert annotation failed, try async upsert. span_id=[%v], recored_id=[%v], err:%v",
			annotation.SpanID, recordID, err)
		return nil
	}
	return nil
}

func (t *TaskCallbackServiceImpl) getSpan(ctx context.Context, tenants []string, spanIds []string, traceId, workspaceId string, startAt, endAt int64) ([]*loop_span.Span, error) {
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
	var spans []*loop_span.Span
	// todo 目前可能有不同tenant在不同存储中，需要上层多次查询。后续逻辑需要下沉到repo中。
	for _, tenant := range tenants {
		res, err := t.traceRepo.ListSpans(ctx, &tracerepo.ListSpansParam{
			WorkSpaceID: workspaceId,
			Tenants:     []string{tenant},
			Filters: &loop_span.FilterFields{
				FilterFields: filterFields,
			},
			StartAt:            startAt,
			EndAt:              endAt,
			NotQueryAnnotation: true,
			Limit:              int32(len(spanIds)),
		})
		if err != nil {
			logs.CtxError(ctx, "failed to list span, %v", err)
			return spans, err
		}
		spans = append(spans, res.Spans...)
	}
	logs.CtxInfo(ctx, "list span, spans: %v", spans)

	return spans, nil
}

// updateTaskRunStatusCount updates the Redis count based on Status
func (t *TaskCallbackServiceImpl) updateTaskRunDetailsCount(ctx context.Context, taskID int64, turn *entity.OnlineExptTurnEvalResult, ttl int64) error {
	taskRunID, err := turn.GetRunID()
	if err != nil {
		return fmt.Errorf("invalid task_run_id, err: %v", err)
	}
	// Increase the corresponding counter based on Status
	switch turn.Status {
	case entity.EvaluatorRunStatus_Success:
		return t.taskRepo.IncrTaskRunSuccessCount(ctx, taskID, taskRunID, ttl)
	case entity.EvaluatorRunStatus_Fail:
		return t.taskRepo.IncrTaskRunFailCount(ctx, taskID, taskRunID, ttl)
	default:
		logs.CtxWarn(ctx, "unknown status, skip count: taskID=%d, taskRunID=%d, status=%d",
			taskID, taskRunID, turn.Status)
		return nil
	}
}
