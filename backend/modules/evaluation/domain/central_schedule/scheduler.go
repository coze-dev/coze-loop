// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central_schedule

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Scheduler 中心调度主链。一次 Tick 只做一轮有界扫描；tick 由
// ByteScheduler/独立 ticker 驱动，不依赖旧的 per-experiment MQ 自循环。
type Scheduler struct {
	Leader     Leader
	Runs       ActiveRunSource
	Admission  AdmissionGuard
	Items      ItemSource
	Ledger     ReservationLedger
	Dispatcher Dispatcher
	Projection ProjectionWriter
	Clock      Clock
	Config     Config
	InstanceID string
	// IDGen 预占唯一编号生成器；为空时使用 crypto/rand。
	IDGen func() string
	// ConfigProvider 按当前配置覆盖 Config；为空时使用 Config。
	ConfigProvider func(ctx context.Context) Config
	// RunDone 在 DB 已无 Queueing/Processing 且额度已全部归还时触发。
	// 实现方负责幂等（重复触发不应重复结束实验）。
	RunDone func(ctx context.Context, run *RunView) error
	// Reconciler 增量对账 + 周期全量重建；为空时跳过。
	Reconciler *Reconciler
	// rebuildMu/lastRebuild 控制全量重建节奏。重建在派发之后执行，
	// 且账本用 generation 双缓冲原子切换，重建期间派发不中断。
	rebuildMu   sync.Mutex
	lastRebuild time.Time
}

// Tick 扫描 active central run，按优先级补 item。
//
// 顺序不可交换：leader -> 扫描 -> 准入 -> 计算缺口 -> 全量预占 -> claim ->
// 发布 -> 投影。预占失败绝不发布；发布失败立即归还。
func (s *Scheduler) Tick(ctx context.Context) (TickStats, error) {
	return s.TickWithScope(ctx, "")
}

// TickWithScope 允许运维入口显式指定 owner scope；空串沿用配置。
func (s *Scheduler) TickWithScope(ctx context.Context, ownerScope string) (TickStats, error) {
	stats := TickStats{}
	cfg := s.Config
	if s.ConfigProvider != nil {
		cfg = s.ConfigProvider(ctx)
	}
	cfg = cfg.WithDefaults()
	if ownerScope != "" {
		cfg.OwnerScope = ownerScope
	}
	if cfg.Disabled {
		return stats, nil
	}
	if cfg.OwnerScope == "" {
		return stats, ErrOwnerScopeMissing
	}
	if s.Leader == nil || s.Runs == nil || s.Ledger == nil || s.Dispatcher == nil {
		return stats, fmt.Errorf("central_schedule: scheduler dependencies missing")
	}
	ok, err := s.Leader.Acquire(ctx, cfg.OwnerScope, s.InstanceID, cfg.LeaseTTL)
	if err != nil {
		return stats, fmt.Errorf("%w: leader acquire: %v", ErrQuotaUnavailable, err)
	}
	if !ok {
		// 其他实例持有 scope lease：本 tick 不做任何派发。
		return stats, nil
	}
	stats.Leader = true

	runs, err := s.Runs.ScanActive(ctx, cfg.OwnerScope, cfg.RunLimit)
	if err != nil {
		return stats, fmt.Errorf("scan active runs: %w", err)
	}
	stats.ScannedRuns = len(runs)
	SortRuns(runs)

	for _, run := range runs {
		if run == nil || run.Mode != ModeCentral {
			continue
		}
		if err := s.tickRun(ctx, run, cfg, &stats); err != nil {
			stats.Errors++
			// 单个 run 失败不影响其他 run；额度源不可用时直接中止本轮。
			if err == ErrQuotaUnavailable {
				return stats, err
			}
		}
	}

	// 周期全量重建在派发之后执行：本轮已派发的 item 不受影响；
	// 账本用影子代原子切换，下一轮 tick 继续用旧代/新代派发，不会出现整段停派。
	if s.Reconciler != nil {
		s.rebuildMu.Lock()
		if s.now().Sub(s.lastRebuild) >= cfg.RebuildInterval {
			s.lastRebuild = s.now()
			rec := *s.Reconciler
			rec.Config = cfg
			rstat, rerr := rec.FullRebuild(ctx, cfg.OwnerScope)
			s.rebuildMu.Unlock()
			if rerr != nil {
				stats.Errors++
				return stats, fmt.Errorf("periodic rebuild: %w", rerr)
			}
			stats.Compensated += rstat.Rebuilt
			stats.Released += rstat.Released
		} else {
			s.rebuildMu.Unlock()
		}
	}
	return stats, nil
}

