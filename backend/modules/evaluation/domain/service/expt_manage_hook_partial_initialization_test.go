// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	cm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func partialInitializationFixture(t *testing.T, count int, ordinals ...int) (*finalizationManagerFixture, []entity.HookExecutionManifest) {
	t.Helper()
	f := neverAdmittedFinalizationFixture(t, true, true, "tx")
	ctx := context.Background()
	_, err := f.repo.CompleteAttempt(ctx, claimTerminationBefore(t, f))
	require.NoError(t, err)
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Delete(&model.ExptLifecycleRunItem{}).Error)
	digest := entity.NewHookPlanDigest()
	var manifests []entity.HookExecutionManifest
	for i := 0; i < count; i++ {
		item := entity.HookPlanItem{ID: finalizationTestIDs.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: finalizationTestIDs.Add(1)}
		require.NoError(t, f.sql.Create(&model.ExptLifecycleRunItem{ID: item.ID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, Ordinal: int64(i), SourceSpaceID: f.space, EvalSetID: 71, ItemID: item.ItemID}).Error)
		digest, err = entity.AppendHookPlanDigest(digest, []entity.HookPlanItem{item})
		require.NoError(t, err)
		manifests = append(manifests, entity.HookExecutionManifest{Version: 1, Key: f.key, Ordinal: int64(i), Frozen: item, ItemResultID: finalizationTestIDs.Add(1), ItemRunLogID: finalizationTestIDs.Add(1), Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, ResultID: finalizationTestIDs.Add(1)}}, TurnLogsInitialized: gptr.Of(false)})
	}
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumns(map[string]any{"plan_count": count, "plan_hash": digest.Hash}).Error)
	init := exptinfra.NewHookExecutionInitializationRepo(f.p)
	for _, ordinal := range ordinals {
		run := finalizationRead(t, f)
		_, err := init.WriteExecutionInitializationPage(ctx, entity.HookExecutionInitializationWriteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ExecutionScope: "local", PlanHash: digest.Hash, StartOrdinal: int64(ordinal), Items: manifests[ordinal : ordinal+1]})
		require.NoError(t, err)
	}
	return f, manifests
}

func TestHookPartialInitializationSparseAndPending(t *testing.T) {
	for _, tc := range []struct {
		name     string
		count    int
		ordinals []int
	}{
		{"sparse", 4, []int{3, 1}}, {"all_before_completion", 3, []int{0, 1, 2}}, {"multiple_pages", 203, []int{202, 100, 0}},
	} {
		for _, after := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/after=%t", tc.name, after), func(t *testing.T) {
				f, _ := partialInitializationFixture(t, tc.count, tc.ordinals...)
				if !after {
					require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("after_enabled", false).Error)
					require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=? AND phase='after'", f.space, f.key.RunID).Delete(&model.ExptLifecycleHookRun{}).Error)
				}
				before := finalizationRead(t, f)
				intent := entity.HookTerminalIntent{Status: entity.ExptStatus_SystemTerminated, Reason: "frozen"}
				pending, err := f.repo.BeginFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: before.Version}, Intent: intent})
				require.NoError(t, err)
				_, err = f.deps.Repository.ReadFinalizationStats(context.Background(), f.key, "local")
				require.ErrorIs(t, err, entity.ErrHookFinalizationUnsettled)
				require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
				done := finalizationRead(t, f)
				require.Equal(t, intent, done.State.Intent)
				require.Equal(t, pending.Run.Snapshot, done.Snapshot)
				require.Equal(t, pending.Run.State.After.ID, done.State.After.ID)
				require.Equal(t, after, done.State.After.Activated)
				stats, err := f.deps.Repository.ReadFinalizationStats(context.Background(), f.key, "local")
				require.NoError(t, err)
				require.Len(t, stats.ItemIDs, tc.count)
				require.Equal(t, int32(len(tc.ordinals)), stats.Items.Terminated)
				require.Equal(t, int32(len(tc.ordinals)), stats.Turns.Terminated)
			})
		}
	}
}

func TestHookPartialInitializationDeletionRecovery(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprint(batch), func(t *testing.T) {
			f, manifests := partialInitializationFixture(t, 3, 2)
			m, _ := deletionManager(t, f)
			ctx := context.Background()
			if batch {
				require.NoError(t, m.MDelete(ctx, []int64{f.expt, f.expt}, f.space, nil))
			} else {
				require.NoError(t, m.Delete(ctx, f.expt, f.space, nil))
			}
			pending := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
			m.exptRepo = nil
			require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			require.Equal(t, entity.HookFinalizeCommitted, finalizationRead(t, f).State.Finalize)
			require.Zero(t, f.notifications)
			var original model.ExptItemResult
			require.NoError(t, f.sql.First(&original, manifests[2].ItemResultID).Error)
			require.Equal(t, int32(entity.ItemRunState_Queueing), original.Status, "no deleted Latest projection")
			var visible int64
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).Count(&visible).Error)
			require.Zero(t, visible)
		})
	}
}

