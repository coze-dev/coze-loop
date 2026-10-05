// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	evalmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	store "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	targetmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
)

type lateProofCommitContextKey struct{}
type lateProofWriterContextKey struct{}

type lateProofCommitRepo struct {
	repo.IHookRepo
	before func()
}

func (r lateProofCommitRepo) AcceptTermination(ctx context.Context, in entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	return r.IHookRepo.(repo.IHookTerminationRepo).AcceptTermination(ctx, in)
}

func (r lateProofCommitRepo) CommitFinalize(ctx context.Context, in entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	if r.before != nil {
		r.before()
	}
	return r.IHookRepo.CommitFinalize(context.WithValue(ctx, lateProofCommitContextKey{}, true), in)
}

func lateProofTurn(t *testing.T, f *finalizationManagerFixture) *entity.ExptTurnResultRunLog {
	t.Helper()
	var po model.ExptTurnResultRunLog
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&po).Error)
	row, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(&po)
	require.NoError(t, err)
	return row
}

// The only double is remote cleanup; parent writes and finalization use the real service/DB path.
func lateProofServiceWriter(t *testing.T, f *finalizationManagerFixture, m entity.HookExecutionManifest, freshTarget ...bool) (func() error, int64) {
	t.Helper()
	base := lateProofTurn(t, f)
	targetID := base.TargetResultID
	if len(freshTarget) > 0 && freshTarget[0] {
		targetID = finalizationTestIDs.Add(1)
		persistActiveTerminationTarget(t, f, &entity.EvalTargetRecord{ID: targetID, SpaceID: f.space + 500, TargetID: 91, TargetVersionID: 92, ExperimentRunID: f.key.RunID, ItemID: base.ItemID, ItemVersionID: base.ItemVersionID, TurnID: base.TurnID, Status: gptr.Of(entity.EvalTargetRunStatusSuccess), EvalTargetOutputData: &entity.EvalTargetOutputData{Ext: map[string]string{"late": "durable"}}})
	}
	id := finalizationTestIDs.Add(1)
	require.NoError(t, f.sql.Create(&evalmodel.EvaluatorRecord{ID: id, SpaceID: f.space, ExperimentID: gptr.Of(f.expt), ExperimentRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, TurnID: base.TurnID, EvaluatorVersionID: 93, Alias_: "late", SourceType: 1, Status: 1, Score: gptr.Of(0.8), OutputData: gptr.Of([]byte(`{"evaluator_result":{"score":0.8},"ext":{"proof":"durable"}}`))}).Error)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("id=?", id).Delete(&evalmodel.EvaluatorRecord{}).Error)
	})
	fixture := newExecutionHookFixture(t, true, false, false)
	etec := fixture.etec
	etec.Expt.ID, etec.Expt.SpaceID, etec.Expt.EvalConf = f.expt, f.space, nil
	etec.Event.SpaceID, etec.Event.ExptID, etec.Event.ExptRunID, etec.Event.EvalSetItemID = f.space, f.expt, f.key.RunID, base.ItemID
	etec.Turn = &entity.Turn{ID: base.TurnID}
	etec.EvalSetItem.ItemID = base.ItemID
	etec.ExistItemEvalResult.TurnResultRunLogs = map[int64]*entity.ExptTurnResultRunLog{base.TurnID: base}
	etec.Ext = map[string]string{"late": "durable"}
	ctx := context.WithValue(context.Background(), itemHookProgressContextKey{}, itemHookProgressBinding{key: f.key, itemID: base.ItemID, itemVersion: base.ItemVersionID, repo: store.NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil })})
	ctx = context.WithValue(ctx, lateProofWriterContextKey{}, true)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	return func() error {
		return (&ExptItemEvalCtxExecutor{}).storeTurnRunResult(canceled, etec, &entity.ExptTurnRunResult{TargetResult: &entity.EvalTargetRecord{ID: targetID}, EvaluatorResults: []*entity.EvaluatorRecord{{ID: id, EvaluatorVersionID: 93, Alias: "late", Status: entity.EvaluatorRunStatusSuccess}}})
	}, id
}

func lateProofSupersede(t *testing.T, f *finalizationManagerFixture) func() {
	t.Helper()
	next := finalizationTestIDs.Add(1)
	key := entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: next}
	created, err := f.repo.CreateRunWithHooks(context.Background(), entity.HookCreateRunInput{Key: key, ExpectedLatestRunID: f.key.RunID, RunLog: &entity.ExptRunLog{ID: next, ExptRunID: next, SpaceID: f.space, ExptID: f.expt, Mode: 1, Status: 3, CreatedBy: "user"}, Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1}, Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", KeyID: "key", ExecutionScope: "local"}, After: &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(next), IdempotencyKey: fmt.Sprint(next)}})
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumns(map[string]any{"pending_cnt": 91, "success_cnt": 92, "credit_cost": 93}).Error)
	var beforeStats model.ExptStats
	var beforeExpt model.Experiment
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&beforeStats).Error)
	require.NoError(t, f.sql.First(&beforeExpt, f.expt).Error)
	return func() {
		current, err := f.repo.GetRun(context.Background(), key)
		require.NoError(t, err)
		assert.Equal(t, created.Run, current)
		var afterStats model.ExptStats
		var afterExpt model.Experiment
		require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&afterStats).Error)
		require.NoError(t, f.sql.First(&afterExpt, f.expt).Error)
		assert.Equal(t, beforeStats, afterStats, "new Latest statistics are not the old Run's archive")
		assert.Equal(t, beforeExpt, afterExpt)
	}
}

