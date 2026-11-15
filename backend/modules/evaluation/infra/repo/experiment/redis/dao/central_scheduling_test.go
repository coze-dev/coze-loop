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
	keys := dao.Keys(scope)
	all := []string{keys.Usage, keys.LeaseIndex, keys.Active, keys.Pending, keys.PendingConfirm, keys.Seq, keys.Leader, keys.Lease("lease-1")}
	for _, key := range all {
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

	keys := dao.Keys(scope)
	raw, ok := redis.Unwrap(dao.cmdable)
	require.True(t, ok)
	ttl, err := raw.TTL(ctx, keys.Lease(lease.LeaseID)).Result()
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

	usageKey := dao.Keys(scope).Usage
	// 模拟账本漂移：usage 被人为抬高到 99。
	require.NoError(t, dao.cmdable.HSet(ctx, usageKey, string(entity.CentralResourceItemSlot), 99).Err())

	before, after, repaired, drift, err := dao.RebuildUsage(ctx, scope, caps, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(99), before[entity.CentralResourceItemSlot])
	assert.Equal(t, int64(1), after[entity.CentralResourceItemSlot])
	assert.Equal(t, 1, repaired)
	require.NotEmpty(t, drift, "rebuild must locate drift to a specific lease")
	assert.Equal(t, lease.LeaseID, drift[0].LeaseID)

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

	ok, err = dao.AcquireLeader(ctx, scope, "instance-a", 30*time.Second)
	require.NoError(t, err)
	assert.True(t, ok, "same owner must be able to renew the leader lock")

	ok, err = dao.AcquireLeader(ctx, scope, "instance-b", 30*time.Second)
	require.NoError(t, err)
	assert.False(t, ok, "second instance must not hold leader lock")
}

func TestLeaseStatesAndPendingConfirmSweep(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()
	caps := map[entity.CentralResourceKind]int64{entity.CentralResourceItemSlot: 2}

	dispatched := newLease(1, 2, 3, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1})
	dispatched.State = entity.CentralLeaseStateReserved
	ok, err := dao.Acquire(ctx, scope, caps, dispatched, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = dao.MarkDispatched(ctx, scope, dispatched.LeaseID)
	require.NoError(t, err)
	assert.True(t, ok)
	// 单一状态：已 dispatched 的 lease 不能被反向改成 pending_confirm。
	ok, err = dao.MarkPendingConfirm(ctx, scope, dispatched.LeaseID, time.Now().Unix())
	require.NoError(t, err)
	assert.False(t, ok)

	pending := newLease(1, 2, 4, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1})
	ok, err = dao.Acquire(ctx, scope, caps, pending, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = dao.MarkPendingConfirm(ctx, scope, pending.LeaseID, time.Now().Unix()-1)
	require.NoError(t, err)
	assert.True(t, ok)

	released, err := dao.SweepPendingConfirms(ctx, scope, time.Now().Unix(), time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, released, "expired pending_confirm must be compensated and released")

	// 迟到确认不能复活已被清扫的预占，也不能重复扣减。
	ok, err = dao.MarkDispatched(ctx, scope, pending.LeaseID)
	require.NoError(t, err)
	assert.False(t, ok)
	usage, err := dao.Usage(ctx, scope)
	require.NoError(t, err)
	assert.Equal(t, int64(1), usage[entity.CentralResourceItemSlot], "only the dispatched lease remains reserved")

	released, err = dao.SweepPendingConfirms(ctx, scope, time.Now().Unix(), time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, released, "sweeper must be idempotent")
}

func TestNextAttemptIsUniqueAndReleaseItemsCoversAllAttempts(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()
	caps := map[entity.CentralResourceKind]int64{entity.CentralResourceItemSlot: 3}

	a1, err := dao.NextAttempt(ctx, scope, 1, 2, 3, time.Minute)
	require.NoError(t, err)
	a2, err := dao.NextAttempt(ctx, scope, 1, 2, 3, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), a1)
	assert.Equal(t, int64(2), a2)

	for _, attempt := range []int64{a1, a2} {
		lease := &entity.CentralLease{
			LeaseID: entity.MakeCentralLeaseIDWithAttempt(1, 2, 3, attempt),
			SpaceID: 100, ExptID: 1, ExptRunID: 2, ItemID: 3, Attempt: attempt,
			Claims: []entity.CentralResourceClaim{{Kind: entity.CentralResourceItemSlot, Amount: 1}},
		}
		ok, err := dao.Acquire(ctx, scope, caps, lease, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
	}
	assert.NotEqual(t, entity.MakeCentralLeaseIDWithAttempt(1, 2, 3, a1), entity.MakeCentralLeaseIDWithAttempt(1, 2, 3, a2))

	released, err := dao.ReleaseItems(ctx, scope, 1, 2, []int64{3})
	require.NoError(t, err)
	assert.Equal(t, 2, released, "release must cover every attempt of the item")
	usage, err := dao.Usage(ctx, scope)
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot])

	released, err = dao.ReleaseItems(ctx, scope, 1, 2, []int64{3})
	require.NoError(t, err)
	assert.Equal(t, 0, released, "release must be idempotent")
}

