// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"github.com/jinzhu/copier"

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptItemEventEvalServiceImpl struct {
	endpoints                RecordEvalEndPoint
	manager                  IExptManager
	publisher                events.ExptEventPublisher
	exptItemResultRepo       repo.IExptItemResultRepo
	exptTurnResultRepo       repo.IExptTurnResultRepo
	exptStatsRepo            repo.IExptStatsRepo
	experimentRepo           repo.IExperimentRepo
	exptItemRefRepo          repo.IExptItemRefRepo // ★ 新实验类型 (MultiSetConfig) 读 item_config 用
	configer                 component.IConfiger
	quotaRepo                repo.QuotaRepo
	mutex                    lock.ILocker
	idem                     idem.IdempotentService
	auditClient              audit.IAuditService
	metric                   metrics.ExptMetric
	resultSvc                ExptResultService
	evaluationSetItemService EvaluationSetItemService
	evaluatorService         EvaluatorService
	evaTargetService         IEvalTargetService
	evaluatorRecordService   EvaluatorRecordService
	idgen                    idgen.IIDGenerator
	benefitService           benefit.IBenefitService
	evalAsyncRepo            repo.IEvalAsyncRepo
	itemCompletePublisher    component.IItemCompletePublisher
	sandboxAgentNotifier     ISandboxAgentNotifier       // 传递给 ExptItemEvalCtxExecutor 用于失败行飞书通知
	sandboxAgentMetrics      metrics.SandboxAgentMetrics // 沙箱 agent 端到端 (turn 粒度) 打点; 可空 → 走 noop
	// centralGuard 中心化调度的额度预占校验闸。开源部署注入 noop（enforce 消息 fail-closed），
	// 商业版由 Wire 注入真实账本适配器。legacy 实验不经过它，行为与引入前一致。
	centralGuard component.ICentralReservationGuard
	// dispatchRepo run log 派发投影读写，用于把 Queueing/reserved 兑现为 Processing/none。
	dispatchRepo repo.IExptItemDispatchRepo
	// centralScopeOwner 判定本进程是否拥有某实验的调度域。开源部署注入 noop（恒定拥有）。
	// 防的是 item 消息跨环境投递后被错误的进程执行（详见该 port 的注释）。
	centralScopeOwner component.ICentralSchedulerScopeOwner
}

func NewExptRecordEvalService(
	manager IExptManager,
	configer component.IConfiger,
	publisher events.ExptEventPublisher,
	exptItemResultRepo repo.IExptItemResultRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	exptStatsRepo repo.IExptStatsRepo,
	experimentRepo repo.IExperimentRepo,
	exptItemRefRepo repo.IExptItemRefRepo,
	quotaRepo repo.QuotaRepo,
	mutex lock.ILocker,
	idem idem.IdempotentService,
	auditClient audit.IAuditService,
	metric metrics.ExptMetric,
	resultSvc ExptResultService,
	evaTargetService IEvalTargetService,
	evaluationSetItemService EvaluationSetItemService,
	evaluatorRecordService EvaluatorRecordService,
	evaluatorService EvaluatorService,
	idgen idgen.IIDGenerator,
	benefitService benefit.IBenefitService,
	evalAsyncRepo repo.IEvalAsyncRepo,
	itemCompletePublisher component.IItemCompletePublisher,
	sandboxAgentMetrics metrics.SandboxAgentMetrics, // 沙箱 agent 端到端 turn 打点; 可空 (走 noop)
	centralGuard component.ICentralReservationGuard,
	dispatchRepo repo.IExptItemDispatchRepo,
	// centralScopeOwner 放在 variadic 之前：Go 只要求 variadic 是最后一个参数。
	// 不做 setter 注入 —— setter 会让 wire 构造出实例但无人调用 setter，字段恒为 nil，
	// 而 nil 在本文件里被解释为"跳过校验"，等于静默关闭防护（此前 centralGuard 已踩过一次）。
	centralScopeOwner component.ICentralSchedulerScopeOwner,
	sandboxAgentNotifier ...ISandboxAgentNotifier, // variadic 兼容 wire_gen 未接入通知器
) ExptItemEvalEvent {
	i := &ExptItemEventEvalServiceImpl{
		manager:                  manager,
		publisher:                publisher,
		exptItemResultRepo:       exptItemResultRepo,
		exptTurnResultRepo:       exptTurnResultRepo,
		exptStatsRepo:            exptStatsRepo,
		experimentRepo:           experimentRepo,
		exptItemRefRepo:          exptItemRefRepo,
		configer:                 configer,
		quotaRepo:                quotaRepo,
		mutex:                    mutex,
		idem:                     idem,
		auditClient:              auditClient,
		metric:                   metric,
		resultSvc:                resultSvc,
		evaTargetService:         evaTargetService,
		evaluationSetItemService: evaluationSetItemService,
		evaluatorRecordService:   evaluatorRecordService,
		evaluatorService:         evaluatorService,
		idgen:                    idgen,
		benefitService:           benefitService,
		evalAsyncRepo:            evalAsyncRepo,
		itemCompletePublisher:    itemCompletePublisher,
		sandboxAgentMetrics:      sandboxAgentMetrics,
		centralGuard:             centralGuard,
		centralScopeOwner:        centralScopeOwner,
		dispatchRepo:             dispatchRepo,
	}
	if len(sandboxAgentNotifier) > 0 {
		i.sandboxAgentNotifier = sandboxAgentNotifier[0]
	}

	i.endpoints = RecordEvalChain(
		i.HandleEventErr,
		i.HandleEventCheck,
		// 额度闸放在 Check 之后、Lock 之前：Check 已排除掉终态 run（那些消息不需要额度校验），
		// 而放在 Lock 之前可避免为一条注定要丢弃的消息去抢 item 锁。
		i.HandleCentralReservation,
		i.HandleEventLock,
		i.HandleEventExec,
	)(func(_ context.Context, _ *entity.ExptItemEvalEvent) error { return nil })

	return i
}

