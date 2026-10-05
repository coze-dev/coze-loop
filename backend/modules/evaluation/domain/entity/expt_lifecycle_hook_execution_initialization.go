// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"math"
)

var (
	ErrHookExecutionStorage     = errors.New("hook execution initialization unavailable")
	ErrHookExecutionUnsupported = errors.New("hook execution initialization scope unsupported")
)

func HookExecutionInitializationRequired(before bool, mode ExptRunMode, exptType ExptType, source ExptEvalSetSourceType) bool {
	if before && mode == EvaluationModeAppend && exptType == ExptType_Online {
		return true
	}
	return before && (mode == EvaluationModeSubmit || mode == EvaluationModeTrialRun) && exptType == ExptType_Offline && (source == 0 || source == ExptEvalSetSourceType_SingleSet)
}

type HookExecutionTurnManifest struct {
	TurnID   int64 `json:"turn_id"`
	TurnIdx  int32 `json:"turn_idx"`
	ResultID int64 `json:"result_id"`
	RunLogID int64 `json:"run_log_id,omitempty"`
}

// This private manifest binds committed records, never source content or identity.
type HookExecutionManifest struct {
	Version             int32                       `json:"version"`
	Key                 HookRunKey                  `json:"key"`
	Ordinal             int64                       `json:"ordinal"`
	Frozen              HookPlanItem                `json:"frozen"`
	ItemResultID        int64                       `json:"item_result_id"`
	ItemRunLogID        int64                       `json:"item_runlog_id"`
	Turns               []HookExecutionTurnManifest `json:"turns"`
	NoExecutionFailure  bool                        `json:"no_execution_failure,omitempty"`
	TurnLogsInitialized *bool                       `json:"turn_logs_initialized,omitempty"`
	ItemRef             *HookExecutionItemRef       `json:"item_ref,omitempty"`
	Retry               *HookExecutionRetrySource   `json:"retry,omitempty"`
}

type HookExecutionItemRef struct {
	ID         int64  `json:"id"`
	ConfigHash string `json:"config_hash"`
}

func (m HookExecutionManifest) Clone() HookExecutionManifest {
	m.Retry = m.Retry.Clone()
	m.Turns = append([]HookExecutionTurnManifest(nil), m.Turns...)
	if m.ItemRef != nil {
		ref := *m.ItemRef
		m.ItemRef = &ref
	}
	if m.TurnLogsInitialized != nil {
		state := *m.TurnLogsInitialized
		m.TurnLogsInitialized = &state
	}
	return m
}

