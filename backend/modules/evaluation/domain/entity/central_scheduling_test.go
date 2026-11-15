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
	assert.Equal(t, int64(200), cfg.GetCapacities()[CentralResourceItemSlot])
	assert.Equal(t, CentralSchedulerModeLegacy, cfg.GetDefaultMode())
	assert.False(t, cfg.IsSpaceEnforced(1))
}

func TestCentralEnvScopeSanitize(t *testing.T) {
	scope := CentralEnvScope{Env: "stone.cozeloop:evaluation", Lane: ""}
	got := scope.Sanitize()
	assert.Equal(t, "stone_cozeloop_evaluation", got.Env)
	assert.Equal(t, "default", got.Lane)
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
