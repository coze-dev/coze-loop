// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrHookStoreConflict   = errors.New("hook storage version or ownership conflict")
	ErrHookStoreMissing    = errors.New("hook storage run not found")
	ErrHookStoreCorrupt    = errors.New("hook storage incomplete or inconsistent")
	ErrHookAdmissionDenied = errors.New("hook item admission denied")
)

// Cipher is supplied by the trusted protector; storage never encrypts or decrypts it.
type HookProtectedSnapshot struct {
	Cipher                      []byte `json:"-"`
	KeyID, Hash, ExecutionScope string
}

type HookOperationSeed struct {
	ID                          int64
	OperationID, IdempotencyKey string
}

type HookCreateRunInput struct {
	Key                 HookRunKey
	RunLog              *ExptRunLog
	Snapshot            HookProtectedSnapshot
	SourceRunID         *int64
	ExpectedLatestRunID int64
	// ExpectedConfigRevision comes from IHookConfigRepo.GetConfig; empty expects absent storage.
	ExpectedConfigRevision string
	Before, After          *HookOperationSeed
}

type HookStoreGuard struct {
	Key             HookRunKey
	ExpectedVersion int64
}

type HookPlanItem struct {
	ID, SourceSpaceID, EvalSetID, EvalSetVersionID, ItemID, ItemVersionID int64
}

type HookPlanPageInput struct {
	HookStoreGuard
	StartOrdinal       int64
	Cursor, NextCursor string
	Items              []HookPlanItem
}

type HookFinishPlanInput struct {
	HookStoreGuard
	Count int64
	Hash  string
}

type HookFinalizeInput struct {
	HookStoreGuard
	Intent HookTerminalIntent
	// Optional normal-run counts; nil preserves the existing finalization contract.
	Stats *HookFinalizationStats
	// Optional display text, independent of the SPI reason code; frozen by BeginFinalize.
	// Replays supply the persisted value; nil retains the existing writer behavior.
	DisplayMessage *string
}

type HookAdmitItemInput struct {
	HookStoreGuard
	ItemID int64
}

type HookStoredOperation struct {
	HookOperationSeed
	Phase                                  HookPhase
	Version                                int64
	ActivatedAt, OccurredAt, NextAttemptAt *time.Time
}

type HookStoredRun struct {
	DisplayMessage              *string
	State                       HookRunState
	Version                     int64
	Snapshot                    HookProtectedSnapshot
	SourceRunID                 *int64
	CreatedBy                   string
	Mode                        ExptRunMode
	PlanReady                   bool
	PlanCursor, PlanHash        string
	PlanCount                   int64
	ExecutionStarted            bool
	ExecutionInitialized        bool
	TerminalAt, NextReconcileAt *time.Time
	Operations                  []HookStoredOperation
}

type HookStoreResult struct {
	Run                      *HookStoredRun
	Changed, LatestProjected bool
	Effects                  HookStateEffects
}

type HookAdmitItemResult struct {
	Admitted, NewlyAdmitted bool
	AdmittedAt              time.Time
	Version                 int64
}

