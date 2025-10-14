// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// TTL策略常量
const (
	TaskTTL        = 30 * time.Minute // 单个任务/任务运行记录：30分钟
	TaskListTTL    = 10 * time.Minute // 任务列表：10分钟
	TaskRunTTL     = 30 * time.Minute // 单个任务运行记录：30分钟
	TaskRunListTTL = 5 * time.Minute  // 任务运行记录列表：5分钟
)

// CacheManager 缓存管理器接口
type CacheManager interface {
	// 任务缓存操作
	GetTask(ctx context.Context, id int64) (*model.ObservabilityTask, error)
	SetTask(ctx context.Context, task *model.ObservabilityTask) error
	DeleteTask(ctx context.Context, id int64) error

	// 任务列表缓存操作
	GetTaskList(ctx context.Context, key string) ([]*model.ObservabilityTask, error)
	SetTaskList(ctx context.Context, key string, tasks []*model.ObservabilityTask) error
	DeleteTaskListByPattern(ctx context.Context, pattern string) error

	// 任务运行记录缓存操作
	GetTaskRun(ctx context.Context, id int64) (*model.ObservabilityTaskRun, error)
	SetTaskRun(ctx context.Context, taskRun *model.ObservabilityTaskRun) error
	DeleteTaskRun(ctx context.Context, id int64) error

	// 任务运行记录列表缓存操作
	GetTaskRunList(ctx context.Context, key string) ([]*model.ObservabilityTaskRun, error)
	SetTaskRunList(ctx context.Context, key string, taskRuns []*model.ObservabilityTaskRun) error
	DeleteTaskRunListByPattern(ctx context.Context, pattern string) error
}

// cacheManagerImpl 缓存管理器实现
type cacheManagerImpl struct {
	redisClient  RedisClient
	keyGenerator KeyGenerator
}

// NewCacheManager 创建缓存管理器
func NewCacheManager(redisClient RedisClient, keyGenerator KeyGenerator) CacheManager {
	return &cacheManagerImpl{
		redisClient:  redisClient,
		keyGenerator: keyGenerator,
	}
}

// GetTask 获取任务缓存
func (c *cacheManagerImpl) GetTask(ctx context.Context, id int64) (*model.ObservabilityTask, error) {
	key := c.keyGenerator.TaskKey(id)
	data, err := c.redisClient.Get(ctx, key)
	if err != nil {
		logs.CtxWarn(ctx, "Failed to get task from cache: %v", err)
		return nil, err
	}

	if data == "" {
		return nil, nil // 缓存未命中
	}

	var task model.ObservabilityTask
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		logs.CtxWarn(ctx, "Failed to unmarshal task from cache: %v", err)
		return nil, err
	}

	return &task, nil
}

// SetTask 设置任务缓存
func (c *cacheManagerImpl) SetTask(ctx context.Context, task *model.ObservabilityTask) error {
	key := c.keyGenerator.TaskKey(task.ID)
	data, err := json.Marshal(task)
	if err != nil {
		logs.CtxWarn(ctx, "Failed to marshal task for cache: %v", err)
		return err
	}

	if err := c.redisClient.Set(ctx, key, string(data), TaskTTL); err != nil {
		logs.CtxWarn(ctx, "Failed to set task cache: %v", err)
		return err
	}

	return nil
}

// DeleteTask 删除任务缓存
func (c *cacheManagerImpl) DeleteTask(ctx context.Context, id int64) error {
	key := c.keyGenerator.TaskKey(id)
	if err := c.redisClient.Del(ctx, key); err != nil {
		logs.CtxWarn(ctx, "Failed to delete task cache: %v", err)
		return err
	}
	return nil
}

// GetTaskList 获取任务列表缓存
func (c *cacheManagerImpl) GetTaskList(ctx context.Context, key string) ([]*model.ObservabilityTask, error) {
	data, err := c.redisClient.Get(ctx, key)
	if err != nil {
		logs.CtxWarn(ctx, "Failed to get task list from cache: %v", err)
		return nil, err
	}

	if data == "" {
		return nil, nil // 缓存未命中
	}

	var tasks []*model.ObservabilityTask
	if err := json.Unmarshal([]byte(data), &tasks); err != nil {
		logs.CtxWarn(ctx, "Failed to unmarshal task list from cache: %v", err)
		return nil, err
	}

	return tasks, nil
}

