// Copyright (c) 2015 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// CentralTicker 中心调度 tick：扫描待处理/活跃实验、按全局优先级与提交顺序驱动 one-shot 调度事件，
// 并负责 pending_confirm 清扫、增量对账与周期性全量重建。
//
// 待处理的初始化（pending）与已活跃的维护（active）存放在不同 key，分别扫描、互不覆盖；
// 真正的 item 扫描/派发仍复用既有调度链，因此旧实验的僵尸清理、沙箱回收、完成判定不会被复制成第二套。
type CentralTicker struct {
	Repo           repo.ICentralSchedulingRepo
	Publisher      events.ExptEventPublisher
	ConfigProvider CentralSchedulerConfigProvider
	ScopeProvider  CentralEnvScopeProvider
	// ReconcileEveryTicks 每多少个 tick 做一次全量账本重建；<=0 时默认 15。
	ReconcileEveryTicks int
	// Owner 主实例标识；为空时自动生成 hostname+pid。
	Owner string

	tickCount int
}

// NewCentralTicker 构造中心 ticker。
func NewCentralTicker(r repo.ICentralSchedulingRepo, publisher events.ExptEventPublisher, configProvider CentralSchedulerConfigProvider, scopeProvider CentralEnvScopeProvider) *CentralTicker {
	if scopeProvider == nil {
		scopeProvider = DefaultCentralEnvScopeProvider
	}
	return &CentralTicker{
		Repo:                r,
		Publisher:           publisher,
		ConfigProvider:      configProvider,
		ScopeProvider:       scopeProvider,
		ReconcileEveryTicks: 15,
		Owner:               defaultCentralTickerOwner(),
	}
}

func defaultCentralTickerOwner() string {
	host, _ := os.Hostname()
	return host + "-" + strconv.Itoa(os.Getpid())
}

func (t *CentralTicker) config(ctx context.Context) *entity.CentralSchedulerConfig {
	if t == nil || t.ConfigProvider == nil {
		return nil
	}
	return t.ConfigProvider.GetCentralSchedulerConfig(ctx)
}

func (t *CentralTicker) scope(ctx context.Context) (entity.CentralEnvScope, bool) {
	if t == nil || t.ScopeProvider == nil {
		return DefaultCentralEnvScopeProvider(ctx)
	}
	return t.ScopeProvider(ctx)
}

// Run 阻塞运行 tick 循环，直到 ctx 结束。
func (t *CentralTicker) Run(ctx context.Context) {
	if t == nil || t.Repo == nil || t.Publisher == nil {
		logs.CtxWarn(ctx, "[central] ticker disabled: repo or publisher is nil")
		return
	}
	for {
		cfg := t.config(ctx)
		if cfg == nil || !cfg.Enabled {
			// 未启用时保持静默，不产生任何 Redis 写入。
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
				continue
			}
		}
		if err := t.Tick(ctx); err != nil {
			logs.CtxWarn(ctx, "[central] tick failed, err=%v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.GetTickInterval()):
		}
	}
}

