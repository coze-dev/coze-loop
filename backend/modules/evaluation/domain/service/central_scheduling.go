// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// CentralItemSubmitter is the narrow write port the central scheduler needs.
// The legacy scheduler implements it by delegating to the exact same
// handleToSubmits path, so central dispatch and legacy dispatch share one
// state-transition implementation instead of forking it.
type CentralItemSubmitter interface {
	SubmitItems(ctx context.Context, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem) error
}

// CentralScheduler is the global scanner. One instance runs per environment.
type CentralScheduler interface {
	Tick(ctx context.Context) (*entity.CentralTickReport, error)
	Run(ctx context.Context)
}

// CentralSchedulerDeps is the dependency set. Everything is an interface so
// the core can be exercised with in-memory fakes and miniredis.
type CentralSchedulerDeps struct {
	Configer      component.IConfiger
	ActiveRunRepo repo.CentralActiveRunRepo
	Reservations  repo.CentralReservationStore
	Ownership     repo.CentralOwnershipStore
	Submitter     CentralItemSubmitter
	// Lease makes sure only one central scanner acts per environment even when
	// the service has several replicas. It may be nil in tests.
	Lease  CentralLease
	Clock  func() time.Time
	Logger func(ctx context.Context, format string, args ...any)
}

// CentralLease is a best-effort mutual exclusion for the scanner loop.
type CentralLease interface {
	// Acquire returns true when this caller owns the lease for ttl.
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string) error
}

// NewCentralScheduler builds the core.
func NewCentralScheduler(deps CentralSchedulerDeps) CentralScheduler {
	if deps.Clock == nil {
		deps.Clock = time.Now
	}
	if deps.Logger == nil {
		deps.Logger = logs.CtxInfo
	}
	return &centralSchedulerImpl{deps: deps}
}

type centralSchedulerImpl struct {
	deps CentralSchedulerDeps
}

func (s *centralSchedulerImpl) Run(ctx context.Context) {
	conf := s.conf(ctx)
	if !conf.Enabled || conf.Mode == entity.CentralModeLegacy {
		s.log(ctx, "[CentralScheduling] disabled or legacy mode, scanner not started")
		return
	}
	interval := conf.TickInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	s.log(ctx, "[CentralScheduling] scanner started, env=%s mode=%s interval=%s", conf.EnvNamespace, conf.Mode, interval)
	for {
		select {
		case <-ctx.Done():
			s.log(ctx, "[CentralScheduling] scanner stopped: %v", ctx.Err())
			return
		case <-ticker.C:
			if _, err := s.Tick(ctx); err != nil {
				s.log(ctx, "[CentralScheduling] tick failed: %v", err)
			}
		}
	}
}

func (s *centralSchedulerImpl) conf(ctx context.Context) *entity.CentralSchedulingConf {
	if s.deps.Configer == nil {
		return (&entity.CentralSchedulingConf{Mode: entity.CentralModeLegacy}).Normalize()
	}
	return s.deps.Configer.GetCentralSchedulingConf(ctx).Normalize()
}

func (s *centralSchedulerImpl) log(ctx context.Context, format string, args ...any) {
	if s.deps.Logger != nil {
		s.deps.Logger(ctx, format, args...)
	}
}

