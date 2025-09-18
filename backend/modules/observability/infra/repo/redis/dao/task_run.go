// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/redis/convert"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate mockgen -destination=mocks/task_run_dao.go -package=mocks . ITaskRunDAO
type ITaskRunDAO interface {
	// 基础缓存操作
	GetTaskRun(ctx context.Context, id int64) (*entity.TaskRun, error)
	SetTaskRun(ctx context.Context, taskRun *entity.TaskRun, ttl time.Duration) error
	DeleteTaskRun(ctx context.Context, id int64) error

	// 列表缓存操作
	GetNonFinalTaskRunList(ctx context.Context) ([]*entity.TaskRun, error)
	SetNonFinalTaskRunList(ctx context.Context, taskRuns []*entity.TaskRun, ttl time.Duration) error
	DeleteNonFinalTaskRunList(ctx context.Context) error

	GetTaskRunListByTask(ctx context.Context, taskID int64) ([]*entity.TaskRun, error)
	SetTaskRunListByTask(ctx context.Context, taskID int64, taskRuns []*entity.TaskRun, ttl time.Duration) error
	DeleteTaskRunListByTask(ctx context.Context, taskID int64) error

	// 计数缓存操作
	GetTaskRunCount(ctx context.Context, taskID, taskRunID int64) (int64, error)
	SetTaskRunCount(ctx context.Context, taskID, taskRunID int64, count int64, ttl time.Duration) error
	DeleteTaskRunCount(ctx context.Context, taskID, taskRunID int64) error

	// 成功/失败计数操作
	IncrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) error
	DecrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) error
	IncrTaskRunFailCount(ctx context.Context, taskID, taskRunID int64) error
	GetTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) (int64, error)
	GetTaskRunFailCount(ctx context.Context, taskID, taskRunID int64) (int64, error)

	// 对象列表缓存操作
	GetObjListWithTaskRun(ctx context.Context) ([]string, []string, error)
	SetObjListWithTaskRun(ctx context.Context, spaceList, botList []string, ttl time.Duration) error
	DeleteObjListWithTaskRun(ctx context.Context) error
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
func (q *TaskRunDAOImpl) makeTaskRunDetailKey(id int64) string {
	return fmt.Sprintf("taskrun:detail:%d", id)
}

func (q *TaskRunDAOImpl) makeNonFinalTaskRunListKey() string {
	return "taskrun:list:non_final"
}

func (q *TaskRunDAOImpl) makeTaskRunListByTaskKey(taskID int64) string {
	return fmt.Sprintf("taskrun:list:task:%d", taskID)
}

func (q *TaskRunDAOImpl) makeTaskRunCountKey(taskID, taskRunID int64) string {
	return fmt.Sprintf("taskrun:count:%d:%d", taskID, taskRunID)
}

func (q *TaskRunDAOImpl) makeTaskRunSuccessCountKey(taskID, taskRunID int64) string {
	return fmt.Sprintf("taskrun:success_count:%d:%d", taskID, taskRunID)
}

func (q *TaskRunDAOImpl) makeTaskRunFailCountKey(taskID, taskRunID int64) string {
	return fmt.Sprintf("taskrun:fail_count:%d:%d", taskID, taskRunID)
}

func (q *TaskRunDAOImpl) makeObjListWithTaskRunKey() string {
	return "taskrun:obj_list"
}

// 基础缓存操作实现

// GetTaskRun 获取单个TaskRun缓存
func (p *TaskRunDAOImpl) GetTaskRun(ctx context.Context, id int64) (*entity.TaskRun, error) {
	key := p.makeTaskRunDetailKey(id)
	got, err := p.cmdable.Get(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil // 缓存未命中
		}
		return nil, errorx.Wrapf(err, "redis get taskrun fail, key: %v", key)
	}
	return convert.NewTaskRunConverter().ToDO(conv.UnsafeStringToBytes(got))
}

// SetTaskRun 设置单个TaskRun缓存
func (p *TaskRunDAOImpl) SetTaskRun(ctx context.Context, taskRun *entity.TaskRun, ttl time.Duration) error {
	bytes, err := convert.NewTaskRunConverter().FromDO(taskRun)
	if err != nil {
		return err
	}
	key := p.makeTaskRunDetailKey(taskRun.ID)
	if err := p.cmdable.Set(ctx, key, bytes, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set taskrun cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set taskrun key: %v", key)
	}
	return nil
}

