// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"time"
)

type HookRunKey struct{ WorkspaceID, ExperimentID, RunID int64 }
type HookPhase string
type HookGateState string
type HookOperationStatus string
type HookFinalizeState string

// A read failure must remain unknown, never be converted to legacy.
type HookRunMarker string

const (
	HookPhaseBefore        HookPhase           = "before"
	HookPhaseAfter         HookPhase           = "after"
	HookGateReady          HookGateState       = "ready"
	HookGateWaiting        HookGateState       = "waiting"
	HookGateClosed         HookGateState       = "closed"
	HookOperationDisabled  HookOperationStatus = "disabled"
	HookOperationPending   HookOperationStatus = "pending"
	HookOperationRunning   HookOperationStatus = "running"
	HookOperationRetryWait HookOperationStatus = "retry_wait"
	HookOperationSucceeded HookOperationStatus = "succeeded"
	HookOperationFailed    HookOperationStatus = "failed"
	HookFinalizeNone       HookFinalizeState   = "none"
	HookFinalizePending    HookFinalizeState   = "pending"
	HookFinalizeCommitted  HookFinalizeState   = "committed"
	HookMarkerLegacy       HookRunMarker       = "legacy"
	HookMarkerManaged      HookRunMarker       = "managed"
)

type HookOperation struct {
	ID                                    string
	Status                                HookOperationStatus
	Activated                             bool
	Attempt                               int32
	Generation                            int64
	LeaseUntil, AttemptDeadline, Deadline time.Time
}

type HookTerminalIntent struct {
	Status ExptStatus
	Reason string
}

type HookRunState struct {
	Key           HookRunKey
	Status        ExptStatus
	Gate          HookGateState
	Before, After HookOperation
	Finalize      HookFinalizeState
	Intent        HookTerminalIntent
}

type HookClaimToken struct {
	Run         HookRunKey
	OperationID string
	Phase       HookPhase
	Attempt     int32
	Generation  int64
}

type HookResultInput struct {
	Token       HookClaimToken
	Outcome     HookOutcome
	Config      *HookConfig // Frozen policy; operation state determines activation.
	CompletedAt time.Time   // HTTP attempt completion (full body or local failure); absent only for internal timeout recovery.
	CommitAt    time.Time   // Current DB clock for lease validation and retry scheduling; CompletedAt must use the same clock basis.
	RetryAfter  string
}

type HookStateEffects struct {
	LateIgnored, BeginFinalize, FenceBefore, ActivateAfter bool
	Retry                                                  HookRetryDecision
}

type HookStateChange struct {
	State   HookRunState
	Changed bool
	Effects HookStateEffects
}

type HookAdmissionInput struct {
	Requested, Actual HookRunKey
	LatestRunID       int64
	Status            ExptStatus
	Marker            HookRunMarker
	StateReadFailed   bool
	State             *HookRunState
}

type HookAdmissionDecision struct {
	Gate   HookGateState
	Reason string
}

// CheckHookAdmission is a read-only rule, not the final atomic AdmitItem.
func CheckHookAdmission(input HookAdmissionInput) HookAdmissionDecision {
	wait := HookAdmissionDecision{Gate: HookGateWaiting, Reason: "HOOK_STATE_UNAVAILABLE"}
	if !validHookRunKey(input.Requested) || !validHookRunKey(input.Actual) || input.LatestRunID <= 0 || !knownHookRunStatus(input.Status) {
		return wait
	}
	if input.Requested != input.Actual || input.Actual.RunID != input.LatestRunID {
		return HookAdmissionDecision{Gate: HookGateClosed, Reason: "RUN_OWNERSHIP_MISMATCH"}
	}
	if IsExptFinished(input.Status) || input.Status == ExptStatus_Terminating {
		return HookAdmissionDecision{Gate: HookGateClosed, Reason: "RUN_CLOSED"}
	}
	if input.Marker == HookMarkerLegacy {
		return HookAdmissionDecision{Gate: HookGateReady}
	}
	if input.Marker != HookMarkerManaged || input.StateReadFailed || input.State == nil {
		return wait
	}
	if input.State.Key != input.Actual || input.State.Status != input.Status || validateHookState(input.State) != nil {
		return wait
	}
	return HookAdmissionDecision{Gate: input.State.Gate}
}

