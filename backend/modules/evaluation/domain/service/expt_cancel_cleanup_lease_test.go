// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	idemrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem"
	idemredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func cleanupLeaseRealRedis(t *testing.T, f *filterSyncFixture, m *ExptMangerImpl) (infraredis.Cmdable, *redis.Client) {
	t.Helper()
	addr := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR")
	require.NotEmpty(t, addr, "real Redis must be configured; this regression cannot skip")
	client, err := infraredis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, err)
	raw, ok := infraredis.Unwrap(client)
	require.True(t, ok)
	info, err := raw.Info(context.Background(), "server").Result()
	require.NoError(t, err)
	require.Contains(t, info, "redis_version:8.2.0")
	t.Log("REAL_REDIS_EXECUTED version=8.2.0")
	m.manualRetryLocker = func() lock.ILocker { return filterRealTestLocker(client) }
	m.mutex = m.manualRetryLocker()
	m.idem = idemrepo.NewIdempotentService(idemredis.NewIdemDAO(client))
	f.svc.resultPublishLocker = m.manualRetryLocker
	keys := []string{m.makeExptMutexLockKey(f.expt), fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.run), f.key(), fmt.Sprintf("[idempotent]idem_key:finish-cleanup:%d:%d", f.expt, f.run)}
	require.NoError(t, raw.Set(context.Background(), keys[0], fmt.Sprint(f.run), 36*time.Hour).Err())
	t.Cleanup(func() {
		_, err := raw.Del(context.Background(), keys...).Result()
		require.NoError(t, err)
		require.NoError(t, raw.Close())
	})
	return client, raw
}

type cleanupLeaseFilterSink struct {
	repo.IExptTurnResultFilterRepo
	wait                  time.Duration
	raw                   *redis.Client
	lifecycleKey, itemKey string
	rows                  []*entity.ExptTurnResultFilterEntity
	observedCancel        error
	before                func(context.Context)
}

func (*cleanupLeaseFilterSink) GetExptTurnResultFilterKeyMappings(context.Context, int64, int64) ([]*entity.ExptTurnResultFilterKeyMapping, error) {
	return nil, nil
}

func (s *cleanupLeaseFilterSink) Save(ctx context.Context, rows []*entity.ExptTurnResultFilterEntity) error {
	if s.before != nil {
		s.before(ctx)
	}
	timer := time.NewTimer(s.wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		s.observedCancel = ctx.Err()
		return ctx.Err()
	case <-timer.C:
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, key := range []string{s.lifecycleKey, s.itemKey} {
		if ok, err := s.raw.Exists(ctx, key).Result(); err != nil || ok != 1 {
			return fmt.Errorf("cleanup lease missing at sink: %s: %v", key, err)
		}
	}
	s.rows = rows
	return nil
}

func TestCancelCleanupLeaseSlowFilterRealRedisMySQL(t *testing.T) {
	f, m := newCancelFilterFixture(t)
	_, raw := cleanupLeaseRealRedis(t, f, m)
	sink := &cleanupLeaseFilterSink{wait: 9 * time.Second, raw: raw, lifecycleKey: m.makeExptMutexLockKey(f.expt), itemKey: f.key()}
	f.svc.exptTurnResultFilterRepo = sink
	start := time.Now()
	_, err := m.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
	t.Logf("SLOW_FILTER_REAL_LEASE elapsed=%s sink_cancel=%v rows=%d", time.Since(start), sink.observedCancel, len(sink.rows))
	require.NoError(t, err, "a legitimate 9s filter must finish under the lifecycle lease")
	require.GreaterOrEqual(t, time.Since(start), 9*time.Second)
	require.Len(t, sink.rows, 1)
	require.Equal(t, entity.ItemRunState_Terminal, sink.rows[0].Status)
	marked, err := m.idem.Exist(context.Background(), fmt.Sprintf("finish-cleanup:%d:%d", f.expt, f.run))
	require.NoError(t, err)
	require.True(t, marked)
	before := f.snapshot()
	sink.wait = 0
	_, err = m.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
	require.NoError(t, err)
	require.Equal(t, before, f.snapshot(), "cleanup replay preserves terminal stats and end time")
}

type cleanupLeaseSlowGuard struct {
	component.ICentralReservationGuard
	raw       *redis.Client
	wait      time.Duration
	keys      map[int64]string
	expected  map[int64]string
	completed []int64
	budgets   []time.Duration
	before    func(context.Context, int64)
	faultOnce error
}

func (g *cleanupLeaseSlowGuard) Release(ctx context.Context, _ string, run, item int64, token, _ string) error {
	if g.before != nil {
		g.before(ctx, item)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return fmt.Errorf("resource IO lacks bounded context")
	}
	g.budgets = append(g.budgets, time.Until(deadline))
	if token != g.expected[item] {
		return fmt.Errorf("resource token changed")
	}
	if g.faultOnce != nil {
		err := g.faultOnce
		g.faultOnce = nil
		return err
	}
	timer := time.NewTimer(g.wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	// The external sink retains the same original-token compare/delete contract.
	const script = `if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]); end; return 0;`
	if _, err := g.raw.Eval(ctx, script, []string{g.keys[item]}, token).Result(); err != nil {
		return err
	}
	g.completed = append(g.completed, item)
	return nil
}

func TestCancelCleanupLeaseCancelDuringResourceIOPreservesBindings(t *testing.T) {
	f, m := newCancelFilterFixture(t)
	_, raw := cleanupLeaseRealRedis(t, f, m)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	guard := &cleanupLeaseSlowGuard{ICentralReservationGuard: component.NewNoopCentralReservationGuard(), raw: raw, keys: map[int64]string{f.item: fmt.Sprintf("cleanup-lease-active-cancel:%d", f.run)}, expected: map[int64]string{f.item: "rq1"}, faultOnce: context.Canceled}
	first := true
	guard.before = func(ioCtx context.Context, item int64) {
		require.NoError(t, ioCtx.Err())
		if first {
			first = false
			cancel()
		}
	}
	require.NoError(t, raw.Set(context.Background(), guard.keys[f.item], "rq1", time.Minute).Err())
	t.Cleanup(func() { _ = raw.Del(context.Background(), guard.keys[f.item]).Err() })
	m.centralGuard = guard
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)
	queries := 0
	require.NoError(t, f.conn.Callback().Query().Before("gorm:query").Register("cleanup_cancel_during_io", func(tx *gorm.DB) {
		if ctx.Err() != nil {
			queries++
		}
	}))
	store := &filterStore{}
	f.svc.exptTurnResultFilterRepo = store
	target := svcmocks.NewMockIEvalTargetService(gomock.NewController(t))
	target.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), f.space, []int64{f.base + 41}, gomock.Any(), gomock.Any(), false).Do(func(ioCtx context.Context, _ int64, ids []int64, _ int32, _ string, _ bool) {
		require.NoError(t, ioCtx.Err())
		deadline, ok := ioCtx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 8*time.Second)
		require.Equal(t, []int64{f.base + 41}, ids)
	})
	m.evalTargetService = target
	_, err := m.TerminateCurrentRun(ctx, f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
	require.True(t, errno.NeedMQRetry(err))
	require.Len(t, guard.completed, 1)
	require.Zero(t, queries)
	require.Empty(t, store.rows)
	marked, err := m.idem.Exist(context.Background(), fmt.Sprintf("finish-cleanup:%d:%d", f.expt, f.run))
	require.NoError(t, err)
	require.False(t, marked)
}

