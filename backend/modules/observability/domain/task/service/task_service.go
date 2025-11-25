// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/storage"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	traceservice "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type CreateTaskReq struct {
	Task *entity.ObservabilityTask
}
type CreateTaskResp struct {
	TaskID *int64
}
type UpdateTaskReq struct {
	TaskID        int64
	WorkspaceID   int64
	TaskStatus    *entity.TaskStatus
	Description   *string
	EffectiveTime *entity.EffectiveTime
	SampleRate    *float64
}
type ListTasksReq struct {
	WorkspaceID int64
	TaskFilters *entity.TaskFilterFields
	Limit       int32
	Offset      int32
	OrderBy     *common.OrderBy
}
type ListTasksResp struct {
	Tasks []*entity.ObservabilityTask
	Total int64
}
type GetTaskReq struct {
	TaskID      int64
	WorkspaceID int64
}
type GetTaskResp struct {
	Task *entity.ObservabilityTask
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

	SendBackfillMessage(ctx context.Context, event *entity.BackFillEvent) error
}

func NewTaskServiceImpl(
	tRepo repo.ITaskRepo,
	idGenerator idgen.IIDGenerator,
	backfillProducer mq.IBackfillProducer,
	taskProcessor *processor.TaskProcessor,
	storageProvider storage.IStorageProvider,
	tenantProvider tenant.ITenantProvider,
	buildHelper traceservice.TraceFilterProcessorBuilder,
) (ITaskService, error) {
	return &TaskServiceImpl{
		TaskRepo:         tRepo,
		idGenerator:      idGenerator,
		backfillProducer: backfillProducer,
		taskProcessor:    *taskProcessor,
		storageProvider:  storageProvider,
		tenantProvider:   tenantProvider,
		buildHelper:      buildHelper,
	}, nil
}

type TaskServiceImpl struct {
	TaskRepo         repo.ITaskRepo
	idGenerator      idgen.IIDGenerator
	backfillProducer mq.IBackfillProducer
	taskProcessor    processor.TaskProcessor
	storageProvider  storage.IStorageProvider
	tenantProvider   tenant.ITenantProvider
	buildHelper      traceservice.TraceFilterProcessorBuilder
}

func (t *TaskServiceImpl) CreateTask(ctx context.Context, req *CreateTaskReq) (resp *CreateTaskResp, err error) {
	taskDO := req.Task
	// 校验task name是否存在
	checkResp, err := t.CheckTaskName(ctx, &CheckTaskNameReq{
		WorkspaceID: taskDO.WorkspaceID,
		Name:        taskDO.Name,
	})
	if err != nil {
		logs.CtxError(ctx, "CheckTaskName err:%v", err)
		return nil, err
	}
	if !*checkResp.Pass {
		logs.CtxError(ctx, "task name exist")
		return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("task name exist"))
	}

	if err := t.buildSpanFilters(ctx, taskDO); err != nil {
		logs.CtxError(ctx, "buildSpanFilters err:%v", err)
		return nil, err
	}

	proc := t.taskProcessor.GetTaskProcessor(taskDO.TaskType)
	// 校验配置项是否有效
	if err = proc.ValidateConfig(ctx, taskDO); err != nil {
		logs.CtxError(ctx, "ValidateConfig err:%v", err)
		return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg(fmt.Sprintf("config invalid:%v", err)))
	}
	id, err := t.TaskRepo.CreateTask(ctx, taskDO)
	if err != nil {
		return nil, err
	}
	// storage准备
	tenants, err := t.tenantProvider.GetTenantsByPlatformType(ctx, loop_span.PlatformType(req.Task.SpanFilter.PlatformType))
	if err != nil {
		return nil, err
	}
	if err = t.storageProvider.PrepareStorageForTask(ctx, strconv.FormatInt(req.Task.WorkspaceID, 10), tenants); err != nil {
		logs.CtxError(ctx, "PrepareStorageForTask err:%v", err)
		return nil, err
	}
	// 创建任务的数据准备
	// 数据回流任务——创建/更新输出数据集
	// 自动评测历史回溯——创建空壳子
	taskDO.ID = id
	if err = proc.OnTaskCreated(ctx, taskDO); err != nil {
		logs.CtxError(ctx, "create initial task run failed, task_id=%d, err=%v", id, err)

		if err1 := t.TaskRepo.DeleteTask(ctx, taskDO); err1 != nil {
			logs.CtxError(ctx, "delete task failed, task_id=%d, err=%v", id, err1)
		}
		return nil, err
	}

	// 历史回溯数据发MQ
	if taskDO.ShouldTriggerBackfill() {
		backfillEvent := &entity.BackFillEvent{
			SpaceID: taskDO.WorkspaceID,
			TaskID:  id,
		}

		if err := t.SendBackfillMessage(context.Background(), backfillEvent); err != nil {
			// 失败了会有定时任务进行补偿
			logs.CtxWarn(ctx, "send backfill message failed, task_id=%d, err=%v", id, err)
		}
	}

	return &CreateTaskResp{TaskID: &id}, nil
}

