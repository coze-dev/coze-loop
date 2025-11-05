// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
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
	backfillLockTTL         = 3 * time.Minute
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
		locked, lockCtx, cancel, lockErr := h.locker.LockWithRenew(ctx, lockKey, backfillLockTTL, backfillLockMaxHold)
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

	sub, err := h.buildSubscriber(ctx, event)
	if err != nil {
		return err
	}
	if sub == nil || sub.t == nil {
		return errors.New("subscriber or task config not found")
	}

	// todo tyf 是否需要
	if sub.t != nil && sub.t.CreatedBy != "" {
		ctx = session.WithCtxUser(ctx, &session.User{ID: sub.t.CreatedBy})
	}

	// 2. Determine whether the backfill task is completed to avoid repeated execution
	isDone, err := h.isBackfillDone(ctx, sub)
	if err != nil {
		logs.CtxError(ctx, "check backfill task done failed, task_id=%d, err=%v", sub.t.ID, err)
		return err
	}
	if isDone {
		logs.CtxInfo(ctx, "backfill already completed, task_id=%d", sub.t.ID)
		return nil
	}

	// 5. Retrieve span data from the observability service
	err = h.listAndSendSpans(ctx, sub)

	return h.onHandleDone(ctx, err, sub)
}

// buildSubscriber sets the context for the current backfill task
func (h *TraceHubServiceImpl) buildSubscriber(ctx context.Context, event *entity.BackFillEvent) (*spanSubscriber, error) {
	taskDO, err := h.taskRepo.GetTask(ctx, event.TaskID, nil, nil)
	if err != nil {
		logs.CtxError(ctx, "get task config failed, task_id=%d, err=%v", event.TaskID, err)
		return nil, err
	}
	if taskDO == nil {
		return nil, errors.New("task config not found")
	}

	taskRun := taskDO.GetBackfillTaskRun()
	if taskRun == nil {
		logs.CtxError(ctx, "get backfill task run failed, task_id=%d, err=%v", taskDO.ID)
		return nil, errors.New("get backfill task run not found")
	}

	proc := h.taskProcessor.GetTaskProcessor(taskDO.TaskType)
	sub := &spanSubscriber{
		taskID:    taskDO.ID,
		t:         taskDO,
		tr:        taskRun,
		processor: proc,
		taskRepo:  h.taskRepo,
		runType:   entity.TaskRunTypeBackFill,
	}

	return sub, nil
}

// isBackfillDone checks whether the backfill task has been completed
func (h *TraceHubServiceImpl) isBackfillDone(ctx context.Context, sub *spanSubscriber) (bool, error) {
	if sub.tr == nil {
		logs.CtxError(ctx, "get backfill task run failed, task_id=%d, err=%v", sub.t.ID, nil)
		return true, nil
	}

	return sub.tr.RunStatus == task.RunStatusDone, nil
}

