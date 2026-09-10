// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
)

type VerificationMode string

const (
	VerificationModeNopOnly    VerificationMode = "nop_only"
	VerificationModeOracleOnly VerificationMode = "oracle_only"
	VerificationModeF2P        VerificationMode = "f2p"
	// BuiltinVerificationTargetID identifies the platform-owned execution adapter,
	// not an application selected by the client. Execution is selected by config.
	BuiltinVerificationTargetID = "builtin:dataset-verification"
)

type VerificationConfig struct {
	Mode VerificationMode `json:"mode"`
}

func (c *VerificationConfig) Validate() error {
	if c == nil {
		return nil
	}
	switch c.Mode {
	case VerificationModeNopOnly, VerificationModeOracleOnly, VerificationModeF2P:
		return nil
	default:
		return fmt.Errorf("verification_config.mode must be nop_only, oracle_only or f2p")
	}
}

// ResolveVerificationConfig is the only compatibility boundary for the old
// runtime_param.verification contract. New consumers use its typed result.
// A name alone never enables verification. Conflicting declarations fail closed.
func ResolveVerificationConfig(config *VerificationConfig, agent *SandboxAgent, raw string) (*VerificationConfig, string, error) {
	if err := config.Validate(); err != nil {
		return config, raw, err
	}
	var payload map[string]json.RawMessage
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &payload); err != nil || payload == nil {
			if config != nil {
				return config, raw, fmt.Errorf("verification runtime parameters must be a JSON object")
			}
			return nil, raw, nil
		}
	}
	value, hasLegacy := payload["verification"]
	if hasLegacy {
		legacy := new(VerificationConfig)
		if err := json.Unmarshal(value, legacy); err != nil {
			return legacy, raw, fmt.Errorf("invalid legacy verification config: %w", err)
		}
		if err := legacy.Validate(); err != nil {
			return legacy, raw, err
		}
		if config == nil {
			if agent == nil || !strings.EqualFold(strings.TrimSpace(agent.Name), "verify") {
				return legacy, raw, fmt.Errorf("legacy verification requires the original Verify target")
			}
			config = legacy
		} else if config.Mode != legacy.Mode {
			return config, raw, fmt.Errorf("verification_config conflicts with legacy runtime_param.verification")
		}
	}
	if config == nil {
		return nil, raw, nil
	}
	if agent == nil {
		return config, raw, fmt.Errorf("verification_config requires a SandboxAgent execution target")
	}
	switch agent.SandboxCountMode {
	case "", SandboxCountModeSingle, SandboxCountModeDual, SandboxCountModeMacVMPlusSandbox, SandboxCountModeMacVMPlusSSH:
	default:
		return config, raw, fmt.Errorf("unsupported verification sandbox_count_mode %q", agent.SandboxCountMode)
	}
	if hasLegacy {
		delete(payload, "verification")
		cleaned, err := json.Marshal(payload)
		if err != nil {
			return config, raw, err
		}
		raw = string(cleaned)
	}
	return config, raw, nil
}

func VerificationAgent(target *EvalTarget) *SandboxAgent {
	if target == nil || target.EvalTargetType != EvalTargetTypeSandboxAgent || target.EvalTargetVersion == nil {
		return nil
	}
	return target.EvalTargetVersion.SandboxAgent
}

func (c *EvaluationConfiguration) ResolveVerificationConfig(target *EvalTarget) (*VerificationConfig, string, error) {
	if c == nil {
		return nil, "", nil
	}
	raw := ""
	for _, field := range c.verificationRuntimeFields() {
		if field != nil && field.FieldName == consts.FieldAdapterBuiltinFieldNameRuntimeParam {
			raw = field.Value
		}
	}
	config, cleaned, err := ResolveVerificationConfig(c.VerificationConfig, VerificationAgent(target), raw)
	if err != nil {
		return config, raw, err
	}
	for _, set := range c.EvalSetConfigs {
		if set == nil {
			continue
		}
		for _, binding := range set.TargetConfs {
			if binding == nil {
				continue
			}
			value, exists := binding.RuntimeParam[consts.FieldAdapterBuiltinFieldNameRuntimeParam]
			if !exists {
				continue
			}
			config, _, err = ResolveVerificationConfig(config, VerificationAgent(target), value)
			if err != nil {
				return config, raw, err
			}
		}
	}
	if config != nil && c.RunModeConfig != nil {
		return config, raw, fmt.Errorf("verification_config and run_mode_config are mutually exclusive")
	}
	return config, cleaned, nil
}

