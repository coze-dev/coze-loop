// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
)

//go:generate mockgen -destination=mocks/task_hook.go -package=mocks . ITaskHookProvider

type WorkflowCallbackParam struct {
	Task    *entity.ObservabilityTask
	TaskRun *entity.TaskRun
}

type ITaskHookProvider interface {
	WorkflowCallback(ctx context.Context, event *WorkflowCallbackParam) error
}

type NoopTaskHookProvider struct{}

func NewNoopTaskHookProvider() ITaskHookProvider {
	return &NoopTaskHookProvider{}
}

func (n *NoopTaskHookProvider) WorkflowCallback(ctx context.Context, event *WorkflowCallbackParam) error {
	return nil
}
