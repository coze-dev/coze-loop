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
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// CentralSchedulerConfigProvider 提供中心调度配置（商业版从 TCC/静态配置读取；开源侧可注入假实现）。
type CentralSchedulerConfigProvider interface {
	GetCentralSchedulerConfig(ctx context.Context) *entity.CentralSchedulerConfig
}

// CentralEnvScopeProvider 提供中心账本的环境所有权命名空间。
type CentralEnvScopeProvider func(ctx context.Context) entity.CentralEnvScope

// DefaultCentralEnvScopeProvider 从进程环境推导归属：prioritize K_ENV / x_tt_env（PPE 泳道），
// 否则回落到 PSM。所有 key 最终仍由 repo 统一加 qn:central: 前缀。
func DefaultCentralEnvScopeProvider(ctx context.Context) entity.CentralEnvScope {
	lane := strings.TrimSpace(os.Getenv("K_ENV"))
	if lane == "" {
		lane = strings.TrimSpace(os.Getenv("x_tt_env"))
	}
	envName := strings.TrimSpace(os.Getenv("TCE_PSM"))
	if envName == "" {
		envName = "unknown"
	}
	if lane == "" {
		lane = "default"
	}
	return entity.CentralEnvScope{Env: envName, Lane: lane}
}

// CentralDispatchGate 是“中心调度是否启用 + 如何预占/释放”的门面。
//
// 旧模式（legacy）下所有方法都保持无副作用，保证旧实验/旧调用方完全不受影响。
type CentralDispatchGate interface {
	// Enabled 判断该实验当前是否走中心调度 enforce 模式。
	Enabled(ctx context.Context, expt *entity.Experiment) bool
	// Observe 在调度 tick 开始时调用：enforce 模式下登记活跃实验并返回 true。
	Observe(ctx context.Context, expt *entity.Experiment, event *entity.ExptScheduleEvent) bool
	// ReserveItems 为待派发 item 原子预占多资源；返回真正允许派发的 item 与已持有的 lease。
	ReserveItems(ctx context.Context, expt *entity.Experiment, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem) ([]*entity.ExptEvalItem, []*entity.CentralLease, error)
	// ReleaseLeases 释放一次派发调用中已获得的 lease（派发失败补偿）。
	ReleaseLeases(ctx context.Context, leases []*entity.CentralLease) error
	// ReleaseItems 按 item 维度幂等释放（item 完成/失败/取消/僵尸）。
	ReleaseItems(ctx context.Context, exptID, runID int64, itemIDs []int64) error
	// UnregisterRun 注销一次运行（实验终态/取消/重试收敛后）。
	UnregisterRun(ctx context.Context, exptID, runID int64) error
}

type centralDispatchGateImpl struct {
	repo           repo.ICentralSchedulingRepo
	configProvider CentralSchedulerConfigProvider
	scopeProvider  CentralEnvScopeProvider
}

var _ CentralDispatchGate = (*centralDispatchGateImpl)(nil)

func NewCentralDispatchGate(repo repo.ICentralSchedulingRepo, configProvider CentralSchedulerConfigProvider, scopeProvider CentralEnvScopeProvider) CentralDispatchGate {
	if scopeProvider == nil {
		scopeProvider = DefaultCentralEnvScopeProvider
	}
	return &centralDispatchGateImpl{repo: repo, configProvider: configProvider, scopeProvider: scopeProvider}
}

func (g *centralDispatchGateImpl) config(ctx context.Context) *entity.CentralSchedulerConfig {
	if g == nil || g.configProvider == nil {
		return nil
	}
	return g.configProvider.GetCentralSchedulerConfig(ctx)
}

func (g *centralDispatchGateImpl) scope(ctx context.Context) entity.CentralEnvScope {
	if g == nil || g.scopeProvider == nil {
		return DefaultCentralEnvScopeProvider(ctx)
	}
	return g.scopeProvider(ctx)
}

func (g *centralDispatchGateImpl) Enabled(ctx context.Context, expt *entity.Experiment) bool {
	if g == nil || g.repo == nil || expt == nil {
		return false
	}
	cfg := g.config(ctx)
	if cfg == nil || !cfg.Enabled {
		return false
	}
	return entity.ResolveCentralSchedulerMode(cfg, expt.SpaceID, extOf(expt)) == entity.CentralSchedulerModeEnforce
}

