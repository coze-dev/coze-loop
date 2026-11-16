// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type pendingSource struct {
	pending []ActiveRun
	active  []ActiveRun
}

func (p *pendingSource) ListActive(_ Ctx, _ Scope) ([]ActiveRun, error)  { return p.active, nil }
func (p *pendingSource) ListPending(_ Ctx, _ Scope) ([]ActiveRun, error) { return p.pending, nil }

type observingStub struct {
	obs ItemObservation
	err error
}

func (o observingStub) ObserveItem(_ Ctx, _ Scope, _ RunKey, _ int64) (ItemObservation, error) {
	return o.obs, o.err
}

func fullCapacity(n int64) map[ResourceKind]int64 {
	return map[ResourceKind]int64{
		ResourceItemSlot: n, ResourceTargetSlot: n, ResourceSandboxSlot: n,
	}
}

func TestReservationStateMachineIsIdempotent(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := testScope().WithSpace(10)
	key := RunKey{SpaceID: 10, ExptID: 20, ExptRunID: 30}
	if err := store.EnsureLedger(nil, scope, fullCapacity(2)); err != nil {
		t.Fatal(err)
	}
	id := ReservationID(scope, key, 7)
	if res, err := store.TryReserve(nil, scope, key, 7, fullCapacity(1), time.Minute); err != nil || res != ReserveGranted {
		t.Fatalf("reserve=%v err=%v", res, err)
	}
	for _, step := range []ReservationState{ReservationDispatched, ReservationRunning, ReservationCompleted, ReservationPendingRelease} {
		ok, state, err := store.Transition(nil, scope, id, "", step, "test", time.Time{})
		if err != nil || !ok || state != step {
			t.Fatalf("transition -> %s ok=%v state=%s err=%v", step, ok, state, err)
		}
		// Repeating the same migration is an idempotent success, not a second write.
		ok, state, err = store.Transition(nil, scope, id, "", step, "test", time.Time{})
		if err != nil || !ok || state != step {
			t.Fatalf("repeat -> %s ok=%v state=%s err=%v", step, ok, state, err)
		}
	}
	if ok, _, err := store.Transition(nil, scope, id, "", ReservationRunning, "bad", time.Time{}); err == nil || ok {
		t.Fatalf("completed -> running must be rejected, ok=%v err=%v", ok, err)
	}
	if ok, err := store.Release(nil, scope, id, "test"); err != nil || !ok {
		t.Fatalf("release ok=%v err=%v", ok, err)
	}
	if ok, err := store.Release(nil, scope, id, "test"); err != nil || ok {
		t.Fatalf("duplicate release must be no-op, ok=%v err=%v", ok, err)
	}
	ledger, _ := store.Ledger(nil, scope)
	if ledger.Available[ResourceItemSlot] != 2 {
		t.Fatalf("available=%d want 2", ledger.Available[ResourceItemSlot])
	}
}

func TestScopeOwnershipHasNoWildcard(t *testing.T) {
	exact := Scope{Env: "ppe_a", Region: "cn", SpaceID: 10}
	res := Reservation{ID: "x", Scope: exact, Key: RunKey{SpaceID: 10, ExptID: 1, ExptRunID: 1}}
	if err := ValidateOwnership(exact, res); err != nil {
		t.Fatalf("exact ownership: %v", err)
	}
	if err := ValidateOwnership(Scope{Env: "ppe_a", Region: "cn", SpaceID: 0}, res); !errors.Is(err, ErrCrossEnvironment) {
		t.Fatalf("space=0 wildcard must be rejected, got %v", err)
	}
	if err := ValidateOwnership(Scope{Env: "ppe_a", Region: "cn", SpaceID: 11}, res); !errors.Is(err, ErrCrossEnvironment) {
		t.Fatalf("different space must be rejected, got %v", err)
	}
	if err := ValidateOwnership(Scope{Env: "ppe_b", Region: "cn", SpaceID: 10}, res); !errors.Is(err, ErrCrossEnvironment) {
		t.Fatalf("different env must be rejected, got %v", err)
	}
}