func (e *ExptItemEventEvalServiceImpl) Eval(ctx context.Context, event *entity.ExptItemEvalEvent) error {
	ctx = ctxcache.Init(ctx)

	if err := e.endpoints(ctx, event); err != nil {
		logs.CtxError(ctx, "[ExptTurnEval] expt record eval fail, event: %v, err: %v", json.Jsonify(event), err)
		return err
	}

	return nil
}

type RecordEvalEndPoint func(ctx context.Context, event *entity.ExptItemEvalEvent) error

type RecordEvalMiddleware func(next RecordEvalEndPoint) RecordEvalEndPoint

func RecordEvalChain(mws ...RecordEvalMiddleware) RecordEvalMiddleware {
	return func(next RecordEvalEndPoint) RecordEvalEndPoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

func (e *ExptItemEventEvalServiceImpl) HandleEventCheck(next RecordEvalEndPoint) RecordEvalEndPoint {
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		runLog, err := e.manager.GetRunLog(ctx, event.ExptID, event.ExptRunID, event.SpaceID, event.Session)
		if err != nil {
			return err
		}

		if status := entity.ExptStatus(runLog.Status); entity.IsExptFinished(status) || entity.IsExptFinishing(status) {
			logs.CtxInfo(ctx, "ExptRecordEvalConsumer consume finished expt run event, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
			return nil
		}

		return next(ctx, event)
	}
}

// HandleCentralReservation 对中心调度纳管的实验校验额度预占。
//
// 模式判定**回查 experiment.scheduler_mode DB 列**，不看 event 上的任何标记：
// 若模式随 event 传递，字段丢失或取默认零值时，一条实际为 central 的消息会被当作 legacy 处理，
// 从而跳过本校验、静默绕过额度执行 —— 这个方向的失败是无声的，比多查一次 DB 危险得多。
func (e *ExptItemEventEvalServiceImpl) HandleCentralReservation(next RecordEvalEndPoint) RecordEvalEndPoint {
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		expt, err := e.experimentRepo.GetByID(ctx, event.ExptID, event.SpaceID)
		if err != nil {
			return err
		}
		if !entity.IsCentralDispatch(expt.ExptDispatchMode) {
			// legacy 实验：完全不经额度闸，行为与改动前一致
			return next(ctx, event)
		}

		// enforce 实验必须有冻结的 Scope。空 Scope 说明数据异常（DDL 未执行、或绕过
		// Create 路径写入），fail-closed 丢弃：没有 Scope 就无法确定去哪本账查 reservation，
		// 猜一本账等于用别人的额度跑这个 item。
		if expt.SchedulerScope == "" {
			logs.CtxError(ctx, "[CentralReservation] enforce experiment with empty scheduler_scope, drop event, expt_id: %v, item_id: %v",
				event.ExptID, event.EvalSetItemID)
			return nil
		}

		// 本进程必须拥有该实验的 Scope 才可执行。
		//
		// 这是"泳道不得执行线上 item"的最后一道闸。前面的调度侧闸门（ScanCandidates 带
		// WHERE scheduler_scope）防的是"泳道去调度线上实验"；本闸防的是消息侧 ——
		// item MQ 的泳道路由靠 producer 的 x_tt_env tag，而 tag 可能因环境变量缺失、
		// 消息重投或 broker 配置而失效，导致一条 PPE 的 item 消息被线上 consumer 拿到
		// （或反之）。届时若不校验归属，就会用一个环境的进程去跑另一个环境的 item，
		// 结果写回共享库。
		//
		// 丢弃而非报错重试：Scope 不匹配是路由问题，重试只会在同一个错误进程上再失败一次；
		// 正确的那个进程会从自己的队列里拿到这条消息（或由中心调度下一拍重新派发）。
		if e.centralScopeOwner != nil {
			owned, err := e.centralScopeOwner.OwnsSchedulerScope(ctx, expt.SchedulerScope)
			if err != nil {
				// 无法判定归属：返回错误重试，而不是赌一把放行。
				return err
			}
			if !owned {
				logs.CtxWarn(ctx, "[CentralReservation] scheduler scope not owned by this instance, discard event, expt_scope: %v, expt_id: %v, item_id: %v",
					expt.SchedulerScope, event.ExptID, event.EvalSetItemID)
				return nil
			}
		}

		guard := e.centralGuard
		if guard == nil {
			// 判定为 enforce 却没有注入闸门：fail-closed。
			// 放行等于让实验在无额度约束下跑，静默且难以发现；停下来则可见。
			logs.CtxWarn(ctx, "[CentralReservation] enforce experiment without guard, drop event, expt_id: %v, item_id: %v",
				event.ExptID, event.EvalSetItemID)
			return nil
		}

		ok, err := guard.ConfirmRunning(ctx, expt.SchedulerScope, event.ExptRunID, event.EvalSetItemID)
		if err != nil {
			// 账本暂时不可用：返回错误让 MQ 重试，而不是丢弃 —— item 已被预占，
			// 丢弃会让它停在 Queueing 直到 reservation 超时清理，白等一轮。
			return err
		}
		if !ok {
			// reservation 不存在：迟到消息、账本已重建、或已被释放。丢弃不执行。
			logs.CtxInfo(ctx, "[CentralReservation] reservation absent, discard event, expt_run_id: %v, item_id: %v",
				event.ExptRunID, event.EvalSetItemID)
			return nil
		}

		// 把 run log 投影从 Queueing/reserved 兑现为 Processing/none。
		//
		// 为什么必须在这里而不是留给下游 handleEventExec：投影是调度器算并发占用的依据，
		// 若 item 已开始执行而投影仍停在 Queueing/reserved，下一拍会把它当"已预占未消费"
		// 继续计入占用（这没错），但一旦 reservation 因超时被清理，它就变成"既不 Processing
		// 也无 reservation"的孤儿，对账要多绕一圈才能修。就地兑现让两侧同步收敛。
		//
		// CAS 未命中（started=false）不阻断执行：可能是重复投递（已 Processing）或
		// 投影已被 repair 修正。此时 reservation 校验已通过，说明额度是真的，继续执行是安全的。
		if e.dispatchRepo != nil {
			started, err := e.dispatchRepo.StartReservedItem(ctx, event.SpaceID, event.ExptID, event.ExptRunID, event.EvalSetItemID)
			if err != nil {
				// 投影写失败：返回错误让 MQ 重试。额度已预占且 reservation 已转 Running，
				// 丢弃消息会让这份额度占着直到超时清理。
				return err
			}
			if !started {
				logs.CtxInfo(ctx, "[CentralReservation] run log projection not claimed (duplicate delivery or repaired), continue, expt_run_id: %v, item_id: %v",
					event.ExptRunID, event.EvalSetItemID)
			}
		}

		return next(ctx, event)
	}
}

