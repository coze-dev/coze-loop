// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"

	"github.com/bytedance/gg/gptr"

	taskapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task"
	taskdomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task/taskservice"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type TaskRPCAdapter struct {
	client taskservice.Client
}

func NewTaskRPCAdapter(client taskservice.Client) rpc.ITaskRPCAdapter {
	return &TaskRPCAdapter{
		client: client,
	}
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

	resp, err := t.client.ListTasks(ctx, req)
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
