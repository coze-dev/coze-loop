// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
)

const BuiltinResultSetTargetID = "builtin:result-set-evaluation"

// ResultSetRuntimePolicy is deployment configuration, never experiment input.
type ResultSetRuntimePolicy struct {
	Enabled                 bool     `json:"enabled"`
	SpaceIDs                []string `json:"space_ids"`
	Environments            []string `json:"environments"`
	SupportedBinaryVersions []string `json:"supported_binary_versions"`
}

func (p *ResultSetRuntimePolicy) Validate(spaceID int64, environment, binaryVersion string, isDevelopmentEnvironment bool) error {
	if p == nil || !p.Enabled || !isDevelopmentEnvironment || environment == "prod" || !slices.Contains(p.SpaceIDs, strconv.FormatInt(spaceID, 10)) || !slices.Contains(p.Environments, environment) {
		return fmt.Errorf("result_set is disabled for this environment or workspace")
	}
	if binaryVersion == "" || !slices.Contains(p.SupportedBinaryVersions, binaryVersion) {
		return fmt.Errorf("result_set requires an explicitly supported binary_version")
	}
	return nil
}

func (c *EvaluationConfiguration) IsResultSet() bool {
	return c != nil && c.RunModeConfig != nil && c.RunModeConfig.RunMode == RunModeResultSet
}

// ExecutionKind is derived from the stored mode; clients cannot relabel a run.
func (c *EvaluationConfiguration) ExecutionKind() string {
	if c.IsResultSet() {
		return RunModeResultSet
	}
	return "agent"
}

func ResultSetBinaryVersion(raw string) (string, error) {
	return parseResultSetBinaryVersion(raw, true)
}

func parseResultSetBinaryVersion(raw string, required bool) (string, error) {
	if !required && strings.TrimSpace(raw) == "" {
		return "", nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil || fields == nil {
		return "", fmt.Errorf("result_set runtime parameters must specify binary_version")
	}
	for key := range fields {
		if key != "binary_version" {
			return "", fmt.Errorf("result_set does not accept Agent runtime parameter %q", key)
		}
	}
	if _, exists := fields["binary_version"]; !required && !exists {
		return "", nil
	}
	var version string
	if err := json.Unmarshal(fields["binary_version"], &version); err != nil || version == "" {
		return "", fmt.Errorf("result_set runtime parameters must specify binary_version")
	}
	return version, nil
}

func (c *EvaluationConfiguration) ResultSetBinaryVersions() ([]string, error) {
	versions, err := c.resultSetBinaryVersions(true)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("result_set requires binary_version")
	}
	return versions, nil
}

func (c *EvaluationConfiguration) RequestedResultSetBinaryVersions() ([]string, error) {
	return c.resultSetBinaryVersions(false)
}

func (c *EvaluationConfiguration) resultSetBinaryVersions(required bool) ([]string, error) {
	var versions []string
	for _, field := range c.verificationRuntimeFields() {
		if field != nil && field.FieldName == consts.FieldAdapterBuiltinFieldNameRuntimeParam {
			v, err := parseResultSetBinaryVersion(field.Value, required)
			if err != nil {
				return nil, err
			}
			if v != "" {
				versions = append(versions, v)
			}
		}
	}
	for _, set := range c.EvalSetConfigs {
		if set == nil {
			continue
		}
		for _, target := range set.TargetConfs {
			if target == nil {
				continue
			}
			for key := range target.RuntimeParam {
				if key != consts.FieldAdapterBuiltinFieldNameRuntimeParam {
					return nil, fmt.Errorf("result_set does not accept Agent runtime field %q", key)
				}
			}
			if raw, ok := target.RuntimeParam[consts.FieldAdapterBuiltinFieldNameRuntimeParam]; ok {
				v, err := parseResultSetBinaryVersion(raw, required)
				if err != nil {
					return nil, err
				}
				if v != "" {
					versions = append(versions, v)
				}
			}
		}
	}
	return versions, nil
}

func (c *EvaluationConfiguration) FreezeResultSetBinaryVersion(version string) error {
	if version == "" {
		return fmt.Errorf("result_set cannot freeze an empty runtime version")
	}
	raw, _ := json.Marshal(map[string]string{"binary_version": version})
	if c.ConnectorConf.TargetConf == nil {
		c.ConnectorConf.TargetConf = &TargetConf{}
	}
	target := c.ConnectorConf.TargetConf
	if target.IngressConf == nil {
		target.IngressConf = &TargetIngressConf{}
	}
	if target.IngressConf.EvalSetAdapter == nil {
		target.IngressConf.EvalSetAdapter = &FieldAdapter{}
	}
	if target.IngressConf.CustomConf == nil {
		target.IngressConf.CustomConf = &FieldAdapter{}
	}
	found := false
	for _, field := range target.IngressConf.CustomConf.FieldConfs {
		if field != nil && field.FieldName == consts.FieldAdapterBuiltinFieldNameRuntimeParam {
			field.Value = string(raw)
			found = true
		}
	}
	if !found {
		target.IngressConf.CustomConf.FieldConfs = append(target.IngressConf.CustomConf.FieldConfs, &FieldConf{FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam, Value: string(raw)})
	}
	for _, set := range c.EvalSetConfigs {
		if set == nil {
			continue
		}
		if len(set.TargetConfs) == 0 {
			set.TargetConfs = []*ExptTargetConf{{}}
		}
		for i, binding := range set.TargetConfs {
			if binding == nil {
				binding = &ExptTargetConf{}
				set.TargetConfs[i] = binding
			}
			binding.RuntimeParam = map[string]string{consts.FieldAdapterBuiltinFieldNameRuntimeParam: string(raw)}
		}
	}
	return nil
}

