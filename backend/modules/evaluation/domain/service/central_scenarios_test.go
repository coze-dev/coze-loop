// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	centraldao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

type dispatchFailPublisher struct {
	events.ExptEventPublisher
	err error
}

func (p *dispatchFailPublisher) BatchPublishExptRecordEvalEvent(context.Context, []*entity.ExptItemEvalEvent, *time.Duration) error {
	return p.err
}

func scenarioDAOs(t *testing.T) (CentralDispatchGate, *centraldao.CentralSchedulingDAO) {
	t.Helper()
	cfg := enforceConfig()
	cfg.Resources[string(entity.CentralResourceItemSlot)] = entity.CentralResourceConfig{Capacity: 5, Enabled: true}
	return newCentralGateForTest(t, cfg)
}

// 场景 1：全局优先级 + 同档 FIFO + 防饿死抬升，且不修改冻结的 base priority。
func TestScenarioPriorityAndAntiStarvation(t *testing.T) {
	ctx := context.Background()
	dao := centraldao.NewCentralSchedulingDAO(redis.NewTestRedis(t))
	seedTickerRun(t, dao, 10, 100, 100, time.Now().Unix()-3600, true) // 低优先级、已等待 1h
	seedTickerRun(t, dao, 20, 200, 5, time.Now().Unix(), false)       // 高优先级、刚提交
	seedTickerRun(t, dao, 30, 300, 50, time.Now().Unix(), true)

	cfg := &entity.CentralSchedulerConfig{Enabled: true, AgingWindowTicks: 15, TickIntervalSecond: 20, AgingStep: 10}
	pub := &recordingPublisher{}
	ticker := NewCentralTicker(dao, pub, &fakeCentralConfigProvider{cfg: cfg}, func(context.Context) (entity.CentralEnvScope, bool) {
		return testScope(), true
	})
	require.NoError(t, ticker.Tick(ctx))
	require.Len(t, pub.scheduleEvents, 3)
	assert.Equal(t, int64(10), pub.scheduleEvents[0].ExptID, "aged low-priority run must be lifted")
	assert.Equal(t, int64(20), pub.scheduleEvents[1].ExptID)
	assert.Equal(t, int64(30), pub.scheduleEvents[2].ExptID, "same priority follows submission seq")

	state, found, err := dao.GetRunState(ctx, testScope(), 10, 100)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, 100, state.Priority, "aging must not rewrite the frozen base priority")
}

// 场景 2：按 target concurrency 与在途的缺口补位，不按固定批量。
func TestScenarioTargetConcurrencyGapFill(t *testing.T) {
	ctx := context.Background()
	cfg := enforceConfig()
	cfg.DefaultTargetConcurrency = 2
	cfg.Resources[string(entity.CentralResourceItemSlot)] = entity.CentralResourceConfig{Capacity: 5, Enabled: true}
	gate, dao := newCentralGateForTest(t, cfg)
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	event := observeEvent()
	items := []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}, {ExptID: 7, ItemID: 2}, {ExptID: 7, ItemID: 3}, {ExptID: 7, ItemID: 4}, {ExptID: 7, ItemID: 5}}

	require.True(t, gate.Observe(ctx, expt, event))
	allowed, leases, err := gate.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	assert.Len(t, allowed, 2, "target concurrency 2 must fill exactly the gap")
	require.NoError(t, gate.ConfirmDispatched(ctx, leases, "user-7"))

	allowed, _, err = gate.ReserveItems(ctx, expt, event, items[2:])
	require.NoError(t, err)
	assert.Empty(t, allowed, "in-flight equals target, no further batch should be forced")

	require.NoError(t, gate.ReleaseItems(ctx, 7, 8, []int64{1, 2}))
	allowed, _, err = gate.ReserveItems(ctx, expt, event, items[2:])
	require.NoError(t, err)
	assert.Len(t, allowed, 2, "released in-flight slots must be refilled by the gap, not by a fixed batch")

	usage, err := dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(2), usage[entity.CentralResourceItemSlot])
}