// SetTaskList 设置任务列表缓存
func (c *cacheManagerImpl) SetTaskList(ctx context.Context, key string, tasks []*model.ObservabilityTask) error {
	data, err := json.Marshal(tasks)
	if err != nil {
		logs.CtxWarn(ctx, "Failed to marshal task list for cache: %v", err)
		return err
	}

	if err := c.redisClient.Set(ctx, key, string(data), TaskListTTL); err != nil {
		logs.CtxWarn(ctx, "Failed to set task list cache: %v", err)
		return err
	}

	return nil
}

// DeleteTaskListByPattern 按模式删除任务列表缓存
func (c *cacheManagerImpl) DeleteTaskListByPattern(ctx context.Context, pattern string) error {
	// TODO: 实现按模式删除缓存的逻辑
	// 这里需要根据具体的Redis客户端实现来扫描和删除匹配的key
	logs.CtxInfo(ctx, "Deleting task list cache by pattern: %s", pattern)
	return nil
}

// GetTaskRun 获取任务运行记录缓存
func (c *cacheManagerImpl) GetTaskRun(ctx context.Context, id int64) (*model.ObservabilityTaskRun, error) {
	key := c.keyGenerator.TaskRunKey(id)
	data, err := c.redisClient.Get(ctx, key)
	if err != nil {
		logs.CtxWarn(ctx, "Failed to get task run from cache: %v", err)
		return nil, err
	}

	if data == "" {
		return nil, nil // 缓存未命中
	}

	var taskRun model.ObservabilityTaskRun
	if err := json.Unmarshal([]byte(data), &taskRun); err != nil {
		logs.CtxWarn(ctx, "Failed to unmarshal task run from cache: %v", err)
		return nil, err
	}

	return &taskRun, nil
}

// SetTaskRun 设置任务运行记录缓存
func (c *cacheManagerImpl) SetTaskRun(ctx context.Context, taskRun *model.ObservabilityTaskRun) error {
	key := c.keyGenerator.TaskRunKey(taskRun.ID)
	data, err := json.Marshal(taskRun)
	if err != nil {
		logs.CtxWarn(ctx, "Failed to marshal task run for cache: %v", err)
		return err
	}

	if err := c.redisClient.Set(ctx, key, string(data), TaskRunTTL); err != nil {
		logs.CtxWarn(ctx, "Failed to set task run cache: %v", err)
		return err
	}

	return nil
}

// DeleteTaskRun 删除任务运行记录缓存
func (c *cacheManagerImpl) DeleteTaskRun(ctx context.Context, id int64) error {
	key := c.keyGenerator.TaskRunKey(id)
	if err := c.redisClient.Del(ctx, key); err != nil {
		logs.CtxWarn(ctx, "Failed to delete task run cache: %v", err)
		return err
	}
	return nil
}

// GetTaskRunList 获取任务运行记录列表缓存
func (c *cacheManagerImpl) GetTaskRunList(ctx context.Context, key string) ([]*model.ObservabilityTaskRun, error) {
	data, err := c.redisClient.Get(ctx, key)
	if err != nil {
		logs.CtxWarn(ctx, "Failed to get task run list from cache: %v", err)
		return nil, err
	}

	if data == "" {
		return nil, nil // 缓存未命中
	}

	var taskRuns []*model.ObservabilityTaskRun
	if err := json.Unmarshal([]byte(data), &taskRuns); err != nil {
		logs.CtxWarn(ctx, "Failed to unmarshal task run list from cache: %v", err)
		return nil, err
	}

	return taskRuns, nil
}

// SetTaskRunList 设置任务运行记录列表缓存
func (c *cacheManagerImpl) SetTaskRunList(ctx context.Context, key string, taskRuns []*model.ObservabilityTaskRun) error {
	data, err := json.Marshal(taskRuns)
	if err != nil {
		logs.CtxWarn(ctx, "Failed to marshal task run list for cache: %v", err)
		return err
	}

	if err := c.redisClient.Set(ctx, key, string(data), TaskRunListTTL); err != nil {
		logs.CtxWarn(ctx, "Failed to set task run list cache: %v", err)
		return err
	}

	return nil
}

// DeleteTaskRunListByPattern 按模式删除任务运行记录列表缓存
func (c *cacheManagerImpl) DeleteTaskRunListByPattern(ctx context.Context, pattern string) error {
	// TODO: 实现按模式删除缓存的逻辑
	// 这里需要根据具体的Redis客户端实现来扫描和删除匹配的key
	logs.CtxInfo(ctx, "Deleting task run list cache by pattern: %s", pattern)
	return nil
}
