// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type hookDeletionTestRepo interface {
	DeleteExperiments(context.Context, []int64, int64, string) ([]*entity.Experiment, error)
}

func deletionRepo(t *testing.T, f *hookTxFixture) hookDeletionTestRepo {
	t.Helper()
	r, ok := f.repo.(hookDeletionTestRepo)
	require.True(t, ok, "Hook repository must persist deletion intents atomically")
	return r
}

func deletionRun(t *testing.T, f *hookTxFixture, latest int64) entity.HookCreateRunInput {
	t.Helper()
	in := f.input(true, latest)
	in.RunLog.Mode = int32(entity.EvaluationModeSubmit)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"expt_type": int32(entity.ExptType_Offline), "eval_set_source_type": int32(entity.ExptEvalSetSourceType_SingleSet)}).Error)
	_, err := f.repo.CreateRunWithHooks(context.Background(), in)
	require.NoError(t, err)
	return in
}

func TestHookDeletionAtomicRollback(t *testing.T) {
	for _, table := range []string{"expt_lifecycle_run", "experiment"} {
		t.Run(table, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := deletionRun(t, f, 0)
			before, err := f.repo.GetRun(context.Background(), in.Key)
			require.NoError(t, err)
			trigger := fmt.Sprintf("hook_delete_fail_%d", f.expt)
			column := "expt_id"
			if table == "experiment" {
				column = "id"
			}
			require.NoError(t, f.sql.Exec(fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE ON %s FOR EACH ROW BEGIN IF NEW.%s=%d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='deletion write failed'; END IF; END", trigger, table, column, f.expt)).Error)
			t.Cleanup(func() { require.NoError(t, f.sql.Exec("DROP TRIGGER "+trigger).Error) })
			deleted, err := deletionRepo(t, f).DeleteExperiments(context.Background(), []int64{f.expt}, f.space, "local")
			require.ErrorContains(t, err, "deletion write failed")
			require.Empty(t, deleted)
			var visible model.Experiment
			require.NoError(t, f.sql.First(&visible, f.expt).Error)
			after, err := f.repo.GetRun(context.Background(), in.Key)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}

func TestHookDeletionAllOriginalRunsAndDuplicate(t *testing.T) {
	f := newHookTxFixture(t)
	var runs []entity.HookCreateRunInput
	var latest int64
	for i := 0; i < 101; i++ {
		in := deletionRun(t, f, latest)
		runs = append(runs, in)
		latest = in.Key.RunID
	}
	ctx := context.Background()
	deleted, err := deletionRepo(t, f).DeleteExperiments(ctx, []int64{f.expt, f.expt}, f.space, "local")
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	for _, in := range runs {
		run, err := f.repo.GetRun(ctx, in.Key)
		require.NoError(t, err)
		require.Equal(t, entity.HookFinalizePending, run.State.Finalize)
		require.Equal(t, entity.HookGateClosed, run.State.Gate)
		require.Equal(t, entity.ExptStatus_Terminated, run.State.Intent.Status)
		require.Equal(t, in.Snapshot, run.Snapshot)
		require.False(t, run.State.After.Activated)
		require.NotNil(t, run.NextReconcileAt)
	}
	repeated, err := deletionRepo(t, f).DeleteExperiments(ctx, []int64{f.expt}, f.space, "local")
	require.NoError(t, err)
	require.Empty(t, repeated)
	var found int
	scan := entity.HookScanInput{ExecutionScope: "local", Now: time.Now().UTC().Add(time.Minute).Truncate(time.Millisecond), Limit: 100}
	for {
		page, err := NewHookScanRepo(f.p).ScanPendingFinalizations(ctx, scan)
		require.NoError(t, err)
		for _, c := range page.Candidates {
			if c.Key.ExperimentID == f.expt {
				found++
			}
		}
		if !page.HasMore {
			break
		}
		scan.Cursor = page.NextCursor
	}
	require.Equal(t, 101, found)
}

func TestHookDeletionPreservesPendingAndCommitted(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprint(committed), func(t *testing.T) {
			f := settledFinalizationFixture(t)
			ctx := context.Background()
			initial, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			req := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: initial.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Failed, Reason: "frozen_failure"}, DisplayMessage: gptr.Of("original detail")}
			old, err := f.repo.BeginFinalize(ctx, req)
			require.NoError(t, err)
			if committed {
				req.ExpectedVersion = old.Run.Version
				old, err = f.repo.CommitFinalize(ctx, req)
				require.NoError(t, err)
			}
			_, err = deletionRepo(t, f.hookTxFixture).DeleteExperiments(ctx, []int64{f.expt}, f.space, "local")
			require.NoError(t, err)
			current, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, old.Run, current)
		})
	}
}

