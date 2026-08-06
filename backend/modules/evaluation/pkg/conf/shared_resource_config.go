// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// sharedResourceConfigKey OSS 侧本地共享配置的键名。
// 结构与技术方案的 TCC 配置同构，但不含任何 TCC 类型/命名空间；纯 OSS 默认无该配置 → fail-closed。
const sharedResourceConfigKey = "cross_space_sharing_resource_level"

// sharedResourceConfigFile 描述本地/动态配置里的共享资源配置形态：
// { "<source_space_id>": { "resources": [ { ... } ] } }
type sharedResourceConfigFile map[string]sharedSpaceRulesFile

type sharedSpaceRulesFile struct {
	Resources []sharedResourceRuleFile `mapstructure:"resources" json:"resources"`
}

type sharedResourceRuleFile struct {
	ResourceID       int64                  `mapstructure:"resource_id" json:"resource_id"`
	ResourceIDs      []int64                `mapstructure:"resource_ids" json:"resource_ids"`
	ResourceType     string                 `mapstructure:"resource_type" json:"resource_type"`
	TargetType       entity.EvalTargetType  `mapstructure:"target_type" json:"target_type"`
	VersionPolicy    string                 `mapstructure:"version_policy" json:"version_policy"`
	SharedVersionIDs []int64                `mapstructure:"shared_version_ids" json:"shared_version_ids"`
	SharedVersions   []string               `mapstructure:"shared_versions" json:"shared_versions"`
	AccessRules      []sharedAccessRuleFile `mapstructure:"access_rules" json:"access_rules"`
}

type sharedAccessRuleFile struct {
	AccessLevel string   `mapstructure:"access_level" json:"access_level"`
	Targets     []string `mapstructure:"targets" json:"targets"`
}

// NewSharedResourceConfigProvider 返回 OSS 默认共享配置提供者。
// 默认读取本地配置键 cross_space_sharing_resource_level；无配置/解析失败时返回空配置，
// 上层据此 fail-closed（拒绝所有跨空间共享读取）。商业版通过 wire 绑定读 TCC 的实现覆盖之。
func NewSharedResourceConfigProvider(configFactory conf.IConfigLoaderFactory) (component.SharedResourceConfigProvider, error) {
	loader, err := configFactory.NewConfigLoader(consts.EvaluationConfigFileName)
	if err != nil {
		return nil, err
	}
	return &sharedResourceConfigProvider{loader: loader}, nil
}

type sharedResourceConfigProvider struct {
	loader conf.IConfigLoader
}

type sharedResourceRuleKey struct {
	ResourceID   int64
	ResourceType string
	TargetType   entity.EvalTargetType
}

type expandedSharedResourceRule struct {
	key  sharedResourceRuleKey
	rule *entity.SharedResourceRule
}

func (p *sharedResourceConfigProvider) GetSharedResourceConfig(ctx context.Context) (*entity.SharedResourceConfig, error) {
	raw := sharedResourceConfigFile{}
	if err := p.loader.UnmarshalKey(ctx, sharedResourceConfigKey, &raw); err != nil {
		// 无配置或解析失败：返回空配置，交由上层 fail-closed。
		logs.CtxInfo(ctx, "shared resource config unset or invalid, default deny; key=%s err=%v", sharedResourceConfigKey, err)
		return &entity.SharedResourceConfig{}, nil
	}
	cfg := convertSharedResourceConfig(raw)
	// 稳定性运维日志: 记录读到的跨空间共享规则规模(来源空间数), 便于确认 TCC 已生效、区分"未配置(默认拒绝)"与"配置为空"。
	logs.CtxInfo(ctx, "shared resource config loaded; key=%s sourceSpaces=%d", sharedResourceConfigKey, len(cfg.SpaceRules))
	return cfg, nil
}

func convertSharedResourceConfig(raw sharedResourceConfigFile) *entity.SharedResourceConfig {
	cfg := &entity.SharedResourceConfig{SpaceRules: make(map[int64]*entity.SpaceSharedRules, len(raw))}
	for spaceKey, spaceRules := range raw {
		sourceSpaceID, err := strconv.ParseInt(spaceKey, 10, 64)
		if err != nil {
			continue
		}
		expandedRules := make([]expandedSharedResourceRule, 0, len(spaceRules.Resources))
		ruleCounts := make(map[sharedResourceRuleKey]int, len(spaceRules.Resources))
		resources := make([]*entity.SharedResourceRule, 0, len(spaceRules.Resources))
		for _, res := range spaceRules.Resources {
			accessRules := make([]*entity.SharedAccessRule, 0, len(res.AccessRules))
			for _, ar := range res.AccessRules {
				accessRules = append(accessRules, &entity.SharedAccessRule{
					AccessLevel: ar.AccessLevel,
					Targets:     ar.Targets,
				})
			}
			for _, resourceID := range collectResourceIDs(res.ResourceID, res.ResourceIDs) {
				key := sharedResourceRuleKey{
					ResourceID:   resourceID,
					ResourceType: res.ResourceType,
					TargetType:   res.TargetType,
				}
				expandedRules = append(expandedRules, expandedSharedResourceRule{key: key, rule: &entity.SharedResourceRule{
					ResourceID:        resourceID,
					ResourceType:      res.ResourceType,
					TargetType:        res.TargetType,
					VersionPolicy:     res.VersionPolicy,
					SpecifiedIDs:      res.SharedVersionIDs,
					SpecifiedVersions: res.SharedVersions,
					AccessRules:       accessRules,
				}})
				ruleCounts[key]++
			}
		}
		for _, expanded := range expandedRules {
			// Multiple config entries for the same resource can carry conflicting
			// policies and make list/detail authorization disagree. Treat every
			// cross-entry duplicate as invalid instead of depending on rule order.
			if ruleCounts[expanded.key] != 1 {
				continue
			}
			resources = append(resources, expanded.rule)
		}
		cfg.SpaceRules[sourceSpaceID] = &entity.SpaceSharedRules{Resources: resources}
	}
	return cfg
}

// collectResourceIDs merges the legacy single resource_id with resource_ids.
// Invalid IDs are ignored and duplicates keep their first occurrence.
func collectResourceIDs(resourceID int64, resourceIDs []int64) []int64 {
	ids := make([]int64, 0, len(resourceIDs)+1)
	seen := make(map[int64]struct{}, len(resourceIDs)+1)
	appendID := func(id int64) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	appendID(resourceID)
	for _, id := range resourceIDs {
		appendID(id)
	}
	return ids
}
