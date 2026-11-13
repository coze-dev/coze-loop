// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central_schedule

import (
	"context"
	"fmt"
	"time"
)

// Reconciler 增量对账 + 定期全量重建。
//
// 权威关系：额度占用以 ledger 为准，item 生命周期以 DB item 状态为准。
type Reconciler struct {
	Runs       ActiveRunSource
	Items      ItemSource
	Ledger     ReservationLedger
	Projection ProjectionWriter
	Clock      Clock
	Config     Config
}

type ReconcileStats struct {
	Runs      int
	Released  int
	Uncertain int
	Rebuilt   int
	Drift     int
}

// Incremental 对活跃 run 做有界增量对账：
//   - DB 终态 + ledger 未归还 -> release；
//   - DB Queueing + 预占超 lease -> 孤儿 release；
//   - DB Processing + dispatching 超 ack timeout -> uncertain（由补偿重发）。
func (r *Reconciler) Incremental(ctx context.Context, run *RunView, limit int) (ReconcileStats, error) {
	stats := ReconcileStats{}
	if run == nil || r.Ledger == nil || r.Items == nil {
		return stats, nil
	}
	stats.Runs = 1
	cfg := r.Config.WithDefaults()
	reservations, err := r.Ledger.ListRunReservations(ctx, run.RunID, "")
	if err != nil {
		return stats, fmt.Errorf("list reservations: %w", err)
	}
	if limit > 0 && len(reservations) > limit {
		reservations = reservations[:limit]
	}
	for _, res := range reservations {
		if res == nil || res.State.Terminal() {
			continue
		}
		status, serr := r.Items.GetItemStatus(ctx, run.RunID, res.ItemID)
		if serr != nil {
			stats.Drift++
			continue
		}
		switch {
		case status.Terminal():
			if _, rerr := r.Ledger.Release(ctx, res.ID, UpdateTime{Now: r.now()}); rerr == nil {
				stats.Released++
				if r.Projection != nil {
					_ = r.Projection.MarkReleased(ctx, run.RunID, res.ItemID, res.ID)
				}
			}
		case status == ItemQueueing && !r.now().Before(res.LeaseExpireAt):
			// 超期孤儿：已占额度但 DB 仍 Queueing。
			if _, rerr := r.Ledger.Release(ctx, res.ID, UpdateTime{Now: r.now()}); rerr == nil {
				stats.Released++
			}
		case status == ItemProcessing && res.State == StateDispatching &&
			res.PublishAt != nil && r.now().Sub(*res.PublishAt) > cfg.AckTimeout:
			if merr := r.Ledger.MarkUncertain(ctx, res.ID, UpdateTime{Now: r.now()}); merr == nil {
				stats.Uncertain++
			}
		}
	}
	return stats, nil
}

// FullRebuild 定期全量重建：影子代从零计算 usage，再原子切换 generation。
// 重建期间正常派发继续使用当前代；切换只是一次原子指针变更。
func (r *Reconciler) FullRebuild(ctx context.Context, scope string) (ReconcileStats, error) {
	stats := ReconcileStats{}
	if scope == "" {
		return stats, ErrOwnerScopeMissing
	}
	if r.Runs == nil || r.Ledger == nil {
		return stats, fmt.Errorf("central_schedule: reconciler dependencies missing")
	}
	cfg := r.Config.WithDefaults()
	runs, err := r.Runs.ScanActive(ctx, scope, cfg.RunLimit)
	if err != nil {
		return stats, fmt.Errorf("scan active runs: %w", err)
	}
	for _, run := range runs {
		if run == nil || run.Mode != ModeCentral {
			continue
		}
		stats.Runs++
		// 先做一次增量清理，避免把明显孤儿算进重建。
		if _, err := r.Incremental(ctx, run, 0); err != nil {
			stats.Drift++
		}
		live, err := r.Ledger.ListRunReservations(ctx, run.RunID, "")
		if err != nil {
			stats.Drift++
			continue
		}
		active := make([]*Reservation, 0, len(live))
		// 账本 live 维度由 Ledger.Rebuild 自己从 res 记录重算；这里只补
		// “DB 已 Processing 但账本缺失”的保守 item 槽，避免重建把在途算没。
		expected := make(map[string]int64)
		// DB 已 Processing 但账本缺失的 item：保守补 1 个 item 槽，避免重建
		// 把在途执行“算没”。维度细节由投影/回收路径修复。
		if r.Items != nil {
			processing, perr := r.Items.ListProcessing(ctx, run)
			if perr == nil {
				present := make(map[int64]struct{}, len(active))
				for _, res := range active {
					present[res.ItemID] = struct{}{}
				}
				for _, item := range processing {
					if _, ok := present[item.ItemID]; ok {
						continue
					}
					expected["item"] += 1
					stats.Drift++
				}
			}
		}
		result, rerr := r.Ledger.Rebuild(ctx, &RunAccount{
			OwnerScope:        run.OwnerScope,
			ExptID:            run.ExptID,
			RunID:             run.RunID,
			Mode:              run.Mode,
			Priority:          run.Priority,
			TargetConcurrency: run.TargetConcurrency,
			DimensionLimits:   run.DimensionLimits,
		}, expected, active)
		if rerr != nil {
			return stats, fmt.Errorf("%w: rebuild run %d: %v", ErrQuotaUnavailable, run.RunID, rerr)
		}
		if result.Swapped {
			stats.Rebuilt++
		}
	}
	return stats, nil
}

func (r *Reconciler) now() time.Time {
	if r.Clock == nil {
		return time.Now()
	}
	return r.Clock.Now()
}
