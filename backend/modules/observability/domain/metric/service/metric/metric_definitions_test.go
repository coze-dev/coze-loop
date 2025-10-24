// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metric_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	generalmetrics "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/general"
	modelmetrics "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/model"
	servicemetrics "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/service"
	toolmetrics "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/tool"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TestMetricDefinitionsUniqueNames(t *testing.T) {
	baseDefs := collectBaseMetricDefinitions()
	expanded := expandMetricDefinitions(baseDefs)
	names := make(map[string]string)
	for _, def := range expanded {
		name := def.Name()
		require.NotEmpty(t, name)
		if prev, ok := names[name]; ok {
			t.Fatalf("duplicate metric name %s for %s and %s", name, fmt.Sprintf("%T", def), prev)
		}
		names[name] = fmt.Sprintf("%T", def)
	}
}

func TestMetricExpressions(t *testing.T) {
	baseDefs := collectBaseMetricDefinitions()
	expanded := expandMetricDefinitions(baseDefs)
	granularities := []entity.MetricGranularity{entity.MetricGranularity1Min, entity.MetricGranularity1Hour}
	baseExprs := make(map[entity.MetricGranularity]map[string]string)
	expandedExprs := make(map[entity.MetricGranularity]map[string]string)
	for _, gran := range granularities {
		baseExprs[gran] = renderExpressions(t, baseDefs, gran)
		expandedExprs[gran] = renderExpressions(t, expanded, gran)
	}

	for name := range baseExprs[entity.MetricGranularity1Hour] {
		if _, ok := baseExpressionGenerators[name]; !ok {
			t.Fatalf("missing expected expression generator for %s", name)
		}
	}

	for _, gran := range granularities {
		for name, expr := range baseExprs[gran] {
			require.NotEmpty(t, expr, "unexpected empty expression for %s", name)
			if expected, ok := expectedBaseExpression(name, gran); ok {
				assert.Equal(t, expected, expr, "unexpected expression for %s with granularity %s", name, gran)
			}
		}
	}

	for _, gran := range granularities {
		for name, expr := range expandedExprs[gran] {
			switch {
			case strings.HasSuffix(name, "_avg"):
				baseName := strings.TrimSuffix(name, "_avg")
				baseExpr, ok := baseExprs[gran][baseName]
				if !ok {
					continue
				}
				expected := fmt.Sprintf("avg(%s)", baseExpr)
				assert.Equal(t, expected, expr, "avg wrapper expression mismatch for %s", name)
			case strings.HasSuffix(name, "_min"):
				baseName := strings.TrimSuffix(name, "_min")
				baseExpr, ok := baseExprs[gran][baseName]
				if !ok {
					continue
				}
				expected := fmt.Sprintf("min(%s)", baseExpr)
				assert.Equal(t, expected, expr, "min wrapper expression mismatch for %s", name)
			case strings.HasSuffix(name, "_max"):
				baseName := strings.TrimSuffix(name, "_max")
				baseExpr, ok := baseExprs[gran][baseName]
				if !ok {
					continue
				}
				expected := fmt.Sprintf("max(%s)", baseExpr)
				assert.Equal(t, expected, expr, "max wrapper expression mismatch for %s", name)
			case strings.HasSuffix(name, "_pct50"):
				baseName := strings.TrimSuffix(name, "_pct50")
				baseExpr, ok := baseExprs[gran][baseName]
				if !ok {
					continue
				}
				expected := fmt.Sprintf("quantile(0.5)(%s)", baseExpr)
				assert.Equal(t, expected, expr, "pct50 wrapper expression mismatch for %s", name)
			case strings.HasSuffix(name, "_pct90"):
				baseName := strings.TrimSuffix(name, "_pct90")
				baseExpr, ok := baseExprs[gran][baseName]
				if !ok {
					continue
				}
				expected := fmt.Sprintf("quantile(0.9)(%s)", baseExpr)
				assert.Equal(t, expected, expr, "pct90 wrapper expression mismatch for %s", name)
			case strings.HasSuffix(name, "_pct99"):
				baseName := strings.TrimSuffix(name, "_pct99")
				baseExpr, ok := baseExprs[gran][baseName]
				if !ok {
					continue
				}
				expected := fmt.Sprintf("quantile(0.99)(%s)", baseExpr)
				assert.Equal(t, expected, expr, "pct99 wrapper expression mismatch for %s", name)
			case strings.HasSuffix(name, "_by_time"):
				baseName := strings.TrimSuffix(name, "_by_time")
				baseExpr, ok := baseExprs[gran][baseName]
				if !ok {
					continue
				}
				assert.Equal(t, baseExpr, expr, "time series wrapper expression mismatch for %s", name)
			}
		}
	}
}

