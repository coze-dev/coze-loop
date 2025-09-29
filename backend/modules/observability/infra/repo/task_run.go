// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

//import (
//	"context"
//	"strconv"
//	"time"
//
//	"github.com/coze-dev/coze-loop/backend/infra/idgen"
//	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
//	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
//	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/convertor"
//	taskRunDao "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/redis/dao"
//	"github.com/coze-dev/coze-loop/backend/pkg/logs"
//)
//
////func NewTaskRunRepoImpl(taskRunDao mysql.ITaskRunDao, idGenerator idgen.IIDGenerator, taskRunRedisDao taskRunDao.ITaskRunDAO) repo.ITaskRunRepo {
////	return &TaskRunRepoImpl{
////		TaskRunDao:      taskRunDao,
////		idGenerator:     idGenerator,
////		TaskRunRedisDao: taskRunRedisDao,
////	}
////}
//
//type TaskRunRepoImpl struct {
//	TaskRunDao      mysql.ITaskRunDao
//	TaskRunRedisDao taskRunDao.ITaskRunDAO
//	idGenerator     idgen.IIDGenerator
//}
//
//// 缓存 TTL 常量
//const (
//	TaskRunDetailTTL       = 15 * time.Minute // TaskRun缓存15分钟
//	NonFinalTaskRunListTTL = 30 * time.Second // 非终态TaskRun缓存30秒
//	TaskRunListByTaskTTL   = 5 * time.Minute  // 按Task分组的TaskRun列表缓存5分钟
//	ObjListWithTaskRunTTL  = 15 * time.Minute // 对象列表缓存15分钟
//)
//
//// GetTaskRun 获取单个TaskRun
//func (v *TaskRunRepoImpl) GetTaskRun(ctx context.Context, id int64, workspaceID *int64, taskID *int64) (*entity.TaskRun, error) {
//	// 先查 Redis 缓存
//	cachedTaskRun, err := v.TaskRunRedisDao.GetTaskRun(ctx, id)
//	if err != nil {
//		logs.CtxWarn(ctx, "failed to get taskrun from redis cache", "id", id, "err", err)
//	} else if cachedTaskRun != nil {
//		// 验证权限（workspaceID 和 taskID）
//		if workspaceID != nil && cachedTaskRun.WorkspaceID != *workspaceID {
//			return nil, nil // 权限不符，返回空
//		}
//		if taskID != nil && cachedTaskRun.TaskID != *taskID {
//			return nil, nil // 权限不符，返回空
//		}
//		return cachedTaskRun, nil
//	}
//
//	// 缓存未命中，查询数据库
//	taskRunPo, err := v.TaskRunDao.GetTaskRun(ctx, id, workspaceID, taskID)
//	if err != nil {
//		return nil, err
//	}
//
//	taskRunDO := convertor.TaskRunPO2DO(taskRunPo)
//
//	// 异步缓存到 Redis
//	go func() {
//		if err := v.TaskRunRedisDao.SetTaskRun(context.Background(), taskRunDO, TaskRunDetailTTL); err != nil {
//			logs.Error("failed to set taskrun cache", "id", id, "err", err)
//		}
//	}()
//
//	return taskRunDO, nil
//}
//
//// GetBackfillTaskRun 获取Backfill类型的TaskRun
//func (v *TaskRunRepoImpl) GetBackfillTaskRun(ctx context.Context, workspaceID *int64, taskID int64) (*entity.TaskRun, error) {
//	taskRunPo, err := v.TaskRunDao.GetBackfillTaskRun(ctx, workspaceID, taskID)
//	if err != nil {
//		return nil, err
//	}
//	if taskRunPo == nil {
//		return nil, nil
//	}
//	return convertor.TaskRunPO2DO(taskRunPo), nil
//}
//
//// GetNewDataTaskRun 获取NewData类型的TaskRun
//
//func (v *TaskRunRepoImpl) GetLatestNewDataTaskRun(ctx context.Context, workspaceID *int64, taskID int64) (*entity.TaskRun, error) {
//	taskRunPo, err := v.TaskRunDao.GetLatestNewDataTaskRun(ctx, workspaceID, taskID)
//	if err != nil {
//		return nil, err
//	}
//	if taskRunPo == nil {
//		return nil, nil
//	}
//	return convertor.TaskRunPO2DO(taskRunPo), nil
//}
//
//// ListTaskRuns 获取TaskRun列表
//func (v *TaskRunRepoImpl) ListTaskRuns(ctx context.Context, param mysql.ListTaskRunParam) ([]*entity.TaskRun, int64, error) {
//	results, total, err := v.TaskRunDao.ListTaskRuns(ctx, param)
//	if err != nil {
//		return nil, 0, err
//	}
//	resp := make([]*entity.TaskRun, len(results))
//	for i, result := range results {
//		resp[i] = convertor.TaskRunPO2DO(result)
//	}
//	return resp, total, nil
//}
//
//// CreateTaskRun 创建TaskRun
//func (v *TaskRunRepoImpl) CreateTaskRun(ctx context.Context, do *entity.TaskRun) (int64, error) {
//	id, err := v.idGenerator.GenID(ctx)
//	if err != nil {
//		return 0, err
//	}
//	taskRunPo := convertor.TaskRunDO2PO(do)
//	taskRunPo.ID = id
//
//	// 先执行数据库操作
//	createdID, err := v.TaskRunDao.CreateTaskRun(ctx, taskRunPo)
//	if err != nil {
//		return 0, err
//	}
//
//	// 数据库操作成功后，更新缓存
//	do.ID = createdID
//	go func() {
//		// 缓存新创建的TaskRun
//		if err := v.TaskRunRedisDao.SetTaskRun(context.Background(), do, TaskRunDetailTTL); err != nil {
//			logs.Error("failed to set taskrun cache after create", "id", createdID, "err", err)
//		}
//
//		// 清理相关列表缓存
//		v.clearTaskRunListCaches(context.Background(), do.WorkspaceID, do.TaskID)
//	}()
//
//	return createdID, nil
//}
//
//// UpdateTaskRun 更新TaskRun
//func (v *TaskRunRepoImpl) UpdateTaskRun(ctx context.Context, do *entity.TaskRun) error {
//	taskRunPo := convertor.TaskRunDO2PO(do)
//
//	// 先执行数据库操作
//	err := v.TaskRunDao.UpdateTaskRun(ctx, taskRunPo)
//	if err != nil {
//		return err
//	}
//
//	// 数据库操作成功后，更新缓存
//	go func() {
//		// 更新单个TaskRun缓存
//		if err := v.TaskRunRedisDao.SetTaskRun(context.Background(), do, TaskRunDetailTTL); err != nil {
//			logs.Error("failed to update taskrun cache", "id", do.ID, "err", err)
//		}
//
//		// 清理相关列表缓存
//		v.clearTaskRunListCaches(context.Background(), do.WorkspaceID, do.TaskID)
//	}()
//
//	return nil
//}
//
//// DeleteTaskRun 删除TaskRun
//func (v *TaskRunRepoImpl) DeleteTaskRun(ctx context.Context, id int64, workspaceID int64, userID string) error {
//	// 先执行数据库操作
//	err := v.TaskRunDao.DeleteTaskRun(ctx, id, workspaceID, userID)
//	if err != nil {
//		return err
//	}
//
//	// 数据库操作成功后，删除缓存
//	go func() {
//		// 删除单个TaskRun缓存
//		if err := v.TaskRunRedisDao.DeleteTaskRun(context.Background(), id); err != nil {
//			logs.Error("failed to delete taskrun cache", "id", id, "err", err)
//		}
//
//		// 清理相关列表缓存
//		v.clearTaskRunListCaches(context.Background(), workspaceID, 0)
//	}()
//
//	return nil
//}
//
//// ListNonFinalTaskRun 获取非终态TaskRun列表
//func (v *TaskRunRepoImpl) ListNonFinalTaskRun(ctx context.Context) ([]*entity.TaskRun, error) {
//	// 先查 Redis 缓存
//	cachedTaskRuns, err := v.TaskRunRedisDao.GetNonFinalTaskRunList(ctx)
//	if err != nil {
//		logs.CtxWarn(ctx, "failed to get non final taskrun list from redis cache", "err", err)
//	} else if cachedTaskRuns != nil {
//		return cachedTaskRuns, nil
//	}
//
//	// 缓存未命中，查询数据库
//	results, err := v.TaskRunDao.ListNonFinalTaskRun(ctx)
//	if err != nil {
//		return nil, err
//	}
//
//	resp := make([]*entity.TaskRun, len(results))
//	for i, result := range results {
//		resp[i] = convertor.TaskRunPO2DO(result)
//	}
//
//	// 异步缓存到 Redis（短TTL，因为非最终状态变化频繁）
//	go func() {
//		if err := v.TaskRunRedisDao.SetNonFinalTaskRunList(context.Background(), resp, NonFinalTaskRunListTTL); err != nil {
//			logs.Error("failed to set non final taskrun list cache", "err", err)
//		}
//	}()
//
//	return resp, nil
//}
//
//// ListNonFinalTaskRunByTaskID 按TaskID获取非终态TaskRun
//func (v *TaskRunRepoImpl) ListNonFinalTaskRunByTaskID(ctx context.Context, taskID int64) ([]*entity.TaskRun, error) {
//	// 先尝试从按Task分组的缓存中获取，然后过滤非终态
//	cachedTaskRuns, err := v.TaskRunRedisDao.GetTaskRunListByTask(ctx, taskID)
//	if err != nil {
//		logs.CtxWarn(ctx, "failed to get taskrun list by task from redis cache", "taskID", taskID, "err", err)
//	} else if cachedTaskRuns != nil {
//		// 过滤出非终态TaskRun
//		var nonFinalTaskRuns []*entity.TaskRun
//		for _, tr := range cachedTaskRuns {
//			if isNonFinalStatus(tr.RunStatus) {
//				nonFinalTaskRuns = append(nonFinalTaskRuns, tr)
//			}
//		}
//		return nonFinalTaskRuns, nil
//	}
//
//	// 缓存未命中，查询数据库
//	results, err := v.TaskRunDao.ListNonFinalTaskRunByTaskID(ctx, taskID)
//	if err != nil {
//		return nil, err
//	}
//
//	resp := make([]*entity.TaskRun, len(results))
//	for i, result := range results {
//		resp[i] = convertor.TaskRunPO2DO(result)
//	}
//
//	return resp, nil
//}
//
//// ListNonFinalTaskRunBySpaceID 按空间ID获取非终态TaskRun
//func (v *TaskRunRepoImpl) ListNonFinalTaskRunBySpaceID(ctx context.Context, spaceID string) []*entity.TaskRun {
//	// 缓存未命中，查询数据库
//	spaceIDInt, _ := strconv.ParseInt(spaceID, 10, 64)
//	results, err := v.TaskRunDao.ListNonFinalTaskRunBySpaceID(ctx, spaceIDInt)
//	if err != nil {
//		logs.CtxError(ctx, "failed to get non final taskrun by space id", "spaceID", spaceID, "err", err)
//		return nil
//	}
//	resp := make([]*entity.TaskRun, len(results))
//	for i, result := range results {
//		resp[i] = convertor.TaskRunPO2DO(result)
//	}
//	return resp
//}
//
//// UpdateTaskRunWithOCC 乐观并发控制更新
//func (v *TaskRunRepoImpl) UpdateTaskRunWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error {
//	// 先执行数据库操作
//	logs.CtxInfo(ctx, "UpdateTaskRunWithOCC", "id", id, "workspaceID", workspaceID, "updateMap", updateMap)
//	err := v.TaskRunDao.UpdateTaskRunWithOCC(ctx, id, workspaceID, updateMap)
//	if err != nil {
//		return err
//	}
//
//	// 数据库操作成功后，删除缓存（因为无法直接更新部分字段）
//	go func() {
//		// 删除单个TaskRun缓存，下次查询时会重新加载
//		if err := v.TaskRunRedisDao.DeleteTaskRun(context.Background(), id); err != nil {
//			logs.Error("failed to delete taskrun cache after OCC update", "id", id, "err", err)
//		}
//
//		// 清理相关列表缓存
//		v.clearTaskRunListCaches(context.Background(), workspaceID, 0)
//
//		// 清理非最终状态TaskRun缓存（状态可能发生变化）
//		if err := v.TaskRunRedisDao.DeleteNonFinalTaskRunList(context.Background()); err != nil {
//			logs.Error("failed to delete non final taskrun list cache after OCC update", "err", err)
//		}
//	}()
//
//	return nil
//}
//
//// GetObjListWithTaskRun 获取有TaskRun的对象列表
//func (v *TaskRunRepoImpl) GetObjListWithTaskRun(ctx context.Context) ([]string, []string) {
//	// 先查 Redis 缓存
//	spaceList, botList, err := v.TaskRunRedisDao.GetObjListWithTaskRun(ctx)
//	if err != nil {
//		logs.CtxWarn(ctx, "failed to get obj list with taskrun from redis cache", "err", err)
//	} else if spaceList != nil || botList != nil {
//		return spaceList, botList
//	}
//
//	// 缓存未命中，查询数据库
//	spaceList, botList, err = v.TaskRunDao.GetObjListWithTaskRun(ctx)
//	if err != nil {
//		logs.CtxWarn(ctx, "failed to get obj list with taskrun from mysql", "err", err)
//		return nil, nil
//	}
//
//	// 异步缓存结果
//	go func() {
//		if err := v.TaskRunRedisDao.SetObjListWithTaskRun(context.Background(), spaceList, botList, ObjListWithTaskRunTTL); err != nil {
//			logs.Error("failed to set obj list with taskrun cache", "err", err)
//		}
//	}()
//
//	return spaceList, botList
//}
//
//// ListActiveTaskRunsByTask 获取Task的活跃TaskRun列表
//func (v *TaskRunRepoImpl) ListActiveTaskRunsByTask(ctx context.Context, taskID int64) ([]*entity.TaskRun, error) {
//	// 先查缓存
//	cachedTaskRuns, err := v.TaskRunRedisDao.GetTaskRunListByTask(ctx, taskID)
//	if err != nil {
//		logs.CtxWarn(ctx, "failed to get taskrun list by task from redis", "taskID", taskID, "err", err)
//	} else if cachedTaskRuns != nil {
//		// 过滤出活跃状态的TaskRun
//		var activeTaskRuns []*entity.TaskRun
//		for _, tr := range cachedTaskRuns {
//			if isActiveStatus(tr.RunStatus) {
//				activeTaskRuns = append(activeTaskRuns, tr)
//			}
//		}
//		return activeTaskRuns, nil
//	}
//
//	// 缓存未命中，查询数据库
//	results, err := v.TaskRunDao.ListActiveTaskRunsByTask(ctx, taskID)
//	if err != nil {
//		return nil, err
//	}
//
//	resp := make([]*entity.TaskRun, len(results))
//	for i, result := range results {
//		resp[i] = convertor.TaskRunPO2DO(result)
//	}
//
//	// 异步缓存
//	go func() {
//		if err := v.TaskRunRedisDao.SetTaskRunListByTask(context.Background(), taskID, resp, TaskRunListByTaskTTL); err != nil {
//			logs.Error("failed to set taskrun list by task cache", "taskID", taskID, "err", err)
//		}
//	}()
//
//	return resp, nil
//}
//
//// GetLatestTaskRunByTask 获取Task的最新TaskRun
//func (v *TaskRunRepoImpl) GetLatestTaskRunByTask(ctx context.Context, taskID int64) (*entity.TaskRun, error) {
//	// 先查缓存
//	cachedTaskRuns, err := v.TaskRunRedisDao.GetTaskRunListByTask(ctx, taskID)
//	if err != nil {
//		logs.CtxWarn(ctx, "failed to get taskrun list by task from redis", "taskID", taskID, "err", err)
//	} else if cachedTaskRuns != nil && len(cachedTaskRuns) > 0 {
//		// 缓存中的TaskRun列表应该已经按创建时间排序，返回第一个
//		return cachedTaskRuns[0], nil
//	}
//
//	// 缓存未命中，查询数据库
//	result, err := v.TaskRunDao.GetLatestTaskRunByTask(ctx, taskID)
//	if err != nil {
//		return nil, err
//	}
//	if result == nil {
//		return nil, nil
//	}
//
//	taskRunDO := convertor.TaskRunPO2DO(result)
//	return taskRunDO, nil
//}
//
//// ListTaskRunsByStatus 按状态获取TaskRun列表
//func (v *TaskRunRepoImpl) ListTaskRunsByStatus(ctx context.Context, status string) ([]*entity.TaskRun, error) {
//	// 直接查询数据库，不缓存（状态查询通常是临时性的）
//	results, err := v.TaskRunDao.ListTaskRunsByStatus(ctx, status)
//	if err != nil {
//		return nil, err
//	}
//
//	resp := make([]*entity.TaskRun, len(results))
//	for i, result := range results {
//		resp[i] = convertor.TaskRunPO2DO(result)
//	}
//
//	return resp, nil
//}
//
//// GetTaskRunCount 获取TaskRun计数
//func (v *TaskRunRepoImpl) GetTaskRunCount(ctx context.Context, taskID, taskRunID int64) (int64, error) {
//	// 先查 Redis 缓存
//	count, err := v.TaskRunRedisDao.GetTaskRunCount(ctx, taskID, taskRunID)
//	if err != nil {
//		logs.CtxWarn(ctx, "failed to get taskrun count from redis cache", "taskID", taskID, "taskRunID", taskRunID, "err", err)
//	} else if count != -1 {
//		return count, nil
//	}
//
//	// 缓存未命中，这里可以根据业务需求实现具体的计数逻辑
//	// 目前返回0，实际使用时需要根据业务需求实现
//	return 0, nil
//}
//
//// clearTaskRunListCaches 清理与指定 workspace 和 task 相关的列表缓存
//func (v *TaskRunRepoImpl) clearTaskRunListCaches(ctx context.Context, workspaceID, taskID int64) {
//	// 清理非终态TaskRun列表缓存
//	if err := v.TaskRunRedisDao.DeleteNonFinalTaskRunList(ctx); err != nil {
//		logs.Error("failed to delete non final taskrun list cache", "err", err)
//	}
//
//	// 清理按Task分组的TaskRun列表缓存
//	if taskID > 0 {
//		if err := v.TaskRunRedisDao.DeleteTaskRunListByTask(ctx, taskID); err != nil {
//			logs.Error("failed to delete taskrun list by task cache", "taskID", taskID, "err", err)
//		}
//	}
//
//	// 清理对象列表缓存
//	if err := v.TaskRunRedisDao.DeleteObjListWithTaskRun(ctx); err != nil {
//		logs.Error("failed to delete obj list with taskrun cache", "err", err)
//	}
//}
//
//// isNonFinalStatus 检查是否为非终态状态
//func isNonFinalStatus(status string) bool {
//	nonFinalStatuses := []string{"pending", "running", "paused", "retrying"}
//	for _, s := range nonFinalStatuses {
//		if status == s {
//			return true
//		}
//	}
//	return false
//}
//
//// isActiveStatus 检查是否为活跃状态
//func isActiveStatus(status string) bool {
//	activeStatuses := []string{"running", "retrying"}
//	for _, s := range activeStatuses {
//		if status == s {
//			return true
//		}
//	}
//	return false
//}
