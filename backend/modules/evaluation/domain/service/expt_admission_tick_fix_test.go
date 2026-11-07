// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func waitFixTickReleased(t *testing.T, f *independentAuditFixture, run int64) {
	t.Helper()
	key := fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, run)
	require.Eventually(t, func() bool { n, err := f.raw.Exists(f.ctx, key).Result(); return err == nil && n == 0 }, 2*time.Second, 5*time.Millisecond)
}

func TestAdmissionTickFixRetryItemsReentryAndActiveJoin(t *testing.T) {
	f := newIndependentAuditFixture(t)
	f.finishR1WithoutCID()
	ctx, stop, err := f.manager.completeTick(f.ctx, f.expt, f.r1)
	require.NoError(t, err)
	defer stop()
	ids := idmocks.NewMockIIDGenerator(gomock.NewController(t))
	ids.EXPECT().GenID(gomock.Any()).Return(f.r2, nil)
	f.manager.idgenerator = ids
	run, joined, err := f.manager.LogRetryItemsRun(ctx, f.expt, entity.EvaluationModeRetryItems, f.space, []int64{f.base + 100}, &entity.Session{})
	require.NoError(t, err)
	require.False(t, joined)
	require.Equal(t, f.r2, run)
	require.Equal(t, strconv.FormatInt(f.r2, 10), f.admission())
	stop()
	waitFixTickReleased(t, f, f.r1)
	require.NoError(t, f.manager.exptRepo.Update(f.ctx, &entity.Experiment{ID: f.expt, SpaceID: f.space, Status: entity.ExptStatus_Processing}))
	require.NoError(t, f.manager.runLogRepo.Update(f.ctx, f.expt, f.r2, f.space, map[string]any{"status": int64(entity.ExptStatus_Processing)}))
	ctx, stop2, err := f.manager.completeTick(f.ctx, f.expt, f.r2)
	require.NoError(t, err)
	defer stop2()
	ids2 := idmocks.NewMockIIDGenerator(gomock.NewController(t))
	ids2.EXPECT().GenID(gomock.Any()).Return(f.r2+1, nil)
	f.manager.idgenerator = ids2
	run, joined, err = f.manager.LogRetryItemsRun(ctx, f.expt, entity.EvaluationModeRetryItems, f.space, []int64{f.base + 101}, &entity.Session{})
	require.NoError(t, err)
	require.True(t, joined)
	require.Equal(t, f.r2, run)
	current, err := f.manager.runLogRepo.Get(f.ctx, f.expt, f.r2)
	require.NoError(t, err)
	require.Equal(t, int64(entity.ExptStatus_Processing), current.Status)
	require.Equal(t, strconv.FormatInt(f.r2, 10), f.admission())
	f.snapshot("FIX_numeric_new_run_and_active_held_tick_join")
}

func TestAdmissionTickFixConcurrentNewRunsAndCancellation(t *testing.T) {
	for _, kind := range []string{"second_run", "caller_cancel", "lease_loss"} {
		t.Run(kind, func(t *testing.T) {
			f := newIndependentAuditFixture(t)
			f.finishR1WithoutCID()
			// First Submit has no predecessor, so it never acquires an extra tick.
			require.Equal(t, "<absent>", f.admission())
			b := &independentAuditCreateBarrier{IExptRunLogRepo: f.manager.runLogRepo, f: f, entered: make(chan struct{}), proceed: make(chan struct{})}
			m := *f.manager
			m.runLogRepo = b
			ctx, cancel := context.WithCancel(f.ctx)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- m.LogRun(ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{})
			}()
			independentAuditWait(t, b.entered)
			require.Equal(t, "evaluation", f.admission())
			if kind == "second_run" {
				err := f.manager.LogRun(f.ctx, f.expt, f.r2+1, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{})
				require.Error(t, err)
				require.True(t, errno.NeedMQRetry(err))
				require.Equal(t, "evaluation", f.admission())
				close(b.proceed)
				require.NoError(t, independentAuditResult(t, done))
			} else {
				if kind == "caller_cancel" {
					cancel()
				} else {
					require.NoError(t, f.raw.Del(f.ctx, fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1)).Err())
				}
				err := independentAuditResult(t, done)
				require.Error(t, err)
				require.True(t, errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))
				var n int64
				require.NoError(t, f.conn.Raw("SELECT COUNT(*) FROM expt_run_log WHERE expt_id=? AND expt_run_id=?", f.expt, f.r2).Scan(&n).Error)
				require.Zero(t, n)
				if kind == "caller_cancel" {
					require.Equal(t, "<absent>", f.admission())
				} else {
					require.Equal(t, "evaluation", f.admission(), "lost tick cannot authorize fixed admission compensation")
				}
				waitFixTickReleased(t, f, f.r1)
				require.NoError(t, f.replay(f.manager))
				require.Equal(t, "<absent>", f.admission())
			}
			f.snapshot("FIX_new_run_" + kind)
		})
	}
}

type fixTokenGuard struct {
	component.ICentralReservationGuard
	f      *independentAuditFixture
	prefix string
	calls  []int64
	after  func(context.Context)
}

func (g *fixTokenGuard) key(item int64) string {
	return fmt.Sprintf("%s:%d:%d", g.prefix, g.f.r1, item)
}

func (g *fixTokenGuard) Release(ctx context.Context, scope string, run, item int64, token, reason string) error {
	g.calls = append(g.calls, item)
	deadline, bounded := ctx.Deadline()
	g.f.log("FIX_token_release", map[string]any{"run": run, "item": item, "token": token, "deadline": deadline, "bounded": bounded, "ctx_error": fmt.Sprint(ctx.Err())})
	const script = `local v=redis.call('GET',KEYS[1]);if not v then return 0 end;if v~=ARGV[1] then return -1 end;redis.call('DEL',KEYS[1]);return 1`
	cas, err := g.f.raw.Eval(ctx, script, []string{g.key(item)}, fmt.Sprintf("%d/%s", run, token)).Int64()
	if err != nil {
		return err
	}
	if cas < 0 {
		return fmt.Errorf("run/token mismatch")
	}
	if g.after != nil {
		g.after(ctx)
	}
	return ctx.Err()
}

