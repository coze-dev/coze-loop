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

func (q *TaskDAOImpl) makeTaskConfigKey(taskID int64) string {
	return fmt.Sprintf("task_config_%d", taskID)
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

// DeleteTaskList 删除任务列表缓存（支持模糊匹配）
func (p *TaskDAOImpl) DeleteTaskList(ctx context.Context, pattern string) error {
	// 由于 redis.Cmdable 接口没有 Keys 方法，这里简化处理
	// 在实际生产环境中，可能需要使用 SCAN 命令或其他方式来实现模糊删除
	logs.CtxWarn(ctx, "DeleteTaskList with pattern not fully implemented", "pattern", pattern)
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
