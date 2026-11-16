// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"errors"
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time          { return f.now }
func (f *fakeClock) Advance(d time.Duration) { f.now = f.now.Add(d) }
func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)}
}

type fakeSource struct {
	runs []ActiveRun
	err  error
}

func (f *fakeSource) ListActive(_ Ctx, _ Scope) ([]ActiveRun, error) {
	return f.runs, f.err
}

type driveCall struct {
	key    RunKey
	budget int64
}

type fakeDriver struct {
	calls []driveCall
	err   error
}

func (f *fakeDriver) Drive(_ Ctx, run ActiveRun) error {
	f.calls = append(f.calls, driveCall{key: run.Key, budget: run.Budget})
	return f.err
}

func testScope() Scope { return Scope{Env: "ppe_central_test", Region: "cn", SpaceID: 0} }

func testRun(space, expt, runID int64, priority int, enqueued time.Time) ActiveRun {
	return ActiveRun{
		Key:            RunKey{SpaceID: space, ExptID: expt, ExptRunID: runID},
		Priority:       priority,
		EnqueuedAt:     enqueued,
		MaxConcurrency: 3,
		RemainingItems: 10,
		PerItem: map[ResourceKind]int64{
			ResourceItemSlot:    1,
			ResourceTargetSlot:  1,
			ResourceSandboxSlot: 1,
		},
	}
}

func TestSortRunsPriorityThenEnqueueThenStableKey(t *testing.T) {
	t1 := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Minute)
	runs := []ActiveRun{
		testRun(1, 3, 30, 5, t1),
		testRun(1, 1, 10, 5, t1),
		testRun(1, 2, 20, 1, t2),
		testRun(1, 4, 40, 5, t2),
	}
	SortRuns(runs)
	got := []string{
		runs[0].Key.String(), runs[1].Key.String(), runs[2].Key.String(), runs[3].Key.String(),
	}
	want := []string{"1/2/20", "1/1/10", "1/3/30", "1/4/40"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sort[%d]=%s want %s (all=%v)", i, got[i], want[i], got)
		}
	}
}

func TestTryReserveIsAllOrNothingAcrossResources(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := Scope{Env: "ppe_a", Region: "cn"}
	capacity := map[ResourceKind]int64{
		ResourceItemSlot:    5,
		ResourceTargetSlot:  5,
		ResourceSandboxSlot: 1,
	}
	if err := store.EnsureLedger(nil, scope, capacity); err != nil {
		t.Fatal(err)
	}
	key := RunKey{SpaceID: 1, ExptID: 10, ExptRunID: 100}
	holds := map[ResourceKind]int64{
		ResourceItemSlot:    1,
		ResourceTargetSlot:  1,
		ResourceSandboxSlot: 1,
	}

	res, err := store.TryReserve(nil, scope, key, 1, holds, time.Minute)
	if err != nil || res != ReserveGranted {
		t.Fatalf("first reserve: result=%v err=%v", res, err)
	}
	// Only the sandbox slot is exhausted; item/target still have capacity.
	// A partial reservation would succeed, the all-or-nothing contract must deny.
	res, err = store.TryReserve(nil, scope, key, 2, holds, time.Minute)
	if err != nil || res != ReserveDenied {
		t.Fatalf("second reserve must be denied atomically: result=%v err=%v", res, err)
	}
	ledger, err := store.Ledger(nil, scope)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Available[ResourceItemSlot] != 4 {
		t.Fatalf("item slot must be untouched after denied reservation, got %d", ledger.Available[ResourceItemSlot])
	}
	if ledger.Available[ResourceSandboxSlot] != 0 {
		t.Fatalf("sandbox slot available=%d want 0", ledger.Available[ResourceSandboxSlot])
	}
}

