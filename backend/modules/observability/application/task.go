// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ITaskApplication interface {
	task.TaskService
}

func NewTaskApplication(
	traceService service.ITraceService,
	viewRepo repo.IViewRepo,
	benefitService benefit.IBenefitService,
	traceMetrics metrics.ITraceMetrics,
	traceConfig config.ITraceConfig,
	authService rpc.IAuthProvider,
	evalService rpc.IEvaluatorRPCAdapter,
	userService rpc.IUserProvider,
	tagService rpc.ITagRPCAdapter,
) (ITraceApplication, error) {
	return &TraceApplication{
		traceService: traceService,
		viewRepo:     viewRepo,
		traceConfig:  traceConfig,
		metrics:      traceMetrics,
		benefit:      benefitService,
		authSvc:      authService,
		evalSvc:      evalService,
		userSvc:      userService,
		tagSvc:       tagService,
	}, nil
}

type TaskApplication struct {
	traceService service.ITraceService
	viewRepo     repo.IViewRepo
	traceConfig  config.ITraceConfig
	metrics      metrics.ITraceMetrics
	benefit      benefit.IBenefitService
	authSvc      rpc.IAuthProvider
	evalSvc      rpc.IEvaluatorRPCAdapter
	userSvc      rpc.IUserProvider
	tagSvc       rpc.ITagRPCAdapter
}

func (t *TaskApplication) CheckTaskName(ctx context.Context, req *task.CheckTaskNameRequest) (*task.CheckTaskNameResponse, error) {
	if req == nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceTaskList,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}

	return &task.CheckTaskNameResponse{
		BaseResp: nil,
	}, nil
}
func (t *TaskApplication) CreateTask(ctx context.Context, req *task.CreateTaskRequest) (*task.CreateTaskResponse, error) {
	if err := t.validateCreateTaskReq(ctx, req); err != nil {
		return nil, err
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceTaskCreate,
		strconv.FormatInt(req.GetTask().GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}
	//taskPO := tconv.CreateTaskDTO2PO(req, userID)
	//
	//id, err := t.taskRepo.CreateTask(ctx, taskPO)
	//if err != nil {
	//	return nil, err
	//}

	return nil, nil
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
	if req == nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	if err := t.authSvc.CheckViewPermission(ctx,
		rpc.AuthActionTraceTaskEdit,
		strconv.FormatInt(req.GetWorkspaceID(), 10),
		strconv.FormatInt(req.GetTaskID(), 10)); err != nil {
		return nil, err
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}

	return task.NewUpdateTaskResponse(), nil
}
func (t *TaskApplication) ListTasks(ctx context.Context, req *task.ListTasksRequest) (*task.ListTasksResponse, error) {
	if req == nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceTaskList,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}

	return &task.ListTasksResponse{
		BaseResp: nil,
	}, nil
}
func (t *TaskApplication) GetTask(ctx context.Context, req *task.GetTaskRequest) (*task.GetTaskResponse, error) {
	if req == nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no request provided"))
	} else if req.GetWorkspaceID() <= 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid workspace_id"))
	}
	if err := t.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceTaskList,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}

	return &task.GetTaskResponse{
		BaseResp: nil,
	}, nil
}
