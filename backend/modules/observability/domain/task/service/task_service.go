// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
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
	TaskStatus    *task.TaskStatus
	Description   *string
	EffectiveTime *task.EffectiveTime
	SampleRate    *float64
}
type ListTasksReq struct {
	WorkspaceID int64
	TaskFilters *filter.TaskFilterFields
	Limit       int32
	Offset      int32
	OrderBy     *common.OrderBy
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
	userProvider rpc.IUserProvider,
	idGenerator idgen.IIDGenerator,
	backfillProducer mq.IBackfillProducer,
	taskProcessor *processor.TaskProcessor,
) (ITaskService, error) {
	return &TaskServiceImpl{
		TaskRepo:         tRepo,
		userProvider:     userProvider,
		idGenerator:      idGenerator,
		backfillProducer: backfillProducer,
		taskProcessor:    *taskProcessor,
	}, nil
}

type TaskServiceImpl struct {
	TaskRepo         repo.ITaskRepo
	userProvider     rpc.IUserProvider
	idGenerator      idgen.IIDGenerator
	backfillProducer mq.IBackfillProducer
	taskProcessor    processor.TaskProcessor
}

func (t *TaskServiceImpl) CreateTask(ctx context.Context, req *CreateTaskReq) (resp *CreateTaskResp, err error) {
	// 校验task name是否存在
	checkResp, err := t.CheckTaskName(ctx, &CheckTaskNameReq{
		WorkspaceID: req.Task.WorkspaceID,
		Name:        req.Task.Name,
	})
	if err != nil {
		logs.CtxError(ctx, "CheckTaskName err:%v", err)
		return nil, err
	}
	if !*checkResp.Pass {
		logs.CtxError(ctx, "task name exist")
		return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("task name exist"))
	}
	proc := t.taskProcessor.GetTaskProcessor(req.Task.TaskType)
	// 校验配置项是否有效
	if err = proc.ValidateConfig(ctx, req.Task); err != nil {
		logs.CtxError(ctx, "ValidateConfig err:%v", err)
		return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg(fmt.Sprintf("config invalid:%v", err)))
	}
	id, err := t.TaskRepo.CreateTask(ctx, req.Task)
	if err != nil {
		return nil, err
	}
	// 创建任务的数据准备
	// 数据回流任务——创建/更新输出数据集
	// 自动评测历史回溯——创建空壳子
	req.Task.ID = id
	if err = proc.OnCreateTaskChange(ctx, req.Task); err != nil {
		logs.CtxError(ctx, "create initial task run failed, task_id=%d, err=%v", id, err)

		if err1 := t.TaskRepo.DeleteTask(ctx, req.Task); err1 != nil {
			logs.CtxError(ctx, "delete task failed, task_id=%d, err=%v", id, err1)
		}
		return nil, err
	}

	// 历史回溯数据发MQ
	if t.shouldTriggerBackfill(req.Task) {
		backfillEvent := &entity.BackFillEvent{
			SpaceID: req.Task.WorkspaceID,
			TaskID:  id,
		}

		// 异步发送MQ消息，不阻塞任务创建流程
		go func() {
			if err := t.sendBackfillMessage(context.Background(), backfillEvent); err != nil {
				logs.CtxWarn(ctx, "send backfill message failed, task_id=%d, err=%v", id, err)
			}
		}()
	}

	return &CreateTaskResp{TaskID: &id}, nil
}

func (t *TaskServiceImpl) UpdateTask(ctx context.Context, req *UpdateTaskReq) (err error) {
	taskDO, err := t.TaskRepo.GetTask(ctx, req.TaskID, &req.WorkspaceID, nil)
	if err != nil {
		return err
	}
	if taskDO == nil {
		logs.CtxError(ctx, "task not found")
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
		validEffectiveTime, err := tconv.CheckEffectiveTime(ctx, req.EffectiveTime, taskDO.TaskStatus, taskDO.EffectiveTime)
		if err != nil {
			return err
		}
		taskDO.EffectiveTime = validEffectiveTime
	}
	if req.SampleRate != nil {
		taskDO.Sampler.SampleRate = *req.SampleRate
	}
	if req.TaskStatus != nil {
		validTaskStatus, err := tconv.CheckTaskStatus(ctx, *req.TaskStatus, taskDO.TaskStatus)
		if err != nil {
			return err
		}
		if validTaskStatus != "" {
			if validTaskStatus == task.TaskStatusDisabled {
				// 禁用操作处理
				proc := t.taskProcessor.GetTaskProcessor(taskDO.TaskType)
				var taskRun *entity.TaskRun
				for _, tr := range taskDO.TaskRuns {
					if tr.RunStatus == task.RunStatusRunning {
						taskRun = tr
						break
					}
				}
				if err = proc.OnFinishTaskRunChange(ctx, taskexe.OnFinishTaskRunChangeReq{
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
			taskDO.TaskStatus = *req.TaskStatus
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
	taskDOs, total, err := t.TaskRepo.ListTasks(ctx, mysql.ListTaskParam{
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
	userMap := make(map[string]bool)
	users := make([]string, 0)
	for _, tp := range taskDOs {
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
	return &ListTasksResp{
		Tasks: tconv.TaskDOs2DTOs(ctx, filterHiddenFilters(taskDOs), userInfoMap),
		Total: ptr.Of(total),
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
	_, userInfoMap, err := t.userProvider.GetUserInfo(ctx, []string{taskDO.CreatedBy, taskDO.UpdatedBy})
	if err != nil {
		logs.CtxError(ctx, "MGetUserInfo err:%v", err)
	}
	return &GetTaskResp{Task: tconv.TaskDO2DTO(ctx, filterHiddenFilters([]*entity.ObservabilityTask{taskDO})[0], userInfoMap)}, nil
}

func filterHiddenFilters(tasks []*entity.ObservabilityTask) []*entity.ObservabilityTask {
	for _, t := range tasks {
		if t == nil || t.SpanFilter == nil {
			continue
		}

		filters := t.SpanFilter.Filters.FilterFields
		if len(filters) == 0 {
			continue
		}

		writeIdx := 0
		for _, filter := range filters {
			if filter == nil || filter.Hidden {
				continue
			}
			filters[writeIdx] = filter
			writeIdx++
		}

		if writeIdx == len(filters) {
			continue
		}

		for i := writeIdx; i < len(filters); i++ {
			filters[i] = nil
		}

		t.SpanFilter.Filters.FilterFields = filters[:writeIdx]
	}
	return tasks
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

// shouldTriggerBackfill 判断是否需要发送历史回溯MQ
func (t *TaskServiceImpl) shouldTriggerBackfill(taskDO *entity.ObservabilityTask) bool {
	// 检查任务类型
	taskType := taskDO.TaskType
	if taskType != task.TaskTypeAutoEval && taskType != task.TaskTypeAutoDataReflow {
		return false
	}

	// 检查回填时间配置

	if taskDO.BackfillEffectiveTime == nil {
		return false
	}

	return taskDO.BackfillEffectiveTime.StartAt > 0 &&
		taskDO.BackfillEffectiveTime.EndAt > 0 &&
		taskDO.BackfillEffectiveTime.StartAt < taskDO.BackfillEffectiveTime.EndAt
}

// sendBackfillMessage 发送MQ消息
func (t *TaskServiceImpl) sendBackfillMessage(ctx context.Context, event *entity.BackFillEvent) error {
	if t.backfillProducer == nil {
		return errorx.NewByCode(obErrorx.CommonInternalErrorCode, errorx.WithExtraMsg("backfill producer not initialized"))
	}

	return t.backfillProducer.SendBackfill(ctx, event)
}
