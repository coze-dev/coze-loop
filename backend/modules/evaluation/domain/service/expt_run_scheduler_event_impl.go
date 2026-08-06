// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/backoff"
	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	gslice "github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptSchedulerImpl struct {
	Manager                  IExptManager
	ExptRepo                 repo.IExperimentRepo
	Publisher                events.ExptEventPublisher
	ExptItemResultRepo       repo.IExptItemResultRepo
	ExptTurnResultRepo       repo.IExptTurnResultRepo
	EvaluatorRecordRepo      repo.IEvaluatorRecordRepo
	ExptStatsRepo            repo.IExptStatsRepo
	ExptRunLogRepo           repo.IExptRunLogRepo
	Idem                     idem.IdempotentService
	Configer                 component.IConfiger
	QuotaRepo                repo.QuotaRepo
	Mutex                    lock.ILocker
	AuditClient              audit.IAuditService
	Metric                   metrics.ExptMetric
	Endpoints                SchedulerEndPoint
	ResultSvc                ExptResultService
	IDGen                    idgen.IIDGenerator
	evaluationSetItemService EvaluationSetItemService
	schedulerModeFactory     SchedulerModeFactory
	evalTargetService        IEvalTargetService
}

func NewExptSchedulerSvc(
	manager IExptManager,
	exptRepo repo.IExperimentRepo,
	exptItemResultRepo repo.IExptItemResultRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	evaluatorRecordRepo repo.IEvaluatorRecordRepo,
	exptStatsRepo repo.IExptStatsRepo,
	exptRunLogRepo repo.IExptRunLogRepo,
	Idem idem.IdempotentService,
	configer component.IConfiger,
	quotaRepo repo.QuotaRepo,
	mutex lock.ILocker,
	publisher events.ExptEventPublisher,
	auditClient audit.IAuditService,
	metric metrics.ExptMetric,
	resultSvc ExptResultService,
	idGen idgen.IIDGenerator,
	evaluationSetItemService EvaluationSetItemService,
	schedulerModeFactory SchedulerModeFactory,
	evalTargetService IEvalTargetService,
) ExptSchedulerEvent {
	i := &ExptSchedulerImpl{
		Manager:                  manager,
		ExptRepo:                 exptRepo,
		ExptItemResultRepo:       exptItemResultRepo,
		ExptTurnResultRepo:       exptTurnResultRepo,
		EvaluatorRecordRepo:      evaluatorRecordRepo,
		ExptStatsRepo:            exptStatsRepo,
		ExptRunLogRepo:           exptRunLogRepo,
		Idem:                     Idem,
		Configer:                 configer,
		QuotaRepo:                quotaRepo,
		Mutex:                    mutex,
		Publisher:                publisher,
		AuditClient:              auditClient,
		Metric:                   metric,
		ResultSvc:                resultSvc,
		IDGen:                    idGen,
		evaluationSetItemService: evaluationSetItemService,
		schedulerModeFactory:     schedulerModeFactory,
		evalTargetService:        evalTargetService,
	}

	i.Endpoints = SchedulerChain(
		i.HandleEventErr,
		i.SysOps,
		i.HandleEventCheck,
		i.HandleEventLock,
		i.HandleEventEndpoint,
	)(func(_ context.Context, _ *entity.ExptScheduleEvent) error { return nil })

	return i
}

func (e *ExptSchedulerImpl) Schedule(ctx context.Context, event *entity.ExptScheduleEvent) error {
	ctx = ctxcache.Init(ctx)

	if err := e.Endpoints(ctx, event); err != nil {
		logs.CtxError(ctx, "[ExptScheduler] expt schedule fail, event: %v, err: %v", json.Jsonify(event), err)
		return err
	}

	return nil
}

type SchedulerEndPoint func(ctx context.Context, event *entity.ExptScheduleEvent) error

type SchedulerMiddleware func(next SchedulerEndPoint) SchedulerEndPoint

