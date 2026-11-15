// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolveCentralSchedulerMode(t *testing.T) {
	cfg := &CentralSchedulerConfig{Enabled: true, DefaultMode: "enforce"}
	assert.Equal(t, CentralSchedulerModeEnforce, ResolveCentralSchedulerMode(cfg, 1, nil))
	assert.Equal(t, CentralSchedulerModeLegacy, ResolveCentralSchedulerMode(nil, 1, nil))
	assert.Equal(t, CentralSchedulerModeEnforce, ResolveCentralSchedulerMode(cfg, 1, map[string]string{CentralExtKeySchedulingMode: "enforce"}))
	assert.Equal(t, CentralSchedulerModeLegacy, ResolveCentralSchedulerMode(cfg, 1, map[string]string{CentralExtKeySchedulingMode: "legacy"}), "experiment ext must override TCC default")

	cfg2 := &CentralSchedulerConfig{Enabled: true, DefaultMode: "legacy", EnforceSpaceIDs: []int64{9}}
	assert.Equal(t, CentralSchedulerModeEnforce, ResolveCentralSchedulerMode(cfg2, 9, nil))
	assert.Equal(t, CentralSchedulerModeLegacy, ResolveCentralSchedulerMode(cfg2, 10, nil))
}

func TestResolveCentralPriority(t *testing.T) {
	assert.Equal(t, DefaultCentralPriority, ResolveCentralPriority(nil, 0))
	assert.Equal(t, 10, ResolveCentralPriority(nil, 10))
	assert.Equal(t, 3, ResolveCentralPriority(map[string]string{CentralExtKeyPriority: "3"}, 10))
	assert.Equal(t, 10, ResolveCentralPriority(map[string]string{CentralExtKeyPriority: "not-a-number"}, 10))
}

func TestCentralSchedulerConfigDefaults(t *testing.T) {
	var cfg *CentralSchedulerConfig
	assert.Equal(t, 20*time.Second, cfg.GetTickInterval())
	assert.Equal(t, time.Duration(0), cfg.GetLeaseTTL()-300*time.Second)
	// 容量只来自配置：未配置时没有生产默认值，相关实验不派发。
	assert.Empty(t, cfg.GetCapacities())
	assert.Empty(t, cfg.GetEffectiveCapacities())
	assert.Equal(t, CentralSchedulerModeLegacy, cfg.GetDefaultMode())
	assert.False(t, cfg.IsSpaceEnforced(1))
	assert.Equal(t, CentralDegradedPolicyBlockNew, cfg.GetDegradedPolicy())
	assert.Equal(t, DefaultCentralAgingWindowTicks, cfg.GetAgingWindowTicks())
	assert.Equal(t, DefaultCentralAgingStep, cfg.GetAgingStep())
	assert.Equal(t, DefaultCentralHighestPriority, cfg.GetHighestPriority())
}

func TestCentralEnvScopeSanitize(t *testing.T) {
	scope := CentralEnvScope{Env: "stone.cozeloop:evaluation", Lane: ""}
	got := scope.Sanitize()
	assert.Equal(t, "stone_cozeloop_evaluation", got.Env)
	assert.Equal(t, "", got.Lane, "empty lane must not fall back to another owner's namespace")
	assert.False(t, scope.IsValid())
	assert.True(t, CentralEnvScope{Env: "e", Lane: "l"}.IsValid())
}