func TestReleaseIsIdempotentAndRefundsAllResources(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := Scope{Env: "ppe_a", Region: "cn"}
	capacity := map[ResourceKind]int64{
		ResourceItemSlot: 2, ResourceTargetSlot: 2, ResourceSandboxSlot: 2,
	}
	_ = store.EnsureLedger(nil, scope, capacity)
	key := RunKey{SpaceID: 1, ExptID: 10, ExptRunID: 100}
	holds := map[ResourceKind]int64{
		ResourceItemSlot: 1, ResourceTargetSlot: 1, ResourceSandboxSlot: 1,
	}
	if res, err := store.TryReserve(nil, scope, key, 7, holds, time.Minute); err != nil || res != ReserveGranted {
		t.Fatalf("reserve: %v %v", res, err)
	}
	id := ReservationID(scope, key, 7)
	if ok, err := store.Release(nil, scope, id, "test"); err != nil || !ok {
		t.Fatalf("first release: ok=%v err=%v", ok, err)
	}
	if ok, err := store.Release(nil, scope, id, "test"); err != nil || ok {
		t.Fatalf("second release must be a no-op: ok=%v err=%v", ok, err)
	}
	ledger, _ := store.Ledger(nil, scope)
	for _, kind := range KnownResources {
		if ledger.Available[kind] != capacity[kind] {
			t.Fatalf("resource %s available=%d want %d", kind, ledger.Available[kind], capacity[kind])
		}
	}
}

func TestCrossEnvironmentReleaseIsRejected(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	owner := Scope{Env: "online_cn", Region: "cn"}
	other := Scope{Env: "ppe_lane_a", Region: "cn"}
	_ = store.EnsureLedger(nil, owner, map[ResourceKind]int64{ResourceItemSlot: 1})
	key := RunKey{SpaceID: 1, ExptID: 10, ExptRunID: 100}
	if res, err := store.TryReserve(nil, owner, key, 1, map[ResourceKind]int64{ResourceItemSlot: 1}, time.Minute); err != nil || res != ReserveGranted {
		t.Fatalf("reserve owner: %v %v", res, err)
	}
	id := ReservationID(owner, key, 1)
	if _, err := store.Release(nil, other, id, "test"); !errors.Is(err, ErrCrossEnvironment) {
		t.Fatalf("cross-env release must be rejected, got %v", err)
	}
	if _, _, err := store.GetReservation(nil, other, id); !errors.Is(err, ErrCrossEnvironment) {
		t.Fatalf("cross-env get must be rejected, got %v", err)
	}
	// The original scope can still release.
	if ok, err := store.Release(nil, owner, id, "test"); err != nil || !ok {
		t.Fatalf("owner release: ok=%v err=%v", ok, err)
	}
}