func (in HookCreateRunInput) Validate() error {
	log := in.RunLog
	if !validHookRunKey(in.Key) || log == nil || log.ID != in.Key.RunID || log.ExptRunID != in.Key.RunID || log.SpaceID != in.Key.WorkspaceID || log.ExptID != in.Key.ExperimentID {
		return invalidParam("invalid hook run identity")
	}
	if strings.TrimSpace(log.CreatedBy) == "" || len(log.CreatedBy) > 128 || !utf8.ValidString(log.CreatedBy) || log.Mode < 1 || log.Mode > 6 || (log.Status != int64(ExptStatus_Pending) && log.Status != int64(ExptStatus_Processing)) {
		return invalidParam("invalid hook run author, mode or initial status")
	}
	if in.ExpectedLatestRunID < 0 || (in.SourceRunID != nil && (*in.SourceRunID <= 0 || *in.SourceRunID == in.Key.RunID)) || (in.Before == nil && in.After == nil) {
		return invalidParam("invalid hook initialization target")
	}
	if len(in.Snapshot.Cipher) == 0 || len(in.Snapshot.Cipher) > 16777215 || !hookStorageASCII(in.Snapshot.KeyID, 128) || !hookStorageHash(in.Snapshot.Hash) || !hookStorageASCII(in.Snapshot.ExecutionScope, 128) {
		return invalidParam("invalid protected hook snapshot reference")
	}
	for _, operation := range []*HookOperationSeed{in.Before, in.After} {
		if operation != nil && (operation.ID <= 0 || !hookStorageASCII(operation.OperationID, 128) || !hookStorageASCII(operation.IdempotencyKey, 256)) {
			return invalidParam("invalid hook operation identity")
		}
	}
	if in.Before != nil && in.After != nil && (in.Before.ID == in.After.ID || in.Before.OperationID == in.After.OperationID || in.Before.IdempotencyKey == in.After.IdempotencyKey) {
		return invalidParam("hook phases require distinct identities")
	}
	return nil
}

func (in HookStoreGuard) Validate() error {
	if !validHookRunKey(in.Key) || in.ExpectedVersion < 0 || in.ExpectedVersion == math.MaxInt64 {
		return invalidParam("invalid hook run key or version")
	}
	return nil
}

func (in HookPlanPageInput) Validate() error {
	if err := in.HookStoreGuard.Validate(); err != nil {
		return err
	}
	if in.StartOrdinal < 0 || len(in.Items) == 0 || in.StartOrdinal > math.MaxInt64-int64(len(in.Items)) || len(in.Cursor) > 65535 || len(in.NextCursor) > 65535 || !utf8.ValidString(in.Cursor) || !utf8.ValidString(in.NextCursor) {
		return invalidParam("invalid hook plan page")
	}
	ids, items := make(map[int64]bool), make(map[int64]bool)
	for _, item := range in.Items {
		if item.ID <= 0 || item.SourceSpaceID <= 0 || item.EvalSetID <= 0 || item.EvalSetVersionID < 0 || item.ItemID <= 0 || item.ItemVersionID < 0 || ids[item.ID] || items[item.ItemID] {
			return invalidParam("invalid or duplicate hook plan item")
		}
		ids[item.ID], items[item.ItemID] = true, true
	}
	return nil
}

func (in HookFinishPlanInput) Validate() error {
	if err := in.HookStoreGuard.Validate(); err != nil {
		return err
	}
	if in.Count < 0 || !hookStorageHash(in.Hash) {
		return invalidParam("invalid hook plan count or hash")
	}
	return nil
}

func (in HookFinalizeInput) Validate() error {
	if err := in.HookStoreGuard.Validate(); err != nil {
		return err
	}
	if !IsExptFinished(in.Intent.Status) || !utf8.ValidString(in.Intent.Reason) || utf8.RuneCountInString(in.Intent.Reason) > 128 {
		return invalidParam("invalid hook terminal intent")
	}
	if in.DisplayMessage != nil && !utf8.ValidString(*in.DisplayMessage) {
		return invalidParam("invalid hook display message")
	}
	return nil
}

func (in HookAdmitItemInput) Validate() error {
	if err := in.HookStoreGuard.Validate(); err != nil {
		return err
	}
	if in.ItemID <= 0 {
		return invalidParam("invalid hook admission item")
	}
	return nil
}

func hookStorageASCII(value string, limit int) bool {
	if value == "" || len(value) > limit || strings.TrimSpace(value) != value {
		return false
	}
	for _, c := range value {
		if c < 33 || c > 126 {
			return false
		}
	}
	return true
}

func hookStorageHash(value string) bool { return len(value) == 64 && hookStorageASCII(value, 64) }

// ValidateHookStorageState shares the lifecycle invariants with the pure rules.
func ValidateHookStorageState(state *HookRunState) error { return validateHookState(state) }
