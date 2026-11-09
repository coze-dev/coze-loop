// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	goredis "github.com/redis/go-redis/v9"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// 评审候选 2 的真实 Redis + MySQL 反例。
//
// 归属不变量：
//  1. 旧 LogRun 写入的固定 "evaluation" holder 不携带 acquisition token，
//     相同 value 的 CAS 无法区分“R1 遗留”与“R2 刚取得”。
//  2. 因此 R1 终态 EndAt 只能证明 R1 结束多久，不能证明当前 Redis owner 是 R1；
//     任何缺失归属证明的路径都不得删除 holder。
//  3. R1 清理必须保持可重放（MQRetry），不能为了不误删而把 AlreadyFinished 全 ACK。

type lifecycleBlockingRunLogRepo struct {
	repo.IExptRunLogRepo
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *lifecycleBlockingRunLogRepo) Create(ctx context.Context, runLog *entity.ExptRunLog) error {
	r.once.Do(func() { close(r.entered) })
	<-r.release
	return r.IExptRunLogRepo.Create(ctx, runLog)
}

type lifecycleQuotaReleaseProbe struct {
	calls atomic.Int32
}

func (p *lifecycleQuotaReleaseProbe) CreateOrUpdate(context.Context, int64, func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), *entity.Session) error {
	p.calls.Add(1)
	return nil
}

type lifecycleCounterexampleFixture struct {
	t        *testing.T
	ctx      context.Context
	provider db.Provider
	conn     *gorm.DB
	client   infraredis.Cmdable
	raw      *goredis.Client
	repo     repo.IExperimentRepo
	space    int64
	expt     int64
	r1       int64
	r2       int64
	key      string
	quota    *lifecycleQuotaReleaseProbe
}

func newLifecycleCounterexampleFixture(t *testing.T) *lifecycleCounterexampleFixture {
	t.Helper()
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

	base := int64(990000000000000000) + time.Now().UnixNano()%1000000000000
	f := &lifecycleCounterexampleFixture{
		t: t, ctx: ctx, provider: provider, conn: conn, client: client, raw: raw,
		space: base + 1, expt: base + 2, r1: base + 3, r2: base + 4,
	}
	f.key = fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)
	f.repo = experimentrepo.NewExptRepo(experimentmysql.NewExptDAO(provider), experimentmysql.NewExptEvaluatorRefDAO(provider), nil)
	f.quota = &lifecycleQuotaReleaseProbe{}
	t.Cleanup(func() {
		require.NoError(t, conn.Exec("DELETE FROM expt_run_log WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
		require.NoError(t, conn.Exec("DELETE FROM experiment WHERE space_id=? AND id=?", f.space, f.expt).Error)
		require.NoError(t, raw.Del(ctx, f.key).Err())
		sqlDB, dbErr := conn.DB()
		require.NoError(t, dbErr)
		require.NoError(t, sqlDB.Close())
		require.NoError(t, raw.Close())
	})
	return f
}

func (f *lifecycleCounterexampleFixture) seedFinishedR1() {
	f.t.Helper()
	endAt := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	require.NoError(f.t, f.conn.Exec("INSERT INTO experiment(id,space_id,created_by,name,experiment_group_key,description,latest_run_id,status,expt_type,end_at) VALUES(?,?,?,?,?,?,?,?,?,?)",
		f.expt, f.space, "", "", "", "", f.r1, entity.ExptStatus_Success, entity.ExptType_Offline, endAt).Error)
	require.NoError(f.t, f.conn.Exec("INSERT INTO expt_run_log(id,space_id,created_by,expt_id,expt_run_id,mode,status) VALUES(?,?,?,?,?,?,?)",
		f.r1, f.space, "", f.expt, f.r1, entity.EvaluationModeSubmit, entity.ExptStatus_Success).Error)
	// R1 的旧 lifecycle key 已释放，正是旧 writer 可被 R2 以同值重新 SETNX 的前提。
	require.NoError(f.t, f.raw.Del(f.ctx, f.key).Err())
}

func (f *lifecycleCounterexampleFixture) latestRunID() int64 {
	f.t.Helper()
	var latest int64
	require.NoError(f.t, f.conn.Raw("SELECT latest_run_id FROM experiment WHERE id=? AND space_id=?", f.expt, f.space).Scan(&latest).Error)
	return latest
}

func (f *lifecycleCounterexampleFixture) currentManager() *ExptMangerImpl {
	f.t.Helper()
	ctrl := gomock.NewController(f.t)
	cfg := componentmocks.NewMockIConfiger(ctrl)
	cfg.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{ZombieIntervalSecond: 36 * 3600}).AnyTimes()
	metric := metricsmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitExptExecRun(gomock.Any(), gomock.Any()).AnyTimes()
	return &ExptMangerImpl{
		runFinishRepo: experimentrepo.NewExptRunFinishRepo(f.provider),
		manualRetryLocker: func() lock.ILocker {
			// 与生产 NewManualRetryLockerFactory 一致：新 cleanup lease 带前缀。
			return lock.NewRedisLockerWithHolder(f.client, lock.LifecycleCleanupHolderPrefix+xid.New().String())
		},
		exptRepo:   f.repo,
		runLogRepo: experimentrepo.NewExptRunLogRepo(experimentmysql.NewExptRunLogDAO(f.provider)),
		quotaRepo:  f.quota,
		idem:       &cleanupTestIdem{seen: true},
		configer:   cfg,
		mtr:        metric,
	}
}

