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
		// 额度准入放在 Check 之后、Lock 之前：Check 已排除掉终态 run（那些消息不需要额度校验），
		// 而放在 Lock 之前可避免为一条注定要丢弃的消息（不归本进程 / 无 Scope）去抢 item 锁。
		i.HandleCentralAdmission,
		i.HandleEventLock,
		// ★ 取执行权与兑现投影必须在锁内：见 HandleCentralReservation 注释。
		i.HandleCentralReservation,
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

// HandleCentralAdmission 判定该 item 消息是否该由本进程执行，并把实验挂到 ctx 供下游复用。
//
// 与 HandleCentralReservation 拆开、夹在 item 锁两侧，是为了让两类判断各就各位：
//
//	本层（锁外）：纯读，"这条消息该不该由我处理" —— 不该处理的消息不必去抢锁
//	下层（锁内）：写，"取额度执行权 + 兑现投影" —— 必须与执行本身在同一临界区
//
// 反过来把归属校验也塞进锁内，会让一个路由错误的进程先抢到 item 锁再发现不归自己，
// 白占一段临界区；而把 ConfirmRunning 留在锁外，则两条并发消息可以各自取到执行权
// （Lua 对已 running 幂等返回 1），虽然靠幂等兜住了正确性，但会产生无谓的 Redis 写与
// 一次注定失败的 CAS。
//
// 模式判定**回查 experiment.scheduler_mode DB 列**，不看 event 上的任何标记：
// 若模式随 event 传递，字段丢失或取默认零值时，一条实际为 central 的消息会被当作 legacy 处理，
// 从而跳过额度校验、静默绕过额度执行 —— 这个方向的失败是无声的，比多查一次 DB 危险得多。
func (e *ExptItemEventEvalServiceImpl) HandleCentralAdmission(next RecordEvalEndPoint) RecordEvalEndPoint {
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

		if e.centralGuard == nil {
			// 判定为 enforce 却没有注入闸门：fail-closed。
			// 放行等于让实验在无额度约束下跑，静默且难以发现；停下来则可见。
			logs.CtxWarn(ctx, "[CentralReservation] enforce experiment without guard, drop event, expt_id: %v, item_id: %v",
				event.ExptID, event.EvalSetItemID)
			return nil
		}

		// 把已判定为"本进程纳管"的实验交给锁内那一层，省掉一次重复的 GetByID。
		// 只在 enforce 且通过全部准入检查后写入 —— 下游据此判断"要不要走额度闸"，
		// 缺失即视为不需要，与 legacy 直通的语义一致。
		event.WithCtxCentralAdmittedExpt(ctx, expt)

		return next(ctx, event)
	}
}

