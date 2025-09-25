// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const pageSize = 100

func (h *TraceHubServiceImpl) BackFill(ctx context.Context, event *entity.BackFillEvent) error {
	// 1. 设置当前任务上下文
	ctx = context.WithValue(ctx, "K_ENV", "boe_auto_task")
	logs.CtxInfo(ctx, "starting backfill task, event_task_id=%d", event.TaskID)
	
	sub, err := h.setBackfillTask(ctx, event)
	if err != nil {
		logs.CtxError(ctx, "failed to set backfill task, event_task_id=%d, err=%v", event.TaskID, err)
		return err
	}
	logs.CtxInfo(ctx, "backfill task setup completed, task_id=%d", sub.t.GetID())

	// 2. 判断回溯任务是否已完成 - 避免重复执行
	isDone, err := h.isBackfillDone(ctx, sub)
	if err != nil {
		logs.CtxError(ctx, "check backfill task done failed, task_id=%d, err=%v", sub.t.GetID(), err)
		return err
	}
	if isDone {
		logs.CtxInfo(ctx, "backfill already completed, task_id=%d", sub.t.GetID())
		return nil
	}

	// 3. 创建并发控制机制 - 设置上下文和等待组
	subCtx, cancel := context.WithCancel(ctx)
	defer func() {
		logs.CtxInfo(ctx, "cancelling backfill context, task_id=%d", sub.t.GetID())
		cancel()
	}()
	wg := sync.WaitGroup{}
	logs.CtxInfo(ctx, "created backfill context and wait group, task_id=%d", sub.t.GetID())

	// 初始化 flushCh 通道和错误收集器
	h.flushCh = make(chan *flushReq, 100) // 缓冲通道，避免阻塞
	h.flushErrLock.Lock()
	h.flushErr = nil // 重置错误收集器
	h.flushErrLock.Unlock()
	logs.CtxInfo(ctx, "initialized flush channel and error collector, task_id=%d", sub.t.GetID())

	// 4. 启动异步刷新处理 - 通过 goroutine 实现并发处理
	wg.Add(1)
	logs.CtxInfo(ctx, "starting flush spans goroutine, task_id=%d", sub.t.GetID())
	goroutine.Go(ctx, func() {
		defer wg.Done()
		h.flushSpans(subCtx, sub)
	})

	// 5. 获取 span 数据 - 从观测服务获取需要处理的数据
	logs.CtxInfo(ctx, "starting to list spans, task_id=%d", sub.t.GetID())
	listErr := h.listSpans(subCtx, sub)
	if listErr != nil {
		logs.CtxError(ctx, "list spans failed, task_id=%d, err=%v", sub.t.GetID(), listErr)
		// continue on error，不中断处理流程
	} else {
		logs.CtxInfo(ctx, "completed listing spans, task_id=%d", sub.t.GetID())
	}

	// 关闭通道并等待处理完成
	logs.CtxInfo(ctx, "closing flush channel and waiting for completion, task_id=%d", sub.t.GetID())
	close(h.flushCh)
	wg.Wait()
	logs.CtxInfo(ctx, "all goroutines completed, task_id=%d", sub.t.GetID())

	// 6. 同步等待完成 - 确保所有数据处理完毕
	logs.CtxInfo(ctx, "handling backfill completion, task_id=%d", sub.t.GetID())
	return h.onHandleDone(ctx, listErr, sub)
}

// setBackfillTask 设置当前回填任务的上下文
func (h *TraceHubServiceImpl) setBackfillTask(ctx context.Context, event *entity.BackFillEvent) (*spanSubscriber, error) {
	taskConfig, err := h.taskRepo.GetTask(ctx, event.TaskID, nil, nil)
	if err != nil {
		logs.CtxError(ctx, "get task config failed, task_id=%d, err=%v", event.TaskID, err)
		return nil, err
	}
	if taskConfig == nil {
		return nil, errors.New("task config not found")
	}
	taskConfigDO := tconv.TaskPO2DTO(ctx, taskConfig, nil)
	proc := h.taskProcessor.GetTaskProcessor(taskConfig.TaskType)
	sub := &spanSubscriber{
		taskID:           taskConfigDO.GetID(),
		RWMutex:          sync.RWMutex{},
		t:                taskConfigDO,
		processor:        proc,
		bufCap:           0,
		flushWait:        sync.WaitGroup{},
		maxFlushInterval: time.Second * 5,
		taskRepo:         h.taskRepo,
		runType:          task.TaskRunTypeBackFill,
		taskRunRepo:      h.taskRunRepo,
	}

	return sub, nil
}

