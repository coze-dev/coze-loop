// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	lock_mocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/stretchr/testify/require"
)

type trackingProcessor struct {
	*stubProcessor
	finishReqs     []taskexe.OnFinishTaskChangeReq
	createRunReqs  []taskexe.OnCreateTaskRunChangeReq
	updateStatuses []string
}

func newTrackingProcessor() *trackingProcessor {
	return &trackingProcessor{stubProcessor: &stubProcessor{}}
}

func (p *trackingProcessor) OnFinishTaskChange(ctx context.Context, req taskexe.OnFinishTaskChangeReq) error {
	p.finishReqs = append(p.finishReqs, req)
	return p.stubProcessor.OnFinishTaskChange(ctx, req)
}

func (p *trackingProcessor) OnCreateTaskRunChange(ctx context.Context, req taskexe.OnCreateTaskRunChangeReq) error {
	p.createRunReqs = append(p.createRunReqs, req)
	return p.stubProcessor.OnCreateTaskRunChange(ctx, req)
}

func (p *trackingProcessor) OnUpdateTaskChange(ctx context.Context, obsTask *entity.ObservabilityTask, status string) error {
	p.updateStatuses = append(p.updateStatuses, status)
	return p.stubProcessor.OnUpdateTaskChange(ctx, obsTask, status)
}

