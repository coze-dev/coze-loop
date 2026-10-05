// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

var ErrHookExecutionUnavailable = errors.New("hook execution dependency unavailable")

// HookAttemptExecutor executes at most one granted delivery. It owns no scanner,
// pool, manager or background lifetime beyond the Execute call.
type HookAttemptExecutor struct {
	repo          repo.IHookRepo
	reader        hookcomponent.AttemptReader
	codec         hookcomponent.StorageCodec
	builder       hookcomponent.RequestBuilder
	transport     hookcomponent.HTTPTransport
	projector     hookcomponent.CompletionProjector
	runtimeConfig hookcomponent.RuntimeConfigProvider
}

// The original constructor remains unchanged for callers using default timing.
func NewHookAttemptExecutorWithConfig(r repo.IHookRepo, codec hookcomponent.StorageCodec, builder hookcomponent.RequestBuilder, transport hookcomponent.HTTPTransport, projector hookcomponent.CompletionProjector, config hookcomponent.RuntimeConfigProvider) (*HookAttemptExecutor, error) {
	e, err := NewHookAttemptExecutor(r, codec, builder, transport, projector)
	if err != nil {
		return nil, err
	}
	if !hookExecutionNil(config) {
		e.runtimeConfig = config
	}
	return e, nil
}

func NewHookAttemptExecutor(r repo.IHookRepo, codec hookcomponent.StorageCodec, builder hookcomponent.RequestBuilder, transport hookcomponent.HTTPTransport, projector hookcomponent.CompletionProjector) (*HookAttemptExecutor, error) {
	for _, dep := range []any{r, codec, builder, transport, projector} {
		if hookExecutionNil(dep) {
			return nil, ErrHookExecutionUnavailable
		}
	}
	reader, ok := r.(hookcomponent.AttemptReader)
	if !ok {
		return nil, ErrHookExecutionUnavailable
	}
	return &HookAttemptExecutor{repo: r, reader: reader, codec: codec, builder: builder, transport: transport, projector: projector}, nil
}

