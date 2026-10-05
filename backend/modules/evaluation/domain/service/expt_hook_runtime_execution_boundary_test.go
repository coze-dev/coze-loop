// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	tm "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestHookBoundExecutionInflightCleanupMySQL(t *testing.T) {
	for _, action := range []string{"cancel-successor", "delete"} {
		t.Run(action, func(t *testing.T) {
			f, deps, pub, target := executionChainFixture(t, entity.EvaluationModeTrialRun, "async")
			runtime, err := NewBoundHookRuntimeExecution(deps)
			require.NoError(t, err)
			runtime.Scheduler.ResultSvc = hookPersistenceFilters{runtime.Scheduler.ResultSvc}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeTrialRun, CreatedAt: time.Now().Unix(), Session: &entity.Session{UserID: "original-user"}}
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			require.Len(t, pub.items, 2)
			require.NoError(t, runtime.Consumer.Eval(ctx, pub.items[0]))
			require.Equal(t, 1, target.calls)
			var original tm.TargetRecord
			require.NoError(t, f.sql.Where("experiment_run_id=?", f.key.RunID).First(&original).Error)
			require.Equal(t, int32(entity.EvalTargetRunStatusAsyncInvoking), original.Status)
			var successor *entity.HookStoredRun
			if action == "delete" {
				require.NotPanics(t, func() { require.NoError(t, runtime.Manager.Delete(ctx, f.expt, f.space, event.Session)) })
			} else {
				require.NoError(t, runtime.Manager.SetExptTerminating(ctx, f.expt, f.key.RunID, f.space, event.Session))
				key := f.key
				key.RunID = finalizationTestIDs.Add(1)
				initial, err := f.manager.hooks.Initialization.ReadRunInitialization(ctx, key)
				require.NoError(t, err)
				created, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{Key: key, ExpectedLatestRunID: f.key.RunID, ExpectedConfigRevision: initial.ConfigRevision, RunLog: &entity.ExptRunLog{ID: key.RunID, SpaceID: f.space, ExptID: f.expt, ExptRunID: key.RunID, Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Processing), CreatedBy: "successor"}, Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{2}, KeyID: "key", Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ExecutionScope: "local"}, After: &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(key.RunID), IdempotencyKey: fmt.Sprint(key.RunID)}})
				require.NoError(t, err)
				successor = created.Run
			}
			require.NoError(t, runtime.Manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			run := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize)
			require.True(t, run.State.After.Activated)
			require.NoError(t, f.sql.First(&original, original.ID).Error)
			require.Equal(t, int32(entity.EvalTargetRunStatusFail), original.Status)
			late := *pub.items[0]
			late.AsyncReportTrigger = true
			require.NoError(t, runtime.Consumer.Eval(ctx, &late))
			require.Equal(t, 1, target.calls)
			if successor != nil {
				unchanged, err := f.repo.GetRun(ctx, successor.State.Key)
				require.NoError(t, err)
				require.Equal(t, successor, unchanged)
			}
		})
	}
}

func TestHookBoundExecutionFailureClosesNormallyMySQL(t *testing.T) {
	f, deps, pub, target := executionChainFixture(t, entity.EvaluationModeSubmit)
	target.fail = true
	runtime, err := NewBoundHookRuntimeExecution(deps)
	require.NoError(t, err)
	runtime.Scheduler.ResultSvc = hookPersistenceFilters{runtime.Scheduler.ResultSvc}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit, CreatedAt: time.Now().Unix(), Session: &entity.Session{UserID: "original-user"}}
	require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
	require.Len(t, pub.items, 2)
	for _, item := range pub.items {
		require.NoError(t, runtime.Consumer.Eval(ctx, item))
	}
	require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
	run := finalizationRead(t, f)
	require.Equal(t, entity.ExptStatus_Failed, run.State.Intent.Status)
	require.True(t, run.State.After.Activated)
	var logs []model.ExptItemResultRunLog
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Find(&logs).Error)
	for _, log := range logs {
		require.Equal(t, int32(entity.ItemRunState_Fail), log.Status)
	}
}

func TestHookBoundExecutionConstructionRejectsIncompleteMySQL(t *testing.T) {
	f, base, _, _ := executionChainFixture(t, entity.EvaluationModeSubmit)
	for _, kind := range []string{"legacy-progress", "legacy-finalizer", "missing-current-repo", "missing-async-repo", "missing-mode-factory"} {
		t.Run(kind, func(t *testing.T) {
			d := base
			c := *base.Consumer.(*ExptItemEventEvalServiceImpl)
			s := *base.Scheduler.(*ExptSchedulerImpl)
			d.Consumer = &c
			d.Scheduler = &s
			switch kind {
			case "legacy-progress":
				d.Repositories.Progress = exptinfra.NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil })
			case "legacy-finalizer":
				d.Repositories.Finalization = exptinfra.NewHookFinalizationRepo(f.p)
			case "missing-current-repo":
				c.experimentRepo = nil
			case "missing-async-repo":
				c.evalAsyncRepo = nil
			case "missing-mode-factory":
				s.schedulerModeFactory = nil
			}
			runtime, err := NewBoundHookRuntimeExecution(d)
			require.Error(t, err)
			require.Nil(t, runtime)
		})
	}
	require.Nil(t, base.Consumer.(*ExptItemEventEvalServiceImpl).boundContext)
}

func TestHookBoundExecutionWrongModeCannotInitializeMySQL(t *testing.T) {
	f, deps, _, _ := executionChainFixture(t, entity.EvaluationModeSubmit)
	runtime, err := NewBoundHookRuntimeExecution(deps)
	require.NoError(t, err)
	runtime.Scheduler.ResultSvc = hookPersistenceFilters{runtime.Scheduler.ResultSvc}
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeTrialRun, CreatedAt: time.Now().Unix(), Session: &entity.Session{UserID: "original-user"}}
	require.Error(t, runtime.Scheduler.Schedule(context.Background(), event))
	var count int64
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("space_id=? AND expt_run_id=? AND execution_manifest IS NOT NULL", f.space, f.key.RunID).Count(&count).Error)
	require.Zero(t, count)
}

func TestHookBoundExecutionCorruptMarkerCannotUseLegacyMySQL(t *testing.T) {
	f, deps, _, target := executionChainFixture(t, entity.EvaluationModeSubmit)
	runtime, err := NewBoundHookRuntimeExecution(deps)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("lifecycle_hook_version", 0).Error)
	_, err = deps.Repositories.Gate.CanDispatch(context.Background(), f.key)
	require.Error(t, err)
	event := &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit, EvalSetItemID: 801, Session: &entity.Session{UserID: "original-user"}}
	require.Error(t, runtime.Consumer.Eval(context.Background(), event))
	require.Zero(t, target.calls)
	var count int64
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Count(&count).Error)
	require.Zero(t, count)
}

func TestHookBoundExecutionSharedAdmissionWaitsForInitializationMySQL(t *testing.T) {
	f, _, _, _ := executionChainFixture(t, entity.EvaluationModeSubmit)
	gate := exptinfra.NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "local", nil })
	decision, err := gate.CanDispatch(context.Background(), f.key)
	require.NoError(t, err)
	require.Equal(t, entity.HookGateWaiting, decision.Gate)
	run := finalizationRead(t, f)
	_, err = f.repo.AdmitItem(context.Background(), entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ItemID: 801})
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	require.False(t, finalizationRead(t, f).ExecutionStarted)
}
