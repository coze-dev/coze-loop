// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe"
)

var _ taskexe.Processor = (*DataReflowProcessor)(nil)

type DataReflowProcessor struct {
}

func newDataReflowProcessor() *DataReflowProcessor {
	return &DataReflowProcessor{}
}

func (p *DataReflowProcessor) ValidateConfig(ctx context.Context, config any, workspaceID int64) error {

	return nil
}

func (p *DataReflowProcessor) Invoke(ctx context.Context, config any, trigger *taskexe.Trigger) error {
	return nil
}

func (p *DataReflowProcessor) Finish(ctx context.Context, config any, trigger *taskexe.Trigger) error {
	return nil
}

func (p *DataReflowProcessor) OnChangeProcessor(ctx context.Context, currentTask *task.Task, taskOp task.TaskStatus) error {
	return nil
}
