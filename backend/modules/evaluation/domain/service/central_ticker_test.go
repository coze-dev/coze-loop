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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	centraldao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

type recordingPublisher struct {
	events.ExptEventPublisher
	scheduleEvents []*entity.ExptScheduleEvent
}

func (p *recordingPublisher) PublishExptScheduleEvent(_ context.Context, event *entity.ExptScheduleEvent, _ *time.Duration) error {
	p.scheduleEvents = append(p.scheduleEvents, event)
	return nil
}

func (p *recordingPublisher) PublishExptRecordEvalEvent(context.Context, *entity.ExptItemEvalEvent, *time.Duration, func(*entity.ExptItemEvalEvent)) error {
	return nil
}
func (p *recordingPublisher) BatchPublishExptRecordEvalEvent(context.Context, []*entity.ExptItemEvalEvent, *time.Duration) error {
	return nil
}
func (p *recordingPublisher) PublishExptAggrCalculateEvent(context.Context, []*entity.AggrCalculateEvent, *time.Duration) error {
	return nil
}
func (p *recordingPublisher) PublishExptOnlineEvalResult(context.Context, *entity.OnlineExptEvalResultEvent, *time.Duration) error {
	return nil
}
func (p *recordingPublisher) PublishExptTurnResultFilterEvent(context.Context, *entity.ExptTurnResultFilterEvent, *time.Duration) error {
	return nil
}
func (p *recordingPublisher) PublishExptExportCSVEvent(context.Context, *entity.ExportCSVEvent, *time.Duration) error {
	return nil
}
func (p *recordingPublisher) PublishExptLifecycleEvent(context.Context, *entity.ExptLifecycleEvent, *time.Duration, string) error {
	return nil
}
func (p *recordingPublisher) PublishExptWebhookNotifyEvent(context.Context, *entity.WebhookRetryEvent, *time.Duration) error {
	return nil
}

func seedTickerRun(t *testing.T, dao *centraldao.CentralSchedulingDAO, exptID, runID int64, priority int, registeredAt int64, active bool) {
	t.Helper()
	ctx := context.Background()
	state := &entity.CentralRunState{
		SpaceID:      1,
		ExptID:       exptID,
		ExptRunID:    runID,
		Mode:         entity.CentralSchedulerModeEnforce,
		Priority:     priority,
		RegisteredAt: registeredAt,
	}
	actual, _, err := dao.EnsureRunState(ctx, testScope(), state, time.Hour)
	require.NoError(t, err)
	run := &entity.CentralActiveRun{SpaceID: 1, ExptID: exptID, ExptRunID: runID, Priority: actual.Priority, Seq: actual.Seq}
	if active {
		require.NoError(t, dao.RegisterActive(ctx, testScope(), run, time.Hour))
	} else {
		require.NoError(t, dao.RegisterPending(ctx, testScope(), run, time.Hour))
	}
}

func TestCentralTickerPublishesOneShotInPriorityThenFIFOOrder(t *testing.T) {
	ctx := context.Background()
	dao := centraldao.NewCentralSchedulingDAO(redis.NewTestRedis(t))
	seedTickerRun(t, dao, 10, 100, 50, 0, false)
	seedTickerRun(t, dao, 20, 200, 5, 0, true)
	seedTickerRun(t, dao, 30, 300, 50, 0, true) // 同档：后提交，应排在 expt 10 之后

	provider := &fakeCentralConfigProvider{cfg: &entity.CentralSchedulerConfig{Enabled: true}}
	pub := &recordingPublisher{}
	ticker := NewCentralTicker(dao, pub, provider, func(context.Context) (entity.CentralEnvScope, bool) {
		return testScope(), true
	})

	require.NoError(t, ticker.Tick(ctx))
	require.Len(t, pub.scheduleEvents, 3)
	assert.Equal(t, int64(20), pub.scheduleEvents[0].ExptID, "priority 5 must be published first")
	assert.Equal(t, int64(10), pub.scheduleEvents[1].ExptID, "same priority must follow submission order")
	assert.Equal(t, int64(30), pub.scheduleEvents[2].ExptID)
	for _, event := range pub.scheduleEvents {
		assert.True(t, event.OneShot, "central tick events must be one-shot")
	}
}

func TestCentralTickerAgingPreventsStarvation(t *testing.T) {
	ctx := context.Background()
	dao := centraldao.NewCentralSchedulingDAO(redis.NewTestRedis(t))
	// 低优先级实验已等待 1 小时；高优先级实验刚刚提交。
	seedTickerRun(t, dao, 10, 100, 100, time.Now().Unix()-3600, true)
	seedTickerRun(t, dao, 20, 200, 5, time.Now().Unix(), false)

	cfg := &entity.CentralSchedulerConfig{Enabled: true, AgingWindowTicks: 15, TickIntervalSecond: 20, AgingStep: 10}
	pub := &recordingPublisher{}
	ticker := NewCentralTicker(dao, pub, &fakeCentralConfigProvider{cfg: cfg}, func(context.Context) (entity.CentralEnvScope, bool) {
		return testScope(), true
	})
	require.NoError(t, ticker.Tick(ctx))
	require.Len(t, pub.scheduleEvents, 2)
	assert.Equal(t, int64(10), pub.scheduleEvents[0].ExptID, "aged low-priority run must be lifted and not starve")
}