func collectBaseMetricDefinitions() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		generalmetrics.NewGeneralTotalCountMetric(),
		generalmetrics.NewGeneralFailRatioMetric(),
		generalmetrics.NewGeneralModelFailRatioMetric(),
		generalmetrics.NewGeneralModelLatencyMetric(),
		generalmetrics.NewGeneralModelTotalTokensMetric(),
		generalmetrics.NewGeneralToolFailRatioMetric(),
		generalmetrics.NewGeneralToolLatencyMetric(),
		generalmetrics.NewGeneralToolTotalCountMetric(),
		modelmetrics.NewModelDurationMetric(),
		modelmetrics.NewModelInputTokenCountMetric(),
		modelmetrics.NewModelNamePieMetric(),
		modelmetrics.NewModelQPMAllMetric(),
		modelmetrics.NewModelQPMFailMetric(),
		modelmetrics.NewModelQPMSuccessMetric(),
		modelmetrics.NewModelQPSAllMetric(),
		modelmetrics.NewModelQPSFailMetric(),
		modelmetrics.NewModelQPSSuccessMetric(),
		modelmetrics.NewModelSuccessRatioMetric(),
		modelmetrics.NewModelSystemTokenCountMetric(),
		modelmetrics.NewModelTokenCountMetric(),
		modelmetrics.NewModelTokenCountPieMetric(),
		modelmetrics.NewModelToolChoiceTokenCountMetric(),
		modelmetrics.NewModelTPMMetric(),
		modelmetrics.NewModelTPOTMetric(),
		modelmetrics.NewModelTPSMetric(),
		modelmetrics.NewModelTTFTMetric(),
		servicemetrics.NewServiceDurationMetric(),
		servicemetrics.NewServiceExecutionStepCountMetric(),
		servicemetrics.NewServiceMessageCountMetric(),
		servicemetrics.NewServiceQPMAllMetric(),
		servicemetrics.NewServiceQPMFailMetric(),
		servicemetrics.NewServiceQPMSuccessMetric(),
		servicemetrics.NewServiceQPSAllMetric(),
		servicemetrics.NewServiceQPSFailMetric(),
		servicemetrics.NewServiceQPSSuccessMetric(),
		servicemetrics.NewServiceSpanCountMetric(),
		servicemetrics.NewServiceSuccessRatioMetric(),
		servicemetrics.NewServiceTraceCountMetric(),
		servicemetrics.NewServiceUserCountMetric(),
		toolmetrics.NewToolDurationMetric(),
		toolmetrics.NewToolNamePieMetric(),
		toolmetrics.NewToolSuccessRatioMetric(),
		toolmetrics.NewToolTotalCountMetric(),
	}
}

func expandMetricDefinitions(defs []entity.IMetricDefinition) []entity.IMetricDefinition {
	result := make([]entity.IMetricDefinition, 0)
	for _, def := range defs {
		if adapter, ok := def.(entity.IMetricAdapter); ok {
			for _, wrapper := range adapter.Wrappers() {
				result = append(result, wrapper.Wrap(def))
			}
		} else {
			result = append(result, def)
		}
	}
	return result
}

func renderExpressions(t *testing.T, defs []entity.IMetricDefinition, gran entity.MetricGranularity) map[string]string {
	t.Helper()
	res := make(map[string]string)
	for _, def := range defs {
		_ = def.Type()
		_ = def.GroupBy()
		_ = def.Source()
		res[def.Name()] = renderExpression(t, def, gran)
	}
	return res
}