// isBackfillDone 检查回填任务是否已完成
func (h *TraceHubServiceImpl) isBackfillDone(ctx context.Context, sub *spanSubscriber) (bool, error) {
	taskRun, err := h.taskRunRepo.GetBackfillTaskRun(ctx, ptr.Of(sub.t.GetWorkspaceID()), sub.t.GetID())
	if err != nil {
		logs.CtxError(ctx, "get backfill task run failed, task_id=%d, err=%v", sub.t.GetID(), err)
		return true, err
	}
	if taskRun == nil {
		logs.CtxError(ctx, "get backfill task run failed, task_id=%d, err=%v", sub.t.GetID(), err)
		return true, err
	}

	return taskRun.RunStatus == task.RunStatusDone, nil
}

func (h *TraceHubServiceImpl) listSpans(ctx context.Context, sub *spanSubscriber) error {
	backfillTime := sub.t.GetRule().GetBackfillEffectiveTime()
	tenants, err := h.getTenants(ctx, loop_span.PlatformType(sub.t.GetRule().GetSpanFilters().GetPlatformType()))
	if err != nil {
		logs.CtxError(ctx, "get tenants failed, task_id=%d, err=%v", sub.t.GetID(), err)
		return err
	}

	// todo: 从配置中获取分页大小
	//batchSize := c.tccCfg.BackfillProcessConfig().ListPageSize
	//if batchSize == 0 {
	//	batchSize = pageSize
	//}
	// 构建查询参数
	listParam := &repo.ListSpansParam{
		Tenants:            tenants,
		Filters:            h.buildSpanFilters(ctx, sub.t),
		StartAt:            backfillTime.GetStartAt(),
		EndAt:              backfillTime.GetEndAt(),
		Limit:              pageSize, // 分页大小
		DescByStartTime:    true,
		NotQueryAnnotation: true, // 回填时不需要查询注解
	}
	taskRun, err := h.taskRunRepo.GetBackfillTaskRun(ctx, ptr.Of(sub.t.GetWorkspaceID()), sub.t.GetID())
	if err != nil {
		logs.CtxError(ctx, "get backfill task run failed, task_id=%d, err=%v", sub.t.GetID(), err)
		return err
	}
	taskRunDTO := tconv.TaskRunPO2DTO(ctx, taskRun, nil)
	if taskRunDTO.BackfillRunDetail.LastSpanPageToken != nil {
		listParam.PageToken = *taskRunDTO.BackfillRunDetail.LastSpanPageToken
	}
	// 分页查询并发送数据
	return h.fetchAndSendSpans(ctx, listParam, sub)
}

type ListSpansReq struct {
	WorkspaceID           int64
	ThirdPartyWorkspaceID string
	StartTime             int64 // ms
	EndTime               int64 // ms
	Filters               *loop_span.FilterFields
	Limit                 int32
	DescByStartTime       bool
	PageToken             string
	PlatformType          loop_span.PlatformType
	SpanListType          loop_span.SpanListType
}

// buildSpanFilters 构建 span 过滤条件
func (h *TraceHubServiceImpl) buildSpanFilters(ctx context.Context, taskConfig *task.Task) *loop_span.FilterFields {
	// 可以根据任务配置构建更复杂的过滤条件
	// 这里简化处理，返回 nil 表示不添加额外过滤

	platformFilter, err := h.buildHelper.BuildPlatformRelatedFilter(ctx, loop_span.PlatformType(taskConfig.GetRule().GetSpanFilters().GetPlatformType()))
	if err != nil {
		return nil
	}
	builtinFilter, err := h.buildBuiltinFilters(ctx, platformFilter, &ListSpansReq{
		WorkspaceID:  taskConfig.GetWorkspaceID(),
		SpanListType: loop_span.SpanListType(taskConfig.GetRule().GetSpanFilters().GetSpanListType()),
	})
	if err != nil {
		return nil
	}
	filters := h.combineFilters(builtinFilter, convertor.FilterFieldsDTO2DO(taskConfig.GetRule().GetSpanFilters().GetFilters()))

	return filters
}

