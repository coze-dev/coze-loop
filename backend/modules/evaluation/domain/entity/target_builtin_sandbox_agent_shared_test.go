// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ResolveSandboxCountMode 的回退是 fail-OPEN 的，所以每个取值都必须显式有 case。
// 漏一个的现象不是报错，而是实验静默按 Single 的旧扁平链路跑完并报成功。
func TestResolveSandboxCountMode_EveryDeclaredValueRoundTrips(t *testing.T) {
	for _, mode := range []SandboxCountMode{
		SandboxCountModeSingle, SandboxCountModeDual,
		SandboxCountModeMacVMPlusSandbox, SandboxCountModeMacVMPlusSSH,
		SandboxCountModeShared,
	} {
		assert.Equal(t, mode, ResolveSandboxCountMode(mode),
			"%q 没有在 ResolveSandboxCountMode 里显式列出，会被静默回落成 single", mode)
	}
}

func TestResolveSandboxCountMode_UnknownAndEmptyFallBackToSingle(t *testing.T) {
	assert.Equal(t, SandboxCountModeSingle, ResolveSandboxCountMode(""))
	assert.Equal(t, SandboxCountModeSingle, ResolveSandboxCountMode("Shared"), "大小写不容错，是刻意的")
	assert.Equal(t, SandboxCountModeSingle, ResolveSandboxCountMode("whatever"))
}

// 五个拓扑判定互斥：任何一对同时为 true 都会让 AsyncExecute 的顺序分流走错分支。
func TestSandboxAgentTopologyPredicatesAreMutuallyExclusive(t *testing.T) {
	for _, mode := range []SandboxCountMode{
		SandboxCountModeSingle, SandboxCountModeDual,
		SandboxCountModeMacVMPlusSandbox, SandboxCountModeMacVMPlusSSH,
		SandboxCountModeShared,
	} {
		a := &SandboxAgent{SandboxCountMode: mode}
		trues := 0
		for _, on := range []bool{a.IsDualSandbox(), a.IsMacVMPlusSandbox(), a.IsMacVMPlusSSH(), a.IsSharedSandbox()} {
			if on {
				trues++
			}
		}
		assert.LessOrEqual(t, trues, 1, "%q 命中了多个拓扑判定", mode)
		if mode == SandboxCountModeShared {
			assert.True(t, a.IsSharedSandbox())
			assert.False(t, a.IsDualSandbox(), "共享沙箱绝不能同时被判成双沙箱")
		}
	}
}

func TestIsSharedSandbox_NilIsSingle(t *testing.T) {
	var a *SandboxAgent
	assert.False(t, a.IsSharedSandbox())
}
