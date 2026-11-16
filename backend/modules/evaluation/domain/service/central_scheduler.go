// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// ErrDefiniteDispatchFailure 表示派发在发送前确定失败（可立即释放）；
// 其它 dispatch error 视为“结果不明”，进入待补偿由对账处理。
var ErrDefiniteDispatchFailure = repo.ErrDefiniteDispatchFailure

// CentralScheduler 中心调度主链。DB 状态只是投影，Redis 预占记录是额度唯一真值。
type CentralScheduler struct {
	cfg         component.ICentralSchedulingConfigProvider
	env         component.IEnvironmentProvider
	ledger      repo.ICentralLedgerRepo
	active      repo.ICentralActiveRepo
	exptRepo    repo.ICentralExperimentRepo
	initializer SchedulerModeFactory
	dispatcher  repo.ICentralItemDispatcher
	owner       string
}

func NewCentralScheduler(
	cfg component.ICentralSchedulingConfigProvider,
	env component.IEnvironmentProvider,
	ledger repo.ICentralLedgerRepo,
	active repo.ICentralActiveRepo,
	exptRepo repo.ICentralExperimentRepo,
	initializer SchedulerModeFactory,
	dispatcher repo.ICentralItemDispatcher,
) *CentralScheduler {
	host, _ := os.Hostname()
	owner := fmt.Sprintf("central-%s-%d", host, os.Getpid())
	return &CentralScheduler{cfg: cfg, env: env, ledger: ledger, active: active, exptRepo: exptRepo, initializer: initializer, dispatcher: dispatcher, owner: owner}
}

// SettlementDeps 供消费者侧按 DB 投影确认/释放预占。
func (s *CentralScheduler) SettlementDeps() CentralItemSettlementDeps {
	return CentralItemSettlementDeps{Ledger: s.ledger, Env: s.env, ItemRepo: s.exptRepo}
}

func (s *CentralScheduler) CurrentEnvID(ctx context.Context) (string, error) {
	return s.env.CurrentEnvID(ctx)
}

func (s *CentralScheduler) Authorized(ctx context.Context, operator string, spaceID int64) bool {
	return s.cfg.IsAuthorized(ctx, operator, spaceID)
}

func (s *CentralScheduler) config(ctx context.Context) (*entity.CentralSchedulingConfig, string, error) {
	cfg, err := s.cfg.GetCentralSchedulingConfig(ctx)
	if err != nil {
		return nil, "", err
	}
	if cfg == nil {
		cfg = entity.DefaultCentralSchedulingConfig()
	}
	env, err := s.env.CurrentEnvID(ctx)
	if err != nil {
		return nil, "", err
	}
	if env == "" {
		return nil, "", errors.New("central scheduling env id is empty")
	}
	return cfg, env, nil
}

// Run 以分钟级 tick 运行；不依赖旧 MQ daemon 在线。
func (s *CentralScheduler) Run(ctx context.Context) {
	cfg, _, err := s.config(ctx)
	if err != nil {
		logs.CtxError(ctx, "[CentralScheduler] load config fail, stop: %v", err)
		return
	}
	if !cfg.Enabled {
		logs.CtxInfo(ctx, "[CentralScheduler] disabled, skip run")
		return
	}
	// 重启恢复：先做活跃集增量重建，再进入 tick/周期对账。
	if err := s.Rebuild(ctx); err != nil {
		logs.CtxWarn(ctx, "[CentralScheduler] initial rebuild err: %v", err)
	}
	tick := time.NewTicker(cfg.TickInterval)
	reconcile := time.NewTicker(cfg.ReconcileInterval)
	defer tick.Stop()
	defer reconcile.Stop()
	for {
		select {
		case <-ctx.Done():
			s.cleanup(ctx)
			return
		case <-tick.C:
			if err := s.Tick(ctx); err != nil {
				logs.CtxWarn(ctx, "[CentralScheduler] tick err: %v", err)
			}
		case <-reconcile.C:
			if err := s.Reconcile(ctx); err != nil {
				logs.CtxWarn(ctx, "[CentralScheduler] reconcile err: %v", err)
			}
		}
	}
}

