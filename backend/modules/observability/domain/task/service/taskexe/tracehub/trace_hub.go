// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/gg/gslice"
	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
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
	traceRepo trace_repo.ITraceRepo,
	tenantProvider tenant.ITenantProvider,
	buildHelper service.TraceFilterProcessorBuilder,
	taskProcessor *processor.TaskProcessor,
	benefitSvc benefit.IBenefitService,
	aid int32,
	backfillProducer mq.IBackfillProducer,
) (ITraceHubService, error) {
	// 创建两个不同间隔的独立定时器
	scheduledTaskTicker := time.NewTicker(5 * time.Minute) // 任务状态生命周期管理 - 5分钟间隔
	syncTaskTicker := time.NewTicker(2 * time.Minute)      // 数据同步 - 1分钟间隔
	impl := &TraceHubServiceImpl{
		taskRepo:            tRepo,
		scheduledTaskTicker: scheduledTaskTicker,
		syncTaskTicker:      syncTaskTicker,
		stopChan:            make(chan struct{}),
		traceRepo:           traceRepo,
		tenantProvider:      tenantProvider,
		buildHelper:         buildHelper,
		taskProcessor:       taskProcessor,
		benefitSvc:          benefitSvc,
		aid:                 aid,
		backfillProducer:    backfillProducer,
	}

	// 立即启动定时任务
	impl.startScheduledTask()
	impl.startSyncTaskRunCounts()
	impl.startSyncTaskCache()

	return impl, nil
}

type TraceHubServiceImpl struct {
	scheduledTaskTicker *time.Ticker // 任务状态生命周期管理定时器 - 5分钟间隔
	syncTaskTicker      *time.Ticker // 数据同步定时器 - 1分钟间隔
	stopChan            chan struct{}
	taskRepo            repo.ITaskRepo
	traceRepo           trace_repo.ITraceRepo
	tenantProvider      tenant.ITenantProvider
	taskProcessor       *processor.TaskProcessor
	buildHelper         service.TraceFilterProcessorBuilder
	benefitSvc          benefit.IBenefitService
	backfillProducer    mq.IBackfillProducer

	flushCh      chan *flushReq
	flushErrLock sync.Mutex
	flushErr     []error

	// 本地缓存 - 缓存非终态任务信息
	taskCache     sync.Map
	taskCacheLock sync.RWMutex

	aid int32
}

type flushReq struct {
	retrievedSpanCount int64
	pageToken          string
	spans              []*loop_span.Span
	noMore             bool
}

const TagKeyResult = "tag_key"

func (h *TraceHubServiceImpl) TraceHub(ctx context.Context, rawSpan *entity.RawSpan) error {
	logs.CtxInfo(ctx, "XttEnv: %s", os.Getenv(XttEnv))
	ctx = fillCtxWithEnv(ctx)
	ctx = metainfo.WithPersistentValue(ctx, "LANE_C_FORNAX_APPID", strconv.FormatInt(int64(h.aid), 10))
	logs.CtxInfo(ctx, "TraceHub start")
	var tags []metrics.T
	// 1、Convert to standard span and perform initial filtering based on space_id
	span := rawSpan.RawSpanConvertToLoopSpan()
	// 1.1 Filter out spans of type Evaluator
	if slices.Contains([]string{"Evaluator"}, span.CallType) {
		return nil
	}
	logSuffix := fmt.Sprintf("log_id=%s, trace_id=%s, span_id=%s", span.LogID, span.TraceID, span.SpanID)
	// 1.2 Filter out spans that do not belong to any space or bot
	spaceIDs, botIDs, tasks := h.getObjListWithTaskFromCache(ctx)

	logs.CtxInfo(ctx, "space list: %v, bot list: %v, task list: %v", spaceIDs, botIDs, tasks)
	if !gslice.Contains(spaceIDs, span.WorkspaceID) && !gslice.Contains(botIDs, span.TagsString["bot_id"]) {
		tags = append(tags, metrics.T{Name: TagKeyResult, Value: "no_space"})
		logs.CtxInfo(ctx, "no space found for span, %s", logSuffix)
		return nil
	}
	// 2、Match spans against task rules
	subs, err := h.getSubscriberOfSpan(ctx, span, tasks)
	if err != nil {
		logs.CtxWarn(ctx, "get subscriber of flow span failed, %s, err: %v", logSuffix, err)
	}

	logs.CtxInfo(ctx, "%d subscriber of flow span found, %s", len(subs), logSuffix)
	if len(subs) == 0 {
		tags = append(tags, metrics.T{Name: TagKeyResult, Value: "no_subscriber"})
		return nil
	}
	// 3、Sample
	subs = gslice.Filter(subs, func(sub *spanSubscriber) bool { return sub.Sampled() })
	logs.CtxInfo(ctx, "%d subscriber of flow span sampled, %s", len(subs), logSuffix)
	if len(subs) == 0 {
		tags = append(tags, metrics.T{Name: TagKeyResult, Value: "sampler_not_hit"})
		return nil
	}
	// 3. PreDispatch
	err = h.preDispatch(ctx, span, subs)
	if err != nil {
		tags = append(tags, metrics.T{Name: TagKeyResult, Value: "preDispatch_failed"})
		logs.CtxWarn(ctx, "preDispatch flow span failed, %s, err: %v", logSuffix, err)
		//return err
	}
	logs.CtxInfo(ctx, "%d preDispatch success, %v", len(subs), subs)
	// 4、Dispatch
	if err = h.dispatch(ctx, span, subs); err != nil {
		tags = append(tags, metrics.T{Name: TagKeyResult, Value: "dispatch_failed"})
		logs.CtxWarn(ctx, "dispatch flow span failed, %s, err: %v", logSuffix, err)
		return err
	}
	tags = append(tags, metrics.T{Name: TagKeyResult, Value: "dispatched"})
	return nil
}