func TestAdmissionLegacyIsNoop(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	adm := &Admission{
		Store:  store,
		Clock:  clock,
		Limits: Limits{Capacity: map[ResourceKind]int64{ResourceItemSlot: 2}},
		ModeFn: func(_ Ctx, _ RunKey) Mode { return ModeLegacy },
	}
	run := testRun(1, 10, 100, 0, clock.Now())
	run.Scope = testScope().WithSpace(1)
	run.Budget = 5
	_ = store.EnsureLedger(nil, run.Scope, map[ResourceKind]int64{
		ResourceItemSlot: 2, ResourceTargetSlot: 2, ResourceSandboxSlot: 2,
	})
	granted, err := adm.Acquire(nil, run, []int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(granted) != 0 {
		t.Fatalf("legacy must not grant through the gate, got %v", granted)
	}
	ledger, _ := store.Ledger(nil, run.Scope)
	if ledger.Available[ResourceItemSlot] != 2 {
		t.Fatalf("legacy must not mutate the ledger, available=%d", ledger.Available[ResourceItemSlot])
	}
}

func TestAdmissionShadowDoesNotMutate(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := Scope{Env: "ppe_a", Region: "cn", SpaceID: 1}
	capacity := map[ResourceKind]int64{ResourceItemSlot: 3, ResourceTargetSlot: 3, ResourceSandboxSlot: 3}
	_ = store.EnsureLedger(nil, scope, capacity)
	rec := &recordingRecorder{}
	adm := &Admission{
		Store:    store,
		Clock:    clock,
		Limits:   Limits{Capacity: capacity},
		ModeFn:   func(_ Ctx, _ RunKey) Mode { return ModeShadow },
		Recorder: rec,
	}
	run := testRun(1, 10, 100, 0, clock.Now())
	run.Scope = scope
	run.Budget = 2
	granted, err := adm.Acquire(nil, run, []int64{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(granted) != 3 {
		t.Fatalf("shadow must not change dispatch, got %v", granted)
	}
	reservations, _ := store.ListReservations(nil, scope)
	if len(reservations) != 0 {
		t.Fatalf("shadow must not create reservations, got %d", len(reservations))
	}
	if rec.lastMode != ModeShadow || rec.lastGranted != 2 {
		t.Fatalf("shadow decision not recorded: %+v", rec)
	}
}

type recordingRecorder struct {
	lastMode    Mode
	lastBudget  int64
	lastGranted int64
}

func (r *recordingRecorder) RecordDecision(_ Scope, _ RunKey, mode Mode, budget, granted int64) {
	r.lastMode = mode
	r.lastBudget = budget
	r.lastGranted = granted
}

func TestAdmissionEnforceRespectsBudgetAndCapacity(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := Scope{Env: "ppe_a", Region: "cn", SpaceID: 1}
	capacity := map[ResourceKind]int64{
		ResourceItemSlot: 3, ResourceTargetSlot: 3, ResourceSandboxSlot: 3,
	}
	_ = store.EnsureLedger(nil, scope, capacity)
	adm := &Admission{
		Store:  store,
		Clock:  clock,
		Limits: Limits{Capacity: capacity, ReservationTTL: time.Minute},
		ModeFn: func(_ Ctx, _ RunKey) Mode { return ModeEnforce },
	}
	run := testRun(1, 10, 100, 0, clock.Now())
	run.Scope = scope
	run.Budget = 2
	granted, err := adm.Acquire(nil, run, []int64{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(granted) != 2 {
		t.Fatalf("budget=2 should grant 2 items, got %v", granted)
	}
	// A second acquire must not re-publish already claimed items: the reservation
	// state is the dispatch fence, so only reserved items can be claimed again.
	again, err := adm.Acquire(nil, run, []int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("claimed items must not be re-granted, got %v", again)
	}
	reservations, _ := store.ListReservations(nil, scope)
	if len(reservations) != 2 {
		t.Fatalf("duplicate reservations created: %d", len(reservations))
	}
}

func TestAcquireCompensationReleasesOnPublishFailure(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := Scope{Env: "ppe_a", Region: "cn", SpaceID: 1}
	capacity := map[ResourceKind]int64{
		ResourceItemSlot: 2, ResourceTargetSlot: 2, ResourceSandboxSlot: 2,
	}
	_ = store.EnsureLedger(nil, scope, capacity)
	adm := &Admission{
		Store: store, Clock: clock,
		Limits: Limits{Capacity: capacity, ReservationTTL: time.Minute},
		ModeFn: func(_ Ctx, _ RunKey) Mode { return ModeEnforce },
	}
	run := testRun(1, 10, 100, 0, clock.Now())
	run.Scope = scope
	run.Budget = 2
	granted, err := adm.Acquire(nil, run, []int64{11, 12})
	if err != nil || len(granted) != 2 {
		t.Fatalf("acquire: %v %v", granted, err)
	}
	// Simulate "publish failed": the caller compensates by releasing the batch.
	if n := adm.ReleaseItems(nil, run, granted); n != 2 {
		t.Fatalf("released=%d want 2", n)
	}
	ledger, _ := store.Ledger(nil, scope)
	for _, kind := range KnownResources {
		if ledger.Available[kind] != capacity[kind] {
			t.Fatalf("after compensation %s available=%d want %d", kind, ledger.Available[kind], capacity[kind])
		}
	}
}

func TestReleaseRunCoversCompletionFailureCancelAndRetry(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := Scope{Env: "ppe_a", Region: "cn", SpaceID: 1}
	capacity := map[ResourceKind]int64{ResourceItemSlot: 10, ResourceTargetSlot: 10, ResourceSandboxSlot: 10}
	_ = store.EnsureLedger(nil, scope, capacity)
	adm := &Admission{
		Store: store, Clock: clock,
		Limits: Limits{Capacity: capacity, ReservationTTL: time.Minute},
		ModeFn: func(_ Ctx, _ RunKey) Mode { return ModeEnforce },
	}
	run := testRun(1, 10, 100, 0, clock.Now())
	run.Scope = scope
	run.Budget = 5

	// normal completion releases per item
	granted, _ := adm.Acquire(nil, run, []int64{1, 2, 3, 4, 5})
	if len(granted) != 5 {
		t.Fatalf("granted=%v", granted)
	}
	if n := adm.ReleaseItems(nil, run, []int64{1, 2}); n != 2 {
		t.Fatalf("completion release=%d", n)
	}

	// failure/cancel/retry path: whole-run release must refund the remainder
	if n := adm.ReleaseRun(nil, run); n != 3 {
		t.Fatalf("run release=%d want 3", n)
	}
	ledger, _ := store.Ledger(nil, scope)
	for _, kind := range KnownResources {
		if ledger.Available[kind] != capacity[kind] {
			t.Fatalf("%s available=%d want %d", kind, ledger.Available[kind], capacity[kind])
		}
	}
	if runs, _ := store.ListRuns(nil, scope); len(runs) != 0 {
		t.Fatalf("run registration should be removed, got %d", len(runs))
	}
}

func TestSchedulerTickOrdersByPriorityAndDrivesOnlyWithBudget(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := Scope{Env: "ppe_a", Region: "cn"}
	capacity := map[ResourceKind]int64{ResourceItemSlot: 4, ResourceTargetSlot: 4, ResourceSandboxSlot: 4}
	_ = store.EnsureLedger(nil, scope, capacity)

	low := testRun(1, 10, 100, 9, clock.Now())
	high := testRun(1, 20, 200, 1, clock.Now().Add(time.Minute))
	high.RemainingItems = 0 // no items -> no budget
	source := &fakeSource{runs: []ActiveRun{low, high}}
	driver := &fakeDriver{}
	sched := &Scheduler{Store: store, Source: source, Driver: driver, Limits: Limits{Capacity: capacity, ReservationTTL: time.Minute}, Clock: clock}

	res, err := sched.Tick(nil, scope)
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if len(driver.calls) != 1 {
		t.Fatalf("expected only the low run (high has no items), calls=%+v errors=%v", driver.calls, res.Errors)
	}
	if driver.calls[0].key != low.Key || driver.calls[0].budget != low.MaxConcurrency {
		t.Fatalf("drive call=%+v", driver.calls[0])
	}
}

func TestSchedulerRetiresVanishedRunAndReleases(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := Scope{Env: "ppe_a", Region: "cn"}
	capacity := map[ResourceKind]int64{ResourceItemSlot: 2, ResourceTargetSlot: 2, ResourceSandboxSlot: 2}
	_ = store.EnsureLedger(nil, scope, capacity)

	run := testRun(1, 10, 100, 0, clock.Now())
	run.Scope = scope.WithSpace(1)
	run.Budget = 1
	adm := &Admission{Store: store, Clock: clock, Limits: Limits{Capacity: capacity}, ModeFn: func(_ Ctx, _ RunKey) Mode { return ModeEnforce }}
	if g, _ := adm.Acquire(nil, run, []int64{1}); len(g) != 1 {
		t.Fatalf("acquire=%v", g)
	}
	if err := store.RegisterRun(nil, run, time.Hour); err != nil {
		t.Fatal(err)
	}

	driver := &fakeDriver{}
	sched := &Scheduler{Store: store, Source: &fakeSource{runs: nil}, Driver: driver, Limits: Limits{Capacity: capacity}, Clock: clock}
	res, err := sched.Tick(nil, scope)
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if res.RunsRetired != 1 {
		t.Fatalf("retired=%d errors=%v", res.RunsRetired, res.Errors)
	}
	ledger, _ := store.Ledger(nil, scope)
	for _, kind := range KnownResources {
		if ledger.Available[kind] != capacity[kind] {
			t.Fatalf("%s available=%d want %d", kind, ledger.Available[kind], capacity[kind])
		}
	}
}

func TestSchedulerRebuildsDriftedLedger(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := Scope{Env: "ppe_a", Region: "cn"}
	capacity := map[ResourceKind]int64{ResourceItemSlot: 2, ResourceTargetSlot: 2, ResourceSandboxSlot: 2}
	_ = store.EnsureLedger(nil, scope, capacity)
	// Simulate drift: somebody wrote an incorrect available counter.
	store.mu.Lock()
	store.ledger[scope.Namespace()][ResourceItemSlot] = 99
	store.mu.Unlock()

	sched := &Scheduler{
		Store:  store,
		Source: &fakeSource{runs: nil},
		Driver: &fakeDriver{},
		Limits: Limits{Capacity: capacity},
		Clock:  clock,
	}
	res, err := sched.Tick(nil, scope)
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if !res.LedgerRebuilt {
		t.Fatalf("expected rebuild, result=%+v", res)
	}
	ledger, _ := store.Ledger(nil, scope)
	if ledger.Available[ResourceItemSlot] != 2 {
		t.Fatalf("available=%d want 2", ledger.Available[ResourceItemSlot])
	}
}

func TestReconcileExpiredReservationRefunds(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := Scope{Env: "ppe_a", Region: "cn"}
	capacity := map[ResourceKind]int64{ResourceItemSlot: 1, ResourceTargetSlot: 1, ResourceSandboxSlot: 1}
	_ = store.EnsureLedger(nil, scope, capacity)
	key := RunKey{SpaceID: 1, ExptID: 10, ExptRunID: 100}
	if res, err := store.TryReserve(nil, scope, key, 1, map[ResourceKind]int64{ResourceItemSlot: 1}, time.Second); err != nil || res != ReserveGranted {
		t.Fatalf("reserve: %v %v", res, err)
	}
	clock.Advance(2 * time.Second)
	sched := &Scheduler{Store: store, Source: &fakeSource{}, Driver: &fakeDriver{}, Limits: Limits{Capacity: capacity}, Clock: clock}
	res, err := sched.Tick(nil, scope)
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if res.Released != 1 {
		t.Fatalf("released=%d result=%+v", res.Released, res)
	}
	ledger, _ := store.Ledger(nil, scope)
	if ledger.Available[ResourceItemSlot] != 1 {
		t.Fatalf("available=%d want 1", ledger.Available[ResourceItemSlot])
	}
}

func TestHoldsOverflowIsDenied(t *testing.T) {
	run := testRun(1, 10, 100, 0, time.Now())
	if _, err := run.HoldsFor(1 << 31); !errors.Is(err, ErrInvalidReserve) {
		// per-item=1 x 2^31 does not overflow int64; use a large per-item to force it.
		run.PerItem = map[ResourceKind]int64{ResourceItemSlot: 1 << 40}
		if _, err := run.HoldsFor(1 << 30); !errors.Is(err, ErrInvalidReserve) {
			t.Fatalf("expected overflow error")
		}
	}
}

func TestScopeRequiresEnvAndRegion(t *testing.T) {
	for _, s := range []Scope{{}, {Env: "x"}, {Region: "cn"}} {
		if err := s.Validate(); !errors.Is(err, ErrInvalidScope) {
			t.Fatalf("scope %+v should be invalid, got %v", s, err)
		}
	}
	if ns := (Scope{Env: "ppe_a", Region: "cn"}).Namespace(); ns != "qn:central:ppe_a:cn" {
		t.Fatalf("namespace=%s", ns)
	}
}

func TestHeartbeatRenewalPreventsExpiryRelease(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryStore(clock)
	scope := Scope{Env: "ppe_renew", Region: "cn"}
	capacity := map[ResourceKind]int64{ResourceItemSlot: 1, ResourceTargetSlot: 1, ResourceSandboxSlot: 1}
	_ = store.EnsureLedger(nil, scope, capacity)
	key := RunKey{SpaceID: 1, ExptID: 20, ExptRunID: 200}
	if res, err := store.TryReserve(nil, scope, key, 1, map[ResourceKind]int64{ResourceItemSlot: 1}, time.Second); err != nil || res != ReserveGranted {
		t.Fatalf("reserve: %v %v", res, err)
	}
	clock.Advance(500 * time.Millisecond)
	if n, err := store.RenewReservations(nil, scope, key, 10*time.Second); err != nil || n != 1 {
		t.Fatalf("renew: n=%d err=%v", n, err)
	}
	clock.Advance(2 * time.Second)
	sched := &Scheduler{Store: store, Source: &fakeSource{}, Driver: &fakeDriver{}, Limits: Limits{Capacity: capacity}, Clock: clock}
	res, err := sched.Tick(nil, scope)
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if res.Released != 0 {
		t.Fatalf("renewed reservation must not be released, result=%+v", res)
	}
	if res.FinishedAt.IsZero() {
		t.Fatalf("TickResult.FinishedAt must be populated")
	}
	clock.Advance(11 * time.Second)
	res, err = sched.Tick(nil, scope)
	if err != nil {
		t.Fatalf("tick2: %v", err)
	}
	if res.Released != 1 {
		t.Fatalf("expired reservation must be released, result=%+v", res)
	}
	ledger, _ := store.Ledger(nil, scope)
	if ledger.Available[ResourceItemSlot] != 1 {
		t.Fatalf("available=%d want 1", ledger.Available[ResourceItemSlot])
	}
}
