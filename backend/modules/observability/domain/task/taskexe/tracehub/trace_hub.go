// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bytedance/gg/gslice"
	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

type ITraceHubService interface {
	TraceHub(ctx context.Context, event *entity.RawSpan) error
}

func NewTraceHubImpl(
	tRepo repo.ITaskRepo,
	datasetServiceProvider *service.DatasetServiceAdaptor,
	evalService rpc.IEvaluatorRPCAdapter,
	evaluationService rpc.IEvaluationRPCAdapter,
) (ITraceHubService, error) {
	processor.InitProcessor(datasetServiceProvider, evalService, evaluationService, tRepo)
	ticker := time.NewTicker(5 * time.Minute) // 每x分钟执行一次定时任务
	impl := &TraceHubServiceImpl{
		taskRepo: tRepo,
		ticker:   ticker,
		stopChan: make(chan struct{}),
	}

	// 立即启动定时任务
	impl.startScheduledTask()

	return impl, nil
}

type TraceHubServiceImpl struct {
	ticker   *time.Ticker
	stopChan chan struct{}
	taskRepo repo.ITaskRepo
}

const TagKeyResult = "tag_key"

func (h *TraceHubServiceImpl) TraceHub(ctx context.Context, rawSpan *entity.RawSpan) error {
	ctx = context.WithValue(ctx, "K_ENV", "boe_auto_task")
	logs.CtxInfo(ctx, "TraceHub start")
	var tags []metrics.T
	// 1、转换成标准span，并根据space_id初步过滤
	span := rawSpan.RawSpanConvertToLoopSpan()
	logSuffix := fmt.Sprintf("log_id=%s, trace_id=%s, span_id=%s", span.LogID, span.TraceID, span.SpanID)
	spaceList, _ := h.taskRepo.GetObjListWithTask(ctx)
	logs.CtxInfo(ctx, "space list: %v", spaceList)
	if !gslice.Contains(spaceList, span.WorkspaceID) {
		tags = append(tags, metrics.T{Name: TagKeyResult, Value: "no_space"})
		logs.CtxInfo(ctx, "no space found for span, %s", logSuffix)
		return nil
	}
	// 2、读redis，获取rule信息，进行匹配，查询订阅者
	subs, err := h.getSubscriberOfSpan(ctx, span)
	if err != nil { // 继续执行，不阻塞。
		logs.CtxWarn(ctx, "get subscriber of flow span failed, %s, err: %v", logSuffix, err)
	}

	logs.CtxInfo(ctx, "%d subscriber of flow span found, %s", len(subs), logSuffix)
	if len(subs) == 0 {
		tags = append(tags, metrics.T{Name: TagKeyResult, Value: "no_subscriber"})
		return nil
	}
	// 3、采样
	subs = gslice.Filter(subs, func(sub *spanSubscriber) bool { return sub.Sampled() })
	logs.CtxInfo(ctx, "%d subscriber of flow span sampled, %s", len(subs), logSuffix)
	if len(subs) == 0 {
		tags = append(tags, metrics.T{Name: TagKeyResult, Value: "sampler_not_hit"})
		return nil
	}
	// 3. 分发预处理
	err = h.preDispatch(ctx, span, subs)
	if err != nil {
		tags = append(tags, metrics.T{Name: TagKeyResult, Value: "preDispatch_failed"})
		logs.CtxWarn(ctx, "preDispatch flow span failed, %s, err: %v", logSuffix, err)
		//return err
	}
	logs.CtxInfo(ctx, "%d preDispatch success, %v", len(subs), subs)
	// 4、按条件分发
	if err = h.dispatch(ctx, span, subs); err != nil {
		tags = append(tags, metrics.T{Name: TagKeyResult, Value: "dispatch_failed"})
		logs.CtxWarn(ctx, "dispatch flow span failed, %s, err: %v", logSuffix, err)
		return err
	}
	tags = append(tags, metrics.T{Name: TagKeyResult, Value: "dispatched"})
	return nil
}