func newFixWholeFixture(t *testing.T) (*independentAuditFixture, *fixTokenGuard, repo.IExptRunTerminationRepo) {
	t.Helper()
	f := newIndependentAuditFixture(t)
	f.seedSources(3, false)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Updates(map[string]any{"scheduler_mode": "enforce", "scheduler_scope": fmt.Sprintf("fix-%d", f.expt)}).Error)
	result := svcmocks.NewMockExptResultService(gomock.NewController(t))
	result.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), f.space, f.expt, gomock.Any()).DoAndReturn(func(ctx context.Context, _, _ int64, items []int64) error {
		f.log("FIX_required_filters", items)
		return ctx.Err()
	}).AnyTimes()
	result.EXPECT().GetIncompleteTurns(gomock.Any(), f.expt, f.space, gomock.Any()).Return(nil, nil).AnyTimes()
	f.manager.exptResultService = result
	g := &fixTokenGuard{ICentralReservationGuard: component.NewNoopCentralReservationGuard(), f: f, prefix: fmt.Sprintf("fix-audit:%d:reservation", f.expt)}
	f.manager.centralGuard = g
	port := f.manager.runFinishRepo.(repo.IExptRunTerminationRepo)
	for i := 0; i < 3; i++ {
		item := f.base + 100 + int64(i)
		s, err := port.GetTerminationItemSource(f.ctx, f.space, f.expt, f.r1, item)
		require.NoError(t, err)
		key := g.key(item)
		f.keys = append(f.keys, key)
		require.NoError(t, f.raw.Set(f.ctx, key, fmt.Sprintf("%d/%s", f.r1, s.Current.RequestID), time.Hour).Err())
	}
	return f, g, port
}

func TestWholeReleaseFixCancellationAndRealLeaseLoss(t *testing.T) {
	for _, kind := range []string{"already_cancelled_filters_required", "already_cancelled_no_filters", "caller_mid_loop", "real_tick_loss"} {
		t.Run(kind, func(t *testing.T) {
			f, g, port := newFixWholeFixture(t)
			ctx, cancel := context.WithCancel(f.ctx)
			defer cancel()
			original := f.manager.runFinishRepo
			opts := []entity.CompleteExptOptionFn{entity.WithStatus(entity.ExptStatus_Terminated)}
			if kind == "already_cancelled_filters_required" || kind == "already_cancelled_no_filters" {
				f.manager.runFinishRepo = &cancelFilterCommitHook{IExptRunFinishRepo: original, port: port, after: cancel}
				if kind == "already_cancelled_no_filters" {
					opts = append(opts, entity.NoCompleteItemTurn())
				}
			} else {
				g.after = func(releaseCtx context.Context) {
					if len(g.calls) != 1 {
						return
					}
					if kind == "caller_mid_loop" {
						cancel()
						return
					}
					require.NoError(t, f.raw.Del(f.ctx, fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1)).Err())
					<-releaseCtx.Done()
				}
			}
			start := time.Now()
			applied, err := f.manager.TerminateCurrentRun(ctx, f.expt, f.r1, f.space, &entity.Session{}, nil, opts...)
			require.True(t, applied)
			require.Error(t, err)
			require.True(t, errno.NeedMQRetry(err))
			require.Less(t, time.Since(start), 10*time.Second)
			if kind == "caller_mid_loop" || kind == "real_tick_loss" {
				require.Len(t, g.calls, 1, "healthy loop stops on caller or actual lease cancellation")
			} else {
				require.Len(t, g.calls, 3, "already-cancelled frozen entry safely cleans original tokens")
			}
			if kind != "already_cancelled_no_filters" {
				require.True(t, f.slot())
				require.Equal(t, "evaluation", f.admission())
				n, err := f.raw.Exists(f.ctx, f.cidKey()).Result()
				require.NoError(t, err)
				require.Zero(t, n)
			} else {
				require.False(t, f.slot())
				require.Equal(t, "<absent>", f.admission())
			}
			f.snapshot("FIX_after_" + kind)
			g.after = nil
			f.manager.runFinishRepo = original
			waitFixTickReleased(t, f, f.r1)
			require.NoError(t, f.replay(f.manager))
			require.False(t, f.slot())
			require.Equal(t, "<absent>", f.admission())
			n, err := f.raw.Exists(f.ctx, f.cidKey()).Result()
			require.NoError(t, err)
			require.EqualValues(t, 1, n)
			for i := 0; i < 3; i++ {
				s, err := port.GetTerminationItemSource(f.ctx, f.space, f.expt, f.r1, f.base+100+int64(i))
				require.NoError(t, err)
				require.Equal(t, f.r1, s.Current.RunID)
				require.NotEmpty(t, s.Current.RequestID)
			}
		})
	}
}

func TestAdmissionTickFixSupersededReleaseStopsWithOwner(t *testing.T) {
	for _, kind := range []string{"caller_cancel", "lease_loss"} {
		t.Run(kind, func(t *testing.T) {
			f, g, _ := newFixWholeFixture(t)
			_, err := f.manager.TerminateCurrentRun(f.ctx, f.expt, f.r1, f.space, &entity.Session{}, nil)
			require.NoError(t, err)
			waitFixTickReleased(t, f, f.r1)
			g.calls = nil
			ctx, cancel := context.WithCancel(f.ctx)
			defer cancel()
			g.after = func(ioCtx context.Context) {
				if kind == "caller_cancel" {
					cancel()
				} else {
					require.NoError(t, f.raw.Del(f.ctx, fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1)).Err())
					<-ioCtx.Done()
				}
			}
			start := time.Now()
			err = f.manager.LogRun(ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{})
			require.Error(t, err)
			require.Less(t, time.Since(start), 6*time.Second)
			require.Len(t, g.calls, 1, "cancelled superseded cleanup must not detach each further IO")
			var count int64
			require.NoError(t, f.conn.Raw("SELECT COUNT(*) FROM expt_run_log WHERE expt_id=? AND expt_run_id=?", f.expt, f.r2).Scan(&count).Error)
			require.Zero(t, count)
			f.snapshot("FIX_superseded_release_" + kind)
		})
	}
}

