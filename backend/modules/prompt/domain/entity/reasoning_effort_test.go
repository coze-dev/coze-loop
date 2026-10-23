// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReasoningEffort_SavePreservesPartialThinking(t *testing.T) {
	previous := &ModelConfig{ModelID: 123, Thinking: &ThinkingConfig{ThinkingOption: ptr.Of(ThinkingOptionEnabled), BudgetTokens: ptr.Of(int64(2048)), ReasoningEffortValue: ptr.Of("tier_v2")}}
	got, err := PreserveModelConfigForSave(&ModelConfig{ModelID: 123, Thinking: &ThinkingConfig{ReasoningEffortValue: ptr.Of("")}}, previous)
	require.NoError(t, err)
	assert.Equal(t, ptr.Of(ThinkingOptionEnabled), got.Thinking.ThinkingOption)
	assert.Equal(t, ptr.Of(""), got.Thinking.ReasoningEffortValue)
	assert.Equal(t, ptr.Of(int64(2048)), got.Thinking.BudgetTokens)
	assert.Equal(t, ptr.Of("tier_v2"), previous.Thinking.ReasoningEffortValue)
	changed, err := PreserveModelConfigForSave(&ModelConfig{ModelID: 456}, previous)
	require.NoError(t, err)
	assert.Nil(t, changed.Thinking)
}