func (e *ExptItemEventEvalServiceImpl) HandleEventErr(next RecordEvalEndPoint) RecordEvalEndPoint {
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		nextErr := func(ctx context.Context, event *entity.ExptItemEvalEvent) (err error) {
			defer goroutine.Recover(ctx, &err)
			return next(ctx, event)
		}(ctx, event)

		retryConf := e.configer.GetErrRetryConf(ctx, event.SpaceID, nextErr)
		needRetry := event.RetryTimes < retryConf.GetRetryTimes()
		if event.MaxRetryTimes > 0 {
			needRetry = event.RetryTimes < event.MaxRetryTimes
		}
		if event.CtxForceNoRetry(ctx) {
			needRetry = false
		}

		defer func() {
			code, stable, _ := errno.ParseStatusError(nextErr)
			e.metric.EmitItemExecResult(event.SpaceID, int64(event.ExptRunMode), nextErr != nil, needRetry, stable, int64(code), event.CreateAt)
		}()

		logs.CtxInfo(ctx, "[ExptTurnEval] handle event done, success: %v, retry: %v, retry_times: %v, err: %v, indebt: %v, event: %v",
			nextErr == nil, needRetry, retryConf.GetRetryTimes(), nextErr, retryConf.IsInDebt, json.Jsonify(event))

		if nextErr == nil {
			return nil
		}

		if retryConf.IsInDebt {
			completeCID := fmt.Sprintf("terminate:indebt:%d", event.ExptRunID)

			if err := e.manager.CompleteRun(ctx, event.ExptID, event.ExptRunID, event.SpaceID, event.Session, entity.WithCID(completeCID), entity.WithCompleteInterval(time.Second*2)); err != nil {
				return errorx.Wrapf(err, "terminate expt run fail, expt_id: %v", event.ExptID)
			}

			if err := e.manager.CompleteExpt(ctx, event.ExptID, &event.ExptRunID, event.SpaceID, event.Session, entity.WithStatus(entity.ExptStatus_Terminated),
				entity.WithStatusMessage(nextErr.Error()), entity.WithCID(completeCID), entity.WithCompleteInterval(time.Second*2)); err != nil {
				return errorx.Wrapf(err, "complete expt fail, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
			}

			return nil
		}

		if needRetry {
			clone := &entity.ExptItemEvalEvent{}
			if err := copier.CopyWithOption(clone, event, copier.Option{DeepCopy: true}); err != nil {
				return errorx.Wrapf(err, "ExptItemEvalEvent copy fail")
			}

			clone.RetryTimes += 1

			return e.publisher.PublishExptRecordEvalEvent(ctx, clone, gptr.Of(retryConf.GetRetryInterval()), func(ne *entity.ExptItemEvalEvent) {
				ne.AsyncReportTrigger = false
				ne.AsyncEvaluatorReportTrigger = false
			})
		}

		// ★ 兜底落 Fail: 失败且不重试时, item 状态此前无人更新 —— 正常路径由 RunItem→CompleteItemRun
		// 落 Fail, 但在 eval() 里 BuildExptRecordEvalCtx 等前置阶段就失败时压根没走到那里(eiec 未构建),
		// item 停在进入执行时写的 Processing 上, 表现为"报错了却永久卡 processing、实验永不收敛"。
		// 此处按 CompleteItemRun 同样的字段兜底(status=Fail + err_msg + result_state=Logged), 幂等可重复写。
		e.completeItemRunOnUnretriableErr(ctx, event, nextErr)

		return nil
	}
}

