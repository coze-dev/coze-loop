// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStringNotEmptyOrDefault(t *testing.T) {
	t.Run("non-empty returns s", func(t *testing.T) {
		assert.Equal(t, "hello", stringNotEmptyOrDefault("hello", "default"))
	})

	t.Run("empty returns defaultVal", func(t *testing.T) {
		assert.Equal(t, "default", stringNotEmptyOrDefault("", "default"))
	})
}

func TestWithPaasPSM(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasPSM(ctx, "test_psm")
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, "test_psm", mc.tagMap["psm"])
}

func TestWithPaaSAccountMode(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaaSAccountMode(ctx, "mode1")
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, "mode1", mc.tagMap["account_mode"])
}

func TestWithPaaSModel(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaaSModel(ctx, "gpt4")
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, "gpt4", mc.tagMap["model"])
}

func TestWithPaasIsBOE(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasIsBOE(ctx, true)
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, "true", mc.tagMap["is_boe"])
}

func TestWithPaasFeature(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasFeature(ctx, "feat1")
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, "feat1", mc.tagMap["feature"])
}

func TestWithPaasPSMVerified(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasPSMVerified(ctx, true)
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, "true", mc.tagMap["psm_verified"])
}

func TestWithPaasPSMInACL(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasPSMInACL(ctx, false)
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, "false", mc.tagMap["psm_in_acl"])
}

func TestWithPaaSUserAllowed(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaaSUserAllowed(ctx, true)
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, "true", mc.tagMap["user_allowed"])
}

func TestWithPaasSecurityLevel(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasSecurityLevel(ctx, "L3")
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, "L3", mc.tagMap["security_level"])
}

func TestWithPaasFirstTokenTime(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasFirstTokenTime(ctx)
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.False(t, mc.firstTokenTime.IsZero())
}

func TestWithPaasTokenConsumption(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasTokenConsumption(ctx, 100, 200)
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, 100, mc.inputToken)
	assert.Equal(t, 200, mc.outputToken)
}

func TestWithPaasMaxToken(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasMaxToken(ctx, 4096)
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, 4096, mc.maxToken)
}

func TestWithOther_Concatenation(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())

	WithOther(ctx, "a")
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, "a", mc.tagMap["other"])

	WithOther(ctx, "b")
	assert.Equal(t, "a|b", mc.tagMap["other"])
}

func TestNewPaasMetricsCtx_AlreadyExists(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	mc1 := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)

	ctx2 := NewPaasMetricsCtx(ctx)
	mc2 := ctx2.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)

	assert.Same(t, mc1, mc2)
}

func TestNewPaasMetricsCtx_StartTimeSet(t *testing.T) {
	before := time.Now()
	ctx := NewPaasMetricsCtx(context.Background())
	after := time.Now()

	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.False(t, mc.start.Before(before))
	assert.False(t, mc.start.After(after))
}

func TestWithPaas_NoMetricsCtx_NoPanic(t *testing.T) {
	ctx := context.Background()
	assert.NotPanics(t, func() {
		WithPaasPSM(ctx, "psm")
		WithPaaSAccountMode(ctx, "mode")
		WithPaaSModel(ctx, "model")
		WithPaasIsBOE(ctx, true)
		WithPaasFeature(ctx, "feat")
		WithPaasPSMVerified(ctx, true)
		WithPaasPSMInACL(ctx, false)
		WithPaaSUserAllowed(ctx, true)
		WithPaasSecurityLevel(ctx, "L1")
		WithPaasFirstTokenTime(ctx)
		WithPaasTokenConsumption(ctx, 1, 2)
		WithPaasMaxToken(ctx, 100)
		WithOther(ctx, "x")
	})
}