func (h *TraceHubServiceImpl) listAndSendSpans(ctx context.Context, sub *spanSubscriber) error {
	backfillTime := sub.t.BackfillEffectiveTime
	tenants, err := h.getTenants(ctx, sub.t.SpanFilter.PlatformType)
	if err != nil {
		logs.CtxError(ctx, "get tenants failed, task_id=%d, err=%v", sub.t.ID, err)
		return err
	}

	// Build query parameters
	listParam := &repo.ListSpansParam{
		Tenants:            tenants,
		Filters:            h.buildSpanFilters(ctx, sub.t),
		StartAt:            backfillTime.StartAt,
		EndAt:              backfillTime.EndAt,
		Limit:              pageSize, // Page size
		DescByStartTime:    true,
		NotQueryAnnotation: true, // No annotation query required during backfill
	}

	if sub.tr.BackfillDetail != nil && sub.tr.BackfillDetail.LastSpanPageToken != nil {
		listParam.PageToken = *sub.tr.BackfillDetail.LastSpanPageToken
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
func (h *TraceHubServiceImpl) buildSpanFilters(ctx context.Context, taskConfig *entity.ObservabilityTask) *loop_span.FilterFields {
	// More complex filters can be built based on the task configuration
	// Simplified here: return nil to indicate no additional filters

	platformFilter, err := h.buildHelper.BuildPlatformRelatedFilter(ctx, taskConfig.SpanFilter.PlatformType)
	if err != nil {
		logs.CtxError(ctx, "build platform filter failed, task_id=%d, err=%v", taskConfig.ID, err)
		// 不需要重试
		return nil
	}
	builtinFilter, err := h.buildBuiltinFilters(ctx, platformFilter, &ListSpansReq{
		WorkspaceID:  taskConfig.WorkspaceID,
		SpanListType: taskConfig.SpanFilter.SpanListType,
	})
	if err != nil {
		logs.CtxError(ctx, "build builtin filter failed, task_id=%d, err=%v", taskConfig.ID, err)
		// 不需要重试
		return nil
	}
	filters := h.combineFilters(builtinFilter, &taskConfig.SpanFilter.Filters)

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
		logs.CtxInfo(ctx, "TaskID: %d, ListSpansParam:%v", sub.t.ID, listParam)
		result, err := h.traceRepo.ListSpans(ctx, listParam)
		if err != nil {
			logs.CtxError(ctx, "List spans failed, task_id=%d, page_token=%s, err=%v", sub.t.ID, pageToken, err)
			return err
		}
		logs.CtxInfo(ctx, "Fetch %d spans, total=%d, task_id=%d", len(result.Spans), totalCount, sub.t.ID)

		spans := result.Spans
		processors, err := h.buildHelper.BuildGetTraceProcessors(ctx, span_processor.Settings{
			WorkspaceId:    sub.t.WorkspaceID,
			PlatformType:   sub.t.SpanFilter.PlatformType,
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
			logs.CtxInfo(ctx, "Processed %d spans completed, total=%d, task_id=%d", len(spans), totalCount, sub.t.ID)
		}

		if !result.HasMore {
			logs.CtxInfo(ctx, "Completed listing spans, total_count=%d, task_id=%d", totalCount, sub.t.ID)
			break
		}
		listParam.PageToken = result.PageToken
		pageToken = result.PageToken
	}

	return nil
}

func (h *TraceHubServiceImpl) flushSpans(ctx context.Context, fr *flushReq, sub *spanSubscriber) error {
	if fr == nil || len(fr.spans) == 0 {
		return nil
	}

	logs.CtxInfo(ctx, "Start processing %d spans for backfill, task_id=%d", len(fr.spans), sub.t.ID)

	// Apply sampling logic
	sampledSpans := h.applySampling(fr.spans, sub)
	if len(sampledSpans) == 0 {
		logs.CtxInfo(ctx, "no spans after sampling, task_id=%d", sub.t.ID)
		return nil
	}

	// Execute specific business logic
	err := h.processSpansForBackfill(ctx, sampledSpans, sub)
	if err != nil {
		logs.CtxError(ctx, "process spans failed, task_id=%d, err=%v", sub.t.ID, err)
		return err
	}

	// todo 不应该这里直接写po字段
	err = h.taskRepo.UpdateTaskRunWithOCC(ctx, sub.tr.ID, sub.tr.WorkspaceID, map[string]interface{}{
		"backfill_detail": ToJSONString(ctx, sub.tr.BackfillDetail),
	})
	if err != nil {
		logs.CtxError(ctx, "update task run failed, task_id=%d, err=%v", sub.t.ID, err)
		return err
	}
	if fr.noMore {
		logs.CtxInfo(ctx, "no more spans to process, task_id=%d", sub.t.ID)
		if err = sub.processor.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{
			Task:     sub.t,
			TaskRun:  sub.tr,
			IsFinish: false,
		}); err != nil {
			return err
		}
	}

	logs.CtxInfo(ctx, "successfully processed %d spans (sampled from %d), task_id=%d",
		len(sampledSpans), len(fr.spans), sub.t.ID)
	return nil
}

// applySampling applies sampling logic
func (h *TraceHubServiceImpl) applySampling(spans []*loop_span.Span, sub *spanSubscriber) []*loop_span.Span {
	sampler := sub.t.Sampler
	if sampler == nil {
		return spans
	}

	sampleRate := sampler.SampleRate
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
	const batchSize = 50

	for i := 0; i < len(spans); i += batchSize {
		end := i + batchSize
		if end > len(spans) {
			end = len(spans)
		}

		batch := spans[i:end]
		if err := h.processBatchSpans(ctx, batch, sub); err != nil {
			logs.CtxError(ctx, "process batch spans failed, task_id=%d, batch_start=%d, err=%v",
				sub.t.ID, i, err)
			// Continue with the next batch without stopping due to a single failure
			continue
		}
		// ml_flow rate-limited: 50/5s
		time.Sleep(5 * time.Second)
	}

	return nil
}

// processBatchSpans processes a batch of span data
func (h *TraceHubServiceImpl) processBatchSpans(ctx context.Context, spans []*loop_span.Span, sub *spanSubscriber) error {
	for _, span := range spans {
		// Execute processing logic according to the task type
		logs.CtxInfo(ctx, "processing span for backfill, span_id=%s, trace_id=%s, task_id=%d",
			span.SpanID, span.TraceID, sub.t.ID)
		taskCount, _ := h.taskRepo.GetTaskCount(ctx, sub.taskID)
		taskRunCount, _ := h.taskRepo.GetTaskRunCount(ctx, sub.taskID, sub.tr.ID)
		sampler := sub.t.Sampler
		if taskCount+1 > sampler.SampleSize {
			logs.CtxInfo(ctx, "taskCount+1 > sampler.GetSampleSize(), task_id=%d,SampleSize=%d", sub.taskID, sampler.SampleSize)
			if err := sub.processor.OnTaskFinished(ctx, taskexe.OnTaskFinishedReq{
				Task:     sub.t,
				TaskRun:  sub.tr,
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
func (h *TraceHubServiceImpl) onHandleDone(ctx context.Context, err error, sub *spanSubscriber) error {
	if err == nil {
		logs.CtxInfo(ctx, "backfill completed successfully, task_id=%d", sub.t.ID)
		return nil
	}

	// failed, need retry
	logs.CtxWarn(ctx, "backfill completed with error: %v, task_id=%d", err, sub.t.ID)
	backfillEvent := &entity.BackFillEvent{
		SpaceID: sub.t.WorkspaceID,
		TaskID:  sub.t.ID,
	}
	if sendErr := h.sendBackfillMessage(context.Background(), backfillEvent); sendErr != nil {
		logs.CtxWarn(ctx, "send backfill message failed, task_id=%d, err=%v", sub.t.ID, sendErr)
		return sendErr
	}
	// 依靠MQ进行重试
	return nil
}

// sendBackfillMessage sends an MQ message
func (h *TraceHubServiceImpl) sendBackfillMessage(ctx context.Context, event *entity.BackFillEvent) error {
	if h.backfillProducer == nil {
		return errorx.NewByCode(obErrorx.CommonInternalErrorCode, errorx.WithExtraMsg("backfill producer not initialized"))
	}

	return h.backfillProducer.SendBackfill(ctx, event)
}
