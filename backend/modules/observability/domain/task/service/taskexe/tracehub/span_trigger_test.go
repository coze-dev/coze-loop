// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	taskconvertor "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	componentconfig "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	config_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	tenant_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	trace_service_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	span_filter_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/require"
)

func TestTraceHubServiceImpl_SpanTriggerSkipNoWorkspace(t *testing.T) {
	t.Parallel()

	impl := &TraceHubServiceImpl{
		localCache: NewLocalCache(),
	}
	impl.localCache.taskCache.Store("ObjListWithTask", TaskCacheInfo{})

	raw := &entity.RawSpan{
		TraceID: "trace",
		SpanID:  "span",
		LogID:   "log",
		Tags: map[string]any{
			"fornax_space_id": "space-1",
			"call_type":       "",
			"bot_id":          "bot-1",
		},
		SensitiveTags: &entity.SensitiveTags{},
		ServerEnv:     &entity.ServerInRawSpan{},
	}

	require.NoError(t, impl.SpanTrigger(context.Background(), raw.RawSpanConvertToLoopSpan()))
}

func TestTraceHubServiceImpl_SpanTriggerDispatchError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	mockBuilder := trace_service_mocks.NewMockTraceFilterProcessorBuilder(ctrl)
	mockFilter := span_filter_mocks.NewMockFilter(ctrl)
	configLoader := config_mocks.NewMockITraceConfig(ctrl)
	tenantProvider := tenant_mocks.NewMockITenantProvider(ctrl)

	now := time.Now()
	workspaceID := int64(1)
	taskDO := &entity.ObservabilityTask{
		ID:          1,
		WorkspaceID: workspaceID,
		TaskType:    entity.TaskTypeAutoEval,
		TaskStatus:  entity.TaskStatusRunning,
		SpanFilter: &entity.SpanFilterFields{
			PlatformType: loop_span.PlatformDefault,
			SpanListType: loop_span.SpanListTypeAllSpan,
			Filters: loop_span.FilterFields{
				QueryAndOr:   ptr.Of(loop_span.QueryAndOrEnumAnd),
				FilterFields: []*loop_span.FilterField{},
			},
		},
		Sampler: &entity.Sampler{
			SampleRate: 1,
			SampleSize: 10,
			IsCycle:    false,
		},
		EffectiveTime: &entity.EffectiveTime{
			StartAt: now.Add(-time.Hour).UnixMilli(),
			EndAt:   now.Add(time.Hour).UnixMilli(),
		},
		TaskRuns: []*entity.TaskRun{
			{
				ID:          101,
				TaskID:      1,
				WorkspaceID: workspaceID,
				TaskType:    entity.TaskRunTypeNewData,
				RunStatus:   entity.TaskRunStatusRunning,
				RunStartAt:  now.Add(-30 * time.Minute),
				RunEndAt:    now.Add(30 * time.Minute),
			},
		},
	}

	mockRepo.EXPECT().ListNonFinalTaskBySpaceID(gomock.Any(), gomock.Any()).Return([]int64{taskDO.ID}, nil).AnyTimes()

	configLoader.EXPECT().GetConsumerListening(gomock.Any()).Return(&componentconfig.ConsumerListening{IsAllSpace: true}, nil).AnyTimes()
	configLoader.EXPECT().UnmarshalKey(gomock.Any(), "consumer_listening", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, value any, _ ...interface{}) error {
			cfg := value.(*componentconfig.ConsumerListening)
			*cfg = componentconfig.ConsumerListening{IsAllSpace: true}
			return nil
		},
	).AnyTimes()
	mockRepo.EXPECT().GetTaskByCache(gomock.Any(), taskDO.ID).Return(taskDO, nil).AnyTimes()
	mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, false, nil).AnyTimes()
	mockFilter.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mockBuilder.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(mockFilter, nil).AnyTimes()
	tenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformDefault, gomock.Any()).Return([]string{"tenant"}, nil).AnyTimes()

	spanRun := &entity.TaskRun{
		ID:          201,
		TaskID:      1,
		WorkspaceID: workspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   entity.TaskRunStatusRunning,
		RunStartAt:  now.Add(-15 * time.Minute),
		RunEndAt:    now.Add(15 * time.Minute),
	}
	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.Any(), int64(1)).Return(spanRun, nil).AnyTimes()
	mockRepo.EXPECT().GetTaskCount(gomock.Any(), int64(1)).Return(int64(0), nil).AnyTimes()
	mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), int64(1), spanRun.ID).Return(int64(0), nil).AnyTimes()

	procMock := &stubProcessor{invokeErr: errors.New("invoke error"), createTaskRunErr: errors.New("create run error")}

	taskProcessor := processor.NewTaskProcessor()
	taskProcessor.Register(entity.TaskTypeAutoEval, procMock)

	impl := &TraceHubServiceImpl{
		taskRepo:       mockRepo,
		buildHelper:    mockBuilder,
		taskProcessor:  taskProcessor,
		localCache:     NewLocalCache(),
		config:         configLoader,
		tenantProvider: tenantProvider,
	}
	impl.localCache.taskCache.Store("ObjListWithTask", TaskCacheInfo{WorkspaceIDs: []string{"space-1"}, Tasks: []*entity.ObservabilityTask{taskDO}})

	raw := &entity.RawSpan{
		TraceID:       "trace",
		SpanID:        "span",
		LogID:         "log",
		StartTimeInUs: now.UnixMicro(),
		Tags: map[string]any{
			"fornax_space_id": "space-1",
			"call_type":       "",
			"bot_id":          "bot-1",
		},
		SystemTags: map[string]any{
			loop_span.SpanFieldTenant: "tenant",
		},
		SensitiveTags: &entity.SensitiveTags{},
		ServerEnv:     &entity.ServerInRawSpan{},
	}

	err := impl.SpanTrigger(context.Background(), raw.RawSpanConvertToLoopSpan())
	require.Error(t, err)
	require.Contains(t, err.Error(), "invoke error")
}