func (s *CentralScheduler) cleanup(ctx context.Context) {
	_, env, err := s.config(ctx)
	if err == nil {
		// 退出只放弃 leader，不清理共享活跃集，避免影响其它副本。
		_ = s.active.ReleaseLeader(ctx, env, s.owner)
	}
}

// Activate 实验进入活跃集，并幂等初始化本次 attempt 的 pending 项。
func (s *CentralScheduler) Activate(ctx context.Context, expt *entity.Experiment, runMode entity.ExptRunMode) error {
	if expt == nil {
		return errors.New("nil experiment")
	}
	meta := entity.GetSchedulingMeta(expt.EvalConf)
	if !meta.Mode.IsCentralDispatchMode() && meta.Mode != entity.SchedulingModeCentralShadow {
		return fmt.Errorf("experiment %d mode %s not central", expt.ID, meta.Mode)
	}
	cfg, env, err := s.config(ctx)
	if err != nil {
		return err
	}
	if expt.LatestRunID <= 0 {
		return fmt.Errorf("experiment %d has no run id", expt.ID)
	}
	if meta.Env != "" && meta.Env != env {
		return fmt.Errorf("experiment %d env %s != current %s", expt.ID, meta.Env, env)
	}
	if meta.Env == "" {
		return fmt.Errorf("experiment %d env unknown, fail-closed", expt.ID)
	}
	if s.initializer != nil {
		if err := s.initializer.EnsureRunInitialized(ctx, expt.ID, expt.LatestRunID, expt.SpaceID, runMode, nil); err != nil {
			return err
		}
	}
	wait := meta.WaitSinceMS
	if wait <= 0 {
		wait = entity.NowUnixMS()
	}
	return s.active.AddActive(ctx, env, &entity.ActiveExperiment{
		ExptID: expt.ID, RunID: expt.LatestRunID, SpaceID: expt.SpaceID,
		RunMode: runMode, Attempt: entity.NowUnixMS(), Priority: meta.Priority,
		WaitSinceMS: wait, Mode: meta.Mode, Env: meta.Env,
	}, cfg.ReservationTTL)
}

// Deactivate 移出活跃集并释放该 attempt 全部维度。
func (s *CentralScheduler) Deactivate(ctx context.Context, exptID, runID int64, reason string) error {
	_, env, err := s.config(ctx)
	if err != nil {
		return err
	}
	_ = s.active.RemoveActive(ctx, env, exptID, runID)
	return s.ReleaseRun(ctx, env, exptID, runID, reason)
}

// ReleaseRun 一次释放该 run 的全部维度；重复调用幂等。
func (s *CentralScheduler) ReleaseRun(ctx context.Context, env string, exptID, runID int64, reason string) error {
	resvs, err := s.ledger.ListByRun(ctx, env, exptID, runID)
	if err != nil {
		return err
	}
	for _, r := range resvs {
		if r == nil || r.State.IsTerminal() {
			continue
		}
		if _, err := s.ledger.Release(ctx, env, r.ID, reason); err != nil {
			logs.CtxWarn(ctx, "[CentralScheduler] release %s fail: %v", r.ID, err)
		}
	}
	return nil
}

func (s *CentralScheduler) ConfirmItem(ctx context.Context, env, resvID string) (entity.ReservationState, error) {
	return s.ledger.Confirm(ctx, env, resvID)
}

func (s *CentralScheduler) ReleaseItem(ctx context.Context, env, resvID, reason string) (entity.ReservationState, error) {
	return s.ledger.Release(ctx, env, resvID, reason)
}