func (h *TraceHubServiceImpl) buildBuiltinFilters(ctx context.Context, f span_filter.Filter, req *ListSpansReq) (*loop_span.FilterFields, error) {
	filters := make([]*loop_span.FilterField, 0)
	env := &span_filter.SpanEnv{
		WorkspaceID:           req.WorkspaceID,
		ThirdPartyWorkspaceID: req.ThirdPartyWorkspaceID,
	}
	basicFilter, forceQuery, err := f.BuildBasicSpanFilter(ctx, env)
	if err != nil {
		return nil, err
	} else if len(basicFilter) == 0 && !forceQuery { // if it's null, no need to query from ck
		return nil, nil
	}
	filters = append(filters, basicFilter...)
	switch req.SpanListType {
	case loop_span.SpanListTypeRootSpan:
		subFilter, err := f.BuildRootSpanFilter(ctx, env)
		if err != nil {
			return nil, err
		}
		filters = append(filters, subFilter...)
	case loop_span.SpanListTypeLLMSpan:
		subFilter, err := f.BuildLLMSpanFilter(ctx, env)
		if err != nil {
			return nil, err
		}
		filters = append(filters, subFilter...)
	case loop_span.SpanListTypeAllSpan:
		subFilter, err := f.BuildALLSpanFilter(ctx, env)
		if err != nil {
			return nil, err
		}
		filters = append(filters, subFilter...)
	default:
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid span list type: %s"))
	}
	filterAggr := &loop_span.FilterFields{
		QueryAndOr:   ptr.Of(loop_span.QueryAndOrEnumAnd),
		FilterFields: filters,
	}
	return filterAggr, nil
}

func (h *TraceHubServiceImpl) combineFilters(filters ...*loop_span.FilterFields) *loop_span.FilterFields {
	filterAggr := &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
	}
	for _, f := range filters {
		if f == nil {
			continue
		}
		filterAggr.FilterFields = append(filterAggr.FilterFields, &loop_span.FilterField{
			QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
			SubFilter:  f,
		})
	}
	return filterAggr
}

// fetchAndSendSpans 分页获取并发送 span 数据
func (h *TraceHubServiceImpl) fetchAndSendSpans(ctx context.Context, listParam *repo.ListSpansParam, sub *spanSubscriber) error {
	totalCount := int64(0)
	pageCount := 0
	pageToken := listParam.PageToken
	
	logs.CtxInfo(ctx, "starting to fetch spans, initial_page_token=%s, task_id=%d", pageToken, sub.t.GetID())
	
	for {
		pageCount++
		logs.CtxDebug(ctx, "fetching page #%d, page_token=%s, task_id=%d", pageCount, pageToken, sub.t.GetID())
		
		result, err := h.traceRepo.ListSpans(ctx, listParam)
		if err != nil {
			logs.CtxError(ctx, "list spans failed, page #%d, task_id=%d, page_token=%s, err=%v", 
				pageCount, sub.t.GetID(), pageToken, err)
			return err
		}

		logs.CtxInfo(ctx, "fetched page #%d with %d spans, has_more=%t, task_id=%d", 
			pageCount, len(result.Spans), result.HasMore, sub.t.GetID())

		if len(result.Spans) > 0 {
			// 发送到通道
			flush := &flushReq{
				retrievedSpanCount: int64(len(result.Spans)),
				pageToken:          result.PageToken,
				spans:              result.Spans,
				noMore:             !result.HasMore,
			}

			select {
			case h.flushCh <- flush:
				totalCount += int64(len(result.Spans))
				logs.CtxInfo(ctx, "sent page #%d (%d spans) to flush channel, total_spans=%d, task_id=%d", 
					pageCount, len(result.Spans), totalCount, sub.t.GetID())
			case <-ctx.Done():
				ctxErr := ctx.Err()
				logs.CtxWarn(ctx, "context cancelled while sending spans on page #%d, task_id=%d, context_error=%v, total_processed=%d", 
					pageCount, sub.t.GetID(), ctxErr, totalCount)
				
				// 详细分析取消原因
				if errors.Is(ctxErr, context.Canceled) {
					logs.CtxWarn(ctx, "fetch operation cancelled manually, page #%d, task_id=%d", pageCount, sub.t.GetID())
				} else if errors.Is(ctxErr, context.DeadlineExceeded) {
					logs.CtxWarn(ctx, "fetch operation timed out, page #%d, task_id=%d", pageCount, sub.t.GetID())
				}
				return ctxErr
			}
		} else {
			logs.CtxInfo(ctx, "page #%d returned no spans, task_id=%d", pageCount, sub.t.GetID())
		}

		if !result.HasMore {
			logs.CtxInfo(ctx, "completed listing spans, total_pages=%d, total_spans=%d, task_id=%d", 
				pageCount, totalCount, sub.t.GetID())
			break
		}

		pageToken = result.PageToken
		listParam.PageToken = pageToken
		logs.CtxDebug(ctx, "continuing to next page, new_page_token=%s, task_id=%d", pageToken, sub.t.GetID())
	}

	return nil
}