func (s *centralSchedulerImpl) Tick(ctx context.Context) (*entity.CentralTickReport, error) {
	conf := s.conf(ctx)
	now := s.deps.Clock()
	report := &entity.CentralTickReport{
		StartedAt: now,
		Env:       conf.EnvNamespace,
		Mode:      conf.Mode,
	}
	if !conf.Enabled || conf.Mode == entity.CentralModeLegacy {
		report.FinishedAt = s.deps.Clock()
		return report, nil
	}
	if s.deps.ActiveRunRepo == nil || s.deps.Reservations == nil || s.deps.Ownership == nil {
		return report, fmt.Errorf("central scheduler dependencies incomplete")
	}

	// The scanner lease ensures one replica plans per env. A missing lease (nil)
	// is allowed for tests / single-replica deployments.
	leaseKey := entity.CentralKeyPrefix + conf.EnvNamespace + ":lease:scanner"
	if s.deps.Lease != nil {
		acquired, err := s.deps.Lease.Acquire(ctx, leaseKey, conf.TickInterval()*3)
		if err != nil {
			return report, fmt.Errorf("acquire central scanner lease: %w", err)
		}
		if !acquired {
			report.FinishedAt = s.deps.Clock()
			return report, nil
		}
		defer func() { _ = s.deps.Lease.Release(ctx, leaseKey) }()
	}

	runs, err := s.deps.ActiveRunRepo.ListRecentRuns(ctx, now.Add(-conf.RecentRunWindow()), conf.ScanLimit)
	if err != nil {
		return report, fmt.Errorf("list recent runs: %w", err)
	}
	report.ScannedRuns = len(runs)

	var candidates []*entity.CentralRunGrant
	for _, run := range runs {
		if run == nil {
			continue
		}
		mode := conf.ModeFor(run.SpaceID, run.ExptID)
		if mode == entity.CentralModeLegacy {
			continue
		}
		owner, oerr := s.deps.Ownership.Owner(ctx, run.ExptID)
		if oerr != nil {
			report.Anomalies = append(report.Anomalies, anomaly(run, 0, entity.CentralAnomalyReserveFailed, "read ownership failed: "+oerr.Error(), now))
			continue
		}
		// Empty owner means the run predates rollout: leave it entirely to the
		// legacy loop. A foreign owner means another environment submitted it.
		if owner == "" || owner != conf.EnvNamespace {
			report.SkippedRuns++
			continue
		}
		report.OwnedRuns++

		if entity.IsExptFinished(run.RunStatus) {
			s.finishRun(ctx, conf, run, report, now)
			continue
		}
		// Draining / Terminating still have in-flight items. Keep their
		// reservations (and reconcile completions) but never refill.
		finishing := entity.IsExptFinishing(run.RunStatus)
		// Pending runs are bootstrapped by the legacy MQ event, not the centre.
		if run.RunStatus != entity.ExptStatus_Processing && !finishing {
			continue
		}

		snap, err := s.snapshot(ctx, run)
		if err != nil {
			report.Anomalies = append(report.Anomalies, anomaly(run, 0, entity.CentralAnomalyLedgerDrift, "snapshot failed: "+err.Error(), now))
			continue
		}
		expt, err := s.deps.ActiveRunRepo.LoadExperiment(ctx, run.ExptID, run.SpaceID)
		if err != nil {
			report.Anomalies = append(report.Anomalies, anomaly(run, 0, entity.CentralAnomalyLedgerDrift, "load experiment failed: "+err.Error(), now))
			continue
		}
		// Experiment-level IDL policy may downgrade an allowlisted run back to
		// legacy (or shadow). The legacy gate reads the same override, so no
		// owner key is created for a downgraded run.
		if override := entity.CentralModeFromExt(expt.Ext, mode); override != mode {
			mode = override
			if mode == entity.CentralModeLegacy {
				report.SkippedRuns++
				continue
			}
		}

		reservations, err := s.deps.Reservations.List(ctx, conf.EnvNamespace, run.ExptID, run.RunID)
		if err != nil {
			report.Anomalies = append(report.Anomalies, anomaly(run, 0, entity.CentralAnomalyLedgerDrift, "list reservations failed: "+err.Error(), now))
			continue
		}
		drift := s.reconcile(ctx, conf, run, expt, snap, reservations, report, now)
		if drift != nil {
			report.Drifts = append(report.Drifts, *drift)
		}

		if finishing {
			// Draining/Terminating: no new grants, only reconciliation above.
			continue
		}

		if mode == entity.CentralModeShadow {
			s.plan(ctx, conf, run, expt, snap, reservations, &candidates, report, now, true)
			continue
		}
		s.plan(ctx, conf, run, expt, snap, reservations, &candidates, report, now, false)
	}

	// Deterministic priority order: higher priority first, then older queue,
	// then experiment id so a tick is reproducible for a fixed DB snapshot.
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		oi, oj := candidates[i].Run.RunCreatedAt, candidates[j].Run.RunCreatedAt
		if !oi.Equal(oj) {
			return oi.Before(oj)
		}
		if candidates[i].Run.ExptID != candidates[j].Run.ExptID {
			return candidates[i].Run.ExptID < candidates[j].Run.ExptID
		}
		return candidates[i].ItemID < candidates[j].ItemID
	})
	report.PlannedGrants = len(candidates)

	if conf.Mode == entity.CentralModeShadow {
		report.FinishedAt = s.deps.Clock()
		s.log(ctx, "[CentralScheduling] shadow tick: scanned=%d owned=%d planned=%d", report.ScannedRuns, report.OwnedRuns, report.PlannedGrants)
		return report, nil
	}

	s.dispatch(ctx, conf, candidates, report, now)
	report.FinishedAt = s.deps.Clock()
	s.log(ctx, "[CentralScheduling] tick: scanned=%d owned=%d skipped=%d planned=%d dispatched=%d released=%d drifts=%d anomalies=%d",
		report.ScannedRuns, report.OwnedRuns, report.SkippedRuns, report.PlannedGrants,
		report.Dispatched, report.ReleasedOrphans, len(report.Drifts), len(report.Anomalies))
	return report, nil
}

