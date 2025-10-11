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

// TaskRunCountInfo represents the TaskRunCount information structure
type TaskRunCountInfo struct {
	TaskID           int64
	TaskRunID        int64
	TaskRunCount     int64
	TaskRunSuccCount int64
	TaskRunFailCount int64
}

// TaskCacheInfo represents task cache information
type TaskCacheInfo struct {
	WorkspaceIDs []string
	BotIDs       []string
	Tasks        []*entity.ObservabilityTask
	UpdateTime   time.Time
}

// startScheduledTask launches the scheduled task goroutine with a five-minute interval timer
func (h *TraceHubServiceImpl) startScheduledTask() {
	go func() {
		for {
			select {
			case <-h.scheduledTaskTicker.C:
				// Execute scheduled task
				h.runScheduledTask()
			case <-h.stopChan:
				// Stop scheduled task
				h.scheduledTaskTicker.Stop()
				return
			}
		}
	}()
}

// startSyncTaskRunCounts launches the data sync scheduled task goroutine with a one-minute interval timer
func (h *TraceHubServiceImpl) startSyncTaskRunCounts() {
	go func() {
		for {
			select {
			case <-h.syncTaskTicker.C:
				// Execute scheduled task
				h.syncTaskRunCounts()
			case <-h.stopChan:
				// Stop scheduled task
				h.syncTaskTicker.Stop()
				return
			}
		}
	}()
}

