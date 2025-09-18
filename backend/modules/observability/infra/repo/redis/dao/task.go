// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/redis/convert"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate mockgen -destination=mocks/Task_dao.go -package=mocks . ITaskDAO
type ITaskDAO interface {
	// Task相关
	GetTask(ctx context.Context, id int64) (*entity.ObservabilityTask, error)
	SetTask(ctx context.Context, task *entity.ObservabilityTask, ttl time.Duration) error

	// TaskList相关
	GetTaskList(ctx context.Context, key string) ([]*entity.ObservabilityTask, int64, error)
	SetTaskList(ctx context.Context, key string, tasks []*entity.ObservabilityTask, total int64, ttl time.Duration) error
	DeleteTaskList(ctx context.Context, pattern string) error

	// NonFinalTaskList相关
	GetNonFinalTaskList(ctx context.Context) ([]*entity.ObservabilityTask, error)
	SetNonFinalTaskList(ctx context.Context, tasks []*entity.ObservabilityTask, ttl time.Duration) error
	DeleteNonFinalTaskList(ctx context.Context) error
	AddNonFinalTask(ctx context.Context, task *entity.ObservabilityTask) error
	RemoveNonFinalTask(ctx context.Context, taskID int64) error

	// TaskCount相关
	GetTaskCount(ctx context.Context, taskID int64) (int64, error)
	SetTaskCount(ctx context.Context, taskID int64, count int64, ttl time.Duration) error
	DeleteTaskCount(ctx context.Context, taskID int64) error
	IncrTaskCount(ctx context.Context, taskID int64, ttl time.Duration) (int64, error)
	DecrTaskCount(ctx context.Context, taskID int64, ttl time.Duration) (int64, error)

	// TaskRunCount相关
	GetTaskRunCount(ctx context.Context, taskID, taskRunID int64) (int64, error)
	SetTaskRunCount(ctx context.Context, taskID, taskRunID int64, count int64, ttl time.Duration) error
	DeleteTaskRunCount(ctx context.Context, taskID, taskRunID int64) error
	IncrTaskRunCount(ctx context.Context, taskID, taskRunID int64, ttl time.Duration) (int64, error)
	DecrTaskRunCount(ctx context.Context, taskID, taskRunID int64, ttl time.Duration) (int64, error)

	GetObjListWithTask(ctx context.Context) ([]string, []string, error)

	// SpaceListWithTask相关
	GetSpaceListWithTask(ctx context.Context) ([]string, error)
	SetSpaceListWithTask(ctx context.Context, spaces []string, ttl time.Duration) error
	DeleteSpaceListWithTask(ctx context.Context) error

	// BotListWithTask相关
	GetBotListWithTask(ctx context.Context) ([]string, error)
	SetBotListWithTask(ctx context.Context, bots []string, ttl time.Duration) error
	DeleteBotListWithTask(ctx context.Context) error

	// WorkflowListWithTask相关
	GetWorkflowListWithTask(ctx context.Context) ([]string, error)
	SetWorkflowListWithTask(ctx context.Context, workflows []string, ttl time.Duration) error
	DeleteWorkflowListWithTask(ctx context.Context) error

	// AppListWithTask相关
	GetAppListWithTask(ctx context.Context) ([]string, error)
	SetAppListWithTask(ctx context.Context, apps []string, ttl time.Duration) error
	DeleteAppListWithTask(ctx context.Context) error

	// 获取所有TaskRunCount键
	GetAllTaskRunCountKeys(ctx context.Context) ([]string, error)
}

type TaskDAOImpl struct {
	cmdable redis.Cmdable
}

// NewTaskDAO creates a new TaskDAO instance
func NewTaskDAO(cmdable redis.Cmdable) ITaskDAO {
	return &TaskDAOImpl{
		cmdable: cmdable,
	}
}

func (q *TaskDAOImpl) makeTaskConfigKey(taskID int64) string {
	return fmt.Sprintf("task_config_%d", taskID)
}

func (q *TaskDAOImpl) makeNonFinalTaskListKey() string {
	return "task:list:non_final"
}

func (q *TaskDAOImpl) makeSpaceListWithTaskKey() string {
	return "space:list:with_task"
}
func (q *TaskDAOImpl) makeBotListWithTaskKey() string {
	return "bot:list:with_task"
}
func (q *TaskDAOImpl) makeWorkflowListWithTaskKey() string {
	return "workflow:list:with_task"
}
func (q *TaskDAOImpl) makeAppListWithTaskKey() string {
	return "app:list:with_task"
}

