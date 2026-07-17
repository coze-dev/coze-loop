// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// TestExptConverter_EvalConf_SkillTOSKeys_RoundTrip 覆盖新增导出字段 SkillTOSKeys 经
// eval_conf blob（DO2PO json.Marshal → PO2DO json.Unmarshal）序列化往返后保持一致。
func TestExptConverter_EvalConf_SkillTOSKeys_RoundTrip(t *testing.T) {
	t.Parallel()

	converter := NewExptConverter()
	do := &entity.Experiment{
		ID:      100,
		SpaceID: 200,
		EvalConf: &entity.EvaluationConfiguration{
			SkillTOSKeys: map[string]string{
				"123:0.0.1": "skills/200/agentbuddy/100/123-0.0.1/skill.zip",
				"456:0.0.1": "skills/200/agentbuddy/100/456-0.0.1/skill.zip",
			},
		},
	}

	po, err := converter.DO2PO(do)
	require.NoError(t, err)
	require.NotNil(t, po.EvalConf)

	got, err := converter.PO2DO(po, nil)
	require.NoError(t, err)
	require.NotNil(t, got.EvalConf)
	assert.Equal(t, do.EvalConf.SkillTOSKeys, got.EvalConf.SkillTOSKeys)
}