func (h *TraceHubServiceImpl) flushSpans(ctx context.Context, sub *spanSubscriber) {
	logs.CtxInfo(ctx, "flush spans goroutine started, task_id=%d", sub.t.GetID())
	defer logs.CtxInfo(ctx, "flush spans goroutine exited, task_id=%d", sub.t.GetID())
	
	processedCount := 0
	for {
		select {
		case fr, ok := <-h.flushCh:
			if !ok {
				// 通道已关闭，退出
				logs.CtxInfo(ctx, "flush channel closed, processed %d batches, task_id=%d", processedCount, sub.t.GetID())
				return
			}

			processedCount++
			logs.CtxDebug(ctx, "processing flush request #%d with %d spans, task_id=%d", 
				processedCount, fr.retrievedSpanCount, sub.t.GetID())
			
			_, _, err := h.doFlush(ctx, fr, sub)
			if err != nil {
				logs.CtxError(ctx, "flush spans failed, batch #%d, task_id=%d, err=%v", 
					processedCount, sub.t.GetID(), err)
				// 收集错误，继续处理
				h.flushErrLock.Lock()
				h.flushErr = append(h.flushErr, err)
				h.flushErrLock.Unlock()
			} else {
				logs.CtxDebug(ctx, "successfully processed flush request #%d, task_id=%d", 
					processedCount, sub.t.GetID())
			}

		case <-ctx.Done():
			// 详细分析上下文取消的原因
			ctxErr := ctx.Err()
			logs.CtxWarn(ctx, "flush spans context cancelled, task_id=%d, processed_batches=%d, context_error=%v", 
				sub.t.GetID(), processedCount, ctxErr)
			
			// 进一步区分取消原因
			if errors.Is(ctxErr, context.Canceled) {
				logs.CtxWarn(ctx, "context was manually cancelled (context.Canceled), task_id=%d, likely due to parent context cancellation or explicit cancel() call", 
					sub.t.GetID())
			} else if errors.Is(ctxErr, context.DeadlineExceeded) {
				logs.CtxWarn(ctx, "context deadline exceeded (context.DeadlineExceeded), task_id=%d, operation timed out", 
					sub.t.GetID())
			} else {
				logs.CtxWarn(ctx, "unknown context cancellation reason, task_id=%d, error=%v", 
					sub.t.GetID(), ctxErr)
			}
			
			// 检查是否还有待处理的数据
			select {
			case fr, ok := <-h.flushCh:
				if ok {
					logs.CtxWarn(ctx, "context cancelled but flush channel still has data, remaining_spans=%d, task_id=%d", 
						fr.retrievedSpanCount, sub.t.GetID())
				}
			default:
				logs.CtxInfo(ctx, "no remaining data in flush channel when context cancelled, task_id=%d", sub.t.GetID())
			}
			
			return
		}
	}
}

