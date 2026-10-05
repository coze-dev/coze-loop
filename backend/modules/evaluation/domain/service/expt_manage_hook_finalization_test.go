// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gg/gptr"
	driver "github.com/go-sql-driver/mysql"
	red "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	mm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	em "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	rm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

type finalizationManagerFixture struct {
	*managerFinalizationData
	manager            *ExptMangerImpl
	base               IExptManager
	deps               ExptManagerFinalizationDependencies
	redis              redis.Cmdable
	quota              repo.QuotaRepo
	notifications      int
	filterRefreshes    []*entity.ExptTurnResultFilterEvent
	filterRefreshError error
}

func newFinalizationManagerFixture(t *testing.T, database ...string) *finalizationManagerFixture {
	t.Helper()
	f := &finalizationManagerFixture{managerFinalizationData: newManagerFinalizationData(t, database...)}
	c, err := redis.NewClient(&red.Options{Addr: miniredis.RunT(t).Addr()})
	require.NoError(t, err)
	f.redis = c
	if raw, ok := redis.Unwrap(c); ok {
		t.Cleanup(func() { require.NoError(t, raw.Close()) })
	}
	locker := lock.NewRedisLocker(c)
	f.quota = exptinfra.NewQuotaService(dao.NewQuotaDAO(c), locker)
	ctrl := gomock.NewController(t)
	runs := rm.NewMockIExptRunLogRepo(ctrl)
	turns := rm.NewMockIExptTurnResultRepo(ctrl)
	pub := em.NewMockExptEventPublisher(ctrl)
	metrics := mm.NewMockExptMetric(ctrl)
	aggr := sm.NewMockExptAggrResultService(ctrl)
	runs.EXPECT().Get(gomock.Any(), f.expt, f.key.RunID).Return(&entity.ExptRunLog{ID: f.key.RunID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, Status: 3}, nil).AnyTimes()
	runs.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	turns.EXPECT().ListTurnResult(gomock.Any(), f.space, f.expt, nil, gomock.Any(), false).Return([]*entity.ExptTurnResult{{Status: 1}}, int64(1), nil).AnyTimes()
	pub.EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.ExptLifecycleEvent, _ *time.Duration, _ string) error {
		f.notifications++
		require.Equal(t, f.key.RunID, *e.ExptRunID)
		return nil
	}).AnyTimes()
	pub.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptTurnResultFilterEvent, _ *time.Duration) error {
		require.Equal(t, f.expt, event.ExperimentID)
		require.Equal(t, f.space, event.SpaceID)
		cloned := *event
		cloned.ItemID = append([]int64(nil), event.ItemID...)
		f.filterRefreshes = append(f.filterRefreshes, &cloned)
		return f.filterRefreshError
	}).AnyTimes()
	metrics.EXPECT().EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	aggr.EXPECT().PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	f.base = NewExptManager(nil, nil, runs, nil, nil, nil, turns, nil, f.quota, locker, nil, pub, nil, nil, metrics, nil, nil, nil, nil, nil, nil, aggr, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	f.deps = ExptManagerFinalizationDependencies{Runs: f.repo, Repository: exptinfra.NewHookFinalizationRepo(f.p), Owners: exptinfra.NewHookFinalizationOwnerReader(c), ExecutionScope: "local"}
	f.manager, err = NewExptManagerWithHookFinalization(f.base, f.deps)
	require.NoError(t, err)
	_, err = c.Set(context.Background(), fmt.Sprintf("expt_run_mutex_lock:%d", f.expt), fmt.Sprintf("hook_run:%d:0123456789abcdef0123456789abcdef", f.key.RunID), time.Hour).Result()
	require.NoError(t, err)
	require.NoError(t, f.quota.CreateOrUpdate(context.Background(), f.space, func(q *entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error) {
		q.ExptID2RunTime[f.expt] = 123
		return q, true, nil
	}, &entity.Session{UserID: "user"}))
	return f
}

func TestHookFinalizationManagerNormalAndReplay(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, &entity.Session{UserID: "user"}))
	state, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, entity.HookFinalizeNone, state.State.Finalize)
	require.False(t, state.State.After.Activated)
	require.Zero(t, f.notifications)
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, gptr.Of(f.key.RunID), f.space, &entity.Session{UserID: "user"}))
	state2, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, entity.HookFinalizeCommitted, state2.State.Finalize)
	require.True(t, state2.State.After.Activated)
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, gptr.Of(f.key.RunID), f.space, nil))
	require.Equal(t, state2, finalizationRead(t, f))
	require.Equal(t, 1, f.notifications)
}

var finalizationTestIDs = func() *atomic.Int64 { v := new(atomic.Int64); v.Store(time.Now().UnixMicro()); return v }()

