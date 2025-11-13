// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/redis"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewTaskRepoImpl(TaskDao mysql.ITaskDao, idGenerator idgen.IIDGenerator, taskRedisDao redis.ITaskDAO, taskRunDao mysql.ITaskRunDao, taskRunRedisDao redis.ITaskRunDAO) repo.ITaskRepo {
	return &TaskRepoImpl{
		TaskDao:         TaskDao,
		idGenerator:     idGenerator,
		TaskRedisDao:    taskRedisDao,
		TaskRunDao:      taskRunDao,
		TaskRunRedisDao: taskRunRedisDao,
	}
}

type TaskRepoImpl struct {
	TaskDao         mysql.ITaskDao
	TaskRunDao      mysql.ITaskRunDao
	TaskRedisDao    redis.ITaskDAO
	TaskRunRedisDao redis.ITaskRunDAO
	idGenerator     idgen.IIDGenerator
}

func (v *TaskRepoImpl) GetTask(ctx context.Context, id int64, workspaceID *int64, userID *string) (*entity.ObservabilityTask, error) {
	TaskPO, err := v.TaskDao.GetTask(ctx, id, workspaceID, userID)
	if err != nil {
		return nil, err
	}

	taskDO := convertor.TaskPO2DO(TaskPO)

	TaskRunPO, _, err := v.TaskRunDao.ListTaskRuns(ctx, mysql.ListTaskRunParam{
		WorkspaceID: ptr.Of(taskDO.WorkspaceID),
		TaskID:      ptr.Of(taskDO.ID),
		ReqLimit:    500,
		ReqOffset:   0,
	})

	taskDO.TaskRuns = convertor.TaskRunsPO2DO(TaskRunPO)
	if err != nil {
		return nil, err
	}

	return taskDO, nil
}

func (v *TaskRepoImpl) ListTasks(ctx context.Context, param repo.ListTaskParam) ([]*entity.ObservabilityTask, int64, error) {
	results, total, err := v.TaskDao.ListTasks(ctx, mysql.ListTaskParam{
		WorkspaceIDs: param.WorkspaceIDs,
		TaskFilters:  param.TaskFilters,
		ReqLimit:     param.ReqLimit,
		ReqOffset:    param.ReqOffset,
		OrderBy:      param.OrderBy,
	})
	if err != nil {
		return nil, 0, err
	}
	resp := make([]*entity.ObservabilityTask, len(results))
	for i, result := range results {
		resp[i] = convertor.TaskPO2DO(result)
	}
	// todo 待优化
	for _, t := range resp {
		taskRuns, _, err := v.TaskRunDao.ListTaskRuns(ctx, mysql.ListTaskRunParam{
			WorkspaceID: ptr.Of(t.WorkspaceID),
			TaskID:      ptr.Of(t.ID),
			ReqLimit:    500,
			ReqOffset:   0,
		})
		if err != nil {
			logs.CtxError(ctx, "ListTaskRuns err, taskID:%d, err:%v", t.ID, err)
			continue
		}
		t.TaskRuns = convertor.TaskRunsPO2DO(taskRuns)
	}

	return resp, total, nil
}

func (v *TaskRepoImpl) CreateTask(ctx context.Context, do *entity.ObservabilityTask) (int64, error) {
	id, err := v.idGenerator.GenID(ctx)
	if err != nil {
		return 0, err
	}
	TaskPo := convertor.TaskDO2PO(do)
	TaskPo.ID = id

	// 先执行数据库操作
	createdID, err := v.TaskDao.CreateTask(ctx, TaskPo)
	if err != nil {
		return 0, err
	}
	err = v.TaskRedisDao.AddNonFinalTask(ctx, strconv.FormatInt(do.WorkspaceID, 10), id)
	if err != nil {
		return createdID, err
	}
	err = v.TaskRedisDao.SetTask(ctx, do)
	if err != nil {
		return createdID, err
	}
	return createdID, nil
}

func (v *TaskRepoImpl) UpdateTask(ctx context.Context, do *entity.ObservabilityTask) error {
	TaskPo := convertor.TaskDO2PO(do)

	// 先执行数据库操作
	err := v.TaskDao.UpdateTask(ctx, TaskPo)
	if err != nil {
		return err
	}
	for _, tr := range do.TaskRuns {
		TaskRunPo := convertor.TaskRunDO2PO(tr)
		err = v.TaskRunDao.UpdateTaskRun(ctx, TaskRunPo)
		if err != nil {
			return err
		}
	}
	err = v.TaskRedisDao.SetTask(ctx, do)
	if err != nil {
		return err
	}

	return nil
}

