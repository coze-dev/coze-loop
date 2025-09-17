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

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo"
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

	// TaskRunCount 同步任务
	h.syncTaskRunCounts(ctx, taskPOs)
}

// syncTaskRunCounts 全量同步TaskRunCount到数据库
func (h *TraceHubServiceImpl) syncTaskRunCounts(ctx context.Context, taskPOs []*entity.ObservabilityTask) {
	startTime := time.Now()
	logs.CtxInfo(ctx, "开始全量同步TaskRunCount到数据库")

	metrics := &SyncMetrics{}

	// 1. 获取Redis中所有TaskRunCount键
	keys, err := h.getAllTaskRunCountKeys(ctx)
	if err != nil {
		logs.CtxError(ctx, "获取TaskRunCount键失败", "err", err)
		return
	}

	metrics.TotalKeys = int64(len(keys))
	logs.CtxInfo(ctx, "发现TaskRunCount键数量", "count", len(keys))

	// 2. 批量处理键
	batchSize := 100 // 每批处理100个键
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		batch := keys[i:end]
		batchMetrics := h.processBatchTaskRunCounts(ctx, batch)

		// 累计统计
		metrics.SuccessCount += batchMetrics.SuccessCount
		metrics.FailureCount += batchMetrics.FailureCount
		metrics.SkippedCount += batchMetrics.SkippedCount
	}

	metrics.ProcessingTime = time.Since(startTime)
	h.recordSyncMetrics(ctx, metrics)

	logs.CtxInfo(ctx, "TaskRunCount全量同步完成")
}

// getAllTaskRunCountKeys 获取Redis中所有TaskRunCount键
func (h *TraceHubServiceImpl) getAllTaskRunCountKeys(ctx context.Context) ([]string, error) {
	// 扫描两种键格式
	patterns := []string{
		"count_*_*",         // Task模块格式：count_{taskID}_{taskRunID}
		"taskrun:count:*:*", // TaskRun模块格式：taskrun:count:{taskID}:{taskRunID}
	}

	var allKeys []string

	// 通过类型断言获取TaskRepoImpl来访问TaskRedisDao
	taskRepoImpl, ok := h.taskRepo.(*repo.TaskRepoImpl)
	if !ok {
		return nil, fmt.Errorf("无法获取TaskRepoImpl实例")
	}

	// 获取底层的Redis客户端
	rawClient, ok := redis.Unwrap(taskRepoImpl.TaskRedisDao.(redis.Cmdable))
	if !ok {
		return nil, fmt.Errorf("无法获取原始Redis客户端")
	}

	for _, pattern := range patterns {
		keys, err := rawClient.Keys(ctx, pattern).Result()
		if err != nil {
			logs.CtxError(ctx, "扫描Redis键失败", "pattern", pattern, "err", err)
			continue
		}
		allKeys = append(allKeys, keys...)
		logs.CtxDebug(ctx, "扫描到键", "pattern", pattern, "count", len(keys))
	}

	return allKeys, nil
}

// processBatchTaskRunCounts 批量处理TaskRunCount键
func (h *TraceHubServiceImpl) processBatchTaskRunCounts(ctx context.Context, keys []string) *SyncMetrics {
	metrics := &SyncMetrics{}

	// 使用map去重，避免同一个TaskRun被重复更新
	processedTaskRuns := make(map[int64]bool)

	for _, key := range keys {
		// 解析键获取taskID和taskRunID
		info, err := h.parseTaskRunCountKey(key)
		if err != nil {
			logs.CtxError(ctx, "解析键失败", "key", key, "err", err)
			metrics.FailureCount++
			continue
		}

		// 检查是否已处理过此TaskRun
		if processedTaskRuns[info.TaskRunID] {
			logs.CtxDebug(ctx, "TaskRun已处理，跳过", "taskRunID", info.TaskRunID, "key", key)
			metrics.SkippedCount++
			continue
		}

		// 获取Redis中的计数值
		var count int64
		if info.KeyType == "task" {
			count, err = h.taskRepo.GetTaskRunCount(ctx, info.TaskID, info.TaskRunID)
		} else {
			count, err = h.taskRunRepo.GetTaskRunCount(ctx, info.TaskID, info.TaskRunID)
		}

		if err != nil {
			logs.CtxError(ctx, "获取TaskRunCount失败", "key", key, "err", err)
			metrics.FailureCount++
			continue
		}

		// 跳过未缓存的数据（count为-1或0）
		if count <= 0 {
			logs.CtxDebug(ctx, "跳过无效计数", "key", key, "count", count)
			metrics.SkippedCount++
			continue
		}

		// 直接更新taskrun表
		if h.updateTaskRunDirectly(ctx, info.TaskID, info.TaskRunID, count) {
			metrics.SuccessCount++
			processedTaskRuns[info.TaskRunID] = true
		} else {
			metrics.FailureCount++
		}
	}

	return metrics
}

