// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	lockmock "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tenant_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	taskrepo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	trepo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	builder_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	spanfilter_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_processor"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TestTraceHubServiceImpl_SetBackfillTask(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	taskProcessor := processor.NewTaskProcessor()
	proc := &stubProcessor{}
	taskProcessor.Register(entity.TaskTypeAutoEval, proc)

	impl := &TraceHubServiceImpl{
		taskRepo:      mockRepo,
		taskProcessor: taskProcessor,
	}

	now := time.Now()
	obsTask := &entity.ObservabilityTask{
		ID:          1,
		WorkspaceID: 1,
		TaskType:    entity.TaskTypeAutoEval,
		SpanFilter: &entity.SpanFilterFields{
			Filters: loop_span.FilterFields{
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
			},
		},
		Sampler: &entity.Sampler{
			SampleRate: 1,
			SampleSize: 10,
		},
		EffectiveTime: &entity.EffectiveTime{StartAt: now.UnixMilli(), EndAt: now.Add(time.Hour).UnixMilli()},
	}
	backfillRun := &entity.TaskRun{
		ID:          2,
		TaskID:      1,
		WorkspaceID: 1,
		TaskType:    entity.TaskRunTypeBackFill,
		RunStatus:   entity.TaskRunStatusRunning,
		RunStartAt:  now.Add(-time.Minute),
		RunEndAt:    now.Add(time.Minute),
	}

	mockRepo.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Nil(), gomock.Nil()).Return(obsTask, nil)
	mockRepo.EXPECT().GetBackfillTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), int64(1)).Return(backfillRun, nil)

	sub, err := impl.buildSubscriber(context.Background(), &entity.BackFillEvent{TaskID: 1})
	require.NoError(t, err)
	require.NotNil(t, sub)
	require.Equal(t, int64(1), sub.taskID)
	require.Equal(t, entity.TaskRunTypeBackFill, sub.runType)
}

func TestTraceHubServiceImpl_SetBackfillTaskNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	mockRepo.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Nil(), gomock.Nil()).Return(nil, nil)

	_, err := impl.buildSubscriber(context.Background(), &entity.BackFillEvent{TaskID: 1})
	require.Error(t, err)
}

func TestTraceHubServiceImpl_ProcessBatchSpans_TaskLimit(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	proc := &stubProcessor{}

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	now := time.Now()
	sampler := &task.Sampler{
		SampleRate:    floatPtr(1),
		SampleSize:    int64Ptr(1),
		IsCycle:       boolPtr(false),
		CycleInterval: int64Ptr(0),
	}
	taskDTO := &task.Task{
		ID:          ptr.Of(int64(1)),
		WorkspaceID: ptr.Of(int64(1)),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusRunning),
		Rule: &task.Rule{
			Sampler: sampler,
			EffectiveTime: &task.EffectiveTime{
				StartAt: ptr.Of(now.Add(-time.Hour).UnixMilli()),
				EndAt:   ptr.Of(now.Add(time.Hour).UnixMilli()),
			},
		},
	}
	taskRunDTO := &task.TaskRun{
		ID:            10,
		TaskRunConfig: &task.TaskRunConfig{},
		RunStatus:     task.RunStatusRunning,
		RunStartAt:    now.Add(-time.Minute).UnixMilli(),
		RunEndAt:      now.Add(time.Minute).UnixMilli(),
	}
	sub := &spanSubscriber{
		taskID:    1,
		t:         taskDTO,
		tr:        taskRunDTO,
		processor: proc,
		taskRepo:  mockRepo,
	}

	mockRepo.EXPECT().GetTaskCount(gomock.Any(), int64(1)).Return(int64(1), nil)
	mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), int64(1), int64(10)).Return(int64(0), nil)

	spans := []*loop_span.Span{{SpanID: "span-1"}}
	ctx := context.Background()

	require.NoError(t, impl.processBatchSpans(ctx, spans, sub))
	require.Equal(t, 1, proc.finishChangeInvoked)
}