func (q *TaskDAOImpl) makeTaskCountCacheKey(taskID int64) string {
	return fmt.Sprintf("count_%d", taskID)
}
func (q *TaskDAOImpl) makeTaskRunCountCacheKey(taskID, taskRunID int64) string {
	return fmt.Sprintf("count_%d_%d", taskID, taskRunID)
}

// generateFilterHash 生成过滤条件的 hash
func (q *TaskDAOImpl) generateFilterHash(param mysql.ListTaskParam) string {
	if param.TaskFilters == nil {
		return "no_filter"
	}

	// 将过滤条件序列化为 JSON 字符串
	filterBytes, err := json.Marshal(param.TaskFilters)
	if err != nil {
		logs.Error("failed to marshal filter: %v", err)
		return "no_filter"
	}

	// 生成 MD5 hash
	hash := md5.Sum(filterBytes)
	return hex.EncodeToString(hash[:])
}

// GetTask 获取单个任务缓存
func (p *TaskDAOImpl) GetTask(ctx context.Context, id int64) (*entity.ObservabilityTask, error) {
	key := p.makeTaskConfigKey(id)
	got, err := p.cmdable.Get(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil // 缓存未命中
		}
		return nil, errorx.Wrapf(err, "redis get task fail, key: %v", key)
	}
	return convert.NewTaskConverter().ToDO(conv.UnsafeStringToBytes(got))
}

// SetTask 设置单个任务缓存
func (p *TaskDAOImpl) SetTask(ctx context.Context, task *entity.ObservabilityTask, ttl time.Duration) error {
	bytes, err := convert.NewTaskConverter().FromDO(task)
	if err != nil {
		return err
	}
	key := p.makeTaskConfigKey(task.ID)
	if err := p.cmdable.Set(ctx, key, bytes, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set task cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set task key: %v", key)
	}
	return nil
}

// TaskListCache 任务列表缓存结构
type TaskListCache struct {
	Items    []*entity.ObservabilityTask `json:"items"`
	Total    int64                       `json:"total"`
	CachedAt time.Time                   `json:"cached_at"`
}

// GetTaskList 获取任务列表缓存
func (p *TaskDAOImpl) GetTaskList(ctx context.Context, key string) ([]*entity.ObservabilityTask, int64, error) {
	got, err := p.cmdable.Get(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, 0, nil // 缓存未命中
		}
		return nil, 0, errorx.Wrapf(err, "redis get task list fail, key: %v", key)
	}

	var cache TaskListCache
	if err := json.Unmarshal(conv.UnsafeStringToBytes(got), &cache); err != nil {
		return nil, 0, errorx.Wrapf(err, "unmarshal task list cache failed")
	}

	return cache.Items, cache.Total, nil
}

// SetTaskList 设置任务列表缓存
func (p *TaskDAOImpl) SetTaskList(ctx context.Context, key string, tasks []*entity.ObservabilityTask, total int64, ttl time.Duration) error {
	cache := TaskListCache{
		Items:    tasks,
		Total:    total,
		CachedAt: time.Now(),
	}

	bytes, err := json.Marshal(cache)
	if err != nil {
		return errorx.Wrapf(err, "marshal task list cache failed")
	}

	if err := p.cmdable.Set(ctx, key, bytes, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set task list cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set task list key: %v", key)
	}
	return nil
}

// DeleteTaskList 删除任务列表缓存（支持模糊匹配）
func (p *TaskDAOImpl) DeleteTaskList(ctx context.Context, pattern string) error {
	// 由于 redis.Cmdable 接口没有 Keys 方法，这里简化处理
	// 在实际生产环境中，可能需要使用 SCAN 命令或其他方式来实现模糊删除
	logs.CtxWarn(ctx, "DeleteTaskList with pattern not fully implemented", "pattern", pattern)
	return nil
}

// GetNonFinalTaskList 获取非最终状态任务列表缓存
func (p *TaskDAOImpl) GetNonFinalTaskList(ctx context.Context) ([]*entity.ObservabilityTask, error) {
	key := p.makeNonFinalTaskListKey()
	got, err := p.cmdable.Get(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil // 缓存未命中
		}
		return nil, errorx.Wrapf(err, "redis get non final task list fail, key: %v", key)
	}

	var tasks []*entity.ObservabilityTask
	if err := json.Unmarshal(conv.UnsafeStringToBytes(got), &tasks); err != nil {
		return nil, errorx.Wrapf(err, "unmarshal non final task list cache failed")
	}

	return tasks, nil
}

