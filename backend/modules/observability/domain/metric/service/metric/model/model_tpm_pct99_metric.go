// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPMPct99Metric Tokens Per Minute 99分位指标
type ModelTPMPct99Metric struct{}

func (m *ModelTPMPct99Metric) Name() string {
	return entity.MetricNameModelTPMPct99
}

func (m *ModelTPMPct99Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPMPct99Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPMPct99Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.99)((tags_long['input_tokens']+tags_long['output_tokens'])/(duration / 60000000))"
}

func (m *ModelTPMPct99Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPMPct99Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPMPct99Metric() entity.IMetricDefinition {
	return &ModelTPMPct99Metric{}
}
