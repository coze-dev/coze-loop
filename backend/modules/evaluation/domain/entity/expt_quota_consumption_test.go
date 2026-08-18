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
