// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// CentralSchedulerConfigProvider 提供中心调度配置（商业版从 TCC/静态配置读取；开源侧可注入假实现）。
type CentralSchedulerConfigProvider interface {
	GetCentralSchedulerConfig(ctx context.Context) *entity.CentralSchedulerConfig
}

// CentralEnvScopeProvider 提供中心账本的环境所有权命名空间。
//
// 第二个返回值为 false 表示当前进程无法确定自己的 Env/Lane（如缺少 TCE_PSM/K_ENV），
// 整条链路必须按失败处理，不能回落到 unknown/default 去借用别人的数据。
type CentralEnvScopeProvider func(ctx context.Context) (entity.CentralEnvScope, bool)

// DefaultCentralEnvScopeProvider 从进程环境推导归属：Env 取 TCE_PSM，Lane 取 K_ENV / x_tt_env。
// 生产环境没有泳道变量时使用同一 Env 下的显式 `prod` 泳道，不跨环境借用数据。
func DefaultCentralEnvScopeProvider(ctx context.Context) (entity.CentralEnvScope, bool) {
	lane := strings.TrimSpace(os.Getenv("K_ENV"))
	if lane == "" {
		lane = strings.TrimSpace(os.Getenv("x_tt_env"))
	}
	envName := strings.TrimSpace(os.Getenv("TCE_PSM"))
	if envName == "" {
		return entity.CentralEnvScope{}, false
	}
	if lane == "" {
		lane = "prod"
	}
	scope := entity.CentralEnvScope{Env: envName, Lane: lane}
	if !scope.IsValid() {
		return entity.CentralEnvScope{}, false
	}
	return scope, true
}

// CentralDispatchGate 是“中心调度是否启用 + 如何预占/释放”的门面。
//
// 旧模式（legacy）下所有派发方法都保持无副作用，保证旧实验/旧调用方完全不受影响；
// 释放类方法只做幂等清理，不受配置开关影响，避免停用后额度悬挂。
type CentralDispatchGate interface {
	// Enabled 判断该 run 当前是否走中心调度 enforce 模式（优先读冻结的 run 状态）。
	Enabled(ctx context.Context, expt *entity.Experiment, runID int64) bool
	// Observe 冻结/读取 run 的 mode/priority/targetConcurrency，并登记 pending/active。
	Observe(ctx context.Context, expt *entity.Experiment, event *entity.ExptScheduleEvent) bool
	// ReserveItems 为待派发 item 原子预占多资源；返回真正允许派发的 item 与已持有的 lease。
	ReserveItems(ctx context.Context, expt *entity.Experiment, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem) ([]*entity.ExptEvalItem, []*entity.CentralLease, error)
	// ConfirmDispatched 派发明确成功后置 dispatched，并把 run 从 pending 迁到 active。
	ConfirmDispatched(ctx context.Context, leases []*entity.CentralLease, sessionUserID string) error
	// MarkPendingConfirm 派发结果不明时进入待确认，由 deadline sweeper 收敛。
	MarkPendingConfirm(ctx context.Context, leases []*entity.CentralLease) error
	// ReleaseLeases 释放一次派发调用中已获得的 lease（明确失败补偿）。
	ReleaseLeases(ctx context.Context, leases []*entity.CentralLease) error
	// ReleaseItems 按 item 维度幂等释放（完成/失败/取消/僵尸/重试）。
	ReleaseItems(ctx context.Context, exptID, runID int64, itemIDs []int64) error
	// ReleaseExpt 释放整个实验的预占（删除/实验级取消）。
	ReleaseExpt(ctx context.Context, exptID int64) error
	// UnregisterRun 注销一次运行（实验终态/取消/重试收敛后）。
	UnregisterRun(ctx context.Context, exptID, runID int64) error
	// Sweep 收敛到期的 pending_confirm 预占。
	Sweep(ctx context.Context) (int, error)
}

type centralDispatchGateImpl struct {
	repo           repo.ICentralSchedulingRepo
	configProvider CentralSchedulerConfigProvider
	scopeProvider  CentralEnvScopeProvider
}

var _ CentralDispatchGate = (*centralDispatchGateImpl)(nil)

