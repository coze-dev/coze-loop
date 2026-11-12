package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFluxItemRunRoundTripPreservesRuntimeRecoveryState(t *testing.T) {
	input := []byte(`{"experiment_id":3,"space_id":4,"invoke_id":"5","state":"reserved","version":0,"binding_id":"b","query_digest":"q","plan":{"targets":[{"node_key":"code"}]},"space_request":{"execution_policy":null},"template_request":{"node_map":{"x":"y"}},"interactions":{"phase_1":{"answer_ref":"private://a"}}}`)
	var run FluxItemRun
	require.NoError(t, json.Unmarshal(input, &run))
	require.Equal(t, int64(5), run.InvokeID)
	run.Version = 1
	output, err := json.Marshal(&run)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(output, &fields))
	require.JSONEq(t, `"5"`, string(fields["invoke_id"]))
	require.JSONEq(t, `{"execution_policy":null}`, string(fields["space_request"]))
	require.JSONEq(t, `{"targets":[{"node_key":"code"}]}`, string(fields["plan"]))
	require.JSONEq(t, `{"phase_1":{"answer_ref":"private://a"}}`, string(fields["interactions"]))
	require.JSONEq(t, `1`, string(fields["version"]))
}

func TestFluxItemRunMarshalUpdatesCleanupFieldsWithoutDroppingRuntimeState(t *testing.T) {
	input := []byte(`{"invoke_id":"5","cleanup_state":"pending","report_completed":true,"cleanup_log_id":"old","cleanup_last_error":"retry","runtime_owned":{"nested":1}}`)
	var run FluxItemRun
	require.NoError(t, json.Unmarshal(input, &run))
	run.CleanupState = "archived"
	run.CleanupLogID = ""
	run.CleanupLastError = ""

	output, err := json.Marshal(&run)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(output, &fields))
	require.JSONEq(t, `"archived"`, string(fields["cleanup_state"]))
	require.JSONEq(t, `true`, string(fields["report_completed"]))
	require.JSONEq(t, `""`, string(fields["cleanup_log_id"]))
	require.JSONEq(t, `""`, string(fields["cleanup_last_error"]))
	require.JSONEq(t, `{"nested":1}`, string(fields["runtime_owned"]))
	require.JSONEq(t, `"5"`, string(fields["invoke_id"]))
}

func TestFluxWorkflowBindingKeepsResolvedModelProvenance(t *testing.T) {
	const stored = `{"resolved_model_name":"gpt-6-sol","resolved_agent_type":"codex","model_source":"workflow_entry_agent","entry_node_key":"clarify-req","entry_agent_id":"agent-1","entry_agent_version_id":"version-2","entry_agent_digest":"sha256:abc"}`
	var binding FluxWorkflowBinding
	require.NoError(t, json.Unmarshal([]byte(stored), &binding))
	encoded, err := json.Marshal(binding)
	require.NoError(t, err)
	require.JSONEq(t, stored, string(encoded))
}
