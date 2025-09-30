// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPMPct90Metric Tokens Per Minute 90分位指标
type ModelTPMPct90Metric struct{}

func (m *ModelTPMPct90Metric) Name() string {
	return entity.MetricNameModelTPMPct90
}

func (m *ModelTPMPct90Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPMPct90Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPMPct90Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.9)((tags_long['input_tokens']+tags_long['output_tokens'])/(duration / 60000000))"
}

func (m *ModelTPMPct90Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPMPct90Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPMPct90Metric() entity.IMetricDefinition {
	return &ModelTPMPct90Metric{}
}
