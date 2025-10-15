// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTokenCountByTimeMetric Tokens消耗时间序列指标
type ModelTokenCountByTimeMetric struct{}

func (m *ModelTokenCountByTimeMetric) Name() string {
	return entity.MetricNameModelTokenCountByTime
}

func (m *ModelTokenCountByTimeMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTokenCountByTimeMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTokenCountByTimeMetric) Expression(granularity entity.MetricGranularity) string {
	return "sum(tags_long['input_tokens'] + tags_long['output_tokens'])"
}

func (m *ModelTokenCountByTimeMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTokenCountByTimeMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTokenCountByTimeMetric() entity.IMetricDefinition {
	return &ModelTokenCountByTimeMetric{}
}
