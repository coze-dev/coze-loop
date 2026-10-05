// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"math"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/stretchr/testify/require"
)

func executionSnapshotInput() HookRunSnapshotInput {
	in := snapshotInput()
	in.Context.Experiment.Type = gptr.Of("offline")
	in.Context.Target = &spi.HookTargetRef{ID: gptr.Of("7"), VersionID: gptr.Of("8"), Type: gptr.Of(EvalTargetTypeSandboxAgent.String())}
	in.Context.EvalSets = []*spi.HookEvalSetRef{{WorkspaceID: gptr.Of("1"), ID: gptr.Of("11")}, {WorkspaceID: gptr.Of("21"), ID: gptr.Of("22"), VersionID: gptr.Of("23")}}
	in.Execution = &HookExecutionSnapshot{Version: 1, Key: in.Key, ExecutionScope: in.ExecutionScope, Mode: EvaluationModeSubmit,
		Target:        HookExecutionTarget{ID: 7, VersionID: 8, Type: EvalTargetTypeSandboxAgent, SourceSpaceID: 9, Config: &TargetConf{TargetVersionID: 8, IngressConf: &TargetIngressConf{EvalSetAdapter: &FieldAdapter{FieldConfs: []*FieldConf{{FieldName: "question", FromField: "source"}}}}}},
		RunModeConfig: &RunModeConfig{MaxRunMinutes: 7, Skills: []*AgentSkillDeclare{{SkillKey: "tool", CredentialsKeys: []string{"credential-name"}, Dist: &SkillDistDeclare{CommitHash: "pinned"}}}},
		Sets: []HookExecutionSet{
			{EvalSetID: 11, EvalSetVersionID: 11, ItemFilter: &ExptItemFilter{FilterFields: []*ExptItemFilterField{{Values: []string{"selected"}}}}, ItemConfig: &ExptItemConfig{EvalTargetConf: &ItemTargetConf{TargetVersionID: 999, DynamicConf: map[string]string{"private": "first"}, RunConf: &ItemRunConf{MaxRunMinutes: 7}}, EvaluatorConfs: []*ItemEvaluatorConf{{EvaluatorVersionID: 71, ScoreWeight: gptr.Of(0.5), FromEvalSet: []*FieldConf{{FromField: "q"}}}}}},
			{EvalSetID: 22, EvalSetVersionID: 23, SourceSpaceID: 21, ItemConfig: &ExptItemConfig{EvalSetSourceSpaceID: 21}},
		}}
	return in
}

func TestHookExecutionSnapshotOwnsNestedConfiguration(t *testing.T) {
	in := executionSnapshotInput()
	s, err := NewHookRunSnapshot(in)
	require.NoError(t, err)
	in.Execution.Target.Config.IngressConf.EvalSetAdapter.FieldConfs[0].FromField = "changed"
	in.Execution.RunModeConfig.Skills[0].Dist.CommitHash = "changed"
	in.Execution.RunModeConfig.Skills[0].CredentialsKeys[0] = "changed"
	in.Execution.Sets[0].ItemConfig.EvalTargetConf.DynamicConf["private"] = "changed"
	in.Execution.Sets[0].ItemConfig.EvaluatorConfs[0].FromEvalSet[0].FromField = "changed"
	*in.Execution.Sets[0].ItemConfig.EvaluatorConfs[0].ScoreWeight = 9
	in.Execution.Sets[0].ItemFilter.FilterFields[0].Values[0] = "changed"
	out := s.Input()
	require.Equal(t, "source", out.Execution.Target.Config.IngressConf.EvalSetAdapter.FieldConfs[0].FromField)
	require.Equal(t, "pinned", out.Execution.RunModeConfig.Skills[0].Dist.CommitHash)
	require.Equal(t, []string{"credential-name"}, out.Execution.RunModeConfig.Skills[0].CredentialsKeys)
	require.Equal(t, "first", out.Execution.Sets[0].ItemConfig.EvalTargetConf.DynamicConf["private"])
	require.Equal(t, "q", out.Execution.Sets[0].ItemConfig.EvaluatorConfs[0].FromEvalSet[0].FromField)
	require.Equal(t, 0.5, *out.Execution.Sets[0].ItemConfig.EvaluatorConfs[0].ScoreWeight)
	require.Equal(t, []string{"selected"}, out.Execution.Sets[0].ItemFilter.FilterFields[0].Values)
	out.Execution.Sets[0].ItemConfig.EvalTargetConf.DynamicConf["private"] = "output mutation"
	out.Execution.Sets[0] = HookExecutionSet{}
	require.Equal(t, "first", s.Input().Execution.Sets[0].ItemConfig.EvalTargetConf.DynamicConf["private"])
	// GLOBAL target fallback remains distinct from the set's dataset-space zero.
	item := &ExptItemEvalCtx{Expt: &Experiment{TargetSpaceID: out.Execution.Target.SourceSpaceID, EvalSetSpaceID: 99}, ItemConfig: s.Input().Execution.Sets[0].ItemConfig}
	require.Zero(t, item.EvalSetSourceSpaceID())
	require.Equal(t, int64(9), item.TargetSourceSpaceID())
}

