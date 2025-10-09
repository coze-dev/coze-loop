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
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_processor"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const pageSize = 500

func (h *TraceHubServiceImpl) BackFill(ctx context.Context, event *entity.BackFillEvent) error {
	// 1. 设置当前任务上下文
	ctx = fillCtxWithEnv(ctx)
	sub, err := h.setBackfillTask(ctx, event)
	if err != nil {
		return err
	}

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
	defer cancel()
	wg := sync.WaitGroup{}

	// 初始化 flushCh 通道和错误收集器
	h.flushCh = make(chan *flushReq, 100) // 缓冲通道，避免阻塞
	h.flushErrLock.Lock()
	h.flushErr = nil // 重置错误收集器
	h.flushErrLock.Unlock()

	// 4. 启动异步刷新处理 - 通过 goroutine 实现并发处理
	wg.Add(1)
	goroutine.Go(ctx, func() {
		defer wg.Done()
		h.flushSpans(subCtx, sub)
	})

	// 5. 获取 span 数据 - 从观测服务获取需要处理的数据
	listErr := h.listSpans(subCtx, sub)
	if listErr != nil {
		logs.CtxError(ctx, "list spans failed, task_id=%d, err=%v", sub.t.GetID(), listErr)
		// continue on error，不中断处理流程
	}

	// 关闭通道并等待处理完成
	close(h.flushCh)
	wg.Wait()

	// 6. 同步等待完成 - 确保所有数据处理完毕
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
	taskRun, err := h.taskRepo.GetBackfillTaskRun(ctx, ptr.Of(taskConfigDO.GetWorkspaceID()), taskConfigDO.GetID())
	if err != nil {
		logs.CtxError(ctx, "get backfill task run failed, task_id=%d, err=%v", taskConfigDO.GetID(), err)
		return nil, err
	}
	taskRunDO := tconv.TaskRunPO2DTO(ctx, taskRun, nil)
	proc := h.taskProcessor.GetTaskProcessor(taskConfig.TaskType)
	sub := &spanSubscriber{
		taskID:           taskConfigDO.GetID(),
		RWMutex:          sync.RWMutex{},
		t:                taskConfigDO,
		tr:               taskRunDO,
		processor:        proc,
		bufCap:           0,
		flushWait:        sync.WaitGroup{},
		maxFlushInterval: time.Second * 5,
		taskRepo:         h.taskRepo,
		runType:          task.TaskRunTypeBackFill,
	}

	return sub, nil
}

// isBackfillDone 检查回填任务是否已完成
func (h *TraceHubServiceImpl) isBackfillDone(ctx context.Context, sub *spanSubscriber) (bool, error) {
	if sub.tr == nil {
		logs.CtxError(ctx, "get backfill task run failed, task_id=%d, err=%v", sub.t.GetID(), nil)
		return true, nil
	}

	return sub.tr.RunStatus == task.RunStatusDone, nil
}

