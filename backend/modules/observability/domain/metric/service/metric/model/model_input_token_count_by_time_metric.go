// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelInputTokenCountByTimeMetric Input Tokens消耗时间序列指标
type ModelInputTokenCountByTimeMetric struct{}

func (m *ModelInputTokenCountByTimeMetric) Name() string {
	return entity.MetricNameModelInputTokenCountByTime
}

func (m *ModelInputTokenCountByTimeMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelInputTokenCountByTimeMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelInputTokenCountByTimeMetric) Expression(granularity entity.MetricGranularity) string {
	return "sum(tags_long['input_tokens'])"
}

func (m *ModelInputTokenCountByTimeMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelInputTokenCountByTimeMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelInputTokenCountByTimeMetric() entity.IMetricDefinition {
	return &ModelInputTokenCountByTimeMetric{}
}