// Tick 执行一轮中心调度驱动 + 对账；导出以便单测直接调用。
func (t *CentralTicker) Tick(ctx context.Context) error {
	cfg := t.config(ctx)
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	scope, ok := t.scope(ctx)
	if !ok {
		logs.CtxWarn(ctx, "[central] env scope unavailable, skip tick (fail closed)")
		return nil
	}

	leader, err := t.Repo.AcquireLeader(ctx, scope, t.Owner, cfg.GetTickInterval()*2)
	if err != nil {
		return err
	}
	if !leader {
		logs.CtxInfo(ctx, "[central] skip tick: another instance holds leader lock")
		return nil
	}

	// 到期未确认的派发必须先收敛，避免资源长期悬挂。
	healthy := true
	if err := t.sweepPendingConfirms(ctx, scope, cfg); err != nil {
		healthy = false
	}

	candidates := t.collectRuns(ctx, scope, cfg)
	for _, candidate := range candidates {
		if candidate == nil || candidate.run == nil {
			continue
		}
		run := candidate.run
		// aging 只改排序 score，成员中的 base priority 保持不变。
		if err := t.Repo.RescoreRun(ctx, scope, run, candidate.effectivePriority, candidate.pending); err != nil {
			logs.CtxWarn(ctx, "[central] rescore run failed, expt_id=%d, run_id=%d, err=%v", run.ExptID, run.ExptRunID, err)
		}
		event := &entity.ExptScheduleEvent{
			SpaceID:   run.SpaceID,
			ExptID:    run.ExptID,
			ExptRunID: run.ExptRunID,
			CreatedAt: time.Now().Unix(),
			OneShot:   true,
		}
		if run.SessionUserID != "" {
			event.Session = &entity.Session{UserID: run.SessionUserID}
		}
		if pubErr := t.Publisher.PublishExptScheduleEvent(ctx, event, gptr.Of(time.Second*3)); pubErr != nil {
			// 不注销：下一 tick 继续重试，避免一次 MQ 抖动把实验从中心调度里丢掉。
			logs.CtxWarn(ctx, "[central] publish schedule event failed, expt_id=%d, run_id=%d, err=%v", run.ExptID, run.ExptRunID, pubErr)
		}
	}

	t.tickCount++
	// 增量对账每 tick 做，只读定位漂移；全量重建按周期做，原子替换且不改变在途预占。
	if !t.incrementalReconcile(ctx, scope) {
		healthy = false
	}
	reconcileEvery := t.ReconcileEveryTicks
	if reconcileEvery <= 0 {
		reconcileEvery = 15
	}
	if t.tickCount%reconcileEvery == 0 {
		if !t.fullRebuild(ctx, scope, cfg) {
			healthy = false
		}
	}
	t.maintainDegraded(ctx, scope, cfg, healthy)
	return nil
}

type centralCandidate struct {
	run               *entity.CentralActiveRun
	pending           bool
	effectivePriority int
}

// collectRuns 读取 pending 与 active 两个集合，按“全局优先级 + 同档提交序号”合并排序，
// 并清理超过 pending TTL 仍未派发成功的待处理初始化。
func (t *CentralTicker) collectRuns(ctx context.Context, scope entity.CentralEnvScope, cfg *entity.CentralSchedulerConfig) []*centralCandidate {
	now := time.Now().Unix()
	windowSeconds := int64(cfg.GetAgingWindowTicks()) * int64(cfg.GetTickInterval()/time.Second)
	out := make([]*centralCandidate, 0)
	seen := make(map[string]bool)

	appendRuns := func(runs []*entity.CentralActiveRun, pending bool) {
		for _, run := range runs {
			if run == nil {
				continue
			}
			key := strconv.FormatInt(run.ExptID, 10) + ":" + strconv.FormatInt(run.ExptRunID, 10)
			if seen[key] {
				continue
			}
			seen[key] = true
			state, found, err := t.Repo.GetRunState(ctx, scope, run.ExptID, run.ExptRunID)
			if err != nil || !found {
				continue
			}
			registeredAt := state.RegisteredAt
			if registeredAt <= 0 {
				registeredAt = now
			}
			if pending && now-registeredAt > int64(cfg.GetPendingTTL()/time.Second) {
				// 待处理初始化长期未派发成功：清理登记，避免僵尸实验永远占着排序位。
				if err := t.Repo.RemovePending(ctx, scope, run.ExptID, run.ExptRunID); err != nil {
					logs.CtxWarn(ctx, "[central] cleanup stale pending failed, expt_id=%d, run_id=%d, err=%v", run.ExptID, run.ExptRunID, err)
				}
				logs.CtxWarn(ctx, "[central] stale pending removed, expt_id=%d, run_id=%d", run.ExptID, run.ExptRunID)
				continue
			}
			run.Seq = state.Seq
			run.Priority = state.Priority
			run.RegisteredAt = registeredAt
			out = append(out, &centralCandidate{
				run:               run,
				pending:           pending,
				effectivePriority: entity.EffectiveCentralPriority(state.Priority, registeredAt, now, windowSeconds, cfg.GetAgingStep(), cfg.GetHighestPriority()),
			})
		}
	}

	pendingRuns, err := t.Repo.ListPending(ctx, scope)
	if err != nil {
		logs.CtxWarn(ctx, "[central] list pending failed, err=%v", err)
	} else {
		appendRuns(pendingRuns, true)
	}
	activeRuns, err := t.Repo.ListActive(ctx, scope)
	if err != nil {
		logs.CtxWarn(ctx, "[central] list active failed, err=%v", err)
	} else {
		appendRuns(activeRuns, false)
	}

	// 全局排序：effective priority 小者优先；同档按冻结的提交序号 FIFO。
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].effectivePriority != out[j].effectivePriority {
			return out[i].effectivePriority < out[j].effectivePriority
		}
		return out[i].run.Seq < out[j].run.Seq
	})
	return out
}

