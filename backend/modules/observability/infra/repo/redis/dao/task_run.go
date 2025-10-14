// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate mockgen -destination=mocks/task_run_dao.go -package=mocks . ITaskRunDAO
type ITaskRunDAO interface {
	// 成功/失败计数操作
	IncrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) error
	DecrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) error
	IncrTaskRunFailCount(ctx context.Context, taskID, taskRunID int64) error
	GetTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) (int64, error)
	GetTaskRunFailCount(ctx context.Context, taskID, taskRunID int64) (int64, error)
}

type TaskRunDAOImpl struct {
	cmdable redis.Cmdable
}

// NewTaskRunDAO creates a new TaskRunDAO instance
func NewTaskRunDAO(cmdable redis.Cmdable) ITaskRunDAO {
	return &TaskRunDAOImpl{
		cmdable: cmdable,
	}
}

// 缓存键生成方法
func (q *TaskRunDAOImpl) makeTaskRunSuccessCountKey(taskID, taskRunID int64) string {
	return fmt.Sprintf("taskrun:success_count:%d:%d", taskID, taskRunID)
}

func (q *TaskRunDAOImpl) makeTaskRunFailCountKey(taskID, taskRunID int64) string {
	return fmt.Sprintf("taskrun:fail_count:%d:%d", taskID, taskRunID)
}

// IncrTaskRunSuccessCount 增加成功计数
func (p *TaskRunDAOImpl) IncrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) error {
	key := p.makeTaskRunSuccessCountKey(taskID, taskRunID)
	if err := p.cmdable.Incr(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis incr taskrun success count failed, key:%v, err:%v", key, err)
		return errorx.Wrapf(err, "redis incr taskrun success count key: %v", key)
	}
	return nil
}

// IncrTaskRunFailCount 增加失败计数
func (p *TaskRunDAOImpl) IncrTaskRunFailCount(ctx context.Context, taskID, taskRunID int64) error {
	key := p.makeTaskRunFailCountKey(taskID, taskRunID)
	if err := p.cmdable.Incr(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis incr taskrun fail count failed, key:", "key", key, "err", err)
		return errorx.Wrapf(err, "redis incr taskrun fail count key: %v", key)
	}
	return nil
}

// GetTaskRunSuccessCount 获取成功计数
func (p *TaskRunDAOImpl) GetTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) (int64, error) {
	key := p.makeTaskRunSuccessCountKey(taskID, taskRunID)
	got, err := p.cmdable.Get(ctx, key).Int64()
	if err != nil {
		if redis.IsNilError(err) {
			return 0, nil // 缓存未命中，返回0
		}
		return 0, errorx.Wrapf(err, "redis get taskrun success count fail, key: %v", key)
	}
	return got, nil
}

// GetTaskRunFailCount 获取失败计数
func (p *TaskRunDAOImpl) GetTaskRunFailCount(ctx context.Context, taskID, taskRunID int64) (int64, error) {
	key := p.makeTaskRunFailCountKey(taskID, taskRunID)
	got, err := p.cmdable.Get(ctx, key).Int64()
	if err != nil {
		if redis.IsNilError(err) {
			return 0, nil // 缓存未命中，返回0
		}
		return 0, errorx.Wrapf(err, "redis get taskrun fail count fail, key: %v", key)
	}
	return got, nil
}
func (p *TaskRunDAOImpl) DecrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) error {
	key := p.makeTaskRunSuccessCountKey(taskID, taskRunID)
	if err := p.cmdable.Decr(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis decr taskrun success count failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis decr taskrun success count key: %v", key)
	}
	return nil
}