func TestTraceHubServiceImpl_ProcessBatchSpans_DispatchError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	proc := &stubProcessor{invokeErr: errors.New("invoke fail")}

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	now := time.Now()
	sampler := &task.Sampler{
		SampleRate:    floatPtr(1),
		SampleSize:    int64Ptr(2),
		IsCycle:       boolPtr(false),
		CycleInterval: int64Ptr(0),
	}
	taskDTO := &task.Task{
		ID:          ptr.Of(int64(1)),
		WorkspaceID: ptr.Of(int64(1)),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusRunning),
		Rule: &task.Rule{
			Sampler: sampler,
			EffectiveTime: &task.EffectiveTime{
				StartAt: ptr.Of(now.Add(-time.Hour).UnixMilli()),
				EndAt:   ptr.Of(now.Add(time.Hour).UnixMilli()),
			},
		},
	}
	taskRunDTO := &task.TaskRun{
		ID:         10,
		RunStatus:  task.RunStatusRunning,
		RunStartAt: now.Add(-time.Minute).UnixMilli(),
		RunEndAt:   now.Add(time.Minute).UnixMilli(),
	}
	sub := &spanSubscriber{
		taskID:    1,
		t:         taskDTO,
		tr:        taskRunDTO,
		processor: proc,
		runType:   entity.TaskRunTypeNewData,
		taskRepo:  mockRepo,
	}

	spanRun := &entity.TaskRun{
		ID:          20,
		TaskID:      1,
		WorkspaceID: 1,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   entity.TaskRunStatusRunning,
		RunStartAt:  now.Add(-time.Minute),
		RunEndAt:    now.Add(time.Minute),
	}

	mockRepo.EXPECT().GetTaskCount(gomock.Any(), int64(1)).Return(int64(0), nil)
	mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), int64(1), int64(10)).Return(int64(0), nil)
	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.Nil(), int64(1)).Return(spanRun, nil)

	spans := []*loop_span.Span{{SpanID: "span-1", StartTime: now.Add(10 * time.Millisecond).UnixMilli(), WorkspaceID: "space", TraceID: "trace"}}

	err := impl.processBatchSpans(context.Background(), spans, sub)
	require.Error(t, err)
	require.ErrorContains(t, err, "invoke fail")
}

func TestTraceHubServiceImpl_BackFill_LockError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	locker := lockmock.NewMockILocker(ctrl)
	impl := &TraceHubServiceImpl{locker: locker}

	event := &entity.BackFillEvent{TaskID: 123}
	lockErr := errors.New("lock failed")
	locker.EXPECT().LockWithRenew(gomock.Any(), fmt.Sprintf(backfillLockKeyTemplate, event.TaskID), transformTaskStatusLockTTL, backfillLockMaxHold).
		Return(false, context.Background(), func() {}, lockErr)

	err := impl.BackFill(context.Background(), event)
	require.Error(t, err)
	require.ErrorIs(t, err, lockErr)
}

func TestTraceHubServiceImpl_BackFill_LockHeldByOthers(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	locker := lockmock.NewMockILocker(ctrl)
	impl := &TraceHubServiceImpl{locker: locker}

	event := &entity.BackFillEvent{TaskID: 456}
	locker.EXPECT().LockWithRenew(gomock.Any(), fmt.Sprintf(backfillLockKeyTemplate, event.TaskID), transformTaskStatusLockTTL, backfillLockMaxHold).
		Return(false, context.Background(), func() {}, nil)

	err := impl.BackFill(context.Background(), event)
	require.NoError(t, err)
}

func TestTraceHubServiceImpl_ListAndSendSpans_GetTenantsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	tenantProvider := tenant_mocks.NewMockITenantProvider(ctrl)
	impl := &TraceHubServiceImpl{tenantProvider: tenantProvider}

	now := time.Now()
	taskStatus := task.TaskStatusRunning
	sub := &spanSubscriber{
		t: &task.Task{
			ID:          ptr.Of(int64(1)),
			Name:        "task",
			WorkspaceID: ptr.Of(int64(2)),
			TaskType:    task.TaskTypeAutoEval,
			TaskStatus:  &taskStatus,
			Rule: &task.Rule{
				SpanFilters: &filter.SpanFilterFields{
					PlatformType: ptr.Of(common.PlatformType(common.PlatformTypeCozeBot)),
					SpanListType: ptr.Of(common.SpanListTypeRootSpan),
					Filters:      &filter.FilterFields{FilterFields: []*filter.FilterField{}},
				},
				BackfillEffectiveTime: &task.EffectiveTime{
					StartAt: ptr.Of(now.Add(-time.Hour).UnixMilli()),
					EndAt:   ptr.Of(now.UnixMilli()),
				},
			},
		},
		tr: &task.TaskRun{},
	}

	tenantErr := errors.New("tenant failed")
	tenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformType(common.PlatformTypeCozeBot)).
		Return(nil, tenantErr)

	err := impl.listAndSendSpans(context.Background(), sub)
	require.Error(t, err)
	require.ErrorIs(t, err, tenantErr)
}

