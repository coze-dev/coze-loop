// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
)

// HookExecutionSnapshot is private recovery data, never SPI business content.
type HookExecutionSnapshot struct {
	Version                 int                             `json:"version"`
	Key                     HookRunKey                      `json:"run_key"`
	ExecutionScope          string                          `json:"execution_scope"`
	Mode                    ExptRunMode                     `json:"mode"`
	SingleSet               bool                            `json:"single_set,omitempty"`
	Target                  HookExecutionTarget             `json:"target"`
	RunModeConfig           *RunModeConfig                  `json:"run_mode_config,omitempty"`
	VerificationConfig      *VerificationConfig             `json:"verification_config,omitempty"`
	EnableExtractTrajectory *bool                           `json:"enable_extract_trajectory,omitempty"`
	SkillTOSKeys            map[string]string               `json:"skill_tos_keys,omitempty"`
	EvaluatorFallback       *HookExecutionEvaluatorFallback `json:"evaluator_fallback,omitempty"`
	Sets                    []HookExecutionSet              `json:"sets"`
}

// Presence distinguishes frozen-empty from legacy snapshots that never captured the fallback.
type HookExecutionEvaluatorFallback struct {
	Confs             []*EvaluatorConf `json:"confs"`
	EnableScoreWeight bool             `json:"enable_score_weight"`
}

func HookBoundExecutionMode(mode ExptRunMode) bool {
	return mode == EvaluationModeSubmit || mode == EvaluationModeTrialRun || mode == EvaluationModeFailRetry || mode == EvaluationModeRetryAll || mode == EvaluationModeRetryItems || mode == EvaluationModeAppend
}

func HookBoundRetryMode(mode ExptRunMode) bool {
	return mode == EvaluationModeFailRetry || mode == EvaluationModeRetryAll || mode == EvaluationModeRetryItems
}

// Preserve opaque JSON number tokens in both owned clones and nested payload reads.
func (d *HookExecutionSnapshot) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return errors.New("invalid hook execution JSON object")
	}
	type executionJSON HookExecutionSnapshot
	var decoded executionJSON
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("invalid trailing hook execution JSON")
	}
	*d = HookExecutionSnapshot(decoded)
	return nil
}

type HookExecutionTarget struct {
	ID            int64          `json:"id"`
	VersionID     int64          `json:"version_id"`
	Type          EvalTargetType `json:"type"`
	SourceSpaceID int64          `json:"source_space_id"`
	Config        *TargetConf    `json:"config,omitempty"`
}

type HookExecutionSet struct {
	EvalSetID        int64           `json:"eval_set_id"`
	EvalSetVersionID int64           `json:"eval_set_version_id"`
	SourceSpaceID    int64           `json:"source_space_id"`
	ItemFilter       *ExptItemFilter `json:"item_filter,omitempty"`
	ItemConfig       *ExptItemConfig `json:"item_config"`
}

func cloneHookExecutionSnapshot(in *HookExecutionSnapshot) (*HookExecutionSnapshot, error) {
	if in == nil {
		return nil, nil
	}
	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out HookExecutionSnapshot
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func validateHookExecutionSnapshot(in HookRunSnapshotInput) error {
	d := in.Execution
	invalid := errors.New("invalid frozen hook execution description")
	if d == nil {
		return nil
	}
	online := d.Mode == EvaluationModeAppend && in.Context.GetExperiment().GetType() == "online"
	if d.Version != 1 || d.Key != in.Key || d.ExecutionScope != in.ExecutionScope || (!online && (in.Context.GetExperiment().GetType() != "offline" || len(d.Sets) == 0)) || len(d.Sets) != len(in.Context.EvalSets) {
		return invalid
	}
	mode := map[ExptRunMode]string{EvaluationModeSubmit: "submit", EvaluationModeTrialRun: "trial_run", EvaluationModeFailRetry: "fail_retry", EvaluationModeRetryAll: "retry_all", EvaluationModeRetryItems: "retry_items"}[d.Mode]
	if online {
		mode = "append"
	}
	if mode == "" {
		return invalid
	}
	if online && !d.SingleSet || !online && d.SingleSet && (!HookBoundRetryMode(d.Mode) || len(d.Sets) != 1) {
		return invalid
	}
	if mode != in.Context.GetRunMode() {
		return invalid
	}
	target, ref := d.Target, in.Context.Target
	if target.ID < 0 || target.VersionID < 0 || target.SourceSpaceID < 0 || (target.Config != nil && target.Config.TargetVersionID != target.VersionID) {
		return invalid
	}
	if target.ID == 0 {
		if target.VersionID != 0 || ref != nil {
			return invalid
		}
	} else if ref == nil || ref.GetID() != strconv.FormatInt(target.ID, 10) || ref.GetType() != target.Type.String() || target.Type.String() == "<UNSET>" || !hookExecutionVersionMatches(target.VersionID, ref.VersionID) {
		return invalid
	}
	for i, set := range d.Sets {
		ref := in.Context.EvalSets[i]
		if set.EvalSetID <= 0 || set.EvalSetVersionID < 0 || set.SourceSpaceID < 0 || set.ItemConfig == nil || set.ItemConfig.EvalSetSourceSpaceID != set.SourceSpaceID || set.ItemConfig.TargetSourceSpaceID < 0 || set.ItemConfig.ExpectedQuotaConsumption != nil || ref == nil {
			return invalid
		}
		space := set.SourceSpaceID
		if space == 0 {
			space = in.Key.WorkspaceID
		}
		version := set.EvalSetVersionID
		if version == set.EvalSetID {
			version = 0 // Preserve the stored draft sentinel; Context omits its version.
		}
		if ref.GetID() != strconv.FormatInt(set.EvalSetID, 10) || ref.GetWorkspaceID() != strconv.FormatInt(space, 10) || !hookExecutionVersionMatches(version, ref.VersionID) {
			return invalid
		}
	}
	return nil
}

func hookExecutionVersionMatches(version int64, ref *string) bool {
	if version == 0 {
		return ref == nil
	}
	return ref != nil && *ref == strconv.FormatInt(version, 10)
}
