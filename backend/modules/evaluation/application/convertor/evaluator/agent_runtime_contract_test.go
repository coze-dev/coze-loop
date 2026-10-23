package evaluator

import (
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	openapieval "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"testing"

	"github.com/bytedance/gg/gptr"
	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	openapidto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	"github.com/stretchr/testify/require"
)

func TestAgentRuntimeSearchWebOpenAPIRoundTrip(t *testing.T) {
	cfg := &commondto.AgentConfig{AgentType: gptr.Of("vibe"), Runtime: gptr.Of("codex_cli"), SearchProvider: gptr.Of("serp_api")}
	do := ConvertAgentConfigDTO2DO(cfg)
	back := ConvertAgentConfigDO2DTO(do)
	require.Equal(t, cfg, back)
	oapi := OpenAPIAgentConfigDO2DTO(do)
	require.Equal(t, &openapidto.AgentConfig{AgentType: gptr.Of(openapidto.AgentType("vibe")), Runtime: gptr.Of("codex_cli"), SearchProvider: gptr.Of("serp_api")}, oapi)
	require.Equal(t, cfg, ConvertAgentConfigDO2DTO(OpenAPIAgentConfigDTO2DO(oapi)))
}

func TestConvertersRejectExplicitInvalidAgentVersionSettings(t *testing.T) {
	dto := &evaluatordto.Evaluator{EvaluatorType: gptr.Of(evaluatordto.EvaluatorType_Agent), CurrentVersion: &evaluatordto.EvaluatorVersion{EvaluatorContent: &evaluatordto.EvaluatorContent{AgentEvaluator: &evaluatordto.AgentEvaluator{AgentConfig: &commondto.AgentConfig{Runtime: gptr.Of("codex")}}}}}
	_, err := ConvertEvaluatorDTO2DO(dto)
	require.Error(t, err)
	_, err = ConvertEvaluatorContent2DO(dto.CurrentVersion.EvaluatorContent, evaluatordto.EvaluatorType_Agent)
	require.Error(t, err)
	_, err = OpenAPIEvaluatorContentDTO2DO(&openapieval.EvaluatorContent{AgentEvaluator: &openapieval.AgentEvaluator{AgentConfig: &openapidto.AgentConfig{SearchProvider: gptr.Of("volcengine")}}}, entity.EvaluatorTypeAgent)
	require.Error(t, err)
}

func TestAgentPublishedConfigurationCrossesWebAndOpenAPIEnvelopes(t *testing.T) {
	web := &evaluatordto.Evaluator{EvaluatorID: gptr.Of(int64(7590122197455058001)), WorkspaceID: gptr.Of(int64(7590122197455058946)), EvaluatorType: gptr.Of(evaluatordto.EvaluatorType_Agent), CurrentVersion: &evaluatordto.EvaluatorVersion{ID: gptr.Of(int64(7590122197455058002)), Version: gptr.Of("1.0.0"), EvaluatorContent: &evaluatordto.EvaluatorContent{AgentEvaluator: &evaluatordto.AgentEvaluator{AgentConfig: &commondto.AgentConfig{AgentType: gptr.Of("vibe"), Runtime: gptr.Of("codex_cli"), SearchProvider: gptr.Of("serp_api")}, ModelConfig: &commondto.ModelConfig{ModelID: gptr.Of(int64(7590122197455058003)), ModelName: gptr.Of(""), Thinking: &commondto.ThinkingConfig{ReasoningEffort: gptr.Of("")}, JSONExt: gptr.Of(`{"reasoning":{"summary":"auto"}}`)}}}}}
	do, err := ConvertEvaluatorDTO2DO(web)
	require.NoError(t, err)
	back := ConvertEvaluatorDO2DTO(do)
	require.Equal(t, web.CurrentVersion.EvaluatorContent.AgentEvaluator.ModelConfig, back.CurrentVersion.EvaluatorContent.AgentEvaluator.ModelConfig)
	open := OpenAPIEvaluatorDO2DTO(do)
	require.Equal(t, web.CurrentVersion.ID, open.CurrentVersion.ID)
	require.Equal(t, "", open.CurrentVersion.EvaluatorContent.AgentEvaluator.ModelConfig.Thinking.GetReasoningEffort())
	require.NotNil(t, open.CurrentVersion.EvaluatorContent.AgentEvaluator.ModelConfig.Thinking.ReasoningEffort)
	require.Equal(t, "codex_cli", open.CurrentVersion.EvaluatorContent.AgentEvaluator.AgentConfig.GetRuntime())
	require.Equal(t, "serp_api", open.CurrentVersion.EvaluatorContent.AgentEvaluator.AgentConfig.GetSearchProvider())
	require.Equal(t, web.CurrentVersion.EvaluatorContent.AgentEvaluator.ModelConfig.JSONExt, open.CurrentVersion.EvaluatorContent.AgentEvaluator.ModelConfig.JSONExt)
}
