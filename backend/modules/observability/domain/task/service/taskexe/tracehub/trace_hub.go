// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/bytedance/gg/gslice"
	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe/processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	trace_repo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

type ITraceHubService interface {
	TraceHub(ctx context.Context, event *entity.RawSpan) error
	CallBack(ctx context.Context, event *entity.AutoEvalEvent) error
	Correction(ctx context.Context, event *entity.CorrectionEvent) error
	BackFill(ctx context.Context, event *entity.BackFillEvent) error
}

func NewTraceHubImpl(
	tRepo repo.ITaskRepo,
	tRunRepo repo.ITaskRunRepo,
	datasetServiceProvider *service.DatasetServiceAdaptor,
	evalService rpc.IEvaluatorRPCAdapter,
	evaluationService rpc.IEvaluationRPCAdapter,
	traceRepo trace_repo.ITraceRepo,
	tenantProvider tenant.ITenantProvider,
	buildHelper service.TraceFilterProcessorBuilder,
) (ITraceHubService, error) {
	processor.InitProcessor(datasetServiceProvider, evalService, evaluationService, tRepo, tRunRepo)
	// 创建两个不同间隔的独立定时器
	scheduledTaskTicker := time.NewTicker(5 * time.Minute) // 任务状态生命周期管理 - 5分钟间隔
	syncTaskTicker := time.NewTicker(2 * time.Minute)      // 数据同步 - 1分钟间隔
	impl := &TraceHubServiceImpl{
		taskRepo:            tRepo,
		taskRunRepo:         tRunRepo,
		scheduledTaskTicker: scheduledTaskTicker,
		syncTaskTicker:      syncTaskTicker,
		stopChan:            make(chan struct{}),
		traceRepo:           traceRepo,
		tenantProvider:      tenantProvider,
		buildHelper:         buildHelper,
	}

	// 立即启动定时任务
	impl.startScheduledTask()
	impl.startSyncTaskRunCounts()

	return impl, nil
}

type TraceHubServiceImpl struct {
	scheduledTaskTicker *time.Ticker // 任务状态生命周期管理定时器 - 5分钟间隔
	syncTaskTicker      *time.Ticker // 数据同步定时器 - 1分钟间隔
	stopChan            chan struct{}
	taskRepo            repo.ITaskRepo
	taskRunRepo         repo.ITaskRunRepo
	traceRepo           trace_repo.ITraceRepo
	tenantProvider      tenant.ITenantProvider

	buildHelper  service.TraceFilterProcessorBuilder
	task         *task.Task
	flushCh      chan *flushReq
	flushErrLock sync.Mutex
	flushErr     []error
}

type flushReq struct {
	retrievedSpanCount int64
	pageToken          string
	spans              []*loop_span.Span
	noMore             bool
}

const TagKeyResult = "tag_key"

