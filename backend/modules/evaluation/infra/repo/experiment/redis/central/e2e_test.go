// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"context"
	"testing"
	"time"

	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	domain "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/central"
)

type e2eSource struct {
	runs []domain.ActiveRun
}

func (s *e2eSource) ListActive(_ domain.Ctx, _ domain.Scope) ([]domain.ActiveRun, error) {
	return s.runs, nil
}

type e2eDriver struct {
	admission *domain.Admission
	order     []domain.RunKey
	granted   map[domain.RunKey][]int64
}

func (d *e2eDriver) Drive(ctx domain.Ctx, run domain.ActiveRun) error {
	d.order = append(d.order, run.Key)
	granted, err := d.admission.Acquire(ctx, run, []int64{101})
	if err != nil {
		return err
	}
	d.granted[run.Key] = granted
	return nil
}

// TestCentralE2EPriorityReserveReleaseCleanup drives the complete central chain
// (scan -> priority sort -> run budget -> atomic Redis reservation -> item
// release -> run retirement -> scope cleanup) against a real Redis protocol
// implementation (miniredis), using the same Lua scripts as production.
func TestCentralE2EPriorityReserveReleaseCleanup(t *testing.T) {
	cmd := infraredis.NewTestRedis(t)
	store := NewStore(cmd)
	ctx := context.Background()
	scope := domain.Scope{Env: "ppe_e2e", Region: "cn"}
	capacity := map[domain.ResourceKind]int64{
		domain.ResourceItemSlot: 4, domain.ResourceTargetSlot: 4, domain.ResourceSandboxSlot: 4,
	}
	if err := store.EnsureLedger(ctx, scope, capacity); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	high := domain.ActiveRun{
		Key:        domain.RunKey{SpaceID: 11, ExptID: 21, ExptRunID: 201},
		EnqueuedAt: now, UpdatedAt: now, MaxConcurrency: 1, RemainingItems: 1,
		PerItem: map[domain.ResourceKind]int64{
			domain.ResourceItemSlot: 1, domain.ResourceTargetSlot: 1, domain.ResourceSandboxSlot: 1,
		},
	}
	low := domain.ActiveRun{
		Key:        domain.RunKey{SpaceID: 11, ExptID: 22, ExptRunID: 202},
		EnqueuedAt: now.Add(time.Hour), UpdatedAt: now, MaxConcurrency: 1, RemainingItems: 1,
		PerItem: map[domain.ResourceKind]int64{
			domain.ResourceItemSlot: 1, domain.ResourceTargetSlot: 1, domain.ResourceSandboxSlot: 1,
		},
	}
	if err := store.SetPriority(ctx, scope.WithSpace(11), high.Key, 1, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPriority(ctx, scope.WithSpace(11), low.Key, 9, time.Hour); err != nil {
		t.Fatal(err)
	}
	admission := &domain.Admission{
		Store:  store,
		Clock:  domain.DefaultClock(),
		Limits: domain.Limits{Capacity: capacity, ReservationTTL: time.Minute},
		ModeFn: func(domain.Ctx, domain.RunKey) domain.Mode { return domain.ModeEnforce },
	}
	driver := &e2eDriver{admission: admission, granted: map[domain.RunKey][]int64{}}
	source := &e2eSource{runs: []domain.ActiveRun{low, high}}
	sched := &domain.Scheduler{
		Store: store, Source: source, Driver: driver,
		Limits: domain.Limits{Capacity: capacity, ReservationTTL: time.Minute, DispatchGrace: time.Minute},
		Clock:  domain.DefaultClock(),
	}
	res, err := sched.Tick(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("tick errors: %v result=%+v granted=%v", res.Errors, res, driver.granted)
	}
	if res.RunsDriven != 2 || len(driver.order) != 2 {
		t.Fatalf("driven=%d order=%v result=%+v", res.RunsDriven, driver.order, res)
	}
	if driver.order[0] != high.Key || driver.order[1] != low.Key {
		t.Fatalf("priority order violated: %v", driver.order)
	}
	for _, run := range []domain.ActiveRun{high, low} {
		if len(driver.granted[run.Key]) != 1 {
			t.Fatalf("run %s was not granted: %v", run.Key, driver.granted)
		}
	}
	ledger, err := store.Ledger(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Available[domain.ResourceItemSlot] != 2 || ledger.Reserved[domain.ResourceItemSlot] != 2 {
		t.Fatalf("after dispatch available=%d reserved=%d, want 2/2",
			ledger.Available[domain.ResourceItemSlot], ledger.Reserved[domain.ResourceItemSlot])
	}

	// Item completion releases both items; run completion retires both runs.
	for _, run := range source.runs {
		itemRun := run
		itemRun.Scope = scope.WithSpace(run.Key.SpaceID)
		if n := admission.ReleaseItems(ctx, itemRun, []int64{101}); n != 1 {
			t.Fatalf("release items for %s = %d", run.Key, n)
		}
	}
	source.runs = nil
	res, err = sched.Tick(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if res.RunsRetired != 2 {
		t.Fatalf("retired=%d result=%+v", res.RunsRetired, res)
	}
	ledger, _ = store.Ledger(ctx, scope)
	for _, kind := range domain.KnownResources {
		if ledger.Available[kind] != capacity[kind] || ledger.Reserved[kind] != 0 {
			t.Fatalf("%s available=%d reserved=%d want %d/0", kind, ledger.Available[kind], ledger.Reserved[kind], capacity[kind])
		}
	}

	// Scope cleanup must remove every key this run created and nothing else.
	if err := store.Cleanup(ctx, scope); err != nil {
		t.Fatal(err)
	}
	raw, ok := infraredis.Unwrap(cmd)
	if !ok {
		t.Fatal("cannot unwrap redis client")
	}
	left, err := raw.Keys(ctx, scope.Namespace()+"*").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("cleanup left keys: %v", left)
	}
}
