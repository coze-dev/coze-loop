// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

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

// syncTaskRunCounts 同步TaskRunCount到数据库
func (h *TraceHubServiceImpl) syncTaskRunCounts(ctx context.Context, taskPOs []*entity.ObservabilityTask) {
	logs.CtxInfo(ctx, "开始同步TaskRunCount到数据库")
	
	for _, taskPO := range taskPOs {
		h.processTaskRuns(ctx, taskPO)
	}
	
	logs.CtxInfo(ctx, "TaskRunCount同步完成")
}

// processTaskRuns 处理单个任务的所有TaskRun
func (h *TraceHubServiceImpl) processTaskRuns(ctx context.Context, task *entity.ObservabilityTask) {
	if len(task.TaskRuns) == 0 {
		logs.CtxDebug(ctx, "任务无TaskRuns，跳过, taskID=%d", task.ID)
		return
	}
	
	logs.CtxDebug(ctx, "处理任务TaskRuns, taskID=%d, taskRunCount=%d", task.ID, len(task.TaskRuns))
	
	for _, taskRun := range task.TaskRuns {
		// 只处理非终态的TaskRun
		if h.isTaskRunFinal(taskRun.RunStatus) {
			logs.CtxDebug(ctx, "TaskRun已为终态，跳过, taskRunID=%d, status=%s", taskRun.ID, taskRun.RunStatus)
			continue
		}
		
		h.updateTaskRunWithCount(ctx, task.ID, taskRun)
	}
}

// updateTaskRunWithCount 更新单个TaskRun的RunDetail
func (h *TraceHubServiceImpl) updateTaskRunWithCount(ctx context.Context, taskID int64, taskRun *entity.TaskRun) {
	// 从Redis获取TaskRunCount
	count, err := h.taskRepo.GetTaskRunCount(ctx, taskID, taskRun.ID)
	if err != nil {
		logs.CtxError(ctx, "获取TaskRunCount失败, taskID=%d, taskRunID=%d, err=%v", taskID, taskRun.ID, err)
		return
	}
	
	// 构造RunDetail
	runDetail := h.buildRunDetail(count, taskRun)
	
	// 更新TaskRun的RunDetail
	taskRun.RunDetail = &runDetail
	if err := h.taskRunRepo.UpdateTaskRun(ctx, taskRun); err != nil {
		logs.CtxError(ctx, "更新TaskRun失败, taskRunID=%d, err=%v", taskRun.ID, err)
		return
	}
	
	logs.CtxInfo(ctx, "成功更新TaskRun, taskRunID=%d, taskID=%d, count=%d", taskRun.ID, taskID, count)
}

// buildRunDetail 构造RunDetail JSON字符串
func (h *TraceHubServiceImpl) buildRunDetail(count int64, taskRun *entity.TaskRun) string {
	detail := map[string]interface{}{
		"task_run_count": count,
		"updated_at":     time.Now().Format(time.RFC3339),
		"status":         taskRun.RunStatus,
	}
	
	// 如果已有RunDetail，尝试合并现有数据
	if taskRun.RunDetail != nil && *taskRun.RunDetail != "" {
		var existing map[string]interface{}
		if err := json.Unmarshal([]byte(*taskRun.RunDetail), &existing); err == nil {
			// 保留现有字段，但覆盖关键字段
			for k, v := range existing {
				if k != "task_run_count" && k != "updated_at" {
					detail[k] = v
				}
			}
		}
	}
	
	jsonData, err := json.Marshal(detail)
	if err != nil {
		// 如果JSON序列化失败，返回简单格式
		logs.CtxWarn(context.Background(), "JSON序列化失败，使用简单格式, err=%v", err)
		return fmt.Sprintf(`{"task_run_count":%d,"updated_at":"%s"}`, count, time.Now().Format(time.RFC3339))
	}
	
	return string(jsonData)
}

// isTaskRunFinal 判断TaskRun是否为终态
func (h *TraceHubServiceImpl) isTaskRunFinal(status string) bool {
	finalStatuses := []string{
		"done",
		"failed", 
		"canceled",
	}
	
	for _, finalStatus := range finalStatuses {
		if status == finalStatus {
			return true
		}
	}
	
	return false
}