// completeItemRunOnUnretriableErr 将 item 落为 Fail 并写入错误信息。
// 仅在"失败且不可重试"时调用; 写库失败只告警不影响主流程(僵尸清理仍是最后防线)。
//
// ★ 必须同时落 turn run log: 本兜底的目标场景是 BuildExptRecordEvalCtx 等前置阶段失败,
// 此时 PreEval 还没执行过、该 run 下一条 turn run log 都没有。item 一旦变 Fail+Logged 就会被
// scanIncompleteAndComplete 归入 complete → recordEvalItemRunLogs → RecordItemRunLogs 因
// turn run log 缺失报 "found null turn log result" → 重试 5min 后整实验被判 Failed(其余 item 全中断)。
// 僵尸清理路径(handleZombies)正是成对做 UpdateItemRunLog + CreateOrUpdateItemsTurnRunLogStatus,
// 此处与之对齐; 缺这一步会把"单 item 失败"放大成"整实验失败", 比原先卡 Processing 更糟。
func (e *ExptItemEventEvalServiceImpl) completeItemRunOnUnretriableErr(ctx context.Context, event *entity.ExptItemEvalEvent, evalErr error) {
	if event == nil || evalErr == nil || e.exptItemResultRepo == nil {
		return
	}

	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), exptRunLogPersistTimeout)
	defer cancel()

	ufields := map[string]any{
		"status":       int32(entity.ItemRunState_Fail),
		"err_msg":      errno.SerializeErr(evalErr),
		"result_state": int32(entity.ExptItemResultStateLogged),
	}
	if err := e.exptItemResultRepo.UpdateItemRunLog(persistCtx, event.ExptID, event.ExptRunID,
		[]int64{event.EvalSetItemID}, ufields, event.SpaceID); err != nil {
		logs.CtxWarn(persistCtx, "completeItemRunOnUnretriableErr update item run log fail, expt_id: %v, expt_run_id: %v, item_id: %v, err: %v",
			event.ExptID, event.ExptRunID, event.EvalSetItemID, err)
	}

	if e.exptTurnResultRepo == nil {
		return
	}
	if err := e.exptTurnResultRepo.CreateOrUpdateItemsTurnRunLogStatus(persistCtx, event.SpaceID, event.ExptID, event.ExptRunID,
		[]int64{event.EvalSetItemID}, entity.TurnRunState_Fail); err != nil {
		logs.CtxWarn(persistCtx, "completeItemRunOnUnretriableErr create/update turn run log fail, expt_id: %v, expt_run_id: %v, item_id: %v, err: %v",
			event.ExptID, event.ExptRunID, event.EvalSetItemID, err)
	}
}

func (e *ExptItemEventEvalServiceImpl) HandleEventLock(next RecordEvalEndPoint) RecordEvalEndPoint {
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		lockKey := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", event.ExptID, event.EvalSetItemID)
		locked, ctx, cancel, err := e.mutex.LockWithRenew(ctx, lockKey, time.Second*5, time.Second*60*60)
		if err != nil {
			return err
		}

		if !locked {
			logs.CtxWarn(ctx, "ExptRecordEvalConsumer.HandleEventLock found locked item eval event: %v. Abort event, err: %v", json.Jsonify(event), err)
			return nil
		}

		defer func() {
			cancel()
			if _, err := e.mutex.Unlock(lockKey); err != nil {
				logs.CtxWarn(ctx, "failed to unlock key: %v, err: %v", lockKey, err)
			}
		}()

		return next(ctx, event)
	}
}

func (e *ExptItemEventEvalServiceImpl) HandleEventExec(next RecordEvalEndPoint) RecordEvalEndPoint {
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		if err := e.eval(ctx, event); err != nil {
			return err
		}
		return next(ctx, event)
	}
}

func (e *ExptItemEventEvalServiceImpl) eval(ctx context.Context, event *entity.ExptItemEvalEvent) error {
	eiec, err := e.BuildExptRecordEvalCtx(ctx, event)
	if err != nil {
		return err
	}

	ctx = e.WithCtx(ctx, eiec)

	mode, err := NewRecordEvalMode(
		eiec.Event,
		e.exptItemResultRepo,
		e.exptTurnResultRepo,
		e.exptStatsRepo,
		e.experimentRepo,
		e.metric,
		e.resultSvc,
		e.idgen,
		e.evaTargetService,
		e.evaluatorRecordService,
	)
	if err != nil {
		return err
	}

	if err := mode.PreEval(ctx, eiec); err != nil {
		return err
	}

	if err := NewExptItemEvaluation(e.exptTurnResultRepo, e.exptItemResultRepo, e.configer, e.metric, e.evaTargetService, e.evaluatorRecordService, e.evaluatorService, e.benefitService, e.evalAsyncRepo, e.evaluationSetItemService, e.itemCompletePublisher, e.sandboxAgentMetrics, e.sandboxAgentNotifier).
		Eval(ctx, eiec); err != nil {
		return err
	}

	if err := mode.PostEval(ctx, eiec); err != nil {
		return err
	}

	return nil
}

func (e *ExptItemEventEvalServiceImpl) WithCtx(ctx context.Context, eiec *entity.ExptItemEvalCtx) context.Context {
	return logs.SetLogID(ctx, eiec.GetRecordEvalLogID(ctx))
}

