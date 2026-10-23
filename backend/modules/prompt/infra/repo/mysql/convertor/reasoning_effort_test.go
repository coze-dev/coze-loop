// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReasoningEffort_DraftCommitSnapshotPreservesString(t *testing.T) {
	for _, raw := range []*string{nil, ptr.Of(""), ptr.Of("XHigh"), ptr.Of("tier_v2")} {
		cfg := &entity.ModelConfig{ModelID: 123, Extra: ptr.Of(`{"reasoning":{"summary":"auto"},"metadata":{"sequence":9007199254740993}}`), Thinking: &entity.ThinkingConfig{ReasoningEffortValue: raw}}
		p := &entity.Prompt{ID: 42, SpaceID: 100, PromptDraft: &entity.PromptDraft{DraftInfo: &entity.DraftInfo{UserID: "u", BaseVersion: "2"}, PromptDetail: &entity.PromptDetail{ModelConfig: cfg}}, PromptCommit: &entity.PromptCommit{CommitInfo: &entity.CommitInfo{Version: "3"}, PromptDetail: &entity.PromptDetail{ModelConfig: cfg}}}
		draftPO := PromptDO2DraftPO(p)
		require.NotNil(t, draftPO.ModelConfig)
		storedDraft := DraftPO2DO(draftPO)
		assert.Equal(t, cfg, storedDraft.PromptDetail.ModelConfig)
		commitPO := PromptDO2CommitPO(p)
		require.NotNil(t, commitPO)
		storedCommit := CommitPO2DO(commitPO)
		assert.Equal(t, cfg, storedCommit.PromptDetail.ModelConfig)
		assert.Equal(t, "3", storedCommit.CommitInfo.Version)
	}
}

func TestLegacyDraftStorageDoesNotInjectValuePresence(t *testing.T) {
	cfg := &entity.ModelConfig{ModelID: 123, Thinking: &entity.ThinkingConfig{ReasoningEffort: ptr.Of(entity.ReasoningEffortHigh)}}
	p := &entity.Prompt{ID: 42, SpaceID: 100, PromptDraft: &entity.PromptDraft{DraftInfo: &entity.DraftInfo{UserID: "u", BaseVersion: "2"}, PromptDetail: &entity.PromptDetail{ModelConfig: cfg}}}
	stored := DraftPO2DO(PromptDO2DraftPO(p))
	require.NotNil(t, stored.PromptDetail.ModelConfig.Thinking)
	assert.Nil(t, stored.PromptDetail.ModelConfig.Thinking.ReasoningEffortValue)
	assert.Equal(t, entity.ReasoningEffortHigh, *stored.PromptDetail.ModelConfig.Thinking.ReasoningEffort)
}
