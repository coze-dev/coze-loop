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
			name:        "nil rule is ignored",
			accessRules: []*SharedAccessRule{nil},
		},
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

func TestSharedResourceOption_Enabled(t *testing.T) {
	var option *SharedResourceOption
	assert.False(t, option.Enabled())
	assert.False(t, (&SharedResourceOption{}).Enabled())
	assert.False(t, (&SharedResourceOption{IsShared: true}).Enabled())
	assert.False(t, (&SharedResourceOption{IsShared: true, SourceSpaceID: int64Ptr(0)}).Enabled())
	assert.True(t, (&SharedResourceOption{IsShared: true, SourceSpaceID: int64Ptr(100)}).Enabled())
}

func TestBuildSharedResourceInfo(t *testing.T) {
	assert.Nil(t, BuildSharedResourceInfo(100, 0, SharedAccessLevelReadable, SharedVersionPolicyAll))
	assert.Nil(t, BuildSharedResourceInfo(100, 100, SharedAccessLevelReadable, SharedVersionPolicyAll))

	info := BuildSharedResourceInfo(100, 200, SharedAccessLevelExecute, SharedVersionPolicyLatest)
	if assert.NotNil(t, info) {
		assert.True(t, info.IsShared)
		assert.Equal(t, int64(200), info.SourceSpaceID)
		assert.Equal(t, SharedAccessLevelExecute, info.AccessLevel)
		assert.Equal(t, SharedVersionPolicyLatest, info.VersionPolicy)
	}
}

func TestSharedResourceConfig_LookupDefaultsAndRejectsInvalidRules(t *testing.T) {
	const sourceSpaceID, callerSpaceID, resourceID = int64(100), int64(200), int64(300)
	var nilConfig *SharedResourceConfig
	assert.Nil(t, nilConfig.Lookup(sourceSpaceID, SharedResourceTypeEvalSet, 0, resourceID, callerSpaceID))
	assert.Nil(t, (&SharedResourceConfig{}).Lookup(sourceSpaceID, SharedResourceTypeEvalSet, 0, resourceID, callerSpaceID))
	assert.Nil(t, (&SharedResourceConfig{SpaceRules: map[int64]*SpaceSharedRules{}}).
		Lookup(sourceSpaceID, SharedResourceTypeEvalSet, 0, resourceID, callerSpaceID))
	assert.Nil(t, (&SharedResourceConfig{SpaceRules: map[int64]*SpaceSharedRules{sourceSpaceID: nil}}).
		Lookup(sourceSpaceID, SharedResourceTypeEvalSet, 0, resourceID, callerSpaceID))

	cfg := &SharedResourceConfig{SpaceRules: map[int64]*SpaceSharedRules{
		sourceSpaceID: {
			Resources: []*SharedResourceRule{
				nil,
				{ResourceID: resourceID, ResourceType: SharedResourceTypeEvalTarget},
				{ResourceID: resourceID + 1, ResourceType: SharedResourceTypeEvalSet},
				{ResourceID: resourceID, ResourceType: SharedResourceTypeEvalSet},
			},
		},
	}}
	assert.Nil(t, cfg.Lookup(sourceSpaceID, SharedResourceTypeEvalSet, 0, resourceID, callerSpaceID))

	cfg.SpaceRules[sourceSpaceID].Resources[3].AccessRules = []*SharedAccessRule{
		{AccessLevel: SharedAccessLevelReadable, Targets: []string{strconv.FormatInt(callerSpaceID, 10)}},
	}
	share := cfg.Lookup(sourceSpaceID, SharedResourceTypeEvalSet, 0, resourceID, callerSpaceID)
	if assert.NotNil(t, share) {
		assert.Equal(t, SharedVersionPolicyAll, share.VersionPolicy)
	}
}

func TestSharedResourceConfig_ListSharedToFilteringAndDefaults(t *testing.T) {
	const sourceSpaceID, callerSpaceID = int64(100), int64(200)
	var nilConfig *SharedResourceConfig
	assert.Nil(t, nilConfig.ListSharedTo(callerSpaceID, SharedResourceTypeEvalSet, 0, nil))

	cfg := &SharedResourceConfig{SpaceRules: map[int64]*SpaceSharedRules{
		99: nil,
		sourceSpaceID: {
			Resources: []*SharedResourceRule{
				nil,
				{ResourceID: 1, ResourceType: SharedResourceTypeEvalTarget, TargetType: EvalTargetTypeLoopPrompt},
				{ResourceID: 2, ResourceType: SharedResourceTypeEvalSet},
				{
					ResourceID:   3,
					ResourceType: SharedResourceTypeEvalSet,
					AccessRules: []*SharedAccessRule{{
						AccessLevel: SharedAccessLevelReadable,
						Targets:     []string{strconv.FormatInt(callerSpaceID, 10)},
					}},
				},
				{
					ResourceID:        4,
					ResourceType:      SharedResourceTypeEvalTarget,
					TargetType:        EvalTargetTypeCozeBot,
					VersionPolicy:     SharedVersionPolicySpecified,
					SpecifiedVersions: []string{"ignored"},
					AccessRules: []*SharedAccessRule{{
						AccessLevel: SharedAccessLevelExecute,
						Targets:     []string{sharedAccessTargetAll},
					}},
				},
			},
		},
	}}

	excludedSource := int64(101)
	assert.Empty(t, cfg.ListSharedTo(callerSpaceID, SharedResourceTypeEvalSet, 0, &excludedSource))

	entries := cfg.ListSharedTo(callerSpaceID, SharedResourceTypeEvalSet, 0, int64Ptr(sourceSpaceID))
	if assert.Len(t, entries, 1) {
		assert.Equal(t, int64(3), entries[0].ResourceID)
		assert.Equal(t, SharedVersionPolicyAll, entries[0].VersionPolicy)
	}

	entries = cfg.ListSharedTo(callerSpaceID, SharedResourceTypeEvalTarget, EvalTargetTypeCozeBot, nil)
	if assert.Len(t, entries, 1) {
		assert.Equal(t, SharedVersionPolicyAll, entries[0].VersionPolicy)
		assert.Empty(t, entries[0].SpecifiedVersions)
	}
}

func TestSharedResourceMatches(t *testing.T) {
	assert.False(t, sharedResourceMatches(nil, SharedResourceTypeEvalSet, 0))
	assert.False(t, sharedResourceMatches(&SharedResourceRule{ResourceType: SharedResourceTypeEvalTarget}, SharedResourceTypeEvalSet, 0))
	assert.True(t, sharedResourceMatches(&SharedResourceRule{ResourceType: SharedResourceTypeEvalSet}, SharedResourceTypeEvalSet, 0))
	assert.False(t, sharedResourceMatches(&SharedResourceRule{ResourceType: SharedResourceTypeEvalTarget, TargetType: EvalTargetTypeCozeBot}, SharedResourceTypeEvalTarget, 0))
}

func int64Ptr(value int64) *int64 {
	return &value
}
