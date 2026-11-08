// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestExptItemRefConvertor_SUAStageRunConfPersists(t *testing.T) {
	const input = `{"eval_target_conf":{"version_id":999,"run_conf":{"max_qa_turns":60,"sua_stage_conf":{"mode":"staged","stages":[{"stage_id":"delivery","files":[],"future":{"id":9007199254740993,"enabled":false}}]},"sua_resume_pe_template":"继续 {{eval_result}}","sua_qa_reply_template":"只回答当前问题"}}}`
	var cfg entity.ExptItemConfig
	require.NoError(t, json.Unmarshal([]byte(input), &cfg))
	conv := NewExptItemRefConvertor()
	po := conv.DO2PO(&entity.ExptItemRef{ItemConfig: &cfg})
	require.NotNil(t, po.ItemConfig)
	assert.JSONEq(t, input, string(*po.ItemConfig))
	assert.Contains(t, string(*po.ItemConfig), "9007199254740993")
	got := conv.PO2DO(po)
	require.NotNil(t, got.ItemConfig)
	out, err := json.Marshal(got.ItemConfig)
	require.NoError(t, err)
	assert.JSONEq(t, input, string(out))
}

func TestExptItemRefConvertor_MaxQATurnsZeroAndUnset(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
	}{
		{"zero", `{"max_qa_turns":0,"sua_goal":"goal"}`, `{"max_qa_turns":0,"sua_goal":"goal"}`},
		{"null", `{"max_qa_turns":null,"sua_goal":"goal"}`, `{"sua_goal":"goal"}`},
		{"absent", `{"sua_goal":"goal"}`, `{"sua_goal":"goal"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var conf entity.ItemRunConf
			require.NoError(t, json.Unmarshal([]byte(tc.input), &conf))
			conv := NewExptItemRefConvertor()
			po := conv.DO2PO(&entity.ExptItemRef{ItemConfig: &entity.ExptItemConfig{
				EvalTargetConf: &entity.ItemTargetConf{TargetVersionID: 999, RunConf: &conf},
			}})
			require.NotNil(t, po.ItemConfig)
			assert.JSONEq(t, `{"eval_target_conf":{"version_id":999,"run_conf":`+tc.want+`}}`, string(*po.ItemConfig))
			got := conv.PO2DO(po)
			require.NotNil(t, got.ItemConfig)
			require.NotNil(t, got.ItemConfig.EvalTargetConf)
			out, err := json.Marshal(got.ItemConfig.EvalTargetConf.RunConf)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(out))
		})
	}
}