func (v *TaskRepoImpl) UpdateTaskWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error {
	// 先执行数据库操作
	logs.CtxInfo(ctx, "UpdateTaskWithOCC, id:%d, workspaceID:%d, updateMap:%+v", id, workspaceID, updateMap)
	err := v.TaskDao.UpdateTaskWithOCC(ctx, id, workspaceID, updateMap)
	if err != nil {
		return err
	}
	return nil
}

func (v *TaskRepoImpl) DeleteTask(ctx context.Context, do *entity.ObservabilityTask) error {
	// 先执行数据库删除操作
	err := v.TaskDao.DeleteTask(ctx, do.ID, do.WorkspaceID, do.CreatedBy)
	if err != nil {
		return err
	}

	err = v.TaskRedisDao.RemoveNonFinalTask(ctx, strconv.FormatInt(do.WorkspaceID, 10), do.ID)
	if err != nil {
		logs.CtxError(ctx, "remove non final task failed, task_id=%d, err=%v", do.ID, err)
	}
	return nil
}

func (v *TaskRepoImpl) ListNonFinalTasks(ctx context.Context) ([]*entity.ObservabilityTask, error) {
	result, err := v.TaskDao.ListNonFinalTasks(ctx)
	if err != nil {
		return nil, err
	}

	resp := make([]*entity.ObservabilityTask, len(result))
	for i, t := range result {
		resp[i] = convertor.TaskPO2DO(t)
	}
	return resp, nil
}

func (v *TaskRepoImpl) CreateTaskRun(ctx context.Context, do *entity.TaskRun) (int64, error) {
	// 1. 生成ID
	id, err := v.idGenerator.GenID(ctx)
	if err != nil {
		return 0, err
	}
	do.ID = id

	// 2. 转换并设置ID
	taskRunPo := convertor.TaskRunDO2PO(do)

	// 3. 数据库创建
	createdID, err := v.TaskRunDao.CreateTaskRun(ctx, taskRunPo)
	if err != nil {
		return 0, err
	}

	return createdID, nil
}

func (v *TaskRepoImpl) UpdateTaskRun(ctx context.Context, do *entity.TaskRun) error {
	// 1. 转换并更新数据库
	taskRunPo := convertor.TaskRunDO2PO(do)
	err := v.TaskRunDao.UpdateTaskRun(ctx, taskRunPo)
	if err != nil {
		return err
	}
	return nil
}

func (v *TaskRepoImpl) UpdateTaskRunWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error {
	// 先执行数据库操作
	logs.CtxInfo(ctx, "UpdateTaskRunWithOCC, id:%d, workspaceID:%d, updateMap:%+v", id, workspaceID, updateMap)
	err := v.TaskRunDao.UpdateTaskRunWithOCC(ctx, id, workspaceID, updateMap)
	if err != nil {
		return err
	}

	return nil
}

