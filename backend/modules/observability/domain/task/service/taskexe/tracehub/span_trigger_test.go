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
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
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

	impl := &TraceHubServiceImpl{}
	impl.taskCache.Store("ObjListWithTask", &TaskCacheInfo{})

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

	require.NoError(t, impl.SpanTrigger(context.Background(), raw))
}

func TestTraceHubServiceImpl_SpanTriggerDispatchError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	mockBuilder := trace_service_mocks.NewMockTraceFilterProcessorBuilder(ctrl)
	mockFilter := span_filter_mocks.NewMockFilter(ctrl)

	now := time.Now()
	workspaceID := int64(1)
	taskDO := &entity.ObservabilityTask{
		ID:          1,
		WorkspaceID: workspaceID,
		TaskType:    task.TaskTypeAutoEval,
		TaskStatus:  task.TaskStatusRunning,
		SpanFilter: &filter.SpanFilterFields{
			Filters:      &filter.FilterFields{FilterFields: []*filter.FilterField{}},
			PlatformType: ptr.Of(common.PlatformTypeLoopAll),
			SpanListType: ptr.Of(common.SpanListTypeAllSpan),
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
				TaskType:    task.TaskRunTypeNewData,
				RunStatus:   task.TaskStatusRunning,
				RunStartAt:  now.Add(-30 * time.Minute),
				RunEndAt:    now.Add(30 * time.Minute),
			},
		},
	}

	mockRepo.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityTask{taskDO}, int64(0), nil)
	mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, false, nil).AnyTimes()
	mockFilter.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mockBuilder.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(mockFilter, nil).AnyTimes()

	spanRun := &entity.TaskRun{
		ID:          201,
		TaskID:      1,
		WorkspaceID: workspaceID,
		TaskType:    task.TaskRunTypeNewData,
		RunStatus:   task.TaskStatusRunning,
		RunStartAt:  now.Add(-15 * time.Minute),
		RunEndAt:    now.Add(15 * time.Minute),
	}
	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), int64(1)).Return(nil, nil)
	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.Nil(), int64(1)).Return(spanRun, nil)

	proc := &stubProcessor{invokeErr: errors.New("invoke error"), createTaskRunErr: errors.New("create run error")}
	taskProcessor := processor.NewTaskProcessor()
	taskProcessor.Register(task.TaskTypeAutoEval, proc)

	impl := &TraceHubServiceImpl{
		taskRepo:      mockRepo,
		buildHelper:   mockBuilder,
		taskProcessor: taskProcessor,
	}
	impl.taskCache.Store("ObjListWithTask", &TaskCacheInfo{WorkspaceIDs: []string{"space-1"}})

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

	err := impl.SpanTrigger(context.Background(), raw)
	require.Error(t, err)
	require.ErrorContains(t, err, "invoke error")
}

func TestTraceHubServiceImpl_preDispatchHandlesUnstartedAndLimits(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	stubProc := &stubProcessor{}

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
		taskID: taskID,
		t: &task.Task{
			ID:          ptr.Of(taskID),
			WorkspaceID: ptr.Of(workspaceID),
			TaskType:    task.TaskTypeAutoEval,
			TaskStatus:  ptr.Of(task.TaskStatusUnstarted),
			Rule:        rule,
			BaseInfo:    &common.BaseInfo{},
		},
		processor: stubProc,
		taskRepo:  mockRepo,
		runType:   task.TaskRunTypeNewData,
	}

	taskRunConfig := &entity.TaskRun{
		ID:          303,
		TaskID:      taskID,
		WorkspaceID: workspaceID,
		TaskType:    task.TaskRunTypeNewData,
		RunStatus:   task.TaskStatusRunning,
		RunStartAt:  now.Add(-90 * time.Minute),
		RunEndAt:    now.Add(-30 * time.Minute),
	}

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(taskRunConfig, nil)
	mockRepo.EXPECT().GetTaskCount(gomock.Any(), taskID).Return(int64(1), nil)
	mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), taskID, taskRunConfig.ID).Return(int64(1), nil)

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}
	span := &loop_span.Span{
		StartTime: now.UnixMilli(),
		TraceID:   "trace",
		SpanID:    "span",
	}

	err := impl.preDispatch(context.Background(), span, []*spanSubscriber{sub})
	require.NoError(t, err)
	require.Equal(t, 2, len(stubProc.createTaskRunReqs))
	require.Equal(t, startAt, stubProc.createTaskRunReqs[0].RunStartAt)
	require.True(t, stubProc.createTaskRunReqs[0].RunEndAt > startAt)
	require.Equal(t, taskRunConfig.RunEndAt.UnixMilli(), stubProc.createTaskRunReqs[1].RunStartAt)
	require.Equal(t, 1, stubProc.updateCallCount)
	require.Equal(t, 4, stubProc.finishChangeInvoked)
	require.Len(t, stubProc.finishChangeReqs, 4)
	require.True(t, stubProc.finishChangeReqs[0].IsFinish)
	require.True(t, stubProc.finishChangeReqs[1].IsFinish)
	require.False(t, stubProc.finishChangeReqs[2].IsFinish)
	require.False(t, stubProc.finishChangeReqs[3].IsFinish)
}

func TestTraceHubServiceImpl_preDispatchHandlesMissingTaskRunConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	stubProc := &stubProcessor{createTaskRunErr: errors.New("create run failed")}

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
		taskID: taskID,
		t: &task.Task{
			ID:          ptr.Of(taskID),
			WorkspaceID: ptr.Of(workspaceID),
			TaskType:    task.TaskTypeAutoEval,
			TaskStatus:  ptr.Of(task.TaskStatusRunning),
			Rule:        rule,
			BaseInfo:    &common.BaseInfo{},
		},
		processor: stubProc,
		taskRepo:  mockRepo,
		runType:   task.TaskRunTypeNewData,
	}

	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), taskID).Return(nil, nil)

	impl := &TraceHubServiceImpl{taskRepo: mockRepo}
	span := &loop_span.Span{
		StartTime: now.UnixMilli(),
		TraceID:   "trace",
		SpanID:    "span",
	}

	err := impl.preDispatch(context.Background(), span, []*spanSubscriber{sub})
	require.Error(t, err)
	require.ErrorContains(t, err, "task run config not found")
	require.Equal(t, 1, len(stubProc.createTaskRunReqs))
	require.Equal(t, startAt, stubProc.createTaskRunReqs[0].RunStartAt)
	expectedEnd := startAt + 2*7*24*time.Hour.Milliseconds()
	require.Equal(t, expectedEnd, stubProc.createTaskRunReqs[0].RunEndAt)
	require.Equal(t, 0, stubProc.finishChangeInvoked)
}