func TestHookDeletionUnsupportedAndMixedBatchRollback(t *testing.T) {
	for _, kind := range []string{"after-only", "retry", "foreign-scope", "missing-life"} {
		t.Run(kind, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := deletionRun(t, f, 0)
			legacy := f.space // Sorts before the managed parent, so its softdelete must roll back too.
			require.NoError(t, f.sql.Create(&model.Experiment{ID: legacy, SpaceID: f.space, Name: fmt.Sprint(legacy)}).Error)
			t.Cleanup(func() { f.sql.Unscoped().Delete(&model.Experiment{}, legacy) })
			switch kind {
			case "after-only":
				require.NoError(t, hookRunScope(f.sql.Model(&model.ExptLifecycleRun{}), in.Key).UpdateColumn("before_enabled", false).Error)
				require.NoError(t, hookRunScope(f.sql, in.Key).Where("phase='before'").Delete(&model.ExptLifecycleHookRun{}).Error)
			case "retry":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", in.Key.RunID).UpdateColumn("mode", int32(entity.EvaluationModeFailRetry)).Error)
			case "foreign-scope":
				require.NoError(t, hookRunScope(f.sql.Model(&model.ExptLifecycleRun{}), in.Key).UpdateColumn("execution_scope", "another").Error)
			case "missing-life":
				require.NoError(t, hookRunScope(f.sql, in.Key).Delete(&model.ExptLifecycleRun{}).Error)
			}
			_, err := deletionRepo(t, f).DeleteExperiments(context.Background(), []int64{legacy, f.expt}, f.space, "local")
			require.Error(t, err)
			var n int64
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id IN ?", []int64{legacy, f.expt}).Count(&n).Error)
			require.Equal(t, int64(2), n)
		})
	}
}

func TestHookDeletionSerializesNewRun(t *testing.T) {
	f := newHookTxFixture(t)
	old := deletionRun(t, f, 0)
	next := f.input(true, old.Key.RunID)
	next.RunLog.Mode = int32(entity.EvaluationModeSubmit)
	r := deletionRepo(t, f)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var createErr, deleteErr error
	wg.Add(2)
	go func() { defer wg.Done(); <-start; _, createErr = f.repo.CreateRunWithHooks(context.Background(), next) }()
	go func() {
		defer wg.Done()
		<-start
		_, deleteErr = r.DeleteExperiments(context.Background(), []int64{f.expt}, f.space, "local")
	}()
	close(start)
	wg.Wait()
	require.NoError(t, deleteErr)
	if createErr == nil {
		run, err := f.repo.GetRun(context.Background(), next.Key)
		require.NoError(t, err)
		require.Equal(t, entity.HookFinalizePending, run.State.Finalize)
	} else {
		require.ErrorIs(t, createErr, entity.ErrHookStoreConflict)
	}
	_, err := f.repo.CreateRunWithHooks(context.Background(), f.input(true, f.latest(t)))
	require.Error(t, err)
}

func TestHookDeletionRecoverySourceRequiresExactDurableRun(t *testing.T) {
	f := newHookTxFixture(t)
	in := deletionRun(t, f, 0)
	ctx := context.Background()
	r := NewHookFinalizationRepo(f.p)
	require.NoError(t, f.sql.Delete(&model.Experiment{}, f.expt).Error)
	_, err := r.ReadFinalizationSource(ctx, in.Key)
	require.Error(t, err, "deleted but not pending is not a recovery source")
	_, err = f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}})
	require.NoError(t, err)
	source, err := r.ReadFinalizationSource(ctx, in.Key)
	require.NoError(t, err)
	require.True(t, source.Managed)
	_, err = r.ReadFinalizationSource(ctx, entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt})
	require.Error(t, err, "optional latest must remain invisible")
	_, err = r.ReadFinalizationSource(ctx, entity.HookRunKey{WorkspaceID: f.space + 1, ExperimentID: f.expt, RunID: in.Key.RunID})
	require.Error(t, err)
}

func TestHookDeletionBatchSeesRunCommittedBeforeSecondParentLock(t *testing.T) {
	f := newHookTxFixture(t)
	deletionRun(t, f, 0)
	second := &hookTxFixture{p: f.p, sql: f.sql, repo: f.repo, space: f.space, expt: hookTxSequence.Add(1)}
	require.NoError(t, f.sql.Create(&model.Experiment{ID: second.expt, SpaceID: f.space, Name: fmt.Sprint(second.expt), Status: 3}).Error)
	t.Cleanup(func() {
		for _, table := range []any{&model.ExptLifecycleHookRun{}, &model.ExptLifecycleRun{}, &model.ExptRunLog{}} {
			require.NoError(t, f.sql.Unscoped().Where("space_id=? AND expt_id=?", f.space, second.expt).Delete(table).Error)
		}
		require.NoError(t, f.sql.Unscoped().Delete(&model.Experiment{}, second.expt).Error)
	})
	old := deletionRun(t, second, 0)
	next := second.input(true, old.Key.RunID)
	next.RunLog.Mode = int32(entity.EvaluationModeSubmit)
	type deletionContextKey struct{}
	queries := 0
	callback := fmt.Sprintf("hook_delete_race_%d", f.expt)
	require.NoError(t, f.sql.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Context.Value(deletionContextKey{}) != true || tx.Statement.Table != "experiment" {
			return
		}
		queries++
		if queries == 2 {
			_, err := f.repo.CreateRunWithHooks(context.Background(), next)
			require.NoError(t, err)
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Query().Remove(callback)) })
	_, err := deletionRepo(t, f).DeleteExperiments(context.WithValue(context.Background(), deletionContextKey{}, true), []int64{second.expt, f.expt}, f.space, "local")
	require.NoError(t, err)
	require.GreaterOrEqual(t, queries, 2)
	current, err := f.repo.GetRun(context.Background(), next.Key)
	require.NoError(t, err)
	require.Equal(t, entity.HookFinalizePending, current.State.Finalize, "batch must not enumerate a stale snapshot of a later parent")
}

