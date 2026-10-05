// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func selectionExperiment() *Experiment {
	return &Experiment{EvalSetSourceType: 2, EvalSetID: 71, EvalSetVersionID: 71, EvalSetSpaceID: 10, TrialRunItemCount: 3, EvalConf: &EvaluationConfiguration{EvalSetConfigs: []*EvalSetConfig{{EvalSetID: 71, EvalSetVersionID: 71, SourceSpaceID: 10, ItemFilter: &ExptItemFilter{QueryAndOr: "and", FilterFields: []*ExptItemFilterField{{FieldName: "tag", FieldType: "string", Values: []string{"chosen"}}}}}, {EvalSetID: 81, EvalSetVersionID: 82, SourceSpaceID: 11}}}}
}
func TestHookSelectionFingerprintTracksOnlySelection(t *testing.T) {
	base, err := HookSelectionConfigFingerprint(selectionExperiment())
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		mutate func(*Experiment)
	}{
		{"source_type", func(e *Experiment) { e.EvalSetSourceType = 1 }},
		{"main_id", func(e *Experiment) { e.EvalSetID++ }},
		{"main_version", func(e *Experiment) { e.EvalSetVersionID = 0 }},
		{"main_source", func(e *Experiment) { e.EvalSetSpaceID++ }},
		{"count", func(e *Experiment) { e.TrialRunItemCount++ }},
		{"set_id", func(e *Experiment) { e.EvalConf.EvalSetConfigs[1].EvalSetID++ }},
		{"set_version", func(e *Experiment) { e.EvalConf.EvalSetConfigs[1].EvalSetVersionID++ }},
		{"set_source", func(e *Experiment) { e.EvalConf.EvalSetConfigs[1].SourceSpaceID++ }},
		{"order", func(e *Experiment) {
			e.EvalConf.EvalSetConfigs[0], e.EvalConf.EvalSetConfigs[1] = e.EvalConf.EvalSetConfigs[1], e.EvalConf.EvalSetConfigs[0]
		}},
		{"filter", func(e *Experiment) { e.EvalConf.EvalSetConfigs[0].ItemFilter.FilterFields[0].Values[0] = "changed" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := selectionExperiment()
			tc.mutate(e)
			got, err := HookSelectionConfigFingerprint(e)
			require.NoError(t, err)
			require.NotEqual(t, base, got)
		})
	}
	e := selectionExperiment()
	e.Name = "renamed"
	e.Description = "changed"
	e.CreatedBy = "other"
	e.SourceID = "template"
	e.SourceType = SourceType_Workflow
	e.TargetID = 91
	e.TargetVersionID = 92
	e.EvalConf.ItemConcurNum = gptr.Of(9)
	e.EvalConf.ItemRetryNum = gptr.Of(3)
	e.EvalConf.Ext = map[string]string{"credential": "do-not-hash"}
	e.EvalConf.EvalSetConfigs[0].TargetConfs = []*ExptTargetConf{{TargetID: 91}}
	e.EvalConf.EvalSetConfigs[0].EvaluatorConfs = []*ExptEvaluatorConf{{EvaluatorID: 101}}
	got, err := HookSelectionConfigFingerprint(e)
	require.NoError(t, err)
	require.Equal(t, base, got)
	_, err = HookSelectionConfigFingerprint(nil)
	require.Error(t, err)
	e.EvalConf.EvalSetConfigs[0] = nil
	_, err = HookSelectionConfigFingerprint(e)
	require.Error(t, err)
	for _, version := range []int64{0, 71, 72} {
		e := &Experiment{EvalSetID: 71, EvalSetVersionID: version}
		first, err := HookSelectionConfigFingerprint(e)
		require.NoError(t, err)
		e.EvalConf = &EvaluationConfiguration{}
		second, err := HookSelectionConfigFingerprint(e)
		require.NoError(t, err)
		require.Equal(t, first, second)
	}
}

func TestHookSelectionSnapshotOwnsAndValidatesSeed(t *testing.T) {
	in := snapshotInput()
	in.Selection = &HookSelectionSeed{Version: 1, TrialRunItemCount: 1, HasExplicitItemIDs: true, ConfigFingerprint: strings.Repeat("a", 64)}
	frozen, err := NewHookRunSnapshot(in)
	require.NoError(t, err)
	in.Selection.TrialRunItemCount = 99
	out := frozen.Input()
	require.Equal(t, int64(1), out.Selection.TrialRunItemCount)
	out.Selection.ConfigFingerprint = "changed"
	require.Equal(t, strings.Repeat("a", 64), frozen.Input().Selection.ConfigFingerprint)
	for _, seed := range []*HookSelectionSeed{{Version: 0, ConfigFingerprint: strings.Repeat("a", 64)}, {Version: 2, ConfigFingerprint: strings.Repeat("a", 64)}, {Version: 1, ConfigFingerprint: "short"}, {Version: 1, ConfigFingerprint: strings.Repeat("A", 64)}, {Version: 1, HasExplicitItemIDs: true, ConfigFingerprint: strings.Repeat("a", 64)}} {
		in.Selection = seed
		_, err := NewHookRunSnapshot(in)
		require.Error(t, err)
	}
	in.Selection = &HookSelectionSeed{Version: 1, ConfigFingerprint: strings.Repeat("a", 64)}
	zero, err := NewHookRunSnapshot(in)
	require.NoError(t, err)
	require.NotNil(t, zero.Input().Selection)
	in.Selection = nil
	old, err := NewHookRunSnapshot(in)
	require.NoError(t, err)
	require.Nil(t, old.Input().Selection)
}
