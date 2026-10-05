// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

const executionPrecisionJSON = `{"fraction":0.123456789012345678901,"nested":[-9007199254740993,{"exponent":1.2300e+20,"tiny":1e-1000}],"opaque_id":9007199254740993}`

func executionPrecisionInput() HookRunSnapshotInput {
	in := executionSnapshotInput()
	in.Execution.Sets[0].ItemConfig.EvalTargetConf.RunConf.FixedQueryList = []*FixedQuery{{Evaluators: map[string]interface{}{
		"opaque_id": json.Number("9007199254740993"),
		"fraction":  json.Number("0.123456789012345678901"),
		"nested":    []interface{}{json.Number("-9007199254740993"), map[string]interface{}{"exponent": json.Number("1.2300e+20"), "tiny": json.Number("1e-1000")}},
	}}}
	return in
}

func assertExecutionPrecision(t *testing.T, d *HookExecutionSnapshot) {
	t.Helper()
	encoded, err := json.Marshal(d.Sets[0].ItemConfig.EvalTargetConf.RunConf.FixedQueryList[0].Evaluators)
	require.NoError(t, err)
	require.Equal(t, executionPrecisionJSON, string(encoded))
}

func TestHookExecutionPrecisionConstructor(t *testing.T) {
	in := executionPrecisionInput()
	s, err := NewHookRunSnapshot(in)
	require.NoError(t, err)
	assertExecutionPrecision(t, s.input.Execution)
	in.Execution.Sets[0].ItemConfig.EvalTargetConf.RunConf.FixedQueryList[0].Evaluators["opaque_id"] = json.Number("7")
	assertExecutionPrecision(t, s.input.Execution)
}

func TestHookExecutionPrecisionInput(t *testing.T) {
	// Isolate the accessor from constructor cloning with an already-frozen value.
	s := &HookRunSnapshot{input: executionPrecisionInput()}
	out := s.Input()
	assertExecutionPrecision(t, out.Execution)
	out.Execution.Sets[0].ItemConfig.EvalTargetConf.RunConf.FixedQueryList[0].Evaluators["nested"].([]interface{})[1].(map[string]interface{})["exponent"] = json.Number("2")
	assertExecutionPrecision(t, s.Input().Execution)
}

func TestHookExecutionPrecisionJSONRejection(t *testing.T) {
	raw, err := json.Marshal(executionPrecisionInput().Execution)
	require.NoError(t, err)
	for _, invalid := range []string{`{"version":`, `{"unknown":1}`, string(raw) + ` {}`, string(raw) + ` false`, `[]`, `"not an object"`} {
		t.Run(invalid[:min(len(invalid), 24)], func(t *testing.T) {
			var value HookExecutionSnapshot
			err := json.Unmarshal([]byte(invalid), &value)
			require.Error(t, err)
		})
	}
}

func TestHookExecutionPrecisionDecoderRejectsTrailingAndPreservesReceiver(t *testing.T) {
	in := executionPrecisionInput().Execution
	raw, err := json.Marshal(in)
	require.NoError(t, err)
	for _, suffix := range []string{` {}`, ` false`, ` trailing`} {
		err := in.UnmarshalJSON(append(append([]byte(nil), raw...), suffix...))
		require.Error(t, err)
		assertExecutionPrecision(t, in)
	}
	require.NoError(t, in.UnmarshalJSON(append(raw, '\n')))
	assertExecutionPrecision(t, in)
}
