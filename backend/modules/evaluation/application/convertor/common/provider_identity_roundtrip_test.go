package common

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/require"
)

func TestProviderIdentityProjectionPreservesDraftConfiguration(t *testing.T) {
	for _, kind := range []entity.EvaluatorType{entity.EvaluatorTypePrompt, entity.EvaluatorTypeAgent} {
		for _, tc := range []struct {
			name, provider string
			displayed      int64
			replace        *int64
			nextProvider   *string
			inherit        bool
		}{
			{name: "canonical numeric projection", provider: "42", displayed: 42, inherit: true},
			{name: "matching explicit provider", provider: "42", displayed: 42, nextProvider: ptr.Of("42"), inherit: true},
			{name: "different displayed id", provider: "42", displayed: 42, replace: ptr.Of(int64(43))},
			{name: "different explicit provider", provider: "42", displayed: 42, nextProvider: ptr.Of("other")},
			{name: "leading zeros are a different provider string", provider: "0042", displayed: 42},
			{name: "nonnumeric provider cannot be inferred from zero", provider: "model-a", displayed: 0},
		} {
			t.Run(fmt.Sprintf("%v/%s", kind, tc.name), func(t *testing.T) {
				old := &entity.ModelConfig{ProviderModelID: ptr.Of(tc.provider), ModelName: "same display name", JSONExt: ptr.Of(`{"metadata":{"sequence":9007199254740993}}`), Thinking: &entity.ThinkingConfig{ReasoningEffort: ptr.Of("xhigh")}}
				before, err := json.Marshal(old)
				require.NoError(t, err)
				visible := ConvertModelConfigDO2DTO(old)
				require.Equal(t, tc.displayed, visible.GetModelID())
				visible.Thinking, visible.JSONExt = nil, nil
				if tc.replace != nil {
					visible.ModelID = tc.replace
				}
				next := ConvertModelConfigDTO2DO(visible)
				next.ProviderModelID = tc.nextProvider
				previous := &entity.Evaluator{EvaluatorType: kind}
				edited := &entity.Evaluator{EvaluatorType: kind}
				if kind == entity.EvaluatorTypePrompt {
					previous.PromptEvaluatorVersion = &entity.PromptEvaluatorVersion{ModelConfig: old}
					edited.PromptEvaluatorVersion = &entity.PromptEvaluatorVersion{ModelConfig: next}
				} else {
					previous.AgentEvaluatorVersion = &entity.AgentEvaluatorVersion{ModelConfig: old}
					edited.AgentEvaluatorVersion = &entity.AgentEvaluatorVersion{ModelConfig: next}
				}
				edited.PreserveDraftConfig(previous)
				if tc.inherit {
					require.NotNil(t, next.Thinking)
					require.Equal(t, "xhigh", *next.Thinking.ReasoningEffort)
					require.Equal(t, *old.JSONExt, *next.JSONExt)
					*next.Thinking.ReasoningEffort = "changed"
					*next.JSONExt = "{}"
				} else {
					require.Nil(t, next.Thinking)
					require.Nil(t, next.JSONExt)
				}
				after, err := json.Marshal(old)
				require.NoError(t, err)
				require.Equal(t, before, after)
			})
		}
	}
}
