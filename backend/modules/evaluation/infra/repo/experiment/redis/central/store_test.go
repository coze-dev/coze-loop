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

func newTestStore(t *testing.T) (*Store, infraredis.Cmdable) {
	t.Helper()
	cmd := infraredis.NewTestRedis(t)
	return NewStore(cmd), cmd
}

func testScope() domain.Scope {
	return domain.Scope{Env: "ppe_ut_lane", Region: "cn"}
}

func testKey() domain.RunKey {
	return domain.RunKey{SpaceID: 101, ExptID: 202, ExptRunID: 303}
}

func holds() map[domain.ResourceKind]int64 {
	return map[domain.ResourceKind]int64{
		domain.ResourceItemSlot:    1,
		domain.ResourceTargetSlot:  1,
		domain.ResourceSandboxSlot: 1,
	}
}

func TestRedisReserveIsAtomicAcrossResources(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	scope := testScope()
	capacity := map[domain.ResourceKind]int64{
		domain.ResourceItemSlot:    5,
		domain.ResourceTargetSlot:  5,
		domain.ResourceSandboxSlot: 1,
	}
	if err := store.EnsureLedger(ctx, scope, capacity); err != nil {
		t.Fatal(err)
	}
	if res, err := store.TryReserve(ctx, scope, testKey(), 1, holds(), time.Minute); err != nil || res != domain.ReserveGranted {
		t.Fatalf("first reserve: %v %v", res, err)
	}
	// sandbox exhausted; item/target still available => must deny atomically.
	if res, err := store.TryReserve(ctx, scope, testKey(), 2, holds(), time.Minute); err != nil || res != domain.ReserveDenied {
		t.Fatalf("second reserve must be denied: %v %v", res, err)
	}
	ledger, err := store.Ledger(ctx, scope)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Available[domain.ResourceItemSlot] != 4 {
		t.Fatalf("item available=%d want 4", ledger.Available[domain.ResourceItemSlot])
	}
}

func TestRedisReleaseIsIdempotent(t *testing.T) {
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
		t.Fatalf("reserve: %v %v", res, err)
	}
	id := domain.ReservationID(key, 7)
	if ok, err := store.Release(ctx, scope, id); err != nil || !ok {
		t.Fatalf("release: %v %v", ok, err)
	}
	if ok, err := store.Release(ctx, scope, id); err != nil || ok {
		t.Fatalf("second release should be no-op: %v %v", ok, err)
	}
	ledger, _ := store.Ledger(ctx, scope)
	for _, kind := range domain.KnownResources {
		if ledger.Available[kind] != capacity[kind] {
			t.Fatalf("%s available=%d want %d", kind, ledger.Available[kind], capacity[kind])
		}
	}
}

func TestRedisCrossEnvironmentReleaseRejected(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	owner := domain.Scope{Env: "online_cn", Region: "cn"}
	other := domain.Scope{Env: "ppe_other", Region: "cn"}
	if err := store.EnsureLedger(ctx, owner, map[domain.ResourceKind]int64{domain.ResourceItemSlot: 1}); err != nil {
		t.Fatal(err)
	}
	key := testKey()
	if res, err := store.TryReserve(ctx, owner, key, 1, map[domain.ResourceKind]int64{domain.ResourceItemSlot: 1}, time.Minute); err != nil || res != domain.ReserveGranted {
		t.Fatalf("reserve: %v %v", res, err)
	}
	// Keys are namespaced by env/region: another environment literally cannot
	// address this reservation, so release is a no-op (not found), never a mutation.
	if ok, err := store.Release(ctx, other, domain.ReservationID(key, 1)); err != nil || ok {
		t.Fatalf("cross-env release must not touch the owner's key: ok=%v err=%v", ok, err)
	}
	if _, found, err := store.GetReservation(ctx, other, domain.ReservationID(key, 1)); err != nil || found {
		t.Fatalf("cross-env get must not find the reservation: found=%v err=%v", found, err)
	}
	ownerLedger, _ := store.Ledger(ctx, owner)
	if ownerLedger.Available[domain.ResourceItemSlot] != 0 {
		t.Fatalf("owner's reservation must remain held, available=%d", ownerLedger.Available[domain.ResourceItemSlot])
	}
}

