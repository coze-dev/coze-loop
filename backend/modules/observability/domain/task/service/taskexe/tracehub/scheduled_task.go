// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/pkg/errors"
)

// TaskRunCountInfo TaskRunCount信息结构
type TaskRunCountInfo struct {
	TaskID           int64
	TaskRunID        int64
	TaskRunCount     int64
	TaskRunSuccCount int64
	TaskRunFailCount int64
}

// TaskCacheInfo 任务缓存信息结构
type TaskCacheInfo struct {
	WorkspaceIDs []string
	BotIDs       []string
	Tasks        []*entity.ObservabilityTask
	UpdateTime   time.Time
}

// startScheduledTask 启动定时任务goroutine - 使用5分钟间隔的定时器
func (h *TraceHubServiceImpl) startScheduledTask() {
	go func() {
		for {
			select {
			case <-h.scheduledTaskTicker.C:
				// 执行定时任务
				h.runScheduledTask()
			case <-h.stopChan:
				// 停止定时任务
				h.scheduledTaskTicker.Stop()
				return
			}
		}
	}()
}

// startSyncTaskRunCounts 启动数据同步定时任务goroutine - 使用1分钟间隔的定时器
func (h *TraceHubServiceImpl) startSyncTaskRunCounts() {
	go func() {
		for {
			select {
			case <-h.syncTaskTicker.C:
				// 执行定时任务
				h.syncTaskRunCounts()
			case <-h.stopChan:
				// 停止定时任务
				h.syncTaskTicker.Stop()
				return
			}
		}
	}()
}

// startSyncTaskCache 启动任务缓存定时任务goroutine - 使用1分钟间隔的定时器
func (h *TraceHubServiceImpl) startSyncTaskCache() {
	go func() {
		for {
			select {
			case <-h.syncTaskTicker.C:
				// 执行定时任务
				h.syncTaskCache()
			case <-h.stopChan:
				// 停止定时任务
				h.syncTaskTicker.Stop()
				return
			}
		}
	}()
}

