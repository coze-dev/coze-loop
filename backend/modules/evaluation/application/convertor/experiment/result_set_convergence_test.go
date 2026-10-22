package experiment

import (
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestConvergedResultSetReadModelKeepsExecutionPrivate(t *testing.T) {
	conf := &entity.EvaluationConfiguration{RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunModeResultSet, MaxRunMinutes: 30}}
	require.NoError(t, conf.FreezeResultSetBinaryVersion("fixed-copy-pin"))
	raw, err := json.Marshal(conf)
	require.NoError(t, err)
	var stored entity.EvaluationConfiguration
	require.NoError(t, json.Unmarshal(raw, &stored))
	for _, source := range []string{"101", "builtin:result-set-evaluation"} {
		detail := ToExptDTO(&entity.Experiment{EvalConf: &stored, Target: &entity.EvalTarget{SourceTargetID: source}})
		require.Nil(t, detail.RunModeConfig, source)
		require.Nil(t, RunModeConfigDomain2OpenAPI(detail.RunModeConfig), source)
		require.JSONEq(t, `{"binary_version":"fixed-copy-pin"}`, detail.GetTargetRuntimeParam().GetJSONValue())
	}
	require.True(t, stored.IsResultSet())
	require.Equal(t, 6, entity.RunModeToInt(stored.RunModeConfig.RunMode))
}

func TestConvergedResultSetOpenAPIRejectsExplicitMode(t *testing.T) {
	_, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{RunMode: gptr.Of("result_set")})
	require.ErrorContains(t, err, "invalid run_mode")
	for _, mode := range []string{"single_turn", "fixed_script_multi_turn", "sua_multi_turn", "goal"} {
		conf, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{RunMode: &mode})
		require.NoError(t, err, mode)
		require.Equal(t, mode, RunModeConfigDomain2OpenAPI(conf).GetRunMode())
	}
}