// 场景 3：模式/优先级冻结，重试继承，未授权不改链，授权只切一次。
func TestScenarioModeFreezeAndRetryInheritance(t *testing.T) {
	ctx := context.Background()
	cfg := enforceConfig()
	gate, dao := newCentralGateForTest(t, cfg)
	expt := makeExpt(100, map[string]string{
		entity.CentralExtKeySchedulingMode: "enforce",
		entity.CentralExtKeyPriority:       "3",
	})
	event := observeEvent()
	require.True(t, gate.Observe(ctx, expt, event))
	state, _, err := dao.GetRunState(ctx, testScope(), 7, 8)
	require.NoError(t, err)
	require.Equal(t, 3, state.Priority)

	// 实验 ext 改成 legacy/priority=1：没有审批授权时仍保持冻结的 enforce/priority=3。
	retryEvent := observeEvent()
	retryEvent.ExptRunMode = entity.EvaluationModeRetryItems
	retryExpt := makeExpt(100, map[string]string{
		entity.CentralExtKeySchedulingMode: "legacy",
		entity.CentralExtKeyPriority:       "1",
	})
	assert.True(t, gate.Observe(ctx, retryExpt, retryEvent), "retry must inherit the frozen enforce mode")
	state, _, err = dao.GetRunState(ctx, testScope(), 7, 8)
	require.NoError(t, err)
	assert.Equal(t, entity.CentralSchedulerModeEnforce, state.Mode)
	assert.Equal(t, 3, state.Priority, "retry must inherit the frozen priority")

	allowed, leases, err := gate.ReserveItems(ctx, retryExpt, retryEvent, []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}})
	require.NoError(t, err)
	require.Len(t, allowed, 1)
	require.Len(t, leases, 1)

	// 审批后的一次性切换生效；第二次不再允许。
	cfg.ModeSwitchAuthorizations = map[string]entity.CentralModeSwitchAuthorization{
		"7": {ToMode: "legacy", TicketID: "T-7", Operator: "owner", Reason: "approved rollback"},
	}
	gate2, dao2 := newCentralGateForTest(t, cfg)
	require.True(t, gate2.Observe(ctx, expt, event))
	assert.False(t, gate2.Observe(ctx, retryExpt, retryEvent))
	state, _, err = dao2.GetRunState(ctx, testScope(), 7, 8)
	require.NoError(t, err)
	assert.Equal(t, entity.CentralSchedulerModeLegacy, state.Mode)
	assert.Equal(t, "T-7", state.SwitchTicketID)
	_ = leases
}

// 场景 4：派发明确失败立即补偿释放；结果不明进入 pending_confirm 并由 sweeper 收敛。
func TestScenarioDispatchFailureCompensatesAndUnknownConfirms(t *testing.T) {
	ctx := context.Background()
	gate, dao := scenarioDAOs(t)
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	event := observeEvent()
	items := []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}, {ExptID: 7, ItemID: 2}}
	require.True(t, gate.Observe(ctx, expt, event))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	configer := componentMocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetExptExecConf(gomock.Any(), int64(100)).Return(&entity.ExptExecConf{}).AnyTimes()

	// 结果不明：不释放、不重派，标记 pending_confirm。
	var err error
	sched := &ExptSchedulerImpl{
		Publisher:   &dispatchFailPublisher{err: errors.New("publish timeout")},
		Configer:    configer,
		centralGate: gate,
	}
	err = sched.handleToSubmits(ctx, event, expt, items)
	require.Error(t, err)
	liveLeases, err := dao.ListLeases(ctx, testScope())
	require.NoError(t, err)
	require.Len(t, liveLeases, 2, "unknown outcome must keep both reservations")
	for _, lease := range liveLeases {
		assert.Equal(t, entity.CentralLeaseStatePendingConfirm, lease.State)
	}
	usage, err := dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(2), usage[entity.CentralResourceItemSlot], "unknown outcome must not release eagerly")

	// deadline 到期后由 leader sweeper 补偿释放。
	released, err := dao.SweepPendingConfirms(ctx, testScope(), time.Now().Add(10*time.Minute).Unix(), time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 2, released)
	usage, err = dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot])

	// 迟到的成功确认不能复活已释放的预占。
	lateLeaseID := liveLeases[0].LeaseID
	ok, err := dao.MarkDispatched(ctx, testScope(), lateLeaseID)
	require.NoError(t, err)
	assert.False(t, ok)

	// 明确失败：立即释放。
	require.True(t, gate.Observe(ctx, expt, event))
	sched.Publisher = &dispatchFailPublisher{err: errors.New("publish rejected")}
	err = sched.handleToSubmits(ctx, event, expt, []*entity.ExptEvalItem{{ExptID: 7, ItemID: 3}})
	require.Error(t, err)
	usage, err = dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot], "explicit failure must release immediately")
}

