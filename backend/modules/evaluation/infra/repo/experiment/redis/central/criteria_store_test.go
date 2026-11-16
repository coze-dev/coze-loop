// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domain "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/central"
)

func TestRedisStateTransitionsAndReleaseAreIdempotent(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	scope := testScope()
	capacity := map[domain.ResourceKind]int64{
		domain.ResourceItemSlot: 2, domain.ResourceTargetSlot: 2, domain.ResourceSandboxSlot: 2,
	}
	if err := store.EnsureLedger(ctx, scope, capacity); err != nil {
		t.Fatal(err)
	}
	key := testKey()
	if res, err := store.TryReserve(ctx, scope, key, 7, holds(), time.Minute); err != nil || res != domain.ReserveGranted {
		t.Fatalf("reserve=%v err=%v", res, err)
	}
	id := domain.ReservationID(scope, key, 7)
	for _, step := range []domain.ReservationState{
		domain.ReservationDispatched, domain.ReservationRunning,
		domain.ReservationCompleted, domain.ReservationPendingRelease,
	} {
		ok, state, err := store.Transition(ctx, scope, id, "", step, "test", time.Time{})
		if err != nil || !ok || state != step {
			t.Fatalf("transition -> %s ok=%v state=%s err=%v", step, ok, state, err)
		}
		ok, state, err = store.Transition(ctx, scope, id, "", step, "test", time.Time{})
		if err != nil || !ok || state != step {
			t.Fatalf("repeat -> %s ok=%v state=%s err=%v", step, ok, state, err)
		}
	}
	if ok, err := store.Release(ctx, scope, id, "test"); err != nil || !ok {
		t.Fatalf("release ok=%v err=%v", ok, err)
	}
	if ok, err := store.Release(ctx, scope, id, "test"); err != nil || ok {
		t.Fatalf("duplicate release must be no-op, ok=%v err=%v", ok, err)
	}
	rec, found, err := store.GetReservation(ctx, scope, id)
	if err != nil || !found || rec.State != domain.ReservationReleased {
		t.Fatalf("released state must remain observable: found=%v rec=%+v err=%v", found, rec, err)
	}
	ledger, _ := store.Ledger(ctx, scope)
	if ledger.Available[domain.ResourceItemSlot] != 2 {
		t.Fatalf("available=%d want 2", ledger.Available[domain.ResourceItemSlot])
	}
}

func TestRedisDispatchClaimHasSingleWinner(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	scope := testScope()
	capacity := map[domain.ResourceKind]int64{
		domain.ResourceItemSlot: 2, domain.ResourceTargetSlot: 2, domain.ResourceSandboxSlot: 2,
	}
	if err := store.EnsureLedger(ctx, scope, capacity); err != nil {
		t.Fatal(err)
	}
	key := testKey()
	if res, err := store.TryReserve(ctx, scope, key, 7, holds(), time.Minute); err != nil || res != domain.ReserveGranted {
		t.Fatalf("reserve=%v err=%v", res, err)
	}
	id := domain.ReservationID(scope, key, 7)
	var winners atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, state, err := store.Claim(ctx, scope, id, "dispatch_claim", time.Time{})
			if err == nil && ok && state == domain.ReservationDispatched {
				winners.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if got := winners.Load(); got != 1 {
		t.Fatalf("claim winners=%d want exactly 1", got)
	}
	rec, found, err := store.GetReservation(ctx, scope, id)
	if err != nil || !found || rec.State != domain.ReservationDispatched {
		t.Fatalf("state=%+v found=%v err=%v", rec, found, err)
	}
}

func TestRedisIncrementalAndFullReconcileGeneration(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	scope := testScope()
	capacity := map[domain.ResourceKind]int64{
		domain.ResourceItemSlot: 2, domain.ResourceTargetSlot: 2, domain.ResourceSandboxSlot: 2,
	}
	if err := store.EnsureLedger(ctx, scope, capacity); err != nil {
		t.Fatal(err)
	}
	if res, err := store.TryReserve(ctx, scope, testKey(), 1, holds(), time.Minute); err != nil || res != domain.ReserveGranted {
		t.Fatalf("reserve=%v err=%v", res, err)
	}
	l1, needFull, err := store.IncrementalReconcile(ctx, scope, capacity, 0)
	if err != nil || needFull || l1.ReconcileMode != "incremental" || l1.Generation != 0 {
		t.Fatalf("incremental=%+v needFull=%v err=%v", l1, needFull, err)
	}
	if l1.Available[domain.ResourceItemSlot] != 1 || l1.Reserved[domain.ResourceItemSlot] != 1 {
		t.Fatalf("incremental ledger=%+v", l1)
	}
	l2, err := store.RebuildLedger(ctx, scope, capacity)
	if err != nil || l2.ReconcileMode != "full" || l2.Generation != 1 {
		t.Fatalf("full=%+v err=%v", l2, err)
	}
}

func TestRedisLeadershipIsSingleHolder(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	scope := testScope()
	if ok, err := store.AcquireLeadership(ctx, scope, "pod-a", time.Minute); err != nil || !ok {
		t.Fatalf("pod-a acquire=%v err=%v", ok, err)
	}
	if ok, err := store.AcquireLeadership(ctx, scope, "pod-b", time.Minute); err != nil || ok {
		t.Fatalf("pod-b must be follower, ok=%v err=%v", ok, err)
	}
	if ok, err := store.RenewLeadership(ctx, scope, "pod-a", time.Minute); err != nil || !ok {
		t.Fatalf("pod-a renew=%v err=%v", ok, err)
	}
	if err := store.ReleaseLeadership(ctx, scope, "pod-a"); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.AcquireLeadership(ctx, scope, "pod-b", time.Minute); err != nil || !ok {
		t.Fatalf("pod-b takeover=%v err=%v", ok, err)
	}
}
