package common

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	openapidto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	"github.com/stretchr/testify/require"
)

func TestModelThinkingWebOpenAPIRoundTrip(t *testing.T) {
	for _, effort := range []*string{nil, gptr.Of(""), gptr.Of("Tier_V2"), gptr.Of("none")} {
		web := &commondto.ModelConfig{ModelID: gptr.Of(int64(7590122197455058946)), Temperature: gptr.Of(0.75), MaxTokens: gptr.Of(int32(4096)), TopP: gptr.Of(0.85), Thinking: &commondto.ThinkingConfig{ThinkingOption: gptr.Of("enabled"), ReasoningEffort: effort}, JSONExt: gptr.Of(`{"reasoning":{"summary":"auto"},"stop":["END"]}`)}
		do := ConvertModelConfigDTO2DO(web)
		round := ConvertModelConfigDO2DTO(do)
		require.Equal(t, web.Thinking, round.Thinking)
		require.Equal(t, web.JSONExt, round.JSONExt)
		require.Equal(t, web.ModelID, round.ModelID)
		oapi := OpenAPIModelConfigDO2DTO(do)
		require.Equal(t, web.Thinking.ReasoningEffort, oapi.Thinking.ReasoningEffort)
		require.Equal(t, web.JSONExt, oapi.JSONExt)
		back := ConvertModelConfigDO2DTO(OpenAPIModelConfigDTO2DO(oapi))
		require.Equal(t, web.Thinking, back.Thinking)
		require.Equal(t, web.JSONExt, back.JSONExt)
		require.Equal(t, web.ModelID, back.ModelID)
	}
}

func TestModelThinkingBudgetDisabledPreservesWireValue(t *testing.T) {
	oapi := &openapidto.ModelConfig{Thinking: &openapidto.ThinkingConfig{ThinkingOption: gptr.Of("disabled"), ReasoningEffort: gptr.Of("max"), BudgetTokens: gptr.Of(int64(2048))}}
	back := OpenAPIModelConfigDO2DTO(OpenAPIModelConfigDTO2DO(oapi))
	require.Equal(t, oapi.Thinking, back.Thinking)
}
