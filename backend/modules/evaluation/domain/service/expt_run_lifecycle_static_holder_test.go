// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// 升级前的生产 holder 不是 <hostname>-<xid>，而是共享 locker 的固定
// "evaluation"（application.NewLock），TTL 为 zombie 默认 36h，且不续期。
const legacyStaticHolderValue = "evaluation"

const legacyStaticZombieTTL = 36 * time.Hour

func seedLegacyStaticLifecycleHolder(t *testing.T, server *miniredis.Miniredis, key string, age time.Duration) {
	t.Helper()
	require.NoError(t, server.Set(key, legacyStaticHolderValue))
	server.SetTTL(key, legacyStaticZombieTTL-age)
}

func legacyStaticLifecycleConfig(ctrl *gomock.Controller) *componentmocks.MockIConfiger {
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{
		ZombieIntervalSecond: int(legacyStaticZombieTTL / time.Second),
	}).AnyTimes()
	return config
}

// 事故 B/C：静态 "evaluation" holder 无法解析年龄，finish/termination 清理
// 会一直 MQ retry 到 36h 自然过期。终态且 latest-run 校验通过时必须按 value 接管。
func TestRunFinishCleanupMigratesStaticLegacyLifecycleHolder(t *testing.T) {
	for _, kind := range []string{"natural", "kill"} {
		t.Run(kind, func(t *testing.T) {
			factory, server := finishRealLockerFactory(t)
			key := "expt_run_mutex_lock:1"
			seedLegacyStaticLifecycleHolder(t, server, key, 10*time.Minute)
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
			manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments, quotaRepo: quota, idem: idem, configer: legacyStaticLifecycleConfig(ctrl)}
			event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{}}
			var err error
			if kind == "natural" {
				err = manager.cleanupNaturallyFinishedRun(context.Background(), event, current)
			} else {
				err = manager.cleanupTerminatedRun(context.Background(), event, current, nil, &entity.CompleteExptOption{NoCompleteItemTurn: true})
			}
			require.NoError(t, err, "static legacy holder must not block cleanup until the 36h TTL")
			_, err = server.Get(key)
			require.Error(t, err, "cleanup must release the migrated lifecycle lock")
		})
	}
}

// 不足迁移宽限期：无法排除旧 writer 刚写入，必须保守保留。
func TestRunFinishCleanupKeepsStaticLegacyHolderWithinMigrationGrace(t *testing.T) {
	factory, server := finishRealLockerFactory(t)
	key := "expt_run_mutex_lock:1"
	seedLegacyStaticLifecycleHolder(t, server, key, time.Minute)
	ctrl := gomock.NewController(t)
	now := time.Now()
	current := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: &now}
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments, configer: legacyStaticLifecycleConfig(ctrl)}
	err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, current)
	require.True(t, errno.NeedMQRetry(err))
	owner, getErr := server.Get(key)
	require.NoError(t, getErr, "holder within the migration grace must be retained")
	require.Equal(t, legacyStaticHolderValue, owner)
}

// TTL/配置无法证明年龄时必须保守：剩余 TTL 大于配置的租约时长，或没有配置。
func TestRunFinishCleanupKeepsStaticLegacyHolderWhenAgeUnprovable(t *testing.T) {
	t.Run("remaining_ttl_exceeds_configured", func(t *testing.T) {
		factory, server := finishRealLockerFactory(t)
		key := "expt_run_mutex_lock:1"
		require.NoError(t, server.Set(key, legacyStaticHolderValue))
		server.SetTTL(key, 2*time.Hour)
		ctrl := gomock.NewController(t)
		config := componentmocks.NewMockIConfiger(ctrl)
		config.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{ZombieIntervalSecond: 3600}).AnyTimes()
		now := time.Now()
		current := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: &now}
		experiments := repomocks.NewMockIExperimentRepo(ctrl)
		manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments, configer: config}
		err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, current)
		require.True(t, errno.NeedMQRetry(err))
		owner, getErr := server.Get(key)
		require.NoError(t, getErr)
		require.Equal(t, legacyStaticHolderValue, owner)
	})
	t.Run("no_configer", func(t *testing.T) {
		factory, server := finishRealLockerFactory(t)
		key := "expt_run_mutex_lock:1"
		seedLegacyStaticLifecycleHolder(t, server, key, 10*time.Minute)
		ctrl := gomock.NewController(t)
		now := time.Now()
		current := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: &now}
		experiments := repomocks.NewMockIExperimentRepo(ctrl)
		manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments}
		err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, current)
		require.True(t, errno.NeedMQRetry(err))
		owner, getErr := server.Get(key)
		require.NoError(t, getErr)
		require.Equal(t, legacyStaticHolderValue, owner)
	})
}

