// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// TaskRunCountInfo TaskRunCount信息结构
type TaskRunCountInfo struct {
	TaskID    int64
	TaskRunID int64
	Count     int64
	KeyType   string // "task" 或 "taskrun"
}

// SyncMetrics 同步统计指标
type SyncMetrics struct {
	TotalKeys      int64
	SuccessCount   int64
	FailureCount   int64
	SkippedCount   int64
	ProcessingTime time.Duration
}

// startScheduledTask 启动定时任务goroutine
func (h *TraceHubServiceImpl) startScheduledTask() {
	go func() {
		for {
			select {
			case <-h.ticker.C:
				// 执行定时任务
				h.runScheduledTask()
			case <-h.stopChan:
				// 停止定时任务
				h.ticker.Stop()
				return
			}
		}
	}()
}

func (h *TraceHubServiceImpl) runScheduledTask() {
	ctx := context.Background()
	logs.CtxInfo(ctx, "定时任务开始执行...")
	logID := logs.NewLogID()
	ctx = logs.SetLogID(ctx, logID)
	ctx = context.WithValue(ctx, "K_ENV", "boe_auto_task")
	// 读取所有非终态（成功/禁用）任务
	taskPOs, err := h.taskRepo.ListNonFinalTask(ctx)
	if err != nil {
		logs.CtxError(ctx, "ListNonFinalTask err:%v", err)
		return
	}
	var tasks []*task.Task
	taskRunstat := make(map[int64]bool)
	logs.CtxInfo(ctx, "定时任务获取到任务数量:%d", len(tasks))
	for _, taskPO := range taskPOs {
		tasks = append(tasks, tconv.TaskPO2DTO(ctx, taskPO, nil))

		// 计算 taskRunstat：只有当所有 run 都为 done 状态时才为 true
		allRunsDone := true
		if len(taskPO.TaskRuns) == 0 {
			// 如果没有 TaskRuns，则认为未完成
			allRunsDone = false
		} else {
			// 检查所有 TaskRuns 是否都为 done 状态
			for _, taskRun := range taskPO.TaskRuns {
				if taskRun.RunStatus != task.RunStatusDone {
					allRunsDone = false
					break
				}
			}
		}

		taskRunstat[taskPO.ID] = allRunsDone
	}
	logs.CtxInfo(ctx, "taskPOs:%v", taskPOs)
	logs.CtxInfo(ctx, "taskRunstat:%v", taskRunstat)
	// 遍历任务
	for _, taskInfo := range tasks {
		endTime := time.Unix(0, taskInfo.GetRule().GetEffectiveTime().GetEndAt()*int64(time.Millisecond))
		startTime := time.Unix(0, taskInfo.GetRule().GetEffectiveTime().GetStartAt()*int64(time.Millisecond))
		proc, err := processor.NewProcessor(ctx, taskInfo.TaskType)
		if err != nil {
			logs.CtxError(ctx, "NewProcessor err:%v", err)
			continue
		}
		// 达到任务时间期限
		// 到任务结束时间就结束
		logs.CtxInfo(ctx, "[auto_task]taskID:%d, endTime:%v, startTime:%v", taskInfo.GetID(), endTime, startTime)
		if time.Now().After(endTime) && taskRunstat[*taskInfo.ID] {
			updateMap := map[string]interface{}{
				"task_status": task.TaskStatusSuccess,
			}
			err = h.taskRepo.UpdateTaskWithOCC(ctx, taskInfo.GetID(), taskInfo.GetWorkspaceID(), updateMap)
			if err != nil {
				logs.CtxError(ctx, "[auto_task] UpdateTask err:%v", err)
				continue
			}
		}
		// 如果任务状态为unstarted，到任务开始时间就开始create
		if taskInfo.GetTaskStatus() == task.TaskStatusUnstarted && time.Now().After(startTime) {
			err = proc.OnChangeProcessor(ctx, taskInfo, task.TaskStatusUnstarted)
			if err != nil {
				logs.CtxError(ctx, "OnChangeProcessor err:%v", err)
				continue
			}
		}
	}
	h.syncTaskRunCounts()
}

// syncTaskRunCounts 同步TaskRunCount到数据库
func (h *TraceHubServiceImpl) syncTaskRunCounts() {
	ctx := context.Background()
	logs.CtxInfo(ctx, "开始同步TaskRunCount数据到数据库")

	// 1. 获取所有TaskRunCount键
	keys, err := h.taskRepo.GetAllTaskRunCountKeys(ctx)
	if err != nil {
		logs.CtxError(ctx, "获取TaskRunCount键失败: %v", err)
		return
	}

	if len(keys) == 0 {
		logs.CtxInfo(ctx, "没有找到TaskRunCount键，跳过同步")
		return
	}

	logs.CtxInfo(ctx, "找到%d个TaskRunCount键，开始同步", len(keys))

	// 2. 批量处理键
	batchSize := 50 // 每批处理50个键
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		batch := keys[i:end]
		h.processBatch(ctx, batch)
	}
}