// Tick 一次扫描/排序/预占/派发。
func (s *CentralScheduler) Tick(ctx context.Context) error {
	cfg, env, err := s.config(ctx)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}
	if !s.ensureLeader(ctx, cfg, env) {
		return nil
	}
	activeList, err := s.active.ListActive(ctx, env, 1000)
	if err != nil {
		return err
	}
	if len(activeList) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(activeList))
	for _, a := range activeList {
		ids = append(ids, a.ExptID)
	}
	expts, err := s.exptRepo.MGetBasicByID(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[int64]*entity.Experiment, len(expts))
	for _, e := range expts {
		byID[e.ID] = e
	}
	type candidate struct {
		active *entity.ActiveExperiment
		expt   *entity.Experiment
		meta   entity.SchedulingMeta
	}
	cands := make([]*candidate, 0, len(activeList))
	for _, a := range activeList {
		expt := byID[a.ExptID]
		if expt == nil {
			_ = s.active.RemoveActive(ctx, env, a.ExptID, a.RunID)
			continue
		}
		meta := entity.GetSchedulingMeta(expt.EvalConf)
		if meta.Mode != entity.SchedulingModeCentral && meta.Mode != entity.SchedulingModeCentralShadow {
			_ = s.active.RemoveActive(ctx, env, a.ExptID, a.RunID)
			continue
		}
		if meta.Env != env || expt.LatestRunID != a.RunID {
			_ = s.active.RemoveActive(ctx, env, a.ExptID, a.RunID)
			continue
		}
		if expt.Status != entity.ExptStatus_Pending && expt.Status != entity.ExptStatus_Processing {
			_ = s.active.RemoveActive(ctx, env, a.ExptID, a.RunID)
			continue
		}
		cands = append(cands, &candidate{active: a, expt: expt, meta: meta})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].meta.Priority != cands[j].meta.Priority {
			return cands[i].meta.Priority > cands[j].meta.Priority
		}
		return cands[i].active.WaitSinceMS < cands[j].active.WaitSinceMS
	})
	budget := cfg.MaxDispatchPerTick
	for _, c := range cands {
		if budget <= 0 {
			break
		}
		planned, err := s.planExperiment(ctx, cfg, env, c.expt, c.meta, c.active)
		if err != nil {
			logs.CtxWarn(ctx, "[CentralScheduler] plan expt %d fail: %v", c.expt.ID, err)
			continue
		}
		if c.meta.Mode == entity.SchedulingModeCentralShadow {
			logs.CtxInfo(ctx, "[CentralScheduler][shadow] expt=%d run=%d priority=%d inflight=%d deficit=%d items=%v",
				c.expt.ID, c.active.RunID, c.meta.Priority, planned.inflight, planned.deficit, planned.items)
			continue
		}
		for _, itemID := range planned.items {
			if budget <= 0 {
				break
			}
			dispatch, err := s.dispatchOne(ctx, cfg, env, c.expt, c.active, itemID)
			if err != nil {
				logs.CtxWarn(ctx, "[CentralScheduler] dispatch expt=%d item=%d fail: %v", c.expt.ID, itemID, err)
				continue
			}
			if dispatch {
				budget--
			}
		}
	}
	return nil
}

type plannedExperiment struct {
	inflight int
	deficit  int
	items    []int64
}

