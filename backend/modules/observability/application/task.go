// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/tracehub"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
)

type ITaskQueueConsumer interface {
	SpanTrigger(ctx context.Context, event *entity.RawSpan) error
	CallBack(ctx context.Context, event *entity.AutoEvalEvent) error
	Correction(ctx context.Context, event *entity.CorrectionEvent) error
	BackFill(ctx context.Context, event *entity.BackFillEvent) error
}

type ITaskApplication interface {
	task.TaskService
	ITaskQueueConsumer
}

func NewTaskApplication(
	taskService service.ITaskService,
	authService rpc.IAuthProvider,
	evalService rpc.IEvaluatorRPCAdapter,
	evaluationService rpc.IEvaluationRPCAdapter,
	userService rpc.IUserProvider,
	tracehubSvc tracehub.ITraceHubService,
	taskProcessor processor.TaskProcessor,
) (ITaskApplication, error) {
	return &TaskApplication{
		taskSvc:       taskService,
		authSvc:       authService,
		evalSvc:       evalService,
		evaluationSvc: evaluationService,
		userSvc:       userService,
		tracehubSvc:   tracehubSvc,
		taskProcessor: taskProcessor,
	}, nil
}

type TaskApplication struct {
	taskSvc       service.ITaskService
	authSvc       rpc.IAuthProvider
	evalSvc       rpc.IEvaluatorRPCAdapter
	evaluationSvc rpc.IEvaluationRPCAdapter
	userSvc       rpc.IUserProvider
	tracehubSvc   tracehub.ITraceHubService
	taskProcessor processor.TaskProcessor
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
		strconv.FormatInt(req.GetWorkspaceID(), 10),
		false); err != nil {
		return nil, err
	}
	sResp, err := t.taskSvc.CheckTaskName(ctx, &service.CheckTaskNameReq{
		WorkspaceID: req.GetWorkspaceID(),
		Name:        req.GetName(),
	})
	if err != nil {
		return resp, err
	}

	return &task.CheckTaskNameResponse{
		Pass: sResp.Pass,
	}, nil
}

func (t *TaskApplication) CreateTask(ctx context.Context, req *task.CreateTaskRequest) (*task.CreateTaskResponse, error) {
	resp := task.NewCreateTaskResponse()
	if err := t.validateCreateTaskReq(ctx, req); err != nil {
		return resp, err
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceTaskCreate,
		strconv.FormatInt(req.GetTask().GetWorkspaceID(), 10),
		false); err != nil {
		return resp, err
	}

	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}

	// 创建task
	taskDO := tconv.TaskDTO2DO(req.GetTask())
	taskDO.TaskStatus = entity.TaskStatusUnstarted
	taskDO.CreatedBy = userID
	taskDO.UpdatedBy = userID
	sResp, err := t.taskSvc.CreateTask(ctx, &service.CreateTaskReq{Task: taskDO})
	if err != nil {
		return resp, err
	}

	return &task.CreateTaskResponse{TaskID: sResp.TaskID}, nil
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
	var taskStatus *entity.TaskStatus
	if req.TaskStatus != nil {
		taskStatus = lo.ToPtr(entity.TaskStatus(req.GetTaskStatus()))
	}
	err := t.taskSvc.UpdateTask(ctx, &service.UpdateTaskReq{
		TaskID:        req.GetTaskID(),
		WorkspaceID:   req.GetWorkspaceID(),
		TaskStatus:    taskStatus,
		Description:   req.Description,
		EffectiveTime: tconv.EffectiveTimeDTO2DO(req.EffectiveTime),
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
		strconv.FormatInt(req.GetWorkspaceID(), 10),
		false); err != nil {
		return resp, err
	}

	sResp, err := t.taskSvc.ListTasks(ctx, &service.ListTasksReq{
		WorkspaceID: req.GetWorkspaceID(),
		TaskFilters: tconv.TaskFiltersDTO2DO(req.GetTaskFilters()),
		Limit:       req.GetLimit(),
		Offset:      req.GetOffset(),
		OrderBy:     convertor.OrderByDTO2DO(req.GetOrderBy()),
	})
	if err != nil {
		return resp, err
	}
	if sResp == nil {
		return resp, nil
	}

	userMap := make(map[string]bool)
	for _, tp := range sResp.Tasks {
		userMap[tp.CreatedBy] = true
		userMap[tp.UpdatedBy] = true
	}
	_, userInfoMap, err := t.userSvc.GetUserInfo(ctx, lo.Keys(userMap))
	if err != nil {
		logs.CtxError(ctx, "MGetUserInfo err:%v", err)
	}
	tasks := tconv.TaskDOs2DTOs(ctx, sResp.Tasks, userInfoMap)

	return &task.ListTasksResponse{
		Tasks: tasks,
		Total: &sResp.Total,
	}, nil
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
		strconv.FormatInt(req.GetWorkspaceID(), 10),
		false); err != nil {
		return resp, err
	}

	sResp, err := t.taskSvc.GetTask(ctx, &service.GetTaskReq{
		TaskID:      req.GetTaskID(),
		WorkspaceID: req.GetWorkspaceID(),
	})
	if err != nil {
		return resp, err
	}
	if sResp == nil {
		return resp, nil
	}

	taskDO := sResp.Task
	_, userInfoMap, err := t.userSvc.GetUserInfo(ctx, []string{taskDO.CreatedBy, taskDO.UpdatedBy})
	if err != nil {
		logs.CtxError(ctx, "MGetUserInfo err:%v", err)
	}

	return &task.GetTaskResponse{
		Task: tconv.TaskDO2DTO(ctx, taskDO, userInfoMap),
	}, nil
}

func (t *TaskApplication) SpanTrigger(ctx context.Context, event *entity.RawSpan) error {
	return t.tracehubSvc.SpanTrigger(ctx, event)
}

func (t *TaskApplication) CallBack(ctx context.Context, event *entity.AutoEvalEvent) error {
	return t.tracehubSvc.CallBack(ctx, event)
}

func (t *TaskApplication) Correction(ctx context.Context, event *entity.CorrectionEvent) error {
	return t.tracehubSvc.Correction(ctx, event)
}

func (t *TaskApplication) BackFill(ctx context.Context, event *entity.BackFillEvent) error {
	return t.tracehubSvc.BackFill(ctx, event)
}
