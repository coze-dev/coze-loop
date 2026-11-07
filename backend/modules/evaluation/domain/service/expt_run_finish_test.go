// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gg/gptr"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/redis/go-redis/v9"
	"github.com/rs/xid"

	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type naturalFinishProbe struct {
	IExptManager
	calls       int
	disposition entity.ExptRunFinishDisposition
	err         error
}

func (p *naturalFinishProbe) FinishCurrentRun(ctx context.Context, event *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error) {
	return p.FinishCurrentRunWithHeldTickLock(ctx, event)
}

func (p *naturalFinishProbe) FinishCurrentRunWithHeldTickLock(context.Context, *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error) {
	p.calls++
	return &entity.ExptRunFinishResult{Disposition: p.disposition}, p.err
}

func TestRunFinishCommonCalibrationBlocksComposition(t *testing.T) {
	for _, fault := range []error{nil, errors.New("calibration unavailable")} {
		ctrl := gomock.NewController(t)
		manager := &naturalFinishProbe{IExptManager: svcmocks.NewMockIExptManager(ctrl)}
		idem := idemmocks.NewMockIdempotentService(ctrl)
		idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
		base := &exptBaseExec{Manager: manager, idem: idem, resultSvc: &finishCalibrationTestResult{err: fault}}
		err := base.exptEnd(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, &entity.Experiment{ExptType: entity.ExptType_Offline})
		require.True(t, errno.NeedMQRetry(err))
		require.Zero(t, manager.calls)
	}
}

func TestRunFinishRescanIncludesLogged(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), gomock.Any(), int64(0), int64(1), int64(3)).DoAndReturn(func(context.Context, int64, int64, *entity.ExptItemRunLogFilter, int64, int64, int64) ([]*entity.ExptItemResultRunLog, int64, error) {
		return nil, 0, nil
	})
	items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), gomock.Any(), int64(0), int64(1), int64(3)).DoAndReturn(func(_ context.Context, _ int64, _ int64, filter *entity.ExptItemRunLogFilter, _ int64, _ int64, _ int64) ([]*entity.ExptItemResultRunLog, int64, error) {
		require.NotNil(t, filter.ResultState)
		require.Equal(t, entity.ExptItemResultStateLogged, *filter.ResultState)
		return []*entity.ExptItemResultRunLog{{ItemID: 11}}, 1, nil
	})
	pending, err := (&exptBaseExec{exptItemResultRepo: &finishCurrentItemRepo{IExptItemResultRepo: items}}).hasPendingAfterRescan(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3})
	require.NoError(t, err)
	require.True(t, pending)
}

type finishRepoProbe struct {
	repo.IExptRunFinishRepo
	calls  int
	result *entity.ExptRunFinishResult
	err    error
}

func (p *finishRepoProbe) FinishCurrentRun(context.Context, *entity.ExptRunFinishRequest) (*entity.ExptRunFinishResult, error) {
	p.calls++
	return p.result, p.err
}

func TestRunFinishManagerCommitFailureAndPendingHaveZeroCleanup(t *testing.T) {
	for _, fault := range []error{errors.New("commit failed"), nil} {
		ctrl := gomock.NewController(t)
		port := &finishRepoProbe{err: fault, result: &entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishPending}}
		manager := &ExptMangerImpl{runFinishRepo: port, mutex: lockmocks.NewMockILocker(ctrl)}
		result, err := manager.FinishCurrentRunWithHeldTickLock(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2})
		if fault != nil {
			require.Error(t, err)
			require.Nil(t, result)
		} else {
			require.NoError(t, err)
			require.Equal(t, entity.ExptRunFinishPending, result.Disposition)
		}
		require.Equal(t, 1, port.calls)
	}
}

func TestRunFinishManagerCurrentRepositoryFailsClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := &ExptMangerImpl{itemResultRepo: &finishCurrentItemRepo{IExptItemResultRepo: repomocks.NewMockIExptItemResultRepo(ctrl)}, mutex: lockmocks.NewMockILocker(ctrl)}
	_, err := manager.FinishCurrentRunWithHeldTickLock(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2})
	require.Error(t, err)
}

