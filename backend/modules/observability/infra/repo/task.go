// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"fmt"
	"strconv"
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

func NewTaskRepoImpl(TaskDao mysql.ITaskDao, idGenerator idgen.IIDGenerator, taskRedisDao dao.ITaskDAO, taskRunDao mysql.ITaskRunDao) repo.ITaskRepo {
	return &TaskRepoImpl{
		TaskDao:      TaskDao,
		idGenerator:  idGenerator,
		TaskRedisDao: taskRedisDao,
		TaskRunDao:   taskRunDao,
	}
}

type TaskRepoImpl struct {
	TaskDao      mysql.ITaskDao
	TaskRunDao   mysql.ITaskRunDao
	TaskRedisDao dao.ITaskDAO
	idGenerator  idgen.IIDGenerator
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
	TaskPo, err := v.TaskDao.GetTask(ctx, id, workspaceID, userID)
	if err != nil {
		return nil, err
	}

	taskDO := convertor.TaskPO2DO(TaskPo)

	TaskRunPo, _, err := v.TaskRunDao.ListTaskRuns(ctx, mysql.ListTaskRunParam{
		WorkspaceID: ptr.Of(taskDO.WorkspaceID),
		TaskID:      ptr.Of(taskDO.ID),
		ReqLimit:    1000,
		ReqOffset:   0,
	})

	taskDO.TaskRuns = convertor.TaskRunsPO2DO(TaskRunPo)
	if err != nil {
		return nil, err
	}

	// 异步缓存到 Redis
	go func() {
		if err := v.TaskRedisDao.SetTask(context.Background(), taskDO, TaskDetailTTL); err != nil {
			logs.Error("failed to set task cache", "id", id, "err", err)
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

	// 数据库操作成功后，更新缓存
	do.ID = createdID
	go func() {
		// 缓存新创建的任务
		if err = v.TaskRedisDao.SetTask(context.Background(), do, TaskDetailTTL); err != nil {
			logs.Error("failed to set task cache after create", "id", createdID, "err", err)
		}
		// 更新非最终状态任务列表缓存
		if err = v.TaskRedisDao.AddNonFinalTask(ctx, do); err != nil {
			logs.Error("failed to set non final task cache after create", "id", createdID, "err", err)
			return
		}
	}()

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
		if err = v.TaskRedisDao.SetTask(context.Background(), do, TaskDetailTTL); err != nil {
			logs.Error("failed to update task cache", "id", do.ID, "err", err)
			return
		}
	}()

	return nil
}

func (v *TaskRepoImpl) ListNonFinalTask(ctx context.Context) ([]*entity.ObservabilityTask, error) {
	// 先查 Redis 缓存
	cachedTasks, err := v.TaskRedisDao.GetNonFinalTaskList(ctx)
	if err != nil {
		logs.CtxWarn(ctx, "failed to get non final task list from redis cache", "err", err)
	} else if cachedTasks != nil {
		return cachedTasks, nil
	}

	// 缓存未命中，查询数据库
	results, err := v.TaskDao.ListNonFinalTask(ctx)
	if err != nil {
		return nil, err
	}

	resp := make([]*entity.ObservabilityTask, len(results))
	for i, result := range results {
		resp[i] = convertor.TaskPO2DO(result)
	}

	// 异步缓存到 Redis（短TTL，因为非最终状态变化频繁）
	go func() {
		if err := v.TaskRedisDao.SetNonFinalTaskList(context.Background(), resp, NonFinalTaskListTTL); err != nil {
			logs.Error("failed to set non final task list cache", "err", err)
		}
	}()

	return resp, nil
}
func (v *TaskRepoImpl) ListNonFinalTaskBySpaceID(ctx context.Context, spaceID string) []*entity.ObservabilityTask {
	// 先查 Redis 缓存
	cachedTasks, err := v.TaskRedisDao.GetNonFinalTaskList(ctx)
	if err != nil {
		logs.CtxWarn(ctx, "failed to get non final task list from redis cache", "err", err)
	} else if cachedTasks != nil {
		return cachedTasks
	}
	// 缓存未命中，查询数据库
	spaceIDInt, _ := strconv.ParseInt(spaceID, 10, 64)
	results, err := v.TaskDao.ListNonFinalTaskBySpaceID(ctx, spaceIDInt)
	if err != nil {
		return nil
	}
	resp := make([]*entity.ObservabilityTask, len(results))
	for i, result := range results {
		resp[i] = convertor.TaskPO2DO(result)
	}
	return resp
}
func (v *TaskRepoImpl) UpdateTaskWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error {
	// 先执行数据库操作
	logs.CtxInfo(ctx, "UpdateTaskWithOCC, id:%d, workspaceID:%d, updateMap:%+v", id, workspaceID, updateMap)
	err := v.TaskDao.UpdateTaskWithOCC(ctx, id, workspaceID, updateMap)
	if err != nil {
		return err
	}

	// 数据库操作成功后，删除缓存（因为无法直接更新部分字段）
	go func() {
		// 清理相关列表缓存
		v.clearListCaches(context.Background(), workspaceID)

		// 清理非最终状态任务缓存（状态可能发生变化）
		if err := v.TaskRedisDao.DeleteNonFinalTaskList(context.Background()); err != nil {
			logs.Error("failed to delete non final task list cache after OCC update", "err", err)
		}
	}()

	return nil
}

// clearListCaches 清理与指定 workspace 相关的列表缓存
func (v *TaskRepoImpl) clearListCaches(ctx context.Context, workspaceID int64) {
	// 清理任务列表缓存（使用模糊匹配）
	pattern := fmt.Sprintf("task:list:%d:*", workspaceID)
	if err := v.TaskRedisDao.DeleteTaskList(ctx, pattern); err != nil {
		logs.Error("failed to delete task list cache", "pattern", pattern, "err", err)
	}

	// 清理任务计数缓存
	if err := v.TaskRedisDao.DeleteTaskCount(ctx, workspaceID); err != nil {
		logs.Error("failed to delete task count cache", "workspaceID", workspaceID, "err", err)
	}

	// 清理非最终状态任务缓存
	if err := v.TaskRedisDao.DeleteNonFinalTaskList(ctx); err != nil {
		logs.Error("failed to delete non final task list cache", "err", err)
	}
}

// isNonFinalTaskStatus 判断任务状态是否为非最终状态
func isNonFinalTaskStatus(status string) bool {
	finalStatuses := []string{"success", "disabled"}
	for _, finalStatus := range finalStatuses {
		if status == finalStatus {
			return false
		}
	}
	return true
}

func (v *TaskRepoImpl) GetObjListWithTask(ctx context.Context) ([]string, []string) {
	// 先查 Redis 缓存
	spaceList, botList, err := v.TaskRedisDao.GetObjListWithTask(ctx)
	if err != nil {
		logs.CtxWarn(ctx, "failed to get obj list with task from redis cache", "err", err)
		// Redis失败时从MySQL获取
		spaceList, botList, err = v.TaskDao.GetObjListWithTask(ctx)
		if err != nil {
			logs.CtxWarn(ctx, "failed to get obj list with task from mysql", "err", err)
			return nil, nil
		}
	}

	return spaceList, botList
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
func (v *TaskRepoImpl) GetTaskRunCount(ctx context.Context, taskID, taskRunID int64) (int64, error) {
	count, err := v.TaskRedisDao.GetTaskRunCount(ctx, taskID, taskRunID)
	if err != nil {
		logs.CtxWarn(ctx, "failed to get task run count from redis cache", "taskID", taskID, "err", err)
	} else if count != 0 {
		return count, nil
	}
	return count, nil
}

func (v *TaskRepoImpl) IncrTaskCount(ctx context.Context, taskID int64) error {
	_, err := v.TaskRedisDao.IncrTaskCount(ctx, taskID, TaskCountTTL)
	if err != nil {
		logs.CtxError(ctx, "failed to increment task count", "taskID", taskID, "err", err)
		return err
	}
	return nil
}

func (v *TaskRepoImpl) DecrTaskCount(ctx context.Context, taskID int64) error {
	_, err := v.TaskRedisDao.DecrTaskCount(ctx, taskID, TaskCountTTL)
	if err != nil {
		logs.CtxError(ctx, "failed to decrement task count", "taskID", taskID, "err", err)
		return err
	}
	return nil
}

func (v *TaskRepoImpl) IncrTaskRunCount(ctx context.Context, taskID, taskRunID int64) error {
	_, err := v.TaskRedisDao.IncrTaskRunCount(ctx, taskID, taskRunID, TaskRunCountTTL)
	if err != nil {
		logs.CtxError(ctx, "failed to increment task run count", "taskID", taskID, "taskRunID", taskRunID, "err", err)
		return err
	}
	return nil
}

func (v *TaskRepoImpl) DecrTaskRunCount(ctx context.Context, taskID, taskRunID int64) error {
	_, err := v.TaskRedisDao.DecrTaskRunCount(ctx, taskID, taskRunID, TaskRunCountTTL)
	if err != nil {
		logs.CtxError(ctx, "failed to decrement task run count", "taskID", taskID, "taskRunID", taskRunID, "err", err)
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

	// 数据库操作成功后，异步清理缓存
	go func() {
		// 清理相关列表缓存
		v.clearListCaches(context.Background(), do.WorkspaceID)
		
		// 如果是非终态任务，需要从非终态任务列表中移除
		if isNonFinalTaskStatus(do.TaskStatus) {
			if err := v.TaskRedisDao.RemoveNonFinalTask(context.Background(), do.ID); err != nil {
				logs.Error("failed to remove task from non final task list after delete", "id", do.ID, "err", err)
			}
		}
	}()

	return nil
}

func (v *TaskRepoImpl) GetTaskRun(ctx context.Context, id int64, workspaceID *int64, userID *string) (*entity.TaskRun, error) {
	// 直接查询数据库(TaskRun通常不单独缓存)
	taskRunPo, err := v.TaskRunDao.GetTaskRun(ctx, id, workspaceID, nil)
	if err != nil {
		return nil, err
	}

	// 如果需要userID验证，需要通过Task表验证创建者权限
	if userID != nil {
		taskPo, err := v.TaskDao.GetTask(ctx, taskRunPo.TaskID, workspaceID, userID)
		if err != nil {
			return nil, err
		}
		if taskPo == nil {
			return nil, nil // 权限不符，返回空
		}
	}

	return convertor.TaskRunPO2DO(taskRunPo), nil
}

func (v *TaskRepoImpl) ListTaskRuns(ctx context.Context, taskID int64, param mysql.ListTaskRunParam) ([]*entity.TaskRun, int64, error) {
	// 设置TaskID过滤条件
	param.TaskID = &taskID

	// 查询数据库
	results, total, err := v.TaskRunDao.ListTaskRuns(ctx, param)
	if err != nil {
		return nil, 0, err
	}

	// 转换为DO
	taskRuns := convertor.TaskRunsPO2DO(results)
	return taskRuns, total, nil
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
	go func() {
		// 清理相关列表缓存(因为TaskRuns列表发生变化，Task的缓存会自然过期)
		v.clearListCaches(context.Background(), do.WorkspaceID)
	}()

	return createdID, nil
}

func (v *TaskRepoImpl) UpdateTaskRun(ctx context.Context, do *entity.TaskRun) error {
	// 1. 转换并更新数据库
	taskRunPo := convertor.TaskRunDO2PO(do)
	err := v.TaskRunDao.UpdateTaskRun(ctx, taskRunPo)
	if err != nil {
		return err
	}

	// 2. 异步清理缓存
	go func() {
		// 清理相关列表缓存(因为TaskRuns信息发生变化，Task的缓存会自然过期)
		v.clearListCaches(context.Background(), do.WorkspaceID)
	}()

	return nil
}

func (v *TaskRepoImpl) UpdateTaskRunWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error {
	// 1. 执行OCC更新
	logs.CtxInfo(ctx, "UpdateTaskRunWithOCC", "id", id, "workspaceID", workspaceID, "updateMap", updateMap)
	err := v.TaskRunDao.UpdateTaskRunWithOCC(ctx, id, workspaceID, updateMap)
	if err != nil {
		return err
	}

	// 2. 异步清理缓存
	go func() {
		// 清理相关列表缓存(Task的缓存会自然过期)
		v.clearListCaches(context.Background(), workspaceID)
	}()

	return nil
}

// GetAllTaskRunCountKeys 获取所有TaskRunCount键
func (v *TaskRepoImpl) GetAllTaskRunCountKeys(ctx context.Context) ([]string, error) {
	return v.TaskRedisDao.GetAllTaskRunCountKeys(ctx)
}