func SchedulerChain(mws ...SchedulerMiddleware) SchedulerMiddleware {
	return func(next SchedulerEndPoint) SchedulerEndPoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

func (e *ExptSchedulerImpl) SysOps(next SchedulerEndPoint) SchedulerEndPoint {
	return func(ctx context.Context, event *entity.ExptScheduleEvent) error {
		if e.Configer.GetSchedulerAbortCtrl(ctx).Abort(event.SpaceID, event.ExptID, event.Session.UserID, event.ExptType) {
			logs.CtxWarn(ctx, "[ExptEval] expt schedule aborted, event: %v", json.Jsonify(event))
			return nil
		}
		return next(ctx, event)
	}
}

func (e *ExptSchedulerImpl) HandleEventCheck(next SchedulerEndPoint) SchedulerEndPoint {
	return func(ctx context.Context, event *entity.ExptScheduleEvent) error {
		runLog, err := e.Manager.GetRunLog(ctx, event.ExptID, event.ExptRunID, event.SpaceID, event.Session)
		if err != nil {
			return err
		}

		if status := entity.ExptStatus(runLog.Status); entity.IsExptFinished(status) || entity.IsExptFinishing(status) {
			logs.CtxInfo(ctx, "ExptSchedulerConsumer consume finished expt run event, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
			return nil
		}

		interval := int64(e.Configer.GetExptExecConf(ctx, event.SpaceID).GetZombieIntervalSecond())
		if time.Now().Unix()-event.CreatedAt >= interval {
			return errno.NewExptZombieTimeoutErr(interval, event.ExptID, event.ExptRunID)
		}

		return next(ctx, event)
	}
}

func (e *ExptSchedulerImpl) makeExptRunExecLockKey(exptID, exptRunID int64) string {
	return fmt.Sprintf("expt_run_exec_lock:%d:%d", exptID, exptRunID)
}

func (e *ExptSchedulerImpl) HandleEventLock(next SchedulerEndPoint) SchedulerEndPoint {
	return func(ctx context.Context, event *entity.ExptScheduleEvent) error {
		key := e.makeExptRunExecLockKey(event.ExptID, event.ExptRunID)
		locked, ctx, cancel, err := e.Mutex.LockBackoffWithRenew(ctx, key, time.Second*5, time.Second*60*5)
		if err != nil {
			return err
		}

		logs.CtxInfo(ctx, "ExptSchedulerConsumer.HandleEventLock locked expt eval event: %v, key: %v", json.Jsonify(event), key)

		if !locked {
			logs.CtxWarn(ctx, "ExptSchedulerConsumer.HandleEventLock found locked expt eval event: %v. Abort event, err: %v", json.Jsonify(event), err)
			return nil
		}

		defer func() {
			cancel()
			if _, err := e.Mutex.Unlock(key); err != nil {
				logs.CtxWarn(ctx, "failed to unlock key: %v, err: %v", key, err)
			}
		}()

		return next(ctx, event)
	}
}

func (e *ExptSchedulerImpl) HandleEventEndpoint(next SchedulerEndPoint) SchedulerEndPoint {
	return func(ctx context.Context, event *entity.ExptScheduleEvent) error {
		err := e.schedule(ctx, event)
		if err != nil {
			return err
		}

		return next(ctx, event)
	}
}

func (e *ExptSchedulerImpl) HandleEventErr(next SchedulerEndPoint) SchedulerEndPoint {
	return func(ctx context.Context, event *entity.ExptScheduleEvent) error {
		nextErr := func(ctx context.Context, event *entity.ExptScheduleEvent) (err error) {
			defer goroutine.Recover(ctx, &err)
			return next(ctx, event)
		}(ctx, event)

		if nextErr == nil {
			logs.CtxInfo(ctx, "[ExptEval] handle event success, event: %v", json.Jsonify(event))
			return nil
		}

		logs.CtxError(ctx, "[ExptEval] HandleEventErr found error: %v, event: %v", nextErr, json.Jsonify(event))

		// 基础设施类错误（Redis抖动、MQ发送失败、context cancel等）：尝试用新ctx重新调度，不直接终止实验
		if isSchedulerInfraError(nextErr) {
			maxInfraRetry := 10
			if event.InfraErrorRetryTimes >= maxInfraRetry {
				logs.CtxError(ctx, "[ExptEval] infra error reschedule exhausted, expt_id: %v, expt_run_id: %v, retries: %d, err: %v",
					event.ExptID, event.ExptRunID, event.InfraErrorRetryTimes, nextErr)
				// 超过最大重试次数，走原有逻辑终止实验
			} else {
				logs.CtxWarn(ctx, "[ExptEval] infra error detected, attempting reschedule (%d/%d), expt_id: %v, expt_run_id: %v, err: %v",
					event.InfraErrorRetryTimes+1, maxInfraRetry, event.ExptID, event.ExptRunID, nextErr)

				// 复制context保留存储内容，仅断开cancel传播
				freshCtx := context.WithoutCancel(ctx)
				event.InfraErrorRetryTimes++

				if pubErr := e.Publisher.PublishExptScheduleEvent(freshCtx, event, gptr.Of(time.Second*5)); pubErr != nil {
					// 重新调度也失败，返回error让MQ框架重投递
					logs.CtxError(freshCtx, "[ExptEval] reschedule publish failed, rely on MQ retry, expt_id: %v, expt_run_id: %v, pub_err: %v",
						event.ExptID, event.ExptRunID, pubErr)
					return nextErr
				}

				logs.CtxInfo(freshCtx, "[ExptEval] reschedule success after infra error, expt_id: %v, expt_run_id: %v",
					event.ExptID, event.ExptRunID)
				return nil
			}
		}

		// 业务逻辑错误：终止实验
		completeCID := fmt.Sprintf("exptexec:onerr:%d", event.ExptRunID)

		if err := e.Manager.CompleteRun(ctx, event.ExptID, event.ExptRunID, event.SpaceID, event.Session, entity.WithCID(completeCID), entity.WithCompleteInterval(time.Second*2)); err != nil {
			return errorx.Wrapf(err, "terminate expt run fail, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
		}

		if err := e.Manager.CompleteExpt(ctx, event.ExptID, &event.ExptRunID, event.SpaceID, event.Session, entity.WithStatus(entity.ExptStatus_Failed),
			entity.WithStatusMessage(userVisibleErrMsg(nextErr)), entity.WithCID(completeCID), entity.WithCompleteInterval(time.Second*2)); err != nil {
			return errorx.Wrapf(err, "complete expt fail, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
		}

		return nil
	}
}

// userVisibleErrMsg 提取用户友好的错误描述：若为 errno.ErrImpl 则取其 Msg（如"实验已超过最大执行时长 …"），
// 否则回退到 err.Error()（历史英文 raw string）。
func userVisibleErrMsg(err error) string {
	if err == nil {
		return ""
	}
	if ei, ok := errno.ParseErrImpl(err); ok && ei != nil && len(ei.ErrMsg()) > 0 {
		return ei.ErrMsg()
	}
	return err.Error()
}

// isSchedulerInfraError 判断是否为基础设施类可重试错误
func isSchedulerInfraError(err error) bool {
	if err == nil {
		return false
	}
	// context cancel / deadline exceeded 通常由Redis锁续期失败触发
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	errMsg := err.Error()
	// MQ发送失败
	if strings.Contains(errMsg, "send batch message fail") {
		return true
	}
	// RPC层错误
	if strings.Contains(errMsg, "context canceled") || strings.Contains(errMsg, "context deadline exceeded") {
		return true
	}
	return false
}

func (e *ExptSchedulerImpl) schedule(ctx context.Context, event *entity.ExptScheduleEvent) error {
	if event.ExptRunMode == entity.EvaluationModeAppend {
		logs.CtxInfo(ctx, "[ExptEval] consume schedule event (Append), expt_id: %v, expt_run_id: %v, space_id: %v", event.ExptID, event.ExptRunID, event.SpaceID)
	}
	exptDetail, err := e.Manager.GetDetail(contexts.WithCtxWriteDB(ctx), event.ExptID, event.SpaceID, event.Session)
	if err != nil {
		return err
	}

	mode, err := e.schedulerModeFactory.NewSchedulerMode(event.ExptRunMode)
	if err != nil {
		return err
	}

	err = mode.ExptStart(ctx, event, exptDetail)
	if err != nil {
		return err
	}

	err = mode.ScheduleStart(ctx, event, exptDetail)
	if err != nil {
		return err
	}

	// Publish Processing lifecycle event on first transition
	if exptDetail.Status != entity.ExptStatus_Processing && e.Publisher != nil {
		currentExpt, getErr := e.ExptRepo.GetByID(ctx, event.ExptID, event.SpaceID)
		if getErr == nil && currentExpt.Status == entity.ExptStatus_Processing {
			lifecycleEvent := &entity.ExptLifecycleEvent{
				ExptID:     event.ExptID,
				ExptRunID:  &event.ExptRunID,
				SpaceID:    event.SpaceID,
				FromStatus: exptDetail.Status,
				ToStatus:   entity.ExptStatus_Processing,
				ExptType:   exptDetail.ExptType,
				SourceType: exptDetail.SourceType,
			}
			idempotentKey := fmt.Sprintf("expt_%d_%d_%d_%d", event.ExptID, event.ExptRunID, lifecycleEvent.FromStatus, lifecycleEvent.ToStatus)
			lifecycleEvent.IdempotentKey = idempotentKey
			if pubErr := e.Publisher.PublishExptLifecycleEvent(ctx, lifecycleEvent, gptr.Of(time.Second*3), idempotentKey); pubErr != nil {
				logs.CtxWarn(ctx, "[ExptEval] PublishExptLifecycleEvent(Processing) failed, expt_id: %v, err: %v", event.ExptID, pubErr)
			}
		}
	}

	toSubmit, incomplete, complete, err := mode.ScanEvalItems(ctx, event, exptDetail)
	if err != nil {
		return err
	}

	incomplete, sandboxTerminated, err := e.sweepTerminatedSandboxItems(ctx, event, incomplete, exptDetail)
	if err != nil {
		return err
	}

	incomplete, zombies, err := e.handleZombies(ctx, event, incomplete, exptDetail)
	if err != nil {
		return err
	}

	complete = append(complete, zombies...)
	complete = append(complete, sandboxTerminated...)
	logs.CtxInfo(ctx, "expt scheduler scan item, to_submit: %v, incomplete: %v, complete: %v",
		entity.ExptEvalItems(toSubmit).GetItemIDs(), entity.ExptEvalItems(incomplete).GetItemIDs(), entity.ExptEvalItems(complete).GetItemIDs())

	if err = e.recordEvalItemRunLogs(ctx, event, complete, mode, exptDetail); err != nil {
		return err
	}

	if err = e.handleToSubmits(ctx, event, toSubmit); err != nil {
		return err
	}

	nextTick, err := mode.ExptEnd(ctx, event, exptDetail, len(toSubmit), len(incomplete))
	if err != nil {
		return err
	}

	if !nextTick {
		if event.ExptRunMode == entity.EvaluationModeAppend {
			logs.CtxInfo(ctx, "[ExptEval] online expt daemon ended, expt_id: %v, expt_run_id: %v, space_id: %v", event.ExptID, event.ExptRunID, event.SpaceID)
		}
		return nil
	}

	logs.CtxInfo(ctx, "[ExptEval] expt daemon with next tick, expt_id: %v, expt_run_id: %v, space_id: %v, to_submit: %v, incomplete: %v", event.ExptID, event.ExptRunID, event.SpaceID, entity.ExptEvalItems(toSubmit).GetItemIDs(), entity.ExptEvalItems(incomplete).GetItemIDs())

	select {
	case <-time.After(time.Second * 3):
	case <-ctx.Done():
		return ctx.Err()
	}
	event.InfraErrorRetryTimes = 0
	return mode.NextTick(ctx, event, nextTick)
}

func (e *ExptSchedulerImpl) recordEvalItemRunLogs(ctx context.Context, event *entity.ExptScheduleEvent, completeItems []*entity.ExptEvalItem, mode entity.ExptSchedulerMode, expt *entity.Experiment) error {
	time.Sleep(time.Millisecond * 1000) // avoid master-slave delay caused by asynchronous and other factors
	for _, item := range completeItems {
		if item.State != entity.ItemRunState_Fail && item.State != entity.ItemRunState_Success {
			return fmt.Errorf("recordEvalItemRunLogs found invalid item run state: %v", item.State)
		}
		var turnEvaluatorRefs []*entity.ExptTurnEvaluatorResultRef
		if err := backoff.RetryFiveMin(ctx, func() error {
			var err error
			turnEvaluatorRefs, err = e.ResultSvc.RecordItemRunLogs(ctx, event.ExptID, event.ExptRunID, item.ItemID, event.SpaceID, expt)
			return err
		}); err != nil {
			return err
		}
		time.Sleep(time.Millisecond * 50)
		logs.CtxInfo(ctx, "[ExptEval] recordEvalItemRunLogs publish result, expt_id: %v, event: %v, item_id: %v, turn_evaluator_refs: %v", event.ExptID, event, item.ItemID, json.Jsonify(turnEvaluatorRefs))
		err := mode.PublishResult(ctx, turnEvaluatorRefs, event)
		if err != nil {
			logs.CtxError(ctx, "publish online result fail, err: %v", err)
		}
	}
	if len(completeItems) == 0 {
		return nil
	}
	err := e.ResultSvc.UpsertExptTurnResultFilter(ctx, event.SpaceID, event.ExptID, gslice.Map(completeItems, func(item *entity.ExptEvalItem) int64 {
		return item.ItemID
	}))
	if err != nil {
		logs.CtxError(ctx, "UpsertExptTurnResultFilter fail, err: %v", err)
	}
	err = e.Publisher.PublishExptTurnResultFilterEvent(ctx, &entity.ExptTurnResultFilterEvent{
		ExperimentID: event.ExptID,
		SpaceID:      event.SpaceID,
		ItemID: gslice.Map(completeItems, func(item *entity.ExptEvalItem) int64 {
			return item.ItemID
		}),
		RetryTimes: ptr.Of(int32(0)),
		FilterType: ptr.Of(entity.UpsertExptTurnResultFilterTypeCheck),
	}, ptr.Of(10*time.Second))
	if err != nil {
		return err
	}

	logs.CtxInfo(ctx, "ExptSchedulerImpl recordEvalItemRunLogs UpsertExptTurnResultFilter done, expt_id: %v, item_ids: %v", event.ExptID, gslice.Map(completeItems, func(item *entity.ExptEvalItem) int64 {
		return item.ItemID
	}))
	return nil
}

func (e *ExptSchedulerImpl) handleToSubmits(ctx context.Context, event *entity.ExptScheduleEvent, toSubmits []*entity.ExptEvalItem) error {
	if len(toSubmits) == 0 {
		return nil
	}

	now := time.Now().Unix()
	itemIDs := make([]int64, 0, len(toSubmits))
	itemEvalEvents := make([]*entity.ExptItemEvalEvent, 0, len(toSubmits))
	for _, ts := range toSubmits {
		if entity.IsItemRunFinished(ts.State) {
			continue
		}
		itemIDs = append(itemIDs, ts.ItemID)
		itemEvalEvents = append(itemEvalEvents, &entity.ExptItemEvalEvent{
			SpaceID:       event.SpaceID,
			ExptID:        event.ExptID,
			ExptRunID:     event.ExptRunID,
			ExptRunMode:   event.ExptRunMode,
			EvalSetItemID: ts.ItemID,
			CreateAt:      now,
			MaxRetryTimes: event.ItemRetryTimes,
			Ext:           event.Ext,
			Session:       event.Session,
		})
	}

	logs.CtxInfo(ctx, "submit item eval events: %v", json.Jsonify(itemEvalEvents))

	interval := e.Configer.GetExptExecConf(ctx, event.SpaceID).GetExptItemEvalConf().GetInterval()
	if err := e.Publisher.BatchPublishExptRecordEvalEvent(ctx, itemEvalEvents, gptr.Of(interval)); err != nil {
		return err
	}

	defer e.Metric.EmitItemExecEval(event.SpaceID, int64(event.ExptRunMode), len(toSubmits))

	if err := e.ExptItemResultRepo.UpdateItemRunLog(ctx, event.ExptID, event.ExptRunID, itemIDs, map[string]any{"status": int32(entity.ItemRunState_Processing)},
		event.SpaceID); err != nil {
		return err
	}

	if err := e.ExptItemResultRepo.UpdateItemsResult(ctx, event.SpaceID, event.ExptID, itemIDs, map[string]any{"status": int32(entity.ItemRunState_Processing)}); err != nil {
		return err
	}

	err := e.ResultSvc.UpsertExptTurnResultFilter(ctx, event.SpaceID, event.ExptID, itemIDs)
	if err != nil {
		logs.CtxError(ctx, "ExptSubmitExec.ExptStart UpsertExptTurnResultFilter fail, expt_id: %v, err: %v", event.ExptID, err)
	}
	logs.CtxInfo(ctx, "ExptSchedulerImpl handleToSubmits UpsertExptTurnResultFilter success, expt_id: %v", event.ExptID)

	if err := e.ExptTurnResultRepo.UpdateTurnResultsWithItemIDs(ctx, event.ExptID, itemIDs, event.SpaceID, map[string]any{"status": int32(entity.TurnRunState_Processing)}); err != nil {
		return err
	}

	itemResults, err := e.ExptItemResultRepo.BatchGet(ctx, event.SpaceID, event.ExptID, itemIDs)
	if err != nil {
		return err
	}

	if err := e.ExptStatsRepo.ArithOperateCount(ctx, event.ExptID, event.SpaceID, &entity.StatsCntArithOp{
		OpStatusCnt: map[entity.ItemRunState]int{
			entity.ItemRunState_Processing: len(itemResults),
			entity.ItemRunState_Queueing:   0 - len(itemResults),
		},
	}); err != nil {
		return err
	}

	return nil
}

func (e *ExptSchedulerImpl) handleZombies(ctx context.Context, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem, expt *entity.Experiment) (alives, zombies []*entity.ExptEvalItem, err error) {
	asyncExec := false
	if expt != nil {
		asyncExec = expt.AsyncExec()
	}
	zombieSecond := e.Configer.GetConsumerConf(ctx).GetExptExecConf(event.SpaceID).GetExptItemEvalConf().GetItemZombieSecond(asyncExec)
	for _, item := range items {
		if item.State == entity.ItemRunState_Processing && item.UpdatedAt != nil && !gptr.Indirect(item.UpdatedAt).IsZero() {
			if time.Since(gptr.Indirect(item.UpdatedAt)).Seconds() > float64(zombieSecond) {
				zombies = append(zombies, item.SetState(entity.ItemRunState_Fail))
			} else {
				alives = append(alives, item)
			}
		}
	}

	zombieItemIDs := gslice.Transform(zombies, func(e *entity.ExptEvalItem, _ int) int64 { return e.ItemID })

	if len(zombies) == 0 {
		return alives, zombies, nil
	}

	logs.CtxWarn(ctx, "[ExptEval] found zombie items, set failure state, expt_id: %v, expt_run_id: %v, item_ids: %v, zombie_second: %v", event.ExptID, event.ExptRunID, zombieItemIDs, zombieSecond)

	if err := e.terminateZombieEvaluatorRecords(ctx, event, zombieItemIDs); err != nil {
		logs.CtxError(ctx, "[ExptEval] terminate async evaluator records for zombie items fail, expt_id: %v, expt_run_id: %v, item_ids: %v, err: %v", event.ExptID, event.ExptRunID, zombieItemIDs, err)
	}

	if err := e.terminateZombieEvalTargetRecords(ctx, event, zombieItemIDs); err != nil {
		logs.CtxError(ctx, "[ExptEval] terminate async eval target records for zombie items fail, expt_id: %v, expt_run_id: %v, item_ids: %v, err: %v", event.ExptID, event.ExptRunID, zombieItemIDs, err)
	}

	// 把行超时错误写入 err_msg，供 API 层（ItemSystemInfo.Error）暴露给用户
	zombieErrBytes := []byte(errno.SerializeErr(errno.NewItemZombieTimeoutErr(zombieSecond, asyncExec)))

	if err := e.ExptItemResultRepo.UpdateItemRunLog(ctx, event.ExptID, event.ExptRunID, zombieItemIDs, map[string]any{
		"status":       int32(entity.ItemRunState_Fail),
		"result_state": int32(entity.ExptItemResultStateLogged),
		"err_msg":      zombieErrBytes,
	}, event.SpaceID); err != nil {
		return nil, nil, err
	}

	// 主表 expt_item_result 也带上 err_msg，供 MGetExperimentResult 构造 ItemSystemInfo 时读取
	if err := e.ExptItemResultRepo.UpdateItemsResult(ctx, event.SpaceID, event.ExptID, zombieItemIDs, map[string]any{
		"status":  int32(entity.ItemRunState_Fail),
		"err_msg": zombieErrBytes,
	}); err != nil {
		logs.CtxError(ctx, "[ExptEval] update zombie items main table err_msg fail, expt_id: %v, expt_run_id: %v, item_ids: %v, err: %v", event.ExptID, event.ExptRunID, zombieItemIDs, err)
	}

	if err := e.ExptTurnResultRepo.CreateOrUpdateItemsTurnRunLogStatus(ctx, event.SpaceID, event.ExptID, event.ExptRunID, zombieItemIDs, entity.TurnRunState_Fail); err != nil {
		return nil, nil, err
	}

	// 不清 run_log 的 target_result_id / evaluator_result_ids：
	// zombie 场景是「终态失败」，需要保留已入库的 record id，
	// 让 /results/batch_get 能返回 eval_target_record.id、evaluator_record.id 供用户查详情。
	// 「清 id」的语义只属于「重跑起点」（见 clearExptTurnRunLogResultRefsOnItems 其他调用点：
	// FailRetry / rerunItems / 手动重跑），失败落地不应触发。

	time.Sleep(time.Millisecond * 1500)

	return alives, zombies, nil
}

// terminateZombieEvaluatorRecords 将僵尸 item 关联的、仍处于 AsyncInvoking 状态的 EvaluatorRecord 置为失败。
// 通过 turn run log 拿到 record id 列表，再按主键过滤与更新，避免在无二级索引的 evaluator_record 表上做条件 UPDATE。
func (e *ExptSchedulerImpl) terminateZombieEvaluatorRecords(ctx context.Context, event *entity.ExptScheduleEvent, zombieItemIDs []int64) error {
	if len(zombieItemIDs) == 0 {
		return nil
	}

	turnRunLogs, err := e.ExptTurnResultRepo.MGetItemTurnRunLogs(ctx, event.ExptID, event.ExptRunID, zombieItemIDs, event.SpaceID)
	if err != nil {
		return err
	}

	recordIDSet := make(map[int64]struct{})
	for _, rl := range turnRunLogs {
		if rl == nil || rl.EvaluatorResultIds == nil {
			continue
		}
		// ★ 支持新旧两种格式
		if rl.EvaluatorResultIds.IsNewFormat() {
			for _, r := range rl.EvaluatorResultIds.Registered {
				if r != nil && r.RecordID > 0 {
					recordIDSet[r.RecordID] = struct{}{}
				}
			}
			for _, r := range rl.EvaluatorResultIds.Inline {
				if r != nil && r.RecordID > 0 {
					recordIDSet[r.RecordID] = struct{}{}
				}
			}
		} else {
			for _, resID := range rl.EvaluatorResultIds.EvalVerIDToResID {
				if resID > 0 {
					recordIDSet[resID] = struct{}{}
				}
			}
		}
	}
	if len(recordIDSet) == 0 {
		return nil
	}

	recordIDs := make([]int64, 0, len(recordIDSet))
	for id := range recordIDSet {
		recordIDs = append(recordIDs, id)
	}

	records, err := e.EvaluatorRecordRepo.BatchGetEvaluatorRecord(ctx, recordIDs, false, false)
	if err != nil {
		return err
	}

	failOutput := &entity.EvaluatorOutputData{
		EvaluatorRunError: &entity.EvaluatorRunError{
			Code:    int32(errno.AsyncEvaluatorZombieTimeoutCode),
			Message: "async evaluator terminated: experiment item exceeded zombie timeout",
		},
	}

	var firstErr error
	for _, r := range records {
		if r == nil || r.Status != entity.EvaluatorRunStatusAsyncInvoking {
			continue
		}
		if err := e.EvaluatorRecordRepo.UpdateEvaluatorRecordResult(ctx, r.ID, entity.EvaluatorRunStatusFail, failOutput); err != nil {
			logs.CtxError(ctx, "[ExptEval] update zombie evaluator record fail, record_id: %v, err: %v", r.ID, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// sweepTerminatedSandboxItems 主动巡检 SandboxAgent 异步评测对象的沙箱状态：
// 如果沙箱 execute 已进入 Failed/Canceled 但结果还没通过 ReportEvalTargetInvokeResult 回调上报，
// 就把对应的 item / turn run log / 主表状态直接置为 Fail，让后续 HandleEventErr 走既有的
// 重试 / 终止决策，而不是等 3h zombie 兜底。
//
// 非 SandboxAgent 类型 / adapter 未接入（开源 stub）时静默 no-op，返回原始 items。
func (e *ExptSchedulerImpl) sweepTerminatedSandboxItems(ctx context.Context, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem, expt *entity.Experiment) (alives, terminated []*entity.ExptEvalItem, err error) {
	// [SweepDebug] TEMP: 上线定位为何 sweep 没触发，验证完这批日志可整体删除
	logs.CtxInfo(ctx, "[SweepDebug] enter, expt_id=%v, expt_run_id=%v, items=%d, evalTargetService_nil=%v, expt_nil=%v",
		event.ExptID, event.ExptRunID, len(items), e.evalTargetService == nil, expt == nil)

	if e.evalTargetService == nil || expt == nil {
		logs.CtxInfo(ctx, "[SweepDebug] short-circuit: evalTargetService or expt is nil, expt_id=%v", event.ExptID)
		return items, nil, nil
	}
	// 只对确实用 SandboxAgent 的实验做 sweep，避免每个 tick 都发无谓 RPC。
	if expt.Target == nil || expt.Target.EvalTargetVersion == nil ||
		expt.Target.EvalTargetVersion.EvalTargetType != entity.EvalTargetTypeSandboxAgent {
		targetNil := expt.Target == nil
		versionNil := expt.Target != nil && expt.Target.EvalTargetVersion == nil
		var typ entity.EvalTargetType
		if expt.Target != nil && expt.Target.EvalTargetVersion != nil {
			typ = expt.Target.EvalTargetVersion.EvalTargetType
		}
		logs.CtxInfo(ctx, "[SweepDebug] short-circuit: not sandbox target, expt_id=%v, target_nil=%v, version_nil=%v, target_type=%v (want %v)",
			event.ExptID, targetNil, versionNil, typ, entity.EvalTargetTypeSandboxAgent)
		return items, nil, nil
	}

	processingItemIDs := make([]int64, 0, len(items))
	for _, item := range items {
		if item == nil || item.State != entity.ItemRunState_Processing {
			continue
		}
		processingItemIDs = append(processingItemIDs, item.ItemID)
	}
	if len(processingItemIDs) == 0 {
		logs.CtxInfo(ctx, "[SweepDebug] short-circuit: no Processing items, expt_id=%v, input_items=%d", event.ExptID, len(items))
		return items, nil, nil
	}
	logs.CtxInfo(ctx, "[SweepDebug] processing items collected, expt_id=%v, count=%d, ids=%v", event.ExptID, len(processingItemIDs), processingItemIDs)

	turnRunLogs, err := e.ExptTurnResultRepo.MGetItemTurnRunLogs(ctx, event.ExptID, event.ExptRunID, processingItemIDs, event.SpaceID)
	if err != nil {
		logs.CtxWarn(ctx, "[SweepDebug] MGetItemTurnRunLogs err, expt_id=%v, err=%v", event.ExptID, err)
		return items, nil, err
	}
	logs.CtxInfo(ctx, "[SweepDebug] turn run logs fetched, expt_id=%v, count=%d", event.ExptID, len(turnRunLogs))

	// recordID -> itemID(s)，一个 record 可能只对应一个 turn，但为稳妥仍走 slice
	recordIDToItemIDs := make(map[int64][]int64)
	recordIDs := make([]int64, 0)
	skippedZero := 0
	for _, rl := range turnRunLogs {
		if rl == nil || rl.TargetResultID <= 0 {
			skippedZero++
			continue
		}
		if _, exists := recordIDToItemIDs[rl.TargetResultID]; !exists {
			recordIDs = append(recordIDs, rl.TargetResultID)
		}
		recordIDToItemIDs[rl.TargetResultID] = append(recordIDToItemIDs[rl.TargetResultID], rl.ItemID)
	}
	if len(recordIDs) == 0 {
		logs.CtxInfo(ctx, "[SweepDebug] short-circuit: no recordIDs (target_result_id<=0), expt_id=%v, turn_run_logs=%d, skipped_zero=%d",
			event.ExptID, len(turnRunLogs), skippedZero)
		return items, nil, nil
	}
	logs.CtxInfo(ctx, "[SweepDebug] recordIDs to check, expt_id=%v, count=%d, ids=%v", event.ExptID, len(recordIDs), recordIDs)

	terminatedRecordIDs, statusMap := e.evalTargetService.CheckSandboxTerminated(ctx, event.SpaceID, recordIDs)
	logs.CtxInfo(ctx, "[SweepDebug] CheckSandboxTerminated returned, expt_id=%v, terminated=%v, status_map=%v",
		event.ExptID, terminatedRecordIDs, statusMap)
	if len(terminatedRecordIDs) == 0 {
		return items, nil, nil
	}

	terminatedItemIDSet := make(map[int64]struct{})
	// 用第一个命中的 record 状态作为整体 err_msg 依据（同一 item 通常只对应一个 record）。
	var firstStatus string
	for _, rid := range terminatedRecordIDs {
		if firstStatus == "" {
			firstStatus = statusMap[rid]
		}
		for _, itemID := range recordIDToItemIDs[rid] {
			terminatedItemIDSet[itemID] = struct{}{}
		}
	}
	if firstStatus == "" {
		firstStatus = "Terminated"
	}

	alives = make([]*entity.ExptEvalItem, 0, len(items))
	terminated = make([]*entity.ExptEvalItem, 0, len(terminatedItemIDSet))
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, hit := terminatedItemIDSet[item.ItemID]; hit && item.State == entity.ItemRunState_Processing {
			terminated = append(terminated, item.SetState(entity.ItemRunState_Fail))
		} else {
			alives = append(alives, item)
		}
	}

	if len(terminated) == 0 {
		return alives, terminated, nil
	}

	terminatedItemIDs := gslice.Transform(terminated, func(it *entity.ExptEvalItem, _ int) int64 { return it.ItemID })
	logs.CtxWarn(ctx, "[ExptEval] sandbox execute terminated before report, mark items failed, expt_id: %v, expt_run_id: %v, item_ids: %v, sandbox_status: %v",
		event.ExptID, event.ExptRunID, terminatedItemIDs, firstStatus)

	// 沙箱已经是终态，Destroy 不需要再发 EndCmd，zombieTimeout=false。
	e.evalTargetService.TerminateAsyncRecordsAndDestroySandbox(
		ctx,
		event.SpaceID,
		terminatedRecordIDs,
		int32(errno.SandboxTerminatedBeforeReportCode),
		fmt.Sprintf("sandbox execute reached terminal state (%s) before result was reported", firstStatus),
		false,
	)

	// 与 handleZombies 一致的写库形状，保证 UI (MGetExperimentResult) 拿到一致的 err_msg。
	errBytes := []byte(errno.SerializeErr(errno.NewSandboxTerminatedBeforeReportErr(firstStatus)))

	if err := e.ExptItemResultRepo.UpdateItemRunLog(ctx, event.ExptID, event.ExptRunID, terminatedItemIDs, map[string]any{
		"status":       int32(entity.ItemRunState_Fail),
		"result_state": int32(entity.ExptItemResultStateLogged),
		"err_msg":      errBytes,
	}, event.SpaceID); err != nil {
		return nil, nil, err
	}
	if err := e.ExptItemResultRepo.UpdateItemsResult(ctx, event.SpaceID, event.ExptID, terminatedItemIDs, map[string]any{
		"status":  int32(entity.ItemRunState_Fail),
		"err_msg": errBytes,
	}); err != nil {
		logs.CtxError(ctx, "[ExptEval] update sandbox-terminated items main table err_msg fail, expt_id: %v, expt_run_id: %v, item_ids: %v, err: %v", event.ExptID, event.ExptRunID, terminatedItemIDs, err)
	}
	if err := e.ExptTurnResultRepo.CreateOrUpdateItemsTurnRunLogStatus(ctx, event.SpaceID, event.ExptID, event.ExptRunID, terminatedItemIDs, entity.TurnRunState_Fail); err != nil {
		return nil, nil, err
	}

	return alives, terminated, nil
}

// terminateZombieEvalTargetRecords 将僵尸 item 关联的 EvalTargetRecord（仅 SandboxAgent 类型且仍 AsyncInvoking）置为 Fail，
// 并 best-effort 销毁对应的沙箱 execute。
func (e *ExptSchedulerImpl) terminateZombieEvalTargetRecords(ctx context.Context, event *entity.ExptScheduleEvent, zombieItemIDs []int64) error {
	if len(zombieItemIDs) == 0 || e.evalTargetService == nil {
		return nil
	}

	turnRunLogs, err := e.ExptTurnResultRepo.MGetItemTurnRunLogs(ctx, event.ExptID, event.ExptRunID, zombieItemIDs, event.SpaceID)
	if err != nil {
		return err
	}

	recordIDSet := make(map[int64]struct{})
	for _, rl := range turnRunLogs {
		if rl == nil || rl.TargetResultID <= 0 {
			continue
		}
		recordIDSet[rl.TargetResultID] = struct{}{}
	}
	if len(recordIDSet) == 0 {
		return nil
	}

	recordIDs := make([]int64, 0, len(recordIDSet))
	for id := range recordIDSet {
		recordIDs = append(recordIDs, id)
	}

	e.evalTargetService.TerminateAsyncRecordsAndDestroySandbox(
		ctx,
		event.SpaceID,
		recordIDs,
		int32(errno.AsyncEvalTargetZombieTimeoutCode),
		"async eval target terminated: experiment item exceeded zombie timeout",
		true,
	)
	return nil
}
