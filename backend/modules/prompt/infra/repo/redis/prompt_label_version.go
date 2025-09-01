// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"
	"fmt"

	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/infra/redis"
	promptconf "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf"
)

//go:generate mockgen -destination=mocks/prompt_label_version_dao.go -package=mocks . IPromptLabelVersionDAO
type IPromptLabelVersionDAO interface {
	// MGet 批量获取prompt label版本缓存
	// 根据space_id, prompt_key, label_key的组合查询对应的版本号
	MGet(ctx context.Context, queries []PromptLabelVersionQuery) (versionMap map[PromptLabelVersionQuery]string, err error)

	// MSet 批量设置prompt label版本缓存
	// 根据space_id, prompt_key, label_key设置对应的版本号
	MSet(ctx context.Context, mappings []PromptLabelVersionMapping) error

	// MDel 批量删除prompt label版本缓存
	// 根据space_id, prompt_key, label_key的组合删除缓存
	MDel(ctx context.Context, queries []PromptLabelVersionQuery) error
}

type PromptLabelVersionQuery struct {
	PromptID int64
	LabelKey string
}

type PromptLabelVersionMapping struct {
	PromptID int64
	LabelKey string
	Version  string
}

type PromptLabelVersionDAOImpl struct {
	redis          redis.Cmdable
	configProvider promptconf.IConfigProvider
}

func NewPromptLabelVersionDAO(redisCli redis.Cmdable, configProvider promptconf.IConfigProvider) IPromptLabelVersionDAO {
	return &PromptLabelVersionDAOImpl{
		redis:          redisCli,
		configProvider: configProvider,
	}
}

func (p *PromptLabelVersionDAOImpl) MGet(ctx context.Context, queries []PromptLabelVersionQuery) (versionMap map[PromptLabelVersionQuery]string, err error) {
	if len(queries) == 0 {
		return nil, nil
	}

	// 检查缓存是否启用
	enable, _, err := p.configProvider.GetPromptLabelVersionCacheConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !enable {
		return nil, nil
	}

	// 构建缓存key列表
	keys := gslice.Map(queries, func(query PromptLabelVersionQuery) string {
		return formatPromptLabelVersionKey(query.PromptID, query.LabelKey)
	})

	// 批量获取缓存值
	values, err := p.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	// 构建结果映射
	versionMap = make(map[PromptLabelVersionQuery]string)
	for i, value := range values {
		if i >= len(queries) {
			break
		}
		if value != nil {
			if versionStr, ok := value.(string); ok {
				versionMap[queries[i]] = versionStr
			}
		}
	}

	return versionMap, nil
}

func (p *PromptLabelVersionDAOImpl) MSet(ctx context.Context, mappings []PromptLabelVersionMapping) error {
	if len(mappings) == 0 {
		return nil
	}

	// 检查缓存是否启用并获取TTL
	enable, ttl, err := p.configProvider.GetPromptLabelVersionCacheConfig(ctx)
	if err != nil {
		return err
	}
	if !enable {
		return nil
	}

	// 使用pipeline批量设置缓存
	pipe := p.redis.Pipeline()
	for _, mapping := range mappings {
		key := formatPromptLabelVersionKey(mapping.PromptID, mapping.LabelKey)
		pipe.Set(ctx, key, mapping.Version, ttl)
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (p *PromptLabelVersionDAOImpl) MDel(ctx context.Context, queries []PromptLabelVersionQuery) error {
	if len(queries) == 0 {
		return nil
	}

	// 检查缓存是否启用
	enable, _, err := p.configProvider.GetPromptLabelVersionCacheConfig(ctx)
	if err != nil {
		return err
	}
	if !enable {
		return nil
	}

	// 构建缓存key列表
	keys := gslice.Map(queries, func(query PromptLabelVersionQuery) string {
		return formatPromptLabelVersionKey(query.PromptID, query.LabelKey)
	})

	// 批量删除缓存
	err = p.redis.Del(ctx, keys...).Err()
	if err != nil {
		return err
	}

	return nil
}

const (
	// promptLabelVersionKey 格式: prompt_label_version:space_id:prompt_id:label_key
	// 存储特定label下的版本号
	promptLabelVersionKey = `prompt_label_version:prompt_id=%d:label_key=%s`
)

// formatPromptLabelVersionKey 格式化prompt label version的缓存key
func formatPromptLabelVersionKey(promptID int64, labelKey string) string {
	return fmt.Sprintf(promptLabelVersionKey, promptID, labelKey)
}
