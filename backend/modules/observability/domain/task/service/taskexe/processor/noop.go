// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
)

var _ taskexe.Processor = (*NoopTaskProcessor)(nil)

type NoopTaskProcessor struct {
}

func NewNoopTaskProcessor() *NoopTaskProcessor {
	return &NoopTaskProcessor{}
}

func (p *NoopTaskProcessor) ValidateConfig(ctx context.Context, config any) error {
	return nil
}

func (p *NoopTaskProcessor) Invoke(ctx context.Context, trigger *taskexe.Trigger) error {
	return nil
}

func (p *NoopTaskProcessor) OnCreateTaskChange(ctx context.Context, currentTask *task.Task) error {
	return nil
}

func (p *NoopTaskProcessor) OnUpdateTaskChange(ctx context.Context, currentTask *task.Task, taskOp task.TaskStatus) error {
	return nil
}

func (p *NoopTaskProcessor) OnFinishTaskChange(ctx context.Context, param taskexe.OnFinishTaskChangeReq) error {
	return nil
}

func (p *NoopTaskProcessor) OnCreateTaskRunChange(ctx context.Context, param taskexe.OnCreateTaskRunChangeReq) error {
	return nil
}

func (p *NoopTaskProcessor) OnFinishTaskRunChange(ctx context.Context, param taskexe.OnFinishTaskRunChangeReq) error {
	return nil
}
