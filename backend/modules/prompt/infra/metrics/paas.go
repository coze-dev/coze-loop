// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/utils/kitexutil"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	promptPaasMetricsPrefix = "prompt_paas"

	paasSpaceTag                = "space_id"
	paasPSMTag                  = "psm"
	paasMethodTag               = "method"
	paasStatusTag               = "status"
	paasOtherTag                = "other"
	paasAccountModeTag          = "account_mode"
	paasStatusCodeTag           = "status_code"
	paasIsErrAffectStabilityTag = "is_err_affect_stability"
	paasModelTag                = "model"
	paasPromptKeyTag            = "prompt_key"
	paasUsageScenarioTag        = "usage_scenario"
	paasVersionTag              = "version"
	paasIsBOETag                = "is_boe"
	paasFeatureTag              = "feature"
	paasSecurityLevelTag        = "security_level"
	paasPSMVerifiedTag          = "psm_verified"
	paasPSMInACLTag             = "psm_in_acl"
	paasUserAllowedTag          = "user_allowed"
	paasPromptTypeTag           = "prompt_type"
	paasHasMessageTag           = "has_message"
	paasHasContexts             = "has_contexts"

	firstTokenLatencySuffix = "first_token_latency"
	inputTokenSuffix        = "input_token"
	outputTokenSuffix       = "output_token"
	maxTokenSuffix          = "max_token"
)

const (
	bizExtraKeyAffectStability = "biz_err_affect_stability"
	unknown                    = "unknown"
)

func promptPaasMtrTags() []string {
	return []string{
		paasSpaceTag,
		paasPromptKeyTag,
		paasPSMTag,
		paasMethodTag,
		paasStatusTag,
		paasOtherTag,
		paasAccountModeTag,
		paasStatusCodeTag,
		paasIsErrAffectStabilityTag,
		paasModelTag,
		paasUsageScenarioTag,
		paasVersionTag,
		paasIsBOETag,
		paasFeatureTag,
		paasPSMVerifiedTag,
		paasPSMInACLTag,
		paasSecurityLevelTag,
		paasUserAllowedTag,
		paasPromptTypeTag,
		paasHasMessageTag,
		paasHasContexts,
	}
}

var (
	promptPaasMetrics         *PromptPaasMetrics
	promptPaasMetricsInitOnce sync.Once
)

func NewPromptPaasMetrics(meter metrics.Meter) *PromptPaasMetrics {
	if meter == nil {
		return nil
	}
	promptPaasMetricsInitOnce.Do(func() {
		metric, err := meter.NewMetric(promptPaasMetricsPrefix, []metrics.MetricType{
			metrics.MetricTypeCounter,
			metrics.MetricTypeTimer,
		}, promptPaasMtrTags())
		if err != nil {
			logs.CtxError(context.Background(), "new prompt paas metrics failed, err = %v", err)
			return
		}
		promptPaasMetrics = &PromptPaasMetrics{metric: metric}
	})
	return promptPaasMetrics
}

type PromptPaasMetrics struct {
	metric metrics.Metric
}

func (m *PromptPaasMetrics) Emit(tags []metrics.T, values ...*metrics.Value) {
	if m == nil || m.metric == nil {
		return
	}
	m.metric.Emit(tags, values...)
}

type paasMetricsCtxKey struct{}

type paasMetricsCtx struct {
	start          time.Time
	firstTokenTime time.Time
	inputToken     int
	outputToken    int
	maxToken       int
	tagMap         map[string]string
}

func NewPaasMetricsCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, paasMetricsCtxKey{}, &paasMetricsCtx{
		start:  time.Now(),
		tagMap: make(map[string]string),
	})
}

func EmitPaasMetric(ctx context.Context) {
	metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx)
	if !ok {
		return
	}
	end := time.Now()
	tsms := end.Sub(metricsCtx.start).Milliseconds()
	method, _ := kitexutil.GetMethod(ctx)
	if method == "" {
		method = unknown
	}
	firstTokenLatency := metricsCtx.firstTokenTime.Sub(metricsCtx.start).Milliseconds()
	if firstTokenLatency < 0 {
		firstTokenLatency = 0
	}

	tags := []metrics.T{
		{Name: paasSpaceTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasSpaceTag], unknown)},
		{Name: paasPromptKeyTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasPromptKeyTag], unknown)},
		{Name: paasPSMTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasPSMTag], method)},
		{Name: paasMethodTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasMethodTag], unknown)},
		{Name: paasStatusTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasStatusTag], unknown)},
		{Name: paasOtherTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasOtherTag], unknown)},
		{Name: paasAccountModeTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasAccountModeTag], unknown)},
		{Name: paasStatusCodeTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasStatusCodeTag], unknown)},
		{Name: paasIsErrAffectStabilityTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasIsErrAffectStabilityTag], unknown)},
		{Name: paasModelTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasModelTag], unknown)},
		{Name: paasUsageScenarioTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasUsageScenarioTag], unknown)},
		{Name: paasVersionTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasVersionTag], unknown)},
		{Name: paasIsBOETag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasIsBOETag], unknown)},
		{Name: paasFeatureTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasFeatureTag], unknown)},
		{Name: paasPSMVerifiedTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasPSMVerifiedTag], unknown)},
		{Name: paasPSMInACLTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasPSMInACLTag], unknown)},
		{Name: paasSecurityLevelTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasSecurityLevelTag], unknown)},
		{Name: paasUserAllowedTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasUserAllowedTag], unknown)},
		{Name: paasPromptTypeTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasPromptTypeTag], unknown)},
		{Name: paasHasMessageTag, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasHasMessageTag], unknown)},
		{Name: paasHasContexts, Value: stringNotEmptyOrDefault(metricsCtx.tagMap[paasHasContexts], unknown)},
	}

	if promptPaasMetrics == nil {
		return
	}
	promptPaasMetrics.Emit(tags,
		metrics.Counter(1),
		metrics.Timer(tsms),
		metrics.Timer(firstTokenLatency, metrics.WithSuffix(firstTokenLatencySuffix)),
		metrics.Counter(int64(metricsCtx.inputToken), metrics.WithSuffix(inputTokenSuffix)),
		metrics.Counter(int64(metricsCtx.outputToken), metrics.WithSuffix(outputTokenSuffix)),
		metrics.Counter(int64(metricsCtx.maxToken), metrics.WithSuffix(maxTokenSuffix)),
	)
}