func lateProofDurableEvaluator(t *testing.T, f *finalizationManagerFixture, id int64) {
	t.Helper()
	var row evalmodel.EvaluatorRecord
	require.NoError(t, f.sql.First(&row, id).Error)
	assert.Equal(t, int32(entity.EvaluatorRunStatusSuccess), row.Status)
	assert.Equal(t, f.key.RunID, row.ExperimentRunID)
	assert.Contains(t, string(gptr.Indirect(row.OutputData)), "durable")
}

// A real late parent write during cleanup or after archive must force another cleanup/archive.
func TestHookLateProofServicePendingRearchivesMySQL(t *testing.T) {
	for _, superseded := range []bool{false, true} {
		for _, stage := range []string{"during_cleanup", "before_commit_stats"} {
			t.Run(fmt.Sprintf("%s/superseded=%t", stage, superseded), func(t *testing.T) {
				f, m, _, cleaner := activeTerminationReferenceFixture(t)
				write, id := lateProofServiceWriter(t, f, m)
				checkLatest := func() {}
				if superseded {
					checkLatest = lateProofSupersede(t, f)
				}
				var writeErr error
				if stage == "during_cleanup" {
					cleaner.after = func(context.Context) { writeErr = write() }
				} else {
					f.deps.Runs = lateProofCommitRepo{IHookRepo: f.repo, before: func() { writeErr = write() }}
					finalizationRecreate(t, f)
				}
				err := f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil)
				require.NoError(t, writeErr)
				assert.Error(t, err, "late evidence must invalidate finalization proof")
				assert.False(t, finalizationRead(t, f).State.After.Activated)
				var item model.ExptItemResultRunLog
				require.NoError(t, f.sql.First(&item, m.ItemRunLogID).Error)
				assert.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(item.ResultState))
				cleaner.after, f.deps.Runs = nil, f.repo
				finalizationRecreate(t, f)
				require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
				assert.True(t, finalizationRead(t, f).State.After.Activated)
				assert.Equal(t, 2, cleaner.calls)
				assert.True(t, itemHookProgressHasRecord(lateProofTurn(t, f).EvaluatorResultIds, id))
				if !superseded {
					var count int64
					require.NoError(t, f.sql.Model(&model.ExptTurnEvaluatorResultRef{}).Where("space_id=? AND expt_id=? AND evaluator_result_id=?", f.space, f.expt, id).Count(&count).Error)
					assert.Equal(t, int64(1), count)
				}
				lateProofDurableEvaluator(t, f, id)
				checkLatest()
			})
		}
	}
}

func TestHookLateProofServiceCommitLocksOutLateWriterMySQL(t *testing.T) {
	for _, superseded := range []bool{false, true} {
		t.Run(fmt.Sprintf("superseded=%t", superseded), func(t *testing.T) {
			f, m, _, _ := activeTerminationReferenceFixture(t)
			write, id := lateProofServiceWriter(t, f, m)
			checkLatest := func() {}
			if superseded {
				checkLatest = lateProofSupersede(t, f)
			}
			f.deps.Runs = lateProofCommitRepo{IHookRepo: f.repo}
			finalizationRecreate(t, f)
			locked, release, writerStarted := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			var paused atomic.Bool
			var started atomic.Bool
			const callback = "late-proof-commit-barrier"
			require.NoError(t, f.sql.Callback().Query().Before("gorm:query").Register(callback+"-writer", func(tx *gorm.DB) {
				if tx.Statement.Context.Value(lateProofWriterContextKey{}) == true && strings.HasPrefix(tx.Statement.Table, "expt_lifecycle_run") && started.CompareAndSwap(false, true) {
					close(writerStarted)
				}
			}))
			t.Cleanup(func() { require.NoError(t, f.sql.Callback().Query().Remove(callback+"-writer")) })
			require.NoError(t, f.sql.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
				if tx.Statement.Context.Value(lateProofCommitContextKey{}) == true && tx.Statement.Table == "expt_lifecycle_run_item" && paused.CompareAndSwap(false, true) {
					close(locked)
					select {
					case <-release:
					case <-tx.Statement.Context.Done():
					}
				}
			}))
			t.Cleanup(func() { require.NoError(t, f.sql.Callback().Query().Remove(callback)) })
			commitDone := make(chan error, 1)
			go func() {
				commitDone <- f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil)
			}()
			select {
			case <-locked:
			case err := <-commitDone:
				t.Fatalf("Commit did not reach DB barrier: %v", err)
			case <-time.After(10 * time.Second):
				t.Fatal("Commit barrier timeout")
			}
			writerDone := make(chan error, 1)
			go func() { writerDone <- write() }()
			select {
			case <-writerStarted:
			case <-time.After(10 * time.Second):
				t.Fatal("writer did not reach its database query")
			}
			var writerErr error
			finished := false
			select {
			case writerErr = <-writerDone:
				finished = true
				t.Error("late parent writer completed while Commit held its lifecycle/proof lock")
			case <-time.After(200 * time.Millisecond):
			}
			unblock()
			select {
			case err := <-commitDone:
				require.NoError(t, err)
			case <-time.After(10 * time.Second):
				t.Fatal("Commit did not finish")
			}
			if !finished {
				select {
				case writerErr = <-writerDone:
				case <-time.After(10 * time.Second):
					t.Fatal("writer did not finish")
				}
			}
			assert.True(t, itemHookControlOnly(writerErr), "real service keeps rejected write in the Hook control path")
			assert.True(t, finalizationRead(t, f).State.After.Activated)
			assert.False(t, itemHookProgressHasRecord(lateProofTurn(t, f).EvaluatorResultIds, id))
			lateProofDurableEvaluator(t, f, id)
			checkLatest()
		})
	}
}