// SetNonFinalTaskList 设置非最终状态任务列表缓存
func (p *TaskDAOImpl) SetNonFinalTaskList(ctx context.Context, tasks []*entity.ObservabilityTask, ttl time.Duration) error {
	key := p.makeNonFinalTaskListKey()

	bytes, err := json.Marshal(tasks)
	if err != nil {
		return errorx.Wrapf(err, "marshal non final task list cache failed")
	}

	if err := p.cmdable.Set(ctx, key, bytes, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set non final task list cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set non final task list key: %v", key)
	}
	return nil
}

// 向非最终状态任务列表中新增任务
func (p *TaskDAOImpl) AddNonFinalTask(ctx context.Context, task *entity.ObservabilityTask) error {
	tasks, err := p.GetNonFinalTaskList(ctx)
	if err != nil {
		logs.CtxError(ctx, "GetNonFinalTaskList failed", "err", err)
		return err
	}
	tasks = append(tasks, task)
	return p.SetNonFinalTaskList(ctx, tasks, time.Hour*24*2)
}

// 向非最终状态任务列表中删除任务
func (p *TaskDAOImpl) RemoveNonFinalTask(ctx context.Context, taskID int64) error {
	tasks, err := p.GetNonFinalTaskList(ctx)
	if err != nil {
		logs.CtxError(ctx, "GetNonFinalTaskList failed", "err", err)
		return err
	}
	for i, task := range tasks {
		if task.ID == taskID {
			tasks = append(tasks[:i], tasks[i+1:]...)
			break
		}
	}
	return p.SetNonFinalTaskList(ctx, tasks, time.Hour*24*2)
}

// DeleteNonFinalTaskList 删除非最终状态任务列表缓存
func (p *TaskDAOImpl) DeleteNonFinalTaskList(ctx context.Context) error {
	key := p.makeNonFinalTaskListKey()
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete non final task list cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete non final task list key: %v", key)
	}
	return nil
}

// GetTaskCount 获取任务计数缓存
func (p *TaskDAOImpl) GetTaskCount(ctx context.Context, taskID int64) (int64, error) {
	key := p.makeTaskCountCacheKey(taskID)
	got, err := p.cmdable.Get(ctx, key).Int64()
	if err != nil {
		if redis.IsNilError(err) {
			return -1, nil // 缓存未命中，返回-1表示未缓存
		}
		return 0, errorx.Wrapf(err, "redis get task count fail, key: %v", key)
	}
	return got, nil
}

// SetTaskCount 设置任务计数缓存
func (p *TaskDAOImpl) SetTaskCount(ctx context.Context, taskID int64, count int64, ttl time.Duration) error {
	key := p.makeTaskCountCacheKey(taskID)
	if err := p.cmdable.Set(ctx, key, count, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set task count cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set task count key: %v", key)
	}
	return nil
}

// DeleteTaskCount 删除任务计数缓存
func (p *TaskDAOImpl) DeleteTaskCount(ctx context.Context, taskID int64) error {
	key := p.makeTaskCountCacheKey(taskID)
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete task count cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete task count key: %v", key)
	}
	return nil
}

// GetTaskRunCount 获取任务运行计数缓存
func (p *TaskDAOImpl) GetTaskRunCount(ctx context.Context, taskID, taskRunID int64) (int64, error) {
	key := p.makeTaskRunCountCacheKey(taskID, taskRunID)
	got, err := p.cmdable.Get(ctx, key).Int64()
	if err != nil {
		if redis.IsNilError(err) {
			return -1, nil // 缓存未命中，返回-1表示未缓存
		}
		return 0, errorx.Wrapf(err, "redis get task count fail, key: %v", key)
	}
	return got, nil
}

// SetTaskRunCount 设置任务运行计数缓存
func (p *TaskDAOImpl) SetTaskRunCount(ctx context.Context, taskID, taskRunID int64, count int64, ttl time.Duration) error {
	key := p.makeTaskRunCountCacheKey(taskID, taskRunID)
	if err := p.cmdable.Set(ctx, key, count, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set task run count cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set task run count key: %v", key)
	}
	return nil
}

// DeleteTaskRunCount 删除任务运行计数缓存
func (p *TaskDAOImpl) DeleteTaskRunCount(ctx context.Context, taskID, taskRunID int64) error {
	key := p.makeTaskRunCountCacheKey(taskID, taskRunID)
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete task run count cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete task run count key: %v", key)
	}
	return nil
}