func (s *centralSchedulerImpl) snapshot(ctx context.Context, run *entity.CentralActiveRun) (*entity.CentralRunSnapshot, error) {
	m, err := s.deps.ActiveRunRepo.Snapshots(ctx, []*entity.CentralActiveRun{run})
	if err != nil {
		return nil, err
	}
	if snap, ok := m[centralRunKey(run.ExptID, run.RunID)]; ok && snap != nil {
		return snap, nil
	}
	return &entity.CentralRunSnapshot{}, nil
}

func (s *centralSchedulerImpl) finishRun(ctx context.Context, conf *entity.CentralSchedulingConf, run *entity.CentralActiveRun, report *entity.CentralTickReport, now time.Time) {
	released, err := s.deps.Reservations.ReleaseRun(ctx, conf.EnvNamespace, run.ExptID, run.RunID)
	if err != nil {
		report.Anomalies = append(report.Anomalies, anomaly(run, 0, entity.CentralAnomalyReleaseFailed, "release finished run: "+err.Error(), now))
		return
	}
	report.ReleasedOrphans += released
	report.FinishedRuns++
	// Remove ownership only after the run is finished, so a retry that arrives
	// later can claim again with its own environment.
	if err := s.deps.Ownership.Release(ctx, run.ExptID, conf.EnvNamespace); err != nil {
		report.Anomalies = append(report.Anomalies, anomaly(run, 0, entity.CentralAnomalyReleaseFailed, "release ownership: "+err.Error(), now))
	}
}