func TestAdmissionTickFixStaleWithoutCancelCannotReenter(t *testing.T) {
	for _, endpoint := range []string{"CompleteExpt", "compensation"} {
		t.Run(endpoint, func(t *testing.T) {
			f := newIndependentAuditFixture(t)
			f.finishR1WithoutCID()
			oldCtx, oldCancel, err := f.manager.completeTick(f.ctx, f.expt, f.r1)
			require.NoError(t, err)
			oldCancel()
			waitFixTickReleased(t, f, f.r1)
			stale := context.WithoutCancel(oldCtx)
			require.NoError(t, stale.Err())
			b := &independentAuditCreateBarrier{IExptRunLogRepo: f.manager.runLogRepo, f: f, entered: make(chan struct{}), proceed: make(chan struct{})}
			newManager := *f.manager
			newManager.runLogRepo = b
			done := make(chan error, 1)
			go func() {
				done <- newManager.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{})
			}()
			independentAuditWait(t, b.entered)
			require.Equal(t, "evaluation", f.admission())
			f.snapshot("STALE_marker_R2_owned_preCreate")
			if endpoint == "CompleteExpt" {
				err = f.manager.CompleteExpt(stale, f.expt, &f.r1, f.space, &entity.Session{})
			} else {
				err = f.manager.cleanupCurrentUnscheduledRun(stale, f.expt, f.r2, f.space, errors.New("late failed Create"), true)
			}
			preserved := f.admission()
			close(b.proceed)
			require.NoError(t, independentAuditResult(t, done))
			require.Equal(t, "evaluation", preserved, "BUSINESS_RED stale held marker cannot delete R2 admission")
			require.Error(t, err)
			require.True(t, errno.NeedMQRetry(err))
			f.snapshot("STALE_marker_rejected_after_actual_owner_cancel")
		})
	}
}

type fixDelayedUnlockClient struct {
	infraredis.Cmdable
	key              string
	owner            atomic.Value
	entered, proceed chan struct{}
	once             sync.Once
	completed        atomic.Int32
	f                *independentAuditFixture
}

func (c *fixDelayedUnlockClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	owner := c.owner.Load()
	delayed := len(keys) == 1 && keys[0] == c.key && strings.Contains(script, "'DEL'") && owner != nil && len(args) > 0 && fmt.Sprint(args[0]) == owner.(string)
	if delayed {
		c.once.Do(func() { close(c.entered) })
		<-c.proceed
	}
	result := c.Cmdable.Eval(ctx, script, keys, args...)
	if delayed {
		value, err := result.Result()
		c.f.log("OLD_actual_delayed_unlock_CAS", map[string]any{"holder": owner, "CAS": value, "error": fmt.Sprint(err)})
		c.completed.Add(1)
	}
	return result
}

func TestAdmissionTickFixSchedulerOldUnlockPreservesNewOwner(t *testing.T) {
	f := newIndependentAuditFixture(t)
	gate := &fixDelayedUnlockClient{Cmdable: f.client, key: fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1), entered: make(chan struct{}), proceed: make(chan struct{}), f: f}
	scheduler := &ExptSchedulerImpl{Manager: f.manager, Mutex: lock.NewRedisLockerWithHolder(gate, "evaluation"), itemLocker: func() lock.ILocker { return lock.NewRedisLocker(gate) }}
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.r1, ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeSubmit, Session: &entity.Session{}}
	oldIn, oldExit, newIn, newExit := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	oldDone, newDone := make(chan error, 1), make(chan error, 1)
	var oldOnce, newOnce, gateOnce sync.Once
	stopOld := func() { oldOnce.Do(func() { close(oldExit) }) }
	stopNew := func() { newOnce.Do(func() { close(newExit) }) }
	openGate := func() { gateOnce.Do(func() { close(gate.proceed) }) }
	defer stopOld()
	defer stopNew()
	defer openGate()
	go func() {
		oldDone <- scheduler.HandleEventLock(func(context.Context, *entity.ExptScheduleEvent) error { close(oldIn); <-oldExit; return nil })(f.ctx, event)
	}()
	independentAuditWait(t, oldIn)
	holder, err := f.raw.Get(f.ctx, gate.key).Result()
	require.NoError(t, err)
	gate.owner.Store(holder)
	stopOld()
	independentAuditWait(t, gate.entered)
	// Both real old Unlock calls are delayed. Its real 5s Redis TTL expires;
	// neither the async renew worker nor scheduler defer has deleted the key yet.
	require.Eventually(t, func() bool { n, err := f.raw.Exists(f.ctx, gate.key).Result(); return err == nil && n == 0 }, 7*time.Second, 10*time.Millisecond)
	go func() {
		newDone <- scheduler.HandleEventLock(func(context.Context, *entity.ExptScheduleEvent) error { close(newIn); <-newExit; return nil })(f.ctx, event)
	}()
	independentAuditWait(t, newIn)
	nextHolder, err := f.raw.Get(f.ctx, gate.key).Result()
	require.NoError(t, err)
	f.log("SHORT_tick_handoff", map[string]any{"old": holder, "new": nextHolder, "lifecycle": f.admission()})
	openGate()
	require.NoError(t, independentAuditResult(t, oldDone))
	require.Eventually(t, func() bool { return gate.completed.Load() >= 2 }, 2*time.Second, 5*time.Millisecond)
	retained, readErr := f.raw.Get(f.ctx, gate.key).Result()
	stopNew()
	require.NoError(t, independentAuditResult(t, newDone))
	require.NoError(t, readErr, "BUSINESS_RED old delayed Unlock must not delete new short tick owner")
	require.Equal(t, nextHolder, retained)
	require.NotEqual(t, holder, nextHolder, "renewed short ticks have independent existing locker owners")
	require.Equal(t, "evaluation", f.admission(), "ordinary lifecycle value remains unchanged")
}

type fixExpiredNilGuard struct {
	component.ICentralReservationGuard
}

