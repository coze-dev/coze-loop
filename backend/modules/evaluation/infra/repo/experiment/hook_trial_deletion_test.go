// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// Catches Trial creation/initialization being accepted but deletion rejecting it or rewriting its execution records.
func TestHookTrialDeletionRepositoryRealInitialization(t *testing.T) {
	f := newHookTxFixture(t)
	ctx := context.Background()
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=? AND space_id=?", f.expt, f.space).UpdateColumns(map[string]any{
		"expt_type": int32(entity.ExptType_Offline), "eval_set_source_type": int32(entity.ExptEvalSetSourceType_SingleSet), "eval_set_id": 71,
	}).Error)
	require.NoError(t, f.sql.Create(&model.ExptStats{ID: hookTxSequence.Add(1), SpaceID: f.space, ExptID: f.expt, CreditCost: 7}).Error)
	t.Cleanup(func() {
		for _, table := range []any{&model.ExptItemResult{}, &model.ExptTurnResult{}, &model.ExptItemResultRunLog{}, &model.ExptTurnResultRunLog{}, &model.ExptStats{}} {
			require.NoError(t, f.sql.Unscoped().Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(table).Error)
		}
	})
	in := f.input(true, 0)
	in.RunLog.Mode = int32(entity.EvaluationModeTrialRun)
	created, err := f.repo.CreateRunWithHooks(ctx, in)
	require.NoError(t, err)
	require.Equal(t, entity.EvaluationModeTrialRun, created.Run.Mode)
	item := entity.HookPlanItem{ID: hookTxSequence.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: hookTxSequence.Add(1)}
	appended, err := f.repo.AppendPlanPage(ctx, entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: created.Run.Version}, Items: []entity.HookPlanItem{item}})
	require.NoError(t, err)
	digest, err := entity.AppendHookPlanDigest(entity.NewHookPlanDigest(), []entity.HookPlanItem{item})
	require.NoError(t, err)
	ready, err := f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: appended.Run.Version}, Count: 1, Hash: digest.Hash})
	require.NoError(t, err)
	claimInput := entity.HookClaimAttemptInput{HookAttemptScope: entity.HookAttemptScope{Key: in.Key, OperationID: in.Before.OperationID, Phase: entity.HookPhaseBefore, SnapshotHash: in.Snapshot.Hash, ExecutionScope: "local"}, Owner: "trial-before-worker", AttemptID: hookTxSequence.Add(1), Config: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}, TimeoutSeconds: gptr.Of(int32(30))}}
	for _, op := range ready.Run.Operations {
		if op.Phase == entity.HookPhaseBefore {
			claimInput.ExpectedVersion = op.Version
		}
	}
	claim, err := f.repo.ClaimAttempt(ctx, claimInput)
	require.NoError(t, err)
	require.NotNil(t, claim.Claim)
	beforeDone, err := f.repo.CompleteAttempt(ctx, hookCompletion(t, f, claimInput, claim.Claim))
	require.NoError(t, err)
	require.Equal(t, entity.HookOperationSucceeded, beforeDone.Run.State.Before.Status)
	manifest := entity.HookExecutionManifest{Version: 1, Key: in.Key, Frozen: item, ItemResultID: hookTxSequence.Add(1), ItemRunLogID: hookTxSequence.Add(1),
		Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, TurnIdx: 0, ResultID: hookTxSequence.Add(1)}, {TurnID: 10, TurnIdx: 1, ResultID: hookTxSequence.Add(1)}}, TurnLogsInitialized: gptr.Of(false)}
	init := NewHookExecutionInitializationRepo(f.p)
	written, err := init.WriteExecutionInitializationPage(ctx, entity.HookExecutionInitializationWriteInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: beforeDone.Run.Version}, ExecutionScope: "local", PlanHash: digest.Hash, Items: []entity.HookExecutionManifest{manifest}})
	require.NoError(t, err)
	complete, err := init.CompleteExecutionInitialization(ctx, entity.HookExecutionInitializationCompleteInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: written.RunVersion}, ExecutionScope: "local", PlanHash: digest.Hash, ExpectedItemCount: 1, ExpectedTurnCount: 2})
	require.NoError(t, err)
	require.True(t, complete.Initialized)
	var life model.ExptLifecycleRun
	require.NoError(t, hookRunScope(f.sql, in.Key).First(&life).Error)
	require.True(t, life.ExecutionInitialized)
	require.False(t, life.ExecutionStarted)
	var log model.ExptRunLog
	require.NoError(t, f.sql.First(&log, in.Key.RunID).Error)
	require.Equal(t, int32(entity.EvaluationModeTrialRun), gptr.Indirect(log.Mode))
	var itemBefore model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&itemBefore, manifest.ItemRunLogID).Error)
	require.Equal(t, int32(entity.ItemRunState_Queueing), itemBefore.Status)
	var turnsBefore []model.ExptTurnResult
	require.NoError(t, hookRunScope(f.sql, in.Key).Order("id").Find(&turnsBefore).Error)
	require.Len(t, turnsBefore, 2)
	for _, turn := range turnsBefore {
		require.Equal(t, int32(entity.TurnRunState_Queueing), turn.Status)
	}
	deleted, err := deletionRepo(t, f).DeleteExperiments(ctx, []int64{f.expt, f.expt}, f.space, "local")
	require.NoError(t, err)
	require.Len(t, deleted, 1)
	pending, err := f.repo.GetRun(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, entity.EvaluationModeTrialRun, pending.Mode)
	require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
	require.Equal(t, entity.HookGateClosed, pending.State.Gate)
	require.Equal(t, entity.ExptStatus_Terminated, pending.State.Intent.Status)
	require.False(t, pending.State.After.Activated)
	require.Equal(t, in.Snapshot, pending.Snapshot)
	var itemAfter model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&itemAfter, manifest.ItemRunLogID).Error)
	require.Equal(t, itemBefore, itemAfter)
	var turnsAfter []model.ExptTurnResult
	require.NoError(t, hookRunScope(f.sql, in.Key).Order("id").Find(&turnsAfter).Error)
	require.Equal(t, turnsBefore, turnsAfter)
	var turnLogs int64
	require.NoError(t, hookRunScope(f.sql.Model(&model.ExptTurnResultRunLog{}), in.Key).Count(&turnLogs).Error)
	require.Zero(t, turnLogs)
	_, err = f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: pending.Version}, ItemID: item.ItemID})
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	repeated, err := deletionRepo(t, f).DeleteExperiments(ctx, []int64{f.expt}, f.space, "local")
	require.NoError(t, err)
	require.Empty(t, repeated)
	replayed, err := f.repo.GetRun(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, pending, replayed)
	source, err := NewHookFinalizationRepo(f.p).ReadFinalizationSource(ctx, in.Key)
	require.NoError(t, err)
	require.Equal(t, int32(entity.EvaluationModeTrialRun), source.RunLog.Mode)
	t.Logf("Trial created/fully initialized/deleted: space=%d expt=%d run=%d mode=%d initialized=%t started=%t turns=%d turn_logs=%d after=%t", f.space, f.expt, in.Key.RunID, pending.Mode, life.ExecutionInitialized, life.ExecutionStarted, len(turnsAfter), turnLogs, pending.State.After.Activated)
}