// HandleCentralReservation 取得该 item 的额度执行权并兑现 run log 投影。
//
// ★ 必须在 item 锁**内**：ConfirmRunning（取执行权）+ StartReservedItem（CAS 投影）+ 执行本身
// 是一个整体，锁外做前两步意味着两条并发消息可以各自"取到执行权"，之后靠 CAS 与 Lua 幂等
// 兜住正确性 —— 兜得住，但多余的 Redis 写与注定失败的 CAS 都是白做的，且把"谁在执行"
// 这个事实拆到了锁的两边，后续任何依赖它的改动都要重新推演一遍并发。
func (e *ExptItemEventEvalServiceImpl) HandleCentralReservation(next RecordEvalEndPoint) RecordEvalEndPoint {
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		expt, admitted := event.CtxCentralAdmittedExpt(ctx)
		if !admitted {
			// legacy 实验，或已在 admission 层被丢弃。不经额度闸。
			return next(ctx, event)
		}

		guard := e.centralGuard
		if guard == nil {
			// admission 已挡过一次，走到这里说明 guard 在两层之间被置空 —— 不可能发生，
			// 但保持 fail-closed 而不是让 nil 解引用把 consumer 打崩。
			logs.CtxWarn(ctx, "[CentralReservation] guard absent inside lock, drop event, expt_id: %v, item_id: %v",
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
			// reservation 不存在：迟到消息、账本已重建、或已被释放。本条消息一律不执行，
			// 但**必须先修正 run log 投影**，否则 item 会停在 Processing 占着并发槽位。
			e.requeueOrphanedItemOnReservationAbsent(ctx, event)
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
		// 现在本层在 item 锁内，同一 item 不会有并发的第二个执行者，未命中只剩"已 Processing
		// 的重复投递"与"被 repair 修正"两种，两者继续执行都由 item 锁 + 幂等写兜住。
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

			// 主表 expt_item_result 也要跟着推进到 Processing。
			//
			// run log 是执行真值，但**用户看到的是主表** —— MGetExperimentResult 走
			// expt_item_result 构造 run_state。只写 run log 会让实验详情把正在执行的 item
			// 一直显示成 Queueing（实测：5 个 item 已 Processing 14 小时，results 接口仍全部
			// 报 queueing），现象是"实验看着没动"，而它其实跑得很正常。
			//
			// legacy 的 handleToSubmits 一直是成对写这两张表的
			// （expt_run_scheduler_event_impl.go 的 UpdateItemRunLog + UpdateItemsResult），
			// 中心调度这条新派发路径漏了后者 —— 平行实现漏字段的又一例。
			//
			// 失败只告警不阻断：主表是展示投影，写不进去不影响执行与额度正确性，
			// 而这里返回错误会让已经拿到执行权的 item 被 MQ 重投一遍。
			//
			// ★ 记账与主表推进必须**同一个判据**，见下方 expt_stats 那段的论证。
			// 所以这里先读主表当前状态，只有真的发生「非 Processing → Processing」才动两处。
			prevState, prevKnown := entity.ItemRunState_Unknown, false
			if e.exptItemResultRepo != nil {
				if got, gerr := e.exptItemResultRepo.BatchGet(ctx, event.SpaceID, event.ExptID, []int64{event.EvalSetItemID}); gerr != nil {
					logs.CtxWarn(ctx, "[CentralReservation] read main table state failed, skip advancing (display only), expt_id: %v, item_id: %v: %v",
						event.ExptID, event.EvalSetItemID, gerr)
				} else if len(got) > 0 && got[0] != nil {
					prevState, prevKnown = got[0].Status, true
				}
			}

			advanced := prevKnown && prevState != entity.ItemRunState_Processing
			if advanced {
				if err := e.exptItemResultRepo.UpdateItemsResult(ctx, event.SpaceID, event.ExptID,
					[]int64{event.EvalSetItemID}, map[string]any{"status": int32(entity.ItemRunState_Processing)}); err != nil {
					logs.CtxWarn(ctx, "[CentralReservation] advance main table to Processing failed (display only, execution unaffected), expt_id: %v, item_id: %v: %v",
						event.ExptID, event.EvalSetItemID, err)
					advanced = false
				}
			}

			// expt_stats 的 Queueing→Processing 也要在此记账，否则计数单向下溢。
			//
			// 完成侧（expt_result_impl.go 的 statsCntOp）做的是「从 item 原状态减 1、往新状态加 1」，
			// 它减的是 Processing 桶。legacy 在 handleToSubmits 里派发时就把 item 计入了 Processing，
			// 两边配对；中心调度的派发只把 run log CAS 成 Queueing/reserved（**status 仍是 Queueing**），
			// 从未有人往 Processing 桶加过 —— 于是完成时减一个从未加过的计数，
			// 表现为 processing_turn_count 走负、pending_turn_count 永不下降。
			// 实测：一个 14 题 enforce 实验 fail 累到 4 时 processing = -4，pending 恒 14。
			//
			// ★ 判据是**主表真的发生了状态迁移**，不是 run log 的 CAS。
			//
			// 原先绑在 started（run log CAS）上，而主表推进是无条件的 —— 两个判据一分叉，
			// 计数行就与主表对不上。而完成侧 statsCntOp 恰恰是**按主表状态**做「-1」的
			// （`statsCntOp[itemResult.Status] -= 1`，见 expt_result_impl.go），
			// 于是它会去减一个计数行里从来没加过的桶。
			// 实测代价：PPE 一个 900 题 enforce 实验，pending 虚高 281、success 少 249，
			// 而同泳道的 legacy 实验分毫不差 —— legacy 的派发是四张表无条件一起写的。
			//
			// 用 prevState 而不是写死 Queueing：中心调度派发时主表可能停在 Queueing，
			// 也可能因为 repair/重投停在别的状态，减错桶同样会让计数行歪掉。
			//
			// 与主表同为展示投影，失败只告警不阻断：返回错误会让已取得执行权的 item 被 MQ 重投，
			// 而重投一次 Agent 执行的代价远大于一次计数偏差。**残留的偏差由调度器每拍的对账收敛**
			// （ExptSchedulerImpl.reconcileExptStats）。
			if advanced && e.exptStatsRepo != nil {
				if err := e.exptStatsRepo.ArithOperateCount(ctx, event.ExptID, event.SpaceID, &entity.StatsCntArithOp{
					OpStatusCnt: map[entity.ItemRunState]int{
						entity.ItemRunState_Processing: 1,
						prevState:                      -1,
					},
				}); err != nil {
					logs.CtxWarn(ctx, "[CentralReservation] advance expt stats to Processing failed (display only, execution unaffected), expt_id: %v, item_id: %v: %v",
						event.ExptID, event.EvalSetItemID, err)
				}
			}

			// turn 主表与 CK 加速表同为展示投影，legacy 的 handleToSubmits 派发时是与
			// run log / 主表 / stats 一起五写的，中心调度这条路径此前只写了前三项。
			//
			// 漏 CK 的后果最隐蔽：`item_run_state` 筛选打的是 etrf.status，且开启加速器时
			// 结果里的 run_state 也从 CK 读（QueryItemIDStates），于是执行期间按「运行中」筛
			// 恒为空、按「排队中」筛反而捞出正在跑的 item 并显示成排队中。完成时那次 upsert
			// 会把它纠回来，所以事后对着终态数据看不出问题。
			// 实测：PPE 一个 enforce 实验，主表已 status=1，CK 仍为 0。
			//
			// 必须在主表推进之后调：CK 行的 status 是 BuildTurnResultFilter 按主表状态回填的。
			// 与上面两项同样只告警不阻断（返回错误会让已取得执行权的 item 被 MQ 重投）。
			if advanced && e.exptTurnResultRepo != nil {
				if err := e.exptTurnResultRepo.UpdateTurnResultsWithItemIDs(ctx, event.ExptID, []int64{event.EvalSetItemID}, event.SpaceID,
					map[string]any{"status": int32(entity.TurnRunState_Processing)}); err != nil {
					logs.CtxWarn(ctx, "[CentralReservation] advance turn results to Processing failed (display only, execution unaffected), expt_id: %v, item_id: %v: %v",
						event.ExptID, event.EvalSetItemID, err)
				}
			}
			if advanced && e.resultSvc != nil {
				if err := e.resultSvc.UpsertExptTurnResultFilter(ctx, event.SpaceID, event.ExptID, []int64{event.EvalSetItemID}); err != nil {
					logs.CtxWarn(ctx, "[CentralReservation] upsert turn result filter failed (display only, execution unaffected), expt_id: %v, item_id: %v: %v",
						event.ExptID, event.EvalSetItemID, err)
				}
			}
		}

		// 执行链返回后释放额度：这是 consumer 侧的主释放点。
		//
		// 为什么放在这一层而不是 CompleteItemRun 等各个终态写库处：终态路径有四条
		// （success / fail / 不可重试前置失败 / indebt 终止），每条都手动调一次释放
		// 意味着以后任何人新增一条终态分支都可能漏掉，而漏掉的后果是额度永久泄漏
		// —— 静默、且要等额度跑满才暴露。收在中间件出口只有一处，且天然覆盖
		// panic recover 后的返回（HandleEventErr 在更外层，已把 panic 转成 err）。
		//
		// 为什么不能无条件释放：MQ 重试路径也会从这里返回。重试消息稍后会被重新投递并
		// 再次执行同一 item，若此刻释放了额度，重投的消息在 ConfirmRunning 处会因
		// reservation 不存在而被丢弃 —— item 永久停在 Processing。因此必须只在
		// item 真的进入终态时释放，靠回查 run log 投影判定，不靠猜。
		//
		// ★ 把释放凭据挂到 ctx：本层判定"未终态、保留 reservation"之后，err 会继续冒到更外层的
		// HandleEventErr，那里的 completeItemRunOnUnretriableErr 兜底把 item 落成 Fail
		// —— 那一步发生在本层**之外**，本层再没有机会释放，reservation 就永久泄漏了
		// （无 TTL 清理、无对账，见 §B1）。凭据让外层能在落 Fail 之后补一次释放，
		// 而不必把 Scope / guard 这些中心调度概念泄进 HandleEventErr 的签名。
		event.WithCtxCentralQuotaHeld(ctx, expt.SchedulerScope)

		execErr := next(ctx, event)
		e.releaseQuotaIfItemTerminal(ctx, event, expt.SchedulerScope, guard, execErr)
		return execErr
	}
}