func TestCentralCapacitiesComeOnlyFromConfig(t *testing.T) {
	cfg := &CentralSchedulerConfig{
		Resources: map[string]CentralResourceConfig{
			string(CentralResourceItemSlot):       {Capacity: 5, Enabled: true},
			string(CentralResourceEvaluatorSlot):  {Capacity: 10, Enabled: false},
			string(CentralResourceSandboxExecute): {Capacity: 0, Enabled: true},
		},
	}
	caps := cfg.GetCapacities()
	assert.Equal(t, int64(5), caps[CentralResourceItemSlot])
	_, ok := caps[CentralResourceEvaluatorSlot]
	assert.False(t, ok, "disabled resource must not be dispatched")
	_, ok = caps[CentralResourceSandboxExecute]
	assert.False(t, ok, "zero capacity must be treated as not configured")

	// 旧版扁平 capacities 继续向后兼容。
	legacy := &CentralSchedulerConfig{Capacities: map[string]int64{string(CentralResourceItemSlot): 7}}
	assert.Equal(t, int64(7), legacy.GetCapacities()[CentralResourceItemSlot])
}

func TestCentralDegradedPolicyOnlyReducesCapacity(t *testing.T) {
	cfg := &CentralSchedulerConfig{
		Resources: map[string]CentralResourceConfig{
			string(CentralResourceItemSlot): {Capacity: 10, Enabled: true},
		},
		Degraded:       true,
		DegradedPolicy: string(CentralDegradedPolicyReduceCapacity),
		DegradedRatio:  0.5,
	}
	assert.Equal(t, int64(5), cfg.GetEffectiveCapacities()[CentralResourceItemSlot])

	cfg.DegradedPolicy = string(CentralDegradedPolicyBlockNew)
	assert.Empty(t, cfg.GetEffectiveCapacities(), "block_new must suspend new reservations")

	cfg.Degraded = false
	assert.Equal(t, int64(10), cfg.GetEffectiveCapacities()[CentralResourceItemSlot], "recovery must never exceed configured capacity")
}

func TestEffectivePriorityAgingCapsAndKeepsBase(t *testing.T) {
	now := int64(1_000_000)
	// 等待 4 个窗口，每个窗口抬 10：100 -> 60。
	assert.Equal(t, 60, EffectiveCentralPriority(100, now-12_000, now, 3000, 10, 1))
	// 抬升封顶在 highest priority，且不会低于它。
	assert.Equal(t, 1, EffectiveCentralPriority(100, now-100_000, now, 3000, 10, 1))
	// 无等待时不抬升。
	assert.Equal(t, 100, EffectiveCentralPriority(100, now, now, 3000, 10, 1))
}

func TestCentralPriorityScoreSameTierFIFO(t *testing.T) {
	assert.Less(t, CentralPriorityScore(5, 1), CentralPriorityScore(5, 2))
	assert.Less(t, CentralPriorityScore(4, 999), CentralPriorityScore(5, 1))
}

func TestMakeCentralLeaseIDAttemptIsUnique(t *testing.T) {
	a := MakeCentralLeaseIDWithAttempt(1, 2, 3, 1)
	b := MakeCentralLeaseIDWithAttempt(1, 2, 3, 2)
	assert.NotEqual(t, a, b, "每次预占必须有唯一身份")
	assert.Equal(t, MakeCentralLeaseIDWithAttempt(1, 2, 3, 1), a)
}

func TestCentralActiveMemberRoundTrip(t *testing.T) {
	member := MakeCentralActiveMember(100, 7, 8, 42, "user:with:colon")
	spaceID, exptID, runID, priority, userID, ok := ParseCentralActiveMember(member)
	assert.True(t, ok)
	assert.Equal(t, int64(100), spaceID)
	assert.Equal(t, int64(7), exptID)
	assert.Equal(t, int64(8), runID)
	assert.Equal(t, 42, priority)
	assert.Equal(t, "user:with:colon", userID)

	_, _, _, _, _, ok = ParseCentralActiveMember("bad")
	assert.False(t, ok)
}

func TestMakeCentralLeaseIDDeterministic(t *testing.T) {
	assert.Equal(t, MakeCentralLeaseID(1, 2, 3), MakeCentralLeaseID(1, 2, 3))
	assert.NotEqual(t, MakeCentralLeaseID(1, 2, 3), MakeCentralLeaseID(1, 2, 4))
}
