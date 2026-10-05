// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
)

func hookStateTime() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

func runningHookState() HookRunState {
	now := hookStateTime()
	return HookRunState{
		Key: HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, Status: ExptStatus_Processing,
		Gate: HookGateWaiting, Finalize: HookFinalizeNone,
		Before: HookOperation{ID: "before", Status: HookOperationRunning, Activated: true, Attempt: 1, Generation: 7,
			LeaseUntil: now.Add(30 * time.Second), AttemptDeadline: now.Add(180 * time.Second), Deadline: now.Add(420 * time.Second)},
		After: HookOperation{ID: "after", Status: HookOperationPending},
	}
}

func beforeResult() HookResultInput {
	return HookResultInput{Token: HookClaimToken{Run: HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, OperationID: "before", Phase: HookPhaseBefore, Attempt: 1, Generation: 7},
		CompletedAt: hookStateTime(), CommitAt: hookStateTime(), Outcome: HookOutcome{Code: HookSucceeded}}
}

func TestHookOnTimeHTTPResponseSurvivesDelayedCommit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"status":"succeeded","result":{"summary":"ok"}}`)
	}))
	t.Cleanup(server.Close)
	response, err := server.Client().Post(server.URL, "application/json", nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	var payload struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))

	// A controlled clock isolates the EOF-to-commit delay from network timing.
	state := runningHookState()
	completedAt := hookStateTime().Add(179 * time.Second)
	state.Before.LeaseUntil = hookStateTime().Add(185 * time.Second)
	input := beforeResult()
	input.Outcome = ClassifyHookOutcome("", response.StatusCode, payload.Status, false)
	input.CompletedAt = completedAt
	input.CommitAt = hookStateTime().Add(181 * time.Second)
	require.True(t, completedAt.Before(state.Before.AttemptDeadline))
	require.True(t, input.CommitAt.After(state.Before.AttemptDeadline))
	require.True(t, input.CommitAt.Before(state.Before.LeaseUntil))
	original := state
	got, err := CompleteHookOperation(&state, input)
	require.NoError(t, err)
	require.True(t, got.Changed, "on-time full HTTP response must survive a delayed commit within the lease")
	require.Equal(t, HookOperationSucceeded, got.State.Before.Status)
	require.Equal(t, HookGateReady, got.State.Gate)
	require.Empty(t, got.Effects)
	require.Equal(t, original, state)
	require.Equal(t, original.After, got.State.After)
}

func TestHookCompletionAndCommitBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		completedAt, commitAt time.Duration
		change                func(*HookRunState, *HookResultInput)
		wantChanged           bool
	}{
		{name: "just before deadline", completedAt: 180*time.Second - time.Nanosecond, commitAt: 181 * time.Second, wantChanged: true},
		{name: "commit at HTTP deadline", completedAt: 179 * time.Second, commitAt: 180 * time.Second, wantChanged: true},
		{name: "EOF at deadline", completedAt: 180 * time.Second, commitAt: 181 * time.Second},
		{name: "EOF after deadline", completedAt: 180*time.Second + time.Nanosecond, commitAt: 181 * time.Second},
		{name: "commit just before lease expires", completedAt: 179 * time.Second, commitAt: 185*time.Second - time.Nanosecond, wantChanged: true},
		{name: "commit at lease expiry", completedAt: 179 * time.Second, commitAt: 185 * time.Second},
		{name: "commit after lease expiry", completedAt: 179 * time.Second, commitAt: 186 * time.Second},
		{name: "old generation after DB delay", completedAt: 179 * time.Second, commitAt: 181 * time.Second,
			change: func(s *HookRunState, _ *HookResultInput) { s.Before.Generation++ }},
		{name: "old attempt after DB delay", completedAt: 179 * time.Second, commitAt: 181 * time.Second,
			change: func(s *HookRunState, _ *HookResultInput) { s.Before.Attempt++ }},
		{name: "cancelled while committing", completedAt: 179 * time.Second, commitAt: 181 * time.Second,
			change: func(s *HookRunState, _ *HookResultInput) { s.Status = ExptStatus_Terminating; s.Gate = HookGateClosed }},
		{name: "closed gate while committing", completedAt: 179 * time.Second, commitAt: 181 * time.Second,
			change: func(s *HookRunState, _ *HookResultInput) { s.Gate = HookGateClosed }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := runningHookState()
			state.Before.LeaseUntil = hookStateTime().Add(185 * time.Second)
			input := beforeResult()
			input.CompletedAt = hookStateTime().Add(tc.completedAt)
			input.CommitAt = hookStateTime().Add(tc.commitAt)
			if tc.change != nil {
				tc.change(&state, &input)
			}
			original := state
			got, err := CompleteHookOperation(&state, input)
			require.NoError(t, err)
			require.Equal(t, tc.wantChanged, got.Changed)
			require.Equal(t, original, state)
			if !tc.wantChanged {
				require.Equal(t, original, got.State)
				require.Equal(t, HookStateEffects{LateIgnored: true}, got.Effects)
				return
			}
			require.Equal(t, HookOperationSucceeded, got.State.Before.Status)
			require.Equal(t, HookGateReady, got.State.Gate)
			require.Equal(t, original.After, got.State.After)
			require.Empty(t, got.Effects)
		})
	}
}

func TestHookInvalidCompletionTimesCannotSucceed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*HookResultInput)
	}{
		{"missing completion", func(i *HookResultInput) { i.CompletedAt = time.Time{} }},
		{"missing commit", func(i *HookResultInput) { i.CommitAt = time.Time{} }},
		{"both missing", func(i *HookResultInput) { i.CompletedAt = time.Time{}; i.CommitAt = time.Time{} }},
		{"completion in future", func(i *HookResultInput) { i.CompletedAt = i.CommitAt.Add(time.Nanosecond) }},
		{"timeout is not an HTTP response", func(i *HookResultInput) { i.Outcome = HookOutcome{Code: HookTimeoutUncertain} }},
		{"timeout cannot smuggle a late response", func(i *HookResultInput) {
			i.Outcome = HookOutcome{Code: HookTimeoutUncertain}
			i.CompletedAt = hookStateTime().Add(180 * time.Second)
			i.CommitAt = hookStateTime().Add(181 * time.Second)
		}},
		{"recovery before attempt deadline", func(i *HookResultInput) {
			i.Outcome = HookOutcome{Code: HookTimeoutUncertain}
			i.CompletedAt = time.Time{}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := runningHookState()
			state.Before.LeaseUntil = hookStateTime().Add(185 * time.Second)
			input := beforeResult()
			tc.change(&input)
			original := state
			got, err := CompleteHookOperation(&state, input)
			require.Error(t, err)
			require.False(t, got.Changed)
			require.Empty(t, got.Effects)
			require.Equal(t, original, got.State)
			require.Equal(t, original, state)
		})
	}
}

func TestHookDelayedCommitUsesCurrentRetryBudget(t *testing.T) {
	for _, tc := range []struct {
		name       string
		deadline   time.Duration
		retryAfter string
		wantRetry  bool
		wantDelay  time.Duration
	}{
		{"full attempt still fits", 366 * time.Second, "", true, 5 * time.Second},
		{"DB delay exhausted budget", 366*time.Second - time.Nanosecond, "", false, 0},
		{"Retry-After uses commit clock", 420 * time.Second, "Fri, 02 Jan 2026 03:07:16 GMT", true, 10 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := runningHookState()
			state.Before.LeaseUntil = hookStateTime().Add(185 * time.Second)
			state.Before.Deadline = hookStateTime().Add(tc.deadline)
			input := beforeResult()
			input.CompletedAt = hookStateTime().Add(179 * time.Second)
			input.CommitAt = hookStateTime().Add(181 * time.Second)
			input.Outcome = HookOutcome{Code: HookHTTPError, HTTPStatus: 503}
			input.Config = &HookConfig{OnFailure: gptr.Of(HookFailurePolicyContinue)}
			input.RetryAfter = tc.retryAfter
			original := state
			got, err := CompleteHookOperation(&state, input)
			require.NoError(t, err)
			require.True(t, got.Changed)
			require.Equal(t, tc.wantRetry, got.Effects.Retry.Retry)
			require.Equal(t, tc.wantDelay, got.Effects.Retry.Delay)
			if tc.wantRetry {
				require.Equal(t, HookOperationRetryWait, got.State.Before.Status)
				require.Equal(t, HookGateWaiting, got.State.Gate)
				require.Equal(t, int32(2), got.Effects.Retry.NextAttempt)
			} else {
				require.Equal(t, HookOperationFailed, got.State.Before.Status)
				require.Equal(t, HookGateReady, got.State.Gate)
				require.Empty(t, got.Effects)
			}
			require.Equal(t, original, state)
			require.Equal(t, original.After, got.State.After)
			require.Equal(t, original.Before.Deadline, got.State.Before.Deadline)
		})
	}
}

func TestHookDBWriteRetryKeepsCapturedCompletionTime(t *testing.T) {
	state := runningHookState()
	state.Before.LeaseUntil = hookStateTime().Add(185 * time.Second)
	input := beforeResult()
	input.CompletedAt = hookStateTime().Add(179 * time.Second)
	input.CommitAt = hookStateTime().Add(181 * time.Second)
	original := state
	first, err := CompleteHookOperation(&state, input)
	require.NoError(t, err)
	require.True(t, first.Changed)
	// Discard the plan to model a failed write; no database transaction is tested here.
	input.CommitAt = hookStateTime().Add(184 * time.Second)
	retried, err := CompleteHookOperation(&state, input)
	require.NoError(t, err)
	require.Equal(t, first, retried)
	require.Equal(t, original, state)
	input.CommitAt = hookStateTime().Add(185 * time.Second)
	expired, err := CompleteHookOperation(&state, input)
	require.NoError(t, err)
	require.False(t, expired.Changed)
	require.Equal(t, HookStateEffects{LateIgnored: true}, expired.Effects)
	require.Equal(t, original, expired.State)
}

func TestHookTimeoutRecoveryUsesCurrentFenceAndBudget(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		commitAt               time.Duration
		generation             int64
		deadline               time.Duration
		wantChanged, wantRetry bool
	}{
		{"at deadline", 180 * time.Second, 7, 420 * time.Second, true, true},
		{"after deadline", 181 * time.Second, 7, 420 * time.Second, true, true},
		{"budget exhausted", 181 * time.Second, 7, 365 * time.Second, true, false},
		{"lease expired", 185 * time.Second, 7, 420 * time.Second, false, false},
		{"stale recovery generation", 181 * time.Second, 8, 420 * time.Second, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := runningHookState()
			state.Before.LeaseUntil = hookStateTime().Add(185 * time.Second)
			state.Before.Generation = tc.generation
			state.Before.Deadline = hookStateTime().Add(tc.deadline)
			input := beforeResult()
			input.CompletedAt = time.Time{}
			input.CommitAt = hookStateTime().Add(tc.commitAt)
			input.Outcome = HookOutcome{Code: HookTimeoutUncertain}
			original := state
			got, err := CompleteHookOperation(&state, input)
			require.NoError(t, err)
			require.Equal(t, tc.wantChanged, got.Changed)
			require.Equal(t, tc.wantRetry, got.Effects.Retry.Retry)
			if !tc.wantChanged {
				require.Equal(t, original, got.State)
				require.Equal(t, HookStateEffects{LateIgnored: true}, got.Effects)
			} else if tc.wantRetry {
				require.Equal(t, HookOperationRetryWait, got.State.Before.Status)
				require.Equal(t, HookGateWaiting, got.State.Gate)
				require.Equal(t, HookRetryDecision{Retry: true, Delay: 5 * time.Second, NextAttempt: 2}, got.Effects.Retry)
			} else {
				require.Equal(t, HookOperationFailed, got.State.Before.Status)
				require.Equal(t, HookGateClosed, got.State.Gate)
				require.True(t, got.Effects.BeginFinalize)
			}
			require.Equal(t, original, state)
			require.Equal(t, original.After, got.State.After)
		})
	}
}

func TestHookAdmissionRules(t *testing.T) {
	state := runningHookState()
	base := HookAdmissionInput{Requested: state.Key, Actual: state.Key, LatestRunID: 3, Status: ExptStatus_Processing, Marker: HookMarkerManaged, State: &state}
	for _, tc := range []struct {
		name   string
		change func(*HookAdmissionInput, *HookRunState)
		want   HookGateState
	}{
		{"waiting", func(*HookAdmissionInput, *HookRunState) {}, HookGateWaiting},
		{"ready", func(_ *HookAdmissionInput, s *HookRunState) {
			s.Gate = HookGateReady
			s.Before.Status = HookOperationSucceeded
		}, HookGateReady},
		{"after only", func(_ *HookAdmissionInput, s *HookRunState) {
			s.Gate = HookGateReady
			s.Before = HookOperation{Status: HookOperationDisabled}
		}, HookGateReady},
		{"closed", func(_ *HookAdmissionInput, s *HookRunState) { s.Gate = HookGateClosed }, HookGateClosed},
		{"old run", func(i *HookAdmissionInput, _ *HookRunState) { i.LatestRunID = 4 }, HookGateClosed},
		{"wrong owner", func(i *HookAdmissionInput, _ *HookRunState) { i.Actual.WorkspaceID = 9 }, HookGateClosed},
		{"missing state", func(i *HookAdmissionInput, _ *HookRunState) { i.State = nil }, HookGateWaiting},
		{"read failure", func(i *HookAdmissionInput, _ *HookRunState) { i.StateReadFailed = true }, HookGateWaiting},
		{"unknown marker", func(i *HookAdmissionInput, _ *HookRunState) { i.Marker = "" }, HookGateWaiting},
		{"unknown gate", func(_ *HookAdmissionInput, s *HookRunState) { s.Gate = "unknown" }, HookGateWaiting},
		{"wrong state owner", func(_ *HookAdmissionInput, s *HookRunState) { s.Key.RunID = 8 }, HookGateWaiting},
		{"unknown operation", func(_ *HookAdmissionInput, s *HookRunState) { s.Before.Status = "unknown" }, HookGateWaiting},
		{"premature ready", func(_ *HookAdmissionInput, s *HookRunState) { s.Gate = HookGateReady }, HookGateWaiting},
		{"unknown run status", func(i *HookAdmissionInput, _ *HookRunState) { i.Status = 0 }, HookGateWaiting},
		{"legacy independent", func(i *HookAdmissionInput, _ *HookRunState) {
			i.Marker = HookMarkerLegacy
			i.State = nil
			i.StateReadFailed = true
		}, HookGateReady},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := state
			i := base
			i.State = &s
			tc.change(&i, &s)
			before := s
			got := CheckHookAdmission(i)
			require.Equal(t, tc.want, got.Gate)
			require.Equal(t, before, s)
		})
	}
	for _, status := range []ExptStatus{11, 12, 13, 14} {
		i := base
		i.Status = status
		require.Equal(t, HookGateClosed, CheckHookAdmission(i).Gate)
	}
	require.Equal(t, HookGateWaiting, CheckHookAdmission(HookAdmissionInput{}).Gate)
}

func TestHookBeforeResultsWaitForFinalOutcome(t *testing.T) {
	for _, tc := range []struct {
		name            string
		outcome         HookOutcome
		config          *HookConfig
		attempt         int32
		wantState       HookOperationStatus
		wantGate        HookGateState
		retry, finalize bool
	}{
		{"success", HookOutcome{Code: HookSucceeded}, nil, 1, HookOperationSucceeded, HookGateReady, false, false},
		{"continue still retries", HookOutcome{Code: HookHTTPError, HTTPStatus: 503, Retryable: true}, &HookConfig{OnFailure: gptr.Of(HookFailurePolicyContinue)}, 1, HookOperationRetryWait, HookGateWaiting, true, false},
		{"continue exhausted", HookOutcome{Code: HookHTTPError, HTTPStatus: 503, Retryable: true}, &HookConfig{OnFailure: gptr.Of(HookFailurePolicyContinue)}, 2, HookOperationFailed, HookGateReady, false, false},
		{"continue final failure", HookOutcome{Code: HookFailed}, &HookConfig{OnFailure: gptr.Of(HookFailurePolicyContinue)}, 1, HookOperationFailed, HookGateReady, false, false},
		{"block exhausted", HookOutcome{Code: HookHTTPError, HTTPStatus: 503, Retryable: true}, nil, 2, HookOperationFailed, HookGateClosed, false, true},
		{"security final", HookOutcome{Code: HookSecurityError, Retryable: true}, nil, 1, HookOperationFailed, HookGateClosed, false, true},
		{"HTTP eligibility wins", HookOutcome{Code: HookHTTPError, HTTPStatus: 503, Retryable: false}, nil, 1, HookOperationRetryWait, HookGateWaiting, true, false},
		{"HTTP denial wins", HookOutcome{Code: HookHTTPError, HTTPStatus: 403, Retryable: true}, nil, 1, HookOperationFailed, HookGateClosed, false, true},
		{"disabled retry", HookOutcome{Code: HookTransportError, Retryable: true}, &HookConfig{Retry: &HookRetryConf{Enabled: gptr.Of(false)}}, 1, HookOperationFailed, HookGateClosed, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := runningHookState()
			state.Before.Attempt = tc.attempt
			original := state
			input := beforeResult()
			input.Token.Attempt = tc.attempt
			input.Outcome = tc.outcome
			input.Config = tc.config
			got, err := CompleteHookOperation(&state, input)
			require.NoError(t, err)
			require.True(t, got.Changed)
			require.Equal(t, tc.wantState, got.State.Before.Status)
			require.Equal(t, tc.wantGate, got.State.Gate)
			require.Equal(t, tc.retry, got.Effects.Retry.Retry)
			require.Equal(t, tc.finalize, got.Effects.BeginFinalize)
			require.Equal(t, original, state)
			require.Equal(t, original.After, got.State.After)
			require.Equal(t, original.Before.Attempt, got.State.Before.Attempt)
			require.Equal(t, original.Before.Deadline, got.State.Before.Deadline)
			if tc.retry {
				require.Equal(t, int32(2), got.Effects.Retry.NextAttempt)
				require.Equal(t, 5*time.Second, got.Effects.Retry.Delay)
			}
			if tc.finalize {
				require.Equal(t, HookTerminalIntent{Status: 13, Reason: "HOOK_BEFORE_FAILED"}, got.State.Intent)
				require.Equal(t, HookFinalizePending, got.State.Finalize)
				require.False(t, got.State.After.Activated)
			}
		})
	}
	state := runningHookState()
	state.Before.Deadline = hookStateTime().Add(180 * time.Second)
	input := beforeResult()
	input.Outcome = HookOutcome{Code: HookTransportError, Retryable: true}
	input.Config = &HookConfig{OnFailure: gptr.Of(HookFailurePolicyContinue)}
	got, err := CompleteHookOperation(&state, input)
	require.NoError(t, err)
	require.Equal(t, HookOperationFailed, got.State.Before.Status)
	require.False(t, got.Effects.Retry.Retry)
}

func TestHookStaleTokensAndLateResponsesAreNoOps(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*HookRunState, *HookResultInput)
	}{
		{"wrong run", func(_ *HookRunState, i *HookResultInput) { i.Token.Run.RunID++ }},
		{"wrong workspace", func(_ *HookRunState, i *HookResultInput) { i.Token.Run.WorkspaceID++ }},
		{"wrong experiment", func(_ *HookRunState, i *HookResultInput) { i.Token.Run.ExperimentID++ }},
		{"wrong operation", func(_ *HookRunState, i *HookResultInput) { i.Token.OperationID = "other" }},
		{"wrong phase", func(_ *HookRunState, i *HookResultInput) { i.Token.Phase = HookPhaseAfter }},
		{"unknown phase", func(_ *HookRunState, i *HookResultInput) { i.Token.Phase = "unknown" }},
		{"old attempt", func(_ *HookRunState, i *HookResultInput) { i.Token.Attempt = 0 }},
		{"old generation", func(_ *HookRunState, i *HookResultInput) { i.Token.Generation = 6 }},
		{"expired lease", func(s *HookRunState, i *HookResultInput) { s.Before.LeaseUntil = i.CommitAt }},
		{"late success", func(s *HookRunState, i *HookResultInput) { s.Before.AttemptDeadline = i.CompletedAt }},
		{"closed gate", func(s *HookRunState, _ *HookResultInput) { s.Gate = HookGateClosed }},
		{"duplicate success", func(s *HookRunState, _ *HookResultInput) {
			s.Before.Status = HookOperationSucceeded
			s.Gate = HookGateReady
		}},
		{"retry wait", func(s *HookRunState, _ *HookResultInput) { s.Before.Status = HookOperationRetryWait }},
		{"final failure", func(s *HookRunState, _ *HookResultInput) {
			s.Before.Status = HookOperationFailed
			s.Gate = HookGateClosed
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := runningHookState()
			input := beforeResult()
			tc.change(&state, &input)
			original := state
			got, err := CompleteHookOperation(&state, input)
			require.NoError(t, err)
			require.False(t, got.Changed)
			require.Equal(t, HookStateEffects{LateIgnored: true}, got.Effects)
			require.Equal(t, original, got.State)
			require.Equal(t, original, state)
		})
	}
	state := runningHookState()
	input := beforeResult()
	state.Before.AttemptDeadline = input.CommitAt
	input.CompletedAt = time.Time{}
	input.Outcome = HookOutcome{Code: HookTimeoutUncertain, Retryable: true}
	got, err := CompleteHookOperation(&state, input)
	require.NoError(t, err)
	require.Equal(t, HookOperationRetryWait, got.State.Before.Status)
}

func TestHookFinalizeTwoPhasesAndCancellation(t *testing.T) {
	for _, status := range []ExptStatus{11, 12, 13, 14} {
		for _, beforeStatus := range []HookOperationStatus{HookOperationPending, HookOperationRunning, HookOperationRetryWait} {
			state := runningHookState()
			state.Before.Status = beforeStatus
			original := state
			intent := HookTerminalIntent{Status: status}
			begun, err := BeginHookFinalize(&state, state.Key, intent)
			require.NoError(t, err)
			require.True(t, begun.Changed)
			require.True(t, begun.Effects.BeginFinalize)
			require.True(t, begun.Effects.FenceBefore)
			require.Equal(t, HookGateClosed, begun.State.Gate)
			require.Equal(t, ExptStatus(3), begun.State.Status)
			require.Equal(t, HookOperationFailed, begun.State.Before.Status)
			require.False(t, begun.State.After.Activated)
			require.Equal(t, original, state)
			require.Equal(t, original.Before.Attempt, begun.State.Before.Attempt)
			late, err := CompleteHookOperation(&begun.State, beforeResult())
			require.NoError(t, err)
			require.True(t, late.Effects.LateIgnored)
			require.Equal(t, begun.State, late.State)
			repeat, err := BeginHookFinalize(&begun.State, state.Key, intent)
			require.NoError(t, err)
			require.False(t, repeat.Changed)
			require.Equal(t, begun.State, repeat.State)
			committed, err := CommitHookFinalize(&begun.State, state.Key, intent)
			require.NoError(t, err)
			require.Equal(t, status, committed.State.Status)
			require.Equal(t, HookFinalizeCommitted, committed.State.Finalize)
			require.True(t, committed.Effects.ActivateAfter)
			require.True(t, committed.State.After.Activated)
			require.Equal(t, HookOperationPending, committed.State.After.Status)
			require.Equal(t, int32(0), committed.State.After.Attempt)
			repeat, err = CommitHookFinalize(&committed.State, state.Key, intent)
			require.NoError(t, err)
			require.False(t, repeat.Changed)
			require.Empty(t, repeat.Effects)
			require.Equal(t, committed.State, repeat.State)
		}
	}
	state := runningHookState()
	for _, status := range []ExptStatus{0, 15, 21, 99} {
		got, err := BeginHookFinalize(&state, state.Key, HookTerminalIntent{Status: status})
		require.Error(t, err)
		require.Equal(t, state, got.State)
	}
	_, err := CommitHookFinalize(&state, state.Key, HookTerminalIntent{Status: 13})
	require.Error(t, err)
	intent := HookTerminalIntent{Status: 13, Reason: "cancel"}
	begun, err := BeginHookFinalize(&state, state.Key, intent)
	require.NoError(t, err)
	for _, conflict := range []HookTerminalIntent{{Status: 11}, {Status: 13, Reason: "other"}} {
		got, err := BeginHookFinalize(&begun.State, state.Key, conflict)
		require.Error(t, err)
		require.Equal(t, begun.State, got.State)
		got, err = CommitHookFinalize(&begun.State, state.Key, conflict)
		require.Error(t, err)
		require.Equal(t, begun.State, got.State)
	}
}

func TestHookOldRunAfterOnlyChangesItsOperation(t *testing.T) {
	for _, tc := range []struct {
		outcome HookOutcome
		status  HookOperationStatus
		retry   bool
	}{
		{HookOutcome{Code: HookSucceeded}, HookOperationSucceeded, false},
		{HookOutcome{Code: HookFailed}, HookOperationFailed, false},
		{HookOutcome{Code: HookHTTPError, HTTPStatus: 503, Retryable: true}, HookOperationRetryWait, true},
	} {
		state := runningHookState()
		state.Gate = HookGateClosed
		state.Status = ExptStatus_Success
		state.Finalize = HookFinalizeCommitted
		state.Intent = HookTerminalIntent{Status: 11}
		state.Before.Status = HookOperationSucceeded
		state.After = state.Before
		state.After.ID = "after"
		state.After.Status = HookOperationRunning
		state.After.LeaseUntil = hookStateTime().Add(185 * time.Second)
		original := state
		newRun := runningHookState()
		newRun.Key.RunID = 4
		unchangedNew := newRun
		input := beforeResult()
		input.Token.Phase = HookPhaseAfter
		input.Token.OperationID = "after"
		input.Outcome = tc.outcome
		input.CompletedAt = hookStateTime().Add(179 * time.Second)
		input.CommitAt = hookStateTime().Add(181 * time.Second)
		got, err := CompleteHookOperation(&state, input)
		require.NoError(t, err)
		require.True(t, got.Changed)
		require.Equal(t, original, state)
		require.Equal(t, unchangedNew, newRun)
		require.Equal(t, tc.status, got.State.After.Status)
		require.Equal(t, tc.retry, got.Effects.Retry.Retry)
		wrongRun, err := CompleteHookOperation(&newRun, input)
		require.NoError(t, err)
		require.True(t, wrongRun.Effects.LateIgnored)
		require.Equal(t, unchangedNew, wrongRun.State)
		expected := original
		expected.After = got.State.After
		require.Equal(t, expected, got.State)
		require.False(t, got.Effects.BeginFinalize)
		require.False(t, got.Effects.ActivateAfter)
		require.Equal(t, original.After.Attempt, got.State.After.Attempt)
		require.Equal(t, original.After.Generation, got.State.After.Generation)
		repeated, err := CommitHookFinalize(&got.State, state.Key, state.Intent)
		require.NoError(t, err)
		require.Equal(t, got.State, repeated.State)
		require.Empty(t, repeated.Effects)
	}
}

func TestHookStateErrorsDoNotMutateInput(t *testing.T) {
	_, err := CompleteHookOperation(nil, beforeResult())
	require.Error(t, err)
	_, err = BeginHookFinalize(nil, HookRunKey{}, HookTerminalIntent{Status: 13})
	require.Error(t, err)
	_, err = CommitHookFinalize(nil, HookRunKey{}, HookTerminalIntent{Status: 13})
	require.Error(t, err)
	for _, change := range []func(*HookRunState){
		func(s *HookRunState) { s.Gate = "unknown" }, func(s *HookRunState) { s.Before.Status = "unknown" },
		func(s *HookRunState) { s.Finalize = "unknown" }, func(s *HookRunState) { s.Before.Attempt = 0 },
		func(s *HookRunState) { s.Key.RunID = 0 }, func(s *HookRunState) { s.Before.Generation = 0 },
	} {
		s := runningHookState()
		change(&s)
		original := s
		got, err := CompleteHookOperation(&s, beforeResult())
		require.Error(t, err)
		require.Equal(t, original, s)
		require.Equal(t, original, got.State)
	}
	s := runningHookState()
	input := beforeResult()
	input.Outcome = HookOutcome{Code: HookFailed}
	input.Config = &HookConfig{OnFailure: gptr.Of(HookFailurePolicy("invalid"))}
	got, err := CompleteHookOperation(&s, input)
	require.Error(t, err)
	require.Equal(t, s, got.State)
}

func TestHookCommitRequiresConsistentActivation(t *testing.T) {
	state := runningHookState()
	state.Finalize = HookFinalizeCommitted
	state.Intent = HookTerminalIntent{Status: 11}
	state.Status = 11
	state.Gate = HookGateClosed
	state.Before.Status = HookOperationSucceeded
	for _, status := range []HookOperationStatus{HookOperationPending, HookOperationRetryWait} {
		state.After.Status = status
		state.After.Activated = false
		original := state
		got, err := CommitHookFinalize(&state, state.Key, state.Intent)
		require.Error(t, err)
		require.Equal(t, original, got.State)
		require.Equal(t, original, state)
	}
}

func TestHookFinalizeDisabledAfterAndWrongRun(t *testing.T) {
	state := runningHookState()
	state.Before.Status = HookOperationSucceeded
	state.After = HookOperation{Status: HookOperationDisabled}
	intent := HookTerminalIntent{Status: 13, Reason: "cancel"}
	wrong := state.Key
	wrong.RunID = 4
	got, err := BeginHookFinalize(&state, wrong, intent)
	require.Error(t, err)
	require.Equal(t, state, got.State)
	begun, err := BeginHookFinalize(&state, state.Key, intent)
	require.NoError(t, err)
	require.False(t, begun.Effects.FenceBefore)
	require.Equal(t, state.Before, begun.State.Before)
	got, err = CommitHookFinalize(&begun.State, wrong, intent)
	require.Error(t, err)
	require.Equal(t, begun.State, got.State)
	committed, err := CommitHookFinalize(&begun.State, state.Key, intent)
	require.NoError(t, err)
	require.False(t, committed.Effects.ActivateAfter)
	require.Equal(t, HookOperationDisabled, committed.State.After.Status)
	repeated, err := BeginHookFinalize(&committed.State, state.Key, intent)
	require.NoError(t, err)
	require.Equal(t, committed.State, repeated.State)
	require.Empty(t, repeated.Effects)
	committed.State.Key.RunID = 99
	require.Equal(t, int64(3), state.Key.RunID)
}