func TestRedisRebuildLedgerRepairsDrift(t *testing.T) {
	store, cmd := newTestStore(t)
	ctx := context.Background()
	scope := testScope()
	capacity := map[domain.ResourceKind]int64{
		domain.ResourceItemSlot: 3, domain.ResourceTargetSlot: 3, domain.ResourceSandboxSlot: 3,
	}
	if err := store.EnsureLedger(ctx, scope, capacity); err != nil {
		t.Fatal(err)
	}
	if res, err := store.TryReserve(ctx, scope, testKey(), 1, holds(), time.Minute); err != nil || res != domain.ReserveGranted {
		t.Fatalf("reserve: %v %v", res, err)
	}
	// Corrupt the counter (simulates a crashed/committed partial write).
	if err := cmd.HSet(ctx, scope.Namespace()+":ledger", string(domain.ResourceItemSlot), 99).Err(); err != nil {
		t.Fatal(err)
	}
	ledger, err := store.RebuildLedger(ctx, scope, capacity)
	if err != nil {
		t.Fatal(err)
	}
	// one item held => available = 3-1
	if ledger.Available[domain.ResourceItemSlot] != 2 {
		t.Fatalf("rebuild available=%d want 2", ledger.Available[domain.ResourceItemSlot])
	}
}

func TestRedisKeysAreNamespacedWithTTL(t *testing.T) {
	store, cmd := newTestStore(t)
	ctx := context.Background()
	scope := testScope()
	if err := store.EnsureLedger(ctx, scope, map[domain.ResourceKind]int64{domain.ResourceItemSlot: 1}); err != nil {
		t.Fatal(err)
	}
	key := testKey()
	if res, err := store.TryReserve(ctx, scope, key, 1, map[domain.ResourceKind]int64{domain.ResourceItemSlot: 1}, time.Minute); err != nil || res != domain.ReserveGranted {
		t.Fatalf("reserve: %v %v", res, err)
	}
	if err := store.RegisterRun(ctx, domain.ActiveRun{
		Scope: scope.WithSpace(key.SpaceID), Key: key, PerItem: map[domain.ResourceKind]int64{domain.ResourceItemSlot: 1},
	}, time.Minute); err != nil {
		t.Fatal(err)
	}
	raw, ok := infraredis.Unwrap(cmd)
	if !ok {
		t.Fatal("cannot unwrap redis client")
	}
	keys, err := raw.Keys(ctx, "*").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) == 0 {
		t.Fatal("no keys created")
	}
	for _, k := range keys {
		if len(k) < len("qn:central:") || k[:len("qn:central:")] != "qn:central:" {
			t.Fatalf("key %q must start with qn:central:", k)
		}
		ttl, terr := raw.TTL(ctx, k).Result()
		if terr != nil {
			t.Fatal(terr)
		}
		if ttl == -1 {
			t.Fatalf("key %q has no TTL", k)
		}
	}
}

func TestRedisCleanupRemovesOnlyOwnNamespace(t *testing.T) {
	store, cmd := newTestStore(t)
	ctx := context.Background()
	scopeA := domain.Scope{Env: "ppe_a", Region: "cn"}
	scopeB := domain.Scope{Env: "ppe_b", Region: "cn"}
	for _, scope := range []domain.Scope{scopeA, scopeB} {
		if err := store.EnsureLedger(ctx, scope, map[domain.ResourceKind]int64{domain.ResourceItemSlot: 1}); err != nil {
			t.Fatal(err)
		}
		if res, err := store.TryReserve(ctx, scope, testKey(), 1, map[domain.ResourceKind]int64{domain.ResourceItemSlot: 1}, time.Minute); err != nil || res != domain.ReserveGranted {
			t.Fatalf("reserve %s: %v %v", scope.Env, res, err)
		}
	}
	if err := store.Cleanup(ctx, scopeA); err != nil {
		t.Fatal(err)
	}
	raw, _ := infraredis.Unwrap(cmd)
	keysA, _ := raw.Keys(ctx, scopeA.Namespace()+"*").Result()
	if len(keysA) != 0 {
		t.Fatalf("cleanup left %d keys: %v", len(keysA), keysA)
	}
	keysB, _ := raw.Keys(ctx, scopeB.Namespace()+"*").Result()
	if len(keysB) == 0 {
		t.Fatal("cleanup must not touch another environment's keys")
	}
}
