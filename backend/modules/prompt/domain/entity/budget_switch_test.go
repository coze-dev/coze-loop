// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"testing"

	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/require"
)

func TestBudgetToSpecificEffortValueSave(t *testing.T) {
	for _, value := range []string{"xhigh", "low", "Tier_V2"} {
		t.Run(value, func(t *testing.T) {
			old := &ModelConfig{ModelID: 123, Thinking: &ThinkingConfig{ThinkingOption: ptr.Of(ThinkingOptionEnabled), BudgetTokens: ptr.Of(int64(2048))}}
			var next ModelConfig
			require.NoError(t, json.Unmarshal([]byte(`{"model_id":123,"thinking":{"reasoning_effort_value":"`+value+`"},"extra":"{\"metadata\":{\"sequence\":9007199254740993}}"}`), &next))
			before, err := json.Marshal(next)
			require.NoError(t, err)
			got, err := PreserveModelConfigForSave(&next, old)
			require.NoError(t, err)
			require.Nil(t, got.Thinking.BudgetTokens)
			data, err := json.Marshal(got.Thinking)
			require.NoError(t, err)
			var thinking map[string]any
			require.NoError(t, json.Unmarshal(data, &thinking))
			require.Equal(t, value, thinking["reasoning_effort_value"])
			require.Equal(t, "enabled", thinking["thinking_option"])
			require.Equal(t, `{"metadata":{"sequence":9007199254740993}}`, *got.Extra)
			after, err := json.Marshal(next)
			require.NoError(t, err)
			require.Equal(t, before, after)
			require.Equal(t, int64(2048), *old.Thinking.BudgetTokens)
		})
	}
}

func TestExplicitValueAndBudgetRejectedBeforeSave(t *testing.T) {
	var next ModelConfig
	require.NoError(t, json.Unmarshal([]byte(`{"model_id":123,"thinking":{"reasoning_effort_value":"xhigh","budget_tokens":1024}}`), &next))
	_, err := PreserveModelConfigForSave(&next, nil)
	require.Error(t, err)
}

func TestLegacyReasoningBudgetSaveRemainsLegacy(t *testing.T) {
	old := &ModelConfig{ModelID: 123, Thinking: &ThinkingConfig{ReasoningEffort: ptr.Of(ReasoningEffortHigh), BudgetTokens: ptr.Of(int64(2048))}}
	got, err := PreserveModelConfigForSave(nil, old)
	require.NoError(t, err)
	data, err := json.Marshal(got.Thinking)
	require.NoError(t, err)
	var thinking map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &thinking))
	require.NotContains(t, thinking, "reasoning_effort_value")
	require.Equal(t, `"high"`, string(thinking["reasoning_effort"]))
	require.Equal(t, "2048", string(thinking["budget_tokens"]))
}

func TestDraftSavePreservesIndependentEffortRepresentations(t *testing.T) {
	old := &ModelConfig{ModelID: 123, Thinking: &ThinkingConfig{ReasoningEffort: ptr.Of(ReasoningEffortHigh), ReasoningEffortValue: ptr.Of("Tier_V2")}}
	got, err := PreserveModelConfigForSave(&ModelConfig{ModelID: 123}, old)
	require.NoError(t, err)
	require.Equal(t, ReasoningEffortHigh, *got.Thinking.ReasoningEffort)
	require.Equal(t, "Tier_V2", *got.Thinking.ReasoningEffortValue)
	*got.Thinking.ReasoningEffort = ReasoningEffortLow
	*got.Thinking.ReasoningEffortValue = "changed"
	require.Equal(t, ReasoningEffortHigh, *old.Thinking.ReasoningEffort)
	require.Equal(t, "Tier_V2", *old.Thinking.ReasoningEffortValue)
	_, err = PreserveModelConfigForSave(&ModelConfig{ModelID: 123, Thinking: &ThinkingConfig{ReasoningEffort: ptr.Of(ReasoningEffortLow)}}, old)
	require.Error(t, err)
	changed, err := PreserveModelConfigForSave(&ModelConfig{ModelID: 456}, old)
	require.NoError(t, err)
	require.Nil(t, changed.Thinking)
	legacy, err := PreserveModelConfigForSave(&ModelConfig{ModelID: 123, Thinking: &ThinkingConfig{ReasoningEffort: ptr.Of(ReasoningEffortLow)}}, &ModelConfig{ModelID: 123, Thinking: &ThinkingConfig{ReasoningEffort: ptr.Of(ReasoningEffortHigh)}})
	require.NoError(t, err)
	require.Nil(t, legacy.Thinking.ReasoningEffortValue)
	require.Equal(t, ReasoningEffortLow, *legacy.Thinking.ReasoningEffort)
}

func TestDisabledExplicitEffortSuppressesBudgetAtSave(t *testing.T) {
	incoming := &ModelConfig{ModelID: 123, Thinking: &ThinkingConfig{ThinkingOption: ptr.Of(ThinkingOptionDisabled), ReasoningEffortValue: ptr.Of("xhigh"), BudgetTokens: ptr.Of(int64(1024))}}
	got, err := PreserveModelConfigForSave(incoming, nil)
	require.NoError(t, err)
	require.Equal(t, "", *got.Thinking.ReasoningEffortValue)
	require.Nil(t, got.Thinking.BudgetTokens)
	require.Equal(t, "xhigh", *incoming.Thinking.ReasoningEffortValue)
	require.Equal(t, int64(1024), *incoming.Thinking.BudgetTokens)
}
