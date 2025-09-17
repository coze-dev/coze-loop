// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
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
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe/processor"
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
	tRunRepo repo.ITaskRunRepo,
	userProvider rpc.IUserProvider,
	idGenerator idgen.IIDGenerator,
	backfillProducer mq.IBackfillProducer,
) (ITaskService, error) {
	return &TaskServiceImpl{
		TaskRepo:         tRepo,
		TaskRunRepo:      tRunRepo,
		userProvider:     userProvider,
		idGenerator:      idGenerator,
		backfillProducer: backfillProducer,
	}, nil
}

type TaskServiceImpl struct {
	TaskRepo         repo.ITaskRepo
	TaskRunRepo      repo.ITaskRunRepo
	userProvider     rpc.IUserProvider
	idGenerator      idgen.IIDGenerator
	backfillProducer mq.IBackfillProducer
}

func (t *TaskServiceImpl) CreateTask(ctx context.Context, req *CreateTaskReq) (resp *CreateTaskResp, err error) {
	// 校验task name是否存在
	checkResp, err := t.CheckTaskName(ctx, &CheckTaskNameReq{
		WorkspaceID: req.Task.GetWorkspaceID(),
		Name:        req.Task.GetName(),
	})
	if err != nil {
		logs.CtxError(ctx, "CheckTaskName err:%v", err)
		return nil, err
	}
	if !*checkResp.Pass {
		logs.CtxError(ctx, "task name exist")
		return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("task name exist"))
	}
	proc, err := processor.NewProcessor(ctx, req.Task.TaskType)
	if err != nil {
		logs.CtxError(ctx, "NewProcessor err:%v", err)
		return nil, err
	}
	// 校验配置项是否有效
	if err = proc.ValidateConfig(ctx, req.Task); err != nil {
		logs.CtxError(ctx, "ValidateConfig err:%v", err)
		return nil, err
	}
	userID := session.UserIDInCtxOrEmpty(ctx)
	if userID == "" {
		return nil, errorx.NewByCode(obErrorx.UserParseFailedCode)
	}
	// 创建task
	taskPO := tconv.CreateTaskDTO2PO(ctx, req.Task, userID)
	id, err := t.TaskRepo.CreateTask(ctx, taskPO)
	if err != nil {
		return nil, err
	}
	// 数据回流任务——创建/更新输出数据集
	// 自动评测历史回溯——创建空壳子
	taskOp := taskexe.TaskOpUndefined
	if req.Task.GetTaskType() == task.TaskTypeAutoDataReflow {
		taskOp = taskexe.TaskOpNewData
	}
	if t.shouldTriggerBackfill(req.Task) {
		taskOp = taskexe.TaskOpCreateBackfill
	}
	if taskOp != taskexe.TaskOpUndefined {
		taskConfig := tconv.TaskPO2DTO(ctx, taskPO, nil)
		if err = proc.OnChangeProcessor(ctx, taskConfig, taskOp); err != nil {
			logs.CtxError(ctx, "create initial task run failed, task_id=%d, err=%v", id, err)
			//任务改为禁用？
			if err = t.TaskRepo.DeleteTask(ctx, taskPO); err != nil {
				logs.CtxError(ctx, "delete task failed, task_id=%d, err=%v", id, err)
			}
			return nil, err
		}
	}
	// 历史回溯数据发MQ
	if t.shouldTriggerBackfill(req.Task) {
		backfillEvent := &entity.BackFillEvent{
			SpaceID: req.Task.GetWorkspaceID(),
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
	// 校验更新参数是否合法
	if req.Description != nil {
		taskPO.Description = req.Description
	}
	if req.EffectiveTime != nil {
		validEffectiveTime, err := tconv.CheckEffectiveTime(ctx, req.EffectiveTime, taskPO.TaskStatus, taskPO.EffectiveTime)
		if err != nil {
			return err
		}
		taskPO.EffectiveTime = ptr.Of(tconv.ToJSONString(ctx, validEffectiveTime))
	}
	if req.SampleRate != nil {
		taskPO.Sampler = ptr.Of(tconv.ToJSONString(ctx, req.SampleRate))
	}
	if req.TaskStatus != nil {
		validTaskStatus, err := tconv.CheckTaskStatus(ctx, *req.TaskStatus, taskPO.TaskStatus)
		if err != nil {
			return err
		}
		if validTaskStatus != "" {
			if validTaskStatus == task.TaskStatusDisabled {
				// 禁用操作处理
				//proc, err := processor.NewProcessor(ctx, task.TaskTypeAutoEval)
				//if err != nil {
				//	logs.CtxError(ctx, "CreateTask NewProcessor err:%v", err)
				//	return err
				//}
				//taskConfig := tconv.TaskPO2DTO(ctx, taskPO, nil)
				//taskRuns := tconv.TaskRunPOs2DOs(ctx, taskPO.TaskRuns, nil)
				//var taskRun *task.TaskRun
				//for _, tr := range taskRuns {
				//	if tr.RunStatus == task.RunStatusRunning {
				//		taskRun = tr
				//		break
				//	}
				//}
				//if err = proc.Finish(ctx, taskRun, &taskexe.Trigger{Task: taskConfig, Span: nil, IsFinish: false}); err != nil {
				//	logs.CtxError(ctx, "proc Finish err:%v", err)
				//	return err
				//
				//}
			}
			taskPO.TaskStatus = *req.TaskStatus
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
	taskPOs, total, err := t.TaskRepo.ListTasks(ctx, mysql.ListTaskParam{
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
	return &ListTasksResp{
		Tasks: tconv.TaskPOs2DOs(ctx, taskPOs, userInfoMap),
		Total: ptr.Of(total),
	}, nil
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
	return &GetTaskResp{Task: tconv.TaskPO2DTO(ctx, taskPO, userInfoMap)}, nil
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
func (t *TaskServiceImpl) shouldTriggerBackfill(taskDO *task.Task) bool {
	// 检查任务类型
	taskType := taskDO.GetTaskType()
	if taskType != task.TaskTypeAutoEval && taskType != task.TaskTypeAutoDataReflow {
		return false
	}

	// 检查回填时间配置
	rule := taskDO.GetRule()
	if rule == nil {
		return false
	}

	backfillTime := rule.GetBackfillEffectiveTime()
	if backfillTime == nil {
		return false
	}

	return backfillTime.GetStartAt() > 0 &&
		backfillTime.GetEndAt() > 0 &&
		backfillTime.GetStartAt() < backfillTime.GetEndAt()
}

// shouldCreateTaskRun 判断是否需要创建TaskRun
func (t *TaskServiceImpl) shouldCreateTaskRun(taskDO *task.Task) bool {
	// 只有数据回流任务需要立即创建TaskRun
	return taskDO.GetTaskType() == task.TaskTypeAutoDataReflow || t.shouldTriggerBackfill(taskDO)
}

// sendBackfillMessage 发送MQ消息
func (t *TaskServiceImpl) sendBackfillMessage(ctx context.Context, event *entity.BackFillEvent) error {
	if t.backfillProducer == nil {
		return errorx.NewByCode(obErrorx.CommonInternalErrorCode, errorx.WithExtraMsg("backfill producer not initialized"))
	}

	return t.backfillProducer.SendBackfill(ctx, event)
}

// toJSONString 将对象转换为JSON字符串
func (t *TaskServiceImpl) toJSONString(ctx context.Context, obj interface{}) string {
	if obj == nil {
		return ""
	}
	jsonData, err := json.Marshal(obj)
	if err != nil {
		logs.CtxError(ctx, "JSON marshal error: %v", err)
		return ""
	}
	return string(jsonData)
}