func renderExpression(t *testing.T, def entity.IMetricDefinition, gran entity.MetricGranularity) string {
	t.Helper()
	expr := def.Expression(gran)
	require.NotNil(t, expr, "expression should not be nil for %s", def.Name())
	require.NotEmpty(t, expr.Expression, "expression string should not be empty for %s", def.Name())
	placeholderCount := strings.Count(expr.Expression, "%s")
	require.Equal(t, placeholderCount, len(expr.Fields), "placeholder count mismatch for %s", def.Name())
	args := make([]any, len(expr.Fields))
	for i, field := range expr.Fields {
		require.NotEmpty(t, field.FieldName, "field name should not be empty for %s", def.Name())
		args[i] = field.FieldName
	}
	rendered := expr.Expression
	if len(args) > 0 {
		rendered = fmt.Sprintf(expr.Expression, args...)
	}
	for _, field := range expr.Fields {
		assert.Contains(t, rendered, field.FieldName, "rendered expression missing field %s for %s", field.FieldName, def.Name())
	}
	return rendered
}

func expectedBaseExpression(name string, gran entity.MetricGranularity) (string, bool) {
	if generator, ok := baseExpressionGenerators[name]; ok {
		return generator(gran), true
	}
	return "", false
}

var baseExpressionGenerators = map[string]func(entity.MetricGranularity) string{
	entity.MetricNameGeneralTotalCount:         countExpr,
	entity.MetricNameGeneralToolTotalCount:     countExpr,
	entity.MetricNameServiceTraceCount:         countExpr,
	entity.MetricNameServiceSpanCount:          countExpr,
	entity.MetricNameToolTotalCount:            countExpr,
	entity.MetricNameServiceExecutionStepCount: countExpr,
	entity.MetricNameGeneralFailRatio:          failRatioExpr,
	entity.MetricNameGeneralModelFailRatio:     failRatioExpr,
	entity.MetricNameGeneralToolFailRatio:      failRatioExpr,
	entity.MetricNameGeneralModelLatencyAvg:    sumDurationAvgExpr,
	entity.MetricNameGeneralToolLatencyAvg:     sumDurationAvgExpr,
	entity.MetricNameGeneralModelTotalTokens:   sumInputOutputTokensExpr,
	entity.MetricNameModelTokenCount:           sumInputOutputTokensExpr,
	entity.MetricNameModelTokenCountPie:        sumInputOutputTokensExpr,
	entity.MetricNameModelDuration:             durationMillisExpr(loop_span.SpanFieldDuration),
	entity.MetricNameServiceDuration:           durationMillisExpr(loop_span.SpanFieldDuration),
	entity.MetricNameToolDuration:              durationMillisExpr(loop_span.SpanFieldDuration),
	entity.MetricNameModelTTFT:                 durationMillisExpr(loop_span.SpanFieldLatencyFirstResp),
	entity.MetricNameModelInputTokenCount:      sumFieldExpr(loop_span.SpanFieldInputTokens),
	entity.MetricNameModelSystemTokenCount:     sumFieldExpr("model_system_tokens"),
	entity.MetricNameModelToolChoiceTokenCount: sumFieldExpr("model_tool_choice_tokens"),
	entity.MetricNameModelNamePie:              constantExpr("1"),
	entity.MetricNameToolNamePie:               constantExpr("1"),
	entity.MetricNameModelSuccessRatio:         successRatioExpr,
	entity.MetricNameServiceSuccessRatio:       successRatioExpr,
	entity.MetricNameToolSuccessRatio:          successRatioExpr,
	entity.MetricNameModelTPM:                  tokenThroughputExpr(60000000),
	entity.MetricNameModelTPOT:                 tpotExpr,
	entity.MetricNameModelTPS:                  tokenThroughputExpr(1000000),
	entity.MetricNameModelQPMAll:               qpmAllExpr,
	entity.MetricNameServiceQPMAll:             qpmAllExpr,
	entity.MetricNameModelQPMFail:              qpmFailExpr,
	entity.MetricNameServiceQPMFail:            qpmFailExpr,
	entity.MetricNameModelQPMSuccess:           qpmSuccessExpr,
	entity.MetricNameServiceQPMSuccess:         qpmSuccessExpr,
	entity.MetricNameModelQPSAll:               qpsAllExpr,
	entity.MetricNameServiceQPSAll:             qpsAllExpr,
	entity.MetricNameModelQPSFail:              qpsFailExpr,
	entity.MetricNameServiceQPSFail:            qpsFailExpr,
	entity.MetricNameModelQPSSuccess:           qpsSuccessExpr,
	entity.MetricNameServiceQPSSuccess:         qpsSuccessExpr,
	entity.MetricNameServiceMessageCount:       uniqFieldExpr(loop_span.SpanFieldMessageID),
	entity.MetricNameServiceUserCount:          uniqFieldExpr(loop_span.SpanFieldUserID),
}

