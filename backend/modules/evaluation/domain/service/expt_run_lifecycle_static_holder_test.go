// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	goredis "github.com/redis/go-redis/v9"
)

// 升级前的生产 holder 不是 <hostname>-<xid>，而是共享 locker 的固定
// "evaluation"（application.NewLock），TTL 为 zombie 默认 36h，且不续期。
const (
	legacyStaticHolderValue = "evaluation"
	legacyStaticZombieTTL   = 36 * time.Hour
)

// The former age-based static-holder migration tests asserted a stronger
// protocol than the fixed evaluation value can provide. Ordinary completion,
// numeric compatibility, current SQL identity and completed replay are exercised
// by expt_run_lock_protocol_regression_test.go instead.
// This test retains real Redis TTL/CAS/renewal and real SQL cleanup coverage.
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
	quota.EXPECT().CreateOrUpdate(gomock.Any(), space, gomock.Any(), gomock.Any()).DoAndReturn(func(opCtx context.Context, _ int64, mutate func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
		owner, ownerErr := raw.Get(opCtx, key).Result()
		require.NoError(t, ownerErr)
		require.Equal(t, legacyStaticHolderValue, owner)
		next, changed, err := mutate(&entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{expt: time.Now().Unix()}})
		quotaReleased = changed && next != nil
		return err
	})
	idem := &cleanupTestIdem{}
	manager := &ExptMangerImpl{
		manualRetryLocker: func() lock.ILocker { return filterRealTestLocker(client) },
		exptRepo:          repo,
		quotaRepo:         quota,
		idem:              idem,
	}
	// Current completion uses the original fixed holder and does not wait for TTL.
	event := &entity.ExptScheduleEvent{ExptID: expt, ExptRunID: run, SpaceID: space, Session: &entity.Session{}}
	require.NoError(t, completeNaturalFixture(manager, ctx, event, stored))
	require.True(t, quotaReleased)
	require.True(t, idem.seen)
	exists, err := raw.Exists(ctx, key).Result()
	require.NoError(t, err)
	require.Zero(t, exists, "current cleanup must release the original static holder")
	// B has acquired the same fixed value but has not advanced SQL yet. A's
	// successfully completed marker must suppress this known duplicate.
	require.NoError(t, raw.Set(ctx, key, legacyStaticHolderValue, legacyStaticZombieTTL).Err())
	require.NoError(t, completeNaturalFixture(manager, ctx, event, stored))
	holder, getErr := raw.Get(ctx, key).Result()
	require.NoError(t, getErr)
	require.Equal(t, legacyStaticHolderValue, holder)
	t.Log("REAL_REDIS_MYSQL_STATIC_HOLDER_COMPLETION_AND_COMPLETED_REPLAY_EXECUTED")
}
