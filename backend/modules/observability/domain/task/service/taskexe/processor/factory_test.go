// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
)

func TestTaskProcessor_RegisterAndGet(t *testing.T) {
	t.Parallel()

	taskProcessor := NewTaskProcessor()

	defaultProcessor := taskProcessor.GetTaskProcessor("unknown")
	_, ok := defaultProcessor.(*NoopTaskProcessor)
	assert.True(t, ok)

	registered := NewNoopTaskProcessor()
	taskProcessor.Register(task.TaskTypeAutoEval, registered)
	assert.Equal(t, registered, taskProcessor.GetTaskProcessor(task.TaskTypeAutoEval))
}

func TestNoopTaskProcessor_Methods(t *testing.T) {
	t.Parallel()
	p := NewNoopTaskProcessor()
	ctx := context.Background()

	assert.NoError(t, p.ValidateConfig(ctx, nil))
	assert.NoError(t, p.Invoke(ctx, nil))
	assert.NoError(t, p.OnCreateTaskChange(ctx, nil))
	assert.NoError(t, p.OnUpdateTaskChange(ctx, nil, task.TaskStatusRunning))
	assert.NoError(t, p.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{}))
	assert.NoError(t, p.OnCreateTaskRunChange(ctx, taskexe.OnCreateTaskRunChangeReq{}))
	assert.NoError(t, p.OnFinishTaskRunChange(ctx, taskexe.OnFinishTaskRunChangeReq{}))
}
