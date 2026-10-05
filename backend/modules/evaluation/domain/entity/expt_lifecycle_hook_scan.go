// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

type HookScanKind string

const (
	HookScanDue       HookScanKind = "due"
	HookScanExpired   HookScanKind = "expired"
	HookScanPreparing HookScanKind = "preparing"
	HookScanFinalize  HookScanKind = "finalize"
)

// HookScanInput uses an explicit millisecond Now in the same session/driver clock
// as storage (e.g. a sampled DB time), fixed for a whole cursor chain.
// Status must be pending/retry_wait for due scans, and empty for the other kinds.
type HookScanInput struct {
	ExecutionScope string
	Status         HookOperationStatus
	Now            time.Time
	Limit          int
	Cursor         *HookScanCursor
}

// HookScanCursor is internal query state, not authorization or a claim token.
type HookScanCursor struct {
	Kind                   HookScanKind
	ExecutionScope, Status string
	Now, At                time.Time
	ID, WorkspaceID, RunID int64
}

type HookScanPage[T any] struct {
	Candidates []T
	// NextCursor is the raw page tail, even when the caller discards every candidate.
	NextCursor *HookScanCursor
	HasMore    bool
}

type HookOperationCandidate struct {
	Key         HookRunKey
	ID          int64
	OperationID string
	Phase       HookPhase
	Version     int64
	// DueAt is next_attempt_at for due scans, lease_until for expired scans.
	DueAt time.Time
}

type HookRunCandidate struct {
	Key     HookRunKey
	Version int64
	// ReconcileAt is zero for a preparing-plan candidate.
	ReconcileAt time.Time
}

func (in HookScanInput) Validate(kind HookScanKind) error {
	if !hookStorageASCII(in.ExecutionScope, 128) || in.Limit < 1 || in.Limit > 100 || !hookScanTime(in.Now) {
		return invalidParam("invalid hook scan scope, limit or clock")
	}
	status := ""
	switch kind {
	case HookScanDue:
		if in.Status != HookOperationPending && in.Status != HookOperationRetryWait {
			return invalidParam("hook due scan requires one pending or retry_wait status")
		}
		status = string(in.Status)
	case HookScanExpired:
		status = "running"
	case HookScanPreparing:
		status = "preparing"
	case HookScanFinalize:
		status = "pending"
	default:
		return invalidParam("invalid hook scan kind")
	}
	if kind != HookScanDue && in.Status != "" {
		return invalidParam("hook scan status is fixed for this kind")
	}
	c := in.Cursor
	if c == nil {
		return nil
	}
	if c.Kind != kind || c.ExecutionScope != in.ExecutionScope || c.Status != status || !c.Now.Equal(in.Now) || c.Now.Format(time.RFC3339Nano) != in.Now.Format(time.RFC3339Nano) {
		return invalidParam("hook scan cursor does not belong to this query")
	}
	if kind == HookScanPreparing {
		if !c.At.IsZero() {
			return invalidParam("preparing cursor has no time key")
		}
	} else if !hookScanTime(c.At) || c.At.After(in.Now) || c.At.Format("2006-01-02 15:04:05.000") > in.Now.Format("2006-01-02 15:04:05.000") {
		return invalidParam("invalid or inverted hook scan time cursor")
	}
	if kind == HookScanDue || kind == HookScanExpired {
		if c.ID <= 0 || c.WorkspaceID != 0 || c.RunID != 0 {
			return invalidParam("invalid hook operation scan cursor")
		}
	} else if c.ID != 0 || c.WorkspaceID <= 0 || c.RunID <= 0 {
		return invalidParam("invalid hook run scan cursor")
	}
	return nil
}

func hookScanTime(at time.Time) bool {
	year := at.Year()
	return year >= 1000 && year <= 9999 && at.Nanosecond()%int(time.Millisecond) == 0
}