// 场景 5：重试/删除/取消/终态全部释放，且重试产生新的 attempt。
func TestScenarioReleaseOnRetryDeleteAndCancel(t *testing.T) {
	ctx := context.Background()
	gate, dao := scenarioDAOs(t)
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	event := observeEvent()
	item := []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}}
	require.True(t, gate.Observe(ctx, expt, event))

	_, first, err := gate.ReserveItems(ctx, expt, event, item)
	require.NoError(t, err)
	require.Len(t, first, 1)
	firstAttempt := first[0].Attempt

	// 未确认的旧 attempt 在再次预占前释放，新 attempt 唯一。
	_, second, err := gate.ReserveItems(ctx, expt, event, item)
	require.NoError(t, err)
	require.Len(t, second, 1)
	assert.Greater(t, second[0].Attempt, firstAttempt)
	usage, err := dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(1), usage[entity.CentralResourceItemSlot], "old attempt must not double count")

	// 取消：注销 run 释放全部在途 lease。
	require.NoError(t, gate.UnregisterRun(ctx, 7, 8))
	usage, err = dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot])

	// 删除：按实验释放全部 run 的 lease。
	require.True(t, gate.Observe(ctx, expt, event))
	_, leases, err := gate.ReserveItems(ctx, expt, event, item)
	require.NoError(t, err)
	require.Len(t, leases, 1)
	require.NoError(t, gate.ReleaseExpt(ctx, 7))
	usage, err = dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot])

	// 重试：释放旧 item 预占后再以新 attempt 重新预占。
	require.True(t, gate.Observe(ctx, expt, event))
	_, retryLease, err := gate.ReserveItems(ctx, expt, event, item)
	require.NoError(t, err)
	require.Len(t, retryLease, 1)
	require.NoError(t, gate.ReleaseItems(ctx, 7, 8, []int64{1}))
	_, retried, err := gate.ReserveItems(ctx, expt, event, item)
	require.NoError(t, err)
	require.Len(t, retried, 1)
	assert.Greater(t, retried[0].Attempt, retryLease[0].Attempt)
	usage, err = dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(1), usage[entity.CentralResourceItemSlot])
}

// 场景 6：重复回调/重复释放幂等，不重复扣也不重复放。
func TestScenarioDuplicateCallbacksAreIdempotent(t *testing.T) {
	ctx := context.Background()
	gate, dao := scenarioDAOs(t)
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	event := observeEvent()
	require.True(t, gate.Observe(ctx, expt, event))
	_, leases, err := gate.ReserveItems(ctx, expt, event, []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}})
	require.NoError(t, err)
	require.Len(t, leases, 1)

	require.NoError(t, gate.ConfirmDispatched(ctx, leases, "user-7"))
	require.NoError(t, gate.ConfirmDispatched(ctx, leases, "user-7"))
	state, ok, err := dao.GetLeaseState(ctx, testScope(), leases[0].LeaseID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, entity.CentralLeaseStateDispatched, state)

	require.NoError(t, gate.ReleaseItems(ctx, 7, 8, []int64{1}))
	require.NoError(t, gate.ReleaseItems(ctx, 7, 8, []int64{1}))
	usage, err := dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot])

	// 终态以后迟到的确认/释放都不得重复扣减。
	ok, err = dao.MarkDispatched(ctx, testScope(), leases[0].LeaseID)
	require.NoError(t, err)
	assert.False(t, ok)
	usage, err = dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot])
}