func TestDispatchClaimFencesTwoInstances(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := testScope().WithSpace(10)
	capacity := fullCapacity(1)
	if err := store.EnsureLedger(nil, scope, capacity); err != nil {
		t.Fatal(err)
	}
	run := testRun(10, 20, 30, 0, clock.Now())
	run.Scope = scope
	run.Budget = 1
	mk := func() *Admission {
		return &Admission{
			Store: store, Clock: clock,
			Limits:    Limits{Capacity: capacity, ReservationTTL: time.Minute},
			ModeFn:    func(Ctx, RunKey) Mode { return ModeEnforce },
			ScopeFn:   func(_ Ctx, id int64) Scope { return scope.WithSpace(id) },
			AllowedFn: func(Ctx, RunKey) bool { return true },
		}
	}
	first, err := mk().Acquire(nil, run, []int64{7})
	if err != nil || len(first) != 1 {
		t.Fatalf("first instance grant=%v err=%v", first, err)
	}
	second, err := mk().Acquire(nil, run, []int64{7})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("second instance must not publish a claimed item, got %v", second)
	}
	rec, found, _ := store.GetReservation(nil, scope, ReservationID(scope, run.Key, 7))
	if !found || rec.State != ReservationDispatched {
		t.Fatalf("fenced reservation state=%+v found=%v", rec, found)
	}
}

