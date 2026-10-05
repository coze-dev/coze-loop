// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type deletionQuotaMySQLFixture struct {
	p         db.Provider
	sql       *gorm.DB
	m         *ExptMangerImpl
	guard     *deletionQuotaGuard
	space     int64
	ids, runs []int64
	older     entity.HookRunKey
}

func newDeletionQuotaMySQLFixture(t *testing.T, mixed bool) *deletionQuotaMySQLFixture {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
	if dsn == "" {
		t.Skip("requires isolated HOOK_MYSQL_TX_DSN")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "hook_7378265404_tx", cfg.DBName)
	require.Equal(t, "/Users/bytedance/fornax_workspace/_local/scratch/hook-mysql-20260922/m.sock", cfg.Addr)
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	f := &deletionQuotaMySQLFixture{p: p, sql: p.NewSession(context.Background()), space: finalizationTestIDs.Add(1), ids: []int64{finalizationTestIDs.Add(1), finalizationTestIDs.Add(1)}, runs: []int64{finalizationTestIDs.Add(1), finalizationTestIDs.Add(1)}}
	pool, err := f.sql.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(4)
	t.Cleanup(func() {
		for _, table := range []string{"expt_item_result_run_log", "expt_lifecycle_run_item", "expt_lifecycle_hook_run", "expt_lifecycle_run", "expt_run_log"} {
			require.NoError(t, f.sql.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id IN ?", f.space, f.ids).Error)
		}
		require.NoError(t, f.sql.Unscoped().Where("space_id=? AND id IN ?", f.space, f.ids).Delete(&model.Experiment{}).Error)
		require.NoError(t, pool.Close())
	})
	views := make([]*entity.Experiment, 0, 2)
	for _, id := range f.ids {
		require.NoError(t, f.sql.Create(&model.Experiment{ID: id, SpaceID: f.space, Name: fmt.Sprintf("quota-delete-%d", id), ExptType: 1, EvalSetSourceType: 1, Status: 3, SchedulerMode: "enforce", SchedulerScope: "billing-scope"}).Error)
	}
	codec := hookinfra.NewStorageCodec(&managerProtector{})
	runs := exptinfra.NewHookRunRepo(p)
	if mixed {
		key := entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.ids[0], RunID: finalizationTestIDs.Add(1)}
		f.older = key
		stage := func() *entity.HookConfig {
			return &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.test/hook")}}
		}
		snapshot, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: key, ExecutionScope: "scope", CreatedAt: time.Now(), Config: &entity.LifecycleHookConf{Before: stage(), After: stage()}, Context: &spi.HookRunContext{
			WorkspaceID: gptr.Of(strconv.FormatInt(key.WorkspaceID, 10)), ExperimentID: gptr.Of(strconv.FormatInt(key.ExperimentID, 10)), RunID: gptr.Of(strconv.FormatInt(key.RunID, 10)), RunMode: gptr.Of("submit"),
			Initiator: &spi.HookInitiator{UserID: gptr.Of("actor"), IdentityType: gptr.Of("fornax_user")}, Experiment: &spi.HookExperimentRef{Name: gptr.Of("original"), Type: gptr.Of("offline")}, EvalSets: []*spi.HookEvalSetRef{}}})
		require.NoError(t, err)
		protected, err := codec.EncodeSnapshot(context.Background(), "key", snapshot)
		require.NoError(t, err)
		_, err = runs.CreateRunWithHooks(context.Background(), entity.HookCreateRunInput{Key: key, RunLog: &entity.ExptRunLog{ID: key.RunID, SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, CreatedBy: "actor", Mode: 1, Status: 2}, Snapshot: protected,
			Before: &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprintf("quota-before-%d", key.RunID), IdempotencyKey: fmt.Sprintf("before-%d", key.RunID)}, After: &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprintf("quota-after-%d", key.RunID), IdempotencyKey: fmt.Sprintf("after-%d", key.RunID)}})
		require.NoError(t, err)
	}
	f.guard = &deletionQuotaGuard{held: map[deletionQuotaKey]string{}, committed: func() bool {
		var count int64
		require.NoError(t, f.sql.Model(&model.Experiment{}).Where("space_id=? AND id IN ?", f.space, f.ids).Count(&count).Error)
		return count == 0
	}}
	for i, id := range f.ids {
		run := f.runs[i]
		item := int64(101 + i)
		require.NoError(t, f.sql.Create(&model.ExptRunLog{ID: run, SpaceID: f.space, ExptID: id, ExptRunID: run, CreatedBy: "actor", Mode: gptr.Of(int32(1)), Status: gptr.Of(int64(3))}).Error)
		require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=? AND space_id=?", id, f.space).Update("latest_run_id", run).Error)
		require.NoError(t, f.sql.Create(&model.ExptItemResultRunLog{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: id, ExptRunID: run, ItemID: item, Status: int32(entity.ItemRunState_Processing)}).Error)
		views = append(views, &entity.Experiment{ID: id, SpaceID: f.space, LatestRunID: run, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "billing-scope"})
		f.guard.held[deletionQuotaKey{"billing-scope", run, item}] = "running"
	}
	base := newTestExptManager(gomock.NewController(t))
	base.exptRepo = &batchDeleteManagerReads{rows: views}
	base.centralGuard = f.guard
	base.itemResultRepo = exptinfra.NewExptItemResultRepo(exptmysql.NewExptItemResultDAO(p))
	base.finalization = &ExptManagerFinalizationDependencies{ExecutionScope: "scope", Runs: runs, Repository: exptinfra.NewHookFinalizationRepo(p)}
	prepared, err := exptinfra.PrepareHookDeletion(context.Background(), p, runs, codec, f.ids, f.space, "scope")
	require.NoError(t, err)
	f.m, err = NewExptManagerForHookDeletion(base, prepared, "scope")
	require.NoError(t, err)
	return f
}