func (h *TraceHubServiceImpl) doFlush(ctx context.Context, fr *flushReq, sub *spanSubscriber) (flushed, sampled int, _ error) {
	if fr == nil || len(fr.spans) == 0 {
		logs.CtxDebug(ctx, "flush request is empty, skipping, task_id=%d", sub.t.GetID())
		return 0, 0, nil
	}

	logs.CtxInfo(ctx, "starting to process %d spans for backfill, page_token=%s, no_more=%t, task_id=%d", 
		len(fr.spans), fr.pageToken, fr.noMore, sub.t.GetID())

	// 应用采样逻辑
	logs.CtxDebug(ctx, "applying sampling logic, original_count=%d, task_id=%d", len(fr.spans), sub.t.GetID())
	sampledSpans := h.applySampling(fr.spans, sub)
	if len(sampledSpans) == 0 {
		logs.CtxInfo(ctx, "no spans after sampling, original_count=%d, task_id=%d", len(fr.spans), sub.t.GetID())
		return len(fr.spans), 0, nil
	}
	logs.CtxInfo(ctx, "sampling completed, sampled_count=%d, original_count=%d, task_id=%d", 
		len(sampledSpans), len(fr.spans), sub.t.GetID())

	// 执行具体的业务逻辑处理
	logs.CtxDebug(ctx, "starting business logic processing, sampled_spans=%d, task_id=%d", 
		len(sampledSpans), sub.t.GetID())
	err := h.processSpansForBackfill(ctx, sampledSpans, sub)
	if err != nil {
		logs.CtxError(ctx, "process spans failed, sampled_spans=%d, task_id=%d, err=%v", 
			len(sampledSpans), sub.t.GetID(), err)
		return len(fr.spans), len(sampledSpans), err
	}
	logs.CtxDebug(ctx, "business logic processing completed, task_id=%d", sub.t.GetID())

	// 更新任务运行状态
	logs.CtxDebug(ctx, "updating task run with page token, page_token=%s, task_id=%d", 
		fr.pageToken, sub.t.GetID())
	taskRun, err := h.taskRunRepo.GetBackfillTaskRun(ctx, sub.t.WorkspaceID, sub.t.GetID())
	if err != nil {
		logs.CtxError(ctx, "get backfill task run failed, task_id=%d, err=%v", sub.t.GetID(), err)
		return len(fr.spans), len(sampledSpans), err
	}
	
	taskRunDTO := tconv.TaskRunPO2DTO(ctx, taskRun, nil)
	taskRunDTO.BackfillRunDetail.LastSpanPageToken = ptr.Of(fr.pageToken)
	err = h.taskRunRepo.UpdateTaskRunWithOCC(ctx, taskRunDTO.ID, taskRunDTO.WorkspaceID, map[string]interface{}{
		"backfill_detail": taskRunDTO.BackfillRunDetail,
	})
	if err != nil {
		logs.CtxError(ctx, "update task run failed, task_run_id=%d, task_id=%d, err=%v", 
			taskRunDTO.ID, sub.t.GetID(), err)
		return len(fr.spans), len(sampledSpans), err
	}
	logs.CtxDebug(ctx, "task run updated successfully, page_token=%s, task_id=%d", 
		fr.pageToken, sub.t.GetID())

	logs.CtxInfo(ctx, "successfully processed %d spans (sampled from %d), page_token=%s, task_id=%d",
		len(sampledSpans), len(fr.spans), fr.pageToken, sub.t.GetID())

	return len(fr.spans), len(sampledSpans), nil
}

// applySampling 应用采样逻辑
func (h *TraceHubServiceImpl) applySampling(spans []*loop_span.Span, sub *spanSubscriber) []*loop_span.Span {
	if sub.t == nil || sub.t.Rule == nil {
		return spans
	}

	sampler := sub.t.GetRule().GetSampler()
	if sampler == nil {
		return spans
	}

	sampleRate := sampler.GetSampleRate()
	if sampleRate >= 1.0 {
		return spans // 100% 采样
	}

	if sampleRate <= 0.0 {
		return nil // 0% 采样
	}

	// 计算采样数量
	sampleSize := int(float64(len(spans)) * sampleRate)
	if sampleSize == 0 && len(spans) > 0 {
		sampleSize = 1 // 至少采样一个
	}

	if sampleSize >= len(spans) {
		return spans
	}

	return spans[:sampleSize]
}

// processSpansForBackfill 处理回填的 span 数据
func (h *TraceHubServiceImpl) processSpansForBackfill(ctx context.Context, spans []*loop_span.Span, sub *spanSubscriber) error {
	// 批量处理 spans，提高效率
	const batchSize = 100

	for i := 0; i < len(spans); i += batchSize {
		end := i + batchSize
		if end > len(spans) {
			end = len(spans)
		}

		batch := spans[i:end]
		if err := h.processBatchSpans(ctx, batch, sub); err != nil {
			logs.CtxError(ctx, "process batch spans failed, task_id=%d, batch_start=%d, err=%v",
				sub.t.GetID(), i, err)
			// 继续处理下一批，不因单批失败而中断
			continue
		}
	}

	return nil
}

