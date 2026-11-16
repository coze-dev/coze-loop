// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"fmt"
	"time"
)

// DecisionRecorder receives central decisions for metrics/observability. It is
// optional; a nil recorder is a no-op.
type DecisionRecorder interface {
	RecordDecision(scope Scope, key RunKey, mode Mode, budget, granted int64)
}

// Admission is the pre-dispatch gate. It is called by the existing experiment
// scheduler immediately before item events are published:
//
//	Acquire          -> reserve one multi-resource hold per item
//	ConfirmDispatch  -> mark reserved -> dispatched after publish succeeds
//	ConfirmRunning   -> mark dispatched -> running after execution is observed
//	ReleaseItem      -> completed/failed/canceled -> pending_release -> released
//
// Legacy mode is a strict no-op, which preserves the historical MQ self-loop and
// 200-running-experiment admission.
type Admission struct {
	Store  Store
	Clock  Clock
	Limits Limits

	// LimitsFn returns the current pool configuration for a space. Returning an
	// error means the config is missing/degraded and the gate fails closed.
	LimitsFn func(ctx Ctx, spaceID int64) (Limits, error)
	// ModeFn returns the raw configured mode (legacy/shadow/enforce), without
	// allow-list filtering, so a recorded enforce mode can survive allow-list changes.
	ModeFn func(ctx Ctx, key RunKey) Mode
	// AllowedFn decides first adoption when the configured mode is enforce.
	AllowedFn func(ctx Ctx, key RunKey) bool
	// ScopeFn builds the ownership scope for a space (env + region + space).
	ScopeFn func(ctx Ctx, spaceID int64) Scope
	// Recorder is optional.
	Recorder DecisionRecorder
}

func (a *Admission) clock() Clock {
	if a.Clock == nil {
		return DefaultClock()
	}
	return a.Clock
}

func (a *Admission) limits(ctx Ctx, spaceID int64) (Limits, error) {
	var limits Limits
	if a.LimitsFn != nil {
		var err error
		limits, err = a.LimitsFn(ctx, spaceID)
		if err != nil {
			return Limits{}, err
		}
	} else {
		limits = a.Limits
	}
	limits = limits.withDefaults()
	if err := limits.Validate(); err != nil {
		return Limits{}, err
	}
	return limits, nil
}

func (a *Admission) scope(ctx Ctx, spaceID int64) Scope {
	if a.ScopeFn != nil {
		return a.ScopeFn(ctx, spaceID)
	}
	return Scope{SpaceID: spaceID}
}

// ScopeOf exposes the configured ownership scope for a space.
func (a *Admission) ScopeOf(ctx Ctx, spaceID int64) Scope {
	return a.scope(ctx, spaceID)
}

// HeldItems returns how many item reservations are currently held for a run.
func (a *Admission) HeldItems(ctx Ctx, run ActiveRun) int64 {
	if a.Store == nil {
		return 0
	}
	reservations, err := a.Store.ListReservations(ctx, run.Scope)
	if err != nil {
		return 0
	}
	return CountHeldByRun(reservations, run.Key)
}

// Mode returns the effective mode for a run. A recorded run mode wins over the
// current allow-list so a retry cannot flip from enforce back to legacy. A
// global legacy config still wins, which is the rollback path.
func (a *Admission) Mode(ctx Ctx, key RunKey) Mode {
	configured := ModeLegacy
	if a.ModeFn != nil {
		configured = a.ModeFn(ctx, key)
	}
	if configured == ModeLegacy {
		return ModeLegacy
	}
	if a.Store == nil {
		return configured
	}
	scope := a.scope(ctx, key.SpaceID)
	if mode, ok, err := a.Store.GetRunMode(ctx, scope, key); err == nil && ok {
		return mode
	}
	if configured == ModeEnforce && a.AllowedFn != nil && !a.AllowedFn(ctx, key) {
		return ModeLegacy
	}
	if configured == ModeEnforce {
		_ = a.Store.SetRunMode(ctx, scope, key, ModeEnforce, 24*time.Hour)
	}
	return configured
}