func TestHookExecutionSnapshotRejectsMismatchedBindings(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*HookRunSnapshotInput)
	}{
		{"version", func(in *HookRunSnapshotInput) { in.Execution.Version = 2 }},
		{"run", func(in *HookRunSnapshotInput) { in.Execution.Key.RunID++ }},
		{"scope", func(in *HookRunSnapshotInput) { in.Execution.ExecutionScope = "another" }},
		{"mode", func(in *HookRunSnapshotInput) { in.Execution.Mode = EvaluationModeTrialRun }},
		{"retry", func(in *HookRunSnapshotInput) { in.Execution.Mode = EvaluationModeRetryAll }},
		{"online", func(in *HookRunSnapshotInput) { in.Context.Experiment.Type = gptr.Of("online") }},
		{"target", func(in *HookRunSnapshotInput) { in.Execution.Target.ID++ }},
		{"target-version", func(in *HookRunSnapshotInput) { in.Execution.Target.VersionID++ }},
		{"global-config-version", func(in *HookRunSnapshotInput) { in.Execution.Target.Config.TargetVersionID++ }},
		{"live-quota", func(in *HookRunSnapshotInput) {
			in.Execution.Sets[0].ItemConfig.ExpectedQuotaConsumption = &ExpectedQuotaConsumption{}
		}},
		{"target-type", func(in *HookRunSnapshotInput) { in.Execution.Target.Type = EvalTargetTypeLoopPrompt }},
		{"target-source", func(in *HookRunSnapshotInput) { in.Execution.Target.SourceSpaceID = -1 }},
		{"target-absent", func(in *HookRunSnapshotInput) { in.Context.Target = nil }},
		{"set-order", func(in *HookRunSnapshotInput) {
			in.Execution.Sets[0], in.Execution.Sets[1] = in.Execution.Sets[1], in.Execution.Sets[0]
		}},
		{"set-version", func(in *HookRunSnapshotInput) { in.Execution.Sets[1].EvalSetVersionID++ }},
		{"set-source", func(in *HookRunSnapshotInput) { in.Execution.Sets[1].SourceSpaceID++ }},
		{"config-source", func(in *HookRunSnapshotInput) { in.Execution.Sets[1].ItemConfig.EvalSetSourceSpaceID++ }},
		{"config-missing", func(in *HookRunSnapshotInput) { in.Execution.Sets[0].ItemConfig = nil }},
		{"empty-sets", func(in *HookRunSnapshotInput) { in.Execution.Sets = nil }},
		{"nan-config", func(in *HookRunSnapshotInput) {
			in.Execution.Sets[0].ItemConfig.EvaluatorConfs[0].ScoreWeight = gptr.Of(math.NaN())
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := executionSnapshotInput()
			tc.mutate(&in)
			s, err := NewHookRunSnapshot(in)
			require.Error(t, err)
			require.Nil(t, s)
		})
	}
}

func TestHookExecutionSnapshotLegacyOptionalAndNoTarget(t *testing.T) {
	in := snapshotInput()
	s, err := NewHookRunSnapshot(in)
	require.NoError(t, err)
	require.Nil(t, s.Input().Execution)
	in = executionSnapshotInput()
	in.Context.Target = nil
	in.Execution.Target = HookExecutionTarget{}
	in.Execution.Mode = EvaluationModeTrialRun
	in.Context.RunMode = gptr.Of("trial_run")
	s, err = NewHookRunSnapshot(in)
	require.NoError(t, err)
	require.Equal(t, EvaluationModeTrialRun, s.Input().Execution.Mode)
}
