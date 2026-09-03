// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpectedQuotaConsumption_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		conf    *ExpectedQuotaConsumption
		wantErr string // 空表示期望通过
	}{
		{
			name: "合法多资源向量",
			conf: &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
				{Category: "sandbox", ResourceKey: "default", Amount: 1},
				{Category: "model", ResourceKey: "gpt5.5", Amount: 2000},
				{Category: "evaluator", ResourceKey: "评估器A", Amount: 3},
			}},
		},
		{
			name:    "nil 拒绝",
			conf:    nil,
			wantErr: "required",
		},
		{
			name:    "空 resources 拒绝：enforce 实验必须有可预占的资源",
			conf:    &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{}},
			wantErr: "required",
		},
		{
			name:    "nil 元素拒绝",
			conf:    &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{nil}},
			wantErr: "is nil",
		},
		{
			name: "空 category 拒绝",
			conf: &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
				{Category: "  ", ResourceKey: "default", Amount: 1},
			}},
			wantErr: "category must not be empty",
		},
		{
			name: "空 resource_key 拒绝",
			conf: &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
				{Category: "sandbox", ResourceKey: "", Amount: 1},
			}},
			wantErr: "resource_key must not be empty",
		},
		{
			name: "wildcard 拒绝：通配只允许出现在上限配置",
			conf: &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
				{Category: "model", ResourceKey: "*", Amount: 1},
			}},
			wantErr: "wildcard is only allowed in quota limit config",
		},
		{
			name: "amount=0 拒绝",
			conf: &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
				{Category: "sandbox", ResourceKey: "default", Amount: 0},
			}},
			wantErr: "must be positive",
		},
		{
			name: "amount 负数拒绝",
			conf: &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
				{Category: "sandbox", ResourceKey: "default", Amount: -1},
			}},
			wantErr: "must be positive",
		},
		{
			name: "重复键拒绝：聚合时会双计但释放只释放一份",
			conf: &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
				{Category: "model", ResourceKey: "gpt5.5", Amount: 1},
				{Category: "model", ResourceKey: "gpt5.5", Amount: 2},
			}},
			wantErr: "duplicated",
		},
		{
			name: "trim 后重复也拒绝",
			conf: &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
				{Category: "model", ResourceKey: "gpt5.5", Amount: 1},
				{Category: " model ", ResourceKey: " gpt5.5 ", Amount: 2},
			}},
			wantErr: "duplicated",
		},
		{
			name: "同 category 不同 resource_key 允许",
			conf: &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
				{Category: "model", ResourceKey: "gpt5.5", Amount: 1},
				{Category: "model", ResourceKey: "doubao_pro", Amount: 2},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.conf.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestExpectedQuotaConsumption_Normalize(t *testing.T) {
	t.Parallel()

	t.Run("nil 返回 nil", func(t *testing.T) {
		t.Parallel()
		var conf *ExpectedQuotaConsumption
		assert.Nil(t, conf.Normalize())
	})

	t.Run("去空白：否则调度期拼 constraint key 匹配不上上限配置", func(t *testing.T) {
		t.Parallel()
		conf := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
			{Category: "  model ", ResourceKey: "\tgpt5.5\n", Amount: 100},
		}}
		got := conf.Normalize()
		require.Len(t, got.Resources, 1)
		assert.Equal(t, "model", got.Resources[0].Category)
		assert.Equal(t, "gpt5.5", got.Resources[0].ResourceKey)
		assert.Equal(t, int64(100), got.Resources[0].Amount)
	})

	t.Run("跳过 nil 元素且不修改原对象", func(t *testing.T) {
		t.Parallel()
		conf := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
			{Category: " sandbox ", ResourceKey: "default", Amount: 1},
			nil,
		}}
		got := conf.Normalize()
		require.Len(t, got.Resources, 1)
		assert.Equal(t, "sandbox", got.Resources[0].Category)
		// 原对象未被就地改写
		assert.Equal(t, " sandbox ", conf.Resources[0].Category)
	})
}

func TestQuotaSpaceExpt_Clone(t *testing.T) {
	t.Parallel()

	t.Run("nil 接收者返回可用空对象", func(t *testing.T) {
		t.Parallel()
		var q *QuotaSpaceExpt
		got := q.Clone()
		require.NotNil(t, got)
		require.NotNil(t, got.ExptID2RunTime)
		got.ExptID2RunTime[1] = 100 // 可直接写入，不 panic
	})

	t.Run("nil map 也返回可写 map", func(t *testing.T) {
		t.Parallel()
		got := (&QuotaSpaceExpt{}).Clone()
		require.NotNil(t, got.ExptID2RunTime)
		got.ExptID2RunTime[1] = 100
	})

	t.Run("深拷贝：改副本不影响原对象", func(t *testing.T) {
		t.Parallel()
		orig := &QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{1: 100, 2: 200}}
		cloned := orig.Clone()

		cloned.ExptID2RunTime[3] = 300
		delete(cloned.ExptID2RunTime, 1)

		assert.Equal(t, map[int64]int64{1: 100, 2: 200}, orig.ExptID2RunTime)
		assert.Equal(t, map[int64]int64{2: 200, 3: 300}, cloned.ExptID2RunTime)
	})
}

