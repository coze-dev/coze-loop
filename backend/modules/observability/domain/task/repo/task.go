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
	// task
	GetTask(ctx context.Context, id int64, workspaceID *int64, userID *string) (*entity.ObservabilityTask, error)
	ListTasks(ctx context.Context, param mysql.ListTaskParam) ([]*entity.ObservabilityTask, int64, error)
	UpdateTask(ctx context.Context, do *entity.ObservabilityTask) error
	CreateTask(ctx context.Context, do *entity.ObservabilityTask) (int64, error)
	DeleteTask(ctx context.Context, do *entity.ObservabilityTask) error
	UpdateTaskWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error

	// task run
	GetTaskRun(ctx context.Context, id int64, workspaceID *int64, userID *string) (*entity.TaskRun, error)
	ListTaskRuns(ctx context.Context, taskID int64, param mysql.ListTaskRunParam) ([]*entity.TaskRun, int64, error)
	UpdateTaskRun(ctx context.Context, do *entity.TaskRun) error
	CreateTaskRun(ctx context.Context, do *entity.TaskRun) (int64, error)
	UpdateTaskRunWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error

	// task count
	GetTaskCount(ctx context.Context, taskID int64) (int64, error)
	IncrTaskCount(ctx context.Context, taskID int64) error
	DecrTaskCount(ctx context.Context, taskID int64) error

	// task run count
	GetTaskRunCount(ctx context.Context, taskID, taskRunID int64) (int64, error)
	IncrTaskRunCount(ctx context.Context, taskID, taskRunID int64) error
	DecrTaskRunCount(ctx context.Context, taskID, taskRunID int64) error

	// task run success/fail count
	IncrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) error
	DecrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) error
	IncrTaskRunFailCount(ctx context.Context, taskID, taskRunID int64) error
	GetTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) (int64, error)
	GetTaskRunFailCount(ctx context.Context, taskID, taskRunID int64) (int64, error)

	//
	ListNonFinalTask(ctx context.Context) ([]*entity.ObservabilityTask, error)
	GetObjListWithTask(ctx context.Context) ([]string, []string, []*entity.ObservabilityTask)
	ListNonFinalTaskBySpaceID(ctx context.Context, spaceID string) []*entity.ObservabilityTask

	// 获取所有TaskRunCount键
	GetAllTaskRunCountKeys(ctx context.Context) ([]string, error)
}
