// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
)

//go:generate mockgen -destination=mocks/task_run.go -package=mocks . ITaskRunRepo
type ITaskRunRepo interface {
	// 基础CRUD操作
	GetTaskRun(ctx context.Context, id int64, workspaceID *int64, taskID *int64) (*entity.TaskRun, error)
	CreateTaskRun(ctx context.Context, do *entity.TaskRun) (int64, error)
	UpdateTaskRun(ctx context.Context, do *entity.TaskRun) error
	DeleteTaskRun(ctx context.Context, id int64, workspaceID int64, userID string) error
	ListTaskRuns(ctx context.Context, param mysql.ListTaskRunParam) ([]*entity.TaskRun, int64, error)
	
	// 业务特定操作
	ListNonFinalTaskRun(ctx context.Context) ([]*entity.TaskRun, error)
	ListNonFinalTaskRunByTaskID(ctx context.Context, taskID int64) ([]*entity.TaskRun, error)
	ListNonFinalTaskRunBySpaceID(ctx context.Context, spaceID string) []*entity.TaskRun
	UpdateTaskRunWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error
	GetObjListWithTaskRun(ctx context.Context) ([]string, []string)
	
	// TaskRun特有操作
	ListActiveTaskRunsByTask(ctx context.Context, taskID int64) ([]*entity.TaskRun, error)
	GetLatestTaskRunByTask(ctx context.Context, taskID int64) (*entity.TaskRun, error)
	ListTaskRunsByStatus(ctx context.Context, status string) ([]*entity.TaskRun, error)
	GetTaskRunCount(ctx context.Context, taskID, taskRunID int64) (int64, error)
}