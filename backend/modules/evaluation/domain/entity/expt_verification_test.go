// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/stretchr/testify/require"
)

func TestResolveVerificationConfig(t *testing.T) {
	for _, mode := range []VerificationMode{VerificationModeNopOnly, VerificationModeOracleOnly, VerificationModeF2P} {
		for _, topology := range []SandboxCountMode{"", SandboxCountModeSingle, SandboxCountModeDual, SandboxCountModeMacVMPlusSandbox, SandboxCountModeMacVMPlusSSH} {
			agent := &SandboxAgent{Name: "any application name", SandboxCountMode: topology}
			cfg := &VerificationConfig{Mode: mode}
			got, raw, err := ResolveVerificationConfig(cfg, agent, `{"binary_version":"pinned"}`)
			require.NoError(t, err)
			require.Equal(t, cfg, got)
			require.Equal(t, `{"binary_version":"pinned"}`, raw)
		}
	}
	cfg := &VerificationConfig{Mode: VerificationModeF2P}
	agent := &SandboxAgent{Name: "verify", SandboxCountMode: SandboxCountModeDual}
	for _, tc := range []struct {
		name       string
		config     *VerificationConfig
		agent      *SandboxAgent
		raw        string
		wantError  bool
		wantConfig *VerificationConfig
	}{
		{"ordinary", nil, &SandboxAgent{Name: "normal"}, "{}", false, nil},
		{"name alone is not a discriminator", nil, agent, "{}", false, nil},
		{"legacy", nil, agent, `{"verification":{"mode":"f2p"},"binary_version":"pinned"}`, false, cfg},
		{"same declaration", cfg, agent, `{"verification":{"mode":"f2p"}}`, false, cfg},
		{"conflict", cfg, agent, `{"verification":{"mode":"nop_only"}}`, true, nil},
		{"legacy wrong target", nil, &SandboxAgent{Name: "normal"}, `{"verification":{"mode":"f2p"}}`, true, nil},
		{"missing mode", &VerificationConfig{}, agent, "", true, nil},
		{"invalid mode", &VerificationConfig{Mode: "auto"}, agent, "", true, nil},
		{"invalid topology", cfg, &SandboxAgent{SandboxCountMode: "invalid"}, "", true, nil},
		{"missing snapshot", cfg, nil, "", true, nil},
		{"null runtime", cfg, agent, "null", true, nil},
		{"array runtime", cfg, agent, "[]", true, nil},
		{"invalid runtime", cfg, agent, "{", true, nil},
		{"ordinary runtime untouched", nil, agent, "{", false, nil},
		{"null legacy", nil, agent, `{"verification":null}`, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, raw, err := ResolveVerificationConfig(tc.config, tc.agent, tc.raw)
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantConfig, got)
			if got != nil {
				require.NotContains(t, raw, "verification")
			}
		})
	}
}

func TestVerificationStoredInExistingConfiguration(t *testing.T) {
	field := &FieldConf{FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam, Value: `{"verification":{"mode":"oracle_only"},"binary_version":"pinned"}`}
	conf := &EvaluationConfiguration{ConnectorConf: Connector{TargetConf: &TargetConf{
		IngressConf: &TargetIngressConf{CustomConf: &FieldAdapter{FieldConfs: []*FieldConf{field}}},
	}}}
	target := &EvalTarget{EvalTargetType: EvalTargetTypeSandboxAgent, EvalTargetVersion: &EvalTargetVersion{SandboxAgent: &SandboxAgent{Name: "Verify"}}}
	require.NoError(t, conf.NormalizeVerificationConfig(target))
	require.Equal(t, VerificationModeOracleOnly, conf.VerificationConfig.Mode)
	require.JSONEq(t, `{"binary_version":"pinned"}`, field.Value)
	encoded, err := json.Marshal(conf)
	require.NoError(t, err)
	var stored EvaluationConfiguration
	require.NoError(t, json.Unmarshal(encoded, &stored))
	require.Equal(t, conf.VerificationConfig, stored.VerificationConfig)
	stored.RunModeConfig = &RunModeConfig{}
	require.Error(t, stored.NormalizeVerificationConfig(target))
}

func TestPrepareVerificationTarget(t *testing.T) {
	cfg := &VerificationConfig{Mode: VerificationModeF2P}
	p := &CreateExptParam{ExptConf: &EvaluationConfiguration{VerificationConfig: cfg}}
	require.NoError(t, p.PrepareVerificationTarget())
	require.Equal(t, BuiltinVerificationTargetID, *p.CreateEvalTargetParam.SourceTargetID)
	require.Equal(t, EvalTargetTypeSandboxAgent, *p.CreateEvalTargetParam.EvalTargetType)
	require.Equal(t, SandboxCountModeDual, p.CreateEvalTargetParam.SandboxAgent.SandboxCountMode)
	snapshot := &SandboxAgent{Name: "caller name", SandboxCountMode: SandboxCountModeMacVMPlusSSH}
	p.CreateEvalTargetParam.SandboxAgent = snapshot
	require.NoError(t, p.PrepareVerificationTarget())
	require.Equal(t, "caller name", snapshot.Name)
	require.Equal(t, SandboxCountModeMacVMPlusSSH, p.CreateEvalTargetParam.SandboxAgent.SandboxCountMode)
	p.ExptType = ExptType_Online
	require.Error(t, p.PrepareVerificationTarget())
	p.ExptType = ExptType_Offline
	p.CreateEvalTargetParam.EvalTargetType = gptr.Of(EvalTargetTypeCozeBot)
	require.Error(t, p.PrepareVerificationTarget())
}

func TestVerificationMultiSetCannotOverrideExperimentMode(t *testing.T) {
	config := &EvaluationConfiguration{VerificationConfig: &VerificationConfig{Mode: VerificationModeF2P},
		EvalSetConfigs: []*EvalSetConfig{{TargetConfs: []*ExptTargetConf{{RuntimeParam: map[string]string{
			consts.FieldAdapterBuiltinFieldNameRuntimeParam: `{"verification":{"mode":"oracle_only"}}`,
		}}}}},
	}
	target := &EvalTarget{EvalTargetType: EvalTargetTypeSandboxAgent, EvalTargetVersion: &EvalTargetVersion{SandboxAgent: &SandboxAgent{Name: "Verify"}}}
	require.ErrorContains(t, config.NormalizeVerificationConfig(target), "conflicts")
	config.EvalSetConfigs[0].TargetConfs[0].RuntimeParam[consts.FieldAdapterBuiltinFieldNameRuntimeParam] = `{"verification":{"mode":"f2p"},"binary_version":"pinned"}`
	require.NoError(t, config.NormalizeVerificationConfig(target))
	require.JSONEq(t, `{"binary_version":"pinned"}`, config.EvalSetConfigs[0].TargetConfs[0].RuntimeParam[consts.FieldAdapterBuiltinFieldNameRuntimeParam])
}
