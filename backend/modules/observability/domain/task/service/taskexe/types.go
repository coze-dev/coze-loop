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
type Config struct {
	DatasetID *int64 `json:"dataset_id"`
	Task      *task.Task
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

type OnCreateTaskRunChangeReq struct {
	CurrentTask *task.Task
	RunType     task.TaskRunType
	RunStartAt  int64
	RunEndAt    int64
}
type OnFinishTaskRunChangeReq struct {
	Task    *task.Task
	TaskRun *task_entity.TaskRun
}
type OnFinishTaskChangeReq struct {
	Task     *task.Task
	TaskRun  *task_entity.TaskRun
	IsFinish bool
}

type EndTaskError struct {
	TaskID int64
	Status task.TaskStatus
	Reason string
}
type Processor interface {
	ValidateConfig(ctx context.Context, config any) error           // 校验配置项是否有效
	Invoke(ctx context.Context, config any, trigger *Trigger) error // 根据不同类型进行执行，如rpc回调、mq投递等

	//Finish(ctx context.Context, config any, trigger *Trigger) error //Finish
	//OnCreateChangeProcessor(ctx context.Context, task *task.Task) error //OnchangeProcessor 调用 evaluation 接口进行前期物料准备

	//OnChangeProcessor(ctx context.Context, config *Config, isBackFill bool) error //OnCreateChangeProcessor
	//OnUpdateChangeProcessor(ctx context.Context, currentTask *task.Task, taskOp task.TaskStatus) error //OnUpdateChangeProcessor
	//OnFinishChangeProcessor(ctx context.Context, task *task.Task) error                                //OnFinishChangeProcessor

	//OnCreateTaskRunProcessor(ctx context.Context, currentTask *task.Task, runConfig *task.TaskRunConfig, runType task.TaskRunType) (*task_entity.TaskRun, error) //OnCreateTaskRunProcessor
	//OnFinishTaskRunProcessor(ctx context.Context, taskRun *task_entity.TaskRun) error                                                                            //OnFinishTaskRunProcessor

	OnCreateTaskChange(ctx context.Context, currentTask *task.Task) error
	OnUpdateTaskChange(ctx context.Context, currentTask *task.Task, taskOp task.TaskStatus) error
	OnFinishTaskChange(ctx context.Context, param OnFinishTaskChangeReq) error

	OnCreateTaskRunChange(ctx context.Context, param OnCreateTaskRunChangeReq) error
	OnFinishTaskRunChange(ctx context.Context, param OnFinishTaskRunChangeReq) error
}

type ProcessorUnion interface {
	Processor
}
