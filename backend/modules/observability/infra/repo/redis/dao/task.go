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
	// 原有方法
	MSetTaskConfig(ctx context.Context, taskConfig *entity.ObservabilityTask) error
	MGetTaskConfig(ctx context.Context, taskID int64) (taskConfig *entity.ObservabilityTask, err error)
	MGetTaskCount(ctx context.Context, taskID int64) (count int64, err error)
	MGetTaskRunCount(ctx context.Context, taskID, runID int64) (count int64, err error)
	MIncrTaskCount(ctx context.Context, taskID int64, count int64) error
	MDecrTaskCount(ctx context.Context, taskID int64, count int64) error
	MIncrTaskRunCount(ctx context.Context, taskID, runID int64, count int64) error
	MDecrTaskRunCount(ctx context.Context, taskID, runID int64, count int64) error

	// 新增 CRUD 缓存方法
	GetTask(ctx context.Context, id int64) (*entity.ObservabilityTask, error)
	SetTask(ctx context.Context, task *entity.ObservabilityTask, ttl time.Duration) error
	DeleteTask(ctx context.Context, id int64) error

	GetTaskList(ctx context.Context, key string) ([]*entity.ObservabilityTask, int64, error)
	SetTaskList(ctx context.Context, key string, tasks []*entity.ObservabilityTask, total int64, ttl time.Duration) error
	DeleteTaskList(ctx context.Context, pattern string) error

	GetNonFinalTaskList(ctx context.Context) ([]*entity.ObservabilityTask, error)
	SetNonFinalTaskList(ctx context.Context, tasks []*entity.ObservabilityTask, ttl time.Duration) error
	DeleteNonFinalTaskList(ctx context.Context) error

	GetTaskCount(ctx context.Context, taskID int64) (int64, error)
	SetTaskCount(ctx context.Context, taskID int64, count int64, ttl time.Duration) error
	DeleteTaskCount(ctx context.Context, taskID int64) error

	GetTaskRunCount(ctx context.Context, taskID, taskRunID int64) (int64, error)
	GetObjListWithTask(ctx context.Context) ([]string, []string, error)
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

// 原有 key 生成方法
func (q *TaskDAOImpl) makeTaskConfigKey(taskID int64) string {
	return fmt.Sprintf("task_config_%d", taskID)
}

func (q *TaskDAOImpl) makeTaskCountKey(taskID int64) string {
	return fmt.Sprintf("count_%d", taskID)
}

func (q *TaskDAOImpl) makeTaskRunCountKey(taskID, runID int64) string {
	return fmt.Sprintf("count_%d_%d", taskID, runID)
}

// 新增 key 生成方法
func (q *TaskDAOImpl) makeTaskDetailKey(id int64) string {
	return fmt.Sprintf("task:detail:%d", id)
}

func (q *TaskDAOImpl) makeTaskListKey(workspaceID int64, filterHash string, page, size int32) string {
	return fmt.Sprintf("task:list:%d:%s:%d:%d", workspaceID, filterHash, page, size)
}

func (q *TaskDAOImpl) makeNonFinalTaskListKey() string {
	return "task:list:non_final"
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

func (p *TaskDAOImpl) MSetTaskConfig(ctx context.Context, taskConfig *entity.ObservabilityTask) error {
	bytes, err := convert.NewTaskConverter().FromDO(taskConfig)
	if err != nil {
		return err
	}
	key := p.makeTaskConfigKey(taskConfig.ID)
	if err := p.cmdable.Set(ctx, key, bytes, time.Hour*24*2).Err(); err != nil {
		return errorx.Wrapf(err, "redis set key: %v", key)
	}
	return nil
}

func (p *TaskDAOImpl) MGetTaskConfig(ctx context.Context, taskID int64) (taskConfig *entity.ObservabilityTask, err error) {
	key := p.makeTaskConfigKey(taskID)
	got, err := p.cmdable.Get(ctx, key).Result()
	if err != nil && !redis.IsNilError(err) {
		return nil, errorx.Wrapf(err, "redis get fail, key: %v", key)
	}
	return convert.NewTaskConverter().ToDO(conv.UnsafeStringToBytes(got))
}

func (p *TaskDAOImpl) MGetTaskCount(ctx context.Context, taskID int64) (count int64, err error) {
	key := p.makeTaskCountKey(taskID)
	got, err := p.cmdable.Get(ctx, key).Int64()
	if err != nil && !redis.IsNilError(err) {
		return 0, errorx.Wrapf(err, "redis get fail, key: %v", key)
	}
	return got, nil
}

func (p *TaskDAOImpl) MGetTaskRunCount(ctx context.Context, taskID, runID int64) (count int64, err error) {
	key := p.makeTaskRunCountKey(taskID, runID)
	got, err := p.cmdable.Get(ctx, key).Int64()
	if err != nil && !redis.IsNilError(err) {
		return 0, errorx.Wrapf(err, "redis get fail, key: %v", key)
	}
	return got, nil
}

func (p *TaskDAOImpl) MIncrTaskCount(ctx context.Context, taskID int64, count int64) error {
	key := p.makeTaskCountKey(taskID)
	if err := p.cmdable.IncrBy(ctx, key, count).Err(); err != nil {
		return errorx.Wrapf(err, "redis incr key: %v", key)
	}
	return nil
}
func (p *TaskDAOImpl) MDecrTaskCount(ctx context.Context, taskID int64, count int64) error {
	key := p.makeTaskCountKey(taskID)
	if err := p.cmdable.DecrBy(ctx, key, count).Err(); err != nil {
		return errorx.Wrapf(err, "redis decr key: %v", key)
	}
	return nil
}

func (p *TaskDAOImpl) MIncrTaskRunCount(ctx context.Context, taskID, runID int64, count int64) error {
	key := p.makeTaskRunCountKey(taskID, runID)
	if err := p.cmdable.IncrBy(ctx, key, count).Err(); err != nil {
		return errorx.Wrapf(err, "redis incr key: %v", key)
	}
	return nil
}
func (p *TaskDAOImpl) MDecrTaskRunCount(ctx context.Context, taskID, runID int64, count int64) error {
	key := p.makeTaskRunCountKey(taskID, runID)
	if err := p.cmdable.DecrBy(ctx, key, count).Err(); err != nil {
		return errorx.Wrapf(err, "redis decr key: %v", key)
	}
	return nil
}

// 新增 CRUD 缓存方法实现

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

// DeleteTask 删除单个任务缓存
func (p *TaskDAOImpl) DeleteTask(ctx context.Context, id int64) error {
	key := p.makeTaskDetailKey(id)
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete task cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete task key: %v", key)
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
func (p *TaskDAOImpl) SetTaskCount(ctx context.Context, workspaceID int64, count int64, ttl time.Duration) error {
	key := p.makeTaskCountCacheKey(workspaceID)
	if err := p.cmdable.Set(ctx, key, count, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set task count cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set task count key: %v", key)
	}
	return nil
}

// DeleteTaskCount 删除任务计数缓存
func (p *TaskDAOImpl) DeleteTaskCount(ctx context.Context, workspaceID int64) error {
	key := p.makeTaskCountCacheKey(workspaceID)
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete task count cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete task count key: %v", key)
	}
	return nil
}

// GetTaskCount 获取任务计数缓存
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