// requeueOrphanedItemOnReservationAbsent 处理「ConfirmRunning 判定 reservation 不存在」时的投影修正。
//
// 这个分支以前只打一条 Info 就 return，是一个能让整个实验停摆的静默故障：
// consumer 已经用 StartReservedItem 把 run log 兑现成 Processing/none，之后额度因故消失
// （执行出错 → MQ 重投、自愈误判陈旧预占而释放、账本重建等），重投的消息在此被丢弃，
// item 就成了「既无额度、也无执行者」的孤儿。而 Processing 会同时被
// ScanEvalItems（算 item_concur_num 槽位）和 LoadDispatchRuntime（算并发占用）当成"在跑"，
// 于是既不派新 item、也永远等不到它完成，只能靠异步僵尸阈值（默认 3h）兜底判 Fail。
// 2026-08-28 线上实测：两个实验各 20 个 item 占满槽位 77 分钟，turn 表零记录、账本零 reservation。
//
// 三种形状分开处理，因为它们的正确动作完全不同：
//   - 终态          —— 迟到消息，item 早已跑完，什么都不能动（退回会重复执行）
//   - Processing    —— 孤儿态，退回 Queueing 让它重新被授予（连带回滚 stats 的 Processing 计数）
//   - Queueing      —— 还没兑现执行；reserved 的交给 ResetQuotaReserved 清回 none，none 的本就在队列里
//
// 全程只告警不返回错误：这条消息注定不执行，返回 error 只会让 MQ 无休止重投
// （reservation 不会因为重投而回来）。修正失败最坏退化成原来的行为 —— 等僵尸阈值。
func (e *ExptItemEventEvalServiceImpl) requeueOrphanedItemOnReservationAbsent(ctx context.Context, event *entity.ExptItemEvalEvent) {
	if e.dispatchRepo == nil {
		logs.CtxWarn(ctx, "[CentralReservation] reservation absent, discard event; dispatch repo absent, cannot repair projection, expt_id: %v, expt_run_id: %v, item_id: %v",
			event.ExptID, event.ExptRunID, event.EvalSetItemID)
		return
	}

	obs, err := e.dispatchRepo.MGetDispatchObservations(ctx, event.SpaceID, event.ExptID, event.ExptRunID, []int64{event.EvalSetItemID})
	if err != nil || len(obs) == 0 || obs[0] == nil {
		// 读不到投影就无法判断该不该退回，宁可不动 —— 盲目退回可能让已终态的 item 重跑。
		logs.CtxWarn(ctx, "[CentralReservation] reservation absent, discard event; load projection failed, projection left as-is, expt_id: %v, expt_run_id: %v, item_id: %v, err: %v",
			event.ExptID, event.ExptRunID, event.EvalSetItemID, err)
		return
	}

	state := entity.ItemRunState(obs[0].Status)
	if entity.IsItemRunFinished(state) {
		// 唯一的预期路径：item 已终态，额度早已正常释放，这就是一条迟到消息。
		logs.CtxInfo(ctx, "[CentralReservation] reservation absent on terminal item, discard event (late delivery), expt_run_id: %v, item_id: %v, state: %v",
			event.ExptRunID, event.EvalSetItemID, state)
		return
	}

	if state == entity.ItemRunState_Processing {
		requeued, rerr := e.dispatchRepo.RequeueProcessingItem(ctx, event.SpaceID, event.ExptID, event.ExptRunID, event.EvalSetItemID)
		if rerr != nil {
			logs.CtxError(ctx, "[CentralReservation] requeue orphaned processing item failed, item will occupy a concurrency slot until zombie timeout, expt_id: %v, expt_run_id: %v, item_id: %v, err: %v",
				event.ExptID, event.ExptRunID, event.EvalSetItemID, rerr)
			return
		}
		logs.CtxWarn(ctx, "[CentralReservation] orphaned item (Processing without reservation) requeued to Queueing, expt_id: %v, expt_run_id: %v, item_id: %v, requeued: %v",
			event.ExptID, event.ExptRunID, event.EvalSetItemID, requeued)

		// stats 的 Processing 桶必须同步退回，否则 processing_turn_count 只增不减。
		// 只在 CAS 真的命中时记账 —— 与 StartReservedItem 处 started 的用法同理，
		// CAS 是这里唯一的"恰好一次"信号。
		if requeued && e.exptStatsRepo != nil {
			if serr := e.exptStatsRepo.ArithOperateCount(ctx, event.ExptID, event.SpaceID, &entity.StatsCntArithOp{
				OpStatusCnt: map[entity.ItemRunState]int{
					entity.ItemRunState_Processing: -1,
					entity.ItemRunState_Queueing:   1,
				},
			}); serr != nil {
				logs.CtxWarn(ctx, "[CentralReservation] rollback expt stats to Queueing failed (display only), expt_id: %v, item_id: %v: %v",
					event.ExptID, event.EvalSetItemID, serr)
			}
		}

		// 主表同为展示投影，跟着退回，避免详情页把已回队列的 item 一直显示成执行中。
		//
		// ★ 必须与 stats 一样绑定 requeued：主表 status 不是纯展示字段，而是 stats 的锚点 ——
		// 完成侧 statsCntOp 读 items_result.Status 做「-1」（expt_result_impl.go），
		// 若 CAS 未命中（并发 zombie / sandbox sweep 抢先落终态）却把主表改成 Queueing，
		// stats 上那笔 Processing 就再也减不掉，processing_turn_count 永远归不了零。
		// 同仓 expt_run_scheduler_event_impl.go 的 zombie 路径已就此写过警示、
		// c4a6a953 也已就此修过一次，这里不能再踩。
		if requeued && e.exptItemResultRepo != nil {
			if uerr := e.exptItemResultRepo.UpdateItemsResult(ctx, event.SpaceID, event.ExptID,
				[]int64{event.EvalSetItemID}, map[string]any{"status": int32(entity.ItemRunState_Queueing)}); uerr != nil {
				logs.CtxWarn(ctx, "[CentralReservation] rollback main table to Queueing failed (display only), expt_id: %v, item_id: %v: %v",
					event.ExptID, event.EvalSetItemID, uerr)
			}
		}

		// turn 主表与 CK 加速表也要跟着退回：派发侧推进的是这五项，回退只做前三项就会留下
		// 「item 已回队列、turn 与 CK 仍显示执行中」的错位。它不会自愈 —— 实验若以 Failed
		// 收口，CompleteExpt 的 default 分支只销毁沙箱、不动 turn 状态。
		// 同样绑定 requeued（判据与 stats、主表一致），失败只告警。
		if requeued && e.exptTurnResultRepo != nil {
			if uerr := e.exptTurnResultRepo.UpdateTurnResultsWithItemIDs(ctx, event.ExptID, []int64{event.EvalSetItemID}, event.SpaceID,
				map[string]any{"status": int32(entity.TurnRunState_Queueing)}); uerr != nil {
				logs.CtxWarn(ctx, "[CentralReservation] rollback turn results to Queueing failed (display only), expt_id: %v, item_id: %v: %v",
					event.ExptID, event.EvalSetItemID, uerr)
			}
		}
		if requeued && e.resultSvc != nil {
			if uerr := e.resultSvc.UpsertExptTurnResultFilter(ctx, event.SpaceID, event.ExptID, []int64{event.EvalSetItemID}); uerr != nil {
				logs.CtxWarn(ctx, "[CentralReservation] rollback turn result filter failed (display only), expt_id: %v, item_id: %v: %v",
					event.ExptID, event.EvalSetItemID, uerr)
			}
		}
		return
	}

	// 剩下只有 Queueing：reserved 的清回 none 让它重新可授予；none 的本就在候选里，无需动作。
	if obs[0].QuotaReservationState.IsQuotaReserved() {
		reset, rerr := e.dispatchRepo.ResetQuotaReserved(ctx, event.SpaceID, event.ExptID, event.ExptRunID, []int64{event.EvalSetItemID})
		if rerr != nil {
			logs.CtxError(ctx, "[CentralReservation] reset stale reserved projection failed, expt_id: %v, expt_run_id: %v, item_id: %v, err: %v",
				event.ExptID, event.ExptRunID, event.EvalSetItemID, rerr)
			return
		}
		logs.CtxWarn(ctx, "[CentralReservation] stale Queueing/reserved projection reset to none, expt_id: %v, expt_run_id: %v, item_id: %v, reset: %v",
			event.ExptID, event.ExptRunID, event.EvalSetItemID, len(reset))
		return
	}

	logs.CtxWarn(ctx, "[CentralReservation] reservation absent while item still Queueing/none, nothing to repair, expt_id: %v, expt_run_id: %v, item_id: %v",
		event.ExptID, event.ExptRunID, event.EvalSetItemID)
}

