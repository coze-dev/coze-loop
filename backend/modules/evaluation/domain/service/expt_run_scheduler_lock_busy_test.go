// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// The API owner, Redis backoff/renewal, modes and middleware remain production
// code. These probes observe calls without replacing their lock/cleanup effects.
type schedulerBusyLocker struct {
	lock.ILocker
	cancels, unlocks int
}

func (l *schedulerBusyLocker) LockBackoffWithRenew(ctx context.Context, key string, ttl, maxHold time.Duration) (bool, context.Context, func(), error) {
	locked, owner, cancel, err := l.ILocker.LockBackoffWithRenew(ctx, key, ttl, maxHold)
	return locked, owner, func() { l.cancels++; cancel() }, err
}

func (l *schedulerBusyLocker) Unlock(key string) (bool, error) {
	l.unlocks++
	return l.ILocker.Unlock(key)
}

type schedulerBusyManager struct {
	*ExptMangerImpl
	completions, details int
	detailErr            error
	detailCtx            context.Context
}

func (m *schedulerBusyManager) CompleteExpt(ctx context.Context, expt int64, run *int64, space int64, session *entity.Session, opts ...entity.CompleteExptOptionFn) error {
	m.completions++
	return m.ExptMangerImpl.CompleteExpt(ctx, expt, run, space, session, opts...)
}

func (m *schedulerBusyManager) GetDetail(ctx context.Context, expt, space int64, session *entity.Session, opts ...entity.GetExptTupleOptionFn) (*entity.Experiment, error) {
	m.details++
	m.detailCtx = ctx
	if m.detailErr != nil {
		return nil, m.detailErr
	}
	return m.ExptMangerImpl.GetDetail(ctx, expt, space, session, opts...)
}

type schedulerBusyPublisher struct {
	events.ExptEventPublisher
	ctx          context.Context
	payloads     [][]byte
	delays       []time.Duration
	err          error
	beforeReturn func()
}

func (p *schedulerBusyPublisher) PublishExptScheduleEvent(ctx context.Context, event *entity.ExptScheduleEvent, delay *time.Duration) error {
	p.ctx = ctx
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	p.payloads = append(p.payloads, payload)
	p.delays = append(p.delays, *delay)
	if p.beforeReturn != nil {
		p.beforeReturn()
	}
	return p.err
}

type schedulerBusyFixture struct {
	*independentAuditFixture
	scheduler *ExptSchedulerImpl
	manager   *schedulerBusyManager
	publisher *schedulerBusyPublisher
	locker    *schedulerBusyLocker
	event     *entity.ExptScheduleEvent
	before    []byte
	ctx       context.Context
}

func newSchedulerBusyFixture(t *testing.T, mode entity.ExptRunMode, dispatch string) *schedulerBusyFixture {
	t.Helper()
	f := newIndependentAuditFixture(t)
	require.NoError(t, f.conn.Exec("UPDATE experiment SET scheduler_mode=? WHERE id=? AND space_id=?", dispatch, f.expt, f.space).Error)
	require.NoError(t, f.manager.runLogRepo.Update(f.ctx, f.expt, f.r1, f.space, map[string]any{"mode": int32(mode)}))
	expt, err := f.manager.exptRepo.GetByID(f.ctx, f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, dispatch, expt.ExptDispatchMode)
	ctx, stop := context.WithCancel(f.ctx)
	t.Cleanup(stop)
	event := &entity.ExptScheduleEvent{
		SpaceID: f.space, ExptID: f.expt, ExptRunID: f.r1, ExptRunMode: mode, ExptType: entity.ExptType_Offline,
		CreatedAt: time.Now().Unix() - 120, Session: &entity.Session{UserID: "lock-busy-operator", AppID: 7},
		Ext:            map[string]string{"__item_ids": "[10,11]", "retry_yield_enabled": "false", "other": "preserve"},
		ItemRetryTimes: 4, ExecEvalSetItemIDs: []int64{10, 11}, InfraErrorRetryTimes: 7,
	}
	config := componentmocks.NewMockIConfiger(gomock.NewController(t))
	config.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{ZombieIntervalSecond: 3600, DaemonIntervalSecond: 37}).AnyTimes()
	// No target call is permitted, including async termination/sandbox destruction.
	f.manager.evalTargetService = svcmocks.NewMockIEvalTargetService(gomock.NewController(t))
	manager := &schedulerBusyManager{ExptMangerImpl: f.manager}
	publisher := &schedulerBusyPublisher{}
	locker := &schedulerBusyLocker{ILocker: lock.NewRedisLocker(f.client)}
	scheduler := &ExptSchedulerImpl{
		Manager: manager, ExptRepo: f.manager.exptRepo, Mutex: locker, Configer: config, Publisher: publisher,
		schedulerModeFactory: &DefaultSchedulerModeFactory{manager: manager, configer: config, publisher: publisher},
	}
	scheduler.itemLocker = func() lock.ILocker { return locker }
	return &schedulerBusyFixture{independentAuditFixture: f, scheduler: scheduler, manager: manager, publisher: publisher, locker: locker, event: event, ctx: ctx}
}