func TestTraceHubServiceImpl_transformTaskStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		setup  func(t *testing.T, ctrl *gomock.Controller) (*TraceHubServiceImpl, *trackingProcessor)
		assert func(t *testing.T, impl *TraceHubServiceImpl, proc *trackingProcessor)
	}{
		{
			name: "backfill run finished triggers finish callback",
			setup: func(t *testing.T, ctrl *gomock.Controller) (*TraceHubServiceImpl, *trackingProcessor) {
				mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
				now := time.Now()
				backfillRun := &entity.TaskRun{
					ID:         2,
					TaskID:     1,
					TaskType:   string(task.TaskRunTypeBackFill),
					RunStatus:  string(task.RunStatusDone),
					RunStartAt: now.Add(-3 * time.Hour),
					RunEndAt:   now.Add(-2 * time.Hour),
				}
				currentRun := &entity.TaskRun{
					ID:         3,
					TaskID:     1,
					TaskType:   string(task.TaskRunTypeNewData),
					RunStatus:  string(task.TaskStatusRunning),
					RunStartAt: now.Add(-4 * time.Hour),
					RunEndAt:   now.Add(2 * time.Hour),
				}
				taskPO := &entity.ObservabilityTask{
					ID:         1,
					TaskType:   string(task.TaskTypeAutoEval),
					TaskStatus: string(task.TaskStatusRunning),
					EffectiveTime: &entity.EffectiveTime{
						StartAt: now.Add(-5 * time.Hour).UnixMilli(),
						EndAt:   now.Add(-1 * time.Hour).UnixMilli(),
					},
					BackfillEffectiveTime: &entity.EffectiveTime{
						StartAt: now.Add(-6 * time.Hour).UnixMilli(),
						EndAt:   now.Add(-2 * time.Hour).UnixMilli(),
					},
					Sampler:  &entity.Sampler{IsCycle: false},
					TaskRuns: []*entity.TaskRun{backfillRun, currentRun},
				}
				mockRepo.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityTask{taskPO}, int64(1), nil)

				proc := newTrackingProcessor()
				tp := processor.NewTaskProcessor()
				tp.Register(task.TaskTypeAutoEval, proc)

				impl := &TraceHubServiceImpl{
					taskRepo:      mockRepo,
					taskProcessor: tp,
				}
				return impl, proc
			},
			assert: func(t *testing.T, _ *TraceHubServiceImpl, proc *trackingProcessor) {
				require.Len(t, proc.finishReqs, 1)
				require.True(t, proc.finishReqs[0].IsFinish)
				require.Equal(t, int64(2), proc.finishReqs[0].TaskRun.ID)
			},
		},
		{
			name: "unstarted task creates new run and updates status",
			setup: func(t *testing.T, ctrl *gomock.Controller) (*TraceHubServiceImpl, *trackingProcessor) {
				mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
				now := time.Now()
				taskPO := &entity.ObservabilityTask{
					ID:         10,
					TaskType:   string(task.TaskTypeAutoEval),
					TaskStatus: string(task.TaskStatusUnstarted),
					EffectiveTime: &entity.EffectiveTime{
						StartAt: now.Add(-2 * time.Hour).UnixMilli(),
						EndAt:   now.Add(time.Hour).UnixMilli(),
					},
					Sampler: &entity.Sampler{IsCycle: false},
				}
				mockRepo.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityTask{taskPO}, int64(1), nil)

				proc := newTrackingProcessor()
				tp := processor.NewTaskProcessor()
				tp.Register(task.TaskTypeAutoEval, proc)

				impl := &TraceHubServiceImpl{
					taskRepo:      mockRepo,
					taskProcessor: tp,
				}
				return impl, proc
			},
			assert: func(t *testing.T, _ *TraceHubServiceImpl, proc *trackingProcessor) {
				require.Len(t, proc.createRunReqs, 1)
				require.Equal(t, task.TaskRunTypeNewData, proc.createRunReqs[0].RunType)
				require.Len(t, proc.updateStatuses, 1)
				require.Equal(t, string(task.TaskStatusRunning), proc.updateStatuses[0])
			},
		},
		{
			name: "cycle task finishes current run and schedules next",
			setup: func(t *testing.T, ctrl *gomock.Controller) (*TraceHubServiceImpl, *trackingProcessor) {
				mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
				now := time.Now()
				currentRun := &entity.TaskRun{
					ID:         30,
					TaskID:     20,
					TaskType:   string(task.TaskRunTypeNewData),
					RunStatus:  string(task.TaskStatusRunning),
					RunStartAt: now.Add(-2 * time.Hour),
					RunEndAt:   now.Add(-time.Minute),
				}
				taskPO := &entity.ObservabilityTask{
					ID:         20,
					TaskType:   string(task.TaskTypeAutoEval),
					TaskStatus: string(task.TaskStatusRunning),
					Sampler:    &entity.Sampler{IsCycle: true},
					TaskRuns:   []*entity.TaskRun{currentRun},
				}
				mockRepo.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityTask{taskPO}, int64(1), nil)

				proc := newTrackingProcessor()
				tp := processor.NewTaskProcessor()
				tp.Register(task.TaskTypeAutoEval, proc)

				impl := &TraceHubServiceImpl{
					taskRepo:      mockRepo,
					taskProcessor: tp,
				}
				return impl, proc
			},
			assert: func(t *testing.T, _ *TraceHubServiceImpl, proc *trackingProcessor) {
				require.Len(t, proc.finishReqs, 1)
				require.False(t, proc.finishReqs[0].IsFinish)
				require.Len(t, proc.createRunReqs, 1)
				require.Equal(t, proc.finishReqs[0].TaskRun.RunEndAt.UnixMilli(), proc.createRunReqs[0].RunStartAt)
			},
		},
		{
			name: "backfill lock failure triggers retry message",
			setup: func(t *testing.T, ctrl *gomock.Controller) (*TraceHubServiceImpl, *trackingProcessor) {
				mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
				locker := lock_mocks.NewMockILocker(ctrl)
				now := time.Now()
				backfillRun := &entity.TaskRun{
					ID:         40,
					TaskID:     40,
					TaskType:   string(task.TaskRunTypeBackFill),
					RunStatus:  string(task.TaskStatusRunning),
					RunStartAt: now.Add(-time.Hour),
					RunEndAt:   now.Add(time.Hour),
				}
				currentRun := &entity.TaskRun{
					ID:         41,
					TaskID:     40,
					TaskType:   string(task.TaskRunTypeNewData),
					RunStatus:  string(task.TaskStatusRunning),
					RunStartAt: now.Add(-time.Hour),
					RunEndAt:   now.Add(time.Hour),
				}
				taskPO := &entity.ObservabilityTask{
					ID:                    40,
					WorkspaceID:           99,
					TaskType:              string(task.TaskTypeAutoEval),
					TaskStatus:            string(task.TaskStatusRunning),
					BackfillEffectiveTime: &entity.EffectiveTime{StartAt: now.Add(-2 * time.Hour).UnixMilli(), EndAt: now.Add(time.Hour).UnixMilli()},
					Sampler:               &entity.Sampler{IsCycle: false},
					TaskRuns:              []*entity.TaskRun{backfillRun, currentRun},
				}
				locker.EXPECT().Lock(gomock.Any(), transformTaskStatusLockKey, transformTaskStatusLockTTL).Return(true, nil)
				mockRepo.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityTask{taskPO}, int64(1), nil)
				locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), transformTaskStatusLockTTL, backfillLockMaxHold).
					Return(false, context.Background(), func() {}, errors.New("lock failed"))
				locker.EXPECT().Unlock(transformTaskStatusLockKey).Return(true, nil)

				proc := newTrackingProcessor()
				tp := processor.NewTaskProcessor()
				tp.Register(task.TaskTypeAutoEval, proc)

				producer := &stubBackfillProducer{ch: make(chan *entity.BackFillEvent, 1)}
				impl := &TraceHubServiceImpl{
					taskRepo:         mockRepo,
					taskProcessor:    tp,
					locker:           locker,
					backfillProducer: producer,
				}
				return impl, proc
			},
			assert: func(t *testing.T, impl *TraceHubServiceImpl, proc *trackingProcessor) {
				require.Empty(t, proc.finishReqs)
				require.Empty(t, proc.createRunReqs)
				producer, ok := impl.backfillProducer.(*stubBackfillProducer)
				require.True(t, ok)
				select {
				case msg := <-producer.ch:
					require.Equal(t, int64(40), msg.TaskID)
				case <-time.After(100 * time.Millisecond):
					t.Fatal("expected backfill message")
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)

			impl, proc := tc.setup(t, ctrl)
			impl.transformTaskStatus()
			tc.assert(t, impl, proc)
		})
	}
}