func NewCentralDispatchGate(r repo.ICentralSchedulingRepo, configProvider CentralSchedulerConfigProvider, scopeProvider CentralEnvScopeProvider) CentralDispatchGate {
	if scopeProvider == nil {
		scopeProvider = DefaultCentralEnvScopeProvider
	}
	return &centralDispatchGateImpl{repo: r, configProvider: configProvider, scopeProvider: scopeProvider}
}

func (g *centralDispatchGateImpl) config(ctx context.Context) *entity.CentralSchedulerConfig {
	if g == nil || g.configProvider == nil {
		return nil
	}
	return g.configProvider.GetCentralSchedulerConfig(ctx)
}

func (g *centralDispatchGateImpl) scope(ctx context.Context) (entity.CentralEnvScope, bool) {
	if g == nil || g.scopeProvider == nil {
		return DefaultCentralEnvScopeProvider(ctx)
	}
	return g.scopeProvider(ctx)
}

func (g *centralDispatchGateImpl) Enabled(ctx context.Context, expt *entity.Experiment, runID int64) bool {
	if g == nil || g.repo == nil || expt == nil {
		return false
	}
	cfg := g.config(ctx)
	if cfg == nil || !cfg.Enabled {
		return false
	}
	scope, ok := g.scope(ctx)
	if !ok {
		logs.CtxWarn(ctx, "[central] env scope unavailable, fail closed")
		return false
	}
	if state, found, err := g.repo.GetRunState(ctx, scope, expt.ID, runID); err == nil && found {
		return state.Mode == entity.CentralSchedulerModeEnforce
	} else if err != nil {
		logs.CtxWarn(ctx, "[central] read frozen run mode failed, fail closed, expt_id=%d, run_id=%d, err=%v", expt.ID, runID, err)
		return false
	}
	return entity.ResolveCentralSchedulerMode(cfg, expt.SpaceID, extOf(expt)) == entity.CentralSchedulerModeEnforce
}

