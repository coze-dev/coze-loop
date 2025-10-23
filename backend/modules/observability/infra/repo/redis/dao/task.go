// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	redisconvert "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/redis/convert"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
)

//go:generate mockgen -destination=mocks/Task_dao.go -package=mocks . ITaskDAO
type ITaskDAO interface {
	// Task相关
	GetTask(ctx context.Context, taskID int64) (*entity.ObservabilityTask, error)
	SetTask(ctx context.Context, task *entity.ObservabilityTask) error
	// 非终态task列表by spaceID
	// ListNonFinalTask lists all non-final tasks in the given space.
	ListNonFinalTask(ctx context.Context, spaceID string) ([]int64, error)
	AddNonFinalTask(ctx context.Context, spaceID string, taskID int64) error
	RemoveNonFinalTask(ctx context.Context, spaceID string, taskID int64) error

	// TaskCount相关
	GetTaskCount(ctx context.Context, taskID int64) (int64, error)
	IncrTaskCount(ctx context.Context, taskID int64, ttl time.Duration) (int64, error)
	DecrTaskCount(ctx context.Context, taskID int64, ttl time.Duration) (int64, error)

	// TaskRunCount相关
	GetTaskRunCount(ctx context.Context, taskID, taskRunID int64) (int64, error)
	IncrTaskRunCount(ctx context.Context, taskID, taskRunID int64, ttl time.Duration) (int64, error)
	DecrTaskRunCount(ctx context.Context, taskID, taskRunID int64, ttl time.Duration) (int64, error)
}

type TaskDAOImpl struct {
	cmdable redis.Cmdable
}

var taskConverter = redisconvert.NewTaskConverter()

const (
	taskDetailCacheKeyPattern = "observability:task:%d"
	taskDetailCacheTTL        = 1 * time.Minute
)

// NewTaskDAO creates a new TaskDAO instance
func NewTaskDAO(cmdable redis.Cmdable) ITaskDAO {
	return &TaskDAOImpl{
		cmdable: cmdable,
	}
}

func (q *TaskDAOImpl) makeTaskCacheKey(taskID int64) string {
	return fmt.Sprintf(taskDetailCacheKeyPattern, taskID)
}

func (q *TaskDAOImpl) makeTaskCountCacheKey(taskID int64) string {
	return fmt.Sprintf("count_%d", taskID)
}

func (q *TaskDAOImpl) makeTaskRunCountCacheKey(taskID, taskRunID int64) string {
	return fmt.Sprintf("count_%d_%d", taskID, taskRunID)
}

func (q *TaskDAOImpl) makeNonFinalTaskCacheKey(spaceID string) string {
	return fmt.Sprintf("tasks_of_%s", spaceID)
}

// GetTask 从缓存中获取任务详情
func (p *TaskDAOImpl) GetTask(ctx context.Context, taskID int64) (*entity.ObservabilityTask, error) {
	key := p.makeTaskCacheKey(taskID)
	bytes, err := p.cmdable.Get(ctx, key).Bytes()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil
		}
		logs.CtxError(ctx, "redis get task failed", "key", key, "err", err)
		return nil, errorx.Wrapf(err, "redis get task fail, key: %v", key)
	}
	if len(bytes) == 0 {
		return nil, nil
	}
	task, err := taskConverter.ToDO(bytes)
	if err != nil {
		logs.CtxError(ctx, "convert task bytes to entity failed", "key", key, "err", err)
		return nil, err
	}
	return task, nil
}

// SetTask 将任务详情写入缓存
func (p *TaskDAOImpl) SetTask(ctx context.Context, task *entity.ObservabilityTask) error {
	if task == nil {
		return nil
	}
	if task.ID == 0 {
		logs.CtxWarn(ctx, "skip caching task with empty id")
		return nil
	}
	key := p.makeTaskCacheKey(task.ID)
	bytes, err := taskConverter.FromDO(task)
	if err != nil {
		logs.CtxError(ctx, "convert task entity to bytes failed", "key", key, "err", err)
		return err
	}
	if err := p.cmdable.Set(ctx, key, bytes, taskDetailCacheTTL).Err(); err != nil {
		logs.CtxError(ctx, "redis set task failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set task fail, key: %v", key)
	}
	return nil
}