func (g *centralDispatchGateImpl) Observe(ctx context.Context, expt *entity.Experiment, event *entity.ExptScheduleEvent) bool {
	if !g.Enabled(ctx, expt) {
		return false
	}
	priority := entity.ResolveCentralPriority(extOf(expt), entity.DefaultCentralPriority)
	run := &entity.CentralActiveRun{
		SpaceID:   expt.SpaceID,
		ExptID:    expt.ID,
		ExptRunID: event.ExptRunID,
		Priority:  priority,
	}
	if event.Session != nil {
		run.SessionUserID = event.Session.UserID
	}
	if err := g.repo.RegisterActive(ctx, g.scope(ctx), run, 24*time.Hour); err != nil {
		// 登记失败不阻断本次调度：当前 tick 仍可正常派发，下一 tick 会再次登记。
		logs.CtxWarn(ctx, "[central] register active failed, expt_id=%d, run_id=%d, err=%v", expt.ID, event.ExptRunID, err)
	}
	logs.CtxInfo(ctx, "[central] observe enforce experiment, expt_id=%d, run_id=%d, priority=%d", expt.ID, event.ExptRunID, priority)
	return true
}

func (g *centralDispatchGateImpl) ReserveItems(ctx context.Context, expt *entity.Experiment, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem) ([]*entity.ExptEvalItem, []*entity.CentralLease, error) {
	if !g.Enabled(ctx, expt) || len(items) == 0 {
		return items, nil, nil
	}
	cfg := g.config(ctx)
	scope := g.scope(ctx)
	capacities := cfg.GetCapacities()
	ttl := cfg.GetLeaseTTL()
	claims := buildCentralClaims(expt)
	allowed := make([]*entity.ExptEvalItem, 0, len(items))
	leases := make([]*entity.CentralLease, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		now := time.Now()
		lease := &entity.CentralLease{
			LeaseID:    entity.MakeCentralLeaseID(event.ExptID, event.ExptRunID, item.ItemID),
			SpaceID:    event.SpaceID,
			ExptID:     event.ExptID,
			ExptRunID:  event.ExptRunID,
			ItemID:     item.ItemID,
			Claims:     claims,
			AcquiredAt: now.Unix(),
			ExpireAt:   now.Add(ttl).Unix(),
		}
		ok, err := g.repo.Acquire(ctx, scope, capacities, lease, ttl)
		if err != nil {
			if relErr := g.ReleaseLeases(ctx, leases); relErr != nil {
				logs.CtxWarn(ctx, "[central] rollback leases failed, expt_id=%d, run_id=%d, err=%v", event.ExptID, event.ExptRunID, relErr)
			}
			return nil, nil, err
		}
		if !ok {
			// 资源池已满：本 item 留在 Queueing，下一 tick 再试；已获得的 lease 保留并在完成时释放。
			logs.CtxInfo(ctx, "[central] resource exhausted, defer item, expt_id=%d, run_id=%d, item_id=%d", event.ExptID, event.ExptRunID, item.ItemID)
			break
		}
		allowed = append(allowed, item)
		leases = append(leases, lease)
	}
	return allowed, leases, nil
}

func (g *centralDispatchGateImpl) ReleaseLeases(ctx context.Context, leases []*entity.CentralLease) error {
	if g == nil || g.repo == nil || len(leases) == 0 {
		return nil
	}
	scope := g.scope(ctx)
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
	cfg := g.config(ctx)
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	scope := g.scope(ctx)
	var firstErr error
	for _, itemID := range itemIDs {
		leaseID := entity.MakeCentralLeaseID(exptID, runID, itemID)
		if _, err := g.repo.Release(ctx, scope, leaseID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (g *centralDispatchGateImpl) UnregisterRun(ctx context.Context, exptID, runID int64) error {
	if g == nil || g.repo == nil {
		return nil
	}
	cfg := g.config(ctx)
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	scope := g.scope(ctx)
	if err := g.repo.UnregisterActive(ctx, scope, exptID, runID); err != nil {
		return err
	}
	// 终态/取消/停跑：把该 run 仍存活的 item 预占一并释放，避免“实验已结束但额度还挂着”。
	released, err := g.repo.ReleaseRun(ctx, scope, exptID, runID)
	if err != nil {
		return err
	}
	if released > 0 {
		logs.CtxInfo(ctx, "[central] released %d leases on run unregister, expt_id=%d, run_id=%d", released, exptID, runID)
	}
	return nil
}

// buildCentralClaims 计算一次 item 派发需要预占的资源。
//
// 这是题面尚未给出业务口径时的暂定模型（已写入分析材料的待确认项）：
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