// TestExpectedQuotaConsumption_Categories 去重 + TrimSpace + nil 安全。
//
// 去重是必需的：同 category 下申报多个具体资源是正常形态（model|A + model|B），
// 不去重会让 admission policy 对同一 category 反复判定、错误信息里也重复列出。
func TestExpectedQuotaConsumption_Categories(t *testing.T) {
	t.Run("nil 与空返回 nil/空", func(t *testing.T) {
		var nilC *ExpectedQuotaConsumption
		assert.Nil(t, nilC.Categories(), "nil receiver 必须安全 —— 调用方在 policy 不放行时会传 nil")
		assert.Empty(t, (&ExpectedQuotaConsumption{}).Categories())
	})

	t.Run("去重且保持首次出现顺序", func(t *testing.T) {
		c := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
			{Category: "model", ResourceKey: "a", Amount: 1},
			{Category: "sandbox", ResourceKey: "mac", Amount: 1},
			{Category: "model", ResourceKey: "b", Amount: 1}, // 同 category 第二个资源
		}}
		assert.Equal(t, []string{"model", "sandbox"}, c.Categories())
	})

	t.Run("TrimSpace 且跳过空 category 与 nil 元素", func(t *testing.T) {
		c := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
			{Category: "  model  ", ResourceKey: "a", Amount: 1},
			nil,
			{Category: "   ", ResourceKey: "b", Amount: 1},
			{Category: "model", ResourceKey: "c", Amount: 1}, // trim 后与第一个同名，应去重
		}}
		assert.Equal(t, []string{"model"}, c.Categories(),
			"trim 后同名必须去重 —— 否则 policy 会把同一 category 判两次")
	})
}

// ---------------------------------------------------------------------------
// source
// ---------------------------------------------------------------------------

// TestValidate_SourceRules source 可选、不得为通配、去重键必须含它。
//
// ★ 去重键含 source 是本组最要紧的一条：同一 (category, resource_key) 经不同来源是
// **两个独立的池子**（同一模型走 LiteLLM 与走业务方自备通道，配额各自独立）。
// 去重键不带 source 会把这个合法形态误判成"重复键"而拒绝创建 —— 那正是加该字段要支持的用法。
func TestValidate_SourceRules(t *testing.T) {
	t.Run("同资源分来源申报是合法的", func(t *testing.T) {
		c := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
			{Category: "model", ResourceKey: "kimi-k3", Amount: 6, Source: "litellm"},
			{Category: "model", ResourceKey: "kimi-k3", Amount: 6, Source: "self-hosted"},
		}}
		assert.NoError(t, c.Validate(), "同一模型不同来源是两个池子，不得判成重复键")
	})

	t.Run("有无 source 也算不同键", func(t *testing.T) {
		c := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
			{Category: "model", ResourceKey: "kimi-k3", Amount: 6},
			{Category: "model", ResourceKey: "kimi-k3", Amount: 6, Source: "litellm"},
		}}
		assert.NoError(t, c.Validate())
	})

	t.Run("同来源重复仍要拒绝", func(t *testing.T) {
		c := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
			{Category: "model", ResourceKey: "kimi-k3", Amount: 6, Source: "litellm"},
			{Category: "model", ResourceKey: "kimi-k3", Amount: 6, Source: "litellm"},
		}}
		err := c.Validate()
		require.Error(t, err, "完全相同的三元组必须判重 —— 否则按 key 聚合会双计、释放只释放一份")
		assert.Contains(t, err.Error(), "litellm", "错误信息要能定位到是哪个键重复")
	})

	t.Run("source 不得为通配", func(t *testing.T) {
		c := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
			{Category: "model", ResourceKey: "kimi-k3", Amount: 6, Source: WildcardResourceKey},
		}}
		require.Error(t, c.Validate(), "申报了通配来源就无从按 key 释放")
	})

	t.Run("空 source 合法（等于不区分来源）", func(t *testing.T) {
		c := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
			{Category: "model", ResourceKey: "kimi-k3", Amount: 6},
		}}
		assert.NoError(t, c.Validate())
	})
}

// TestNormalize_TrimsSource source 进账本 key，带空白会与上限配置里的同名来源匹配不上。
func TestNormalize_TrimsSource(t *testing.T) {
	c := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
		{Category: " model ", ResourceKey: " kimi-k3 ", Amount: 6, Source: "  litellm  "},
	}}
	got := c.Normalize()
	require.Len(t, got.Resources, 1)
	assert.Equal(t, "model", got.Resources[0].Category)
	assert.Equal(t, "kimi-k3", got.Resources[0].ResourceKey)
	assert.Equal(t, "litellm", got.Resources[0].Source)
}
