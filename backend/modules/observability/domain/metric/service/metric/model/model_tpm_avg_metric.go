// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPMAvgMetric Tokens Per Minute平均值指标
type ModelTPMAvgMetric struct{}

func (m *ModelTPMAvgMetric) Name() string {
	return entity.MetricNameModelTPMAvg
}

func (m *ModelTPMAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPMAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPMAvgMetric) Expression(granularity entity.MetricGranularity) string {
	return "avg((tags_long['input_tokens']+tags_long['output_tokens'])/(duration / 60000000))"
}

func (m *ModelTPMAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPMAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPMAvgMetric() entity.IMetricDefinition {
	return &ModelTPMAvgMetric{}
}
