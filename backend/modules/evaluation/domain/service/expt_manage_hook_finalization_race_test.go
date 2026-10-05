// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

type finalizationUnlockProbe struct {
	lock.ILocker
	fail         bool
	beforeUnlock func()
	forceCalls   int
}

func (p *finalizationUnlockProbe) UnlockWithValue(ctx context.Context, key, owner string) (bool, error) {
	if p.beforeUnlock != nil {
		p.beforeUnlock()
	}
	if p.fail {
		return false, errors.New("compare-delete unavailable")
	}
	return p.ILocker.UnlockWithValue(ctx, key, owner)
}

func (p *finalizationUnlockProbe) UnlockForce(ctx context.Context, key string) (bool, error) {
	p.forceCalls++
	return false, errors.New("force unlock forbidden")
}

func TestHookFinalizationManagerUnlockFailureAfterCommit(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	locker := &finalizationUnlockProbe{ILocker: f.base.(*ExptMangerImpl).mutex, fail: true, beforeUnlock: func() { s := finalizationRead(t, f); require.Equal(t, entity.HookFinalizeCommitted, s.State.Finalize) }}
	f.base.(*ExptMangerImpl).mutex = locker
	finalizationRecreate(t, f)
	require.Error(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil))
	require.Equal(t, int64(1), f.redis.Exists(ctx, fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)).Val())
	s := finalizationRead(t, f)
	locker.fail = false
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil))
	require.Zero(t, f.redis.Exists(ctx, fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)).Val())
	require.Equal(t, s, finalizationRead(t, f))
	require.Zero(t, locker.forceCalls)
	require.Equal(t, 1, f.notifications)
}

func TestHookFinalizationManagerDuplicateConcurrentCalls(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i == 0 {
				errs <- f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil)
			} else {
				errs <- f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	s := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, s.State.Finalize)
	require.True(t, s.State.After.Activated)
	require.Equal(t, 1, f.notifications)
}

func TestHookFinalizationManagerQuotaLockCoversLatestReadAndSet(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	source := &finalizationSourceProbe{IHookFinalizationRepo: f.deps.Repository}
	checks := 0
	next := finalizationTestIDs.Add(1)
	source.afterRead = func(key entity.HookRunKey) {
		checks++
		if checks != 3 {
			return
		}
		require.Equal(t, f.key, key)
		// New Run publication is independent of the workspace quota critical section.
		require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("latest_run_id", next).Error)
		taken, err := f.redis.SetNX(ctx, fmt.Sprintf("lock:quota_space_expt:%d", f.space), "successor", time.Second).Result()
		require.NoError(t, err)
		require.False(t, taken, "successor cannot acquire quota while old finalizer's updater is running")
	}
	f.deps.Repository = source
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil))
	q, err := dao.NewQuotaDAO(f.redis).GetQuotaSpaceExpt(ctx, f.space)
	require.NoError(t, err)
	require.NotContains(t, q.ExptID2RunTime, f.expt)
	// The successor's quota acquisition follows publication and waits for the old Set.
	require.NoError(t, f.quota.CreateOrUpdate(ctx, f.space, func(q *entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error) {
		q.ExptID2RunTime[f.expt] = 789
		return q, true, nil
	}, nil))
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	q, err = dao.NewQuotaDAO(f.redis).GetQuotaSpaceExpt(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, int64(789), q.ExptID2RunTime[f.expt])
	require.Zero(t, f.notifications)
}

func TestHookFinalizationManagerLegacyFallback(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("lifecycle_hook_version", 0).Error)
	require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil))
	var life model.ExptLifecycleRun
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&life).Error)
	require.Zero(t, life.FinalizeState)
	require.Zero(t, f.notifications)
	require.Zero(t, f.redis.Exists(ctx, fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)).Val(), "legacy path retains existing UnlockForce behavior")
}
