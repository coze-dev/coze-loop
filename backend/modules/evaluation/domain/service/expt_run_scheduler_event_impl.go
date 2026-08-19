// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	// itemCompletePublisher 发送单行评测完成(item-complete)事件供下游离线分析。
	// 发送点由链路A(CompleteItemRun)后移至此(recordEvalItemRunLogs, 读侧就绪后), 消除下游反查竞态。
	// 允许为 nil: 开源侧 ProvideNilItemCompletePublisher 返回 nil, 循环内以非空守卫跳过发送。
	itemCompletePublisher component.IItemCompletePublisher
	// exptItemRefRepo 取每个 item 的 per-item 归属集/版本(expt_item_ref), 供 item-complete 事件组装。
	// 多评测集下各 item 可属不同集/版本, 不能用 ExptEvalItem.EvalSetVersionID(主集硬编码)。
	exptItemRefRepo repo.IExptItemRefRepo
	// sandboxAgentMetrics 复用沙箱 agent 评测对象既有打点体系。
	// sweep 命中沙箱提前终态时，对每个受影响 item 补一次 EmitInvokeFinished，
	// 让终态命中路径与正常回调路径在同一 dashboard 上可比。
	// 允许为 nil：开源部署 / 未接入 metrics 时静默 no-op。
	sandboxAgentMetrics metrics.SandboxAgentMetrics
	// sandboxAgentNotifier 每 1h 进度快照飞书通知; 允许为 nil (未接入通知)。
	sandboxAgentNotifier ISandboxAgentNotifier
	// centralGuard 中心调度额度闸，用于 zombie / 沙箱提前终态这两条**不经 consumer** 的
	// 终态路径释放额度。consumer 侧的释放在 HandleCentralReservation 出口统一处理。
	//
	// 为什么这两条必须单独接：它们由 daemon 直接把 item 判为 Fail 落库，consumer 那条
	// 消息可能已经卡死或永不返回 —— 不在这里释放，这些 item 的额度会永久泄漏。
	// 允许为 nil（开源部署注入 noop）。
	centralGuard component.ICentralReservationGuard
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
	itemCompletePublisher component.IItemCompletePublisher,
	exptItemRefRepo repo.IExptItemRefRepo,
	sandboxAgentMetrics metrics.SandboxAgentMetrics,
	// centralGuard 放在 variadic 之前（Go 只要求 variadic 最后）。不做 setter 注入：
	// setter 会让 wire 构造出实例但无人调用、字段恒 nil，而 nil 在此被解释为"跳过释放"，
	// 等于静默恢复额度泄漏。
	centralGuard component.ICentralReservationGuard,
	sandboxAgentNotifier ...ISandboxAgentNotifier, // variadic 兼容旧单测
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
		centralGuard:             centralGuard,
		evalTargetService:        evalTargetService,
		itemCompletePublisher:    itemCompletePublisher,
		exptItemRefRepo:          exptItemRefRepo,
		sandboxAgentMetrics:      sandboxAgentMetrics,
	}
	if len(sandboxAgentNotifier) > 0 {
		i.sandboxAgentNotifier = sandboxAgentNotifier[0]
	}

	i.Endpoints = SchedulerChain(
		i.HandleEventErr,
		i.SysOps,
		i.HandleEventCheck,
		i.HandleEventLock,
		i.HandleEventEndpoint,
		i.SandboxAgentHourlyNotify,
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

// SandboxAgentHourlyNotify 中间件: 沙箱 agent 实验每 1h 一张进度快照飞书卡。
// 主流程 schedule 出错时不发通知,避免叠加噪音; Notifier 内部做 sandbox agent + Enable + 1h 闸门判定,
// 非目标 case 静默。发送失败仅 log,不影响调度链。
func (e *ExptSchedulerImpl) SandboxAgentHourlyNotify(next SchedulerEndPoint) SchedulerEndPoint {
	return func(ctx context.Context, event *entity.ExptScheduleEvent) error {
		if err := next(ctx, event); err != nil {
			return err
		}
		if e.sandboxAgentNotifier == nil {
			return nil
		}
		// GetDetail 会填 Target/NotificationConf/Name; GetByID 不填 Target,
		// isSandboxAgentExperiment 会误判为 false。
		exptDetail, err := e.Manager.GetDetail(ctx, event.ExptID, event.SpaceID, event.Session)
		if err != nil {
			logs.CtxWarn(ctx, "[SandboxAgentNotify] GetDetail fail, expt_id=%v, err=%v", event.ExptID, err)
			return nil
		}
		if err := e.sandboxAgentNotifier.NotifyProgressIfDue(ctx, exptDetail); err != nil {
			logs.CtxWarn(ctx, "[SandboxAgentNotify] progress notify err, expt_id=%v, err=%v", event.ExptID, err)
		}
		return nil
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

	// ★ 中心化调度防双驱动：enforce 实验的新 item 派发权归中心调度器独有。
	//
	// 旧 per-experiment tick 在此**丢弃 toSubmit**，但保留其余全部职责 ——
	// 完成 item 归档（上面的 recordEvalItemRunLogs 已执行）、zombie/sandbox terminated 处理、
	// run/实验终态收口、NextTick 续跳。若这里直接 return，实验会因为没人收口而永远停在 Processing。
	//
	// 为什么必须丢弃而不是"让它也派"：旧链路按实验自己的配置并发补 item，既不看全局优先级
	// 也不经额度账本，两个驱动并存会直接超发。
	dispatchByCentral := entity.IsCentralDispatch(exptDetail.ExptDispatchMode)
	if dispatchByCentral {
		if len(toSubmit) > 0 {
			logs.CtxInfo(ctx, "[CentralDispatch] legacy tick suppressed %d to-submit item(s) for enforce experiment, expt_id: %v, expt_run_id: %v",
				len(toSubmit), event.ExptID, event.ExptRunID)
		}
		toSubmit = nil
	} else if err = e.handleToSubmits(ctx, event, toSubmit); err != nil {
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

	// 循环外批量补齐 item-complete 事件组装所需的 per-item 归属集/版本/ItemKey(供循环内 sendItemComplete 用)。
	// 仅当接了 publisher(商业化真实 producer; 开源为 nil) 且有 item 时才做, 避免开源侧无谓 IO。
	var itemMeta map[int64]*entity.EvaluationSetItem
	var itemVer map[int64]int64
	if e.itemCompletePublisher != nil && len(completeItems) > 0 {
		itemMeta, itemVer = e.resolveItemCompleteMeta(ctx, event, completeItems, expt)
	}

	for _, item := range completeItems {
		if item.State != entity.ItemRunState_Fail && item.State != entity.ItemRunState_Success {
			return fmt.Errorf("recordEvalItemRunLogs found invalid item run state: %v", item.State)
		}

		// item-complete(success) 发送点: 每个 item 一进来先发, 仅发成功行(fail/zombie 不发, 下游只消费成功行)。
		// 发送作为旁路, 只读不写 result_state, 发失败在 sendItemComplete 内 CtxError + return(仅本 item), 不阻断落库/后续。
		// 是否真正投递由 producer 依空间开关(item_complete_space_config) + 评测对象 enable_analysis 判定, 此处不重复判。
		// 不追求消竞态: 下游侧 defer 投递已覆盖读侧就绪窗口, 本处只保障"成功行必发一次 MQ"。
		if e.itemCompletePublisher != nil && item.State == entity.ItemRunState_Success {
			e.sendItemComplete(ctx, event, expt, item, itemMeta[item.ItemID], itemVer[item.ItemID])
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

// resolveItemCompleteMeta 循环外批量补齐 item-complete 事件组装所需元数据。
// 返回 itemID→EvaluationSetItem(提供 ItemKey/SpaceID/EvaluationSetID) 与 itemID→per-item 归属集版本。
// per-item 版本必须来自 expt_item_ref(多集非主集也正确), 不能用 ExptEvalItem.EvalSetVersionID(主集硬编码)。
// 任一步失败只 CtxWarn 不阻断: 组装侧对缺失 item 跳过发送, 不影响读侧写入与在线结果发布。
func (e *ExptSchedulerImpl) resolveItemCompleteMeta(ctx context.Context, event *entity.ExptScheduleEvent, completeItems []*entity.ExptEvalItem, expt *entity.Experiment) (map[int64]*entity.EvaluationSetItem, map[int64]int64) {
	itemMeta := make(map[int64]*entity.EvaluationSetItem, len(completeItems))
	itemVer := make(map[int64]int64, len(completeItems))

	itemIDs := gslice.Map(completeItems, func(item *entity.ExptEvalItem) int64 { return item.ItemID })

	// 按归属 (EvalSetID, EvalSetVersionID, EvalSetSourceSpaceID) 分组: 多集下各 item 可属不同集/版本,
	// BatchGetEvaluationSetItems 一次只接受单集+单版本, 故须分组各查一次。
	type setGroup struct {
		evalSetID        int64
		evalSetVersionID int64
		sourceSpaceID    int64
		itemIDs          []int64
	}
	groups := make(map[string]*setGroup)
	addToGroup := func(itemID, evalSetID, evalSetVersionID, sourceSpaceID int64) {
		key := fmt.Sprintf("%d:%d:%d", evalSetID, evalSetVersionID, sourceSpaceID)
		g := groups[key]
		if g == nil {
			g = &setGroup{evalSetID: evalSetID, evalSetVersionID: evalSetVersionID, sourceSpaceID: sourceSpaceID}
			groups[key] = g
		}
		g.itemIDs = append(g.itemIDs, itemID)
		itemVer[itemID] = evalSetVersionID // per-item 归属集版本, 直接来自 ref/主集(单集)
	}

	if expt.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig && e.exptItemRefRepo != nil {
		refs, err := e.exptItemRefRepo.MGetByExptIDAndItemIDs(ctx, event.SpaceID, event.ExptID, itemIDs)
		if err != nil {
			logs.CtxWarn(ctx, "[ExptEval] item complete resolve refs fail, skip publish batch, expt_id: %v, err: %v", event.ExptID, err)
			return itemMeta, itemVer
		}
		refByItem := make(map[int64]*entity.ExptItemRef, len(refs))
		for _, ref := range refs {
			if ref != nil {
				refByItem[ref.ItemID] = ref
			}
		}
		for _, itemID := range itemIDs {
			ref := refByItem[itemID]
			if ref == nil || ref.EvalSetID <= 0 {
				logs.CtxWarn(ctx, "[ExptEval] item complete ref missing, skip item, expt_id: %v, item_id: %v", event.ExptID, itemID)
				continue
			}
			addToGroup(itemID, ref.EvalSetID, ref.EvalSetVersionID, ref.EvalSetSourceSpaceID)
		}
	} else {
		// 单评测集/老实验: 全部 item 归属主集, 版本用主集版本, 来源空间取实验冻结列。
		if expt.EvalSet == nil || expt.EvalSet.EvaluationSetVersion == nil {
			logs.CtxWarn(ctx, "[ExptEval] item complete primary eval set missing, skip publish batch, expt_id: %v", event.ExptID)
			return itemMeta, itemVer
		}
		evalSetID := expt.EvalSet.ID
		evalSetVersionID := expt.EvalSet.EvaluationSetVersion.ID
		for _, itemID := range itemIDs {
			addToGroup(itemID, evalSetID, evalSetVersionID, expt.EvalSetSpaceID)
		}
	}

	for _, g := range groups {
		items, err := e.evaluationSetItemService.BatchGetEvaluationSetItems(ctx, &entity.BatchGetEvaluationSetItemsParam{
			SpaceID:         resolveLoadSpaceID(event.SpaceID, g.sourceSpaceID),
			EvaluationSetID: g.evalSetID,
			VersionID:       resolveSetReadVersionID(g.evalSetID, g.evalSetVersionID),
			ItemIDs:         g.itemIDs,
		})
		if err != nil {
			logs.CtxWarn(ctx, "[ExptEval] item complete batch get items fail, skip group, expt_id: %v, eval_set_id: %v, err: %v", event.ExptID, g.evalSetID, err)
			continue
		}
		for _, it := range items {
			if it != nil {
				itemMeta[it.ItemID] = it
			}
		}
	}

	return itemMeta, itemVer
}

// sendItemComplete 组装并发送单行 item-complete(success) 事件。发送失败打 CtxError 告警但不阻断(靠下游 defer/幂等兜底)。
func (e *ExptSchedulerImpl) sendItemComplete(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, item *entity.ExptEvalItem, evalSetItem *entity.EvaluationSetItem, evalSetVersionID int64) {
	if evalSetItem == nil {
		logs.CtxWarn(ctx, "[ExptEval] item complete meta missing, skip publish, expt_id: %v, item_id: %v", event.ExptID, item.ItemID)
		return
	}
	completeEvent := buildItemCompleteEventFromScheduler(event.SpaceID, event.ExptID, event.ExptRunID, expt, item, evalSetItem, evalSetVersionID)
	if err := e.itemCompletePublisher.PublishItemComplete(ctx, completeEvent); err != nil {
		logs.CtxError(ctx, "[ExptEval] publish item complete event failed, expt_id: %v, item_id: %v, err: %v", event.ExptID, item.ItemID, err)
	}
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

	if err := e.terminateZombieEvalTargetRecords(ctx, event, expt, zombieItemIDs); err != nil {
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

	// item 已落终态 → 释放其额度预占。放在状态写库之后：先确保终态可见，
	// 再释放额度，避免"额度已放但 item 仍显示 Processing"这一瞬间被下一拍读到而重复授予。
	e.releaseCentralQuotaForItems(ctx, expt, event.ExptRunID, zombieItemIDs, "item zombie timeout")

	// 不清 run_log 的 target_result_id / evaluator_result_ids：
	// zombie 场景是「终态失败」，需要保留已入库的 record id，
	// 让 /results/batch_get 能返回 eval_target_record.id、evaluator_record.id 供用户查详情。
	// 「清 id」的语义只属于「重跑起点」（见 clearExptTurnRunLogResultRefsOnItems 其他调用点：
	// FailRetry / rerunItems / 手动重跑），失败落地不应触发。

	// 沙箱 agent 实验: 每个 zombie item 单独发一张飞书失败卡, 帮助用户第一时间感知卡死的行。
	// notifier 内部会先判 enabled (非沙箱 agent / FeishuNotification.Enable=false 直接跳过),
	// 所以这里不额外加类型判断; err 用 zombie timeout err, 让卡片 err_msg 明确表达超时原因。
	if e.sandboxAgentNotifier != nil {
		zombieNotifyErr := errno.NewItemZombieTimeoutErr(zombieSecond, asyncExec)
		for _, itemID := range zombieItemIDs {
			if nerr := e.sandboxAgentNotifier.NotifyItemFail(ctx, expt, itemID, zombieNotifyErr); nerr != nil {
				logs.CtxWarn(ctx, "[ExptEval] sandbox agent notify zombie item fail err, expt_id: %v, item_id: %v, err: %v", event.ExptID, itemID, nerr)
			}
		}
	}

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
	if e.evalTargetService == nil || expt == nil {
		return items, nil, nil
	}
	// 只对确实用 SandboxAgent 的实验做 sweep，避免每个 tick 都发无谓 RPC。
	if !isSandboxAgentExpt(expt) {
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
		return items, nil, nil
	}

	turnRunLogs, err := e.ExptTurnResultRepo.MGetItemTurnRunLogs(ctx, event.ExptID, event.ExptRunID, processingItemIDs, event.SpaceID)
	if err != nil {
		return items, nil, err
	}

	// recordID -> itemID(s)，一个 record 可能只对应一个 turn，但为稳妥仍走 slice
	recordIDToItemIDs := make(map[int64][]int64)
	recordIDs := make([]int64, 0)
	for _, rl := range turnRunLogs {
		if rl == nil || rl.TargetResultID <= 0 {
			continue
		}
		if _, exists := recordIDToItemIDs[rl.TargetResultID]; !exists {
			recordIDs = append(recordIDs, rl.TargetResultID)
		}
		recordIDToItemIDs[rl.TargetResultID] = append(recordIDToItemIDs[rl.TargetResultID], rl.ItemID)
	}
	if len(recordIDs) == 0 {
		return items, nil, nil
	}

	// 跨空间共享: EvalTargetRecord.SpaceID 是来源空间, 用消费方 event.SpaceID 查会漏。
	var targetSourceSpaceID int64
	if expt != nil {
		targetSourceSpaceID = expt.TargetSpaceID
	}
	targetSpaceID := resolveLoadSpaceID(event.SpaceID, targetSourceSpaceID)
	terminatedRecordIDs, statusMap := e.evalTargetService.CheckSandboxTerminated(ctx, targetSpaceID, recordIDs)
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
	// targetSpaceID 复用上面 CheckSandboxTerminated 用的来源空间, 保持一致。
	e.evalTargetService.TerminateAsyncRecordsAndDestroySandbox(
		ctx,
		targetSpaceID,
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

	// item 已落终态 → 释放额度预占。沙箱已提前终止，对应 consumer 消息不会再回来，
	// 不在此释放则这些 item 的额度永久泄漏。
	e.releaseCentralQuotaForItems(ctx, expt, event.ExptRunID, terminatedItemIDs, "sandbox terminated before report")

	// 打点：复用沙箱 agent 评测对象的 EmitInvokeFinished，让"沙箱终态未回调 → 兜底失败"
	// 与正常回调路径的 invoke_finished 在同一 dashboard 上可比。err_code 用
	// SandboxTerminatedBeforeReportCode，classifier 归入 non_engineering。
	// invoke_id 对齐提交侧：一次 invocation 的 invokeID = record.ID。
	e.emitSandboxSweptInvokeFinished(ctx, event, expt, terminatedRecordIDs, recordIDToItemIDs)

	return alives, terminated, nil
}

// emitSandboxSweptInvokeFinished 为 sweep 命中的每个 record 补一次 invoke_finished 打点。
// tag 语义与 emitInvokeStarted / EvalOpenAPIApplication.emitSandboxAgentInvokeFinished 对齐；
// item_key / dataset_key 需要额外查 dataset item, 这里为控代价留空 (tag 层会填 "-")。
// 允许 sandboxAgentMetrics == nil（未接入 metrics 时静默 no-op）。
func (e *ExptSchedulerImpl) emitSandboxSweptInvokeFinished(
	ctx context.Context,
	event *entity.ExptScheduleEvent,
	expt *entity.Experiment,
	terminatedRecordIDs []int64,
	recordIDToItemIDs map[int64][]int64,
) {
	if e.sandboxAgentMetrics == nil || len(terminatedRecordIDs) == 0 {
		return
	}
	var datasetID, datasetVersion int64
	if expt != nil && expt.EvalSet != nil {
		datasetID = expt.EvalSet.ID
		if expt.EvalSet.EvaluationSetVersion != nil {
			datasetVersion = expt.EvalSet.EvaluationSetVersion.ID
		}
	}
	var targetID int64
	if expt != nil {
		targetID = expt.TargetID
	}
	// submitTime 无法从 sweep 上下文精确得到 (record 上有 CreatedAt, 但获取要额外 RPC);
	// 传 zero time, emit 侧会把 duration 归 0, 与开源 stub 保持一致语义。
	var zero time.Time
	reportErr := errno.NewSandboxTerminatedBeforeReportErr("swept")
	errCode := int32(errno.SandboxTerminatedBeforeReportCode)
	for _, recordID := range terminatedRecordIDs {
		itemIDs := recordIDToItemIDs[recordID]
		// 一个 record 通常对应一个 turn / 一个 item, 保底遍历。
		if len(itemIDs) == 0 {
			itemIDs = []int64{0}
		}
		for _, itemID := range itemIDs {
			tags := metrics.SandboxAgentInvokeTags{
				ExperimentID:   event.ExptID,
				ItemID:         itemID,
				InvokeID:       strconv.FormatInt(recordID, 10),
				DatasetID:      datasetID,
				DatasetVersion: datasetVersion,
				TargetID:       targetID,
			}
			e.sandboxAgentMetrics.EmitInvokeFinished(tags, reportErr, errCode, zero)
		}
	}
	logs.CtxInfo(ctx, "[ExptEval] sandbox sweep invoke_finished emitted, expt_id=%v, records=%d", event.ExptID, len(terminatedRecordIDs))
}

// terminateZombieEvalTargetRecords 将僵尸 item 关联的 EvalTargetRecord（仅 SandboxAgent 类型且仍 AsyncInvoking）置为 Fail，
// 并 best-effort 销毁对应的沙箱 execute。expt 用来判断实验类型: 只有 SandboxAgent 才补 EmitInvokeFinished 打点,
// 避免非 sandbox zombie 污染 sandbox_agent 看板。
func (e *ExptSchedulerImpl) terminateZombieEvalTargetRecords(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, zombieItemIDs []int64) error {
	if len(zombieItemIDs) == 0 || e.evalTargetService == nil {
		return nil
	}

	turnRunLogs, err := e.ExptTurnResultRepo.MGetItemTurnRunLogs(ctx, event.ExptID, event.ExptRunID, zombieItemIDs, event.SpaceID)
	if err != nil {
		return err
	}

	// recordID -> 关联 itemIDs, 打点时需要按 (record, item) 展开 tag
	recordIDToItemIDs := make(map[int64][]int64)
	recordIDs := make([]int64, 0)
	for _, rl := range turnRunLogs {
		if rl == nil || rl.TargetResultID <= 0 {
			continue
		}
		if _, exists := recordIDToItemIDs[rl.TargetResultID]; !exists {
			recordIDs = append(recordIDs, rl.TargetResultID)
		}
		recordIDToItemIDs[rl.TargetResultID] = append(recordIDToItemIDs[rl.TargetResultID], rl.ItemID)
	}
	if len(recordIDs) == 0 {
		return nil
	}

	// 跨空间共享: EvalTargetRecord 落库时 SpaceID = 评测对象来源空间 (resolveLoadSpaceID 结果),
	// 若这里仍用 event.SpaceID (消费方) 去查, DAO 的 SpaceID.Eq 过滤会返回空 → Destroy RPC 根本发不出,
	// 导致沙箱在来源空间空跑到 session TTL 兜底才回收。与 expt_run_item_turn_impl.go:130 保持同一套解析。
	var targetSourceSpaceID int64
	if expt != nil {
		targetSourceSpaceID = expt.TargetSpaceID
	}
	targetSpaceID := resolveLoadSpaceID(event.SpaceID, targetSourceSpaceID)
	e.evalTargetService.TerminateAsyncRecordsAndDestroySandbox(
		ctx,
		targetSpaceID,
		recordIDs,
		int32(errno.AsyncEvalTargetZombieTimeoutCode),
		"async eval target terminated: experiment item exceeded zombie timeout",
		true,
	)

	// 打点: 与 sweep 命中路径复用同一 EmitInvokeFinished, 让 zombie 兜底 fail 与其它 fail 在同一 dashboard 上可比。
	// errCode = AsyncEvalTargetZombieTimeoutCode, classifier 归入 non_engineering (未在 engineering 白名单)。
	if isSandboxAgentExpt(expt) {
		e.emitSandboxZombieInvokeFinished(ctx, event, expt, recordIDs, recordIDToItemIDs)
	}
	return nil
}

// isSandboxAgentExpt 判断实验是否 SandboxAgent 类型。判空后再取 EvalTargetType, 保持与 sweep 一致。
func isSandboxAgentExpt(expt *entity.Experiment) bool {
	return expt != nil &&
		expt.Target != nil &&
		expt.Target.EvalTargetVersion != nil &&
		expt.Target.EvalTargetVersion.EvalTargetType == entity.EvalTargetTypeSandboxAgent
}

// emitSandboxZombieInvokeFinished 与 emitSandboxSweptInvokeFinished 语义一致, 只是 errCode 换成 zombie timeout。
func (e *ExptSchedulerImpl) emitSandboxZombieInvokeFinished(
	ctx context.Context,
	event *entity.ExptScheduleEvent,
	expt *entity.Experiment,
	recordIDs []int64,
	recordIDToItemIDs map[int64][]int64,
) {
	if e.sandboxAgentMetrics == nil || len(recordIDs) == 0 {
		return
	}
	var datasetID, datasetVersion int64
	if expt != nil && expt.EvalSet != nil {
		datasetID = expt.EvalSet.ID
		if expt.EvalSet.EvaluationSetVersion != nil {
			datasetVersion = expt.EvalSet.EvaluationSetVersion.ID
		}
	}
	var targetID int64
	if expt != nil {
		targetID = expt.TargetID
	}
	var zero time.Time
	reportErr := errno.NewItemZombieTimeoutErr(0, true)
	errCode := int32(errno.AsyncEvalTargetZombieTimeoutCode)
	for _, recordID := range recordIDs {
		itemIDs := recordIDToItemIDs[recordID]
		if len(itemIDs) == 0 {
			itemIDs = []int64{0}
		}
		for _, itemID := range itemIDs {
			tags := metrics.SandboxAgentInvokeTags{
				ExperimentID:   event.ExptID,
				ItemID:         itemID,
				InvokeID:       strconv.FormatInt(recordID, 10),
				DatasetID:      datasetID,
				DatasetVersion: datasetVersion,
				TargetID:       targetID,
			}
			e.sandboxAgentMetrics.EmitInvokeFinished(tags, reportErr, errCode, zero)
		}
	}
	logs.CtxInfo(ctx, "[ExptEval] sandbox zombie invoke_finished emitted, expt_id=%v, records=%d", event.ExptID, len(recordIDs))
}

// releaseCentralQuotaForItems 为一批已被判为终态的 item 释放中心调度额度预占。
//
// 用于 zombie 清理与沙箱提前终态两条 daemon 路径：它们直接把 item 落成 Fail，
// 对应的 consumer 消息可能已经卡死或永不返回，因此必须在这里释放，否则额度永久泄漏
// （现象是"额度跑满后再也调度不动"，且看起来像上限配小了，极易误判）。
//
// legacy 实验直接返回：它们从不预占额度，调用释放只是无谓的 Redis 往返。
// 全程 best-effort：单个 item 释放失败只告警，不影响 zombie 清理本身 ——
// 清理失败会让实验永不收敛，比额度泄漏更严重，而额度有对账兜底。
func (e *ExptSchedulerImpl) releaseCentralQuotaForItems(ctx context.Context, expt *entity.Experiment, exptRunID int64, itemIDs []int64, reason string) {
	if e.centralGuard == nil || expt == nil || len(itemIDs) == 0 {
		return
	}
	if !entity.IsCentralDispatch(expt.ExptDispatchMode) {
		return
	}
	if expt.SchedulerScope == "" {
		// enforce 却无 Scope：数据异常，无法确定去哪本账释放。告警交由对账处理，
		// 不猜一本账 —— 猜错会归还别人的额度，比不归还更糟。
		logs.CtxError(ctx, "[CentralReservation] enforce experiment without scheduler_scope, skip release, expt_id: %v, item_ids: %v",
			expt.ID, itemIDs)
		return
	}

	for _, itemID := range itemIDs {
		if err := e.centralGuard.Release(ctx, expt.SchedulerScope, exptRunID, itemID, reason); err != nil {
			logs.CtxWarn(ctx, "[CentralReservation] release quota fail, scope: %v, expt_run_id: %v, item_id: %v, reason: %v, err: %v",
				expt.SchedulerScope, exptRunID, itemID, reason, err)
		}
	}
	logs.CtxInfo(ctx, "[CentralReservation] quota released for daemon-terminated items, scope: %v, expt_run_id: %v, count: %v, reason: %v",
		expt.SchedulerScope, exptRunID, len(itemIDs), reason)
}
