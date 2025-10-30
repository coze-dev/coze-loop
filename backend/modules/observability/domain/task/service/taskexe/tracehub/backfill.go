// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
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
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	pageSize                = 500
	backfillLockKeyTemplate = "observability:tracehub:backfill:%d"
	backfillLockMaxHold     = 24 * time.Hour
)

// 定时任务+锁
func (h *TraceHubServiceImpl) BackFill(ctx context.Context, event *entity.BackFillEvent) error {
	// 1. Set the current task context
	ctx = h.fillCtx(ctx)
	logs.CtxInfo(ctx, "BackFill msg %+v", event)

	var (
		lockKey    string
		lockCancel func()
	)
	if h.locker != nil && event != nil {
		lockKey = fmt.Sprintf(backfillLockKeyTemplate, event.TaskID)
		locked, lockCtx, cancel, lockErr := h.locker.LockWithRenew(ctx, lockKey, transformTaskStatusLockTTL, backfillLockMaxHold)
		if lockErr != nil {
			logs.CtxError(ctx, "backfill acquire lock failed", "task_id", event.TaskID, "err", lockErr)
			return lockErr
		}
		if !locked {
			logs.CtxInfo(ctx, "backfill lock held by others, skip execution", "task_id", event.TaskID)
			return nil
		}
		lockCancel = cancel
		ctx = lockCtx
		defer func(cancel func()) {
			if cancel != nil {
				cancel()
			} else if lockKey != "" {
				if _, err := h.locker.Unlock(lockKey); err != nil {
					logs.CtxWarn(ctx, "backfill release lock failed", "task_id", event.TaskID, "err", err)
				}
			}
		}(lockCancel)
	}

	sub, err := h.setBackfillTask(ctx, event)
	if err != nil {
		return err
	}

	if sub != nil && sub.t != nil && sub.t.GetBaseInfo() != nil && sub.t.GetBaseInfo().GetCreatedBy() != nil {
		ctx = session.WithCtxUser(ctx, &session.User{ID: sub.t.GetBaseInfo().GetCreatedBy().GetUserID()})
	}

	// 2. Determine whether the backfill task is completed to avoid repeated execution
	isDone, err := h.isBackfillDone(ctx, sub)
	if err != nil {
		logs.CtxError(ctx, "check backfill task done failed, task_id=%d, err=%v", sub.t.GetID(), err)
		return err
	}
	if isDone {
		logs.CtxInfo(ctx, "backfill already completed, task_id=%d", sub.t.GetID())
		return nil
	}

	// 顺序执行时重置 flush 错误收集器
	h.flushErrLock.Lock()
	h.flushErr = nil
	h.flushErrLock.Unlock()

	// 5. Retrieve span data from the observability service
	listErr := h.listAndSendSpans(ctx, sub)
	if listErr != nil {
		logs.CtxError(ctx, "list spans failed, task_id=%d, err=%v", sub.t.GetID(), listErr)
	}

	// 6. Synchronously wait for completion to ensure all data is processed
	return h.onHandleDone(ctx, listErr, sub)
}

// setBackfillTask sets the context for the current backfill task
func (h *TraceHubServiceImpl) setBackfillTask(ctx context.Context, event *entity.BackFillEvent) (*spanSubscriber, error) {
	taskConfig, err := h.taskRepo.GetTask(ctx, event.TaskID, nil, nil)
	if err != nil {
		logs.CtxError(ctx, "get task config failed, task_id=%d, err=%v", event.TaskID, err)
		return nil, err
	}
	if taskConfig == nil {
		return nil, errors.New("task config not found")
	}
	taskConfigDO := tconv.TaskDO2DTO(ctx, taskConfig, nil)
	taskRun, err := h.taskRepo.GetBackfillTaskRun(ctx, ptr.Of(taskConfigDO.GetWorkspaceID()), taskConfigDO.GetID())
	if err != nil {
		logs.CtxError(ctx, "get backfill task run failed, task_id=%d, err=%v", taskConfigDO.GetID(), err)
		return nil, err
	}
	taskRunDTO := tconv.TaskRunDO2DTO(ctx, taskRun, nil)
	proc := h.taskProcessor.GetTaskProcessor(taskConfig.TaskType)
	sub := &spanSubscriber{
		taskID:           taskConfigDO.GetID(),
		t:                taskConfigDO,
		tr:               taskRunDTO,
		processor:        proc,
		bufCap:           0,
		maxFlushInterval: time.Second * 5,
		taskRepo:         h.taskRepo,
		runType:          task.TaskRunTypeBackFill,
	}

	return sub, nil
}