func (t *TaskServiceImpl) UpdateTask(ctx context.Context, req *UpdateTaskReq) (err error) {
	taskDO, err := t.TaskRepo.GetTask(ctx, req.TaskID, &req.WorkspaceID, nil)
	if err != nil {
		return err
	}
	if taskDO == nil {
		logs.CtxError(ctx, "task [%d] not found", req.TaskID)
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("task not found"))
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return errorx.NewByCode(obErrorx.UserParseFailedCode)
	}
	// 校验更新参数是否合法
	if req.Description != nil {
		taskDO.Description = req.Description
	}
	if req.EffectiveTime != nil {
		if err := taskDO.SetEffectiveTime(ctx, *req.EffectiveTime); err != nil {
			return err
		}
	}
	if req.SampleRate != nil {
		taskDO.Sampler.SampleRate = *req.SampleRate
	}
	if req.TaskStatus != nil {
		event, err := taskDO.SetTaskStatus(ctx, *req.TaskStatus)
		if err != nil {
			return err
		}

		if event != nil {
			if event.After == entity.TaskStatusDisabled {
				// 禁用操作处理
				proc := t.taskProcessor.GetTaskProcessor(taskDO.TaskType)
				var taskRun *entity.TaskRun
				for _, tr := range taskDO.TaskRuns {
					if tr.RunStatus == entity.TaskRunStatusRunning {
						taskRun = tr
						break
					}
				}
				if err = proc.OnTaskRunFinished(ctx, taskexe.OnTaskRunFinishedReq{
					Task:    taskDO,
					TaskRun: taskRun,
				}); err != nil {
					logs.CtxError(ctx, "proc Finish err:%v", err)
					return err
				}
				err = t.TaskRepo.RemoveNonFinalTask(ctx, strconv.FormatInt(taskDO.WorkspaceID, 10), taskDO.ID)
				if err != nil {
					logs.CtxError(ctx, "remove non final task failed, task_id=%d, err=%v", taskDO.ID, err)
				}
			}
		}
	}
	taskDO.UpdatedBy = userID
	taskDO.UpdatedAt = time.Now()
	if err = t.TaskRepo.UpdateTask(ctx, taskDO); err != nil {
		return err
	}
	return nil
}

func (t *TaskServiceImpl) ListTasks(ctx context.Context, req *ListTasksReq) (resp *ListTasksResp, err error) {
	taskDOs, total, err := t.TaskRepo.ListTasks(ctx, repo.ListTaskParam{
		WorkspaceIDs: []int64{req.WorkspaceID},
		TaskFilters:  req.TaskFilters,
		ReqLimit:     req.Limit,
		ReqOffset:    req.Offset,
		OrderBy:      req.OrderBy,
	})
	if err != nil {
		logs.CtxError(ctx, "ListTasks err:%v", err)
		return resp, err
	}
	if len(taskDOs) == 0 {
		logs.CtxInfo(ctx, "GetTasks tasks is nil")
		return resp, nil
	}

	taskDOs = filterHiddenFilters(taskDOs)

	return &ListTasksResp{
		Tasks: taskDOs,
		Total: total,
	}, nil
}

func (t *TaskServiceImpl) GetTask(ctx context.Context, req *GetTaskReq) (resp *GetTaskResp, err error) {
	taskDO, err := t.TaskRepo.GetTask(ctx, req.TaskID, &req.WorkspaceID, nil)
	if err != nil {
		logs.CtxError(ctx, "GetTasks err:%v", err)
		return resp, err
	}
	if taskDO == nil {
		logs.CtxError(ctx, "GetTasks tasks is nil")
		return resp, nil
	}

	taskDO = filterHiddenFilters([]*entity.ObservabilityTask{taskDO})[0]

	return &GetTaskResp{Task: taskDO}, nil
}