type finishCurrentItemRepo struct {
	repo.IExptItemResultRepo
	repo.IExptCurrentResultRepo
}

func TestRunFinishManagerCleanupFailureReplaysAfterCommit(t *testing.T) {
	ctrl := gomock.NewController(t)
	port := &finishRepoProbe{result: &entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishFinished, Experiment: &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: gptr.Of(time.Now())}}}
	locker := lockmocks.NewMockILocker(ctrl)
	var locks atomic.Int32
	locker.EXPECT().UnlockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "2").Return(false, nil).Times(2)
	locker.EXPECT().LockWithRenew(gomock.Any(), "expt_run_mutex_lock:1", gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ string, _ time.Duration, _ time.Duration) (bool, context.Context, func(), error) {
		if locks.Add(1) == 1 {
			return false, ctx, func() {}, errors.New("Redis unavailable")
		}
		return true, ctx, func() {}, nil
	}).Times(2)
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 99, Status: entity.ExptStatus_Processing}, nil)
	manager := &ExptMangerImpl{runFinishRepo: port, mutex: locker, exptRepo: expts, manualRetryLocker: func() lock.ILocker { return locker }}
	_, err := manager.FinishCurrentRunWithHeldTickLock(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2})
	require.True(t, errno.NeedMQRetry(err))
	require.Equal(t, 1, port.calls)
	port.result.Disposition = entity.ExptRunFinishAlreadyFinished
	_, err = manager.FinishCurrentRunWithHeldTickLock(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2})
	require.NoError(t, err)
	require.Equal(t, 2, port.calls)
}

func TestRunFinishLogRunBindsLifecycleOwnerToRunID(t *testing.T) {
	ctrl := gomock.NewController(t)
	locker := lockmocks.NewMockILocker(ctrl)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{})
	locker.EXPECT().BackoffLockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "2", gomock.Any(), time.Second).Return(true, "2", nil)
	runs := repomocks.NewMockIExptRunLogRepo(ctrl)
	runs.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	expts.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitExptExecRun(int64(3), int64(entity.EvaluationModeSubmit))
	manager := &ExptMangerImpl{runFinishRepo: &finishRepoProbe{}, mutex: locker, configer: config, runLogRepo: runs, exptRepo: expts, mtr: metric}
	require.NoError(t, manager.LogRun(context.Background(), 1, 2, entity.EvaluationModeSubmit, 3, nil, &entity.Session{UserID: "test"}))
}

// 用真实 XID 构造可迁移的老 holder，或模拟刚获取的新 holder。
func legacyHolderAt(t *testing.T, when time.Time) string {
	t.Helper()
	return "test-host-" + xid.NewWithTime(when).String()
}

func finishRealLockerFactory(t *testing.T) (ManualRetryLockerFactory, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { _ = raw.Close() })
	return func() lock.ILocker { return filterRealTestLocker(client) }, server
}

func TestRunFinishCleanupDoesNotDeleteR2LifecycleOwner(t *testing.T) {
	factory, server := finishRealLockerFactory(t)
	key := "expt_run_mutex_lock:1"
	require.NoError(t, server.Set(key, "99"))
	manager := &ExptMangerImpl{manualRetryLocker: factory}
	err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, &entity.Experiment{ID: 1})
	require.True(t, errno.NeedMQRetry(err))
	owner, err := server.Get(key)
	require.NoError(t, err)
	require.Equal(t, "99", owner)
	// A pre-upgrade random owner cannot be safely migrated or force deleted.
	require.NoError(t, server.Set(key, "legacy-random-holder"))
	err = manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, &entity.Experiment{ID: 1})
	require.Error(t, err)
	owner, _ = server.Get(key)
	require.Equal(t, "legacy-random-holder", owner)
}