func (s *CentralScheduler) planExperiment(ctx context.Context, cfg *entity.CentralSchedulingConfig, env string, expt *entity.Experiment, meta entity.SchedulingMeta, active *entity.ActiveExperiment) (*plannedExperiment, error) {
	target := cfg.MaxInflightPerExpt
	if expt.EvalConf != nil && expt.EvalConf.ItemConcurNum != nil && *expt.EvalConf.ItemConcurNum > 0 && *expt.EvalConf.ItemConcurNum < target {
		target = *expt.EvalConf.ItemConcurNum
	}
	if target <= 0 {
		target = 1
	}
	resvs, err := s.ledger.ListByRun(ctx, env, expt.ID, active.RunID)
	if err != nil {
		return nil, err
	}
	reserved := make(map[int64]bool, len(resvs))
	inflight := 0
	for _, r := range resvs {
		if r == nil || r.State.IsTerminal() {
			continue
		}
		reserved[r.ItemID] = true
		inflight++
	}
	deficit := target - inflight
	if deficit <= 0 {
		return &plannedExperiment{inflight: inflight, deficit: 0}, nil
	}
	items, err := s.exptRepo.ScanQueueingItems(ctx, expt.ID, active.RunID, expt.SpaceID, deficit)
	if err != nil {
		return nil, err
	}
	filtered := items[:0]
	for _, id := range items {
		if !reserved[id] {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) > deficit {
		filtered = filtered[:deficit]
	}
	return &plannedExperiment{inflight: inflight, deficit: deficit, items: filtered}, nil
}

func (s *CentralScheduler) dispatchOne(ctx context.Context, cfg *entity.CentralSchedulingConfig, env string, expt *entity.Experiment, active *entity.ActiveExperiment, itemID int64) (bool, error) {
	vector := s.resourceVector(cfg, expt)
	enabled, capacity, err := s.dimensionState(ctx, cfg, vector)
	if err != nil {
		return false, err
	}
	attempt := active.Attempt
	if attempt <= 0 {
		attempt = entity.NowUnixMS()
	}
	resv := &entity.Reservation{
		ID:  fmt.Sprintf("%s-%d-%d-%d-%d-%d", env, expt.ID, active.RunID, itemID, attempt, entity.NowUnixMS()),
		Env: env, SpaceID: expt.SpaceID, ExptID: expt.ID, RunID: active.RunID, ItemID: itemID,
		Attempt: attempt, Vector: vector, Owner: s.owner, CreatedAt: entity.NowUnixMS(),
	}
	if _, err := s.ledger.Reserve(ctx, &repo.ReserveRequest{Reservation: resv, Enabled: enabled, Capacity: capacity, TTL: cfg.ReservationTTL}); err != nil {
		if errors.Is(err, repo.ErrReservationExists) {
			// 已有非终态预占，说明该 item 已在飞；本轮跳过即可。
			return false, nil
		}
		return false, err
	}
	if err := s.exptRepo.MarkItemsProcessing(ctx, expt.ID, active.RunID, expt.SpaceID, []int64{itemID}); err != nil {
		_, _ = s.ledger.Release(ctx, env, resv.ID, "db_projection_failed")
		return false, err
	}
	event := &entity.ExptItemEvalEvent{
		SpaceID: expt.SpaceID, ExptID: expt.ID, ExptRunID: active.RunID, ExptRunMode: active.RunMode,
		EvalSetItemID: itemID, CreateAt: entity.NowUnixMS(),
		CentralReservationID: resv.ID, CentralEnv: env, CentralMode: string(active.Mode),
	}
	if s.dispatcher == nil {
		_, _ = s.ledger.Release(ctx, env, resv.ID, "no_dispatcher")
		_ = s.exptRepo.MarkItemsQueueing(ctx, expt.ID, active.RunID, expt.SpaceID, []int64{itemID})
		return false, errors.New("central dispatcher not configured")
	}
	if err := s.dispatcher.Dispatch(ctx, event); err != nil {
		if errors.Is(err, ErrDefiniteDispatchFailure) {
			_, _ = s.ledger.Release(ctx, env, resv.ID, "dispatch_definite_failure")
			_ = s.exptRepo.MarkItemsQueueing(ctx, expt.ID, active.RunID, expt.SpaceID, []int64{itemID})
			return false, err
		}
		// 结果不明：不判成功也不判失败，进入待补偿。
		_, _ = s.ledger.MarkPendingCompensation(ctx, env, resv.ID)
		return false, err
	}
	_, _ = s.ledger.MarkPendingCompensation(ctx, env, resv.ID)
	return true, nil
}

func (s *CentralScheduler) resourceVector(cfg *entity.CentralSchedulingConfig, expt *entity.Experiment) entity.ResourceVector {
	v := entity.ResourceVector{entity.ResourceItemExec: 1}
	if expt.IsSandboxAgentTarget() {
		slots := cfg.SandboxSlotsPerItem
		if slots <= 0 {
			slots = 1
		}
		v[entity.ResourceSandboxExec] = slots
	}
	credit := cfg.CreditPerItem
	if credit > 0 {
		v[entity.ResourceModelCredit] = credit
	}
	return v
}

func (s *CentralScheduler) dimensionState(ctx context.Context, cfg *entity.CentralSchedulingConfig, vector entity.ResourceVector) (map[entity.ResourceDimension]bool, map[entity.ResourceDimension]int64, error) {
	enabled := make(map[entity.ResourceDimension]bool, len(entity.AllResourceDimensions))
	capacity := make(map[entity.ResourceDimension]int64, len(entity.AllResourceDimensions))
	for _, dim := range entity.AllResourceDimensions {
		on, cap, err := s.cfg.GetDimensionCapacity(ctx, dim)
		if err != nil {
			return nil, nil, fmt.Errorf("dimension %s capacity unknown: %w", dim, err)
		}
		if !on {
			enabled[dim] = false
			continue
		}
		if cap < 0 {
			return nil, nil, fmt.Errorf("dimension %s capacity unknown", dim)
		}
		enabled[dim] = true
		capacity[dim] = cap
	}
	return enabled, capacity, nil
}

func (s *CentralScheduler) ensureLeader(ctx context.Context, cfg *entity.CentralSchedulingConfig, env string) bool {
	ok, err := s.active.TryLeader(ctx, env, s.owner, cfg.LeaderLease)
	if err != nil {
		logs.CtxWarn(ctx, "[CentralScheduler] try leader fail: %v", err)
		return false
	}
	if ok {
		return true
	}
	renewed, err := s.active.RenewLeader(ctx, env, s.owner, cfg.LeaderLease)
	if err != nil {
		logs.CtxWarn(ctx, "[CentralScheduler] renew leader fail: %v", err)
		return false
	}
	return renewed
}

// Reconcile 周期全量对账：只确认/释放/过期，不重派发。
func (s *CentralScheduler) Reconcile(ctx context.Context) error {
	cfg, env, err := s.config(ctx)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}
	all, err := s.ledger.ListAll(ctx, env, 5000)
	if err != nil {
		return err
	}
	for _, r := range all {
		if r == nil || r.State.IsTerminal() {
			continue
		}
		s.reconcileOne(ctx, cfg, env, r)
	}
	return s.rebuildPoolFromLedger(ctx, env, cfg.ReservationTTL)
}