func (p *TaskDAOImpl) GetObjListWithTask(ctx context.Context) ([]string, []string, error) {
	spaceKey := "spaceList"
	botKey := "botList"
	gotSpaceList, err := p.cmdable.Get(ctx, spaceKey).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil, errorx.Wrapf(err, "redis get fail, key: %v", spaceKey) // 缓存未命中
		}
		return nil, nil, errorx.Wrapf(err, "redis get fail, key: %v", spaceKey)
	}
	var spaceList []string
	if err = json.Unmarshal(conv.UnsafeStringToBytes(gotSpaceList), &spaceList); err != nil {
		return nil, nil, errorx.Wrapf(err, "redis get fail, key: %v", spaceKey)
	}
	gotBotList, err := p.cmdable.Get(ctx, botKey).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil, errorx.Wrapf(err, "redis get fail, key: %v", spaceKey) // 缓存未命中
		}
		return nil, nil, errorx.Wrapf(err, "redis get fail, key: %v", spaceKey)
	}
	var botList []string
	if err = json.Unmarshal(conv.UnsafeStringToBytes(gotBotList), &botList); err != nil {
		return nil, nil, errorx.Wrapf(err, "redis get fail, key: %v", spaceKey)
	}
	return spaceList, botList, nil
}

// GetSpaceListWithTask 获取包含任务的空间列表缓存
func (p *TaskDAOImpl) GetSpaceListWithTask(ctx context.Context) ([]string, error) {
	key := p.makeSpaceListWithTaskKey()
	got, err := p.cmdable.Get(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil // 缓存未命中
		}
		return nil, errorx.Wrapf(err, "redis get space list with task fail, key: %v", key)
	}

	var spaces []string
	if err := json.Unmarshal(conv.UnsafeStringToBytes(got), &spaces); err != nil {
		return nil, errorx.Wrapf(err, "unmarshal space list with task cache failed")
	}

	return spaces, nil
}

// SetSpaceListWithTask 设置包含任务的空间列表缓存
func (p *TaskDAOImpl) SetSpaceListWithTask(ctx context.Context, spaces []string, ttl time.Duration) error {
	key := p.makeSpaceListWithTaskKey()

	bytes, err := json.Marshal(spaces)
	if err != nil {
		return errorx.Wrapf(err, "marshal space list with task cache failed")
	}

	if err := p.cmdable.Set(ctx, key, bytes, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set space list with task cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set space list with task key: %v", key)
	}
	return nil
}

// DeleteSpaceListWithTask 删除包含任务的空间列表缓存
func (p *TaskDAOImpl) DeleteSpaceListWithTask(ctx context.Context) error {
	key := p.makeSpaceListWithTaskKey()
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete space list with task cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete space list with task key: %v", key)
	}
	return nil
}

// GetBotListWithTask 获取包含任务的机器人列表缓存
func (p *TaskDAOImpl) GetBotListWithTask(ctx context.Context) ([]string, error) {
	key := p.makeBotListWithTaskKey()
	got, err := p.cmdable.Get(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil // 缓存未命中
		}
		return nil, errorx.Wrapf(err, "redis get bot list with task fail, key: %v", key)
	}

	var bots []string
	if err := json.Unmarshal(conv.UnsafeStringToBytes(got), &bots); err != nil {
		return nil, errorx.Wrapf(err, "unmarshal bot list with task cache failed")
	}

	return bots, nil
}

// SetBotListWithTask 设置包含任务的机器人列表缓存
func (p *TaskDAOImpl) SetBotListWithTask(ctx context.Context, bots []string, ttl time.Duration) error {
	key := p.makeBotListWithTaskKey()

	bytes, err := json.Marshal(bots)
	if err != nil {
		return errorx.Wrapf(err, "marshal bot list with task cache failed")
	}

	if err := p.cmdable.Set(ctx, key, bytes, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set bot list with task cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set bot list with task key: %v", key)
	}
	return nil
}

// DeleteBotListWithTask 删除包含任务的机器人列表缓存
func (p *TaskDAOImpl) DeleteBotListWithTask(ctx context.Context) error {
	key := p.makeBotListWithTaskKey()
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete bot list with task cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete bot list with task key: %v", key)
	}
	return nil
}

