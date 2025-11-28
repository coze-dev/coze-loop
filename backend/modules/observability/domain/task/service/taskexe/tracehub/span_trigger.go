// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/gg/gslice"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

func (h *TraceHubServiceImpl) SpanTrigger(ctx context.Context, span *loop_span.Span) error {
	logSuffix := fmt.Sprintf("log_id=%s, trace_id=%s, span_id=%s", span.LogID, span.TraceID, span.SpanID)
	logs.CtxInfo(ctx, "auto_task start, %s", logSuffix)

	// 1. perform initial filtering based on space_id
	// 1.1 Filter out spans that do not belong to any space or bot
	cacheInfo := h.localCache.LoadTaskCache(ctx)
	spaceIDs, botIDs := cacheInfo.WorkspaceIDs, cacheInfo.BotIDs
	if !gslice.Contains(spaceIDs, span.WorkspaceID) && !gslice.Contains(botIDs, span.TagsString["bot_id"]) {
		logs.CtxInfo(ctx, "no space or bot found for span, space_id=%s, bot_id=%s, %s", span.WorkspaceID, span.TagsString["bot_id"], logSuffix)
		return nil
	}
	// 1.2 Filter out spans of type Evaluator
	if gslice.Contains([]string{loop_span.CallTypeEvaluator}, span.CallType) {
		return nil
	}

	// 2、Match spans against task rules
	subs, err := h.buildSubscriberOfSpan(ctx, span)
	if err != nil {
		logs.CtxWarn(ctx, "get subscriber of flow span failed, %s, err: %v", logSuffix, err)
		return err
	}

	logs.CtxInfo(ctx, "%d subscriber of flow span found, %s", len(subs), logSuffix)
	if len(subs) == 0 {
		return nil
	}

	// 3. PreDispatch
	if err = h.preDispatch(ctx, subs); err != nil {
		logs.CtxWarn(ctx, "preDispatch flow span failed, %s, err: %v", logSuffix, err)
		return err
	}
	logs.CtxInfo(ctx, "%d preDispatch success, %v", len(subs), subs)

	// 4、Dispatch
	if err = h.dispatch(ctx, span, subs); err != nil {
		logs.CtxError(ctx, "dispatch flow span failed, %s, err: %v", logSuffix, err)
		return err
	}
	return nil
}