func (s *CentralScheduler) reconcileOne(ctx context.Context, cfg *entity.CentralSchedulingConfig, env string, r *entity.Reservation) {
	expt, err := s.exptRepo.GetByID(ctx, r.ExptID, r.SpaceID)
	if err != nil || expt == nil {
		return // DB 不可读：保持现状，下一轮再判，不猜
	}
	if entity.IsExptFinished(expt.Status) || entity.IsExptFinishing(expt.Status) {
		_ = s.ReleaseRun(ctx, env, r.ExptID, r.RunID, "reconcile_expt_terminal")
		return
	}
	states, err := s.exptRepo.GetItemStates(ctx, r.ExptID, r.RunID, r.SpaceID, []int64{r.ItemID})
	if err != nil {
		return
	}
	item := states[r.ItemID]
	now := entity.NowUnixMS()
	age := time.Duration(now-r.UpdatedAt) * time.Millisecond
	switch r.State {
	case entity.ReservationStateReserved:
		if item == entity.ItemRunState_Processing || entity.IsItemRunFinished(item) {
			// 消息实际已到达并开始执行，只是下游尚未回写状态；转为待补偿，不能释放。
			_, _ = s.ledger.MarkPendingCompensation(ctx, env, r.ID)
		} else if age > cfg.PublishTimeout {
			_, _ = s.ledger.Release(ctx, env, r.ID, "reserved_publish_timeout")
			_ = s.exptRepo.MarkItemsQueueing(ctx, r.ExptID, r.RunID, r.SpaceID, []int64{r.ItemID})
		}
	case entity.ReservationStatePendingCompensation:
		if age > cfg.AckTimeout {
			if entity.IsItemRunFinished(item) && item == entity.ItemRunState_Success {
				_, _ = s.ledger.Confirm(ctx, env, r.ID)
				_, _ = s.ledger.Release(ctx, env, r.ID, "reconcile_item_success")
			} else if entity.IsItemRunFinished(item) {
				_, _ = s.ledger.Release(ctx, env, r.ID, "reconcile_item_fail")
			} else if item == entity.ItemRunState_Queueing {
				_, _ = s.ledger.Release(ctx, env, r.ID, "compensation_no_evidence")
				_ = s.exptRepo.MarkItemsQueueing(ctx, r.ExptID, r.RunID, r.SpaceID, []int64{r.ItemID})
			}
		}
	case entity.ReservationStateConfirmed:
		if entity.IsItemRunFinished(item) {
			_, _ = s.ledger.Release(ctx, env, r.ID, "reconcile_item_terminal")
		} else if item == entity.ItemRunState_Queueing && age > cfg.CompensationTimeout {
			_, _ = s.ledger.Release(ctx, env, r.ID, "confirmed_projection_lost")
		}
	}
}

