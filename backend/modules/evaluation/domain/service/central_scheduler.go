// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// DispatchGate 中心调度在 item 真正发布前插入的闸门。
// Filter 只放行已经预占到额度的 item；OnCompleted 立即归还已完成 item 的额度。
type DispatchGate interface {
	Filter(ctx context.Context, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem) ([]*entity.ExptEvalItem, error)
	OnCompleted(ctx context.Context, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem) error
}

// CentralTickExecutor 单实验 tick 执行器。由 ExptSchedulerImpl.ScheduleOnce 实现。
type CentralTickExecutor interface {
	ScheduleOnce(ctx context.Context, event *entity.ExptScheduleEvent, gate DispatchGate) error
}

// CentralSchedulerConfiger 读取中心调度配置。
type CentralSchedulerConfiger interface {
	GetCentralSchedulerConf(ctx context.Context) *entity.CentralSchedulerConf
}

// LeaderLease env 级 leader 租约。
type LeaderLease interface {
	Renew(ctx context.Context) (bool, error)
	Release(ctx context.Context) error
}

// LeaderElector env 级 leader 选举。一个 env 同一时刻只允许一个 leader 执行 tick。
type LeaderElector interface {
	Acquire(ctx context.Context, ownerEnv string, ttl time.Duration) (LeaderLease, bool, error)
}

// NewCentralScheduler 构造中心调度服务。
func NewCentralScheduler(
	repo repo.CentralSchedulerRepo,
	ledger RateLedger,
	executor CentralTickExecutor,
	configer CentralSchedulerConfiger,
	leader LeaderElector,
	ownerEnv string,
) CentralScheduler {
	return &CentralSchedulerImpl{
		Repo:     repo,
		Ledger:   ledger,
		Executor: executor,
		Configer: configer,
		Leader:   leader,
		OwnerEnv: ownerEnv,
	}
}

// CentralScheduler 中心调度主链。
type CentralScheduler interface {
	// Tick 执行一轮：结算已完成预占、扫描活跃实验、按优先级分配缺口、预占后派发。
	Tick(ctx context.Context) error
	// Rebuild 以 DB 为准全量重建当前 env 的额度账本。
	Rebuild(ctx context.Context) error
}

// CentralSchedulerImpl 中心调度实现。
type CentralSchedulerImpl struct {
	Repo     repo.CentralSchedulerRepo
	Ledger   RateLedger
	Executor CentralTickExecutor
	Configer CentralSchedulerConfiger
	Leader   LeaderElector
	// OwnerEnv 当前实例归属环境（prod / ppe_xxx）。空值 fail-closed。
	OwnerEnv string
	// Now 可注入时钟，便于测试。
	Now func() time.Time
	// NewDispatchID 可注入 dispatch id 生成器。
	NewDispatchID func() string
	// MaxActiveExperiments 单轮扫描上限。
	MaxActiveExperiments int
}

func (s *CentralSchedulerImpl) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *CentralSchedulerImpl) newDispatchID() string {
	if s.NewDispatchID != nil {
		return s.NewDispatchID()
	}
	return fmt.Sprintf("cs-%d", s.now().UnixNano())
}

func (s *CentralSchedulerImpl) conf(ctx context.Context) *entity.CentralSchedulerConf {
	if s.Configer == nil {
		return nil
	}
	conf := s.Configer.GetCentralSchedulerConf(ctx)
	if conf == nil {
		return nil
	}
	conf.FillDefaults()
	return conf
}