func TestTraceHubServiceImpl_syncTaskRunCounts(t *testing.T) {
	t.Parallel()

	t.Run("sync success", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
		impl := &TraceHubServiceImpl{taskRepo: mockRepo}

		taskRun := &entity.TaskRun{ID: 101, TaskID: 1}
		taskPO := &entity.ObservabilityTask{ID: 1, TaskRuns: []*entity.TaskRun{taskRun}}

		gomock.InOrder(
			mockRepo.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityTask{taskPO}, int64(1), nil),
			mockRepo.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityTask{}, int64(0), nil),
		)

		gomock.InOrder(
			mockRepo.EXPECT().GetTaskRunCount(gomock.Any(), int64(1), int64(101)).Return(int64(5), nil),
			mockRepo.EXPECT().GetTaskRunSuccessCount(gomock.Any(), int64(1), int64(101)).Return(int64(3), nil),
			mockRepo.EXPECT().GetTaskRunFailCount(gomock.Any(), int64(1), int64(101)).Return(int64(2), nil),
			mockRepo.EXPECT().UpdateTaskRunWithOCC(gomock.Any(), int64(101), int64(0), gomock.Any()).DoAndReturn(
				func(ctx context.Context, runID, version int64, data map[string]interface{}) error {
					require.Equal(t, int64(101), runID)
					require.Equal(t, int64(0), version)
					detailStr, ok := data["run_detail"].(string)
					require.True(t, ok)
					var detail map[string]int64
					require.NoError(t, json.Unmarshal([]byte(detailStr), &detail))
					require.Equal(t, int64(5), detail["total_count"])
					require.Equal(t, int64(3), detail["success_count"])
					require.Equal(t, int64(2), detail["failed_count"])
					return nil
				},
			),
		)

		impl.syncTaskRunCounts()
	})

	t.Run("lock not acquired", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
		locker := lock_mocks.NewMockILocker(ctrl)
		locker.EXPECT().Lock(gomock.Any(), syncTaskRunCountsLockKey, transformTaskStatusLockTTL).Return(false, nil)

		impl := &TraceHubServiceImpl{
			taskRepo: mockRepo,
			locker:   locker,
		}

		impl.syncTaskRunCounts()
	})
}

