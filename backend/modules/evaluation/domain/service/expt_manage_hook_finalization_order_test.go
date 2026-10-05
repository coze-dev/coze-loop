// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	"github.com/stretchr/testify/require"
)

type managerOrderingProbeState struct {
	Status         entity.ExptStatus        `json:"status"`
	IntentStatus   entity.ExptStatus        `json:"intent_status"`
	Reason         string                   `json:"reason"`
	Finalize       entity.HookFinalizeState `json:"finalize"`
	AfterActivated bool                     `json:"after_activated"`
}

func managerOrderingProbeSnapshot(t *testing.T, f *finalizationManagerFixture) managerOrderingProbeState {
	t.Helper()
	state, err := f.repo.GetRun(context.Background(), f.key)
	require.NoError(t, err)
	return managerOrderingProbeState{Status: state.State.Status, IntentStatus: state.State.Intent.Status, Reason: state.State.Intent.Reason, Finalize: state.State.Finalize, AfterActivated: state.State.After.Activated}
}

func managerOrderingProbeSettled(t *testing.T, mode entity.ExptRunMode) *finalizationManagerFixture {
	t.Helper()
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=? AND space_id=? AND expt_id=?", f.key.RunID, f.space, f.expt).UpdateColumn("mode", int32(mode)).Error)
	stats, err := f.deps.Repository.ReadFinalizationStats(ctx, f.key, "local")
	require.NoError(t, err)
	require.Equal(t, entity.HookFinalizationCounts{Success: 1}, stats.Items)
	require.Equal(t, entity.HookFinalizationCounts{Success: 1}, stats.Turns)
	require.False(t, managerOrderingProbeSnapshot(t, f).AfterActivated)
	return f
}

func managerOrderingProbeError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func managerOrderingProbeEmit(t *testing.T, mode entity.ExptRunMode, kind string, firstErr, secondErr error, first, second managerOrderingProbeState, notifications int) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"mode": mode, "kind": kind, "first_error": managerOrderingProbeError(firstErr), "second_error": managerOrderingProbeError(secondErr), "after_first": first, "after_second": second, "notifications": notifications})
	require.NoError(t, err)
	t.Logf("MANAGER_ORDERING_PROBE %s", raw)
}

// Catches irreversible normal completion before the later error-path decision is known.
func TestManagerOrderingProbe_ExplicitFailure(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := managerOrderingProbeSettled(t, mode)
			ctx := context.Background()
			cid := fmt.Sprintf("ordering-probe:%d", f.key.RunID)
			firstErr := f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, &entity.Session{UserID: "user"}, entity.WithCID(cid))
			first := managerOrderingProbeSnapshot(t, f)
			assertManagerOrderingPreparation(t, f)
			reason := "scheduler error after settled items"
			secondErr := f.manager.CompleteExpt(ctx, f.expt, gptr.Of(f.key.RunID), f.space, &entity.Session{UserID: "user"}, entity.WithCID(cid), entity.WithStatus(entity.ExptStatus_Failed), entity.WithStatusMessage(reason))
			second := managerOrderingProbeSnapshot(t, f)
			managerOrderingProbeEmit(t, mode, "explicit_failed_after_default_run", firstErr, secondErr, first, second, f.notifications)
			if firstErr != nil {
				t.Errorf("default CompleteRun unexpectedly failed: %v", firstErr)
			}
			require.Equal(t, entity.HookFinalizeNone, first.Finalize)
			if first.AfterActivated {
				t.Error("incorrect Success after activated before explicit terminal decision")
			}
			if second.AfterActivated && second.IntentStatus == entity.ExptStatus_Success {
				t.Error("explicit Failed decision left an activated Success after")
			}
			require.NoError(t, secondErr)
			require.Equal(t, entity.HookFinalizeCommitted, second.Finalize)
			require.Equal(t, entity.ExptStatus_Failed, second.Status)
			require.True(t, second.AfterActivated)
			require.Equal(t, 1, f.notifications)
			var runLog model.ExptRunLog
			require.NoError(t, f.sql.First(&runLog, f.key.RunID).Error)
			require.Equal(t, int32(1), runLog.SuccessCnt)
			require.Zero(t, runLog.FailCnt)
			var expt model.Experiment
			require.NoError(t, f.sql.First(&expt, f.expt).Error)
			require.Equal(t, int32(entity.ExptStatus_Failed), expt.Status)
			require.Equal(t, reason, string(gptr.Indirect(expt.StatusMessage)))
			if secondErr == nil {
				require.Equal(t, entity.ExptStatus_Failed, second.IntentStatus)
				require.Empty(t, second.Reason)
			}
		})
	}
}

