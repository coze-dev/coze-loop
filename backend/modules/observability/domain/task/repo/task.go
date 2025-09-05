// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
)

//go:generate mockgen -destination=mocks/Task.go -package=mocks . ITaskRepo
type ITaskRepo interface {
	GetTask(ctx context.Context, id int64, workspaceID *int64, userID *string) (*entity.ObservabilityTask, error)
	ListTasks(ctx context.Context, param mysql.ListTaskParam) ([]*entity.ObservabilityTask, int64, error)
	UpdateTask(ctx context.Context, do *entity.ObservabilityTask) error
	CreateTask(ctx context.Context, do *entity.ObservabilityTask) (int64, error)
	DeleteTask(ctx context.Context, id int64, workspaceID int64, userID string) error
	ListNonFinalTask(ctx context.Context) ([]*entity.ObservabilityTask, error)
	UpdateTaskWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error
}