func (g *centralDispatchGateImpl) Observe(ctx context.Context, expt *entity.Experiment, event *entity.ExptScheduleEvent) bool {
	if g == nil || g.repo == nil || expt == nil || event == nil {
		return false
	}
	cfg := g.config(ctx)
	if cfg == nil || !cfg.Enabled {
		return false
	}
	scope, ok := g.scope(ctx)
	if !ok {
		logs.CtxWarn(ctx, "[central] env scope unavailable, skip observe, expt_id=%d", expt.ID)
		return false
	}

	desired := entity.ResolveCentralSchedulerMode(cfg, expt.SpaceID, extOf(expt))
	state, found, err := g.repo.GetRunState(ctx, scope, expt.ID, event.ExptRunID)
	if err != nil {
		logs.CtxWarn(ctx, "[central] read frozen run state failed, fail closed, expt_id=%d, run_id=%d, err=%v", expt.ID, event.ExptRunID, err)
		return false
	}
	if !found {
		// legacy 实验不写任何中心状态，保持“起步零变化”；只有首次进入 enforce 才冻结。
		if desired != entity.CentralSchedulerModeEnforce {
			return false
		}
		priority := entity.ResolveCentralPriority(extOf(expt), entity.DefaultCentralPriority)
		targetConcurrency, _ := cfg.GetTargetConcurrency(expt.SpaceID, extOf(expt))
		frozen := &entity.CentralRunState{
			SpaceID:           expt.SpaceID,
			ExptID:            expt.ID,
			ExptRunID:         event.ExptRunID,
			Mode:              desired,
			Priority:          priority,
			TargetConcurrency: targetConcurrency,
			Status:            entity.CentralRunStatusPending,
		}
		state, _, err = g.repo.EnsureRunState(ctx, scope, frozen, 7*24*time.Hour)
		if err != nil {
			logs.CtxWarn(ctx, "[central] freeze run state failed, fail closed, expt_id=%d, run_id=%d, err=%v", expt.ID, event.ExptRunID, err)
			return false
		}
		logs.CtxInfo(ctx, "[central] run mode frozen, expt_id=%d, run_id=%d, mode=%s, priority=%d, seq=%d, target_concurrency=%d",
			expt.ID, event.ExptRunID, state.Mode, state.Priority, state.Seq, state.TargetConcurrency)
	}

	// 模式冻结：切换必须消费实验负责人提工单、审批后写入白名单的一次性授权。
	leftEnforce := false
	if state.Mode != desired {
		auth := cfg.GetModeSwitchAuthorization(expt.ID)
		if auth == nil || strings.TrimSpace(auth.TicketID) == "" || entity.ParseCentralSchedulerMode(auth.ToMode) != desired {
			logs.CtxWarn(ctx, "[central] mode frozen, ignored unauthorized switch, expt_id=%d, run_id=%d, frozen=%s, requested=%s",
				expt.ID, event.ExptRunID, state.Mode, desired)
		} else {
			switched, switchErr := g.repo.SwitchRunMode(ctx, scope, expt.ID, event.ExptRunID, desired, auth)
			if switchErr != nil {
				logs.CtxWarn(ctx, "[central] switch mode failed, keep frozen mode, expt_id=%d, err=%v", expt.ID, switchErr)
			} else if switched {
				logs.CtxInfo(ctx, "[central] mode switched with approval, expt_id=%d, run_id=%d, from=%s, to=%s, ticket=%s, operator=%s, reason=%s",
					expt.ID, event.ExptRunID, state.Mode, desired, auth.TicketID, auth.Operator, auth.Reason)
				leftEnforce = state.Mode == entity.CentralSchedulerModeEnforce && desired != entity.CentralSchedulerModeEnforce
				state.Mode = desired
			} else {
				logs.CtxWarn(ctx, "[central] mode switch authorization already consumed, keep frozen mode, expt_id=%d", expt.ID)
			}
		}
	}

	if state.Mode != entity.CentralSchedulerModeEnforce {
		if leftEnforce {
			// 经授权切回 legacy：注销中心维护并释放全部在途预占，旧链路从干净状态接管。
			if unregErr := g.repo.UnregisterActive(ctx, scope, expt.ID, event.ExptRunID); unregErr != nil {
				logs.CtxWarn(ctx, "[central] unregister after mode switch failed, expt_id=%d, err=%v", expt.ID, unregErr)
			}
			if released, relErr := g.repo.ReleaseRun(ctx, scope, expt.ID, event.ExptRunID); relErr != nil {
				logs.CtxWarn(ctx, "[central] release run after mode switch failed, expt_id=%d, err=%v", expt.ID, relErr)
			} else if released > 0 {
				logs.CtxInfo(ctx, "[central] released %d leases after switching to legacy, expt_id=%d, run_id=%d", released, expt.ID, event.ExptRunID)
			}
		}
		return false
	}

	run := &entity.CentralActiveRun{
		SpaceID:      expt.SpaceID,
		ExptID:       expt.ID,
		ExptRunID:    event.ExptRunID,
		Priority:     state.Priority,
		Seq:          state.Seq,
		RegisteredAt: state.RegisteredAt,
	}
	if event.Session != nil {
		run.SessionUserID = event.Session.UserID
	}
	if state.Status == entity.CentralRunStatusActive {
		if err := g.repo.RegisterActive(ctx, scope, run, 24*time.Hour); err != nil {
			logs.CtxWarn(ctx, "[central] refresh active run failed, expt_id=%d, run_id=%d, err=%v", expt.ID, event.ExptRunID, err)
		}
	} else if err := g.repo.RegisterPending(ctx, scope, run, 24*time.Hour); err != nil {
		logs.CtxWarn(ctx, "[central] register pending run failed, expt_id=%d, run_id=%d, err=%v", expt.ID, event.ExptRunID, err)
	}
	return true
}