// GetWorkflowListWithTask 获取包含任务的工作流列表缓存
func (p *TaskDAOImpl) GetWorkflowListWithTask(ctx context.Context) ([]string, error) {
	key := p.makeWorkflowListWithTaskKey()
	got, err := p.cmdable.Get(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil // 缓存未命中
		}
		return nil, errorx.Wrapf(err, "redis get workflow list with task fail, key: %v", key)
	}

	var workflows []string
	if err := json.Unmarshal(conv.UnsafeStringToBytes(got), &workflows); err != nil {
		return nil, errorx.Wrapf(err, "unmarshal workflow list with task cache failed")
	}

	return workflows, nil
}

// SetWorkflowListWithTask 设置包含任务的工作流列表缓存
func (p *TaskDAOImpl) SetWorkflowListWithTask(ctx context.Context, workflows []string, ttl time.Duration) error {
	key := p.makeWorkflowListWithTaskKey()

	bytes, err := json.Marshal(workflows)
	if err != nil {
		return errorx.Wrapf(err, "marshal workflow list with task cache failed")
	}

	if err := p.cmdable.Set(ctx, key, bytes, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set workflow list with task cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set workflow list with task key: %v", key)
	}
	return nil
}

// DeleteWorkflowListWithTask 删除包含任务的工作流列表缓存
func (p *TaskDAOImpl) DeleteWorkflowListWithTask(ctx context.Context) error {
	key := p.makeWorkflowListWithTaskKey()
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete workflow list with task cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete workflow list with task key: %v", key)
	}
	return nil
}

// GetAppListWithTask 获取包含任务的应用列表缓存
func (p *TaskDAOImpl) GetAppListWithTask(ctx context.Context) ([]string, error) {
	key := p.makeAppListWithTaskKey()
	got, err := p.cmdable.Get(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil // 缓存未命中
		}
		return nil, errorx.Wrapf(err, "redis get app list with task fail, key: %v", key)
	}

	var apps []string
	if err := json.Unmarshal(conv.UnsafeStringToBytes(got), &apps); err != nil {
		return nil, errorx.Wrapf(err, "unmarshal app list with task cache failed")
	}

	return apps, nil
}

// SetAppListWithTask 设置包含任务的应用列表缓存
func (p *TaskDAOImpl) SetAppListWithTask(ctx context.Context, apps []string, ttl time.Duration) error {
	key := p.makeAppListWithTaskKey()

	bytes, err := json.Marshal(apps)
	if err != nil {
		return errorx.Wrapf(err, "marshal app list with task cache failed")
	}

	if err := p.cmdable.Set(ctx, key, bytes, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set app list with task cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set app list with task key: %v", key)
	}
	return nil
}

// DeleteAppListWithTask 删除包含任务的应用列表缓存
func (p *TaskDAOImpl) DeleteAppListWithTask(ctx context.Context) error {
	key := p.makeAppListWithTaskKey()
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete app list with task cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete app list with task key: %v", key)
	}
	return nil
}

// IncrTaskCount 原子增加任务计数
func (p *TaskDAOImpl) IncrTaskCount(ctx context.Context, taskID int64, ttl time.Duration) (int64, error) {
	key := p.makeTaskCountCacheKey(taskID)
	result, err := p.cmdable.Incr(ctx, key).Result()
	if err != nil {
		logs.CtxError(ctx, "redis incr task count failed", "key", key, "err", err)
		return 0, errorx.Wrapf(err, "redis incr task count key: %v", key)
	}
	
	// 设置TTL
	if err := p.cmdable.Expire(ctx, key, ttl).Err(); err != nil {
		logs.CtxWarn(ctx, "failed to set TTL for task count", "key", key, "err", err)
	}
	
	return result, nil
}

// DecrTaskCount 原子减少任务计数，确保不会变为负数
func (p *TaskDAOImpl) DecrTaskCount(ctx context.Context, taskID int64, ttl time.Duration) (int64, error) {
	key := p.makeTaskCountCacheKey(taskID)
	
	// 先获取当前值
	current, err := p.cmdable.Get(ctx, key).Int64()
	if err != nil {
		if redis.IsNilError(err) {
			// 如果key不存在，返回0
			return 0, nil
		}
		logs.CtxError(ctx, "redis get task count failed before decr", "key", key, "err", err)
		return 0, errorx.Wrapf(err, "redis get task count key: %v", key)
	}
	
	// 如果当前值已经是0或负数，不再减少
	if current <= 0 {
		return 0, nil
	}
	
	// 执行减操作
	result, err := p.cmdable.Decr(ctx, key).Result()
	if err != nil {
		logs.CtxError(ctx, "redis decr task count failed", "key", key, "err", err)
		return 0, errorx.Wrapf(err, "redis decr task count key: %v", key)
	}
	
	// 如果减少后变为负数，重置为0
	if result < 0 {
		if err := p.cmdable.Set(ctx, key, 0, ttl).Err(); err != nil {
			logs.CtxError(ctx, "failed to reset negative task count", "key", key, "err", err)
		}
		return 0, nil
	}
	
	// 设置TTL
	if err := p.cmdable.Expire(ctx, key, ttl).Err(); err != nil {
		logs.CtxWarn(ctx, "failed to set TTL for task count", "key", key, "err", err)
	}
	
	return result, nil
}