func TestManagerOrderingProbe_NormalControl(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := managerOrderingProbeSettled(t, mode)
			ctx := context.Background()
			cid := fmt.Sprintf("ordering-control:%d", f.key.RunID)
			firstErr := f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, &entity.Session{UserID: "user"}, entity.WithCID(cid))
			first := managerOrderingProbeSnapshot(t, f)
			assertManagerOrderingPreparation(t, f)
			secondErr := f.manager.CompleteExpt(ctx, f.expt, gptr.Of(f.key.RunID), f.space, &entity.Session{UserID: "user"}, entity.WithCID(cid))
			second := managerOrderingProbeSnapshot(t, f)
			managerOrderingProbeEmit(t, mode, "normal_control", firstErr, secondErr, first, second, f.notifications)
			require.NoError(t, firstErr)
			require.NoError(t, secondErr)
			require.Equal(t, entity.ExptStatus_Success, second.IntentStatus)
			require.True(t, second.AfterActivated)
			require.Equal(t, entity.HookFinalizeCommitted, second.Finalize)
			require.Equal(t, 1, f.notifications)
		})
	}
}

func TestManagerOrderingProbe_UnsettledTermination(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := managerOrderingProbeSettled(t, mode)
			ctx := context.Background()
			require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil, entity.WithCID("arbitrary")))
			before := managerOrderingProbeSnapshot(t, f)
			assertManagerOrderingPreparation(t, f)
			require.Equal(t, entity.HookFinalizeNone, before.Finalize)
			require.False(t, before.AfterActivated)
			require.ErrorIs(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Terminated), entity.WithStatusMessage("debt error")), entity.ErrHookFinalizationUnsettled)
			after := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizePending, after.State.Finalize)
			require.Equal(t, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}, after.State.Intent)
			require.Equal(t, "debt error", gptr.Indirect(after.DisplayMessage))
			require.False(t, after.State.After.Activated)
			require.Zero(t, f.notifications)
		})
	}
}

func assertManagerOrderingPreparation(t *testing.T, f *finalizationManagerFixture) {
	t.Helper()
	ctx := context.Background()
	state := managerOrderingProbeSnapshot(t, f)
	require.Equal(t, entity.HookFinalizeNone, state.Finalize)
	require.False(t, state.AfterActivated)
	quota, err := dao.NewQuotaDAO(f.redis).GetQuotaSpaceExpt(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, int64(123), quota.ExptID2RunTime[f.expt])
	require.Equal(t, int64(1), f.redis.Exists(ctx, fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)).Val())
	require.Zero(t, f.notifications)
}

func TestManagerOrderingProbe_ExplicitRunDecisionRejectedBeforeEffects(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Failed, entity.ExptStatus_Terminated} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := newFinalizationManagerFixture(t)
			require.ErrorIs(t, f.manager.CompleteRun(context.Background(), f.expt, f.key.RunID, f.space, nil, entity.WithStatus(status)), entity.ErrHookFinalizationUnsupported)
			assertManagerOrderingPreparation(t, f)
		})
	}
}

func TestManagerOrderingProbe_ExplicitFailureRecoveryKeepsIntent(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	fault := &finalizationRunFault{IHookRepo: f.repo, failCommit: true}
	f.deps.Runs = fault
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil))
	require.Error(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Failed), entity.WithStatusMessage("run failure")))
	pending := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
	require.Equal(t, entity.HookTerminalIntent{Status: entity.ExptStatus_Failed}, pending.State.Intent)
	require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil))
	require.Equal(t, pending, finalizationRead(t, f))
	fault.failCommit = false
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	done := finalizationRead(t, f)
	require.Equal(t, pending.State.Intent, done.State.Intent)
	require.True(t, done.State.After.Activated)
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil))
	require.Equal(t, done, finalizationRead(t, f))
	require.Equal(t, 1, f.notifications)
}
