package entity

import (
	"crypto/sha256"
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
)

func TestAgentVersionRejectsExplicitRuntimeSearchAndThinkingConflict(t *testing.T) {
	for _, cfg := range []*AgentConfig{{Runtime: gptr.Of("codex")}, {Runtime: gptr.Of("")}, {SearchProvider: gptr.Of("volcengine")}, {SearchProvider: gptr.Of("")}} {
		v := &AgentEvaluatorVersion{AgentConfig: cfg}
		require.Error(t, v.ValidateBaseInfo())
	}
	for _, cfg := range []*ThinkingConfig{{ThinkingOption: gptr.Of("off")}, {ReasoningEffort: gptr.Of("tier"), BudgetTokens: gptr.Of(int64(1024))}, {BudgetTokens: gptr.Of(int64(-1))}} {
		v := &AgentEvaluatorVersion{ModelConfig: &ModelConfig{Thinking: cfg}}
		require.Error(t, v.ValidateBaseInfo())
	}
	require.NoError(t, (&AgentEvaluatorVersion{}).ValidateBaseInfo())
	require.NoError(t, (&AgentEvaluatorVersion{AgentConfig: &AgentConfig{Runtime: gptr.Of("codex_cli"), SearchProvider: gptr.Of("serp_api")}, ModelConfig: &ModelConfig{Thinking: &ThinkingConfig{ReasoningEffort: gptr.Of("Private_Tier")}}}).ValidateBaseInfo())
	require.NoError(t, (&AgentEvaluatorVersion{ModelConfig: &ModelConfig{Thinking: &ThinkingConfig{ThinkingOption: gptr.Of("disabled"), ReasoningEffort: gptr.Of("Private_Tier"), BudgetTokens: gptr.Of(int64(1024))}}}).ValidateBaseInfo())
}

func TestPreserveDraftModelThinkingAndAgentVersionSettings(t *testing.T) {
	original := &Evaluator{EvaluatorType: EvaluatorTypeAgent, AgentEvaluatorVersion: &AgentEvaluatorVersion{ModelConfig: &ModelConfig{ModelID: gptr.Of(int64(100)), Temperature: gptr.Of(0.8), JSONExt: gptr.Of(`{"stop":["END"]}`), Thinking: &ThinkingConfig{ReasoningEffort: gptr.Of("Historical_Tier"), ThinkingOption: gptr.Of("enabled")}}, AgentConfig: &AgentConfig{AgentType: AgentType_Vibe, Runtime: gptr.Of("codex_cli"), SearchProvider: gptr.Of("serp_api")}}}
	before, _ := json.Marshal(original)
	for _, change := range []string{"omitted_model", "same_model_omitted_thinking", "same_model_empty_thinking", "explicit_auto", "different_model"} {
		t.Run(change, func(t *testing.T) {
			next := &Evaluator{EvaluatorType: EvaluatorTypeAgent, AgentEvaluatorVersion: &AgentEvaluatorVersion{PromptConfig: &AgentEvaluatorPromptConfig{}, AgentConfig: &AgentConfig{AgentType: AgentType_Vibe}}}
			switch change {
			case "same_model_omitted_thinking":
				next.AgentEvaluatorVersion.ModelConfig = &ModelConfig{ModelID: gptr.Of(int64(100)), Temperature: gptr.Of(0.3)}
			case "same_model_empty_thinking":
				next.AgentEvaluatorVersion.ModelConfig = &ModelConfig{ModelID: gptr.Of(int64(100)), Thinking: &ThinkingConfig{}}
			case "explicit_auto":
				next.AgentEvaluatorVersion.ModelConfig = &ModelConfig{ModelID: gptr.Of(int64(100)), Thinking: &ThinkingConfig{ReasoningEffort: gptr.Of("")}}
			case "different_model":
				next.AgentEvaluatorVersion.ModelConfig = &ModelConfig{ModelID: gptr.Of(int64(101))}
			}
			next.PreserveDraftConfig(original)
			require.Equal(t, "codex_cli", gptr.Indirect(next.AgentEvaluatorVersion.AgentConfig.Runtime))
			require.Equal(t, "serp_api", gptr.Indirect(next.AgentEvaluatorVersion.AgentConfig.SearchProvider))
			switch change {
			case "explicit_auto":
				require.Equal(t, "", gptr.Indirect(next.AgentEvaluatorVersion.ModelConfig.Thinking.ReasoningEffort))
			case "different_model":
				require.Nil(t, next.AgentEvaluatorVersion.ModelConfig.Thinking)
			default:
				require.Equal(t, "Historical_Tier", gptr.Indirect(next.AgentEvaluatorVersion.ModelConfig.Thinking.ReasoningEffort))
				require.Equal(t, original.AgentEvaluatorVersion.ModelConfig.JSONExt, next.AgentEvaluatorVersion.ModelConfig.JSONExt)
			}
			if change == "same_model_omitted_thinking" {
				require.Equal(t, 0.3, gptr.Indirect(next.AgentEvaluatorVersion.ModelConfig.Temperature))
			}
			after, _ := json.Marshal(original)
			require.Equal(t, before, after)
		})
	}
}

