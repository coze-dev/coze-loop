// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"strings"
)

// WildcardResourceKey 通配资源键。只允许出现在额度**上限配置**里（表示"该 category 下所有资源"），
// 调用方申报单 item 消耗时不得使用 —— 否则无法确定实际占用的是哪个具体资源，账本也无从按 key 释放。
const WildcardResourceKey = "*"

// Validate 校验单 item 消耗向量的结构合法性。
//
// 在创建/提交阶段同步校验并返回参数错误，不把问题拖到异步调度阶段：调度期发现向量非法只能让实验干等或
// 误置失败，用户既看不到原因也无法修正。
//
// 校验项与 DB/账本的前置条件一一对应：
//   - resources 非空：enforce 实验必须有可预占的资源，空向量意味着无限制占用
//   - category / resource_key 去空白后非空：空串会污染 constraint key，导致账本 key 冲撞
//   - resource_key != "*"：见 WildcardResourceKey
//   - amount > 0：0 或负数会让 maxGrant 的木桶计算失去意义（除零 / 负额度）
//   - (category, resource_key) 唯一：重复键在按 constraint key 聚合时会双计，释放时又只释放一份
func (c *ExpectedQuotaConsumption) Validate() error {
	if c == nil || len(c.Resources) == 0 {
		return fmt.Errorf("expected_quota_consumption is required and must not be empty")
	}

	seen := make(map[string]struct{}, len(c.Resources))
	for i, r := range c.Resources {
		if r == nil {
			return fmt.Errorf("expected_quota_consumption.resources[%d] is nil", i)
		}

		category := strings.TrimSpace(r.Category)
		resourceKey := strings.TrimSpace(r.ResourceKey)

		if category == "" {
			return fmt.Errorf("expected_quota_consumption.resources[%d].category must not be empty", i)
		}
		if resourceKey == "" {
			return fmt.Errorf("expected_quota_consumption.resources[%d].resource_key must not be empty", i)
		}
		if resourceKey == WildcardResourceKey {
			return fmt.Errorf("expected_quota_consumption.resources[%d].resource_key must not be %q; wildcard is only allowed in quota limit config", i, WildcardResourceKey)
		}
		if r.Amount <= 0 {
			return fmt.Errorf("expected_quota_consumption.resources[%d].amount must be positive, got %d", i, r.Amount)
		}

		dedupKey := category + "|" + resourceKey
		if _, dup := seen[dedupKey]; dup {
			return fmt.Errorf("expected_quota_consumption has duplicated (category,resource_key): %s", dedupKey)
		}
		seen[dedupKey] = struct{}{}
	}

	return nil
}

// Normalize 返回一份 category/resource_key 已去空白的副本，供落库前调用。
// 冻结进 eval_conf 的值必须是规范形态：调度期按 category|resource_key 拼 constraint key 时不再 trim，
// 若带前后空白会与上限配置中的同名资源匹配不上，静默变成"未登记资源"而被放行。
func (c *ExpectedQuotaConsumption) Normalize() *ExpectedQuotaConsumption {
	if c == nil {
		return nil
	}
	normalized := &ExpectedQuotaConsumption{Resources: make([]*ExpectedResourceConsumption, 0, len(c.Resources))}
	for _, r := range c.Resources {
		if r == nil {
			continue
		}
		normalized.Resources = append(normalized.Resources, &ExpectedResourceConsumption{
			Category:    strings.TrimSpace(r.Category),
			ResourceKey: strings.TrimSpace(r.ResourceKey),
			Amount:      r.Amount,
		})
	}
	return normalized
}
