// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bytedance/gg/gslice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

func (h *TraceHubServiceImpl) SpanTrigger(ctx context.Context, rawSpan *entity.RawSpan) error {
	ctx = h.fillCtx(ctx)
	logSuffix := fmt.Sprintf("log_id=%s, trace_id=%s, span_id=%s", rawSpan.LogID, rawSpan.TraceID, rawSpan.SpanID)
	logs.CtxInfo(ctx, "auto_task start, log_suffix=%s", logSuffix)
	// 1、Convert to standard span and perform initial filtering based on space_id
	span := rawSpan.RawSpanConvertToLoopSpan()
	// 1.1 Filter out spans that do not belong to any space or bot
	spaceIDs, botIDs, _ := h.getObjListWithTaskFromCache(ctx)
	logs.CtxInfo(ctx, "space list: %v, bot list: %v, log_suffix=%s", spaceIDs, botIDs, logSuffix)
	if !gslice.Contains(spaceIDs, span.WorkspaceID) && !gslice.Contains(botIDs, span.TagsString["bot_id"]) {
		logs.CtxInfo(ctx, "no space or bot found for span, space_id=%s,bot_id=%s, log_suffix=%s", span.WorkspaceID, span.TagsString["bot_id"], logSuffix)
		return nil
	}
	// 1.2 Filter out spans of type Evaluator
	if gslice.Contains([]string{"Evaluator"}, span.CallType) {
		return nil
	}
	// 2、Match spans against task rules
	subs, err := h.getSubscriberOfSpan(ctx, span)
	if err != nil {
		logs.CtxWarn(ctx, "get subscriber of flow span failed, %s, err: %v", logSuffix, err)
	}

	logs.CtxInfo(ctx, "%d subscriber of flow span found, %s", len(subs), logSuffix)
	if len(subs) == 0 {
		return nil
	}
	// 3、Sample
	subs = gslice.Filter(subs, func(sub *spanSubscriber) bool { return sub.Sampled() })
	logs.CtxInfo(ctx, "%d subscriber of flow span sampled, %s", len(subs), logSuffix)
	if len(subs) == 0 {
		return nil
	}
	// 3. PreDispatch
	err = h.preDispatch(ctx, span, subs)
	if err != nil {
		logs.CtxWarn(ctx, "preDispatch flow span failed, %s, err: %v", logSuffix, err)
	}
	logs.CtxInfo(ctx, "%d preDispatch success, %v", len(subs), subs)
	// 4、Dispatch
	if err = h.dispatch(ctx, span, subs); err != nil {
		logs.CtxWarn(ctx, "dispatch flow span failed, %s, err: %v", logSuffix, err)
		return err
	}
	return nil
}

func (h *TraceHubServiceImpl) getSubscriberOfSpan(ctx context.Context, span *loop_span.Span) ([]*spanSubscriber, error) {
	var subscribers []*spanSubscriber
	taskDOs, err := h.listNonFinalTask(ctx)
	if err != nil {
		logs.CtxError(ctx, "Failed to get non-final task list, err: %v", err)
		return nil, err
	}
	taskList := tconv.TaskDOs2DTOs(ctx, taskDOs, nil)
	for _, taskDO := range taskList {
		proc := h.taskProcessor.GetTaskProcessor(taskDO.TaskType)
		subscribers = append(subscribers, &spanSubscriber{
			taskID:           taskDO.GetID(),
			RWMutex:          sync.RWMutex{},
			t:                taskDO,
			processor:        proc,
			bufCap:           0,
			flushWait:        sync.WaitGroup{},
			maxFlushInterval: time.Second * 5,
			taskRepo:         h.taskRepo,
			runType:          task.TaskRunTypeNewData,
			buildHelper:      h.buildHelper,
		})
	}

	var (
		merr = &multierror.Error{}
		keep int
	)
	// Match data according to detailed filter rules
	for _, s := range subscribers {
		ok, err := s.Match(ctx, span)
		logs.CtxInfo(ctx, "Match span, task_id=%d, trace_id=%s, span_id=%s, ok=%v, err=%v", s.taskID, span.TraceID, span.SpanID, ok, err)
		if err != nil {
			merr = multierror.Append(merr, errors.WithMessagef(err, "match span,task_id=%d, trace_id=%s, span_id=%s", s.taskID, span.TraceID, span.SpanID))
			continue
		}
		if ok {
			subscribers[keep] = s
			keep++
		}
	}
	return subscribers[:keep], merr.ErrorOrNil()
}

