package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	loopjson "github.com/coze-dev/coze-loop/backend/pkg/json"
)

func TestFixedQuery_EvaluatorConfRoundTrip(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "absent_fields",
			input:    `{}`,
			expected: `{}`,
		},
		{
			name:     "legacy_only",
			input:    `{"query":"synthetic","evaluators":{"legacy":{"weight":2}}}`,
			expected: `{"query":"synthetic","evaluators":{"legacy":{"weight":2}}}`,
		},
		{
			name:     "legacy_empty_still_omitted",
			input:    `{"query":"synthetic","evaluators":{}}`,
			expected: `{"query":"synthetic"}`,
		},
		{
			name:     "complete_canonical_config",
			input:    `{"evaluator_conf":{"evaluators":[{"evaluator_key":"synthetic","evaluator_type":"future","params":{"threshold":0.5,"unknown":{"flags":[true,false],"optional":null}},"timeout":17,"score_weight":2}],"total_score":{"type":"future","params":{"weights":[1,2]}},"total_score_type":"weighted","unknown_nested":{"values":[{},[],null]}}}`,
			expected: `{"evaluator_conf":{"evaluators":[{"evaluator_key":"synthetic","evaluator_type":"future","params":{"threshold":0.5,"unknown":{"flags":[true,false],"optional":null}},"timeout":17,"score_weight":2}],"total_score":{"type":"future","params":{"weights":[1,2]}},"total_score_type":"weighted","unknown_nested":{"values":[{},[],null]}}}`,
		},
		{
			name:     "explicit_empty_evaluators",
			input:    `{"evaluator_conf":{"evaluators":[]}}`,
			expected: `{"evaluator_conf":{"evaluators":[]}}`,
		},
		{
			name:     "explicit_empty_object",
			input:    `{"evaluator_conf":{}}`,
			expected: `{"evaluator_conf":{}}`,
		},
		{
			name:     "explicit_null",
			input:    `{"evaluator_conf":null}`,
			expected: `{"evaluator_conf":null}`,
		},
		{
			name:     "legacy_and_canonical_coexist",
			input:    `{"evaluators":{"legacy":{"weight":2}},"evaluator_conf":{"evaluators":[{"evaluator_key":"canonical","params":{"enabled":true}}]}}`,
			expected: `{"evaluators":{"legacy":{"weight":2}},"evaluator_conf":{"evaluators":[{"evaluator_key":"canonical","params":{"enabled":true}}]}}`,
		},
		{
			name:     "complex_query_coexists",
			input:    `{"query":"fallback","complex_query":{"parts":[{"content_type":"text","text":"synthetic"},{"content_type":"file","file":{"file_name":"synthetic.txt","file_url":"https://example.invalid/synthetic.txt"}}],"extra":{"synthetic":"value"}},"evaluator_conf":{"evaluators":[],"timeout":17}}`,
			expected: `{"query":"fallback","complex_query":{"parts":[{"content_type":"text","text":"synthetic"},{"content_type":"file","file":{"file_name":"synthetic.txt","file_url":"https://example.invalid/synthetic.txt"}}],"extra":{"synthetic":"value"}},"evaluator_conf":{"evaluators":[],"timeout":17}}`,
		},
	}
	codecs := []struct {
		name      string
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
	}{
		{name: "standard", marshal: json.Marshal, unmarshal: json.Unmarshal},
		{name: "platform", marshal: loopjson.Marshal, unmarshal: loopjson.Unmarshal},
	}
	for _, codec := range codecs {
		t.Run(codec.name, func(t *testing.T) {
			t.Parallel()
			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()
					var conf ItemRunConf
					input := `{"fixed_query_list":[` + testCase.input + `]}`
					require.NoError(t, codec.unmarshal([]byte(input), &conf))
					require.Len(t, conf.FixedQueryList, 1)
					output, err := codec.marshal(&conf)
					require.NoError(t, err)
					expected := `{"fixed_query_list":[` + testCase.expected + `]}`
					assert.JSONEq(t, expected, string(output))
				})
			}
		})
	}
}
