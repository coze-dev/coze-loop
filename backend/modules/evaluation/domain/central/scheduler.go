// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"fmt"
	"time"
)

// TickResult is the observable outcome of one central scheduling pass.
type TickResult struct {
	Scope              Scope
	PendingScanned     int
	PendingRegistered  int
	RunsScanned        int
	RunsRegistered     int
	RunsDriven         int
	RunsFailed         int
	RunsRetired        int
	Released           int
	DispatchesResolved int
	UncertainHeld      int
	LedgerRebuilt      bool
	ReconcileMode      string
	LedgerGeneration   int64
	Errors             []string
	StartedAt          time.Time
	FinishedAt         time.Time
}

// Scheduler periodically scans active runs, orders them by priority, and drives
// the existing per-experiment scheduling chain. The actual item-level resource
// admission happens in Admission, immediately before item events are published.
type Scheduler struct {
	Store    Store
	Source   RunSource
	Driver   Driver
	Observer ItemObserver
	Limits   Limits
	Clock    Clock
	// ModeFn marks a central-owned run as enforce in the run-mode lineage. A
	// retry then inherits enforce even if the allow-list later changes.
	ModeFn func(ctx Ctx, scope Scope, key RunKey) Mode

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
// It is intentionally best-effort per run, but fail-closed on scope/limits/
// ledger/source errors: if no decision can be trusted, the pass returns an error
// and drives nothing.
func (s *Scheduler) Tick(ctx Ctx, scope Scope) (result TickResult, err error) {
	limits := s.limits()
	now := s.Clock.Now()
	result = TickResult{Scope: scope, StartedAt: now}
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

	// Pending initialization is a separate phase and never consumes quota or
	// shares a cursor with active maintenance.
	if pendingSource, ok := s.Source.(PendingRunSource); ok {
		pending, perr := pendingSource.ListPending(ctx, scope)
		if perr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("list pending runs: %v", perr))
		} else {
			result.PendingScanned = len(pending)
			for _, run := range pending {
				run = run.Normalize()
				run.Scope = scope.WithSpace(run.Key.SpaceID)
				if err := run.Validate(); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("pending run %s invalid: %v", run.Key, err))
					continue
				}
				if err := s.Store.RegisterRun(ctx, run, s.RunTTL); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("pending run %s register: %v", run.Key, err))
					continue
				}
				result.PendingRegistered++
			}
		}
	}

	runs, err := s.Source.ListActive(ctx, scope)
	if err != nil {
		return result, fmt.Errorf("central: list active runs: %w", err)
	}
	result.RunsScanned = len(runs)

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
		if s.ModeFn != nil {
			if mode := s.ModeFn(ctx, run.Scope, run.Key); mode != ModeLegacy {
				_ = s.Store.SetRunMode(ctx, run.Scope, run.Key, mode, 24*time.Hour)
			}
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

	reservations, rerr := s.Store.ListReservations(ctx, scope)
	if rerr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("list reservations: %v", rerr))
	}
	heldByRun := map[RunKey]int64{}
	for _, r := range reservations {
		if r.State.HoldsQuota() && r.State != ReservationPendingRelease {
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

	// Resolve uncertain dispatches and late/duplicate callbacks using the
	// authoritative item state, not just TTL expiry.
	reservations, _ = s.Store.ListReservations(ctx, scope)
	resolved, held, derrs := s.reconcileDispatch(ctx, scope, reservations, limits, now)
	result.DispatchesResolved = resolved
	result.UncertainHeld = held
	result.Errors = append(result.Errors, derrs...)

	registered, lerr := s.Store.ListRuns(ctx, scope)
	if lerr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("list registered runs: %v", lerr))
	} else {
		if fresh, ferr := s.Store.ListReservations(ctx, scope); ferr == nil {
			reservations = fresh
		}
		for _, run := range registered {
			if _, ok := activeKeys[run.Key]; ok {
				continue
			}
			if err := s.releaseRun(ctx, run.Scope, run.Key, reservations); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("retire run %s: %v", run.Key, err))
				continue
			}
			result.RunsRetired++
		}
	}

	// Expired/pending releases are drained deterministically.
	released, rerrs := s.reconcileExpired(ctx, scope, reservations, now)
	result.Released += released
	result.Errors = append(result.Errors, rerrs...)

	// Daily path is incremental; a drift above MaxDrift upgrades to full rebuild.
	if s.shouldReconcile() {
		ledger, needFull, ierr := s.Store.IncrementalReconcile(ctx, scope, limits.Capacity, limits.MaxDrift)
		if ierr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("incremental reconcile: %v", ierr))
		} else if needFull {
			ledger, err = s.Store.RebuildLedger(ctx, scope, limits.Capacity)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("full rebuild: %v", err))
			} else {
				result.LedgerRebuilt = true
			}
		}
		result.ReconcileMode = ledger.ReconcileMode
		result.LedgerGeneration = ledger.Generation
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
		if err := ValidateOwnership(r.Scope, r); err != nil {
			continue
		}
		if _, _, terr := s.Store.Transition(ctx, r.Scope, r.ID, "", ReservationPendingRelease, "run_retired", time.Time{}); terr != nil {
			continue
		}
		if _, err := s.Store.Release(ctx, r.Scope, r.ID, "run_retired"); err != nil {
			return err
		}
	}
	return s.Store.UnregisterRun(ctx, scope, key)
}

