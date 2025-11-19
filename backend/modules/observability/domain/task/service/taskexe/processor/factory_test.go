// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
)

func TestTaskProcessor_RegisterAndGet(t *testing.T) {
	t.Parallel()

	taskProcessor := NewTaskProcessor()

	defaultProcessor := taskProcessor.GetTaskProcessor(entity.TaskType("unknown"))
	_, ok := defaultProcessor.(*NoopTaskProcessor)
	assert.True(t, ok)

	registered := NewNoopTaskProcessor()
	taskProcessor.Register(entity.TaskTypeAutoEval, registered)
	assert.Equal(t, registered, taskProcessor.GetTaskProcessor(entity.TaskTypeAutoEval))
}

func TestNoopTaskProcessor_Methods(t *testing.T) {
	t.Parallel()
	p := NewNoopTaskProcessor()
	ctx := context.Background()

	assert.NoError(t, p.ValidateConfig(ctx, nil))
	assert.NoError(t, p.Invoke(ctx, nil))
	assert.NoError(t, p.OnTaskCreated(ctx, nil))
	assert.NoError(t, p.OnTaskUpdated(ctx, nil, task.TaskStatusRunning))
	assert.NoError(t, p.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{}))
	assert.NoError(t, p.OnTaskRunCreated(ctx, taskexe.OnTaskRunCreatedReq{}))
	assert.NoError(t, p.OnTaskRunFinished(ctx, taskexe.OnTaskRunFinishedReq{}))
}