func (f *lifecycleCounterexampleFixture) legacyManager(blocking *lifecycleBlockingRunLogRepo) *ExptMangerImpl {
	f.t.Helper()
	ctrl := gomock.NewController(f.t)
	cfg := componentmocks.NewMockIConfiger(ctrl)
	cfg.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{ZombieIntervalSecond: 36 * 3600}).AnyTimes()
	metric := metricsmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitExptExecRun(gomock.Any(), gomock.Any()).AnyTimes()
	return &ExptMangerImpl{
		// runFinishRepo 为空走 LogRun 的旧版锁契约：固定 holder "evaluation"。
		mutex:      lock.NewRedisLockerWithHolder(f.client, legacyStaticHolderValue),
		configer:   cfg,
		exptRepo:   f.repo,
		runLogRepo: blocking,
		mtr:        metric,
	}
}

func lifecycleReplayR1(ctx context.Context, manager *ExptMangerImpl, f *lifecycleCounterexampleFixture) error {
	scheduler := &ExptSchedulerImpl{Manager: manager}
	return scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
		f.t.Fatal("finished R1 event must not reach execution")
		return nil
	})(ctx, &entity.ExptScheduleEvent{
		ExptID: f.expt, ExptRunID: f.r1, SpaceID: f.space,
		ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeSubmit,
		CreatedAt: time.Now().Add(-time.Hour).Unix(), Session: &entity.Session{},
	})
}