func (g *fixExpiredNilGuard) Release(ctx context.Context, _ string, _, _ int64, _, _ string) error {
	<-ctx.Done()
	return nil
}

func TestWholeReleaseFixLegacyNilReturnUnchanged(t *testing.T) {
	f, _, _ := newFixWholeFixture(t)
	f.manager.centralGuard = &fixExpiredNilGuard{ICentralReservationGuard: component.NewNoopCentralReservationGuard()}
	expt, err := f.manager.exptRepo.GetByID(f.ctx, f.expt, f.space)
	require.NoError(t, err)
	err = f.manager.releaseCentralQuotaForRun(f.ctx, expt, &f.r1, "legacy contract", "regression")
	require.NoError(t, err, "BUSINESS_RED non-frozen legacy path preserves the original guard nil result")
}

func TestAdmissionTickFixOrdinaryNumericFourCombinations(t *testing.T) {
	for _, oldMode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeRetryItems} {
		for _, newMode := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryItems} {
			t.Run(fmt.Sprintf("%d_to_%d", oldMode, newMode), func(t *testing.T) {
				f := newIndependentAuditFixture(t, oldMode)
				f.finishR1WithoutCID()
				old, newManager := *f.manager, *f.manager
				barrier := &independentAuditQuotaRepo{QuotaRepo: f.quota, f: f, entered: make(chan struct{}), proceed: make(chan struct{})}
				old.quotaRepo = barrier
				var once sync.Once
				resume := func() { once.Do(func() { close(barrier.proceed) }) }
				defer resume()
				done := make(chan error, 1)
				go func() { done <- f.replay(&old) }()
				independentAuditWait(t, barrier.entered)
				admit := func() error {
					if newMode == entity.EvaluationModeRetryItems {
						ids := idmocks.NewMockIIDGenerator(gomock.NewController(t))
						ids.EXPECT().GenID(gomock.Any()).Return(f.r2, nil)
						newManager.idgenerator = ids
						_, joined, err := newManager.LogRetryItemsRun(f.ctx, f.expt, newMode, f.space, nil, &entity.Session{})
						require.False(t, joined)
						return err
					}
					return newManager.LogRun(f.ctx, f.expt, f.r2, newMode, f.space, nil, &entity.Session{})
				}
				err := admit()
				require.Error(t, err)
				require.True(t, errno.NeedMQRetry(err))
				require.Equal(t, "<absent>", f.admission())
				require.False(t, f.slot())
				resume()
				require.NoError(t, independentAuditResult(t, done))
				waitFixTickReleased(t, f, f.r1)
				require.NoError(t, admit())
				require.NoError(t, NewQuotaService(f.quota, newManager.configer).AllowExptRun(f.ctx, f.expt, f.space, &entity.Session{}))
				want := "evaluation"
				if newMode == entity.EvaluationModeRetryItems {
					want = strconv.FormatInt(f.r2, 10)
				}
				require.Equal(t, want, f.admission())
				require.True(t, f.slot())
				require.NoError(t, f.replay(&old))
				require.True(t, f.slot())
				require.Equal(t, want, f.admission())
				run, err := newManager.runLogRepo.Get(f.ctx, f.expt, f.r2)
				require.NoError(t, err)
				require.Equal(t, int32(newMode), run.Mode)
				f.snapshot("FIX_four_protocol_combination")
			})
		}
	}
}

type fixChangedAfterTickRepo struct {
	repo.IExperimentRepo
	f     *independentAuditFixture
	reads int
}

func (r *fixChangedAfterTickRepo) GetByID(ctx context.Context, expt, space int64) (*entity.Experiment, error) {
	r.reads++
	snapshot, err := r.IExperimentRepo.GetByID(ctx, expt, space)
	if err == nil && r.reads == 1 {
		// Return A's captured lookup only after a real competing new run has
		// completed Create/Latest/Allow and released A's previous tick.
		if err := r.f.manager.LogRun(r.f.ctx, expt, r.f.r2+1, entity.EvaluationModeRetryAll, space, nil, &entity.Session{}); err != nil {
			return nil, err
		}
		if err := NewQuotaService(r.f.quota, r.f.manager.configer).AllowExptRun(r.f.ctx, expt, space, &entity.Session{}); err != nil {
			return nil, err
		}
		waitFixTickReleased(r.f.t, r.f, r.f.r1)
	}
	return snapshot, err
}

func TestAdmissionTickFixRechecksPredecessorAfterLock(t *testing.T) {
	f := newIndependentAuditFixture(t)
	f.finishR1WithoutCID()
	m := *f.manager
	m.exptRepo = &fixChangedAfterTickRepo{IExperimentRepo: f.manager.exptRepo, f: f}
	err := m.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{})
	require.Error(t, err)
	require.True(t, errno.NeedMQRetry(err))
	require.Equal(t, "evaluation", f.admission())
	require.True(t, f.slot())
	var count int64
	require.NoError(t, f.conn.Raw("SELECT COUNT(*) FROM expt_run_log WHERE expt_id=? AND expt_run_id=?", f.expt, f.r2).Scan(&count).Error)
	require.Zero(t, count)
	f.snapshot("FIX_changed_predecessor_preserves_competing_run")
}

type fixSingleTimeoutGuard struct {
	*fixTokenGuard
	first bool
}

func (g *fixSingleTimeoutGuard) Release(ctx context.Context, scope string, run, item int64, token, reason string) error {
	if g.first {
		g.first = false
		g.f.log("FIX_one_IO_timeout_start", map[string]any{"run": run, "item": item, "token": token})
		<-ctx.Done()
		return nil
	}
	return g.fixTokenGuard.Release(ctx, scope, run, item, token, reason)
}