// parseTaskRunCountKey 解析TaskRunCount键
func (h *TraceHubServiceImpl) parseTaskRunCountKey(key string) (*TaskRunCountInfo, error) {
	// 解析 count_{taskID}_{taskRunID} 格式
	if strings.HasPrefix(key, "count_") {
		parts := strings.Split(key, "_")
		if len(parts) != 3 {
			return nil, fmt.Errorf("无效的task键格式: %s", key)
		}

		taskID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("解析taskID失败: %s", parts[1])
		}

		taskRunID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("解析taskRunID失败: %s", parts[2])
		}

		return &TaskRunCountInfo{
			TaskID:    taskID,
			TaskRunID: taskRunID,
			KeyType:   "task",
		}, nil
	}

	// 解析 taskrun:count:{taskID}:{taskRunID} 格式
	if strings.HasPrefix(key, "taskrun:count:") {
		parts := strings.Split(key, ":")
		if len(parts) != 4 {
			return nil, fmt.Errorf("无效的taskrun键格式: %s", key)
		}

		taskID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("解析taskID失败: %s", parts[2])
		}

		taskRunID, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("解析taskRunID失败: %s", parts[3])
		}

		return &TaskRunCountInfo{
			TaskID:    taskID,
			TaskRunID: taskRunID,
			KeyType:   "taskrun",
		}, nil
	}

	return nil, fmt.Errorf("未知的键格式: %s", key)
}

// updateTaskRunDirectly 直接更新TaskRun表
func (h *TraceHubServiceImpl) updateTaskRunDirectly(ctx context.Context, taskID, taskRunID, count int64) bool {
	// 1. 先获取现有的TaskRun记录
	taskRun, err := h.taskRunRepo.GetTaskRun(ctx, taskRunID, nil, &taskID)
	if err != nil {
		logs.CtxError(ctx, "获取TaskRun失败", "taskRunID", taskRunID, "err", err)
		return false
	}

	if taskRun == nil {
		logs.CtxWarn(ctx, "TaskRun不存在，跳过更新", "taskRunID", taskRunID, "taskID", taskID)
		return false
	}

	// 2. 构造新的RunDetail
	runDetail := h.buildRunDetailWithCount(count, taskRun)

	// 3. 更新TaskRun的RunDetail
	taskRun.RunDetail = &runDetail
	if err := h.taskRunRepo.UpdateTaskRun(ctx, taskRun); err != nil {
		logs.CtxError(ctx, "更新TaskRun失败", "taskRunID", taskRunID, "err", err)
		return false
	}

	logs.CtxInfo(ctx, "成功更新TaskRun", "taskRunID", taskRunID, "taskID", taskID, "count", count)
	return true
}

// buildRunDetailWithCount 构造包含计数的RunDetail
func (h *TraceHubServiceImpl) buildRunDetailWithCount(count int64, taskRun *entity.TaskRun) string {
	detail := map[string]interface{}{
		"task_run_count": count,
		"updated_at":     time.Now().Format(time.RFC3339),
		"status":         taskRun.RunStatus,
		"sync_source":    "redis_full_sync", // 标识数据来源
	}

	// 如果已有RunDetail，尝试合并现有数据
	if taskRun.RunDetail != nil && *taskRun.RunDetail != "" {
		var existing map[string]interface{}
		if err := json.Unmarshal([]byte(*taskRun.RunDetail), &existing); err == nil {
			// 保留现有字段，但覆盖关键字段
			for k, v := range existing {
				if k != "task_run_count" && k != "updated_at" && k != "sync_source" {
					detail[k] = v
				}
			}
		}
	}

	jsonData, err := json.Marshal(detail)
	if err != nil {
		// 如果JSON序列化失败，返回简单格式
		logs.CtxWarn(context.Background(), "JSON序列化失败，使用简单格式", "err", err)
		return fmt.Sprintf(`{"task_run_count":%d,"updated_at":"%s","sync_source":"redis_full_sync"}`,
			count, time.Now().Format(time.RFC3339))
	}

	return string(jsonData)
}

// recordSyncMetrics 记录同步统计指标
func (h *TraceHubServiceImpl) recordSyncMetrics(ctx context.Context, metrics *SyncMetrics) {
	logs.CtxInfo(ctx, "同步统计",
		"totalKeys", metrics.TotalKeys,
		"success", metrics.SuccessCount,
		"failure", metrics.FailureCount,
		"skipped", metrics.SkippedCount,
		"duration", metrics.ProcessingTime)
}
