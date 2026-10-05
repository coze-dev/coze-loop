// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestHookProjectionExecutorRetryClassificationAndClearOldError(t *testing.T) {
	for _, code := range []string{"HTTP_ERROR", "HOOK_SECURITY_ERROR", "资源未就绪😀"} {
		t.Run(code, func(t *testing.T) {
			f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
			e := projectionExecutor(t, f, 200, fmt.Sprintf(`{"status":"failed","error":{"code":%q,"message":"可重试的业务失败","retryable":true}}`, code))
			out, err := e.Execute(context.Background(), f.input)
			require.NoError(t, err)
			require.True(t, out.Effects.Retry.Retry)
			require.False(t, out.Effects.BeginFinalize)
			require.Equal(t, "retry_wait", f.operation(t).Status)
			summary := projectionSummary(t, f)
			require.Nil(t, summary.Response)
			require.Nil(t, summary.Error)
			var audit model.ExptLifecycleHookAttempt
			require.NoError(t, f.sql.Where("id=?", f.input.AttemptID).First(&audit).Error)
			require.Equal(t, "HOOK_FAILED", gptr.Indirect(audit.ResultCategory))
			require.EqualValues(t, 200, gptr.Indirect(audit.HTTPStatus))
			require.NoError(t, f.sql.Exec("UPDATE expt_lifecycle_hook_run SET next_attempt_at=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND) WHERE operation_id=? AND space_id=? AND expt_id=?", f.input.OperationID, f.input.Key.WorkspaceID, f.input.Key.ExperimentID).Error)
			f.input.AttemptID = executorIDs.Add(1)
			e = projectionExecutor(t, f, 200, `{"status":"succeeded","result":{"custom":"第二次完成"}}`)
			out, err = e.Execute(context.Background(), f.input)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateReady, out.Run.State.Gate)
			row := f.operation(t)
			require.Equal(t, int32(2), row.Attempt)
			require.Nil(t, row.ErrorCode)
			require.Nil(t, row.ErrorMessage)
			summary = projectionSummary(t, f)
			require.Equal(t, map[string]string{"custom": "第二次完成"}, summary.Response.GetResult_())
			require.Nil(t, summary.Error)
		})
	}
}

func TestHookProjectionExecutorNonBusinessErrorsNeverExposeBody(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body, code string
	}{
		{"non200", 403, `{"status":"failed","error":{"code":"raw-secret-code","message":"raw-secret-message","retryable":true}}`, "HTTP_ERROR"},
		{"invalidJSON", 200, `{"raw-secret":`, "PROTOCOL_ERROR"},
		{"conflictingStatus", 200, `{"status":"succeeded","error":{"message":"raw-secret"}}`, "PROTOCOL_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
			out, err := projectionExecutor(t, f, tc.status, tc.body).Execute(context.Background(), f.input)
			require.NoError(t, err)
			require.True(t, out.Effects.BeginFinalize)
			summary := projectionSummary(t, f)
			require.Equal(t, tc.code, summary.Error.GetCode())
			require.Equal(t, tc.code, summary.Error.GetMessage())
			encoded, err := json.Marshal(summary)
			require.NoError(t, err)
			for _, private := range []string{"raw-secret", "opaque-user", "local-test-key", "original name"} {
				require.NotContains(t, string(encoded), private)
			}
			var audit model.ExptLifecycleHookAttempt
			require.NoError(t, f.sql.Where("id=?", f.input.AttemptID).First(&audit).Error)
			require.NotContains(t, string(gptr.Indirect(audit.ErrorRedacted)), "raw-secret")
		})
	}
}

type projectionErrorFault struct {
	hookcomponent.CompletionProjector
}

func (projectionErrorFault) ProjectError(context.Context, entity.HookRunKey, entity.HookPhase, *spi.InvokeExperimentHookResponse) (*spi.HookError, error) {
	return nil, errors.New("internal-key-ref-and-secret")
}

type projectionNilError struct {
	hookcomponent.CompletionProjector
}

func (projectionNilError) ProjectError(context.Context, entity.HookRunKey, entity.HookPhase, *spi.InvokeExperimentHookResponse) (*spi.HookError, error) {
	return nil, nil
}

func TestHookProjectionExecutorFaultAndInconsistentResponseFailClosed(t *testing.T) {
	for _, name := range []string{"projector error", "nil projected error", "retry mismatch", "non200 success", "wrong status", "nil response"} {
		t.Run(name, func(t *testing.T) {
			f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
			p := hookinfra.NewCompletionProjector()
			if name == "projector error" {
				p = projectionErrorFault{p}
			}
			if name == "nil projected error" {
				p = projectionNilError{p}
			}
			e := f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult {
				r := entity.HookTransportResult{Outcome: entity.HookOutcome{Code: entity.HookFailed, HTTPStatus: 200}, Response: &spi.InvokeExperimentHookResponse{Status: gptr.Of("failed"), Error: &spi.HookError{Message: gptr.Of("business-display")}}, LocalCompletedAt: time.Now()}
				switch name {
				case "retry mismatch":
					r.Response.Error.Retryable = gptr.Of(true)
				case "non200 success":
					r.Outcome = entity.HookOutcome{Code: entity.HookSucceeded, HTTPStatus: 403}
					r.Response = &spi.InvokeExperimentHookResponse{Status: gptr.Of("succeeded"), Result_: map[string]string{"raw": "secret"}}
				case "wrong status":
					r.Response.Status = gptr.Of("succeeded")
				case "nil response":
					r.Response = nil
				}
				return r
			}), p)
			_, err := e.Execute(context.Background(), f.input)
			require.NoError(t, err)
			summary := projectionSummary(t, f)
			require.Equal(t, "HOOK_SECURITY_ERROR", summary.Error.GetCode())
			require.Equal(t, "HOOK_SECURITY_ERROR", summary.Error.GetMessage())
			require.Empty(t, gptr.Indirect(f.operation(t).ResultRedacted))
		})
	}
}

