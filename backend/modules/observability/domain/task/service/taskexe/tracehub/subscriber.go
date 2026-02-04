// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"math/rand"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type spanSubscriber struct {
	taskID       int64
	t            *entity.ObservabilityTask
	tr           *entity.TaskRun
	processor    taskexe.Processor
	tenants      []string
	taskRepo     repo.ITaskRepo
	runType      entity.TaskRunType
	buildHelper  service.TraceFilterProcessorBuilder
	traceService service.ITraceService
}

// Sampled determines whether a span is sampled based on the sampling rate; the sample size will be validated during flush.
func (s *spanSubscriber) Sampled() bool {
	if s.t == nil || s.t.Sampler == nil {
		return false
	}

	const base = 10000
	threshold := int64(float64(base) * s.t.Sampler.SampleRate)
	r := rand.Int63n(base)
	return r <= threshold
}

func combineFilters(filters ...*loop_span.FilterFields) *loop_span.FilterFields {
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

// Match checks whether the span matches the task filter.
func (s *spanSubscriber) Match(ctx context.Context, span *loop_span.Span) (bool, error) {
	task := s.t
	if task == nil {
		return false, nil
	}

	filters := s.buildSpanFilters(ctx, task, span)
	if !filters.Satisfied(span) {
		return false, nil
	}

	return true, nil
}

func (s *spanSubscriber) buildSpanFilters(ctx context.Context, taskDO *entity.ObservabilityTask, span *loop_span.Span) *loop_span.FilterFields {
	// Additional filters can be constructed based on task configuration if needed.
	// Simplified handling here: returning nil means no extra filters are applied.
	filters := &loop_span.FilterFields{}
	platformFilter, err := s.buildHelper.BuildPlatformRelatedFilter(ctx, taskDO.SpanFilter.PlatformType)
	if err != nil {
		return filters
	}
	builtinFilter, err := buildBuiltinFilters(ctx, platformFilter, &ListSpansReq{
		WorkspaceID:  taskDO.WorkspaceID,
		SpanListType: taskDO.SpanFilter.SpanListType,
	})
	if err != nil {
		return filters
	}
	if err = taskDO.SpanFilter.Filters.Traverse(processSpecificFilter); err != nil {
		logs.CtxError(ctx, "traverse filter fields failed, task_id=%d, err=%v", taskDO.ID, err)
		return filters
	}
	var tenantFilter *loop_span.FilterFields = nil
	if len(span.GetTenant()) > 0 {
		tenantFilter = buildTenantFilter(s.tenants)
	}
	filters = combineFilters(builtinFilter, &taskDO.SpanFilter.Filters, tenantFilter)

	return filters
}

func buildTenantFilter(tenants []string) *loop_span.FilterFields {
	return &loop_span.FilterFields{
		FilterFields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldTenant,
				FieldType: loop_span.FieldTypeString,
				Values:    tenants,
				QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				IsSystem:  true,
			},
		},
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
	}
}

func buildBuiltinFilters(ctx context.Context, f span_filter.Filter, req *ListSpansReq) (*loop_span.FilterFields, error) {
	filters := make([]*loop_span.FilterField, 0)
	env := &span_filter.SpanEnv{
		WorkspaceID:           req.WorkspaceID,
		ThirdPartyWorkspaceID: req.ThirdPartyWorkspaceID,
		Source:                span_filter.SourceTypeAutoTask,
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

func (s *spanSubscriber) Creative(ctx context.Context, runStartAt, runEndAt int64) error {
	err := s.processor.OnTaskRunCreated(ctx, taskexe.OnTaskRunCreatedReq{
		CurrentTask: s.t,
		RunType:     s.runType,
		RunStartAt:  runStartAt,
		RunEndAt:    runEndAt,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *spanSubscriber) AddSpan(ctx context.Context, span *loop_span.Span) error {
	var taskRunConfig *entity.TaskRun
	var err error
	if s.runType == entity.TaskRunTypeNewData {
		taskRunConfig, err = s.taskRepo.GetLatestNewDataTaskRun(ctx, nil, s.t.ID)
		if err != nil {
			logs.CtxWarn(ctx, "get latest new data task run failed, task_id=%d, err: %v", s.t.ID, err)
			return err
		}
	} else {
		taskRunConfig, err = s.taskRepo.GetBackfillTaskRun(ctx, nil, s.t.ID)
		if err != nil {
			logs.CtxWarn(ctx, "get backfill task run failed, task_id=%d, err: %v", s.t.ID, err)
			return err
		}
	}

	if taskRunConfig == nil {
		logs.CtxWarn(ctx, "no taskRunConfig：%v", taskRunConfig)
		return nil
	}
	// 仅允许处于 running 状态的 TaskRun 继续触发处理器，避免已结束 run 仍被触发
	if taskRunConfig.RunStatus != entity.TaskRunStatusRunning {
		logs.CtxInfo(ctx, "skip non-running task run: task_id=%d, run_id=%d, status=%s, span_id=%s", s.t.ID, taskRunConfig.ID, taskRunConfig.RunStatus, span.SpanID)
		return nil
	}

	if taskRunConfig.RunEndAt.UnixMilli() < time.Now().UnixMilli() || taskRunConfig.RunStartAt.UnixMilli() > time.Now().UnixMilli() {
		return nil
	}
	if span.StartTime < taskRunConfig.RunStartAt.UnixMilli() {
		logs.CtxWarn(ctx, "span start time is before task cycle start time, trace_id=%s, span_id=%s", span.TraceID, span.SpanID)
		return nil
	}
	trigger := &taskexe.Trigger{Task: s.t, Span: span, TaskRun: taskRunConfig}
	logs.CtxDebug(ctx, "invoke processor, trigger: %v", trigger)
	// New Data 在这里处理
	// Back fill 在前置批量处理
	if s.runType == entity.TaskRunTypeNewData {
		span.EnsurePreviousResponseID(ctx)
		err := s.traceService.MergeHistoryMessagesByRespIDBatch(ctx, []*loop_span.Span{span}, s.t.GetPlatformType())
		if err != nil {
			logs.CtxError(ctx, "merge history messages failed, task_id=%d, span_id=%s err: %v", s.t.ID, span.SpanID, err)
			return err
		}
	}
	err = s.processor.Invoke(ctx, trigger)
	if err != nil {
		logs.CtxWarn(ctx, "invoke processor failed, trace_id=%s, span_id=%s, err: %v", span.TraceID, span.SpanID, err)
		return err
	}

	return nil
}
