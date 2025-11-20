// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package scheduledtask

import (
	"context"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/scheduledtask"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/tracehub"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
)

type LocalCacheRefreshTask struct {
	*scheduledtask.BaseScheduledTask

	traceHubService tracehub.ITraceHubService
	taskRepo        repo.ITaskRepo
}

func NewLocalCacheRefreshTask(traceHubService tracehub.ITraceHubService, taskRepo repo.ITaskRepo) scheduledtask.ScheduledTask {
	t := &LocalCacheRefreshTask{
		BaseScheduledTask: scheduledtask.NewBaseScheduledTask("LocalCacheRefreshTask", 2*time.Minute),
		traceHubService:   traceHubService,
		taskRepo:          taskRepo,
	}
	t.BaseScheduledTask.ScheduledTask = t
	return t
}

func (t *LocalCacheRefreshTask) RunOnce(ctx context.Context) error {
	logs.CtxInfo(ctx, "Start syncing task cache...")

	// 1. Retrieve spaceID, botID, and task information for all non-final tasks from the database
	spaceIDs, botIDs, tasks, err := t.getNonFinalTaskInfos(ctx)
	if err != nil {
		logs.CtxError(ctx, "Failed to get non-final task list", "err", err)
		return err
	}
	logs.CtxInfo(ctx, "Retrieved task information, taskCount:%d, spaceCount:%d, botCount:%d", len(tasks), len(spaceIDs), len(botIDs))

	if err := t.traceHubService.StoneTaskCache(ctx, tracehub.TaskCacheInfo{
		WorkspaceIDs: spaceIDs,
		BotIDs:       botIDs,
		Tasks:        tasks,
		UpdateTime:   time.Now(), // Set the current time as the update time
	}); err != nil {
		logs.CtxError(ctx, "Failed to update task cache, err:%v", err)
		return err
	}
	return nil
}

func (t *LocalCacheRefreshTask) getNonFinalTaskInfos(ctx context.Context) ([]string, []string, []*entity.ObservabilityTask, error) {
	tasks, err := t.taskRepo.ListNonFinalTasks(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	spaceMap := make(map[string]interface{})
	botMap := make(map[string]interface{})

	for _, task := range tasks {
		spaceMap[strconv.FormatInt(task.WorkspaceID, 10)] = struct{}{}
		if task.SpanFilter != nil && task.SpanFilter.Filters.FilterFields != nil {
			extractBotIDFromFilters(task.SpanFilter.Filters.FilterFields, botMap)
		}
	}

	return lo.Keys(spaceMap), lo.Keys(botMap), tasks, nil
}

// extractBotIDFromFilters 递归提取过滤器中的 bot_id 值，包括 SubFilter
func extractBotIDFromFilters(filterFields []*loop_span.FilterField, botMap map[string]interface{}) {
	for _, filterField := range filterFields {
		if filterField == nil {
			continue
		}
		// 检查当前 FilterField 的 FieldName
		if filterField.FieldName == "bot_id" {
			for _, v := range filterField.Values {
				botMap[v] = struct{}{}
			}
		}
		// 递归处理 SubFilter
		if filterField.SubFilter != nil && filterField.SubFilter.FilterFields != nil {
			extractBotIDFromFilters(filterField.SubFilter.FilterFields, botMap)
		}
	}
}