// startSyncTaskCache launches the task cache scheduled task goroutine with a one-minute interval timer
func (h *TraceHubServiceImpl) startSyncTaskCache() {
	go func() {
		for {
			select {
			case <-h.syncTaskTicker.C:
				// Execute scheduled task
				h.syncTaskCache()
			case <-h.stopChan:
				// Stop scheduled task
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
	ctx = h.fillCtx(ctx)
	logs.CtxInfo(ctx, "Scheduled task started...")
	// Read all non-final (success/disabled) tasks
	var taskPOs []*entity.ObservabilityTask
	var err error
	var offset int32 = 0
	const limit int32 = 1000

	// Paginate through all tasks
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
			logs.CtxError(ctx, "Failed to get non-final task list", "err", err)
			return
		}

		// Add tasks from the current page to the full list
		taskPOs = append(taskPOs, tasklist...)

		// If fewer tasks than limit are returned, this is the last page
		if len(tasklist) < int(limit) {
			break
		}

		// Move to the next page, increasing offset by 1000
		offset += limit
	}
	logs.CtxInfo(ctx, "Scheduled task retrieved number of tasks:%d", len(taskPOs))
	for _, taskPO := range taskPOs {
		var taskRun, backfillTaskRun *entity.TaskRun
		backfillTaskRun = taskPO.GetBackfillTaskRun()
		taskRun = taskPO.GetCurrentTaskRun()

		taskInfo := tconv.TaskPO2DTO(ctx, taskPO, nil)
		endTime := time.UnixMilli(taskInfo.GetRule().GetEffectiveTime().GetEndAt())
		startTime := time.UnixMilli(taskInfo.GetRule().GetEffectiveTime().GetStartAt())
		proc := h.taskProcessor.GetTaskProcessor(taskInfo.TaskType)
		// Task time horizon reached
		// End when the task end time is reached
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
		// If the task status is unstarted, create it once the task start time is reached
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
		// Handle taskRun
		if taskInfo.GetTaskStatus() == task.TaskStatusRunning && taskInfo.GetTaskStatus() == task.TaskStatusPending {
			logs.CtxInfo(ctx, "taskID:%d, taskRun.RunEndAt:%v", taskInfo.GetID(), taskRun.RunEndAt)
			// Handling repeated tasks: single task time horizon reached
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

// syncTaskRunCounts synchronizes TaskRunCount data to the database
func (h *TraceHubServiceImpl) syncTaskRunCounts() {
	ctx := context.Background()
	logID := logs.NewLogID()
	ctx = logs.SetLogID(ctx, logID)
	ctx = h.fillCtx(ctx)

	logs.CtxInfo(ctx, "Start syncing TaskRunCounts to database...")

	// 1. Retrieve non-final task list
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
		logs.CtxInfo(ctx, "No non-final tasks need syncing")
		return
	}

	logs.CtxInfo(ctx, "Retrieved non-final task count:%d", len(taskPOs))

	// 2. Collect all TaskRun information that needs syncing
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
		logs.CtxInfo(ctx, "No TaskRun requires syncing")
		return
	}

	logs.CtxInfo(ctx, "Number of TaskRun entries requiring syncing:%d", len(taskRunInfos))

	// 3. Process TaskRun entries in batches of 50
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

// processBatch synchronizes TaskRun counts in batches
func (h *TraceHubServiceImpl) processBatch(ctx context.Context, batch []*TaskRunCountInfo) {
	logs.CtxInfo(ctx, "Start processing batch, batchSize:%d", len(batch))

	// 1. Read Redis count data in batch
	for _, info := range batch {
		// Read taskruncount
		count, err := h.taskRepo.GetTaskRunCount(ctx, info.TaskID, info.TaskRunID)
		if err != nil || count == -1 {
			logs.CtxWarn(ctx, "Failed to get TaskRunCount, taskID:%d, taskRunID:%d, err:%v", info.TaskID, info.TaskRunID, err)
		} else {
			info.TaskRunCount = count
		}

		// Read taskrun success count
		successCount, err := h.taskRepo.GetTaskRunSuccessCount(ctx, info.TaskID, info.TaskRunID)
		if err != nil || successCount == -1 {
			logs.CtxWarn(ctx, "Failed to get TaskRunSuccessCount, taskID:%d, taskRunID:%d, err:%v", info.TaskID, info.TaskRunID, err)
			successCount = 0
		} else {
			info.TaskRunSuccCount = successCount
		}

		// Read taskrun fail count
		failCount, err := h.taskRepo.GetTaskRunFailCount(ctx, info.TaskID, info.TaskRunID)
		if err != nil || failCount == -1 {
			logs.CtxWarn(ctx, "Failed to get TaskRunFailCount, taskID:%d, taskRunID:%d, err:%v", info.TaskID, info.TaskRunID, err)
			failCount = 0
		} else {
			info.TaskRunFailCount = failCount
		}

		logs.CtxDebug(ctx, "Read count data",
			"taskID", info.TaskID,
			"taskRunID", info.TaskRunID,
			"runCount", info.TaskRunCount,
			"successCount", info.TaskRunSuccCount,
			"failCount", info.TaskRunFailCount)
	}

	// 2. Update database in batch
	for _, info := range batch {
		err := h.updateTaskRunDetail(ctx, info)
		if err != nil {
			logs.CtxError(ctx, "Failed to update TaskRun detail",
				"taskID", info.TaskID,
				"taskRunID", info.TaskRunID,
				"err", err)
		} else {
			logs.CtxDebug(ctx, "Succeeded in updating TaskRun detail",
				"taskID", info.TaskID,
				"taskRunID", info.TaskRunID)
		}
	}

	logs.CtxInfo(ctx, "Batch processing completed",
		"batchSize", len(batch))
}

// updateTaskRunDetail updates the run_detail field of TaskRun
func (h *TraceHubServiceImpl) updateTaskRunDetail(ctx context.Context, info *TaskRunCountInfo) error {
	// Build run_detail JSON data
	runDetail := map[string]interface{}{
		"total_count":   info.TaskRunCount,
		"success_count": info.TaskRunSuccCount,
		"failed_count":  info.TaskRunFailCount,
	}

	// Serialize into JSON string
	runDetailJSON, err := json.Marshal(runDetail)
	if err != nil {
		return errors.Wrap(err, "Failed to serialize run_detail")
	}

	runDetailStr := string(runDetailJSON)

	// Build update map
	updateMap := map[string]interface{}{
		"run_detail": &runDetailStr,
	}

	// Update using optimistic locking
	err = h.taskRepo.UpdateTaskRunWithOCC(ctx, info.TaskRunID, 0, updateMap)
	if err != nil {
		return errors.Wrap(err, "Failed to update TaskRun")
	}

	return nil
}

func (h *TraceHubServiceImpl) syncTaskCache() {
	ctx := context.Background()
	logID := logs.NewLogID()
	ctx = logs.SetLogID(ctx, logID)
	ctx = h.fillCtx(ctx)

	logs.CtxInfo(ctx, "Start syncing task cache...")

	// 1. Retrieve spaceID, botID, and task information for all non-final tasks from the database
	spaceIDs, botIDs, tasks := h.taskRepo.GetObjListWithTask(ctx)
	logs.CtxInfo(ctx, "Retrieved task information, taskCount:%d, spaceCount:%d, botCount:%d", len(tasks), len(spaceIDs), len(botIDs))

	// 2. Build a new cache map
	var newCache = TaskCacheInfo{
		WorkspaceIDs: spaceIDs,
		BotIDs:       botIDs,
		Tasks:        tasks,
		UpdateTime:   time.Now(), // Set the current time as the update time
	}

	// 3. Clear old cache and update with new cache
	h.taskCacheLock.Lock()
	defer h.taskCacheLock.Unlock()

	// Clear old cache
	h.taskCache.Delete("ObjListWithTask")

	// 4. Write new cache into local cache
	h.taskCache.Store("ObjListWithTask", &newCache)

	logs.CtxInfo(ctx, "Task cache sync completed, taskCount:%d, updateTime:%s", len(tasks), newCache.UpdateTime.Format(time.RFC3339))
}
