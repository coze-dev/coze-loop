// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate mockgen -destination=mocks/Task_dao.go -package=mocks . ITaskDAO
type ITaskDAO interface {
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

// NewTaskDAO creates a new TaskDAO instance
func NewTaskDAO(cmdable redis.Cmdable) ITaskDAO {
	return &TaskDAOImpl{
		cmdable: cmdable,
	}
}

func (q *TaskDAOImpl) makeTaskCountCacheKey(taskID int64) string {
	return fmt.Sprintf("count_%d", taskID)
}

func (q *TaskDAOImpl) makeTaskRunCountCacheKey(taskID, taskRunID int64) string {
	return fmt.Sprintf("count_%d_%d", taskID, taskRunID)
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
