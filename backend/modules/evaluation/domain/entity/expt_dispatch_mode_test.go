// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidExptDispatchMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode ExptDispatchMode
		want bool
	}{
		{name: "legacy 合法", mode: ExptDispatchModeLegacy, want: true},
		{name: "enforce 合法", mode: ExptDispatchModeEnforce, want: true},
		{name: "空串非法：写入路径必须显式给值，不靠 DB 默认值兜底", mode: "", want: false},
		{name: "未知模式非法", mode: "central", want: false},
		{name: "大小写敏感：Enforce 非法", mode: "Enforce", want: false},
		{name: "已废弃的 shadow 非法", mode: "shadow", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsValidExptDispatchMode(tt.mode))
		})
	}
}

func TestIsCentralDispatch(t *testing.T) {
	t.Parallel()

	assert.True(t, IsCentralDispatch(ExptDispatchModeEnforce))
	assert.False(t, IsCentralDispatch(ExptDispatchModeLegacy))
	// 未知模式不得被当成中心调度：那样会让实验绕过 legacy 闸又拿不到 reservation
	assert.False(t, IsCentralDispatch(""))
	assert.False(t, IsCentralDispatch("shadow"))
}

func TestNormalizeExptDispatchMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode ExptDispatchMode
		want ExptDispatchMode
	}{
		{name: "合法值原样返回", mode: ExptDispatchModeEnforce, want: ExptDispatchModeEnforce},
		{name: "legacy 原样返回", mode: ExptDispatchModeLegacy, want: ExptDispatchModeLegacy},
		{name: "空串收敛为 legacy", mode: "", want: ExptDispatchModeLegacy},
		{name: "脏数据收敛为 legacy（安全侧：走旧链路而非绕过额度）", mode: "bogus", want: ExptDispatchModeLegacy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, NormalizeExptDispatchMode(tt.mode))
		})
	}
}

func TestNormalizeExptPriorityLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		priority int32
		want     int32
	}{
		{name: "0（未申报）收敛为缺省 1", priority: 0, want: DefaultExptPriorityLevel},
		{name: "负数收敛为缺省 1", priority: -5, want: DefaultExptPriorityLevel},
		{name: "下界 1 保持", priority: 1, want: 1},
		{name: "区间内保持", priority: 50, want: 50},
		{name: "上界 99 保持", priority: 99, want: 99},
		{name: "超上界截断为 99", priority: 100, want: MaxExptPriorityLevel},
		{name: "极大值截断为 99", priority: 1 << 20, want: MaxExptPriorityLevel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, NormalizeExptPriorityLevel(tt.priority))
		})
	}
}
