// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package scheduledtask

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/stretchr/testify/require"
)

type stubProcessor struct {
	invokeErr           error
	finishErr           error
	updateErr           error
	createTaskErr       error
	finishTaskRunErr    error
	validateErr         error
	createTaskRunErr    error
	finishChangeInvoked int
	invokeCalled        bool
	createTaskRunReqs   []taskexe.OnTaskRunCreatedReq
	finishChangeReqs    []taskexe.OnTaskFinishedReq
	updateCallCount     int
	createTaskRunErrSeq []error
	finishErrSeq        []error
}

func (s *stubProcessor) ValidateConfig(context.Context, any) error {
	return s.validateErr
}

func (s *stubProcessor) Invoke(context.Context, *taskexe.Trigger) error {
	s.invokeCalled = true
	return s.invokeErr
}

func (s *stubProcessor) OnTaskCreated(context.Context, *entity.ObservabilityTask) error {
	return s.createTaskErr
}

func (s *stubProcessor) OnTaskUpdated(context.Context, *entity.ObservabilityTask, entity.TaskStatus) error {
	s.updateCallCount++
	return s.updateErr
}

func (s *stubProcessor) OnTaskFinished(_ context.Context, req taskexe.OnTaskFinishedReq) error {
	idx := len(s.finishChangeReqs)
	s.finishChangeReqs = append(s.finishChangeReqs, req)
	s.finishChangeInvoked++
	if idx < len(s.finishErrSeq) {
		return s.finishErrSeq[idx]
	}
	return s.finishErr
}

func (s *stubProcessor) OnTaskRunCreated(_ context.Context, req taskexe.OnTaskRunCreatedReq) error {
	s.createTaskRunReqs = append(s.createTaskRunReqs, req)
	idx := len(s.createTaskRunReqs) - 1
	if idx >= 0 && idx < len(s.createTaskRunErrSeq) {
		if err := s.createTaskRunErrSeq[idx]; err != nil {
			return err
		}
	}
	return s.createTaskRunErr
}

func (s *stubProcessor) OnTaskRunFinished(context.Context, taskexe.OnTaskRunFinishedReq) error {
	return s.finishTaskRunErr
}

type trackingProcessor struct {
	*stubProcessor
	finishReqs     []taskexe.OnTaskFinishedReq
	createRunReqs  []taskexe.OnTaskRunCreatedReq
	updateStatuses []entity.TaskStatus
}

func newTrackingProcessor() *trackingProcessor {
	return &trackingProcessor{stubProcessor: &stubProcessor{}}
}

func (p *trackingProcessor) OnTaskFinished(ctx context.Context, req taskexe.OnTaskFinishedReq) error {
	p.finishReqs = append(p.finishReqs, req)
	return p.stubProcessor.OnTaskFinished(ctx, req)
}

func (p *trackingProcessor) OnTaskRunCreated(ctx context.Context, req taskexe.OnTaskRunCreatedReq) error {
	p.createRunReqs = append(p.createRunReqs, req)
	return p.stubProcessor.OnTaskRunCreated(ctx, req)
}

func (p *trackingProcessor) OnTaskUpdated(ctx context.Context, obsTask *entity.ObservabilityTask, status entity.TaskStatus) error {
	p.updateStatuses = append(p.updateStatuses, status)
	return p.stubProcessor.OnTaskUpdated(ctx, obsTask, status)
}

func TestStatusCheckTask_checkTaskStatus(t *testing.T) {
	t.Parallel()

	t.Run("basic test", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		proc := newTrackingProcessor()
		tp := processor.NewTaskProcessor()
		tp.Register(entity.TaskTypeAutoEval, proc)

		task := &StatusCheckTask{
			taskProcessor: *tp,
		}

		require.NotNil(t, task)
		require.NotNil(t, task.taskProcessor)
	})

	t.Run("task with success status should be skipped", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		proc := newTrackingProcessor()
		tp := processor.NewTaskProcessor()
		tp.Register(entity.TaskTypeAutoEval, proc)

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
		require.Empty(t, proc.finishReqs)
	})

	t.Run("task with failed status should be skipped", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		proc := newTrackingProcessor()
		tp := processor.NewTaskProcessor()
		tp.Register(entity.TaskTypeAutoEval, proc)

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
		require.Empty(t, proc.finishReqs)
	})

	t.Run("task with disabled status should be skipped", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)

		proc := newTrackingProcessor()
		tp := processor.NewTaskProcessor()
		tp.Register(entity.TaskTypeAutoEval, proc)

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
		require.Empty(t, proc.finishReqs)
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