func TestTraceHubServiceImpl_preDispatchHandlesUnstartedAndLimits(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{}

	now := time.Now()
	startAt := now.Add(-2 * time.Hour).UnixMilli()
	endAt := now.Add(-time.Minute).UnixMilli()
	workspaceID := int64(101)
	taskID := int64(202)

	cycleUnitDay := task.TimeUnitDay
	sampl := &task.Sampler{
		SampleRate:    floatPtr(1),
		SampleSize:    int64Ptr(1),
		IsCycle:       boolPtr(true),
		CycleCount:    int64Ptr(1),
		CycleInterval: int64Ptr(1),
		CycleTimeUnit: &cycleUnitDay,
	}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(endAt),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusUnstarted),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	taskRunConfig := &entity.TaskRun{
		ID:          303,
		TaskID:      taskID,
		WorkspaceID: workspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   task.TaskStatusRunning,
		RunStartAt:  now.Add(-90 * time.Minute),
		RunEndAt:    now.Add(-30 * time.Minute),
	}

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(taskRunConfig, nil)
	mockRepo.EXPECT().GetTaskCount(gomock.Any(), taskID).Return(int64(1), nil)
	mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), taskID, taskRunConfig.ID).Return(int64(1), nil)

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.NoError(t, err)
	require.Equal(t, 2, len(procMock.createTaskRunReqs))
	require.Equal(t, startAt, procMock.createTaskRunReqs[0].RunStartAt)
	require.True(t, procMock.createTaskRunReqs[0].RunEndAt > startAt)
	require.Equal(t, taskRunConfig.RunEndAt.UnixMilli(), procMock.createTaskRunReqs[1].RunStartAt)
	require.Equal(t, 1, procMock.updateCallCount)
	require.Equal(t, 4, procMock.finishChangeInvoked)
	require.Len(t, procMock.finishChangeReqs, 4)
	require.True(t, procMock.finishChangeReqs[0].IsFinish)
	require.True(t, procMock.finishChangeReqs[1].IsFinish)
	require.False(t, procMock.finishChangeReqs[2].IsFinish)
	require.False(t, procMock.finishChangeReqs[3].IsFinish)
}

