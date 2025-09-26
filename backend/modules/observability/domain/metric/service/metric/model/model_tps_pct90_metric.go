// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPSPct90Metric Tokens Per Second 90分位指标
type ModelTPSPct90Metric struct{}

func (m *ModelTPSPct90Metric) Name() string {
	return entity.MetricNameModelTPSPct90
}

func (m *ModelTPSPct90Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPSPct90Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPSPct90Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.9)(sum(tags_long['input_tokens']+tags_long['output_tokens']) * 1000/sum(duration))"
}

func (m *ModelTPSPct90Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPSPct90Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPSPct90Metric() entity.IMetricDefinition {
	return &ModelTPSPct90Metric{}
}