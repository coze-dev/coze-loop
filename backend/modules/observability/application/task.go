// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	domain_task "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service"
	task_processor "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/tracehub"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	trace_Svc "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
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
	taskProcessor task_processor.TaskProcessor,
	buildHelper trace_Svc.TraceFilterProcessorBuilder,
) (ITaskApplication, error) {
	return &TaskApplication{
		taskSvc:       taskService,
		authSvc:       authService,
		evalSvc:       evalService,
		evaluationSvc: evaluationService,
		userSvc:       userService,
		tracehubSvc:   tracehubSvc,
		taskProcessor: taskProcessor,
		buildHelper:   buildHelper,
	}, nil
}

type TaskApplication struct {
	taskSvc       service.ITaskService
	authSvc       rpc.IAuthProvider
	evalSvc       rpc.IEvaluatorRPCAdapter
	evaluationSvc rpc.IEvaluationRPCAdapter
	userSvc       rpc.IUserProvider
	tracehubSvc   tracehub.ITraceHubService
	taskProcessor task_processor.TaskProcessor
	buildHelper   trace_Svc.TraceFilterProcessorBuilder
}

func (t *TaskApplication) CheckTaskName(ctx context.Context, req *task.CheckTaskNameRequest) (*task.CheckTaskNameResponse, error) {
	resp := task.NewCheckTaskNameResponse()
	if req == nil {
		return resp, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return resp, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	appID := session.AppIDInCtxOrEmpty(ctx)
	var action string
	if appID == 717152 {
		action = rpc.AuthActionTraceTaskList
	} else {
		action = rpc.AuthActionTaskReadable
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		action,
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
	appID := session.AppIDInCtxOrEmpty(ctx)
	var action string
	if appID == 717152 {
		action = rpc.AuthActionTraceTaskCreate
	} else {
		action = rpc.AuthActionTaskWritable
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		action,
		strconv.FormatInt(req.GetTask().GetWorkspaceID(), 10),
		false); err != nil {
		return resp, err
	}

	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}
	// 创建task
	req.Task.TaskStatus = ptr.Of(domain_task.TaskStatusUnstarted)
	spanFilers, err := t.buildSpanFilters(ctx, req.Task.GetRule().GetSpanFilters(), req.GetTask().GetWorkspaceID())
	if err != nil {
		return nil, err
	}
	sResp, err := t.taskSvc.CreateTask(ctx, &service.CreateTaskReq{Task: tconv.TaskDTO2DO(req.GetTask(), userID, spanFilers)})
	if err != nil {
		return resp, err
	}

	return &task.CreateTaskResponse{TaskID: sResp.TaskID}, nil
}

func (t *TaskApplication) buildSpanFilters(ctx context.Context, spanFilterFields *filter.SpanFilterFields, workspaceID int64) (*filter.SpanFilterFields, error) {
	filtersDO := make([]*filter.FilterField, 0, len(spanFilterFields.GetFilters().GetFilterFields()))
	if spanFilterFields.GetFilters() != nil {
		for _, v := range spanFilterFields.GetFilters().GetFilterFields() {
			filtersDO = append(filtersDO, &filter.FilterField{
				FieldName:  v.FieldName,
				FieldType:  v.FieldType,
				Values:     v.Values,
				QueryType:  v.QueryType,
				QueryAndOr: v.QueryAndOr,
				SubFilter:  v.SubFilter,
				Hidden:     v.Hidden,
			})
		}
	}
	filters := &filter.FilterFields{
		QueryAndOr:   ptr.Of(filter.QueryRelationAnd),
		FilterFields: filtersDO,
	}
	switch spanFilterFields.GetPlatformType() {
	case common.PlatformTypeCozeBot, common.PlatformTypeProject, common.PlatformTypeWorkflow, common.PlatformTypeInnerCozeBot:
		platformFilter, err := t.buildHelper.BuildPlatformRelatedFilter(ctx, loop_span.PlatformType(spanFilterFields.GetPlatformType()))
		if err != nil {
			return nil, err
		}
		env := &span_filter.SpanEnv{
			WorkspaceID: workspaceID,
		}
		basicFilter, forceQuery, err := platformFilter.BuildBasicSpanFilter(ctx, env)
		if err != nil {
			return nil, err
		} else if len(basicFilter) == 0 && !forceQuery { // if it's null, no need to query from ck
			return nil, nil
		}
		basicFilterFields := &loop_span.FilterFields{
			QueryAndOr:   ptr.Of(loop_span.QueryAndOrEnumAnd),
			FilterFields: basicFilter,
		}
		filters.FilterFields = append(filters.FilterFields, convertor.FilterFieldsDO2DTO(basicFilterFields, true)...)
		return &filter.SpanFilterFields{
			Filters:      filters,
			PlatformType: spanFilterFields.PlatformType,
			SpanListType: spanFilterFields.SpanListType,
		}, nil
	default:
		return spanFilterFields, nil
	}
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
	appID := session.AppIDInCtxOrEmpty(ctx)
	var action string
	if appID == 717152 {
		action = rpc.AuthActionTraceTaskEdit
	} else {
		action = rpc.AuthActionTaskWritable
	}
	if err := t.authSvc.CheckTaskPermission(ctx,
		action,
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
	appID := session.AppIDInCtxOrEmpty(ctx)
	var action string
	if appID == 717152 {
		action = rpc.AuthActionTraceTaskList
	} else {
		action = rpc.AuthActionTaskReadable
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		action,
		strconv.FormatInt(req.GetWorkspaceID(), 10),
		false); err != nil {
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
	if sResp == nil {
		return resp, nil
	}
	return &task.ListTasksResponse{
		Tasks: sResp.Tasks,
		Total: sResp.Total,
	}, nil
}

func (t *TaskApplication) GetTask(ctx context.Context, req *task.GetTaskRequest) (*task.GetTaskResponse, error) {
	resp := task.NewGetTaskResponse()
	if req == nil {
		return resp, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return resp, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	appID := session.AppIDInCtxOrEmpty(ctx)
	var action string
	if appID == 717152 {
		action = rpc.AuthActionTraceTaskList
	} else {
		action = rpc.AuthActionTaskReadable
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		action,
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

	return &task.GetTaskResponse{
		Task: sResp.Task,
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
