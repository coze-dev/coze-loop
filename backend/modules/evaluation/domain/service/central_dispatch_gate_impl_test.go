// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	centraldao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

type fakeCentralConfigProvider struct {
	cfg *entity.CentralSchedulerConfig
}

func (f *fakeCentralConfigProvider) GetCentralSchedulerConfig(context.Context) *entity.CentralSchedulerConfig {
	return f.cfg
}

func testScope() entity.CentralEnvScope {
	return entity.CentralEnvScope{Env: "test_psm", Lane: "ppe_unit"}
}

func newCentralGateForTest(t *testing.T, cfg *entity.CentralSchedulerConfig) (CentralDispatchGate, *centraldao.CentralSchedulingDAO) {
	t.Helper()
	dao := centraldao.NewCentralSchedulingDAO(redis.NewTestRedis(t))
	provider := &fakeCentralConfigProvider{cfg: cfg}
	gate := NewCentralDispatchGate(dao, provider, func(context.Context) (entity.CentralEnvScope, bool) {
		return testScope(), true
	})
	return gate, dao
}

func enforceConfig() *entity.CentralSchedulerConfig {
	return &entity.CentralSchedulerConfig{
		Enabled: true,
		Resources: map[string]entity.CentralResourceConfig{
			string(entity.CentralResourceItemSlot):       {Capacity: 2, Enabled: true},
			string(entity.CentralResourceEvaluatorSlot):  {Capacity: 4, Enabled: true},
			string(entity.CentralResourceSandboxExecute): {Capacity: 2, Enabled: true},
		},
	}
}

func makeExpt(spaceID int64, ext map[string]string) *entity.Experiment {
	return &entity.Experiment{ID: 7, SpaceID: spaceID, EvalConf: &entity.EvaluationConfiguration{Ext: ext}}
}

func observeEvent() *entity.ExptScheduleEvent {
	return &entity.ExptScheduleEvent{SpaceID: 100, ExptID: 7, ExptRunID: 8, Session: &entity.Session{UserID: "user-7"}}
}

func TestCentralGateLegacyIsNoOp(t *testing.T) {
	ctx := context.Background()
	gate, _ := newCentralGateForTest(t, &entity.CentralSchedulerConfig{Enabled: false})
	expt := makeExpt(100, nil)
	event := observeEvent()
	items := []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}, {ExptID: 7, ItemID: 2}}

	assert.False(t, gate.Enabled(ctx, expt, event.ExptRunID), "disabled config must keep legacy mode")
	allowed, leases, err := gate.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	assert.Len(t, allowed, 2, "legacy mode must pass every item through")
	assert.Empty(t, leases)
	assert.False(t, gate.Observe(ctx, expt, event))
}

func TestCentralGateModeResolution(t *testing.T) {
	ctx := context.Background()
	gate, _ := newCentralGateForTest(t, enforceConfig())

	assert.False(t, gate.Enabled(ctx, makeExpt(100, nil), 8), "default mode is legacy")
	assert.True(t, gate.Enabled(ctx, makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"}), 8))
	assert.False(t, gate.Enabled(ctx, makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "bogus"}), 8))

	cfg := enforceConfig()
	cfg.EnforceSpaceIDs = []int64{200}
	gate2, _ := newCentralGateForTest(t, cfg)
	assert.True(t, gate2.Enabled(ctx, makeExpt(200, nil), 8), "space whitelist must enforce")
	assert.False(t, gate2.Enabled(ctx, makeExpt(100, nil), 8))
}

func TestCentralGateReserveItemsAllOrNoneAndRelease(t *testing.T) {
	ctx := context.Background()
	gate, dao := newCentralGateForTest(t, enforceConfig())
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	event := observeEvent()
	items := []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}, {ExptID: 7, ItemID: 2}, {ExptID: 7, ItemID: 3}}

	require.True(t, gate.Observe(ctx, expt, event))
	allowed, leases, err := gate.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	assert.Len(t, allowed, 2, "item_slot capacity is 2")
	assert.Len(t, leases, 2)

	usage, err := dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(2), usage[entity.CentralResourceItemSlot])

	require.NoError(t, gate.ConfirmDispatched(ctx, leases, "user-7"))
	for _, lease := range leases {
		state, ok, err := dao.GetLeaseState(ctx, testScope(), lease.LeaseID)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, entity.CentralLeaseStateDispatched, state)
	}

	require.NoError(t, gate.ReleaseItems(ctx, 7, 8, []int64{1, 2}))
	usage, err = dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot])

	// 幂等：重复释放不报错、不产生负占用。
	require.NoError(t, gate.ReleaseItems(ctx, 7, 8, []int64{1, 2}))
	usage, _ = dao.Usage(ctx, testScope())
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot])
}

func TestCentralGateObserveRegistersPendingRun(t *testing.T) {
	ctx := context.Background()
	gate, dao := newCentralGateForTest(t, enforceConfig())
	expt := makeExpt(100, map[string]string{
		entity.CentralExtKeySchedulingMode: "enforce",
		entity.CentralExtKeyPriority:       "3",
	})
	event := observeEvent()

	assert.True(t, gate.Observe(ctx, expt, event))
	runs, err := dao.ListPending(ctx, testScope())
	require.NoError(t, err)
	require.Len(t, runs, 1, "first observation must register pending initialization, not active maintenance")
	assert.Equal(t, "user-7", runs[0].SessionUserID)
	assert.Equal(t, 3, runs[0].Priority)
	_ = time.Now()
}