// Tick 实现中心调度主链。任一 infra 读取失败时按 fail-closed 跳过本轮对应实验，
// 不做猜测性扣减或派发。
func (s *CentralSchedulerImpl) Tick(ctx context.Context) error {
	conf := s.conf(ctx)
	if conf == nil || !conf.Enabled {
		return nil
	}
	ownerEnv := entity.NormalizeOwnerEnv(s.OwnerEnv)
	if ownerEnv == "" {
		return fmt.Errorf("central scheduler owner env is empty")
	}
	if s.Repo == nil || s.Ledger == nil || s.Executor == nil || s.Leader == nil {
		return fmt.Errorf("central scheduler dependencies are incomplete")
	}

	lease, acquired, err := s.Leader.Acquire(ctx, ownerEnv, conf.LeaseTTL())
	if err != nil {
		return err
	}
	if !acquired {
		logs.CtxInfo(ctx, "[CentralScheduler] not leader, skip tick, env: %v", ownerEnv)
		return nil
	}
	defer func() {
		if err := lease.Release(ctx); err != nil {
			logs.CtxWarn(ctx, "[CentralScheduler] release lease fail, env: %v, err: %v", ownerEnv, err)
		}
	}()
	stopRenew := s.keepLease(ctx, lease, conf.LeaseRenew())
	defer stopRenew()

	// 权威源读主库，避免主从延迟导致把 Processing 误判成 Queueing。
	ctx = contexts.WithCtxWriteDB(ctx)

	if err := s.settle(ctx, ownerEnv, conf); err != nil {
		logs.CtxWarn(ctx, "[CentralScheduler] settle before tick fail, env: %v, err: %v", ownerEnv, err)
	}

	active, err := s.Repo.ListActiveExperiments(ctx, ownerEnv, s.maxActive())
	if err != nil {
		return err
	}
	sortActiveExperiments(active, conf, s.now())

	capacity := conf.Capacity(ownerEnv)
	snapshot, err := s.Ledger.Snapshot(ctx, ownerEnv, capacity)
	if err != nil {
		return err
	}
	used := snapshot.Used

	var expected []*entity.Reservation
	now := s.now()

	for _, expt := range active {
		if expt == nil || expt.OwnerEnv != ownerEnv {
			continue
		}
		// 只有活跃状态才参与调度；终态/收尾中实验留给 settle 收尾。
		if entity.IsExptFinished(expt.ExptStatus) || entity.IsExptFinishing(expt.ExptStatus) {
			continue
		}

		states, err := s.Repo.ListRunItemStates(ctx, expt.SpaceID, expt.ExptID, expt.ExptRunID)
		if err != nil {
			logs.CtxWarn(ctx, "[CentralScheduler] list item states fail, expt_id: %v, run_id: %v, err: %v", expt.ExptID, expt.ExptRunID, err)
			continue
		}
		processing, queueingIDs := summarizeItemStates(states)
		perItem := expt.ResourcePerItem()
		expected = append(expected, expectedReservations(expt, states, perItem, ownerEnv)...)

		gap := s.calculateGap(expt, processing, perItem, capacity, used, conf.MaxBatchPerExpt)
		if gap <= 0 || len(queueingIDs) == 0 {
			continue
		}
		if gap > len(queueingIDs) {
			gap = len(queueingIDs)
		}

		requests := make([]*entity.Reservation, 0, gap)
		for _, itemID := range queueingIDs[:gap] {
			requests = append(requests, &entity.Reservation{
				ID:         entity.ReservationID(expt.ExptRunID, itemID),
				SpaceID:    expt.SpaceID,
				ExptID:     expt.ExptID,
				ExptRunID:  expt.ExptRunID,
				ItemID:     itemID,
				OwnerEnv:   ownerEnv,
				Vector:     perItem,
				DispatchID: s.newDispatchID(),
				State:      entity.ReservationStateDispatching,
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}

		result, err := s.Ledger.Reserve(ctx, ownerEnv, capacity, requests)
		if err != nil {
			logs.CtxWarn(ctx, "[CentralScheduler] reserve fail, expt_id: %v, run_id: %v, err: %v", expt.ExptID, expt.ExptRunID, err)
			continue
		}
		if result == nil || len(result.Granted) == 0 {
			continue
		}
		for _, r := range result.Granted {
			used = used.Add(r.Vector)
		}

		reservedByItem := make(map[int64]*entity.Reservation, len(result.Granted))
		for _, r := range result.Granted {
			reservedByItem[r.ItemID] = r
		}
		gate := &ledgerDispatchGate{
			ownerEnv:       ownerEnv,
			exptID:         expt.ExptID,
			exptRunID:      expt.ExptRunID,
			ledger:         s.Ledger,
			reservedByItem: reservedByItem,
			granted:        make(map[int64]bool, len(reservedByItem)),
		}

		event := &entity.ExptScheduleEvent{
			SpaceID:            expt.SpaceID,
			ExptID:             expt.ExptID,
			ExptRunID:          expt.ExptRunID,
			ExptRunMode:        expt.ExptRunMode,
			ExptType:           expt.ExptType,
			CreatedAt:          now.Unix(),
			Session:            &entity.Session{UserID: expt.CreatedBy},
			ExecEvalSetItemIDs: expt.RetryItemIDs,
			Ext:                map[string]string{"__central_scheduler": "1"},
		}
		if err := s.Executor.ScheduleOnce(ctx, event, gate); err != nil {
			logs.CtxWarn(ctx, "[CentralScheduler] schedule once fail, expt_id: %v, run_id: %v, err: %v", expt.ExptID, expt.ExptRunID, err)
		}

		// 本次预占但未被 gate 放行的 item 一定没有发布（仍在 Queueing），可安全释放，
		// 避免额度被未派发 item 占住。已放行的 item 交给 settle 按 DB 实际状态收尾。
		for itemID, r := range reservedByItem {
			if gate.granted[itemID] {
				continue
			}
			if _, rerr := s.Ledger.Release(ctx, ownerEnv, r.ID); rerr != nil {
				logs.CtxWarn(ctx, "[CentralScheduler] release undelivered reservation fail, key: %v, err: %v", r.ID, rerr)
			} else {
				used = used.Sub(r.Vector)
			}
		}
	}

	// 漂移校验：DB 期望 vs Redis 快照。超过阈值时以 DB 全量重建。
	finalSnapshot, err := s.Ledger.Snapshot(ctx, ownerEnv, capacity)
	if err != nil {
		logs.CtxWarn(ctx, "[CentralScheduler] snapshot after tick fail, env: %v, err: %v", ownerEnv, err)
		return nil
	}
	expectedVector := aggregateReservationVector(expected)
	drift := expectedVector.Sub(finalSnapshot.Used)
	if abs64(drift.ItemSlots) > conf.DriftItemThreshold || abs64(drift.EvaluatorSlots) > conf.DriftEvaluatorThreshold {
		logs.CtxWarn(ctx, "[CentralScheduler] ledger drift detected, env: %v, expected: %+v, actual: %+v, rebuilding", ownerEnv, expectedVector, finalSnapshot.Used)
		if rerr := s.Ledger.Rebuild(ctx, ownerEnv, capacity, expected); rerr != nil {
			logs.CtxWarn(ctx, "[CentralScheduler] rebuild after drift fail, env: %v, err: %v", ownerEnv, rerr)
		}
	}
	return nil
}

func (s *CentralSchedulerImpl) maxActive() int {
	if s.MaxActiveExperiments > 0 {
		return s.MaxActiveExperiments
	}
	return 1000
}

func (s *CentralSchedulerImpl) calculateGap(expt *entity.CentralActiveExperiment, processing int, perItem, capacity, used entity.ResourceVector, maxBatch int) int {
	gap := expt.Gap(processing)
	if maxBatch > 0 && gap > maxBatch {
		gap = maxBatch
	}
	remainItem := capacity.ItemSlots - used.ItemSlots
	if remainItem <= 0 {
		return 0
	}
	if int64(gap) > remainItem {
		gap = int(remainItem)
	}
	if perItem.EvaluatorSlots > 0 {
		remainEval := capacity.EvaluatorSlots - used.EvaluatorSlots
		if remainEval <= 0 {
			return 0
		}
		byEval := int(remainEval / perItem.EvaluatorSlots)
		if gap > byEval {
			gap = byEval
		}
	}
	if gap < 0 {
		return 0
	}
	return gap
}

func summarizeItemStates(states []*entity.CentralItemState) (processing int, queueingIDs []int64) {
	for _, st := range states {
		if st == nil {
			continue
		}
		switch st.Status {
		case entity.ItemRunState_Processing:
			processing++
		case entity.ItemRunState_Queueing:
			queueingIDs = append(queueingIDs, st.ItemID)
		}
	}
	return processing, queueingIDs
}

func expectedReservations(expt *entity.CentralActiveExperiment, states []*entity.CentralItemState, perItem entity.ResourceVector, ownerEnv string) []*entity.Reservation {
	var out []*entity.Reservation
	for _, st := range states {
		if st == nil || st.Status != entity.ItemRunState_Processing {
			continue
		}
		out = append(out, &entity.Reservation{
			ID:        entity.ReservationID(expt.ExptRunID, st.ItemID),
			SpaceID:   expt.SpaceID,
			ExptID:    expt.ExptID,
			ExptRunID: expt.ExptRunID,
			ItemID:    st.ItemID,
			OwnerEnv:  ownerEnv,
			Vector:    perItem,
			State:     entity.ReservationStateDispatched,
			UpdatedAt: st.UpdatedAt,
		})
	}
	return out
}

func aggregateReservationVector(rsvs []*entity.Reservation) entity.ResourceVector {
	var v entity.ResourceVector
	for _, r := range rsvs {
		if r == nil {
			continue
		}
		v = v.Add(r.Vector)
	}
	return v
}

func sortActiveExperiments(active []*entity.CentralActiveExperiment, conf *entity.CentralSchedulerConf, now time.Time) {
	sort.SliceStable(active, func(i, j int) bool {
		a, b := active[i], active[j]
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		ap := a.Priority
		bp := b.Priority
		if conf != nil && conf.AgingPointsPerMinute > 0 {
			ap += agingPoints(a.StartAt, now) * int32(conf.AgingPointsPerMinute)
			bp += agingPoints(b.StartAt, now) * int32(conf.AgingPointsPerMinute)
		}
		if ap != bp {
			return ap > bp
		}
		if !a.StartAt.Equal(b.StartAt) {
			return a.StartAt.Before(b.StartAt)
		}
		return a.ExptID < b.ExptID
	})
}

func agingPoints(startAt, now time.Time) int32 {
	if startAt.IsZero() || now.Before(startAt) {
		return 0
	}
	return int32(now.Sub(startAt).Minutes())
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// keepLease 在 tick/rebuild 期间续租，避免 DB 扫描耗时超过 TTL 后出现双 leader。
func (s *CentralSchedulerImpl) keepLease(ctx context.Context, lease LeaderLease, interval time.Duration) func() {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				ok, err := lease.Renew(ctx)
				if err != nil || !ok {
					logs.CtxWarn(ctx, "[CentralScheduler] renew lease fail, ok: %v, err: %v", ok, err)
				}
			}
		}
	}()
	return func() { close(stop) }
}