type managerFinalizationData struct {
	p           db.Provider
	sql         *gorm.DB
	repo        repo.IHookRepo
	space, expt int64
	key         entity.HookRunKey
}

func newManagerFinalizationData(t *testing.T, database ...string) *managerFinalizationData {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_EXECUTION_DSN")
	expectedDB := "hook_7378265404_execution"
	if len(database) > 0 && database[0] == "tx" {
		dsn = os.Getenv("HOOK_MYSQL_TX_DSN")
		expectedDB = "hook_7378265404_tx"
	}
	if dsn == "" {
		t.Skip("requires isolated HOOK_MYSQL_EXECUTION_DSN")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, expectedDB, cfg.DBName)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "/Users/bytedance/fornax_workspace/_local/scratch/hook-mysql-20260922/m.sock", cfg.Addr)
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	f := &managerFinalizationData{p: p, sql: p.NewSession(context.Background()), repo: exptinfra.NewHookRunRepo(p), space: finalizationTestIDs.Add(1), expt: finalizationTestIDs.Add(1)}
	f.key = entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: finalizationTestIDs.Add(1)}
	pool, err := f.sql.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(8)
	t.Cleanup(func() {
		for _, table := range []string{"expt_turn_result", "expt_item_result", "expt_turn_result_run_log", "expt_item_result_run_log", "expt_lifecycle_run_item", "expt_lifecycle_hook_run", "expt_lifecycle_run", "expt_run_log", "expt_stats"} {
			require.NoError(t, f.sql.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
		}
		require.NoError(t, f.sql.Unscoped().Delete(&model.Experiment{}, "id=? AND space_id=?", f.expt, f.space).Error)
		require.NoError(t, pool.Close())
	})
	require.NoError(t, f.sql.Create(&model.Experiment{ID: f.expt, SpaceID: f.space, ExptType: 1, Status: 3, Name: "finalization", SchedulerMode: "legacy"}).Error)
	require.NoError(t, f.sql.Create(&model.ExptStats{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, PendingCnt: 1, CreditCost: 7}).Error)
	if len(database) > 0 && database[0] == "empty" {
		return f
	}
	_, err = f.repo.CreateRunWithHooks(context.Background(), entity.HookCreateRunInput{
		Key: f.key, RunLog: &entity.ExptRunLog{ID: f.key.RunID, ExptRunID: f.key.RunID, SpaceID: f.space, ExptID: f.expt, Mode: 1, Status: 3, CreatedBy: "user"},
		Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1}, Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", KeyID: "key", ExecutionScope: "local"},
		After:    &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(f.key.RunID), IdempotencyKey: fmt.Sprint(f.key.RunID)},
	})
	require.NoError(t, err)
	item := entity.HookPlanItem{ID: finalizationTestIDs.Add(1), ItemID: finalizationTestIDs.Add(1), SourceSpaceID: f.space, EvalSetID: 71}
	appended, err := f.repo.AppendPlanPage(context.Background(), entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key}, Items: []entity.HookPlanItem{item}})
	require.NoError(t, err)
	digest, err := entity.AppendHookPlanDigest(entity.NewHookPlanDigest(), []entity.HookPlanItem{item})
	require.NoError(t, err)
	_, err = f.repo.FinishPlan(context.Background(), entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: appended.Run.Version}, Count: 1, Hash: digest.Hash})
	require.NoError(t, err)
	manifest := entity.HookExecutionManifest{Version: 1, Key: f.key, Frozen: item, ItemResultID: finalizationTestIDs.Add(1), ItemRunLogID: finalizationTestIDs.Add(1), Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, ResultID: finalizationTestIDs.Add(1)}}}
	raw, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", item.ID).UpdateColumn("execution_manifest", raw).Error)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("execution_initialized", true).Error)
	require.NoError(t, f.sql.Create(&model.ExptItemResultRunLog{ID: manifest.ItemRunLogID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, Status: 2}).Error)
	require.NoError(t, f.sql.Create(&model.ExptTurnResultRunLog{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, TurnID: 0, Status: 1}).Error)
	require.NoError(t, f.sql.Create(&model.ExptItemResult{ID: manifest.ItemResultID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, ItemIdx: gptr.Of(int32(0)), Status: int32(entity.ItemRunState_Success)}).Error)
	require.NoError(t, f.sql.Create(&model.ExptTurnResult{ID: manifest.Turns[0].ResultID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, TurnID: 0, TurnIdx: gptr.Of(int32(0)), Status: int32(entity.TurnRunState_Success)}).Error)
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", manifest.ItemRunLogID).UpdateColumn("result_state", int32(entity.ExptItemResultStateResulted)).Error)
	return f
}