// reconcile releases Redis-only reservations and adopts DB-processing rows
// that lost their reservation (Redis eviction / restart). Returns a drift
// record when counts disagree.
func (s *centralSchedulerImpl) reconcile(ctx context.Context, conf *entity.CentralSchedulingConf, run *entity.CentralActiveRun, expt *entity.CentralExperimentScheduling, snap *entity.CentralRunSnapshot, reservations []*entity.CentralReservation, report *entity.CentralTickReport, now time.Time) *entity.CentralDrift {
	drift := &entity.CentralDrift{ExptID: run.ExptID, SpaceID: run.SpaceID, RunID: run.RunID}

	itemIDs := make([]int64, 0, len(reservations))
	for _, r := range reservations {
		if r != nil {
			itemIDs = append(itemIDs, r.ItemID)
		}
	}
	states := map[int64]entity.ItemRunState{}
	if len(itemIDs) > 0 {
		if got, err := s.deps.ActiveRunRepo.ItemStates(ctx, run, itemIDs); err == nil {
			states = got
		}
	}

	heldByItem := make(map[int64]*entity.CentralReservation, len(reservations))
	for _, r := range reservations {
		if r == nil {
			continue
		}
		heldByItem[r.ItemID] = r
		state, ok := states[r.ItemID]
		// A reservation whose item already became terminal, disappeared, or is
		// still queueing after dispatch has no live DB owner. Release it so the
		// next plan can re-dispatch idempotently.
		live := ok && state == entity.ItemRunState_Processing
		if live {
			continue
		}
		if released, err := s.deps.Reservations.Release(ctx, conf.EnvNamespace, r.ID); err != nil {
			report.Anomalies = append(report.Anomalies, anomaly(run, r.ItemID, entity.CentralAnomalyReleaseFailed, "release orphan: "+err.Error(), now))
		} else if released {
			report.ReleasedOrphans++
			drift.OrphanReservations = append(drift.OrphanReservations, r.ID)
			delete(heldByItem, r.ItemID)
		}
	}

	// DB processing rows must have a reservation. Adopt missing ones so the
	// ledger charges the resources they are actually consuming.
	processingIDs, err := s.processingItemIDs(ctx, run, len(reservations)+snap.InFlight)
	if err == nil {
		for _, id := range processingIDs {
			if _, ok := heldByItem[id]; ok {
				continue
			}
			claims := expt.ResourceDemand()
			req := &entity.CentralReserveRequest{
				Env:           conf.EnvNamespace,
				ExptID:        run.ExptID,
				SpaceID:       run.SpaceID,
				RunID:         run.RunID,
				ItemID:        id,
				ReservationID: entity.CentralReservationID(run.ExptID, run.RunID, id),
				Claims:        claims,
				Capacities:    s.enforcedCapacities(conf, claims),
				TTL:           conf.ReservationTTL(),
				Adopted:       true,
			}
			res, rerr := s.deps.Reservations.Reserve(ctx, req)
			if rerr != nil {
				drift.UnreservedProcessing = append(drift.UnreservedProcessing, id)
				report.Anomalies = append(report.Anomalies, anomaly(run, id, entity.CentralAnomalyReservationLost, "adopt reservation failed: "+rerr.Error(), now))
				continue
			}
			if !res.Granted {
				drift.UnreservedProcessing = append(drift.UnreservedProcessing, id)
				report.Anomalies = append(report.Anomalies, anomaly(run, id, entity.CentralAnomalyReservationLost, "adopt reservation denied: "+res.ConstrainedBy, now))
				continue
			}
			heldByItem[id] = &entity.CentralReservation{ItemID: id, Claims: claims}
		}
	}

	drift.DBProcessing = snap.InFlight
	drift.RedisHeld = len(heldByItem)
	if drift.RedisHeld != drift.DBProcessing {
		report.Anomalies = append(report.Anomalies, anomaly(run, 0, entity.CentralAnomalyLedgerDrift,
			fmt.Sprintf("db_processing=%d redis_held=%d", drift.DBProcessing, drift.RedisHeld), now))
		if conf.RebuildOnDrift {
			drift.Rebuilt = s.rebuild(ctx, conf, run, expt, report, now)
		}
	}

	if len(heldByItem) > 0 {
		if err := s.deps.Reservations.RenewRun(ctx, conf.EnvNamespace, run.ExptID, run.RunID, conf.ReservationTTL()); err != nil {
			report.Anomalies = append(report.Anomalies, anomaly(run, 0, entity.CentralAnomalyReleaseFailed, "renew run: "+err.Error(), now))
		}
	}
	return drift
}

// rebuild drops the Redis ledger for the run and re-adopts MySQL processing
// rows. MySQL is the source of truth for "these items are actually running".
func (s *centralSchedulerImpl) rebuild(ctx context.Context, conf *entity.CentralSchedulingConf, run *entity.CentralActiveRun, expt *entity.CentralExperimentScheduling, report *entity.CentralTickReport, now time.Time) bool {
	if _, err := s.deps.Reservations.ReleaseRun(ctx, conf.EnvNamespace, run.ExptID, run.RunID); err != nil {
		report.Anomalies = append(report.Anomalies, anomaly(run, 0, entity.CentralAnomalyReleaseFailed, "rebuild release: "+err.Error(), now))
		return false
	}
	ids, err := s.processingItemIDs(ctx, run, conf.ScanLimit)
	if err != nil {
		report.Anomalies = append(report.Anomalies, anomaly(run, 0, entity.CentralAnomalyLedgerDrift, "rebuild scan: "+err.Error(), now))
		return false
	}
	claims := expt.ResourceDemand()
	for _, id := range ids {
		_, err := s.deps.Reservations.Reserve(ctx, &entity.CentralReserveRequest{
			Env:           conf.EnvNamespace,
			ExptID:        run.ExptID,
			SpaceID:       run.SpaceID,
			RunID:         run.RunID,
			ItemID:        id,
			ReservationID: entity.CentralReservationID(run.ExptID, run.RunID, id),
			Claims:        claims,
			Capacities:    s.enforcedCapacities(conf, claims),
			TTL:           conf.ReservationTTL(),
			Adopted:       true,
		})
		if err != nil {
			report.Anomalies = append(report.Anomalies, anomaly(run, id, entity.CentralAnomalyReservationLost, "rebuild adopt: "+err.Error(), now))
		}
	}
	s.log(ctx, "[CentralScheduling] rebuilt ledger for expt=%d run=%d processing=%d", run.ExptID, run.RunID, len(ids))
	return true
}

