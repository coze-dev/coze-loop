// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe"
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
	sync.RWMutex // protect t, buf
	t            *task.Task
	processor    taskexe.Processor
	buf          []*loop_span.Span
	bufCap       int // max buffer size

	flushWait        sync.WaitGroup
	maxFlushInterval time.Duration
	taskRepo         repo.ITaskRepo
	taskRunRepo      repo.ITaskRunRepo
	runType          task.TaskRunType
	buildHelper      service.TraceFilterProcessorBuilder
}

// Sampled 根据采样率计算是否被采样；采样数量将在 flush 时强制校验。
func (s *spanSubscriber) Sampled() bool {
	t := s.getTask()
	if t == nil || t.Rule == nil || t.Rule.Sampler == nil {
		return false
	}

	const base = 10000
	threshold := int64(float64(base) * t.GetRule().GetSampler().GetSampleRate())
	r := rand.Int63n(base) // todo: rand seed
	return r <= threshold
}
func (s *spanSubscriber) getTask() *task.Task {
	s.RLock()
	defer s.RUnlock()
	return s.t
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

// Match 检查 span 是否与 task 的 filter 匹配。
func (s *spanSubscriber) Match(ctx context.Context, span *loop_span.Span) (bool, error) {

	task := s.t
	if task == nil || task.Rule == nil {
		return false, nil
	}
	//var customFilterFields, obsFilterFields, filterFields []*loop_span.FilterField
	//platformFilter, err := s.buildHelper.BuildPlatformRelatedFilter(context.Background(), loop_span.PlatformType(task.Rule.SpanFilters.GetPlatformType()))
	//if err != nil {
	//	return false, err
	//}
	//builtinFilter, err := buildBuiltinFilters(ctx, platformFilter, &ListSpansReq{
	//	WorkspaceID:  task.GetWorkspaceID(),
	//	SpanListType: loop_span.SpanListType(task.GetRule().GetSpanFilters().GetSpanListType()),
	//})
	//if err != nil {
	//	return false, err
	//}
	//if builtinFilter == nil {
	//	return false, err
	//}
	//filters := combineFilters(builtinFilter, convertor.FilterFieldsDTO2DO(task.GetRule().GetSpanFilters().GetFilters()))

	//for _, v := range builtinFilter.FilterFields {
	//	obsFilterFields = append(obsFilterFields, &loop_span.FilterField{
	//		FieldName:  v.FieldName,
	//		FieldType:  v.FieldType,
	//		Values:     v.Values,
	//		QueryType:  v.QueryType,
	//		QueryAndOr: v.QueryAndOr,
	//		SubFilter:  v.SubFilter,
	//	})
	//}
	//filterFields = append(filterFields, obsFilterFields...)
	//for _, v := range task.Rule.SpanFilters.Filters.FilterFields {
	//	customFilterFields = append(customFilterFields, &loop_span.FilterField{
	//		FieldName:  v.GetFieldName(),
	//		FieldType:  loop_span.FieldType(v.GetFieldType()),
	//		Values:     v.Values,
	//		QueryType:  ptr.Of(loop_span.QueryTypeEnum(v.GetQueryType())),
	//		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnum(v.GetQueryAndOr())),
	//	})
	//}
	//filterFields = append(filterFields, customFilterFields...)
	//filter := &loop_span.FilterFields{
	//	FilterFields: filterFields,
	//	QueryAndOr:   gptr.Of(loop_span.QueryAndOrEnumAnd),
	//}
	filters := s.buildSpanFilters(ctx, task)
	if !filters.Satisfied(span) {
		return false, nil
	}

	return true, nil
}
func (s *spanSubscriber) buildSpanFilters(ctx context.Context, taskConfig *task.Task) *loop_span.FilterFields {
	// 可以根据任务配置构建更复杂的过滤条件
	// 这里简化处理，返回 nil 表示不添加额外过滤

	platformFilter, err := s.buildHelper.BuildPlatformRelatedFilter(ctx, loop_span.PlatformType(taskConfig.GetRule().GetSpanFilters().GetPlatformType()))
	if err != nil {
		return nil
	}
	builtinFilter, err := buildBuiltinFilters(ctx, platformFilter, &ListSpansReq{
		WorkspaceID:  taskConfig.GetWorkspaceID(),
		SpanListType: loop_span.SpanListType(taskConfig.GetRule().GetSpanFilters().GetSpanListType()),
	})
	if err != nil {
		return nil
	}
	filters := combineFilters(builtinFilter, convertor.FilterFieldsDTO2DO(taskConfig.GetRule().GetSpanFilters().GetFilters()))

	return filters
}
func buildBuiltinFilters(ctx context.Context, f span_filter.Filter, req *ListSpansReq) (*loop_span.FilterFields, error) {
	filters := make([]*loop_span.FilterField, 0)
	env := &span_filter.SpanEnv{
		WorkspaceID:           req.WorkspaceID,
		ThirdPartyWorkspaceID: req.ThirdPartyWorkspaceID,
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

func (s *spanSubscriber) Creative(ctx context.Context) error {
	err := s.processor.OnChangeProcessor(ctx, &taskexe.Config{
		Task: s.t,
	}, false)
	if err != nil {
		return err
	}
	return nil
}

func (s *spanSubscriber) AddSpan(ctx context.Context, span *loop_span.Span) error {
	var taskRunConfig *entity.TaskRun
	var err error
	if s.runType == task.TaskRunTypeNewData {
		taskRunConfig, err = s.taskRunRepo.GetLatestNewDataTaskRun(ctx, nil, s.t.GetID())
		if err != nil {
			logs.CtxWarn(ctx, "get latest new data task run failed, task_id=%d, err: %v", s.t.GetID(), err)
			return err
		}
	} else {
		taskRunConfig, err = s.taskRunRepo.GetBackfillTaskRun(ctx, nil, s.t.GetID())
		if err != nil {
			logs.CtxWarn(ctx, "get backfill task run failed, task_id=%d, err: %v", s.t.GetID(), err)
			return err
		}
	}

	if taskRunConfig == nil {
		logs.CtxWarn(ctx, "no taskRunConfig：%v", taskRunConfig)
		return nil
	}

	if taskRunConfig.RunEndAt.UnixMilli() < time.Now().UnixMilli() || taskRunConfig.RunStartAt.UnixMilli() > time.Now().UnixMilli() {
		return nil
	}
	if span.StartTime < taskRunConfig.RunStartAt.UnixMilli() {
		logs.CtxWarn(ctx, "span start time is before task cycle start time, trace_id=%s, span_id=%s", span.TraceID, span.SpanID)
		return nil
	}
	trigger := &taskexe.Trigger{Task: s.t, Span: span}
	logs.CtxInfo(ctx, "invoke processor, trigger: %v", trigger)
	err = s.processor.Invoke(ctx, taskRunConfig, trigger)
	if err != nil {
		logs.CtxWarn(ctx, "invoke processor failed, trace_id=%s, span_id=%s, err: %v", span.TraceID, span.SpanID, err)
		return err
	}

	return nil
}
