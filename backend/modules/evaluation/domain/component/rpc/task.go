// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	taskcommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	taskfilter "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	taskdomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
)

//go:generate mockgen -destination=mocks/task.go -package=mocks . ITaskRPCAdapter
type ITaskRPCAdapter interface {
	ListTasks(ctx context.Context, param *ListTasksParam) (tasks []*taskdomain.Task, total *int64, err error)
}

type ListTasksParam struct {
	WorkspaceID int64
	TaskFilters *taskfilter.TaskFilterFields
	Limit       *int32
	Offset      *int32
	OrderBy     *taskcommon.OrderBy
}
