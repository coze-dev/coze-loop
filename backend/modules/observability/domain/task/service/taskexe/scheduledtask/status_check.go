// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package scheduledtask

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/scheduledtask"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/tracehub"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/pkg/errors"
)

type TaskRunCountInfo struct {
	TaskID           int64
	TaskRunID        int64
	TaskRunCount     int64
	TaskRunSuccCount int64
	TaskRunFailCount int64
}

const (
	syncTaskRunCountLockTTL = 3 * time.Minute
	checkTaskStatusLockKey  = "observability:task:check_task_status"
	checkTaskStatusLockTTL  = 3 * time.Minute
	backfillLockKeyTemplate = "observability:tracehub:backfill:%d"
	backfillLockMaxHold     = 24 * time.Hour

	limit = 500
)

type StatusCheckTask struct {
	*scheduledtask.BaseScheduledTask

	config          config.ITraceConfig
	locker          lock.ILocker
	traceHubService tracehub.ITraceHubService
	taskService     service.ITaskService
	taskProcessor   processor.TaskProcessor
	taskRepo        repo.ITaskRepo
}

func NewStatusCheckTask(
	locker lock.ILocker,
	config config.ITraceConfig,
	traceHubService tracehub.ITraceHubService,
	taskService service.ITaskService,
	taskProcessor processor.TaskProcessor,
	taskRepo repo.ITaskRepo,
) scheduledtask.ScheduledTask {
	t := &StatusCheckTask{
		BaseScheduledTask: scheduledtask.NewBaseScheduledTask("StatusCheckTask", 5*time.Minute, false),
		locker:            locker,
		config:            config,
		traceHubService:   traceHubService,
		taskService:       taskService,
		taskProcessor:     taskProcessor,
		taskRepo:          taskRepo,
	}
	t.ScheduledTask = t
	return t
}

func (t *StatusCheckTask) RunOnce(ctx context.Context) error {
	cfg, err := t.config.GetConsumerListening(ctx)
	if err != nil {
		return err
	}
	if !cfg.IsEnabled || !cfg.IsAllSpace {
		return nil
	}

	if t.locker != nil {
		locked, lockErr := t.locker.Lock(ctx, checkTaskStatusLockKey, checkTaskStatusLockTTL)
		if lockErr != nil {
			logs.CtxError(ctx, "transformTaskStatus acquire lock failed", "err", lockErr)
			return lockErr
		}
		if !locked {
			logs.CtxInfo(ctx, "transformTaskStatus lock held by others, skip execution")
			return nil
		}
	}

	tasks, err := t.listTasks(ctx)
	if err != nil {
		logs.CtxError(ctx, "Failed to get tasks", "err", err)
		return err
	}
	logs.CtxInfo(ctx, "Got [%d] tasks", len(tasks))

	if err = t.checkTaskStatus(ctx, tasks); err != nil {
		logs.CtxError(ctx, "Failed to check task status", "err", err)
		return err
	}
	if err = t.syncTaskRunCount(ctx, tasks); err != nil {
		logs.CtxError(ctx, "Failed to sync task run count", "err", err)
		return err
	}

	return nil
}

