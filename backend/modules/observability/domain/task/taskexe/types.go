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
	Task     *task.Task
	Span     *loop_span.Span
	IsFinish bool
}

var (
	ErrInvalidConfig  = errors.New("invalid config")
	ErrInvalidTrigger = errors.New("invalid span trigger")
)

type TaskOp string

const (
	TaskOpUndefined      TaskOp = "undefined"
	TaskOpCreateBackfill TaskOp = "create_backfill"
	TaskOpNewData        TaskOp = "create_new_data"
	TaskOpFinish         TaskOp = "finish"
)

type Processor interface {
	ValidateConfig(ctx context.Context, config any) error                                 // 校验配置项是否有效
	Invoke(ctx context.Context, config any, trigger *Trigger) error                       //根据不同类型进行执行，如rpc回调、mq投递等
	OnChangeProcessor(ctx context.Context, task *task.Task, taskOp task.TaskStatus) error //OnchangeProcessor 调用 evaluation 接口进行前期物料准备
	Finish(ctx context.Context, config any, trigger *Trigger) error                       //Finish

	OnCreateChangeProcessor(ctx context.Context, task *task.Task) error //OnCreateChangeProcessor
	OnUpdateChangeProcessor(ctx context.Context, task *task.Task) error //OnUpdateChangeProcessor
	OnFinishChangeProcessor(ctx context.Context, task *task.Task) error //OnFinishChangeProcessor

	OnCreateTaskRunProcessor(ctx context.Context, currentTask *task.Task, runConfig *task.TaskRunConfig, runType task.TaskRunType) (*task_entity.TaskRun, error) //OnCreateTaskRunProcessor
	OnFinishTaskRunProcessor(ctx context.Context, taskRun *task_entity.TaskRun) error                                                                            //OnFinishTaskRunProcessor
}

type ProcessorUnion interface {
	Processor
}