func TestRunFinishCleanupMigratesLegacyLifecycleHolder(t *testing.T) {
	for _, kind := range []string{"natural", "kill"} {
		t.Run(kind, func(t *testing.T) {
			factory, server := finishRealLockerFactory(t)
			key := "expt_run_mutex_lock:1"
			require.NoError(t, server.Set(key, legacyHolderAt(t, time.Now().Add(-10*time.Minute))))
			server.SetTTL(key, time.Hour)
			ctrl := gomock.NewController(t)
			now := time.Now()
			current := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: &now}
			experiments := repomocks.NewMockIExperimentRepo(ctrl)
			experiments.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(current, nil).AnyTimes()
			quota := repomocks.NewMockQuotaRepo(ctrl)
			quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, mutate func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
				_, _, err := mutate(&entity.QuotaSpaceExpt{})
				return err
			})
			idem := idemmocks.NewMockIdempotentService(ctrl)
			idem.EXPECT().Exist(gomock.Any(), "finish-cleanup:1:2").Return(true, nil)
			manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments, quotaRepo: quota, idem: idem}
			event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{}}
			var err error
			if kind == "natural" {
				err = manager.cleanupNaturallyFinishedRun(context.Background(), event, current)
			} else {
				err = manager.cleanupTerminatedRun(context.Background(), event, current, nil, &entity.CompleteExptOption{NoCompleteItemTurn: true})
			}
			require.NoError(t, err)
			_, err = server.Get(key)
			require.Error(t, err, "cleanup must release the migrated lifecycle lock")
		})
	}
}

func TestRunFinishCleanupDoesNotMigrateFreshLegacyHolder(t *testing.T) {
	factory, server := finishRealLockerFactory(t)
	key := "expt_run_mutex_lock:1"
	require.NoError(t, server.Set(key, legacyHolderAt(t, time.Now())))
	server.SetTTL(key, 36*time.Hour)
	ctrl := gomock.NewController(t)
	now := time.Now()
	current := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: &now}
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments}
	err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, current)
	require.Error(t, err)
	_, getErr := server.Get(key)
	require.NoError(t, getErr, "fresh holder must be retained")
}

// 评审反例：写锁时 TTL 600s，迁移时配置改成 1200s。
// 按 TTL 反推会把刚获取的 holder 判老；XID 时间戳与配置无关，必须保留。
func TestRunFinishCleanupKeepsFreshHolderAfterConfigIncrease(t *testing.T) {
	factory, server := finishRealLockerFactory(t)
	key := "expt_run_mutex_lock:1"
	require.NoError(t, server.Set(key, legacyHolderAt(t, time.Now())))
	server.SetTTL(key, 10*time.Minute)
	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1200}).AnyTimes()
	now := time.Now()
	current := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: &now}
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments, configer: config}
	err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, current)
	require.Error(t, err)
	_, getErr := server.Get(key)
	require.NoError(t, getErr, "config increase must not make a fresh holder look old")
}

func TestRunFinishCleanupKeepsLegacyHolderWhenLockTTLIsShort(t *testing.T) {
	factory, server := finishRealLockerFactory(t)
	key := "expt_run_mutex_lock:1"
	require.NoError(t, server.Set(key, "legacy-random-holder"))
	server.SetTTL(key, 30*time.Second)
	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{ZombieIntervalSecond: 60}).AnyTimes()
	now := time.Now()
	current := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: &now}
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments, configer: config}
	err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, current)
	require.Error(t, err)
	owner, getErr := server.Get(key)
	require.NoError(t, getErr)
	require.Equal(t, "legacy-random-holder", owner)
}

func TestRunFinishCleanupDoesNotMigrateCurrentCleanupHolder(t *testing.T) {
	factory, server := finishRealLockerFactory(t)
	key := "expt_run_mutex_lock:1"
	holder := lock.LifecycleCleanupHolderPrefix + "another-active-cleanup"
	require.NoError(t, server.Set(key, holder))
	ctrl := gomock.NewController(t)
	now := time.Now()
	current := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: &now}
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments}
	err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, current)
	require.True(t, errno.NeedMQRetry(err))
	owner, getErr := server.Get(key)
	require.NoError(t, getErr)
	require.Equal(t, holder, owner)
}