func TestTraceHubServiceImpl_preDispatchHandlesMissingTaskRunConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{createTaskRunErr: errors.New("create run failed")}

	now := time.Now()
	startAt := now.Add(-10 * time.Minute).UnixMilli()
	workspaceID := int64(303)
	taskID := int64(404)

	cycleUnitWeek := task.TimeUnitWeek
	sampl := &task.Sampler{
		IsCycle:       boolPtr(true),
		CycleInterval: int64Ptr(2),
		CycleTimeUnit: &cycleUnitWeek,
	}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(now.Add(time.Hour).UnixMilli()),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusRunning),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(nil, nil)

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.Error(t, err)
	require.ErrorContains(t, err, "task run config not found")
	require.Equal(t, 1, len(procMock.createTaskRunReqs))
	require.Equal(t, startAt, procMock.createTaskRunReqs[0].RunStartAt)
	expectedEnd := startAt + 2*7*24*time.Hour.Milliseconds()
	require.Equal(t, expectedEnd, procMock.createTaskRunReqs[0].RunEndAt)
	require.Equal(t, 0, procMock.finishChangeInvoked)
}

func TestTraceHubServiceImpl_preDispatchHandlesNonCycle(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{}

	now := time.Now()
	startAt := now.Add(-time.Hour).UnixMilli()
	endAt := now.Add(time.Hour).UnixMilli()
	workspaceID := int64(505)
	taskID := int64(606)

	sampl := &task.Sampler{
		SampleRate: floatPtr(1),
		SampleSize: int64Ptr(5),
		IsCycle:    boolPtr(false),
	}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(endAt),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusUnstarted),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	taskRunConfig := &entity.TaskRun{
		ID:          707,
		TaskID:      taskID,
		WorkspaceID: workspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   task.TaskStatusRunning,
		RunStartAt:  now.Add(-30 * time.Minute),
		RunEndAt:    now.Add(30 * time.Minute),
	}

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(taskRunConfig, nil)
	mockRepo.EXPECT().GetTaskCount(gomock.Any(), taskID).Return(int64(0), nil)
	mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), taskID, taskRunConfig.ID).Return(int64(0), nil)

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.NoError(t, err)
	require.Equal(t, 1, len(procMock.createTaskRunReqs))
	require.Equal(t, endAt, procMock.createTaskRunReqs[0].RunEndAt)
	require.Equal(t, 1, procMock.updateCallCount)
	require.Zero(t, procMock.finishChangeInvoked)
}

func TestTraceHubServiceImpl_preDispatchHandlesCycleDefaultUnit(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{createTaskRunErrSeq: []error{nil, errors.New("create fail")}}

	now := time.Now()
	startAt := now.Add(-15 * time.Minute).UnixMilli()
	workspaceID := int64(707)
	taskID := int64(808)
	cycleUnitNull := task.TimeUnitNull
	sampl := &task.Sampler{
		IsCycle:       boolPtr(true),
		CycleInterval: int64Ptr(3),
		CycleTimeUnit: &cycleUnitNull,
	}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(now.Add(time.Hour).UnixMilli()),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusUnstarted),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(nil, nil)

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.Error(t, err)
	require.ErrorContains(t, err, "create fail")
	require.Equal(t, 2, len(procMock.createTaskRunReqs))
	delta := int64(3) * 10 * time.Minute.Milliseconds()
	require.Equal(t, startAt+delta, procMock.createTaskRunReqs[0].RunEndAt)
	require.Equal(t, startAt+delta, procMock.createTaskRunReqs[1].RunEndAt)
}

func TestTraceHubServiceImpl_preDispatchTimeLimitFinishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{finishErrSeq: []error{errors.New("finish error")}}

	now := time.Now()
	startAt := now.Add(-2 * time.Hour).UnixMilli()
	endAt := now.Add(-time.Minute).UnixMilli()
	workspaceID := int64(909)
	taskID := int64(1001)
	cycleUnitDay := task.TimeUnitDay
	sampl := &task.Sampler{
		SampleRate:    floatPtr(1),
		SampleSize:    int64Ptr(5),
		IsCycle:       boolPtr(true),
		CycleCount:    int64Ptr(2),
		CycleInterval: int64Ptr(1),
		CycleTimeUnit: &cycleUnitDay,
	}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(endAt),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusRunning),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	taskRunConfig := &entity.TaskRun{
		ID:          1101,
		TaskID:      taskID,
		WorkspaceID: workspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   task.TaskStatusRunning,
		RunStartAt:  now.Add(-3 * time.Hour),
		RunEndAt:    now.Add(-2 * time.Hour),
	}

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(taskRunConfig, nil)
	mockRepo.EXPECT().GetTaskCount(gomock.Any(), taskID).Return(int64(0), nil).AnyTimes()
	mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), taskID, taskRunConfig.ID).Return(int64(0), nil).AnyTimes()

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.Error(t, err)
	require.ErrorContains(t, err, "finish error")
	require.Equal(t, 1, procMock.finishChangeInvoked)
}

