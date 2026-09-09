// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRetryYieldConf_IsSpaceEnabled 覆盖让位降权灰度开关的空间级判定。
//
// 该函数是整条改造的唯一入口开关(值在实验发起时读一次并固化进 event.Ext)，
// 判错的后果是两极：误判 true 会让未灰度空间提前走新链路；误判 false 会让
// 已放量空间静默退回老行为(实测踩过——TCC 配错目录导致读到 false，
// 表现为让位完全不生效但无任何报错)。故对 nil / 全局开 / 白名单三条路径逐一固定。
func TestRetryYieldConf_IsSpaceEnabled(t *testing.T) {
	const targetSpace int64 = 7533126599059701761

	tests := []struct {
		name    string
		conf    *RetryYieldConf
		spaceID int64
		want    bool
	}{
		{
			name:    "nil conf -> 关闭(未配置默认不生效)",
			conf:    nil,
			spaceID: targetSpace,
			want:    false,
		},
		{
			name:    "零值 conf -> 关闭",
			conf:    &RetryYieldConf{},
			spaceID: targetSpace,
			want:    false,
		},
		{
			name:    "Enabled=true -> 全局开, 任意空间生效",
			conf:    &RetryYieldConf{Enabled: true},
			spaceID: targetSpace,
			want:    true,
		},
		{
			name: "Enabled=true 时忽略白名单(白名单不含该空间也开)",
			conf: &RetryYieldConf{
				Enabled:  true,
				SpaceIDs: []int64{1, 2, 3},
			},
			spaceID: targetSpace,
			want:    true,
		},
		{
			name: "Enabled=false + 命中白名单 -> 开",
			conf: &RetryYieldConf{
				Enabled:  false,
				SpaceIDs: []int64{1, targetSpace, 3},
			},
			spaceID: targetSpace,
			want:    true,
		},
		{
			name: "Enabled=false + 白名单首位命中 -> 开",
			conf: &RetryYieldConf{
				Enabled:  false,
				SpaceIDs: []int64{targetSpace},
			},
			spaceID: targetSpace,
			want:    true,
		},
		{
			name: "Enabled=false + 白名单不含 -> 关",
			conf: &RetryYieldConf{
				Enabled:  false,
				SpaceIDs: []int64{1, 2, 3},
			},
			spaceID: targetSpace,
			want:    false,
		},
		{
			name: "Enabled=false + 空白名单 -> 关",
			conf: &RetryYieldConf{
				Enabled:  false,
				SpaceIDs: []int64{},
			},
			spaceID: targetSpace,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.conf.IsSpaceEnabled(tt.spaceID))
		})
	}
}

// TestRetryYieldConf_IsIndexReady 覆盖降权索引就绪标志。
//
// 与 IsSpaceEnabled 语义不同: 这是 schema 事实声明而非业务灰度，只影响是否下
// ForceIndex hint、不改排序语义。nil 必须返回 false —— 否则配置缺失时会对未建成的
// idx_expt_run_retry_pick 下 hint，挑选直接报 Key doesn't exist 使整个实验调度瘫痪
// (线上是方案 B: 只加列不加索引, 该索引确实不存在)。
func TestRetryYieldConf_IsIndexReady(t *testing.T) {
	tests := []struct {
		name string
		conf *RetryYieldConf
		want bool
	}{
		{name: "nil conf -> false(安全侧, 绝不下 hint)", conf: nil, want: false},
		{name: "零值 -> false", conf: &RetryYieldConf{}, want: false},
		{name: "IndexReady=true -> true", conf: &RetryYieldConf{IndexReady: true}, want: true},
		{
			name: "IndexReady 与 Enabled 相互独立: Enabled=true 不隐含索引就绪",
			conf: &RetryYieldConf{Enabled: true, IndexReady: false},
			want: false,
		},
		{
			name: "IndexReady 与 Enabled 相互独立: Enabled=false 也可声明索引就绪",
			conf: &RetryYieldConf{Enabled: false, IndexReady: true},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.conf.IsIndexReady())
		})
	}
}