// isBackfillDone checks whether the backfill task has been completed
func (h *TraceHubServiceImpl) isBackfillDone(ctx context.Context, sub *spanSubscriber) (bool, error) {
	if sub.tr == nil {
		logs.CtxError(ctx, "get backfill task run failed, task_id=%d, err=%v", sub.t.GetID(), nil)
		return true, nil
	}

	return sub.tr.RunStatus == task.RunStatusDone, nil
}

func (h *TraceHubServiceImpl) listAndSendSpans(ctx context.Context, sub *spanSubscriber) error {
	backfillTime := sub.t.GetRule().GetBackfillEffectiveTime()
	tenants, err := h.getTenants(ctx, loop_span.PlatformType(sub.t.GetRule().GetSpanFilters().GetPlatformType()))
	if err != nil {
		logs.CtxError(ctx, "get tenants failed, task_id=%d, err=%v", sub.t.GetID(), err)
		return err
	}

	// Build query parameters
	listParam := &repo.ListSpansParam{
		Storage:            h.storageProvider.GetTraceStorage(ctx, strconv.FormatInt(sub.t.GetWorkspaceID(), 10)),
		Tenants:            tenants,
		Filters:            h.buildSpanFilters(ctx, sub.t),
		StartAt:            backfillTime.GetStartAt(),
		EndAt:              backfillTime.GetEndAt(),
		Limit:              pageSize, // Page size
		DescByStartTime:    true,
		NotQueryAnnotation: true, // No annotation query required during backfill
	}

	if sub.tr.BackfillRunDetail != nil && sub.tr.BackfillRunDetail.LastSpanPageToken != nil {
		listParam.PageToken = *sub.tr.BackfillRunDetail.LastSpanPageToken
	}
	// Paginate query and send data
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

// buildSpanFilters constructs span filter conditions
func (h *TraceHubServiceImpl) buildSpanFilters(ctx context.Context, taskConfig *task.Task) *loop_span.FilterFields {
	// More complex filters can be built based on the task configuration
	// Simplified here: return nil to indicate no additional filters

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
		Source:                span_filter.SourceTypeAutoTask,
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

// fetchAndSendSpans paginates and sends span data
func (h *TraceHubServiceImpl) fetchAndSendSpans(ctx context.Context, listParam *repo.ListSpansParam, sub *spanSubscriber) error {
	totalCount := int64(0)
	pageToken := listParam.PageToken
	for {
		logs.CtxInfo(ctx, "ListSpansParam:%v", listParam)
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
			flush := &flushReq{
				retrievedSpanCount: int64(len(spans)),
				pageToken:          result.PageToken,
				spans:              spans,
				noMore:             !result.HasMore,
			}

			if err = h.flushSpans(ctx, flush, sub); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return err
				}
			}

			totalCount += int64(len(spans))
			logs.CtxInfo(ctx, "processed %d spans, total=%d, task_id=%d", len(spans), totalCount, sub.t.GetID())
		}

		if !result.HasMore {
			logs.CtxInfo(ctx, "completed listing spans, total_count=%d, task_id=%d", totalCount, sub.t.GetID())
			break
		}

		pageToken = result.PageToken
	}

	return nil
}

func (h *TraceHubServiceImpl) flushSpans(ctx context.Context, fr *flushReq, sub *spanSubscriber) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	_, _, err := h.doFlush(ctx, fr, sub)
	if err != nil {
		logs.CtxError(ctx, "flush spans failed, task_id=%d, err=%v", sub.t.GetID(), err)
		h.flushErrLock.Lock()
		h.flushErr = append(h.flushErr, err)
		h.flushErrLock.Unlock()
	}

	return nil
}