// RED（88d77a30）：R1 的旧 EndAt 让我们按 value CAS 删掉 R2 刚 SETNX 的
// "evaluation"。GREEN：静态 holder 不做无归属证明的删除，R1 清理保持 MQRetry，
// R2 恢复后照常 Create 并推进 LatestRunID。
func TestLifecycleStaticHolderDoesNotEvictConcurrentLegacyRun(t *testing.T) {
	f := newLifecycleCounterexampleFixture(t)
	f.seedFinishedR1()

	blocking := &lifecycleBlockingRunLogRepo{
		IExptRunLogRepo: experimentrepo.NewExptRunLogRepo(experimentmysql.NewExptRunLogDAO(f.provider)),
		entered:         make(chan struct{}),
		release:         make(chan struct{}),
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(blocking.release) }) }
	legacyErr := make(chan error, 1)
	legacy := f.legacyManager(blocking)
	go func() {
		legacyErr <- legacy.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeSubmit, f.space, nil, &entity.Session{})
	}()
	t.Cleanup(release)

	select {
	case <-blocking.entered:
	case <-time.After(15 * time.Second):
		t.Fatal("legacy R2 LogRun never reached Create")
	}

	// R2 已通过真实 LogRun 取得 lifecycle key，但 DB 仍完整显示 R1 与其旧 EndAt。
	owner, err := f.raw.Get(f.ctx, f.key).Result()
	require.NoError(t, err)
	require.Equal(t, legacyStaticHolderValue, owner, "legacy LogRun must SETNX the fixed evaluation holder")
	require.Equal(t, f.r1, f.latestRunID())
	var r2Rows int64
	require.NoError(t, f.conn.Raw("SELECT COUNT(*) FROM expt_run_log WHERE space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.r2).Scan(&r2Rows).Error)
	require.Zero(t, r2Rows, "R2 must pause before its run log Create")

	// 迟到的 R1 schedule：真实 HandleEventCheck -> AlreadyFinished -> cleanup。
	replayErr := lifecycleReplayR1(f.ctx, f.currentManager(), f)

	owner, err = f.raw.Get(f.ctx, f.key).Result()
	require.NoError(t, err, "R2 lifecycle holder must survive R1 AlreadyFinished replay")
	require.Equal(t, legacyStaticHolderValue, owner)
	require.True(t, errno.NeedMQRetry(replayErr), "R1 cleanup must stay replayable while R2 owns the lease")
	require.Zero(t, f.quota.calls.Load(), "R1 cleanup must not release shared quota while R2 owns the lease")
	require.Equal(t, f.r1, f.latestRunID())

	// R2 恢复：真实 Create + LatestRunID 推进必须不受 R1 cleanup 影响。
	release()
	select {
	case err := <-legacyErr:
		require.NoError(t, err, "legacy R2 LogRun must complete after the replay")
	case <-time.After(15 * time.Second):
		t.Fatal("legacy R2 LogRun did not finish")
	}
	require.Equal(t, f.r2, f.latestRunID())
	require.NoError(t, f.conn.Raw("SELECT COUNT(*) FROM expt_run_log WHERE space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.r2).Scan(&r2Rows).Error)
	require.EqualValues(t, 1, r2Rows)

	// R1 再次重放已是 NonCurrent：既不删锁也不做清理。
	require.NoError(t, lifecycleReplayR1(f.ctx, f.currentManager(), f))
	owner, err = f.raw.Get(f.ctx, f.key).Result()
	require.NoError(t, err)
	require.Equal(t, legacyStaticHolderValue, owner, "non-current R1 replay must leave R2's holder untouched")
}

// 干净的同 run 旧静态 holder（无 R2 竞争）：修复后不删 holder，但 AlreadyFinished
// 仍持续可重放；holder 自然到期后重放即可完成清理，证明没有放弃 finish 恢复。
func TestStaticSameRunCleanupRecoversAfterLeaseExpiry(t *testing.T) {
	f := newLifecycleCounterexampleFixture(t)
	f.seedFinishedR1()
	require.NoError(t, f.raw.Set(f.ctx, f.key, legacyStaticHolderValue, 700*time.Millisecond).Err())

	manager := f.currentManager()
	result, err := manager.FinishCurrentRun(f.ctx, &entity.ExptScheduleEvent{
		ExptID: f.expt, ExptRunID: f.r1, SpaceID: f.space, Session: &entity.Session{},
	})
	require.NotNil(t, result)
	require.Equal(t, entity.ExptRunFinishAlreadyFinished, result.Disposition)
	require.True(t, errno.NeedMQRetry(err), "terminal R1 replay must stay retryable while the holder is unowned")
	require.Zero(t, f.quota.calls.Load())
	owner, getErr := f.raw.Get(f.ctx, f.key).Result()
	require.NoError(t, getErr)
	require.Equal(t, legacyStaticHolderValue, owner, "unproven static holder must be kept")

	// 等租约自然到期（不删除 holder、不依赖 EndAt 版本）。
	deadline := time.Now().Add(10 * time.Second)
	for {
		n, existsErr := f.raw.Exists(f.ctx, f.key).Result()
		require.NoError(t, existsErr)
		if n == 0 {
			break
		}
		require.False(t, time.Now().After(deadline), "static holder did not expire")
		time.Sleep(100 * time.Millisecond)
	}

	require.NoError(t, lifecycleReplayR1(f.ctx, manager, f), "replay must recover after the lease expires")
	require.EqualValues(t, 1, f.quota.calls.Load(), "recovered replay must release the finished run's quota")
}

// 真实中间件上的并发重放：多个 R1 消费者同时到达，仍不得删除 R2 刚取得的 holder。
func TestLifecycleStaticHolderConcurrentReplayRace(t *testing.T) {
	f := newLifecycleCounterexampleFixture(t)
	f.seedFinishedR1()

	blocking := &lifecycleBlockingRunLogRepo{
		IExptRunLogRepo: experimentrepo.NewExptRunLogRepo(experimentmysql.NewExptRunLogDAO(f.provider)),
		entered:         make(chan struct{}),
		release:         make(chan struct{}),
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(blocking.release) }) }
	legacyErr := make(chan error, 1)
	legacy := f.legacyManager(blocking)
	go func() {
		legacyErr <- legacy.LogRun(f.ctx, f.expt, f.r2, entity.EvaluationModeSubmit, f.space, nil, &entity.Session{})
	}()
	t.Cleanup(release)
	select {
	case <-blocking.entered:
	case <-time.After(15 * time.Second):
		t.Fatal("legacy R2 LogRun never reached Create")
	}

	const workers = 4
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- lifecycleReplayR1(f.ctx, f.currentManager(), f)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.True(t, err == nil || errno.NeedMQRetry(err), "concurrent replay must not surface a hard error: %v", err)
	}

	owner, err := f.raw.Get(f.ctx, f.key).Result()
	require.NoError(t, err, "concurrent replays must not delete R2's holder")
	require.Equal(t, legacyStaticHolderValue, owner)
	require.Equal(t, f.r1, f.latestRunID())
	require.Zero(t, f.quota.calls.Load())

	release()
	select {
	case err := <-legacyErr:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("legacy R2 LogRun did not finish")
	}
	require.Equal(t, f.r2, f.latestRunID())
}