func TestHookProjectionExecutorCancelRejectsLateDisplay(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	original := f.repo
	f.repo = &executorRepoProbe{IHookRepo: original, complete: func(c context.Context, in entity.HookCompleteAttemptInput) (entity.HookAttemptStoreResult, error) {
		run, err := original.GetRun(c, in.Key)
		if err != nil {
			return entity.HookAttemptStoreResult{}, err
		}
		_, err = original.BeginFinalize(c, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancel"}})
		if err != nil {
			return entity.HookAttemptStoreResult{}, err
		}
		return original.CompleteAttempt(c, in)
	}}
	out, err := projectionExecutor(t, f, 200, `{"status":"succeeded","result":{"custom":"late-business"}}`).Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.True(t, out.Effects.LateIgnored)
	require.Equal(t, entity.HookGateClosed, out.Run.State.Gate)
	require.Empty(t, gptr.Indirect(f.operation(t).ResultRedacted))
	summary := projectionSummary(t, f)
	require.Nil(t, summary.Response)
	var audit model.ExptLifecycleHookAttempt
	require.NoError(t, f.sql.Where("id=?", f.input.AttemptID).First(&audit).Error)
	require.True(t, audit.LateIgnored)
}

func TestHookProjectionExecutorAfterOldRunAndScopeIsolation(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseAfter, 10)
	require.NoError(t, f.sql.Exec("UPDATE experiment SET latest_run_id=? WHERE id=? AND space_id=?", f.input.Key.RunID+1, f.input.Key.ExperimentID, f.input.Key.WorkspaceID).Error)
	_, err := projectionExecutor(t, f, 200, `{"status":"succeeded","result":{"custom":"old-run"}}`).Execute(context.Background(), f.input)
	require.NoError(t, err)
	summary := projectionSummary(t, f)
	require.Equal(t, map[string]string{"custom": "old-run"}, summary.Response.GetResult_())
	wrong := f.input.Key
	wrong.WorkspaceID++
	got, err := experiment.NewHookSummaryRepo(f.p).MGetSummaries(context.Background(), []entity.HookRunKey{wrong})
	require.Error(t, err)
	require.Nil(t, got)
	bad := f.input
	bad.Key = wrong
	_, err = projectionExecutor(t, f, 200, `{"status":"succeeded"}`).Execute(context.Background(), bad)
	require.Error(t, err)
}

func TestHookProjectionExecutorFullSizedResponseRemainsReadable(t *testing.T) {
	const empty = `{"status":"succeeded","result":{"a":"","b":"","c":"","d":""}}`
	last := 32768 - len(empty) - 3*8192
	body := fmt.Sprintf(`{"status":"succeeded","result":{"a":"%s","b":"%s","c":"%s","d":"%s"}}`, strings.Repeat("<", 8192), strings.Repeat("&", 8192), strings.Repeat(">", 8192), strings.Repeat("x", last))
	require.Len(t, body, 32768)
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	_, err := projectionExecutor(t, f, 200, body).Execute(context.Background(), f.input)
	require.NoError(t, err)
	summary := projectionSummary(t, f)
	require.Equal(t, entity.HookOperationSucceeded, summary.Status)
	require.Equal(t, map[string]string{"a": strings.Repeat("<", 8192), "b": strings.Repeat("&", 8192), "c": strings.Repeat(">", 8192), "d": strings.Repeat("x", last)}, summary.Response.GetResult_())
}

func TestHookProjectionExecutorDefaultsAndExhaustedBusinessRetry(t *testing.T) {
	for _, body := range []string{`{"status":"succeeded"}`, `{"status":"succeeded","result":null}`, `{"status":"succeeded","result":{}}`} {
		f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
		_, err := projectionExecutor(t, f, 200, body).Execute(context.Background(), f.input)
		require.NoError(t, err)
		summary := projectionSummary(t, f)
		require.NotNil(t, summary.Response.Result_)
		require.Empty(t, summary.Response.Result_)
	}
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	e := projectionExecutor(t, f, 200, `{"status":"failed","error":{"message":"业务失败继续可重试","retryable":true}}`)
	out, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.True(t, out.Effects.Retry.Retry)
	require.NoError(t, f.sql.Exec("UPDATE expt_lifecycle_hook_run SET next_attempt_at=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND) WHERE operation_id=? AND space_id=? AND expt_id=?", f.input.OperationID, f.input.Key.WorkspaceID, f.input.Key.ExperimentID).Error)
	f.input.AttemptID = executorIDs.Add(1)
	out, err = e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.False(t, out.Effects.Retry.Retry)
	summary := projectionSummary(t, f)
	require.Equal(t, "HOOK_FAILED", summary.Error.GetCode())
	require.Equal(t, "业务失败继续可重试", summary.Error.GetMessage())
	require.False(t, summary.Error.GetRetryable())
}
