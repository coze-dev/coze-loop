// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// CentralTicker 中心调度 tick：扫描活跃实验、按优先级驱动 one-shot 调度事件，并周期性对账。
//
// 这是题面“中心服务扫描活跃实验”的落地：活跃集合由 enforce 模式的调度 tick 登记，
// 中心 tick 只负责重新投递 one-shot 事件；真正的 item 扫描/派发仍复用既有调度链，
// 因此旧实验的僵尸清理、沙箱回收、完成判定逻辑不会被复制成第二套。
type CentralTicker struct {
	Repo           repo.ICentralSchedulingRepo
	Publisher      events.ExptEventPublisher
	ConfigProvider CentralSchedulerConfigProvider
	ScopeProvider  CentralEnvScopeProvider
	// ReconcileEveryTicks 每多少个 tick 做一次账本漂移修复；<=0 时默认 15。
	ReconcileEveryTicks int
	// Owner 主实例标识；为空时自动生成 hostname+pid。
	Owner string

	tickCount int
}

var _ = fmt.Sprintf

// NewCentralTicker 构造中心 ticker。
func NewCentralTicker(repo repo.ICentralSchedulingRepo, publisher events.ExptEventPublisher, configProvider CentralSchedulerConfigProvider, scopeProvider CentralEnvScopeProvider) *CentralTicker {
	if scopeProvider == nil {
		scopeProvider = DefaultCentralEnvScopeProvider
	}
	return &CentralTicker{
		Repo:                repo,
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

func (t *CentralTicker) scope(ctx context.Context) entity.CentralEnvScope {
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

// Tick 执行一轮中心调度驱动 + 周期性对账；导出以便单测直接调用。
func (t *CentralTicker) Tick(ctx context.Context) error {
	cfg := t.config(ctx)
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	scope := t.scope(ctx)

	leader, err := t.Repo.AcquireLeader(ctx, scope, t.Owner, cfg.GetTickInterval()*2)
	if err != nil {
		return err
	}
	if !leader {
		logs.CtxInfo(ctx, "[central] skip tick: another instance holds leader lock")
		return nil
	}

	runs, err := t.Repo.ListActive(ctx, scope)
	if err != nil {
		return err
	}
	for _, run := range runs {
		if run == nil {
			continue
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
			// 不注销活跃实验：下一 tick 继续重试，避免一次 MQ 抖动把实验从中心调度里丢掉。
			logs.CtxWarn(ctx, "[central] publish schedule event failed, expt_id=%d, run_id=%d, err=%v", run.ExptID, run.ExptRunID, pubErr)
		}
	}

	t.tickCount++
	reconcileEvery := t.ReconcileEveryTicks
	if reconcileEvery <= 0 {
		reconcileEvery = 15
	}
	if t.tickCount%reconcileEvery == 0 {
		t.reconcile(ctx, scope, cfg)
	}
	return nil
}

func (t *CentralTicker) reconcile(ctx context.Context, scope entity.CentralEnvScope, cfg *entity.CentralSchedulerConfig) {
	before, after, repaired, err := t.Repo.RebuildUsage(ctx, scope, cfg.GetCapacities(), cfg.GetLeaseTTL())
	if err != nil {
		logs.CtxWarn(ctx, "[central] ledger rebuild failed, err=%v", err)
		return
	}
	if repaired > 0 {
		logs.CtxInfo(ctx, "[central] ledger rebuild, live_leases=%d, before=%v, after=%v", repaired, before, after)
	}
}