func TestWholeReleaseFixSingleTimeoutCannotMarkDone(t *testing.T) {
	f, guard, _ := newFixWholeFixture(t)
	slow := &fixSingleTimeoutGuard{fixTokenGuard: guard, first: true}
	f.manager.centralGuard = slow
	_, err := f.manager.TerminateCurrentRun(f.ctx, f.expt, f.r1, f.space, &entity.Session{}, nil)
	require.Error(t, err)
	require.True(t, errno.NeedMQRetry(err))
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.True(t, f.slot())
	require.Equal(t, "evaluation", f.admission())
	n, err := f.raw.Exists(f.ctx, f.cidKey()).Result()
	require.NoError(t, err)
	require.Zero(t, n)
	f.snapshot("FIX_single_IO_timeout_keeps_shared_resources_and_Done")
	waitFixTickReleased(t, f, f.r1)
	require.NoError(t, f.replay(f.manager))
	require.False(t, f.slot())
	require.Equal(t, "<absent>", f.admission())
	n, err = f.raw.Exists(f.ctx, f.cidKey()).Result()
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
}

func TestAdmissionTickFixBorrowerCannotCancelOwner(t *testing.T) {
	f := newIndependentAuditFixture(t)
	f.finishR1WithoutCID()
	owner, stopOwner, err := f.manager.completeTick(f.ctx, f.expt, f.r1)
	require.NoError(t, err)
	defer stopOwner()
	masked := context.WithoutCancel(owner)
	borrowed, stopBorrow, err := f.manager.completeTick(masked, f.expt, f.r1)
	require.NoError(t, err)
	stopBorrow()
	require.ErrorIs(t, borrowed.Err(), context.Canceled)
	require.NoError(t, owner.Err())
	n, err := f.raw.Exists(f.ctx, fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1)).Result()
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	borrowed, stopBorrow, err = f.manager.completeTick(masked, f.expt, f.r1)
	require.NoError(t, err)
	defer stopBorrow()
	stopOwner()
	select {
	case <-borrowed.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("borrowed WithoutCancel context did not follow real owner cancellation")
	}
	require.ErrorIs(t, borrowed.Err(), context.Canceled)
	require.NoError(t, masked.Err())
}

type fixCreateFailureRepo struct {
	repo.IExptRunLogRepo
	failure error
}

func (r *fixCreateFailureRepo) Create(context.Context, *entity.ExptRunLog) error { return r.failure }

type fixLatestFailureRepo struct {
	repo.IExperimentRepo
	run     int64
	failure error
}

func (r *fixLatestFailureRepo) Update(ctx context.Context, x *entity.Experiment) error {
	if x.LatestRunID == r.run {
		return r.failure
	}
	return r.IExperimentRepo.Update(ctx, x)
}

func TestAdmissionTickFixHeldTickFailureCompensation(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		for _, fault := range []string{"Create", "Latest"} {
			t.Run(fmt.Sprintf("%d_%s", mode, fault), func(t *testing.T) {
				f := newIndependentAuditFixture(t)
				f.finishR1WithoutCID()
				ctx, stop, err := f.manager.completeTick(f.ctx, f.expt, f.r1)
				require.NoError(t, err)
				defer stop()
				m := *f.manager
				failure := errors.New("injected " + fault + " failure")
				if fault == "Create" {
					m.runLogRepo = &fixCreateFailureRepo{IExptRunLogRepo: m.runLogRepo, failure: failure}
				} else {
					m.exptRepo = &fixLatestFailureRepo{IExperimentRepo: m.exptRepo, run: f.r2, failure: failure}
				}
				start := time.Now()
				if mode == entity.EvaluationModeRetryItems {
					ids := idmocks.NewMockIIDGenerator(gomock.NewController(t))
					ids.EXPECT().GenID(gomock.Any()).Return(f.r2, nil)
					m.idgenerator = ids
					_, _, err = m.LogRetryItemsRun(ctx, f.expt, mode, f.space, nil, &entity.Session{})
				} else {
					err = m.LogRun(ctx, f.expt, f.r2, mode, f.space, nil, &entity.Session{})
				}
				require.ErrorIs(t, err, failure)
				require.Less(t, time.Since(start), 3*time.Second)
				require.Equal(t, "<absent>", f.admission())
				require.False(t, f.slot())
				require.NoError(t, ctx.Err())
				current, err := f.manager.exptRepo.GetByID(f.ctx, f.expt, f.space)
				require.NoError(t, err)
				require.Equal(t, f.r1, current.LatestRunID)
				f.snapshot("FIX_held_tick_failure_" + fault)
			})
		}
	}
}

type fixCancelFinishRepo struct {
	repo.IExptRunFinishRepo
	cancel   context.CancelFunc
	captured context.Context
}

func (r *fixCancelFinishRepo) FinishCurrentRun(ctx context.Context, q *entity.ExptRunFinishRequest) (*entity.ExptRunFinishResult, error) {
	x, err := r.IExptRunFinishRepo.FinishCurrentRun(ctx, q)
	if err == nil {
		r.captured = ctx
		r.cancel()
	}
	return x, err
}

