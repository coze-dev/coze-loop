// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package scheduledtask

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
)

func TestStatusCheckTask_checkTaskStatus(t *testing.T) {
	t.Parallel()

	t.Run("basic test", func(t *testing.T) {
		t.Parallel()

		// 使用noop processor
		tp := processor.NewTaskProcessor()
		tp.Register(entity.TaskTypeAutoEval, processor.NewNoopTaskProcessor())

		task := &StatusCheckTask{
			taskProcessor: *tp,
		}

		require.NotNil(t, task)
		require.NotNil(t, task.taskProcessor)
	})

	t.Run("task with success status should be skipped", func(t *testing.T) {
		t.Parallel()

		// 使用noop processor
		tp := processor.NewTaskProcessor()
		tp.Register(entity.TaskTypeAutoEval, processor.NewNoopTaskProcessor())

		task := &StatusCheckTask{
			taskProcessor: *tp,
		}

		tasks := []*entity.ObservabilityTask{
			{
				ID:         1,
				TaskStatus: entity.TaskStatusSuccess,
				TaskType:   entity.TaskTypeAutoEval,
			},
		}

		err := task.checkTaskStatus(context.Background(), tasks)
		require.NoError(t, err)
	})

	t.Run("task with failed status should be skipped", func(t *testing.T) {
		t.Parallel()

		// 使用noop processor
		tp := processor.NewTaskProcessor()
		tp.Register(entity.TaskTypeAutoEval, processor.NewNoopTaskProcessor())

		task := &StatusCheckTask{
			taskProcessor: *tp,
		}

		tasks := []*entity.ObservabilityTask{
			{
				ID:         1,
				TaskStatus: entity.TaskStatusFailed,
				TaskType:   entity.TaskTypeAutoEval,
			},
		}

		err := task.checkTaskStatus(context.Background(), tasks)
		require.NoError(t, err)
	})

	t.Run("task with disabled status should be skipped", func(t *testing.T) {
		t.Parallel()

		// 使用noop processor
		tp := processor.NewTaskProcessor()
		tp.Register(entity.TaskTypeAutoEval, processor.NewNoopTaskProcessor())

		task := &StatusCheckTask{
			taskProcessor: *tp,
		}

		tasks := []*entity.ObservabilityTask{
			{
				ID:         1,
				TaskStatus: entity.TaskStatusDisabled,
				TaskType:   entity.TaskTypeAutoEval,
			},
		}

		err := task.checkTaskStatus(context.Background(), tasks)
		require.NoError(t, err)
	})
}

func TestStatusCheckTask_syncTaskRunCount(t *testing.T) {
	t.Parallel()

	t.Run("basic functionality", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		task := &StatusCheckTask{}
		require.NotNil(t, task)
	})

	t.Run("sync with no task runs", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		task := &StatusCheckTask{}

		tasks := []*entity.ObservabilityTask{
			{
				ID:       1,
				TaskRuns: []*entity.TaskRun{},
			},
		}

		err := task.syncTaskRunCount(context.Background(), tasks)
		require.NoError(t, err)
	})
}

func TestStatusCheckTask_listNonFinalTasks(t *testing.T) {
	t.Parallel()

	t.Run("basic functionality", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		task := &StatusCheckTask{}
		require.NotNil(t, task)
	})
}

func TestStatusCheckTask_updateTaskRunDetail(t *testing.T) {
	t.Parallel()

	t.Run("basic functionality", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		task := &StatusCheckTask{}
		require.NotNil(t, task)
	})
}

func TestStatusCheckTask_listRecentTasks(t *testing.T) {
	t.Parallel()

	t.Run("basic functionality", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		task := &StatusCheckTask{}
		require.NotNil(t, task)
	})
}

func TestStatusCheckTask_processBatch(t *testing.T) {
	t.Parallel()

	t.Run("basic functionality", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		task := &StatusCheckTask{}
		require.NotNil(t, task)
	})
}

