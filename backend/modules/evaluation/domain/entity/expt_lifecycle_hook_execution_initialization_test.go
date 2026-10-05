package entity

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func executionManifestFixture() HookExecutionManifest {
	return HookExecutionManifest{Version: 1, Key: HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, Ordinal: 0,
		Frozen: HookPlanItem{ID: 4, SourceSpaceID: 1, EvalSetID: 5, ItemID: 6}, ItemResultID: 7, ItemRunLogID: 8,
		Turns: []HookExecutionTurnManifest{{TurnID: 0, TurnIdx: 0, ResultID: 9}}}
}

func TestHookExecutionManifest(t *testing.T) {
	m := executionManifestFixture()
	require.NoError(t, m.Validate())
	copy := m.Clone()
	copy.Turns[0].TurnID = 5
	require.Zero(t, m.Turns[0].TurnID)
	for _, change := range []func(*HookExecutionManifest){
		func(m *HookExecutionManifest) { m.Version = 2 },
		func(m *HookExecutionManifest) { m.Key.RunID = 0 },
		func(m *HookExecutionManifest) { m.Ordinal = -1 },
		func(m *HookExecutionManifest) { m.Frozen.ItemVersionID = -1 },
		func(m *HookExecutionManifest) { m.ItemResultID = m.Frozen.ID },
		func(m *HookExecutionManifest) { m.Turns = nil },
		func(m *HookExecutionManifest) { m.Turns[0].TurnID = -1 },
		func(m *HookExecutionManifest) { m.Turns[0].TurnIdx = 1 },
		func(m *HookExecutionManifest) {
			m.Turns = append(m.Turns, HookExecutionTurnManifest{TurnID: 0, TurnIdx: 1, ResultID: 10})
		},
		func(m *HookExecutionManifest) { m.Turns[0].ResultID = m.ItemResultID },
	} {
		bad := m.Clone()
		change(&bad)
		require.Error(t, bad.Validate())
	}
}