func TestTraceHubServiceImpl_ListAndSendSpans_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockTaskRepo := repo_mocks.NewMockITaskRepo(ctrl)
	mockTraceRepo := trepo_mocks.NewMockITraceRepo(ctrl)
	mockTenant := tenant_mocks.NewMockITenantProvider(ctrl)
	mockBuilder := builder_mocks.NewMockTraceFilterProcessorBuilder(ctrl)
	filterMock := spanfilter_mocks.NewMockFilter(ctrl)

	impl := &TraceHubServiceImpl{
		taskRepo:       mockTaskRepo,
		traceRepo:      mockTraceRepo,
		tenantProvider: mockTenant,
		buildHelper:    mockBuilder,
	}

	now := time.Now()
	sub, proc := newBackfillSubscriber(mockTaskRepo, now)
	sub.tr.BackfillRunDetail = &task.BackfillDetail{LastSpanPageToken: ptr.Of("prev")}
	domainRun := newDomainBackfillTaskRun(now)
	span := newTestSpan(now)

	mockBuilder.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformType(common.PlatformTypeCozeBot)).
		Return(filterMock, nil)
	filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, true, nil)
	filterMock.EXPECT().BuildRootSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, nil)
	mockBuilder.EXPECT().BuildGetTraceProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor(nil), nil)
	mockTenant.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformType(common.PlatformTypeCozeBot)).Return([]string{"tenant"}, nil)

	mockTraceRepo.EXPECT().ListSpans(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param *repo.ListSpansParam) (*repo.ListSpansResult, error) {
		require.Equal(t, "tenant", param.Tenants[0])
		require.Equal(t, "prev", param.PageToken)
		return &repo.ListSpansResult{
			Spans:     loop_span.SpanList{span},
			PageToken: "next",
			HasMore:   false,
		}, nil
	})

	mockTaskRepo.EXPECT().GetTaskCount(gomock.Any(), int64(1)).Return(int64(0), nil)
	mockTaskRepo.EXPECT().GetTaskRunCount(gomock.Any(), int64(1), sub.tr.ID).Return(int64(0), nil)
	mockTaskRepo.EXPECT().GetBackfillTaskRun(gomock.Any(), gomock.Nil(), int64(1)).Return(domainRun, nil)
	mockTaskRepo.EXPECT().UpdateTaskRunWithOCC(gomock.Any(), sub.tr.ID, sub.tr.WorkspaceID, gomock.AssignableToTypeOf(map[string]interface{}{})).Return(nil)

	err := impl.listAndSendSpans(context.Background(), sub)
	require.NoError(t, err)
	require.True(t, proc.invokeCalled)
	require.NotNil(t, sub.tr.BackfillRunDetail)
	require.Equal(t, "next", sub.tr.BackfillRunDetail.GetLastSpanPageToken())
}

func TestTraceHubServiceImpl_FetchAndSendSpans_ListError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockTaskRepo := repo_mocks.NewMockITaskRepo(ctrl)
	mockTraceRepo := trepo_mocks.NewMockITraceRepo(ctrl)
	impl := &TraceHubServiceImpl{
		taskRepo:  mockTaskRepo,
		traceRepo: mockTraceRepo,
	}

	now := time.Now()
	sub, _ := newBackfillSubscriber(mockTaskRepo, now)

	mockTraceRepo.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, errors.New("list failed"))

	err := impl.fetchAndSendSpans(context.Background(), &repo.ListSpansParam{Tenants: []string{"tenant"}}, sub)
	require.Error(t, err)
}

