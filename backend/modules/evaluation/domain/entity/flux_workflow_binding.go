// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"strconv"
)

// FluxWorkflowBinding pins a Workflow source to an experiment. Large YAML lives in private object storage.
type FluxWorkflowBinding struct {
	ExperimentID          int64           `json:"experiment_id,omitempty"`
	SpaceID               int64           `json:"space_id,omitempty"`
	SourceKind            string          `json:"source_kind,omitempty"`
	SourceSpaceID         string          `json:"source_space_id,omitempty"`
	ConsumerSpaceID       string          `json:"consumer_space_id,omitempty"`
	SourceWorkflowID      string          `json:"source_workflow_id,omitempty"`
	SourceRevision        int64           `json:"source_revision,omitempty"`
	SourceDigest          string          `json:"source_digest,omitempty"`
	SourceTemplateID      string          `json:"source_template_id,omitempty"`
	SourceSnapshotID      string          `json:"source_snapshot_id,omitempty"`
	SourceReleaseID       string          `json:"source_release_id,omitempty"`
	ExecutionSpaceID      string          `json:"execution_space_id,omitempty"`
	InlineYAMLRef         string          `json:"inline_yaml_ref,omitempty"`
	InlineYAMLSHA256      string          `json:"inline_yaml_sha256,omitempty"`
	EvalSpaceID           string          `json:"eval_space_id,omitempty"`
	EvalWorkflowID        string          `json:"eval_workflow_id,omitempty"`
	CopyTemplateID        string          `json:"copy_template_id,omitempty"`
	CopySnapshotID        string          `json:"copy_snapshot_id,omitempty"`
	CopyReleaseID         string          `json:"copy_release_id,omitempty"`
	CopyRevision          int64           `json:"copy_revision,omitempty"`
	CopyDigest            string          `json:"copy_digest,omitempty"`
	CopyYAMLRef           string          `json:"copy_yaml_ref,omitempty"`
	CopyYAMLSHA256        string          `json:"copy_yaml_sha256,omitempty"`
	PolicyDigest          string          `json:"policy_digest,omitempty"`
	ExecutionPolicyJSON   []byte          `json:"execution_policy_json,omitempty"`
	TransformVersion      string          `json:"transform_version,omitempty"`
	NodeMapJSON           []byte          `json:"node_map_json,omitempty"`
	AssetMapJSON          json.RawMessage `json:"asset_map_json,omitempty"`
	AssetMapSHA256        string          `json:"asset_map_sha256,omitempty"`
	BindingID             string          `json:"binding_id,omitempty"`
	EvalOperatorUsername  string          `json:"eval_operator_username,omitempty"`
	WorkerPostSetupURL    string          `json:"worker_post_setup_url,omitempty"`
	WorkerPostSetupSHA256 string          `json:"worker_post_setup_sha256,omitempty"`
	State                 string          `json:"state,omitempty"`
}

// FluxItemRun is the run state kept in eval_target_record.ext for one invoke_id.
type FluxItemRun struct {
	ExperimentID         int64  `json:"experiment_id,omitempty"`
	ExperimentSpaceID    int64  `json:"experiment_space_id,omitempty"`
	SpaceID              int64  `json:"space_id,omitempty"`
	InvokeID             int64  `json:"invoke_id,omitempty"`
	BindingID            string `json:"binding_id,omitempty"`
	QueryDigest          string `json:"query_digest,omitempty"`
	SourceKind           string `json:"source_kind,omitempty"`
	TemplateID           string `json:"template_id,omitempty"`
	IdempotencyKey       string `json:"idempotency_key,omitempty"`
	DeliveryID           string `json:"delivery_id,omitempty"`
	RequestID            string `json:"request_id,omitempty"`
	RunIdempotencyKey    string `json:"run_idempotency_key,omitempty"`
	RunOperationID       string `json:"run_operation_id,omitempty"`
	ExecutionSpecID      string `json:"execution_spec_id,omitempty"`
	AdmissionEventID     string `json:"admission_event_id,omitempty"`
	PlanDigest           string `json:"plan_digest,omitempty"`
	FrozenDefinitionsRef string `json:"frozen_definitions_ref,omitempty"`
	DeliveryYAMLKey      string `json:"delivery_yaml_key,omitempty"`
	LastEventCursor      string `json:"last_event_cursor,omitempty"`
	FinalDigest          string `json:"final_digest,omitempty"`
	FinalReceiptKey      string `json:"final_receipt_key,omitempty"`
	State                string `json:"state,omitempty"`
	Version              int64  `json:"version,omitempty"`
	rawFields            map[string]json.RawMessage
}

// UnmarshalJSON accepts the Runtime's string invoke_id and retains fields owned by its state machine.
func (r *FluxItemRun) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	decoded := make(map[string]json.RawMessage, len(fields))
	for k, v := range fields {
		decoded[k] = v
	}
	if raw := decoded["invoke_id"]; len(raw) != 0 && raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		decoded["invoke_id"], _ = json.Marshal(id)
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return err
	}
	type wire FluxItemRun
	var parsed wire
	if err := json.Unmarshal(normalized, &parsed); err != nil {
		return err
	}
	*r = FluxItemRun(parsed)
	r.rawFields = fields
	return nil
}

func (r FluxItemRun) MarshalJSON() ([]byte, error) {
	if r.rawFields == nil {
		type wire FluxItemRun
		return json.Marshal(wire(r))
	}
	fields := make(map[string]json.RawMessage, len(r.rawFields)+1)
	for k, v := range r.rawFields {
		fields[k] = v
	}
	version, err := json.Marshal(r.Version)
	if err != nil {
		return nil, err
	}
	fields["version"] = version
	if r.ExperimentSpaceID > 0 {
		fields["experiment_space_id"], err = json.Marshal(r.ExperimentSpaceID)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(fields)
}