func TestStatusCheckTask_checkTaskStatus_BackfillLockAcquired_SendsMessage(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLocker := lockmocks.NewMockILocker(ctrl)
	mockTaskService := servicemocks.NewMockITaskService(ctrl)

	tp := processor.NewTaskProcessor()
	tp.Register(entity.TaskTypeAutoEval, processor.NewNoopTaskProcessor())

	st := &StatusCheckTask{
		locker:        mockLocker,
		taskService:   mockTaskService,
		taskProcessor: *tp,
	}

	now := time.Now()
	backfillRun := &entity.TaskRun{
		ID:         10,
		TaskType:   entity.TaskRunTypeBackFill,
		RunStatus:  entity.TaskRunStatusRunning,
		RunStartAt: now.Add(-2 * time.Hour),
		RunEndAt:   now.Add(2 * time.Hour),
	}

	tasks := []*entity.ObservabilityTask{
		{
			ID:          100,
			WorkspaceID: 200,
			TaskStatus:  entity.TaskStatusRunning,
			TaskType:    entity.TaskTypeAutoEval,
			BackfillEffectiveTime: &entity.EffectiveTime{
				StartAt: now.Add(-3 * time.Hour).UnixMilli(),
				EndAt:   now.Add(-1 * time.Hour).UnixMilli(),
			},
			EffectiveTime: &entity.EffectiveTime{
				StartAt: now.Add(-1 * time.Hour).UnixMilli(),
				EndAt:   now.Add(3 * time.Hour).UnixMilli(),
			},
			TaskRuns: []*entity.TaskRun{backfillRun},
		},
	}

	cancelCalled := false
	mockLocker.EXPECT().LockWithRenew(gomock.Any(), "observability:tracehub:backfill:100", syncTaskRunCountLockTTL, backfillLockMaxHold).
		Return(true, context.Background(), func() { cancelCalled = true }, nil)

	mockTaskService.EXPECT().SendBackfillMessage(gomock.Any(), &entity.BackFillEvent{
		TaskID:  100,
		SpaceID: 200,
	}).Return(nil)

	err := st.checkTaskStatus(context.Background(), tasks)
	assert.NoError(t, err)
	assert.True(t, cancelCalled, "cancel should be called when lock is acquired")
}

func TestStatusCheckTask_checkTaskStatus_BackfillLockNotAcquired_NoMessage(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLocker := lockmocks.NewMockILocker(ctrl)
	mockTaskService := servicemocks.NewMockITaskService(ctrl)

	tp := processor.NewTaskProcessor()
	tp.Register(entity.TaskTypeAutoEval, processor.NewNoopTaskProcessor())

	st := &StatusCheckTask{
		locker:        mockLocker,
		taskService:   mockTaskService,
		taskProcessor: *tp,
	}

	now := time.Now()
	backfillRun := &entity.TaskRun{
		ID:         10,
		TaskType:   entity.TaskRunTypeBackFill,
		RunStatus:  entity.TaskRunStatusRunning,
		RunStartAt: now.Add(-2 * time.Hour),
		RunEndAt:   now.Add(2 * time.Hour),
	}

	tasks := []*entity.ObservabilityTask{
		{
			ID:          100,
			WorkspaceID: 200,
			TaskStatus:  entity.TaskStatusRunning,
			TaskType:    entity.TaskTypeAutoEval,
			BackfillEffectiveTime: &entity.EffectiveTime{
				StartAt: now.Add(-3 * time.Hour).UnixMilli(),
				EndAt:   now.Add(-1 * time.Hour).UnixMilli(),
			},
			EffectiveTime: &entity.EffectiveTime{
				StartAt: now.Add(-1 * time.Hour).UnixMilli(),
				EndAt:   now.Add(3 * time.Hour).UnixMilli(),
			},
			TaskRuns: []*entity.TaskRun{backfillRun},
		},
	}

	mockLocker.EXPECT().LockWithRenew(gomock.Any(), "observability:tracehub:backfill:100", syncTaskRunCountLockTTL, backfillLockMaxHold).
		Return(false, context.Background(), func() {}, nil)

	// SendBackfillMessage should NOT be called when lock is not acquired
	mockTaskService.EXPECT().SendBackfillMessage(gomock.Any(), gomock.Any()).Times(0)

	err := st.checkTaskStatus(context.Background(), tasks)
	assert.NoError(t, err)
}

