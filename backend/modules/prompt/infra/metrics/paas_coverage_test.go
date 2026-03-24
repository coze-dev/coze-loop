// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/stretchr/testify/assert"

	inframetrics "github.com/coze-dev/coze-loop/backend/infra/metrics"
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

type captureMetric struct {
	tags   []inframetrics.T
	values []*inframetrics.Value
}

func (m *captureMetric) Emit(tags []inframetrics.T, values ...*inframetrics.Value) {
	m.tags = append([]inframetrics.T(nil), tags...)
	m.values = append([]*inframetrics.Value(nil), values...)
}

type captureMeter struct {
	metric   inframetrics.Metric
	err      error
	name     string
	types    []inframetrics.MetricType
	tagNames []string
}

func (m *captureMeter) NewMetric(name string, types []inframetrics.MetricType, tagNames []string) (inframetrics.Metric, error) {
	m.name = name
	m.types = append([]inframetrics.MetricType(nil), types...)
	m.tagNames = append([]string(nil), tagNames...)
	return m.metric, m.err
}

func resetPromptPaasMetrics() {
	promptPaasMetrics = nil
	promptPaasMetricsInitOnce = sync.Once{}
}

func tagValue(tags []inframetrics.T, name string) string {
	for _, tag := range tags {
		if tag.Name == name {
			return tag.Value
		}
	}
	return ""
}

func TestPromptPaasMetricsCreationAndEmit(t *testing.T) {
	resetPromptPaasMetrics()
	t.Cleanup(resetPromptPaasMetrics)

	assert.Nil(t, NewPromptPaasMetrics(nil))

	resetPromptPaasMetrics()
	errMeter := &captureMeter{err: errors.New("new metric error")}
	assert.Nil(t, NewPromptPaasMetrics(errMeter))

	resetPromptPaasMetrics()
	metric := &captureMetric{}
	meter := &captureMeter{metric: metric}

	got := NewPromptPaasMetrics(meter)
	assert.NotNil(t, got)
	assert.Equal(t, promptPaasMetricsPrefix, meter.name)
	assert.Equal(t, []inframetrics.MetricType{inframetrics.MetricTypeCounter, inframetrics.MetricTypeTimer}, meter.types)
	assert.Equal(t, promptPaasMtrTags(), meter.tagNames)

	got.Emit([]inframetrics.T{{Name: "space_id", Value: "1"}}, inframetrics.Counter(1))
	assert.Len(t, metric.tags, 1)
	assert.Equal(t, "space_id", metric.tags[0].Name)
	assert.Len(t, metric.values, 1)
	assert.Equal(t, inframetrics.MetricTypeCounter, metric.values[0].GetType())
	assert.Equal(t, int64(1), *metric.values[0].GetValue())
}

func TestWithPaasStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := NewPaasMetricsCtx(context.Background())
		WithPaasStatus(ctx, nil)

		mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
		assert.Equal(t, "success", mc.tagMap[paasStatusTag])
		assert.Equal(t, "0", mc.tagMap[paasStatusCodeTag])
		assert.Equal(t, "0", mc.tagMap[paasIsErrAffectStabilityTag])
	})

	t.Run("biz error", func(t *testing.T) {
		ctx := NewPaasMetricsCtx(context.Background())
		WithPaasStatus(ctx, kerrors.NewBizStatusErrorWithExtra(4001, "biz err", map[string]string{
			bizExtraKeyAffectStability: "0",
		}))

		mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
		assert.Equal(t, "error", mc.tagMap[paasStatusTag])
		assert.Equal(t, "4001", mc.tagMap[paasStatusCodeTag])
		assert.Equal(t, "0", mc.tagMap[paasIsErrAffectStabilityTag])
	})
}

