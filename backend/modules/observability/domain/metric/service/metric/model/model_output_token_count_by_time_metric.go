// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelOutputTokenCountByTimeMetric Output Tokens消耗时间序列指标
type ModelOutputTokenCountByTimeMetric struct{}

func (m *ModelOutputTokenCountByTimeMetric) Name() string {
	return entity.MetricNameModelOutputTokenCountByTime
}

func (m *ModelOutputTokenCountByTimeMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelOutputTokenCountByTimeMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelOutputTokenCountByTimeMetric) Expression(granularity entity.MetricGranularity) string {
	return "sum(tags_long['output_tokens'])"
}

func (m *ModelOutputTokenCountByTimeMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelOutputTokenCountByTimeMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelOutputTokenCountByTimeMetric() entity.IMetricDefinition {
	return &ModelOutputTokenCountByTimeMetric{}
}