func TestHookBatchDeletionQuotaMySQLFailurePreservesRunningQuota(t *testing.T) {
	for _, kind := range []string{"new_run", "write_failure"} {
		t.Run(kind, func(t *testing.T) {
			f := newDeletionQuotaMySQLFixture(t, false)
			if kind == "new_run" {
				run := finalizationTestIDs.Add(1)
				require.NoError(t, f.sql.Create(&model.ExptRunLog{ID: run, SpaceID: f.space, ExptID: f.ids[1], ExptRunID: run, CreatedBy: "actor", Mode: gptr.Of(int32(1)), Status: gptr.Of(int64(3))}).Error)
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.ids[1]).Update("latest_run_id", run).Error)
			} else {
				name := fmt.Sprintf("quota_fail_%d", f.ids[1])
				require.NoError(t, f.sql.Exec(fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE ON experiment FOR EACH ROW BEGIN IF NEW.id=%d AND NEW.deleted_at IS NOT NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='quota delete rollback'; END IF; END", name, f.ids[1])).Error)
				t.Cleanup(func() { require.NoError(t, f.sql.Exec("DROP TRIGGER "+name).Error) })
			}
			err := f.m.MDelete(context.Background(), f.ids, f.space, &entity.Session{UserID: "actor"})
			require.Error(t, err)
			if kind == "new_run" {
				require.ErrorIs(t, err, entity.ErrHookStoreConflict)
			} else {
				require.ErrorContains(t, err, "quota delete rollback")
			}
			var count int64
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("space_id=? AND id IN ?", f.space, f.ids).Count(&count).Error)
			require.Equal(t, int64(2), count)
			require.Empty(t, f.guard.calls, "SQL rollback must leave Running reservations untouched")
			require.Len(t, f.guard.held, 2)
		})
	}
}

func TestHookBatchDeletionQuotaMySQLMixedLegacyLatestCleanupAfterCommit(t *testing.T) {
	f := newDeletionQuotaMySQLFixture(t, true)
	require.NoError(t, f.m.MDelete(context.Background(), f.ids, f.space, &entity.Session{UserID: "actor"}))
	require.ElementsMatch(t, []deletionQuotaKey{{"billing-scope", f.runs[0], 101}, {"billing-scope", f.runs[1], 102}}, f.guard.calls)
	require.Equal(t, []bool{false, false}, f.guard.beforeCommit)
	require.Empty(t, f.guard.held)
	old, err := exptinfra.NewHookRunRepo(f.p).GetRun(context.Background(), f.older)
	require.NoError(t, err)
	require.Equal(t, entity.HookFinalizePending, old.State.Finalize)
	require.Equal(t, entity.HookGateClosed, old.State.Gate)
	require.NoError(t, f.m.MDelete(context.Background(), f.ids, f.space, nil))
	require.Len(t, f.guard.calls, 2)
}