func TestTraceHubServiceImpl_preDispatchSampleLimitFinishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{finishErrSeq: []error{errors.New("sample limit error")}}

	now := time.Now()
	startAt := now.Add(-time.Hour).UnixMilli()
	endAt := now.Add(time.Hour).UnixMilli()
	workspaceID := int64(1202)
	taskID := int64(1303)
	cycleUnitDay := task.TimeUnitDay
	sampl := &task.Sampler{
		SampleRate:    floatPtr(1),
		SampleSize:    int64Ptr(1),
		IsCycle:       boolPtr(true),
		CycleCount:    int64Ptr(2),
		CycleInterval: int64Ptr(1),
		CycleTimeUnit: &cycleUnitDay,
	}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(endAt),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusRunning),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	taskRunConfig := &entity.TaskRun{
		ID:          1404,
		TaskID:      taskID,
		WorkspaceID: workspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   task.TaskStatusRunning,
		RunStartAt:  now.Add(-30 * time.Minute),
		RunEndAt:    now.Add(30 * time.Minute),
	}

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(taskRunConfig, nil)
	mockRepo.EXPECT().GetTaskCount(gomock.Any(), taskID).Return(int64(1), nil)
	mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), taskID, taskRunConfig.ID).Return(int64(0), nil)

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.Error(t, err)
	require.ErrorContains(t, err, "sample limit error")
	require.Equal(t, 1, procMock.finishChangeInvoked)
}

func TestTraceHubServiceImpl_preDispatchCycleTimeLimitFinishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{finishErrSeq: []error{errors.New("cycle time error")}}

	now := time.Now()
	startAt := now.Add(-time.Hour).UnixMilli()
	endAt := now.Add(time.Hour).UnixMilli()
	workspaceID := int64(1505)
	taskID := int64(1606)
	cycleUnitDay := task.TimeUnitDay
	sampl := &task.Sampler{
		SampleRate:    floatPtr(1),
		SampleSize:    int64Ptr(5),
		IsCycle:       boolPtr(true),
		CycleCount:    int64Ptr(2),
		CycleInterval: int64Ptr(1),
		CycleTimeUnit: &cycleUnitDay,
	}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(endAt),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusRunning),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	taskRunConfig := &entity.TaskRun{
		ID:          1707,
		TaskID:      taskID,
		WorkspaceID: workspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   task.TaskStatusRunning,
		RunStartAt:  now.Add(-2 * time.Hour),
		RunEndAt:    now.Add(-time.Minute),
	}

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(taskRunConfig, nil)
	mockRepo.EXPECT().GetTaskCount(gomock.Any(), taskID).Return(int64(0), nil)
	mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), taskID, taskRunConfig.ID).Return(int64(0), nil)

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.Error(t, err)
	require.ErrorContains(t, err, "cycle time error")
	require.Equal(t, 1, procMock.finishChangeInvoked)
}

func TestTraceHubServiceImpl_preDispatchCycleCountFinishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{finishErrSeq: []error{errors.New("cycle count error")}}

	now := time.Now()
	startAt := now.Add(-time.Hour).UnixMilli()
	endAt := now.Add(time.Hour).UnixMilli()
	workspaceID := int64(1808)
	taskID := int64(1909)
	cycleUnitDay := task.TimeUnitDay
	sampl := &task.Sampler{
		SampleRate:    floatPtr(1),
		SampleSize:    int64Ptr(5),
		IsCycle:       boolPtr(true),
		CycleCount:    int64Ptr(1),
		CycleInterval: int64Ptr(1),
		CycleTimeUnit: &cycleUnitDay,
	}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(endAt),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusRunning),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	taskRunConfig := &entity.TaskRun{
		ID:          2009,
		TaskID:      taskID,
		WorkspaceID: workspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   task.TaskStatusRunning,
		RunStartAt:  now.Add(-30 * time.Minute),
		RunEndAt:    now.Add(30 * time.Minute),
	}

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(taskRunConfig, nil)
	mockRepo.EXPECT().GetTaskCount(gomock.Any(), taskID).Return(int64(0), nil)
	mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), taskID, taskRunConfig.ID).Return(int64(1), nil)

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.Error(t, err)
	require.ErrorContains(t, err, "cycle count error")
	require.Equal(t, 1, procMock.finishChangeInvoked)
}

