// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"fmt"
	"time"
)

// TickResult is the observable outcome of one central scheduling pass.
type TickResult struct {
	Scope          Scope
	RunsScanned    int
	RunsRegistered int
	RunsDriven     int
	RunsFailed     int
	RunsRetired    int
	Released       int
	LedgerRebuilt  bool
	Errors         []string
	StartedAt      time.Time
	FinishedAt     time.Time
}

// Scheduler periodically scans active runs, orders them by priority, and drives
// the existing per-experiment scheduling chain. The actual item-level resource
// admission happens in Admission, immediately before item events are published.
type Scheduler struct {
	Store  Store
	Source RunSource
	Driver Driver
	Limits Limits
	Clock  Clock

	// ReconcileEvery controls how often a drift rebuild is allowed. 0 means every tick.
	ReconcileEvery int
	// RunTTL bounds run registrations so a crashed process cannot leave a run
	// registered forever.
	RunTTL time.Duration

	tickCount int
}

func (s *Scheduler) limits() Limits {
	l := s.Limits
	l = l.withDefaults()
	if s.RunTTL <= 0 {
		s.RunTTL = 2 * l.ReservationTTL
	}
	if s.Clock == nil {
		s.Clock = DefaultClock()
	}
	return l
}

// Tick executes one scheduling pass for a scope.
//
// It is intentionally best-effort: a failure to drive one run does not abort the
// pass for other runs. The returned error is non-nil only when the source,
// ledger or listing is unavailable, because then no decision can be trusted.
func (s *Scheduler) Tick(ctx Ctx, scope Scope) (TickResult, error) {
	limits := s.limits()
	now := s.Clock.Now()
	result := TickResult{Scope: scope, StartedAt: now}
	defer func() { result.FinishedAt = s.Clock.Now() }()

	if err := scope.Validate(); err != nil {
		return result, err
	}
	if err := limits.Validate(); err != nil {
		return result, err
	}
	if s.Store == nil || s.Source == nil || s.Driver == nil {
		return result, fmt.Errorf("central: scheduler requires store, source and driver")
	}
	if err := s.Store.EnsureLedger(ctx, scope, limits.Capacity); err != nil {
		return result, fmt.Errorf("central: ensure ledger: %w", err)
	}

	runs, err := s.Source.ListActive(ctx, scope)
	if err != nil {
		return result, fmt.Errorf("central: list active runs: %w", err)
	}
	result.RunsScanned = len(runs)

	// Merge persisted priorities (Redis is the only scheduler state allowed for
	// this feature; no schema changes).
	active := make([]ActiveRun, 0, len(runs))
	activeKeys := make(map[RunKey]struct{}, len(runs))
	for _, run := range runs {
		run = run.Normalize()
		run.Scope = scope.WithSpace(run.Key.SpaceID)
		if err := run.Validate(); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("run %s invalid: %v", run.Key, err))
			continue
		}
		if p, ok, perr := s.Store.GetPriority(ctx, run.Scope, run.Key); perr == nil && ok {
			run.Priority = p
		}
		if err := s.Store.RegisterRun(ctx, run, s.RunTTL); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("run %s register: %v", run.Key, err))
			continue
		}
		active = append(active, run)
		activeKeys[run.Key] = struct{}{}
		result.RunsRegistered++
	}

	SortRuns(active)

	// Resolve held reservations once per pass; budgets are run-local deficits.
	// Global capacity is still enforced atomically in Admission.TryReserve.
	reservations, rerr := s.Store.ListReservations(ctx, scope)
	if rerr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("list reservations: %v", rerr))
	}
	heldByRun := map[RunKey]int64{}
	for _, r := range reservations {
		if r.State != ReservationReleased {
			heldByRun[r.Key]++
		}
	}

	for _, run := range active {
		run.Budget = Plan(run, heldByRun[run.Key])
		if run.Budget <= 0 {
			continue
		}
		if err := s.Driver.Drive(ctx, run); err != nil {
			result.RunsFailed++
			result.Errors = append(result.Errors, fmt.Sprintf("drive run %s: %v", run.Key, err))
			continue
		}
		result.RunsDriven++
	}

	// Retire runs that the source no longer reports as active: release all of
	// their reservations and remove the registration. This covers completion,
	// cancellation, failure and retries (a retry has a new run id).
	registered, lerr := s.Store.ListRuns(ctx, scope)
	if lerr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("list registered runs: %v", lerr))
	} else {
		for _, run := range registered {
			if _, ok := activeKeys[run.Key]; ok {
				continue
			}
			if err := s.releaseRun(ctx, scope, run.Key, reservations); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("retire run %s: %v", run.Key, err))
				continue
			}
			result.RunsRetired++
		}
	}

	result.Released = s.reconcileExpired(ctx, scope, reservations, limits, now)

	// Ledger drift detection: available + reserved must equal capacity and the
	// reserved side must equal the sum of live holds.
	if s.shouldReconcile() {
		if rebuilt, err := s.reconcileLedger(ctx, scope, limits, reservations); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("reconcile ledger: %v", err))
		} else if rebuilt {
			result.LedgerRebuilt = true
		}
	}

	return result, nil
}

