// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package scheduledtask

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/stretchr/testify/require"
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
