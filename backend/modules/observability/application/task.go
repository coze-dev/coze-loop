// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ITaskQueueConsumer interface {
	TraceHub(ctx context.Context, event *entity.TaskEvent) error
}
type ITaskApplication interface {
	task.TaskService
}

func NewTaskApplication(
	taskService service.ITaskService,
	authService rpc.IAuthProvider,
	evalService rpc.IEvaluatorRPCAdapter,
	userService rpc.IUserProvider,
) (ITaskApplication, error) {
	return &TaskApplication{
		taskSvc: taskService,
		authSvc: authService,
		evalSvc: evalService,
		userSvc: userService,
	}, nil
}

type TaskApplication struct {
	taskSvc service.ITaskService
	authSvc rpc.IAuthProvider
	evalSvc rpc.IEvaluatorRPCAdapter
	userSvc rpc.IUserProvider
}

func (t *TaskApplication) CheckTaskName(ctx context.Context, req *task.CheckTaskNameRequest) (*task.CheckTaskNameResponse, error) {
	resp := task.NewCheckTaskNameResponse()
	if req == nil {
		return resp, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return resp, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceTaskList,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	sResp, err := t.taskSvc.CheckTaskName(ctx, &service.CheckTaskNameReq{
		WorkspaceID: req.GetWorkspaceID(),
		Name:        req.GetName(),
	})
	if err != nil {
		return resp, err
	}
	resp.Pass = sResp.Pass

	return resp, nil
}
func (t *TaskApplication) CreateTask(ctx context.Context, req *task.CreateTaskRequest) (*task.CreateTaskResponse, error) {
	resp := task.NewCreateTaskResponse()
	if err := t.validateCreateTaskReq(ctx, req); err != nil {
		return resp, err
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceTaskCreate,
		strconv.FormatInt(req.GetTask().GetWorkspaceID(), 10)); err != nil {
		return resp, err
	}
	sResp, err := t.taskSvc.CreateTask(ctx, &service.CreateTaskReq{Task: req.GetTask()})
	if err != nil {
		return resp, err
	}
	resp.TaskID = sResp.TaskID

	return resp, nil
}

func (t *TaskApplication) validateCreateTaskReq(ctx context.Context, req *task.CreateTaskRequest) error {
	// 参数验证
	if req == nil || req.GetTask() == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetTask().GetWorkspaceID() <= 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	} else if req.GetTask().GetName() == "" {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid task_name"))
	}
	if req.GetTask().GetRule() != nil && req.GetTask().GetRule().GetEffectiveTime() != nil {
		startAt := req.GetTask().GetRule().GetEffectiveTime().GetStartAt()
		endAt := req.GetTask().GetRule().GetEffectiveTime().GetEndAt()
		if startAt <= time.Now().Add(-10*time.Minute).UnixMilli() {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("The start time must be no earlier than 10 minutes ago."))
		}
		if startAt >= endAt {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("The start time must be earlier than the end time."))
		}
	}
	var evaluatorVersionIDs []int64
	for _, autoEvaluateConfig := range req.GetTask().GetTaskConfig().GetAutoEvaluateConfigs() {
		evaluatorVersionIDs = append(evaluatorVersionIDs, autoEvaluateConfig.GetEvaluatorVersionID())
	}
	if len(evaluatorVersionIDs) == 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("Invalid parameter. Please check the parameter and try again."))
	}
	// 检查评估器版本是否合法
	evaluators, _, err := t.evalSvc.BatchGetEvaluatorVersions(ctx, &rpc.BatchGetEvaluatorVersionsParam{
		WorkspaceID:         req.GetTask().GetWorkspaceID(),
		EvaluatorVersionIds: evaluatorVersionIDs,
	})
	if err != nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithMsgParam("evaluatorVersionIDs is invalid, BatchGetEvaluators err: %v", err.Error()))
	}
	if len(evaluators) != len(evaluatorVersionIDs) {
		logs.CtxError(ctx, "evaluators len: %d, evaluatorVersionIDs len: %d", len(evaluators), len(evaluatorVersionIDs))
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("evaluatorVersionIDs is invalid, len(evaluators) != len(evaluatorVersionIDs)"))
	}
	return nil
}
func (t *TaskApplication) UpdateTask(ctx context.Context, req *task.UpdateTaskRequest) (*task.UpdateTaskResponse, error) {
	resp := task.NewUpdateTaskResponse()
	if req == nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	if err := t.authSvc.CheckTaskPermission(ctx,
		rpc.AuthActionTraceTaskEdit,
		strconv.FormatInt(req.GetWorkspaceID(), 10),
		strconv.FormatInt(req.GetTaskID(), 10)); err != nil {
		return nil, err
	}
	err := t.taskSvc.UpdateTask(ctx, &service.UpdateTaskReq{
		TaskID:        req.GetTaskID(),
		WorkspaceID:   req.GetWorkspaceID(),
		TaskStatus:    req.TaskStatus,
		Description:   req.Description,
		EffectiveTime: req.EffectiveTime,
		SampleRate:    req.SampleRate,
	})
	if err != nil {
		return resp, err
	}

	return resp, nil
}
func (t *TaskApplication) ListTasks(ctx context.Context, req *task.ListTasksRequest) (*task.ListTasksResponse, error) {
	resp := task.NewListTasksResponse()
	if req == nil {
		return resp, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return resp, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceTaskList,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return resp, err
	}
	sResp, err := t.taskSvc.ListTasks(ctx, &service.ListTasksReq{
		WorkspaceID: req.GetWorkspaceID(),
		TaskFilters: req.GetTaskFilters(),
		Limit:       req.GetLimit(),
		Offset:      req.GetOffset(),
		OrderBy:     req.GetOrderBy(),
	})
	if err != nil {
		return resp, err
	}
	resp.Tasks = sResp.Tasks
	resp.Total = sResp.Total
	return resp, nil
}
func (t *TaskApplication) GetTask(ctx context.Context, req *task.GetTaskRequest) (*task.GetTaskResponse, error) {
	resp := task.NewGetTaskResponse()
	if req == nil {
		return resp, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return resp, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceTaskList,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return resp, err
	}
	sResp, err := t.taskSvc.GetTask(ctx, &service.GetTaskReq{
		TaskID:      req.GetTaskID(),
		WorkspaceID: req.GetWorkspaceID(),
	})
	if err != nil {
		return resp, err
	}
	resp.Task = sResp.Task

	return resp, nil
}
