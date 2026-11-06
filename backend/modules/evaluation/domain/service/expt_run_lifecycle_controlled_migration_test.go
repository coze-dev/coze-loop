// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	goredis "github.com/redis/go-redis/v9"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// controlledMigrationEnv 是一套真实 Redis + 真实 MySQL 的最小受控迁移环境。
// 不触碰线上地址：地址只能来自测试专用的环境变量，且库名必须完全一致。
type controlledMigrationEnv struct {
	ctx    context.Context
	client infraredis.Cmdable
	raw    *goredis.Client
	conn   *gorm.DB
	repo   repo.IExperimentRepo
	space  int64
	expt   int64
	run    int64
	key    string
}

func newControlledMigrationEnv(t *testing.T) *controlledMigrationEnv {
	t.Helper()
	addr := os.Getenv("EXPT_RETRY_REAL_REDIS_ADDR")
	dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
	require.NotEmpty(t, addr, "real Redis must be configured; the controlled migration cannot skip")
	require.NotEmpty(t, dsn, "real MySQL must be configured; the controlled migration cannot skip")
	ctx := context.Background()

	client, err := infraredis.NewClient(&goredis.Options{Addr: addr})
	require.NoError(t, err)
	raw, ok := infraredis.Unwrap(client)
	require.True(t, ok)
	require.NoError(t, raw.Ping(ctx).Err())

	provider, err := db.NewDB(mysqldriver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn := provider.NewSession(ctx, db.WithMaster())
	var identity struct{ Version, Schema string }
	require.NoError(t, conn.Raw("SELECT VERSION() AS version,DATABASE() AS `schema`").Scan(&identity).Error)
	require.Equal(t, "expt_running_retry_test", identity.Schema, "refuse to run against a non-test database")

	base := int64(890000000000000000) + time.Now().UnixNano()%1000000000000
	env := &controlledMigrationEnv{
		ctx:    ctx,
		client: client,
		raw:    raw,
		conn:   conn,
		space:  base + 1,
		expt:   base + 2,
		run:    base + 3,
		key:    fmt.Sprintf("expt_run_mutex_lock:%d", base+2),
	}
	env.repo = experimentrepo.NewExptRepo(experimentmysql.NewExptDAO(provider), experimentmysql.NewExptEvaluatorRefDAO(provider), nil)
	t.Cleanup(func() {
		_ = conn.Exec("DELETE FROM experiment WHERE space_id=? AND id=?", env.space, env.expt).Error
		_ = raw.Del(ctx, env.key).Err()
		if sqlDB, dbErr := conn.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		_ = raw.Close()
	})
	t.Logf("CONTROLLED_MIGRATION_REAL_ENV version=%s schema=%s", identity.Version, identity.Schema)
	return env
}

func (e *controlledMigrationEnv) seedExperiment(t *testing.T, status entity.ExptStatus, latestRunID int64, endAt *time.Time) *entity.Experiment {
	t.Helper()
	var end any
	if endAt != nil {
		end = *endAt
	}
	require.NoError(t, e.conn.Exec("INSERT INTO experiment(id,space_id,latest_run_id,status,expt_type,end_at) VALUES(?,?,?,?,?,?)",
		e.expt, e.space, latestRunID, status, entity.ExptType_Offline, end).Error)
	stored, err := e.repo.GetByID(e.ctx, e.expt, e.space)
	require.NoError(t, err)
	require.NotNil(t, stored)
	return stored
}

// controlledStaticHolderMigration 是 runbook 中“操作者动作”的可执行版本：
// 只允许在排空完成后，按固定旧值 "evaluation" 做一次 CAS 删除。
// 锁 owner 已变成 R2 数字 runID（Create 之前窗口）、数据库 latest/终态/EndAt
// 任一不符（旧写入未排空）时都必须拒绝，并保持当前锁原样。
func controlledStaticHolderMigration(ctx context.Context, client infraredis.Cmdable, repo repo.IExperimentRepo, exptID, spaceID, runID int64) (bool, string, error) {
	key := fmt.Sprintf("expt_run_mutex_lock:%d", exptID)
	owner, err := client.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return false, "fixed legacy holder is already absent; do not replay a migration twice", nil
	}
	if err != nil {
		return false, "", err
	}
	if owner != legacyStaticHolderValue {
		return false, fmt.Sprintf("lock owner %q is not the drained fixed legacy holder; R2 Create-before window or a live cleanup lease is present", owner), nil
	}
	current, err := repo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return false, "", err
	}
	if current == nil || current.LatestRunID != runID || !entity.IsExptFinished(current.Status) || current.EndAt == nil {
		return false, "main database does not prove drained latest terminal R1; a writer may still be in flight", nil
	}
	released, err := lock.NewRedisLockerWithHolder(client, legacyStaticHolderValue).UnlockWithValue(ctx, key, legacyStaticHolderValue)
	if err != nil {
		return false, "", err
	}
	if !released {
		return false, "fixed legacy holder changed during migration; keep the current owner", nil
	}
	return true, "", nil
}