func TestCentralTickerDisabledDoesNothing(t *testing.T) {
	ctx := context.Background()
	dao := centraldao.NewCentralSchedulingDAO(redis.NewTestRedis(t))
	pub := &recordingPublisher{}
	ticker := NewCentralTicker(dao, pub, &fakeCentralConfigProvider{cfg: &entity.CentralSchedulerConfig{Enabled: false}}, nil)

	require.NoError(t, ticker.Tick(ctx))
	assert.Empty(t, pub.scheduleEvents)
}

func TestCentralTickerFailsClosedOnInvalidScope(t *testing.T) {
	ctx := context.Background()
	dao := centraldao.NewCentralSchedulingDAO(redis.NewTestRedis(t))
	seedTickerRun(t, dao, 10, 100, 10, 0, false)
	pub := &recordingPublisher{}
	ticker := NewCentralTicker(dao, pub, &fakeCentralConfigProvider{cfg: &entity.CentralSchedulerConfig{Enabled: true}},
		func(context.Context) (entity.CentralEnvScope, bool) { return entity.CentralEnvScope{}, false })

	require.NoError(t, ticker.Tick(ctx))
	assert.Empty(t, pub.scheduleEvents, "unresolvable env scope must not publish anything")
}

func TestCentralTickerSweepsExpiredPendingConfirm(t *testing.T) {
	ctx := context.Background()
	dao := centraldao.NewCentralSchedulingDAO(redis.NewTestRedis(t))
	caps := map[entity.CentralResourceKind]int64{entity.CentralResourceItemSlot: 1}
	lease := &entity.CentralLease{
		LeaseID: entity.MakeCentralLeaseIDWithAttempt(1, 2, 3, 1),
		SpaceID: 1, ExptID: 1, ExptRunID: 2, ItemID: 3, Attempt: 1,
		Claims: []entity.CentralResourceClaim{{Kind: entity.CentralResourceItemSlot, Amount: 1}},
	}
	ok, err := dao.Acquire(ctx, testScope(), caps, lease, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	_, err = dao.MarkPendingConfirm(ctx, testScope(), lease.LeaseID, time.Now().Unix()-1)
	require.NoError(t, err)

	pub := &recordingPublisher{}
	ticker := NewCentralTicker(dao, pub, &fakeCentralConfigProvider{cfg: &entity.CentralSchedulerConfig{Enabled: true}},
		func(context.Context) (entity.CentralEnvScope, bool) { return testScope(), true })
	require.NoError(t, ticker.Tick(ctx))

	usage, err := dao.Usage(ctx, testScope())
	require.NoError(t, err)
	assert.Equal(t, int64(0), usage[entity.CentralResourceItemSlot], "expired pending_confirm must be released by the tick")
}

func TestCentralTickerRecoversDegradedAfterHealthyTicks(t *testing.T) {
	ctx := context.Background()
	dao := centraldao.NewCentralSchedulingDAO(redis.NewTestRedis(t))
	cfg := enforceConfig()
	cfg.Degraded = true
	cfg.DegradedPolicy = string(entity.CentralDegradedPolicyBlockNew)
	cfg.DegradedRecoverTicks = 2

	gate := NewCentralDispatchGate(dao, &fakeCentralConfigProvider{cfg: cfg}, func(context.Context) (entity.CentralEnvScope, bool) {
		return testScope(), true
	})
	expt := makeExpt(100, map[string]string{entity.CentralExtKeySchedulingMode: "enforce"})
	event := observeEvent()
	items := []*entity.ExptEvalItem{{ExptID: 7, ItemID: 1}, {ExptID: 7, ItemID: 2}}
	require.True(t, gate.Observe(ctx, expt, event))
	allowed, _, err := gate.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	assert.Empty(t, allowed, "block_new must suspend dispatch before recovery")

	pub := &recordingPublisher{}
	ticker := NewCentralTicker(dao, pub, &fakeCentralConfigProvider{cfg: cfg}, func(context.Context) (entity.CentralEnvScope, bool) {
		return testScope(), true
	})
	ticker.ReconcileEveryTicks = 1
	require.NoError(t, ticker.Tick(ctx))
	require.NoError(t, ticker.Tick(ctx))

	recovered, err := dao.IsDegradedRecovered(ctx, testScope())
	require.NoError(t, err)
	assert.True(t, recovered, "continuous healthy ticks must eventually clear degraded state")

	allowed, _, err = gate.ReserveItems(ctx, expt, event, items)
	require.NoError(t, err)
	assert.NotEmpty(t, allowed, "after recovery capacity returns to the configured value, not above it")
}