func (t *CentralTicker) sweepPendingConfirms(ctx context.Context, scope entity.CentralEnvScope, cfg *entity.CentralSchedulerConfig) error {
	released, err := t.Repo.SweepPendingConfirms(ctx, scope, time.Now().Unix(), 7*24*time.Hour)
	if err != nil {
		logs.CtxWarn(ctx, "[central] sweep pending confirm failed, err=%v", err)
		return err
	}
	if released > 0 {
		logs.CtxWarn(ctx, "[central] dispatch unconfirmed, released %d leases", released)
	}
	return nil
}

// maintainDegraded 依据明确口径维护降级恢复：只有连续 N 个“无漂移、无清扫错误”的 tick
// 才标记恢复；任何异常 tick 都清零计数。恢复只回到配置容量，不会高于配置。
func (t *CentralTicker) maintainDegraded(ctx context.Context, scope entity.CentralEnvScope, cfg *entity.CentralSchedulerConfig, healthy bool) {
	if cfg == nil || !cfg.Degraded {
		return
	}
	if !healthy {
		if err := t.Repo.ResetHealthyDegradedTicks(ctx, scope); err != nil {
			logs.CtxWarn(ctx, "[central] reset degrade healthy ticks failed, err=%v", err)
		}
		if err := t.Repo.ClearDegradedRecovered(ctx, scope); err != nil {
			logs.CtxWarn(ctx, "[central] clear degrade recovered failed, err=%v", err)
		}
		return
	}
	ticks, err := t.Repo.RecordHealthyDegradedTick(ctx, scope)
	if err != nil {
		logs.CtxWarn(ctx, "[central] record degrade healthy tick failed, err=%v", err)
		return
	}
	recovered, err := t.Repo.IsDegradedRecovered(ctx, scope)
	if err != nil {
		logs.CtxWarn(ctx, "[central] read degrade recovered failed, err=%v", err)
		return
	}
	if recovered || ticks >= cfg.GetDegradedRecoverTicks() {
		if markErr := t.Repo.MarkDegradedRecovered(ctx, scope, 3*cfg.GetTickInterval()); markErr != nil {
			logs.CtxWarn(ctx, "[central] mark degrade recovered failed, err=%v", markErr)
			return
		}
		logs.CtxInfo(ctx, "[central] degraded recovered after %d healthy ticks", ticks)
	}
}

func (t *CentralTicker) incrementalReconcile(ctx context.Context, scope entity.CentralEnvScope) bool {
	drift, err := t.Repo.InspectLedger(ctx, scope)
	if err != nil {
		logs.CtxWarn(ctx, "[central] incremental reconcile failed, err=%v", err)
		return false
	}
	for _, item := range drift {
		if item == nil {
			continue
		}
		logs.CtxWarn(ctx, "[central] ledger drift, lease_id=%s, kind=%s, amount=%d, usage=%d, reason=%s",
			item.LeaseID, item.Kind, item.Amount, item.Usage, item.Reason)
	}
	return len(drift) == 0
}

func (t *CentralTicker) fullRebuild(ctx context.Context, scope entity.CentralEnvScope, cfg *entity.CentralSchedulerConfig) bool {
	before, after, repaired, drift, err := t.Repo.RebuildUsage(ctx, scope, cfg.GetCapacities(), cfg.GetLeaseTTL())
	if err != nil {
		logs.CtxWarn(ctx, "[central] ledger rebuild failed, err=%v", err)
		return false
	}
	if repaired > 0 || len(drift) > 0 {
		logs.CtxInfo(ctx, "[central] ledger rebuild, live_leases=%d, before=%v, after=%v", repaired, before, after)
	}
	for _, item := range drift {
		if item == nil {
			continue
		}
		logs.CtxWarn(ctx, "[central] ledger rebuild drift, lease_id=%s, kind=%s, amount=%d, usage=%d, reason=%s",
			item.LeaseID, item.Kind, item.Amount, item.Usage, item.Reason)
	}
	return len(drift) == 0
}
