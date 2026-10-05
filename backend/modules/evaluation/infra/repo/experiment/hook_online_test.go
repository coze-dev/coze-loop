// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type onlineStoreTestPort interface {
	PrepareOnlinePlan(context.Context, entity.HookRunKey, string) error
	AppendOnlinePage(context.Context, entity.HookRunKey, string, []entity.HookExecutionManifest, map[string]string) (bool, error)
	DrainOnlineRun(context.Context, entity.HookRunKey, string) error
}

func onlineStoreFixture(t *testing.T) (*executionFixture, *hookRunRepo, onlineStoreTestPort) {
	t.Helper()
	f := newExecutionFixture(t, 0)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=? AND space_id=?", f.expt, f.space).UpdateColumn("expt_type", int32(entity.ExptType_Online)).Error)
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("mode", int32(entity.EvaluationModeAppend)).Error)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("space_id=? AND expt_run_id=? AND phase='before'", f.space, f.key.RunID).UpdateColumns(map[string]any{"status": "pending", "attempt": 0}).Error)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("gate", 0).Error)
	r := f.init.(*hookRunRepo)
	run, err := f.repo.GetRun(context.Background(), f.key)
	require.NoError(t, err)
	r.executionBinding = &boundHookExecution{source: entity.HookExecutionInitializationSource{Key: f.key, ExecutionScope: "local", SnapshotHash: run.Snapshot.Hash, CreatedBy: run.CreatedBy, Mode: entity.EvaluationModeAppend, BeforeEnabled: true, AfterEnabled: run.State.After.Status != entity.HookOperationDisabled, Execution: &entity.HookExecutionSnapshot{SingleSet: true}}}
	p, ok := any(r).(onlineStoreTestPort)
	require.True(t, ok, "Online requires an atomic accepting/draining repository, not the old direct record writer")
	return f, r, p
}

func onlineStoreManifest(f *executionFixture) entity.HookExecutionManifest {
	return entity.HookExecutionManifest{Version: 1, Key: f.key, Frozen: entity.HookPlanItem{ID: hookTxSequence.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: hookTxSequence.Add(1)}, ItemResultID: hookTxSequence.Add(1), ItemRunLogID: hookTxSequence.Add(1), Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, ResultID: hookTxSequence.Add(1)}}, TurnLogsInitialized: gptr.Of(false)}
}

func TestHookOnlineWaitingAcceptDrainMySQL(t *testing.T) {
	f, r, p := onlineStoreFixture(t)
	ctx := context.Background()
	first := onlineStoreManifest(f)
	changed, err := p.AppendOnlinePage(ctx, f.key, "local", []entity.HookExecutionManifest{first}, map[string]string{"caller": "original-batch"})
	require.NoError(t, err)
	require.True(t, changed)
	run, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, int64(1), run.PlanCount)
	require.Equal(t, entity.HookGateWaiting, run.State.Gate)
	_, err = r.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ItemID: first.Frozen.ItemID})
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	changed, err = p.AppendOnlinePage(ctx, f.key, "local", []entity.HookExecutionManifest{first}, nil)
	require.NoError(t, err)
	require.False(t, changed)
	second := onlineStoreManifest(f)
	changed, err = p.AppendOnlinePage(ctx, f.key, "local", []entity.HookExecutionManifest{second}, nil)
	require.NoError(t, err)
	require.True(t, changed)
	require.NoError(t, p.DrainOnlineRun(ctx, f.key, "local"))
	require.NoError(t, p.DrainOnlineRun(ctx, f.key, "local"))
	_, err = p.AppendOnlinePage(ctx, f.key, "local", []entity.HookExecutionManifest{onlineStoreManifest(f)}, nil)
	require.Error(t, err, "Draining stops acceptance, not execution")
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("space_id=? AND expt_run_id=? AND phase='before'", f.space, f.key.RunID).UpdateColumns(map[string]any{"status": "succeeded", "attempt": 1}).Error)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("gate", 1).Error)
	page, err := r.ReadExecutionInitializationPage(ctx, f.readInput())
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	done, err := r.CompleteExecutionInitialization(ctx, entity.HookExecutionInitializationCompleteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: page.RunVersion}, ExecutionScope: "local", PlanHash: page.Hash, ExpectedItemCount: 2, ExpectedTurnCount: 2})
	require.NoError(t, err)
	require.True(t, done.Initialized)
	run, err = f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, entity.ExptStatus_Draining, run.State.Status)
	require.Equal(t, entity.HookFinalizeNone, run.State.Finalize)
	admitted, err := r.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ItemID: first.Frozen.ItemID})
	require.NoError(t, err)
	require.True(t, admitted.Admitted)
	var stats model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
	require.Equal(t, int32(2), stats.PendingCnt)
}

func TestHookOnlineAtomicPageRollbackMySQL(t *testing.T) {
	f, _, p := onlineStoreFixture(t)
	errInjected := errors.New("injected turn write failure")
	name := "hook_online_test_rollback"
	require.NoError(t, f.sql.Callback().Create().After("gorm:create").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_turn_result" {
			tx.AddError(errInjected)
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Create().Remove(name)) })
	_, err := p.AppendOnlinePage(context.Background(), f.key, "local", []entity.HookExecutionManifest{onlineStoreManifest(f)}, nil)
	require.Error(t, err)
	for _, table := range []any{&model.ExptLifecycleRunItem{}, &model.ExptItemResult{}, &model.ExptItemResultRunLog{}, &model.ExptTurnResult{}} {
		var count int64
		require.NoError(t, hookRunScope(f.sql.Model(table), f.key).Count(&count).Error)
		require.Zero(t, count)
	}
	run, err := f.repo.GetRun(context.Background(), f.key)
	require.NoError(t, err)
	require.Zero(t, run.PlanCount)
}

func TestHookOnlineAcceptFinishLinearizationMySQL(t *testing.T) {
	f, _, p := onlineStoreFixture(t)
	ctx := context.Background()
	m := onlineStoreManifest(f)
	type outcome struct {
		append  bool
		changed bool
		err     error
	}
	results := make(chan outcome, 2)
	start := make(chan struct{})
	go func() {
		<-start
		changed, err := p.AppendOnlinePage(ctx, f.key, "local", []entity.HookExecutionManifest{m}, nil)
		results <- outcome{append: true, changed: changed, err: err}
	}()
	go func() { <-start; results <- outcome{err: p.DrainOnlineRun(ctx, f.key, "local")} }()
	close(start)
	accepted := false
	for i := 0; i < 2; i++ {
		out := <-results
		if out.append {
			if out.err == nil {
				accepted = out.changed
			} else {
				require.ErrorIs(t, out.err, entity.ErrHookAdmissionDenied)
			}
		} else {
			require.NoError(t, out.err)
		}
	}
	run, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, entity.ExptStatus_Draining, run.State.Status)
	want := int64(0)
	if accepted {
		want = 1
	}
	require.Equal(t, want, run.PlanCount)
	for _, table := range []any{&model.ExptLifecycleRunItem{}, &model.ExptItemResult{}, &model.ExptItemResultRunLog{}, &model.ExptTurnResult{}} {
		var n int64
		require.NoError(t, hookRunScope(f.sql.Model(table), f.key).Count(&n).Error)
		require.Equal(t, want, n)
	}
	_, err = p.AppendOnlinePage(ctx, f.key, "local", []entity.HookExecutionManifest{onlineStoreManifest(f)}, nil)
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
}
