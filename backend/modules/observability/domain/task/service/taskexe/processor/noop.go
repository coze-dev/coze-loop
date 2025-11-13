// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
)

var _ taskexe.Processor = (*NoopTaskProcessor)(nil)

type NoopTaskProcessor struct{}

func NewNoopTaskProcessor() *NoopTaskProcessor {
	return &NoopTaskProcessor{}
}

func (p *NoopTaskProcessor) ValidateConfig(ctx context.Context, config any) error {
	return nil
}

func (p *NoopTaskProcessor) Invoke(ctx context.Context, trigger *taskexe.Trigger) error {
	return nil
}

func (p *NoopTaskProcessor) OnTaskCreated(ctx context.Context, currentTask *entity.ObservabilityTask) error {
	return nil
}

func (p *NoopTaskProcessor) OnTaskUpdated(ctx context.Context, currentTask *entity.ObservabilityTask, taskOp entity.TaskStatus) error {
	return nil
}

func (p *NoopTaskProcessor) OnTaskFinished(ctx context.Context, param taskexe.OnTaskFinishedReq) error {
	return nil
}

func (p *NoopTaskProcessor) OnTaskRunCreated(ctx context.Context, param taskexe.OnTaskRunCreatedReq) error {
	return nil
}

func (p *NoopTaskProcessor) OnTaskRunFinished(ctx context.Context, param taskexe.OnTaskRunFinishedReq) error {
	return nil
}
