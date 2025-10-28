// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
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

const (
	transformTaskStatusLockKey = "observability:tracehub:transform_task_status"
	transformTaskStatusLockTTL = 3 * time.Minute
	syncTaskRunCountsLockKey   = "observability:tracehub:sync_task_run_counts"
)

// startScheduledTask launches the scheduled task goroutine
func (h *TraceHubServiceImpl) startScheduledTask() {
	h.syncTaskCache()
	go func() {
		for {
			select {
			case <-h.scheduledTaskTicker.C:
				// Execute scheduled task
				h.transformTaskStatus() // 抢锁
			case <-h.stopChan:
				// Stop scheduled task
				h.scheduledTaskTicker.Stop()
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case <-h.syncTaskTicker.C:
				// Execute scheduled task
				h.syncTaskRunCounts() // 抢锁
				h.syncTaskCache()
			case <-h.stopChan:
				// Stop scheduled task
				h.syncTaskTicker.Stop()
				return
			}
		}
	}()
}

func (h *TraceHubServiceImpl) transformTaskStatus() {
	const key = "consumer_listening"
	cfg := &config.ConsumerListening{}
	if err := h.loader.UnmarshalKey(context.Background(), key, cfg); err != nil {
		return
	}
	if !cfg.IsEnabled || !cfg.IsAllSpace {
		return
	}

	if slices.Contains([]string{TracehubClusterName, InjectClusterName}, os.Getenv(TceCluster)) {
		return
	}
	ctx := context.Background()
	ctx = h.fillCtx(ctx)

	if h.locker != nil {
		locked, lockErr := h.locker.Lock(ctx, transformTaskStatusLockKey, transformTaskStatusLockTTL)
		if lockErr != nil {
			logs.CtxError(ctx, "transformTaskStatus acquire lock failed", "err", lockErr)
			return
		}
		if !locked {
			logs.CtxInfo(ctx, "transformTaskStatus lock held by others, skip execution")
			return
		}
	}
	logs.CtxInfo(ctx, "Scheduled task started...")

	// Read all non-final (success/disabled) tasks
	taskPOs, err := h.listNonFinalTask(ctx)
	if err != nil {
		logs.CtxError(ctx, "Failed to get non-final task list", "err", err)
		return
	}
	logs.CtxInfo(ctx, "Scheduled task retrieved number of tasks:%d", len(taskPOs))
	for _, taskPO := range taskPOs {
		var taskRun, backfillTaskRun *entity.TaskRun
		backfillTaskRun = taskPO.GetBackfillTaskRun()
		taskRun = taskPO.GetCurrentTaskRun()
		var startTime, endTime time.Time
		// taskInfo := tconv.TaskDO2DTO(ctx, taskPO, nil)

		if taskPO.EffectiveTime != nil {
			endTime = time.UnixMilli(taskPO.EffectiveTime.EndAt)
			startTime = time.UnixMilli(taskPO.EffectiveTime.StartAt)
		}
		proc := h.taskProcessor.GetTaskProcessor(taskPO.TaskType)
		// Task time horizon reached
		// End when the task end time is reached
		logs.CtxInfo(ctx, "[auto_task]taskID:%d, endTime:%v, startTime:%v", taskPO.ID, endTime, startTime)
		if taskPO.BackfillEffectiveTime != nil && taskPO.EffectiveTime != nil && backfillTaskRun != nil {
			if time.Now().After(endTime) && backfillTaskRun.RunStatus == task.RunStatusDone {
				logs.CtxInfo(ctx, "[OnFinishTaskChange]taskID:%d, time.Now().After(endTime) && backfillTaskRun.RunStatus == task.RunStatusDone", taskPO.ID)
				err = proc.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
					Task:     taskPO,
					TaskRun:  backfillTaskRun,
					IsFinish: true,
				})
				if err != nil {
					logs.CtxError(ctx, "OnFinishTaskChange err:%v", err)
					continue
				}
			}
			if backfillTaskRun.RunStatus != task.RunStatusDone {
				lockKey := fmt.Sprintf(backfillLockKeyTemplate, taskPO.ID)
				locked, _, cancel, lockErr := h.locker.LockWithRenew(ctx, lockKey, transformTaskStatusLockTTL, backfillLockMaxHold)
				if lockErr != nil || !locked {
					_ = h.sendBackfillMessage(ctx, &entity.BackFillEvent{
						TaskID:  taskPO.ID,
						SpaceID: taskPO.WorkspaceID,
					})
				}
				defer cancel()
			}
		} else if taskPO.BackfillEffectiveTime != nil && backfillTaskRun != nil {
			if backfillTaskRun.RunStatus == task.RunStatusDone {
				logs.CtxInfo(ctx, "[OnFinishTaskChange]taskID:%d, backfillTaskRun.RunStatus == task.RunStatusDone", taskPO.ID)
				err = proc.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
					Task:     taskPO,
					TaskRun:  backfillTaskRun,
					IsFinish: true,
				})
				if err != nil {
					logs.CtxError(ctx, "OnFinishTaskChange err:%v", err)
					continue
				}
			}
			if backfillTaskRun.RunStatus != task.RunStatusDone {
				lockKey := fmt.Sprintf(backfillLockKeyTemplate, taskPO.ID)
				locked, _, cancel, lockErr := h.locker.LockWithRenew(ctx, lockKey, transformTaskStatusLockTTL, backfillLockMaxHold)
				if lockErr != nil || !locked {
					_ = h.sendBackfillMessage(ctx, &entity.BackFillEvent{
						TaskID:  taskPO.ID,
						SpaceID: taskPO.WorkspaceID,
					})
				}
				defer cancel()
			}
		} else if taskPO.EffectiveTime != nil {
			if time.Now().After(endTime) {
				logs.CtxInfo(ctx, "[OnFinishTaskChange]taskID:%d, time.Now().After(endTime)", taskPO.ID)
				err = proc.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
					Task:     taskPO,
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
		if taskPO.TaskStatus == task.TaskStatusUnstarted && time.Now().After(startTime) {
			if !taskPO.Sampler.IsCycle {
				err = proc.OnCreateTaskRunChange(ctx, taskexe.OnCreateTaskRunChangeReq{
					CurrentTask: taskPO,
					RunType:     task.TaskRunTypeNewData,
					RunStartAt:  taskPO.EffectiveTime.StartAt,
					RunEndAt:    taskPO.EffectiveTime.EndAt,
				})
				if err != nil {
					logs.CtxError(ctx, "OnCreateTaskRunChange err:%v", err)
					continue
				}
				err = proc.OnUpdateTaskChange(ctx, taskPO, task.TaskStatusRunning)
				if err != nil {
					logs.CtxError(ctx, "OnUpdateTaskChange err:%v", err)
					continue
				}
			} else {
				err = proc.OnCreateTaskRunChange(ctx, taskexe.OnCreateTaskRunChangeReq{
					CurrentTask: taskPO,
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
		if taskPO.TaskStatus == task.TaskStatusRunning || taskPO.TaskStatus == task.TaskStatusPending {
			if taskRun == nil {
				logs.CtxError(ctx, "taskID:%d, taskRun is nil", taskPO.ID)
				continue
			}
			logs.CtxInfo(ctx, "taskID:%d, taskRun.RunEndAt:%v", taskPO.ID, taskRun.RunEndAt)
			// Handling repeated tasks: single task time horizon reached
			if time.Now().After(taskRun.RunEndAt) {
				logs.CtxInfo(ctx, "[OnFinishTaskChange]taskID:%d, time.Now().After(cycleEndTime)", taskPO.ID)
				err = proc.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
					Task:     taskPO,
					TaskRun:  taskRun,
					IsFinish: false,
				})
				if err != nil {
					logs.CtxError(ctx, "OnFinishTaskChange err:%v", err)
					continue
				}
				if taskPO.Sampler.IsCycle {
					err = proc.OnCreateTaskRunChange(ctx, taskexe.OnCreateTaskRunChangeReq{
						CurrentTask: taskPO,
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
	if slices.Contains([]string{TracehubClusterName, InjectClusterName}, os.Getenv(TceCluster)) {
		return
	}
	ctx := context.Background()
	ctx = h.fillCtx(ctx)

	if h.locker != nil {
		locked, lockErr := h.locker.Lock(ctx, syncTaskRunCountsLockKey, transformTaskStatusLockTTL)
		if lockErr != nil {
			logs.CtxError(ctx, "syncTaskRunCounts acquire lock failed", "err", lockErr)
			return
		}
		if !locked {
			logs.CtxInfo(ctx, "syncTaskRunCounts lock held by others, skip execution")
			return
		}
	}
	logs.CtxInfo(ctx, "Start syncing TaskRunCounts to database...")
	// 1. Retrieve non-final task list
	taskDOs, err := h.listSyncTaskRunTask(ctx)
	if err != nil {
		logs.CtxError(ctx, "Failed to get non-final task list", "err", err)
		return
	}
	if len(taskDOs) == 0 {
		logs.CtxInfo(ctx, "No non-final tasks need syncing")
		return
	}

	// 2. Collect all TaskRun information that needs syncing
	var taskRunInfos []*TaskRunCountInfo
	for _, taskPO := range taskDOs {
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

func (h *TraceHubServiceImpl) syncTaskCache() {
	ctx := context.Background()
	ctx = h.fillCtx(ctx)

	logs.CtxInfo(ctx, "Start syncing task cache...")

	// 1. Retrieve spaceID, botID, and task information for all non-final tasks from the database
	spaceIDs, botIDs, tasks := h.taskRepo.GetObjListWithTask(ctx)
	logs.CtxInfo(ctx, "Retrieved task information, taskCount:%d, spaceCount:%d, botCount:%d", len(tasks), len(spaceIDs), len(botIDs))

	// 2. Build a new cache map
	newCache := TaskCacheInfo{
		WorkspaceIDs: spaceIDs,
		BotIDs:       botIDs,
		Tasks:        tasks,
		UpdateTime:   time.Now(), // Set the current time as the update time
	}

	// 3. Clear old cache and update with new cache
	h.taskCacheLock.Lock()
	defer h.taskCacheLock.Unlock()

	// 4. Write new cache into local cache
	h.taskCache.Store("ObjListWithTask", newCache)

	logs.CtxInfo(ctx, "Task cache sync completed, taskCount:%d, updateTime:%s", len(tasks), newCache.UpdateTime.Format(time.RFC3339))
}

// processBatch synchronizes TaskRun counts in batches
func (h *TraceHubServiceImpl) processBatch(ctx context.Context, batch []*TaskRunCountInfo) {
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
		} else {
			info.TaskRunSuccCount = successCount
		}

		// Read taskrun fail count
		failCount, err := h.taskRepo.GetTaskRunFailCount(ctx, info.TaskID, info.TaskRunID)
		if err != nil || failCount == -1 {
			logs.CtxWarn(ctx, "Failed to get TaskRunFailCount, taskID:%d, taskRunID:%d, err:%v", info.TaskID, info.TaskRunID, err)
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
	logs.CtxInfo(ctx, "Start updating TaskRun detail in batch, batchSize:%d, batch:%v", len(batch), batch)
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

	logs.CtxInfo(ctx, "Batch processing completed, batchSize:%d", len(batch))
}

// updateTaskRunDetail updates the run_detail field of TaskRun
func (h *TraceHubServiceImpl) updateTaskRunDetail(ctx context.Context, info *TaskRunCountInfo) error {
	// Build run_detail JSON data
	runDetail := map[string]interface{}{
		"total_count":   info.TaskRunCount,
		"success_count": info.TaskRunSuccCount,
		"failed_count":  info.TaskRunFailCount,
	}

	// Update using optimistic locking
	err := h.taskRepo.UpdateTaskRunWithOCC(ctx, info.TaskRunID, 0, map[string]interface{}{
		"run_detail": ToJSONString(ctx, runDetail),
	})
	if err != nil {
		return errors.Wrap(err, "Failed to update TaskRun")
	}

	return nil
}

func (h *TraceHubServiceImpl) listNonFinalTaskByRedis(ctx context.Context, spaceID string) ([]*entity.ObservabilityTask, error) {
	var taskPOs []*entity.ObservabilityTask
	nonFinalTaskIDs, err := h.taskRepo.ListNonFinalTask(ctx, spaceID)
	if err != nil {
		logs.CtxError(ctx, "Failed to get non-final task list", "err", err)
		return nil, err
	}
	logs.CtxInfo(ctx, "Start listing non-final tasks, taskCount:%d, nonFinalTaskIDs:%v", len(nonFinalTaskIDs), nonFinalTaskIDs)
	if len(nonFinalTaskIDs) == 0 {
		return taskPOs, nil
	}
	for _, taskID := range nonFinalTaskIDs {
		taskPO, err := h.taskRepo.GetTaskByRedis(ctx, taskID)
		if err != nil {
			logs.CtxError(ctx, "Failed to get task", "err", err)
			return nil, err
		}
		if taskPO == nil {
			continue
		}
		taskPOs = append(taskPOs, taskPO)
	}
	return taskPOs, nil
}

func (h *TraceHubServiceImpl) listNonFinalTask(ctx context.Context) ([]*entity.ObservabilityTask, error) {
	var taskPOs []*entity.ObservabilityTask
	var offset int32 = 0
	const limit int32 = 500
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
			return nil, err
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
	return taskPOs, nil
}

func (h *TraceHubServiceImpl) listSyncTaskRunTask(ctx context.Context) ([]*entity.ObservabilityTask, error) {
	var taskDOs []*entity.ObservabilityTask
	taskDOs, err := h.listNonFinalTask(ctx)
	if err != nil {
		logs.CtxError(ctx, "Failed to get non-final task list", "err", err)
		return nil, err
	}
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
							string(task.TaskStatusSuccess),
							string(task.TaskStatusDisabled),
						},
						QueryType: ptr.Of(filter.QueryTypeIn),
						FieldType: ptr.Of(filter.FieldTypeString),
					},
					{
						FieldName: ptr.Of("updated_at"),
						Values: []string{
							fmt.Sprintf("%d", time.Now().Add(-24*time.Hour).UnixMilli()),
						},
						QueryType: ptr.Of(filter.QueryTypeGt),
						FieldType: ptr.Of(filter.FieldTypeLong),
					},
				},
			},
		})
		if err != nil {
			logs.CtxError(ctx, "Failed to get non-final task list", "err", err)
			break
		}

		// Add tasks from the current page to the full list
		taskDOs = append(taskDOs, tasklist...)

		// If fewer tasks than limit are returned, this is the last page
		if len(tasklist) < int(limit) {
			break
		}

		// Move to the next page, increasing offset by 1000
		offset += limit
	}
	return taskDOs, nil
}
