// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsValidRunMode 钉住"合法跑法集"。
// ⚠️ 新增 RunMode 常量时本测试必须同步扩充 —— 漏加会让新跑法被判非法、误落旧链路租户
// (见 application 层 dualSandboxTenantByRunMode)。
func TestIsValidRunMode(t *testing.T) {
	valid := []RunMode{
		RunModeSingleTurn,
		RunModeFixedScriptMultiTurn,
		RunModeSUAMultiTurn,
		RunModeSUALoopMultiTurn,
		RunModeSUAHumanLoopMultiTurn,
		RunModeGoal,
	}
	for _, m := range valid {
		assert.True(t, IsValidRunMode(m), "run_mode %q 应判为合法", m)
	}

	invalid := []RunMode{
		"",              // 空值 (透传丢字段的形态)
		"bogus_mode",    // 乱码
		"SINGLE_TURN",   // 大小写不匹配
		"single turn",   // 分隔符错误
		"sua_multiturn", // 少下划线
	}
	for _, m := range invalid {
		assert.False(t, IsValidRunMode(m), "run_mode %q 应判为非法", m)
	}
}

// TestIsValidRunModeNotSilentFallback 钉住 IsValidRunMode 与 RunModeToInt 的区别:
// RunModeToInt 对非法值 default 回落 1 (= single_turn 的编号), 无法区分"传了 single_turn"
// 和"传了乱码"。分流新旧链路必须用 IsValidRunMode 而不是 RunModeToInt。
func TestIsValidRunModeNotSilentFallback(t *testing.T) {
	// 两者在 RunModeToInt 下不可区分
	assert.Equal(t, RunModeToInt(RunModeSingleTurn), RunModeToInt(RunMode("bogus_mode")))
	// 但 IsValidRunMode 能区分
	assert.True(t, IsValidRunMode(RunModeSingleTurn))
	assert.False(t, IsValidRunMode(RunMode("bogus_mode")))
}

// TestIsNewRunModeLink 校验新旧链路判据: config 空 / run_mode 空或非法 → 旧链路。
func TestIsNewRunModeLink(t *testing.T) {
	cases := []struct {
		name string
		cfg  *RunModeConfig
		want bool
	}{
		{name: "nil config -> 旧链路", cfg: nil, want: false},
		{name: "config 在但 run_mode 空 -> 旧链路", cfg: &RunModeConfig{}, want: false},
		{name: "run_mode 非法 -> 旧链路", cfg: &RunModeConfig{RunMode: RunMode("bogus_mode")}, want: false},
		// 其余子字段有值但 run_mode 缺失, 仍是旧链路 —— 判据只看 run_mode
		{name: "只有子字段无 run_mode -> 旧链路", cfg: &RunModeConfig{SuaGoal: "x", MaxTurns: 3}, want: false},
		{name: "single_turn -> 新链路", cfg: &RunModeConfig{RunMode: RunModeSingleTurn}, want: true},
		{name: "sua_multi_turn -> 新链路", cfg: &RunModeConfig{RunMode: RunModeSUAMultiTurn}, want: true},
		{name: "goal -> 新链路", cfg: &RunModeConfig{RunMode: RunModeGoal}, want: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, IsNewRunModeLink(c.cfg))
		})
	}
}