func TestEmitPaasMetric(t *testing.T) {
	resetPromptPaasMetrics()
	t.Cleanup(resetPromptPaasMetrics)

	metric := &captureMetric{}
	promptPaasMetrics = &PromptPaasMetrics{metric: metric}

	ctx := NewPaasMetricsCtx(context.Background())
	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	mc.start = time.Now().Add(-2 * time.Second)
	mc.firstTokenTime = mc.start.Add(500 * time.Millisecond)

	WithPaasSpace(ctx, 100)
	WithPaasPSM(ctx, "prompt.psm")
	WithPaasMethod(ctx, "Execute")
	WithOther(ctx, "from_test")
	WithPaaSAccountMode(ctx, "shared")
	WithPaasPromptKey(ctx, "prompt_key")
	WithPaasPromptType(ctx, 2)
	WithHasMessage(ctx, true)
	WithHasContexts(ctx, false)
	WithPaasUsageScenario(ctx, "debug")
	WithPaasVersion(ctx, "v1.0.0")
	WithPaasIsBOE(ctx, true)
	WithPaasFeature(ctx, "streaming")
	WithPaasPSMVerified(ctx, true)
	WithPaasPSMInACL(ctx, false)
	WithPaaSUserAllowed(ctx, true)
	WithPaasSecurityLevel(ctx, "L3")
	WithPaasTokenConsumption(ctx, 12, 34)
	WithPaasMaxToken(ctx, 4096)
	WithPaasStatus(ctx, kerrors.NewBizStatusError(5001, "failed"))

	EmitPaasMetric(ctx)

	assert.Equal(t, "100", tagValue(metric.tags, paasSpaceTag))
	assert.Equal(t, "prompt_key", tagValue(metric.tags, paasPromptKeyTag))
	assert.Equal(t, "prompt.psm", tagValue(metric.tags, paasPSMTag))
	assert.Equal(t, "Execute", tagValue(metric.tags, paasMethodTag))
	assert.Equal(t, "error", tagValue(metric.tags, paasStatusTag))
	assert.Equal(t, "5001", tagValue(metric.tags, paasStatusCodeTag))
	assert.Equal(t, "debug", tagValue(metric.tags, paasUsageScenarioTag))
	assert.Equal(t, "v1.0.0", tagValue(metric.tags, paasVersionTag))
	assert.Equal(t, "2", tagValue(metric.tags, paasPromptTypeTag))
	assert.Equal(t, "true", tagValue(metric.tags, paasHasMessageTag))
	assert.Equal(t, "false", tagValue(metric.tags, paasHasContexts))

	assert.Len(t, metric.values, 6)
	assert.Equal(t, inframetrics.MetricTypeCounter, metric.values[0].GetType())
	assert.Equal(t, int64(1), *metric.values[0].GetValue())
	assert.Equal(t, inframetrics.MetricTypeTimer, metric.values[1].GetType())
	assert.Equal(t, firstTokenLatencySuffix, metric.values[2].GetSuffix())
	assert.Equal(t, inputTokenSuffix, metric.values[3].GetSuffix())
	assert.Equal(t, int64(12), *metric.values[3].GetValue())
	assert.Equal(t, outputTokenSuffix, metric.values[4].GetSuffix())
	assert.Equal(t, int64(34), *metric.values[4].GetValue())
	assert.Equal(t, maxTokenSuffix, metric.values[5].GetSuffix())
	assert.Equal(t, int64(4096), *metric.values[5].GetValue())
}

func TestEmitPaasMetric_NoMetricsCtx(t *testing.T) {
	assert.NotPanics(t, func() {
		EmitPaasMetric(context.Background())
	})
}

func TestEmitPaasMetric_NilPromptPaasMetrics(t *testing.T) {
	resetPromptPaasMetrics()
	t.Cleanup(resetPromptPaasMetrics)

	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasSpace(ctx, 1)
	WithPaasStatus(ctx, nil)

	assert.NotPanics(t, func() {
		EmitPaasMetric(ctx)
	})
}

func TestEmitPaasMetric_NegativeFirstTokenLatency(t *testing.T) {
	resetPromptPaasMetrics()
	t.Cleanup(resetPromptPaasMetrics)

	metric := &captureMetric{}
	promptPaasMetrics = &PromptPaasMetrics{metric: metric}

	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasStatus(ctx, nil)

	EmitPaasMetric(ctx)

	assert.NotNil(t, metric.values)
	assert.Equal(t, firstTokenLatencySuffix, metric.values[2].GetSuffix())
	assert.Equal(t, int64(0), *metric.values[2].GetValue())
}

func TestPromptPaasMetrics_Emit_NilReceiver(t *testing.T) {
	var m *PromptPaasMetrics
	assert.NotPanics(t, func() {
		m.Emit([]inframetrics.T{{Name: "k", Value: "v"}}, inframetrics.Counter(1))
	})
}

func TestPromptPaasMetrics_Emit_NilMetric(t *testing.T) {
	m := &PromptPaasMetrics{metric: nil}
	assert.NotPanics(t, func() {
		m.Emit([]inframetrics.T{{Name: "k", Value: "v"}}, inframetrics.Counter(1))
	})
}

func TestWithPaasStatus_RegularError(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasStatus(ctx, errors.New("plain error"))

	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, "error", mc.tagMap[paasStatusTag])
	assert.Empty(t, mc.tagMap[paasStatusCodeTag])
	assert.Empty(t, mc.tagMap[paasIsErrAffectStabilityTag])
}

func TestWithPaasStatus_NoMetricsCtx(t *testing.T) {
	assert.NotPanics(t, func() {
		WithPaasStatus(context.Background(), errors.New("err"))
	})
}

func TestGetIsErrAffectStability_AffectStabilityIsOne(t *testing.T) {
	ctx := NewPaasMetricsCtx(context.Background())
	WithPaasStatus(ctx, kerrors.NewBizStatusErrorWithExtra(5000, "err", map[string]string{
		bizExtraKeyAffectStability: "1",
	}))

	mc := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	assert.Equal(t, "error", mc.tagMap[paasStatusTag])
	assert.Equal(t, "5000", mc.tagMap[paasStatusCodeTag])
	assert.Equal(t, "1", mc.tagMap[paasIsErrAffectStabilityTag])
}

func TestWithPaas_NoMetricsCtx_AllFunctions(t *testing.T) {
	ctx := context.Background()
	assert.NotPanics(t, func() {
		WithPaasSpace(ctx, 1)
		WithPaasMethod(ctx, "m")
		WithPaasPromptKey(ctx, "k")
		WithPaasPromptType(ctx, 1)
		WithHasMessage(ctx, true)
		WithHasContexts(ctx, true)
		WithPaasUsageScenario(ctx, "s")
		WithPaasVersion(ctx, "v")
		WithPaasStatus(ctx, nil)
	})
}