func (e *HookAttemptExecutor) Execute(ctx context.Context, in hookcomponent.AttemptExecutionInput) (hookcomponent.AttemptExecutionResult, error) {
	var empty hookcomponent.AttemptExecutionResult
	if e == nil || ctx == nil {
		return empty, ErrHookExecutionUnavailable
	}
	if ctx.Err() != nil {
		return empty, ctx.Err()
	}
	run, err := e.repo.GetRun(ctx, in.Key)
	if err != nil {
		return empty, ErrHookExecutionUnavailable
	}
	scope, status, err := hookExecutionScope(run, in)
	if err != nil {
		return empty, err
	}
	if status == entity.HookOperationSucceeded || status == entity.HookOperationFailed {
		return hookcomponent.AttemptExecutionResult{HookAttemptStoreResult: entity.HookAttemptStoreResult{HookStoreResult: entity.HookStoreResult{Run: run}}}, nil
	}
	config, hash, err := e.codec.DecodePhase(ctx, in.Key, in.ExecutionScope, run.Snapshot, in.Phase)
	if err != nil || hash != scope.SnapshotHash {
		return empty, ErrHookExecutionUnavailable
	}
	if status == entity.HookOperationRunning {
		return e.recover(ctx, scope, config)
	}
	timing := entity.HookRuntimeConfig{LeaseSeconds: entity.HookDefaultLeaseSeconds, RenewSeconds: entity.HookDefaultRenewSeconds}
	if e.runtimeConfig != nil {
		timing, err = e.runtimeConfig.GetRuntimeConfig(ctx)
		if err != nil || ctx.Err() != nil {
			return empty, ErrHookExecutionUnavailable
		}
	}
	if entity.ValidateHookLeaseTiming(timing.LeaseSeconds, timing.RenewSeconds) != nil {
		return empty, ErrHookExecutionUnavailable
	}
	claimed, err := e.repo.ClaimAttempt(ctx, entity.HookClaimAttemptInput{HookAttemptScope: scope, Owner: in.Owner, AttemptID: in.AttemptID, Config: config, LeaseSeconds: &timing.LeaseSeconds})
	if errors.Is(err, entity.ErrHookStoreConflict) {
		return empty, nil
	}
	if err != nil {
		return empty, ErrHookExecutionUnavailable
	}
	if claimed.Claim == nil {
		return hookcomponent.AttemptExecutionResult{HookAttemptStoreResult: claimed}, nil
	}
	if claimed.Clock == nil {
		return empty, ErrHookExecutionUnavailable
	}
	scope.ExpectedVersion = claimed.Claim.Version
	lease, err := e.keepLease(ctx, scope, claimed, timing)
	if err != nil {
		return empty, err
	}
	defer lease.stop()
	recoverCurrent := func() (hookcomponent.AttemptExecutionResult, error) {
		lease.stop()
		scope.ExpectedVersion = lease.snapshot().Claim.Version
		return e.recover(ctx, scope, config)
	}
	sampledAt := time.Now()
	budget, err := hookExecutionRemaining(*claimed.Clock, claimed.Claim.AttemptDeadline, sampledAt)
	if err != nil || budget <= 0 {
		return recoverCurrent()
	}
	invokeCtx, cancelInvoke := context.WithDeadline(lease.ctx, sampledAt.Add(budget))
	defer cancelInvoke()
	request, _, buildErr := e.builder.BuildClaimedRequest(invokeCtx, in.ExecutionScope, claimed.Run, claimed.Claim)
	result := entity.HookTransportResult{Outcome: entity.HookOutcome{Code: entity.HookSecurityError}, LocalCompletedAt: time.Now()}
	if ctx.Err() != nil {
		return empty, ctx.Err()
	}
	if lease.ctx.Err() != nil {
		return empty, ErrHookExecutionLeaseLost
	}
	if invokeCtx.Err() != nil {
		return recoverCurrent()
	}
	if buildErr == nil {
		budget, err := hookExecutionRemaining(*claimed.Clock, claimed.Claim.AttemptDeadline, time.Now())
		if err != nil || budget <= 0 {
			return recoverCurrent()
		}
		result = e.transport.Invoke(invokeCtx, entity.HookTransportInput{WorkspaceID: in.Key.WorkspaceID, Config: config, Request: request, Remaining: budget})
	}
	if ctx.Err() != nil {
		return empty, ctx.Err()
	}
	if lease.ctx.Err() != nil {
		return empty, ErrHookExecutionLeaseLost
	}
	if result.Outcome.Code == entity.HookTimeoutUncertain {
		return recoverCurrent()
	}
	window, completionClock, err := e.observeCompletion(lease.ctx, scope, *claimed.Clock, result.LocalCompletedAt, claimed.Claim.AttemptDeadline)
	if err != nil {
		return recoverCurrent()
	}
	var redacted []byte
	var displayCode, displayMessage string
	if result.Outcome.Code == entity.HookSucceeded || result.Outcome.Code == entity.HookFailed {
		var projectionErr error
		redacted, displayCode, displayMessage, projectionErr = e.projectCompletion(lease.ctx, in, result)
		if projectionErr != nil {
			result.Outcome = entity.HookOutcome{Code: entity.HookSecurityError}
			redacted = nil
			displayCode, displayMessage = "", ""
			// This is a new local failure, not the earlier HTTP completion.
			window, completionClock, err = e.observeCompletion(lease.ctx, scope, completionClock, time.Now(), claimed.Claim.AttemptDeadline)
			if err != nil {
				return recoverCurrent()
			}
		}
	}
	if ctx.Err() != nil {
		return empty, ctx.Err()
	}
	if lease.ctx.Err() != nil {
		return empty, ErrHookExecutionLeaseLost
	}
	// The upper endpoint can initially be in the DB's near future. Wait for its
	// lower bound to pass it; do not replace HTTP completion with commit time.
	nowWindow, err := completionClock.Window(time.Now())
	if err != nil {
		return empty, ErrHookExecutionUnavailable
	}
	if delay := window.Latest.Sub(nowWindow.Earliest) + time.Millisecond; delay > 0 {
		if err := hookExecutionWait(lease.ctx, delay); err != nil {
			return empty, ErrHookExecutionLeaseLost
		}
	}
	scope.ExpectedVersion = lease.snapshot().Claim.Version
	completion := entity.HookCompleteAttemptInput{HookRenewAttemptInput: entity.HookRenewAttemptInput{HookAttemptScope: scope, HookAttemptIdentity: claimed.Claim.HookAttemptIdentity, LeaseSeconds: &timing.LeaseSeconds}, Config: config, Outcome: result.Outcome, CompletedAt: window.Latest, CompletionWindow: &window, RetryAfter: result.RetryAfter, ResultRedacted: redacted}
	// Persist the convention and bounds in the existing redacted audit payload;
	// finished_at must not be mistaken for an exact wall-clock measurement.
	completion.ErrorRedacted, err = json.Marshal(map[string]any{"completion_time": map[string]any{"convention": "conservative_upper_bound", "earliest": window.Earliest, "latest": window.Latest}})
	if err != nil {
		return empty, ErrHookExecutionUnavailable
	}
	if result.Outcome.Code != entity.HookSucceeded {
		completion.ErrorMessage = string(result.Outcome.Code)
		if displayCode != "" {
			completion.DisplayErrorCode, completion.ErrorMessage = displayCode, displayMessage
		}
	}
	finished, err := e.complete(lease.ctx, completion)
	if err != nil {
		return empty, ErrHookExecutionUnavailable
	}
	return hookcomponent.AttemptExecutionResult{HookAttemptStoreResult: finished, CompletionWindow: &window}, nil
}

