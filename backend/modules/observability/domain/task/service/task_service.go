// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type CreateTaskReq struct {
	Task *task.Task
}
type CreateTaskResp struct {
	TaskID *int64
}
type UpdateTaskReq struct {
	TaskID        int64
	WorkspaceID   int64
	TaskStatus    task.TaskStatus
	Description   string
	EffectiveTime *task.EffectiveTime
	SampleRate    float64
}
type ListTasksReq struct {
	WorkspaceID int64
	TaskFilters *filter.TaskFilterFields
	Limit       int32
	Offset      int32
	OrderBy     common.OrderBy
}
type ListTasksResp struct {
	Tasks []*task.Task
	Total *int64
}
type GetTaskReq struct {
	TaskID      int64
	WorkspaceID int64
}
type GetTaskResp struct {
	Task *task.Task
}
type CheckTaskNameReq struct {
	WorkspaceID int64
	Name        string
}
type CheckTaskNameResp struct {
	Pass *bool
}

//go:generate mockgen -destination=mocks/task_service.go -package=mocks . ITaskService
type ITaskService interface {
	CreateTask(ctx context.Context, req *CreateTaskReq) (resp *CreateTaskResp, err error)
	UpdateTask(ctx context.Context, req *UpdateTaskReq) (err error)
	ListTasks(ctx context.Context, req *ListTasksReq) (resp *ListTasksResp, err error)
	GetTask(ctx context.Context, req *GetTaskReq) (resp *GetTaskResp, err error)
	CheckTaskName(ctx context.Context, req *CheckTaskNameReq) (resp *CheckTaskNameResp, err error)
}

func NewTaskServiceImpl(
	tRepo repo.ITaskRepo,
	tenantProvider tenant.ITenantProvider,
	evalServiceAdaptor rpc.IEvaluatorRPCAdapter,
	userProvider rpc.IUserProvider,
) (ITaskService, error) {
	return &TaskServiceImpl{
		TaskRepo:           tRepo,
		tenantProvider:     tenantProvider,
		evalServiceAdaptor: evalServiceAdaptor,
		userProvider:       userProvider,
	}, nil
}

type TaskServiceImpl struct {
	TaskRepo           repo.ITaskRepo
	tenantProvider     tenant.ITenantProvider
	evalServiceAdaptor rpc.IEvaluatorRPCAdapter
	userProvider       rpc.IUserProvider
}