func (v *TaskRepoImpl) GetBackfillTaskRun(ctx context.Context, workspaceID *int64, taskID int64) (*entity.TaskRun, error) {
	taskRunPo, err := v.TaskRunDao.GetBackfillTaskRun(ctx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	if taskRunPo == nil {
		return nil, nil
	}
	return convertor.TaskRunPO2DO(taskRunPo), nil
}

func (v *TaskRepoImpl) GetLatestNewDataTaskRun(ctx context.Context, workspaceID *int64, taskID int64) (*entity.TaskRun, error) {
	taskRunPo, err := v.TaskRunDao.GetLatestNewDataTaskRun(ctx, workspaceID, taskID)
	if err != nil {
		return nil, err
	}
	if taskRunPo == nil {
		return nil, nil
	}
	return convertor.TaskRunPO2DO(taskRunPo), nil
}

func (v *TaskRepoImpl) GetTaskCount(ctx context.Context, taskID int64) (int64, error) {
	count, err := v.TaskRedisDao.GetTaskCount(ctx, taskID)
	if err != nil {
		logs.CtxWarn(ctx, "failed to get task count from redis cache", "taskID", taskID, "err", err)
	} else if count != 0 {
		return count, nil
	}
	return count, nil
}

func (v *TaskRepoImpl) IncrTaskCount(ctx context.Context, taskID, ttl int64) error {
	_, err := v.TaskRedisDao.IncrTaskCount(ctx, taskID, time.Duration(ttl)*time.Second)
	if err != nil {
		logs.CtxError(ctx, "failed to increment task count", "taskID", taskID, "err", err)
		return err
	}
	return nil
}

func (v *TaskRepoImpl) DecrTaskCount(ctx context.Context, taskID, ttl int64) error {
	_, err := v.TaskRedisDao.DecrTaskCount(ctx, taskID, time.Duration(ttl)*time.Second)
	if err != nil {
		logs.CtxError(ctx, "failed to decrement task count", "taskID", taskID, "err", err)
		return err
	}
	return nil
}

func (v *TaskRepoImpl) GetTaskRunCount(ctx context.Context, taskID, taskRunID int64) (int64, error) {
	count, err := v.TaskRedisDao.GetTaskRunCount(ctx, taskID, taskRunID)
	if err != nil {
		logs.CtxWarn(ctx, "failed to get task run count from redis cache", "taskID", taskID, "err", err)
	} else if count != 0 {
		return count, nil
	}
	return count, nil
}

func (v *TaskRepoImpl) IncrTaskRunCount(ctx context.Context, taskID, taskRunID int64, ttl int64) error {
	_, err := v.TaskRedisDao.IncrTaskRunCount(ctx, taskID, taskRunID, time.Duration(ttl)*time.Second)
	if err != nil {
		logs.CtxError(ctx, "failed to increment task run count", "taskID", taskID, "taskRunID", taskRunID, "err", err)
		return err
	}
	return nil
}

func (v *TaskRepoImpl) DecrTaskRunCount(ctx context.Context, taskID, taskRunID int64, ttl int64) error {
	_, err := v.TaskRedisDao.DecrTaskRunCount(ctx, taskID, taskRunID, time.Duration(ttl)*time.Second)
	if err != nil {
		logs.CtxError(ctx, "failed to decrement task run count", "taskID", taskID, "taskRunID", taskRunID, "err", err)
		return err
	}
	return nil
}

func (v *TaskRepoImpl) GetTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) (int64, error) {
	return v.TaskRunRedisDao.GetTaskRunSuccessCount(ctx, taskID, taskRunID)
}

func (v *TaskRepoImpl) IncrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64, ttl int64) error {
	return v.TaskRunRedisDao.IncrTaskRunSuccessCount(ctx, taskID, taskRunID, time.Duration(ttl)*time.Second)
}

func (v *TaskRepoImpl) DecrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) error {
	return v.TaskRunRedisDao.DecrTaskRunSuccessCount(ctx, taskID, taskRunID)
}

func (v *TaskRepoImpl) GetTaskRunFailCount(ctx context.Context, taskID, taskRunID int64) (int64, error) {
	return v.TaskRunRedisDao.GetTaskRunFailCount(ctx, taskID, taskRunID)
}

func (v *TaskRepoImpl) IncrTaskRunFailCount(ctx context.Context, taskID, taskRunID int64, ttl int64) error {
	return v.TaskRunRedisDao.IncrTaskRunFailCount(ctx, taskID, taskRunID, time.Duration(ttl)*time.Second)
}

func (v *TaskRepoImpl) ListNonFinalTaskBySpaceID(ctx context.Context, spaceID string) ([]int64, error) {
	return v.TaskRedisDao.ListNonFinalTask(ctx, spaceID)
}

func (v *TaskRepoImpl) AddNonFinalTask(ctx context.Context, spaceID string, taskID int64) error {
	return v.TaskRedisDao.AddNonFinalTask(ctx, spaceID, taskID)
}

func (v *TaskRepoImpl) RemoveNonFinalTask(ctx context.Context, spaceID string, taskID int64) error {
	return v.TaskRedisDao.RemoveNonFinalTask(ctx, spaceID, taskID)
}

func (v *TaskRepoImpl) GetTaskByCache(ctx context.Context, taskID int64) (*entity.ObservabilityTask, error) {
	taskDO, err := v.TaskRedisDao.GetTask(ctx, taskID)
	if err != nil {
		logs.CtxError(ctx, "Failed to get task", "err", err)
		return nil, err
	}
	if taskDO == nil {
		taskPO, err := v.TaskDao.GetTask(ctx, taskID, nil, nil)
		if err != nil {
			logs.CtxError(ctx, "Failed to get task", "err", err)
			return nil, err
		}
		if taskPO == nil {
			return nil, nil
		}
		taskDO = convertor.TaskPO2DO(taskPO)
		err = v.TaskRedisDao.SetTask(ctx, taskDO)
		if err != nil {
			logs.CtxError(ctx, "Failed to set task", "err", err)
			return nil, err
		}
	}
	return taskDO, nil
}
