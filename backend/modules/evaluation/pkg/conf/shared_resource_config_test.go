// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestConvertSharedResourceConfig_ExpandsResourceIDs(t *testing.T) {
	raw := sharedResourceConfigFile{
		"1001": {
			Resources: []sharedResourceRuleFile{
				{
					ResourceID:       2001,
					ResourceIDs:      []int64{2001, 2002, 0, -1, 2003, 2002},
					ResourceType:     entity.SharedResourceTypeEvalSet,
					VersionPolicy:    entity.SharedVersionPolicyLatest,
					SharedVersionIDs: []int64{3001},
					AccessRules: []sharedAccessRuleFile{
						{
							AccessLevel: entity.SharedAccessLevelReadable,
							Targets:     []string{"4001"},
						},
					},
				},
				{
					ResourceIDs:  []int64{0, -1},
					ResourceType: entity.SharedResourceTypeEvalSet,
				},
			},
		},
	}

	cfg := convertSharedResourceConfig(raw)
	require.NotNil(t, cfg)
	spaceRules := cfg.SpaceRules[1001]
	require.NotNil(t, spaceRules)
	require.Len(t, spaceRules.Resources, 3)

	assert.Equal(t, []int64{2001, 2002, 2003}, []int64{
		spaceRules.Resources[0].ResourceID,
		spaceRules.Resources[1].ResourceID,
		spaceRules.Resources[2].ResourceID,
	})
	for _, resource := range spaceRules.Resources {
		assert.Equal(t, entity.SharedResourceTypeEvalSet, resource.ResourceType)
		assert.Equal(t, entity.SharedVersionPolicyLatest, resource.VersionPolicy)
		assert.Equal(t, []int64{3001}, resource.SpecifiedIDs)
		if assert.Len(t, resource.AccessRules, 1) {
			assert.Equal(t, entity.SharedAccessLevelReadable, resource.AccessRules[0].AccessLevel)
			assert.Equal(t, []string{"4001"}, resource.AccessRules[0].Targets)
		}
	}
}

func TestConvertSharedResourceConfig_LegacyResourceID(t *testing.T) {
	raw := sharedResourceConfigFile{
		"1001": {
			Resources: []sharedResourceRuleFile{
				{
					ResourceID:    2001,
					ResourceType:  entity.SharedResourceTypeEvalSet,
					VersionPolicy: entity.SharedVersionPolicyAll,
				},
			},
		},
	}

	cfg := convertSharedResourceConfig(raw)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.SpaceRules[1001])
	require.Len(t, cfg.SpaceRules[1001].Resources, 1)
	assert.Equal(t, int64(2001), cfg.SpaceRules[1001].Resources[0].ResourceID)
}

func TestConvertSharedResourceConfig_RejectsCrossRuleDuplicate(t *testing.T) {
	raw := sharedResourceConfigFile{
		"1001": {
			Resources: []sharedResourceRuleFile{
				{
					ResourceIDs:  []int64{2001, 2002},
					ResourceType: entity.SharedResourceTypeEvalSet,
					AccessRules: []sharedAccessRuleFile{
						{AccessLevel: entity.SharedAccessLevelReadable, Targets: []string{"3001"}},
					},
				},
				{
					ResourceIDs:  []int64{2002, 2003},
					ResourceType: entity.SharedResourceTypeEvalSet,
					AccessRules: []sharedAccessRuleFile{
						{AccessLevel: entity.SharedAccessLevelReadable, Targets: []string{"3002"}},
					},
				},
			},
		},
	}

	cfg := convertSharedResourceConfig(raw)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.SpaceRules[1001])
	require.Len(t, cfg.SpaceRules[1001].Resources, 2)
	assert.Equal(t, int64(2001), cfg.SpaceRules[1001].Resources[0].ResourceID)
	assert.Equal(t, int64(2003), cfg.SpaceRules[1001].Resources[1].ResourceID)

	assert.Nil(t, cfg.Lookup(1001, entity.SharedResourceTypeEvalSet, 0, 2002, 3001))
	assert.Nil(t, cfg.Lookup(1001, entity.SharedResourceTypeEvalSet, 0, 2002, 3002))
	listed := cfg.ListSharedTo(3001, entity.SharedResourceTypeEvalSet, 0, nil)
	require.Len(t, listed, 1)
	assert.Equal(t, int64(2001), listed[0].ResourceID)
	listed = cfg.ListSharedTo(3002, entity.SharedResourceTypeEvalSet, 0, nil)
	require.Len(t, listed, 1)
	assert.Equal(t, int64(2003), listed[0].ResourceID)
}

func TestSharedResourceConfigFile_UnmarshalResourceIDs(t *testing.T) {
	const rawJSON = `{
		"1001": {
			"resources": [{
				"resource_ids": [2001, 2002],
				"resource_type": "eval_set",
				"version_policy": "latest",
				"access_rules": [{
					"access_level": "readable",
					"targets": ["3001"]
				}]
			}]
		}
	}`

	var raw sharedResourceConfigFile
	require.NoError(t, json.Unmarshal([]byte(rawJSON), &raw))
	cfg := convertSharedResourceConfig(raw)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.SpaceRules[1001])
	require.Len(t, cfg.SpaceRules[1001].Resources, 2)
	assert.Equal(t, int64(2001), cfg.SpaceRules[1001].Resources[0].ResourceID)
	assert.Equal(t, int64(2002), cfg.SpaceRules[1001].Resources[1].ResourceID)
}

func TestCollectResourceIDs(t *testing.T) {
	assert.Equal(t, []int64{1, 2, 3}, collectResourceIDs(1, []int64{1, 2, 0, -1, 3, 2}))
	assert.Equal(t, []int64{2, 3}, collectResourceIDs(0, []int64{2, 3}))
	assert.Empty(t, collectResourceIDs(0, nil))
}