func TestHookPartialInitializationOldLatest(t *testing.T) {
	f, manifests := partialInitializationFixture(t, 2, 1)
	ctx := context.Background()
	original := finalizationRead(t, f)
	nextKey := entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: finalizationTestIDs.Add(1)}
	next, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{Key: nextKey, ExpectedLatestRunID: f.key.RunID, RunLog: &entity.ExptRunLog{ID: nextKey.RunID, SpaceID: f.space, ExptID: f.expt, ExptRunID: nextKey.RunID, Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Pending), CreatedBy: "successor"}, Snapshot: original.Snapshot, Before: &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprintf("next-before-%d", nextKey.RunID), IdempotencyKey: fmt.Sprint(nextKey.RunID)}})
	require.NoError(t, err)
	lockKey := fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)
	owner := fmt.Sprintf("hook_run:%d:0123456789abcdef0123456789abcdef", nextKey.RunID)
	require.NoError(t, f.redis.Set(ctx, lockKey, owner, time.Hour).Err())
	var before model.Experiment
	require.NoError(t, f.sql.First(&before, f.expt).Error)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}))
	var after model.Experiment
	require.NoError(t, f.sql.First(&after, f.expt).Error)
	require.Equal(t, before, after)
	var projection model.ExptItemResult
	require.NoError(t, f.sql.First(&projection, manifests[1].ItemResultID).Error)
	require.Equal(t, int32(entity.ItemRunState_Queueing), projection.Status)
	q, err := dao.NewQuotaDAO(f.redis).GetQuotaSpaceExpt(ctx, f.space)
	require.NoError(t, err)
	require.Contains(t, q.ExptID2RunTime, f.expt)
	require.Equal(t, owner, f.redis.Get(ctx, lockKey).Val())
	nextAfter, err := f.repo.GetRun(ctx, nextKey)
	require.NoError(t, err)
	require.Equal(t, next.Run, nextAfter)
	require.Zero(t, f.notifications)
}

func TestHookPartialInitializationQuotaIncludesUnmaterializedPlan(t *testing.T) {
	f, manifests := partialInitializationFixture(t, 3, 2)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"scheduler_mode": "enforce", "scheduler_scope": "partial-scheduler"}).Error)
	guard := cm.NewMockICentralReservationGuard(gomock.NewController(t))
	for _, m := range manifests {
		guard.EXPECT().Release(gomock.Any(), "partial-scheduler", f.key.RunID, m.Frozen.ItemID, gomock.Any()).Return(nil)
	}
	f.base.(*ExptMangerImpl).centralGuard = guard
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}))
	stats, err := f.deps.Repository.ReadFinalizationStats(context.Background(), f.key, "local")
	require.NoError(t, err)
	require.Equal(t, int32(1), stats.Items.Terminated)
	require.Len(t, stats.ItemIDs, 3)
}

func TestHookPartialInitializationCancellationRacesWriters(t *testing.T) {
	for _, complete := range []bool{false, true} {
		for n := 0; n < 5; n++ {
			t.Run(fmt.Sprintf("complete=%t/%d", complete, n), func(t *testing.T) {
				ordinals := []int{0}
				if complete {
					ordinals = append(ordinals, 1)
				}
				f, manifests := partialInitializationFixture(t, 2, ordinals...)
				f.deps.NewItemLocker = func() lock.ILocker { return lock.NewRedisLocker(f.redis) }
				finalizationRecreate(t, f)
				before := finalizationRead(t, f)
				init := exptinfra.NewHookExecutionInitializationRepo(f.p)
				start := make(chan struct{})
				writeDone := make(chan error, 1)
				cancelDone := make(chan error, 1)
				go func() {
					<-start
					var err error
					if complete {
						_, err = init.CompleteExecutionInitialization(context.Background(), entity.HookExecutionInitializationCompleteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: before.Version}, ExecutionScope: "local", PlanHash: before.PlanHash, ExpectedItemCount: 2, ExpectedTurnCount: 2})
					} else {
						_, err = init.WriteExecutionInitializationPage(context.Background(), entity.HookExecutionInitializationWriteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: before.Version}, ExecutionScope: "local", PlanHash: before.PlanHash, StartOrdinal: 1, Items: manifests[1:]})
					}
					writeDone <- err
				}()
				go func() {
					<-start
					cancelDone <- f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated})
				}()
				close(start)
				writeErr := <-writeDone
				require.NoError(t, <-cancelDone)
				if writeErr != nil {
					require.True(t, errors.Is(writeErr, entity.ErrHookAdmissionDenied) || errors.Is(writeErr, entity.ErrHookStoreConflict), writeErr)
				}
				done := finalizationRead(t, f)
				require.Equal(t, entity.HookFinalizeCommitted, done.State.Finalize)
				require.True(t, done.State.After.Activated)
				var life model.ExptLifecycleRun
				require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&life).Error)
				require.Equal(t, complete && writeErr == nil, life.ExecutionInitialized)
				require.False(t, life.ExecutionStarted)
				_, err := init.WriteExecutionInitializationPage(context.Background(), entity.HookExecutionInitializationWriteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: done.Version}, ExecutionScope: "local", PlanHash: done.PlanHash, StartOrdinal: 1, Items: manifests[1:]})
				require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
			})
		}
	}
}

