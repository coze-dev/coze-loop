// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	stdjson "encoding/json"
	"strings"
	"testing"

	jsoncodec "github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/stretchr/testify/require"
)

func TestEvalTargetOutputLegacyUsage(t *testing.T) {
	for name, decode := range map[string]func([]byte, any) error{"standard": stdjson.Unmarshal, "repository": jsoncodec.Unmarshal} {
		t.Run(name, func(t *testing.T) {
			var output EvalTargetOutputData
			err := decode([]byte(`{"OutputFields":{"actual_output":{"text":"answer"}},"EvaluatorUsage":{"InputTokens":17,"OutputTokens":23,"TotalTokens":40},"TimeConsumingMS":11}`), &output)
			require.NoError(t, err)
			require.Equal(t, &EvalTargetUsage{InputTokens: 17, OutputTokens: 23, TotalTokens: 40}, output.EvalTargetUsage)
			require.Equal(t, "answer", output.OutputFields["actual_output"].GetText())
			require.Equal(t, int64(11), *output.TimeConsumingMS)
		})
	}
}

func TestEvalTargetOutputLegacyUsagePreservesLargeContent(t *testing.T) {
	text := strings.Repeat("x", 2<<20)
	input := []byte(`{"OutputFields":{"actual_output":{"text":"` + text + `"}},"EvaluatorUsage":{"InputTokens":17},"Ext":{"origin":"kept"},"EvalTargetSteps":[{"StepName":"step","EventType":"FINISHED"}]}`)
	var output EvalTargetOutputData
	require.NoError(t, jsoncodec.Unmarshal(input, &output))
	require.Equal(t, text, output.OutputFields["actual_output"].GetText())
	require.Equal(t, int64(17), output.EvalTargetUsage.InputTokens)
	require.Equal(t, "kept", output.Ext["origin"])
	require.Equal(t, "step", output.EvalTargetSteps[0].StepName)
	encoded, err := jsoncodec.Marshal(&output)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"EvalTargetUsage"`)
	require.NotContains(t, string(encoded), `"EvaluatorUsage"`)
}

func TestEvalTargetOutputUsagePresenceAndPrecedence(t *testing.T) {
	for _, test := range []struct {
		name, input string
		want        *EvalTargetUsage
		fail        bool
	}{
		{"new wins", `{"EvalTargetUsage":{"InputTokens":2,"OutputTokens":3,"TotalTokens":5},"EvaluatorUsage":{"InputTokens":99}}`, &EvalTargetUsage{InputTokens: 2, OutputTokens: 3, TotalTokens: 5}, false},
		{"new zeros win", `{"EvaluatorUsage":{"InputTokens":99},"EvalTargetUsage":{"InputTokens":0,"OutputTokens":0,"TotalTokens":0}}`, &EvalTargetUsage{}, false},
		{"new empty object wins", `{"EvalTargetUsage":{},"EvaluatorUsage":{"InputTokens":99}}`, &EvalTargetUsage{}, false},
		{"new null wins", `{"EvalTargetUsage":null,"EvaluatorUsage":{"InputTokens":99}}`, nil, false},
		{"new null ignores old bad type", `{"EvalTargetUsage":null,"EvaluatorUsage":"unused"}`, nil, false},
		{"new value ignores old bad type", `{"EvaluatorUsage":[1],"EvalTargetUsage":{"TotalTokens":9}}`, &EvalTargetUsage{TotalTokens: 9}, false},
		{"old null", `{"EvaluatorUsage":null}`, nil, false},
		{"missing usage", `{"OutputFields":{}}`, nil, false},
		{"new case insensitive", `{"evaltargetusage":{"InputTokens":1},"EvaluatorUsage":{"InputTokens":99}}`, &EvalTargetUsage{InputTokens: 1}, false},
		{"bad new does not fall back", `{"EvalTargetUsage":"invalid","EvaluatorUsage":{"InputTokens":99}}`, nil, true},
		{"bad old type", `{"EvaluatorUsage":1}`, nil, true},
		{"bad old counter", `{"EvaluatorUsage":{"InputTokens":"17"}}`, nil, true},
		{"overflow", `{"EvaluatorUsage":{"InputTokens":9223372036854775808}}`, nil, true},
		{"malformed JSON", `{"EvaluatorUsage":`, nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, decode := range []func([]byte, any) error{stdjson.Unmarshal, jsoncodec.Unmarshal} {
				var output EvalTargetOutputData
				err := decode([]byte(test.input), &output)
				if test.fail {
					require.Error(t, err)
					continue
				}
				require.NoError(t, err)
				require.Equal(t, test.want, output.EvalTargetUsage)
			}
		})
	}
}

func TestEvalTargetOutputDecodePreservesExistingPartialContent(t *testing.T) {
	output := EvalTargetOutputData{OutputFields: map[string]*Content{"keep": {}}, EvalTargetUsage: &EvalTargetUsage{TotalTokens: 9}}
	require.NoError(t, jsoncodec.Unmarshal([]byte(`{"TimeConsumingMS":0}`), &output))
	require.Contains(t, output.OutputFields, "keep")
	require.Equal(t, int64(9), output.EvalTargetUsage.TotalTokens)
	require.NoError(t, jsoncodec.Unmarshal([]byte(`{"EvalTargetUsage":null}`), &output))
	require.Nil(t, output.EvalTargetUsage)
}
