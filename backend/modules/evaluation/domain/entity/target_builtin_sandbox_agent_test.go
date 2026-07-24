// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestResolveSandboxCountMode 校验 空/未识别值一律回退到 Single；仅 Dual 保持不变。
func TestResolveSandboxCountMode(t *testing.T) {
	cases := []struct {
		name string
		in   SandboxCountMode
		want SandboxCountMode
	}{
		{name: "empty -> Single", in: "", want: SandboxCountModeSingle},
		{name: "single -> Single", in: SandboxCountModeSingle, want: SandboxCountModeSingle},
		{name: "dual preserved", in: SandboxCountModeDual, want: SandboxCountModeDual},
		{name: "unknown value -> Single", in: SandboxCountMode("triple"), want: SandboxCountModeSingle},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, ResolveSandboxCountMode(c.in))
		})
	}
}

// TestSandboxAgent_IsDualSandbox nil / 空 mode / unknown 均返回 false，只有显式 Dual 返回 true。
func TestSandboxAgent_IsDualSandbox(t *testing.T) {
	cases := []struct {
		name  string
		agent *SandboxAgent
		want  bool
	}{
		{name: "nil receiver", agent: nil, want: false},
		{name: "zero-value mode", agent: &SandboxAgent{}, want: false},
		{name: "explicit single", agent: &SandboxAgent{SandboxCountMode: SandboxCountModeSingle}, want: false},
		{name: "unknown mode falls back", agent: &SandboxAgent{SandboxCountMode: SandboxCountMode("triple")}, want: false},
		{name: "explicit dual", agent: &SandboxAgent{SandboxCountMode: SandboxCountModeDual}, want: true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.agent.IsDualSandbox())
		})
	}
}