// Effects are plans only; callers must atomically persist against the same
// claim/fence before acting. This function never claims or renews a lease.
func CompleteHookOperation(state *HookRunState, input HookResultInput) (HookStateChange, error) {
	change, err := hookStateChange(state)
	if err != nil {
		return change, err
	}
	late := change
	late.Effects.LateIgnored = true
	if input.Token.Run != state.Key {
		return late, nil
	}
	var operation *HookOperation
	switch input.Token.Phase {
	case HookPhaseBefore:
		operation = &change.State.Before
	case HookPhaseAfter:
		operation = &change.State.After
	default:
		return late, nil
	}
	if operation.Status != HookOperationRunning || !operation.Activated || input.Token.OperationID != operation.ID || input.Token.Attempt != operation.Attempt || input.Token.Generation != operation.Generation {
		return late, nil
	}
	if input.CommitAt.IsZero() {
		return change, invalidParam("hook result requires an explicit current time")
	}
	if input.Outcome.Code == HookTimeoutUncertain {
		// Internal deadline recovery has no completed HTTP response to accept.
		if !input.CompletedAt.IsZero() || input.CommitAt.Before(operation.AttemptDeadline) {
			return change, invalidParam("hook timeout recovery requires an elapsed attempt deadline and no completion time")
		}
	} else if input.CompletedAt.IsZero() || input.CompletedAt.After(input.CommitAt) {
		return change, invalidParam("hook result requires a completion time no later than commit time")
	}
	if !input.CommitAt.Before(operation.LeaseUntil) || (input.Outcome.Code != HookTimeoutUncertain && !input.CompletedAt.Before(operation.AttemptDeadline)) {
		return late, nil
	}
	if input.Token.Phase == HookPhaseBefore && (state.Gate == HookGateClosed || state.Finalize != HookFinalizeNone || IsExptFinished(state.Status) || state.Status == ExptStatus_Terminating) {
		return late, nil
	}
	outcome, err := hookStateOutcome(input.Outcome)
	if err != nil {
		return change, err
	}
	config := HookConfig{}
	if input.Config != nil {
		config = *input.Config
	}
	policy := HookFailurePolicyBlock
	if input.Token.Phase == HookPhaseBefore && config.OnFailure != nil {
		policy = *config.OnFailure
		if policy != HookFailurePolicyBlock && policy != HookFailurePolicyContinue {
			return change, invalidParam("invalid before failure policy")
		}
	}
	timeout := int32(0)
	if config.TimeoutSeconds != nil {
		timeout = *config.TimeoutSeconds
	}
	retry, err := DecideHookRetry(HookRetryInput{Outcome: outcome, Retry: config.Retry, Attempt: operation.Attempt, TimeoutSeconds: timeout, Now: input.CommitAt, Deadline: operation.Deadline, RetryAfter: input.RetryAfter})
	if err != nil {
		return change, err
	}
	change.Changed = true
	if outcome.Code == HookSucceeded {
		operation.Status = HookOperationSucceeded
		operation.Activated = false
	} else if retry.Retry {
		operation.Status = HookOperationRetryWait
		change.Effects.Retry = retry
	} else {
		operation.Status = HookOperationFailed
		operation.Activated = false
	}
	if input.Token.Phase == HookPhaseAfter {
		return change, nil
	}
	if retry.Retry {
		change.State.Gate = HookGateWaiting
		return change, nil
	}
	if operation.Status == HookOperationSucceeded || policy == HookFailurePolicyContinue {
		change.State.Gate = HookGateReady
		return change, nil
	}
	return BeginHookFinalize(&change.State, state.Key, HookTerminalIntent{Status: ExptStatus_Terminated, Reason: "HOOK_BEFORE_FAILED"})
}

func BeginHookFinalize(state *HookRunState, key HookRunKey, intent HookTerminalIntent) (HookStateChange, error) {
	change, err := hookStateChange(state)
	if err != nil {
		return change, err
	}
	if key != state.Key || !IsExptFinished(intent.Status) {
		return change, invalidParam("invalid hook finalization target")
	}
	if state.Finalize != HookFinalizeNone {
		if state.Intent != intent {
			return change, invalidParam("hook terminal intent is immutable")
		}
		return change, nil
	}
	if IsExptFinished(state.Status) && state.Status != intent.Status {
		return change, invalidParam("hook run terminal status is immutable")
	}
	change.State.Intent = intent
	change.State.Finalize = HookFinalizePending
	change.State.Gate = HookGateClosed
	if unfinishedHookOperation(state.Before.Status) {
		change.State.Before.Status = HookOperationFailed
		change.State.Before.Activated = false
		change.Effects.FenceBefore = true
	}
	change.Changed = true
	change.Effects.BeginFinalize = true
	return change, nil
}