func TestCentralGateModeIsFrozenUntilApprovedSwitch(t *testing.T) {
	ctx := context.Background()
	gate, dao := newCentralGateForTest(t, enforceConfig())
	exptEnforce := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	exptLegacy := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "legacy"})
	event := observeEvent()

	require.True(t, gate.Observe(ctx, exptEnforce, event))
	// 修改 ext 不足以切链：没有审批授权时保持冻结的 enforce。
	assert.True(t, gate.Observe(ctx, exptLegacy, event), "mode must stay frozen to enforce without authorization")

	cfg := enforceConfig()
	cfg.ModeSwitchAuthorizations = map[string]entity.CentralModeSwitchAuthorization{
		"7": {ToMode: "legacy", TicketID: "T-7", Operator: "owner", Reason: "approved rollback"},
	}
	gate2, dao2 := newCentralGateForTest(t, cfg)
	require.True(t, gate2.Observe(ctx, exptEnforce, event))
	assert.False(t, gate2.Observe(ctx, exptLegacy, event), "approved one-time switch must take effect")
	state, found, err := dao2.GetRunState(ctx, testScope(), 7, 8)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, entity.CentralSchedulerModeLegacy, state.Mode)
	assert.Equal(t, "T-7", state.SwitchTicketID)
	_ = dao
}

func TestCentralGateTargetConcurrencyGap(t *testing.T) {
	ctx := context.Background()
	cfg := enforceConfig()
	cfg.DefaultTargetConcurrency = 1
	gate, _ := newCentralGateForTest(t, cfg)
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	event := observeEvent()
	items := []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}, {ExptID: 7, ItemID: 2}, {ExptID: 7, ItemID: 3}}

	require.True(t, gate.Observe(ctx, expt, event))
	allowed, leases, err := gate.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	assert.Len(t, allowed, 1, "must fill only the gap to target concurrency, not a fixed batch")
	assert.Len(t, leases, 1)

	// 已有在途 item 时缺口为 0，不再派发。
	allowed, _, err = gate.ReserveItems(ctx, expt, event, items[1:])
	require.NoError(t, err)
	assert.Empty(t, allowed)
}

func TestCentralGateDisabledResourceBlocksDispatch(t *testing.T) {
	ctx := context.Background()
	cfg := enforceConfig()
	itemCfg := cfg.Resources[string(entity.CentralResourceItemSlot)]
	itemCfg.Enabled = false
	cfg.Resources[string(entity.CentralResourceItemSlot)] = itemCfg
	gate, _ := newCentralGateForTest(t, cfg)
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	event := observeEvent()
	items := []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}}

	require.True(t, gate.Observe(ctx, expt, event))
	allowed, leases, err := gate.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	assert.Empty(t, allowed, "disabled resource must block related experiments")
	assert.Empty(t, leases)
}

func TestCentralGateDegradedPolicyNeverSilentlyIncreasesCapacity(t *testing.T) {
	ctx := context.Background()
	cfg := enforceConfig()
	cfg.Degraded = true
	cfg.DegradedPolicy = string(entity.CentralDegradedPolicyReduceCapacity)
	cfg.DegradedRatio = 0.5
	gate, _ := newCentralGateForTest(t, cfg)
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	event := observeEvent()
	items := []*entity.ExptEvalItem{
		{ExptID: 7, ItemID: 1}, {ExptID: 7, ItemID: 2}, {ExptID: 7, ItemID: 3},
	}

	require.True(t, gate.Observe(ctx, expt, event))
	allowed, _, err := gate.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	assert.Len(t, allowed, 1, "degraded capacity is half of configured item_slot capacity (2 -> 1)")

	cfg.DegradedPolicy = string(entity.CentralDegradedPolicyBlockNew)
	gate2, _ := newCentralGateForTest(t, cfg)
	require.True(t, gate2.Observe(ctx, expt, event))
	allowed, _, err = gate2.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	assert.Empty(t, allowed, "block_new must suspend all new reservations")
}

func TestCentralGateReleaseExpt(t *testing.T) {
	ctx := context.Background()
	gate, dao := newCentralGateForTest(t, enforceConfig())
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	event := observeEvent()
	items := []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}, {ExptID: 7, ItemID: 2}}
	require.True(t, gate.Observe(ctx, expt, event))
	_, leases, err := gate.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	require.Len(t, leases, 2)

	require.NoError(t, gate.ReleaseExpt(ctx, 7))
	usage, err := dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot])
}

func TestBuildCentralClaims(t *testing.T) {
	expt := &entity.Experiment{
		Evaluators: []*entity.Evaluator{{}, {}},
		TargetType: entity.EvalTargetTypeSandboxAgent,
	}
	claims := buildCentralClaims(expt)
	assert.Equal(t, int64(1), claims[0].Amount)
	assert.Equal(t, entity.CentralResourceItemSlot, claims[0].Kind)
	assert.Equal(t, entity.CentralResourceEvaluatorSlot, claims[1].Kind)
	assert.Equal(t, int64(2), claims[1].Amount)
	assert.Equal(t, len(claims), 3)
	assert.Equal(t, entity.CentralResourceSandboxExecute, claims[2].Kind)
}
