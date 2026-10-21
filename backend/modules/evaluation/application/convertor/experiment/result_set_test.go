package experiment

import (
	"encoding/json"
	"github.com/bytedance/gg/gptr"
	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestResultSetRunModeSurvivesStorageAndReadback(t *testing.T) {
	domain, err := OpenAPIRunModeConfigDTO2Domain(&openapiExperiment.RunModeConfig{RunMode: gptr.Of(openapiExperiment.ExptRunModeResultSet), MaxRunMinutes: gptr.Of(int32(30))})
	require.NoError(t, err)
	require.Equal(t, domainExpt.ExptRunMode_ResultSet, domain.GetRunMode())
	conf := &entity.EvaluationConfiguration{RunModeConfig: runModeConfigDTO2DO(domain)}
	raw, err := json.Marshal(conf)
	require.NoError(t, err)
	var stored entity.EvaluationConfiguration
	require.NoError(t, json.Unmarshal(raw, &stored))
	require.True(t, stored.IsResultSet())
	api := RunModeConfigDomain2OpenAPI(runModeConfigDO2DTO(stored.RunModeConfig))
	require.Equal(t, "result_set", api.GetRunMode())
	require.EqualValues(t, 30, api.GetMaxRunMinutes())
	require.Equal(t, 6, entity.RunModeToInt(stored.RunModeConfig.RunMode))
	detail := ToExptDTO(&entity.Experiment{EvalConf: &stored})
	require.Equal(t, domainExpt.ExptRunMode_ResultSet, detail.GetRunModeConfig().GetRunMode())
}
