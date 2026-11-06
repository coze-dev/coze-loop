// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	goredis "github.com/redis/go-redis/v9"
)

// 升级前的生产 holder 不是 <hostname>-<xid>，而是共享 locker 的固定
// "evaluation"（application.NewLock），TTL 为 zombie 默认 36h，且不续期。
const (
	legacyStaticHolderValue = "evaluation"
	legacyStaticZombieTTL   = 36 * time.Hour
)

func seedLegacyStaticLifecycleHolder(t *testing.T, server *miniredis.Miniredis, key string, age time.Duration) {
	t.Helper()
	require.NoError(t, server.Set(key, legacyStaticHolderValue))
	server.SetTTL(key, legacyStaticZombieTTL-age)
}

func staticHolderExperiment(id, space, run int64, status entity.ExptStatus, endAt *time.Time) *entity.Experiment {
	return &entity.Experiment{ID: id, SpaceID: space, ExptType: entity.ExptType_Offline, LatestRunID: run, Status: status, EndAt: endAt}
}

// 评审反例后的保守语义：固定 "evaluation" holder 没有 acquisition token，旧
// LogRun 可以在上一 run 释放后以同值被下一个 run 重新取得；终态 EndAt 与同值
// CAS 都不能证明当前 owner 属于本 run。因此 finish/termination 清理不得删除它，
// 必须保持 MQRetry，让 AlreadyFinished 重放在租约自然到期后恢复。
func TestRunFinishCleanupKeepsStaticLegacyLifecycleHolder(t *testing.T) {
	for _, kind := range []string{"natural", "kill"} {
		t.Run(kind, func(t *testing.T) {
			factory, server := finishRealLockerFactory(t)
			key := "expt_run_mutex_lock:1"
			seedLegacyStaticLifecycleHolder(t, server, key, 10*time.Minute)
			ctrl := gomock.NewController(t)
			endAt := time.Now().Add(-10 * time.Minute)
			current := staticHolderExperiment(1, 3, 2, entity.ExptStatus_Success, &endAt)
			experiments := repomocks.NewMockIExperimentRepo(ctrl)
			experiments.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(current, nil).AnyTimes()
			manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments}
			event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{}}
			var err error
			if kind == "natural" {
				err = manager.cleanupNaturallyFinishedRun(context.Background(), event, current)
			} else {
				err = manager.cleanupTerminatedRun(context.Background(), event, current, nil, &entity.CompleteExptOption{NoCompleteItemTurn: true})
			}
			require.True(t, errno.NeedMQRetry(err), "cleanup must stay replayable instead of deleting an unowned holder")
			owner, getErr := server.Get(key)
			require.NoError(t, getErr, "unproven static holder must be retained")
			require.Equal(t, legacyStaticHolderValue, owner)
		})
	}
}

// 终态 EndAt 距现在不足迁移宽限：无法排除同一 run 的旧 writer 仍在收尾，
// 必须保守保留，交给后续 MQ 重投。
func TestRunFinishCleanupKeepsStaticLegacyHolderWithinMigrationGrace(t *testing.T) {
	factory, server := finishRealLockerFactory(t)
	key := "expt_run_mutex_lock:1"
	seedLegacyStaticLifecycleHolder(t, server, key, 10*time.Minute)
	ctrl := gomock.NewController(t)
	endAt := time.Now().Add(-time.Minute)
	current := staticHolderExperiment(1, 3, 2, entity.ExptStatus_Success, &endAt)
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	experiments.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(current, nil).AnyTimes()
	manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments}
	err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, current)
	require.True(t, errno.NeedMQRetry(err))
	owner, getErr := server.Get(key)
	require.NoError(t, getErr, "holder within the migration grace must be retained")
	require.Equal(t, legacyStaticHolderValue, owner)
}