func (f *schedulerBusyFixture) holdAPI(t *testing.T) (context.Context, func(), string) {
	t.Helper()
	// completeTick is the API's actual same-run acquisition, not a timed guess.
	owner, release, err := f.independentAuditFixture.manager.completeTick(f.ctx, f.expt, f.r1)
	require.NoError(t, err)
	key := fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1)
	holder, err := f.raw.Get(f.ctx, key).Result()
	require.NoError(t, err)
	require.NotEmpty(t, holder)
	t.Cleanup(func() {
		release()
		require.Eventually(t, func() bool { return f.raw.Exists(context.Background(), key).Val() == 0 }, 3*time.Second, 10*time.Millisecond)
	})
	t.Logf("API_OWNER_ACQUIRED key=%s holder=%s", key, holder)
	return owner, release, holder
}

func (f *schedulerBusyFixture) deliver(t *testing.T) error {
	t.Helper()
	var err error
	f.before, err = json.Marshal(f.event)
	require.NoError(t, err)
	return SchedulerChain(f.scheduler.HandleEventErr, f.scheduler.HandleEventCheck, f.scheduler.HandleEventLock, f.scheduler.HandleEventEndpoint)(
		func(context.Context, *entity.ExptScheduleEvent) error {
			t.Error("busy tick entered endpoint continuation")
			return nil
		},
	)(f.ctx, f.event)
}

func (f *schedulerBusyFixture) assertBusyPreserved(t *testing.T) {
	t.Helper()
	require.Zero(t, f.manager.details, "busy tick entered the real HandleEventEndpoint/schedule")
	require.Zero(t, f.manager.completions, "scheduling recovery must not invoke CompleteExpt(Failed)/async cleanup")
	require.Zero(t, f.locker.cancels, "busy consumer cancelled a lock owner")
	require.Zero(t, f.locker.unlocks, "busy consumer released a lock owner")
	after, err := json.Marshal(f.event)
	require.NoError(t, err)
	require.Equal(t, f.before, after, "event deadline, identity, Ext or retry data changed")
	expt, err := f.independentAuditFixture.manager.exptRepo.GetByID(f.ctx, f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, entity.ExptStatus_Processing, expt.Status)
	run, err := f.independentAuditFixture.manager.runLogRepo.Get(f.ctx, f.expt, f.r1)
	require.NoError(t, err)
	require.Equal(t, int64(entity.ExptStatus_Processing), run.Status)
}

func (f *schedulerBusyFixture) assertOwner(t *testing.T, owner context.Context, holder string) {
	t.Helper()
	require.NoError(t, owner.Err(), "API owner was cancelled by competing scheduler tick")
	key := fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1)
	value, err := f.raw.Get(f.ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, holder, value)
	require.Positive(t, f.raw.PTTL(f.ctx, key).Val())
}