// 排空完成后的受控迁移验收：R1 是 latest 且终态、Redis 仍是固定 "evaluation"、
// 不存在 R2 Create 前窗口。操作者按固定旧值删除后重放 R1 cleanup，必须取得
// 独立 cleanup 锁并完成额度和通知清理；首次失败必须保持 MQRetry（AlreadyFinished
// 不得被当成一律 ACK），再次重放可以完成。
func TestControlledStaticHolderMigrationAfterDrain(t *testing.T) {
	env := newControlledMigrationEnv(t)
	endAt := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	stored := env.seedExperiment(t, entity.ExptStatus_Success, env.run, &endAt)
	require.NoError(t, env.client.Set(env.ctx, env.key, legacyStaticHolderValue, legacyStaticZombieTTL).Err())

	owner, err := env.client.Get(env.ctx, env.key).Result()
	require.NoError(t, err)
	require.Equal(t, legacyStaticHolderValue, owner, "drained state must still hold the untouched fixed legacy value")
	if _, parseErr := strconv.ParseInt(owner, 10, 64); parseErr == nil {
		t.Fatalf("test state must not contain an R2 admission lease, got %q", owner)
	}

	operator := lock.NewRedisLockerWithHolder(env.client, legacyStaticHolderValue)
	released, err := operator.UnlockWithValue(env.ctx, env.key, legacyStaticHolderValue)
	require.NoError(t, err)
	require.True(t, released, "operator must delete exactly the fixed legacy value after drain")
	exists, err := env.client.Exists(env.ctx, env.key).Result()
	require.NoError(t, err)
	require.Zero(t, exists)

	ctrl := gomock.NewController(t)
	quotaCalls := 0
	quotaReleased := false
	var leaseOwners []string
	quota := repomocks.NewMockQuotaRepo(ctrl)
	notificationKey := fmt.Sprintf("finish-cleanup:%d:%d", env.expt, env.run)
	quota.EXPECT().CreateOrUpdate(gomock.Any(), env.space, gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ int64, mutate func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
			quotaCalls++
			leaseOwner, ownerErr := env.client.Get(ctx, env.key).Result()
			require.NoError(t, ownerErr, "cleanup must hold an independent lifecycle lease")
			require.NotEqual(t, legacyStaticHolderValue, leaseOwner)
			require.NotEqual(t, strconv.FormatInt(env.run, 10), leaseOwner)
			ttl, ttlErr := env.raw.TTL(env.ctx, env.key).Result()
			require.NoError(t, ttlErr)
			require.Greater(t, ttl, time.Duration(0), "independent cleanup lease must have a live TTL")
			leaseOwners = append(leaseOwners, leaseOwner)
			if quotaCalls == 1 {
				return errors.New("quota backend unavailable")
			}
			next, changed, mutateErr := mutate(&entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{env.expt: time.Now().Unix()}})
			quotaReleased = changed && next != nil
			return mutateErr
		}).Times(2)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), notificationKey).Return(false, nil).Times(2)
	idem.EXPECT().Set(gomock.Any(), notificationKey, gomock.Any()).Return(nil).Times(1)

	manager := &ExptMangerImpl{
		runFinishRepo:     &finishRepoProbe{result: &entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishAlreadyFinished, Experiment: stored}},
		manualRetryLocker: func() lock.ILocker { return filterRealTestLocker(env.client) },
		exptRepo:          env.repo,
		quotaRepo:         quota,
		idem:              idem,
	}
	event := &entity.ExptScheduleEvent{ExptID: env.expt, ExptRunID: env.run, SpaceID: env.space, Session: &entity.Session{}}

	_, err = manager.FinishCurrentRunWithHeldTickLock(env.ctx, event)
	require.True(t, errno.NeedMQRetry(err), "AlreadyFinished cleanup failure must remain retryable, not ACK")
	require.False(t, quotaReleased)
	exists, err = env.client.Exists(env.ctx, env.key).Result()
	require.NoError(t, err)
	require.Zero(t, exists, "a failed cleanup attempt must release its own lifecycle lease")

	_, err = manager.FinishCurrentRunWithHeldTickLock(env.ctx, event)
	require.NoError(t, err)
	require.True(t, quotaReleased, "replayed R1 cleanup must release quota")
	require.Equal(t, 2, quotaCalls)
	require.Len(t, leaseOwners, 2, "both attempts must acquire a fresh independent cleanup lease")
	require.NotEqual(t, leaseOwners[0], leaseOwners[1], "cleanup leases must not reuse the legacy value")
	exists, err = env.client.Exists(env.ctx, env.key).Result()
	require.NoError(t, err)
	require.Zero(t, exists, "completed cleanup must release the lifecycle lease")
	t.Log("CONTROLLED_STATIC_HOLDER_MIGRATION_AFTER_DRAIN_EXECUTED quota_and_notification_replayed_with_independent_leases")
}