// 场景 7：增量对账逐笔定位 + 全量重建原子修复，且不改变在途预占。
func TestScenarioIncrementalAndFullRebuild(t *testing.T) {
	ctx := context.Background()
	gate, dao := scenarioDAOs(t)
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	event := observeEvent()
	require.True(t, gate.Observe(ctx, expt, event))
	_, leases, err := gate.ReserveItems(ctx, expt, event, []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}, {ExptID: 7, ItemID: 2}})
	require.NoError(t, err)
	require.Len(t, leases, 2)
	require.NoError(t, gate.ConfirmDispatched(ctx, leases, "user-7"))
	inFlightBefore, err := dao.CountRunLeases(ctx, testScope(), 7, 8)
	require.NoError(t, err)

	// 人为制造漂移：usage 被抬高。
	usageKey := dao.Keys(testScope()).Usage
	require.NoError(t, dao.CmdableForTest().HSet(ctx, usageKey, string(entity.CentralResourceItemSlot), 99).Err())
	drift, err := dao.InspectLedger(ctx, testScope())
	require.NoError(t, err)
	require.NotEmpty(t, drift)
	assert.Equal(t, "usage_drift", drift[0].Reason)
	assert.NotEmpty(t, drift[0].LeaseID, "incremental reconcile must locate a specific lease")

	caps := map[entity.CentralResourceKind]int64{
		entity.CentralResourceItemSlot:       5,
		entity.CentralResourceEvaluatorSlot:  4,
		entity.CentralResourceSandboxExecute: 2,
	}
	before, after, repaired, rebuildDrift, err := dao.RebuildUsage(ctx, testScope(), caps, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(99), before[entity.CentralResourceItemSlot])
	assert.Equal(t, int64(2), after[entity.CentralResourceItemSlot])
	assert.Equal(t, 2, repaired)
	require.NotEmpty(t, rebuildDrift)
	inFlightAfter, err := dao.CountRunLeases(ctx, testScope(), 7, 8)
	require.NoError(t, err)
	assert.Equal(t, inFlightBefore, inFlightAfter, "rebuild must not change in-flight reservations")
	for _, lease := range leases {
		state, ok, stateErr := dao.GetLeaseState(ctx, testScope(), lease.LeaseID)
		require.NoError(t, stateErr)
		require.True(t, ok)
		assert.Equal(t, entity.CentralLeaseStateDispatched, state, "rebuild must not reset lease state")
	}
	usage, err := dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(2), usage[entity.CentralResourceItemSlot])
}