func TestCancelCleanupLeaseCancelDuringFilterStopsNotification(t *testing.T) {
	f, m := newCancelFilterFixture(t)
	_, raw := cleanupLeaseRealRedis(t, f, m)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sink := &cleanupLeaseFilterSink{wait: 9 * time.Second, raw: raw, lifecycleKey: m.makeExptMutexLockKey(f.expt), itemKey: f.key(), before: func(context.Context) { cancel() }}
	f.svc.exptTurnResultFilterRepo = sink
	_, err := m.TerminateCurrentRun(ctx, f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
	require.True(t, errno.NeedMQRetry(err))
	require.Empty(t, sink.rows)
	require.ErrorIs(t, sink.observedCancel, context.Canceled)
	marked, err := m.idem.Exist(context.Background(), fmt.Sprintf("finish-cleanup:%d:%d", f.expt, f.run))
	require.NoError(t, err)
	require.False(t, marked)
}

type cleanupLeaseLostLocker struct{ lock.ILocker }

func (l *cleanupLeaseLostLocker) LockWithRenew(ctx context.Context, key string, ttl, hold time.Duration) (bool, context.Context, func(), error) {
	if strings.HasPrefix(key, "expt_run_mutex_lock:") {
		child, cancel := context.WithCancel(ctx)
		cancel()
		return true, child, func() {}, nil
	}
	return l.ILocker.LockWithRenew(ctx, key, ttl, hold)
}

func TestCancelCleanupLeaseLossIsNotParentCancelResourcePermission(t *testing.T) {
	f, m := newCancelFilterFixture(t)
	client, _ := cleanupLeaseRealRedis(t, f, m)
	m.manualRetryLocker = func() lock.ILocker { return &cleanupLeaseLostLocker{ILocker: filterRealTestLocker(client)} }
	store := &filterStore{}
	f.svc.exptTurnResultFilterRepo = store
	_, err := m.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
	require.True(t, errno.NeedMQRetry(err))
	require.Empty(t, store.rows)
	marked, err := m.idem.Exist(context.Background(), fmt.Sprintf("finish-cleanup:%d:%d", f.expt, f.run))
	require.NoError(t, err)
	require.False(t, marked)
}

func TestCancelCleanupLeaseCancelledCommitOnlyFrozenResources(t *testing.T) {
	f, m := newCancelFilterFixture(t)
	_, raw := cleanupLeaseRealRedis(t, f, m)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queriesAfterCancel := 0
	require.NoError(t, f.conn.Callback().Query().Before("gorm:query").Register("cleanup_no_detached_sql", func(tx *gorm.DB) {
		if ctx.Err() != nil {
			queriesAfterCancel++
		}
	}))
	guard := &cleanupLeaseSlowGuard{ICentralReservationGuard: component.NewNoopCentralReservationGuard(), raw: raw, keys: map[int64]string{f.item: fmt.Sprintf("cleanup-lease-cancel:%d", f.run)}, expected: map[int64]string{f.item: "rq1"}}
	guard.before = func(ioCtx context.Context, item int64) {
		require.NoError(t, ioCtx.Err())
		require.Equal(t, f.item, item)
	}
	m.centralGuard = guard
	require.NoError(t, raw.Set(context.Background(), guard.keys[f.item], "rq1", time.Minute).Err())
	t.Cleanup(func() { _ = raw.Del(context.Background(), guard.keys[f.item]).Err() })
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)
	store := &filterStore{}
	f.svc.exptTurnResultFilterRepo = store
	m.runFinishRepo = &cancelFilterCommitHook{IExptRunFinishRepo: m.runFinishRepo, port: m.runFinishRepo.(repo.IExptRunTerminationRepo), after: cancel}
	_, err := m.TerminateCurrentRun(ctx, f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
	require.True(t, errno.NeedMQRetry(err))
	require.Len(t, guard.completed, 1)
	require.Zero(t, queriesAfterCancel, "cancelled commit cleanup must use only its frozen source")
	require.Empty(t, store.rows)
	marked, err := m.idem.Exist(context.Background(), fmt.Sprintf("finish-cleanup:%d:%d", f.expt, f.run))
	require.NoError(t, err)
	require.False(t, marked)
}

func TestCancelCleanupLeaseReplacementOwnerHasZeroActions(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
			f, m := newCancelFilterFixture(t)
			_, raw := cleanupLeaseRealRedis(t, f, m)
			source := f.source()
			finished, err := m.exptRepo.GetByID(context.Background(), f.expt, f.space)
			require.NoError(t, err)
			finished.Status = entity.ExptStatus_Terminated
			require.NoError(t, raw.Set(context.Background(), m.makeExptMutexLockKey(f.expt), fmt.Sprint(f.run+100), time.Minute).Err())
			guard := &fakeGuard{}
			m.centralGuard = guard
			store := &filterStore{}
			f.svc.exptTurnResultFilterRepo = store
			before := f.snapshot()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelled {
				cancel()
			}
			err = m.cleanupTerminatedRun(ctx, &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, Session: &entity.Session{}}, finished, []*entity.ExptCurrentItemRunSource{source}, &entity.CompleteExptOption{})
			require.True(t, errno.NeedMQRetry(err))
			require.Equal(t, fmt.Sprint(f.run+100), raw.Get(context.Background(), m.makeExptMutexLockKey(f.expt)).Val())
			require.Equal(t, before, f.snapshot())
			require.Empty(t, guard.releases())
			require.Empty(t, store.rows)
			marked, err := m.idem.Exist(context.Background(), fmt.Sprintf("finish-cleanup:%d:%d", f.expt, f.run))
			require.NoError(t, err)
			require.False(t, marked)
		})
	}
}

