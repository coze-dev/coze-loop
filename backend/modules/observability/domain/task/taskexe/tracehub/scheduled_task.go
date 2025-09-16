// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
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
}