// 评审反例：锁刚按 1h TTL 写入，随后 zombie 配置被调到 36h。
// 旧实现用 configuredTTL-remainingTTL 会把 1h 的锁算成 35h 并立即接管；
// 新实现完全不再读配置，只认不可变的终态 EndAt。此处的实验终态时间不足
// 宽限/缺失，必须保留。configer 的 GetExptExecConf 设为 Times(0)，任何
// 基于配置的推龄都会让测试失败。
func TestStaticLegacyHolderConfigIncreaseDoesNotFabricateAge(t *testing.T) {
	for _, tc := range []struct {
		name  string
		endAt *time.Time
	}{
		{name: "fresh_lock_recent_terminal", endAt: func() *time.Time { v := time.Now().Add(-time.Minute); return &v }()},
		{name: "fresh_lock_missing_terminal_time", endAt: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory, server := finishRealLockerFactory(t)
			key := "expt_run_mutex_lock:1"
			// 刚写入：剩余 TTL 就是写入时的 1h 租约。
			require.NoError(t, server.Set(key, legacyStaticHolderValue))
			server.SetTTL(key, time.Hour)
			ctrl := gomock.NewController(t)
			config := componentmocks.NewMockIConfiger(ctrl)
			config.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Times(0)
			current := staticHolderExperiment(1, 3, 2, entity.ExptStatus_Success, tc.endAt)
			experiments := repomocks.NewMockIExperimentRepo(ctrl)
			experiments.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(current, nil).AnyTimes()
			manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments, configer: config}
			err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, current)
			require.True(t, errno.NeedMQRetry(err))
			owner, getErr := server.Get(key)
			require.NoError(t, getErr, "config increase must not age a freshly written static holder")
			require.Equal(t, legacyStaticHolderValue, owner)
		})
	}
}

// 终态时间缺失、异常（未来时间）或实验仍活跃：均无法证明静态 holder 已过宽限。
func TestRunFinishCleanupKeepsStaticLegacyHolderWhenTerminalTimeUnproven(t *testing.T) {
	future := time.Now().Add(time.Hour)
	old := time.Now().Add(-10 * time.Minute)
	for _, tc := range []struct {
		name    string
		current *entity.Experiment
	}{
		{name: "missing_end_at", current: staticHolderExperiment(1, 3, 2, entity.ExptStatus_Success, nil)},
		{name: "future_end_at", current: staticHolderExperiment(1, 3, 2, entity.ExptStatus_Success, &future)},
		{name: "active_run", current: staticHolderExperiment(1, 3, 2, entity.ExptStatus_Processing, &old)},
		{name: "non_latest_run", current: staticHolderExperiment(1, 3, 99, entity.ExptStatus_Success, &old)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory, server := finishRealLockerFactory(t)
			key := "expt_run_mutex_lock:1"
			seedLegacyStaticLifecycleHolder(t, server, key, 10*time.Minute)
			ctrl := gomock.NewController(t)
			experiments := repomocks.NewMockIExperimentRepo(ctrl)
			experiments.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(tc.current, nil).AnyTimes()
			manager := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: experiments}
			err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, tc.current)
			require.True(t, errno.NeedMQRetry(err))
			owner, getErr := server.Get(key)
			require.NoError(t, getErr)
			require.Equal(t, legacyStaticHolderValue, owner)
		})
	}
}

