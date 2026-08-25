// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findResource 在向量里找指定维度，找不到返回 nil。
func findResource(c *ExpectedQuotaConsumption, category, key string) *ExpectedResourceConsumption {
	if c == nil {
		return nil
	}
	for _, r := range c.Resources {
		if r != nil && r.Category == category && r.ResourceKey == key {
			return r
		}
	}
	return nil
}

// TestWithConcurrencyDimension_InjectsWhenAbsent 守住核心不变量：
// 并发维度必须被补齐，否则"全 Scope 在跑 item 总数"没有任何上限
// —— 实测中心调度只受单实验 deficit + 资源额度约束，某实验只申报 model token
// 不申报 sandbox 时并发完全失控。
func TestWithConcurrencyDimension_InjectsWhenAbsent(t *testing.T) {
	in := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
		{Category: "sandbox", ResourceKey: "default", Amount: 1},
		{Category: "model", ResourceKey: "gpt5.5", Amount: 1000},
	}}

	out := in.WithConcurrencyDimension()

	got := findResource(out, QuotaCategoryConcurrency, QuotaResourceKeyItem)
	require.NotNil(t, got, "必须补齐 concurrency|item，否则并发无上限")
	assert.Equal(t, int64(1), got.Amount, "单 item 恒占 1 份并发")

	// 原有维度不能丢
	assert.NotNil(t, findResource(out, "sandbox", "default"))
	assert.NotNil(t, findResource(out, "model", "gpt5.5"))
	assert.Len(t, out.Resources, 3)

	// 不得原地修改入参：它是创建期冻结进 eval_conf 的快照，改了"冻结"语义就失效
	assert.Nil(t, findResource(in, QuotaCategoryConcurrency, QuotaResourceKeyItem),
		"WithConcurrencyDimension 不能原地改 receiver")
	assert.Len(t, in.Resources, 2)
}

// TestWithConcurrencyDimension_IdempotentOnExplicitDeclaration 已显式申报时保留申报值。
// 这是把额度下沉到 item 粒度后要留的扩展口：未来"重型 item 占 2 份并发"靠它表达。
func TestWithConcurrencyDimension_IdempotentOnExplicitDeclaration(t *testing.T) {
	in := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
		{Category: "sandbox", ResourceKey: "default", Amount: 1},
		{Category: QuotaCategoryConcurrency, ResourceKey: QuotaResourceKeyItem, Amount: 2},
	}}

	out := in.WithConcurrencyDimension()

	got := findResource(out, QuotaCategoryConcurrency, QuotaResourceKeyItem)
	require.NotNil(t, got)
	assert.Equal(t, int64(2), got.Amount, "显式申报的并发占用量不得被覆盖成 1")
	assert.Len(t, out.Resources, 2, "不得重复追加同一维度")

	// 再调一次仍然稳定（幂等）
	again := out.WithConcurrencyDimension()
	assert.Len(t, again.Resources, 2)
	assert.Equal(t, int64(2), findResource(again, QuotaCategoryConcurrency, QuotaResourceKeyItem).Amount)
}

// TestWithConcurrencyDimension_NilReceiver 守的是**本方法自身的 nil 安全契约**，
// 不是调度器的不变量：两个生产调用点（商业版 toRequirements / frozenConstraintsOf）
// 都先挡了 nil，所以这条路径不会被生产触达。
//
// 之所以仍要钉：nil 分支是本方法唯一的 nil 安全保障，删掉它方法就变成 nil 直接 panic。
// 本用例是唯一能抓住那次删除的防线（实测：把该分支改成 return nil，全 evaluation 模块
// 只有本用例 FAIL，商业版 51 个包零失败 —— 即它护的确实只是契约本身）。
func TestWithConcurrencyDimension_NilReceiver(t *testing.T) {
	var in *ExpectedQuotaConsumption

	out := in.WithConcurrencyDimension()

	require.NotNil(t, out)
	got := findResource(out, QuotaCategoryConcurrency, QuotaResourceKeyItem)
	require.NotNil(t, got)
	assert.Equal(t, int64(1), got.Amount)
}

// TestWithConcurrencyDimension_SkipsNilEntries 向量里的 nil 项不得导致 panic 或被带出。
func TestWithConcurrencyDimension_SkipsNilEntries(t *testing.T) {
	in := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
		nil,
		{Category: "sandbox", ResourceKey: "default", Amount: 1},
		nil,
	}}

	out := in.WithConcurrencyDimension()

	assert.Len(t, out.Resources, 2, "nil 项应被过滤，只留 sandbox + 注入的 concurrency")
	for _, r := range out.Resources {
		assert.NotNil(t, r)
	}
}

// TestSameAs 守住切批的正确性判据。
//
// 中心调度按 SameAs 把候选 item 分批预占：ReserveBatch 的 Requirements 是"同批共享"的，
// Lua 靠 floor(available/amount) 一次除法算本批能放几个 —— 若把 amount 不同的 item
// 混进同一批，该除法会按其中一个 amount 算，导致**多扣或少扣**。
func TestSameAs(t *testing.T) {
	v := func(pairs ...any) *ExpectedQuotaConsumption {
		out := &ExpectedQuotaConsumption{}
		for i := 0; i+2 < len(pairs)+1; i += 3 {
			out.Resources = append(out.Resources, &ExpectedResourceConsumption{
				Category:    pairs[i].(string),
				ResourceKey: pairs[i+1].(string),
				Amount:      int64(pairs[i+2].(int)),
			})
		}
		return out
	}

	t.Run("完全相同 → 等价", func(t *testing.T) {
		a := v("sandbox", "default", 1, "model", "gpt5.5", 1000)
		b := v("sandbox", "default", 1, "model", "gpt5.5", 1000)
		assert.True(t, a.SameAs(b))
	})

	t.Run("顺序不同但内容相同 → 等价（不能因排序误切批）", func(t *testing.T) {
		a := v("sandbox", "default", 1, "model", "gpt5.5", 1000)
		b := v("model", "gpt5.5", 1000, "sandbox", "default", 1)
		assert.True(t, a.SameAs(b), "维度顺序不该影响等价判定")
	})

	t.Run("用量不同 → 不等价（混批会多扣/少扣）", func(t *testing.T) {
		a := v("sandbox", "default", 1)
		b := v("sandbox", "default", 2)
		assert.False(t, a.SameAs(b))
	})

	t.Run("维度数不同 → 不等价", func(t *testing.T) {
		a := v("sandbox", "default", 1)
		b := v("sandbox", "default", 1, "model", "gpt5.5", 1000)
		assert.False(t, a.SameAs(b))
	})

	t.Run("资源键不同 → 不等价", func(t *testing.T) {
		a := v("model", "gpt5.5", 1000)
		b := v("model", "gpt4", 1000)
		assert.False(t, a.SameAs(b))
	})

	t.Run("两个 nil → 等价；nil 与非空 → 不等价", func(t *testing.T) {
		var n *ExpectedQuotaConsumption
		assert.True(t, n.SameAs(nil), "都无向量时应可进同一批")
		assert.False(t, n.SameAs(v("sandbox", "default", 1)))
		assert.False(t, v("sandbox", "default", 1).SameAs(nil))
	})

	t.Run("nil 项被忽略，不影响等价", func(t *testing.T) {
		a := &ExpectedQuotaConsumption{Resources: []*ExpectedResourceConsumption{
			nil, {Category: "sandbox", ResourceKey: "default", Amount: 1}, nil,
		}}
		b := v("sandbox", "default", 1)
		assert.True(t, a.SameAs(b))
	})
}
