// Copyright (c) 2026 coze-dev Authors
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

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	red "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// Delay cancellation delivery, not the lock implementation's compare-delete semantics.
type delayedHookCancelLocker struct {
	lock.ILocker
	cancels chan context.CancelFunc
}

func (l *delayedHookCancelLocker) LockWithRenew(ctx context.Context, key string, ttl, max time.Duration) (bool, context.Context, func(), error) {
	ok, held, cancel, err := l.ILocker.LockWithRenew(ctx, key, ttl, max)
	return ok, held, func() { l.cancels <- cancel }, err
}

type hookOldUnlockProbe struct {
	redis.Cmdable
	armed        atomic.Bool
	old          string
	done         chan struct{}
	once         sync.Once
	unlockEvents chan struct{}
}

func (p *hookOldUnlockProbe) Eval(ctx context.Context, script string, keys []string, args ...any) *red.Cmd {
	cmd := p.Cmdable.Eval(ctx, script, keys, args...)
	if strings.Contains(script, "'DEL'") {
		p.unlockEvents <- struct{}{}
	}
	if p.armed.Load() && strings.Contains(script, "'DEL'") && len(args) > 0 && args[0] == p.old {
		p.once.Do(func() { close(p.done) })
	}
	return cmd
}

type hookArchiveLeaseProbe struct {
	repo.IHookActiveTerminationRepo
	before func()
}

func (p hookArchiveLeaseProbe) ArchiveHookItem(ctx context.Context, in entity.HookItemArchiveInput) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	p.before()
	return p.IHookActiveTerminationRepo.ArchiveHookItem(ctx, in)
}

func TestHookActiveTerminationOldRenewalCannotDeleteNewHolder(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, ms[0].Frozen.ItemID)
	p := &hookOldUnlockProbe{Cmdable: f.redis, done: make(chan struct{}), unlockEvents: make(chan struct{}, 8)}
	cancels := make(chan context.CancelFunc, 4)
	// Pre-fix cleanup reuses this locker; the approved factory deliberately produces fresh instances.
	f.manager.mutex = &delayedHookCancelLocker{ILocker: lock.NewRedisLocker(p), cancels: cancels}
	f.manager.finalization.NewItemLocker = func() lock.ILocker {
		return &delayedHookCancelLocker{ILocker: lock.NewRedisLocker(p), cancels: cancels}
	}
	ctx, cancelAll := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelAll()
	require.NoError(t, f.manager.SetExptTerminating(ctx, f.expt, f.key.RunID, f.space, nil))
	source, err := f.deps.Repository.ReadFinalizationSource(ctx, f.key)
	require.NoError(t, err)
	storage := f.deps.Repository.(repo.IHookActiveTerminationRepo)
	first := hookArchiveLeaseProbe{IHookActiveTerminationRepo: storage, before: func() { p.old = f.redis.Get(ctx, key).Val() }}
	require.NoError(t, f.manager.prepareActiveHookItem(ctx, first, source, ms[0].Frozen.ItemID))
	oldCancel := <-cancels
	defer oldCancel()
	var secondOwner string
	second := hookArchiveLeaseProbe{IHookActiveTerminationRepo: storage, before: func() {
		secondOwner = f.redis.Get(ctx, key).Val()
		p.armed.Store(true)
		oldCancel()
		select {
		case <-p.done:
		case <-ctx.Done():
			t.Fatal("old renewal did not finish")
		}
		require.Equal(t, secondOwner, f.redis.Get(ctx, key).Val(), "delayed renewal cleanup deleted the reacquired item lease")
		require.NotEqual(t, p.old, secondOwner, "every acquisition needs its own holder")
	}}
	require.NoError(t, f.manager.prepareActiveHookItem(ctx, second, source, ms[0].Frozen.ItemID))
	(<-cancels)()
	for i := 0; i < 4; i++ {
		select {
		case <-p.unlockEvents:
		case <-ctx.Done():
			t.Fatal("cleanup renewal still active")
		}
	}
}