// processBatch 批量处理TaskRunCount键
func (h *TraceHubServiceImpl) processBatch(ctx context.Context, keys []string) {
	// 1. 解析键并获取计数信息
	var taskRunInfos []*TaskRunCountInfo

	for _, key := range keys {
		info, err := h.parseTaskRunCountKey(ctx, key)
		if err != nil {
			logs.CtxWarn(ctx, "解析键失败: key=%s, err=%v", key, err)
			continue
		}

		// 获取Redis中的计数值
		count, err := h.taskRepo.GetTaskRunCount(ctx, info.TaskID, info.TaskRunID)
		if err != nil {
			logs.CtxWarn(ctx, "获取TaskRunCount失败: taskID=%d, taskRunID=%d, err=%v",
				info.TaskID, info.TaskRunID, err)
			continue
		}

		// 如果计数为-1，表示Redis中不存在该键，跳过
		if count == -1 {
			logs.CtxDebug(ctx, "Redis中不存在键: taskID=%d, taskRunID=%d", info.TaskID, info.TaskRunID)
			continue
		}

		info.Count = count
		taskRunInfos = append(taskRunInfos, info)
	}

	// 2. 批量更新数据库
	for _, info := range taskRunInfos {
		if err := h.syncSingleTaskRunCount(ctx, info); err != nil {
			logs.CtxError(ctx, "同步TaskRunCount失败: taskID=%d, taskRunID=%d, count=%d, err=%v",
				info.TaskID, info.TaskRunID, info.Count, err)
		} else {
			logs.CtxDebug(ctx, "同步TaskRunCount成功: taskID=%d, taskRunID=%d, count=%d",
				info.TaskID, info.TaskRunID, info.Count)
		}
	}
}

// parseTaskRunCountKey 解析TaskRunCount键获取taskID和taskRunID
func (h *TraceHubServiceImpl) parseTaskRunCountKey(ctx context.Context, key string) (*TaskRunCountInfo, error) {
	// 键格式: count_{taskID}_{taskRunID}
	parts := strings.Split(key, "_")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid key format: %s", key)
	}

	taskID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid taskID in key %s: %v", key, err)
	}

	taskRunID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid taskRunID in key %s: %v", key, err)
	}

	return &TaskRunCountInfo{
		TaskID:    taskID,
		TaskRunID: taskRunID,
		KeyType:   "taskrun",
	}, nil
}

// syncSingleTaskRunCount 同步单个TaskRunCount到数据库
func (h *TraceHubServiceImpl) syncSingleTaskRunCount(ctx context.Context, info *TaskRunCountInfo) error {
	// 获取TaskRun记录
	taskRun, err := h.taskRunRepo.GetTaskRun(ctx, info.TaskRunID, nil, nil)
	if err != nil {
		return fmt.Errorf("获取TaskRun记录失败: taskRunID=%d, err=%v", info.TaskRunID, err)
	}

	if taskRun == nil {
		return fmt.Errorf("TaskRun记录不存在: taskRunID=%d", info.TaskRunID)
	}

	// 处理RunDetail
	var runDetail *task.RunDetail
	if taskRun.RunDetail != nil && *taskRun.RunDetail != "" {
		runDetail = &task.RunDetail{}
		if err := json.Unmarshal([]byte(*taskRun.RunDetail), runDetail); err != nil {
			logs.CtxWarn(ctx, "反序列化RunDetail失败，创建新的: taskRunID=%d, err=%v", info.TaskRunID, err)
			runDetail = &task.RunDetail{}
		}
	} else {
		runDetail = &task.RunDetail{}
	}

	// 更新TotalCount
	runDetail.TotalCount = &info.Count

	// 获取并更新成功计数
	successCount, err := h.taskRepo.GetTaskRunSuccessCount(ctx, info.TaskID, info.TaskRunID)
	if err != nil {
		logs.CtxWarn(ctx, "获取TaskRunSuccessCount失败: taskID=%d, taskRunID=%d, err=%v", 
			info.TaskID, info.TaskRunID, err)
	} else if successCount >= 0 {
		// 只有当获取成功且计数>=0时才更新
		runDetail.SuccessCount = &successCount
		logs.CtxDebug(ctx, "成功获取SuccessCount: taskID=%d, taskRunID=%d, count=%d", 
			info.TaskID, info.TaskRunID, successCount)
	}

	// 获取并更新失败计数
	failedCount, err := h.taskRepo.GetTaskRunFailCount(ctx, info.TaskID, info.TaskRunID)
	if err != nil {
		logs.CtxWarn(ctx, "获取TaskRunFailCount失败: taskID=%d, taskRunID=%d, err=%v", 
			info.TaskID, info.TaskRunID, err)
	} else if failedCount >= 0 {
		// 只有当获取成功且计数>=0时才更新
		runDetail.FailedCount = &failedCount
		logs.CtxDebug(ctx, "成功获取FailedCount: taskID=%d, taskRunID=%d, count=%d", 
			info.TaskID, info.TaskRunID, failedCount)
	}

	// 序列化RunDetail
	runDetailJSON, err := json.Marshal(runDetail)
	if err != nil {
		return fmt.Errorf("序列化RunDetail失败: taskRunID=%d, err=%v", info.TaskRunID, err)
	}

	// 更新数据库
	updateMap := map[string]interface{}{
		"run_detail": string(runDetailJSON),
		"updated_at": time.Now(),
	}

	err = h.taskRunRepo.UpdateTaskRunWithOCC(ctx, info.TaskRunID, taskRun.WorkspaceID, updateMap)
	if err != nil {
		return fmt.Errorf("更新TaskRun记录失败: taskRunID=%d, err=%v", info.TaskRunID, err)
	}

	logs.CtxInfo(ctx, "成功更新TaskRun的run_detail: taskRunID=%d, totalCount=%d, successCount=%v, failedCount=%v", 
		info.TaskRunID, info.Count, runDetail.SuccessCount, runDetail.FailedCount)
	return nil
}