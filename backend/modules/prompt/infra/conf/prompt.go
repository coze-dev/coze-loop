// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"
	"time"

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

type promptLabelVersionCacheConfig struct {
	Enable     bool `mapstructure:"enable"`
	TTLSeconds int  `mapstructure:"ttl_seconds"`
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

// ListPresetLabels returns a list of preset labels from configuration
func (c *PromptConfigProvider) ListPresetLabels() (presetLabels []string, err error) {
	const PresetLabelsConfigKey = "preset_labels"
	err = c.ConfigLoader.UnmarshalKey(context.Background(), PresetLabelsConfigKey, &presetLabels)
	if err != nil {
		return nil, err
	}
	return presetLabels, nil
}

// GetPromptLabelVersionCacheConfig returns the cache configuration for prompt label versions
func (c *PromptConfigProvider) GetPromptLabelVersionCacheConfig(ctx context.Context) (enable bool, ttl time.Duration, err error) {
	const PromptLabelVersionCacheConfigKey = "prompt_label_version_cache"
	config := &promptLabelVersionCacheConfig{}
	err = c.ConfigLoader.UnmarshalKey(ctx, PromptLabelVersionCacheConfigKey, config)
	if err != nil {
		return false, 0, err
	}
	return config.Enable, time.Duration(config.TTLSeconds) * time.Second, nil
}

