// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSharedResourceConfig_EvalTargetMatchesTargetType(t *testing.T) {
	const (
		sourceSpaceID = int64(100)
		callerSpaceID = int64(200)
		resourceID    = int64(300)
	)
	cfg := &SharedResourceConfig{
		SpaceRules: map[int64]*SpaceSharedRules{
			sourceSpaceID: {
				Resources: []*SharedResourceRule{
					{
						ResourceID:        resourceID,
						ResourceType:      SharedResourceTypeEvalTarget,
						TargetType:        EvalTargetTypeLoopPrompt,
						VersionPolicy:     SharedVersionPolicySpecified,
						SpecifiedVersions: []string{"v1"},
						AccessRules: []*SharedAccessRule{
							{AccessLevel: SharedAccessLevelReadable, Targets: []string{strconv.FormatInt(callerSpaceID, 10)}},
						},
					},
				},
			},
		},
	}

	assert.Nil(t, cfg.Lookup(sourceSpaceID, SharedResourceTypeEvalTarget, EvalTargetTypeCozeBot, resourceID, callerSpaceID))

	share := cfg.Lookup(sourceSpaceID, SharedResourceTypeEvalTarget, EvalTargetTypeLoopPrompt, resourceID, callerSpaceID)
	if assert.NotNil(t, share) {
		assert.Equal(t, EvalTargetTypeLoopPrompt, share.TargetType)
		assert.Equal(t, SharedVersionPolicySpecified, share.VersionPolicy)
		assert.Equal(t, []string{"v1"}, share.SpecifiedVersions)
	}

	entries := cfg.ListSharedTo(callerSpaceID, SharedResourceTypeEvalTarget, EvalTargetTypeCozeBot, nil)
	assert.Empty(t, entries)

	entries = cfg.ListSharedTo(callerSpaceID, SharedResourceTypeEvalTarget, EvalTargetTypeLoopPrompt, nil)
	if assert.Len(t, entries, 1) {
		assert.Equal(t, EvalTargetTypeLoopPrompt, entries[0].TargetType)
	}
}

func TestSharedResourceConfig_NonPromptEvalTargetIgnoresVersionPolicy(t *testing.T) {
	const (
		sourceSpaceID = int64(100)
		callerSpaceID = int64(200)
		resourceID    = int64(300)
	)
	cfg := &SharedResourceConfig{
		SpaceRules: map[int64]*SpaceSharedRules{
			sourceSpaceID: {
				Resources: []*SharedResourceRule{
					{
						ResourceID:        resourceID,
						ResourceType:      SharedResourceTypeEvalTarget,
						TargetType:        EvalTargetTypeCozeBot,
						VersionPolicy:     SharedVersionPolicySpecified,
						SpecifiedVersions: []string{"v1"},
						AccessRules: []*SharedAccessRule{
							{AccessLevel: SharedAccessLevelReadable, Targets: []string{strconv.FormatInt(callerSpaceID, 10)}},
						},
					},
				},
			},
		},
	}

	share := cfg.Lookup(sourceSpaceID, SharedResourceTypeEvalTarget, EvalTargetTypeCozeBot, resourceID, callerSpaceID)
	if assert.NotNil(t, share) {
		assert.Equal(t, SharedVersionPolicyAll, share.VersionPolicy)
		assert.Empty(t, share.SpecifiedVersions)
	}
}

func TestMatchAccessLevel(t *testing.T) {
	callerKey := "200"

	tests := []struct {
		name        string
		accessRules []*SharedAccessRule
		wantLevel   string
		wantMatched bool
	}{
		{
			name:        "unknown level with exact target is denied",
			accessRules: []*SharedAccessRule{{AccessLevel: "read", Targets: []string{callerKey}}},
		},
		{
			name:        "unknown level with wildcard is denied",
			accessRules: []*SharedAccessRule{{AccessLevel: "executor", Targets: []string{sharedAccessTargetAll}}},
		},
		{
			name:        "empty level is denied",
			accessRules: []*SharedAccessRule{{Targets: []string{callerKey}}},
		},
		{
			name: "readable exact target",
			accessRules: []*SharedAccessRule{{
				AccessLevel: SharedAccessLevelReadable,
				Targets:     []string{callerKey},
			}},
			wantLevel:   SharedAccessLevelReadable,
			wantMatched: true,
		},
		{
			name: "readable wildcard",
			accessRules: []*SharedAccessRule{{
				AccessLevel: SharedAccessLevelReadable,
				Targets:     []string{sharedAccessTargetAll},
			}},
			wantLevel:   SharedAccessLevelReadable,
			wantMatched: true,
		},
		{
			name: "execute exact target",
			accessRules: []*SharedAccessRule{{
				AccessLevel: SharedAccessLevelExecute,
				Targets:     []string{callerKey},
			}},
			wantLevel:   SharedAccessLevelExecute,
			wantMatched: true,
		},
		{
			name: "execute wildcard",
			accessRules: []*SharedAccessRule{{
				AccessLevel: SharedAccessLevelExecute,
				Targets:     []string{sharedAccessTargetAll},
			}},
			wantLevel:   SharedAccessLevelExecute,
			wantMatched: true,
		},
		{
			name: "readable takes precedence over execute",
			accessRules: []*SharedAccessRule{
				{AccessLevel: SharedAccessLevelExecute, Targets: []string{sharedAccessTargetAll}},
				{AccessLevel: SharedAccessLevelReadable, Targets: []string{callerKey}},
			},
			wantLevel:   SharedAccessLevelReadable,
			wantMatched: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, matched := matchAccessLevel(tt.accessRules, callerKey)
			assert.Equal(t, tt.wantLevel, level)
			assert.Equal(t, tt.wantMatched, matched)
		})
	}
}
