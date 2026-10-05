// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	tm "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestHookBoundExecutionDispatchAndTurnPinsMySQL(t *testing.T) {
	f := newBoundInitFixture(t, 2, entity.EvaluationModeSubmit, false, true)
	r := boundInitializationRepo(t, f)
	_, err := f.initialize(t, r)
	require.NoError(t, err)
	bindRuntimeFinalization(t, f)
	changed, err := f.manager.finalization.Repository.(repo.IHookSchedulerRepo).PersistHookDispatch(context.Background(), entity.HookSchedulerDispatchInput{Key: f.key, ExecutionScope: "local", ItemIDs: []int64{f.items[0].ItemID, f.items[1].ItemID}})
	require.NoError(t, err)
	require.Equal(t, []int64{f.items[0].ItemID, f.items[1].ItemID}, changed)
}

func TestHookBoundExecutionTurnInitializationMySQL(t *testing.T) {
	f := newBoundInitFixture(t, 2, entity.EvaluationModeSubmit, false, true)
	r := boundInitializationRepo(t, f)
	_, err := f.initialize(t, r)
	require.NoError(t, err)
	ctx := context.Background()
	run, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	_, err = r.(repo.IHookRepo).AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ItemID: f.items[0].ItemID})
	require.NoError(t, err)
	stores, err := exptinfra.NewBoundHookRuntimeRepositories(f.p, f.binding)
	require.NoError(t, err)
	progress := stores.Progress
	handled, _, err := progress.(repo.IHookTurnLogInitializer).InitializeHookTurnRunLogs(ctx, f.key, f.items[0].ItemID, f.items[0].ItemVersionID, nil)
	require.NoError(t, err)
	require.True(t, handled, "bound multi-set PreEval must not fall back to legacy unpinned turn logs")
	control, err := progress.(repo.IHookConsumerControlRepo).ApplyHookConsumerControl(ctx, entity.HookConsumerControlInput{Key: f.key, ItemID: f.items[0].ItemID, Action: entity.HookConsumerFail, ErrorMessage: "deliberate execution failure"})
	require.NoError(t, err)
	require.True(t, control.Handled)
	require.True(t, control.Changed)
}

func TestHookBoundExecutionFullChainMySQL(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			option := "before"
			if mode == entity.EvaluationModeTrialRun {
				option = "async"
			}
			f, deps, pub, target := executionChainFixture(t, mode, option)
			runtime, err := NewBoundHookRuntimeExecution(deps)
			require.NoError(t, err)
			// Only the external filter/index boundary is replaced.
			runtime.Scheduler.ResultSvc = hookPersistenceFilters{runtime.Scheduler.ResultSvc}
			runtime.Scheduler.schedulerModeFactory.(*DefaultSchedulerModeFactory).resultSvc = runtime.Scheduler.ResultSvc
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			gate, err := deps.Repositories.Gate.CanDispatch(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateWaiting, gate.Gate)
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: mode, CreatedAt: time.Now().Unix(), Session: &entity.Session{UserID: "original-user"}}
			if option == "before" {
				require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
				require.Empty(t, pub.items)
				require.Zero(t, target.calls)
				completeExecutionChainBefore(t, f)
			}
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"target_id": 999, "target_version_id": 998}).Error)
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			require.Len(t, pub.items, 2)
			for _, item := range pub.items {
				require.NoError(t, runtime.Consumer.Eval(ctx, item))
				if option == "async" {
					var record tm.TargetRecord
					require.NoError(t, f.sql.Where("experiment_run_id=? AND item_id=?", f.key.RunID, item.EvalSetItemID).First(&record).Error)
					resume, err := runtime.Consumer.evalAsyncRepo.GetEvalAsyncCtx(ctx, strconv.FormatInt(record.ID, 10))
					require.NoError(t, err)
					require.Equal(t, item.ExptRunID, resume.Event.ExptRunID)
					require.NoError(t, f.sql.Model(&tm.TargetRecord{}).Where("id=?", record.ID).UpdateColumn("status", int32(entity.EvalTargetRunStatusSuccess)).Error)
					callback := *item
					callback.AsyncReportTrigger = true
					require.NoError(t, runtime.Consumer.Eval(ctx, &callback))
				}
			}
			require.Equal(t, 2, target.calls)
			var logs []model.ExptItemResultRunLog
			require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Find(&logs).Error)
			require.Len(t, logs, 2)
			for _, log := range logs {
				require.Equal(t, int32(entity.ItemRunState_Success), log.Status)
			}
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			run := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize)
			require.True(t, run.State.After.Activated)
			var refs int64
			require.NoError(t, f.sql.Model(&model.ExptTurnEvaluatorResultRef{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&refs).Error)
			require.Equal(t, int64(2), refs)
		})
	}
}

func completeExecutionChainBefore(t *testing.T, f *finalizationManagerFixture) {
	t.Helper()
	completeExecutionChainBeforeOutcome(t, f, entity.HookOutcome{Code: entity.HookSucceeded})
}

func completeExecutionChainBeforeOutcome(t *testing.T, f *finalizationManagerFixture, outcome entity.HookOutcome) {
	t.Helper()
	ctx := context.Background()
	run := finalizationRead(t, f)
	config, hash, err := f.manager.hooks.Codec.DecodePhase(ctx, f.key, "local", run.Snapshot, entity.HookPhaseBefore)
	require.NoError(t, err)
	scope := entity.HookAttemptScope{Key: f.key, OperationID: run.State.Before.ID, Phase: entity.HookPhaseBefore, ExecutionScope: "local", SnapshotHash: hash}
	for _, op := range run.Operations {
		if op.Phase == entity.HookPhaseBefore {
			scope.ExpectedVersion = op.Version
		}
	}
	claim, err := f.repo.ClaimAttempt(ctx, entity.HookClaimAttemptInput{HookAttemptScope: scope, Owner: "chain-before", AttemptID: finalizationTestIDs.Add(1), Config: config})
	require.NoError(t, err)
	require.NotNil(t, claim.Claim)
	scope.ExpectedVersion = claim.Claim.Version
	_, err = f.repo.CompleteAttempt(ctx, entity.HookCompleteAttemptInput{HookRenewAttemptInput: entity.HookRenewAttemptInput{HookAttemptScope: scope, HookAttemptIdentity: claim.Claim.HookAttemptIdentity}, Config: config, Outcome: outcome, CompletedAt: claim.Claim.StartedAt})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, f.sql.Where("operation_id=?", scope.OperationID).Delete(&model.ExptLifecycleHookAttempt{}).Error)
	})
}