func TestTraceHubServiceImpl_FlushSpans_ContextCanceled(t *testing.T) {
	impl := &TraceHubServiceImpl{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := impl.flushSpans(ctx, &flushReq{}, &spanSubscriber{})
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestTraceHubServiceImpl_DoFlush_UpdateTaskRunError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockTaskRepo := repo_mocks.NewMockITaskRepo(ctrl)
	impl := &TraceHubServiceImpl{taskRepo: mockTaskRepo}

	now := time.Now()
	sub, _ := newBackfillSubscriber(mockTaskRepo, now)
	span := newTestSpan(now)
	domainRun := newDomainBackfillTaskRun(now)

	mockTaskRepo.EXPECT().GetTaskCount(gomock.Any(), int64(1)).Return(int64(0), nil)
	mockTaskRepo.EXPECT().GetTaskRunCount(gomock.Any(), int64(1), sub.tr.ID).Return(int64(0), nil)
	mockTaskRepo.EXPECT().GetBackfillTaskRun(gomock.Any(), gomock.Nil(), int64(1)).Return(domainRun, nil)
	mockTaskRepo.EXPECT().UpdateTaskRunWithOCC(gomock.Any(), sub.tr.ID, sub.tr.WorkspaceID, gomock.AssignableToTypeOf(map[string]interface{}{})).Return(errors.New("update fail"))

	flushed, sampled, err := impl.doFlush(context.Background(), &flushReq{retrievedSpanCount: 1, pageToken: "token", spans: []*loop_span.Span{span}}, sub)
	require.Equal(t, 1, flushed)
	require.Equal(t, 1, sampled)
	require.Error(t, err)
}

func TestTraceHubServiceImpl_DoFlush_NoMoreFinishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockTaskRepo := repo_mocks.NewMockITaskRepo(ctrl)
	impl := &TraceHubServiceImpl{taskRepo: mockTaskRepo}

	now := time.Now()
	sub, proc := newBackfillSubscriber(mockTaskRepo, now)
	proc.finishErr = errors.New("finish fail")
	span := newTestSpan(now)
	domainRun := newDomainBackfillTaskRun(now)

	mockTaskRepo.EXPECT().GetTaskCount(gomock.Any(), int64(1)).Return(int64(0), nil)
	mockTaskRepo.EXPECT().GetTaskRunCount(gomock.Any(), int64(1), sub.tr.ID).Return(int64(0), nil)
	mockTaskRepo.EXPECT().GetBackfillTaskRun(gomock.Any(), gomock.Nil(), int64(1)).Return(domainRun, nil)
	mockTaskRepo.EXPECT().UpdateTaskRunWithOCC(gomock.Any(), sub.tr.ID, sub.tr.WorkspaceID, gomock.AssignableToTypeOf(map[string]interface{}{})).Return(nil)

	flushed, sampled, err := impl.doFlush(context.Background(), &flushReq{retrievedSpanCount: 1, pageToken: "token", spans: []*loop_span.Span{span}, noMore: true}, sub)
	require.Equal(t, 1, flushed)
	require.Equal(t, 1, sampled)
	require.Error(t, err)
	require.ErrorContains(t, err, "finish fail")
}

func TestTraceHubServiceImpl_DoFlush_SamplingZero(t *testing.T) {
	impl := &TraceHubServiceImpl{}
	sub := &spanSubscriber{
		t: &task.Task{Rule: &task.Rule{Sampler: &task.Sampler{SampleRate: ptr.Of(float64(0))}}},
	}
	fr := &flushReq{retrievedSpanCount: 2, spans: []*loop_span.Span{{SpanID: "s1"}, {SpanID: "s2"}}}

	flushed, sampled, err := impl.doFlush(context.Background(), fr, sub)
	require.NoError(t, err)
	require.Equal(t, 2, flushed)
	require.Zero(t, sampled)
}

func TestTraceHubServiceImpl_IsBackfillDone(t *testing.T) {
	t.Parallel()

	impl := &TraceHubServiceImpl{}
	taskDTO := &task.Task{ID: ptr.Of(int64(1))}

	t.Run("nil task run", func(t *testing.T) {
		t.Parallel()
		sub := &spanSubscriber{t: taskDTO}
		isDone, err := impl.isBackfillDone(context.Background(), sub)
		require.NoError(t, err)
		require.True(t, isDone)
	})

	t.Run("status running", func(t *testing.T) {
		t.Parallel()
		sub := &spanSubscriber{t: taskDTO, tr: &task.TaskRun{RunStatus: task.RunStatusRunning}}
		isDone, err := impl.isBackfillDone(context.Background(), sub)
		require.NoError(t, err)
		require.False(t, isDone)
	})

	t.Run("status done", func(t *testing.T) {
		t.Parallel()
		sub := &spanSubscriber{t: taskDTO, tr: &task.TaskRun{RunStatus: task.RunStatusDone}}
		isDone, err := impl.isBackfillDone(context.Background(), sub)
		require.NoError(t, err)
		require.True(t, isDone)
	})
}