// 场景 8：legacy 每空间 200 准入对已存在实验幂等，不双计也不漏计。
func TestScenarioLegacyAdmissionNoDoubleOrMissedCount(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	quotaRepo := repoMocks.NewMockQuotaRepo(ctrl)
	configer := componentMocks.NewMockIConfiger(ctrl)
	svc := NewQuotaService(quotaRepo, configer)
	session := &entity.Session{UserID: "u"}

	now := time.Now().Unix()
	cur := &entity.QuotaSpaceExpt{ExptID2RunTime: make(map[int64]int64, 200)}
	for i := int64(1); i <= 200; i++ {
		cur.ExptID2RunTime[i] = now
	}
	configer.EXPECT().GetExptExecConf(gomock.Any(), int64(100)).Return(&entity.ExptExecConf{ZombieIntervalSecond: 3600, SpaceExptConcurLimit: 200}).AnyTimes()
	quotaRepo.EXPECT().CreateOrUpdate(gomock.Any(), int64(100), gomock.Any(), session).DoAndReturn(
		func(_ context.Context, _ int64, updater func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
			newCur, _, err := updater(cur)
			if err != nil {
				return err
			}
			cur = newCur
			return nil
		}).Times(2)

	require.NoError(t, svc.AllowExptRun(ctx, 1, 100, session), "existing experiment must be idempotent at the 200 limit")
	assert.Len(t, cur.ExptID2RunTime, 200, "retry must not increase the count")
	require.Error(t, svc.AllowExptRun(ctx, 201, 100, session), "a new experiment beyond 200 must be rejected")
	assert.Len(t, cur.ExptID2RunTime, 200, "rejected admission must not change the count")
}

// 场景 9：环境/泳道读不到即失败，所有 key 只在 qn:central: 下的本环境命名空间。
func TestScenarioEnvIsolationFailsClosed(t *testing.T) {
	ctx := context.Background()
	dao := centraldao.NewCentralSchedulingDAO(redis.NewTestRedis(t))
	invalid := entity.CentralEnvScope{Env: "", Lane: "lane"}
	_, err := dao.Usage(ctx, invalid)
	require.Error(t, err)
	_, err = dao.Acquire(ctx, invalid, map[entity.CentralResourceKind]int64{entity.CentralResourceItemSlot: 1}, &entity.CentralLease{
		LeaseID: "x", ExptID: 1, ExptRunID: 2, ItemID: 3,
		Claims: []entity.CentralResourceClaim{{Kind: entity.CentralResourceItemSlot, Amount: 1}},
	}, time.Minute)
	require.Error(t, err)

	gate := NewCentralDispatchGate(dao, &fakeCentralConfigProvider{cfg: enforceConfig()}, func(context.Context) (entity.CentralEnvScope, bool) {
		return entity.CentralEnvScope{}, false
	})
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	assert.False(t, gate.Enabled(ctx, expt, 8))
	assert.False(t, gate.Observe(ctx, expt, observeEvent()), "unresolvable env must not register anything")
	_, found, err := dao.GetRunState(ctx, testScope(), 7, 8)
	require.NoError(t, err)
	assert.False(t, found, "no state may be written when env is unresolvable")

	// 合法 scope 下写入的 key 必须全部带 qn:central: 前缀且含本环境片段。
	validGate, validDAO := newCentralGateForTest(t, enforceConfig())
	require.True(t, validGate.Observe(ctx, expt, observeEvent()))
	raw, ok := redis.Unwrap(validDAO.CmdableForTest())
	require.True(t, ok)
	keys, err := raw.Keys(ctx, "*").Result()
	require.NoError(t, err)
	require.NotEmpty(t, keys)
	for _, key := range keys {
		assert.Contains(t, key, "qn:central:")
		assert.Contains(t, key, "test_psm")
		assert.NotContains(t, key, "unknown")
	}
}

// 场景 10：legacy 实验零中心状态、零额外预占，旧链路行为不变。
func TestScenarioLegacyChainStaysUnchanged(t *testing.T) {
	ctx := context.Background()
	gate, dao := scenarioDAOs(t)
	expt := makeExpt(100, nil) // 无 central_scheduling_mode
	event := observeEvent()
	items := []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}, {ExptID: 7, ItemID: 2}}

	assert.False(t, gate.Observe(ctx, expt, event))
	_, found, err := dao.GetRunState(ctx, testScope(), 7, 8)
	require.NoError(t, err)
	assert.False(t, found, "legacy experiment must not write frozen central state")
	allowed, leases, err := gate.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	assert.Len(t, allowed, 2, "legacy mode must pass every item through")
	assert.Empty(t, leases)
	usage, err := dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Empty(t, usage, "legacy mode must not consume central ledger")
}