func (e *ExptItemEventEvalServiceImpl) BuildExptRecordEvalCtx(ctx context.Context, event *entity.ExptItemEvalEvent) (*entity.ExptItemEvalCtx, error) {
	exptDetail, err := e.manager.GetDetail(ctx, event.ExptID, event.SpaceID, event.Session)
	if err != nil {
		return nil, err
	}

	// 默认用实验级主集 (老实验 / SingleSet); MultiSetConfig 下面按 item 归属集覆盖。
	evalSetID := exptDetail.EvalSet.EvaluationSetVersion.EvaluationSetID
	evalSetVerID := exptDetail.EvalSet.EvaluationSetVersion.ID

	// ★ 新实验类型 (MultiSetConfig): 先读 expt_item_ref。
	// 一行 ref 同时承载两件事:
	//   ① 单行执行的唯一配置源 item_config;
	//   ② 该 item 真正归属的 (eval_set_id, eval_set_version_id) —— 多评测集下各 item 归属不同集,
	//      绝不能用实验级主集去捞 item, 否则非主集 item 捞不到 (len=0) 直接报错卡死, 永远停在 incomplete。
	// 老实验类型: ItemConfig 留 nil, 执行侧 fallback 到 expt 级 EvaluatorsConf 老路径; 集 id/version 用主集。
	// ⚠️ 读失败不能静默降级: exptStartMultiSet 对每个入队 item 都会写非 nil ItemConfig(空评估器集也写
	//    &entity.ExptItemConfig{}), 所以 MultiSetConfig 实验里读到 refErr / ref==nil / ItemConfig==nil,
	//    只可能是"读失败"(DB 抖动 / ref 未写全 / item_config 反序列化失败 / 列 NULL), 绝不可能是"合法空集"。
	//    此时必须报错触发重试, 否则 CallEvaluators 会把 nil ItemConfig 当成"合法空集"跑 0 个评估器并把 turn
	//    标成 Success —— 本该有评估器的正常集被静默漏评还显示成功 (fail-silent 数据正确性缺陷)。
	//    区分口径: ref!=nil && ItemConfig!=nil 才是可信配置; EvaluatorConfs 空由执行侧判为"真空集跑 0 个"。
	var itemConfig *entity.ExptItemConfig
	var itemVersionID *int64 // 新数据集 item 级版本; nil=老数据集(无 item 版本)
	if exptDetail.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig && e.exptItemRefRepo != nil {
		ref, refErr := e.exptItemRefRepo.GetByExptIDAndItemID(ctx, event.SpaceID, event.ExptID, event.EvalSetItemID)
		if refErr != nil {
			logs.CtxError(ctx, "BuildExptRecordEvalCtx GetByExptIDAndItemID fail, expt_id: %v, item_id: %v, err: %v",
				event.ExptID, event.EvalSetItemID, refErr)
			return nil, errorx.Wrapf(refErr, "get expt_item_ref fail, expt_id: %v, item_id: %v", event.ExptID, event.EvalSetItemID)
		}
		if ref == nil || ref.ItemConfig == nil {
			// 正常调度必已写非 nil ItemConfig; 走到这里是 ref 未写全或 item_config 读空 —— 报错重试, 不静默漏评。
			logs.CtxError(ctx, "BuildExptRecordEvalCtx expt_item_ref missing item_config, expt_id: %v, item_id: %v, ref_nil: %v",
				event.ExptID, event.EvalSetItemID, ref == nil)
			return nil, errorx.NewByCode(errno.CommonInternalErrorCode,
				errorx.WithExtraMsg(fmt.Sprintf("expt_item_ref missing item_config, expt_id: %v, item_id: %v", event.ExptID, event.EvalSetItemID)))
		}
		itemConfig = ref.ItemConfig
		if ref.EvalSetID > 0 {
			evalSetID = ref.EvalSetID
			// 该 item 真正归属集的版本以 ref 为准 (含草稿: ref.EvalSetVersionID==0 → 走下面 live 分支)。
			// 不能再用实验级主集版本兜底, 否则非主集 item 会被错误地按主集版本查询。
			evalSetVerID = ref.EvalSetVersionID
		}
		if ref.ItemVersionID > 0 {
			itemVersionID = gptr.Of(ref.ItemVersionID)
		}
	}

	// 老链路 (SingleSet) 或 ref 未命中时 itemVersionID 仍为 nil。
	// item 版本在 ExptStart 已落进 expt_item_result_run_log, 这里读回 (供版本评测集按 item 版本取数);
	// 无版本评测集 run_log.ItemVersionID==0 → 保持 nil, 按集版本定位, 行为不变。
	if itemVersionID == nil {
		if runLog, rlErr := e.exptItemResultRepo.GetItemRunLog(ctx, event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID); rlErr != nil {
			logs.CtxWarn(ctx, "BuildExptRecordEvalCtx GetItemRunLog for item version fail, expt_id: %v, item_id: %v, err: %v",
				event.ExptID, event.EvalSetItemID, rlErr)
		} else if runLog != nil && runLog.ItemVersionID > 0 {
			itemVersionID = gptr.Of(runLog.ItemVersionID)
		}
	}

	// 统一走 ItemVersionQueries: 每个 query 必带 ItemID; 新数据集额外带 ItemVersionID, 老数据集 versionID 留空。
	// 集级 VersionID 仍透传, 供老数据集(versionID 留空)按集版本定位。
	// ★ 跨空间共享: 执行期加载评测集 item 必须按来源空间; 多集取 item_config 冻结的 EvalSetSourceSpaceID,
	// 单集/老实验取 exptDetail.EvalSetSpaceID; 缺此切换时用调用方空间读来源空间评测集 → get dataset_version not found, turn 执行失败。
	// 多集(itemConfig != nil)行级冻结值即权威, 0 表示"该集在调用方空间"而非"未设置", 不可回退顶层列
	// (顶层 EvalSetSpaceID 由 configs[0] 兜底回填, 混合空间多集下回退会把同空间集错送到主集来源空间)。
	evalSetSourceSpaceID := int64(0)
	if itemConfig != nil {
		evalSetSourceSpaceID = itemConfig.EvalSetSourceSpaceID
	} else if exptDetail != nil {
		evalSetSourceSpaceID = exptDetail.EvalSetSpaceID
	}
	batchGetEvaluationSetItemsParam := &entity.BatchGetEvaluationSetItemsParam{
		SpaceID:         resolveLoadSpaceID(event.SpaceID, evalSetSourceSpaceID),
		EvaluationSetID: evalSetID,
		VersionID:       gptr.Of(evalSetVerID),
		ItemVersionQueries: []*entity.EvaluationItemVersionRef{
			{ItemID: event.EvalSetItemID, ItemVersionID: itemVersionID},
		},
	}
	// 草稿哨兵: evalSetVerID==0 (ref 落 0 的草稿) 或 evalSetID==evalSetVerID (提交侧占位哨兵)
	// → 不锁版本, 读侧走 live (BatchGetEvaluationSetItems VersionID=nil 读当前 dataset_item 草稿)。
	// committed (真实 version_id) 维持 ByVersion 冻结快照不变。
	if evalSetVerID == 0 || evalSetID == evalSetVerID {
		batchGetEvaluationSetItemsParam.VersionID = nil
	}
	items, err := e.evaluationSetItemService.BatchGetEvaluationSetItems(ctx, batchGetEvaluationSetItemsParam)
	if err != nil {
		return nil, err
	}

	if len(items) != 1 {
		return nil, fmt.Errorf("BatchGetEvaluationSetItems with invalid item result, eval_set_id: %v, eval_set_ver_id: %v, item_id: %v, got items len: %v", evalSetID, evalSetVerID, event.EvalSetItemID, len(items))
	}
	// DataSet 读侧在部分路径不会回填集/条目版本元信息。执行上下文已根据
	// expt_item_ref / run_log 解析出 per-item 归属, 在这里补齐给下游 ItemMeta 使用,
	// 避免 MultiSetConfig 非主集 item 又回退到实验顶层主集。
	if items[0].EvaluationSetID == 0 {
		items[0].EvaluationSetID = evalSetID
	}
	if items[0].ItemVersionID == nil && itemVersionID != nil {
		items[0].ItemVersionID = itemVersionID
	}

	existResult, err := e.GetExistExptRecordEvalResult(ctx, event)
	if err != nil {
		return nil, err
	}

	return &entity.ExptItemEvalCtx{
		Event:               event,
		Expt:                exptDetail,
		EvalSetItem:         items[0],
		ExistItemEvalResult: existResult,
		ItemConfig:          itemConfig,
		EvalSetVersionID:    evalSetVerID, // per-item 归属集版本 (多评测集非主集也正确)
	}, nil
}