func (t *TaskServiceImpl) CreateTask(ctx context.Context, req *CreateTaskReq) (resp *CreateTaskResp, err error) {
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}
	taskPO := tconv.CreateTaskDTO2PO(ctx, req.Task, userID)
	id, err := t.TaskRepo.CreateTask(ctx, taskPO)
	if err != nil {
		return nil, err
	}
	resp.TaskID = &id
	//todo[xun]:历史回溯数据发mq
	return resp, nil
}
func (t *TaskServiceImpl) UpdateTask(ctx context.Context, req *UpdateTaskReq) (err error) {
	taskPO, err := t.TaskRepo.GetTask(ctx, req.TaskID, &req.WorkspaceID, nil)
	if err != nil {
		return err
	}
	if taskPO == nil {
		logs.CtxError(ctx, "task not found")
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("task not found"))
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return errorx.NewByCode(obErrorx.UserParseFailedCode)
	}
	if req.Description != "" {
		taskPO.Description = &req.Description
	}
	if req.EffectiveTime != nil {
		validEffectiveTime, err := tconv.CheckEffectiveTime(ctx, req.EffectiveTime, taskPO.TaskStatus, taskPO.EffectiveTime)
		if err != nil {
			return err
		}
		taskPO.EffectiveTime = ptr.Of(tconv.ToJSONString(ctx, validEffectiveTime))
	}
	if req.SampleRate != 0 {
		taskPO.Sampler = ptr.Of(tconv.ToJSONString(ctx, req.SampleRate))
	}
	if req.TaskStatus != "" {
		validTaskStatus, err := tconv.CheckTaskStatus(ctx, req.TaskStatus, taskPO.TaskStatus)
		if err != nil {
			return err
		}
		if validTaskStatus != "" {
			if validTaskStatus == task.TaskStatusDisabled {
				//todo[xun]:禁用操作处理
			}
			taskPO.TaskStatus = req.TaskStatus
		}
	}
	taskPO.UpdatedBy = userID
	taskPO.UpdatedAt = time.Now()
	if err = t.TaskRepo.UpdateTask(ctx, taskPO); err != nil {
		return err
	}
	return nil
}
func (t *TaskServiceImpl) ListTasks(ctx context.Context, req *ListTasksReq) (resp *ListTasksResp, err error) {
	taskPOs, _, err := t.TaskRepo.ListTasks(ctx, mysql.ListTaskParam{
		WorkspaceIDs: []int64{req.WorkspaceID},
		TaskFilters:  req.TaskFilters,
		ReqLimit:     req.Limit,
		ReqOffset:    req.Offset,
		OrderBy:      req.OrderBy,
	})
	if len(taskPOs) == 0 {
		logs.CtxInfo(ctx, "GetTasks tasks is nil")
		return resp, nil
	}
	userMap := make(map[string]bool)
	users := make([]string, 0)
	for _, tp := range taskPOs {
		userMap[tp.CreatedBy] = true
		userMap[tp.UpdatedBy] = true
	}
	for u := range userMap {
		users = append(users, u)
	}
	_, userInfoMap, err := t.userProvider.GetUserInfo(ctx, users)
	if err != nil {
		logs.CtxError(ctx, "MGetUserInfo err:%v", err)
	}
	tasks, err := tconv.TaskPOs2DOs(ctx, taskPOs, userInfoMap)
	if err != nil {
		logs.CtxError(ctx, "TaskPOs2DOs err:%v", err)
		return resp, err
	}
	resp.Tasks = tasks
	return resp, nil
}
func (t *TaskServiceImpl) GetTask(ctx context.Context, req *GetTaskReq) (resp *GetTaskResp, err error) {
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return resp, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}
	taskPO, err := t.TaskRepo.GetTask(ctx, req.TaskID, &req.WorkspaceID, &userID)
	if err != nil {
		logs.CtxError(ctx, "GetTasks err:%v", err)
		return resp, err
	}
	if taskPO == nil {
		logs.CtxError(ctx, "GetTasks tasks is nil")
		return resp, nil
	}
	_, userInfoMap, err := t.userProvider.GetUserInfo(ctx, []string{taskPO.CreatedBy, taskPO.UpdatedBy})
	if err != nil {
		logs.CtxError(ctx, "MGetUserInfo err:%v", err)
	}
	resp.Task = tconv.TaskPO2DTO(ctx, taskPO, userInfoMap)
	return resp, nil
}
func (t *TaskServiceImpl) CheckTaskName(ctx context.Context, req *CheckTaskNameReq) (resp *CheckTaskNameResp, err error) {
	taskPOs, _, err := t.TaskRepo.ListTasks(ctx, mysql.ListTaskParam{
		WorkspaceIDs: []int64{req.WorkspaceID},
		TaskFilters: &filter.TaskFilterFields{
			FilterFields: []*filter.TaskFilterField{
				{
					FieldName: gptr.Of(filter.TaskFieldNameTaskName),
					FieldType: gptr.Of(filter.FieldTypeString),
					Values:    []string{req.Name},
					QueryType: gptr.Of(filter.QueryTypeMatch),
				},
			},
		},
		ReqLimit:  10,
		ReqOffset: 0,
	})
	if err != nil {
		logs.CtxError(ctx, "GetTasks err:%v", err)
		return nil, err
	}
	if len(taskPOs) > 0 {
		resp.Pass = gptr.Of(false)
	} else {
		resp.Pass = gptr.Of(true)
	}
	return resp, nil
}