func (g *centralDispatchGateImpl) ReserveItems(ctx context.Context, expt *entity.Experiment, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem) ([]*entity.ExptEvalItem, []*entity.CentralLease, error) {
	if g == nil || g.repo == nil || !g.Enabled(ctx, expt, event.ExptRunID) || len(items) == 0 {
		return items, nil, nil
	}
	cfg := g.config(ctx)
	scope, ok := g.scope(ctx)
	if !ok {
		return nil, nil, errorx.New("central: env scope unavailable, refuse to reserve")
	}
	recovered, recoveredErr := g.repo.IsDegradedRecovered(ctx, scope)
	if recoveredErr != nil {
		// 读不到恢复标记时按未恢复处理，宁可少放不可静默放量。
		logs.CtxWarn(ctx, "[central] read degrade recovered failed, treat as degraded, expt_id=%d, err=%v", event.ExptID, recoveredErr)
		recovered = false
	}
	capacities := cfg.GetEffectiveCapacitiesWithRecovery(recovered)
	ttl := cfg.GetLeaseTTL()
	claims := buildCentralClaims(expt)

	// 某类资源 disabled/未配置：相关实验整体不派发，不允许只占一部分资源。
	for _, claim := range claims {
		if capacities[claim.Kind] <= 0 {
			logs.CtxInfo(ctx, "[central] resource disabled or not configured, defer dispatch, expt_id=%d, run_id=%d, kind=%s",
				event.ExptID, event.ExptRunID, claim.Kind)
			return nil, nil, nil
		}
	}

	state, found, err := g.repo.GetRunState(ctx, scope, event.ExptID, event.ExptRunID)
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return nil, nil, nil
	}

	limit := len(items)
	if state.TargetConcurrency > 0 {
		inFlight, countErr := g.repo.CountRunLeases(ctx, scope, event.ExptID, event.ExptRunID)
		if countErr != nil {
			return nil, nil, countErr
		}
		gap := state.TargetConcurrency - inFlight
		if gap <= 0 {
			logs.CtxInfo(ctx, "[central] target concurrency reached, defer all, expt_id=%d, run_id=%d, target=%d, in_flight=%d",
				event.ExptID, event.ExptRunID, state.TargetConcurrency, inFlight)
			return nil, nil, nil
		}
		if gap < limit {
			limit = gap
		}
	}

	existingLeases, err := g.repo.ListLeases(ctx, scope)
	if err != nil {
		return nil, nil, err
	}
	held := make(map[int64][]entity.CentralLeaseState, len(existingLeases))
	for _, lease := range existingLeases {
		if lease == nil || lease.ExptID != event.ExptID || lease.ExptRunID != event.ExptRunID {
			continue
		}
		held[lease.ItemID] = append(held[lease.ItemID], lease.State)
	}

	allowed := make([]*entity.ExptEvalItem, 0, limit)
	leases := make([]*entity.CentralLease, 0, limit)
	for _, item := range items {
		if item == nil {
			continue
		}
		if len(allowed) >= limit {
			break
		}
		skip := false
		// 已确认派发或待确认的 item 绝不能重复预占/重复派发，也不在本 tick 推进状态；
		// 待确认由 deadline sweeper 收敛，迟到回调由释放幂等兜底。
		for _, stateOfItem := range held[item.ItemID] {
			if stateOfItem == entity.CentralLeaseStateDispatched || stateOfItem == entity.CentralLeaseStatePendingConfirm {
				logs.CtxInfo(ctx, "[central] item already in flight, defer duplicate dispatch, expt_id=%d, run_id=%d, item_id=%d, state=%s",
					event.ExptID, event.ExptRunID, item.ItemID, stateOfItem)
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		// 重试：旧 attempt 必须先释放，再用新的唯一 attempt 预占。
		if _, relErr := g.repo.ReleaseItems(ctx, scope, event.ExptID, event.ExptRunID, []int64{item.ItemID}); relErr != nil {
			return nil, nil, relErr
		}
		attempt, attemptErr := g.repo.NextAttempt(ctx, scope, event.ExptID, event.ExptRunID, item.ItemID, 24*time.Hour)
		if attemptErr != nil {
			return nil, nil, attemptErr
		}
		now := time.Now()
		lease := &entity.CentralLease{
			LeaseID:    entity.MakeCentralLeaseIDWithAttempt(event.ExptID, event.ExptRunID, item.ItemID, attempt),
			SpaceID:    event.SpaceID,
			ExptID:     event.ExptID,
			ExptRunID:  event.ExptRunID,
			ItemID:     item.ItemID,
			Attempt:    attempt,
			State:      entity.CentralLeaseStateReserved,
			Claims:     claims,
			AcquiredAt: now.Unix(),
			ExpireAt:   now.Add(ttl).Unix(),
		}
		ok, acquireErr := g.repo.Acquire(ctx, scope, capacities, lease, ttl)
		if acquireErr != nil {
			if relErr := g.ReleaseLeases(ctx, leases); relErr != nil {
				logs.CtxWarn(ctx, "[central] rollback leases failed, expt_id=%d, run_id=%d, err=%v", event.ExptID, event.ExptRunID, relErr)
			}
			return nil, nil, acquireErr
		}
		if !ok {
			logs.CtxInfo(ctx, "[central] resource exhausted, defer item, expt_id=%d, run_id=%d, item_id=%d", event.ExptID, event.ExptRunID, item.ItemID)
			break
		}
		allowed = append(allowed, item)
		leases = append(leases, lease)
	}
	return allowed, leases, nil
}

func (g *centralDispatchGateImpl) ConfirmDispatched(ctx context.Context, leases []*entity.CentralLease, sessionUserID string) error {
	if g == nil || g.repo == nil || len(leases) == 0 {
		return nil
	}
	scope, ok := g.scope(ctx)
	if !ok {
		return errorx.New("central: env scope unavailable, refuse to confirm dispatch")
	}
	var firstErr error
	for _, lease := range leases {
		if lease == nil {
			continue
		}
		if _, err := g.repo.MarkDispatched(ctx, scope, lease.LeaseID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	// 首个 item 确认派发成功即把 run 从 pending 初始化迁到 active 维护。
	first := firstLease(leases)
	if first != nil {
		if err := g.repo.ActivateRun(ctx, scope, first.ExptID, first.ExptRunID, sessionUserID, 24*time.Hour); err != nil && firstErr == nil {
			// 迁移失败不回滚已确认的派发；下一 tick Observe 会按 run 状态补登记。
			logs.CtxWarn(ctx, "[central] activate run failed, expt_id=%d, run_id=%d, err=%v", first.ExptID, first.ExptRunID, err)
		}
	}
	return firstErr
}

func (g *centralDispatchGateImpl) MarkPendingConfirm(ctx context.Context, leases []*entity.CentralLease) error {
	if g == nil || g.repo == nil || len(leases) == 0 {
		return nil
	}
	scope, ok := g.scope(ctx)
	if !ok {
		return errorx.New("central: env scope unavailable, refuse to mark pending")
	}
	cfg := g.config(ctx)
	deadline := time.Now().Add(2 * time.Minute)
	if cfg != nil {
		deadline = time.Now().Add(cfg.GetDispatchDeadline())
	}
	var firstErr error
	for _, lease := range leases {
		if lease == nil {
			continue
		}
		if _, err := g.repo.MarkPendingConfirm(ctx, scope, lease.LeaseID, deadline.Unix()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (g *centralDispatchGateImpl) ReleaseLeases(ctx context.Context, leases []*entity.CentralLease) error {
	if g == nil || g.repo == nil || len(leases) == 0 {
		return nil
	}
	scope, ok := g.scope(ctx)
	if !ok {
		return errorx.New("central: env scope unavailable, refuse to release leases")
	}
	var firstErr error
	for _, lease := range leases {
		if lease == nil {
			continue
		}
		if _, err := g.repo.Release(ctx, scope, lease.LeaseID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (g *centralDispatchGateImpl) ReleaseItems(ctx context.Context, exptID, runID int64, itemIDs []int64) error {
	if g == nil || g.repo == nil || len(itemIDs) == 0 {
		return nil
	}
	scope, ok := g.scope(ctx)
	if !ok {
		return errorx.New("central: env scope unavailable, refuse to release items")
	}
	_, err := g.repo.ReleaseItems(ctx, scope, exptID, runID, itemIDs)
	return err
}

func (g *centralDispatchGateImpl) ReleaseExpt(ctx context.Context, exptID int64) error {
	if g == nil || g.repo == nil || exptID <= 0 {
		return nil
	}
	scope, ok := g.scope(ctx)
	if !ok {
		return errorx.New("central: env scope unavailable, refuse to release expt")
	}
	released, err := g.repo.ReleaseExpt(ctx, scope, exptID)
	if err != nil {
		return err
	}
	if released > 0 {
		logs.CtxInfo(ctx, "[central] released %d leases on expt release, expt_id=%d", released, exptID)
	}
	return nil
}

func (g *centralDispatchGateImpl) UnregisterRun(ctx context.Context, exptID, runID int64) error {
	if g == nil || g.repo == nil {
		return nil
	}
	scope, ok := g.scope(ctx)
	if !ok {
		return errorx.New("central: env scope unavailable, refuse to unregister run")
	}
	if err := g.repo.UnregisterActive(ctx, scope, exptID, runID); err != nil {
		return err
	}
	released, err := g.repo.ReleaseRun(ctx, scope, exptID, runID)
	if err != nil {
		return err
	}
	if released > 0 {
		logs.CtxInfo(ctx, "[central] released %d leases on run unregister, expt_id=%d, run_id=%d", released, exptID, runID)
	}
	return nil
}

func (g *centralDispatchGateImpl) Sweep(ctx context.Context) (int, error) {
	if g == nil || g.repo == nil {
		return 0, nil
	}
	scope, ok := g.scope(ctx)
	if !ok {
		return 0, errorx.New("central: env scope unavailable, refuse to sweep")
	}
	return g.repo.SweepPendingConfirms(ctx, scope, time.Now().Unix(), 7*24*time.Hour)
}

// isCentralDispatchUnknownError 区分“明确失败”与“结果不明”。
// 结果不明时既不能当作成功，也不能直接释放（可能已经投递），必须进入 pending_confirm。
func isCentralDispatchUnknownError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, token := range []string{
		"timeout", "timed out", "deadline exceeded", "context canceled",
		"connection reset", "broken pipe", "eof", "unavailable", "temporarily",
	} {
		if strings.Contains(msg, token) {
			return true
		}
	}
	return false
}

func firstLease(leases []*entity.CentralLease) *entity.CentralLease {
	for _, lease := range leases {
		if lease != nil {
			return lease
		}
	}
	return nil
}

// buildCentralClaims 计算一次 item 派发需要预占的资源：
//   - item_slot：每 item 1；
//   - evaluator_slot：按实验评估器数量，至少 1；
//   - sandbox_execute：SandboxAgent 评测对象每 item 1。
func buildCentralClaims(expt *entity.Experiment) []entity.CentralResourceClaim {
	if expt == nil {
		return []entity.CentralResourceClaim{{Kind: entity.CentralResourceItemSlot, Amount: 1}}
	}
	claims := []entity.CentralResourceClaim{
		{Kind: entity.CentralResourceItemSlot, Amount: 1},
	}
	evaluatorCnt := len(expt.Evaluators)
	if evaluatorCnt <= 0 {
		evaluatorCnt = 1
	}
	claims = append(claims, entity.CentralResourceClaim{Kind: entity.CentralResourceEvaluatorSlot, Amount: int64(evaluatorCnt)})
	if expt.TargetType == entity.EvalTargetTypeSandboxAgent {
		claims = append(claims, entity.CentralResourceClaim{Kind: entity.CentralResourceSandboxExecute, Amount: 1})
	}
	return claims
}

func extOf(expt *entity.Experiment) map[string]string {
	if expt == nil || expt.EvalConf == nil {
		return nil
	}
	return expt.EvalConf.Ext
}

// ---------- 全局桥接：商业版 main 在 wire 之外注入，不改生成代码 ----------

var globalCentralGate atomic.Value

// SetCentralDispatchGate 由部署侧（商业版 main）调用，把中心调度门面注入 OSS 调度链。
func SetCentralDispatchGate(gate CentralDispatchGate) {
	if gate == nil {
		globalCentralGate.Store((*CentralDispatchGate)(nil))
		return
	}
	globalCentralGate.Store(&gate)
}

func getGlobalCentralDispatchGate() CentralDispatchGate {
	raw := globalCentralGate.Load()
	if raw == nil {
		return nil
	}
	gate, _ := raw.(*CentralDispatchGate)
	if gate == nil {
		return nil
	}
	return *gate
}

// ReleaseCentralExptOnDelete 供实验删除路径调用：删除实验必须释放其全部在途预占。
// 未接入中心调度（gate 为 nil）时是 no-op。
func ReleaseCentralExptOnDelete(ctx context.Context, exptID int64) error {
	gate := getGlobalCentralDispatchGate()
	if gate == nil {
		return nil
	}
	return gate.ReleaseExpt(ctx, exptID)
}

// SetCentralGate 允许测试或调用方直接在具体调度器实例上注入（优先于全局桥接）。
func (e *ExptSchedulerImpl) SetCentralGate(gate CentralDispatchGate) {
	e.centralGate = gate
}

func (e *ExptSchedulerImpl) centralDispatchGate() CentralDispatchGate {
	if e != nil && e.centralGate != nil {
		return e.centralGate
	}
	return getGlobalCentralDispatchGate()
}
