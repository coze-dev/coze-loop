// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	entitymocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// tickShortLeaseLocker keeps the compare-and-delete holder semantics but hands out a short
// lease without renewal, so the test can move the key to the next tick while the previous
// tick is still running.
type tickShortLeaseLocker struct {
	lock.ILocker
	ttl time.Duration
}

func (l *tickShortLeaseLocker) LockBackoffWithRenew(_ context.Context, key string, _, _ time.Duration) (bool, context.Context, func(), error) {
	locked, err := l.ILocker.LockBackoff(context.Background(), key, l.ttl, time.Second)
	if err != nil || !locked {
		return locked, context.Background(), func() {}, err
	}
	return true, context.Background(), func() {}, nil
}

// After a TTL handoff the expired tick's deferred Unlock must only delete its own lease.
// A shared holder would compare equal to the replacement owner and erase the live exec lock,
// letting a third tick run next concurrently with the replacement.
func TestSchedulerTickLateUnlockKeepsReplacementExecLock(t *testing.T) {
	addr := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR")
	if addr == "" {
		t.Skip("EXPT_RETRY_REAL_REDIS_ADDR missing; REAL REDIS NOT EXECUTED")
	}
	client, err := infraredis.NewClient(&goredis.Options{Addr: addr})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { _ = raw.Close() })
	t.Logf("REAL_REDIS_EXECUTED addr=%s case=scheduler_tick_lock_ttl_handoff", addr)

	ctrl := gomock.NewController(t)
	factory := svcmocks.NewMockSchedulerModeFactory(ctrl)
	mode := entitymocks.NewMockExptSchedulerMode(ctrl)
	factory.EXPECT().NewSchedulerMode(gomock.Any()).Return(mode, nil).AnyTimes()
	// The third tick finds the replacement lease and reschedules instead of running next.
	mode.EXPECT().NextTick(gomock.Any(), gomock.Any(), true).Return(nil).Times(1)

	// The first tick holds a 1s lease so the key can be handed off; later ticks keep a
	// long lease so the replacement owner survives the expired tick's late Unlock.
	var factoryCalls atomic.Int32
	shared := &tickShortLeaseLocker{ILocker: lock.NewRedisLocker(client), ttl: time.Second}
	scheduler := &ExptSchedulerImpl{
		Mutex: shared,
		itemLocker: func() lock.ILocker {
			ttl := 5 * time.Second
			if factoryCalls.Add(1) == 1 {
				ttl = time.Second
			}
			return &tickShortLeaseLocker{ILocker: lock.NewRedisLocker(client), ttl: ttl}
		},
		schedulerModeFactory: factory,
	}

	base := time.Now().UnixNano()
	event := &entity.ExptScheduleEvent{
		ExptID: base, ExptRunID: base + 1, SpaceID: base + 2,
		ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeSubmit,
	}
	key := scheduler.makeExptRunExecLockKey(event.ExptID, event.ExptRunID)
	cleanup := lock.NewRedisLocker(client)
	t.Cleanup(func() { _, _ = cleanup.UnlockForce(context.Background(), key) })

	var nextRuns atomic.Int32
	entered1, release1 := make(chan struct{}), make(chan struct{})
	entered2, release2 := make(chan struct{}), make(chan struct{})

	blockingNext := func(entered, release chan struct{}) SchedulerEndPoint {
		return func(context.Context, *entity.ExptScheduleEvent) error {
			nextRuns.Add(1)
			close(entered)
			<-release
			return nil
		}
	}

	done1 := make(chan error, 1)
	go func() {
		done1 <- scheduler.HandleEventLock(blockingNext(entered1, release1))(context.Background(), event)
	}()
	select {
	case <-entered1:
	case <-time.After(5 * time.Second):
		t.Fatal("first tick never acquired the exec lock")
	}

	// The first tick's lease expires while it is still executing.
	require.Eventually(t, func() bool {
		exists, existsErr := cleanup.Exists(context.Background(), key)
		return existsErr == nil && !exists
	}, 5*time.Second, 20*time.Millisecond, "first tick lease never expired")

	done2 := make(chan error, 1)
	go func() {
		done2 <- scheduler.HandleEventLock(blockingNext(entered2, release2))(context.Background(), event)
	}()
	select {
	case <-entered2:
	case <-time.After(5 * time.Second):
		t.Fatal("replacement tick never acquired the handed-off exec lock")
	}

	// Let the expired tick finish: its deferred Unlock runs against the replacement lease.
	close(release1)
	require.NoError(t, <-done1)

	// A third tick must not run next while the replacement still holds the lease.
	thirdRuns := atomic.Int32{}
	done3 := make(chan error, 1)
	go func() {
		done3 <- scheduler.HandleEventLock(func(context.Context, *entity.ExptScheduleEvent) error {
			thirdRuns.Add(1)
			return nil
		})(context.Background(), event)
	}()
	require.NoError(t, <-done3)

	exists, err := cleanup.Exists(context.Background(), key)
	require.NoError(t, err)
	require.True(t, exists, "late unlock from the expired tick deleted the replacement exec lock")
	require.Zero(t, thirdRuns.Load(), "a third tick ran next concurrently after the replacement lock was deleted")
	require.Equal(t, int32(2), nextRuns.Load(), "only the expired tick and the replacement tick may run next")

	close(release2)
	require.NoError(t, <-done2)
}