func filterHiddenFilters(tasks []*entity.ObservabilityTask) []*entity.ObservabilityTask {
	for _, t := range tasks {
		if t == nil || t.SpanFilter == nil {
			continue
		}

		filtered := filterVisibleFilterFields(&t.SpanFilter.Filters)
		if filtered != nil {
			t.SpanFilter.Filters = *filtered
		}
	}
	return tasks
}

func filterVisibleFilterFields(fields *loop_span.FilterFields) *loop_span.FilterFields {
	if fields == nil {
		return nil
	}

	filters := fields.FilterFields
	if len(filters) == 0 {
		return fields
	}

	writeIdx := 0
	for _, f := range filters {
		if f == nil || f.Hidden {
			continue
		}
		if f.SubFilter != nil {
			filteredSub := filterVisibleFilterFields(f.SubFilter)
			if filteredSub == nil || len(filteredSub.FilterFields) == 0 {
				f.SubFilter = nil
			} else {
				f.SubFilter = filteredSub
			}
		}
		filters[writeIdx] = f
		writeIdx++
	}

	if writeIdx == len(filters) {
		return fields
	}

	for i := writeIdx; i < len(filters); i++ {
		filters[i] = nil
	}

	fields.FilterFields = filters[:writeIdx]
	return fields
}

func (t *TaskServiceImpl) CheckTaskName(ctx context.Context, req *CheckTaskNameReq) (resp *CheckTaskNameResp, err error) {
	taskPOs, _, err := t.TaskRepo.ListTasks(ctx, repo.ListTaskParam{
		WorkspaceIDs: []int64{req.WorkspaceID},
		TaskFilters: &entity.TaskFilterFields{
			FilterFields: []*entity.TaskFilterField{
				{
					FieldName: gptr.Of(entity.TaskFieldNameTaskName),
					FieldType: gptr.Of(entity.FieldTypeString),
					Values:    []string{req.Name},
					QueryType: gptr.Of(entity.QueryTypeMatch),
				},
			},
		},
		ReqLimit:  10,
		ReqOffset: 0,
	})
	if err != nil {
		logs.CtxError(ctx, "ListTasks err:%v", err)
		return nil, err
	}
	var pass bool
	if len(taskPOs) > 0 {
		pass = false
	} else {
		pass = true
	}
	return &CheckTaskNameResp{Pass: gptr.Of(pass)}, nil
}

// SendBackfillMessage 发送MQ消息
func (t *TaskServiceImpl) SendBackfillMessage(ctx context.Context, event *entity.BackFillEvent) error {
	if t.backfillProducer == nil {
		return errorx.NewByCode(obErrorx.CommonInternalErrorCode, errorx.WithExtraMsg("backfill producer not initialized"))
	}

	return t.backfillProducer.SendBackfill(ctx, event)
}

func (t *TaskServiceImpl) buildSpanFilters(ctx context.Context, taskDO *entity.ObservabilityTask) error {
	f, err := t.buildHelper.BuildPlatformRelatedFilter(ctx, taskDO.SpanFilter.PlatformType)
	if err != nil {
		return err
	}
	env := &span_filter.SpanEnv{
		WorkspaceID: taskDO.WorkspaceID,
	}

	// coze场景中，需要将basic filter提前固化到数据库中，避免任务触发时重复调用coze接口
	basicFilter, forceQuery, err := f.BuildBasicSpanFilter(ctx, env)
	if err != nil {
		return err
	} else if len(basicFilter) == 0 && !forceQuery {
		logs.CtxInfo(ctx, "Build basic filter failed, platform type: [%s], workspaceID: [%d]",
			taskDO.SpanFilter.PlatformType, taskDO.WorkspaceID)
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("User has no permission"))
	}

	// basic filter对用户不可见
	for _, filter := range basicFilter {
		filter.SetHidden(true)
	}

	taskDO.SpanFilter.Filters.FilterFields = append(taskDO.SpanFilter.Filters.FilterFields, basicFilter...)
	return nil
}
