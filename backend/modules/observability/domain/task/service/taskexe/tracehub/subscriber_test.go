// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type noopProcessor struct{ invoked bool }

func (n *noopProcessor) ValidateConfig(ctx context.Context, config any) error { return nil }
func (n *noopProcessor) Invoke(ctx context.Context, trigger *taskexe.Trigger) error {
	n.invoked = true
	return nil
}

func (n *noopProcessor) OnTaskRunCreated(ctx context.Context, param taskexe.OnTaskRunCreatedReq) error {
	return nil
}

func (n *noopProcessor) OnTaskRunFinished(ctx context.Context, param taskexe.OnTaskRunFinishedReq) error {
	return nil
}

func (n *noopProcessor) OnTaskFinished(ctx context.Context, param taskexe.OnTaskFinishedReq) error {
	return nil
}

func (n *noopProcessor) OnTaskUpdated(ctx context.Context, currentTask *entity.ObservabilityTask, taskOp entity.TaskStatus) error {
	return nil
}

func (n *noopProcessor) OnTaskCreated(ctx context.Context, currentTask *entity.ObservabilityTask) error {
	return nil
}

func TestSpanSubscriber_AddSpan_SkipNonRunning(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	proc := &noopProcessor{}

	task := &entity.ObservabilityTask{ID: 42, WorkspaceID: 7, TaskStatus: entity.TaskStatusRunning}
	sub := &spanSubscriber{
		taskID:    task.ID,
		t:         task,
		processor: proc,
		taskRepo:  mockRepo,
		runType:   entity.TaskRunTypeNewData,
	}

	run := &entity.TaskRun{
		ID:          1001,
		TaskID:      task.ID,
		WorkspaceID: task.WorkspaceID,
		TaskType:    entity.TaskRunTypeNewData,
		RunStatus:   entity.TaskRunStatusDone,
		RunStartAt:  time.Now().Add(-time.Minute),
		RunEndAt:    time.Now().Add(time.Minute),
	}
	mockRepo.EXPECT().GetLatestNewDataTaskRun(gomock.Any(), gomock.Nil(), task.ID).Return(run, nil)

	span := &loop_span.Span{TraceID: "trace", SpanID: "span", StartTime: time.Now().UnixMilli()}
	err := sub.AddSpan(context.Background(), span)
	assert.NoError(t, err)
	assert.False(t, proc.invoked, "Invoke should not be called for non-running TaskRun")
}
