// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func batchDeletionMySQLRun(t *testing.T, f *hookTxFixture, mode entity.ExptRunMode, bound, single bool, latest int64) entity.HookCreateRunInput {
	t.Helper()
	in := f.input(true, latest)
	in.RunLog.Mode = int32(mode)
	in.RunLog.CreatedBy = "actor"
	in.RunLog.Status = int64(entity.ExptStatus_Pending)
	if entity.HookBoundRetryMode(mode) {
		require.Positive(t, latest)
		in.SourceRunID = gptr.Of(latest)
	}
	name := map[entity.ExptRunMode]string{1: "submit", 2: "fail_retry", 4: "retry_all", 6: "trial_run"}[mode]
	source := entity.ExptEvalSetSourceType_MultiSetConfig
	if single {
		source = entity.ExptEvalSetSourceType_SingleSet
	}
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=? AND space_id=?", f.expt, f.space).UpdateColumns(map[string]any{"expt_type": int32(entity.ExptType_Offline), "eval_set_source_type": int32(source)}).Error)
	data := entity.HookRunSnapshotInput{Key: in.Key, ExecutionScope: "local", CreatedAt: time.Now(),
		Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.test/before")}}, After: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.test/after")}}},
		Context: &spi.HookRunContext{WorkspaceID: gptr.Of(strconv.FormatInt(f.space, 10)), ExperimentID: gptr.Of(strconv.FormatInt(f.expt, 10)), RunID: gptr.Of(strconv.FormatInt(in.Key.RunID, 10)), RunMode: &name,
			Initiator: &spi.HookInitiator{UserID: gptr.Of("actor"), IdentityType: gptr.Of("fornax_user")}, Experiment: &spi.HookExperimentRef{Name: gptr.Of("original"), Type: gptr.Of("offline")},
			EvalSets: []*spi.HookEvalSetRef{{WorkspaceID: gptr.Of(strconv.FormatInt(f.space, 10)), ID: gptr.Of("10"), VersionID: gptr.Of("11")}}}}
	if bound {
		data.Execution = &entity.HookExecutionSnapshot{Version: 1, Key: in.Key, ExecutionScope: "local", Mode: mode, SingleSet: single, EvaluatorFallback: &entity.HookExecutionEvaluatorFallback{}, Sets: []entity.HookExecutionSet{{EvalSetID: 10, EvalSetVersionID: 11, ItemConfig: &entity.ExptItemConfig{}}}}
	}
	snapshot, err := entity.NewHookRunSnapshot(data)
	require.NoError(t, err)
	codec := hookinfra.NewStorageCodec(batchDeletionProtector{})
	in.Snapshot, err = codec.EncodeSnapshot(context.Background(), "key", snapshot)
	require.NoError(t, err)
	_, err = f.repo.CreateRunWithHooks(context.Background(), in)
	require.NoError(t, err)
	return in
}

func batchDeletionSecondParent(t *testing.T, f *hookTxFixture) *hookTxFixture {
	t.Helper()
	other := &hookTxFixture{p: f.p, sql: f.sql, repo: f.repo, space: f.space, expt: hookTxSequence.Add(1)}
	require.NoError(t, f.sql.Create(&model.Experiment{ID: other.expt, SpaceID: f.space, Name: fmt.Sprintf("other-%d", other.expt), Status: int32(entity.ExptStatus_Processing)}).Error)
	t.Cleanup(func() {
		for _, table := range []string{"expt_lifecycle_run_item", "expt_lifecycle_hook_run", "expt_lifecycle_run", "expt_run_log"} {
			require.NoError(t, f.sql.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", f.space, other.expt).Error)
		}
		require.NoError(t, f.sql.Unscoped().Where("id=? AND space_id=?", other.expt, f.space).Delete(&model.Experiment{}).Error)
	})
	return other
}

// Parent must grant deletion_may_run in db-ownership.json before running this prefix.
func TestHookBatchDeletionMySQLMixedMultipleRunsAndOriginalAfter(t *testing.T) {
	f := newHookTxFixture(t)
	g := batchDeletionSecondParent(t, f)
	a := batchDeletionMySQLRun(t, f, entity.EvaluationModeSubmit, true, false, 0)
	b := batchDeletionMySQLRun(t, f, entity.EvaluationModeFailRetry, true, false, a.Key.RunID)
	c := batchDeletionMySQLRun(t, g, entity.EvaluationModeTrialRun, false, true, 0)
	d := batchDeletionMySQLRun(t, g, entity.EvaluationModeRetryAll, true, true, c.Key.RunID)
	legacy := batchDeletionSecondParent(t, f)
	codec := hookinfra.NewStorageCodec(batchDeletionProtector{})
	ids := []int64{g.expt, f.expt, legacy.expt, hookTxSequence.Add(1), f.expt}
	r, err := PrepareHookDeletion(context.Background(), f.p, f.repo, codec, ids, f.space, "local")
	require.NoError(t, err)
	deleted, err := r.DeleteExperiments(context.Background(), ids, f.space, "local")
	require.NoError(t, err)
	require.Len(t, deleted, 3)
	for _, input := range []entity.HookCreateRunInput{a, b, c, d} {
		run, err := f.repo.GetRun(context.Background(), input.Key)
		require.NoError(t, err)
		require.Equal(t, entity.HookFinalizePending, run.State.Finalize)
		require.Equal(t, entity.HookGateClosed, run.State.Gate)
		require.Equal(t, entity.ExptStatus_Terminated, run.State.Intent.Status)
		require.Equal(t, input.Snapshot, run.Snapshot)
		require.False(t, run.State.After.Activated)
		// Per-original-Run primitive recovery activates the same after operation exactly once.
		original := f.repo
		snapshot, err := codec.DecodeSnapshot(context.Background(), input.Key, "local", run.Snapshot)
		require.NoError(t, err)
		if snapshot.Input().Execution != nil {
			binding, err := entity.NewHookExecutionInitializationBinding(run, snapshot)
			require.NoError(t, err)
			bound, err := NewBoundHookExecutionInitializationRepo(f.p, binding)
			require.NoError(t, err)
			original = bound.(repo.IHookRepo)
		}
		got, err := original.CommitFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: input.Key, ExpectedVersion: run.Version}, Intent: run.State.Intent})
		require.NoError(t, err)
		require.True(t, got.Run.State.After.Activated)
		require.Equal(t, input.After.OperationID, got.Run.State.After.ID)
	}
	again, err := r.DeleteExperiments(context.Background(), ids, f.space, "local")
	require.NoError(t, err)
	require.Empty(t, again)
}

