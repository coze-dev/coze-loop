// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// A late write must not strand normal completion behind its own closed admission gate.
func TestHookNormalFinalizeLateWriteRemainsRecoverableMySQL(t *testing.T) {
	f := newFinalizationManagerFixture(t, "tx")
	ctx := context.Background()
	var ledger model.ExptLifecycleRunItem
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&ledger).Error)
	var manifest entity.HookExecutionManifest
	require.NoError(t, json.Unmarshal(gptr.Indirect(ledger.ExecutionManifest), &manifest))
	stored, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	_, err = f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: stored.Version}, ItemID: manifest.Frozen.ItemID})
	require.NoError(t, err)
	write, recordID := lateProofServiceWriter(t, f, manifest)
	f.deps.Runs = lateProofCommitRepo{IHookRepo: f.repo, before: func() { _ = write() }}
	finalizationRecreate(t, f)
	_ = f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil)
	f.deps.Runs = f.repo
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}), "normal completion must recover without an item consumer behind the closed gate")
	state := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, state.State.Finalize)
	require.True(t, state.State.After.Activated)
	lateProofDurableEvaluator(t, f, recordID)
}

func normalProofArchiveFixture(t *testing.T) (*finalizationManagerFixture, entity.HookExecutionManifest, func(context.Context) error) {
	t.Helper()
	f, m, _, _ := activeTerminationReferenceFixture(t)
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("status", int32(entity.TurnRunState_Success)).Error)
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumns(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
	require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumns(map[string]any{"pending_cnt": 0, "processing_cnt": 1}).Error)
	result := f.base.(*ExptMangerImpl).exptResultService.(*ExptResultServiceImpl)
	result.idgen = activeReferenceIDs{}
	svc, err := result.WithHookArchive(f.deps.Repository.(repo.IHookItemArchiveRepo), "local")
	require.NoError(t, err)
	source, err := f.deps.Repository.ReadFinalizationSource(context.Background(), f.key)
	require.NoError(t, err)
	return f, m, func(ctx context.Context) error {
		_, err := svc.RecordItemRunLogs(ctx, f.expt, f.key.RunID, m.Frozen.ItemID, f.space, source.Experiment)
		return err
	}
}

func normalProofRefOnlyWriter(t *testing.T, f *finalizationManagerFixture, m entity.HookExecutionManifest) (func() error, int64) {
	t.Helper()
	base, next := lateProofTurn(t, f), lateProofTurn(t, f)
	id := finalizationTestIDs.Add(1)
	require.NoError(t, f.sql.Create(&evalmodel.EvaluatorRecord{ID: id, SpaceID: f.space, ExperimentID: gptr.Of(f.expt), ExperimentRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, TurnID: base.TurnID, EvaluatorVersionID: 93, Alias_: "late-normal", SourceType: 1, Status: 1, OutputData: gptr.Of([]byte(`{"ext":{"proof":"durable"}}`))}).Error)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("id=?", id).Delete(&evalmodel.EvaluatorRecord{}).Error)
	})
	next.EvaluatorResultIds.Registered = append(next.EvaluatorResultIds.Registered, &entity.RegisteredEvalResult{VersionID: 93, Alias: "late-normal", RecordID: id})
	r := store.NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil }).(repo.IHookTurnResultWriteRepo)
	return func() error {
		_, err := r.WriteTurnResult(context.WithValue(context.Background(), lateProofWriterContextKey{}, true), entity.HookTurnProgressInput{Base: base, Progress: next})
		return err
	}, id
}

// Neither unchanged terminal counts nor a newer Latest permits changing an archived normal parent.
func TestHookNormalArchiveLateRefsKeepCompletionLiveMySQL(t *testing.T) {
	for _, pending := range []bool{false, true} {
		for _, superseded := range []bool{false, true} {
			t.Run(fmt.Sprintf("pending=%t/superseded=%t", pending, superseded), func(t *testing.T) {
				f, m, archive := normalProofArchiveFixture(t)
				write, id := normalProofRefOnlyWriter(t, f, m)
				require.NoError(t, archive(context.Background()), "exercise real ordinary archive, not a Resulted fixture assignment")
				checkLatest := func() {}
				if superseded {
					checkLatest = lateProofSupersede(t, f)
				}
				if pending {
					state := finalizationRead(t, f)
					_, err := f.repo.BeginFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: state.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Success}})
					require.NoError(t, err)
				}
				before := lateProofParentSnapshot(t, f)
				assert.ErrorIs(t, write(), entity.ErrHookStoreConflict)
				assert.Equal(t, string(before), string(lateProofParentSnapshot(t, f)))
				assert.False(t, itemHookProgressHasRecord(lateProofTurn(t, f).EvaluatorResultIds, id))
				require.NoError(t, f.manager.CompleteExpt(context.Background(), f.expt, &f.key.RunID, f.space, nil))
				require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
				assert.Equal(t, entity.HookFinalizeCommitted, finalizationRead(t, f).State.Finalize)
				assert.True(t, finalizationRead(t, f).State.After.Activated)
				lateProofDurableEvaluator(t, f, id)
				checkLatest()
			})
		}
	}
}

type normalProofArchiveContextKey struct{}

func TestHookNormalArchiveLocksLateRefOnlyWriterMySQL(t *testing.T) {
	f, m, archive := normalProofArchiveFixture(t)
	write, id := normalProofRefOnlyWriter(t, f, m)
	locked, release, started := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var paused, writerStarted atomic.Bool
	const name = "normal-proof-archive-barrier"
	require.NoError(t, f.sql.Callback().Update().After("gorm:update").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Context.Value(normalProofArchiveContextKey{}) == true && tx.Statement.Table == "expt_item_result_run_log" && paused.CompareAndSwap(false, true) {
			close(locked)
			<-release
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Update().Remove(name)) })
	require.NoError(t, f.sql.Callback().Query().Before("gorm:query").Register(name+"-writer", func(tx *gorm.DB) {
		if tx.Statement.Context.Value(lateProofWriterContextKey{}) == true && writerStarted.CompareAndSwap(false, true) {
			close(started)
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Query().Remove(name+"-writer")) })
	archiveDone := make(chan error, 1)
	go func() {
		archiveDone <- archive(context.WithValue(context.Background(), normalProofArchiveContextKey{}, true))
	}()
	select {
	case <-locked:
	case err := <-archiveDone:
		t.Fatalf("archive did not reach barrier: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("archive barrier timeout")
	}
	writerDone := make(chan error, 1)
	go func() { writerDone <- write() }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("writer did not start")
	}
	select {
	case err := <-writerDone:
		t.Fatalf("writer crossed archive lock: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	unblock()
	require.NoError(t, <-archiveDone)
	assert.ErrorIs(t, <-writerDone, entity.ErrHookStoreConflict)
	assert.Equal(t, entity.HookFinalizeNone, finalizationRead(t, f).State.Finalize, "boundary precedes BeginFinalize")
	assert.False(t, itemHookProgressHasRecord(lateProofTurn(t, f).EvaluatorResultIds, id))
	require.NoError(t, f.manager.CompleteExpt(context.Background(), f.expt, &f.key.RunID, f.space, nil))
	lateProofDurableEvaluator(t, f, id)
}
