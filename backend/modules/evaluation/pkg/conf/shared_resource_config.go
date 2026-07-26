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
	ResourceType     string                 `mapstructure:"resource_type" json:"resource_type"`
	VersionPolicy    string                 `mapstructure:"version_policy" json:"version_policy"`
	SharedVersionIDs []int64                `mapstructure:"shared_version_ids" json:"shared_version_ids"`
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

func (p *sharedResourceConfigProvider) GetSharedResourceConfig(ctx context.Context) (*entity.SharedResourceConfig, error) {
	raw := sharedResourceConfigFile{}
	if err := p.loader.UnmarshalKey(ctx, sharedResourceConfigKey, &raw); err != nil {
		// 无配置或解析失败：返回空配置，交由上层 fail-closed。
		logs.CtxInfo(ctx, "shared resource config unset or invalid, default deny; key=%s err=%v", sharedResourceConfigKey, err)
		return &entity.SharedResourceConfig{}, nil
	}
	return convertSharedResourceConfig(raw), nil
}

func convertSharedResourceConfig(raw sharedResourceConfigFile) *entity.SharedResourceConfig {
	cfg := &entity.SharedResourceConfig{SpaceRules: make(map[int64]*entity.SpaceSharedRules, len(raw))}
	for spaceKey, spaceRules := range raw {
		sourceSpaceID, err := strconv.ParseInt(spaceKey, 10, 64)
		if err != nil {
			continue
		}
		resources := make([]*entity.SharedResourceRule, 0, len(spaceRules.Resources))
		for _, res := range spaceRules.Resources {
			accessRules := make([]*entity.SharedAccessRule, 0, len(res.AccessRules))
			for _, ar := range res.AccessRules {
				accessRules = append(accessRules, &entity.SharedAccessRule{
					AccessLevel: ar.AccessLevel,
					Targets:     ar.Targets,
				})
			}
			resources = append(resources, &entity.SharedResourceRule{
				ResourceID:    res.ResourceID,
				ResourceType:  res.ResourceType,
				VersionPolicy: res.VersionPolicy,
				SpecifiedIDs:  res.SharedVersionIDs,
				AccessRules:   accessRules,
			})
		}
		cfg.SpaceRules[sourceSpaceID] = &entity.SpaceSharedRules{Resources: resources}
	}
	return cfg
}
