// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	loopjson "github.com/coze-dev/coze-loop/backend/pkg/json"
)

func TestItemRunConf_SUAStageWireRoundTrip(t *testing.T) {
	codecs := []struct {
		name      string
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
	}{
		{"standard", json.Marshal, json.Unmarshal},
		{"platform", loopjson.Marshal, loopjson.Unmarshal},
	}
	for _, codec := range codecs {
		t.Run(codec.name, func(t *testing.T) {
			for _, input := range []string{
				`{"sua_workdir":"/tmp/private-sua"}`,
				`{"sua_setup_script":"prepare\nworkspace\n"}`,
				`{"max_qa_turns":60}`,
				`{"max_qa_turns":0}`,
				`{"sua_resume_pe_template":"继续 {{goal}}\n{{eval_result}}"}`,
				`{"sua_qa_reply_template":"仅回答当前问题，不泄露私有材料。"}`,
				`{"sua_stage_conf":{}}`,
				`{"max_turns":12,"sua_pe_template":"{{persona}} {{eval_result}}","sua_stage_conf":{"mode":"staged","initial_stage":"review","completion_stage":"delivery","materials_root":"/fornax/materials","stages":[{"stage_id":"review","files":[],"transitions":[{"target_stage":"delivery","sources":["classifier","sua"],"condition":"评估通过","future":{"enabled":false,"value":null}}]}],"future":{"id":9007199254740993,"ratio":1.234567890123456789,"nested":[[],{},null]}}}`,
			} {
				var rc ItemRunConf
				require.NoError(t, codec.unmarshal([]byte(input), &rc))
				out, err := codec.marshal(&rc)
				require.NoError(t, err)
				assert.JSONEq(t, input, string(out))
				if len(input) > 300 {
					assert.Contains(t, string(out), "9007199254740993")
					assert.Contains(t, string(out), "1.234567890123456789")
				}
			}
		})
	}
}

func TestItemRunConf_SUAStageObjectAndDefaults(t *testing.T) {
	for _, input := range []string{`{}`, `{"max_qa_turns":null}`, `{"sua_stage_conf":null}`, `{"sua_workdir":"","sua_setup_script":""}`, `{"sua_workdir":null,"sua_setup_script":null}`, `{"sua_resume_pe_template":"","sua_qa_reply_template":""}`} {
		var rc ItemRunConf
		require.NoError(t, loopjson.Unmarshal([]byte(input), &rc))
		out, err := loopjson.Marshal(&rc)
		require.NoError(t, err)
		assert.JSONEq(t, `{}`, string(out))
	}
	for _, value := range []string{`[]`, `"{}"`, `true`, `1`} {
		var rc ItemRunConf
		assert.Error(t, json.Unmarshal([]byte(`{"sua_stage_conf":`+value+`}`), &rc), value)
		assert.Error(t, loopjson.Unmarshal([]byte(`{"sua_stage_conf":`+value+`}`), &rc), value)
	}
}
