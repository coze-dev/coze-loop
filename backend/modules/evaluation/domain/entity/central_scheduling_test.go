// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplySchedulingMetaCreateAndAuthorize(t *testing.T) {
	// legacy 默认：只要 env 可判定就写入归属，不要求授权。
	conf := &EvaluationConfiguration{Ext: map[string]string{}}
	err := ApplySchedulingMeta(conf, SchedulingApplyInput{Env: "ppe:lane-a", Operator: "u1", NowMS: 100})
	require.NoError(t, err)
	meta := GetSchedulingMeta(conf)
	assert.Equal(t, SchedulingModeLegacy, meta.Mode)
	assert.Equal(t, DefaultSchedulingPriority, meta.Priority)
	assert.Equal(t, "ppe:lane-a", meta.Env)
	assert.Equal(t, int64(1), meta.Epoch)

	// central 未授权必须拒绝，且不落库任何模式。
	conf2 := &EvaluationConfiguration{Ext: map[string]string{ExtKeySchedulingMode: string(SchedulingModeCentral)}}
	err = ApplySchedulingMeta(conf2, SchedulingApplyInput{Env: "ppe:lane-a", Operator: "u1"})
	require.Error(t, err)

	// 授权后冻结 central，并写入审计 trace。
	conf3 := &EvaluationConfiguration{Ext: map[string]string{
		ExtKeySchedulingMode:     string(SchedulingModeCentral),
		ExtKeySchedulingPriority: "80",
	}}
	err = ApplySchedulingMeta(conf3, SchedulingApplyInput{
		Authorized: true, Authorization: "allow-list:u1", Operator: "u1", Reason: "create", Env: "ppe:lane-a", NowMS: 200,
	})
	require.NoError(t, err)
	meta = GetSchedulingMeta(conf3)
	assert.Equal(t, SchedulingModeCentral, meta.Mode)
	assert.Equal(t, int32(80), meta.Priority)
	assert.Equal(t, "ppe:lane-a", meta.Env)
	require.Len(t, meta.ModeTrace, 1)
	assert.Equal(t, "u1", meta.ModeTrace[0].Operator)
	assert.Equal(t, "allow-list:u1", meta.ModeTrace[0].Authorization)
	require.Len(t, meta.PriorityTrace, 1)
	assert.Equal(t, "80", meta.PriorityTrace[0].To)
}

func TestApplySchedulingMetaFailClosed(t *testing.T) {
	// 跨环境请求拒绝，防止 PPe 泳道互相串用额度池。
	conf := &EvaluationConfiguration{Ext: map[string]string{
		ExtKeySchedulingMode: string(SchedulingModeCentral),
		ExtKeySchedulingEnv:  "ppe:lane-b",
	}}
	err := ApplySchedulingMeta(conf, SchedulingApplyInput{Authorized: true, Env: "ppe:lane-a"})
	require.Error(t, err)

	// 环境不可判定拒绝。
	conf2 := &EvaluationConfiguration{Ext: map[string]string{ExtKeySchedulingMode: string(SchedulingModeCentral)}}
	err = ApplySchedulingMeta(conf2, SchedulingApplyInput{Authorized: true, Env: ""})
	require.Error(t, err)

	// 优先级越界拒绝。
	conf3 := &EvaluationConfiguration{Ext: map[string]string{
		ExtKeySchedulingMode:     string(SchedulingModeLegacy),
		ExtKeySchedulingPriority: "101",
	}}
	err = ApplySchedulingMeta(conf3, SchedulingApplyInput{Env: "prod"})
	require.Error(t, err)
}

func TestSchedulingModeHelpers(t *testing.T) {
	assert.True(t, SchedulingModeCentral.IsCentralDispatchMode())
	assert.False(t, SchedulingModeCentral.IsLegacyLoopMode())
	assert.True(t, SchedulingModeCentralShadow.IsLegacyLoopMode())
	assert.True(t, SchedulingModeLegacy.IsLegacyLoopMode())
	assert.False(t, SchedulingModeLegacy.IsCentralDispatchMode())
}
