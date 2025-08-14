// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"

	promptconf "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

type PromptConfigProvider struct {
	ConfigLoader conf.IConfigLoader
}

func NewPromptConfigProvider(factory conf.IConfigLoaderFactory) (promptconf.IConfigProvider, error) {
	configLoader, err := factory.NewConfigLoader("prompt.yaml")
	if err != nil {
		return nil, err
	}
	return &PromptConfigProvider{
		ConfigLoader: configLoader,
	}, nil
}

type promptHubRateLimitConfig struct {
	DefaultMaxQPS int           `mapstructure:"default_max_qps"`
	SpaceMaxQPS   map[int64]int `mapstructure:"space_max_qps"`
}

func (c *PromptConfigProvider) GetPromptHubMaxQPSBySpace(ctx context.Context, spaceID int64) (maxQPS int, err error) {
	const PromptHubRateLimitConfigKey = "prompt_hub_rate_limit_config"
	config := &promptHubRateLimitConfig{}
	err = c.ConfigLoader.UnmarshalKey(ctx, PromptHubRateLimitConfigKey, config)
	if err != nil {
		return 0, err
	}
	if qps, ok := config.SpaceMaxQPS[spaceID]; ok {
		return qps, nil
	}
	return config.DefaultMaxQPS, nil
}

func (c *PromptConfigProvider) GetPromptDefaultConfig(ctx context.Context) (config *prompt.PromptDetail, err error) {
	return nil, nil
}
