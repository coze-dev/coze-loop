// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// WorkerClock samples the same primary DB/session/driver wall clock as storage.
// A sample is fixed for a complete scan cursor chain; it is not an HTTP clock.
type WorkerClock interface {
	Now(context.Context) (time.Time, error)
}

type AttemptExecutor interface {
	Execute(context.Context, AttemptExecutionInput) (AttemptExecutionResult, error)
}

type WorkerRunInput struct {
	ExecutionScope string
	Candidate      entity.HookRunCandidate
}

// WorkerEffects contains only coordination signals, never SPI results or secrets.
// WakeEvaluation is a hint to recheck the Gate, not permission to admit an item.
type WorkerEffects struct {
	ExecutionScope                               string
	Key                                          entity.HookRunKey
	OperationID                                  string
	WakeEvaluation, BeginFinalize, ActivateAfter bool
}

// WorkerCoordinator must recheck current scoped state/fences, be idempotent and
// honor cancellation. PreparePlan performs one bounded page, not a full plan.
// ApplyEffects is retried locally at most three times. Durable reconciliation
// and lost evaluation wake recovery remain the caller's integration obligations.
type WorkerCoordinator interface {
	PreparePlan(context.Context, WorkerRunInput) error
	FinalizeRun(context.Context, WorkerRunInput) error
	ApplyEffects(context.Context, WorkerEffects) error
}

type WorkerEventCode string

const (
	WorkerConfigFailed       WorkerEventCode = "config_failed"
	WorkerPaused             WorkerEventCode = "paused"
	WorkerClockFailed        WorkerEventCode = "clock_failed"
	WorkerScanFailed         WorkerEventCode = "scan_failed"
	WorkerExecutionFailed    WorkerEventCode = "execution_failed"
	WorkerCoordinationFailed WorkerEventCode = "coordination_failed"
	WorkerRecoveryPending    WorkerEventCode = "recovery_pending"
	WorkerEffectsRetry       WorkerEventCode = "effects_retry"
	WorkerEffectsExhausted   WorkerEventCode = "effects_exhausted"
	WorkerEffectsAbandoned   WorkerEventCode = "effects_abandoned"
	WorkerIDFailed           WorkerEventCode = "id_failed"
)

// WorkerEvent deliberately excludes raw errors, business IDs and payloads.
type WorkerEvent struct {
	Code WorkerEventCode
	Kind entity.HookScanKind
}

// WorkerObserver is a required, concurrency-safe, nonblocking local metric/log
// sink. It must not call Run/Stop synchronously or start unbounded background work.
type WorkerObserver interface{ Observe(WorkerEvent) }