func (e *ExptItemEventEvalServiceImpl) GetExistExptRecordEvalResult(ctx context.Context, event *entity.ExptItemEvalEvent) (*entity.ExptItemEvalResult, error) {
	turnRunLogs, err := e.exptTurnResultRepo.GetItemTurnRunLogs(ctx, event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID)
	if err != nil {
		return nil, err
	}

	turnRunResultMap := make(map[int64]*entity.ExptTurnResultRunLog, len(turnRunLogs))
	for _, result := range turnRunLogs {
		turnRunResultMap[result.TurnID] = result
	}

	itemRunLog, err := e.exptItemResultRepo.GetItemRunLog(ctx, event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID)
	if err != nil {
		return nil, err
	}

	return &entity.ExptItemEvalResult{
		ItemResultRunLog:  itemRunLog,
		TurnResultRunLogs: turnRunResultMap,
	}, nil
}

// RecordEvalMode task execution mode
type RecordEvalMode interface {
	PreEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error
	PostEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error
}

func NewRecordEvalMode(
	event *entity.ExptItemEvalEvent, exptItemResultRepo repo.IExptItemResultRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	exptStatsRepo repo.IExptStatsRepo,
	experimentRepo repo.IExperimentRepo,
	metric metrics.ExptMetric,
	resultSvc ExptResultService,
	idgen idgen.IIDGenerator,
	evalTargetService IEvalTargetService,
	evaluatorRecordService EvaluatorRecordService,
) (RecordEvalMode, error) {
	switch event.ExptRunMode {
	case entity.EvaluationModeSubmit, entity.EvaluationModeAppend, entity.EvaluationModeTrialRun:
		return &ExptRecordEvalModeSubmit{
			exptItemResultRepo: exptItemResultRepo,
			exptTurnResultRepo: exptTurnResultRepo,
			exptRepo:           experimentRepo,
			idgen:              idgen,
		}, nil
	case entity.EvaluationModeFailRetry:
		return &ExptRecordEvalModeFailRetry{
			exptItemResultRepo: exptItemResultRepo,
			exptTurnResultRepo: exptTurnResultRepo,
			exptStatsRepo:      exptStatsRepo,
			experimentRepo:     experimentRepo,
			metric:             metric,
			resultSvc:          resultSvc,
			idgen:              idgen,
			evalTargetService:  evalTargetService,
			evaluatorRecordSvc: evaluatorRecordService,
		}, nil
	case entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems:
		return &ExptRecordEvalModeRetryIgnoreResult{
			exptTurnResultRepo: exptTurnResultRepo,
			idgen:              idgen,
		}, nil
	default:
		return nil, fmt.Errorf("NewRecordEvalMode with unknown expt mode: %v", event.ExptRunMode)
	}
}

type ExptRecordEvalModeSubmit struct {
	exptItemResultRepo repo.IExptItemResultRepo
	exptTurnResultRepo repo.IExptTurnResultRepo
	exptRepo           repo.IExperimentRepo
	idgen              idgen.IIDGenerator
}