// 事故 B 的另一条入口：CompleteRun 的 currentRunCleanup 也不得删除无法证明
// 归属的固定 holder；必须先返回 MQRetry，等租约到期后由重放恢复。
func TestCompleteRunCleanupKeepsStaticLegacyLifecycleHolder(t *testing.T) {
	factory, server := finishRealLockerFactory(t)
	key := "expt_run_mutex_lock:7"
	seedLegacyStaticLifecycleHolder(t, server, key, 10*time.Minute)
	ctrl := gomock.NewController(t)
	endAt := time.Now().Add(-10 * time.Minute)
	current := staticHolderExperiment(7, 9, 8, entity.ExptStatus_Success, &endAt)
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	experiments.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(current, nil).AnyTimes()
	manager := &ExptMangerImpl{
		runFinishRepo:     &finishRepoProbe{},
		mutex:             factory(),
		manualRetryLocker: factory,
		exptRepo:          experiments,
	}
	err := manager.CompleteRun(context.Background(), 7, 8, 9, &entity.Session{}, entity.WithStatus(entity.ExptStatus_Failed))
	require.True(t, errno.NeedMQRetry(err))
	owner, getErr := server.Get(key)
	require.NoError(t, getErr, "static holder must be kept")
	require.Equal(t, legacyStaticHolderValue, owner)
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
			endAt := time.Now().Add(-10 * time.Minute)
			current := staticHolderExperiment(7, 9, 8, entity.ExptStatus_Success, &endAt)
			experiments := repomocks.NewMockIExperimentRepo(ctrl)
			experiments.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(current, nil).AnyTimes()
			manager := &ExptMangerImpl{
				runFinishRepo:     &finishRepoProbe{},
				mutex:             factory(),
				manualRetryLocker: factory,
				exptRepo:          experiments,
			}
			err := manager.CompleteRun(context.Background(), 7, 8, 9, &entity.Session{}, entity.WithStatus(entity.ExptStatus_Failed))
			require.True(t, errno.NeedMQRetry(err))
			owner, getErr := server.Get(key)
			require.NoError(t, getErr)
			require.Equal(t, holder, owner)
		})
	}
}