func TestHookDeletionDuringAfterKeepsClaimAndFrozenResult(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	in := hookAttemptInput(t, f, false)
	claim, err := f.repo.ClaimAttempt(ctx, in)
	require.NoError(t, err)
	before, err := f.repo.GetRun(ctx, in.Key)
	require.NoError(t, err)
	_, err = deletionRepo(t, f).DeleteExperiments(ctx, []int64{f.expt}, f.space, "local")
	require.NoError(t, err)
	after, err := f.repo.GetRun(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, before, after)
	completed, err := f.repo.CompleteAttempt(ctx, hookCompletion(t, f, in, claim.Claim))
	require.NoError(t, err)
	require.True(t, completed.Changed)
	require.Equal(t, entity.HookOperationSucceeded, completed.Run.State.After.Status)
	require.Equal(t, before.Snapshot, completed.Run.Snapshot)
	require.Equal(t, before.State.Intent, completed.Run.State.Intent)
	_, err = NewHookFinalizationRepo(f.p).ReadFinalizationSource(ctx, in.Key)
	require.NoError(t, err)
}

func TestHookDeletionNoHookDoesNotCreateLifecycle(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	r := deletionRepo(t, f)
	deleted, err := r.DeleteExperiments(ctx, []int64{f.expt}, f.space+1, "local")
	require.NoError(t, err)
	require.Empty(t, deleted)
	deleted, err = r.DeleteExperiments(ctx, []int64{f.expt}, f.space, "local")
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	var count int64
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&count).Error)
	require.Zero(t, count)
}

func TestHookDeletionLateWriteProofSurvivesSoftDelete(t *testing.T) {
	for _, state := range []int32{1, 2} {
		t.Run(fmt.Sprint(state), func(t *testing.T) {
			f, r, itemID := lateProofProgressFixture(t, state)
			require.NoError(t, f.sql.Delete(&model.Experiment{}, f.row.ExptID).Error)
			base, next := f.read(t), f.read(t)
			next.TargetResultID = 101
			_, err := r.WriteTurnResult(context.Background(), entity.HookTurnProgressInput{Base: base, Progress: next})
			if state == 2 {
				require.ErrorIs(t, err, entity.ErrHookStoreConflict)
				require.Equal(t, base, f.read(t))
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(101), f.read(t).TargetResultID)
				var item model.ExptItemResultRunLog
				require.NoError(t, f.sql.First(&item, itemID).Error)
				require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(item.ResultState))
			}
		})
	}
}

func TestHookDeletionAdmissionRaceClosesOriginalGate(t *testing.T) {
	f := newExecutionFixture(t, 1)
	ctx := context.Background()
	run, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	page, err := f.init.WriteExecutionInitializationPage(ctx, f.writeInput(run.Version))
	require.NoError(t, err)
	ready, err := f.init.CompleteExecutionInitialization(ctx, f.completeInput(page.RunVersion))
	require.NoError(t, err)
	admit := entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: ready.RunVersion}, ItemID: f.manifests[0].Frozen.ItemID}
	start := make(chan struct{})
	var wg sync.WaitGroup
	var deleteErr, admitErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, deleteErr = deletionRepo(t, f.hookTxFixture).DeleteExperiments(ctx, []int64{f.expt}, f.space, "local")
	}()
	go func() { defer wg.Done(); <-start; _, admitErr = f.repo.AdmitItem(ctx, admit) }()
	close(start)
	wg.Wait()
	require.NoError(t, deleteErr)
	if admitErr != nil {
		require.ErrorIs(t, admitErr, entity.ErrHookAdmissionDenied)
	}
	_, err = f.repo.AdmitItem(ctx, admit)
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	gate, err := NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "local", nil }).CanDispatch(ctx, f.key)
	if err == nil {
		require.NotEqual(t, entity.HookGateReady, gate.Gate)
	}
	current, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, entity.HookGateClosed, current.State.Gate)
}