func TestTraceHubServiceImpl_preDispatchCreativeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{createTaskRunErrSeq: []error{errors.New("creative fail")}}

	now := time.Now()
	startAt := now.Add(-time.Hour).UnixMilli()
	workspaceID := int64(2101)
	taskID := int64(2202)
	cycleUnitWeek := task.TimeUnitWeek
	sampl := &task.Sampler{
		IsCycle:       boolPtr(true),
		CycleInterval: int64Ptr(2),
		CycleTimeUnit: &cycleUnitWeek,
	}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(now.Add(time.Hour).UnixMilli()),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusUnstarted),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.Error(t, err)
	require.ErrorContains(t, err, "creative fail")
	require.Equal(t, 1, len(procMock.createTaskRunReqs))
}

func toObservabilityTask(dto *task.Task) *entity.ObservabilityTask {
	return taskconvertor.TaskDTO2DO(dto)
}

func TestTraceHubServiceImpl_preDispatchAggregatesErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)

	now := time.Now()
	firstStartAt := now.Add(-time.Hour).UnixMilli()
	firstSamplerUnit := task.TimeUnitWeek
	firstProc := &stubProcessor{createTaskRunErrSeq: []error{errors.New("first fail")}}
	firstSampler := &task.Sampler{
		IsCycle:       boolPtr(true),
		CycleInterval: int64Ptr(1),
		CycleTimeUnit: &firstSamplerUnit,
	}
	firstSub := &spanSubscriber{
		taskID:    11,
		processor: firstProc,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	firstSub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(int64(11)),
		WorkspaceID: ptr.Of(int64(21)),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusUnstarted),
		Rule: &task.Rule{
			EffectiveTime: &task.EffectiveTime{StartAt: ptr.Of(firstStartAt), EndAt: ptr.Of(now.Add(time.Hour).UnixMilli())},
			Sampler:       firstSampler,
		},
		BaseInfo: &common.BaseInfo{},
	})

	secondStartAt := now.Add(-2 * time.Hour).UnixMilli()
	secondEndAt := now.Add(-time.Minute).UnixMilli()
	secondSamplerUnit := task.TimeUnitDay
	secondSampler := &task.Sampler{
		SampleRate:    floatPtr(1),
		SampleSize:    int64Ptr(1),
		IsCycle:       boolPtr(false),
		CycleTimeUnit: &secondSamplerUnit,
	}
	secondTaskID := int64(12)
	secondWorkspaceID := int64(22)
	secondRun := &entity.TaskRun{
		ID:          101,
		TaskID:      secondTaskID,
		WorkspaceID: secondWorkspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   task.TaskStatusRunning,
		RunStartAt:  now.Add(-3 * time.Hour),
		RunEndAt:    now.Add(-90 * time.Minute),
	}
	secondProc := &stubProcessor{finishErrSeq: []error{errors.New("second fail")}}
	secondSub := &spanSubscriber{
		taskID:    secondTaskID,
		processor: secondProc,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	secondSub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(secondTaskID),
		WorkspaceID: ptr.Of(secondWorkspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusRunning),
		Rule: &task.Rule{
			EffectiveTime: &task.EffectiveTime{StartAt: ptr.Of(secondStartAt), EndAt: ptr.Of(secondEndAt)},
			Sampler:       secondSampler,
		},
		BaseInfo: &common.BaseInfo{},
	})

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), secondTaskID).Return(secondRun, nil)
	mockRepo.EXPECT().GetTaskCount(gomock.Any(), secondTaskID).Return(int64(0), nil).AnyTimes()
	mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), secondTaskID, secondRun.ID).Return(int64(0), nil).AnyTimes()

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{firstSub, secondSub})
	require.Error(t, err)
	require.Contains(t, err.Error(), "first fail")
	require.Contains(t, err.Error(), "second fail")
	require.Equal(t, 1, len(firstProc.createTaskRunReqs))
	require.Equal(t, 1, secondProc.finishChangeInvoked)
}

