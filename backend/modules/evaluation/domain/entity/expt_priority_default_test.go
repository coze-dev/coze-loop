// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNormalizeExptPriorityLevelWithDefault_UsesConfiguredDefault 未申报时采纳配置的缺省值。
//
// 这是本次修复的核心：此前缺省值硬编码为 1，TCC 里的 default_priority 配了完全无效
// 且静默 —— 现象是"配了看着生效实际没生效"。
func TestNormalizeExptPriorityLevelWithDefault_UsesConfiguredDefault(t *testing.T) {
	assert.Equal(t, int32(5), NormalizeExptPriorityLevelWithDefault(0, 5), "未申报应采纳配置缺省值")
	assert.Equal(t, int32(5), NormalizeExptPriorityLevelWithDefault(-3, 5), "负值同样视为未申报")
	assert.Equal(t, int32(99), NormalizeExptPriorityLevelWithDefault(0, 99), "边界值 99 可采纳")
	assert.Equal(t, int32(1), NormalizeExptPriorityLevelWithDefault(0, 1), "边界值 1 可采纳")
}

// TestNormalizeExptPriorityLevelWithDefault_FallsBackWhenDefaultAbsent
// defaultPriority=0 表示"没有意见"，回落到 1。
// 这一条同时覆盖两种真实情况：TCC 里没配这个字段、以及 noop policy（开源部署）。
func TestNormalizeExptPriorityLevelWithDefault_FallsBackWhenDefaultAbsent(t *testing.T) {
	assert.Equal(t, DefaultExptPriorityLevel, NormalizeExptPriorityLevelWithDefault(0, 0))
	assert.Equal(t, int32(1), NormalizeExptPriorityLevelWithDefault(0, 0))
}

// TestNormalizeExptPriorityLevelWithDefault_RejectsOutOfRangeDefault
// 越界的 defaultPriority 回落到 1，**不截断到 99**。
//
// 方向很关键：这个值来自人工维护的配置，配成 999 更可能是笔误而非"想要最高优先级"。
// 若截断到 99，一次笔误会静默变成"该空间所有实验都最高优" —— 那是最难发现的一类事故，
// 因为每个实验单看都正常，只有整体排序全乱。
func TestNormalizeExptPriorityLevelWithDefault_RejectsOutOfRangeDefault(t *testing.T) {
	assert.Equal(t, int32(1), NormalizeExptPriorityLevelWithDefault(0, 999), "越界缺省值回落 1 而非截断 99")
	assert.Equal(t, int32(1), NormalizeExptPriorityLevelWithDefault(0, 100), "刚越界也回落")
	assert.Equal(t, int32(1), NormalizeExptPriorityLevelWithDefault(0, -5), "负缺省值回落")
}

// TestNormalizeExptPriorityLevelWithDefault_DeclaredValueWins 已申报时缺省值不参与。
func TestNormalizeExptPriorityLevelWithDefault_DeclaredValueWins(t *testing.T) {
	assert.Equal(t, int32(7), NormalizeExptPriorityLevelWithDefault(7, 5), "申报值优先")
	assert.Equal(t, int32(99), NormalizeExptPriorityLevelWithDefault(150, 5),
		"申报值越界按边界截断（不是回落缺省）—— 调用方明确表达了'要最高优'")
}

// TestNormalizeExptPriorityLevel_BackwardCompatible 旧入口行为不变。
// 它现在是 WithDefault(priority, 0) 的薄包装，既有调用点（读路径、DB 转换）无需改动。
func TestNormalizeExptPriorityLevel_BackwardCompatible(t *testing.T) {
	assert.Equal(t, int32(1), NormalizeExptPriorityLevel(0))
	assert.Equal(t, int32(1), NormalizeExptPriorityLevel(-1))
	assert.Equal(t, int32(50), NormalizeExptPriorityLevel(50))
	assert.Equal(t, int32(99), NormalizeExptPriorityLevel(1000))
}