func TestHookPartialInitializationInterruptedPageRecovery(t *testing.T) {
	f, manifests := partialInitializationFixture(t, 102, 0, 100, 101)
	var updates atomic.Int32
	callback := fmt.Sprintf("partial-page-%d", f.expt)
	require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_item_result_run_log" && updates.Add(1) == 3 {
			tx.AddError(errors.New("page interrupted"))
		}
	}))
	t.Cleanup(func() { f.sql.Callback().Update().Remove(callback) })
	ctx := context.Background()
	require.ErrorContains(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}), "page interrupted")
	pending := finalizationRead(t, f)
	require.False(t, pending.State.After.Activated)
	var first, second model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&first, manifests[0].ItemRunLogID).Error)
	require.NoError(t, f.sql.First(&second, manifests[100].ItemRunLogID).Error)
	require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(first.ResultState))
	require.Zero(t, gptr.Indirect(second.ResultState), "the failed page rolls back as a unit")
	require.NoError(t, f.sql.Callback().Update().Remove(callback))
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	require.Equal(t, entity.HookFinalizeCommitted, finalizationRead(t, f).State.Finalize)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
}

func TestHookPartialInitializationCommitRequiresProof(t *testing.T) {
	f, _ := partialInitializationFixture(t, 2, 0)
	before := finalizationRead(t, f)
	in := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: before.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}}
	pending, err := f.repo.BeginFinalize(context.Background(), in)
	require.NoError(t, err)
	in.ExpectedVersion = pending.Run.Version
	_, err = f.repo.CommitFinalize(context.Background(), in)
	require.Error(t, err, "nil stats must not bypass partial scaffolding proof")
	require.Equal(t, pending.Run, finalizationRead(t, f))
}

func TestHookPartialInitializationCommitRejectsChangedQuotaScope(t *testing.T) {
	f, _ := partialInitializationFixture(t, 2, 0)
	ctx := context.Background()
	before := finalizationRead(t, f)
	in := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: before.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}}
	pending, err := f.repo.BeginFinalize(ctx, in)
	require.NoError(t, err)
	_, err = f.deps.Repository.(repo.IHookPartialInitializationFinalizer).PreparePartialInitializationTermination(ctx, f.key, "local")
	require.NoError(t, err)
	stats, err := f.deps.Repository.ReadFinalizationStats(ctx, f.key, "local")
	require.NoError(t, err)
	stats.ItemIDs = stats.ItemIDs[:1]
	in.ExpectedVersion, in.Stats = pending.Run.Version, stats
	_, err = f.repo.CommitFinalize(ctx, in)
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Equal(t, pending.Run, finalizationRead(t, f))
}

func TestHookPartialInitializationCommitRechecksLateCorruption(t *testing.T) {
	f, manifests := partialInitializationFixture(t, 2, 0)
	before := finalizationRead(t, f)
	in := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: before.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}}
	pending, err := f.repo.BeginFinalize(context.Background(), in)
	require.NoError(t, err)
	_, err = f.deps.Repository.(repo.IHookPartialInitializationFinalizer).PreparePartialInitializationTermination(context.Background(), f.key, "local")
	require.NoError(t, err)
	stats, err := f.deps.Repository.ReadFinalizationStats(context.Background(), f.key, "local")
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", manifests[0].Turns[0].ResultID).UpdateColumn("weighted_score", 0).Error)
	in.ExpectedVersion, in.Stats = pending.Run.Version, stats
	_, err = f.repo.CommitFinalize(context.Background(), in)
	require.Error(t, err)
	require.Equal(t, pending.Run, finalizationRead(t, f))
}

// A committed initialization page must not strand a cancelled Run before its completion bit.
func TestHookPartialInitializationCancellation(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f, _ := partialInitializationFixture(t, 2, 0)
			require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{Status: status}))
			done := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizeCommitted, done.State.Finalize)
			require.True(t, done.State.After.Activated)
			var life model.ExptLifecycleRun
			require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&life).Error)
			require.False(t, life.ExecutionInitialized)
			require.False(t, life.ExecutionStarted)
			stats, err := f.deps.Repository.ReadFinalizationStats(context.Background(), f.key, "local")
			require.NoError(t, err)
			require.Equal(t, int32(1), stats.Items.Terminated)
			require.Equal(t, int32(1), stats.Turns.Terminated)
			require.Len(t, stats.ItemIDs, 2)
			var count int64
			require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Count(&count).Error)
			require.Zero(t, count)
			require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
			require.Equal(t, done, finalizationRead(t, f))
		})
	}
}