// DeleteTaskRun 删除单个TaskRun缓存
func (p *TaskRunDAOImpl) DeleteTaskRun(ctx context.Context, id int64) error {
	key := p.makeTaskRunDetailKey(id)
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete taskrun cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete taskrun key: %v", key)
	}
	return nil
}

// 列表缓存操作实现

// GetNonFinalTaskRunList 获取非终态TaskRun列表缓存
func (p *TaskRunDAOImpl) GetNonFinalTaskRunList(ctx context.Context) ([]*entity.TaskRun, error) {
	key := p.makeNonFinalTaskRunListKey()
	got, err := p.cmdable.Get(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil // 缓存未命中
		}
		return nil, errorx.Wrapf(err, "redis get non final taskrun list fail, key: %v", key)
	}

	var taskRuns []*entity.TaskRun
	if err := json.Unmarshal(conv.UnsafeStringToBytes(got), &taskRuns); err != nil {
		return nil, errorx.Wrapf(err, "unmarshal non final taskrun list cache failed")
	}

	return taskRuns, nil
}

// SetNonFinalTaskRunList 设置非终态TaskRun列表缓存
func (p *TaskRunDAOImpl) SetNonFinalTaskRunList(ctx context.Context, taskRuns []*entity.TaskRun, ttl time.Duration) error {
	key := p.makeNonFinalTaskRunListKey()

	bytes, err := json.Marshal(taskRuns)
	if err != nil {
		return errorx.Wrapf(err, "marshal non final taskrun list cache failed")
	}

	if err := p.cmdable.Set(ctx, key, bytes, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set non final taskrun list cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set non final taskrun list key: %v", key)
	}
	return nil
}

// DeleteNonFinalTaskRunList 删除非终态TaskRun列表缓存
func (p *TaskRunDAOImpl) DeleteNonFinalTaskRunList(ctx context.Context) error {
	key := p.makeNonFinalTaskRunListKey()
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete non final taskrun list cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete non final taskrun list key: %v", key)
	}
	return nil
}

// GetTaskRunListByTask 获取按Task分组的TaskRun列表缓存
func (p *TaskRunDAOImpl) GetTaskRunListByTask(ctx context.Context, taskID int64) ([]*entity.TaskRun, error) {
	key := p.makeTaskRunListByTaskKey(taskID)
	got, err := p.cmdable.Get(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil // 缓存未命中
		}
		return nil, errorx.Wrapf(err, "redis get taskrun list by task fail, key: %v", key)
	}

	var taskRuns []*entity.TaskRun
	if err := json.Unmarshal(conv.UnsafeStringToBytes(got), &taskRuns); err != nil {
		return nil, errorx.Wrapf(err, "unmarshal taskrun list by task cache failed")
	}

	return taskRuns, nil
}

// SetTaskRunListByTask 设置按Task分组的TaskRun列表缓存
func (p *TaskRunDAOImpl) SetTaskRunListByTask(ctx context.Context, taskID int64, taskRuns []*entity.TaskRun, ttl time.Duration) error {
	key := p.makeTaskRunListByTaskKey(taskID)

	bytes, err := json.Marshal(taskRuns)
	if err != nil {
		return errorx.Wrapf(err, "marshal taskrun list by task cache failed")
	}

	if err := p.cmdable.Set(ctx, key, bytes, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set taskrun list by task cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set taskrun list by task key: %v", key)
	}
	return nil
}

// DeleteTaskRunListByTask 删除按Task分组的TaskRun列表缓存
func (p *TaskRunDAOImpl) DeleteTaskRunListByTask(ctx context.Context, taskID int64) error {
	key := p.makeTaskRunListByTaskKey(taskID)
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete taskrun list by task cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete taskrun list by task key: %v", key)
	}
	return nil
}

// 计数缓存操作实现

// GetTaskRunCount 获取TaskRun计数缓存
func (p *TaskRunDAOImpl) GetTaskRunCount(ctx context.Context, taskID, taskRunID int64) (int64, error) {
	key := p.makeTaskRunCountKey(taskID, taskRunID)
	got, err := p.cmdable.Get(ctx, key).Int64()
	if err != nil {
		if redis.IsNilError(err) {
			return -1, nil // 缓存未命中，返回-1表示未缓存
		}
		return 0, errorx.Wrapf(err, "redis get taskrun count fail, key: %v", key)
	}
	return got, nil
}

