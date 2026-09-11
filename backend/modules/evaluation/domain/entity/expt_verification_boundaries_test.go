// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/stretchr/testify/require"
)

func TestExptTemplateConfigurationVerificationValidation(t *testing.T) {
	for _, mode := range []VerificationMode{VerificationModeNopOnly, VerificationModeOracleOnly, VerificationModeF2P, "unknown"} {
		t.Run(string(mode), func(t *testing.T) {
			conf := &ExptTemplateConfiguration{VerificationConfig: &VerificationConfig{Mode: mode}}
			err := conf.Valid(context.Background())
			if mode == "unknown" {
				require.ErrorContains(t, err, "verification_config.mode")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVerificationConfigurationOptionalBoundaries(t *testing.T) {
	var absent *EvaluationConfiguration
	config, raw, err := absent.ResolveVerificationConfig(nil)
	require.NoError(t, err)
	require.Nil(t, config)
	require.Empty(t, raw)
	require.NoError(t, absent.NormalizeVerificationConfig(nil))
	for _, target := range []*EvalTarget{nil, {}, {EvalTargetType: EvalTargetTypeSandboxAgent}, {EvalTargetType: EvalTargetTypeCozeBot, EvalTargetVersion: &EvalTargetVersion{SandboxAgent: &SandboxAgent{}}}} {
		require.Nil(t, VerificationAgent(target))
	}
	target := &EvalTarget{EvalTargetType: EvalTargetTypeSandboxAgent, EvalTargetVersion: &EvalTargetVersion{SandboxAgent: &SandboxAgent{Name: "Verify"}}}
	field := &FieldConf{FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam, Value: `{"binary_version":"pinned"}`}
	other := &FieldConf{FieldName: "other", Value: "unchanged"}
	binding := &ExptTargetConf{RuntimeParam: map[string]string{consts.FieldAdapterBuiltinFieldNameRuntimeParam: `{"verification":{"mode":"f2p"},"item":"value"}`}}
	c := &EvaluationConfiguration{
		VerificationConfig: &VerificationConfig{Mode: VerificationModeF2P},
		ConnectorConf:      Connector{TargetConf: &TargetConf{IngressConf: &TargetIngressConf{CustomConf: &FieldAdapter{FieldConfs: []*FieldConf{nil, other, field}}}}},
		EvalSetConfigs:     []*EvalSetConfig{nil, {TargetConfs: []*ExptTargetConf{nil, {}, binding}}},
	}
	require.NoError(t, c.NormalizeVerificationConfig(target))
	require.Equal(t, VerificationModeF2P, c.VerificationConfig.Mode)
	require.JSONEq(t, `{"item":"value"}`, binding.RuntimeParam[consts.FieldAdapterBuiltinFieldNameRuntimeParam])
	require.JSONEq(t, `{"binary_version":"pinned"}`, field.Value)
	require.Equal(t, "unchanged", other.Value)
	for _, raw := range []string{`{"verification":[]}`, `{"verification":"f2p"}`, `{"verification":{"mode":"auto"}}`} {
		_, _, err := ResolveVerificationConfig(nil, target.EvalTargetVersion.SandboxAgent, raw)
		require.Error(t, err)
	}
}

func TestPrepareVerificationTargetBoundaries(t *testing.T) {
	for _, p := range []*CreateExptParam{{}, {ExptConf: &EvaluationConfiguration{}}} {
		require.NoError(t, p.PrepareVerificationTarget())
		require.Nil(t, p.CreateEvalTargetParam)
	}
	for _, tc := range []struct {
		name      string
		mutate    func(*CreateExptParam)
		wantError bool
	}{
		{"invalid mode", func(p *CreateExptParam) { p.ExptConf.VerificationConfig.Mode = "invalid" }, true},
		{"run mode conflict", func(p *CreateExptParam) { p.ExptConf.RunModeConfig = &RunModeConfig{} }, true},
		{"existing target", func(p *CreateExptParam) { p.TargetID = gptr.Of(int64(123)) }, false},
		{"existing version", func(p *CreateExptParam) { p.TargetVersionID = 456 }, false},
		{"registered application", func(p *CreateExptParam) { p.CreateEvalTargetParam.SourceTargetID = gptr.Of("registered") }, false},
		{"invalid topology", func(p *CreateExptParam) { p.CreateEvalTargetParam.SandboxAgent.SandboxCountMode = "unknown" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := &SandboxAgent{Name: "unchanged", SandboxCountMode: SandboxCountModeSingle}
			p := &CreateExptParam{ExptConf: &EvaluationConfiguration{VerificationConfig: &VerificationConfig{Mode: VerificationModeF2P}}, CreateEvalTargetParam: &CreateEvalTargetParam{SandboxAgent: snapshot}}
			tc.mutate(p)
			err := p.PrepareVerificationTarget()
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, "unchanged", snapshot.Name)
		})
	}
}
