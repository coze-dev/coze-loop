// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// Construct only from a repository Run and its successfully decoded snapshot.
type HookExecutionInitializationBinding struct {
	snapshot    *HookRunSnapshot
	hash        string
	creator     string
	sourceRunID int64
}

type HookExecutionInitializationSource struct {
	Key                                     HookRunKey
	ExecutionScope, SnapshotHash, CreatedBy string
	Mode                                    ExptRunMode
	BeforeEnabled, AfterEnabled             bool
	Execution                               *HookExecutionSnapshot
	SourceRunID                             int64
}

func NewHookExecutionInitializationBinding(run *HookStoredRun, decoded *HookRunSnapshot) (*HookExecutionInitializationBinding, error) {
	if run == nil || decoded == nil || !hookStorageHash(run.Snapshot.Hash) || ValidateHookStorageState(&run.State) != nil {
		return nil, ErrHookStoreCorrupt
	}
	snapshot, err := NewHookRunSnapshot(decoded.Input())
	if err != nil {
		return nil, ErrHookStoreCorrupt
	}
	in := snapshot.Input()
	if in.Execution == nil {
		return nil, ErrHookExecutionUnsupported
	}
	if in.Key != run.State.Key || in.ExecutionScope != run.Snapshot.ExecutionScope || in.Execution.Mode != run.Mode || in.Context.Initiator.GetUserID() != run.CreatedBy || snapshotStageEnabled(in.Config.Before) != (run.State.Before.Status != HookOperationDisabled) || snapshotStageEnabled(in.Config.After) != (run.State.After.Status != HookOperationDisabled) {
		return nil, ErrHookStoreConflict
	}
	sourceID := int64(0)
	if run.SourceRunID != nil {
		sourceID = *run.SourceRunID
	}
	if HookBoundRetryMode(run.Mode) && (sourceID <= 0 || sourceID == run.State.Key.RunID) {
		return nil, ErrHookStoreConflict
	}
	return &HookExecutionInitializationBinding{snapshot: snapshot, hash: run.Snapshot.Hash, creator: run.CreatedBy, sourceRunID: sourceID}, nil
}

func (b *HookExecutionInitializationBinding) Input() HookExecutionInitializationSource {
	if b == nil || b.snapshot == nil {
		return HookExecutionInitializationSource{}
	}
	in := b.snapshot.Input()
	return HookExecutionInitializationSource{Key: in.Key, ExecutionScope: in.ExecutionScope, SnapshotHash: b.hash, CreatedBy: b.creator, Mode: in.Execution.Mode, BeforeEnabled: snapshotStageEnabled(in.Config.Before), AfterEnabled: snapshotStageEnabled(in.Config.After), Execution: in.Execution, SourceRunID: b.sourceRunID}
}