// SetTaskRunCount 设置TaskRun计数缓存
func (p *TaskRunDAOImpl) SetTaskRunCount(ctx context.Context, taskID, taskRunID int64, count int64, ttl time.Duration) error {
	key := p.makeTaskRunCountKey(taskID, taskRunID)
	if err := p.cmdable.Set(ctx, key, count, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set taskrun count cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set taskrun count key: %v", key)
	}
	return nil
}

// DeleteTaskRunCount 删除TaskRun计数缓存
func (p *TaskRunDAOImpl) DeleteTaskRunCount(ctx context.Context, taskID, taskRunID int64) error {
	key := p.makeTaskRunCountKey(taskID, taskRunID)
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete taskrun count cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete taskrun count key: %v", key)
	}
	return nil
}

// 对象列表缓存操作实现

// ObjListWithTaskRunCache 对象列表缓存结构
type ObjListWithTaskRunCache struct {
	SpaceList []string  `json:"space_list"`
	BotList   []string  `json:"bot_list"`
	CachedAt  time.Time `json:"cached_at"`
}

// GetObjListWithTaskRun 获取对象列表缓存
func (p *TaskRunDAOImpl) GetObjListWithTaskRun(ctx context.Context) ([]string, []string, error) {
	key := p.makeObjListWithTaskRunKey()
	got, err := p.cmdable.Get(ctx, key).Result()
	if err != nil {
		if redis.IsNilError(err) {
			return nil, nil, nil // 缓存未命中
		}
		return nil, nil, errorx.Wrapf(err, "redis get obj list with taskrun fail, key: %v", key)
	}

	var cache ObjListWithTaskRunCache
	if err := json.Unmarshal(conv.UnsafeStringToBytes(got), &cache); err != nil {
		return nil, nil, errorx.Wrapf(err, "unmarshal obj list with taskrun cache failed")
	}

	return cache.SpaceList, cache.BotList, nil
}

// SetObjListWithTaskRun 设置对象列表缓存
func (p *TaskRunDAOImpl) SetObjListWithTaskRun(ctx context.Context, spaceList, botList []string, ttl time.Duration) error {
	key := p.makeObjListWithTaskRunKey()

	cache := ObjListWithTaskRunCache{
		SpaceList: spaceList,
		BotList:   botList,
		CachedAt:  time.Now(),
	}

	bytes, err := json.Marshal(cache)
	if err != nil {
		return errorx.Wrapf(err, "marshal obj list with taskrun cache failed")
	}

	if err := p.cmdable.Set(ctx, key, bytes, ttl).Err(); err != nil {
		logs.CtxError(ctx, "redis set obj list with taskrun cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis set obj list with taskrun key: %v", key)
	}
	return nil
}

// DeleteObjListWithTaskRun 删除对象列表缓存
func (p *TaskRunDAOImpl) DeleteObjListWithTaskRun(ctx context.Context) error {
	key := p.makeObjListWithTaskRunKey()
	if err := p.cmdable.Del(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis delete obj list with taskrun cache failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis delete obj list with taskrun key: %v", key)
	}
	return nil
}

// 成功/失败计数操作实现

// IncrTaskRunSuccessCount 增加成功计数
func (p *TaskRunDAOImpl) IncrTaskRunSuccessCount(ctx context.Context, taskID, taskRunID int64) error {
	key := p.makeTaskRunSuccessCountKey(taskID, taskRunID)
	if err := p.cmdable.Incr(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis incr taskrun success count failed", "key", key, "err", err)
		return errorx.Wrapf(err, "redis incr taskrun success count key: %v", key)
	}
	return nil
}

// IncrTaskRunFailCount 增加失败计数
func (p *TaskRunDAOImpl) IncrTaskRunFailCount(ctx context.Context, taskID, taskRunID int64) error {
	key := p.makeTaskRunFailCountKey(taskID, taskRunID)
	if err := p.cmdable.Incr(ctx, key).Err(); err != nil {
		logs.CtxError(ctx, "redis incr taskrun fail count failed", "key", key, "err", err)
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