func (c *EvaluationConfiguration) NormalizeVerificationConfig(target *EvalTarget) error {
	config, raw, err := c.ResolveVerificationConfig(target)
	if err != nil || c == nil {
		return err
	}
	if config != nil && c.RunModeConfig != nil {
		return fmt.Errorf("verification_config and run_mode_config are mutually exclusive")
	}
	c.VerificationConfig = config
	for _, set := range c.EvalSetConfigs {
		if set == nil {
			continue
		}
		for _, binding := range set.TargetConfs {
			if binding == nil {
				continue
			}
			value, exists := binding.RuntimeParam[consts.FieldAdapterBuiltinFieldNameRuntimeParam]
			if !exists {
				continue
			}
			_, cleaned, err := ResolveVerificationConfig(config, VerificationAgent(target), value)
			if err != nil {
				return err
			}
			binding.RuntimeParam[consts.FieldAdapterBuiltinFieldNameRuntimeParam] = cleaned
		}
	}
	for _, field := range c.verificationRuntimeFields() {
		if field != nil && field.FieldName == consts.FieldAdapterBuiltinFieldNameRuntimeParam {
			field.Value = raw
		}
	}
	return nil
}

func (c *EvaluationConfiguration) verificationRuntimeFields() []*FieldConf {
	if c == nil || c.ConnectorConf.TargetConf == nil || c.ConnectorConf.TargetConf.IngressConf == nil || c.ConnectorConf.TargetConf.IngressConf.CustomConf == nil {
		return nil
	}
	return c.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs
}

// PrepareVerificationTarget fills platform-owned adapter metadata. Callers only
// provide verification_config and an optional SandboxAgent environment snapshot.
func (p *CreateExptParam) PrepareVerificationTarget() error {
	if p.ExptConf == nil || p.ExptConf.VerificationConfig == nil {
		return nil
	}
	if err := p.ExptConf.VerificationConfig.Validate(); err != nil {
		return err
	}
	if p.ExptType == ExptType_Online || p.ExptConf.RunModeConfig != nil {
		return fmt.Errorf("verification_config requires an offline experiment without run_mode_config")
	}
	if p.TargetVersionID != 0 || (p.TargetID != nil && *p.TargetID != 0) {
		return nil // Existing snapshots are validated after loading, never renamed.
	}
	if p.CreateEvalTargetParam == nil {
		p.CreateEvalTargetParam = &CreateEvalTargetParam{}
	}
	target := p.CreateEvalTargetParam
	if target.EvalTargetType != nil && *target.EvalTargetType != EvalTargetTypeSandboxAgent {
		return fmt.Errorf("verification_config requires a SandboxAgent execution target")
	}
	if target.SourceTargetID != nil && *target.SourceTargetID != "" && *target.SourceTargetID != BuiltinVerificationTargetID {
		return nil // Registered applications still resolve through normal loading.
	}
	if target.SandboxAgent == nil {
		target.SandboxAgent = &SandboxAgent{SandboxCountMode: SandboxCountModeDual}
	} else {
		snapshot := *target.SandboxAgent
		target.SandboxAgent = &snapshot
	}
	if _, _, err := ResolveVerificationConfig(p.ExptConf.VerificationConfig, target.SandboxAgent, ""); err != nil {
		return err
	}
	targetType := EvalTargetTypeSandboxAgent
	sourceID := BuiltinVerificationTargetID
	target.EvalTargetType = &targetType
	target.SourceTargetID = &sourceID
	target.SandboxAgent.Name = "Dataset verification"
	target.SandboxAgent.Type = SandboxAgentTypeSingleRunCLI
	return nil
}