func (s *CentralScheduler) rebuildPoolFromLedger(ctx context.Context, env string, ttl time.Duration) error {
	all, err := s.ledger.ListAll(ctx, env, 10000)
	if err != nil {
		return err
	}
	used := entity.ResourceVector{}
	for _, r := range all {
		if r == nil || r.State.IsTerminal() {
			continue
		}
		used = used.Add(r.Vector)
	}
	return s.ledger.RebuildPool(ctx, env, used, ttl)
}

// Rebuild 重启后先按活跃集增量恢复，再从 DB 重建缺失活跃集/Processing 行。
func (s *CentralScheduler) Rebuild(ctx context.Context) error {
	cfg, env, err := s.config(ctx)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}
	expts, err := s.exptRepo.ListCentralActive(ctx, env, 1000)
	if err != nil {
		return err
	}
	for _, expt := range expts {
		meta := entity.GetSchedulingMeta(expt.EvalConf)
		if (meta.Mode != entity.SchedulingModeCentral && meta.Mode != entity.SchedulingModeCentralShadow) ||
			meta.Env != env || expt.LatestRunID <= 0 {
			continue
		}
		wait := meta.WaitSinceMS
		if wait <= 0 {
			wait = entity.NowUnixMS()
		}
		_ = s.active.AddActive(ctx, env, &entity.ActiveExperiment{
			ExptID: expt.ID, RunID: expt.LatestRunID, SpaceID: expt.SpaceID,
			Attempt: entity.NowUnixMS(), Priority: meta.Priority, WaitSinceMS: wait, Mode: meta.Mode, Env: meta.Env,
		}, cfg.ReservationTTL)
		// 初始化被打断时（Pod 重启等）幂等补齐本次 run 的 item 投影。
		if s.initializer != nil {
			if ierr := s.initializer.EnsureRunInitialized(ctx, expt.ID, expt.LatestRunID, expt.SpaceID, entity.EvaluationModeSubmit, nil); ierr != nil {
				logs.CtxWarn(ctx, "[CentralScheduler] rebuild init expt=%d run=%d fail: %v", expt.ID, expt.LatestRunID, ierr)
				continue
			}
		}
		if ierr := s.exptRepo.EnsureRunItems(ctx, expt); ierr != nil {
			logs.CtxWarn(ctx, "[CentralScheduler] rebuild run items expt=%d run=%d: %v", expt.ID, expt.LatestRunID, ierr)
			continue
		}
		processing, err := s.exptRepo.ScanProcessingItems(ctx, expt.ID, expt.LatestRunID, expt.SpaceID, 1000)
		if err != nil {
			continue
		}
		existing, _ := s.ledger.ListByRun(ctx, env, expt.ID, expt.LatestRunID)
		adopted := make(map[int64]bool, len(existing))
		for _, r := range existing {
			if r != nil {
				adopted[r.ItemID] = true
			}
		}
		for _, itemID := range processing {
			if adopted[itemID] {
				continue
			}
			r := &entity.Reservation{
				ID:  fmt.Sprintf("adopt:%s:%d:%d:%d", env, expt.ID, expt.LatestRunID, itemID),
				Env: env, SpaceID: expt.SpaceID, ExptID: expt.ID, RunID: expt.LatestRunID, ItemID: itemID,
				Attempt: expt.LatestRunID, Vector: s.resourceVector(cfg, expt), CreatedAt: entity.NowUnixMS(), Adopted: true,
			}
			if err := s.ledger.Adopt(ctx, r, cfg.ReservationTTL); err != nil {
				logs.CtxWarn(ctx, "[CentralScheduler] adopt %s fail: %v", r.ID, err)
			}
		}
	}
	return s.rebuildPoolFromLedger(ctx, env, cfg.ReservationTTL)
}