// TestExptExecConf_GetRetryYieldConf 覆盖 nil-safe 取配置的两条路径。
func TestExptExecConf_GetRetryYieldConf(t *testing.T) {
	t.Run("nil receiver -> nil", func(t *testing.T) {
		var conf *ExptExecConf
		assert.Nil(t, conf.GetRetryYieldConf())
		// 与 IsSpaceEnabled/IsIndexReady 串起来必须仍是安全默认值
		assert.False(t, conf.GetRetryYieldConf().IsSpaceEnabled(1))
		assert.False(t, conf.GetRetryYieldConf().IsIndexReady())
	})

	t.Run("RetryYield 未配置 -> nil", func(t *testing.T) {
		conf := &ExptExecConf{}
		assert.Nil(t, conf.GetRetryYieldConf())
		assert.False(t, conf.GetRetryYieldConf().IsSpaceEnabled(1))
	})

	t.Run("RetryYield 已配置 -> 原样返回", func(t *testing.T) {
		ry := &RetryYieldConf{Enabled: true, IndexReady: true}
		conf := &ExptExecConf{RetryYield: ry}
		assert.Same(t, ry, conf.GetRetryYieldConf())
		assert.True(t, conf.GetRetryYieldConf().IsSpaceEnabled(1))
		assert.True(t, conf.GetRetryYieldConf().IsIndexReady())
	})
}

// TestExptConsumerConf_GetExptExecConf_RetryYieldResolution 固定「per-space 命中后不回退全局」
// 这条实测踩过的坑：目标空间一旦有自己的 space_expt_exec_conf 条目，顶层 expt_exec_conf
// 里的 retry_yield 就读不到了 —— 灰度必须配进该空间自己的条目。
func TestExptConsumerConf_GetExptExecConf_RetryYieldResolution(t *testing.T) {
	const spaceWithOwnEntry int64 = 7533126599059701761
	const spaceWithoutEntry int64 = 999

	cc := &ExptConsumerConf{
		// 顶层(全局)开着
		ExptExecConf: &ExptExecConf{
			RetryYield: &RetryYieldConf{Enabled: true},
		},
		SpaceExptExecConf: map[int64]*ExptExecConf{
			// 该空间有独立条目但【没配】retry_yield
			spaceWithOwnEntry: {
				ExptItemEvalConf: &ExptItemEvalConf{ConcurNum: 2},
			},
		},
	}

	t.Run("有独立条目的空间: 不回退全局, retry_yield 读不到 -> 关", func(t *testing.T) {
		got := cc.GetExptExecConf(spaceWithOwnEntry).GetRetryYieldConf().IsSpaceEnabled(spaceWithOwnEntry)
		assert.False(t, got, "per-space 条目命中后不回退全局，顶层 Enabled=true 不应生效")
	})

	t.Run("无独立条目的空间: 回退全局 -> 开", func(t *testing.T) {
		got := cc.GetExptExecConf(spaceWithoutEntry).GetRetryYieldConf().IsSpaceEnabled(spaceWithoutEntry)
		assert.True(t, got)
	})

	t.Run("独立条目内配了 retry_yield -> 按自己的条目生效", func(t *testing.T) {
		cc2 := &ExptConsumerConf{
			ExptExecConf: &ExptExecConf{RetryYield: &RetryYieldConf{Enabled: false}},
			SpaceExptExecConf: map[int64]*ExptExecConf{
				spaceWithOwnEntry: {
					ExptItemEvalConf: &ExptItemEvalConf{ConcurNum: 2},
					RetryYield:       &RetryYieldConf{Enabled: true},
				},
			},
		}
		assert.True(t, cc2.GetExptExecConf(spaceWithOwnEntry).GetRetryYieldConf().IsSpaceEnabled(spaceWithOwnEntry))
	})

	t.Run("nil ExptConsumerConf -> 全链 nil-safe 且关闭", func(t *testing.T) {
		var nilCC *ExptConsumerConf
		assert.False(t, nilCC.GetExptExecConf(spaceWithOwnEntry).GetRetryYieldConf().IsSpaceEnabled(spaceWithOwnEntry))
	})
}
