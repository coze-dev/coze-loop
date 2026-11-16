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
//	Acquire  -> atomically reserve one multi-resource hold per item
//	publish  -> existing MQ event
//	Release  -> on item completion/failure, or on publish failure (compensation)
//
// Legacy mode is a strict no-op, which preserves the historical MQ self-loop and
// 200-running-experiment admission.
type Admission struct {
	Store  Store
	Clock  Clock
	Limits Limits

	// LimitsFn returns the current pool configuration for a space.
	LimitsFn func(ctx Ctx, spaceID int64) Limits
	// ModeFn returns the effective mode for a run. Allow-list logic belongs here.
	ModeFn func(ctx Ctx, key RunKey) Mode
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

func (a *Admission) limits(ctx Ctx, spaceID int64) Limits {
	if a.LimitsFn != nil {
		return a.LimitsFn(ctx, spaceID).withDefaults()
	}
	return a.Limits.withDefaults()
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

// Mode returns the effective mode for a run.
func (a *Admission) Mode(ctx Ctx, key RunKey) Mode {
	if a.ModeFn == nil {
		return ModeLegacy
	}
	return a.ModeFn(ctx, key)
}

// Acquire reserves resources for the supplied items and returns the subset that
// was granted. In legacy mode it returns (nil, nil) to signal "gate disabled";
// in shadow mode it returns all items unchanged and records a would-be decision.
//
// run.Budget bounds how many *new* holds may be created this pass. Holds that
// already exist (retry/idempotency) do not consume budget.
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
	limits := a.limits(ctx, run.Key.SpaceID)
	if err := limits.Validate(); err != nil {
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

	granted := make([]int64, 0, len(itemIDs))
	seen := make(map[int64]struct{}, len(itemIDs))
	var newHolds int64
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
			newHolds++
			granted = append(granted, itemID)
		case ReserveExisting:
			// Already held from a previous pass; safe to dispatch again because the
			// item event is idempotent downstream.
			granted = append(granted, itemID)
		case ReserveDenied:
			// Capacity exhausted for this priority level; leave the remaining items
			// queued. Higher-priority runs were driven first, so this is the intended
			// preemption point.
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
	limits := a.limits(ctx, run.Key.SpaceID)
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
	_ = limits
	if a.Recorder != nil {
		a.Recorder.RecordDecision(run.Scope, run.Key, ModeShadow, run.Budget, feasible)
	}
}

// ReleaseItems releases holds for completed/failed items. Idempotent.
func (a *Admission) ReleaseItems(ctx Ctx, run ActiveRun, itemIDs []int64) int {
	run = run.Normalize()
	if a.Mode(ctx, run.Key) == ModeLegacy || a.Store == nil {
		return 0
	}
	released := 0
	for _, itemID := range itemIDs {
		if itemID <= 0 {
			continue
		}
		if ok, err := a.Store.Release(ctx, run.Scope, ReservationID(run.Key, itemID)); err == nil && ok {
			released++
		}
	}
	return released
}

// ReleaseRun releases every reservation owned by the run. Used on completion,
// cancellation, terminal failure and retry supersession.
func (a *Admission) ReleaseRun(ctx Ctx, run ActiveRun) int {
	run = run.Normalize()
	if a.Store == nil {
		return 0
	}
	if a.Mode(ctx, run.Key) == ModeLegacy {
		return 0
	}
	reservations, err := a.Store.ListReservations(ctx, run.Scope)
	if err != nil {
		return 0
	}
	released := 0
	for _, r := range reservations {
		if r.Key != run.Key {
			continue
		}
		if err := ValidateOwnership(run.Scope, r); err != nil {
			continue
		}
		if ok, rerr := a.Store.Release(ctx, run.Scope, r.ID); rerr == nil && ok {
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
	limits := a.limits(ctx, run.Key.SpaceID)
	if err := a.Store.RegisterRun(ctx, run, 2*limits.ReservationTTL); err != nil {
		return err
	}
	_, err := a.Store.RenewReservations(ctx, run.Scope, run.Key, limits.ReservationTTL)
	return err
}

// ExpiresAt is a helper for tests.
func ExpiresAt(now time.Time, ttl time.Duration) time.Time {
	return now.Add(ttl)
}