func (s *Scheduler) tickRun(ctx context.Context, run *RunView, cfg Config, stats *TickStats) error {
	// 先做增量对账：释放 DB 已终态/超期孤儿的预占，再按释放后的额度派发。
	if s.Reconciler != nil {
		rec := *s.Reconciler
		rec.Config = cfg
		rstat, rerr := rec.Incremental(ctx, run, 0)
		if rerr != nil {
			return fmt.Errorf("incremental reconcile: %w", rerr)
		}
		stats.Released += rstat.Released
		stats.Compensated += rstat.Uncertain
	}

	if s.Admission != nil {
		if err := s.Admission.Check(ctx, run); err != nil {
			return fmt.Errorf("admission check: %w", err)
		}
	}
	usage, err := s.Ledger.Usage(ctx, run.RunID)
	if err != nil {
		return fmt.Errorf("%w: usage: %v", ErrQuotaUnavailable, err)
	}
	available := Available(run.TargetConcurrency, 0, 0, usage["item"])
	if available <= 0 {
		return nil
	}
	limit := available
	if int64(cfg.BatchLimit) < limit {
		limit = int64(cfg.BatchLimit)
	}
	items, err := s.Items.ScanQueueing(ctx, run, int(limit))
	if err != nil {
		return fmt.Errorf("scan queueing items: %w", err)
	}
	stats.ScannedItems += len(items)
	if len(items) == 0 {
		if available > 0 && usage["item"] <= 0 && s.RunDone != nil {
			processing, perr := s.Items.ListProcessing(ctx, run)
			if perr != nil {
				return fmt.Errorf("list processing: %w", perr)
			}
			if len(processing) == 0 {
				// 以 DB 状态为准判定收工，重复触发由回调方幂等。
				if derr := s.RunDone(ctx, run); derr != nil {
					return fmt.Errorf("run done: %w", derr)
				}
				stats.Compensated++
			}
		}
		return nil
	}

	// 按剩余维度额度裁剪；任一维度为 0 时整批为空。
	remaining := make(map[string]int64, len(run.DimensionLimits))
	for k, v := range run.DimensionLimits {
		remaining[k] = v - usage[k]
		if remaining[k] < 0 {
			remaining[k] = 0
		}
	}
	// item 维度以 target 并发为准，防止 DimensionLimits 未配时漏裁剪。
	remaining["item"] = available
	batch := MinBatchBy(items, remaining)
	if len(batch) == 0 {
		stats.ReserveShortfall++
		return nil
	}

	reservations := make([]*Reservation, 0, len(batch))
	for _, item := range batch {
		reservations = append(reservations, &Reservation{
			ID:            s.reservationID(),
			DedupKey:      DedupKey(run.RunID, item.ItemID, item.Attempt),
			OwnerScope:    run.OwnerScope,
			ExptID:        item.ExptID,
			RunID:         item.RunID,
			ItemID:        item.ItemID,
			Attempt:       item.Attempt,
			Dimensions:    item.Dimensions,
			CreatedAt:     s.now(),
			UpdatedAt:     s.now(),
			LeaseExpireAt: s.now().Add(cfg.AckTimeout + cfg.LeaseTTL),
		})
	}
	// 一批 item 的所有维度一次占齐；任一维度不足则整批零变更。
	if err := s.Ledger.ReserveBatch(ctx, &RunAccount{
		OwnerScope:        run.OwnerScope,
		ExptID:            run.ExptID,
		RunID:             run.RunID,
		Mode:              run.Mode,
		Priority:          run.Priority,
		TargetConcurrency: run.TargetConcurrency,
		DimensionLimits:   run.DimensionLimits,
	}, reservations); err != nil {
		if _, ok := err.(*ShortfallError); ok {
			stats.ReserveShortfall++
			return nil
		}
		return fmt.Errorf("%w: reserve: %v", ErrQuotaUnavailable, err)
	}
	stats.Reserved += len(reservations)

	for i, item := range batch {
		res := reservations[i]
		claimed, cerr := s.Ledger.Claim(ctx, run.RunID, item.ItemID, item.Attempt, res.ID, int64(cfg.LeaseTTL/time.Second)+int64(cfg.AckTimeout/time.Second))
		if cerr != nil {
			_, _ = s.Ledger.Release(ctx, res.ID, UpdateTime{Now: s.now()})
			return fmt.Errorf("%w: claim: %v", ErrQuotaUnavailable, cerr)
		}
		if !claimed {
			// 另一实例/上一轮已 claim：归还本条预占，不重复派发。
			_, _ = s.Ledger.Release(ctx, res.ID, UpdateTime{Now: s.now()})
			continue
		}
		if err := s.Dispatcher.Publish(ctx, item, res.ID, res.DedupKey); err != nil {
			_, _ = s.Ledger.Release(ctx, res.ID, UpdateTime{Now: s.now()})
			stats.Errors++
			continue
		}
		if err := s.Ledger.MarkDispatching(ctx, res); err != nil {
			// 已发布但状态未落：保持占用，交给 Reconciler/补偿处理。
			_ = s.Ledger.MarkUncertain(ctx, res.ID, UpdateTime{Now: s.now()})
			stats.Errors++
			continue
		}
		if s.Projection != nil {
			if err := s.Projection.MarkProcessing(ctx, item, res.ID); err != nil {
				// 投影失败不归还：执行可能已经发生；由 Reconciler 修投影。
				stats.Errors++
			}
		}
		stats.Dispatched++
	}
	return nil
}