// Acquire reserves resources for the supplied items and returns the subset that
// should be published this pass. In legacy mode it returns (nil, nil) to signal
// "gate disabled"; in shadow mode it returns all items unchanged.
//
// run.Budget bounds how many *new* holds may be created this pass. Existing
// reservations in dispatched/running state are not re-published; only a
// reserved hold (a previous publish that never confirmed) may be retried.
func (a *Admission) Acquire(ctx Ctx, run ActiveRun, itemIDs []int64) ([]int64, error) {
	run = run.Normalize()
	if err := run.Validate(); err != nil {
		return nil, err
	}
	mode := a.Mode(ctx, run.Key)
	switch mode {
	case ModeLegacy:
		return nil, nil
	case ModeShadow:
		a.observeShadow(ctx, run, itemIDs)
		return itemIDs, nil
	}

	if run.Budget <= 0 || len(itemIDs) == 0 {
		return nil, nil
	}
	limits, err := a.limits(ctx, run.Key.SpaceID)
	if err != nil {
		return nil, err
	}
	scope := run.Scope
	if err := a.Store.EnsureLedger(ctx, scope, limits.Capacity); err != nil {
		return nil, fmt.Errorf("central: ensure ledger: %w", err)
	}

	holds, err := run.HoldsFor(1)
	if err != nil {
		return nil, err
	}

	now := a.clock().Now()
	granted := make([]int64, 0, len(itemIDs))
	seen := make(map[int64]struct{}, len(itemIDs))
	var newHolds int64
	claim := func(itemID int64) (bool, error) {
		id := ReservationID(scope, run.Key, itemID)
		claimed, state, err := a.Store.Claim(ctx, scope, id, "dispatch_claim", now)
		if err != nil {
			return false, err
		}
		return claimed && state == ReservationDispatched, nil
	}
	for _, itemID := range itemIDs {
		if itemID <= 0 {
			continue
		}
		if _, ok := seen[itemID]; ok {
			continue
		}
		seen[itemID] = struct{}{}
		if newHolds >= run.Budget {
			break
		}
		result, rerr := a.Store.TryReserve(ctx, scope, run.Key, itemID, holds, limits.ReservationTTL)
		if rerr != nil {
			return granted, rerr
		}
		switch result {
		case ReserveGranted:
			ok, cerr := claim(itemID)
			if cerr != nil {
				_, _, _ = a.Store.Transition(ctx, scope, ReservationID(scope, run.Key, itemID), "", ReservationPendingRelease, "claim_error", time.Time{})
				_, _ = a.Store.Release(ctx, scope, ReservationID(scope, run.Key, itemID), "claim_error")
				return granted, cerr
			}
			if !ok {
				// Another dispatcher claimed this item between reserve and claim.
				_, _, _ = a.Store.Transition(ctx, scope, ReservationID(scope, run.Key, itemID), "", ReservationPendingRelease, "claim_lost", time.Time{})
				_, _ = a.Store.Release(ctx, scope, ReservationID(scope, run.Key, itemID), "claim_lost")
				continue
			}
			newHolds++
			granted = append(granted, itemID)
		case ReserveExisting:
			// The reservation state is the dispatch fence: only the instance that
			// atomically moves reserved -> dispatched may publish. A second instance
			// sees dispatched/running and must not publish again.
			id := ReservationID(scope, run.Key, itemID)
			res, ok, gerr := a.Store.GetReservation(ctx, scope, id)
			if gerr != nil {
				return granted, gerr
			}
			if !ok || res.State != ReservationReserved {
				continue
			}
			claimed, cerr := claim(itemID)
			if cerr != nil {
				return granted, cerr
			}
			if claimed {
				granted = append(granted, itemID)
			}
		case ReserveDenied:
			if a.Recorder != nil {
				a.Recorder.RecordDecision(scope, run.Key, mode, run.Budget, int64(len(granted)))
			}
			return granted, nil
		}
	}
	if a.Recorder != nil {
		a.Recorder.RecordDecision(scope, run.Key, mode, run.Budget, int64(len(granted)))
	}
	return granted, nil
}

// observeShadow evaluates feasibility without mutating the ledger.
func (a *Admission) observeShadow(ctx Ctx, run ActiveRun, itemIDs []int64) {
	limits, lerr := a.limits(ctx, run.Key.SpaceID)
	if lerr != nil || a.Store == nil {
		return
	}
	_ = limits
	ledger, err := a.Store.Ledger(ctx, run.Scope)
	if err != nil {
		return
	}
	holds, err := run.HoldsFor(1)
	if err != nil {
		return
	}
	var feasible int64
	for range itemIDs {
		if feasible >= run.Budget {
			break
		}
		ok := true
		for kind, amount := range holds {
			if ledger.Available[kind] < amount {
				ok = false
				break
			}
		}
		if !ok {
			break
		}
		for kind, amount := range holds {
			ledger.Available[kind] -= amount
		}
		feasible++
	}
	if a.Recorder != nil {
		a.Recorder.RecordDecision(run.Scope, run.Key, ModeShadow, run.Budget, feasible)
	}
}