// 负向屏障：R2 在 Create 之前已用数字 runID 取得准入锁，或 R1 的旧写入仍未
// 排空（数据库仍在 Processing）时，受控迁移必须拒绝，且不得删除当前 owner。
func TestControlledStaticHolderMigrationRejectedWhenWriterInFlight(t *testing.T) {
	t.Run("r2_pre_create_window", func(t *testing.T) {
		env := newControlledMigrationEnv(t)
		endAt := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
		stored := env.seedExperiment(t, entity.ExptStatus_Success, env.run, &endAt)
		r2 := env.run + 1
		r2Owner := strconv.FormatInt(r2, 10)
		require.NoError(t, env.client.Set(env.ctx, env.key, r2Owner, legacyStaticZombieTTL).Err())

		migrated, reason, err := controlledStaticHolderMigration(env.ctx, env.client, env.repo, env.expt, env.space, env.run)
		require.NoError(t, err)
		require.False(t, migrated, "R2 acquired the admission lease but has not created its run yet")
		require.Contains(t, reason, "R2 Create-before window")
		require.Equal(t, env.run, stored.LatestRunID)
		owner, err := env.client.Get(env.ctx, env.key).Result()
		require.NoError(t, err)
		require.Equal(t, r2Owner, owner, "R2 admission lease must be retained")
		t.Log("CONTROLLED_MIGRATION_REJECTED_R2_PRE_CREATE_WINDOW_LEASE_RETAINED")
	})

	t.Run("legacy_writer_not_drained", func(t *testing.T) {
		env := newControlledMigrationEnv(t)
		stored := env.seedExperiment(t, entity.ExptStatus_Processing, env.run, nil)
		require.NoError(t, env.client.Set(env.ctx, env.key, legacyStaticHolderValue, legacyStaticZombieTTL).Err())

		migrated, reason, err := controlledStaticHolderMigration(env.ctx, env.client, env.repo, env.expt, env.space, env.run)
		require.NoError(t, err)
		require.False(t, migrated, "a non-terminal main database row means the old writer has not drained")
		require.Contains(t, reason, "writer may still be in flight")
		require.Equal(t, entity.ExptStatus_Processing, stored.Status)
		owner, err := env.client.Get(env.ctx, env.key).Result()
		require.NoError(t, err)
		require.Equal(t, legacyStaticHolderValue, owner, "in-flight writer's lifecycle lease must be retained")
		t.Log("CONTROLLED_MIGRATION_REJECTED_LEGACY_WRITER_IN_FLIGHT_LEASE_RETAINED")
	})
}
