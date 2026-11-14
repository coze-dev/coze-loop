// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestReservationStore_AtomicMultiResourceReserve(t *testing.T) {
	ctx := context.Background()
	store := NewCentralReservationStore(infraredis.NewTestRedis(t))

	capabilities := map[string]int64{
		entity.CentralResourceItem:   2,
		entity.CentralResourceTarget: 2,
	}
	first := &entity.CentralReserveRequest{
		Env: "ppe_test", ExptID: 1, SpaceID: 1, RunID: 1, ItemID: 11,
		ReservationID: entity.CentralReservationID(1, 1, 11),
		Claims:        entity.CentralResourceClaims{entity.CentralResourceItem: 1, entity.CentralResourceTarget: 1},
		Capacities:    capabilities,
		TTL:           time.Minute,
	}
	res, err := store.Reserve(ctx, first)
	require.NoError(t, err)
	require.True(t, res.Granted)

	// This request fits the target pool but not the item pool. A correct
	// implementation must not increment the target counter either.
	second := &entity.CentralReserveRequest{
		Env: "ppe_test", ExptID: 1, SpaceID: 1, RunID: 1, ItemID: 12,
		ReservationID: entity.CentralReservationID(1, 1, 12),
		Claims:        entity.CentralResourceClaims{entity.CentralResourceItem: 2, entity.CentralResourceTarget: 1},
		Capacities:    capabilities,
		TTL:           time.Minute,
	}
	res, err = store.Reserve(ctx, second)
	require.NoError(t, err)
	assert.False(t, res.Granted)
	assert.Equal(t, entity.CentralResourceItem, res.ConstrainedBy)

	list, err := store.List(ctx, "ppe_test", 1, 1)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, int64(11), list[0].ItemID)
}

func TestReservationStore_IdempotentReserveAndRelease(t *testing.T) {
	ctx := context.Background()
	store := NewCentralReservationStore(infraredis.NewTestRedis(t))
	req := &entity.CentralReserveRequest{
		Env: "ppe_test", ExptID: 1, SpaceID: 1, RunID: 1, ItemID: 11,
		ReservationID: entity.CentralReservationID(1, 1, 11),
		Claims:        entity.CentralResourceClaims{entity.CentralResourceItem: 1},
		Capacities:    map[string]int64{entity.CentralResourceItem: 5},
		TTL:           time.Minute,
	}
	first, err := store.Reserve(ctx, req)
	require.NoError(t, err)
	require.True(t, first.Granted)

	again, err := store.Reserve(ctx, req)
	require.NoError(t, err)
	assert.True(t, again.Granted)
	assert.True(t, again.AlreadyHeld)

	require.NoError(t, store.MarkDispatched(ctx, "ppe_test", req.ReservationID))
	require.NoError(t, store.MarkDispatched(ctx, "ppe_test", req.ReservationID))
	ledger, err := store.Ledger(ctx, "ppe_test", 1, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, ledger.Dispatched)

	released, err := store.Release(ctx, "ppe_test", req.ReservationID)
	require.NoError(t, err)
	assert.True(t, released)
	releasedAgain, err := store.Release(ctx, "ppe_test", req.ReservationID)
	require.NoError(t, err)
	assert.False(t, releasedAgain)

	// Re-reserve after release proves the counter was restored.
	res, err := store.Reserve(ctx, req)
	require.NoError(t, err)
	assert.True(t, res.Granted)
}

func TestReservationStore_ReleaseRunAndLedger(t *testing.T) {
	ctx := context.Background()
	store := NewCentralReservationStore(infraredis.NewTestRedis(t))
	caps := map[string]int64{entity.CentralResourceItem: 10}
	for _, itemID := range []int64{11, 12} {
		_, err := store.Reserve(ctx, &entity.CentralReserveRequest{
			Env: "ppe_test", ExptID: 1, SpaceID: 1, RunID: 1, ItemID: itemID,
			ReservationID: entity.CentralReservationID(1, 1, itemID),
			Claims:        entity.CentralResourceClaims{entity.CentralResourceItem: 1},
			Capacities:    caps, TTL: time.Minute,
		})
		require.NoError(t, err)
	}
	ledger, err := store.Ledger(ctx, "ppe_test", 1, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, ledger.Held)

	n, err := store.ReleaseRun(ctx, "ppe_test", 1, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	list, err := store.List(ctx, "ppe_test", 1, 1)
	require.NoError(t, err)
	assert.Empty(t, list)
	ledger, err = store.Ledger(ctx, "ppe_test", 1, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, ledger.Released)
	assert.Equal(t, 0, ledger.Held)
}

func TestOwnershipStore_NeverStealsForeignOwner(t *testing.T) {
	ctx := context.Background()
	store := NewCentralOwnershipStore(infraredis.NewTestRedis(t))
	ok, holder, err := store.Claim(ctx, 1, 100, "prod", time.Hour)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "prod", holder)

	ok, holder, err = store.Claim(ctx, 1, 100, "ppe_test", time.Hour)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "prod", holder)

	owner, err := store.Owner(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "prod", owner)

	require.NoError(t, store.Release(ctx, 1, "ppe_test"))
	owner, err = store.Owner(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "prod", owner)

	require.NoError(t, store.Release(ctx, 1, "prod"))
	owner, err = store.Owner(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "", owner)
}

func TestRedisCentralLease(t *testing.T) {
	ctx := context.Background()
	lease := NewRedisCentralLease(infraredis.NewTestRedis(t))
	ok, err := lease.Acquire(ctx, "qn:central:ppe_test:lease:scanner", time.Minute)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = lease.Acquire(ctx, "qn:central:ppe_test:lease:scanner", time.Minute)
	require.NoError(t, err)
	assert.False(t, ok)
	require.NoError(t, lease.Release(ctx, "qn:central:ppe_test:lease:scanner"))
	ok, err = lease.Acquire(ctx, "qn:central:ppe_test:lease:scanner", time.Minute)
	require.NoError(t, err)
	assert.True(t, ok)
}