func (h *TraceHubServiceImpl) runScheduledTask() {
	ctx := context.Background()
	logID := logs.NewLogID()
	ctx = logs.SetLogID(ctx, logID)
	ctx = fillCtxWithEnv(ctx)
	logs.CtxInfo(ctx, "定时任务开始执行...")
	// 读取所有非终态（成功/禁用）任务
	var taskPOs []*entity.ObservabilityTask
	var err error
	var offset int32 = 0
	const limit int32 = 1000

	// 分页循环读取所有任务
	for {
		tasklist, _, err := h.taskRepo.ListTasks(ctx, mysql.ListTaskParam{
			ReqLimit:  limit,
			ReqOffset: offset,
			TaskFilters: &filter.TaskFilterFields{
				FilterFields: []*filter.TaskFilterField{
					{
						FieldName: ptr.Of(filter.TaskFieldNameTaskStatus),
						Values: []string{
							string(task.TaskStatusUnstarted),
							string(task.TaskStatusRunning),
							string(task.TaskStatusPending),
						},
						QueryType: ptr.Of(filter.QueryTypeIn),
						FieldType: ptr.Of(filter.FieldTypeString),
					},
				},
			},
		})
		if err != nil {
			logs.CtxError(ctx, "获取非终态任务列表失败", "err", err)
			return
		}

		// 将当前页的任务添加到总列表中
		taskPOs = append(taskPOs, tasklist...)

		// 如果当前页返回的任务数量小于限制数量，说明已经是最后一页
		if len(tasklist) < int(limit) {
			break
		}

		// 继续下一页，offset增加1000
		offset += limit
	}
	logs.CtxInfo(ctx, "定时任务获取到任务数量:%d", len(taskPOs))
	for _, taskPO := range taskPOs {
		var taskRun, backfillTaskRun *entity.TaskRun
		backfillTaskRun = taskPO.GetBackfillTaskRun()
		taskRun = taskPO.GetCurrentTaskRun()

		taskInfo := tconv.TaskPO2DTO(ctx, taskPO, nil)
		endTime := time.UnixMilli(taskInfo.GetRule().GetEffectiveTime().GetEndAt())
		startTime := time.UnixMilli(taskInfo.GetRule().GetEffectiveTime().GetStartAt())
		proc := h.taskProcessor.GetTaskProcessor(taskInfo.TaskType)
		// 达到任务时间期限
		// 到任务结束时间就结束
		logs.CtxInfo(ctx, "[auto_task]taskID:%d, endTime:%v, startTime:%v", taskInfo.GetID(), endTime, startTime)
		if taskInfo.GetRule().GetBackfillEffectiveTime().GetEndAt() != 0 && taskInfo.GetRule().GetEffectiveTime().GetEndAt() != 0 {
			if time.Now().After(endTime) && backfillTaskRun.RunStatus == task.RunStatusDone {
				err = proc.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
					Task:     taskInfo,
					TaskRun:  backfillTaskRun,
					IsFinish: true,
				})
				if err != nil {
					logs.CtxError(ctx, "OnFinishTaskChange err:%v", err)
					continue
				}
			}
		} else if taskInfo.GetRule().GetBackfillEffectiveTime().GetEndAt() != 0 {
			if backfillTaskRun.RunStatus == task.RunStatusDone {
				err = proc.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
					Task:     taskInfo,
					TaskRun:  backfillTaskRun,
					IsFinish: true,
				})
				if err != nil {
					logs.CtxError(ctx, "OnFinishTaskChange err:%v", err)
					continue
				}
			}
		} else if taskInfo.GetRule().GetEffectiveTime().GetEndAt() != 0 {
			if time.Now().After(endTime) {
				err = proc.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
					Task:     taskInfo,
					TaskRun:  taskRun,
					IsFinish: true,
				})
				if err != nil {
					logs.CtxError(ctx, "OnFinishTaskChange err:%v", err)
					continue
				}
			}
		}
		// 如果任务状态为unstarted，到任务开始时间就开始create
		if taskInfo.GetTaskStatus() == task.TaskStatusUnstarted && time.Now().After(startTime) {
			if !taskInfo.GetRule().GetSampler().GetIsCycle() {
				err = proc.OnCreateTaskRunChange(ctx, taskexe.OnCreateTaskRunChangeReq{
					CurrentTask: taskInfo,
					RunType:     task.TaskRunTypeNewData,
					RunStartAt:  taskInfo.GetRule().GetEffectiveTime().GetStartAt(),
					RunEndAt:    taskInfo.GetRule().GetEffectiveTime().GetEndAt(),
				})
				err = proc.OnUpdateTaskChange(ctx, taskInfo, task.TaskStatusRunning)
				if err != nil {
					logs.CtxError(ctx, "OnUpdateTaskChange err:%v", err)
					continue
				}
			} else {
				err = proc.OnCreateTaskRunChange(ctx, taskexe.OnCreateTaskRunChangeReq{
					CurrentTask: taskInfo,
					RunType:     task.TaskRunTypeNewData,
					RunStartAt:  taskRun.RunEndAt.UnixMilli(),
					RunEndAt:    taskRun.RunEndAt.UnixMilli() + (taskRun.RunEndAt.UnixMilli() - taskRun.RunStartAt.UnixMilli()),
				})
				if err != nil {
					logs.CtxError(ctx, "OnCreateTaskRunChange err:%v", err)
					continue
				}
			}
		}
		// 处理taskRun
		if taskInfo.GetTaskStatus() == task.TaskStatusRunning && taskInfo.GetTaskStatus() == task.TaskStatusPending {
			logs.CtxInfo(ctx, "taskID:%d, taskRun.RunEndAt:%v", taskInfo.GetID(), taskRun.RunEndAt)
			// 重复任务的处理:达到单次任务时间期限
			if time.Now().After(taskRun.RunEndAt) {
				logs.CtxInfo(ctx, "time.Now().After(cycleEndTime)")
				err = proc.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
					Task:     taskInfo,
					TaskRun:  taskRun,
					IsFinish: false,
				})
				if err != nil {
					logs.CtxError(ctx, "OnFinishTaskChange err:%v", err)
					continue
				}
				if taskInfo.GetRule().GetSampler().GetIsCycle() {
					err = proc.OnCreateTaskRunChange(ctx, taskexe.OnCreateTaskRunChangeReq{
						CurrentTask: taskInfo,
						RunType:     task.TaskRunTypeNewData,
						RunStartAt:  taskRun.RunEndAt.UnixMilli(),
						RunEndAt:    taskRun.RunEndAt.UnixMilli() + (taskRun.RunEndAt.UnixMilli() - taskRun.RunStartAt.UnixMilli()),
					})
					if err != nil {
						logs.CtxError(ctx, "OnCreateTaskRunChange err:%v", err)
						continue
					}
				}
			}
		}
	}

}

