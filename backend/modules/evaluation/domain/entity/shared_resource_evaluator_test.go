// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSharedResourceConfig_EvaluatorLookup(t *testing.T) {
	const (
		sourceSpaceID = int64(100)
		callerSpaceID = int64(200)
		otherSpaceID  = int64(300)
		evaluatorID   = int64(400)
	)
	newCfg := func(rule *SharedResourceRule) *SharedResourceConfig {
		return &SharedResourceConfig{SpaceRules: map[int64]*SpaceSharedRules{
			sourceSpaceID: {Resources: []*SharedResourceRule{rule}},
		}}
	}

	t.Run("readable 精确授权命中", func(t *testing.T) {
		cfg := newCfg(&SharedResourceRule{
			ResourceID:   evaluatorID,
			ResourceType: SharedResourceTypeEvaluator,
			AccessRules: []*SharedAccessRule{
				{AccessLevel: SharedAccessLevelReadable, Targets: []string{"200"}},
			},
		})
		got := cfg.Lookup(sourceSpaceID, SharedResourceTypeEvaluator, 0, evaluatorID, callerSpaceID)
		assert.NotNil(t, got)
		assert.Equal(t, SharedAccessLevelReadable, got.AccessLevel)
	})

	t.Run("execute 通配授权命中", func(t *testing.T) {
		cfg := newCfg(&SharedResourceRule{
			ResourceID:   evaluatorID,
			ResourceType: SharedResourceTypeEvaluator,
			AccessRules: []*SharedAccessRule{
				{AccessLevel: SharedAccessLevelExecute, Targets: []string{"*"}},
			},
		})
		got := cfg.Lookup(sourceSpaceID, SharedResourceTypeEvaluator, 0, evaluatorID, callerSpaceID)
		assert.NotNil(t, got)
		assert.Equal(t, SharedAccessLevelExecute, got.AccessLevel)
	})

	t.Run("未授权给该调用方则不命中", func(t *testing.T) {
		cfg := newCfg(&SharedResourceRule{
			ResourceID:   evaluatorID,
			ResourceType: SharedResourceTypeEvaluator,
			AccessRules: []*SharedAccessRule{
				{AccessLevel: SharedAccessLevelReadable, Targets: []string{"999"}},
			},
		})
		assert.Nil(t, cfg.Lookup(sourceSpaceID, SharedResourceTypeEvaluator, 0, evaluatorID, callerSpaceID))
	})

	t.Run("资源类型不同不串号", func(t *testing.T) {
		cfg := newCfg(&SharedResourceRule{
			ResourceID:   evaluatorID,
			ResourceType: SharedResourceTypeEvalSet,
			AccessRules: []*SharedAccessRule{
				{AccessLevel: SharedAccessLevelReadable, Targets: []string{"*"}},
			},
		})
		assert.Nil(t, cfg.Lookup(sourceSpaceID, SharedResourceTypeEvaluator, 0, evaluatorID, callerSpaceID))
	})

	t.Run("配了版本策略也按 all 处理", func(t *testing.T) {
		cfg := newCfg(&SharedResourceRule{
			ResourceID:        evaluatorID,
			ResourceType:      SharedResourceTypeEvaluator,
			VersionPolicy:     SharedVersionPolicySpecified,
			SpecifiedVersions: []string{"1.0.0"},
			AccessRules: []*SharedAccessRule{
				{AccessLevel: SharedAccessLevelReadable, Targets: []string{"*"}},
			},
		})
		got := cfg.Lookup(sourceSpaceID, SharedResourceTypeEvaluator, 0, evaluatorID, callerSpaceID)
		assert.NotNil(t, got)
		assert.Equal(t, SharedVersionPolicyAll, got.VersionPolicy)
		assert.Nil(t, got.SpecifiedVersions)
	})

	t.Run("ListSharedTo 枚举与 Lookup 口径一致", func(t *testing.T) {
		cfg := newCfg(&SharedResourceRule{
			ResourceID:    evaluatorID,
			ResourceType:  SharedResourceTypeEvaluator,
			VersionPolicy: SharedVersionPolicySpecified,
			AccessRules: []*SharedAccessRule{
				{AccessLevel: SharedAccessLevelExecute, Targets: []string{"200"}},
			},
		})
		entries := cfg.ListSharedTo(callerSpaceID, SharedResourceTypeEvaluator, 0, nil)
		assert.Len(t, entries, 1)
		assert.Equal(t, SharedVersionPolicyAll, entries[0].VersionPolicy)
		assert.Empty(t, cfg.ListSharedTo(otherSpaceID, SharedResourceTypeEvaluator, 0, nil))
	})
}

func TestForceSharedAllVersions(t *testing.T) {
	assert.True(t, forceSharedAllVersions(SharedResourceTypeEvaluator, 0))
	assert.False(t, forceSharedAllVersions(SharedResourceTypeEvalSet, 0))
	assert.True(t, forceSharedAllVersions(SharedResourceTypeEvalTarget, EvalTargetTypeCozeBot))
	assert.False(t, forceSharedAllVersions(SharedResourceTypeEvalTarget, EvalTargetTypeLoopPrompt))
}