// releaseQuotaIfItemTerminal 在 item 确已进入终态时释放其额度预占。
//
// 判定依据是 run log 的实际状态，而不是 execErr 是否为 nil：
//   - execErr == nil 未必终态（asyncAbort 场景下 item 仍在异步执行中）；
//   - execErr != nil 未必非终态（不可重试的前置失败已被兜底落成 Fail/Logged）。
//
// 用状态判定才能两个方向都不出错 —— 少释放会泄漏额度，多释放会让重投消息被丢弃、
// item 卡死 Processing。
//
// 全程 best-effort：释放失败只告警 —— 让终态收口因为额度模块失败而报错，会把
// "额度泄漏"升级成"实验不收敛"，后者严重得多。
//
// ⚠️ 此前这里写着"额度对账（spec §3.11）是最终防线"，那是**规划而非现状**：
// 对账目前只有调度器每拍在 Reserve 前跑的一小段（只清"投影 none + 账本 reserved"、
// 且只覆盖当前 LatestRunID）。本函数释放失败后的那条 reservation **没有任何兜底**，
// 会一直占着额度。所以这里的 Warn 日志是唯一线索，排查额度异常时必须查它。
func (e *ExptItemEventEvalServiceImpl) releaseQuotaIfItemTerminal(
	ctx context.Context,
	event *entity.ExptItemEvalEvent,
	schedulerScope string,
	guard component.ICentralReservationGuard,
	execErr error,
) {
	if guard == nil || e.dispatchRepo == nil {
		return
	}

	// 用独立超时的 ctx：主 ctx 可能已因执行失败被取消，而释放必须尽力完成。
	relCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), exptRunLogPersistTimeout)
	defer cancel()

	obs, err := e.dispatchRepo.MGetDispatchObservations(relCtx, event.SpaceID, event.ExptID, event.ExptRunID,
		[]int64{event.EvalSetItemID})
	if err != nil {
		logs.CtxWarn(relCtx, "[CentralReservation] load dispatch observation for release fail, expt_run_id: %v, item_id: %v, err: %v",
			event.ExptRunID, event.EvalSetItemID, err)
		return
	}
	if len(obs) == 0 {
		// run log 查不到：item 记录已被清理（重跑清表等）。此时 reservation 也该走，
		// 交由对账处理 —— 这里贸然释放可能释放的是新一轮 run 的额度。
		return
	}

	if !entity.IsItemRunFinished(entity.ItemRunState(obs[0].Status)) {
		// 仍在执行 / 等待重投：保留 reservation。
		return
	}

	reason := terminalReleaseReason(entity.ItemRunState(obs[0].Status), execErr)
	if err := guard.Release(relCtx, schedulerScope, event.ExptRunID, event.EvalSetItemID, reason); err != nil {
		logs.CtxWarn(relCtx, "[CentralReservation] release quota fail, expt_run_id: %v, item_id: %v, err: %v",
			event.ExptRunID, event.EvalSetItemID, err)
		return
	}
	logs.CtxInfo(relCtx, "[CentralReservation] quota released on item terminal, scope: %v, expt_run_id: %v, item_id: %v, reason: %v",
		schedulerScope, event.ExptRunID, event.EvalSetItemID, reason)
}