func TestBuildBuiltinFiltersVariants(t *testing.T) {
	t.Parallel()

	t.Run("root span", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		filterMock := spanfilter_mocks.NewMockFilter(ctrl)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, false, nil)
		filterMock.EXPECT().BuildRootSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, nil)

		res, err := buildBuiltinFilters(context.Background(), filterMock, &ListSpansReq{WorkspaceID: 1, SpanListType: loop_span.SpanListTypeRootSpan})
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Len(t, res.FilterFields, 2)
	})

	t.Run("llm span", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		filterMock := spanfilter_mocks.NewMockFilter(ctrl)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, false, nil)
		filterMock.EXPECT().BuildLLMSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, nil)

		res, err := buildBuiltinFilters(context.Background(), filterMock, &ListSpansReq{WorkspaceID: 1, SpanListType: loop_span.SpanListTypeLLMSpan})
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Len(t, res.FilterFields, 2)
	})

	t.Run("all span", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		filterMock := spanfilter_mocks.NewMockFilter(ctrl)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, false, nil)
		filterMock.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, nil)

		res, err := buildBuiltinFilters(context.Background(), filterMock, &ListSpansReq{WorkspaceID: 1, SpanListType: loop_span.SpanListTypeAllSpan})
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Len(t, res.FilterFields, 2)
	})
}

func TestBuildBuiltinFiltersInvalidType(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	filterMock := spanfilter_mocks.NewMockFilter(ctrl)
	filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, false, nil)

	_, err := buildBuiltinFilters(context.Background(), filterMock, &ListSpansReq{WorkspaceID: 1, SpanListType: loop_span.SpanListType("invalid")})
	require.Error(t, err)
	statusErr, ok := errorx.FromStatusError(err)
	require.True(t, ok)
	require.EqualValues(t, obErrorx.CommercialCommonInvalidParamCodeCode, statusErr.Code())
}

func TestTraceHubServiceImpl_CombineFilters(t *testing.T) {
	t.Parallel()

	impl := &TraceHubServiceImpl{}
	inner := &loop_span.FilterFields{FilterFields: []*loop_span.FilterField{{}}}
	res := impl.combineFilters(nil, inner)
	require.NotNil(t, res)
	require.Len(t, res.FilterFields, 1)
	require.Equal(t, inner, res.FilterFields[0].SubFilter)
}

func TestTraceHubServiceImpl_ApplySampling(t *testing.T) {
	t.Parallel()

	impl := &TraceHubServiceImpl{}
	spans := []*loop_span.Span{{SpanID: "1"}, {SpanID: "2"}, {SpanID: "3"}}

	sub := &spanSubscriber{t: &task.Task{Rule: &task.Rule{Sampler: &task.Sampler{SampleRate: ptr.Of(float64(1.0))}}}}
	res := impl.applySampling(spans, sub)
	require.Len(t, res, 3)

	subZero := &spanSubscriber{t: &task.Task{Rule: &task.Rule{Sampler: &task.Sampler{SampleRate: ptr.Of(float64(0.0))}}}}
	resZero := impl.applySampling(spans, subZero)
	require.Nil(t, resZero)

	subHalf := &spanSubscriber{t: &task.Task{Rule: &task.Rule{Sampler: &task.Sampler{SampleRate: ptr.Of(float64(0.4))}}}}
	resHalf := impl.applySampling(spans, subHalf)
	require.Len(t, resHalf, 1)
	require.Equal(t, spans[:1], resHalf)
}