func (h *TraceHubServiceImpl) getSubscriberOfSpan(ctx context.Context, span *loop_span.Span, tasks []*entity.ObservabilityTask) ([]*spanSubscriber, error) {
	logs.CtxInfo(ctx, "getSubscriberOfSpan start")
	var subscribers []*spanSubscriber
	taskList := tconv.TaskPOs2DOs(ctx, tasks, nil)
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
				needDispatchSubs = append(needDispatchSubs, sub)
				continue
			}
		}
		sampler := sub.t.GetRule().GetSampler()
		//获取对应的taskcount和subtaskcount
		taskCount, _ := h.taskRepo.GetTaskCount(ctx, sub.taskID)
		taskRunCount, _ := h.taskRepo.GetTaskRunCount(ctx, sub.taskID, taskRunConfig.ID)
		logs.CtxInfo(ctx, "preDispatch, task_id=%d, taskCount=%d, taskRunCount=%d", sub.taskID, taskCount, taskRunCount)
		endTime := time.UnixMilli(sub.t.GetRule().GetEffectiveTime().GetEndAt())
		// 达到任务时间期限
		if time.Now().After(endTime) {
			logs.CtxWarn(ctx, "time.Now().After(endTime) Finish processor, task_id=%d, endTime=%v, now=%v", sub.taskID, endTime, time.Now())
			if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
				Task:     sub.t,
				TaskRun:  taskRunConfig,
				IsFinish: true,
			}); err != nil {
				logs.CtxWarn(ctx, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID)
				merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
				continue
			}
		}
		// 达到任务上限
		if taskCount+1 > sampler.GetSampleSize() {
			logs.CtxWarn(ctx, "taskCount+1 > sampler.GetSampleSize() Finish processor, task_id=%d", sub.taskID)
			if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
				Task:     sub.t,
				TaskRun:  taskRunConfig,
				IsFinish: true,
			}); err != nil {
				merr = multierror.Append(merr, errors.WithMessagef(err, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID))
				continue
			}
		}
		if sampler.GetIsCycle() {
			cycleEndTime := time.Unix(0, taskRunConfig.RunEndAt.UnixMilli()*1e6)
			// 达到单次任务时间期限
			if time.Now().After(cycleEndTime) {
				logs.CtxInfo(ctx, "time.Now().After(cycleEndTime) Finish processor, task_id=%d", sub.taskID)
				if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
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
					needDispatchSubs = append(needDispatchSubs, sub)
					continue
				}
			}
			// 达到单次任务上限
			if taskRunCount+1 > sampler.GetCycleCount() {
				logs.CtxWarn(ctx, "taskRunCount+1 > sampler.GetCycleCount(), task_id=%d", sub.taskID)
				if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
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
	subs = needDispatchSubs
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

func (h *TraceHubServiceImpl) Close() {
	close(h.stopChan)
}

// getObjListWithTaskFromCache 从缓存中获取任务列表，如果缓存为空则回退到数据库
func (h *TraceHubServiceImpl) getObjListWithTaskFromCache(ctx context.Context) ([]string, []string, []*entity.ObservabilityTask) {
	// 首先尝试从缓存中获取任务
	objListWithTask, ok := h.taskCache.Load("ObjListWithTask")
	if !ok {
		// 缓存为空，回退到数据库
		logs.CtxInfo(ctx, "缓存为空，从数据库获取任务列表")
		return h.taskRepo.GetObjListWithTask(ctx)
	}

	cacheInfo, ok := objListWithTask.(*TaskCacheInfo)
	if !ok {
		logs.CtxError(ctx, "缓存数据类型错误")
		return h.taskRepo.GetObjListWithTask(ctx)
	}

	logs.CtxInfo(ctx, "从缓存获取任务列表", "taskCount", len(cacheInfo.Tasks), "spaceCount", len(cacheInfo.WorkspaceIDs), "botCount", len(cacheInfo.BotIDs))
	return cacheInfo.WorkspaceIDs, cacheInfo.BotIDs, cacheInfo.Tasks
}
