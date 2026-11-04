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
