// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestSuaAgentType_OpenAPIStorageRoundTrip(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		value *string
		want  string
	}{
		{name: "omitted"},
		{name: "empty", value: gptr.Of("")},
		{name: "claude_code", value: gptr.Of("claude_code"), want: "claude_code"},
		{name: "codex", value: gptr.Of("codex"), want: "codex"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dto, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{
				RunMode:      gptr.Of(openapiExperiment.ExptRunModeSuaMultiTurn),
				SuaAgentType: tc.value,
			})
			require.NoError(t, err)
			conf := &entity.EvaluationConfiguration{RunModeConfig: runModeConfigDTO2DO(dto)}
			assert.Equal(t, tc.want, conf.RunModeConfig.SuaAgentType)

			raw, err := json.Marshal(conf)
			require.NoError(t, err)
			var restored entity.EvaluationConfiguration
			require.NoError(t, json.Unmarshal(raw, &restored))
			back := RunModeConfigDomain2OpenAPI(runModeConfigDO2DTO(restored.RunModeConfig))
			require.NotNil(t, back)
			assert.Equal(t, tc.want, back.GetSuaAgentType())
			if tc.want == "" {
				assert.NotContains(t, string(raw), "sua_agent_type")
				assert.Nil(t, back.SuaAgentType, "omitted harness must not become an explicit Claude choice")
			} else {
				assert.Contains(t, string(raw), `"sua_agent_type":"`+tc.want+`"`)
				assert.Equal(t, tc.value, back.SuaAgentType)
			}
		})
	}
}

func TestSuaAgentType_OpenAPIRejectsUnknownValues(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"bogus", "Claude_Code", "claude-code", " codex", "codex "} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			dto, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{SuaAgentType: gptr.Of(value)})
			require.Error(t, err)
			assert.Nil(t, dto)
			status, ok := errorx.FromStatusError(err)
			require.True(t, ok)
			assert.Equal(t, int32(errno.CommonInvalidParamCode), status.Code())
			assert.Contains(t, err.Error(), "invalid sua_agent_type")
		})
	}
}