// terminalReleaseReason 由**回查到的 run log 终态**推导释放原因，而不是由 execErr 推导。
//
// ★ 为什么不能用 execErr：`CompleteItemRun` 在写完 status=Fail + err_msg 之后，只有
// in-debt 错误（evalErrNeedTerminateExpt）才 return evalErr，普通 item 失败一律 return nil；
// 而 `Eval` 只透传它的返回值。于是普通失败对本层**完全不可见**，execErr 恒为 nil。
//
// 实测代价：BOE 129 条释放日志**全部**写着 `item success`，其中 104 个 item 的 DB 状态
// 是 Fail —— 失败被伪装成成功，靠日志根本发现不了实验在大面积失败，
// 而"额度都被谁吃了"这类排查会得到完全错误的图景。
//
// 用 status 推导后，标签与 DB 事实一致。execErr 仍然带上（非 nil 时附错误文本）：
// 它在 in-debt 那条路径上是有信息量的，能区分"失败"与"失败且触发了实验终止"。
//
// 注意这**只修标签**，不改变释放行为（终态判定仍是 IsItemRunFinished），也没有让
// HandleEventErr 的错误分支重新执行 —— 后者需要改 CompleteItemRun 的返回契约，
// 影响面大得多（见 AUDIT-FINDINGS-2026-08-21.md P1-1 方向 2）。
func terminalReleaseReason(status entity.ItemRunState, execErr error) string {
	var reason string
	switch status {
	case entity.ItemRunState_Success:
		reason = "item success"
	case entity.ItemRunState_Fail:
		reason = "item failed"
	case entity.ItemRunState_Terminal:
		reason = "item terminated"
	default:
		// 走到这里说明 IsItemRunFinished 的终态集合扩展了但本函数没跟上。
		// 回落时带上原始状态码，便于反查是哪个新状态。
		reason = fmt.Sprintf("item terminal(status=%d)", int32(status))
	}
	if execErr != nil {
		// execErr 非 nil 说明是 in-debt 一类会向上抛的错误，附上文本便于定位。
		reason += ": " + execErr.Error()
	}
	return reason
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

			// ★ 欠费终止同样要归还额度：整个实验已 Terminated，这条 item 的 run log 却停在
			// Processing（本分支不落 item 终态），内层额度闸按状态判定时又已经返回 —— 不补这一次
			// 释放，该 item 的预占就跟着终止的实验一起永久留在账本里。
			// 每条 item 消息都会各自走到这里（IsInDebt 是空间级配置），所以按 item 粒度释放即可覆盖全 run。
			e.releaseCentralQuotaOutsideGate(ctx, event, nextErr, "expt terminated on indebt")

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

		// ★ 顺序不能反：必须先落 Fail 再释放。
		// 落 Fail 让 item 不再计入并发占用，此时归还额度才是守恒的；反过来先释放，
		// 会出现"额度已归还但 item 仍算 Processing"的窗口，下一拍调度器按虚高的占用少派 item。
		e.releaseCentralQuotaOutsideGate(ctx, event, nextErr, "item unretriable pre-exec failure")

		return nil
	}
}

