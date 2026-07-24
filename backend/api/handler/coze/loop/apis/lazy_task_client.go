// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"context"
	"sync"

	"github.com/cloudwego/kitex/client/callopt"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task/taskservice"
)

// lazyTaskClient defers resolving the underlying taskservice.Client until it is
// first used at request time.
//
// EvaluationHandler and ObservabilityHandler depend on each other: the task
// client is backed by ObservabilityHandler.ITaskApplication, but
// ObservabilityHandler is constructed only after InitEvaluationHandler returns.
// Resolving the client eagerly during InitEvaluationHandler therefore
// dereferences a still-nil ObservabilityHandler and panics. Wrapping the
// factory in a lazy proxy breaks that cycle, mirroring how tracerFactory is
// resolved lazily by the trajectory adapter.
type lazyTaskClient struct {
	factory func() taskservice.Client
	once    sync.Once
	client  taskservice.Client
}

func newLazyTaskClient(factory func() taskservice.Client) taskservice.Client {
	return &lazyTaskClient{factory: factory}
}

func (l *lazyTaskClient) resolve() taskservice.Client {
	l.once.Do(func() {
		l.client = l.factory()
	})
	return l.client
}

func (l *lazyTaskClient) CheckTaskName(ctx context.Context, req *task.CheckTaskNameRequest, callOptions ...callopt.Option) (*task.CheckTaskNameResponse, error) {
	return l.resolve().CheckTaskName(ctx, req, callOptions...)
}

func (l *lazyTaskClient) CreateTask(ctx context.Context, req *task.CreateTaskRequest, callOptions ...callopt.Option) (*task.CreateTaskResponse, error) {
	return l.resolve().CreateTask(ctx, req, callOptions...)
}

func (l *lazyTaskClient) UpdateTask(ctx context.Context, req *task.UpdateTaskRequest, callOptions ...callopt.Option) (*task.UpdateTaskResponse, error) {
	return l.resolve().UpdateTask(ctx, req, callOptions...)
}

func (l *lazyTaskClient) ListTasks(ctx context.Context, req *task.ListTasksRequest, callOptions ...callopt.Option) (*task.ListTasksResponse, error) {
	return l.resolve().ListTasks(ctx, req, callOptions...)
}

func (l *lazyTaskClient) GetTask(ctx context.Context, req *task.GetTaskRequest, callOptions ...callopt.Option) (*task.GetTaskResponse, error) {
	return l.resolve().GetTask(ctx, req, callOptions...)
}