func (h *TraceHubServiceImpl) preDispatch(ctx context.Context, span *loop_span.Span, subs []*spanSubscriber) error {
	merr := &multierror.Error{}
	for _, sub := range subs {
		if span.StartTime < sub.t.GetRule().GetEffectiveTime().GetStartAt() {
			logs.CtxWarn(ctx, "span start time is before task cycle start time, trace_id=%s, span_id=%s", span.TraceID, span.SpanID)
			continue
		}
		// First step: lock for task status change
		// Task run status
		var runStartAt, runEndAt int64
		if sub.t.GetTaskStatus() == task.TaskStatusUnstarted {
			logs.CtxWarn(ctx, "task is unstarted, need sub.Creative")
			runStartAt = sub.t.GetRule().GetEffectiveTime().GetStartAt()
			if !sub.t.GetRule().GetSampler().GetIsCycle() {
				runEndAt = sub.t.GetRule().GetEffectiveTime().GetEndAt()
			} else {
				switch *sub.t.GetRule().GetSampler().CycleTimeUnit {
				case task.TimeUnitDay:
					runEndAt = runStartAt + (*sub.t.GetRule().GetSampler().CycleInterval)*24*time.Hour.Milliseconds()
				case task.TimeUnitWeek:
					runEndAt = runStartAt + (*sub.t.GetRule().GetSampler().CycleInterval)*7*24*time.Hour.Milliseconds()
				default:
					runEndAt = runStartAt + (*sub.t.GetRule().GetSampler().CycleInterval)*10*time.Minute.Milliseconds()
				}
			}
			if err := sub.Creative(ctx, runStartAt, runEndAt); err != nil {
				merr = multierror.Append(merr, errors.WithMessagef(err, "task is unstarted, need sub.Creative,creative processor, task_id=%d", sub.taskID))
				continue
			}
			if err := sub.processor.OnUpdateTaskChange(ctx, tconv.TaskDTO2DO(sub.t, "", nil), task.TaskStatusRunning); err != nil {
				logs.CtxWarn(ctx, "OnUpdateTaskChange, task_id=%d, err=%v", sub.taskID, err)
				continue
			}
		}
		// Fetch the corresponding task config
		taskRunConfig, err := h.taskRepo.GetLatestNewDataTaskRun(ctx, sub.t.WorkspaceID, sub.taskID)
		if err != nil {
			logs.CtxWarn(ctx, "GetLatestNewDataTaskRun, task_id=%d, err=%v", sub.taskID, err)
			continue
		}
		if taskRunConfig == nil {
			logs.CtxWarn(ctx, "task run config not found, task_id=%d", sub.taskID)
			runStartAt = sub.t.GetRule().GetEffectiveTime().GetStartAt()
			if !sub.t.GetRule().GetSampler().GetIsCycle() {
				runEndAt = sub.t.GetRule().GetEffectiveTime().GetEndAt()
			} else {
				switch *sub.t.GetRule().GetSampler().CycleTimeUnit {
				case task.TimeUnitDay:
					runEndAt = runStartAt + (*sub.t.GetRule().GetSampler().CycleInterval)*24*time.Hour.Milliseconds()
				case task.TimeUnitWeek:
					runEndAt = runStartAt + (*sub.t.GetRule().GetSampler().CycleInterval)*7*24*time.Hour.Milliseconds()
				default:
					runEndAt = runStartAt + (*sub.t.GetRule().GetSampler().CycleInterval)*10*time.Minute.Milliseconds()
				}
			}
			if err = sub.Creative(ctx, runStartAt, runEndAt); err != nil {
				merr = multierror.Append(merr, errors.WithMessagef(err, "task run config not found,creative processor, task_id=%d", sub.taskID))
			}
			continue
		}
		sampler := sub.t.GetRule().GetSampler()
		// Fetch the corresponding task count and subtask count
		taskCount, _ := h.taskRepo.GetTaskCount(ctx, sub.taskID)
		taskRunCount, _ := h.taskRepo.GetTaskRunCount(ctx, sub.taskID, taskRunConfig.ID)
		logs.CtxInfo(ctx, "preDispatch, task_id=%d, taskCount=%d, taskRunCount=%d", sub.taskID, taskCount, taskRunCount)
		endTime := time.UnixMilli(sub.t.GetRule().GetEffectiveTime().GetEndAt())
		// Reached task time limit
		if time.Now().After(endTime) {
			logs.CtxWarn(ctx, "[OnFinishTaskChange]time.Now().After(endTime) Finish processor, task_id=%d, endTime=%v, now=%v", sub.taskID, endTime, time.Now())
			if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
				Task:     tconv.TaskDTO2DO(sub.t, "", nil),
				TaskRun:  taskRunConfig,
				IsFinish: true,
			}); err != nil {
				logs.CtxWarn(ctx, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID)
				merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
				continue
			}
		}
		// Reached task limit
		if taskCount+1 > sampler.GetSampleSize() {
			logs.CtxWarn(ctx, "[OnFinishTaskChange]taskCount+1 > sampler.GetSampleSize() Finish processor, task_id=%d", sub.taskID)
			if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
				Task:     tconv.TaskDTO2DO(sub.t, "", nil),
				TaskRun:  taskRunConfig,
				IsFinish: true,
			}); err != nil {
				merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
				continue
			}
		}
		if sampler.GetIsCycle() {
			cycleEndTime := time.Unix(0, taskRunConfig.RunEndAt.UnixMilli()*1e6)
			// Reached single cycle task time limit
			if time.Now().After(cycleEndTime) {
				logs.CtxInfo(ctx, "[OnFinishTaskChange]time.Now().After(cycleEndTime) Finish processor, task_id=%d", sub.taskID)
				if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
					Task:     tconv.TaskDTO2DO(sub.t, "", nil),
					TaskRun:  taskRunConfig,
					IsFinish: false,
				}); err != nil {
					merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
					continue
				}
				runStartAt = taskRunConfig.RunEndAt.UnixMilli()
				runEndAt = taskRunConfig.RunEndAt.UnixMilli() + (taskRunConfig.RunEndAt.UnixMilli() - taskRunConfig.RunStartAt.UnixMilli())
				if err := sub.Creative(ctx, runStartAt, runEndAt); err != nil {
					merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(cycleEndTime) creative processor, task_id=%d", sub.taskID))
					continue
				}
			}
			// Reached single cycle task limit
			if taskRunCount+1 > sampler.GetCycleCount() {
				logs.CtxWarn(ctx, "[OnFinishTaskChange]taskRunCount+1 > sampler.GetCycleCount(), task_id=%d", sub.taskID)
				if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
					Task:     tconv.TaskDTO2DO(sub.t, "", nil),
					TaskRun:  taskRunConfig,
					IsFinish: false,
				}); err != nil {
					merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
					continue
				}
			}
		}
	}
	return merr.ErrorOrNil()
}