func TestAdmissionTickFixTwoCancelledOldCompletionsPreserveNewSlot(t *testing.T) {
	f := newIndependentAuditFixture(t)
	f.finishR1WithoutCID()
	first, second, late := *f.manager, *f.manager, *f.manager
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	hook := &fixCancelFinishRepo{IExptRunFinishRepo: first.runFinishRepo, cancel: cancel}
	first.runFinishRepo = hook
	makeBarrier := func() *independentAuditQuotaRepo {
		return &independentAuditQuotaRepo{QuotaRepo: f.quota, f: f, entered: make(chan struct{}), proceed: make(chan struct{})}
	}
	b1, b2, bLate := makeBarrier(), makeBarrier(), makeBarrier()
	first.quotaRepo = b1
	second.quotaRepo = b2
	late.quotaRepo = bLate
	var once1, once2, onceLate sync.Once
	resume1 := func() { once1.Do(func() { close(b1.proceed) }) }
	resume2 := func() { once2.Do(func() { close(b2.proceed) }) }
	resumeLate := func() { onceLate.Do(func() { close(bLate.proceed) }) }
	defer resume1()
	defer resume2()
	defer resumeLate()
	firstDone, secondDone, lateDone := make(chan error, 1), make(chan error, 1), make(chan error, 1)
	go func() {
		firstDone <- first.CompleteExpt(ctx, f.expt, &f.r1, f.space, &entity.Session{}, entity.NoCompleteItemTurn())
	}()
	independentAuditWait(t, b1.entered)
	require.True(t, errno.NeedMQRetry(f.replay(&second)), "second old completion cannot enter first's recovered real tick")
	require.NoError(t, f.raw.Del(f.ctx, fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1)).Err())
	require.Error(t, independentAuditResult(t, firstDone))
	waitFixTickReleased(t, f, f.r1)
	go func() { secondDone <- f.replay(&second) }()
	independentAuditWait(t, b2.entered)
	// A1's cancelled actual owner remains in the values of WithoutCancel.
	// A2 now owns the same real tick. A1 must reject this stale reentry.
	go func() {
		lateDone <- late.CompleteExpt(context.WithoutCancel(hook.captured), f.expt, &f.r1, f.space, &entity.Session{}, entity.NoCompleteItemTurn())
	}()
	latePaused := false
	select {
	case err := <-lateDone:
		require.True(t, errno.NeedMQRetry(err))
	case <-bLate.entered:
		latePaused = true
	case <-time.After(3 * time.Second):
		t.Fatal("late completion did not reject or reach its real quota barrier")
	}
	resume2()
	require.NoError(t, independentAuditResult(t, secondDone))
	waitFixTickReleased(t, f, f.r1)
	require.NoError(t, f.manager.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{}))
	require.NoError(t, NewQuotaService(f.quota, f.manager.configer).AllowExptRun(f.ctx, f.expt, f.space, &entity.Session{}))
	require.True(t, f.slot())
	require.Equal(t, "evaluation", f.admission())
	if latePaused {
		resumeLate()
		_ = independentAuditResult(t, lateDone)
	}
	require.True(t, f.slot(), "BUSINESS_RED delayed cancelled old completion must preserve actual new slot")
	require.Equal(t, "evaluation", f.admission())
	f.snapshot("FIX_two_old_cancelled_completions_preserve_R2")
}

type residualLifecycleEvalClient struct {
	infraredis.Cmdable
	f                *independentAuditFixture
	key              string
	entered, proceed chan struct{}
	once             atomic.Int32
}

func (c *residualLifecycleEvalClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	if len(keys) != 1 || keys[0] != c.key || !strings.Contains(script, "'DEL'") || !c.once.CompareAndSwap(0, 1) {
		return c.Cmdable.Eval(ctx, script, keys, args...)
	}
	deadline, hasDeadline := ctx.Deadline()
	c.f.log("RR_F1_1_actual_lifecycle_Eval_enter", map[string]any{"key": c.key, "holder": fmt.Sprint(args[0]), "Done_nil": ctx.Done() == nil, "Err": fmt.Sprint(ctx.Err()), "has_deadline": hasDeadline, "deadline": deadline})
	close(c.entered)
	selected := "proceed"
	select {
	case <-c.proceed:
	case <-ctx.Done():
		selected = "ctx_done"
	}
	c.f.log("RR_F1_1_actual_lifecycle_Eval_delegate", map[string]any{"selected": selected, "Done_nil": ctx.Done() == nil, "Err": fmt.Sprint(ctx.Err()), "has_deadline": hasDeadline, "deadline": deadline})
	// Delegate the unchanged context and exact Lua/key/holder to real Redis.
	result := c.Cmdable.Eval(ctx, script, keys, args...)
	value, err := result.Result()
	c.f.log("RR_F1_1_actual_lifecycle_Eval_CAS", map[string]any{"CAS": value, "Err": fmt.Sprint(err), "ctx_Err": fmt.Sprint(ctx.Err())})
	return result
}

func TestResidualF1LifecycleEvalCancellationPreservesR2(t *testing.T) {
	f := newIndependentAuditFixture(t)
	f.finishR1WithoutCID()
	gate := &residualLifecycleEvalClient{Cmdable: f.client, f: f, key: f.manager.makeExptMutexLockKey(f.expt), entered: make(chan struct{}), proceed: make(chan struct{})}
	a1 := *f.manager
	// Only A1 admission IO is delayed; the real tick factory is unchanged.
	a1.mutex = lock.NewRedisLockerWithHolder(gate, "evaluation")
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	var once sync.Once
	release := func() { once.Do(func() { close(gate.proceed) }) }
	defer release()
	done := make(chan error, 1)
	go func() { done <- a1.CompleteExpt(ctx, f.expt, &f.r1, f.space, &entity.Session{}) }()
	independentAuditWait(t, gate.entered)
	f.snapshot("RR_F1_1_A1_at_final_admission_Eval")
	cancel()
	waitFixTickReleased(t, f, f.r1)
	f.snapshot("RR_F1_1_A1_cancelled_actual_tick_absent")
	require.NoError(t, f.replay(f.manager), "A2 normal client legitimately replays completed R1")
	waitFixTickReleased(t, f, f.r1)
	f.snapshot("RR_F1_1_A2_completed")
	require.NoError(t, f.manager.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{}))
	require.NoError(t, NewQuotaService(f.quota, f.manager.configer).AllowExptRun(f.ctx, f.expt, f.space, &entity.Session{}))
	require.Equal(t, "evaluation", f.admission())
	require.True(t, f.slot())
	f.snapshot("RR_F1_1_R2_real_Create_Latest_Allow_before_A1_Eval")
	release()
	a1err := independentAuditResult(t, done)
	f.log("RR_F1_1_A1_return", map[string]any{"Err": fmt.Sprint(a1err)})
	f.snapshot("RR_F1_1_after_late_A1_Eval")
	current, err := f.manager.exptRepo.GetByID(f.ctx, f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, f.r2, current.LatestRunID)
	run, err := f.manager.runLogRepo.Get(f.ctx, f.expt, f.r2)
	require.NoError(t, err)
	require.Equal(t, int64(entity.ExptStatus_Pending), run.Status)
	require.True(t, f.slot(), "RR-F1-1 isolates admission deletion, not a shared quota failure")
	require.Equal(t, "evaluation", f.admission(), "BUSINESS_RED RR-F1-1: late lifecycle Eval must preserve actual R2 admission")
}