func (t *StatusCheckTask) checkTaskStatus(ctx context.Context, tasks []*entity.ObservabilityTask) error {
	startTime := time.Now()
	logs.CtxInfo(ctx, "Check task status started...")
	for _, taskDO := range tasks {
		if taskDO.TaskStatus == entity.TaskStatusSuccess ||
			taskDO.TaskStatus == entity.TaskStatusFailed ||
			taskDO.TaskStatus == entity.TaskStatusDisabled {
			continue
		}

		var taskRun, backfillTaskRun *entity.TaskRun
		var err error
		backfillTaskRun = taskDO.GetBackfillTaskRun()
		taskRun = taskDO.GetCurrentTaskRun()
		var startTime, endTime time.Time

		if taskDO.EffectiveTime != nil {
			endTime = time.UnixMilli(taskDO.EffectiveTime.EndAt)
			startTime = time.UnixMilli(taskDO.EffectiveTime.StartAt)
		}
		proc := t.taskProcessor.GetTaskProcessor(taskDO.TaskType)
		// Task time horizon reached
		// End when the task end time is reached
		logs.CtxInfo(ctx, "[auto_task]taskID:%d, endTime:%v, startTime:%v", taskDO.ID, endTime, startTime)
		if taskDO.BackfillEffectiveTime != nil && taskDO.EffectiveTime != nil && backfillTaskRun != nil {
			if time.Now().After(endTime) && backfillTaskRun.RunStatus == entity.TaskRunStatusDone {
				logs.CtxInfo(ctx, "[OnTaskFinished]taskID:%d, time.Now().After(endTime) && backfillTaskRun.RunStatus == task.RunStatusDone", taskDO.ID)
				err = proc.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{
					Task:     taskDO,
					TaskRun:  backfillTaskRun,
					IsFinish: true,
				})
				if err != nil {
					logs.CtxError(ctx, "OnTaskFinished err:%v", err)
					continue
				}
			}
			if backfillTaskRun.RunStatus != entity.TaskRunStatusDone {
				if time.Now().Add(-backfillTaskRun.RunEndAt.Sub(backfillTaskRun.RunStartAt)).Before(backfillTaskRun.RunEndAt) {
					lockKey := fmt.Sprintf(backfillLockKeyTemplate, taskDO.ID)
					locked, _, cancel, lockErr := t.locker.LockWithRenew(ctx, lockKey, syncTaskRunCountLockTTL, backfillLockMaxHold)
					if lockErr != nil || !locked {
						_ = t.taskService.SendBackfillMessage(ctx, &entity.BackFillEvent{
							TaskID:  taskDO.ID,
							SpaceID: taskDO.WorkspaceID,
						})
					}
					defer cancel()
				} else {
					err = proc.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{
						Task:     taskDO,
						TaskRun:  backfillTaskRun,
						IsFinish: false,
					})
					if err != nil {
						logs.CtxError(ctx, "OnFinishTaskChange err:%v", err)
						continue
					}
				}
			}
		} else if taskDO.BackfillEffectiveTime != nil && backfillTaskRun != nil {
			if backfillTaskRun.RunStatus == entity.TaskRunStatusDone {
				logs.CtxInfo(ctx, "[OnTaskFinished]taskID:%d, backfillTaskRun.RunStatus == task.RunStatusDone", taskDO.ID)
				err = proc.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{
					Task:     taskDO,
					TaskRun:  backfillTaskRun,
					IsFinish: true,
				})
				if err != nil {
					logs.CtxError(ctx, "OnTaskFinished err:%v", err)
					continue
				}
			}
			if backfillTaskRun.RunStatus != entity.TaskRunStatusDone {
				if time.Now().Add(-backfillTaskRun.RunEndAt.Sub(backfillTaskRun.RunStartAt)).Before(backfillTaskRun.RunEndAt) {
					lockKey := fmt.Sprintf(backfillLockKeyTemplate, taskDO.ID)
					locked, _, cancel, lockErr := t.locker.LockWithRenew(ctx, lockKey, syncTaskRunCountLockTTL, backfillLockMaxHold)
					if lockErr != nil || !locked {
						_ = t.taskService.SendBackfillMessage(ctx, &entity.BackFillEvent{
							TaskID:  taskDO.ID,
							SpaceID: taskDO.WorkspaceID,
						})
					}
					defer cancel()
				} else {
					err = proc.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{
						Task:     taskDO,
						TaskRun:  backfillTaskRun,
						IsFinish: false,
					})
					if err != nil {
						logs.CtxError(ctx, "OnFinishTaskChange err:%v", err)
						continue
					}
				}
			}
		} else if taskDO.EffectiveTime != nil {
			if time.Now().After(endTime) {
				logs.CtxInfo(ctx, "[OnTaskFinished]taskID:%d, time.Now().After(endTime)", taskDO.ID)
				err = proc.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{
					Task:     taskDO,
					TaskRun:  taskRun,
					IsFinish: true,
				})
				if err != nil {
					logs.CtxError(ctx, "OnTaskFinished err:%v", err)
					continue
				}
			}
		}
		// If the task status is unstarted, create it once the task start time is reached
		if taskDO.TaskStatus == entity.TaskStatusUnstarted && time.Now().After(startTime) {
			if processor.ShouldTriggerNewData(ctx, taskDO) {
				runStartAt, runEndAt := taskDO.GetRunTimeRange()
				err = proc.OnTaskRunCreated(ctx, taskexe.OnTaskRunCreatedReq{
					CurrentTask: taskDO,
					RunType:     entity.TaskRunTypeNewData,
					RunStartAt:  runStartAt,
					RunEndAt:    runEndAt,
				})
				if err != nil {
					logs.CtxError(ctx, "OnCreateTaskRunChange err:%v", err)
					continue
				}
			}
			if processor.ShouldTriggerBackfill(taskDO) {
				runStartAt, runEndAt := taskDO.BackfillEffectiveTime.StartAt, taskDO.BackfillEffectiveTime.EndAt
				err = proc.OnTaskRunCreated(ctx, taskexe.OnTaskRunCreatedReq{
					CurrentTask: taskDO,
					RunType:     entity.TaskRunTypeBackFill,
					RunStartAt:  runStartAt,
					RunEndAt:    runEndAt,
				})
				if err != nil {
					logs.CtxError(ctx, "OnCreateTaskRunChange err:%v", err)
					continue
				}
			}

			err = proc.OnTaskUpdated(ctx, taskDO, entity.TaskStatusRunning)
			if err != nil {
				logs.CtxError(ctx, "OnUpdateTaskChange err:%v", err)
				continue
			}
		}
		// Handle taskRun
		if taskDO.TaskStatus == entity.TaskStatusRunning || taskDO.TaskStatus == entity.TaskStatusPending {
			if taskRun == nil {
				logs.CtxError(ctx, "taskID:%d, taskRun is nil", taskDO.ID)
				continue
			}
			logs.CtxInfo(ctx, "taskID:%d, taskRun.RunEndAt:%v", taskDO.ID, taskRun.RunEndAt)
			// Handling repeated tasks: single task time horizon reached
			if time.Now().After(taskRun.RunEndAt) {
				logs.CtxInfo(ctx, "[OnTaskFinished]taskID:%d, time.Now().After(cycleEndTime)", taskDO.ID)
				err = proc.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{
					Task:     taskDO,
					TaskRun:  taskRun,
					IsFinish: false,
				})
				if err != nil {
					logs.CtxError(ctx, "OnTaskFinished err:%v", err)
					continue
				}
				if taskDO.Sampler.IsCycle {
					err = proc.OnTaskRunCreated(ctx, taskexe.OnTaskRunCreatedReq{
						CurrentTask: taskDO,
						RunType:     entity.TaskRunTypeNewData,
						RunStartAt:  taskRun.RunEndAt.UnixMilli(),
						RunEndAt:    taskRun.RunEndAt.UnixMilli() + (taskRun.RunEndAt.UnixMilli() - taskRun.RunStartAt.UnixMilli()),
					})
					if err != nil {
						logs.CtxError(ctx, "OnTaskRunCreated err:%v", err)
						continue
					}
				}
			}
		}
	}

	logs.CtxInfo(ctx, "Check task status finished. Cost time:%v", time.Since(startTime))
	return nil
}