func (e *ExptRecordEvalModeSubmit) PreEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	if eiec.GetExistItemResultLog() != nil && len(eiec.GetExistTurnResultLogs()) > 0 {
		return nil
	}

	event := eiec.Event
	turns := eiec.EvalSetItem.Turns

	got, err := e.exptTurnResultRepo.GetItemTurnRunLogs(ctx, event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID)
	if err != nil {
		return err
	}

	for _, turnResult := range got {
		eiec.ExistItemEvalResult.TurnResultRunLogs[turnResult.TurnID] = turnResult
	}

	absentRunLogTurnIDs := make([]int64, 0, len(turns))
	for _, turn := range turns {
		if turn == nil {
			continue
		}
		if eiec.GetExistTurnResultRunLog(turn.ID) == nil {
			absentRunLogTurnIDs = append(absentRunLogTurnIDs, turn.ID)
		}
	}

	if len(absentRunLogTurnIDs) > 0 {
		ids, err := e.idgen.GenMultiIDs(ctx, len(absentRunLogTurnIDs))
		if err != nil {
			return err
		}

		logID := logs.GetLogID(ctx)

		turnRunResults := make([]*entity.ExptTurnResultRunLog, 0, len(absentRunLogTurnIDs))
		for idx, turnID := range absentRunLogTurnIDs {
			turnRunResults = append(turnRunResults, &entity.ExptTurnResultRunLog{
				ID:        ids[idx],
				SpaceID:   event.SpaceID,
				ExptID:    event.ExptID,
				ExptRunID: event.ExptRunID,
				ItemID:    event.EvalSetItemID,
				TurnID:    turnID,
				Status:    entity.TurnRunState_Processing,
				LogID:     logID,
			})
		}

		if err := e.exptTurnResultRepo.BatchCreateNXRunLog(ctx, turnRunResults); err != nil {
			return err
		}

		eiec.ExistItemEvalResult.TurnResultRunLogs = gslice.ToMap(turnRunResults, func(t *entity.ExptTurnResultRunLog) (int64, *entity.ExptTurnResultRunLog) {
			return t.TurnID, t
		})
	}

	return nil
}

func (e *ExptRecordEvalModeSubmit) PostEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	return nil
}

// failRetrySelectTurnRunLogRefs 失败重试时为新建的 turn run_log 选择保留的 Target / 评估器引用：
// - Target：仅当存在 target_result_id 且对应 Target 记录状态为 Success 时保留，否则清零并重跑 Target。
// - 评估器：在 Target 已成功（或无 Target 需校验）时，仅保留状态为 Success 的 EvaluatorRecord；失败/异步等一律剔除以便重跑。
func failRetrySelectTurnRunLogRefs(
	ctx context.Context,
	spaceID int64,
	tr *entity.ExptTurnResult,
	evalTarget IEvalTargetService,
	evalRecord EvaluatorRecordService,
) (targetResultID int64, evalResults *entity.EvaluatorResults) {
	if tr == nil {
		return 0, nil
	}
	if tr.TargetResultID > 0 {
		if evalTarget == nil {
			return 0, nil
		}
		targetRec, err := evalTarget.GetRecordByID(ctx, spaceID, tr.TargetResultID)
		if err != nil || targetRec == nil || gptr.Indirect(targetRec.Status) != entity.EvalTargetRunStatusSuccess {
			return 0, nil
		}
		return tr.TargetResultID, pruneSuccessfulEvaluatorRecords(ctx, evalRecord, tr)
	}
	return 0, pruneSuccessfulEvaluatorRecords(ctx, evalRecord, tr)
}

func pruneSuccessfulEvaluatorRecords(ctx context.Context, evalRecord EvaluatorRecordService, tr *entity.ExptTurnResult) *entity.EvaluatorResults {
	if evalRecord == nil || tr.EvaluatorResults == nil {
		return nil
	}
	erids := tr.EvaluatorResults

	// 新格式 (Registered/Inline) 与老格式 (EvalVerIDToResID) 都要覆盖: 失败重试时若只识别老 map,
	// 新实验类型下会把已成功的 Registered/Inline 记录当作"缺失", 触发全部评估器重跑。
	ids := make([]int64, 0)
	seen := make(map[int64]struct{})
	addID := func(id int64) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, r := range erids.Registered {
		if r != nil {
			addID(r.RecordID)
		}
	}
	for _, r := range erids.Inline {
		if r != nil {
			addID(r.RecordID)
		}
	}
	for _, id := range erids.EvalVerIDToResID {
		addID(id)
	}
	if len(ids) == 0 {
		return nil
	}

	records, err := evalRecord.BatchGetEvaluatorRecord(ctx, ids, false, false)
	if err != nil || len(records) == 0 {
		return nil
	}
	successIDs := make(map[int64]struct{}, len(records))
	successVerID2RecID := make(map[int64]int64)
	for _, rec := range records {
		if rec == nil || rec.Status != entity.EvaluatorRunStatusSuccess {
			continue
		}
		successIDs[rec.ID] = struct{}{}
		successVerID2RecID[rec.EvaluatorVersionID] = rec.ID
	}
	if len(successIDs) == 0 {
		return nil
	}

	// 保持原格式回填, 保留 (VersionID, Alias) / InlineKey 元数据; 老数据兜底走 EvalVerIDToResID。
	out := &entity.EvaluatorResults{}
	for _, r := range erids.Registered {
		if r == nil {
			continue
		}
		if _, ok := successIDs[r.RecordID]; !ok {
			continue
		}
		out.Registered = append(out.Registered, &entity.RegisteredEvalResult{
			VersionID: r.VersionID,
			Alias:     r.Alias,
			RecordID:  r.RecordID,
		})
	}
	for _, r := range erids.Inline {
		if r == nil {
			continue
		}
		if _, ok := successIDs[r.RecordID]; !ok {
			continue
		}
		out.Inline = append(out.Inline, &entity.InlineEvalResult{
			InlineKey: r.InlineKey,
			RecordID:  r.RecordID,
		})
	}
	// 老格式: 无 Registered/Inline 信息时按 versionID→recordID 兜底
	if len(out.Registered) == 0 && len(out.Inline) == 0 && len(erids.EvalVerIDToResID) > 0 {
		out.EvalVerIDToResID = successVerID2RecID
	}
	if len(out.Registered) == 0 && len(out.Inline) == 0 && len(out.EvalVerIDToResID) == 0 {
		return nil
	}
	return out
}