// 活跃 run 或非 latest run 即使 holder 已老，也必须保留终态/latest-run 校验。
func TestRunFinishCleanupKeepsStaticLegacyHolderForOwnershipMismatch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		current *entity.Experiment
	}{
		{"active_run", &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Processing}},
		{"non_latest_run", &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 99, Status: entity.ExptStatus_Success}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory, server := finishRealLockerFactory(t)
			key := "expt_run_mutex_lock:1"
			seedLegacyStaticLifecycleHolder(t, server, key, 10*time.Minute)
			ctrl := gomock.NewController(t)
			experiments := repomocks.NewMockIExperimentRepo(ctrl)
			experiments.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(tc.current, nil).AnyTimes()
			manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments, configer: legacyStaticLifecycleConfig(ctrl)}
			err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, tc.current)
			require.True(t, errno.NeedMQRetry(err))
			owner, getErr := server.Get(key)
			require.NoError(t, getErr)
			require.Equal(t, legacyStaticHolderValue, owner)
		})
	}
}

// 事故 B 的另一条入口：CompleteRun 的 currentRunCleanup 也必须复用同一迁移。
func TestCompleteRunCleanupMigratesStaticLegacyLifecycleHolder(t *testing.T) {
	factory, server := finishRealLockerFactory(t)
	key := "expt_run_mutex_lock:7"
	seedLegacyStaticLifecycleHolder(t, server, key, 10*time.Minute)
	ctrl := gomock.NewController(t)
	now := time.Now()
	current := &entity.Experiment{ID: 7, SpaceID: 9, ExptType: entity.ExptType_Offline, LatestRunID: 8, Status: entity.ExptStatus_Success, EndAt: &now}
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	experiments.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(current, nil).AnyTimes()
	runs := repomocks.NewMockIExptRunLogRepo(ctrl)
	runs.EXPECT().Get(gomock.Any(), int64(7), int64(8)).Return(&entity.ExptRunLog{ID: 8, ExptID: 7, ExptRunID: 8, SpaceID: 9, Status: int64(entity.ExptStatus_Success)}, nil)
	runs.EXPECT().Update(gomock.Any(), int64(7), int64(8), int64(9), gomock.Any()).Return(nil)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	turns.EXPECT().ListTurnResult(gomock.Any(), int64(9), int64(7), nil, gomock.Any(), false).Return(nil, int64(0), nil)
	quota := repomocks.NewMockQuotaRepo(ctrl)
	released := false
	quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(9), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, mutate func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
		next, changed, err := mutate(&entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{7: time.Now().Unix()}})
		released = changed && next != nil
		return err
	})
	manager := &ExptMangerImpl{
		runFinishRepo:     &finishRepoProbe{},
		mutex:             factory(),
		manualRetryLocker: factory,
		exptRepo:          experiments,
		runLogRepo:        runs,
		turnResultRepo:    turns,
		quotaRepo:         quota,
		configer:          legacyStaticLifecycleConfig(ctrl),
	}
	require.NoError(t, manager.CompleteRun(context.Background(), 7, 8, 9, &entity.Session{}, entity.WithStatus(entity.ExptStatus_Success)))
	require.True(t, released, "cleanup must reach quota release instead of retrying for the 36h TTL")
	_, err := server.Get(key)
	require.Error(t, err, "migrated holder must be released")
}

// 数字 runID（其他 run 的 admission lease）与新 cleanup lease 都不能被迁移误删。
func TestCompleteRunCleanupKeepsNonLegacyLifecycleHolders(t *testing.T) {
	for _, holder := range []string{"99", "run-cleanup-live-owner"} {
		t.Run(holder, func(t *testing.T) {
			factory, server := finishRealLockerFactory(t)
			key := "expt_run_mutex_lock:7"
			require.NoError(t, server.Set(key, holder))
			server.SetTTL(key, 10*time.Minute)
			ctrl := gomock.NewController(t)
			now := time.Now()
			current := &entity.Experiment{ID: 7, SpaceID: 9, ExptType: entity.ExptType_Offline, LatestRunID: 8, Status: entity.ExptStatus_Success, EndAt: &now}
			experiments := repomocks.NewMockIExperimentRepo(ctrl)
			experiments.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(current, nil).AnyTimes()
			manager := &ExptMangerImpl{
				runFinishRepo:     &finishRepoProbe{},
				mutex:             factory(),
				manualRetryLocker: factory,
				exptRepo:          experiments,
			}
			err := manager.CompleteRun(context.Background(), 7, 8, 9, &entity.Session{}, entity.WithStatus(entity.ExptStatus_Success))
			require.True(t, errno.NeedMQRetry(err))
			owner, getErr := server.Get(key)
			require.NoError(t, getErr)
			require.Equal(t, holder, owner)
		})
	}
}
