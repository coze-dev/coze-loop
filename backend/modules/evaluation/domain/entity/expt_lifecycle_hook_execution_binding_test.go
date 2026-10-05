// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHookBoundManifestLegacyBytesAndReferenceValidation(t *testing.T) {
	const legacy = `{"version":1,"key":{"WorkspaceID":1,"ExperimentID":2,"RunID":3},"ordinal":0,"frozen":{"ID":4,"SourceSpaceID":1,"EvalSetID":5,"EvalSetVersionID":6,"ItemID":7,"ItemVersionID":0},"item_result_id":8,"item_runlog_id":9,"turns":[{"turn_id":0,"turn_idx":0,"result_id":10}]}`
	var m HookExecutionManifest
	require.NoError(t, json.Unmarshal([]byte(legacy), &m))
	require.NoError(t, m.Validate())
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	require.Equal(t, legacy, string(raw))
	m.ItemRef = &HookExecutionItemRef{ID: 11, ConfigHash: strings.Repeat("a", 64)}
	require.NoError(t, m.Validate())
	clone := m.Clone()
	clone.ItemRef.ConfigHash = strings.Repeat("b", 64)
	require.Equal(t, strings.Repeat("a", 64), m.ItemRef.ConfigHash)
	for _, ref := range []*HookExecutionItemRef{{ID: 0, ConfigHash: strings.Repeat("a", 64)}, {ID: 8, ConfigHash: strings.Repeat("a", 64)}, {ID: 10, ConfigHash: strings.Repeat("a", 64)}, {ID: 11, ConfigHash: "invalid"}} {
		m.ItemRef = ref
		require.Error(t, m.Validate())
	}
}

func TestHookBoundSourceOwnsItsDecodedInput(t *testing.T) {
	in := executionSnapshotInput()
	s, err := NewHookRunSnapshot(in)
	require.NoError(t, err)
	run := &HookStoredRun{State: HookRunState{Key: in.Key, Status: ExptStatus_Pending, Gate: HookGateWaiting, Finalize: HookFinalizeNone, Before: HookOperation{ID: "before", Status: HookOperationPending}, After: HookOperation{Status: HookOperationDisabled}}, Snapshot: HookProtectedSnapshot{Hash: strings.Repeat("a", 64), ExecutionScope: in.ExecutionScope}, Mode: EvaluationModeSubmit, CreatedBy: "trusted-user"}
	binding, err := NewHookExecutionInitializationBinding(run, s)
	require.NoError(t, err)
	run.Snapshot.Hash = strings.Repeat("b", 64)
	run.CreatedBy = "changed"
	got := binding.Input()
	got.Execution.Sets[0].ItemConfig.EvalTargetConf.DynamicConf["private"] = "changed"
	owned := binding.Input()
	require.Equal(t, strings.Repeat("a", 64), owned.SnapshotHash)
	require.Equal(t, "trusted-user", owned.CreatedBy)
	require.Equal(t, "first", owned.Execution.Sets[0].ItemConfig.EvalTargetConf.DynamicConf["private"])
	_, err = NewHookExecutionInitializationBinding(run, s)
	require.Error(t, err)
	_, err = NewHookExecutionInitializationBinding(nil, s)
	require.Error(t, err)
	require.Nil(t, (*HookExecutionInitializationBinding)(nil).Input().Execution)
}
