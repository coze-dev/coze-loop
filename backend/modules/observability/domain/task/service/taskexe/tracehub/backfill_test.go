// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestTraceHubServiceImpl_SetBackfillTask(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	taskProcessor := processor.NewTaskProcessor()
	proc := &stubProcessor{}
	taskProcessor.Register(task.TaskTypeAutoEval, proc)

	impl := &TraceHubServiceImpl{
		taskRepo:      mockRepo,
		taskProcessor: taskProcessor,
	}

	now := time.Now()
	obsTask := &entity.ObservabilityTask{
		ID:          1,
		WorkspaceID: 1,
		TaskType:    task.TaskTypeAutoEval,
		SpanFilter:  &filter.SpanFilterFields{},
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
		TaskType:    task.TaskRunTypeBackFill,
		RunStatus:   task.RunStatusRunning,
		RunStartAt:  now.Add(-time.Minute),
		RunEndAt:    now.Add(time.Minute),
	}

	mockRepo.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Nil(), gomock.Nil()).Return(obsTask, nil)
	mockRepo.EXPECT().GetBackfillTaskRun(gomock.Any(), gomock.AssignableToTypeOf(ptr.Of(int64(0))), int64(1)).Return(backfillRun, nil)

	sub, err := impl.setBackfillTask(context.Background(), &entity.BackFillEvent{TaskID: 1})
	require.NoError(t, err)
	require.NotNil(t, sub)
	require.Equal(t, int64(1), sub.taskID)
	require.Equal(t, task.TaskRunTypeBackFill, sub.runType)
}

func TestTraceHubServiceImpl_SetBackfillTaskNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	mockRepo.EXPECT().GetTask(gomock.Any(), int64(1), gomock.Nil(), gomock.Nil()).Return(nil, nil)

	_, err := impl.setBackfillTask(context.Background(), &entity.BackFillEvent{TaskID: 1})
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
		runType:   task.TaskRunTypeNewData,
		taskRepo:  mockRepo,
	}

	spanRun := &entity.TaskRun{
		ID:          20,
		TaskID:      1,
		WorkspaceID: 1,
		TaskType:    task.TaskRunTypeNewData,
		RunStatus:   task.RunStatusRunning,
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