// ListNonFinalTask 获取非终态任务ID列表
func (p *TaskDAOImpl) ListNonFinalTask(ctx context.Context, spaceID string) ([]int64, error) {
	key := p.makeNonFinalTaskCacheKey(spaceID)
	bytes, err := p.cmdable.Get(ctx, key).Bytes()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil
		}
		logs.CtxError(ctx, "redis get task failed", "key", key, "err", err)
		return nil, errorx.Wrapf(err, "redis get task fail, key: %v", key)
	}
	if len(bytes) == 0 {
		return nil, nil
	}
	var tasks []int64
	if err := lo.TernaryF(
		len(bytes) > 0,
		func() error { return json.Unmarshal(bytes, &tasks) },
		func() error { return nil },
	); err != nil {
		return nil, errorx.Wrapf(err, "TaskExpt json unmarshal failed")
	}

	return tasks, nil
}

// AddNonFinalTask 将任务加入非终态列表
func (p *TaskDAOImpl) AddNonFinalTask(ctx context.Context, spaceID string, taskID int64) error {
	if taskID == 0 {
		logs.CtxWarn(ctx, "skip adding non final task with empty id")
		return nil
	}

	key := p.makeNonFinalTaskCacheKey(spaceID)
	bytes, err := p.cmdable.Get(ctx, key).Bytes()
	var tasks []int64

	if err != nil && !redis.IsNilError(err) {
		logs.CtxError(ctx, "redis get task failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis get task fail, key: %v", key)
	}

	if len(bytes) > 0 {
		if err := json.Unmarshal(bytes, &tasks); err != nil {
			return errorx.Wrapf(err, "TaskExpt json unmarshal failed")
		}
	}

	if !lo.Contains(tasks, taskID) {
		tasks = append(tasks, taskID)
	}

	bytes, err = json.Marshal(tasks)
	if err != nil {
		return errorx.Wrapf(err, "TaskExpt json marshal failed")
	}

	if err := p.cmdable.Set(ctx, key, bytes, 0).Err(); err != nil {
		logs.CtxError(ctx, "redis set task failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set task fail, key: %v", key)
	}

	return nil
}

// RemoveNonFinalTask 将任务从非终态列表移除
func (p *TaskDAOImpl) RemoveNonFinalTask(ctx context.Context, spaceID string, taskID int64) error {
	if taskID == 0 {
		logs.CtxWarn(ctx, "skip removing non final task with empty id")
		return nil
	}

	key := p.makeNonFinalTaskCacheKey(spaceID)
	bytes, err := p.cmdable.Get(ctx, key).Bytes()
	if err != nil {
		if redis.IsNilError(err) {
			return nil
		}
		logs.CtxError(ctx, "redis get task failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis get task fail, key: %v", key)
	}
	if len(bytes) == 0 {
		return nil
	}
	var tasks []int64
	if err := lo.TernaryF(
		len(bytes) > 0,
		func() error { return json.Unmarshal(bytes, &tasks) },
		func() error { return nil },
	); err != nil {
		return errorx.Wrapf(err, "TaskExpt json unmarshal failed")
	}
	tasks = lo.Filter(tasks, func(item int64, _ int) bool { return item != taskID })
	bytes, err = json.Marshal(tasks)
	if err != nil {
		return errorx.Wrapf(err, "TaskExpt json marshal failed")
	}
	if err := p.cmdable.Set(ctx, key, bytes, 0).Err(); err != nil {
		logs.CtxError(ctx, "redis set task failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set task fail, key: %v", key)
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

// IncrTaskCount 原子增加任务计数
func (p *TaskDAOImpl) IncrTaskCount(ctx context.Context, taskID int64, ttl time.Duration) (int64, error) {
	key := p.makeTaskCountCacheKey(taskID)
	result, err := p.cmdable.Incr(ctx, key).Result()
	logs.CtxInfo(ctx, "redis incr task count success, taskID: %v, key: %v, result: %v", taskID, key, result)
	if err != nil {
		logs.CtxError(ctx, "redis incr task count failed", "key", key, "err", err)
		return 0, errorx.Wrapf(err, "redis incr task count key: %v", key)
	}

	// 设置TTL
	if err = p.cmdable.Expire(ctx, key, ttl).Err(); err != nil {
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
	logs.CtxInfo(ctx, "redis decr task count success, taskID: %v, key: %v, result: %v", taskID, key, result)
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
	logs.CtxInfo(ctx, "redis incr task run count success, taskID: %v,taskRunID: %v, key: %v, result: %v", taskID, taskRunID, key, result)
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
