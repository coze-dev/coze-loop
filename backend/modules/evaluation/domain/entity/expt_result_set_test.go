// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestResultSetRunModeDoesNotFallBackToAgent(t *testing.T) {
	if !IsValidRunMode("result_set") || RunModeToInt("result_set") != 6 {
		t.Fatal("result_set must map explicitly to private mode 6")
	}
	for mode, want := range map[string]int{"single_turn": 1, "fixed_script_multi_turn": 2, "sua_loop_multi_turn": 3, "sua_human_loop_multi_turn": 4, "goal": 5} {
		if got := RunModeToInt(mode); got != want {
			t.Fatalf("%s: got %d, want %d", mode, got, want)
		}
	}
}

func TestResultSetRuntimePolicyFailsClosed(t *testing.T) {
	p := &ResultSetRuntimePolicy{Enabled: true, SpaceIDs: []string{"1001"}, Environments: []string{"boe_result_set"}, SupportedBinaryVersions: []string{"v0.0.110-result-set.1"}}
	require.NoError(t, p.Validate(1001, "boe_result_set", "v0.0.110-result-set.1", true))
	for _, c := range []struct {
		space        int64
		env, version string
		boe          bool
	}{
		{1002, "boe_result_set", "v0.0.110-result-set.1", true}, {1001, "prod", "v0.0.110-result-set.1", true},
		{1001, "boe_result_set", "", true}, {1001, "boe_result_set", "v0.0.109", true}, {1001, "boe_result_set", "v0.0.110-result-set.1", false},
	} {
		require.Error(t, p.Validate(c.space, c.env, c.version, c.boe))
	}
	require.Error(t, (*ResultSetRuntimePolicy)(nil).Validate(1001, "boe_result_set", "v0.0.110-result-set.1", true))
	p.Enabled = false
	require.Error(t, p.Validate(1001, "boe_result_set", "v0.0.110-result-set.1", true))
}

func TestResultSetSystemBindingAndSnapshotIsolation(t *testing.T) {
	makeParam := func() *CreateExptParam {
		return &CreateExptParam{WorkspaceID: 1001, EvalSetSourceType: ExptEvalSetSourceType_MultiSetConfig, EvalSetConfigs: []*EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 11}}, ExptConf: &EvaluationConfiguration{RunModeConfig: &RunModeConfig{RunMode: RunModeResultSet}, EnableExtractTrajectory: gptr.Of(true)}}
	}
	p := makeParam()
	require.NoError(t, p.PrepareResultSetTarget())
	require.Equal(t, BuiltinResultSetTargetID, *p.CreateEvalTargetParam.SourceTargetID)
	require.False(t, *p.ExptConf.EnableExtractTrajectory)
	target := &EvalTarget{SpaceID: 1001, SourceTargetID: BuiltinResultSetTargetID, EvalTargetType: EvalTargetTypeSandboxAgent, EvalTargetVersion: &EvalTargetVersion{SpaceID: 1001, SandboxAgent: p.CreateEvalTargetParam.SandboxAgent}}
	require.NoError(t, ValidateResultSetTarget(target, 1001))
	require.Error(t, ValidateResultSetTarget(target, 1002))
	target.SourceTargetID = "registered-agent"
	require.Error(t, ValidateResultSetTarget(target, 1001))
	target.SourceTargetID = BuiltinResultSetTargetID
	target.EvalTargetVersion.SandboxAgent.ModelID = 42
	require.Error(t, ValidateResultSetTarget(target, 1001))
	for _, mutate := range []func(*CreateExptParam){
		func(p *CreateExptParam) { p.ExptConf.RunModeConfig.SuaMode = SuaModeLoop },
		func(p *CreateExptParam) { p.ExptConf.RunModeConfig.MaxTurns = 4 },
		func(p *CreateExptParam) {
			p.CreateEvalTargetParam = &CreateEvalTargetParam{SourceTargetID: gptr.Of("123")}
		},
		func(p *CreateExptParam) { p.EvalSetConfigs[0].EvalSetVersionID = 0 },
	} {
		p := makeParam()
		mutate(p)
		require.Error(t, p.PrepareResultSetTarget())
	}
}

func TestResultSetRuntimeParameterRejectsAgentSecrets(t *testing.T) {
	got, err := ResultSetBinaryVersion(`{"binary_version":"fixed-rc"}`)
	require.NoError(t, err)
	require.Equal(t, "fixed-rc", got)
	for _, raw := range []string{"", "null", "{}", `{"binary_version":123}`, `{"binary_version":""}`, `{"binary_version":"fixed-rc","api_key":"secret"}`, `{"binary_version":"fixed-rc","verification":{}}`} {
		_, err := ResultSetBinaryVersion(raw)
		require.Error(t, err, raw)
	}
}

func TestResultSetApplicationSnapshotRequiresTrustedIdentityAndProvenance(t *testing.T) {
	target := &EvalTarget{SpaceID: 1001, SourceTargetID: "101", EvalTargetType: EvalTargetTypeSandboxAgent, EvalTargetVersion: &EvalTargetVersion{SpaceID: 1001, SourceTargetVersion: "result-set-app:2002:99", SandboxAgent: NewResultSetApplicationSnapshot("renamed")}}
	require.NoError(t, ValidateResultSetTarget(target, 1001, "101"))
	require.Error(t, ValidateResultSetTarget(target, 1001))
	require.Error(t, ValidateResultSetTarget(target, 2002, "101"))
	target.EvalTargetVersion.SandboxAgent.Envs = []*SandboxEnvVar{{Key: "agent_id", Value: "result-set-evaluation"}}
	require.Error(t, ValidateResultSetTarget(target, 1001, "101"))
	target.EvalTargetVersion.SandboxAgent.Envs = nil
	target.EvalTargetVersion.SourceTargetVersion = "user-version"
	require.Error(t, ValidateResultSetTarget(target, 1001, "101"))
}

func TestResultSetApplicationProvenanceRejectsInlineFingerprints(t *testing.T) {
	require.Equal(t, int64(2002), ResultSetApplicationSourceSpace("result-set-app:2002:99"))
	for _, version := range []string{"result-set-app:2002:99#fake", "result-set-app:0:99", "result-set-app:2002:0", "result-set-app:2002:unknown"} {
		require.Zero(t, ResultSetApplicationSourceSpace(version))
	}
}
