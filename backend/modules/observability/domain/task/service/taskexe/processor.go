// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package taskexe

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type Trigger struct {
	Task    *entity.ObservabilityTask
	Span    *loop_span.Span
	TaskRun *entity.TaskRun
}

type OnTaskRunCreatedReq struct {
	CurrentTask *entity.ObservabilityTask
	RunType     entity.TaskRunType
	RunStartAt  int64
	RunEndAt    int64
}
type OnTaskRunFinishedReq struct {
	Task    *entity.ObservabilityTask
	TaskRun *entity.TaskRun
}

type OnTaskFinishedReq struct {
	Task     *entity.ObservabilityTask
	TaskRun  *entity.TaskRun
	IsFinish bool
}

type Processor interface {
	ValidateConfig(ctx context.Context, config any) error
	Invoke(ctx context.Context, trigger *Trigger) error

	OnTaskCreated(ctx context.Context, currentTask *entity.ObservabilityTask) error
	OnTaskUpdated(ctx context.Context, currentTask *entity.ObservabilityTask, taskOp entity.TaskStatus) error
	OnTaskFinished(ctx context.Context, param OnTaskFinishedReq) error

	OnTaskRunCreated(ctx context.Context, param OnTaskRunCreatedReq) error
	OnTaskRunFinished(ctx context.Context, param OnTaskRunFinishedReq) error
}
