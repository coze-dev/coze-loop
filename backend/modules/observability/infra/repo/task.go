// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/redis/dao"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewTaskRepoImpl(TaskDao mysql.ITaskDao, idGenerator idgen.IIDGenerator, taskRedisDao dao.ITaskDAO) repo.ITaskRepo {
	return &TaskRepoImpl{
		TaskDao:      TaskDao,
		idGenerator:  idGenerator,
		TaskRedisDao: taskRedisDao,
	}
}

type TaskRepoImpl struct {
	TaskDao      mysql.ITaskDao
	TaskRedisDao dao.ITaskDAO
	idGenerator  idgen.IIDGenerator
}

// 缓存 TTL 常量
const (
	TaskDetailTTL       = 30 * time.Minute // 单个任务缓存30分钟
	TaskListTTL         = 5 * time.Minute  // 任务列表缓存5分钟
	NonFinalTaskListTTL = 1 * time.Minute  // 非最终状态任务缓存1分钟
	TaskCountTTL        = 10 * time.Minute // 任务计数缓存10分钟
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
	
	// 异步缓存到 Redis
	go func() {
		if err := v.TaskRedisDao.SetTask(context.Background(), taskDO, TaskDetailTTL); err != nil {
			logs.Error("failed to set task cache", "id", id, "err", err)
		}
	}()
	
	return taskDO, nil
}

func (v *TaskRepoImpl) ListTasks(ctx context.Context, param mysql.ListTaskParam) ([]*entity.ObservabilityTask, int64, error) {
	// 生成缓存 key
	var workspaceID int64
	if len(param.WorkspaceIDs) > 0 {
		workspaceID = param.WorkspaceIDs[0] // 简化处理，取第一个
	}
	filterHash := v.generateFilterHash(param)
	cacheKey := fmt.Sprintf("task:list:%d:%s:%d:%d", workspaceID, filterHash, param.ReqOffset, param.ReqLimit)
	
	// 先查 Redis 缓存
	cachedTasks, cachedTotal, err := v.TaskRedisDao.GetTaskList(ctx, cacheKey)
	if err != nil {
		logs.CtxWarn(ctx, "failed to get task list from redis cache", "key", cacheKey, "err", err)
	} else if cachedTasks != nil {
		return cachedTasks, cachedTotal, nil
	}

	// 缓存未命中，查询数据库
	results, total, err := v.TaskDao.ListTasks(ctx, param)
	if err != nil {
		return nil, 0, err
	}
	
	resp := make([]*entity.ObservabilityTask, len(results))
	for i, result := range results {
		resp[i] = convertor.TaskPO2DO(result)
	}
	
	// 异步缓存到 Redis
	go func() {
		if err := v.TaskRedisDao.SetTaskList(context.Background(), cacheKey, resp, total, TaskListTTL); err != nil {
			logs.Error("failed to set task list cache", "key", cacheKey, "err", err)
		}
	}()
	
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
		if err := v.TaskRedisDao.SetTask(context.Background(), do, TaskDetailTTL); err != nil {
			logs.Error("failed to set task cache after create", "id", createdID, "err", err)
		}
		
		// 清理相关列表缓存
		v.clearListCaches(context.Background(), do.WorkspaceID)
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
	
	// 数据库操作成功后，更新缓存
	go func() {
		// 更新单个任务缓存
		if err := v.TaskRedisDao.SetTask(context.Background(), do, TaskDetailTTL); err != nil {
			logs.Error("failed to update task cache", "id", do.ID, "err", err)
		}
		
		// 清理相关列表缓存
		v.clearListCaches(context.Background(), do.WorkspaceID)
	}()
	
	return nil
}

func (v *TaskRepoImpl) DeleteTask(ctx context.Context, id int64, workspaceID int64, userID string) error {
	// 先执行数据库操作
	err := v.TaskDao.DeleteTask(ctx, id, workspaceID, userID)
	if err != nil {
		return err
	}
	
	// 数据库操作成功后，删除缓存
	go func() {
		// 删除单个任务缓存
		if err := v.TaskRedisDao.DeleteTask(context.Background(), id); err != nil {
			logs.Error("failed to delete task cache", "id", id, "err", err)
		}
		
		// 清理相关列表缓存
		v.clearListCaches(context.Background(), workspaceID)
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
func (v *TaskRepoImpl) UpdateTaskWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error {
	// 先执行数据库操作
	err := v.TaskDao.UpdateTaskWithOCC(ctx, id, workspaceID, updateMap)
	if err != nil {
		return err
	}
	
	// 数据库操作成功后，删除缓存（因为无法直接更新部分字段）
	go func() {
		// 删除单个任务缓存，下次查询时会重新加载
		if err := v.TaskRedisDao.DeleteTask(context.Background(), id); err != nil {
			logs.Error("failed to delete task cache after OCC update", "id", id, "err", err)
		}
		
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

// generateFilterHash 生成过滤条件的 hash
func (v *TaskRepoImpl) generateFilterHash(param mysql.ListTaskParam) string {
	if param.TaskFilters == nil {
		return "no_filter"
	}
	
	// 将过滤条件序列化为字符串
	filterStr := fmt.Sprintf("%+v", param.TaskFilters)
	
	// 生成简单的 hash（在实际生产环境中可能需要更复杂的 hash 算法）
	return fmt.Sprintf("%x", len(filterStr))
}