func countExpr(entity.MetricGranularity) string {
	return "count()"
}

func failRatioExpr(entity.MetricGranularity) string {
	return fmt.Sprintf("countIf(1, %s != 0) / count()", loop_span.SpanFieldStatusCode)
}

func successRatioExpr(entity.MetricGranularity) string {
	return fmt.Sprintf("countIf(1, %s = 0) / count()", loop_span.SpanFieldStatusCode)
}

func sumDurationAvgExpr(entity.MetricGranularity) string {
	return fmt.Sprintf("sum(%s) / (1000 * count())", loop_span.SpanFieldDuration)
}

func sumInputOutputTokensExpr(entity.MetricGranularity) string {
	return fmt.Sprintf("sum(%s + %s)", loop_span.SpanFieldInputTokens, loop_span.SpanFieldOutputTokens)
}

func durationMillisExpr(field string) func(entity.MetricGranularity) string {
	return func(entity.MetricGranularity) string {
		return fmt.Sprintf("%s/1000", field)
	}
}

func sumFieldExpr(field string) func(entity.MetricGranularity) string {
	return func(entity.MetricGranularity) string {
		return fmt.Sprintf("sum(%s)", field)
	}
}

func uniqFieldExpr(field string) func(entity.MetricGranularity) string {
	return func(entity.MetricGranularity) string {
		return fmt.Sprintf("uniq(%s)", field)
	}
}

func constantExpr(value string) func(entity.MetricGranularity) string {
	return func(entity.MetricGranularity) string { return value }
}

func tokenThroughputExpr(divisor int64) func(entity.MetricGranularity) string {
	return func(entity.MetricGranularity) string {
		return fmt.Sprintf("(%s+%s)/(%s / %d)", loop_span.SpanFieldInputTokens, loop_span.SpanFieldOutputTokens, loop_span.SpanFieldDuration, divisor)
	}
}

func tpotExpr(entity.MetricGranularity) string {
	return fmt.Sprintf("(%s-%s)/(1000*%s)", loop_span.SpanFieldDuration, loop_span.SpanFieldLatencyFirstResp, loop_span.SpanFieldOutputTokens)
}

func qpmAllExpr(gran entity.MetricGranularity) string {
	den := entity.GranularityToSecond(gran) / 60
	return fmt.Sprintf("count()/%d", den)
}

func qpmFailExpr(gran entity.MetricGranularity) string {
	den := entity.GranularityToSecond(gran) / 60
	return fmt.Sprintf("countIf(1, %s != 0)/%d", loop_span.SpanFieldStatusCode, den)
}

func qpmSuccessExpr(gran entity.MetricGranularity) string {
	den := entity.GranularityToSecond(gran) / 60
	return fmt.Sprintf("countIf(1, %s = 0)/%d", loop_span.SpanFieldStatusCode, den)
}

func qpsAllExpr(gran entity.MetricGranularity) string {
	den := entity.GranularityToSecond(gran)
	return fmt.Sprintf("count()/%d", den)
}

func qpsFailExpr(gran entity.MetricGranularity) string {
	den := entity.GranularityToSecond(gran)
	return fmt.Sprintf("countIf(1, %s != 0)/%d", loop_span.SpanFieldStatusCode, den)
}

func qpsSuccessExpr(gran entity.MetricGranularity) string {
	den := entity.GranularityToSecond(gran)
	return fmt.Sprintf("countIf(1, %s = 0)/%d", loop_span.SpanFieldStatusCode, den)
}