func (m HookExecutionManifest) Validate() error {
	if m.Version != 1 || !validHookRunKey(m.Key) || m.Ordinal < 0 || m.Ordinal > math.MaxInt32 || m.Frozen.ID <= 0 ||
		m.Frozen.SourceSpaceID <= 0 || m.Frozen.EvalSetID <= 0 || m.Frozen.ItemID <= 0 || m.Frozen.EvalSetVersionID < 0 || m.Frozen.ItemVersionID < 0 || len(m.Turns) == 0 || int64(len(m.Turns)) > math.MaxInt32 {
		return ErrHookStoreCorrupt
	}
	ids := map[int64]bool{m.Frozen.ID: true}
	for _, id := range []int64{m.ItemResultID, m.ItemRunLogID} {
		if id <= 0 || ids[id] {
			return ErrHookStoreCorrupt
		}
		ids[id] = true
	}
	turns := make(map[int64]bool, len(m.Turns))
	if m.ItemRef != nil {
		if m.ItemRef.ID <= 0 || ids[m.ItemRef.ID] || !hookStorageHash(m.ItemRef.ConfigHash) {
			return ErrHookStoreCorrupt
		}
		ids[m.ItemRef.ID] = true
	}
	if s := m.Retry; s != nil {
		if s.RunID <= 0 || s.RunID == m.Key.RunID || s.Ordinal < 0 || s.Ordinal > math.MaxInt32 || len(s.ItemConfig) == 0 || len(s.Turns) != len(m.Turns) {
			return ErrHookStoreCorrupt
		}
		for i, t := range s.Turns {
			if t.RunID <= 0 || t.RunID == m.Key.RunID || t.TurnID != m.Turns[i].TurnID || t.ResultID != m.Turns[i].ResultID {
				return ErrHookStoreCorrupt
			}
		}
		for id, run := range s.ReusedTargets {
			if id <= 0 || run <= 0 || run == m.Key.RunID {
				return ErrHookStoreCorrupt
			}
		}
		for id, run := range s.ReusedEvaluators {
			if id <= 0 || run <= 0 || run == m.Key.RunID {
				return ErrHookStoreCorrupt
			}
		}
	}
	initialized := m.TurnLogsInitialized != nil && *m.TurnLogsInitialized
	if initialized && m.NoExecutionFailure {
		return ErrHookStoreCorrupt
	}
	for i, turn := range m.Turns {
		if turn.TurnID < 0 || turn.TurnIdx != int32(i) || turns[turn.TurnID] || turn.ResultID <= 0 || ids[turn.ResultID] {
			return ErrHookStoreCorrupt
		}
		turns[turn.TurnID], ids[turn.ResultID] = true, true
		if initialized {
			if turn.RunLogID <= 0 || ids[turn.RunLogID] {
				return ErrHookStoreCorrupt
			}
			ids[turn.RunLogID] = true
		} else if turn.RunLogID != 0 {
			return ErrHookStoreCorrupt
		}
	}
	return nil
}

type HookExecutionInitializationReadInput = HookPlanReadInput

type HookExecutionInitializationItem struct {
	Ordinal           int64
	Frozen            HookPlanItem
	Manifest          *HookExecutionManifest
	ItemRefConfigHash string
	Reuse             *HookExecutionManifest
}

type HookExecutionInitializationPage struct {
	Count             int64
	Hash              string
	RunVersion        int64
	Initialized       bool
	NextOrdinal       int64
	HasMore           bool
	Items             []HookExecutionInitializationItem
	BoundSnapshotHash string
}

type HookExecutionInitializationWriteInput struct {
	HookStoreGuard
	ExecutionScope string
	PlanHash       string
	StartOrdinal   int64
	Items          []HookExecutionManifest
}

func (in HookExecutionInitializationWriteInput) Validate() error {
	if err := in.HookStoreGuard.Validate(); err != nil {
		return err
	}
	if !hookStorageASCII(in.ExecutionScope, 128) || !hookStorageHash(in.PlanHash) || in.StartOrdinal < 0 || len(in.Items) == 0 || len(in.Items) > 100 {
		return ErrHookStoreCorrupt
	}
	seen := map[int64]bool{}
	for i, m := range in.Items {
		if m.Validate() != nil || m.Key != in.Key || m.Ordinal != in.StartOrdinal+int64(i) || seen[m.Frozen.ItemID] {
			return ErrHookStoreCorrupt
		}
		seen[m.Frozen.ItemID] = true
	}
	return nil
}

type HookExecutionInitializationCompleteInput struct {
	HookStoreGuard
	ExecutionScope    string
	PlanHash          string
	ExpectedItemCount int64
	ExpectedTurnCount int64
}

func (in HookExecutionInitializationCompleteInput) Validate() error {
	if err := in.HookStoreGuard.Validate(); err != nil {
		return err
	}
	if !hookStorageASCII(in.ExecutionScope, 128) || !hookStorageHash(in.PlanHash) || in.ExpectedItemCount < 0 || in.ExpectedItemCount > math.MaxInt32 || in.ExpectedTurnCount < 0 {
		return ErrHookStoreCorrupt
	}
	return nil
}

type HookExecutionInitializationCompletion struct {
	Initialized bool
	Changed     bool
	RunVersion  int64
}