func TestResidualF1InjectedHolderAndNumericProtocol(t *testing.T) {
	t.Run("ordinary_custom", func(t *testing.T) {
		f := newIndependentAuditFixture(t)
		custom := *f.manager
		custom.mutex = lock.NewRedisLockerWithHolder(f.client, "custom-residual-holder")
		// A different injected holder must not impersonate the current evaluation.
		require.NoError(t, custom.releaseCompleteAdmission(f.ctx, f.expt, f.r1, f.space))
		require.Equal(t, "evaluation", f.admission())
		require.NoError(t, f.manager.releaseCompleteAdmission(f.ctx, f.expt, f.r1, f.space))
		require.Equal(t, "<absent>", f.admission())
		require.NoError(t, custom.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{}))
		require.Equal(t, "custom-residual-holder", f.admission())
		ctx, cancel := context.WithCancel(f.ctx)
		cancel()
		require.ErrorIs(t, custom.releaseCompleteAdmission(ctx, f.expt, f.r2, f.space), context.Canceled)
		require.Equal(t, "custom-residual-holder", f.admission())
		require.NoError(t, custom.releaseCompleteAdmission(f.ctx, f.expt, f.r2, f.space))
		require.Equal(t, "<absent>", f.admission())
		f.snapshot("RR_F1_1_custom_injected_holder_preserved")
	})
	t.Run("numeric", func(t *testing.T) {
		f := newIndependentAuditFixture(t, entity.EvaluationModeRetryItems)
		numeric := *f.manager
		numeric.mutex = lock.NewRedisLockerWithHolder(f.client, "different-injected-holder")
		require.Equal(t, strconv.FormatInt(f.r1, 10), f.admission())
		ctx, cancel := context.WithCancel(f.ctx)
		cancel()
		require.ErrorIs(t, numeric.releaseCompleteAdmission(ctx, f.expt, f.r1, f.space), context.Canceled)
		require.Equal(t, strconv.FormatInt(f.r1, 10), f.admission())
		require.NoError(t, numeric.releaseCompleteAdmission(f.ctx, f.expt, f.r1, f.space))
		require.Equal(t, "<absent>", f.admission())
		f.snapshot("RR_F1_1_numeric_protocol_unchanged")
	})
}

type compensationFollowupCreateFault struct {
	repo.IExptRunLogRepo
	owner   chan context.Context
	failure error
}

func (r *compensationFollowupCreateFault) Create(ctx context.Context, _ *entity.ExptRunLog) error {
	owner, _ := ctx.Value(completeTickOwnerKey{}).(context.Context)
	r.owner <- owner
	return r.failure
}

func TestResidualF1CompensationLifecycleEvalLeaseLossPreservesC(t *testing.T) {
	f := newIndependentAuditFixture(t)
	f.finishR1WithoutCID()
	gate := &residualLifecycleEvalClient{Cmdable: f.client, f: f, key: f.manager.makeExptMutexLockKey(f.expt), entered: make(chan struct{}), proceed: make(chan struct{})}
	b := *f.manager
	b.mutex = lock.NewRedisLockerWithHolder(gate, "evaluation")
	failure := errors.New("B Create failed before SQL insert")
	fault := &compensationFollowupCreateFault{IExptRunLogRepo: b.runLogRepo, owner: make(chan context.Context, 1), failure: failure}
	b.runLogRepo = fault
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	var once sync.Once
	release := func() { once.Do(func() { close(gate.proceed) }) }
	defer release()
	done := make(chan error, 1)
	go func() {
		done <- b.LogRun(ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{})
	}()
	independentAuditWait(t, gate.entered)
	owner := <-fault.owner
	require.NotNil(t, owner)
	require.NoError(t, owner.Err())
	f.snapshot("COMP_B_Create_fault_inside_final_lifecycle_Eval")
	// Cancelling only the request does not invalidate the retained predecessor
	// lease. Both old completion and the next admission are still excluded.
	cancel()
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.NoError(t, owner.Err())
	require.True(t, errno.NeedMQRetry(f.replay(f.manager)))
	c := f.r2 + 1
	require.True(t, errno.NeedMQRetry(f.manager.LogRun(f.ctx, f.expt, c, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{})))
	f.snapshot("COMP_parent_cancel_only_retained_tick_blocks_R1_and_C")
	require.NoError(t, f.raw.Del(f.ctx, fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1)).Err())
	select {
	case <-owner.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("actual predecessor lease did not cancel after real key loss")
	}
	require.Error(t, owner.Err())
	waitFixTickReleased(t, f, f.r1)
	f.log("COMP_actual_predecessor_owner_cancelled", map[string]any{"Err": fmt.Sprint(owner.Err())})
	f.snapshot("COMP_B_actual_predecessor_lease_lost")
	require.NoError(t, f.replay(f.manager))
	waitFixTickReleased(t, f, f.r1)
	f.snapshot("COMP_R1_legitimate_replay_releases_B_residual_admission")
	require.NoError(t, f.manager.LogRun(f.ctx, f.expt, c, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{}))
	require.NoError(t, NewQuotaService(f.quota, f.manager.configer).AllowExptRun(f.ctx, f.expt, f.space, &entity.Session{}))
	require.Equal(t, "evaluation", f.admission())
	require.True(t, f.slot())
	f.snapshot("COMP_C_actual_Create_Latest_Allow_before_late_B_Eval")
	release()
	require.ErrorIs(t, independentAuditResult(t, done), failure)
	f.snapshot("COMP_after_late_B_compensation_Eval")
	current, err := f.manager.exptRepo.GetByID(f.ctx, f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, c, current.LatestRunID)
	run, err := f.manager.runLogRepo.Get(f.ctx, f.expt, c)
	require.NoError(t, err)
	require.Equal(t, int64(entity.ExptStatus_Pending), run.Status)
	var count int64
	require.NoError(t, f.conn.Raw("SELECT COUNT(*) FROM expt_run_log WHERE expt_id=? AND expt_run_id=?", f.expt, f.r2).Scan(&count).Error)
	require.Zero(t, count)
	require.True(t, f.slot())
	require.Equal(t, "evaluation", f.admission(), "BUSINESS_RED RR-F1-1 compensation: late Background Eval cannot delete C admission")
}