func TestMemoryDispatchClaimHasSingleWinner(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := testScope().WithSpace(10)
	capacity := fullCapacity(2)
	if err := store.EnsureLedger(nil, scope, capacity); err != nil {
		t.Fatal(err)
	}
	run := testRun(10, 20, 30, 0, clock.Now())
	run.Scope = scope
	if res, err := store.TryReserve(nil, scope, run.Key, 7, map[ResourceKind]int64{ResourceItemSlot: 1}, time.Minute); err != nil || res != ReserveGranted {
		t.Fatalf("reserve=%v err=%v", res, err)
	}
	id := ReservationID(scope, run.Key, 7)
	var winners int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, state, err := store.Claim(nil, scope, id, "dispatch_claim", time.Time{})
			if err == nil && ok && state == ReservationDispatched {
				atomic.AddInt32(&winners, 1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if winners != 1 {
		t.Fatalf("claim winners=%d want exactly 1", winners)
	}
}

func TestAdmissionFailsClosedWhenCapacityDisabled(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	adm := &Admission{Store: store, Clock: clock, ModeFn: func(Ctx, RunKey) Mode { return ModeEnforce }}
	run := testRun(10, 20, 30, 0, clock.Now())
	run.Scope = testScope().WithSpace(10)
	run.Budget = 1
	if _, err := adm.Acquire(nil, run, []int64{1}); !errors.Is(err, ErrQuotaDisabled) {
		t.Fatalf("disabled capacity must fail closed, got %v", err)
	}
	ledger, _ := store.Ledger(nil, run.Scope)
	if len(ledger.Capacity) != 0 {
		t.Fatalf("disabled capacity must not create a default pool: %v", ledger.Capacity)
	}
}

func TestRetryInheritsRecordedModeAndRollbackWins(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := testScope().WithSpace(10)
	key := RunKey{SpaceID: 10, ExptID: 20, ExptRunID: 30}
	if err := store.SetRunMode(nil, scope, key, ModeEnforce, time.Hour); err != nil {
		t.Fatal(err)
	}
	adm := &Admission{
		Store: store, Clock: clock,
		ModeFn:    func(Ctx, RunKey) Mode { return ModeEnforce },
		AllowedFn: func(Ctx, RunKey) bool { return false },
		ScopeFn:   func(_ Ctx, id int64) Scope { return scope.WithSpace(id) },
	}
	if got := adm.Mode(nil, key); got != ModeEnforce {
		t.Fatalf("retry must inherit enforce after allow-list removal, got=%v", got)
	}
	// A global rollback to legacy must win over the recorded mode.
	adm.ModeFn = func(Ctx, RunKey) Mode { return ModeLegacy }
	if got := adm.Mode(nil, key); got != ModeLegacy {
		t.Fatalf("global legacy rollback must win, got=%v", got)
	}
}

func TestSchedulerResolvesDispatchedItemAndLateCallback(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := testScope()
	key := RunKey{SpaceID: 10, ExptID: 20, ExptRunID: 30}
	if err := store.EnsureLedger(nil, scope, fullCapacity(1)); err != nil {
		t.Fatal(err)
	}
	id := ReservationID(scope.WithSpace(10), key, 7)
	if res, err := store.TryReserve(nil, scope.WithSpace(10), key, 7, fullCapacity(1), 10*time.Minute); err != nil || res != ReserveGranted {
		t.Fatalf("reserve=%v err=%v", res, err)
	}
	if _, _, err := store.Transition(nil, scope.WithSpace(10), id, ReservationReserved, ReservationDispatched, "dispatch", clock.Now().Add(-2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	run := testRun(10, 20, 30, 0, clock.Now())
	run.Scope = scope.WithSpace(10)
	src := &pendingSource{active: []ActiveRun{run}}
	sched := &Scheduler{
		Store: store, Source: src, Driver: &fakeDriver{},
		Observer: observingStub{obs: ItemSuccess},
		Limits:   Limits{Capacity: fullCapacity(1), ReservationTTL: 5 * time.Minute, DispatchGrace: time.Minute},
		Clock:    clock,
	}
	res, err := sched.Tick(nil, scope)
	if err != nil {
		t.Fatal(err)
	}
	if res.DispatchesResolved != 1 {
		t.Fatalf("resolved=%d result=%+v", res.DispatchesResolved, res)
	}
	if _, ok, _ := store.GetReservation(nil, run.Scope, id); !ok {
		t.Fatalf("released reservation must remain observable for grace")
	}
	ledger, _ := store.Ledger(nil, scope)
	if ledger.Available[ResourceItemSlot] != 1 {
		t.Fatalf("available=%d want 1", ledger.Available[ResourceItemSlot])
	}
	// Late/duplicate terminal callback must not refund twice.
	if adm := (&Admission{Store: store, Clock: clock}); adm.ReleaseItem(nil, run, 7, ReservationCompleted, "late") {
		t.Fatalf("late callback after released must be a no-op")
	}
}

func TestSchedulerHoldsWhenObserverFails(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := testScope()
	key := RunKey{SpaceID: 10, ExptID: 20, ExptRunID: 30}
	_ = store.EnsureLedger(nil, scope, fullCapacity(1))
	_, _ = store.TryReserve(nil, scope.WithSpace(10), key, 7, fullCapacity(1), 10*time.Minute)
	id := ReservationID(scope.WithSpace(10), key, 7)
	_, _, _ = store.Transition(nil, scope.WithSpace(10), id, ReservationReserved, ReservationDispatched, "dispatch", clock.Now().Add(-2*time.Minute))
	run := testRun(10, 20, 30, 0, clock.Now())
	run.Scope = scope.WithSpace(10)
	sched := &Scheduler{
		Store: store, Source: &pendingSource{active: []ActiveRun{run}}, Driver: &fakeDriver{},
		Observer: observingStub{err: errors.New("item store unavailable")},
		Limits:   Limits{Capacity: fullCapacity(1), ReservationTTL: 5 * time.Minute, DispatchGrace: time.Minute},
		Clock:    clock,
	}
	res, _ := sched.Tick(nil, scope)
	if res.UncertainHeld != 1 || res.Released != 0 {
		t.Fatalf("observer error must hold, result=%+v", res)
	}
	if _, ok, _ := store.GetReservation(nil, run.Scope, id); !ok {
		t.Fatalf("hold must remain on observer error")
	}
}

func TestPendingInitializationUsesSeparatePhase(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := testScope()
	pending := testRun(10, 20, 30, 0, clock.Now())
	pending.Scope = scope.WithSpace(10)
	active := testRun(11, 21, 31, 0, clock.Now())
	active.Scope = scope.WithSpace(11)
	driver := &fakeDriver{}
	sched := &Scheduler{
		Store: store, Source: &pendingSource{pending: []ActiveRun{pending}, active: []ActiveRun{active}},
		Driver: driver, Limits: Limits{Capacity: fullCapacity(1), ReservationTTL: time.Minute}, Clock: clock,
	}
	res, err := sched.Tick(nil, scope)
	if err != nil {
		t.Fatal(err)
	}
	if res.PendingScanned != 1 || res.PendingRegistered != 1 || res.RunsDriven != 1 {
		t.Fatalf("pending/active phases mixed: %+v", res)
	}
	if len(driver.calls) != 1 || driver.calls[0].key != active.Key {
		t.Fatalf("pending run must not be driven: %+v", driver.calls)
	}
}

func TestDrainReleasesAllAndUnregisters(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := testScope()
	_ = store.EnsureLedger(nil, scope, fullCapacity(2))
	run := testRun(10, 20, 30, 0, clock.Now())
	run.Scope = scope.WithSpace(10)
	_ = store.RegisterRun(nil, run, time.Minute)
	for _, item := range []int64{1, 2} {
		_, _ = store.TryReserve(nil, run.Scope, run.Key, item, fullCapacity(1), time.Minute)
	}
	sched := &Scheduler{Store: store, Clock: clock}
	n, err := sched.Drain(nil, scope)
	if err != nil || n != 2 {
		t.Fatalf("drain=%d err=%v", n, err)
	}
	ledger, _ := store.Ledger(nil, scope)
	if ledger.Available[ResourceItemSlot] != 2 {
		t.Fatalf("drain left capacity unavailable: %+v", ledger)
	}
	runs, _ := store.ListRuns(nil, scope)
	if len(runs) != 0 {
		t.Fatalf("drain left registrations: %+v", runs)
	}
}

func TestIncrementalAndFullReconcileAreDistinguishable(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := testScope()
	_ = store.EnsureLedger(nil, scope, fullCapacity(2))
	_, _ = store.TryReserve(nil, scope.WithSpace(10), RunKey{SpaceID: 10, ExptID: 20, ExptRunID: 30}, 1, fullCapacity(1), time.Minute)
	l1, needFull, err := store.IncrementalReconcile(nil, scope, fullCapacity(2), 0)
	if err != nil || needFull || l1.ReconcileMode != "incremental" || l1.Generation != 0 {
		t.Fatalf("incremental ledger=%+v needFull=%v err=%v", l1, needFull, err)
	}
	l2, err := store.RebuildLedger(nil, scope, fullCapacity(2))
	if err != nil || l2.ReconcileMode != "full" || l2.Generation != 1 {
		t.Fatalf("full ledger=%+v err=%v", l2, err)
	}
}

func TestLeaderElectionAllowsSingleDispatcher(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := testScope()
	if ok, err := store.AcquireLeadership(nil, scope, "pod-a", time.Minute); err != nil || !ok {
		t.Fatalf("pod-a acquire=%v err=%v", ok, err)
	}
	if ok, err := store.AcquireLeadership(nil, scope, "pod-b", time.Minute); err != nil || ok {
		t.Fatalf("pod-b must be follower, ok=%v err=%v", ok, err)
	}
	if ok, err := store.RenewLeadership(nil, scope, "pod-a", time.Minute); err != nil || !ok {
		t.Fatalf("pod-a renew=%v err=%v", ok, err)
	}
	_ = store.ReleaseLeadership(nil, scope, "pod-a")
	if ok, err := store.AcquireLeadership(nil, scope, "pod-b", time.Minute); err != nil || !ok {
		t.Fatalf("pod-b should take over after release, ok=%v err=%v", ok, err)
	}
}