// IncrTaskRunCount 原子增加任务运行计数
func (p *TaskDAOImpl) IncrTaskRunCount(ctx context.Context, taskID, taskRunID int64, ttl time.Duration) (int64, error) {
	key := p.makeTaskRunCountCacheKey(taskID, taskRunID)
	result, err := p.cmdable.Incr(ctx, key).Result()
	if err != nil {
		logs.CtxError(ctx, "redis incr task run count failed", "key", key, "err", err)
		return 0, errorx.Wrapf(err, "redis incr task run count key: %v", key)
	}
	
	// 设置TTL
	if err := p.cmdable.Expire(ctx, key, ttl).Err(); err != nil {
		logs.CtxWarn(ctx, "failed to set TTL for task run count", "key", key, "err", err)
	}
	
	return result, nil
}

// DecrTaskRunCount 原子减少任务运行计数，确保不会变为负数
func (p *TaskDAOImpl) DecrTaskRunCount(ctx context.Context, taskID, taskRunID int64, ttl time.Duration) (int64, error) {
	key := p.makeTaskRunCountCacheKey(taskID, taskRunID)
	
	// 先获取当前值
	current, err := p.cmdable.Get(ctx, key).Int64()
	if err != nil {
		if redis.IsNilError(err) {
			// 如果key不存在，返回0
			return 0, nil
		}
		logs.CtxError(ctx, "redis get task run count failed before decr", "key", key, "err", err)
		return 0, errorx.Wrapf(err, "redis get task run count key: %v", key)
	}
	
	// 如果当前值已经是0或负数，不再减少
	if current <= 0 {
		return 0, nil
	}
	
	// 执行减操作
	result, err := p.cmdable.Decr(ctx, key).Result()
	if err != nil {
		logs.CtxError(ctx, "redis decr task run count failed", "key", key, "err", err)
		return 0, errorx.Wrapf(err, "redis decr task run count key: %v", key)
	}
	
	// 如果减少后变为负数，重置为0
	if result < 0 {
		if err := p.cmdable.Set(ctx, key, 0, ttl).Err(); err != nil {
			logs.CtxError(ctx, "failed to reset negative task run count", "key", key, "err", err)
		}
		return 0, nil
	}
	
	// 设置TTL
	if err := p.cmdable.Expire(ctx, key, ttl).Err(); err != nil {
		logs.CtxWarn(ctx, "failed to set TTL for task run count", "key", key, "err", err)
	}
	
	return result, nil
}

// GetAllTaskRunCountKeys 获取所有TaskRunCount键
func (p *TaskDAOImpl) GetAllTaskRunCountKeys(ctx context.Context) ([]string, error) {
	pattern := "count_*_*" // 匹配 count_{taskID}_{taskRunID} 格式的键
	var allKeys []string
	
	// 使用SCAN命令遍历匹配的键
	iter := p.cmdable.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		// 验证键格式是否为TaskRunCount键（包含两个下划线，表示taskID和taskRunID）
		if p.isTaskRunCountKey(key) {
			allKeys = append(allKeys, key)
		}
	}
	
	if err := iter.Err(); err != nil {
		logs.CtxError(ctx, "scan task run count keys failed", "pattern", pattern, "err", err)
		return nil, errorx.Wrapf(err, "scan task run count keys with pattern: %s", pattern)
	}
	
	logs.CtxInfo(ctx, "found %d task run count keys", len(allKeys))
	return allKeys, nil
}

// isTaskRunCountKey 检查键是否为TaskRunCount键格式
func (p *TaskDAOImpl) isTaskRunCountKey(key string) bool {
	// TaskRunCount键格式: count_{taskID}_{taskRunID}
	// TaskCount键格式: count_{taskID}
	// 通过下划线数量来区分
	underscoreCount := 0
	for _, char := range key {
		if char == '_' {
			underscoreCount++
		}
	}
	// TaskRunCount键应该有2个下划线
	return underscoreCount == 2
}