func TestPendingAndActiveAreSeparated(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()
	state := &entity.CentralRunState{SpaceID: 100, ExptID: 1, ExptRunID: 2, Mode: entity.CentralSchedulerModeEnforce, Priority: 5}
	actual, created, err := dao.EnsureRunState(ctx, scope, state, time.Hour)
	require.NoError(t, err)
	assert.True(t, created)
	require.NotNil(t, actual)

	run := &entity.CentralActiveRun{SpaceID: 100, ExptID: 1, ExptRunID: 2, Priority: actual.Priority, Seq: actual.Seq}
	require.NoError(t, dao.RegisterPending(ctx, scope, run, time.Hour))
	pending, err := dao.ListPending(ctx, scope)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	active, err := dao.ListActive(ctx, scope)
	require.NoError(t, err)
	assert.Empty(t, active)

	require.NoError(t, dao.ActivateRun(ctx, scope, 1, 2, "u1", time.Hour))
	pending, err = dao.ListPending(ctx, scope)
	require.NoError(t, err)
	assert.Empty(t, pending, "activation must remove the pending initialization")
	active, err = dao.ListActive(ctx, scope)
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, "u1", active[0].SessionUserID)

	// active 维护被刷新时不能重新写回 pending。
	require.NoError(t, dao.RegisterActive(ctx, scope, run, time.Hour))
	pending, err = dao.ListPending(ctx, scope)
	require.NoError(t, err)
	assert.Empty(t, pending)
}

func TestRunStateFreezesModePriorityAndTarget(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()

	first, created, err := dao.EnsureRunState(ctx, scope, &entity.CentralRunState{
		SpaceID: 100, ExptID: 1, ExptRunID: 2, Mode: entity.CentralSchedulerModeEnforce, Priority: 10, TargetConcurrency: 3,
	}, time.Hour)
	require.NoError(t, err)
	require.True(t, created)
	assert.Equal(t, entity.CentralSchedulerModeEnforce, first.Mode)

	second, created, err := dao.EnsureRunState(ctx, scope, &entity.CentralRunState{
		SpaceID: 100, ExptID: 1, ExptRunID: 2, Mode: entity.CentralSchedulerModeLegacy, Priority: 1, TargetConcurrency: 99,
	}, time.Hour)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, entity.CentralSchedulerModeEnforce, second.Mode, "mode must stay frozen")
	assert.Equal(t, 10, second.Priority, "priority must stay frozen")
	assert.Equal(t, 3, second.TargetConcurrency, "target concurrency must stay frozen")

	auth := &entity.CentralModeSwitchAuthorization{ToMode: "legacy", TicketID: "T-1", Operator: "owner", Reason: "approved rollback"}
	switched, err := dao.SwitchRunMode(ctx, scope, 1, 2, entity.CentralSchedulerModeLegacy, auth)
	require.NoError(t, err)
	assert.True(t, switched)
	current, found, err := dao.GetRunState(ctx, scope, 1, 2)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, entity.CentralSchedulerModeLegacy, current.Mode)
	assert.Equal(t, "T-1", current.SwitchTicketID)
	assert.Equal(t, "owner", current.SwitchOperator)
	assert.Equal(t, "approved rollback", current.SwitchReason)

	// 一个实验只允许切换一次。
	switched, err = dao.SwitchRunMode(ctx, scope, 1, 2, entity.CentralSchedulerModeEnforce, auth)
	require.NoError(t, err)
	assert.False(t, switched)
}

func TestInspectLedgerLocatesDriftToLease(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()
	caps := testCapacities()
	lease := newLease(1, 2, 3, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1})
	ok, err := dao.Acquire(ctx, scope, caps, lease, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)

	usageKey := dao.Keys(scope).Usage
	require.NoError(t, dao.cmdable.HSet(ctx, usageKey, string(entity.CentralResourceItemSlot), 99).Err())

	drift, err := dao.InspectLedger(ctx, scope)
	require.NoError(t, err)
	require.NotEmpty(t, drift)
	assert.Equal(t, lease.LeaseID, drift[0].LeaseID)
	assert.Equal(t, "usage_drift", drift[0].Reason)
}

func TestInvalidScopeFailsClosed(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := entity.CentralEnvScope{Env: "", Lane: "lane"}
	_, err := dao.Usage(ctx, scope)
	require.Error(t, err)
	_, err = dao.Acquire(ctx, scope, testCapacities(), newLease(1, 2, 3, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1}), time.Minute)
	require.Error(t, err)
	_, err = dao.ReleaseItems(ctx, scope, 1, 2, []int64{3})
	require.Error(t, err)
	_, _, err = dao.EnsureRunState(ctx, scope, &entity.CentralRunState{SpaceID: 1, ExptID: 2, ExptRunID: 3, Mode: entity.CentralSchedulerModeEnforce}, time.Minute)
	require.Error(t, err)
}

func TestReleaseExptReleasesAllRuns(t *testing.T) {
	ctx := context.Background()
	dao := NewCentralSchedulingDAO(redis.NewTestRedis(t))
	scope := testScope()
	caps := map[entity.CentralResourceKind]int64{entity.CentralResourceItemSlot: 3}
	for _, lease := range []*entity.CentralLease{
		newLease(1, 2, 3, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1}),
		newLease(1, 5, 6, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1}),
		newLease(2, 7, 8, entity.CentralResourceClaim{Kind: entity.CentralResourceItemSlot, Amount: 1}),
	} {
		ok, err := dao.Acquire(ctx, scope, caps, lease, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
	}
	released, err := dao.ReleaseExpt(ctx, scope, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, released)
	leases, err := dao.ListLeases(ctx, scope)
	require.NoError(t, err)
	require.Len(t, leases, 1)
	assert.Equal(t, int64(2), leases[0].ExptID)
}
