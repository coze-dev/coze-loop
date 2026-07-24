// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"
	"sync"

	"github.com/bytedance/gg/gptr"

	taskdomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	taskapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task/taskservice"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type TaskRPCAdapter struct {
	clientFactory func() taskservice.Client

	client taskservice.Client
	mu     sync.Mutex
}

// NewTaskRPCAdapter takes a factory rather than a resolved client so the
// underlying taskservice.Client is created on first use instead of at wire
// time. The client is backed by ObservabilityHandler, which is constructed
// after the evaluation applications, so resolving it eagerly would dereference
// a still-nil handler. This mirrors how the trajectory adapter defers its
// tracer factory.
func NewTaskRPCAdapter(clientFactory func() taskservice.Client) rpc.ITaskRPCAdapter {
	return &TaskRPCAdapter{
		clientFactory: clientFactory,
	}
}

func (t *TaskRPCAdapter) getClient() taskservice.Client {
	if t.client != nil {
		return t.client
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.client == nil {
		t.client = t.clientFactory()
	}
	return t.client
}

func (t *TaskRPCAdapter) ListTasks(ctx context.Context, param *rpc.ListTasksParam) (tasks []*taskdomain.Task, total *int64, err error) {
	req := &taskapi.ListTasksRequest{
		WorkspaceID: param.WorkspaceID,
	}
	if param.TaskFilters != nil {
		req.TaskFilters = param.TaskFilters
	}
	if param.Limit != nil {
		req.Limit = param.Limit
	}
	if param.Offset != nil {
		req.Offset = param.Offset
	}
	if param.OrderBy != nil {
		req.OrderBy = param.OrderBy
	}

	resp, err := t.getClient().ListTasks(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	if resp == nil {
		return nil, nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}

	if resp.Tasks != nil {
		tasks = resp.Tasks
	}
	if resp.Total != nil {
		total = gptr.Of(*resp.Total)
	}
	return tasks, total, nil
}