func TestModelExtraRequiresJSONObjectAndPreservesNativeSiblings(t *testing.T) {
	cfg := &ModelConfig{JSONExt: gptr.Of(`{"reasoning":{"summary":"auto"},"stop":["END"]}`)}
	got, err := cfg.ExtraObject()
	require.NoError(t, err)
	require.Equal(t, map[string]any{"reasoning": map[string]any{"summary": "auto"}, "stop": []any{"END"}}, got)
	for _, raw := range []string{"[]", "null", `"{}"`, `{"secret":`} {
		_, err := (&ModelConfig{JSONExt: gptr.Of(raw)}).ExtraObject()
		require.Error(t, err)
	}
}

func TestPreserveDraftSameModelIDAllowsOmittedProviderIdentity(t *testing.T) {
	const modelID int64 = 7590122197455058946
	previous := &Evaluator{
		EvaluatorType: EvaluatorTypeAgent,
		AgentEvaluatorVersion: &AgentEvaluatorVersion{
			ModelConfig: &ModelConfig{
				ModelID: gptr.Of(modelID), ProviderModelID: gptr.Of("provider-ep-123"),
				Thinking: &ThinkingConfig{ThinkingOption: gptr.Of("enabled"), ReasoningEffort: gptr.Of("Historical_Tier")},
				JSONExt:  gptr.Of(`{"reasoning":{"summary":"auto"},"stop":["END"]}`),
			},
		},
	}
	before, err := json.Marshal(previous)
	require.NoError(t, err)
	beforeHash := sha256.Sum256(before)
	for _, tc := range []struct {
		name     string
		id       int64
		provider *string
		preserve bool
	}{
		{"same_id_omitted_provider", modelID, nil, true},
		{"same_id_same_provider", modelID, gptr.Of("provider-ep-123"), true},
		{"same_id_explicit_different_provider", modelID, gptr.Of("provider-ep-456"), false},
		{"different_id_omitted_provider", modelID + 1, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := &Evaluator{EvaluatorType: EvaluatorTypeAgent, AgentEvaluatorVersion: &AgentEvaluatorVersion{
				Description: "updated text", ModelConfig: &ModelConfig{ModelID: gptr.Of(tc.id), ProviderModelID: tc.provider, Temperature: gptr.Of(0.3)},
			}}
			next.PreserveDraftConfig(previous)
			got := next.AgentEvaluatorVersion.ModelConfig
			require.Equal(t, 0.3, gptr.Indirect(got.Temperature))
			if tc.preserve {
				require.NotNil(t, got.Thinking)
				require.Equal(t, "Historical_Tier", gptr.Indirect(got.Thinking.ReasoningEffort))
				require.Equal(t, previous.AgentEvaluatorVersion.ModelConfig.JSONExt, got.JSONExt)
			} else {
				require.Nil(t, got.Thinking)
				require.Nil(t, got.JSONExt)
			}
			after, err := json.Marshal(previous)
			require.NoError(t, err)
			require.Equal(t, before, after)
			afterHash := sha256.Sum256(after)
			require.Equal(t, beforeHash, afterHash)
			t.Logf("original_snapshot_sha256=%x", afterHash)
		})
	}
}

func TestModelExtraObjectPreservesLargeIntegerAndRejectsTrailingJSON(t *testing.T) {
	cfg := &ModelConfig{JSONExt: gptr.Of(`{"native_integer":9007199254740993,"reasoning":{"summary":"auto"},"stop":["END"]}`)}
	got, err := cfg.ExtraObject()
	require.NoError(t, err)
	raw, err := json.Marshal(got)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	require.Equal(t, "9007199254740993", string(fields["native_integer"]))
	require.JSONEq(t, `{"summary":"auto"}`, string(fields["reasoning"]))
	require.JSONEq(t, `["END"]`, string(fields["stop"]))
	for _, invalid := range []string{`{} {}`, `{} null`, `{"native_integer":9007199254740993} trailing`, `[]`, `null`} {
		_, err := (&ModelConfig{JSONExt: gptr.Of(invalid)}).ExtraObject()
		require.Error(t, err)
	}
}