// 真实 Redis + MySQL：验证静态 holder 的 TTL 保真、UnlockWithValue 的按值
// CAS（错误 owner 不能删）、LockWithRenew 续租，以及终态 EndAt 证据下清理
// 能真正删锁并走到额度释放。缺环境时必须显式失败，不能把 skip 当通过。
func TestStaticLegacyHolderRealRedisAndMySQL(t *testing.T) {
	addr := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR")
	dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
	require.NotEmpty(t, addr, "real Redis must be configured; this regression cannot skip")
	require.NotEmpty(t, dsn, "real MySQL must be configured; this regression cannot skip")
	ctx := context.Background()

	client, err := infraredis.NewClient(&goredis.Options{Addr: addr})
	require.NoError(t, err)
	raw, ok := infraredis.Unwrap(client)
	require.True(t, ok)
	info, err := raw.Info(ctx, "server").Result()
	require.NoError(t, err)
	require.Contains(t, info, "redis_version:")

	provider, err := db.NewDB(mysqldriver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn := provider.NewSession(ctx, db.WithMaster())
	var identity struct{ Version, Schema string }
	require.NoError(t, conn.Raw("SELECT VERSION() AS version,DATABASE() AS `schema`").Scan(&identity).Error)
	require.Equal(t, "expt_running_retry_test", identity.Schema)

	base := int64(880000000000000000) + time.Now().UnixNano()%1000000000000
	space, expt, run := base+1, base+2, base+3
	key := fmt.Sprintf("expt_run_mutex_lock:%d", expt)
	t.Cleanup(func() {
		require.NoError(t, conn.Exec("DELETE FROM experiment WHERE space_id=? AND id=?", space, expt).Error)
		require.NoError(t, raw.Del(ctx, key).Err())
		sqlDB, dbErr := conn.DB()
		require.NoError(t, dbErr)
		require.NoError(t, sqlDB.Close())
		require.NoError(t, raw.Close())
	})

	endAt := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	require.NoError(t, conn.Exec("INSERT INTO experiment(id,space_id,latest_run_id,status,expt_type,end_at) VALUES(?,?,?,?,?,?)",
		expt, space, run, entity.ExptStatus_Success, entity.ExptType_Offline, endAt).Error)
	repo := experimentrepo.NewExptRepo(experimentmysql.NewExptDAO(provider), experimentmysql.NewExptEvaluatorRefDAO(provider), nil)
	stored, err := repo.GetByID(ctx, expt, space)
	require.NoError(t, err)
	require.NotNil(t, stored.EndAt)
	t.Logf("REAL_MYSQL_EXECUTED version=%s schema=%s end_at_age=%v", identity.Version, identity.Schema, time.Since(*stored.EndAt))

	// 旧 writer 的静态 holder：TTL 保真，且按值 CAS 拒绝陌生 owner。
	require.NoError(t, raw.Set(ctx, key, legacyStaticHolderValue, legacyStaticZombieTTL).Err())
	ttl, err := raw.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, legacyStaticZombieTTL-time.Minute)
	holderLocker := lock.NewRedisLockerWithHolder(client, legacyStaticHolderValue)
	released, err := holderLocker.UnlockWithValue(ctx, key, "another-run-holder")
	require.NoError(t, err)
	require.False(t, released, "UnlockWithValue must not delete a foreign owner")
	owner, err := raw.Get(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, legacyStaticHolderValue, owner)
	t.Logf("REAL_REDIS_EXECUTED static_holder_ttl=%v value_cas_rejects_foreign_owner", ttl)

	// 新 cleanup lease 的续租：TTL 必须在过期前被刷新。
	renewKey := key + ":renew"
	locked, _, cancel, err := holderLocker.LockWithRenew(ctx, renewKey, 8*time.Second, 10*time.Second)
	require.NoError(t, err)
	require.True(t, locked)
	ttlBefore, err := raw.TTL(ctx, renewKey).Result()
	require.NoError(t, err)
	time.Sleep(3 * time.Second)
	ttlAfter, err := raw.TTL(ctx, renewKey).Result()
	require.NoError(t, err)
	require.Greater(t, ttlAfter, ttlBefore-1500*time.Millisecond, "LockWithRenew must refresh the lease")
	cancel()
	_, _ = raw.Del(ctx, renewKey).Result()
	t.Logf("REAL_REDIS_RENEW_EXECUTED ttl_before=%v ttl_after=%v", ttlBefore, ttlAfter)

	ctrl := gomock.NewController(t)
	quotaReleased := false
	quota := repomocks.NewMockQuotaRepo(ctrl)
	quota.EXPECT().CreateOrUpdate(gomock.Any(), space, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, mutate func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
		next, changed, err := mutate(&entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{expt: time.Now().Unix()}})
		quotaReleased = changed && next != nil
		return err
	})
	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), fmt.Sprintf("finish-cleanup:%d:%d", expt, run)).Return(true, nil)
	manager := &ExptMangerImpl{
		manualRetryLocker: func() lock.ILocker { return filterRealTestLocker(client) },
		exptRepo:          repo,
		quotaRepo:         quota,
		idem:              idem,
	}
	// 把 36h 租约推到尾巴上模拟自然到期；不删除 holder、不依赖 EndAt 版本。
	require.NoError(t, raw.PExpire(ctx, key, 600*time.Millisecond).Err())
	err = manager.cleanupNaturallyFinishedRun(ctx, &entity.ExptScheduleEvent{ExptID: expt, ExptRunID: run, SpaceID: space, Session: &entity.Session{}}, stored)
	require.True(t, errno.NeedMQRetry(err), "unowned fixed holder must keep cleanup replayable")
	require.False(t, quotaReleased)
	holder, getErr := raw.Get(ctx, key).Result()
	require.NoError(t, getErr)
	require.Equal(t, legacyStaticHolderValue, holder, "unproven static holder must be kept")

	deadline := time.Now().Add(10 * time.Second)
	for {
		n, existsErr := raw.Exists(ctx, key).Result()
		require.NoError(t, existsErr)
		if n == 0 {
			break
		}
		require.False(t, time.Now().After(deadline), "static holder did not expire")
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, manager.cleanupNaturallyFinishedRun(ctx, &entity.ExptScheduleEvent{ExptID: expt, ExptRunID: run, SpaceID: space, Session: &entity.Session{}}, stored))
	require.True(t, quotaReleased, "replay after natural expiry must recover the finish cleanup")
	exists, err := raw.Exists(ctx, key).Result()
	require.NoError(t, err)
	require.Zero(t, exists, "cleanup lease must be released after recovery")
	t.Log("REAL_REDIS_MYSQL_STATIC_HOLDER_KEPT_THEN_EXPIRY_RECOVERY_EXECUTED")
}