func TestTraceHubServiceImpl_syncTaskCache(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	impl := &TraceHubServiceImpl{taskRepo: mockRepo}
	impl.taskCache.Store("ObjListWithTask", &TaskCacheInfo{})

	workspaceIDs := []string{"space-1"}
	botIDs := []string{"bot-1"}
	tasks := []*entity.ObservabilityTask{{ID: 100}}
	mockRepo.EXPECT().GetObjListWithTask(gomock.Any()).Return(workspaceIDs, botIDs, tasks)

	before := time.Now()
	impl.syncTaskCache()

	val, ok := impl.taskCache.Load("ObjListWithTask")
	require.True(t, ok)
	cache, ok := val.(*TaskCacheInfo)
	require.True(t, ok)
	require.Equal(t, workspaceIDs, cache.WorkspaceIDs)
	require.Equal(t, botIDs, cache.BotIDs)
	require.Equal(t, tasks, cache.Tasks)
	require.WithinDuration(t, before, cache.UpdateTime, time.Second*5)
}

func TestTraceHubServiceImpl_updateTaskRunDetail(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
		impl := &TraceHubServiceImpl{taskRepo: mockRepo}

		info := &TaskRunCountInfo{TaskRunID: 200, TaskRunCount: 8, TaskRunSuccCount: 5, TaskRunFailCount: 3}

		mockRepo.EXPECT().UpdateTaskRunWithOCC(gomock.Any(), int64(200), int64(0), gomock.Any()).DoAndReturn(
			func(ctx context.Context, runID, version int64, data map[string]interface{}) error {
				detailStr, ok := data["run_detail"].(string)
				require.True(t, ok)
				var detail map[string]int64
				require.NoError(t, json.Unmarshal([]byte(detailStr), &detail))
				require.Equal(t, int64(8), detail["total_count"])
				require.Equal(t, int64(5), detail["success_count"])
				require.Equal(t, int64(3), detail["failed_count"])
				return nil
			},
		)

		require.NoError(t, impl.updateTaskRunDetail(context.Background(), info))
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
		impl := &TraceHubServiceImpl{taskRepo: mockRepo}

		info := &TaskRunCountInfo{TaskRunID: 201}
		expectErr := errors.New("db err")

		mockRepo.EXPECT().UpdateTaskRunWithOCC(gomock.Any(), int64(201), int64(0), gomock.Any()).Return(expectErr)

		err := impl.updateTaskRunDetail(context.Background(), info)
		require.Error(t, err)
		require.ErrorIs(t, err, expectErr)
	})
}

func TestTraceHubServiceImpl_listNonFinalTask(t *testing.T) {
	t.Parallel()

	t.Run("multi page", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
		impl := &TraceHubServiceImpl{taskRepo: mockRepo}

		firstBatch := make([]*entity.ObservabilityTask, 1000)
		for i := range firstBatch {
			firstBatch[i] = &entity.ObservabilityTask{ID: int64(i)}
		}
		secondBatch := []*entity.ObservabilityTask{{ID: 1000}}

		gomock.InOrder(
			mockRepo.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(firstBatch, int64(len(firstBatch)), nil),
			mockRepo.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(secondBatch, int64(len(secondBatch)), nil),
		)

		tasks, err := impl.listNonFinalTask(context.Background())
		require.NoError(t, err)
		require.Len(t, tasks, len(firstBatch)+len(secondBatch))
		require.Equal(t, int64(0), tasks[0].ID)
		require.Equal(t, int64(1000), tasks[len(tasks)-1].ID)
	})

	t.Run("repo error", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
		impl := &TraceHubServiceImpl{taskRepo: mockRepo}

		expectErr := errors.New("list error")
		mockRepo.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(nil, int64(0), expectErr)

		tasks, err := impl.listNonFinalTask(context.Background())
		require.Error(t, err)
		require.Nil(t, tasks)
	})
}
