// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// Delay the actual Redis CAS from A's renewal goroutine, after cancellation and
// before it executes. The scheduler's synchronous second CAS can still release
// A, allowing B to acquire while A's first CAS remains in flight.
type tickOwnerReleaseGate struct {
	started atomic.Bool
	entered chan struct{}
	resume  chan struct{}
	done    chan struct{}
	once    sync.Once
}

func (g *tickOwnerReleaseGate) unblock() { g.once.Do(func() { close(g.resume) }) }

type tickOwnerRedis struct {
	infraredis.Cmdable
	gate *tickOwnerReleaseGate
}

func (r *tickOwnerRedis) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	if r.gate != nil && strings.Contains(script, "redis.call('DEL'") && r.gate.started.CompareAndSwap(false, true) {
		close(r.gate.entered)
		<-r.gate.resume
		result := r.Cmdable.Eval(ctx, script, keys, args...)
		close(r.gate.done)
		return result
	}
	return r.Cmdable.Eval(ctx, script, keys, args...)
}

type tickOwnerLocker struct {
	lock.ILocker
	gate *tickOwnerReleaseGate
}

func (l *tickOwnerLocker) LockBackoffWithRenew(ctx context.Context, key string, ttl, hold time.Duration) (bool, context.Context, func(), error) {
	locked, child, cancel, err := l.ILocker.LockBackoffWithRenew(ctx, key, ttl, hold)
	if !locked || err != nil || l.gate == nil {
		return locked, child, cancel, err
	}
	return locked, child, func() {
		cancel()
		select {
		case <-l.gate.entered:
		case <-time.After(2 * time.Second):
		}
	}, nil
}

func newTickOwnerFixture(t *testing.T) (infraredis.Cmdable, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { _ = raw.Close() })
	return client, server
}

func newTickOwnerScheduler(mutex lock.ILocker, factory ManualRetryLockerFactory) *ExptSchedulerImpl {
	// Exercise the production constructor's existing factory wiring.
	return NewExptSchedulerSvc(
		&ExptMangerImpl{manualRetryLocker: factory},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, mutex,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	).(*ExptSchedulerImpl)
}

func TestSchedulerTickOwnerDelayedReleaseKeepsSuccessor(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("prior_legacy_%v", legacy), func(t *testing.T) {
			testSchedulerTickOwnerDelayedRelease(t, legacy)
		})
	}
}

func testSchedulerTickOwnerDelayedRelease(t *testing.T, priorLegacy bool) {
	client, server := newTickOwnerFixture(t)
	gate := &tickOwnerReleaseGate{entered: make(chan struct{}), resume: make(chan struct{}), done: make(chan struct{})}
	t.Cleanup(gate.unblock)
	makeLocker := func(holder string, delayed bool) lock.ILocker {
		if delayed {
			return &tickOwnerLocker{ILocker: lock.NewRedisLockerWithHolder(&tickOwnerRedis{Cmdable: client, gate: gate}, holder), gate: gate}
		}
		return lock.NewRedisLockerWithHolder(client, holder)
	}
	var calls atomic.Int32
	factory := func() lock.ILocker {
		return makeLocker(lock.LifecycleCleanupHolderPrefix+xid.New().String(), calls.Add(1) == 1)
	}
	scheduler := newTickOwnerScheduler(makeLocker("evaluation", true), factory)
	prior := scheduler
	if priorLegacy {
		prior = &ExptSchedulerImpl{Mutex: makeLocker("evaluation", true)}
	}
	event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2}
	key := scheduler.makeExptRunExecLockKey(event.ExptID, event.ExptRunID)
	var firstOwner string
	require.NoError(t, prior.HandleEventLock(func(context.Context, *entity.ExptScheduleEvent) error {
		var err error
		firstOwner, err = server.Get(key)
		return err
	})(context.Background(), event))
	require.False(t, server.Exists(key), "the synchronous release must finish before B starts")
	select {
	case <-gate.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("A's cancelled renewal did not reach the Redis CAS barrier")
	}
	bEntered, bResume := make(chan string, 1), make(chan struct{})
	var resumeOnce sync.Once
	unblockB := func() { resumeOnce.Do(func() { close(bResume) }) }
	t.Cleanup(unblockB)
	bDone := make(chan error, 1)
	go func() {
		bDone <- scheduler.HandleEventLock(func(context.Context, *entity.ExptScheduleEvent) error {
			owner, err := server.Get(key)
			if err != nil {
				return err
			}
			bEntered <- owner
			<-bResume
			return nil
		})(context.Background(), event)
	}()
	var secondOwner string
	select {
	case secondOwner = <-bEntered:
	case err := <-bDone:
		t.Fatalf("B failed before entering: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("B did not acquire after A's synchronous release")
	}
	gate.unblock()
	select {
	case <-gate.done:
	case <-time.After(2 * time.Second):
		t.Fatal("A's delayed Redis CAS did not complete")
	}
	owner, err := server.Get(key)
	require.NoError(t, err, "A's delayed release must preserve B's new lease")
	require.Equal(t, secondOwner, owner)
	require.NotEqual(t, firstOwner, secondOwner, "each tick must acquire with its own holder")
	if priorLegacy {
		require.Equal(t, int32(1), calls.Load())
	} else {
		require.Equal(t, int32(2), calls.Load())
	}
	locked, err := lock.NewRedisLocker(client).Lock(context.Background(), key, 5*time.Second)
	require.NoError(t, err)
	require.False(t, locked, "a competing tick must remain excluded while B executes")
	unblockB()
	require.NoError(t, <-bDone)
	deadline := time.Now().Add(2 * time.Second)
	for server.Exists(key) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	require.False(t, server.Exists(key))
}

