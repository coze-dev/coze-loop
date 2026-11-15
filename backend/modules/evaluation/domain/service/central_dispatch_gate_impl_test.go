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

func newCentralGateForTest(t *testing.T, cfg *entity.CentralSchedulerConfig) (CentralDispatchGate, *centraldao.CentralSchedulingDAO) {
	t.Helper()
	dao := centraldao.NewCentralSchedulingDAO(redis.NewTestRedis(t))
	provider := &fakeCentralConfigProvider{cfg: cfg}
	gate := NewCentralDispatchGate(dao, provider, func(context.Context) entity.CentralEnvScope {
		return entity.CentralEnvScope{Env: "test_psm", Lane: "ppe_unit"}
	})
	return gate, dao
}

func enforceConfig() *entity.CentralSchedulerConfig {
	return &entity.CentralSchedulerConfig{
		Enabled: true,
		Capacities: map[string]int64{
			string(entity.CentralResourceItemSlot):       2,
			string(entity.CentralResourceEvaluatorSlot):  4,
			string(entity.CentralResourceSandboxExecute): 2,
		},
	}
}

func makeExpt(spaceID int64, ext map[string]string) *entity.Experiment {
	return &entity.Experiment{ID: 7, SpaceID: spaceID, EvalConf: &entity.EvaluationConfiguration{Ext: ext}}
}

func TestCentralGateLegacyIsNoOp(t *testing.T) {
	ctx := context.Background()
	gate, _ := newCentralGateForTest(t, &entity.CentralSchedulerConfig{Enabled: false})
	expt := makeExpt(100, nil)
	event := &entity.ExptScheduleEvent{SpaceID: 100, ExptID: 7, ExptRunID: 8}
	items := []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}, {ExptID: 7, ItemID: 2}}

	assert.False(t, gate.Enabled(ctx, expt), "disabled config must keep legacy mode")
	allowed, leases, err := gate.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	assert.Len(t, allowed, 2, "legacy mode must pass every item through")
	assert.Empty(t, leases)
	assert.False(t, gate.Observe(ctx, expt, event))
}

func TestCentralGateModeResolution(t *testing.T) {
	ctx := context.Background()
	gate, _ := newCentralGateForTest(t, enforceConfig())

	assert.False(t, gate.Enabled(ctx, makeExpt(100, nil)), "default mode is legacy")
	assert.True(t, gate.Enabled(ctx, makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})))
	assert.False(t, gate.Enabled(ctx, makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "bogus"})))

	cfg := enforceConfig()
	cfg.EnforceSpaceIDs = []int64{200}
	gate2, _ := newCentralGateForTest(t, cfg)
	assert.True(t, gate2.Enabled(ctx, makeExpt(200, nil)), "space whitelist must enforce")
	assert.False(t, gate2.Enabled(ctx, makeExpt(100, nil)))
}

func TestCentralGateReserveItemsAllOrNoneAndRelease(t *testing.T) {
	ctx := context.Background()
	gate, dao := newCentralGateForTest(t, enforceConfig())
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	event := &entity.ExptScheduleEvent{SpaceID: 100, ExptID: 7, ExptRunID: 8}
	items := []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}, {ExptID: 7, ItemID: 2}, {ExptID: 7, ItemID: 3}}

	allowed, leases, err := gate.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	assert.Len(t, allowed, 2, "item_slot capacity is 2")
	assert.Len(t, leases, 2)

	usage, err := dao.Usage(ctx, entity.CentralEnvScope{Env: "test_psm", Lane: "ppe_unit"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), usage[entity.CentralResourceItemSlot])

	require.NoError(t, gate.ReleaseItems(ctx, 7, 8, []int64{1, 2}))
	usage, err = dao.Usage(ctx, entity.CentralEnvScope{Env: "test_psm", Lane: "ppe_unit"})
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot])

	// 幂等：重复释放不报错、不产生负占用。
	require.NoError(t, gate.ReleaseItems(ctx, 7, 8, []int64{1, 2}))
	usage, _ = dao.Usage(ctx, entity.CentralEnvScope{Env: "test_psm", Lane: "ppe_unit"})
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot])
}

func TestCentralGateObserveRegistersActiveRun(t *testing.T) {
	ctx := context.Background()
	gate, dao := newCentralGateForTest(t, enforceConfig())
	expt := makeExpt(100, map[string]string{
		entity.CentralExtKeySchedulingMode: "enforce",
		entity.CentralExtKeyPriority:       "3",
	})
	event := &entity.ExptScheduleEvent{SpaceID: 100, ExptID: 7, ExptRunID: 8, Session: &entity.Session{UserID: "user-7"}}

	assert.True(t, gate.Observe(ctx, expt, event))
	runs, err := dao.ListActive(ctx, entity.CentralEnvScope{Env: "test_psm", Lane: "ppe_unit"})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, "user-7", runs[0].SessionUserID)
	assert.Equal(t, 3, runs[0].Priority)
	_ = time.Now()
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