func TestCancelCleanupLeaseWholeResourcesPastEightSeconds(t *testing.T) {
	f, m := newCancelFilterFixture(t)
	_, raw := cleanupLeaseRealRedis(t, f, m)
	guard := &cleanupLeaseSlowGuard{ICentralReservationGuard: component.NewNoopCentralReservationGuard(), raw: raw, wait: 3 * time.Second, keys: map[int64]string{}, expected: map[int64]string{}}
	sources := []*entity.ExptCurrentItemRunSource{}
	for i := int64(0); i < 3; i++ {
		item := f.item + i
		key, token := fmt.Sprintf("cleanup-lease-test:%d:%d", f.run, item), fmt.Sprintf("rq%d", i)
		guard.keys[item], guard.expected[item] = key, token
		require.NoError(t, raw.Set(context.Background(), key, token, time.Minute).Err())
		sources = append(sources, &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, ItemID: item, RequestID: token, Status: entity.ItemRunState_Terminal}})
	}
	t.Cleanup(func() {
		for _, key := range guard.keys {
			_ = raw.Del(context.Background(), key).Err()
		}
	})
	m.centralGuard = guard
	start := time.Now()
	err := m.releaseTerminationResources(context.Background(), &entity.Experiment{ID: f.expt, SpaceID: f.space, SchedulerScope: testScope, ExptDispatchMode: entity.ExptDispatchModeEnforce}, f.run, sources, true, &entity.Session{})
	t.Logf("WHOLE_RESOURCES elapsed=%s completed=%d bounded_io=%v", time.Since(start), len(guard.completed), guard.budgets)
	require.NoError(t, err, "three valid bounded releases must reach the tail after 9s")
	require.Len(t, guard.completed, 3)
	for _, budget := range guard.budgets {
		require.Positive(t, budget)
		require.LessOrEqual(t, budget, 8*time.Second)
	}
	for _, key := range guard.keys {
		require.Equal(t, int64(0), raw.Exists(context.Background(), key).Val())
	}
}