func (h *TraceHubServiceImpl) buildSubscriberOfSpan(ctx context.Context, span *loop_span.Span) ([]*spanSubscriber, error) {
	cfg, err := h.config.GetConsumerListening(ctx)
	if err != nil {
		logs.CtxError(ctx, "Failed to get consumer listening config, err: %v", err)
		return nil, err
	}

	var subscribers []*spanSubscriber
	taskDOs, err := h.listNonFinalTaskByRedis(ctx, span.WorkspaceID)
	if err != nil {
		logs.CtxError(ctx, "Failed to get non-final task list, err: %v", err)
		return nil, err
	}
	for _, taskDO := range taskDOs {
		if !cfg.IsAllSpace && !gslice.Contains(cfg.SpaceList, taskDO.WorkspaceID) {
			continue
		}
		if taskDO.EffectiveTime == nil || taskDO.EffectiveTime.StartAt == 0 {
			continue
		}
		if span.StartTime < taskDO.EffectiveTime.StartAt {
			logs.CtxInfo(ctx, "span start time is before task cycle start time, trace_id=%s, span_id=%s", span.TraceID, span.SpanID)
			continue
		}

		proc := h.taskProcessor.GetTaskProcessor(taskDO.TaskType)
		subscribers = append(subscribers, &spanSubscriber{
			taskID:      taskDO.ID,
			t:           taskDO,
			processor:   proc,
			taskRepo:    h.taskRepo,
			runType:     entity.TaskRunTypeNewData,
			buildHelper: h.buildHelper,
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
			if s.Sampled() {
				subscribers[keep] = s
				keep++
			} else {
				logs.CtxInfo(ctx, "span not sampled, task_id=%d, trace_id=%s, span_id=%s", s.taskID, span.TraceID, span.SpanID)
			}
		}
	}
	return subscribers[:keep], merr.ErrorOrNil()
}

func (h *TraceHubServiceImpl) preDispatch(ctx context.Context, subs []*spanSubscriber) error {
	merr := &multierror.Error{}
	for _, sub := range subs {
		// First step: lock for task status change
		// Task run status
		var runStartAt, runEndAt int64
		if sub.t.TaskStatus == entity.TaskStatusUnstarted {
			logs.CtxWarn(ctx, "task is unstarted, need sub.Creative")
			runStartAt = sub.t.EffectiveTime.StartAt
			if !sub.t.Sampler.IsCycle {
				runEndAt = sub.t.EffectiveTime.EndAt
			} else {
				switch sub.t.Sampler.CycleTimeUnit {
				case entity.TimeUnitDay:
					runEndAt = runStartAt + (sub.t.Sampler.CycleInterval)*24*time.Hour.Milliseconds()
				case entity.TimeUnitWeek:
					runEndAt = runStartAt + (sub.t.Sampler.CycleInterval)*7*24*time.Hour.Milliseconds()
				default:
					runEndAt = runStartAt + (sub.t.Sampler.CycleInterval)*10*time.Minute.Milliseconds()
				}
			}
			if err := sub.Creative(ctx, runStartAt, runEndAt); err != nil {
				merr = multierror.Append(merr, errors.WithMessagef(err, "task is unstarted, need sub.Creative,creative processor, task_id=%d", sub.taskID))
				continue
			}
			if err := sub.processor.OnTaskUpdated(ctx, sub.t, entity.TaskStatusRunning); err != nil {
				logs.CtxWarn(ctx, "OnTaskUpdated, task_id=%d, err=%v", sub.taskID, err)
				continue
			}
		}
		// Fetch the corresponding task config
		taskRunConfig, err := h.taskRepo.GetLatestNewDataTaskRun(ctx, &sub.t.WorkspaceID, sub.taskID)
		if err != nil {
			logs.CtxWarn(ctx, "GetLatestNewDataTaskRun, task_id=%d, err=%v", sub.taskID, err)
			continue
		}
		if taskRunConfig == nil {
			logs.CtxWarn(ctx, "task run config not found, task_id=%d", sub.taskID)
			runStartAt = sub.t.EffectiveTime.StartAt
			if !sub.t.Sampler.IsCycle {
				runEndAt = sub.t.EffectiveTime.EndAt
			} else {
				switch sub.t.Sampler.CycleTimeUnit {
				case entity.TimeUnitDay:
					runEndAt = runStartAt + sub.t.Sampler.CycleInterval*24*time.Hour.Milliseconds()
				case entity.TimeUnitWeek:
					runEndAt = runStartAt + sub.t.Sampler.CycleInterval*7*24*time.Hour.Milliseconds()
				default:
					runEndAt = runStartAt + sub.t.Sampler.CycleInterval*10*time.Minute.Milliseconds()
				}
			}
			if err = sub.Creative(ctx, runStartAt, runEndAt); err != nil {
				merr = multierror.Append(merr, errors.WithMessagef(err, "task run config not found,creative processor, task_id=%d", sub.taskID))
			}
			continue
		}

		endTime := time.UnixMilli(sub.t.EffectiveTime.EndAt)
		// Reached task time limit
		if time.Now().After(endTime) {
			logs.CtxWarn(ctx, "[OnTaskFinished]time.Now().After(endTime) Finish processor, task_id=%d, endTime=%v, now=%v", sub.taskID, endTime, time.Now())
			if err := sub.processor.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{
				Task:     sub.t,
				TaskRun:  taskRunConfig,
				IsFinish: true,
			}); err != nil {
				logs.CtxWarn(ctx, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID)
				merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
				continue
			}
		}

		sampler := sub.t.Sampler
		// Fetch the corresponding task count and subtask count
		taskCount, _ := h.taskRepo.GetTaskCount(ctx, sub.taskID)
		taskRunCount, _ := h.taskRepo.GetTaskRunCount(ctx, sub.taskID, taskRunConfig.ID)
		logs.CtxInfo(ctx, "preDispatch, task_id=%d, taskCount=%d, taskRunCount=%d", sub.taskID, taskCount, taskRunCount)
		// Reached task limit
		if taskCount+1 > sampler.SampleSize {
			logs.CtxWarn(ctx, "[OnTaskFinished]taskCount+1 > sampler.GetSampleSize() Finish processor, task_id=%d", sub.taskID)
			if err := sub.processor.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{
				Task:     sub.t,
				TaskRun:  taskRunConfig,
				IsFinish: true,
			}); err != nil {
				merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
				continue
			}
		}
		if sampler.IsCycle {
			cycleEndTime := time.Unix(0, taskRunConfig.RunEndAt.UnixMilli()*1e6)
			// Reached single cycle task time limit
			if time.Now().After(cycleEndTime) {
				logs.CtxInfo(ctx, "[OnTaskFinished]time.Now().After(cycleEndTime) Finish processor, task_id=%d", sub.taskID)
				if err := sub.processor.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{
					Task:     sub.t,
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
			if taskRunCount+1 > sampler.CycleCount {
				logs.CtxWarn(ctx, "[OnTaskFinished]taskRunCount+1 > sampler.GetCycleCount(), task_id=%d", sub.taskID)
				if err := sub.processor.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{
					Task:     sub.t,
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
		if sub.t.TaskStatus != entity.TaskStatusRunning {
			continue
		}
		if err := sub.AddSpan(ctx, span); err != nil {
			merr = multierror.Append(merr, errors.WithMessagef(err, "add span to subscriber, log_id=%s, trace_id=%s, span_id=%s, task_id=%d",
				span.LogID, span.TraceID, span.SpanID, sub.taskID))
		} else {
			logs.CtxInfo(ctx, "add span to subscriber, task_id=%d, log_id=%s, trace_id=%s, span_id=%s", sub.taskID,
				span.LogID, span.TraceID, span.SpanID)
		}
	}
	return merr.ErrorOrNil()
}

func (h *TraceHubServiceImpl) listNonFinalTaskByRedis(ctx context.Context, spaceID string) ([]*entity.ObservabilityTask, error) {
	var taskPOs []*entity.ObservabilityTask
	nonFinalTaskIDs, err := h.taskRepo.ListNonFinalTaskBySpaceID(ctx, spaceID)
	if err != nil {
		logs.CtxError(ctx, "Failed to get non-final task list", "err", err)
		return nil, err
	}
	logs.CtxInfo(ctx, "Start listing non-final tasks, taskCount:%d, nonFinalTaskIDs:%v", len(nonFinalTaskIDs), nonFinalTaskIDs)
	if len(nonFinalTaskIDs) == 0 {
		return taskPOs, nil
	}
	for _, taskID := range nonFinalTaskIDs {
		taskPO, err := h.taskRepo.GetTaskByCache(ctx, taskID)
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