func hookExecutionScope(run *entity.HookStoredRun, in hookcomponent.AttemptExecutionInput) (entity.HookAttemptScope, entity.HookOperationStatus, error) {
	var scope entity.HookAttemptScope
	if run == nil || run.State.Key != in.Key || run.Snapshot.ExecutionScope != in.ExecutionScope {
		return scope, "", ErrHookExecutionUnavailable
	}
	for _, op := range run.Operations {
		if op.OperationID == in.OperationID && op.Phase == in.Phase {
			scope = entity.HookAttemptScope{Key: in.Key, OperationID: in.OperationID, Phase: in.Phase, ExpectedVersion: op.Version, ExecutionScope: in.ExecutionScope, SnapshotHash: run.Snapshot.Hash}
			if scope.Validate() != nil {
				return scope, "", ErrHookExecutionUnavailable
			}
			if in.Phase == entity.HookPhaseBefore {
				return scope, run.State.Before.Status, nil
			}
			return scope, run.State.After.Status, nil
		}
	}
	return scope, "", ErrHookExecutionUnavailable
}

func (e *HookAttemptExecutor) recover(ctx context.Context, scope entity.HookAttemptScope, config *entity.HookConfig) (hookcomponent.AttemptExecutionResult, error) {
	out, err := e.repo.RecoverExpiredAttempt(ctx, entity.HookRecoverExpiredAttemptInput{HookAttemptScope: scope, Config: config})
	if errors.Is(err, entity.ErrHookStoreConflict) {
		return hookcomponent.AttemptExecutionResult{RecoveryPending: true}, nil
	}
	if err != nil {
		return hookcomponent.AttemptExecutionResult{}, ErrHookExecutionUnavailable
	}
	return hookcomponent.AttemptExecutionResult{HookAttemptStoreResult: out, RecoveryPending: !out.Changed}, nil
}

func hookExecutionRemaining(anchor entity.HookClockAnchor, deadline, at time.Time) (time.Duration, error) {
	w, err := anchor.Window(at)
	return deadline.Sub(w.Latest), err
}

func hookExecutionWait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func hookExecutionNil(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan:
		return r.IsNil()
	}
	return false
}
