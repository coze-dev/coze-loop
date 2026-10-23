package convertor

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestAgentModelConfigurationSurvivesVersionMetainfo(t *testing.T) {
	for _, effort := range []*string{nil, gptr.Of(""), gptr.Of("Deleted_Tier")} {
		source := &entity.Evaluator{ID: 111, SpaceID: 7590122197455058946, EvaluatorType: entity.EvaluatorTypeAgent, AgentEvaluatorVersion: &entity.AgentEvaluatorVersion{ID: 222, SpaceID: 7590122197455058946, EvaluatorID: 111, Version: "1.0.0", AgentConfig: &entity.AgentConfig{AgentType: entity.AgentType_Vibe, Runtime: gptr.Of("codex_cli"), SearchProvider: gptr.Of("serp_api")}, ModelConfig: &entity.ModelConfig{ModelID: gptr.Of(int64(333)), JSONExt: gptr.Of(`{"reasoning":{"summary":"auto"}}`), Thinking: &entity.ThinkingConfig{ReasoningEffort: effort}}}}
		po, err := ConvertEvaluatorVersionDO2PO(source)
		require.NoError(t, err)
		back, err := ConvertEvaluatorVersionPO2DO(po)
		require.NoError(t, err)
		require.Equal(t, source.AgentEvaluatorVersion.ModelConfig, back.AgentEvaluatorVersion.ModelConfig)
		require.Equal(t, source.AgentEvaluatorVersion.AgentConfig, back.AgentEvaluatorVersion.AgentConfig)
		require.Equal(t, source.SpaceID, back.AgentEvaluatorVersion.SpaceID)
		require.Equal(t, "1.0.0", back.AgentEvaluatorVersion.Version)
	}
}