// releaseCentralQuotaOutsideGate 在额度闸之外的终态路径上释放中心调度额度预占。
//
// 补的是 HandleCentralReservation 够不着的两条路径 —— 它们都在额度闸的**外层**把
// item / 实验判成终态，而额度闸按 run log 状态判定时那些状态还没写下去：
//
//	① 不可重试的前置失败：BuildExptRecordEvalCtx 等阶段失败，item 由
//	   completeItemRunOnUnretriableErr 兜底落 Fail
//	② 欠费终止：整个实验落 Terminated，item run log 不动
//
// 没有这一步，这两条路径上的额度**永久泄漏**：既无 reservation TTL 清理，也还没有对账（§B1），
// 现象是"额度慢慢跑满后整个 Scope 再也调度不动"，且看起来像上限配小了。
//
// 靠 ctx 凭据而非回查 DB 判定是否需要释放：凭据由额度闸在取得执行权后写入，
// 天然排除掉 legacy 实验与在取得执行权之前就被丢弃的消息，也免去一次 experiment 查询。
//
// 与额度闸内的 releaseQuotaIfItemTerminal 重复释放是安全的：Release 落到 Redis 是
// HDEL + used 递减，同一 field 二次删除不生效，不会把 used 扣成负数。
//
// best-effort：释放失败只告警。让"额度归还失败"阻断终态收口会把额度泄漏升级成实验不收敛。
func (e *ExptItemEventEvalServiceImpl) releaseCentralQuotaOutsideGate(ctx context.Context, event *entity.ExptItemEvalEvent, evalErr error, reasonPrefix string) {
	if event == nil || evalErr == nil || e.centralGuard == nil {
		return
	}

	schedulerScope, held := event.CtxCentralQuotaHeldScope(ctx)
	if !held {
		// 没持有预占：legacy 实验，或消息在取得执行权之前就被丢弃/失败了。
		return
	}

	// 独立超时的 ctx：主 ctx 可能已因执行失败被取消，而释放必须尽力完成。
	relCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), exptRunLogPersistTimeout)
	defer cancel()

	reason := reasonPrefix + ": " + evalErr.Error()
	if err := e.centralGuard.Release(relCtx, schedulerScope, event.ExptRunID, event.EvalSetItemID, reason); err != nil {
		logs.CtxWarn(relCtx, "[CentralReservation] release quota outside gate fail, scope: %v, expt_run_id: %v, item_id: %v, err: %v",
			schedulerScope, event.ExptRunID, event.EvalSetItemID, err)
		return
	}
	logs.CtxInfo(relCtx, "[CentralReservation] quota released outside gate, scope: %v, expt_run_id: %v, item_id: %v, reason: %v",
		schedulerScope, event.ExptRunID, event.EvalSetItemID, reason)
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