func (h *TraceHubServiceImpl) dispatch(ctx context.Context, span *loop_span.Span, subs []*spanSubscriber) error {
	merr := &multierror.Error{}
	for _, sub := range subs {
		if sub.t.GetTaskStatus() != task.TaskStatusRunning {
			continue
		}
		logs.CtxInfo(ctx, " sub.AddSpan: %v", sub)
		if err := sub.AddSpan(ctx, span); err != nil {
			merr = multierror.Append(merr, errors.WithMessagef(err, "add span to subscriber, task_id=%d", sub.taskID))
			continue
		}
		logs.CtxInfo(ctx, "add span to subscriber, task_id=%d, log_id=%s, trace_id=%s, span_id=%s", sub.taskID,
			span.LogID, span.TraceID, span.SpanID)
	}
	return merr.ErrorOrNil()
}

// getObjListWithTaskFromCache retrieves the task list from cache, falling back to the database if cache is empty
func (h *TraceHubServiceImpl) getObjListWithTaskFromCache(ctx context.Context) ([]string, []string, []*entity.ObservabilityTask) {
	// First, try to retrieve tasks from cache
	objListWithTask, ok := h.taskCache.Load("ObjListWithTask")
	if !ok {
		// Cache is empty, fallback to the database
		logs.CtxInfo(ctx, "Cache is empty, retrieving task list from database")
		return h.taskRepo.GetObjListWithTask(ctx)
	}

	cacheInfo, ok := objListWithTask.(*TaskCacheInfo)
	if !ok {
		logs.CtxError(ctx, "Cache data type mismatch")
		return h.taskRepo.GetObjListWithTask(ctx)
	}

	logs.CtxInfo(ctx, "Retrieve task list from cache, taskCount=%d, spaceCount=%d, botCount=%d", len(cacheInfo.Tasks), len(cacheInfo.WorkspaceIDs), len(cacheInfo.BotIDs))
	return cacheInfo.WorkspaceIDs, cacheInfo.BotIDs, cacheInfo.Tasks
}