// reconcileExpired releases reservations whose lease is no longer renewed and
// any hold explicitly moved to pending_release.
func (s *Scheduler) reconcileExpired(ctx Ctx, scope Scope, reservations []Reservation, now time.Time) (int, []string) {
	released := 0
	var errs []string
	for _, r := range reservations {
		// dispatched/running holds are resolved by reconcileDispatch using the
		// authoritative item state; releasing them here would be expiry-only.
		if r.State == ReservationDispatched || r.State == ReservationRunning {
			continue
		}
		if !r.State.HoldsQuota() {
			continue
		}
		if err := ValidateOwnership(r.Scope, r); err != nil {
			continue
		}
		if r.State != ReservationPendingRelease && !r.ExpiresAt.IsZero() && r.ExpiresAt.After(now) {
			continue
		}
		if _, _, terr := s.Store.Transition(ctx, r.Scope, r.ID, "", ReservationPendingRelease, "expired_or_pending", time.Time{}); terr != nil {
			errs = append(errs, fmt.Sprintf("pending release %s: %v", r.ID, terr))
			continue
		}
		if _, err := s.Store.Release(ctx, r.Scope, r.ID, "expired_or_pending"); err != nil {
			errs = append(errs, fmt.Sprintf("release %s: %v", r.ID, err))
			continue
		}
		released++
	}
	return released, errs
}

// reconcileDispatch resolves dispatched holds after DispatchGrace using the
// item's authoritative state. Missing/queueing -> release; processing -> running
// and renew; terminal -> terminal state + release. Observer errors hold the
// reservation (fail-closed) instead of releasing blindly.
func (s *Scheduler) reconcileDispatch(ctx Ctx, scope Scope, reservations []Reservation, limits Limits, now time.Time) (int, int, []string) {
	resolved, held := 0, 0
	var errs []string
	for _, r := range reservations {
		if s.Observer == nil {
			continue
		}
		if r.State == ReservationDispatched {
			base := r.DispatchedAt
			if base.IsZero() {
				base = r.CreatedAt
			}
			if !base.IsZero() && base.Add(limits.DispatchGrace).After(now) {
				continue
			}
		} else if r.State == ReservationRunning {
			if r.ExpiresAt.IsZero() || r.ExpiresAt.After(now) {
				continue
			}
		} else {
			continue
		}
		if err := ValidateOwnership(r.Scope, r); err != nil {
			continue
		}
		obs, err := s.Observer.ObserveItem(ctx, r.Scope, r.Key, r.ItemID)
		if err != nil {
			held++
			errs = append(errs, fmt.Sprintf("observe item %s: %v", r.ID, err))
			continue
		}
		switch obs {
		case ItemProcessing:
			if r.State == ReservationDispatched {
				_, _, _ = s.Store.Transition(ctx, r.Scope, r.ID, ReservationDispatched, ReservationRunning, "observe_processing", time.Time{})
			}
			_, _ = s.Store.RenewReservations(ctx, r.Scope, r.Key, limits.ReservationTTL)
			resolved++
		case ItemSuccess, ItemFail, ItemTerminal:
			terminal := ReservationCompleted
			if obs == ItemFail {
				terminal = ReservationFailed
			} else if obs == ItemTerminal {
				terminal = ReservationCanceled
			}
			_, _, _ = s.Store.Transition(ctx, r.Scope, r.ID, "", terminal, "observe_terminal", time.Time{})
			_, _, _ = s.Store.Transition(ctx, r.Scope, r.ID, "", ReservationPendingRelease, "observe_terminal", time.Time{})
			if _, err := s.Store.Release(ctx, r.Scope, r.ID, "observe_terminal"); err != nil {
				errs = append(errs, fmt.Sprintf("release observed %s: %v", r.ID, err))
			} else {
				resolved++
			}
		case ItemMissing, ItemQueueing:
			_, _, _ = s.Store.Transition(ctx, r.Scope, r.ID, "", ReservationPendingRelease, "dispatch_not_observed", time.Time{})
			if _, err := s.Store.Release(ctx, r.Scope, r.ID, "dispatch_not_observed"); err != nil {
				errs = append(errs, fmt.Sprintf("release unobserved %s: %v", r.ID, err))
			} else {
				resolved++
			}
		}
	}
	return resolved, held, errs
}

// Drain actively releases every central reservation in the scope. It is used
// when the deployment rolls back to legacy so quota is not left to TTL expiry.
func (s *Scheduler) Drain(ctx Ctx, scope Scope) (int, error) {
	if s.Store == nil {
		return 0, fmt.Errorf("central: drain requires store")
	}
	released := 0
	for {
		reservations, err := s.Store.ListReservations(ctx, scope)
		if err != nil {
			return released, err
		}
		if len(reservations) == 0 {
			break
		}
		progress := 0
		for _, r := range reservations {
			if err := ValidateOwnership(r.Scope, r); err != nil {
				continue
			}
			_, _, _ = s.Store.Transition(ctx, r.Scope, r.ID, "", ReservationPendingRelease, "rollback_drain", time.Time{})
			ok, rerr := s.Store.Release(ctx, r.Scope, r.ID, "rollback_drain")
			if rerr != nil {
				return released, rerr
			}
			if ok {
				released++
				progress++
			}
		}
		if progress == 0 {
			break
		}
	}
	runs, err := s.Store.ListRuns(ctx, scope)
	if err != nil {
		return released, err
	}
	for _, run := range runs {
		_ = s.Store.UnregisterRun(ctx, run.Scope, run.Key)
	}
	return released, nil
}