// processBatchSpans 批量处理 span 数据
func (h *TraceHubServiceImpl) processBatchSpans(ctx context.Context, spans []*loop_span.Span, sub *spanSubscriber) error {
	// 这里实现具体的批量处理逻辑
	// 例如：数据转换、存储、触发下游处理等

	for _, span := range spans {
		// 执行单个 span 的处理逻辑
		if err := h.processIndividualSpan(ctx, span, sub); err != nil {
			logs.CtxWarn(ctx, "process individual span failed, span_id=%s, trace_id=%s, err=%v",
				span.SpanID, span.TraceID, err)
			// 继续处理其他span，不因单个失败而中断批处理
		}
	}

	return nil
}

// processIndividualSpan 处理单个 span
func (h *TraceHubServiceImpl) processIndividualSpan(ctx context.Context, span *loop_span.Span, sub *spanSubscriber) error {
	// 根据任务类型执行相应的处理逻辑
	logs.CtxDebug(ctx, "processing span for backfill, span_id=%s, trace_id=%s, task_id=%d",
		span.SpanID, span.TraceID, sub.t.GetID())
	taskRunConfig, err := h.taskRunRepo.GetBackfillTaskRun(ctx, sub.t.WorkspaceID, sub.t.GetID())
	if err != nil {
		logs.CtxWarn(ctx, "GetLatestNewDataTaskRun, task_id=%d, err=%v", sub.taskID, err)
		return err
	}
	taskCount, _ := h.taskRepo.GetTaskCount(ctx, sub.taskID)
	taskRunCount, _ := h.taskRepo.GetTaskRunCount(ctx, sub.taskID, taskRunConfig.ID)
	sampler := sub.t.GetRule().GetSampler()
	if taskCount+1 > sampler.GetSampleSize() {
		if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
			Task:     sub.t,
			TaskRun:  taskRunConfig,
			IsFinish: true,
		}); err != nil {
			logs.CtxWarn(ctx, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID)
			return err
		}
	}
	if sampler.GetIsCycle() {
		// 达到单次任务上限
		if taskRunCount+1 > sampler.GetCycleCount() {
			if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
				Task:     sub.t,
				TaskRun:  taskRunConfig,
				IsFinish: false,
			}); err != nil {
				logs.CtxWarn(ctx, "taskRunCount+1 > sampler.GetCycleCount(), task_id=%d", sub.taskID)
				return err
			}
		}
	}
	logs.CtxInfo(ctx, "preDispatch, task_id=%d, taskCount=%d, taskRunCount=%d", sub.taskID, taskCount, taskRunCount)
	if err := h.dispatch(ctx, span, []*spanSubscriber{sub}); err != nil {
		return err
	}

	return nil
}

// onHandleDone 处理完成回调
func (h *TraceHubServiceImpl) onHandleDone(ctx context.Context, listErr error, sub *spanSubscriber) error {
	logs.CtxInfo(ctx, "handling backfill completion, task_id=%d", sub.t.GetID())
	
	// 收集所有错误
	h.flushErrLock.Lock()
	allErrors := append([]error{}, h.flushErr...)
	flushErrorCount := len(h.flushErr)
	if listErr != nil {
		allErrors = append(allErrors, listErr)
	}
	h.flushErrLock.Unlock()

	if len(allErrors) > 0 {
		logs.CtxWarn(ctx, "backfill completed with errors, total_errors=%d, flush_errors=%d, list_error=%v, task_id=%d", 
			len(allErrors), flushErrorCount, listErr, sub.t.GetID())
		
		// 详细记录所有错误
		for i, err := range allErrors {
			logs.CtxError(ctx, "backfill error #%d: %v, task_id=%d", i+1, err, sub.t.GetID())
		}
		
		// 返回第一个错误作为代表
		logs.CtxError(ctx, "returning first error as representative, error=%v, task_id=%d", 
			allErrors[0], sub.t.GetID())
		return allErrors[0]
	}

	logs.CtxInfo(ctx, "backfill completed successfully without errors, task_id=%d", sub.t.GetID())
	return nil
}