func TestTraceHubServiceImpl_preDispatchUpdateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{updateErr: errors.New("update fail")}

	now := time.Now()
	startAt := now.Add(-time.Hour).UnixMilli()
	endAt := now.Add(time.Hour).UnixMilli()
	workspaceID := int64(2303)
	taskID := int64(2404)
	cycleUnitDay := task.TimeUnitDay
	sampl := &task.Sampler{
		IsCycle:       boolPtr(true),
		CycleInterval: int64Ptr(1),
		CycleTimeUnit: &cycleUnitDay,
	}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(endAt),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusUnstarted),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.NoError(t, err)
	require.Equal(t, 1, len(procMock.createTaskRunReqs))
	require.Equal(t, 1, procMock.updateCallCount)
	require.Zero(t, procMock.finishChangeInvoked)
}

func TestTraceHubServiceImpl_preDispatchListTaskRunError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{}

	now := time.Now()
	startAt := now.Add(-time.Hour).UnixMilli()
	endAt := now.Add(time.Hour).UnixMilli()
	workspaceID := int64(2505)
	taskID := int64(2606)
	sampl := &task.Sampler{IsCycle: boolPtr(false)}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(endAt),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusRunning),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(nil, errors.New("repo fail"))

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.NoError(t, err)
	require.Empty(t, procMock.createTaskRunReqs)
}

func TestTraceHubServiceImpl_preDispatchTaskRunConfigDay(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{createTaskRunErrSeq: []error{errors.New("create fail")}}

	now := time.Now()
	startAt := now.Add(-10 * time.Minute).UnixMilli()
	workspaceID := int64(2707)
	taskID := int64(2808)
	cycleUnitDay := task.TimeUnitDay
	sampl := &task.Sampler{
		IsCycle:       boolPtr(true),
		CycleInterval: int64Ptr(2),
		CycleTimeUnit: &cycleUnitDay,
	}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(now.Add(time.Hour).UnixMilli()),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusRunning),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(nil, nil)

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.Error(t, err)
	require.ErrorContains(t, err, "create fail")
	require.Equal(t, 1, len(procMock.createTaskRunReqs))
	delta := int64(2) * 24 * time.Hour.Milliseconds()
	require.Equal(t, startAt+delta, procMock.createTaskRunReqs[0].RunEndAt)
}

func TestTraceHubServiceImpl_preDispatchCycleCreativeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	procMock := &stubProcessor{createTaskRunErrSeq: []error{errors.New("cycle create fail")}}

	now := time.Now()
	startAt := now.Add(-time.Hour).UnixMilli()
	endAt := now.Add(time.Hour).UnixMilli()
	workspaceID := int64(2909)
	taskID := int64(3001)
	cycleUnitDay := task.TimeUnitDay
	sampl := &task.Sampler{
		SampleRate:    floatPtr(1),
		SampleSize:    int64Ptr(5),
		IsCycle:       boolPtr(true),
		CycleCount:    int64Ptr(2),
		CycleInterval: int64Ptr(1),
		CycleTimeUnit: &cycleUnitDay,
	}
	rule := &task.Rule{
		EffectiveTime: &task.EffectiveTime{
			StartAt: ptr.Of(startAt),
			EndAt:   ptr.Of(endAt),
		},
		Sampler: sampl,
	}

	sub := &spanSubscriber{
		taskID:    taskID,
		processor: procMock,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}
	sub.t = toObservabilityTask(&task.Task{
		ID:          ptr.Of(taskID),
		WorkspaceID: ptr.Of(workspaceID),
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  ptr.Of(task.TaskStatusRunning),
		Rule:        rule,
		BaseInfo:    &common.BaseInfo{},
	})

	taskRunConfig := &entity.TaskRun{
		ID:          3102,
		TaskID:      taskID,
		WorkspaceID: workspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   task.TaskStatusRunning,
		RunStartAt:  now.Add(-2 * time.Hour),
		RunEndAt:    now.Add(-time.Minute),
	}

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(taskRunConfig, nil)
	mockRepo.EXPECT().GetTaskCount(gomock.Any(), taskID).Return(int64(0), nil)
	mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), taskID, taskRunConfig.ID).Return(int64(0), nil)

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	err := impl.preDispatch(context.Background(), []*spanSubscriber{sub})
	require.Error(t, err)
	require.ErrorContains(t, err, "cycle create fail")
	require.Equal(t, 1, len(procMock.createTaskRunReqs))
}
