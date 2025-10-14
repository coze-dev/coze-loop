// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/redis/dao"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewTaskRepoImpl(TaskDao mysql.ITaskDao, idGenerator idgen.IIDGenerator, taskRedisDao dao.ITaskDAO, taskRunDao mysql.ITaskRunDao, taskRunRedisDao dao.ITaskRunDAO) repo.ITaskRepo {
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
	TaskRedisDao    dao.ITaskDAO
	TaskRunRedisDao dao.ITaskRunDAO
	idGenerator     idgen.IIDGenerator
}

// 缓存 TTL 常量
const (
	TaskDetailTTL       = 30 * time.Minute // 单个任务缓存30分钟
	NonFinalTaskListTTL = 1 * time.Minute  // 非最终状态任务缓存1分钟
	TaskCountTTL        = 10 * time.Minute // 任务计数缓存10分钟
)

// 任务运行计数TTL常量
const (
	TaskRunCountTTL = 10 * time.Minute // 任务运行计数缓存10分钟
)

func (v *TaskRepoImpl) GetTask(ctx context.Context, id int64, workspaceID *int64, userID *string) (*entity.ObservabilityTask, error) {
	// 先查 Redis 缓存
	cachedTask, err := v.TaskRedisDao.GetTask(ctx, id)
	if err != nil {
		logs.CtxWarn(ctx, "failed to get task from redis cache", "id", id, "err", err)
	} else if cachedTask != nil {
		// 验证权限（workspaceID 和 userID）
		if workspaceID != nil && cachedTask.WorkspaceID != *workspaceID {
			return nil, nil // 权限不符，返回空
		}
		if userID != nil && cachedTask.CreatedBy != *userID {
			return nil, nil // 权限不符，返回空
		}
		return cachedTask, nil
	}

	// 缓存未命中，查询数据库
	TaskPO, err := v.TaskDao.GetTask(ctx, id, workspaceID, userID)
	if err != nil {
		return nil, err
	}

	taskDO := convertor.TaskPO2DO(TaskPO)

	TaskRunPO, _, err := v.TaskRunDao.ListTaskRuns(ctx, mysql.ListTaskRunParam{
		WorkspaceID: ptr.Of(taskDO.WorkspaceID),
		TaskID:      ptr.Of(taskDO.ID),
		ReqLimit:    1000,
		ReqOffset:   0,
	})

	taskDO.TaskRuns = convertor.TaskRunsPO2DO(TaskRunPO)
	if err != nil {
		return nil, err
	}

	// 异步缓存到 Redis
	go func() {
		if len(taskDO.TaskRuns) > 0 {
			if err := v.TaskRedisDao.SetTask(context.Background(), taskDO, TaskDetailTTL); err != nil {
				logs.Error("failed to set task cache", "id", id, "err", err)
			}
		}
	}()

	return taskDO, nil
}
func (v *TaskRepoImpl) ListTasks(ctx context.Context, param mysql.ListTaskParam) ([]*entity.ObservabilityTask, int64, error) {
	results, total, err := v.TaskDao.ListTasks(ctx, param)
	if err != nil {
		return nil, 0, err
	}
	resp := make([]*entity.ObservabilityTask, len(results))
	for i, result := range results {
		resp[i] = convertor.TaskPO2DO(result)
	}
	for _, t := range resp {
		taskRuns, _, err := v.TaskRunDao.ListTaskRuns(ctx, mysql.ListTaskRunParam{
			WorkspaceID: ptr.Of(t.WorkspaceID),
			TaskID:      ptr.Of(t.ID),
			ReqLimit:    param.ReqLimit,
			ReqOffset:   param.ReqOffset,
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

	// 数据库操作成功后，更新缓存
	go func() {
		// 更新单个任务缓存
		if len(do.TaskRuns) > 0 {
			if err = v.TaskRedisDao.SetTask(context.Background(), do, TaskDetailTTL); err != nil {
				logs.Error("failed to update task cache", "id", do.ID, "err", err)
				return
			}
		}
	}()

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
func (v *TaskRepoImpl) GetObjListWithTask(ctx context.Context) ([]string, []string, []*entity.ObservabilityTask) {
	var tasks []*entity.ObservabilityTask
	spaceList, botList, results, err := v.TaskDao.GetObjListWithTask(ctx)
	if err != nil {
		logs.CtxWarn(ctx, "failed to get obj list with task from mysql", "err", err)
		return nil, nil, nil
	}
	tasks = make([]*entity.ObservabilityTask, len(results))
	for i, result := range results {
		tasks[i] = convertor.TaskPO2DO(result)
	}

	return spaceList, botList, tasks
}
func (v *TaskRepoImpl) DeleteTask(ctx context.Context, do *entity.ObservabilityTask) error {
	// 先执行数据库删除操作
	err := v.TaskDao.DeleteTask(ctx, do.ID, do.WorkspaceID, do.CreatedBy)
	if err != nil {
		return err
	}

	return nil
}

func (v *TaskRepoImpl) CreateTaskRun(ctx context.Context, do *entity.TaskRun) (int64, error) {
	// 1. 生成ID
	id, err := v.idGenerator.GenID(ctx)
	if err != nil {
		return 0, err
	}

	// 2. 转换并设置ID
	taskRunPo := convertor.TaskRunDO2PO(do)
	taskRunPo.ID = id

	// 3. 数据库创建
	createdID, err := v.TaskRunDao.CreateTaskRun(ctx, taskRunPo)
	if err != nil {
		return 0, err
	}

	// 4. 异步更新缓存
	do.ID = createdID
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
func (v *TaskRepoImpl) IncrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) error {
	return v.TaskRunRedisDao.IncrTaskRunSuccessCount(ctx, taskID, taskRunID)
}
func (v *TaskRepoImpl) DecrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) error {
	return v.TaskRunRedisDao.DecrTaskRunSuccessCount(ctx, taskID, taskRunID)
}

func (v *TaskRepoImpl) GetTaskRunFailCount(ctx context.Context, taskID, taskRunID int64) (int64, error) {
	return v.TaskRunRedisDao.GetTaskRunFailCount(ctx, taskID, taskRunID)
}
func (v *TaskRepoImpl) IncrTaskRunFailCount(ctx context.Context, taskID, taskRunID int64) error {
	return v.TaskRunRedisDao.IncrTaskRunFailCount(ctx, taskID, taskRunID)
}
