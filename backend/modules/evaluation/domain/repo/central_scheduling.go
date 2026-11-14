// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// CentralActiveRunRepo is the read-only global scan used by the central
// scheduler. It deliberately never writes to MySQL: scheduling state lives in
// the qn:central: Redis namespace.
type CentralActiveRunRepo interface {
	// ListRecentRuns returns runs updated within [since, now]. Finished runs are
	// included so their reservations can be released; callers must check status.
	ListRecentRuns(ctx context.Context, since time.Time, limit int) ([]*entity.CentralActiveRun, error)
	// Snapshots returns item-status aggregates for the given runs. The returned
	// map is keyed by "expt_id:run_id"; missing runs mean zero items.
	Snapshots(ctx context.Context, runs []*entity.CentralActiveRun) (map[string]*entity.CentralRunSnapshot, error)
	// LoadExperiment returns the minimum experiment projection needed to
	// compute a resource demand. It must not load evaluator payloads.
	LoadExperiment(ctx context.Context, exptID, spaceID int64) (*entity.CentralExperimentScheduling, error)
	// QueueingItemIDs returns the oldest queueing item ids up to limit.
	QueueingItemIDs(ctx context.Context, run *entity.CentralActiveRun, limit int) ([]int64, error)
	// ProcessingItemIDs returns item ids currently processing.
	ProcessingItemIDs(ctx context.Context, run *entity.CentralActiveRun, limit int) ([]int64, error)
	// ItemStates returns the current state for the given item ids. Missing ids
	// are absent from the map.
	ItemStates(ctx context.Context, run *entity.CentralActiveRun, itemIDs []int64) (map[int64]entity.ItemRunState, error)
}

// CentralReservationStore owns the atomic multi-resource quota ledger.
type CentralReservationStore interface {
	// Reserve is all-or-nothing across every capacity in the request.
	Reserve(ctx context.Context, req *entity.CentralReserveRequest) (*entity.CentralReserveResult, error)
	// Release is idempotent; it returns false when the reservation was already
	// gone. It never decrements a counter below zero.
	Release(ctx context.Context, env, reservationID string) (bool, error)
	// List returns all held reservations for one run.
	List(ctx context.Context, env string, exptID, runID int64) ([]*entity.CentralReservation, error)
	// ReleaseRun releases every reservation of a run and returns the count.
	ReleaseRun(ctx context.Context, env string, exptID, runID int64) (int, error)
	// MarkDispatched flips a held reservation to dispatched and bumps the
	// ledger's dispatched counter. It is idempotent.
	MarkDispatched(ctx context.Context, env, reservationID string) error
	// RenewRun extends TTLs for still-running items.
	RenewRun(ctx context.Context, env string, exptID, runID int64, ttl time.Duration) error
	// Ledger returns the run-level double-entry counters used by reconcile.
	Ledger(ctx context.Context, env string, exptID, runID int64) (*entity.CentralRunLedger, error)
}

// CentralOwnershipStore pins an experiment to the environment that submitted
// it. The ownership key is deliberately NOT namespaced by environment so every
// environment sees the same value and can refuse to touch a foreign run.
type CentralOwnershipStore interface {
	Claim(ctx context.Context, exptID, runID int64, owner string, ttl time.Duration) (bool, string, error)
	Owner(ctx context.Context, exptID int64) (string, error)
	Release(ctx context.Context, exptID int64, owner string) error
}