func (h *TraceHubServiceImpl) doFlush(ctx context.Context, fr *flushReq, sub *spanSubscriber) (flushed, sampled int, _ error) {
	if fr == nil || len(fr.spans) == 0 {
		return 0, 0, nil
	}

	logs.CtxInfo(ctx, "processing %d spans for backfill, task_id=%d", len(fr.spans), sub.t.GetID())

	// Apply sampling logic
	sampledSpans := h.applySampling(fr.spans, sub)
	if len(sampledSpans) == 0 {
		logs.CtxInfo(ctx, "no spans after sampling, task_id=%d", sub.t.GetID())
		return len(fr.spans), 0, nil
	}

	// Execute specific business logic
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
		logs.CtxInfo(ctx, "no more spans to process, task_id=%d", sub.t.GetID())
		if err = sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
			Task:     tconv.TaskDTO2DO(sub.t, "", nil),
			TaskRun:  tconv.TaskRunDTO2DO(sub.tr),
			IsFinish: false,
		}); err != nil {
			return len(fr.spans), len(sampledSpans), err
		}
	}

	logs.CtxInfo(ctx, "successfully processed %d spans (sampled from %d), task_id=%d",
		len(sampledSpans), len(fr.spans), sub.t.GetID())
	return len(fr.spans), len(sampledSpans), nil
}

// applySampling applies sampling logic
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
		return spans // 100% sampling
	}

	if sampleRate <= 0.0 {
		return nil // 0% sampling
	}

	// Calculate sampling size
	sampleSize := int(float64(len(spans)) * sampleRate)
	if sampleSize == 0 && len(spans) > 0 {
		sampleSize = 1 // Sample at least one
	}

	if sampleSize >= len(spans) {
		return spans
	}

	return spans[:sampleSize]
}

// processSpansForBackfill handles spans for backfill
func (h *TraceHubServiceImpl) processSpansForBackfill(ctx context.Context, spans []*loop_span.Span, sub *spanSubscriber) error {
	// Batch processing spans for efficiency
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
			// Continue with the next batch without stopping due to a single failure
			continue
		}
	}

	return nil
}

// processBatchSpans processes a batch of span data
func (h *TraceHubServiceImpl) processBatchSpans(ctx context.Context, spans []*loop_span.Span, sub *spanSubscriber) error {
	for _, span := range spans {
		// Execute processing logic according to the task type
		logs.CtxInfo(ctx, "processing span for backfill, span_id=%s, trace_id=%s, task_id=%d",
			span.SpanID, span.TraceID, sub.t.GetID())
		taskCount, _ := h.taskRepo.GetTaskCount(ctx, sub.taskID)
		taskRunCount, _ := h.taskRepo.GetTaskRunCount(ctx, sub.taskID, sub.tr.GetID())
		sampler := sub.t.GetRule().GetSampler()
		if taskCount+1 > sampler.GetSampleSize() {
			logs.CtxWarn(ctx, "taskCount+1 > sampler.GetSampleSize(), task_id=%d,SampleSize=%d", sub.taskID, sampler.GetSampleSize())
			if err := sub.processor.OnFinishTaskChange(ctx, taskexe.OnFinishTaskChangeReq{
				Task:     tconv.TaskDTO2DO(sub.t, "", nil),
				TaskRun:  tconv.TaskRunDTO2DO(sub.tr),
				IsFinish: true,
			}); err != nil {
				return err
			}
			break
		}
		logs.CtxInfo(ctx, "preDispatch, task_id=%d, taskCount=%d, taskRunCount=%d", sub.taskID, taskCount, taskRunCount)
		if err := h.dispatch(ctx, span, []*spanSubscriber{sub}); err != nil {
			return err
		}
	}

	return nil
}

// onHandleDone handles completion callback
func (h *TraceHubServiceImpl) onHandleDone(ctx context.Context, listErr error, sub *spanSubscriber) error {
	// Collect all errors
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

		// Send MQ message asynchronously without blocking task creation flow
		go func() {
			if err := h.sendBackfillMessage(context.Background(), backfillEvent); err != nil {
				logs.CtxWarn(ctx, "send backfill message failed, task_id=%d, err=%v", sub.t.GetID(), err)
			}
		}()
		logs.CtxWarn(ctx, "backfill completed with %d errors, task_id=%d", len(allErrors), sub.t.GetID())
		// Return the first error as a representative
		return allErrors[0]

	}

	logs.CtxInfo(ctx, "backfill completed successfully, task_id=%d", sub.t.GetID())
	return nil
}

// sendBackfillMessage sends an MQ message
func (h *TraceHubServiceImpl) sendBackfillMessage(ctx context.Context, event *entity.BackFillEvent) error {
	if h.backfillProducer == nil {
		return errorx.NewByCode(obErrorx.CommonInternalErrorCode, errorx.WithExtraMsg("backfill producer not initialized"))
	}

	return h.backfillProducer.SendBackfill(ctx, event)
}
