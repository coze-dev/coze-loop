// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func testScope() entity.CentralEnvScope {
	return entity.CentralEnvScope{Env: "stone.cozeloop.evaluation", Lane: "ppe_central_test"}
}

func testCapacities() map[entity.CentralResourceKind]int64 {
	return map[entity.CentralResourceKind]int64{
		entity.CentralResourceItemSlot:       1,
		entity.CentralResourceEvaluatorSlot:  2,
		entity.CentralResourceSandboxExecute: 1,
	}
}

func newLease(exptID, runID, itemID int64, claims ...entity.CentralResourceClaim) *entity.CentralLease {
	return &entity.CentralLease{
		LeaseID:    entity.MakeCentralLeaseID(exptID, runID, itemID),
		SpaceID:    100,
		ExptID:     exptID,
		ExptRunID:  runID,
		ItemID:     itemID,
		Claims:     claims,
		AcquiredAt: time.Now().Unix(),
	}
}

func TestCentralKeysUseRequiredPrefixAndScope(t *testing.T) {
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()
	usage, leaseIndex, active, leaseKey := dao.Keys(scope)
	keys := []string{usage, leaseIndex, active, leaseKey("lease-1")}
	for _, key := range keys {
		assert.True(t, strings.HasPrefix(key, CentralKeyPrefix), "key must start with %s, got %s", CentralKeyPrefix, key)
		assert.Contains(t, key, "stone_cozeloop_evaluation")
		assert.Contains(t, key, "ppe_central_test")
	}
}

func TestAcquireIsAtomicAllOrNothing(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()
	caps := testCapacities()

	first := newLease(1, 2, 3,
		entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1},
		entity.CentralResourceClaim{Kind: entity.CentralResourceEvaluatorSlot, Amount: 1},
	)
	ok, err := dao.Acquire(ctx, scope, caps, first, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	// item_slot 容量为 1，第二个 item 即使 evaluator 仍有余量也必须整体失败。
	second := newLease(1, 2, 4,
		entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1},
		entity.CentralResourceClaim{Kind: entity.CentralResourceEvaluatorSlot, Amount: 1},
	)
	ok, err = dao.Acquire(ctx, scope, caps, second, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "must fail all-or-nothing when any resource is exhausted")

	usage, err := dao.Usage(ctx, scope)
	require.NoError(t, err)
	assert.Equal(t, int64(1), usage[entity.CentralResourceItemSlot])
	assert.Equal(t, int64(1), usage[entity.CentralResourceEvaluatorSlot], "failed acquire must not partially consume evaluator_slot")
}

func TestAcquireSameLeaseIsIdempotent(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()
	caps := testCapacities()
	lease := newLease(1, 2, 3, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1})

	ok, err := dao.Acquire(ctx, scope, caps, lease, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = dao.Acquire(ctx, scope, caps, lease, time.Minute)
	require.NoError(t, err)
	assert.True(t, ok, "same lease should be treated as already held")

	usage, err := dao.Usage(ctx, scope)
	require.NoError(t, err)
	assert.Equal(t, int64(1), usage[entity.CentralResourceItemSlot], "idempotent acquire must not double count")
}

func TestReleaseIsIdempotentAndRestoresUsage(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()
	caps := testCapacities()
	lease := newLease(1, 2, 3,
		entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1},
		entity.CentralResourceClaim{Kind: entity.CentralResourceEvaluatorSlot, Amount: 2},
	)

	ok, err := dao.Acquire(ctx, scope, caps, lease, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	released, err := dao.Release(ctx, scope, lease.LeaseID)
	require.NoError(t, err)
	assert.True(t, released)

	usage, err := dao.Usage(ctx, scope)
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot])
	assert.Equal(t, int64(0), usage[entity.CentralResourceEvaluatorSlot])

	released, err = dao.Release(ctx, scope, lease.LeaseID)
	require.NoError(t, err)
	assert.False(t, released, "second release must be a no-op")
}

func TestLeaseHasTTL(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()
	lease := newLease(1, 2, 3, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1})
	ok, err := dao.Acquire(ctx, scope, testCapacities(), lease, 90*time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	_, _, _, leaseKey := dao.Keys(scope)
	raw, ok := redis.Unwrap(dao.cmdable)
	require.True(t, ok)
	ttl, err := raw.TTL(ctx, leaseKey(lease.LeaseID)).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0), "lease key must have a TTL")
}

func TestRebuildUsageRepairsDrift(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()
	caps := testCapacities()
	lease := newLease(1, 2, 3, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1})
	_, err := dao.Acquire(ctx, scope, caps, lease, time.Minute)
	require.NoError(t, err)

	usageKey, _, _, _ := dao.Keys(scope)
	// 模拟账本漂移：usage 被人为抬高到 99。
	require.NoError(t, dao.cmdable.HSet(ctx, usageKey, string(entity.CentralResourceItemSlot), 99).Err())

	before, after, repaired, err := dao.RebuildUsage(ctx, scope, caps, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(99), before[entity.CentralResourceItemSlot])
	assert.Equal(t, int64(1), after[entity.CentralResourceItemSlot])
	assert.Equal(t, 1, repaired)

	usage, err := dao.Usage(ctx, scope)
	require.NoError(t, err)
	assert.Equal(t, int64(1), usage[entity.CentralResourceItemSlot])
}

func TestActiveRegistryOrdersByPriorityAndUnregisters(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()

	require.NoError(t, dao.RegisterActive(ctx, scope, &entity.CentralActiveRun{SpaceID: 100, ExptID: 1, ExptRunID: 11, Priority: 50}, time.Hour))
	require.NoError(t, dao.RegisterActive(ctx, scope, &entity.CentralActiveRun{SpaceID: 100, ExptID: 2, ExptRunID: 22, Priority: 5, SessionUserID: "user-2"}, time.Hour))

	runs, err := dao.ListActive(ctx, scope)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, int64(2), runs[0].ExptID, "smaller priority value must come first")
	assert.Equal(t, 5, runs[0].Priority)
	assert.Equal(t, "user-2", runs[0].SessionUserID)
	assert.Equal(t, int64(1), runs[1].ExptID)

	require.NoError(t, dao.UnregisterActive(ctx, scope, 2, 22))
	runs, err = dao.ListActive(ctx, scope)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, int64(1), runs[0].ExptID)
}

func TestReleaseRunReleasesOnlyTargetRun(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()
	caps := map[entity.CentralResourceKind]int64{entity.CentralResourceItemSlot: 3}

	for _, lease := range []*entity.CentralLease{
		newLease(1, 2, 3, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1}),
		newLease(1, 2, 4, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1}),
		newLease(1, 5, 6, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1}),
	} {
		ok, err := dao.Acquire(ctx, scope, caps, lease, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
	}

	released, err := dao.ReleaseRun(ctx, scope, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, released)

	leases, err := dao.ListLeases(ctx, scope)
	require.NoError(t, err)
	require.Len(t, leases, 1)
	assert.Equal(t, int64(5), leases[0].ExptRunID)
}

func TestAcquireLeaderIsExclusiveWithTTL(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()

	ok, err := dao.AcquireLeader(ctx, scope, "instance-a", 30*time.Second)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = dao.AcquireLeader(ctx, scope, "instance-b", 30*time.Second)
	require.NoError(t, err)
	assert.False(t, ok, "second instance must not hold leader lock")
}