// ConfirmDispatch marks the holds as dispatched after a successful publish.
// Repeating it is idempotent; already running/completed holds are left alone.
func (a *Admission) ConfirmDispatch(ctx Ctx, run ActiveRun, itemIDs []int64) int {
	if a.Store == nil {
		return 0
	}
	run = run.Normalize()
	now := a.clock().Now()
	n := 0
	for _, itemID := range itemIDs {
		if itemID <= 0 {
			continue
		}
		id := ReservationID(run.Scope, run.Key, itemID)
		ok, state, err := a.Store.Transition(ctx, run.Scope, id, ReservationReserved, ReservationDispatched, "dispatch_confirmed", now)
		if err == nil && (ok || state == ReservationDispatched) {
			n++
		}
	}
	return n
}

// ConfirmRunning marks a dispatched hold as running after execution is observed.
func (a *Admission) ConfirmRunning(ctx Ctx, run ActiveRun, itemIDs []int64) int {
	if a.Store == nil {
		return 0
	}
	run = run.Normalize()
	n := 0
	for _, itemID := range itemIDs {
		if itemID <= 0 {
			continue
		}
		id := ReservationID(run.Scope, run.Key, itemID)
		if ok, _, err := a.Store.Transition(ctx, run.Scope, id, ReservationDispatched, ReservationRunning, "execution_observed", time.Time{}); err == nil && ok {
			n++
		}
	}
	return n
}

// ReleaseItem moves a terminal item through pending_release to released.
// Repeated/late callbacks are no-ops once released.
func (a *Admission) ReleaseItem(ctx Ctx, run ActiveRun, itemID int64, terminal ReservationState, reason string) bool {
	if a.Store == nil || itemID <= 0 {
		return false
	}
	run = run.Normalize()
	scope := run.Scope
	id := ReservationID(scope, run.Key, itemID)
	res, ok, err := a.Store.GetReservation(ctx, scope, id)
	if err != nil || !ok {
		return false
	}
	if res.State == ReservationReleased {
		return false
	}
	if terminal.Valid() && terminal != res.State {
		_, _, _ = a.Store.Transition(ctx, scope, id, "", terminal, reason, time.Time{})
	}
	_, _, _ = a.Store.Transition(ctx, scope, id, "", ReservationPendingRelease, reason, time.Time{})
	released, rerr := a.Store.Release(ctx, scope, id, reason)
	return rerr == nil && released
}

// ReleaseItems releases holds for items whose precise terminal state is unknown.
func (a *Admission) ReleaseItems(ctx Ctx, run ActiveRun, itemIDs []int64) int {
	released := 0
	for _, itemID := range itemIDs {
		if a.ReleaseItem(ctx, run, itemID, "", "item_terminal") {
			released++
		}
	}
	return released
}

// ReleaseRun releases every reservation owned by the run. Used on completion,
// cancellation, terminal failure and retry supersession.
func (a *Admission) ReleaseRun(ctx Ctx, run ActiveRun) int {
	if a.Store == nil {
		return 0
	}
	run = run.Normalize()
	reservations, err := a.Store.ListReservations(ctx, run.Scope)
	if err != nil {
		return 0
	}
	released := 0
	for _, r := range reservations {
		if r.Key != run.Key {
			continue
		}
		if err := ValidateOwnership(r.Scope, r); err != nil {
			continue
		}
		if a.ReleaseItem(ctx, run, r.ItemID, ReservationCanceled, "run_terminal") {
			released++
		}
	}
	_ = a.Store.UnregisterRun(ctx, run.Scope, run.Key)
	return released
}

// Heartbeat renews run registration and in-flight reservation leases. In
// enforce mode it replaces the old per-experiment MQ self-loop.
func (a *Admission) Heartbeat(ctx Ctx, run ActiveRun) error {
	run = run.Normalize()
	if a.Mode(ctx, run.Key) == ModeLegacy {
		return nil
	}
	limits, err := a.limits(ctx, run.Key.SpaceID)
	if err != nil {
		return err
	}
	if err := a.Store.RegisterRun(ctx, run, 2*limits.ReservationTTL); err != nil {
		return err
	}
	_, err = a.Store.RenewReservations(ctx, run.Scope, run.Key, limits.ReservationTTL)
	return err
}

// ExpiresAt is a helper for tests.
func ExpiresAt(now time.Time, ttl time.Duration) time.Time {
	return now.Add(ttl)
}
