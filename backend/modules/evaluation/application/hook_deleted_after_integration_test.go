// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// Catches deletion losing committed after work, recovery reading live/latest data,
// and duplicate delivery reissuing an already terminal operation.
func TestHookDeletedAfterRealRepository(t *testing.T) {
	for _, tc := range []struct {
		name    string
		recover bool
	}{
		{name: "pending_after_survives_delete"},
		{name: "uncertain_after_recovers_original_operation", recover: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			f := newExecutorFixture(t, entity.HookPhaseAfter, 10)
			key := f.input.Key
			original, err := f.repo.GetRun(ctx, key)
			require.NoError(t, err)
			require.Equal(t, entity.HookFinalizeCommitted, original.State.Finalize)
			require.Equal(t, entity.HookOperationPending, original.State.After.Status)
			require.Equal(t, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}, original.State.Intent)
			pending := f.operation(t)

			// Divergent live metadata/config makes accidentally rebuilding the frozen request observable.
			liveConfig, err := f.codec.EncodeConfig(ctx, "local-test-key", hookcomponent.ConfigOwner{
				Kind: hookcomponent.ConfigOwnerExperiment, WorkspaceID: key.WorkspaceID,
				ObjectID: key.ExperimentID, ExecutionScope: f.input.ExecutionScope,
			}, &entity.LifecycleHookConf{After: &entity.HookConfig{
				Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/replacement")},
				TimeoutSeconds: gptr.Of(int32(1)), ParametersJSON: gptr.Of(`{"business_key":"replacement"}`),
			}})
			require.NoError(t, err)
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=? AND space_id=?", key.ExperimentID, key.WorkspaceID).
				Updates(map[string]any{"name": "replacement name", "created_by": "replacement-user", "latest_run_id": executorIDs.Add(1), "lifecycle_hook_conf": liveConfig}).Error)
			public := exptmysql.NewExptDAO(f.p)
			visible, err := public.MGetByID(ctx, []int64{key.ExperimentID})
			require.NoError(t, err)
			require.Len(t, visible, 1)
			require.Equal(t, "replacement name", visible[0].Name)
			deletion := experiment.NewHookDeletionRepo(f.p)
			deleted, err := deletion.DeleteExperiments(ctx, []int64{key.ExperimentID}, key.WorkspaceID, f.input.ExecutionScope)
			require.NoError(t, err)
			require.Len(t, deleted, 1)
			require.Equal(t, key.ExperimentID, deleted[0].ID)
			visible, err = public.MGetByID(ctx, []int64{key.ExperimentID})
			require.NoError(t, err)
			require.Empty(t, visible)
			retained, err := f.repo.GetRun(ctx, key)
			require.NoError(t, err)
			require.Equal(t, original, retained, "delete must preserve the committed original Run")
			require.Equal(t, pending, f.operation(t))
			candidates := deletedAfterCandidates(t, f, entity.HookOperationPending)
			require.Len(t, candidates, 1)
			require.Equal(t, f.input.OperationID, candidates[0].OperationID)
			require.Equal(t, entity.HookPhaseAfter, candidates[0].Phase)
			f.input.Key, f.input.OperationID, f.input.Phase = candidates[0].Key, candidates[0].OperationID, candidates[0].Phase
			deleted, err = deletion.DeleteExperiments(ctx, []int64{key.ExperimentID}, key.WorkspaceID, f.input.ExecutionScope)
			require.NoError(t, err)
			require.Empty(t, deleted)
			require.Equal(t, pending, f.operation(t))

			var deliveries []string
			var occurredAt string
			transport := executorTransport(func(_ context.Context, in entity.HookTransportInput) entity.HookTransportResult {
				require.Equal(t, key.WorkspaceID, in.WorkspaceID)
				require.Equal(t, "https://example.com/hook", gptr.Indirect(in.Config.InvokeHTTPInfo.URL))
				require.Equal(t, int32(10), gptr.Indirect(in.Config.TimeoutSeconds))
				require.Equal(t, "hook_executor_"+strconv.FormatInt(key.RunID, 10), in.Request.GetOperationID())
				require.Equal(t, "stable_"+strconv.FormatInt(key.RunID, 10), in.Request.GetIdempotencyKey())
				require.Equal(t, int32(len(deliveries)+1), in.Request.GetAttempt())
				require.NotEmpty(t, in.Request.GetDeliveryID())
				require.Equal(t, strconv.FormatInt(key.WorkspaceID, 10), in.Request.Context.GetWorkspaceID())
				require.Equal(t, strconv.FormatInt(key.ExperimentID, 10), in.Request.Context.GetExperimentID())
				require.Equal(t, strconv.FormatInt(key.RunID, 10), in.Request.Context.GetRunID())
				require.Equal(t, "submit", in.Request.Context.GetRunMode())
				require.Equal(t, "original name", in.Request.Context.Experiment.GetName())
				require.Equal(t, "opaque-user", in.Request.Context.Initiator.GetUserID())
				require.Equal(t, "terminated", in.Request.Context.GetTerminalStatus())
				require.Equal(t, "cancel", in.Request.Context.GetTerminalReason())
				require.Equal(t, `{"business_key":"generic","number":"9007199254740993"}`, in.Request.GetParameters())
				if len(deliveries) == 0 {
					occurredAt = in.Request.GetOccurredAt()
					require.NotEmpty(t, occurredAt)
				} else {
					require.Equal(t, occurredAt, in.Request.GetOccurredAt())
					require.NotContains(t, deliveries, in.Request.GetDeliveryID())
				}
				deliveries = append(deliveries, in.Request.GetDeliveryID())
				if tc.recover && len(deliveries) == 1 {
					return entity.HookTransportResult{Outcome: entity.HookOutcome{Code: entity.HookTimeoutUncertain}, LocalCompletedAt: time.Now()}
				}
				response, outcome := hookinfra.DecodeResponse(200, "application/json", strings.NewReader(`{"status":"succeeded","result":{"summary":"original run cleaned"}}`))
				return entity.HookTransportResult{Response: response, Outcome: outcome, LocalCompletedAt: time.Now()}
			})
			executor := f.executor(t, transport, hookinfra.NewCompletionProjector())
			out, err := executor.Execute(ctx, f.input)
			require.NoError(t, err)
			wantAttempts := int32(1)
			if tc.recover {
				wantAttempts = 2
				require.True(t, out.RecoveryPending)
				running := f.operation(t)
				require.Equal(t, "running", running.Status)
				deleted, err = deletion.DeleteExperiments(ctx, []int64{key.ExperimentID}, key.WorkspaceID, f.input.ExecutionScope)
				require.NoError(t, err)
				require.Empty(t, deleted)
				require.Equal(t, running, f.operation(t))
				f.repo = experiment.NewHookRunRepo(f.p)
				executor = f.executor(t, transport, hookinfra.NewCompletionProjector())
				out, err = executor.Execute(ctx, f.input)
				require.NoError(t, err)
				require.True(t, out.RecoveryPending)
				require.Len(t, deliveries, 1, "unexpired duplicate must not send HTTP again")
				// Advance only this fixture's lease/deadline; recovery and retry transitions stay real.
				require.NoError(t, f.sql.Exec("UPDATE expt_lifecycle_hook_run SET lease_until=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND),attempt_deadline=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND) WHERE space_id=? AND expt_id=? AND expt_run_id=? AND operation_id=?", key.WorkspaceID, key.ExperimentID, key.RunID, f.input.OperationID).Error)
				out, err = executor.Execute(ctx, f.input)
				require.NoError(t, err)
				require.True(t, out.Effects.Retry.Retry)
				require.Equal(t, "retry_wait", f.operation(t).Status)
				require.Equal(t, int32(1), f.operation(t).Attempt)
				require.Len(t, deliveries, 1)
				require.NoError(t, f.sql.Exec("UPDATE expt_lifecycle_hook_run SET next_attempt_at=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND) WHERE space_id=? AND expt_id=? AND expt_run_id=? AND operation_id=?", key.WorkspaceID, key.ExperimentID, key.RunID, f.input.OperationID).Error)
				retry := deletedAfterCandidates(t, f, entity.HookOperationRetryWait)
				require.Len(t, retry, 1)
				require.Equal(t, candidates[0].OperationID, retry[0].OperationID)
				require.Equal(t, candidates[0].Phase, retry[0].Phase)
				f.input.AttemptID = executorIDs.Add(1)
				out, err = executor.Execute(ctx, f.input)
				require.NoError(t, err)
			}
			require.False(t, out.RecoveryPending)
			require.False(t, out.LatestProjected)
			require.Equal(t, entity.HookGateClosed, out.Run.State.Gate)
			require.Equal(t, entity.HookFinalizeCommitted, out.Run.State.Finalize)
			require.Equal(t, original.State.Intent, out.Run.State.Intent)
			require.Equal(t, original.State.Status, out.Run.State.Status)
			require.Equal(t, original.TerminalAt, out.Run.TerminalAt)
			require.Equal(t, original.Snapshot, out.Run.Snapshot)
			terminal := f.operation(t)
			require.Equal(t, "succeeded", terminal.Status)
			require.Equal(t, wantAttempts, terminal.Attempt)
			require.JSONEq(t, `{"summary":"original run cleaned"}`, string(gptr.Indirect(terminal.ResultRedacted)))
			var audits []model.ExptLifecycleHookAttempt
			require.NoError(t, f.sql.Where("operation_id=?", f.input.OperationID).Order("attempt").Find(&audits).Error)
			require.Len(t, audits, int(wantAttempts))
			require.Equal(t, "SUCCEEDED", gptr.Indirect(audits[len(audits)-1].ResultCategory))
			require.NotNil(t, audits[len(audits)-1].FinishedAt)
			if tc.recover {
				require.Equal(t, "TIMEOUT_UNCERTAIN", gptr.Indirect(audits[0].ResultCategory))
				require.Nil(t, audits[0].FinishedAt)
			}
			finalRun, err := f.repo.GetRun(ctx, key)
			require.NoError(t, err)
			deleted, err = deletion.DeleteExperiments(ctx, []int64{key.ExperimentID}, key.WorkspaceID, f.input.ExecutionScope)
			require.NoError(t, err)
			require.Empty(t, deleted)
			f.input.AttemptID = executorIDs.Add(1)
			_, err = executor.Execute(ctx, f.input)
			require.NoError(t, err)
			require.Len(t, deliveries, int(wantAttempts))
			require.Equal(t, terminal, f.operation(t))
			unchanged, err := f.repo.GetRun(ctx, key)
			require.NoError(t, err)
			require.Equal(t, finalRun, unchanged)
			var duplicateAudits []model.ExptLifecycleHookAttempt
			require.NoError(t, f.sql.Where("operation_id=?", f.input.OperationID).Order("attempt").Find(&duplicateAudits).Error)
			require.Equal(t, audits, duplicateAudits)
			require.Empty(t, deletedAfterCandidates(t, f, entity.HookOperationPending))
			require.Empty(t, deletedAfterCandidates(t, f, entity.HookOperationRetryWait))
			visible, err = public.MGetByID(ctx, []int64{key.ExperimentID})
			require.NoError(t, err)
			require.Empty(t, visible)
		})
	}
}

func deletedAfterCandidates(t *testing.T, f *executorFixture, status entity.HookOperationStatus) []entity.HookOperationCandidate {
	t.Helper()
	clock, err := experiment.NewHookWorkerClock(f.p)
	require.NoError(t, err)
	now, err := clock.Now(context.Background())
	require.NoError(t, err)
	scanner := experiment.NewHookScanRepo(f.p)
	in := entity.HookScanInput{ExecutionScope: f.input.ExecutionScope, Status: status, Now: now, Limit: 100}
	var found []entity.HookOperationCandidate
	for {
		page, err := scanner.ScanDueOperations(context.Background(), in)
		require.NoError(t, err)
		for _, candidate := range page.Candidates {
			if candidate.Key == f.input.Key {
				found = append(found, candidate)
			}
		}
		if !page.HasMore {
			return found
		}
		require.NotNil(t, page.NextCursor)
		in.Cursor = page.NextCursor
	}
}