func TestResidualF1CompensationCustomHolder(t *testing.T) {
	f := newIndependentAuditFixture(t)
	f.finishR1WithoutCID()
	m := *f.manager
	m.mutex = lock.NewRedisLockerWithHolder(f.client, "custom-held-compensation")
	failure := errors.New("custom holder Create failure")
	m.runLogRepo = &fixCreateFailureRepo{IExptRunLogRepo: m.runLogRepo, failure: failure}
	err := m.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{})
	require.ErrorIs(t, err, failure)
	require.Equal(t, "<absent>", f.admission())
	require.False(t, f.slot())
	f.snapshot("COMP_custom_holder_released_by_same_injected_locker")
}

type inferredCompensationObservedRepo struct {
	repo.IExptRunLogRepo
	owner chan context.Context
}

func (r *inferredCompensationObservedRepo) Update(ctx context.Context, expt, run, space int64, fields map[string]any) error {
	err := r.IExptRunLogRepo.Update(ctx, expt, run, space, fields)
	if err == nil && fields["status"] == int64(entity.ExptStatus_Failed) {
		owner, _ := ctx.Value(completeTickOwnerKey{}).(context.Context)
		r.owner <- owner
	}
	return err
}

func TestResidualF1InferredPendingCompensationEvalPreservesNewRun(t *testing.T) {
	f := newIndependentAuditFixture(t)
	f.seedSources(1, true)
	require.NoError(t, f.manager.runLogRepo.Update(f.ctx, f.expt, f.r1, f.space, map[string]any{"status": int64(entity.ExptStatus_Pending)}))
	end := time.Now().Add(-time.Minute).Truncate(time.Second)
	require.NoError(t, f.manager.exptRepo.Update(f.ctx, &entity.Experiment{ID: f.expt, SpaceID: f.space, Status: entity.ExptStatus_Success, EndAt: &end}))
	before := f.manager.runFinishRepo.(repo.IExptRunTerminationRepo)
	source, err := before.GetTerminationItemSource(f.ctx, f.space, f.expt, f.r1, f.base+100)
	require.NoError(t, err)
	f.snapshot("INFERRED_R15_initial_Success_experiment_Pending_run")
	gate := &residualLifecycleEvalClient{Cmdable: f.client, f: f, key: f.manager.makeExptMutexLockKey(f.expt), entered: make(chan struct{}), proceed: make(chan struct{})}
	old := *f.manager
	old.mutex = lock.NewRedisLockerWithHolder(gate, "evaluation")
	observed := &inferredCompensationObservedRepo{IExptRunLogRepo: old.runLogRepo, owner: make(chan context.Context, 1)}
	old.runLogRepo = observed
	var once sync.Once
	release := func() { once.Do(func() { close(gate.proceed) }) }
	defer release()
	done := make(chan error, 1)
	// This calls the actual special Pending compensation with quotaReserved=true
	// and NO captured ordinary/numeric admission option.
	go func() { done <- old.CompleteRun(f.ctx, f.expt, f.r1, f.space, &entity.Session{}) }()
	independentAuditWait(t, gate.entered)
	owner := <-observed.owner
	require.NotNil(t, owner)
	require.NoError(t, owner.Err())
	oldRun, err := f.manager.runLogRepo.Get(f.ctx, f.expt, f.r1)
	require.NoError(t, err)
	require.Equal(t, int64(entity.ExptStatus_Failed), oldRun.Status)
	current, err := f.manager.exptRepo.GetByID(f.ctx, f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, entity.ExptStatus_Success, current.Status)
	require.Equal(t, end, *current.EndAt)
	unchanged, err := before.GetTerminationItemSource(f.ctx, f.space, f.expt, f.r1, f.base+100)
	require.NoError(t, err)
	require.Equal(t, source, unchanged)
	f.snapshot("INFERRED_R15_actual_Pending_to_Failed_before_final_Eval")
	require.NoError(t, f.raw.Del(f.ctx, fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.r1)).Err())
	select {
	case <-owner.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("inferred Pending compensation actual owner did not cancel")
	}
	waitFixTickReleased(t, f, f.r1)
	f.log("INFERRED_actual_owner_cancelled", map[string]any{"Err": fmt.Sprint(owner.Err())})
	require.NoError(t, f.replay(f.manager))
	waitFixTickReleased(t, f, f.r1)
	require.NoError(t, f.manager.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeRetryAll, f.space, nil, &entity.Session{}))
	require.NoError(t, NewQuotaService(f.quota, f.manager.configer).AllowExptRun(f.ctx, f.expt, f.space, &entity.Session{}))
	require.Equal(t, "evaluation", f.admission())
	require.True(t, f.slot())
	f.snapshot("INFERRED_new_run_actual_Create_Latest_Allow")
	release()
	oldErr := independentAuditResult(t, done)
	f.log("INFERRED_old_CompleteRun_result", map[string]any{"Err": fmt.Sprint(oldErr)})
	f.snapshot("INFERRED_after_late_Pending_compensation_Eval")
	current, err = f.manager.exptRepo.GetByID(f.ctx, f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, f.r2, current.LatestRunID)
	require.Equal(t, entity.ExptStatus_Success, current.Status)
	require.Equal(t, end, *current.EndAt)
	unchanged, err = before.GetTerminationItemSource(f.ctx, f.space, f.expt, f.r1, f.base+100)
	require.NoError(t, err)
	require.Equal(t, source, unchanged)
	newRun, err := f.manager.runLogRepo.Get(f.ctx, f.expt, f.r2)
	require.NoError(t, err)
	require.Equal(t, int64(entity.ExptStatus_Pending), newRun.Status)
	require.Equal(t, int32(entity.EvaluationModeRetryAll), newRun.Mode)
	require.True(t, f.slot())
	require.Equal(t, "evaluation", f.admission(), "BUSINESS_RED inferred R15: late ordinary Eval must preserve new run admission")
}