func (s *centralSchedulerImpl) processingItemIDs(ctx context.Context, run *entity.CentralActiveRun, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = entity.CentralDefaultScanLimit
	}
	return s.deps.ActiveRunRepo.ProcessingItemIDs(ctx, run, limit)
}

// plan builds grants for one run. It is side-effect free in shadow mode.
func (s *centralSchedulerImpl) plan(ctx context.Context, conf *entity.CentralSchedulingConf, run *entity.CentralActiveRun, expt *entity.CentralExperimentScheduling, snap *entity.CentralRunSnapshot, reservations []*entity.CentralReservation, out *[]*entity.CentralRunGrant, report *entity.CentralTickReport, now time.Time, shadow bool) {
	desired := conf.DefaultItemConcurNum
	if expt != nil && expt.ItemConcurNum > 0 {
		desired = expt.ItemConcurNum
	}
	if expt != nil {
		if cap, ok := entity.CentralMaxConcurFromExt(expt.Ext); ok && cap < desired {
			desired = cap
		}
	}
	if desired > conf.MaxItemsPerExperiment {
		desired = conf.MaxItemsPerExperiment
	}
	if desired < 1 {
		desired = 1
	}
	inFlight := snap.InFlight
	gap := desired - inFlight
	if gap <= 0 {
		return
	}

	priority := entity.CentralPriorityBase(run, expt)
	if override, ok := conf.ExplicitPriority(run.ExptID); ok {
		priority = override
	}
	if expt != nil {
		if override, ok := entity.CentralPriorityFromExt(expt.Ext); ok {
			priority = override
		}
	}
	priority = entity.CentralAgedPriority(priority, gptr.Indirect(snap.OldestQueuedAt), now, conf)

	ids, err := s.deps.ActiveRunRepo.QueueingItemIDs(ctx, run, gap)
	if err != nil {
		report.Anomalies = append(report.Anomalies, anomaly(run, 0, entity.CentralAnomalyLedgerDrift, "scan queueing: "+err.Error(), now))
		return
	}
	claims := expt.ResourceDemand()
	for _, id := range ids {
		*out = append(*out, &entity.CentralRunGrant{
			Run:      run,
			ItemID:   id,
			Claims:   claims.Clone(),
			Priority: priority,
			Desired:  desired,
			InFlight: inFlight,
			Queueing: snap.Queueing,
		})
	}
	_ = shadow
}

