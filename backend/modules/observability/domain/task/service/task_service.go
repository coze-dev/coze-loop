// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
)

type CreateTaskReq struct {
}
type CreateTaskResp struct {
}
type UpdateTaskReq struct {
}
type UpdateTaskResp struct {
}
type ListTasksReq struct {
}
type ListTasksResp struct {
}
type GetTaskReq struct {
}
type GetTaskResp struct {
}
type CheckTaskNameReq struct {
}
type CheckTaskNameResp struct {
}

//go:generate mockgen -destination=mocks/task_service.go -package=mocks . ITaskService
type ITaskService interface {
	CreateTask(ctx context.Context, req *CreateTaskReq) (resp *CreateTaskResp, err error)
	UpdateTask(ctx context.Context, req *UpdateTaskReq) (resp *UpdateTaskResp, err error)
	ListTasks(ctx context.Context, req *ListTasksReq) (resp *ListTasksResp, err error)
	GetTask(ctx context.Context, req *GetTaskReq) (resp *GetTaskResp, err error)
	CheckTaskName(ctx context.Context, req *CheckTaskNameReq) (resp *CheckTaskNameResp, err error)
}

func NewTaskServiceImpl(
	tRepo repo.ITaskRepo,
	tenantProvider tenant.ITenantProvider,
	evalServiceAdaptor rpc.IEvaluatorRPCAdapter,
) (ITaskService, error) {
	return &TaskServiceImpl{
		TaskRepo:           tRepo,
		tenantProvider:     tenantProvider,
		evalServiceAdaptor: evalServiceAdaptor,
	}, nil
}

type TaskServiceImpl struct {
	TaskRepo           repo.ITaskRepo
	tenantProvider     tenant.ITenantProvider
	evalServiceAdaptor rpc.IEvaluatorRPCAdapter
}

func (t *TaskServiceImpl) CreateTask(ctx context.Context, req *CreateTaskReq) (resp *CreateTaskResp, err error) {
	return nil, nil
}
func (t *TaskServiceImpl) UpdateTask(ctx context.Context, req *UpdateTaskReq) (resp *UpdateTaskResp, err error) {
	return nil, nil
}
func (t *TaskServiceImpl) ListTasks(ctx context.Context, req *ListTasksReq) (resp *ListTasksResp, err error) {
	return nil, nil
}
func (t *TaskServiceImpl) GetTask(ctx context.Context, req *GetTaskReq) (resp *GetTaskResp, err error) {
	return nil, nil
}
func (t *TaskServiceImpl) CheckTaskName(ctx context.Context, req *CheckTaskNameReq) (resp *CheckTaskNameResp, err error) {
	return nil, nil
}