func TestHookBatchDeletionMySQLRollbackAllParentsAndIntents(t *testing.T) {
	f := newHookTxFixture(t)
	g := batchDeletionSecondParent(t, f)
	a := batchDeletionMySQLRun(t, f, entity.EvaluationModeSubmit, true, false, 0)
	b := batchDeletionMySQLRun(t, g, entity.EvaluationModeSubmit, true, false, 0)
	beforeA, err := f.repo.GetRun(context.Background(), a.Key)
	require.NoError(t, err)
	beforeB, err := f.repo.GetRun(context.Background(), b.Key)
	require.NoError(t, err)
	trigger := fmt.Sprintf("batch_delete_fail_%d", g.expt)
	require.NoError(t, f.sql.Exec(fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE ON experiment FOR EACH ROW BEGIN IF NEW.id=%d AND NEW.deleted_at IS NOT NULL THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='batch delete rollback'; END IF; END", trigger, g.expt)).Error)
	t.Cleanup(func() { require.NoError(t, f.sql.Exec("DROP TRIGGER "+trigger).Error) })
	r, err := PrepareHookDeletion(context.Background(), f.p, f.repo, hookinfra.NewStorageCodec(batchDeletionProtector{}), []int64{f.expt, g.expt}, f.space, "local")
	require.NoError(t, err)
	deleted, err := r.DeleteExperiments(context.Background(), []int64{f.expt, g.expt}, f.space, "local")
	require.ErrorContains(t, err, "batch delete rollback")
	require.Empty(t, deleted)
	var count int64
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id IN ? AND space_id=?", []int64{f.expt, g.expt}, f.space).Count(&count).Error)
	require.Equal(t, int64(2), count)
	afterA, err := f.repo.GetRun(context.Background(), a.Key)
	require.NoError(t, err)
	afterB, err := f.repo.GetRun(context.Background(), b.Key)
	require.NoError(t, err)
	require.Equal(t, beforeA, afterA)
	require.Equal(t, beforeB, afterB)
}

func TestHookBatchDeletionMySQLNewRunAfterPreloadConflicts(t *testing.T) {
	f := newHookTxFixture(t)
	a := batchDeletionMySQLRun(t, f, entity.EvaluationModeSubmit, true, false, 0)
	r, err := PrepareHookDeletion(context.Background(), f.p, f.repo, hookinfra.NewStorageCodec(batchDeletionProtector{}), []int64{f.expt}, f.space, "local")
	require.NoError(t, err)
	b := batchDeletionMySQLRun(t, f, entity.EvaluationModeRetryAll, true, false, a.Key.RunID)
	deleted, err := r.DeleteExperiments(context.Background(), []int64{f.expt}, f.space, "local")
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Empty(t, deleted)
	for _, in := range []entity.HookCreateRunInput{a, b} {
		run, err := f.repo.GetRun(context.Background(), in.Key)
		require.NoError(t, err)
		require.Equal(t, entity.HookFinalizeNone, run.State.Finalize)
	}
}