// syncTaskRunCounts 同步TaskRunCount到数据库
func (h *TraceHubServiceImpl) syncTaskRunCounts() {
	ctx := context.Background()
	logID := logs.NewLogID()
	ctx = logs.SetLogID(ctx, logID)
	ctx = fillCtxWithEnv(ctx)

	logs.CtxInfo(ctx, "开始同步TaskRunCounts到数据库...")

	// 1. 获取非终态任务列表
	taskPOs, _, err := h.taskRepo.ListTasks(ctx, mysql.ListTaskParam{
		ReqLimit:  1000,
		ReqOffset: 0,
		TaskFilters: &filter.TaskFilterFields{
			FilterFields: []*filter.TaskFilterField{
				{
					FieldName: ptr.Of(filter.TaskFieldNameTaskStatus),
					Values: []string{
						string(task.TaskStatusPending),
						string(task.TaskStatusRunning),
						string(task.TaskStatusSuccess),
					},
					QueryType: ptr.Of(filter.QueryTypeIn),
					FieldType: ptr.Of(filter.FieldTypeString),
				},
			},
		},
	})
	if err != nil {
		logs.CtxError(ctx, "ListNonFinalTask err:%v", err)
		return
	}
	if len(taskPOs) == 0 {
		logs.CtxInfo(ctx, "没有非终态任务需要同步")
		return
	}

	logs.CtxInfo(ctx, "获取到非终态任务数量,count:%d", len(taskPOs))

	// 2. 收集所有需要同步的TaskRun信息
	var taskRunInfos []*TaskRunCountInfo
	for _, taskPO := range taskPOs {
		if len(taskPO.TaskRuns) == 0 {
			continue
		}

		for _, taskRun := range taskPO.TaskRuns {
			taskRunInfos = append(taskRunInfos, &TaskRunCountInfo{
				TaskID:    taskPO.ID,
				TaskRunID: taskRun.ID,
			})
		}
	}

	if len(taskRunInfos) == 0 {
		logs.CtxInfo(ctx, "没有TaskRun需要同步")
		return
	}

	logs.CtxInfo(ctx, "需要同步的TaskRun数量", "count", len(taskRunInfos))

	// 3. 批量处理TaskRun，每批50个
	batchSize := 50
	for i := 0; i < len(taskRunInfos); i += batchSize {
		end := i + batchSize
		if end > len(taskRunInfos) {
			end = len(taskRunInfos)
		}

		batch := taskRunInfos[i:end]
		h.processBatch(ctx, batch)
	}
}