// settle 以 DB 实际状态收尾已有 reservation：
//   - run 已结束/不存在 → 释放整个 run；
//   - item 已成功/失败/终止 → 释放；
//   - item 仍在 Processing → 标记 dispatched 并保留；
//   - item 仍 Queueing 且超过结算窗口 → 视为“发出后情况不明且无执行证据”，释放并允许重派。
func (s *CentralSchedulerImpl) settle(ctx context.Context, ownerEnv string, conf *entity.CentralSchedulerConf) error {
	rsvs, err := s.Ledger.List(ctx, ownerEnv)
	if err != nil {
		return err
	}
	type runKey struct {
		spaceID, exptID, runID int64
	}
	byRun := make(map[runKey][]*entity.Reservation)
	for _, r := range rsvs {
		if r == nil || r.State == entity.ReservationStateReleased {
			continue
		}
		k := runKey{spaceID: r.SpaceID, exptID: r.ExptID, runID: r.ExptRunID}
		byRun[k] = append(byRun[k], r)
	}

	now := s.now()
	settleWindow := conf.SettleWindow()
	for k, group := range byRun {
		status, found, rerr := s.Repo.GetRunStatus(ctx, k.spaceID, k.exptID, k.runID)
		if rerr != nil {
			logs.CtxWarn(ctx, "[CentralScheduler] get run status fail, run_id: %v, err: %v", k.runID, rerr)
			continue
		}
		if !found || entity.IsExptFinished(status) || entity.IsExptFinishing(status) {
			if _, rerr := s.Ledger.ReleaseRun(ctx, ownerEnv, k.runID); rerr != nil {
				logs.CtxWarn(ctx, "[CentralScheduler] release ended run fail, run_id: %v, err: %v", k.runID, rerr)
			}
			continue
		}

		states, rerr := s.Repo.ListRunItemStates(ctx, k.spaceID, k.exptID, k.runID)
		if rerr != nil {
			logs.CtxWarn(ctx, "[CentralScheduler] list run item states fail, run_id: %v, err: %v", k.runID, rerr)
			continue
		}
		stateByItem := make(map[int64]*entity.CentralItemState, len(states))
		for _, st := range states {
			if st != nil {
				stateByItem[st.ItemID] = st
			}
		}

		for _, r := range group {
			st, ok := stateByItem[r.ItemID]
			switch {
			case !ok:
				// item 已不存在（删除/重建），没有执行证据，释放。
				_, _ = s.Ledger.Release(ctx, ownerEnv, r.ID)
			case entity.IsItemRunFinished(st.Status):
				_, _ = s.Ledger.Release(ctx, ownerEnv, r.ID)
			case st.Status == entity.ItemRunState_Processing:
				if r.State != entity.ReservationStateDispatched {
					_ = s.Ledger.SetState(ctx, ownerEnv, r.ID, entity.ReservationStateDispatched)
				}
			case st.Status == entity.ItemRunState_Queueing:
				if !r.UpdatedAt.IsZero() && now.Sub(r.UpdatedAt) >= settleWindow {
					if _, rerr := s.Ledger.Release(ctx, ownerEnv, r.ID); rerr != nil {
						logs.CtxWarn(ctx, "[CentralScheduler] release unknown dispatch fail, key: %v, err: %v", r.ID, rerr)
					}
				}
			}
		}
	}
	return nil
}

