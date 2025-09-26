// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPSPct99Metric Tokens Per Second 99分位指标
type ModelTPSPct99Metric struct{}

func (m *ModelTPSPct99Metric) Name() string {
	return entity.MetricNameModelTPSPct99
}

func (m *ModelTPSPct99Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPSPct99Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPSPct99Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.99)(sum(tags_long['input_tokens']+tags_long['output_tokens']) * 1000/sum(duration))"
}

func (m *ModelTPSPct99Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPSPct99Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPSPct99Metric() entity.IMetricDefinition {
	return &ModelTPSPct99Metric{}
}