func TestStatusCheckTask_checkTaskStatus_BackfillLockError_SendsMessage(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLocker := lockmocks.NewMockILocker(ctrl)
	mockTaskService := servicemocks.NewMockITaskService(ctrl)

	tp := processor.NewTaskProcessor()
	tp.Register(entity.TaskTypeAutoEval, processor.NewNoopTaskProcessor())

	st := &StatusCheckTask{
		locker:        mockLocker,
		taskService:   mockTaskService,
		taskProcessor: *tp,
	}

	now := time.Now()
	backfillRun := &entity.TaskRun{
		ID:         10,
		TaskType:   entity.TaskRunTypeBackFill,
		RunStatus:  entity.TaskRunStatusRunning,
		RunStartAt: now.Add(-2 * time.Hour),
		RunEndAt:   now.Add(2 * time.Hour),
	}

	tasks := []*entity.ObservabilityTask{
		{
			ID:          100,
			WorkspaceID: 200,
			TaskStatus:  entity.TaskStatusRunning,
			TaskType:    entity.TaskTypeAutoEval,
			BackfillEffectiveTime: &entity.EffectiveTime{
				StartAt: now.Add(-3 * time.Hour).UnixMilli(),
				EndAt:   now.Add(-1 * time.Hour).UnixMilli(),
			},
			EffectiveTime: &entity.EffectiveTime{
				StartAt: now.Add(-1 * time.Hour).UnixMilli(),
				EndAt:   now.Add(3 * time.Hour).UnixMilli(),
			},
			TaskRuns: []*entity.TaskRun{backfillRun},
		},
	}

	mockLocker.EXPECT().LockWithRenew(gomock.Any(), "observability:tracehub:backfill:100", syncTaskRunCountLockTTL, backfillLockMaxHold).
		Return(false, context.Background(), func() {}, errors.New("redis error"))

	mockTaskService.EXPECT().SendBackfillMessage(gomock.Any(), &entity.BackFillEvent{
		TaskID:  100,
		SpaceID: 200,
	}).Return(nil)

	err := st.checkTaskStatus(context.Background(), tasks)
	assert.NoError(t, err)
}

func TestStatusCheckTask_checkTaskStatus_BackfillOnlyMode_LockAcquired_SendsMessage(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLocker := lockmocks.NewMockILocker(ctrl)
	mockTaskService := servicemocks.NewMockITaskService(ctrl)

	tp := processor.NewTaskProcessor()
	tp.Register(entity.TaskTypeAutoEval, processor.NewNoopTaskProcessor())

	st := &StatusCheckTask{
		locker:        mockLocker,
		taskService:   mockTaskService,
		taskProcessor: *tp,
	}

	now := time.Now()
	backfillRun := &entity.TaskRun{
		ID:         10,
		TaskType:   entity.TaskRunTypeBackFill,
		RunStatus:  entity.TaskRunStatusRunning,
		RunStartAt: now.Add(-2 * time.Hour),
		RunEndAt:   now.Add(2 * time.Hour),
	}

	// BackfillEffectiveTime set, EffectiveTime nil → enters second branch (line 178)
	tasks := []*entity.ObservabilityTask{
		{
			ID:          101,
			WorkspaceID: 201,
			TaskStatus:  entity.TaskStatusRunning,
			TaskType:    entity.TaskTypeAutoEval,
			BackfillEffectiveTime: &entity.EffectiveTime{
				StartAt: now.Add(-3 * time.Hour).UnixMilli(),
				EndAt:   now.Add(-1 * time.Hour).UnixMilli(),
			},
			TaskRuns: []*entity.TaskRun{backfillRun},
		},
	}

	cancelCalled := false
	mockLocker.EXPECT().LockWithRenew(gomock.Any(), "observability:tracehub:backfill:101", syncTaskRunCountLockTTL, backfillLockMaxHold).
		Return(true, context.Background(), func() { cancelCalled = true }, nil)

	mockTaskService.EXPECT().SendBackfillMessage(gomock.Any(), &entity.BackFillEvent{
		TaskID:  101,
		SpaceID: 201,
	}).Return(nil)

	err := st.checkTaskStatus(context.Background(), tasks)
	assert.NoError(t, err)
	assert.True(t, cancelCalled)
}