func TestRunFinishDirectAcquiresTickAndLeaseLossStopsSQL(t *testing.T) {
	ctrl := gomock.NewController(t)
	locker := lockmocks.NewMockILocker(ctrl)
	port := &finishRepoProbe{}
	locker.EXPECT().LockWithRenew(gomock.Any(), "expt_run_exec_lock:1:2", 5*time.Second, 5*time.Minute).DoAndReturn(func(ctx context.Context, _ string, _ time.Duration, _ time.Duration) (bool, context.Context, func(), error) {
		child, cancel := context.WithCancel(ctx)
		cancel()
		return true, child, cancel, nil
	})
	manager := &ExptMangerImpl{runFinishRepo: port, manualRetryLocker: func() lock.ILocker { return locker }}
	_, err := manager.FinishCurrentRun(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2})
	require.True(t, errno.NeedMQRetry(err))
	require.Zero(t, port.calls)
}

func TestRunFinishHeldHelperCancelledLeaseStopsSQL(t *testing.T) {
	port := &finishRepoProbe{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager := &ExptMangerImpl{runFinishRepo: port}
	_, err := manager.FinishCurrentRunWithHeldTickLock(ctx, &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2})
	require.True(t, errno.NeedMQRetry(err))
	require.Zero(t, port.calls)
}

func TestRunFinishCleanupLostLeaseStopsQuotaAndNotifications(t *testing.T) {
	ctrl := gomock.NewController(t)
	locker := lockmocks.NewMockILocker(ctrl)
	locker.EXPECT().UnlockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "2").Return(true, nil)
	locker.EXPECT().LockWithRenew(gomock.Any(), "expt_run_mutex_lock:1", 5*time.Second, 5*time.Minute).DoAndReturn(func(ctx context.Context, _ string, _ time.Duration, _ time.Duration) (bool, context.Context, func(), error) {
		child, cancel := context.WithCancel(ctx)
		cancel()
		return true, child, cancel, nil
	})
	manager := &ExptMangerImpl{manualRetryLocker: func() lock.ILocker { return locker }, quotaRepo: repomocks.NewMockQuotaRepo(ctrl), publisher: eventmocks.NewMockExptEventPublisher(ctrl), exptRepo: repomocks.NewMockIExperimentRepo(ctrl)}
	require.Error(t, manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, &entity.Experiment{ID: 1}))
}

// Numeric ownership on Submit must not broaden legacy live RetryItems join.
func TestRunFinishNumericSubmitOwnerDoesNotAllowRetryItemsJoin(t *testing.T) {
	ctrl := gomock.NewController(t)
	locker := lockmocks.NewMockILocker(ctrl)
	config := componentmocks.NewMockIConfiger(ctrl)
	ids := idgenmocks.NewMockIIDGenerator(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{})
	ids.EXPECT().GenID(gomock.Any()).Return(int64(99), nil)
	locker.EXPECT().BackoffLockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "99", gomock.Any(), time.Second).Return(false, "2", nil)
	locker.EXPECT().Lock(gomock.Any(), "expt_completing_mutex_lock:1:2", 3*time.Minute).Return(true, nil)
	locker.EXPECT().Unlock("expt_completing_mutex_lock:1:2").Return(true, nil)
	runs := repomocks.NewMockIExptRunLogRepo(ctrl)
	runs.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&entity.ExptRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Processing)}, nil).AnyTimes()
	manager := &ExptMangerImpl{runFinishRepo: &finishRepoProbe{}, mutex: locker, configer: config, idgenerator: ids, runLogRepo: runs}
	_, _, err := manager.LogRetryItemsRun(context.Background(), 1, entity.EvaluationModeRetryItems, 3, []int64{11}, &entity.Session{UserID: "test"})
	require.Error(t, err)
}