type ExptRecordEvalModeFailRetry struct {
	resultSvc          ExptResultService
	exptItemResultRepo repo.IExptItemResultRepo
	exptTurnResultRepo repo.IExptTurnResultRepo
	exptStatsRepo      repo.IExptStatsRepo
	experimentRepo     repo.IExperimentRepo
	metric             metrics.ExptMetric
	idgen              idgen.IIDGenerator
	evalTargetService  IEvalTargetService
	evaluatorRecordSvc EvaluatorRecordService
}

func (e *ExptRecordEvalModeFailRetry) PreEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	if eiec.GetExistItemResultLog() != nil && len(eiec.GetExistTurnResultLogs()) > 0 {
		return nil
	}

	itemTurnResults, err := e.resultSvc.GetExptItemTurnResults(ctx, eiec.Event.ExptID, eiec.Event.EvalSetItemID, eiec.Event.SpaceID, eiec.Event.Session)
	if err != nil {
		return err
	}

	ids, err := e.idgen.GenMultiIDs(ctx, len(itemTurnResults))
	if err != nil {
		return err
	}

	turnRunLogDOs := make([]*entity.ExptTurnResultRunLog, 0, len(itemTurnResults))
	for idx, tr := range itemTurnResults {
		runLog := tr.ToRunLogDO()
		runLog.ID = ids[idx]
		runLog.Status = entity.TurnRunState_Processing
		runLog.ExptRunID = eiec.Event.ExptRunID
		runLog.ErrMsg = ""
		// 跨空间共享: Target 记录随执行落来源空间(冻结 TargetSpaceID), 失败重试选引用时须按来源空间读;
		// 用调用方空间读会得 nil → 误判 Target 非 Success → 清零 target_result_id 触发无谓重跑 Target。
		targetID, evalIDs := failRetrySelectTurnRunLogRefs(ctx, resolveLoadSpaceID(eiec.Event.SpaceID, eiec.TargetSourceSpaceID()), tr, e.evalTargetService, e.evaluatorRecordSvc)
		runLog.TargetResultID = targetID
		runLog.EvaluatorResultIds = evalIDs
		turnRunLogDOs = append(turnRunLogDOs, runLog)
	}

	if err := e.exptTurnResultRepo.BatchCreateNXRunLog(ctx, turnRunLogDOs); err != nil {
		return err
	}

	trrls := make(map[int64]*entity.ExptTurnResultRunLog, len(turnRunLogDOs))
	for _, rl := range turnRunLogDOs {
		if existed := trrls[rl.TurnID]; existed != nil && existed.UpdatedAt.After(rl.UpdatedAt) {
			continue
		}
		trrls[rl.TurnID] = rl
	}
	eiec.ExistItemEvalResult.TurnResultRunLogs = trrls

	return nil
}

func (e *ExptRecordEvalModeFailRetry) PostEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	return nil
}

type ExptRecordEvalModeRetryIgnoreResult struct {
	exptTurnResultRepo repo.IExptTurnResultRepo
	idgen              idgen.IIDGenerator
}

func (e *ExptRecordEvalModeRetryIgnoreResult) PreEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	if eiec.GetExistItemResultLog() != nil && len(eiec.GetExistTurnResultLogs()) > 0 {
		return nil
	}

	event := eiec.Event
	logID := logs.GetLogID(ctx)

	ids, err := e.idgen.GenMultiIDs(ctx, len(eiec.EvalSetItem.Turns))
	if err != nil {
		return err
	}

	turnRunLogs := make([]*entity.ExptTurnResultRunLog, 0, len(eiec.EvalSetItem.Turns))
	for idx, turn := range eiec.EvalSetItem.Turns {
		turnRunLogs = append(turnRunLogs, &entity.ExptTurnResultRunLog{
			ID:        ids[idx],
			SpaceID:   event.SpaceID,
			ExptID:    event.ExptID,
			ExptRunID: event.ExptRunID,
			ItemID:    event.EvalSetItemID,
			TurnID:    turn.ID,
			Status:    entity.TurnRunState_Processing,
			LogID:     logID,
		})
	}

	if err := e.exptTurnResultRepo.BatchCreateNXRunLog(ctx, turnRunLogs); err != nil {
		return err
	}

	eiec.ExistItemEvalResult.TurnResultRunLogs = gslice.ToMap(turnRunLogs, func(t *entity.ExptTurnResultRunLog) (int64, *entity.ExptTurnResultRunLog) {
		return t.TurnID, t
	})

	return nil
}

func (e *ExptRecordEvalModeRetryIgnoreResult) PostEval(ctx context.Context, eiec *entity.ExptItemEvalCtx) error {
	return nil
}
