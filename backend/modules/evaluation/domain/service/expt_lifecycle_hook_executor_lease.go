// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

var ErrHookExecutionLeaseLost = errors.New("hook execution lease lost; remote outcome unknown")

const hookExecutionCompleteAttempts = 3

type hookExecutionLease struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
	done    chan struct{}
	mu      sync.Mutex
	current entity.HookAttemptStoreResult
}

func (l *hookExecutionLease) snapshot() entity.HookAttemptStoreResult {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.current
}

func (l *hookExecutionLease) stop() { l.cancel(context.Canceled); <-l.done }

func (e *HookAttemptExecutor) keepLease(ctx context.Context, scope entity.HookAttemptScope, claimed entity.HookAttemptStoreResult, timing entity.HookRuntimeConfig) (*hookExecutionLease, error) {
	sampledAt := time.Now()
	delay, err := hookExecutionRemaining(*claimed.Clock, claimed.Claim.LeaseUntil, sampledAt)
	if err != nil || delay <= 0 {
		return nil, ErrHookExecutionLeaseLost
	}
	leaseCtx, cancel := context.WithCancelCause(ctx)
	l := &hookExecutionLease{ctx: leaseCtx, cancel: cancel, done: make(chan struct{}), current: claimed}
	// Arm against the original monotonic instant, before scheduling the renewer.
	expiry := time.AfterFunc(time.Until(sampledAt.Add(delay)), func() { cancel(ErrHookExecutionLeaseLost) })
	go func() {
		defer close(l.done)
		ticker := time.NewTicker(time.Duration(timing.RenewSeconds) * time.Second)
		defer ticker.Stop()
		// An independent timer cancels HTTP even while RenewAttempt is blocked in DB.
		defer expiry.Stop()
		for {
			select {
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				current := l.snapshot()
				scope.ExpectedVersion = current.Claim.Version
				renewed, err := e.repo.RenewAttempt(leaseCtx, entity.HookRenewAttemptInput{HookAttemptScope: scope, HookAttemptIdentity: current.Claim.HookAttemptIdentity, LeaseSeconds: &timing.LeaseSeconds})
				if err != nil || renewed.Claim == nil || renewed.Clock == nil || renewed.Claim.HookAttemptIdentity != claimed.Claim.HookAttemptIdentity {
					cancel(ErrHookExecutionLeaseLost)
					return
				}
				sampledAt := time.Now()
				delay, err := hookExecutionRemaining(*renewed.Clock, renewed.Claim.LeaseUntil, sampledAt)
				if err != nil || delay <= 0 || leaseCtx.Err() != nil {
					cancel(ErrHookExecutionLeaseLost)
					return
				}
				l.mu.Lock()
				l.current = renewed
				l.mu.Unlock()
				if !expiry.Stop() {
					cancel(ErrHookExecutionLeaseLost)
					return
				}
				expiry.Reset(time.Until(sampledAt.Add(delay)))
			}
		}
	}()
	return l, nil
}

func (e *HookAttemptExecutor) complete(ctx context.Context, in entity.HookCompleteAttemptInput) (entity.HookAttemptStoreResult, error) {
	for attempt := 0; attempt < hookExecutionCompleteAttempts; attempt++ {
		out, err := e.repo.CompleteAttempt(ctx, in)
		if err == nil {
			return out, nil
		}
		if !errors.Is(err, entity.ErrHookStoreConflict) {
			return entity.HookAttemptStoreResult{}, ErrHookExecutionUnavailable
		}
		if attempt == hookExecutionCompleteAttempts-1 {
			break
		}
		current, err := e.reader.ReadAttempt(ctx, in.HookAttemptScope)
		if err != nil {
			return entity.HookAttemptStoreResult{}, ErrHookExecutionUnavailable
		}
		if current.Claim == nil || current.Claim.HookAttemptIdentity != in.HookAttemptIdentity {
			// Let the repository record a fenced late response; never adopt a new owner/token.
			out, err = e.repo.CompleteAttempt(ctx, in)
			if err != nil {
				return entity.HookAttemptStoreResult{}, ErrHookExecutionUnavailable
			}
			return out, nil
		}
		in.ExpectedVersion = current.Claim.Version
	}
	return entity.HookAttemptStoreResult{}, ErrHookExecutionUnavailable
}