func (p *CreateExptParam) PrepareResultSetTarget() error {
	if !p.ExptConf.IsResultSet() {
		return nil
	}
	if p.ExptType == ExptType_Online || p.ExptConf.VerificationConfig != nil || p.EvalSetSourceType != ExptEvalSetSourceType_MultiSetConfig {
		return fmt.Errorf("result_set requires an offline MultiSetConfig experiment without verification_config")
	}
	if len(p.EvaluatorVersionIds) > 0 {
		return fmt.Errorf("result_set reads evaluators from each item evaluator_conf")
	}
	cfg := p.ExptConf.RunModeConfig
	if cfg.MaxRunMinutes < 0 {
		return fmt.Errorf("max_run_minutes must not be negative")
	}
	if cfg.SuaMode != "" || cfg.SuaModelID != 0 || cfg.SuaModelName != "" || cfg.SuaGoal != "" || cfg.SuaPersona != "" || cfg.SuaBehavioralConstraints != "" || cfg.SuaPETemplate != "" || cfg.MaxTurns != 0 || cfg.SkillsMode != "" || len(cfg.Skills) != 0 {
		return fmt.Errorf("result_set only accepts max_run_minutes in run_mode_config")
	}
	if p.TargetSharedOption != nil && p.TargetSharedOption.Enabled() {
		return fmt.Errorf("result_set cannot use a shared Agent target")
	}
	for _, set := range p.EvalSetConfigs {
		if set == nil || set.EvalSetVersionID == 0 {
			return fmt.Errorf("result_set requires fixed evaluation set versions")
		}
		if len(set.EvaluatorConfs) > 0 {
			return fmt.Errorf("result_set reads evaluators from each item evaluator_conf")
		}
		if set.TargetSharedOption != nil && set.TargetSharedOption.Enabled() {
			return fmt.Errorf("result_set cannot use a shared Agent target")
		}
	}
	off := false
	p.ExptConf.EnableExtractTrajectory = &off
	if p.TargetVersionID != 0 || (p.TargetID != nil && *p.TargetID != 0) {
		return nil
	}
	if p.CreateEvalTargetParam == nil {
		p.CreateEvalTargetParam = &CreateEvalTargetParam{}
	}
	target := p.CreateEvalTargetParam
	if target.EvalTargetType != nil && *target.EvalTargetType != EvalTargetTypeSandboxAgent {
		return fmt.Errorf("result_set uses a system Runtime binding")
	}
	if target.SourceTargetID != nil && *target.SourceTargetID != "" && *target.SourceTargetID != BuiltinResultSetTargetID {
		return fmt.Errorf("result_set cannot select an Agent application")
	}
	if target.SandboxAgent != nil && !isResultSetEnvironment(target.SandboxAgent) {
		return fmt.Errorf("result_set requires an empty Linux dual-sandbox Runtime environment")
	}
	typeID, sourceID := EvalTargetTypeSandboxAgent, BuiltinResultSetTargetID
	target.EvalTargetType, target.SourceTargetID = &typeID, &sourceID
	target.SandboxAgent = &SandboxAgent{Name: "Result set evaluation", Type: SandboxAgentTypeSingleRunCLI, SandboxCountMode: SandboxCountModeDual}
	return nil
}

func isResultSetEnvironment(a *SandboxAgent) bool {
	return a != nil && a.SandboxCountMode == SandboxCountModeDual && a.ModelID == 0 && a.ModelName == "" && a.AgentSetupCmd == "" && a.AgentRunCmd == "" && len(a.Envs) == 0 && !a.EnableAnalysis && len(a.CustomFieldSchemas) == 0 && a.Image == ""
}

func ValidateResultSetTarget(target *EvalTarget, spaceID int64) error {
	if target == nil || target.SourceTargetID != BuiltinResultSetTargetID || target.SpaceID != spaceID || target.EvalTargetType != EvalTargetTypeSandboxAgent || target.EvalTargetVersion == nil || target.EvalTargetVersion.SpaceID != spaceID || target.EvalTargetVersion.SandboxAgent == nil || target.EvalTargetVersion.SandboxAgent.Type != SandboxAgentTypeSingleRunCLI || !isResultSetEnvironment(target.EvalTargetVersion.SandboxAgent) {
		return fmt.Errorf("result_set requires its system Runtime snapshot in the experiment workspace")
	}
	return nil
}