// Missing busy-path NextTick loses the only delivery, for both dispatch modes.
func TestSchedulerBusyUniqueTickLeavesSuccessor(t *testing.T) {
	for _, dispatch := range []string{entity.ExptDispatchModeLegacy, entity.ExptDispatchModeEnforce} {
		for _, mode := range []struct {
			name string
			mode entity.ExptRunMode
		}{{"submit", entity.EvaluationModeSubmit}, {"retry_all", entity.EvaluationModeRetryAll}, {"fail_retry", entity.EvaluationModeFailRetry}, {"retry_items", entity.EvaluationModeRetryItems}, {"trial", entity.EvaluationModeTrialRun}} {
			t.Run(dispatch+"/"+mode.name, func(t *testing.T) {
				f := newSchedulerBusyFixture(t, mode.mode, dispatch)
				owner, _, holder := f.holdAPI(t)
				require.NoError(t, f.deliver(t))
				f.assertBusyPreserved(t)
				f.assertOwner(t, owner, holder)
				require.Len(t, f.publisher.payloads, 1, "BUSINESS_RED unique busy tick was ACKed without a successor")
				require.Equal(t, f.before, f.publisher.payloads[0])
				require.Equal(t, []time.Duration{37 * time.Second}, f.publisher.delays)
				require.True(t, f.publisher.ctx == f.ctx, "busy rescheduling detached or substituted the caller context")
				require.Nil(t, f.publisher.ctx.Value(completeTickKey{}), "busy rescheduling forged held ownership")
			})
		}
	}
}

// Unwrapped send errors can enter business failure, especially at retry limit 10.
func TestSchedulerBusyPublishFailureRemainsMQRetry(t *testing.T) {
	for _, send := range []struct {
		name, message string
		retries       int
	}{{"ordinary", "producer unavailable", 0}, {"infra_exhausted", "send batch message fail", 10}} {
		for _, releaseBeforeError := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/release_%t", send.name, releaseBeforeError), func(t *testing.T) {
				f := newSchedulerBusyFixture(t, entity.EvaluationModeRetryAll, entity.ExptDispatchModeLegacy)
				f.event.InfraErrorRetryTimes = send.retries
				owner, release, holder := f.holdAPI(t)
				f.publisher.err = errors.New(send.message)
				if releaseBeforeError {
					f.publisher.beforeReturn = func() {
						release()
						key := fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1)
						require.Eventually(t, func() bool { return f.raw.Exists(f.ctx, key).Val() == 0 }, 3*time.Second, 10*time.Millisecond)
					}
				}
				err := f.deliver(t)
				f.assertBusyPreserved(t)
				if !releaseBeforeError {
					f.assertOwner(t, owner, holder)
				}
				require.True(t, errno.NeedMQRetry(err), "BUSINESS_RED busy tick send failure must return native MQRetry, got %v", err)
				require.ErrorContains(t, err, send.message)
				require.Len(t, f.publisher.payloads, 1, "recovery must attempt the original NextTick once")
				require.Equal(t, f.before, f.publisher.payloads[0])
				require.Equal(t, []time.Duration{37 * time.Second}, f.publisher.delays)
				require.True(t, f.publisher.ctx == f.ctx)
			})
		}
	}
}

func TestSchedulerBusyOnlineAndAppendKeepOriginalACK(t *testing.T) {
	for _, kind := range []string{"online", "append"} {
		t.Run(kind, func(t *testing.T) {
			f := newSchedulerBusyFixture(t, entity.EvaluationModeSubmit, entity.ExptDispatchModeLegacy)
			if kind == "online" {
				f.event.ExptType = entity.ExptType_Online
			} else {
				f.event.ExptRunMode = entity.EvaluationModeAppend
			}
			f.scheduler.itemLocker = func() lock.ILocker { t.Fatal("Online/Append selected offline tick locker"); return nil }
			owner, _, holder := f.holdAPI(t)
			require.NoError(t, f.deliver(t))
			f.assertBusyPreserved(t)
			f.assertOwner(t, owner, holder)
			require.Empty(t, f.publisher.payloads)
		})
	}
}

func TestSchedulerBusyAcquiredLockEntersOriginalEndpoint(t *testing.T) {
	f := newSchedulerBusyFixture(t, entity.EvaluationModeSubmit, entity.ExptDispatchModeLegacy)
	f.manager.detailErr = errno.NewMQRetryErr("original endpoint reached")
	require.Same(t, f.manager.detailErr, f.deliver(t))
	require.Equal(t, 1, f.manager.details, "acquired tick did not enter production schedule")
	require.Equal(t, completeTickIdentity{f.expt, f.r1}, f.manager.detailCtx.Value(completeTickKey{}))
	require.Equal(t, 1, f.locker.cancels)
	require.Equal(t, 1, f.locker.unlocks)
	require.Zero(t, f.manager.completions)
	require.Empty(t, f.publisher.payloads)
}