func TestTraceHubServiceImpl_OnHandleDone(t *testing.T) {
	t.Parallel()

	t.Run("with errors triggers retry", func(t *testing.T) {
		t.Parallel()
		ch := make(chan *entity.BackFillEvent, 1)
		impl := &TraceHubServiceImpl{
			backfillProducer: &stubBackfillProducer{ch: ch},
			flushErr:         []error{errors.New("flush err"), errors.New("other")},
		}
		sub := &spanSubscriber{t: &task.Task{ID: ptr.Of(int64(10)), WorkspaceID: ptr.Of(int64(20))}}

		err := impl.onHandleDone(context.Background(), nil, sub)
		require.Error(t, err)
		require.EqualError(t, err, "flush err")

		select {
		case msg := <-ch:
			require.Equal(t, int64(20), msg.SpaceID)
			require.Equal(t, int64(10), msg.TaskID)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("expected backfill message")
		}
	})

	t.Run("no errors", func(t *testing.T) {
		t.Parallel()
		ch := make(chan *entity.BackFillEvent, 1)
		impl := &TraceHubServiceImpl{backfillProducer: &stubBackfillProducer{ch: ch}}
		sub := &spanSubscriber{t: &task.Task{ID: ptr.Of(int64(10)), WorkspaceID: ptr.Of(int64(20))}}

		err := impl.onHandleDone(context.Background(), nil, sub)
		require.NoError(t, err)

		select {
		case <-ch:
			t.Fatal("unexpected message sent")
		case <-time.After(100 * time.Millisecond):
		}
	})
}

func TestTraceHubServiceImpl_SendBackfillMessage(t *testing.T) {
	t.Parallel()

	impl := &TraceHubServiceImpl{}
	err := impl.sendBackfillMessage(context.Background(), &entity.BackFillEvent{})
	require.Error(t, err)

	impl.backfillProducer = &stubBackfillProducer{}
	require.NoError(t, impl.sendBackfillMessage(context.Background(), &entity.BackFillEvent{}))
}

func newBackfillSubscriber(taskRepo taskrepo.ITaskRepo, now time.Time) (*spanSubscriber, *stubProcessor) {
	sampler := &task.Sampler{
		SampleRate: ptr.Of(float64(1)),
		SampleSize: ptr.Of(int64(5)),
	}
	filters := &filter.FilterFields{FilterFields: []*filter.FilterField{}}
	spanFilters := &filter.SpanFilterFields{
		PlatformType: ptr.Of(common.PlatformType(common.PlatformTypeCozeBot)),
		SpanListType: ptr.Of(common.SpanListTypeRootSpan),
		Filters:      filters,
	}
	rule := &task.Rule{
		Sampler:     sampler,
		SpanFilters: spanFilters,
		BackfillEffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(now.Add(-time.Hour).UnixMilli()),
			EndAt:   ptr.Of(now.UnixMilli()),
		},
	}
	status := task.TaskStatusRunning
	taskDTO := &task.Task{
		ID:          ptr.Of(int64(1)),
		Name:        "task",
		WorkspaceID: ptr.Of(int64(2)),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  &status,
		Rule:        rule,
	}
	taskRun := &task.TaskRun{
		ID:          10,
		WorkspaceID: 2,
		TaskID:      1,
		TaskType:    task.TaskRunTypeBackFill,
		RunStatus:   task.RunStatusRunning,
		RunStartAt:  now.Add(-time.Minute).UnixMilli(),
		RunEndAt:    now.Add(time.Minute).UnixMilli(),
	}
	proc := &stubProcessor{}
	sub := &spanSubscriber{
		taskID:    1,
		t:         taskDTO,
		tr:        taskRun,
		processor: proc,
		taskRepo:  taskRepo,
		runType:   entity.TaskRunTypeBackFill,
	}
	return sub, proc
}

func newDomainBackfillTaskRun(now time.Time) *entity.TaskRun {
	return &entity.TaskRun{
		ID:          10,
		TaskID:      1,
		WorkspaceID: 2,
		TaskType:    entity.TaskRunTypeBackFill,
		RunStatus:   entity.TaskRunStatusRunning,
		RunStartAt:  now.Add(-time.Minute),
		RunEndAt:    now.Add(time.Minute),
	}
}

func newTestSpan(now time.Time) *loop_span.Span {
	return &loop_span.Span{
		SpanID:      "span-1",
		TraceID:     "trace-1",
		WorkspaceID: "2",
		StartTime:   now.Add(-30 * time.Second).UnixMilli(),
	}
}

type stubBackfillProducer struct {
	ch  chan *entity.BackFillEvent
	err error
}

func (s *stubBackfillProducer) SendBackfill(ctx context.Context, message *entity.BackFillEvent) error {
	if s.ch != nil {
		s.ch <- message
	}
	return s.err
}
