// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPMPct50Metric Tokens Per Minute 50分位指标
type ModelTPMPct50Metric struct{}

func (m *ModelTPMPct50Metric) Name() string {
	return entity.MetricNameModelTPMPct50
}

func (m *ModelTPMPct50Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPMPct50Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPMPct50Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.5)(sum(tags_long['input_tokens']+tags_long['output_tokens']) * 60 * 1000/sum(duration))"
}

func (m *ModelTPMPct50Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPMPct50Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPMPct50Metric() entity.IMetricDefinition {
	return &ModelTPMPct50Metric{}
}