func WithPaasSpace(ctx context.Context, spaceID int64) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasSpaceTag] = strconv.FormatInt(spaceID, 10)
	}
}

func WithPaasPSM(ctx context.Context, psm string) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasPSMTag] = psm
	}
}

func WithPaasMethod(ctx context.Context, method string) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasMethodTag] = method
	}
}

func WithOther(ctx context.Context, other string) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		if metricsCtx.tagMap[paasOtherTag] != "" {
			metricsCtx.tagMap[paasOtherTag] = metricsCtx.tagMap[paasOtherTag] + "|" + other
		} else {
			metricsCtx.tagMap[paasOtherTag] = other
		}
	}
}

func WithPaaSAccountMode(ctx context.Context, accountMode string) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasAccountModeTag] = accountMode
	}
}

func WithPaaSModel(ctx context.Context, model string) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasModelTag] = model
	}
}

func WithPaasStatus(ctx context.Context, err error) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		if err == nil {
			metricsCtx.tagMap[paasStatusTag] = "success"
			metricsCtx.tagMap[paasStatusCodeTag] = "0"
			metricsCtx.tagMap[paasIsErrAffectStabilityTag] = "0"
		} else {
			bizError, ok := kerrors.FromBizStatusError(err)
			if ok && bizError != nil {
				metricsCtx.tagMap[paasStatusCodeTag] = strconv.FormatInt(int64(bizError.BizStatusCode()), 10)
				metricsCtx.tagMap[paasIsErrAffectStabilityTag] = getIsErrAffectStability(bizError)
			}
			metricsCtx.tagMap[paasStatusTag] = "error"
		}
	}
}

func WithPaasPromptKey(ctx context.Context, promptKey string) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasPromptKeyTag] = promptKey
	}
}

func WithPaasPromptType(ctx context.Context, promptType int64) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasPromptTypeTag] = fmt.Sprintf("%v", promptType)
	}
}

func WithHasMessage(ctx context.Context, hasMessage bool) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasHasMessageTag] = fmt.Sprintf("%v", hasMessage)
	}
}

func WithHasContexts(ctx context.Context, hasContexts bool) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasHasContexts] = fmt.Sprintf("%v", hasContexts)
	}
}

func WithPaasUsageScenario(ctx context.Context, usageScenario string) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasUsageScenarioTag] = usageScenario
	}
}

func WithPaasVersion(ctx context.Context, version string) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasVersionTag] = version
	}
}

func WithPaasIsBOE(ctx context.Context, isBOE bool) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasIsBOETag] = fmt.Sprintf("%t", isBOE)
	}
}

func WithPaasFeature(ctx context.Context, feature string) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasFeatureTag] = feature
	}
}

func WithPaasPSMVerified(ctx context.Context, verified bool) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasPSMVerifiedTag] = fmt.Sprintf("%v", verified)
	}
}

func WithPaasPSMInACL(ctx context.Context, inACL bool) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasPSMInACLTag] = fmt.Sprintf("%v", inACL)
	}
}

func WithPaaSUserAllowed(ctx context.Context, allowed bool) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasUserAllowedTag] = fmt.Sprintf("%v", allowed)
	}
}

func WithPaasSecurityLevel(ctx context.Context, securityLevel string) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.tagMap[paasSecurityLevelTag] = securityLevel
	}
}

func WithPaasFirstTokenTime(ctx context.Context) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.firstTokenTime = time.Now()
	}
}

func WithPaasTokenConsumption(ctx context.Context, inputToken int64, outToken int64) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.inputToken = int(inputToken)
		metricsCtx.outputToken = int(outToken)
	}
}

func WithPaasMaxToken(ctx context.Context, maxToken int32) {
	if metricsCtx, ok := ctx.Value(paasMetricsCtxKey{}).(*paasMetricsCtx); ok {
		metricsCtx.maxToken = int(maxToken)
	}
}

func getIsErrAffectStability(bizError kerrors.BizStatusErrorIface) string {
	bizExtra := bizError.BizExtra()

	affectStability := "1"
	affectStabilityVal, ok := bizExtra[bizExtraKeyAffectStability]
	if ok && affectStabilityVal != "1" {
		affectStability = "0"
	}
	return affectStability
}

func stringNotEmptyOrDefault(s, defaultVal string) string {
	if s != "" {
		return s
	}
	return defaultVal
}
