// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCentralSchedulingConf_NormalizeAndModeFor(t *testing.T) {
	conf := (&CentralSchedulingConf{
		Enabled:         true,
		Mode:            CentralModeEnforce,
		AllowlistSpaces: []int64{1, 2},
		DenylistExpts:   []int64{22},
	}).Normalize()

	assert.Equal(t, CentralDefaultEnv, conf.EnvNamespace)
	assert.Equal(t, int(CentralDefaultTickInterval/time.Second), conf.TickIntervalSecond)
	assert.Equal(t, CentralModeEnforce, conf.ModeFor(1, 100))
	assert.Equal(t, CentralModeLegacy, conf.ModeFor(3, 100))
	assert.Equal(t, CentralModeLegacy, conf.ModeFor(2, 22))

	// Disabled always wins.
	conf.Enabled = false
	assert.Equal(t, CentralModeLegacy, conf.ModeFor(1, 100))

	// Unknown mode falls back to legacy, never enforce.
	conf.Enabled = true
	conf.Mode = "garbage"
	assert.Equal(t, CentralModeLegacy, (&CentralSchedulingConf{Enabled: true, Mode: "garbage"}).Normalize().Mode)
}

func TestCentralPriorityBase_AgingAndOverrideTie(t *testing.T) {
	online := &CentralActiveRun{ExptType: ExptType_Online, RunMode: EvaluationModeSubmit}
	offline := &CentralActiveRun{ExptType: ExptType_Offline, RunMode: EvaluationModeSubmit}
	retry := &CentralActiveRun{ExptType: ExptType_Offline, RunMode: EvaluationModeRetryAll}
	trial := &CentralActiveRun{ExptType: ExptType_Offline, RunMode: EvaluationModeTrialRun}

	assert.Equal(t, int64(100), CentralPriorityBase(online, nil))
	assert.Equal(t, int64(50), CentralPriorityBase(offline, nil))
	assert.Equal(t, int64(70), CentralPriorityBase(retry, nil))
	assert.Equal(t, int64(30), CentralPriorityBase(trial, nil))

	conf := (&CentralSchedulingConf{AgingSecond: 60, MaxAgingPriorityBonus: 20}).Normalize()
	now := time.Unix(10_000, 0)
	assert.Equal(t, int64(55), CentralAgedPriority(50, now.Add(-5*time.Minute), now, conf))
	assert.Equal(t, int64(70), CentralAgedPriority(50, now.Add(-24*time.Hour), now, conf))
}

func TestCentralResourceDemand_MultiResource(t *testing.T) {
	expt := &CentralExperimentScheduling{
		TargetID:            7,
		TargetType:          EvalTargetTypeSandboxAgent,
		EvaluatorVersionIDs: []int64{101, 102},
	}
	claims := expt.ResourceDemand()
	assert.Equal(t, int64(1), claims[CentralResourceItem])
	assert.Equal(t, int64(1), claims[CentralResourceTarget])
	assert.Equal(t, int64(2), claims[CentralResourceEvaluator])
	assert.Equal(t, int64(1), claims[CentralResourceSandbox])
	assert.Equal(t, int64(1), claims["target:7"])
	assert.Equal(t, int64(1), claims["evaluator_version:101"])
	assert.Equal(t, int64(1), claims["evaluator_version:102"])

	// Only enforced dimensions are returned as pool keys.
	keys := CentralPoolKeys(claims, map[string]int64{CentralResourceItem: 5, "target:7": 1})
	assert.Equal(t, []string{CentralResourceItem, "target:7"}, keys)
}

func TestCentralReservationID_Deterministic(t *testing.T) {
	assert.Equal(t, "resv:1:2:3", CentralReservationID(1, 2, 3))
	assert.Equal(t, CentralReservationID(1, 2, 3), CentralReservationID(1, 2, 3))
}

func TestCentralExtPolicyOverrides(t *testing.T) {
	ext := map[string]string{
		CentralExtModeKey:      string(CentralModeEnforce),
		CentralExtPriorityKey:  "88",
		CentralExtMaxConcurKey: "12",
	}
	assert.Equal(t, CentralModeEnforce, CentralModeFromExt(ext, CentralModeLegacy))
	assert.Equal(t, CentralModeLegacy, CentralModeFromExt(map[string]string{CentralExtModeKey: "garbage"}, CentralModeLegacy))
	v, ok := CentralPriorityFromExt(ext)
	assert.True(t, ok)
	assert.Equal(t, int64(88), v)
	n, ok := CentralMaxConcurFromExt(ext)
	assert.True(t, ok)
	assert.Equal(t, 12, n)
}
