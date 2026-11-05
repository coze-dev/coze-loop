// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type completeRunCleanupLeaseGate struct {
	attempts     atomic.Int32
	server       *miniredis.Miniredis
	aCancelled   chan struct{}
	allowAUnlock chan struct{}
}

type completeRunCleanupAttemptLocker struct {
	lock.ILocker
	gate *completeRunCleanupLeaseGate
}

func (l *completeRunCleanupAttemptLocker) LockWithRenew(ctx context.Context, key string, ttl, hold time.Duration) (bool, context.Context, func(), error) {
	attempt := l.gate.attempts.Add(1)
	locked, lockCtx, cancel, err := l.ILocker.LockWithRenew(ctx, key, ttl, hold)
	if attempt != 1 || !locked || err != nil {
		return locked, lockCtx, cancel, err
	}
	return locked, lockCtx, func() {
		cancel()
		deadline := time.Now().Add(2 * time.Second)
		for l.gate.server.Exists(key) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		close(l.gate.aCancelled)
		<-l.gate.allowAUnlock
	}, nil
}

type completeRunCleanupUpdateGate struct {
	repo.IExptRunLogRepo
	updates     atomic.Int32
	bAtQuota    chan struct{}
	allowBQuota chan struct{}
}

func (r *completeRunCleanupUpdateGate) Update(ctx context.Context, exptID, runID, spaceID int64, fields map[string]any) error {
	if r.updates.Add(1) == 2 {
		close(r.bAtQuota)
		<-r.allowBQuota
	}
	return r.IExptRunLogRepo.Update(ctx, exptID, runID, spaceID, fields)
}

func TestCompleteRunCleanupAttemptsHaveIndependentOwners(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := newTestExptManager(ctrl)
	provider := db.NewTestDB(t, &model.Experiment{}, &model.ExptRunLog{}, &model.ExptStats{}, &model.ExptEvaluatorRef{})
	ctx := context.Background()
	endedAt := time.Now().Add(-time.Minute)
	require.NoError(t, provider.NewSession(ctx).Create(&model.Experiment{
		ID: 7, SpaceID: 9, ExptType: int32(entity.ExptType_Offline), LatestRunID: 8,
		Status: int32(entity.ExptStatus_Success), EndAt: &endedAt,
	}).Error)
	require.NoError(t, provider.NewSession(ctx).Create(&model.ExptRunLog{
		ID: 8, ExptID: 7, ExptRunID: 8, SpaceID: 9, Status: gptr.Of(int64(entity.ExptStatus_Pending)),
	}).Error)

	server := miniredis.RunT(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, ok := infraredis.Unwrap(client)
	require.True(t, ok)
	t.Cleanup(func() { _ = raw.Close() })
	gate := &completeRunCleanupLeaseGate{
		server: server, aCancelled: make(chan struct{}), allowAUnlock: make(chan struct{}),
	}
	manager.mutex = &completeRunCleanupAttemptLocker{
		ILocker: lock.NewRedisLockerWithHolder(client, "shared-cleanup-holder"), gate: gate,
	}
	manager.manualRetryLocker = func() lock.ILocker {
		return &completeRunCleanupAttemptLocker{ILocker: lock.NewRedisLocker(client), gate: gate}
	}
	manager.exptRepo = exptrepo.NewExptRepo(exptmysql.NewExptDAO(provider), exptmysql.NewExptEvaluatorRefDAO(provider), manager.idgenerator)
	manager.runFinishRepo = exptrepo.NewExptRunFinishRepo(provider)
	updateGate := &completeRunCleanupUpdateGate{
		IExptRunLogRepo: exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(provider)),
		bAtQuota:        make(chan struct{}), allowBQuota: make(chan struct{}),
	}
	manager.runLogRepo = updateGate
	manager.turnResultRepo.(*repomocks.MockIExptTurnResultRepo).EXPECT().
		ListTurnResult(gomock.Any(), int64(9), int64(7), nil, gomock.Any(), false).
		Return([]*entity.ExptTurnResult{}, int64(0), nil).AnyTimes()

	var quotaMu sync.Mutex
	quota := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{7: time.Now().Add(-time.Minute).Unix()}}
	manager.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), int64(9), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
			quotaMu.Lock()
			defer quotaMu.Unlock()
			next, changed, err := update(quota.Clone())
			if err == nil && changed {
				quota = next
			}
			return err
		}).AnyTimes()

	lockKey := manager.makeExptMutexLockKey(7)
	require.NoError(t, server.Set(lockKey, "8"))
	aErr := make(chan error, 1)
	bErr := make(chan error, 1)
	go func() {
		aErr <- manager.CompleteRun(ctx, 7, 8, 9, &entity.Session{}, entity.WithStatus(entity.ExptStatus_Failed))
	}()
	waitCompleteRunCleanupSignal(t, gate.aCancelled)
	go func() {
		bErr <- manager.CompleteRun(ctx, 7, 8, 9, &entity.Session{}, entity.WithStatus(entity.ExptStatus_Failed))
	}()
	waitCompleteRunCleanupSignal(t, updateGate.bAtQuota)

	close(gate.allowAUnlock)
	require.NoError(t, <-aErr)
	r2StartedEarly, _, err := manager.mutex.BackoffLockWithValue(ctx, lockKey, "9", time.Hour, time.Millisecond)
	require.NoError(t, err)
	r2QuotaTime := time.Now().Unix()
	setR2State := func() {
		require.NoError(t, manager.exptRepo.Update(ctx, &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 9}))
		quotaMu.Lock()
		quota.ExptID2RunTime[7] = r2QuotaTime
		quotaMu.Unlock()
	}
	if r2StartedEarly {
		setR2State()
	}
	close(updateGate.allowBQuota)
	require.NoError(t, <-bErr)
	if !r2StartedEarly {
		locked, _, err := manager.mutex.BackoffLockWithValue(ctx, lockKey, "9", time.Hour, time.Millisecond)
		require.NoError(t, err)
		require.True(t, locked)
		setR2State()
	}

	assert.False(t, r2StartedEarly, "A's cancel deleted B's cleanup lease")
	owner, err := server.Get(lockKey)
	require.NoError(t, err)
	require.Equal(t, "9", owner)
	quotaMu.Lock()
	require.Equal(t, r2QuotaTime, quota.ExptID2RunTime[7], "B released R2's replacement quota")
	quotaMu.Unlock()
	run, err := manager.runLogRepo.Get(ctx, 7, 8)
	require.NoError(t, err)
	require.Equal(t, int64(entity.ExptStatus_Failed), run.Status)
}

func waitCompleteRunCleanupSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for cleanup interleaving")
	}
}
