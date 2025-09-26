// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPOTMaxMetric Time Per Output Token最大值指标
type ModelTPOTMaxMetric struct{}

func (m *ModelTPOTMaxMetric) Name() string {
	return entity.MetricNameModelTPOTMax
}

func (m *ModelTPOTMaxMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPOTMaxMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPOTMaxMetric) Expression(granularity entity.MetricGranularity) string {
	return "max(tags_long['output_tokens'] / (duration-tags_long['latency_first_resp']))"
}

func (m *ModelTPOTMaxMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPOTMaxMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPOTMaxMetric() entity.IMetricDefinition {
	return &ModelTPOTMaxMetric{}
}