// processBatch 批量处理TaskRun计数同步
func (h *TraceHubServiceImpl) processBatch(ctx context.Context, batch []*TaskRunCountInfo) {
	logs.CtxInfo(ctx, "开始处理批次", "batchSize", len(batch))

	// 1. 批量读取Redis计数数据
	for _, info := range batch {
		// 读取taskruncount
		count, err := h.taskRepo.GetTaskRunCount(ctx, info.TaskID, info.TaskRunID)
		if err != nil || count == -1 {
			logs.CtxWarn(ctx, "获取TaskRunCount失败", "taskID", info.TaskID, "taskRunID", info.TaskRunID, "err", err)
		} else {
			info.TaskRunCount = count
		}

		// 读取taskrunscesscount
		successCount, err := h.taskRepo.GetTaskRunSuccessCount(ctx, info.TaskID, info.TaskRunID)
		if err != nil || successCount == -1 {
			logs.CtxWarn(ctx, "获取TaskRunSuccessCount失败", "taskID", info.TaskID, "taskRunID", info.TaskRunID, "err", err)
			successCount = 0
		} else {
			info.TaskRunSuccCount = successCount
		}

		// 读取taskrunfailcount
		failCount, err := h.taskRepo.GetTaskRunFailCount(ctx, info.TaskID, info.TaskRunID)
		if err != nil || failCount == -1 {
			logs.CtxWarn(ctx, "获取TaskRunFailCount失败", "taskID", info.TaskID, "taskRunID", info.TaskRunID, "err", err)
			failCount = 0
		} else {
			info.TaskRunFailCount = failCount
		}

		logs.CtxDebug(ctx, "读取计数数据",
			"taskID", info.TaskID,
			"taskRunID", info.TaskRunID,
			"runCount", info.TaskRunCount,
			"successCount", info.TaskRunSuccCount,
			"failCount", info.TaskRunFailCount)
	}

	// 2. 批量更新数据库
	for _, info := range batch {
		err := h.updateTaskRunDetail(ctx, info)
		if err != nil {
			logs.CtxError(ctx, "更新TaskRun详情失败",
				"taskID", info.TaskID,
				"taskRunID", info.TaskRunID,
				"err", err)
		} else {
			logs.CtxDebug(ctx, "更新TaskRun详情成功",
				"taskID", info.TaskID,
				"taskRunID", info.TaskRunID)
		}
	}

	logs.CtxInfo(ctx, "批次处理完成",
		"batchSize", len(batch))
}

// updateTaskRunDetail 更新TaskRun的run_detail字段
func (h *TraceHubServiceImpl) updateTaskRunDetail(ctx context.Context, info *TaskRunCountInfo) error {
	// 构建run_detail JSON数据
	runDetail := map[string]interface{}{
		"total_count":   info.TaskRunCount,
		"success_count": info.TaskRunSuccCount,
		"failed_count":  info.TaskRunFailCount,
	}

	// 序列化为JSON字符串
	runDetailJSON, err := json.Marshal(runDetail)
	if err != nil {
		return errors.Wrap(err, "序列化run_detail失败")
	}

	runDetailStr := string(runDetailJSON)

	// 构建更新映射
	updateMap := map[string]interface{}{
		"run_detail": &runDetailStr,
	}

	// 使用乐观锁更新
	err = h.taskRepo.UpdateTaskRunWithOCC(ctx, info.TaskRunID, 0, updateMap)
	if err != nil {
		return errors.Wrap(err, "更新TaskRun失败")
	}

	return nil
}

func (h *TraceHubServiceImpl) syncTaskCache() {
	ctx := context.Background()
	logID := logs.NewLogID()
	ctx = logs.SetLogID(ctx, logID)
	ctx = fillCtxWithEnv(ctx)

	logs.CtxInfo(ctx, "开始同步任务缓存...")

	// 1. 从数据库中获取所有非终态任务的spaceID、botID和task信息
	spaceIDs, botIDs, tasks := h.taskRepo.GetObjListWithTask(ctx)
	logs.CtxInfo(ctx, "获取到任务数量", "taskCount", len(tasks), "spaceCount", len(spaceIDs), "botCount", len(botIDs))

	// 2. 构建新的缓存映射
	var newCache = TaskCacheInfo{
		WorkspaceIDs: spaceIDs,
		BotIDs:       botIDs,
		Tasks:        tasks,
		UpdateTime:   time.Now(), // 设置当前时间为更新时间
	}

	// 3. 清空旧缓存并更新新缓存
	h.taskCacheLock.Lock()
	defer h.taskCacheLock.Unlock()

	// 清空旧缓存
	h.taskCache.Range(func(key, value interface{}) bool {
		h.taskCache.Delete(key)
		return true
	})

	// 4. 将新缓存写入本地缓存
	h.taskCache.Store("ObjListWithTask", &newCache)

	logs.CtxInfo(ctx, "任务缓存同步完成", "taskCount", len(tasks), "updateTime", newCache.UpdateTime.Format(time.RFC3339))
}
