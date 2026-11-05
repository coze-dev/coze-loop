// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	quotadao "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

type completeOwnerFixture struct {
	f       *lateWriteFixture
	manager *ExptMangerImpl
	server  *miniredis.Miniredis
	quota   quotadao.IQuotaDAO
	session *entity.Session
	key     string
}

func newCompleteOwnerFixture(t *testing.T) *completeOwnerFixture {
	t.Helper()
	f := newLateWriteFixture(t, "")
	ctx := context.Background()
	for _, table := range []any{&model.ExptItemResult{}, &model.ExptItemResultRunLog{}} {
		require.NoError(t, f.conn.Model(table).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Success)).Error)
	}
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=?", f.space).Update("result_state", int32(entity.ExptItemResultStateResulted)).Error)
	for _, table := range []any{&model.ExptTurnResult{}, &model.ExptTurnResultRunLog{}} {
		require.NoError(t, f.conn.Model(table).Where("space_id=?", f.space).Update("status", int32(entity.TurnRunState_Success)).Error)
	}
	finishRepo := exptrepo.NewExptRunFinishRepo(f.provider)
	finished, err := finishRepo.FinishCurrentRun(ctx, &entity.ExptRunFinishRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run})
	require.NoError(t, err)
	require.Equal(t, entity.ExptRunFinishFinished, finished.Disposition)
	client, server := newTickOwnerFixture(t)
	quota := quotadao.NewQuotaDAO(client)
	require.NoError(t, quota.SetQuotaSpaceExpt(ctx, f.space, &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{f.expt: 111, f.expt + 99: 222}}))
	ctrl := gomock.NewController(t)
	config := confmocks.NewMockIConfiger(ctrl)
	// A configuration increase must not manufacture evidence about the fixed
	// holder's acquisition time from its current TTL.
	config.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1200}).AnyTimes()
	metric := metricsmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitExptExecRun(f.space, gomock.Any()).AnyTimes()
	manager := &ExptMangerImpl{
		mutex:             lock.NewRedisLockerWithHolder(client, "evaluation"),
		manualRetryLocker: func() lock.ILocker { return filterRealTestLocker(client) },
		runFinishRepo:     finishRepo,
		exptRepo:          f.experimentRepo(),
		runLogRepo:        exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(f.provider)),
		turnResultRepo:    f.turnRepo(),
		quotaRepo:         exptrepo.NewQuotaService(quota, lock.NewRedisLocker(client)),
		configer:          config,
		mtr:               metric,
	}
	return &completeOwnerFixture{f: f, manager: manager, server: server, quota: quota, session: &entity.Session{UserID: "complete-owner-test"}, key: manager.makeExptMutexLockKey(f.expt)}
}

func (p *completeOwnerFixture) complete() error {
	return p.manager.CompleteRun(context.Background(), p.f.expt, p.f.run, p.f.space, p.session)
}

func TestCompleteRunOwnerMigratesMatureLegacyAndOwnNumeric(t *testing.T) {
	for _, kind := range []string{"legacy_xid", "own_numeric"} {
		t.Run(kind, func(t *testing.T) {
			p := newCompleteOwnerFixture(t)
			holder := fmt.Sprint(p.f.run)
			if kind == "legacy_xid" {
				holder = legacyHolderAt(t, time.Now().Add(-10*time.Minute))
			}
			require.NoError(t, p.server.Set(p.key, holder))
			p.server.SetTTL(p.key, time.Hour)
			require.NoError(t, p.complete(), "CompleteRun must use the same precise owner migration as natural finish")
			require.False(t, p.server.Exists(p.key))
			q, err := p.quota.GetQuotaSpaceExpt(context.Background(), p.f.space)
			require.NoError(t, err)
			require.NotContains(t, q.ExptID2RunTime, p.f.expt)
			require.Equal(t, int64(222), q.ExptID2RunTime[p.f.expt+99])
			snapshot := p.f.snapshot()
			require.NoError(t, p.complete(), "cleanup must remain replayable after its exact owner is released")
			require.Equal(t, snapshot, p.f.snapshot())
			current, err := p.manager.exptRepo.GetByID(context.Background(), p.f.expt, p.f.space)
			require.NoError(t, err)
			require.Equal(t, p.f.run, current.LatestRunID)
			require.True(t, entity.IsExptFinished(current.Status))
			require.NotNil(t, current.EndAt)
			t.Logf("REAL_MYSQL_COMPLETE_OWNER kind=%s migrated=true other_experiment_quota_preserved=true", kind)
		})
	}
}

