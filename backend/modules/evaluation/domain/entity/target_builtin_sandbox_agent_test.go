// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestResolveSandboxCountMode 校验已知模式保留原值，空/未识别值回退到 Single。
func TestResolveSandboxCountMode(t *testing.T) {
	cases := []struct {
		name string
		in   SandboxCountMode
		want SandboxCountMode
	}{
		{name: "empty -> Single", in: "", want: SandboxCountModeSingle},
		{name: "single -> Single", in: SandboxCountModeSingle, want: SandboxCountModeSingle},
		{name: "dual preserved", in: SandboxCountModeDual, want: SandboxCountModeDual},
		{name: "mac_vm_plus_sandbox preserved", in: SandboxCountModeMacVMPlusSandbox, want: SandboxCountModeMacVMPlusSandbox},
		{name: "mac_vm_plus_ssh preserved", in: SandboxCountModeMacVMPlusSSH, want: SandboxCountModeMacVMPlusSSH},
		{name: "shared preserved", in: SandboxCountModeShared, want: SandboxCountModeShared},
		{name: "arm_mac_vm_plus_sandbox preserved", in: SandboxCountMode("arm_mac_vm_plus_sandbox"), want: SandboxCountMode("arm_mac_vm_plus_sandbox")},
		{name: "windows_plus_sandbox preserved", in: SandboxCountMode("windows_plus_sandbox"), want: SandboxCountMode("windows_plus_sandbox")},
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

// TestSandboxAgent_IsMacVMPlusSandbox 校验传统与 ARM Mac 的双资源拓扑。
func TestSandboxAgent_IsMacVMPlusSandbox(t *testing.T) {
	cases := []struct {
		name  string
		agent *SandboxAgent
		want  bool
	}{
		{name: "nil receiver", agent: nil, want: false},
		{name: "zero-value mode", agent: &SandboxAgent{}, want: false},
		{name: "dual is not mac_vm_plus_sandbox", agent: &SandboxAgent{SandboxCountMode: SandboxCountModeDual}, want: false},
		{name: "mac_vm_plus_ssh is not mac_vm_plus_sandbox", agent: &SandboxAgent{SandboxCountMode: SandboxCountModeMacVMPlusSSH}, want: false},
		{name: "unknown mode falls back", agent: &SandboxAgent{SandboxCountMode: SandboxCountMode("triple")}, want: false},
		{name: "explicit arm_mac_vm_plus_sandbox", agent: &SandboxAgent{SandboxCountMode: SandboxCountMode("arm_mac_vm_plus_sandbox")}, want: true},
		{name: "windows is not mac_vm_plus_sandbox", agent: &SandboxAgent{SandboxCountMode: SandboxCountMode("windows_plus_sandbox")}, want: false},
		{name: "explicit mac_vm_plus_sandbox", agent: &SandboxAgent{SandboxCountMode: SandboxCountModeMacVMPlusSandbox}, want: true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.agent.IsMacVMPlusSandbox())
		})
	}
}

// TestSandboxAgent_IsMacVMPlusSSH nil / 空 mode / 其它 mode 均返回 false，只有显式 mac_vm_plus_ssh 返回 true。
func TestSandboxAgent_IsMacVMPlusSSH(t *testing.T) {
	cases := []struct {
		name  string
		agent *SandboxAgent
		want  bool
	}{
		{name: "nil receiver", agent: nil, want: false},
		{name: "zero-value mode", agent: &SandboxAgent{}, want: false},
		{name: "dual is not mac_vm_plus_ssh", agent: &SandboxAgent{SandboxCountMode: SandboxCountModeDual}, want: false},
		{name: "mac_vm_plus_sandbox is not mac_vm_plus_ssh", agent: &SandboxAgent{SandboxCountMode: SandboxCountModeMacVMPlusSandbox}, want: false},
		{name: "unknown mode falls back", agent: &SandboxAgent{SandboxCountMode: SandboxCountMode("triple")}, want: false},
		{name: "explicit mac_vm_plus_ssh", agent: &SandboxAgent{SandboxCountMode: SandboxCountModeMacVMPlusSSH}, want: true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.agent.IsMacVMPlusSSH())
		})
	}
}

func TestSandboxAgent_WindowsHasNoSupportedTopology(t *testing.T) {
	agent := &SandboxAgent{SandboxCountMode: SandboxCountMode("windows_plus_sandbox")}
	cases := []struct {
		name    string
		matches func() bool
	}{
		{name: "dual", matches: agent.IsDualSandbox},
		{name: "mac_vm_plus_sandbox", matches: agent.IsMacVMPlusSandbox},
		{name: "mac_vm_plus_ssh", matches: agent.IsMacVMPlusSSH},
		{name: "shared", matches: agent.IsSharedSandbox},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.False(t, c.matches())
		})
	}
}
