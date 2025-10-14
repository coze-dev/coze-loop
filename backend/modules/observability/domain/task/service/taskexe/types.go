// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package taskexe

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	task_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type Trigger struct {
	Task    *task_entity.ObservabilityTask
	Span    *loop_span.Span
	TaskRun *task_entity.TaskRun
}

var (
	ErrInvalidConfig  = errors.New("invalid config")
	ErrInvalidTrigger = errors.New("invalid span trigger")
)

type OnCreateTaskRunChangeReq struct {
	CurrentTask *task_entity.ObservabilityTask
	RunType     task.TaskRunType
	RunStartAt  int64
	RunEndAt    int64
}
type OnFinishTaskRunChangeReq struct {
	Task    *task_entity.ObservabilityTask
	TaskRun *task_entity.TaskRun
}
type OnFinishTaskChangeReq struct {
	Task     *task_entity.ObservabilityTask
	TaskRun  *task_entity.TaskRun
	IsFinish bool
}

type Processor interface {
	ValidateConfig(ctx context.Context, config any) error
	Invoke(ctx context.Context, trigger *Trigger) error

	OnCreateTaskChange(ctx context.Context, currentTask *task_entity.ObservabilityTask) error
	OnUpdateTaskChange(ctx context.Context, currentTask *task_entity.ObservabilityTask, taskOp task.TaskStatus) error
	OnFinishTaskChange(ctx context.Context, param OnFinishTaskChangeReq) error

	OnCreateTaskRunChange(ctx context.Context, param OnCreateTaskRunChangeReq) error
	OnFinishTaskRunChange(ctx context.Context, param OnFinishTaskRunChangeReq) error
}

type ProcessorUnion interface {
	Processor
}