func (h *TraceHubServiceImpl) getSubscriberOfSpan(ctx context.Context, span *loop_span.Span) ([]*spanSubscriber, error) {
	logs.CtxInfo(ctx, "getSubscriberOfSpan start")
	var subscribers []*spanSubscriber
	// 获取该空间非终态任务列表
	tasksPOList := h.taskRepo.ListNonFinalTaskBySpaceID(ctx, span.WorkspaceID)
	if len(tasksPOList) == 0 {
		logs.CtxWarn(ctx, "no subscriber found for span, trace_id=%s, span_id=%s", span.TraceID, span.SpanID)
		return nil, nil
	}
	taskList := tconv.TaskPOs2DOs(ctx, tasksPOList, nil)
	for _, taskDO := range taskList {
		proc, err := processor.NewProcessor(ctx, taskDO.TaskType)
		if err != nil {
			return nil, err
		}
		subscribers = append(subscribers, &spanSubscriber{
			taskID:           taskDO.GetID(),
			RWMutex:          sync.RWMutex{},
			t:                taskDO,
			processor:        proc,
			bufCap:           0,
			flushWait:        sync.WaitGroup{},
			maxFlushInterval: time.Second * 5,
			taskRepo:         h.taskRepo,
		})
	}

	var (
		merr = &multierror.Error{}
		keep int
	)
	// 按照详细的filter规则匹配数据
	for _, s := range subscribers {
		ok, err := s.Match(ctx, span)
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

func (h *TraceHubServiceImpl) dispatch(ctx context.Context, span *loop_span.Span, subs []*spanSubscriber) error {
	merr := &multierror.Error{}
	for _, sub := range subs {
		if sub.t.GetTaskStatus() == task.TaskStatusSuccess {
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
	logs.CtxError(ctx, "dispatch error:%v", merr.ErrorOrNil())
	return merr.ErrorOrNil()
}
func (h *TraceHubServiceImpl) preDispatch(ctx context.Context, span *loop_span.Span, subs []*spanSubscriber) error {
	merr := &multierror.Error{}
	var needDispatchSubs []*spanSubscriber
	for _, sub := range subs {
		if Usec2Msec(span.StartTime) < sub.t.GetRule().GetEffectiveTime().GetStartAt() {
			logs.CtxWarn(ctx, "span start time is before task cycle start time, trace_id=%s, span_id=%s", span.TraceID, span.SpanID)
			continue
		}
		// 第一步task状态变更的锁
		// taskrun的状态
		if sub.t.GetTaskStatus() == task.TaskStatusUnstarted {
			logs.CtxWarn(ctx, "task is unstarted, need sub.Creative")
			if err := sub.Creative(ctx); err != nil {
				merr = multierror.Append(merr, errors.WithMessagef(err, "task is unstarted, need sub.Creative,creative processor, task_id=%d", sub.taskID))
				needDispatchSubs = append(needDispatchSubs, sub)
				continue
			}
		}
		//获取对应的taskconfig
		taskConfig, _ := h.taskRepo.GetTask(ctx, sub.taskID, nil, nil)
		if taskConfig == nil {
			logs.CtxWarn(ctx, "task config not found, task_id=%d", sub.taskID)
		}
		var taskRunConfig *entity.TaskRun
		if taskConfig != nil {
			var latestStartAt int64
			for _, run := range taskConfig.TaskRuns {
				if run.RunStartAt.UnixMilli() > latestStartAt {
					latestStartAt = run.RunStartAt.UnixMilli()
					taskRunConfig = run
				}
			}
		}
		if taskRunConfig == nil {
			continue
		}
		sampler := sub.t.GetRule().GetSampler()
		//获取对应的taskcount和subtaskcount
		taskCount, _ := h.taskRepo.GetTaskCount(ctx, sub.taskID)
		taskRunCount, _ := h.taskRepo.GetTaskRunCount(ctx, sub.taskID, taskRunConfig.ID)

		if taskRunConfig == nil {
			logs.CtxWarn(ctx, "task run config not found, task_id=%d", sub.taskID)
			if err := sub.Creative(ctx); err != nil {
				merr = multierror.Append(merr, errors.WithMessagef(err, "task run config not found,creative processor, task_id=%d", sub.taskID))
				needDispatchSubs = append(needDispatchSubs, sub)
				continue
			}
		}
		endTime := time.Unix(0, taskRunConfig.RunEndAt.UnixMilli())
		// 达到任务时间期限
		if time.Now().After(endTime) {
			if err := sub.processor.Finish(ctx, taskRunConfig, &taskexe.Trigger{Task: sub.t, Span: span, IsFinish: true}); err != nil {
				merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
				continue
			}
		}
		if sampler.GetIsCycle() {
			cycleEndTime := time.Unix(0, taskRunConfig.RunEndAt.UnixMilli())
			// 达到单次任务时间期限
			if time.Now().After(cycleEndTime) {
				logs.CtxInfo(ctx, "time.Now().After(cycleEndTime)")
				if err := sub.processor.Finish(ctx, taskRunConfig, &taskexe.Trigger{Task: sub.t, Span: span, IsFinish: false}); err != nil {
					merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(cycleEndTime) Finish processor, task_id=%d", sub.taskID))
				}
				if err := sub.Creative(ctx); err != nil {
					merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(cycleEndTime) creative processor, task_id=%d", sub.taskID))
					needDispatchSubs = append(needDispatchSubs, sub)
					continue
				}
			}
			// 达到单次任务上限
			if taskRunCount+1 > sampler.GetCycleCount() {
				if err := sub.processor.Finish(ctx, taskRunConfig, &taskexe.Trigger{Task: sub.t, Span: span, IsFinish: false}); err != nil {
					merr = multierror.Append(merr, errors.WithMessagef(err, "subTaskCount > sampler.GetCycleCount()+1 Finish processor, task_id=%d", sub.taskID))
					continue
				}
			}
		}

		// 达到任务上限
		if taskCount+1 > sampler.GetSampleSize() {
			if err := sub.processor.Finish(ctx, taskRunConfig, &taskexe.Trigger{Task: sub.t, Span: span, IsFinish: true}); err != nil {
				merr = multierror.Append(merr, errors.WithMessagef(err, "taskCount > sampler.GetSampleSize()+1 Finish processor, task_id=%d", sub.taskID))
				continue
			}
		}
	}
	subs = needDispatchSubs
	return merr.ErrorOrNil()
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
	tasks := tconv.TaskPOs2DOs(ctx, taskPOs, nil)
	logs.CtxInfo(ctx, "定时任务获取到任务数量:%d", len(tasks))

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
		if time.Now().After(endTime) {
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
func (h *TraceHubServiceImpl) Close() {
	close(h.stopChan)
}