func TestCompleteRunOwnerRetainsUnprovenAndForeignLeases(t *testing.T) {
	for _, kind := range []string{"static_config_changed", "fresh_xid", "foreign_numeric", "cleanup", "noncurrent"} {
		t.Run(kind, func(t *testing.T) {
			p := newCompleteOwnerFixture(t)
			holder := "evaluation"
			switch kind {
			case "fresh_xid":
				holder = legacyHolderAt(t, time.Now())
			case "foreign_numeric":
				holder = fmt.Sprint(p.f.run + 1)
			case "cleanup":
				holder = lock.LifecycleCleanupHolderPrefix + xid.New().String()
			case "noncurrent":
				holder = fmt.Sprint(p.f.run + 1)
				require.NoError(t, p.f.conn.Model(&model.Experiment{}).Where("space_id=?", p.f.space).Update("latest_run_id", p.f.run+1).Error)
			}
			require.NoError(t, p.server.Set(p.key, holder))
			p.server.SetTTL(p.key, 10*time.Minute)
			before := p.f.snapshot()
			err := p.complete()
			if kind == "noncurrent" {
				require.NoError(t, err)
			} else {
				require.True(t, errno.NeedMQRetry(err), "a retained lifecycle holder must not report successful cleanup")
			}
			require.Equal(t, before, p.f.snapshot())
			owner, err := p.server.Get(p.key)
			require.NoError(t, err)
			require.Equal(t, holder, owner)
			q, err := p.quota.GetQuotaSpaceExpt(context.Background(), p.f.space)
			require.NoError(t, err)
			require.Equal(t, map[int64]int64{p.f.expt: 111, p.f.expt + 99: 222}, q.ExptID2RunTime)
		})
	}
}

type completeOwnerPausedRunRepo struct {
	repo.IExptRunLogRepo
	entered chan struct{}
	resume  chan struct{}
}

func (r *completeOwnerPausedRunRepo) Create(ctx context.Context, run *entity.ExptRunLog) error {
	close(r.entered)
	<-r.resume
	return r.IExptRunLogRepo.Create(ctx, run)
}

func TestCompleteRunOwnerLegacyLogRunABAStaysClosed(t *testing.T) {
	for _, numeric := range []bool{false, true} {
		t.Run(fmt.Sprintf("numeric_%v", numeric), func(t *testing.T) {
			p := newCompleteOwnerFixture(t)
			// Start R2 through the real retained old LogRun contract, then pause
			// exactly after SETNX and before the actual MySQL run insert.
			paused := &completeOwnerPausedRunRepo{IExptRunLogRepo: p.manager.runLogRepo, entered: make(chan struct{}), resume: make(chan struct{})}
			var once sync.Once
			unblock := func() { once.Do(func() { close(paused.resume) }) }
			t.Cleanup(unblock)
			writer := *p.manager
			writer.runLogRepo = paused
			if !numeric {
				writer.runFinishRepo = nil
			}
			done := make(chan error, 1)
			go func() {
				done <- writer.LogRun(context.Background(), p.f.expt, p.f.run+1, entity.EvaluationModeRetryAll, p.f.space, nil, p.session)
			}()
			select {
			case <-paused.entered:
			case err := <-done:
				t.Fatalf("R2 failed before the SETNX→Create barrier: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatal("R2 did not enter the real run insert barrier")
			}
			current, err := p.manager.exptRepo.GetByID(context.Background(), p.f.expt, p.f.space)
			require.NoError(t, err)
			require.Equal(t, p.f.run, current.LatestRunID)
			require.True(t, entity.IsExptFinished(current.Status))
			before := p.f.snapshot()
			owner, err := p.server.Get(p.key)
			require.NoError(t, err)
			if numeric {
				require.Equal(t, fmt.Sprint(p.f.run+1), owner)
			} else {
				require.Equal(t, "evaluation", owner)
			}
			// A config increase makes a newly acquired fixed lease appear old to
			// configuredTTL - remainingTTL. It remains R2's live lease regardless.
			p.server.SetTTL(p.key, 10*time.Minute)
			require.True(t, errno.NeedMQRetry(p.complete()))
			require.Equal(t, before, p.f.snapshot())
			still, err := p.server.Get(p.key)
			require.NoError(t, err)
			require.Equal(t, owner, still)
			q, err := p.quota.GetQuotaSpaceExpt(context.Background(), p.f.space)
			require.NoError(t, err)
			require.Equal(t, int64(111), q.ExptID2RunTime[p.f.expt], "R1 cleanup cannot release quota while R2 owns lifecycle protection")
			unblock()
			require.NoError(t, <-done)
			current, err = p.manager.exptRepo.GetByID(context.Background(), p.f.expt, p.f.space)
			require.NoError(t, err)
			require.Equal(t, p.f.run+1, current.LatestRunID)
			require.NoError(t, p.complete(), "stale R1 completion must converge without touching R2")
			still, err = p.server.Get(p.key)
			require.NoError(t, err)
			require.Equal(t, owner, still)
			t.Logf("REAL_MYSQL_LEGACY_LOGRUN_ABA numeric=%v old_sql_terminal=true r2_lease_preserved=true", numeric)
		})
	}
}