func TestSchedulerTickOwnerExpiryCancellationAndLegacyAlternation(t *testing.T) {
	for _, stage := range []string{"expiry", "cancellation", "legacy_before", "legacy_after"} {
		t.Run(stage, func(t *testing.T) {
			client, server := newTickOwnerFixture(t)
			factory := func() lock.ILocker {
				return lock.NewRedisLockerWithHolder(client, lock.LifecycleCleanupHolderPrefix+xid.New().String())
			}
			legacy := lock.NewRedisLockerWithHolder(client, "evaluation")
			scheduler := newTickOwnerScheduler(legacy, factory)
			event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2}
			key := scheduler.makeExptRunExecLockKey(1, 2)
			if stage == "legacy_before" {
				locked, err := legacy.Lock(context.Background(), key, 5*time.Second)
				require.NoError(t, err)
				require.True(t, locked)
				server.FastForward(6 * time.Second)
			}
			var currentOwner string
			parent, cancelParent := context.WithCancel(context.Background())
			defer cancelParent()
			require.NoError(t, scheduler.HandleEventLock(func(ctx context.Context, _ *entity.ExptScheduleEvent) error {
				var err error
				currentOwner, err = server.Get(key)
				if err != nil {
					return err
				}
				if stage == "legacy_before" {
					released, err := legacy.Unlock(key)
					if err != nil || released {
						return fmt.Errorf("old evaluation holder released new tick: released=%v err=%v", released, err)
					}
				}
				if stage == "expiry" {
					server.FastForward(6 * time.Second)
					locked, err := legacy.Lock(ctx, key, 5*time.Second)
					if err != nil || !locked {
						return fmt.Errorf("legacy replacement failed: locked=%v err=%v", locked, err)
					}
				}
				if stage == "cancellation" {
					cancelParent()
					if ctx.Err() == nil {
						return fmt.Errorf("tick context did not observe parent cancellation")
					}
				}
				return nil
			})(parent, event))
			require.NotEqual(t, "evaluation", currentOwner)
			if stage == "expiry" {
				owner, err := server.Get(key)
				require.NoError(t, err)
				require.Equal(t, "evaluation", owner, "expired tick cleanup cannot delete the replacement legacy lease")
				_, err = legacy.Unlock(key)
				require.NoError(t, err)
			}
			if stage == "legacy_after" {
				locked, err := legacy.Lock(context.Background(), key, 5*time.Second)
				require.NoError(t, err)
				require.True(t, locked)
				_, err = legacy.Unlock(key)
				require.NoError(t, err)
			}
			var nextOwner string
			require.NoError(t, scheduler.HandleEventLock(func(context.Context, *entity.ExptScheduleEvent) error {
				var err error
				nextOwner, err = server.Get(key)
				return err
			})(context.Background(), event))
			require.NotEqual(t, currentOwner, nextOwner)
			require.NotEqual(t, "evaluation", nextOwner)
		})
	}
}

func TestSchedulerTickOwnerFactoryFailureKeepsEvent(t *testing.T) {
	scheduler := &ExptSchedulerImpl{itemLocker: func() lock.ILocker { return nil }}
	err := scheduler.HandleEventLock(func(context.Context, *entity.ExptScheduleEvent) error {
		t.Fatal("missing factory lease must not enter scheduling")
		return nil
	})(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2})
	require.True(t, errno.NeedMQRetry(err))
}