func lateProofParentSnapshot(t *testing.T, f *finalizationManagerFixture) []byte {
	t.Helper()
	out := map[string]any{}
	for _, table := range []string{"expt_lifecycle_run", "expt_lifecycle_hook_run", "expt_run_log", "expt_item_result_run_log", "expt_turn_result_run_log", "expt_item_result", "expt_turn_result", "expt_stats", "expt_turn_evaluator_result_ref"} {
		var rows []map[string]any
		q := f.sql.Table(table).Where("space_id=? AND expt_id=?", f.space, f.expt)
		if table == "expt_lifecycle_run" {
			q = q.Order("expt_run_id")
		} else {
			q = q.Order("id")
		}
		require.NoError(t, q.Find(&rows).Error)
		out[table] = rows
	}
	var expt model.Experiment
	require.NoError(t, f.sql.First(&expt, f.expt).Error)
	out["experiment"] = expt
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	return raw
}

func TestHookLateProofServiceAfterCommitImmutableMySQL(t *testing.T) {
	for _, superseded := range []bool{false, true} {
		t.Run(fmt.Sprintf("superseded=%t", superseded), func(t *testing.T) {
			f, m, ids, _ := activeTerminationReferenceFixture(t)
			write, id := lateProofServiceWriter(t, f, m, true)
			checkLatest := func() {}
			if superseded {
				checkLatest = lateProofSupersede(t, f)
			}
			require.NoError(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil))
			before := lateProofParentSnapshot(t, f)
			assert.True(t, itemHookControlOnly(write()))
			assert.Equal(t, before, lateProofParentSnapshot(t, f), "all committed parent snapshots must stay immutable")
			lateProofDurableEvaluator(t, f, id)
			var target targetmodel.TargetRecord
			require.NoError(t, f.sql.First(&target, ids[0]).Error)
			assert.Equal(t, f.key.RunID, target.ExperimentRunID)
			var freshTargets []targetmodel.TargetRecord
			require.NoError(t, f.sql.Where("space_id=? AND experiment_run_id=? AND id<>?", f.space+500, f.key.RunID, ids[0]).Find(&freshTargets).Error)
			require.Len(t, freshTargets, 1)
			assert.Equal(t, int32(entity.EvalTargetRunStatusSuccess), freshTargets[0].Status)
			assert.Contains(t, string(gptr.Indirect(freshTargets[0].OutputData)), "durable")
			checkLatest()
		})
	}
}

func TestHookLateProofServiceCanceledCompletionNeverLegacyMySQL(t *testing.T) {
	for _, action := range []string{"turn", "complete", "read"} {
		t.Run(action, func(t *testing.T) {
			f := newHookLateWriteFixture(t)
			require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.base.SpaceID, f.base.ExptRunID).UpdateColumn("finalize_state", 2).Error)
			require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", f.item.ID).UpdateColumns(map[string]any{"status": int32(entity.ItemRunState_Terminal), "result_state": int32(entity.ExptItemResultStateResulted)}).Error)
			require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", f.base.ID).UpdateColumn("status", int32(entity.TurnRunState_Terminal)).Error)
			before := f.readTurn(t)
			ctx, cancel := context.WithCancel(f.ctx)
			cancel()
			var err error
			switch action {
			case "turn":
				err = f.exec.storeTurnRunResult(ctx, f.etec, f.result(false))
			case "complete":
				err = f.exec.CompleteItemRun(ctx, f.etec.ExptItemEvalCtx, nil)
			case "read":
				f.etec.Event.HookControlContinuation = true
				_, err = readItemHookContinuation(context.WithoutCancel(ctx), f.etec)
			}
			assert.True(t, itemHookControlOnly(err))
			assert.Equal(t, before, f.readTurn(t))
			var item model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&item, f.item.ID).Error)
			assert.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
			assert.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState), "legacy completion would set Logged")
		})
	}
}