func (t *StatusCheckTask) syncTaskRunCount(ctx context.Context, tasks []*entity.ObservabilityTask) error {
	startTime := time.Now()
	logs.CtxInfo(ctx, "Start syncing TaskRunCounts to database...")

	// 2. Collect all TaskRun information that needs syncing
	var taskRunInfos []*TaskRunCountInfo
	for _, taskDO := range tasks {
		if len(taskDO.TaskRuns) == 0 {
			continue
		}

		for _, taskRun := range taskDO.TaskRuns {
			taskRunInfos = append(taskRunInfos, &TaskRunCountInfo{
				TaskID:    taskDO.ID,
				TaskRunID: taskRun.ID,
			})
		}
	}

	if len(taskRunInfos) == 0 {
		logs.CtxInfo(ctx, "No TaskRun requires syncing")
		return nil
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
		t.processBatch(ctx, batch)
	}

	logs.CtxInfo(ctx, "Finish syncing TaskRunCounts to database. Cost time:%v", time.Since(startTime))
	return nil
}

func (t *StatusCheckTask) listRecentTasks(ctx context.Context) ([]*entity.ObservabilityTask, error) {
	var taskDOs []*entity.ObservabilityTask

	var offset int32 = 0
	// Paginate through all tasks
	for {
		tasklist, _, err := t.taskRepo.ListTasks(ctx, repo.ListTaskParam{
			ReqLimit:  limit,
			ReqOffset: offset,
			TaskFilters: &entity.TaskFilterFields{
				FilterFields: []*entity.TaskFilterField{
					{
						FieldName: ptr.Of(entity.TaskFieldNameTaskStatus),
						Values: []string{
							string(entity.TaskStatusSuccess),
							string(entity.TaskStatusDisabled),
							string(entity.TaskStatusFailed),
						},
						QueryType: ptr.Of(entity.QueryTypeIn),
						FieldType: ptr.Of(entity.FieldTypeString),
					},
					{
						FieldName: ptr.Of(entity.TaskFieldName("updated_at")),
						Values: []string{
							fmt.Sprintf("%d", time.Now().Add(-24*time.Hour).UnixMilli()),
						},
						QueryType: ptr.Of(entity.QueryTypeGt),
						FieldType: ptr.Of(entity.FieldTypeLong),
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
		if len(tasklist) < limit {
			break
		}

		// Move to the next page, increasing offset by 1000
		offset += limit
	}
	logs.CtxInfo(ctx, "Get recent task list. Total count:%d", len(taskDOs))
	return taskDOs, nil
}

func (t *StatusCheckTask) listNonFinalTasks(ctx context.Context) ([]*entity.ObservabilityTask, error) {
	var taskPOs []*entity.ObservabilityTask
	var offset int32 = 0
	// Paginate through all tasks
	for {
		tasklist, _, err := t.taskRepo.ListTasks(ctx, repo.ListTaskParam{
			ReqLimit:  limit,
			ReqOffset: offset,
			TaskFilters: &entity.TaskFilterFields{
				FilterFields: []*entity.TaskFilterField{
					{
						FieldName: ptr.Of(entity.TaskFieldNameTaskStatus),
						Values: []string{
							string(entity.TaskStatusUnstarted),
							string(entity.TaskStatusRunning),
							string(entity.TaskStatusPending),
						},
						QueryType: ptr.Of(entity.QueryTypeIn),
						FieldType: ptr.Of(entity.FieldTypeString),
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
		if len(tasklist) < limit {
			break
		}

		// Move to the next page, increasing offset by 1000
		offset += limit
	}
	logs.CtxInfo(ctx, "Got non-final task list, total:%d", len(taskPOs))
	return taskPOs, nil
}

// processBatch synchronizes TaskRun counts in batches
func (t *StatusCheckTask) processBatch(ctx context.Context, batch []*TaskRunCountInfo) {
	// 1. Read Redis count data in batch
	for _, info := range batch {
		// Read taskruncount
		count, err := t.taskRepo.GetTaskRunCount(ctx, info.TaskID, info.TaskRunID)
		if err != nil || count == -1 {
			logs.CtxWarn(ctx, "Failed to get TaskRunCount, taskID:%d, taskRunID:%d, err:%v", info.TaskID, info.TaskRunID, err)
		} else {
			info.TaskRunCount = count
		}

		// Read taskrun success count
		successCount, err := t.taskRepo.GetTaskRunSuccessCount(ctx, info.TaskID, info.TaskRunID)
		if err != nil || successCount == -1 {
			logs.CtxWarn(ctx, "Failed to get TaskRunSuccessCount, taskID:%d, taskRunID:%d, err:%v", info.TaskID, info.TaskRunID, err)
		} else {
			info.TaskRunSuccCount = successCount
		}

		// Read taskrun fail count
		failCount, err := t.taskRepo.GetTaskRunFailCount(ctx, info.TaskID, info.TaskRunID)
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
		err := t.updateTaskRunDetail(ctx, info)
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
func (t *StatusCheckTask) updateTaskRunDetail(ctx context.Context, info *TaskRunCountInfo) error {
	// Build run_detail JSON data
	runDetail := map[string]interface{}{
		"total_count":   info.TaskRunCount,
		"success_count": info.TaskRunSuccCount,
		"failed_count":  info.TaskRunFailCount,
	}

	// Update using optimistic locking
	err := t.taskRepo.UpdateTaskRunWithOCC(ctx, info.TaskRunID, 0, map[string]interface{}{
		"run_detail": json.MarshalStringIgnoreErr(runDetail),
	})
	if err != nil {
		return errors.Wrap(err, "Failed to update TaskRun")
	}

	return nil
}

func (t *StatusCheckTask) listTasks(ctx context.Context) ([]*entity.ObservabilityTask, error) {
	tasks := make([]*entity.ObservabilityTask, 0)
	nonFinalTasks, err := t.listNonFinalTasks(ctx)
	if err != nil {
		return nil, err
	}
	tasks = append(tasks, nonFinalTasks...)

	recentTasks, err := t.listRecentTasks(ctx)
	if err != nil {
		return nil, err
	}
	tasks = append(tasks, recentTasks...)

	return tasks, nil
}
