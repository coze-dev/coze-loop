// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/convertor"
)

func NewTaskRepoImpl(TaskDao mysql.ITaskDao, idGenerator idgen.IIDGenerator) repo.ITaskRepo {
	return &TaskRepoImpl{
		TaskDao:     TaskDao,
		idGenerator: idGenerator,
	}
}

type TaskRepoImpl struct {
	TaskDao     mysql.ITaskDao
	idGenerator idgen.IIDGenerator
}

func (v *TaskRepoImpl) GetTask(ctx context.Context, id int64, workspaceID *int64, userID *string) (*entity.ObservabilityTask, error) {
	TaskPo, err := v.TaskDao.GetTask(ctx, id, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	return convertor.TaskPO2DO(TaskPo), nil
}

func (v *TaskRepoImpl) ListTasks(ctx context.Context, workspaceID int64, userID string) ([]*entity.ObservabilityTask, error) {
	results, err := v.TaskDao.ListTasks(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	resp := make([]*entity.ObservabilityTask, len(results))
	for i, result := range results {
		resp[i] = convertor.TaskPO2DO(result)
	}
	return resp, nil
}

func (v *TaskRepoImpl) CreateTask(ctx context.Context, do *entity.ObservabilityTask) (int64, error) {
	id, err := v.idGenerator.GenID(ctx)
	if err != nil {
		return 0, err
	}
	TaskPo := convertor.TaskDO2PO(do)
	TaskPo.ID = id
	return v.TaskDao.CreateTask(ctx, TaskPo)
}

func (v *TaskRepoImpl) UpdateTask(ctx context.Context, do *entity.ObservabilityTask) error {
	TaskPo := convertor.TaskDO2PO(do)
	return v.TaskDao.UpdateTask(ctx, TaskPo)
}

func (v *TaskRepoImpl) DeleteTask(ctx context.Context, id int64, workspaceID int64, userID string) error {
	return v.TaskDao.DeleteTask(ctx, id, workspaceID, userID)
}