// Rebuild 以 DB 为准重建账本。
func (s *CentralSchedulerImpl) Rebuild(ctx context.Context) error {
	conf := s.conf(ctx)
	if conf == nil || !conf.Enabled {
		return nil
	}
	ownerEnv := entity.NormalizeOwnerEnv(s.OwnerEnv)
	if ownerEnv == "" {
		return fmt.Errorf("central scheduler owner env is empty")
	}
	lease, acquired, err := s.Leader.Acquire(ctx, ownerEnv, conf.LeaseTTL())
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer func() { _ = lease.Release(ctx) }()
	stopRenew := s.keepLease(ctx, lease, conf.LeaseRenew())
	defer stopRenew()

	ctx = contexts.WithCtxWriteDB(ctx)
	active, err := s.Repo.ListActiveExperiments(ctx, ownerEnv, s.maxActive())
	if err != nil {
		return err
	}
	var expected []*entity.Reservation
	for _, expt := range active {
		if expt == nil || expt.OwnerEnv != ownerEnv {
			continue
		}
		states, err := s.Repo.ListRunItemStates(ctx, expt.SpaceID, expt.ExptID, expt.ExptRunID)
		if err != nil {
			return err
		}
		expected = append(expected, expectedReservations(expt, states, expt.ResourcePerItem(), ownerEnv)...)
	}
	return s.Ledger.Rebuild(ctx, ownerEnv, conf.Capacity(ownerEnv), expected)
}

// ledgerDispatchGate 把本次已预占的 item 放行给现有 handleToSubmits。
type ledgerDispatchGate struct {
	ownerEnv       string
	exptID         int64
	exptRunID      int64
	ledger         RateLedger
	reservedByItem map[int64]*entity.Reservation
	granted        map[int64]bool
}

func (g *ledgerDispatchGate) Filter(ctx context.Context, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem) ([]*entity.ExptEvalItem, error) {
	granted := make([]*entity.ExptEvalItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, ok := g.reservedByItem[item.ItemID]; !ok {
			continue
		}
		g.granted[item.ItemID] = true
		granted = append(granted, item)
	}
	return granted, nil
}

func (g *ledgerDispatchGate) OnCompleted(ctx context.Context, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem) error {
	var firstErr error
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, err := g.ledger.Release(ctx, g.ownerEnv, entity.ReservationID(g.exptRunID, item.ItemID)); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
