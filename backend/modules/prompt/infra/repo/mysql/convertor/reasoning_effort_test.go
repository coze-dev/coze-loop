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
	for _, raw := range []*entity.ReasoningEffort{nil, ptr.Of(entity.ReasoningEffort("")), ptr.Of(entity.ReasoningEffort("XHigh")), ptr.Of(entity.ReasoningEffort("tier_v2"))} {
		cfg := &entity.ModelConfig{ModelID: 123, Extra: ptr.Of(`{"reasoning":{"summary":"auto"}}`), Thinking: &entity.ThinkingConfig{ReasoningEffort: raw}}
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