func (s *centralSchedulerImpl) dispatch(ctx context.Context, conf *entity.CentralSchedulingConf, grants []*entity.CentralRunGrant, report *entity.CentralTickReport, now time.Time) {
	spaceGrants := map[int64]int{}
	itemCapacity, itemCapacitySet := conf.PoolCapacity(entity.CentralResourceItem)
	perSpaceCap := 0
	if itemCapacitySet && itemCapacity > 0 {
		perSpaceCap = int(float64(itemCapacity) * conf.FairShareRatio)
		if perSpaceCap < 1 {
			perSpaceCap = 1
		}
	}

	for _, g := range grants {
		if g == nil || g.Run == nil {
			continue
		}
		if perSpaceCap > 0 && spaceGrants[g.Run.SpaceID] >= perSpaceCap {
			continue
		}
		req := &entity.CentralReserveRequest{
			Env:           conf.EnvNamespace,
			ExptID:        g.Run.ExptID,
			SpaceID:       g.Run.SpaceID,
			RunID:         g.Run.RunID,
			ItemID:        g.ItemID,
			ReservationID: entity.CentralReservationID(g.Run.ExptID, g.Run.RunID, g.ItemID),
			Claims:        g.Claims,
			Capacities:    s.enforcedCapacities(conf, g.Claims),
			TTL:           conf.ReservationTTL(),
		}
		res, err := s.deps.Reservations.Reserve(ctx, req)
		if err != nil {
			report.Anomalies = append(report.Anomalies, anomaly(g.Run, g.ItemID, entity.CentralAnomalyReserveFailed, err.Error(), now))
			continue
		}
		if !res.Granted {
			if res.ConstrainedBy == entity.CentralResourceItem {
				// Global item pool is exhausted: no lower-priority experiment
				// can make progress either, so stop the tick here.
				break
			}
			report.Anomalies = append(report.Anomalies, anomaly(g.Run, g.ItemID, entity.CentralAnomalyReserveFailed, "constrained by "+res.ConstrainedBy, now))
			continue
		}
		report.ReservedGrants++

		if s.deps.Submitter == nil {
			// No submitter means we cannot safely hold quota.
			if _, rerr := s.deps.Reservations.Release(ctx, conf.EnvNamespace, req.ReservationID); rerr != nil {
				report.Anomalies = append(report.Anomalies, anomaly(g.Run, g.ItemID, entity.CentralAnomalyReleaseFailed, rerr.Error(), now))
			}
			report.Anomalies = append(report.Anomalies, anomaly(g.Run, g.ItemID, entity.CentralAnomalyDispatchFailed, "submitter not configured", now))
			continue
		}

		event := &entity.ExptScheduleEvent{
			SpaceID:     g.Run.SpaceID,
			ExptID:      g.Run.ExptID,
			ExptRunID:   g.Run.RunID,
			ExptRunMode: g.Run.RunMode,
			ExptType:    g.Run.ExptType,
			CreatedAt:   now.Unix(),
			Env:         conf.EnvNamespace,
			Session: &entity.Session{
				UserID: g.Run.CreatedBy,
				AppID:  conf.DefaultAppID,
			},
		}
		items := []*entity.ExptEvalItem{{ExptID: g.Run.ExptID, ItemID: g.ItemID, State: entity.ItemRunState_Queueing}}
		if err := s.deps.Submitter.SubmitItems(ctx, event, items); err != nil {
			// Publish failed: release the reservation so the item can be
			// retried next tick. The item consumer is idempotent, so a partial
			// publish with a failed DB update is safe to retry.
			if _, rerr := s.deps.Reservations.Release(ctx, conf.EnvNamespace, req.ReservationID); rerr != nil {
				report.Anomalies = append(report.Anomalies, anomaly(g.Run, g.ItemID, entity.CentralAnomalyReleaseFailed, rerr.Error(), now))
			}
			report.Anomalies = append(report.Anomalies, anomaly(g.Run, g.ItemID, entity.CentralAnomalyDispatchFailed, err.Error(), now))
			continue
		}
		if err := s.deps.Reservations.MarkDispatched(ctx, conf.EnvNamespace, req.ReservationID); err != nil {
			report.Anomalies = append(report.Anomalies, anomaly(g.Run, g.ItemID, entity.CentralAnomalyReleaseFailed, "mark dispatched: "+err.Error(), now))
		}
		report.Dispatched++
		spaceGrants[g.Run.SpaceID]++
	}
}

// enforcedCapacities intersects the configured pool with the claim vector.
// Scoped capacities are only enforced when configured.
func (s *centralSchedulerImpl) enforcedCapacities(conf *entity.CentralSchedulingConf, claims entity.CentralResourceClaims) map[string]int64 {
	out := make(map[string]int64, len(claims))
	for k := range claims {
		if cap, ok := conf.PoolCapacity(k); ok {
			out[k] = cap
			continue
		}
		if cap, ok := conf.ScopedPoolCapacity(k); ok {
			out[k] = cap
		}
	}
	return out
}

func centralRunKey(exptID, runID int64) string {
	return strconv.FormatInt(exptID, 10) + ":" + strconv.FormatInt(runID, 10)
}

func anomaly(run *entity.CentralActiveRun, itemID int64, kind, msg string, now time.Time) entity.CentralAnomaly {
	a := entity.CentralAnomaly{ItemID: itemID, Kind: kind, Message: msg, At: now}
	if run != nil {
		a.ExptID = run.ExptID
		a.RunID = run.RunID
	}
	return a
}