func (h *TraceHubServiceImpl) TraceHub(ctx context.Context, rawSpan *entity.RawSpan) error {
	ctx = context.WithValue(ctx, "K_ENV", "boe_auto_task")
	logs.CtxInfo(ctx, "TraceHub start")
	var tags []metrics.T
	// 1、转换成标准span，并根据space_id初步过滤
	span := rawSpan.RawSpanConvertToLoopSpan()
	// 1.1 过滤掉 Evaluator 类型的 span
	if slices.Contains([]string{"Evaluator"}, span.CallType) {
		return nil
	}
	logSuffix := fmt.Sprintf("log_id=%s, trace_id=%s, span_id=%s", span.LogID, span.TraceID, span.SpanID)
	spaceList, _ := h.taskRepo.GetObjListWithTask(ctx)
	logs.CtxInfo(ctx, "space list: %v", spaceList)
	// 1.2 过滤掉不在 spaceList 中的 span
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
			runType:          task.TaskRunTypeNewData,
			taskRunRepo:      h.taskRunRepo,
			buildHelper:      h.buildHelper,
		})
	}

	var (
		merr = &multierror.Error{}
		keep int
	)
	// 按照详细的filter规则匹配数据
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
func (h *TraceHubServiceImpl) preDispatch(ctx context.Context, span *loop_span.Span, subs []*spanSubscriber) error {
	merr := &multierror.Error{}
	var needDispatchSubs []*spanSubscriber
	for _, sub := range subs {
		if span.StartTime < sub.t.GetRule().GetEffectiveTime().GetStartAt() {
			logs.CtxWarn(ctx, "span start time is before task cycle start time, trace_id=%s, span_id=%s", span.TraceID, span.SpanID)
			continue
		}
		// 第一步task状态变更的锁
		// taskrun的状态
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
				needDispatchSubs = append(needDispatchSubs, sub)
				continue
			}
			if err := sub.processor.OnUpdateTaskChange(ctx, sub.t, task.TaskStatusRunning); err != nil {
				logs.CtxWarn(ctx, "OnUpdateTaskChange, task_id=%d, err=%v", sub.taskID, err)
				continue
			}
		}
		//获取对应的taskconfig
		taskRunConfig, err := h.taskRunRepo.GetLatestNewDataTaskRun(ctx, sub.t.WorkspaceID, sub.taskID)
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
				needDispatchSubs = append(needDispatchSubs, sub)
				continue
			}
		}
		sampler := sub.t.GetRule().GetSampler()
		//获取对应的taskcount和subtaskcount
		taskCount, _ := h.taskRepo.GetTaskCount(ctx, sub.taskID)
		taskRunCount, _ := h.taskRepo.GetTaskRunCount(ctx, sub.taskID, taskRunConfig.ID)
		logs.CtxInfo(ctx, "preDispatch, task_id=%d, taskCount=%d, taskRunCount=%d", sub.taskID, taskCount, taskRunCount)
		endTime := time.Unix(0, taskRunConfig.RunEndAt.UnixMilli()*1e6)
		// 达到任务时间期限
		if time.Now().After(endTime) {
			if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
				Task:     sub.t,
				TaskRun:  taskRunConfig,
				IsFinish: true,
			}); err != nil {
				logs.CtxWarn(ctx, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID)
				merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
				continue
			}
			//if err := sub.processor.Finish(ctx, taskRunConfig, &taskexe.Trigger{Task: sub.t, Span: span, IsFinish: true}); err != nil {
			//	logs.CtxWarn(ctx, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID)
			//	merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
			//	continue
			//}
		}
		// 达到任务上限
		if taskCount+1 > sampler.GetSampleSize() {
			if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
				Task:     sub.t,
				TaskRun:  taskRunConfig,
				IsFinish: true,
			}); err != nil {
				logs.CtxWarn(ctx, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID)
				merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
				continue
			}
			//if err := sub.processor.Finish(ctx, taskRunConfig, &taskexe.Trigger{Task: sub.t, Span: span, IsFinish: true}); err != nil {
			//	logs.CtxWarn(ctx, "taskCount > sampler.GetSampleSize()+1 Finish processor, task_id=%d", sub.taskID)
			//	merr = multierror.Append(merr, errors.WithMessagef(err, "taskCount > sampler.GetSampleSize()+1 Finish processor, task_id=%d", sub.taskID))
			//	continue
			//}
		}
		if sampler.GetIsCycle() {
			cycleEndTime := time.Unix(0, taskRunConfig.RunEndAt.UnixMilli()*1e6)
			// 达到单次任务时间期限
			if time.Now().After(cycleEndTime) {
				logs.CtxInfo(ctx, "time.Now().After(cycleEndTime)")
				if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
					Task:     sub.t,
					TaskRun:  taskRunConfig,
					IsFinish: false,
				}); err != nil {
					logs.CtxWarn(ctx, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID)
					merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
					continue
				}
				//if err := sub.processor.Finish(ctx, taskRunConfig, &taskexe.Trigger{Task: sub.t, Span: span, IsFinish: false}); err != nil {
				//	merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(cycleEndTime) Finish processor, task_id=%d", sub.taskID))
				//}
				runStartAt = taskRunConfig.RunEndAt.UnixMilli()
				runEndAt = taskRunConfig.RunEndAt.UnixMilli() + (taskRunConfig.RunEndAt.UnixMilli() - taskRunConfig.RunStartAt.UnixMilli())
				if err := sub.Creative(ctx, runStartAt, runEndAt); err != nil {
					merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(cycleEndTime) creative processor, task_id=%d", sub.taskID))
					needDispatchSubs = append(needDispatchSubs, sub)
					continue
				}
			}
			// 达到单次任务上限
			if taskRunCount+1 > sampler.GetCycleCount() {
				if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
					Task:     sub.t,
					TaskRun:  taskRunConfig,
					IsFinish: false,
				}); err != nil {
					logs.CtxWarn(ctx, "taskRunCount+1 > sampler.GetCycleCount(), task_id=%d", sub.taskID)
					merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
					continue
				}
				//if err := sub.processor.Finish(ctx, taskRunConfig, &taskexe.Trigger{Task: sub.t, Span: span, IsFinish: false}); err != nil {
				//	merr = multierror.Append(merr, errors.WithMessagef(err, "subTaskCount > sampler.GetCycleCount()+1 Finish processor, task_id=%d", sub.taskID))
				//	continue
				//}
			}
		}
	}
	subs = needDispatchSubs
	return merr.ErrorOrNil()
}

func (h *TraceHubServiceImpl) Close() {
	close(h.stopChan)
}