// AckItem 执行侧入口调用：CAS 到 confirmed。返回 false 时调用方必须跳过执行。
func (s *Scheduler) AckItem(ctx context.Context, reservationID string, item *ItemView) (bool, error) {
	result, err := s.Ledger.Ack(ctx, reservationID, UpdateTime{Now: s.now()})
	if err != nil {
		return false, err
	}
	switch result {
	case AckConfirmed:
		if s.Projection != nil && item != nil {
			_ = s.Projection.MarkProcessing(ctx, item, reservationID)
		}
		return true, nil
	case AckDuplicate, AckLate, AckMissing:
		// 重复回调/迟到回调/账本缺失：不执行、不重复扣。
		return false, nil
	default:
		return false, fmt.Errorf("unknown ack result: %d", result)
	}
}

// Interval 返回当前配置的 tick 间隔；配置关闭或未配置时回落到默认 1 分钟。
func (s *Scheduler) Interval(ctx context.Context) time.Duration {
	cfg := s.Config
	if s.ConfigProvider != nil {
		cfg = s.ConfigProvider(ctx)
	}
	cfg = cfg.WithDefaults()
	if cfg.TickInterval <= 0 {
		return time.Minute
	}
	return cfg.TickInterval
}

// InitRun 进入 Pending 时初始化额度账户。重复调用由账本脚本幂等处理。
func (s *Scheduler) InitRun(ctx context.Context, account *RunAccount) error {
	if s == nil || s.Ledger == nil {
		return ErrNotInitialized
	}
	return s.Ledger.InitRun(ctx, account)
}

// AckItemByItem 执行侧入口：按 run/item/attempt 反查本次派发的 reservation。
// 无凭证（legacy 事件或已释放）返回 false，调用方必须跳过执行。
func (s *Scheduler) AckItemByItem(ctx context.Context, item *ItemView) (bool, error) {
	if s == nil || s.Ledger == nil || item == nil {
		return false, nil
	}
	rid, err := s.Ledger.ResolveReservation(ctx, item.RunID, item.ItemID, item.Attempt)
	if err != nil {
		if err == ErrReservationAbsent {
			return false, nil
		}
		return false, err
	}
	return s.AckItem(ctx, rid, item)
}

// ReleaseItem 归还单条 item 预占（成功/失败/取消路径）。重复调用幂等。
func (s *Scheduler) ReleaseItem(ctx context.Context, runID, itemID int64, attempt int32) error {
	if s == nil || s.Ledger == nil {
		return nil
	}
	rid, err := s.Ledger.ResolveReservation(ctx, runID, itemID, attempt)
	if err != nil {
		if err == ErrReservationAbsent {
			return nil
		}
		return err
	}
	_, err = s.Ledger.Release(ctx, rid, UpdateTime{Now: s.now()})
	return err
}

// ReleaseRun 归还该 run 所有未终态预占，重复调用幂等。所有结束路径
// （成功、失败、取消、重试、派发异常）都必须走这里或逐条 Release。
func (s *Scheduler) ReleaseRun(ctx context.Context, runID int64) (int, error) {
	if s == nil || s.Ledger == nil {
		return 0, nil
	}
	live, err := s.Ledger.ListRunReservations(ctx, runID, "")
	if err != nil {
		return 0, err
	}
	released := 0
	for _, res := range live {
		if res == nil || res.State.Terminal() {
			continue
		}
		result, rerr := s.Ledger.Release(ctx, res.ID, UpdateTime{Now: s.now()})
		if rerr != nil {
			return released, rerr
		}
		if result == ReleaseDone {
			released++
		}
	}
	return released, nil
}

func (s *Scheduler) reservationID() string {
	if s.IDGen != nil {
		return s.IDGen()
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	// crypto/rand 不可用时退回时间 + 大小写不敏感的唯一串；实际部署不会走到。
	return fmt.Sprintf("rid-%d-%d-%d", s.now().UnixNano(), len(s.InstanceID), s.now().Unix())
}

func (s *Scheduler) now() time.Time {
	if s.Clock == nil {
		return time.Now()
	}
	return s.Clock.Now()
}

// DedupKey 同 run + item + attempt 唯一；重试产生新 attempt，不复用旧预占。
func DedupKey(runID, itemID int64, attempt int32) string {
	return fmt.Sprintf("%d:%d:%d", runID, itemID, attempt)
}