// Commit confirms the captured intent after external finalization work. It
// does not perform that work, publish after, or project onto a newer Run.
func CommitHookFinalize(state *HookRunState, key HookRunKey, intent HookTerminalIntent) (HookStateChange, error) {
	change, err := hookStateChange(state)
	if err != nil {
		return change, err
	}
	if key != state.Key || !IsExptFinished(intent.Status) || state.Finalize == HookFinalizeNone || state.Intent != intent {
		return change, invalidParam("hook finalization intent mismatch")
	}
	if state.Finalize == HookFinalizeCommitted {
		return change, nil
	}
	change.State.Status = intent.Status
	change.State.Finalize = HookFinalizeCommitted
	change.State.Gate = HookGateClosed
	if state.After.Status == HookOperationPending && !state.After.Activated {
		change.State.After.Activated = true
		change.Effects.ActivateAfter = true
	}
	change.Changed = true
	return change, nil
}

func hookStateChange(state *HookRunState) (HookStateChange, error) {
	change := HookStateChange{}
	if state != nil {
		change.State = *state
	}
	return change, validateHookState(state)
}

func validateHookState(state *HookRunState) error {
	invalid := invalidParam("invalid hook run state")
	if state == nil || !validHookRunKey(state.Key) || !knownHookRunStatus(state.Status) {
		return invalid
	}
	if state.Gate != HookGateReady && state.Gate != HookGateWaiting && state.Gate != HookGateClosed {
		return invalid
	}
	if !validHookOperation(state.Before) || !validHookOperation(state.After) {
		return invalid
	}
	if state.Before.Status != HookOperationDisabled && state.After.Status != HookOperationDisabled && state.Before.ID == state.After.ID {
		return invalid
	}
	if state.Gate == HookGateReady && unfinishedHookOperation(state.Before.Status) {
		return invalid
	}
	switch state.Finalize {
	case HookFinalizeNone:
		if state.Intent != (HookTerminalIntent{}) {
			return invalid
		}
	case HookFinalizePending, HookFinalizeCommitted:
		if !IsExptFinished(state.Intent.Status) || state.Gate != HookGateClosed || unfinishedHookOperation(state.Before.Status) {
			return invalid
		}
		if state.Finalize == HookFinalizeCommitted && state.Status != state.Intent.Status {
			return invalid
		}
	default:
		return invalid
	}
	if state.Finalize != HookFinalizeCommitted && (state.After.Activated || (state.After.Status != HookOperationDisabled && state.After.Status != HookOperationPending)) {
		return invalid
	}
	if state.Finalize == HookFinalizeCommitted && unfinishedHookOperation(state.After.Status) && !state.After.Activated {
		return invalid
	}
	return nil
}

func validHookOperation(operation HookOperation) bool {
	switch operation.Status {
	case HookOperationDisabled:
		return !operation.Activated
	case HookOperationPending, HookOperationRunning, HookOperationRetryWait, HookOperationSucceeded, HookOperationFailed:
	default:
		return false
	}
	if strings.TrimSpace(operation.ID) == "" || operation.Attempt < 0 || operation.Attempt > 11 || operation.Generation < 0 {
		return false
	}
	if operation.Status == HookOperationRunning {
		return operation.Activated && operation.Attempt > 0 && operation.Generation > 0 && !operation.LeaseUntil.IsZero() && !operation.AttemptDeadline.IsZero() && !operation.Deadline.IsZero() && !operation.AttemptDeadline.After(operation.Deadline)
	}
	return true
}

func validHookRunKey(key HookRunKey) bool {
	return key.WorkspaceID > 0 && key.ExperimentID > 0 && key.RunID > 0
}
func knownHookRunStatus(status ExptStatus) bool {
	return status == ExptStatus_Pending || status == ExptStatus_Processing || status == ExptStatus_Terminating || status == ExptStatus_Draining || IsExptFinished(status)
}
func unfinishedHookOperation(status HookOperationStatus) bool {
	return status == HookOperationPending || status == HookOperationRunning || status == HookOperationRetryWait
}

func hookStateOutcome(outcome HookOutcome) (HookOutcome, error) {
	switch outcome.Code {
	case HookSucceeded, HookSecurityError, HookHTTPRedirectError, HookResultNotFinal, HookProtocolError, HookResponseTooLarge:
		outcome.Retryable = false
	case HookTransportError, HookTimeoutUncertain:
		outcome.Retryable = true
	case HookHTTPError:
		classified := ClassifyHookOutcome("", outcome.HTTPStatus, "", false)
		if classified.Code != HookHTTPError {
			return outcome, invalidParam("invalid hook HTTP outcome")
		}
		outcome = classified
	case HookFailed:
	default:
		return outcome, invalidParam("invalid hook outcome")
	}
	return outcome, nil
}