func (h *TraceHubServiceImpl) listSpans(ctx context.Context, sub *spanSubscriber) error {
	backfillTime := sub.t.GetRule().GetBackfillEffectiveTime()
	tenants, err := h.getTenants(ctx, loop_span.PlatformType(sub.t.GetRule().GetSpanFilters().GetPlatformType()))
	if err != nil {
		logs.CtxError(ctx, "get tenants failed, task_id=%d, err=%v", sub.t.GetID(), err)
		return err
	}

	// todo: 从tcc配置中获取分页大小
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

	if sub.tr.BackfillRunDetail != nil && sub.tr.BackfillRunDetail.LastSpanPageToken != nil {
		listParam.PageToken = *sub.tr.BackfillRunDetail.LastSpanPageToken
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
	pageToken := listParam.PageToken
	for {
		result, err := h.traceRepo.ListSpans(ctx, listParam)
		if err != nil {
			logs.CtxError(ctx, "list spans failed, task_id=%d, page_token=%s, err=%v", sub.t.GetID(), pageToken, err)
			return err
		}
		spans := result.Spans
		processors, err := h.buildHelper.BuildGetTraceProcessors(ctx, span_processor.Settings{
			WorkspaceId:    sub.t.GetWorkspaceID(),
			PlatformType:   loop_span.PlatformType(sub.t.GetRule().GetSpanFilters().GetPlatformType()),
			QueryStartTime: listParam.StartAt,
			QueryEndTime:   listParam.EndAt,
		})
		if err != nil {
			return errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
		}
		for _, p := range processors {
			spans, err = p.Transform(ctx, spans)
			if err != nil {
				return errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
			}
		}

		if len(spans) > 0 {
			// 发送到通道
			flush := &flushReq{
				retrievedSpanCount: int64(len(spans)),
				pageToken:          result.PageToken,
				spans:              spans,
				noMore:             !result.HasMore,
			}

			select {
			case h.flushCh <- flush:
				totalCount += int64(len(spans))
				logs.CtxInfo(ctx, "sent %d spans to flush channel, total=%d, task_id=%d", len(spans), totalCount, sub.t.GetID())
			case <-ctx.Done():
				logs.CtxWarn(ctx, "context cancelled while sending spans, task_id=%d", sub.t.GetID())
				return ctx.Err()
			}
		}

		if !result.HasMore {
			logs.CtxInfo(ctx, "completed listing spans, total_count=%d, task_id=%d", totalCount, sub.t.GetID())
			break
		}

		pageToken = result.PageToken
	}

	return nil
}

func (h *TraceHubServiceImpl) flushSpans(ctx context.Context, sub *spanSubscriber) {
	for {
		select {
		case fr, ok := <-h.flushCh:
			if !ok {
				// 通道已关闭，退出
				return
			}

			_, _, err := h.doFlush(ctx, fr, sub)
			if err != nil {
				logs.CtxError(ctx, "flush spans failed, task_id=%d, err=%v", sub.t.GetID(), err)
				// 收集错误，继续处理
				h.flushErrLock.Lock()
				h.flushErr = append(h.flushErr, err)
				h.flushErrLock.Unlock()
			}

		case <-ctx.Done():
			logs.CtxWarn(ctx, "flush spans context cancelled, task_id=%d", sub.t.GetID())
			return
		}
	}
}

func (h *TraceHubServiceImpl) doFlush(ctx context.Context, fr *flushReq, sub *spanSubscriber) (flushed, sampled int, _ error) {
	if fr == nil || len(fr.spans) == 0 {
		return 0, 0, nil
	}

	logs.CtxInfo(ctx, "processing %d spans for backfill, task_id=%d", len(fr.spans), sub.t.GetID())

	// 应用采样逻辑
	sampledSpans := h.applySampling(fr.spans, sub)
	if len(sampledSpans) == 0 {
		logs.CtxInfo(ctx, "no spans after sampling, task_id=%d", sub.t.GetID())
		return len(fr.spans), 0, nil
	}

	// 执行具体的业务逻辑处理
	err := h.processSpansForBackfill(ctx, sampledSpans, sub)
	if err != nil {
		logs.CtxError(ctx, "process spans failed, task_id=%d, err=%v", sub.t.GetID(), err)
		return len(fr.spans), len(sampledSpans), err
	}

	sub.tr.BackfillRunDetail = &task.BackfillDetail{
		LastSpanPageToken: ptr.Of(fr.pageToken),
	}
	err = h.taskRepo.UpdateTaskRunWithOCC(ctx, sub.tr.ID, sub.tr.WorkspaceID, map[string]interface{}{
		"backfill_detail": ToJSONString(ctx, sub.tr.BackfillRunDetail),
	})
	if err != nil {
		logs.CtxError(ctx, "update task run failed, task_id=%d, err=%v", sub.t.GetID(), err)
		return len(fr.spans), len(sampledSpans), err
	}
	if fr.noMore {
		if err = sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
			Task:     sub.t,
			TaskRun:  tconv.TaskRunDO2PO(ctx, sub.tr, nil),
			IsFinish: false,
		}); err != nil {
			logs.CtxWarn(ctx, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID)
			return len(fr.spans), len(sampledSpans), err
		}
	}

	logs.CtxInfo(ctx, "successfully processed %d spans (sampled from %d), task_id=%d",
		len(sampledSpans), len(fr.spans), sub.t.GetID())
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
	for _, span := range spans {
		// 执行单个 span 的处理逻辑
		if err := h.processSpan(ctx, span, sub); err != nil {
			logs.CtxWarn(ctx, "process individual span failed, span_id=%s, trace_id=%s, err=%v",
				span.SpanID, span.TraceID, err)
			// 继续处理其他span，不因单个失败而中断批处理
		}
	}

	return nil
}

// processIndividualSpan 处理单个 span
func (h *TraceHubServiceImpl) processSpan(ctx context.Context, span *loop_span.Span, sub *spanSubscriber) error {
	// 根据任务类型执行相应的处理逻辑
	logs.CtxDebug(ctx, "processing span for backfill, span_id=%s, trace_id=%s, task_id=%d",
		span.SpanID, span.TraceID, sub.t.GetID())

	taskCount, _ := h.taskRepo.GetTaskCount(ctx, sub.taskID)
	taskRunCount, _ := h.taskRepo.GetTaskRunCount(ctx, sub.taskID, sub.tr.GetID())
	sampler := sub.t.GetRule().GetSampler()
	if taskCount+1 > sampler.GetSampleSize() {
		if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
			Task:     sub.t,
			TaskRun:  tconv.TaskRunDO2PO(ctx, sub.tr, nil),
			IsFinish: true,
		}); err != nil {
			logs.CtxWarn(ctx, "time.Now().After(endTime) Finish processor, task_id=%d", sub.taskID)
			return err
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
	// 收集所有错误
	h.flushErrLock.Lock()
	allErrors := append([]error{}, h.flushErr...)
	if listErr != nil {
		allErrors = append(allErrors, listErr)
	}
	h.flushErrLock.Unlock()

	if len(allErrors) > 0 {
		backfillEvent := &entity.BackFillEvent{
			SpaceID: sub.t.GetWorkspaceID(),
			TaskID:  sub.t.GetID(),
		}

		// 异步发送MQ消息，不阻塞任务创建流程
		go func() {
			if err := h.sendBackfillMessage(context.Background(), backfillEvent); err != nil {
				logs.CtxWarn(ctx, "send backfill message failed, task_id=%d, err=%v", sub.t.GetID(), err)
			}
		}()
		logs.CtxWarn(ctx, "backfill completed with %d errors, task_id=%d", len(allErrors), sub.t.GetID())
		// 返回第一个错误作为代表
		return allErrors[0]

	}

	logs.CtxInfo(ctx, "backfill completed successfully, task_id=%d", sub.t.GetID())
	return nil
}

// sendBackfillMessage 发送MQ消息
func (h *TraceHubServiceImpl) sendBackfillMessage(ctx context.Context, event *entity.BackFillEvent) error {
	if h.backfillProducer == nil {
		return errorx.NewByCode(obErrorx.CommonInternalErrorCode, errorx.WithExtraMsg("backfill producer not initialized"))
	}

	return h.backfillProducer.SendBackfill(ctx, event)
}