func (s *Scheduler) shouldReconcile() bool {
	if s.ReconcileEvery <= 0 {
		return true
	}
	s.tickCount++
	return s.tickCount%s.ReconcileEvery == 0
}

func (s *Scheduler) releaseRun(ctx Ctx, scope Scope, key RunKey, reservations []Reservation) error {
	for _, r := range reservations {
		if r.Key != key {
			continue
		}
		if err := ValidateOwnership(scope, r); err != nil {
			continue
		}
		if _, err := s.Store.Release(ctx, scope, r.ID); err != nil {
			return err
		}
	}
	return s.Store.UnregisterRun(ctx, scope, key)
}

// reconcileExpired releases reservations that are past their TTL or that have
// stayed held beyond the dispatch grace window. The latter is the recovery path
// for an uncertain dispatch: the event may or may not have reached the consumer,
// so releasing is safe because a still-queueing item will be picked up again.
func (s *Scheduler) reconcileExpired(ctx Ctx, scope Scope, reservations []Reservation, limits Limits, now time.Time) int {
	released := 0
	for _, r := range reservations {
		if r.State == ReservationReleased {
			continue
		}
		if err := ValidateOwnership(scope, r); err != nil {
			continue
		}
		expired := r.ExpiresAt.IsZero() || !r.ExpiresAt.After(now)
		stale := r.CreatedAt.IsZero() || now.Sub(r.CreatedAt) > limits.DispatchGrace+limits.ReservationTTL
		if !expired && !stale {
			continue
		}
		if _, err := s.Store.Release(ctx, scope, r.ID); err != nil {
			continue
		}
		released++
	}
	return released
}

// reconcileLedger compares the stored ledger with the authoritative reservation
// set and rebuilds it when the absolute drift exceeds Limits.MaxDrift.
func (s *Scheduler) reconcileLedger(ctx Ctx, scope Scope, limits Limits, reservations []Reservation) (bool, error) {
	ledger, err := s.Store.Ledger(ctx, scope)
	if err != nil {
		return false, err
	}
	held := SumHolds(reservations)
	drift := int64(0)
	for _, kind := range KnownResources {
		expectedAvailable := limits.capacityOf(kind) - held[kind]
		if expectedAvailable < 0 {
			expectedAvailable = 0
		}
		diff := ledger.Available[kind] - expectedAvailable
		if diff < 0 {
			diff = -diff
		}
		if diff > drift {
			drift = diff
		}
	}
	if drift <= limits.MaxDrift {
		return false, nil
	}
	if _, err := s.Store.RebuildLedger(ctx, scope, limits.Capacity); err != nil {
		return false, err
	}
	return true, nil
}
